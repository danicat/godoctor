// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package godoc implements the core logic for retrieving and parsing Go documentation.
package godoc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/danicat/godoctor/internal/safeshell"
	"github.com/danicat/godoctor/internal/text"
)

// Cache structures for in-memory caching of parsed docs and resolved paths
type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

func (e cacheEntry[T]) isExpired() bool {
	return !e.expiresAt.IsZero() && time.Now().After(e.expiresAt)
}

type memoryCache struct {
	mu         sync.RWMutex
	docs       map[string]cacheEntry[*Doc]
	dirs       map[string]cacheEntry[string]
	subpkgs    map[string]cacheEntry[[]string]
	defaultTTL time.Duration
	enabled    bool
}

var (
	globalCache = &memoryCache{
		docs:       make(map[string]cacheEntry[*Doc]),
		dirs:       make(map[string]cacheEntry[string]),
		subpkgs:    make(map[string]cacheEntry[[]string]),
		defaultTTL: 15 * time.Minute,
		enabled:    true,
	}

	stdlibPackages []string
	stdlibMu       sync.RWMutex
)

func (c *memoryCache) getDoc(key string) (*Doc, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.enabled {
		return nil, false
	}
	entry, ok := c.docs[key]
	if !ok || entry.isExpired() {
		return nil, false
	}
	return entry.value.Clone(), true
}

func (c *memoryCache) setDoc(key string, doc *Doc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.enabled || doc == nil {
		return
	}
	var expiresAt time.Time
	if c.defaultTTL > 0 {
		expiresAt = time.Now().Add(c.defaultTTL)
	}
	c.docs[key] = cacheEntry[*Doc]{
		value:     doc.Clone(),
		expiresAt: expiresAt,
	}
}

func (c *memoryCache) getDir(pkgPath string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.enabled {
		return "", false
	}
	entry, ok := c.dirs[pkgPath]
	if !ok || entry.isExpired() {
		return "", false
	}
	return entry.value, true
}

func (c *memoryCache) setDir(pkgPath, dir string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.enabled {
		return
	}
	var expiresAt time.Time
	if c.defaultTTL > 0 {
		expiresAt = time.Now().Add(c.defaultTTL)
	}
	c.dirs[pkgPath] = cacheEntry[string]{
		value:     dir,
		expiresAt: expiresAt,
	}
}

func (c *memoryCache) getSubpkgs(pkgDir string) ([]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.enabled {
		return nil, false
	}
	entry, ok := c.subpkgs[pkgDir]
	if !ok || entry.isExpired() {
		return nil, false
	}
	return append([]string(nil), entry.value...), true
}

func (c *memoryCache) setSubpkgs(pkgDir string, subs []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.enabled {
		return
	}
	var expiresAt time.Time
	if c.defaultTTL > 0 {
		expiresAt = time.Now().Add(c.defaultTTL)
	}
	c.subpkgs[pkgDir] = cacheEntry[[]string]{
		value:     append([]string(nil), subs...),
		expiresAt: expiresAt,
	}
}

// LoadWithFallback attempts to resolve an import path and return documentation.
// It attempts to find parent packages if the exact match fails.
func LoadWithFallback(ctx context.Context, pkgPath, symbolName string) (*Doc, error) {
	return loadInternal(ctx, pkgPath, symbolName, true)
}

func docCacheKey(pkgPath, symbolName string, allowFallback bool) string {
	return fmt.Sprintf("%s|%s|%t", pkgPath, symbolName, allowFallback)
}

func loadInternal(ctx context.Context, pkgPath, symbolName string, allowFallback bool) (*Doc, error) {
	key := docCacheKey(pkgPath, symbolName, allowFallback)
	if cached, ok := globalCache.getDoc(key); ok {
		return cached, nil
	}

	// Try to find the package directory locally
	pkgDir, err := resolvePackageDir(ctx, pkgPath)
	if err != nil {
		// Fallback: try to fetch the package in a temp directory
		doc, fetchErr := fetchAndRetryStructured(ctx, pkgPath, symbolName, err)
		if fetchErr == nil {
			globalCache.setDoc(key, doc)
			return doc, nil
		}

		if allowFallback {
			// Try walking up the path
			parts := strings.Split(pkgPath, "/")
			minParts := 1
			if strings.Contains(parts[0], ".") {
				minParts = 3
			}

			for i := len(parts) - 1; i >= minParts; i-- {
				parentPath := strings.Join(parts[:i], "/")
				if parentDoc, err := loadInternal(ctx, parentPath, "", false); err == nil {
					parentDoc.ResolvedPath = pkgPath
					globalCache.setDoc(key, parentDoc)
					return parentDoc, nil
				}
			}
		}
		return nil, fmt.Errorf("could not find documentation for %s: %w", pkgPath, fetchErr)
	}

	result, err := parsePackageDocs(ctx, pkgPath, pkgDir, symbolName, pkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse documentation: %w", err)
	}
	globalCache.setDoc(key, result)
	return result, nil
}

