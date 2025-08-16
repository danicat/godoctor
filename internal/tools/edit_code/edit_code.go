package edit_code

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danicat/godoctor/internal/mcp/result"
	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/imports"
)

// Register registers the edit_code tool with the server.
func Register(server *mcp.Server) {
	name := "edit_code"
	schema, err := jsonschema.For[EditCodeParams]()
	if err != nil {
		panic(err)
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       "Edit Go File",
		Description: "Edits a Go source file by replacing the first occurrence of a specified 'old_string' with a 'new_string'. Use this for surgical edits like adding, deleting, or renaming code when the changes affect less than 25% of the file. To ensure precision, the 'old_string' must be a unique anchor string that includes enough context to target only the desired location.",
		InputSchema: schema,
	}, editCodeHandler)
}

// EditCodeParams defines the input parameters for the edit_code tool.
type EditCodeParams struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func editCodeHandler(ctx context.Context, _ *mcp.ServerSession, request *mcp.CallToolParamsFor[EditCodeParams]) (*mcp.CallToolResult, error) {
	path := request.Arguments.FilePath
	oldString := request.Arguments.OldString
	newString := request.Arguments.NewString

	// Check if the file exists.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return result.NewError("file does not exist: %s", path), nil
	} else if err != nil {
		return result.NewError("failed to check file status: %v", err), nil
	}

	originalContent, err := os.ReadFile(path)
	if err != nil {
		return result.NewError("failed to read file: %v", err), nil
	}

	newContent := strings.Replace(string(originalContent), oldString, newString, 1)
	byteContent := []byte(newContent)

	if err := os.WriteFile(path, byteContent, 0644); err != nil {
		return result.NewError("failed to write file: %v", err), nil
	}

	if filepath.Ext(path) != ".go" {
		return result.NewText("File edited successfully."), nil
	}

	check, err := goCheck(ctx, path)
	if err != nil {
		return result.NewError("go check failed: %v", err), nil
	}
	if check != "" {
		// Revert the file to its original content before returning the error.
		if err := os.WriteFile(path, originalContent, 0644); err != nil {
			return result.NewError("failed to revert file: %v\n\nOriginal error:\n%s", err, check), nil
		}
		return result.NewError("Edit code replacement resulted in invalid Go code. The file has been reverted. You MUST fix the Go code in your `new_string` parameter before trying again. Compiler error:\n%s", check), nil
	}

	formattedSrc, err := formatGoSource(path, byteContent)
	if err != nil {
		return result.NewError("formatting failed: %v", err), nil
	}

	if err := os.WriteFile(path, formattedSrc, 0644); err != nil {
		return result.NewError("failed to write formatted file: %v", err), nil
	}

	return result.NewText("File edited successfully."), nil
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
	return imports.Process(path, content, nil)
}
