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

// Package getdocs implements the documentation retrieval tool.
package getdocs

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
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	// ErrPackageMissing indicates that the requested package could not be found locally.
	ErrPackageMissing = errors.New("package missing")
)

// Register registers the get_docs tool with the server.
func Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:  "read_godoc",
		Title: "Read Go Documentation",
		Description: "Retrieves Go documentation for packages or symbols using the standard go/doc format. " +
			"Use this to inspect function signatures, type definitions, and usage examples before writing code.",
	}, Handler)
}

// Params defines the input parameters for the get_docs tool.
type Params struct {
	PackagePath string `json:"package_path"`
	SymbolName  string `json:"symbol_name,omitempty"`
}

// Example represents a code example extracted from documentation.
type Example struct {
	Name   string `json:"name"`
	Code   string `json:"code"`
	Output string `json:"output,omitempty"`
}

// StructuredDoc represents the parsed documentation.
type StructuredDoc struct {
	Package     string    `json:"package"`
	ImportPath  string    `json:"importPath"`
	SymbolName  string    `json:"symbolName,omitempty"`
	Type        string    `json:"type,omitempty"` // "function", "type", "var", "const"
	Definition  string    `json:"definition,omitempty"`
	Description string    `json:"description"`
	Examples    []Example `json:"examples,omitempty"`
	PkgGoDevURL string    `json:"pkgGoDevURL"`
}

// Handler handles the get_docs tool execution.
func Handler(ctx context.Context, _ *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	pkgPath := args.PackagePath
	symbolName := args.SymbolName

	if pkgPath == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "package_path cannot be empty"},
			},
		}, nil, nil
	}

	// Try to find the package directory locally
	pkgDir, err := resolvePackageDir(ctx, pkgPath)
	if err != nil {
		// Fallback: try to fetch the package in a temp directory
		return fetchAndRetry(ctx, pkgPath, symbolName, err.Error())
	}

	result, err := parsePackageDocs(pkgPath, pkgDir, symbolName)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("failed to parse documentation: %v", err)},
			},
		}, nil, nil
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal structured doc: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, nil, nil
}

func resolvePackageDir(ctx context.Context, pkgPath string) (string, error) {
	// Use 'go list' to find the directory of the package
	cmd := exec.CommandContext(ctx, "go", "list", "-f", "{{.Dir}}", pkgPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go list failed: %v", string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func parsePackageDocs(importPath, pkgDir, symbolName string) (*StructuredDoc, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, pkgDir, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parser.ParseDir failed: %w", err)
	}

	// Collect all files from all packages (e.g. "http" and "http_test")
	var files []*ast.File
	var packageName string
	for name, pkg := range pkgs {
		if !strings.HasSuffix(name, "_test") {
			packageName = name
		}
		for _, file := range pkg.Files {
			files = append(files, file)
		}
	}

	// If we didn't find a main package name, pick any
	if packageName == "" {
		for name := range pkgs {
			packageName = name
			break
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files found in package %s", importPath)
	}

	// Compute documentation using all files
	targetPkg, err := doc.NewFromFiles(fset, files, importPath)
	if err != nil {
		return nil, fmt.Errorf("doc.NewFromFiles failed: %w", err)
	}

	result := &StructuredDoc{
		Package:     targetPkg.Name,
		ImportPath:  importPath,
		PkgGoDevURL: fmt.Sprintf("https://pkg.go.dev/%s", importPath),
	}

	if symbolName == "" {
		result.Description = targetPkg.Doc
		result.Definition = fmt.Sprintf("package %s // import %q", targetPkg.Name, importPath)
		result.Examples = extractExamples(fset, targetPkg.Examples)
		return result, nil
	}

	result.SymbolName = symbolName
	result.PkgGoDevURL = fmt.Sprintf("https://pkg.go.dev/%s#%s", importPath, symbolName)

	found, candidates := findSymbol(fset, targetPkg, symbolName, result)
	if !found {
		// Limit candidates to avoid huge error messages
		maxCandidates := 20
		if len(candidates) > maxCandidates {
			candidates = append(candidates[:maxCandidates], fmt.Sprintf("...and %d more", len(candidates)-maxCandidates))
		}
		return nil, fmt.Errorf("symbol %q not found in package %s. Available symbols: %s",
			symbolName, importPath, strings.Join(candidates, ", "))
	}

	return result, nil
}