// Example represents a code example extracted from documentation.
type Example struct {
	Name   string `json:"name"`
	Code   string `json:"code"`
	Output string `json:"output,omitempty"`
}

// Symbol types for Doc
const (
	TypeFunction = "function"
	TypeType     = "type"
	TypeMethod   = "method"
	TypeVar      = "var"
	TypeConst    = "const"
)

// Doc represents the parsed documentation.
type Doc struct {
	Package      string    `json:"package"`
	ImportPath   string    `json:"importPath"`
	ResolvedPath string    `json:"resolvedPath,omitempty"`
	SymbolName   string    `json:"symbolName,omitempty"`
	Type         string    `json:"type,omitempty"` // "function", "type", "var", "const"
	Definition   string    `json:"definition,omitempty"`
	Description  string    `json:"description"`
	Examples     []Example `json:"examples,omitempty"`
	SubPackages  []string  `json:"subPackages,omitempty"`
	PkgGoDevURL  string    `json:"pkgGoDevURL"`

	// Lists of symbols (signatures or summaries)
	Funcs  []string `json:"funcs,omitempty"`
	Types  []string `json:"types,omitempty"`
	Vars   []string `json:"vars,omitempty"`
	Consts []string `json:"consts,omitempty"`

	// Extra fields for Describe superset
	SourcePath string   `json:"sourcePath,omitempty"`
	Line       int      `json:"line,omitempty"`
	References []string `json:"references,omitempty"`
}

// Clone creates a deep copy of Doc for concurrency safety.
func (d *Doc) Clone() *Doc {
	if d == nil {
		return nil
	}
	clone := *d
	if d.Examples != nil {
		clone.Examples = append([]Example(nil), d.Examples...)
	}
	if d.SubPackages != nil {
		clone.SubPackages = append([]string(nil), d.SubPackages...)
	}
	if d.Funcs != nil {
		clone.Funcs = append([]string(nil), d.Funcs...)
	}
	if d.Types != nil {
		clone.Types = append([]string(nil), d.Types...)
	}
	if d.Vars != nil {
		clone.Vars = append([]string(nil), d.Vars...)
	}
	if d.Consts != nil {
		clone.Consts = append([]string(nil), d.Consts...)
	}
	if d.References != nil {
		clone.References = append([]string(nil), d.References...)
	}
	return &clone
}

func resolvePackageDir(ctx context.Context, pkgPath string) (string, error) {
	if cachedDir, ok := globalCache.getDir(pkgPath); ok {
		return cachedDir, nil
	}

	// Use 'go list' to find the directory of the package
	cmd, err := safeshell.CommandContext(ctx, "go", "list", "-f", "{{.Dir}}", pkgPath)
	if err != nil {
		return "", err
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go list failed: %v", string(out))
	}
	dir := strings.TrimSpace(string(out))
	globalCache.setDir(pkgPath, dir)
	return dir, nil
}

func parsePackageDocs(ctx context.Context, importPath, pkgDir, symbolName, requestedPath string) (*Doc, error) {
	fset := token.NewFileSet()
	files, err := loadPackageFiles(fset, pkgDir)
	if err != nil && len(files) == 0 {
		return nil, fmt.Errorf("failed to load package files in %s: %w", pkgDir, err)
	}

	result := initializeDoc(ctx, importPath, requestedPath, pkgDir)

	if len(files) == 0 {
		return handleEmptyFiles(result, importPath)
	}

	targetPkg, err := doc.NewFromFiles(fset, files, importPath)
	if err != nil {
		return nil, fmt.Errorf("doc.NewFromFiles failed: %w", err)
	}
	if targetPkg == nil {
		return nil, errors.New("doc.NewFromFiles returned nil package")
	}

	setPackageName(result, targetPkg, importPath)

	if symbolName == "" {
		populatePackageDoc(fset, targetPkg, result, importPath)
		return result, nil
	}

	return populateSymbolDoc(fset, targetPkg, result, symbolName, importPath)
}

