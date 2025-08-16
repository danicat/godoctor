package result

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewError creates a new CallToolResult with IsError set to true.
func NewError(format string, a ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf(format, a...)},
		},
	}
}

// NewText creates a new CallToolResult with a single text content.
func NewText(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}