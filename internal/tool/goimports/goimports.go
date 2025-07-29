package goimports

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/imports"
)

// Register registers the goimports tool with the server.
func Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "goimports",
		Description: "Formats a Go source file using goimports.",
	}, goimportsHandler)
}

// GoImportsParams defines the input parameters for the goimports tool.
type GoImportsParams struct {
	FilePath string `json:"file_path"`
}

func goimportsHandler(ctx context.Context, s *mcp.ServerSession, request *mcp.CallToolParamsFor[GoImportsParams]) (*mcp.CallToolResult, error) {
	filePath := request.Arguments.FilePath
	if filePath == "" {
		return newErrorResult("file_path cannot be empty"), nil
	}

	src, err := os.ReadFile(filePath)
	if err != nil {
		return newErrorResult("failed to read file: %v", err), nil
	}

	res, err := imports.Process(filePath, src, nil)
	if err != nil {
		return newErrorResult("failed to process file: %v", err), nil
	}

	if err := os.WriteFile(filePath, res, 0644); err != nil {
		return newErrorResult("failed to write file: %v", err), nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "file formatted successfully"},
		},
	}, nil
}

func newErrorResult(format string, a ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf(format, a...)},
		},
	}
}