func loadPackageFiles(fset *token.FileSet, pkgDir string) ([]*ast.File, error) {
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, err
	}

	var files []*ast.File
	var parseErrors []error

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasPrefix(name, ".") {
			continue
		}
		// Skip non-example test files
		if strings.HasSuffix(name, "_test.go") && !isExampleFile(name) {
			continue
		}

		filePath := filepath.Join(pkgDir, name)
		file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
		if err != nil {
			parseErrors = append(parseErrors, err)
			continue
		}
		files = append(files, file)
	}

	if len(parseErrors) > 0 && len(files) == 0 {
		return nil, errors.Join(parseErrors...)
	}
	return files, nil
}

func isExampleFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "example_") ||
		strings.Contains(lower, "_example_") ||
		strings.HasSuffix(lower, "_example_test.go") ||
		lower == "example_test.go"
}

func initializeDoc(ctx context.Context, importPath, requestedPath, pkgDir string) *Doc {
	result := &Doc{
		ImportPath:  importPath,
		PkgGoDevURL: fmt.Sprintf("https://pkg.go.dev/%s", importPath),
	}
	if requestedPath != "" && requestedPath != importPath {
		result.ResolvedPath = requestedPath
	}
	subs := ListSubPackages(ctx, pkgDir)
	for _, sub := range subs {
		if sub != importPath { // Exclude self
			result.SubPackages = append(result.SubPackages, sub)
		}
	}
	return result
}

func handleEmptyFiles(result *Doc, importPath string) (*Doc, error) {
	if len(result.SubPackages) > 0 {
		result.Package = "module_root"
		desc := fmt.Sprintf("Module root for %s. No Go source files found in the root directory,\n"+
			"but sub-packages exist.", importPath)
		result.Description = desc
		return result, nil
	}
	return nil, fmt.Errorf("no files found in package %s", importPath)
}

func setPackageName(result *Doc, targetPkg *doc.Package, importPath string) {
	pkgName := targetPkg.Name
	if pkgName == "" {
		parts := strings.Split(importPath, "/")
		pkgName = parts[len(parts)-1]
	}
	result.Package = pkgName
}

func populatePackageDoc(fset *token.FileSet, targetPkg *doc.Package, result *Doc, importPath string) {
	result.Description = targetPkg.Doc
	result.Definition = fmt.Sprintf("package %s // import %q", result.Package, importPath)
	result.Examples = extractExamples(fset, targetPkg.Examples)

	for _, f := range targetPkg.Funcs {
		result.Funcs = append(result.Funcs, bufferCode(fset, f.Decl))
	}
	for _, t := range targetPkg.Types {
		result.Types = append(result.Types, bufferCode(fset, t.Decl))
	}
	for _, v := range targetPkg.Vars {
		result.Vars = append(result.Vars, bufferCode(fset, v.Decl))
	}
	for _, c := range targetPkg.Consts {
		result.Consts = append(result.Consts, bufferCode(fset, c.Decl))
	}
}

func populateSymbolDoc(
	fset *token.FileSet,
	targetPkg *doc.Package,
	result *Doc,
	symbolName,
	importPath string,
) (*Doc, error) {
	result.SymbolName = symbolName
	result.PkgGoDevURL = fmt.Sprintf("https://pkg.go.dev/%s#%s", importPath, symbolName)

	found, candidates := findSymbol(fset, targetPkg, symbolName, result)
	if !found {
		fuzzyMatches := findFuzzyMatches(symbolName, candidates)
		msg := fmt.Sprintf("symbol %q not found in package %s", symbolName, importPath)
		if len(fuzzyMatches) > 0 {
			msg += fmt.Sprintf(". Did you mean: %s?", strings.Join(fuzzyMatches, ", "))
		}
		return nil, errors.New(msg)
	}
	return result, nil
}

