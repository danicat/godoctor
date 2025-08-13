package scribble

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the scribble tool with the server.
func Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "scribble",
		Description: "Writes content to a new Go source file and checks it for errors. This tool should be used whenever you are creating a new Go file.",
	}, scribbleHandler)
}

// ScribbleParams defines the input parameters for the scribble tool.
type ScribbleParams struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// DiagnosticError defines the structured output for a single diagnostic error.
type DiagnosticError struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

func scribbleHandler(_ context.Context, _ *mcp.ServerSession, request *mcp.CallToolParamsFor[ScribbleParams]) (*mcp.CallToolResult, error) {
	path := request.Arguments.FilePath
	content := request.Arguments.Content

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return newErrorResult("failed to write file: %v", err), nil
	}

	if filepath.Ext(path) != ".go" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "File written successfully."},
			},
		}, nil
	}

	errs, err := goCheck(path)
	if err != nil {
		return newErrorResult("go check failed: %v", err), nil
	}
	if len(errs) > 0 {
		jsonErrs, err := json.MarshalIndent(errs, "", "  ")
		if err != nil {
			return newErrorResult("failed to marshal errors: %v", err), nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(jsonErrs)},
			},
		}, nil
	}

	if err := runGoTool("gofmt", path); err != nil {
		return newErrorResult("gofmt failed: %v", err), nil
	}

	if err := runGoTool("goimports", path); err != nil {
		return newErrorResult("goimports failed: %v", err), nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "File written successfully."},
		},
	}, nil
}

func goCheck(path string) ([]DiagnosticError, error) {
	cmd := exec.Command("gopls", "check", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gopls check failed: %w\noutput: %s", err, string(output))
	}

	var errors []DiagnosticError
	for _, line := range strings.Split(string(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 4 {
			continue
		}
		lineNum, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to parse line number: %w", err)
		}
		errors = append(errors, DiagnosticError{
			File:    parts[0],
			Line:    lineNum,
			Message: strings.TrimSpace(parts[3]),
		})
	}
	return errors, nil
}

func runGoTool(name, path string) error {
	cmd := exec.Command(name, "-w", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s failed: %w\noutput: %s", name, err, string(output))
	}
	return nil
}

func newErrorResult(format string, args ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf(format, args...)},
		},
	}
}
