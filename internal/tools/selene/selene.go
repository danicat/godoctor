// Package selene implements the mutation testing tool using selene.
package selene

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/danicat/godoctor/internal/config"
	"github.com/danicat/godoctor/internal/safeshell"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	toolName          = "selene"
	defaultPackages   = "./..."
	flagWorkers       = "-workers"
	flagDoubleWorkers = "--workers"
	flagDB            = "-db"
	flagDoubleDB      = "--db"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:  toolName,
		Title: "Mutation Test",
		Description: "Runs mutation testing using Selene. Introduces small code mutations " +
			"(flipped conditions, swapped operators) and checks if existing tests catch them, " +
			"objectively measuring test suite quality.",
	}, Handler)
}

// Params defines the input parameters.
type Params struct {
	Dir      string `json:"dir" jsonschema:"The absolute directory path to run mutation testing in. Required."`
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

// BuildArgs constructs the full argument list for invoking Selene, applying configured
// worker concurrency (GOMAXPROCS) and TestQuery SQLite DB integration.
func BuildArgs(workspaceDir string, pkgList []string, cfg *config.Config) []string {
	if cfg == nil {
		cfg = config.NewDefaultConfig()
	}

	args := append([]string(nil), cfg.Tools.Selene.Args...)

	// 1. Worker concurrency: default to configured workers or runtime.GOMAXPROCS(0)
	hasWorkers := false
	for _, a := range args {
		if a == flagWorkers || a == flagDoubleWorkers ||
			strings.HasPrefix(a, flagWorkers+"=") ||
			strings.HasPrefix(a, flagDoubleWorkers+"=") {
			hasWorkers = true
			break
		}
	}
	if !hasWorkers {
		workers := cfg.GetSeleneWorkers()
		args = append(args, flagWorkers, strconv.Itoa(workers))
	}

	// 2. TestQuery compatibility: pass --db <path> if enabled
	hasDB := false
	for _, a := range args {
		if a == flagDB || a == flagDoubleDB ||
			strings.HasPrefix(a, flagDB+"=") ||
			strings.HasPrefix(a, flagDoubleDB+"=") {
			hasDB = true
			break
		}
	}
	if !hasDB && cfg.IsSeleneTestQueryCompat() {
		dbPath := cfg.GetSeleneDBPath(workspaceDir)
		relDB, err := filepath.Rel(workspaceDir, dbPath)
		if err == nil && !strings.HasPrefix(relDB, "..") {
			args = append(args, flagDoubleDB, relDB)
		} else {
			args = append(args, flagDoubleDB, dbPath)
		}
	}

	// 3. Target packages
	args = append(args, pkgList...)
	return args
}

// Handler handles the selene tool execution.
func Handler(ctx context.Context, _ *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(args.Dir) == "" || !filepath.IsAbs(args.Dir) {
		return errorResult("dir is required and must be an absolute path"), nil, nil
	}

	absDir := filepath.Clean(args.Dir)
	cfg, err := config.LoadFromWorkspace(absDir)
	if err != nil || cfg == nil {
		cfg = config.NewDefaultConfig()
	}

	pkgs := args.Packages
	if strings.TrimSpace(pkgs) == "" {
		pkgs = defaultPackages
	}
	pkgList := parsePackages(pkgs)

	seleneCmd := cfg.Tools.Selene.Command
	if seleneCmd == "" {
		seleneCmd = toolName
	}
	if _, lookErr := CommandRunner.LookPath(seleneCmd); lookErr != nil && !filepath.IsAbs(seleneCmd) {
		msg := fmt.Sprintf(
			"selene binary (%q) not found in PATH; configure tools.selene.command or install selene",
			seleneCmd,
		)
		return errorResult(msg), nil, nil
	}

	seleneArgs := BuildArgs(absDir, pkgList, cfg)

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
		return []string{defaultPackages}
	}
	normalized := strings.ReplaceAll(pkgs, ",", " ")
	fields := strings.Fields(normalized)
	if len(fields) == 0 {
		return []string{defaultPackages}
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