func findSymbol(fset *token.FileSet, pkg *doc.Package, symName string, result *Doc) (bool, []string) {
	var candidates []string
	add := func(name string) { candidates = append(candidates, name) }

	if checkFuncs(fset, pkg, symName, result, add) {
		return true, nil
	}
	if checkTypes(fset, pkg, symName, result, add) {
		return true, nil
	}
	if checkVars(fset, pkg, symName, result, add) {
		return true, nil
	}
	if checkConsts(fset, pkg, symName, result, add) {
		return true, nil
	}

	return false, candidates
}

func checkFuncs(fset *token.FileSet, pkg *doc.Package, symName string, result *Doc, add func(string)) bool {
	if pkg == nil {
		return false
	}
	for _, f := range pkg.Funcs {
		if f == nil {
			continue
		}
		if f.Name == symName {
			populateFunc(fset, pkg, f, result)
			return true
		}
		add(f.Name)
	}
	return false
}

func checkTypes(fset *token.FileSet, pkg *doc.Package, symName string, result *Doc, add func(string)) bool {
	if pkg == nil {
		return false
	}
	for _, t := range pkg.Types {
		if t == nil {
			continue
		}
		if t.Name == symName {
			result.Type = TypeType
			if t.Decl != nil {
				result.Definition = bufferCode(fset, t.Decl)
			}
			result.Description = t.Doc
			result.Examples = extractExamples(fset, t.Examples)
			return true
		}
		add(t.Name)

		for _, f := range t.Funcs {
			if f == nil {
				continue
			}
			if f.Name == symName {
				populateFunc(fset, pkg, f, result)
				return true
			}
			add(f.Name)
		}

		for _, m := range t.Methods {
			if m == nil {
				continue
			}
			if m.Name == symName {
				result.Type = TypeMethod
				if m.Decl != nil {
					result.Definition = bufferCode(fset, m.Decl)
				}
				result.Description = m.Doc
				result.Examples = extractExamples(fset, m.Examples)
				return true
			}
			add(m.Name)
		}
	}
	return false
}

func checkVars(fset *token.FileSet, pkg *doc.Package, symName string, result *Doc, add func(string)) bool {
	if pkg == nil {
		return false
	}
	for _, v := range pkg.Vars {
		if v == nil {
			continue
		}
		for _, name := range v.Names {
			if name == symName {
				result.Type = TypeVar
				if v.Decl != nil {
					result.Definition = bufferCode(fset, v.Decl)
				}
				result.Description = v.Doc
				return true
			}
			add(name)
		}
	}
	return false
}

func checkConsts(fset *token.FileSet, pkg *doc.Package, symName string, result *Doc, add func(string)) bool {
	if pkg == nil {
		return false
	}
	for _, c := range pkg.Consts {
		if c == nil {
			continue
		}
		for _, name := range c.Names {
			if name == symName {
				result.Type = TypeConst
				if c.Decl != nil {
					result.Definition = bufferCode(fset, c.Decl)
				}
				result.Description = c.Doc
				return true
			}
			add(name)
		}
	}
	return false
}

func populateFunc(fset *token.FileSet, pkg *doc.Package, f *doc.Func, result *Doc) {
	if f == nil {
		return
	}
	result.Type = TypeFunction
	if f.Decl != nil {
		result.Definition = bufferCode(fset, f.Decl)
	}

	if typeDef := findReturnTypeDefinition(fset, pkg, f); typeDef != "" {
		result.Definition += "\n\n" + typeDef
	}

	result.Description = f.Doc
	result.Examples = extractExamples(fset, f.Examples)
}

func findReturnTypeDefinition(fset *token.FileSet, pkg *doc.Package, f *doc.Func) string {
	if f == nil || f.Decl == nil || f.Decl.Type == nil ||
		f.Decl.Type.Results == nil || len(f.Decl.Type.Results.List) == 0 {
		return ""
	}

	var typeNames []string
	seenNames := make(map[string]bool)

	// Iterate across all return values in Results.List
	for _, field := range f.Decl.Type.Results.List {
		if field == nil || field.Type == nil {
			continue
		}
		for _, name := range extractTypeNames(field.Type) {
			if name != "" && !seenNames[name] {
				typeNames = append(typeNames, name)
				seenNames[name] = true
			}
		}
	}

	if len(typeNames) == 0 || pkg == nil {
		return ""
	}

	// Search for matching types defined in the package
	var definitions []string
	seenDefs := make(map[string]bool)

	for _, name := range typeNames {
		for _, t := range pkg.Types {
			if t != nil && t.Name == name && t.Decl != nil {
				code := bufferCode(fset, t.Decl)
				if code != "" && !seenDefs[code] {
					definitions = append(definitions, code)
					seenDefs[code] = true
				}
			}
		}
	}

	if len(definitions) == 0 {
		return ""
	}

	return strings.Join(definitions, "\n\n")
}

