// Package readdocs implements the documentation retrieval tool.
package readdocs

import (
	"context"
	"fmt"
	"strings"

	"github.com/danicat/godoctor/internal/godoc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	//nolint:lll
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_docs",
		Title:       "Get Documentation",
		Description: "Retrieves authoritative Go documentation for any package or symbol. Streamlines development by providing API signatures and usage examples directly within the workflow.",
	}, Handler)
}

// Params defines the input parameters for the read_docs tool.
type Params struct {
	ImportPath string `json:"import_path" jsonschema:"Import path of the package (e.g. 'fmt')"`
	SymbolName string `json:"symbol_name,omitempty" jsonschema:"Optional symbol name to lookup"`
	Format     string `json:"format,omitempty" jsonschema:"Output format: 'markdown' (default) or 'json'"`
}

// Handler handles the read_docs tool execution.
func Handler(ctx context.Context, _ *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	if args.ImportPath == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "import_path cannot be empty"},
			},
		}, nil, nil
	}

	// Default to markdown
	if args.Format == "" {
		args.Format = "markdown"
	}
	args.Format = strings.ToLower(args.Format)
	if args.Format != "markdown" && args.Format != "json" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "invalid format: must be 'markdown' or 'json'"},
			},
		}, nil, nil
	}

	// Use LoadWithFallback for flexibility on typos
	doc, err := godoc.LoadWithFallback(ctx, args.ImportPath, args.SymbolName)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("failed to read documentation: %v", err)},
			},
		}, nil, nil
	}

	var output string

	if args.Format == "json" {
		var jsonErr error
		output, jsonErr = doc.RenderJSON()
		if jsonErr != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("failed to marshal JSON: %v", jsonErr)},
				},
			}, nil, nil
		}
	} else {
		output = doc.RenderMarkdown()
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: output},
		},
	}, nil, nil
}
