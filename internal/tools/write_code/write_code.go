package write_code

import (
	"context"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/danicat/godoctor/internal/mcp/result"
	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/imports"
)

// Register registers the write_code tool with the server.
func Register(server *mcp.Server) {
	name := "write_code"
	schema, err := jsonschema.For[WriteCodeParams]()
	if err != nil {
		panic(err)
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       "Create Go File",
		Description: "Creates or replaces an entire Go source file with the provided content. Use this tool when the extent of edits to a file is substantial, affecting more than 25% of the file's content. It automatically formats the code and manages imports.",
		InputSchema: schema,
	}, writeCodeHandler)
}

// WriteCodeParams defines the input parameters for the write_code tool.
type WriteCodeParams struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func writeCodeHandler(ctx context.Context, _ *mcp.ServerSession, request *mcp.CallToolParamsFor[WriteCodeParams]) (*mcp.CallToolResult, error) {
	path := request.Arguments.FilePath
	content := request.Arguments.Content
	byteContent := []byte(content)

	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return result.NewError("failed to create directory: %v", err), nil
		}
	}

	if err := os.WriteFile(path, byteContent, 0644); err != nil {
		return result.NewError("failed to write file: %v", err), nil
	}

	if filepath.Ext(path) != ".go" {
		return result.NewText("File written successfully."), nil
	}

	check, err := goCheck(ctx, path)
	if err != nil {
		return result.NewError("go check failed: %v", err), nil
	}
	if check != "" {
		// Remove the invalid file before returning the error.
		if err := os.Remove(path); err != nil {
			return result.NewError("failed to remove invalid file: %v\n\nOriginal error:\n%s", err, check), nil
		}
		return result.NewError("Write code resulted in invalid Go code. The file has been deleted. You MUST fix the Go code in your `content` parameter before trying again. Compiler error:\n%s", check), nil
	}

	formattedSrc, err := formatGoSource(path, byteContent)
	if err != nil {
		return result.NewError("formatting failed: %v", err), nil
	}

	if err := os.WriteFile(path, formattedSrc, 0644); err != nil {
		return result.NewError("failed to write formatted file: %v", err), nil
	}

	return result.NewText("File written successfully."), nil
}

func goCheck(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "gopls", "check", path)
	cmd.Dir = filepath.Dir(path)
	output, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return "", fmt.Errorf("failed to run gopls: %w", err)
		}
	}
	return string(output), nil
}

func formatGoSource(path string, content []byte) ([]byte, error) {
	importedSrc, err := imports.Process(path, content, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to process imports: %w", err)
	}
	return format.Source(importedSrc)
}