func extractTypeNames(expr ast.Expr) []string {
	if expr == nil {
		return nil
	}

	switch t := expr.(type) {
	case *ast.Ident:
		return []string{t.Name}
	case *ast.StarExpr:
		return extractTypeNames(t.X)
	case *ast.ArrayType:
		return extractTypeNames(t.Elt)
	case *ast.MapType:
		keyNames := extractTypeNames(t.Key)
		valNames := extractTypeNames(t.Value)
		names := make([]string, 0, len(keyNames)+len(valNames))
		names = append(names, keyNames...)
		names = append(names, valNames...)
		return names
	case *ast.ChanType:
		return extractTypeNames(t.Value)
	case *ast.Ellipsis:
		return extractTypeNames(t.Elt)
	case *ast.ParenExpr:
		return extractTypeNames(t.X)
	}

	return nil
}

func extractExamples(fset *token.FileSet, examples []*doc.Example) []Example {
	result := make([]Example, 0, len(examples))
	for _, ex := range examples {
		if ex == nil {
			continue
		}
		code := bufferCode(fset, ex.Code)
		result = append(result, Example{
			Name:   ex.Name,
			Code:   code,
			Output: ex.Output,
		})
	}
	return result
}

func bufferCode(fset *token.FileSet, node any) string {
	if node == nil {
		return ""
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return fmt.Sprintf("error printing code: %v", err)
	}
	return buf.String()
}

// RenderJSON marshals the Doc into formatted JSON.
func (d *Doc) RenderJSON() (string, error) {
	if d == nil {
		return "{}", nil
	}
	bytes, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// RenderMarkdown converts a Doc to Markdown.
func (d *Doc) RenderMarkdown() string {
	return Render(d)
}

// Render converts a Doc to Markdown.
func Render(doc *Doc) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "# %s\n\n", doc.ImportPath)

	if doc.ResolvedPath != "" {
		if strings.HasPrefix(doc.ResolvedPath, doc.ImportPath) {
			fmt.Fprintf(&buf, "> ℹ️ **Note:** Could not find `%s`.\n"+
				"> Showing documentation for parent module `%s` instead.\n\n",
				doc.ResolvedPath, doc.ImportPath)
		} else {
			fmt.Fprintf(&buf, "> **Note:** Redirected from %s\n\n", doc.ResolvedPath)
		}
	}

	if doc.SymbolName != "" {
		fmt.Fprintf(&buf, "## %s %s\n\n", doc.Type, doc.SymbolName)
	}

	if doc.SourcePath != "" {
		fmt.Fprintf(&buf, "Defined in: `%s:%d`\n\n", doc.SourcePath, doc.Line)
	}

	if doc.Definition != "" {
		buf.WriteString("```go\n")
		buf.WriteString(doc.Definition)
		buf.WriteString("\n```\n\n")
	}

	buf.WriteString(doc.Description)
	buf.WriteString("\n\n")

	renderExamples(&buf, doc.Examples)

	// Render Symbol Lists (if available and not focusing on a single symbol)
	if doc.SymbolName == "" {
		renderSymbolLists(&buf, doc)
	}

	if len(doc.References) > 0 {
		buf.WriteString("### Usages\n\n")
		for _, ref := range doc.References {
			fmt.Fprintf(&buf, "- %s\n", ref)
		}
		buf.WriteString("\n")
	}

	if len(doc.SubPackages) > 0 {
		buf.WriteString("### Sub-packages\n\n")
		for _, sub := range doc.SubPackages {
			fmt.Fprintf(&buf, "- %s\n", sub)
		}
		buf.WriteString("\n")
	}

	fmt.Fprintf(&buf, "[View on pkg.go.dev](%s)\n", doc.PkgGoDevURL)
	return buf.String()
}

