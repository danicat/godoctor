// Package smartread implements the file reading tool with automatic type enrichment and outline modes.
package smartread

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"

	"github.com/danicat/godoctor/internal/godoc"
	"github.com/danicat/godoctor/internal/roots"
	"github.com/danicat/godoctor/internal/text"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the smart_read tool with the server.
func Register(server *mcp.Server) {
	//nolint:lll
	mcp.AddTool(server, &mcp.Tool{
		Name:        "smart_read",
		Title:       "Read File",
		Description: "High-density multi-file code reader with unconditional type-tag enrichment. Automatically uses native Go AST parsing and godoc to extract and append Go struct/interface schemas in a custom <types> block.",
	}, readCodeHandler)
}

// Params defines the input parameters for the smart_read tool.
type Params struct {
	Filenames []string `json:"filenames,omitempty" jsonschema:"The absolute paths to the Go files to read."`
	Filename  string   `json:"filename,omitempty" jsonschema:"Deprecated: use filenames instead"`
	Outline   bool     `json:"outline,omitempty" jsonschema:"Optional: if true, returns the structure (AST) only"`
	StartLine int      `json:"start_line,omitempty" jsonschema:"Optional: start reading from this line number"`
	EndLine   int      `json:"end_line,omitempty" jsonschema:"Optional: stop reading at this line number"`
}

func readCodeHandler(ctx context.Context, req *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	var session *mcp.ServerSession
	if req != nil {
		session = req.Session
	}
	filenames := args.Filenames
	if len(filenames) == 0 && args.Filename != "" {
		filenames = []string{args.Filename}
	}

	if len(filenames) == 0 {
		return errorResult("at least one filename must be specified"), nil, nil
	}

	if args.Outline && args.StartLine == 0 {
		return handleOutlineMode(ctx, session, filenames)
	}

	return handleReadMode(ctx, session, args, filenames)
}

func handleOutlineMode(
	ctx context.Context,
	session *mcp.ServerSession,
	filenames []string,
) (*mcp.CallToolResult, any, error) {
	var sb strings.Builder
	for _, filename := range filenames {
		absPath, err := roots.Global.Validate(session, filename)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		out, imports, errs, err := GetOutline(ctx, absPath)
		if err != nil {
			return errorResult(fmt.Sprintf("failed to generate outline for %s: %v", filename, err)), nil, nil
		}
		fmt.Fprintf(&sb, "# File: %s (Outline)\n\n", absPath)
		if len(errs) > 0 {
			sb.WriteString("## Analysis (Problems)\n")
			for _, e := range errs {
				fmt.Fprintf(&sb, "- ⚠️ %v\n", e)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("```go\n")
		sb.WriteString(out)
		sb.WriteString("\n```\n\n")

		if len(imports) > 0 {
			var thirdParty []string
			for _, imp := range imports {
				clean := strings.Trim(imp, "\"")
				if parts := strings.Split(clean, "/"); len(parts) > 0 && strings.Contains(parts[0], ".") {
					thirdParty = append(thirdParty, imp)
				}
			}
			if len(thirdParty) > 0 {
				sb.WriteString("## Third-Party Imports\n")
				for _, imp := range thirdParty {
					fmt.Fprintf(&sb, "- %s\n", imp)
				}
				sb.WriteString("\n")
			}
		}
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: sb.String()},
		},
	}, nil, nil
}

func handleReadMode(
	ctx context.Context,
	session *mcp.ServerSession,
	args Params,
	filenames []string,
) (*mcp.CallToolResult, any, error) {
	var sb strings.Builder
	var allTypesEnrichment strings.Builder

	for _, filename := range filenames {
		fContent, enrich, errRes := readSingleFile(ctx, session, args, filename)
		if errRes != nil {
			return errRes, nil, nil
		}
		sb.WriteString(fContent)
		if enrich != "" {
			allTypesEnrichment.WriteString(enrich)
		}
	}

	if allTypesEnrichment.Len() > 0 {
		sb.WriteString("## Type Specifications\n")
		sb.WriteString(allTypesEnrichment.String())
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: sb.String()},
		},
	}, nil, nil
}

