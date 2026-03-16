// Package mutation implements the mutation testing tool using selene.
package mutation

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
	def := toolnames.Registry["mutation_test"]
	mcp.AddTool(server, &mcp.Tool{
		Name:        def.Name,
		Title:       def.Title,
		Description: def.Description,
	}, toolHandler)
}

// Params defines the input parameters.
type Params struct {
	Dir string `json:"dir,omitempty" jsonschema:"Directory to run mutation testing in (default: current)"`
}

func toolHandler(ctx context.Context, _ *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	dir := args.Dir
	if dir == "" {
		dir = "."
	}

	absDir, err := roots.Global.Validate(dir)
	if err != nil {
		return errorResult(err.Error()), nil, nil
	}

	cmd := exec.CommandContext(ctx, "go", "run", "github.com/danicat/selene/cmd/selene@latest", "./...")
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
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("🧬 Mutation testing results:\n\n%s", output)},
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
