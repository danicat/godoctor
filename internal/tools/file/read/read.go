// Package read implements the code reading and symbol extraction tool with unconditional type enrichment.
package read

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"sync"

	"github.com/danicat/godoctor/internal/lsp"
	"github.com/danicat/godoctor/internal/roots"
	"github.com/danicat/godoctor/internal/toolnames"
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

	startOffset, endOffset, err := shared.GetLineOffsets(original, startLine, endLine)
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
			return "", "", errorResult(fmt.Sprintf("failed to enrich types via gopls: %v", enrichErr))
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

func enrichTypes(ctx context.Context, filename string, content []byte) (string, error) {
	fset := token.NewFileSet()
	f, parseErr := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if parseErr != nil {
		// If the file is syntactically invalid (e.g. broken, incomplete, or a plain text mock file),
		// we gracefully skip type enrichment instead of throwing a fatal LSP client error.
		//nolint:nilerr // syntax/parse errors are gracefully skipped during type enrichment
		return "", nil
	}

	posList := extractTypePositions(fset, f)
	if len(posList) == 0 {
		return "", nil
	}

	return resolveDefinitions(ctx, filename, posList)
}

func extractTypePositions(fset *token.FileSet, f *ast.File) []token.Position {
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
	return posList
}

func resolveDefinitions(ctx context.Context, filename string, posList []token.Position) (string, error) {
	client, err := lsp.GlobalManager.Client(ctx)
	if err != nil {
		return "", err
	}

	var mu sync.Mutex
	var typeDefinitions []string
	var uniqueDefs = make(map[string]bool)
	var wg sync.WaitGroup

	for _, pos := range posList {
		wg.Add(1)
		go func(position token.Position) {
			defer wg.Done()
			locs, err := client.GetDefinition(ctx, filename, position.Line, position.Column)
			if err == nil && len(locs) > 0 {
				loc := locs[0]
				defStr := fmt.Sprintf(
					"%s:%d:%d -> Definition coordinate resolved",
					loc.URI,
					loc.Range.Start.Line+1,
					loc.Range.Start.Character+1,
				)
				mu.Lock()
				if !uniqueDefs[defStr] {
					uniqueDefs[defStr] = true
					typeDefinitions = append(typeDefinitions, defStr)
				}
				mu.Unlock()
			}
		}(pos)
	}
	wg.Wait()

	return formatDefinitions(typeDefinitions), nil
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
