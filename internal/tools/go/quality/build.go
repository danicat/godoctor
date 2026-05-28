// Package quality implements the smart_build tool.
package quality

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/danicat/godoctor/internal/roots"
	"github.com/danicat/godoctor/internal/toolnames"
	"github.com/danicat/godoctor/internal/tools/shared"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	def := toolnames.Registry["smart_build"]
	mcp.AddTool(server, &mcp.Tool{
		Name:        def.Name,
		Title:       def.Title,
		Description: def.Description,
	}, Handler)
}

// Params defines the input parameters.
type Params struct {
	Dir      string `json:"dir,omitempty" jsonschema:"The absolute directory path to build in. Always pass absolute paths in multi-root workspaces."`
	Packages string `json:"packages,omitempty" jsonschema:"Packages to build (default: ./...)"`
}

// Runner defines the interface for running commands.
type Runner interface {
	Run(ctx context.Context, dir, name string, args ...string) error
	RunWithOutput(ctx context.Context, dir, name string, args ...string) (string, error)
	LookPath(file string) (string, error)
}

type stdRunner struct{}

func (r *stdRunner) Run(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

func (r *stdRunner) RunWithOutput(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (r *stdRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

var CommandRunner Runner = &stdRunner{}

func Handler(ctx context.Context, req *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	var session *mcp.ServerSession
	if req != nil {
		session = req.Session
	}
	dir := args.Dir
	if dir == "" {
		dir = "."
	}
	absDir, err := roots.Global.Validate(session, dir)
	if err != nil {
		return result(err.Error(), true), nil, nil
	}
	dir = absDir
	pkgs := args.Packages
	if pkgs == "" {
		pkgs = "./..."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Smart Build Report (`%s`)\n\n", pkgs)

	runAutoFix(ctx, dir, &sb)

	if err := runBuild(ctx, dir, pkgs, &sb); err != nil {
		//nolint:nilerr // Returning a JSON formatted tool error rather than an actual Go error
		return result(sb.String(), true), nil, nil
	}

	if err := runTestsPhase(ctx, dir, pkgs, &sb); err != nil {
		//nolint:nilerr // Returning a JSON formatted tool error rather than an actual Go error
		return result(sb.String(), true), nil, nil
	}

	if err := runLinterPhase(ctx, dir, pkgs, &sb); err != nil {
		//nolint:nilerr // Returning a JSON formatted tool error rather than an actual Go error
		return result(sb.String(), true), nil, nil
	}

	return result(sb.String(), false), nil, nil
}

func runAutoFix(ctx context.Context, dir string, sb *strings.Builder) {
	if err := CommandRunner.Run(ctx, dir, "go", "mod", "tidy"); err != nil {
		fmt.Fprintf(sb, "### ⚠️ Auto-Fix: `go mod tidy` Failed\n> %v\n\n", err)
	}

	// Run Modernize directly from the CLI tool
	runAnalyzer := func(cmd string) {
		out, err := CommandRunner.RunWithOutput(ctx, dir, "go", "run", cmd, "-fix", "./...")
		// These analyzers return exit code 3 if they found an issue and fixed it.
		// Exit code 1 means a genuine failure (e.g. compile error).
		if err != nil {
			// We don't want to fail the whole build for a lint fix error, just warn the user.
			if !strings.Contains(err.Error(), "exit status 3") {
				fmt.Fprintf(sb, "  - ⚠️ Modernize `%s` Warning: %v\n    %s\n", cmd, err, strings.TrimSpace(out))
			}
		}
	}

	runAnalyzer("golang.org/x/tools/go/analysis/passes/defers/cmd/defers@latest")
	runAnalyzer("golang.org/x/tools/go/analysis/passes/errorsas/cmd/errorsas@latest")
	runAnalyzer("golang.org/x/tools/go/analysis/passes/sortslice/cmd/sortslice@latest")
	runAnalyzer("golang.org/x/tools/go/analysis/passes/timeformat/cmd/timeformat@latest")

	if err := CommandRunner.Run(ctx, dir, "gofmt", "-w", "."); err != nil {
		// gofmt might fail if syntax is very broken, which build will catch
	}
}

func runBuild(ctx context.Context, dir, pkgs string, sb *strings.Builder) error {
	sb.WriteString("### 🛠️ Build: ")
	buildOut, buildErr := CommandRunner.RunWithOutput(ctx, dir, "go", "build", pkgs)
	if buildErr != nil {
		sb.WriteString("❌ FAILED\n\n")
		sb.WriteString(formatOutput(buildOut))
		sb.WriteString(shared.GetDocHintFromOutput(buildOut))
		return buildErr
	}
	sb.WriteString("✅ PASS\n\n")
	return nil
}

func runTestsPhase(ctx context.Context, dir, pkgs string, sb *strings.Builder) error {
	sb.WriteString("### 🧪 Tests: ")

	// Create a temporary file for coverage
	covFile := "coverage.out"
	defer func() {
		_ = os.Remove(covFile)
	}()

	// -v for verbose, -coverprofile for coverage
	testArgs := []string{"test", "-v", "-coverprofile=" + covFile, pkgs}
	testOut, testErr := CommandRunner.RunWithOutput(ctx, dir, "go", testArgs...)

	if testErr != nil {
		sb.WriteString("❌ FAILED\n\n")
		sb.WriteString(formatOutput(testOut))
		return testErr
	}
	sb.WriteString("✅ PASS\n\n")

	// Process coverage
	sb.WriteString("#### Coverage\n")

	// 1. Get Total Coverage from go tool cover -func
	funcOut, funcErr := CommandRunner.RunWithOutput(ctx, dir, "go", "tool", "cover", "-func="+covFile)
	if funcErr == nil {
		lines := strings.Split(strings.TrimSpace(funcOut), "\n")
		if len(lines) > 0 {
			lastLine := lines[len(lines)-1]
			if strings.HasPrefix(lastLine, "total:") {
				// Format: "total: (statements) 80.0%"
				parts := strings.Fields(lastLine)
				if len(parts) >= 3 {
					fmt.Fprintf(sb, "- **Total Project Coverage**: %s\n", parts[len(parts)-1])
				}
			}
		}
	}

	// 2. Parse per-package coverage from test output
	lines := strings.Split(testOut, "\n")
	hasCoverage := false
	for _, line := range lines {
		if strings.Contains(line, "\tcoverage: ") {
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				pkg := parts[1]
				covStr := parts[4] // "50.0%"
				if covStr != "0.0%" && covStr != "[no" {
					if !hasCoverage {
						sb.WriteString("- **Packages**:\n")
						hasCoverage = true
					}
					fmt.Fprintf(sb, "  - `%s`: %s\n", pkg, covStr)
				}
			}
		}
	}
	sb.WriteString("\n")
	return nil
}

func runLinterPhase(ctx context.Context, dir, pkgs string, sb *strings.Builder) error {
	sb.WriteString("### 🧹 Lint: ")

	lintCmd := "golangci-lint"
	lintArgs := []string{"run", pkgs}

	if _, err := CommandRunner.LookPath("golangci-lint"); err != nil {
		lintCmd = "go"
		lintArgs = []string{"vet", pkgs}
		sb.WriteString("(using `go vet`) ")
	}

	lintOut, lintErr := CommandRunner.RunWithOutput(ctx, dir, lintCmd, lintArgs...)
	if lintErr != nil {
		sb.WriteString("⚠️ ISSUES FOUND\n\n")
		sb.WriteString(formatOutput(lintOut))
		return lintErr
	}
	sb.WriteString("✅ PASS\n")
	return nil
}

func formatOutput(out string) string {
	if out == "" {
		return ""
	}
	return "```text\n" + strings.TrimSpace(out) + "\n```\n"
}

func result(content string, isError bool) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: isError,
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}
}
