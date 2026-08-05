// Package navigation implements tools for navigating Go source code.
package navigation

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/danicat/godoctor/internal/godoc"
	"github.com/danicat/godoctor/internal/roots"
	"github.com/danicat/godoctor/internal/toolnames"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	def := toolnames.Registry["describe_symbol"]
	mcp.AddTool(server, &mcp.Tool{
		Name:        def.Name,
		Title:       def.Title,
		Description: def.Description,
	}, Handler)
}

// Params defines the input parameters for describe_symbol.
type Params struct {
	Filename string `json:"filename" jsonschema:"The absolute path to the Go file. Must be absolute."`
	Line     int    `json:"line" jsonschema:"The 1-indexed line number of the symbol"`
	Col      int    `json:"col" jsonschema:"The 1-indexed column number of the symbol"`
}

// Handler handles the describe_symbol tool execution.
func Handler(ctx context.Context, req *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	var session *mcp.ServerSession
	if req != nil {
		session = req.Session
	}
	absPath, err := roots.Global.Validate(session, args.Filename)
	if err != nil {
		return errorResult(err.Error()), nil, err
	}

	definition, symbol, err := fetchASTDefinition(ctx, absPath, args.Line, args.Col)
	if err != nil {
		return errorResult("Failed to query symbol definition: " + err.Error()), nil, err
	}

	references := fetchASTReferences(absPath, symbol)

	// Format into Markdown
	var sb strings.Builder
	fmt.Fprintf(&sb, "## Symbol Description for `%s:%d:%d`\n\n", filepathBase(absPath), args.Line, args.Col)
	sb.WriteString("### Definition & Signature\n")
	sb.WriteString("```\n")
	sb.WriteString(definition)
	sb.WriteString("\n```\n\n")

	sb.WriteString("### Workspace References\n")
	sb.WriteString("```\n")
	sb.WriteString(references)
	sb.WriteString("\n```\n")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: sb.String()},
		},
	}, nil, nil
}

func fetchASTDefinition(ctx context.Context, path string, line, col int) (string, string, error) {
	fset := token.NewFileSet()
	//nolint:gosec // G304: File path provided by user is expected.
	content, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}

	f, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return "", "", err
	}

	var targetIdent *ast.Ident
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		pos := fset.Position(n.Pos())
		if pos.Line == line {
			if id, ok := n.(*ast.Ident); ok {
				endPos := fset.Position(n.End())
				if col >= pos.Column && col <= endPos.Column {
					targetIdent = id
					return false
				}
				if targetIdent == nil {
					targetIdent = id
				}
			}
		}
		return true
	})

	if targetIdent == nil {
		return "No symbol found at given coordinates.", "", nil
	}

	symName := targetIdent.Name

	if targetIdent.Obj != nil && targetIdent.Obj.Decl != nil {
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, fset, targetIdent.Obj.Decl); err == nil {
			pos := fset.Position(targetIdent.Obj.Pos())
			res := fmt.Sprintf("Symbol: %s\nLocation: %s:%d:%d\n\n%s",
				symName, filepath.Base(pos.Filename), pos.Line, pos.Column, buf.String())
			return res, symName, nil
		}
	}

	// Try godoc lookup if local obj decl isn't directly attached
	dir := filepath.Dir(path)
	doc, docErr := godoc.Load(ctx, dir, symName)
	if docErr == nil && doc != nil && doc.Definition != "" {
		res := fmt.Sprintf("Symbol: %s\nPackage: %s\n\n%s\n\n%s",
			symName, doc.Package, doc.Definition, doc.Description)
		return res, symName, nil
	}

	return fmt.Sprintf("Symbol: %s", symName), symName, nil
}

const noReferencesFound = "No references found."

func fetchASTReferences(path, symbol string) string {
	if symbol == "" {
		return noReferencesFound
	}
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "No references found."
	}

	var refs []string
	fset := token.NewFileSet()

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		filePath := filepath.Join(dir, entry.Name())
		//nolint:gosec // G304: File path provided by user is expected.
		f, err := parser.ParseFile(fset, filePath, nil, 0)
		if err != nil {
			continue
		}

		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == symbol {
				pos := fset.Position(id.Pos())
				refs = append(refs, fmt.Sprintf("%s:%d:%d", entry.Name(), pos.Line, pos.Column))
			}
			return true
		})
	}

	if len(refs) == 0 {
		return "No references found."
	}
	return strings.Join(refs, "\n")
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}
}

func filepathBase(path string) string {
	idx := strings.LastIndexAny(path, "/\\")
	if idx == -1 {
		return path
	}
	return path[idx+1:]
}
