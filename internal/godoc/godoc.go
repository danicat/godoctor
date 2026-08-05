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
	"errors"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"regexp"
	"strings"

	"github.com/danicat/godoctor/internal/safeshell"
	"github.com/danicat/godoctor/internal/textdist"
)

// Load resolves an import path and returns documentation.
// It performs disk I/O ("go list") and parsing. Use this when starting from a string path.
func Load(ctx context.Context, pkgPath, symbolName string) (*Doc, error) {
	return loadInternal(ctx, pkgPath, symbolName, false)
}

// LoadWithFallback is like Load but attempts to find parent packages if the exact match fails.
func LoadWithFallback(ctx context.Context, pkgPath, symbolName string) (*Doc, error) {
	return loadInternal(ctx, pkgPath, symbolName, true)
}

func loadInternal(ctx context.Context, pkgPath, symbolName string, allowFallback bool) (*Doc, error) {
	// Try to find the package directory locally
	pkgDir, err := resolvePackageDir(ctx, pkgPath)
	if err != nil {
		// Fallback: try to fetch the package in a temp directory
		doc, fetchErr := fetchAndRetryStructured(ctx, pkgPath, symbolName, err)
		if fetchErr == nil {
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
				if doc, err := loadInternal(ctx, parentPath, "", false); err == nil {
					doc.ResolvedPath = pkgPath
					return doc, nil
				}
			}
		}
		return nil, fetchErr
	}

	result, err := parsePackageDocs(ctx, pkgPath, pkgDir, symbolName, pkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse documentation: %w", err)
	}
	return result, nil
}


// GetDocumentation retrieves the documentation for a package or symbol as Markdown.
func GetDocumentation(ctx context.Context, pkgPath, symbolName string) (string, error) {
	doc, err := Load(ctx, pkgPath, symbolName)
	if err != nil {
		return "", err
	}
	return Render(doc), nil
}

// GetDocumentationWithFallback attempts to retrieve documentation for a package.
// If the specific package documentation is not found, it attempts to find documentation
// for parent paths (module roots) and returns it with a hint.
func GetDocumentationWithFallback(ctx context.Context, pkgPath string) (string, error) {
	// 1. Try exact match
	doc, err := Load(ctx, pkgPath, "")
	if err == nil && doc.Package != "" {
		return Render(doc), nil
	}

	// 2. Fallback: Walk up the path
	parts := strings.Split(pkgPath, "/")
	// Heuristic: For domain-based packages (github.com/...), keep at least 3 parts.
	minParts := 1
	if strings.Contains(parts[0], ".") {
		minParts = 3
	}

	for i := len(parts) - 1; i >= minParts; i-- {
		parentPath := strings.Join(parts[:i], "/")
		doc, err := Load(ctx, parentPath, "")
		if err == nil && doc.Package != "" {
			note := fmt.Sprintf("> ℹ️ **Note:** Could not find `%s`.\n"+
				"> Showing documentation for parent module `%s` instead.\n\n%s", pkgPath, parentPath, Render(doc))
			return note, nil
		}
	}

	return "", fmt.Errorf("could not find documentation for %s or its parents", pkgPath)
}

// Example represents a code example extracted from documentation.
type Example struct {
	Name   string `json:"name"`
	Code   string `json:"code"`
	Output string `json:"output,omitempty"`
}

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

func resolvePackageDir(ctx context.Context, pkgPath string) (string, error) {
	// Use 'go list' to find the directory of the package
	cmd, err := safeshell.CommandContext(ctx, "go", "list", "-f", "{{.Dir}}", pkgPath)
	if err != nil {
		return "", err
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go list failed: %v", string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func parsePackageDocs(ctx context.Context, importPath, pkgDir, symbolName, requestedPath string) (*Doc, error) {
	fset := token.NewFileSet()
	//nolint:staticcheck // SA1019: parser.ParseDir is used for fast parsing of comments without type-checking
	pkgs, err := parser.ParseDir(fset, pkgDir, nil, parser.ParseComments)
	files := collectFiles(pkgs)
	if err != nil && len(files) == 0 {
		return nil, fmt.Errorf("parser.ParseDir failed: %w", err)
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

//nolint:staticcheck // SA1019: ast.Package is deprecated but kept for backwards-compatibility.
func collectFiles(pkgs map[string]*ast.Package) []*ast.File {
	var files []*ast.File
	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		for _, file := range pkg.Files {
			if file != nil {
				files = append(files, file)
			}
		}
	}
	return files
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
			result.Type = "type"
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
				result.Type = "method"
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
				result.Type = "var"
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
				result.Type = "const"
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
	result.Type = "function"
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
	if f == nil || f.Decl == nil || f.Decl.Type == nil || f.Decl.Type.Results == nil || len(f.Decl.Type.Results.List) == 0 {
		return ""
	}

	// Get the first return value type
	if f.Decl.Type.Results.List[0] == nil {
		return ""
	}
	retType := f.Decl.Type.Results.List[0].Type
	if retType == nil {
		return ""
	}

	var typeName string

	// Handle pointers (*Type) or values (Type)
	switch t := retType.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok && ident != nil {
			typeName = ident.Name
		}
	case *ast.Ident:
		typeName = t.Name
	case *ast.ArrayType:
		if ident, ok := t.Elt.(*ast.Ident); ok && ident != nil {
			typeName = ident.Name
		} else if star, ok := t.Elt.(*ast.StarExpr); ok && star != nil {
			if ident, ok := star.X.(*ast.Ident); ok && ident != nil {
				typeName = ident.Name
			}
		}
	}

	if typeName == "" {
		return ""
	}

	// Search for this type in the package
	if pkg != nil {
		for _, t := range pkg.Types {
			if t != nil && t.Name == typeName && t.Decl != nil {
				return bufferCode(fset, t.Decl)
			}
		}
	}

	return ""
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

		// Levenshtein distance < 3 (allow small typos)
		dist := textdist.Levenshtein(lowerQuery, strings.ToLower(c))
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
	var candidates []string
	seen := make(map[string]bool)

	// Helper to add unique candidates
	add := func(out []byte) {
		for pkg := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			if pkg != "" && !seen[pkg] {
				candidates = append(candidates, pkg)
				seen[pkg] = true
			}
		}
	}

	// 1. Standard Library (Reliable, works everywhere)
	if cmd, err := safeshell.CommandContext(ctx, "go", "list", "std"); err == nil {
		if out, err := cmd.Output(); err == nil {
			add(out)
		}
	}

	// 2. Local context (Context-dependent, might fail outside modules)
	if cmd, err := safeshell.CommandContext(ctx, "go", "list", "all"); err == nil {
		if out, err := cmd.Output(); err == nil {
			add(out)
		}
	}

	// 3. Parent context (If query is a path, try listing sibling packages)
	if parts := strings.Split(query, "/"); len(parts) > 1 {
		parent := strings.Join(parts[:len(parts)-1], "/")
		// Attempt to list sub-packages of the parent
		if cmd, err := safeshell.CommandContext(ctx, "go", "list", parent+"/..."); err == nil {
			if out, err := cmd.Output(); err == nil {
				add(out)
			}
		}
	}

	return findFuzzyMatches(query, candidates)
}

// ListSubPackages finds sub-packages within a directory using go list.
func ListSubPackages(ctx context.Context, pkgDir string) []string {
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
	return strings.Split(trimmed, "\n")
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
