// Package mutationtest implements the mutation testing tool using selene.
package mutationtest

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danicat/godoctor/internal/safeshell"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	//nolint:lll
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mutation_test",
		Title:       "Mutation Test",
		Description: "Runs mutation testing using Selene. Introduces small code mutations (flipped conditions, swapped operators) and checks if existing tests catch them, objectively measuring test suite quality.",
	}, toolHandler)
}

// Params defines the input parameters.
type Params struct {
	//nolint:lll
	Dir string `json:"dir,omitempty" jsonschema:"The absolute directory path to run mutation testing in. Always pass absolute paths in multi-root workspaces."`
}

func toolHandler(ctx context.Context, _ *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	dir := args.Dir
	if dir == "" {
		dir = "."
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return errorResult(err.Error()), nil, nil
	}

	var seleneCmd string
	var seleneArgs []string
	if _, err := exec.LookPath("selene"); err == nil {
		seleneCmd = "selene"
		seleneArgs = []string{"./..."}
	} else {
		seleneCmd = "go"
		seleneArgs = []string{"run", "github.com/danicat/selene/cmd/selene@latest", "./..."}
	}

	cmd, err := safeshell.CommandContext(ctx, seleneCmd, seleneArgs...)
	if err != nil {
		return errorResult(fmt.Sprintf("secure execution validation failed: %v", err)), nil, nil
	}
	cmd.Dir = absDir
	out, runErr := cmd.CombinedOutput()

	output := filterNoise(string(out))

	if runErr != nil && output == "" {
		return errorResult(fmt.Sprintf("mutation testing failed to run: %v", runErr)), nil, nil
	}

	if output == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "✅ All mutations were caught by tests."},
			},
		}, nil, nil
	}

	// selene exits with code 1 if mutations survive
	if runErr != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("🧬 Mutation testing results:\n%v\n%s", runErr, output)},
			},
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("✅ Mutation testing results:\n\n%s", output)},
		},
	}, nil, nil
}

func filterNoise(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	var filtered []string
	for _, line := range lines {
		if strings.HasPrefix(line, "go: downloading ") || strings.Contains(line, "exit status") {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.TrimSpace(strings.Join(filtered, "\n"))
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}
}
