// Package read implements the code reading and symbol extraction tool with unconditional type enrichment.
package read

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/danicat/godoctor/internal/roots"
	"github.com/danicat/godoctor/internal/toolnames"
	"github.com/danicat/godoctor/internal/tools/file/outline"
	"github.com/danicat/godoctor/internal/tools/shared"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the smart_read tool with the server.
func Register(server *mcp.Server) {
	def := toolnames.Registry["smart_read"]
	mcp.AddTool(server, &mcp.Tool{
		Name:        def.Name,
		Title:       def.Title,
		Description: def.Description,
	}, readCodeHandler)
}

// Params defines the input parameters for the smart_read tool.
type Params struct {
	Filenames []string `json:"filenames,omitempty" jsonschema:"The absolute paths to the Go files to read. You MUST use absolute paths in multi-root workspaces."`
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

	// 0. Outline Mode
	if args.Outline && args.StartLine == 0 {
		var sb strings.Builder
		for _, filename := range filenames {
			absPath, err := roots.Global.Validate(session, filename)
			if err != nil {
				return errorResult(err.Error()), nil, nil
			}
			out, imports, errs, err := outline.GetOutline(absPath)
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

	// 1. Multi-File Read Content
	var sb strings.Builder
	var allTypesEnrichment strings.Builder

	for _, filename := range filenames {
		absPath, err := roots.Global.Validate(session, filename)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}

		//nolint:gosec // G304: File path provided by user is validated against roots.
		content, err := os.ReadFile(absPath)
		if err != nil {
			return errorResult(fmt.Sprintf("failed to read file %s: %v", filename, err)), nil, nil
		}

		isGo := strings.HasSuffix(absPath, ".go")
		original := string(content)

		startLine := args.StartLine
		if startLine <= 0 {
			startLine = 1
		}
		endLine := args.EndLine

		startOffset, endOffset, err := shared.GetLineOffsets(original, startLine, endLine)
		if err != nil {
			return errorResult(fmt.Sprintf("line range error for %s: %v", filename, err)), nil, nil
		}

		viewContent := original[startOffset:endOffset]
		lines := strings.Split(viewContent, "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" && !strings.HasSuffix(viewContent, "\n") {
			lines = lines[:len(lines)-1]
		}

		var contentWithLines strings.Builder
		for i, line := range lines {
			fmt.Fprintf(&contentWithLines, "%4d | %s\n", startLine+i, line)
		}

		isPartial := args.StartLine > 1 || args.EndLine > 0
		rangeInfo := ""
		if isPartial {
			rangeInfo = fmt.Sprintf(" (Lines %d-%d)", startLine, startLine+len(lines)-1)
		}
		fmt.Fprintf(&sb, "# File: %s%s\n\n", absPath, rangeInfo)

		sb.WriteString("```")
		if isGo {
			sb.WriteString("go")
		}
		sb.WriteString("\n")
		sb.WriteString(contentWithLines.String())
		sb.WriteString("```\n\n")

		if isGo {
			// Type enrichment
			enrichment := enrichTypes(ctx, absPath, content)
			if enrichment != "" {
				allTypesEnrichment.WriteString(enrichment)
			}
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

func getInterestingTypePos(n ast.Expr) token.Pos {
	if n == nil {
		return token.NoPos
	}
	switch node := n.(type) {
	case *ast.Ident:
		switch node.Name {
		case "string", "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
			"float32", "float64", "complex64", "complex128",
			"bool", "byte", "rune", "error", "any":
			return token.NoPos
		}
		return node.Pos()
	case *ast.SelectorExpr:
		return node.Sel.Pos()
	case *ast.StarExpr:
		return getInterestingTypePos(node.X)
	case *ast.ArrayType:
		return getInterestingTypePos(node.Elt)
	case *ast.MapType:
		pos := getInterestingTypePos(node.Value)
		if pos != token.NoPos {
			return pos
		}
		return getInterestingTypePos(node.Key)
	}
	return token.NoPos
}

func enrichTypes(ctx context.Context, filename string, content []byte) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if err != nil {
		return ""
	}

	visitedPositions := make(map[string]bool)
	var posList []token.Position

	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		var typeExpr ast.Expr
		switch node := n.(type) {
		case *ast.TypeSpec:
			typeExpr = node.Name
		case *ast.Field:
			typeExpr = node.Type
		case *ast.ValueSpec:
			typeExpr = node.Type
		case *ast.CompositeLit:
			typeExpr = node.Type
		}

		if typeExpr != nil {
			pos := getInterestingTypePos(typeExpr)
			if pos.IsValid() {
				position := fset.Position(pos)
				key := fmt.Sprintf("%d:%d", position.Line, position.Column)
				if !visitedPositions[key] {
					visitedPositions[key] = true
					posList = append(posList, position)
				}
			}
		}
		return true
	})

	if len(posList) == 0 {
		return ""
	}

	var mu sync.Mutex
	var typeDefinitions []string
	var uniqueDefs = make(map[string]bool)

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)

	for _, pos := range posList {
		wg.Add(1)
		go func(position token.Position) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			posStr := fmt.Sprintf("%s:%d:%d", filename, position.Line, position.Column)
			cmd := exec.CommandContext(ctx, "gopls", "definition", posStr)
			out, err := cmd.CombinedOutput()
			if err == nil {
				defStr := strings.TrimSpace(string(out))
				if defStr != "" && (strings.Contains(defStr, "struct {") || strings.Contains(defStr, "interface {") || strings.Contains(defStr, "func(")) {
					mu.Lock()
					if !uniqueDefs[defStr] {
						uniqueDefs[defStr] = true
						typeDefinitions = append(typeDefinitions, defStr)
					}
					mu.Unlock()
				}
			}
		}(pos)
	}
	wg.Wait()

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
