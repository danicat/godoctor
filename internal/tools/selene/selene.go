// Package selene implements the mutation testing tool using selene.
package selene

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danicat/godoctor/internal/safeshell"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const toolName = "selene"

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	//nolint:lll
	mcp.AddTool(server, &mcp.Tool{
		Name:        toolName,
		Title:       "Mutation Test",
		Description: "Runs mutation testing using Selene. Introduces small code mutations (flipped conditions, swapped operators) and checks if existing tests catch them, objectively measuring test suite quality.",
	}, Handler)
}

// Params defines the input parameters.
type Params struct {
	//nolint:lll
	Dir string `json:"dir" jsonschema:"The absolute directory path to run mutation testing in. Required. Relative paths are rejected."`
	//nolint:lll
	Packages string `json:"packages,omitempty" jsonschema:"Packages to run mutation testing on (default: ./...)"`
}

// Runner defines the interface for running CLI commands.
type Runner interface {
	Run(ctx context.Context, dir, name string, args ...string) error
	RunWithOutput(ctx context.Context, dir, name string, args ...string) (string, error)
	LookPath(file string) (string, error)
}

type stdRunner struct{}

func (r *stdRunner) Run(ctx context.Context, dir, name string, args ...string) error {
	cmd, err := safeshell.CommandContext(ctx, name, args...)
	if err != nil {
		return err
	}
	cmd.Dir = dir
	return cmd.Run()
}

func (r *stdRunner) RunWithOutput(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd, err := safeshell.CommandContext(ctx, name, args...)
	if err != nil {
		return "", err
	}
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (r *stdRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// CommandRunner is used to execute CLI commands.
var CommandRunner Runner = &stdRunner{}

// Handler handles the selene tool execution.
func Handler(ctx context.Context, _ *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(args.Dir) == "" || !filepath.IsAbs(args.Dir) {
		return errorResult("dir is required and must be an absolute path"), nil, nil
	}

	absDir := filepath.Clean(args.Dir)

	pkgs := args.Packages
	if strings.TrimSpace(pkgs) == "" {
		pkgs = "./..."
	}
	pkgList := parsePackages(pkgs)

	var seleneCmd string
	var seleneArgs []string
	if _, err := CommandRunner.LookPath(toolName); err == nil {
		seleneCmd = toolName
		seleneArgs = pkgList
	} else {
		seleneCmd = "go"
		seleneArgs = append([]string{"run", "github.com/danicat/selene/cmd/selene@latest"}, pkgList...)
	}

	out, runErr := CommandRunner.RunWithOutput(ctx, absDir, seleneCmd, seleneArgs...)

	output := filterNoise(out)

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

func parsePackages(pkgs string) []string {
	if strings.TrimSpace(pkgs) == "" {
		return []string{"./..."}
	}
	normalized := strings.ReplaceAll(pkgs, ",", " ")
	fields := strings.Fields(normalized)
	if len(fields) == 0 {
		return []string{"./..."}
	}
	return fields
}

func filterNoise(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "go: downloading ") ||
			strings.HasPrefix(trimmed, "exit status ") ||
			trimmed == "exit status" {
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