func renderExamples(buf *strings.Builder, examples []Example) {
	if len(examples) == 0 {
		return
	}
	buf.WriteString("### Examples\n\n")
	for _, ex := range examples {
		name := ex.Name
		if name == "" {
			name = "Package Example"
		}
		fmt.Fprintf(buf, "#### %s\n\n", name)
		buf.WriteString("```go\n")
		buf.WriteString(ex.Code)
		buf.WriteString("\n```\n")
		if ex.Output != "" {
			buf.WriteString("\n**Output:**\n```\n")
			buf.WriteString(ex.Output)
			buf.WriteString("\n```\n")
		}
		buf.WriteString("\n")
	}
}

func renderSymbolLists(buf *strings.Builder, doc *Doc) {
	if len(doc.Consts) > 0 {
		buf.WriteString("### Constants\n\n")
		buf.WriteString("```go\n")
		for _, c := range doc.Consts {
			buf.WriteString(c)
			buf.WriteString("\n")
		}
		buf.WriteString("```\n\n")
	}
	if len(doc.Vars) > 0 {
		buf.WriteString("### Variables\n\n")
		buf.WriteString("```go\n")
		for _, v := range doc.Vars {
			buf.WriteString(v)
			buf.WriteString("\n")
		}
		buf.WriteString("```\n\n")
	}
	if len(doc.Funcs) > 0 {
		buf.WriteString("### Functions\n\n")
		buf.WriteString("```go\n")
		for _, f := range doc.Funcs {
			buf.WriteString(f)
			buf.WriteString("\n\n")
		}
		buf.WriteString("```\n\n")
	}
	if len(doc.Types) > 0 {
		buf.WriteString("### Types\n\n")
		buf.WriteString("```go\n")
		for _, t := range doc.Types {
			buf.WriteString(t)
			buf.WriteString("\n\n")
		}
		buf.WriteString("```\n\n")
	}
}

func findFuzzyMatches(query string, candidates []string) []string {
	var matches []string
	lowerQuery := strings.ToLower(query)

	for _, c := range candidates {
		// Case insensitive match
		if strings.EqualFold(query, c) {
			matches = append(matches, c)
			continue
		}

		// Levenshtein distance <= 2 (allow small typos)
		dist := text.Levenshtein(lowerQuery, strings.ToLower(c))
		if dist <= 2 {
			matches = append(matches, c)
		}
	}
	// Limit to top 5
	if len(matches) > 5 {
		return matches[:5]
	}
	return matches
}

func fetchAndRetryStructured(ctx context.Context, pkgPath, symbolName string, originalErr error) (*Doc, error) {
	tempDir, err := setupTempModule(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to setup temp module: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	pkgDir, actualPkgPath, err := downloadPackage(ctx, tempDir, pkgPath)
	if err != nil {
		// Attempt to provide suggestions from standard library and local context
		suggestions := suggestPackages(ctx, pkgPath)

		if len(suggestions) > 0 {
			return nil, fmt.Errorf("package %q not found. Did you mean: %s?", pkgPath, strings.Join(suggestions, ", "))
		}

		return nil, fmt.Errorf("failed to download package %q: %v\nOriginal error: %v",
			pkgPath, err, originalErr)
	}

	result, err := parsePackageDocs(ctx, actualPkgPath, pkgDir, symbolName, pkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse documentation after download: %w", err)
	}

	return result, nil
}

func suggestPackages(ctx context.Context, query string) []string {
	// Guard suggestion searching with a 500ms timeout context to prevent latency spikes
	sugCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	var candidates []string
	seen := make(map[string]bool)

	add := func(out []byte) {
		for pkg := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			if pkg != "" && !seen[pkg] {
				candidates = append(candidates, pkg)
				seen[pkg] = true
			}
		}
	}

	// 1. Standard Library (Cached globally after first fetch)
	stdlibOnceFetch(sugCtx)
	for _, pkg := range stdlibPackages {
		if !seen[pkg] {
			candidates = append(candidates, pkg)
			seen[pkg] = true
		}
	}

	// 2. Local module packages (Fast, bounded by sugCtx)
	if cmd, err := safeshell.CommandContext(sugCtx, "go", "list", "./..."); err == nil {
		if out, err := cmd.Output(); err == nil {
			add(out)
		}
	}

	// 3. Parent context (If query is a path, try listing sibling packages)
	if parts := strings.Split(query, "/"); len(parts) > 1 {
		parent := strings.Join(parts[:len(parts)-1], "/")
		if cmd, err := safeshell.CommandContext(sugCtx, "go", "list", parent+"/..."); err == nil {
			if out, err := cmd.Output(); err == nil {
				add(out)
			}
		}
	}

	return findFuzzyMatches(query, candidates)
}

