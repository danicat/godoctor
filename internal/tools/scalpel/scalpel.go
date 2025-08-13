package scalpel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/imports"
)

// Register registers the scalpel tool with the server.
func Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "scalpel",
		Description: "Edits an existing Go source file by replacing a fragment and checks it for errors.",
	}, scalpelHandler)
}

// ScalpelParams defines the input parameters for the scalpel tool.
type ScalpelParams struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func scalpelHandler(_ context.Context, _ *mcp.ServerSession, request *mcp.CallToolParamsFor[ScalpelParams]) (*mcp.CallToolResult, error) {
	path := request.Arguments.FilePath
	oldString := request.Arguments.OldString
	newString := request.Arguments.NewString

	// Check if the file exists.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return newErrorResult("file does not exist: %s", path), nil
	} else if err != nil {
		return newErrorResult("failed to check file status: %v", err), nil
	}

	originalContent, err := os.ReadFile(path)
	if err != nil {
		return newErrorResult("failed to read file: %v", err), nil
	}

	newContent := strings.Replace(string(originalContent), oldString, newString, 1)
	byteContent := []byte(newContent)

	if err := os.WriteFile(path, byteContent, 0644); err != nil {
		return newErrorResult("failed to write file: %v", err), nil
	}

	if filepath.Ext(path) != ".go" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "File edited successfully."},
			},
		}, nil
	}

	check, err := goCheck(path)
	if err != nil {
		return newErrorResult("go check failed: %v", err), nil
	}
	if check != "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: check},
			},
		}, nil
	}

	formattedSrc, err := formatGoSource(path, byteContent)
	if err != nil {
		return newErrorResult("formatting failed: %v", err), nil
	}

	if err := os.WriteFile(path, formattedSrc, 0644); err != nil {
		return newErrorResult("failed to write formatted file: %v", err), nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "File edited successfully."},
		},
	}, nil
}

func goCheck(path string) (string, error) {
	cmd := exec.Command("gopls", "check", path)
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
	return imports.Process(path, content, nil)
}

func newErrorResult(format string, args ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf(format, args...)},
		},
	}
}