func readSingleFile(
	ctx context.Context,
	session *mcp.ServerSession,
	args Params,
	filename string,
) (string, string, *mcp.CallToolResult) {
	absPath, err := roots.Global.Validate(session, filename)
	if err != nil {
		return "", "", errorResult(err.Error())
	}

	//nolint:gosec // G304: File path provided by user is validated against roots.
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", "", errorResult(fmt.Sprintf("failed to read file %s: %v", filename, err))
	}

	isGo := strings.HasSuffix(absPath, ".go")
	original := string(content)

	startLine := args.StartLine
	if startLine <= 0 {
		startLine = 1
	}
	endLine := args.EndLine

	startOffset, endOffset, err := text.GetLineOffsets(original, startLine, endLine)
	if err != nil {
		return "", "", errorResult(fmt.Sprintf("line range error for %s: %v", filename, err))
	}

	viewContent := original[startOffset:endOffset]
	contentWithLines, linesCount := renderContentWithLines(viewContent, startLine)

	isPartial := args.StartLine > 1 || args.EndLine > 0
	rangeInfo := ""
	if isPartial {
		rangeInfo = fmt.Sprintf(" (Lines %d-%d)", startLine, startLine+linesCount-1)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# File: %s%s\n\n", absPath, rangeInfo)

	sb.WriteString("```")
	if isGo {
		sb.WriteString("go")
	}
	sb.WriteString("\n")
	sb.WriteString(contentWithLines)
	sb.WriteString("```\n\n")

	var enrichment string
	if isGo {
		var enrichErr error
		enrichment, enrichErr = enrichTypes(ctx, absPath, content)
		if enrichErr != nil {
			return "", "", errorResult(fmt.Sprintf("failed to enrich types: %v", enrichErr))
		}
	}

	return sb.String(), enrichment, nil
}

func renderContentWithLines(viewContent string, startLine int) (string, int) {
	lines := strings.Split(viewContent, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" && !strings.HasSuffix(viewContent, "\n") {
		lines = lines[:len(lines)-1]
	}

	var sb strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&sb, "%4d | %s\n", startLine+i, line)
	}
	return sb.String(), len(lines)
}

func enrichTypes(ctx context.Context, filename string, content []byte) (string, error) {
	fset := token.NewFileSet()
	f, parseErr := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if parseErr != nil {
		//nolint:nilerr // syntax/parse errors are gracefully skipped during type enrichment
		return "", nil
	}

	importMap := parseImports(f)
	var typeDefs []string
	seenDefs := make(map[string]bool)

	extractLocalTypes(f, fset, &typeDefs, seenDefs)
	resolveImportedTypes(ctx, f, importMap, &typeDefs, seenDefs)

	return formatDefinitions(typeDefs), nil
}

func parseImports(f *ast.File) map[string]string {
	importMap := make(map[string]string)
	for _, imp := range f.Imports {
		if imp.Path == nil {
			continue
		}
		path := strings.Trim(imp.Path.Value, `"`)
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		} else {
			parts := strings.Split(path, "/")
			alias = parts[len(parts)-1]
		}
		if alias != "" && alias != "_" && alias != "." {
			importMap[alias] = path
		}
	}
	return importMap
}

func extractLocalTypes(f *ast.File, fset *token.FileSet, typeDefs *[]string, seenDefs map[string]bool) {
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			var buf bytes.Buffer
			if err := printer.Fprint(&buf, fset, typeSpec); err == nil {
				def := buf.String()
				if !seenDefs[def] {
					seenDefs[def] = true
					*typeDefs = append(*typeDefs, def)
				}
			}
		}
	}
}

func resolveImportedTypes(ctx context.Context, f *ast.File, importMap map[string]string,
	typeDefs *[]string, seenDefs map[string]bool) {
	type importedSym struct {
		pkgPath string
		symName string
	}
	var importedSyms []importedSym
	seenSyms := make(map[string]bool)

	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		xIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		pkgPath, exists := importMap[xIdent.Name]
		if !exists {
			return true
		}
		symName := sel.Sel.Name
		key := pkgPath + "." + symName
		if !seenSyms[key] {
			seenSyms[key] = true
			importedSyms = append(importedSyms, importedSym{pkgPath: pkgPath, symName: symName})
		}
		return true
	})

	for _, sym := range importedSyms {
		doc, err := godoc.Load(ctx, sym.pkgPath, sym.symName)
		if err == nil && doc != nil && doc.Definition != "" {
			def := fmt.Sprintf("// %s.%s\n%s", doc.Package, sym.symName, doc.Definition)
			if !seenDefs[def] {
				seenDefs[def] = true
				*typeDefs = append(*typeDefs, def)
			}
		}
	}
}

func formatDefinitions(typeDefinitions []string) string {
	if len(typeDefinitions) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("<types>\n")
	for _, def := range typeDefinitions {
		sb.WriteString(def)
		sb.WriteString("\n\n")
	}
	sb.WriteString("</types>\n")
	return sb.String()
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}
}
