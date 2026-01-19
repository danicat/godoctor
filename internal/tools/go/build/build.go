// Package build implements the go build tool.
package build

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/danicat/godoctor/internal/toolnames"
	"github.com/danicat/godoctor/internal/tools/shared"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	def := toolnames.Registry["go_build"]
	mcp.AddTool(server, &mcp.Tool{
		Name:        def.Name,
		Title:       def.Title,
		Description: def.Description,
	}, Handler)
}

// Params defines the input parameters.
type Params struct {
	Dir      string   `json:"dir,omitempty" jsonschema:"Directory to build in (default: current)"`
	Packages []string `json:"packages,omitempty" jsonschema:"Packages to build (default: ./...)"`
	Args     []string `json:"args,omitempty" jsonschema:"Additional build arguments (e.g. -o, -race, -tags, -v)"`
}

func Handler(ctx context.Context, _ *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	dir := args.Dir
	if dir == "" {
		dir = "."
	}
	pkgs := args.Packages
	if len(pkgs) == 0 {
		pkgs = []string{"./..."}
	}

	cmdArgs := append([]string{"build"}, args.Args...)
	cmdArgs = append(cmdArgs, pkgs...)

	//nolint:gosec // G204: Subprocess launched with variable is expected behavior for this tool.
	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		if output == "" {
			output = fmt.Sprintf("Build failed: %v", err)
		} else {
			output = "Build Failed:\n" + output
		}
		output += shared.GetDocHintFromOutput(output)
	} else {
		if output == "" {
			output = "Build Successful."
		} else {
			output = "Build Successful:\n" + output
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: output},
		},
	}, nil, nil
}
