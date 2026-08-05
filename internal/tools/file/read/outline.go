package read

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/danicat/godoctor/internal/safeshell"
)

// GetOutline loads a file and returns its outline, list of imports, and build errors.
func GetOutline(ctx context.Context, file string) (string, []string, []error, error) {
	fset := token.NewFileSet()
	//nolint:gosec // G304: File path provided by user is expected.
	content, err := os.ReadFile(file)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	targetFile, err := parser.ParseFile(fset, file, content, parser.ParseComments)
	var errs []error
	if err != nil {
		errs = append(errs, err)
	}

	if targetFile == nil {
		return "", nil, errs, fmt.Errorf("failed to parse file: %w", err)
	}

	// 1. Extract imports (always reliable via Go parser)
	var imports []string
	for _, imp := range targetFile.Imports {
		if imp.Path != nil {
			imports = append(imports, imp.Path.Value)
		}
	}

	// 2. Try generating outline via gopls symbols (compiler-accurate)
	cmd, err := safeshell.CommandContext(ctx, "gopls", "symbols", file)
	if err == nil {
		cmd.Dir = filepath.Dir(file)
		goplsOut, cmdErr := cmd.CombinedOutput()
		if cmdErr == nil && len(strings.TrimSpace(string(goplsOut))) > 0 {
			return string(goplsOut), imports, errs, nil
		}
	}

	// 3. Fallback to custom AST Outlinizer if gopls fails or is empty
	outline := outlinize(targetFile)

	var buf bytes.Buffer
	config := &printer.Config{Mode: printer.TabIndent | printer.UseSpaces, Tabwidth: 8}
	if err := config.Fprint(&buf, fset, outline); err != nil {
		return "", nil, errs, fmt.Errorf("failed to format outline: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		formatted = buf.Bytes()
	}

	return string(formatted), imports, errs, nil
}

func outlinize(f *ast.File) *ast.File {
	res := *f
	res.Decls = make([]ast.Decl, len(f.Decls))

	allowedComments := make(map[*ast.CommentGroup]bool)
	if f.Doc != nil {
		allowedComments[f.Doc] = true
	}
	for _, cg := range f.Comments {
		if cg.End() < f.Package {
			allowedComments[cg] = true
		}
	}

	for i, decl := range f.Decls {
		res.Decls[i] = processDeclOutline(decl, allowedComments)
	}

	var newComments []*ast.CommentGroup
	for _, cg := range f.Comments {
		if allowedComments[cg] {
			newComments = append(newComments, cg)
		}
	}
	res.Comments = newComments

	return &res
}

func processDeclOutline(decl ast.Decl, allowedComments map[*ast.CommentGroup]bool) ast.Decl {
	switch fn := decl.(type) {
	case *ast.FuncDecl:
		newFn := *fn
		newFn.Body = nil
		if fn.Doc != nil {
			allowedComments[fn.Doc] = true
		}
		return &newFn
	case *ast.GenDecl:
		if fn.Doc != nil {
			allowedComments[fn.Doc] = true
		}
		for _, spec := range fn.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				if s.Doc != nil {
					allowedComments[s.Doc] = true
				}
			case *ast.ValueSpec:
				if s.Doc != nil {
					allowedComments[s.Doc] = true
				}
			}
		}
		return decl
	default:
		return decl
	}
}
