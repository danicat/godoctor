package godoc

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the go-doc tool with the server.
func Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "go-doc",
		Description: `
The go-doc tool retrieves Go documentation for a specified package and, optionally, a specific symbol within that package. It acts as a direct interface to the standard 'go doc' command-line tool.

When to use:
This tool should be used to understand the functionality of a Go package or a specific symbol (function, type, etc.) within it. It is useful for exploring the codebase, understanding dependencies, or finding the correct function for a task before using or modifying it.

Parameters:
- package_path (string, required): The full import path of the Go package (e.g., "fmt", "github.com/spf13/cobra").
- symbol_name (string, optional): The name of a specific symbol within the package (e.g., "Println", "Command").

Output:
- On success: A string containing the documentation text.
- If not found: The string "documentation not found".
- On error: An error message detailing the failure.

Examples:
- Get documentation for the 'fmt' package:
  {"package_path": "fmt"}
- Get documentation for the 'Println' function in the 'fmt' package:
  {"package_path": "fmt", "symbol_name": "Println"}
`,
	}, getDocHandler)
}

// GetDocParams defines the input parameters for the go-doc tool.
type GetDocParams struct {
	PackagePath string `json:"package_path"`
	SymbolName  string `json:"symbol_name,omitempty"`
}

func getDocHandler(ctx context.Context, s *mcp.ServerSession, request *mcp.CallToolParamsFor[GetDocParams]) (*mcp.CallToolResult, error) {
	pkgPath := request.Arguments.PackagePath
	symbolName := request.Arguments.SymbolName

	if pkgPath == "" {
		return newErrorResult("package_path cannot be empty"), nil
	}

	args := []string{"doc"}
	if symbolName == "" {
		args = append(args, pkgPath)
	} else {
		args = append(args, "-short", pkgPath, symbolName)
	}

	cmd := exec.CommandContext(ctx, "go", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return newErrorResult("`go doc` failed for package %q, symbol %q: %s", pkgPath, symbolName, out.String()), nil
	}

	docString := strings.TrimSpace(out.String())
	if docString == "" {
		docString = "documentation not found"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: docString},
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