//nolint:staticcheck // ast.Package is deprecated but required by go/doc
func findTargetPackage(pkgs map[string]*ast.Package, importPath string) *doc.Package {
	// Prefer non-test packages
	for _, pkg := range pkgs {
		if !strings.HasSuffix(pkg.Name, "_test") {
			return doc.New(pkg, importPath, doc.AllDecls)
		}
	}
	// Fallback to any package
	for _, pkg := range pkgs {
		return doc.New(pkg, importPath, doc.AllDecls)
	}
	return nil
}

func findSymbol(fset *token.FileSet, pkg *doc.Package, symName string, result *StructuredDoc) (bool, []string) {
	var candidates []string

	// Helper to add candidate
	addCandidate := func(name string) {
		candidates = append(candidates, name)
	}

	// Check Functions (Top-level)
	for _, f := range pkg.Funcs {
		if f.Name == symName {
			result.Type = "function"
			result.Definition = bufferCode(fset, f.Decl)
			result.Description = f.Doc
			result.Examples = extractExamples(fset, f.Examples)
			return true, nil
		}
		addCandidate(f.Name)
	}

	// Check Types
	for _, t := range pkg.Types {
		if t.Name == symName {
			result.Type = "type"
			result.Definition = bufferCode(fset, t.Decl)
			result.Description = t.Doc
			result.Examples = extractExamples(fset, t.Examples)
			return true, nil
		}
		addCandidate(t.Name)

		// Check constructors/factories associated with the type
		for _, f := range t.Funcs {
			if f.Name == symName {
				result.Type = "function"
				result.Definition = bufferCode(fset, f.Decl)
				result.Description = f.Doc
				result.Examples = extractExamples(fset, f.Examples)
				return true, nil
			}
			addCandidate(f.Name)
		}

		// Check methods
		for _, m := range t.Methods {
			if m.Name == symName {
				result.Type = "method"
				result.Definition = bufferCode(fset, m.Decl)
				result.Description = m.Doc
				result.Examples = extractExamples(fset, m.Examples)
				return true, nil
			}
			addCandidate(m.Name)
		}
	}

	// Check Variables
	for _, v := range pkg.Vars {
		for _, name := range v.Names {
			if name == symName {
				result.Type = "var"
				result.Definition = bufferCode(fset, v.Decl)
				result.Description = v.Doc
				return true, nil
			}
			addCandidate(name)
		}
	}

	// Check Constants
	for _, c := range pkg.Consts {
		for _, name := range c.Names {
			if name == symName {
				result.Type = "const"
				result.Definition = bufferCode(fset, c.Decl)
				result.Description = c.Doc
				return true, nil
			}
			addCandidate(name)
		}
	}

	return false, candidates
}

func extractExamples(fset *token.FileSet, examples []*doc.Example) []Example {
	result := make([]Example, 0, len(examples))
	for _, ex := range examples {
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
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return fmt.Sprintf("error printing code: %v", err)
	}
	return buf.String()
}

func fetchAndRetry(ctx context.Context, pkgPath, symbolName, originalErr string) (*mcp.CallToolResult, any, error) {
	tempDir, err := setupTempModule(ctx)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("failed to setup temp module: %v", err)},
			},
		}, nil, nil
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	pkgDir, err := downloadPackage(ctx, tempDir, pkgPath)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("failed to download package %q: %v\nOriginal error: %s",
					pkgPath, err, originalErr)},
			},
		}, nil, nil
	}

	result, err := parsePackageDocs(pkgPath, pkgDir, symbolName)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("failed to parse documentation after download: %v", err)},
			},
		}, nil, nil
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal structured doc: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, nil, nil
}

func setupTempModule(ctx context.Context) (string, error) {
	tempDir, err := os.MkdirTemp("", "godoctor_docs_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	initCmd := exec.CommandContext(ctx, "go", "mod", "init", "temp_docs_fetcher")
	initCmd.Dir = tempDir
	if out, err := initCmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to init temp module: %v\nOutput: %s", err, out)
	}
	return tempDir, nil
}

func downloadPackage(ctx context.Context, tempDir, pkgPath string) (string, error) {
	getCmd := exec.CommandContext(ctx, "go", "get", pkgPath)
	getCmd.Dir = tempDir
	if out, err := getCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go get failed: %v\nOutput: %s", err, out)
	}

	listCmd := exec.CommandContext(ctx, "go", "list", "-f", "{{.Dir}}", pkgPath)
	listCmd.Dir = tempDir
	out, err := listCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to locate package: %v\nOutput: %s", err, out)
	}
	return strings.TrimSpace(string(out)), nil
}