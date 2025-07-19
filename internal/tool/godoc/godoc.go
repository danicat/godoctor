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
		Description: "Retrieves Go documentation for a given package and optional symbol. This tool shells out to the `go doc` command.",
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