func stdlibOnceFetch(ctx context.Context) {
	stdlibMu.RLock()
	if len(stdlibPackages) > 0 {
		stdlibMu.RUnlock()
		return
	}
	stdlibMu.RUnlock()

	stdlibMu.Lock()
	defer stdlibMu.Unlock()
	if len(stdlibPackages) > 0 {
		return
	}

	fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	if cmd, err := safeshell.CommandContext(fetchCtx, "go", "list", "std"); err == nil {
		if out, err := cmd.Output(); err == nil {
			for pkg := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
				if pkg != "" {
					stdlibPackages = append(stdlibPackages, pkg)
				}
			}
		}
	}
}

// ListSubPackages finds sub-packages within a directory using go list.
func ListSubPackages(ctx context.Context, pkgDir string) []string {
	if cachedSubs, ok := globalCache.getSubpkgs(pkgDir); ok {
		return cachedSubs
	}

	cmd, err := safeshell.CommandContext(ctx, "go", "list", "-f", "{{.ImportPath}}", "./...")
	if err != nil {
		return nil
	}
	cmd.Dir = pkgDir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}
	subs := strings.Split(trimmed, "\n")
	globalCache.setSubpkgs(pkgDir, subs)
	return subs
}

func setupTempModule(ctx context.Context) (string, error) {
	tempDir, err := os.MkdirTemp("", "godoctor_docs_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	initCmd, err := safeshell.CommandContext(ctx, "go", "mod", "init", "temp_docs_fetcher")
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to validate secure init command: %w", err)
	}
	initCmd.Dir = tempDir
	if out, err := initCmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to init temp module: %v\nOutput: %s", err, out)
	}
	return tempDir, nil
}

var vanityImportRe = regexp.MustCompile(`module declares its path as:\s+([^\s]+)`)

func downloadPackage(ctx context.Context, tempDir, pkgPath string) (string, string, error) {
	getCmd, err := safeshell.CommandContext(ctx, "go", "get", pkgPath)
	if err != nil {
		return "", "", fmt.Errorf("secure validation failed for download path: %w", err)
	}
	getCmd.Dir = tempDir
	out, err := getCmd.CombinedOutput()

	actualPath := pkgPath

	if err != nil {
		// Check for vanity import error
		matches := vanityImportRe.FindSubmatch(out)
		if len(matches) > 1 {
			extractedPath := strings.Trim(string(matches[1]), "\"'`()[]{} \t\r\n")
			if extractedPath != "" && extractedPath != pkgPath {
				actualPath = extractedPath
				// Retry with correct path
				retryCmd, retryErr := safeshell.CommandContext(ctx, "go", "get", actualPath)
				if retryErr != nil {
					return "", "", fmt.Errorf("secure validation failed for vanity path: %w", retryErr)
				}
				retryCmd.Dir = tempDir
				if retryOut, retryErr := retryCmd.CombinedOutput(); retryErr != nil {
					return "", "", fmt.Errorf("go get failed after vanity retry: %v\nOutput: %s", retryErr, retryOut)
				}
				// Success on retry
			} else {
				return "", "", fmt.Errorf("go get failed: %v\nOutput: %s", err, out)
			}
		} else {
			return "", "", fmt.Errorf("go get failed: %v\nOutput: %s", err, out)
		}
	}

	// Try to locate as a package first
	listCmd, err := safeshell.CommandContext(ctx, "go", "list", "-f", "{{.Dir}}", actualPath)
	if err != nil {
		return "", "", fmt.Errorf("secure validation failed for list path: %w", err)
	}
	listCmd.Dir = tempDir
	out, err = listCmd.CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(out)), actualPath, nil
	}

	// If failed, try to locate as a module (e.g. root of repo with no root package files)
	modCmd, err := safeshell.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Dir}}", actualPath)
	if err != nil {
		return "", "", fmt.Errorf("secure validation failed for locate path: %w", err)
	}
	modCmd.Dir = tempDir
	out, err = modCmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("failed to locate package or module: %v\nOutput: %s", err, out)
	}

	return strings.TrimSpace(string(out)), actualPath, nil
}
