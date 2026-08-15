// Package smarttest implements the smart_test tool.
package smarttest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danicat/godoctor/internal/safeshell"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the tool with the MCP server.
func Register(server *mcp.Server) {
	//nolint:lll
	mcp.AddTool(server, &mcp.Tool{
		Name:        "smart_test",
		Title:       "Smart Test",
		Description: "GoDoctor's specialized test runner. Executes Go tests across packages or specific functions, delivering structured failure diagnostics, coverage gap analysis, benchmark metrics, and automated test history tracking.",
	}, Handler)
}

// Params defines the input parameters for smart_test.
type Params struct {
	//nolint:lll
	Dir string `json:"dir,omitempty" jsonschema:"The absolute directory path to test in. Always pass absolute paths in multi-root workspaces."`
	//nolint:lll
	Packages string `json:"packages,omitempty" jsonschema:"Packages to test (default: ./...)"`
	//nolint:lll
	Level string `json:"level,omitempty" jsonschema:"Testing depth/mode: 'fast' (unit tests only), 'basic' (tests + coverage + testquery.db sync, default), 'benchmark' (benchmarks with sensible defaults), 'complete' (tests + coverage + Selene mutation testing)."`
	//nolint:lll
	Run string `json:"run,omitempty" jsonschema:"Regex pattern to filter specific tests or benchmark functions (maps to -run or -bench)."`
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

// CommandRunner is used for executing CLI commands during testing or runtime.
var CommandRunner Runner = &stdRunner{}

// Handler executes the smart_test tool.
func Handler(ctx context.Context, _ *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	projectDir := args.Dir
	if projectDir == "" {
		projectDir = "."
	}

	workspaceDir, err := filepath.Abs(projectDir)
	if err != nil {
		return result(err.Error(), true), nil, nil
	}

	pkgs := args.Packages
	if pkgs == "" {
		pkgs = "./..."
	}

	level := strings.ToLower(strings.TrimSpace(args.Level))
	if level == "" {
		level = "basic"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Smart Test Report (`%s`)\n\n", pkgs)

	switch level {
	case "fast":
		err = runFastLevel(ctx, workspaceDir, pkgs, args.Run, &sb)
	case "benchmark":
		err = runBenchmarkLevel(ctx, workspaceDir, pkgs, args.Run, &sb)
	case "complete":
		err = runBasicLevel(ctx, workspaceDir, pkgs, args.Run, &sb)
		if err == nil {
			runMutationLevel(ctx, workspaceDir, pkgs, &sb)
		}
	case "basic":
		fallthrough
	default:
		err = runBasicLevel(ctx, workspaceDir, pkgs, args.Run, &sb)
	}

	isError := err != nil
	return result(sb.String(), isError), nil, nil
}

func runFastLevel(ctx context.Context, workspaceDir, pkgs, runFilter string, sb *strings.Builder) error {
	sb.WriteString("### ⚡ Fast Test Pass\n\n")

	testArgs := []string{"test", "-v"}
	if runFilter != "" {
		testArgs = append(testArgs, "-run="+runFilter)
	}
	testArgs = append(testArgs, pkgs)

	testOut, testErr := CommandRunner.RunWithOutput(ctx, workspaceDir, "go", testArgs...)
	if testErr != nil {
		sb.WriteString("❌ **Tests Failed**\n\n")
		sb.WriteString(formatFailures(testOut))
		return testErr
	}

	sb.WriteString("✅ **All Tests Passed**\n\n")
	sb.WriteString(formatTestSummary(testOut))
	return nil
}

func runBasicLevel(ctx context.Context, workspaceDir, pkgs, runFilter string, sb *strings.Builder) error {
	sb.WriteString("### 🧪 Test & Coverage Analysis\n\n")

	covFile := filepath.Join(workspaceDir, "coverage.out")
	defer func() {
		_ = os.Remove(covFile)
	}()

	testArgs := []string{"test", "-v", "-coverprofile=" + covFile}
	if runFilter != "" {
		testArgs = append(testArgs, "-run="+runFilter)
	}
	testArgs = append(testArgs, pkgs)

	testOut, testErr := CommandRunner.RunWithOutput(ctx, workspaceDir, "go", testArgs...)

	if testErr != nil {
		sb.WriteString("❌ **Tests Failed**\n\n")
		sb.WriteString(formatFailures(testOut))
		syncTestQueryDB(ctx, workspaceDir, pkgs)
		return testErr
	}

	sb.WriteString("✅ **All Tests Passed**\n\n")
	sb.WriteString(formatTestSummary(testOut))

	// Coverage processing
	sb.WriteString("#### 📊 Coverage Summary\n")
	if totalCov := parseTotalCoverage(ctx, workspaceDir, covFile); totalCov != "" {
		fmt.Fprintf(sb, "* **Total Coverage**: `%s`\n", totalCov)
	}
	parsePackageCoverage(testOut, sb)

	// Sync testquery.db in background/sub-step
	syncTestQueryDB(ctx, workspaceDir, pkgs)
	sb.WriteString("\n*Indexed test run to `testquery.db`*\n\n")

	return nil
}

func runBenchmarkLevel(ctx context.Context, workspaceDir, pkgs, runFilter string, sb *strings.Builder) error {
	sb.WriteString("### 🚀 Benchmark Results\n\n")

	benchPattern := runFilter
	if benchPattern == "" {
		benchPattern = "."
	}

	benchArgs := []string{"test", "-bench=" + benchPattern, "-benchmem", "-run=NONE", pkgs}
	benchOut, benchErr := CommandRunner.RunWithOutput(ctx, workspaceDir, "go", benchArgs...)

	if benchErr != nil {
		sb.WriteString("❌ **Benchmark Execution Failed**\n\n")
		sb.WriteString(formatOutput(benchOut))
		return benchErr
	}

	table := formatBenchmarkTable(benchOut)
	if table != "" {
		sb.WriteString(table)
	} else {
		sb.WriteString(formatOutput(benchOut))
	}

	return nil
}

func runMutationLevel(ctx context.Context, workspaceDir, pkgs string, sb *strings.Builder) {
	sb.WriteString("### 🧬 Selene AST Mutation Testing\n\n")

	var seleneCmd string
	var seleneArgs []string
	if _, err := CommandRunner.LookPath("selene"); err == nil {
		seleneCmd = "selene"
		seleneArgs = []string{pkgs}
	} else {
		seleneCmd = "go"
		seleneArgs = []string{"run", "github.com/danicat/selene/cmd/selene@latest", pkgs}
	}

	out, err := CommandRunner.RunWithOutput(ctx, workspaceDir, seleneCmd, seleneArgs...)
	filtered := filterNoise(out)

	if err != nil {
		sb.WriteString("⚠️ **Surviving Mutants Detected**:\n\n")
		sb.WriteString(formatOutput(filtered))
	} else if filtered == "" {
		sb.WriteString("✅ **All Mutations Caught by Test Suite**\n\n")
	} else {
		sb.WriteString(formatOutput(filtered))
	}
}

func parseTotalCoverage(ctx context.Context, workspaceDir, covFile string) string {
	funcOut, funcErr := CommandRunner.RunWithOutput(ctx, workspaceDir, "go", "tool", "cover", "-func="+covFile)
	if funcErr != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(funcOut), "\n")
	if len(lines) == 0 {
		return ""
	}
	lastLine := lines[len(lines)-1]
	if !strings.HasPrefix(lastLine, "total:") {
		return ""
	}
	parts := strings.Fields(lastLine)
	if len(parts) < 3 {
		return ""
	}
	return parts[len(parts)-1]
}

func parsePackageCoverage(testOut string, sb *strings.Builder) {
	lines := strings.Split(testOut, "\n")
	seenPkgs := make(map[string]bool)
	hasPkg := false

	for _, line := range lines {
		if !strings.Contains(line, "coverage:") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 4 || parts[0] != "ok" {
			continue
		}
		pkg := parts[1]
		if seenPkgs[pkg] {
			continue
		}
		covIdx := findCoverageIndex(parts)
		if covIdx == -1 || covIdx+1 >= len(parts) {
			continue
		}
		covStr := parts[covIdx+1]
		if covStr == "0.0%" || covStr == "[no" {
			continue
		}
		if !hasPkg {
			sb.WriteString("* **Package Details**:\n")
			hasPkg = true
		}
		seenPkgs[pkg] = true
		fmt.Fprintf(sb, "  * `%s`: `%s`\n", pkg, covStr)
	}
}

func findCoverageIndex(parts []string) int {
	for i, part := range parts {
		if part == "coverage:" {
			return i
		}
	}
	return -1
}

func syncTestQueryDB(ctx context.Context, workspaceDir, pkgs string) {
	var tqCmd string
	var tqArgs []string

	if _, err := CommandRunner.LookPath("testquery"); err == nil {
		tqCmd = "testquery"
		tqArgs = []string{"build", "--pkg", pkgs, "--output", "testquery.db"}
	} else if _, err := CommandRunner.LookPath("tq"); err == nil {
		tqCmd = "tq"
		tqArgs = []string{"build", "--pkg", pkgs, "--output", "testquery.db"}
	} else {
		tqCmd = "go"
		tqArgs = []string{"run", "github.com/danicat/testquery@latest", "build", "--pkg", pkgs, "--output", "testquery.db"}
	}

	_, _ = CommandRunner.RunWithOutput(ctx, workspaceDir, tqCmd, tqArgs...)
}

func formatFailures(out string) string {
	lines := strings.Split(out, "\n")
	var failures []string
	inFailBlock := false

	for _, line := range lines {
		if strings.Contains(line, "--- FAIL:") || strings.Contains(line, "FAIL\t") {
			inFailBlock = true
		}
		if inFailBlock {
			failures = append(failures, line)
			if strings.TrimSpace(line) == "" {
				inFailBlock = false
			}
		}
	}

	if len(failures) > 0 {
		return "```text\n" + strings.TrimSpace(strings.Join(failures, "\n")) + "\n```\n"
	}
	return formatOutput(out)
}

func formatTestSummary(out string) string {
	lines := strings.Split(out, "\n")
	var summary []string
	for _, line := range lines {
		if strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "FAIL ") || strings.HasPrefix(line, "PASS") {
			summary = append(summary, line)
		}
	}
	if len(summary) > 0 {
		return "- " + strings.Join(summary, "\n- ") + "\n\n"
	}
	return ""
}

var benchLineRe = regexp.MustCompile(`^(Benchmark[a-zA-Z0-9_/-]+)\s+(\d+)\s+([0-9.]+\s+ns/op)(?:\s+([0-9.]+\s+B/op))?(?:\s+([0-9.]+\s+allocs/op))?`)

func formatBenchmarkTable(out string) string {
	lines := strings.Split(out, "\n")
	var rows [][]string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		matches := benchLineRe.FindStringSubmatch(trimmed)
		if len(matches) > 0 {
			name := matches[1]
			iters := matches[2]
			nsOp := matches[3]
			bOp := matches[4]
			if bOp == "" {
				bOp = "N/A"
			}
			allocsOp := matches[5]
			if allocsOp == "" {
				allocsOp = "N/A"
			}
			rows = append(rows, []string{name, iters, nsOp, bOp, allocsOp})
		}
	}

	if len(rows) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("| Benchmark | Iterations | Time / Op | Memory / Op | Allocs / Op |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")

	for _, row := range rows {
		fmt.Fprintf(&sb, "| `%s` | %s | `%s` | `%s` | `%s` |\n", row[0], row[1], row[2], row[3], row[4])
	}
	sb.WriteString("\n")
	return sb.String()
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
