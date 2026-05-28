// Package navigation implements tools for navigating Go source code.
package navigation

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/danicat/godoctor/internal/roots"
	"github.com/danicat/godoctor/internal/toolnames"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	def := toolnames.Registry["describe_symbol"]
	mcp.AddTool(server, &mcp.Tool{
		Name:        def.Name,
		Title:       def.Title,
		Description: def.Description,
	}, Handler)
}

// Params defines the input parameters for describe_symbol.
type Params struct {
	Filename string `json:"filename" jsonschema:"The absolute path to the Go file containing the symbol. You MUST pass the absolute path in multi-root workspaces."`
	Line     int    `json:"line" jsonschema:"The 1-indexed line number of the symbol"`
	Col      int    `json:"col" jsonschema:"The 1-indexed column number of the symbol"`
}

// Runner defines the interface for running commands (facilitates testing).
type Runner interface {
	Run(ctx context.Context, dir, name string, args ...string) (string, error)
}

type stdRunner struct{}

func (r *stdRunner) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// CommandRunner is the standard command executor.
var CommandRunner Runner = &stdRunner{}

// Handler handles the describe_symbol tool execution.
func Handler(ctx context.Context, req *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	var session *mcp.ServerSession
	if req != nil {
		session = req.Session
	}
	absPath, err := roots.Global.Validate(session, args.Filename)
	if err != nil {
		return errorResult(err.Error()), nil, nil
	}

	position := fmt.Sprintf("%s:%d:%d", absPath, args.Line, args.Col)

	// 1. Run gopls definition
	defOut, defErr := CommandRunner.Run(ctx, "", "gopls", "definition", position)
	if defErr != nil {
		// Clean up the error message slightly for better LLM consumption
		errMsg := strings.TrimSpace(defOut)
		if errMsg == "" {
			errMsg = defErr.Error()
		}
		return errorResult(fmt.Sprintf("Failed to find symbol definition at %s: %s", position, errMsg)), nil, nil
	}

	// 2. Run gopls references
	refOut, refErr := CommandRunner.Run(ctx, "", "gopls", "references", position)
	var references string
	if refErr != nil {
		references = fmt.Sprintf("⚠️ Failed to find references: %s", strings.TrimSpace(refOut))
	} else {
		references = strings.TrimSpace(refOut)
		if references == "" {
			references = "No references found."
		}
	}

	// 3. Format into Markdown
	var sb strings.Builder
	fmt.Fprintf(&sb, "## Symbol Description for `%s:%d:%d`\n\n", filepathBase(absPath), args.Line, args.Col)
	sb.WriteString("### Definition & Signature\n")
	sb.WriteString("```\n")
	sb.WriteString(strings.TrimSpace(defOut))
	sb.WriteString("\n```\n\n")

	sb.WriteString("### Workspace References\n")
	sb.WriteString("```\n")
	sb.WriteString(references)
	sb.WriteString("\n```\n")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: sb.String()},
		},
	}, nil, nil
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}
}

// Simple helper to avoid importing "path/filepath" unless necessary, or just extract basename.
func filepathBase(path string) string {
	idx := strings.LastIndexAny(path, "/\\")
	if idx == -1 {
		return path
	}
	return path[idx+1:]
}
