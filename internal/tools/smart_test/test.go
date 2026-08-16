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
	Dir string `json:"dir" jsonschema:"The absolute directory path to test in. Required. Relative paths are rejected."`
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

const (
	allPackagesPattern = "./..."
	levelFast          = "fast"
	levelBasic         = "basic"
	levelBenchmark     = "benchmark"
	levelComplete      = "complete"
	seleneTool         = "selene"
	testqueryTool      = "testquery"
	tqTool             = "tq"
	flagPkg            = "--pkg"
	flagOutput         = "--output"
	cmdBuild           = "build"
	dbFile             = "testquery.db"
)

// CommandRunner is used for executing CLI commands during testing or runtime.
var CommandRunner Runner = &stdRunner{}

// Handler executes the smart_test tool.
func Handler(ctx context.Context, _ *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(args.Dir) == "" || !filepath.IsAbs(args.Dir) {
		return result("dir is required and must be an absolute path", true), nil, nil
	}

	workspaceDir := filepath.Clean(args.Dir)

	pkgs := args.Packages
	if pkgs == "" {
		pkgs = allPackagesPattern
	}

	level := strings.ToLower(strings.TrimSpace(args.Level))
	if level == "" {
		level = levelBasic
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Smart Test Report (`%s`)\n\n", pkgs)

	var err error
	switch level {
	case levelFast:
		err = runFastLevel(ctx, workspaceDir, pkgs, args.Run, &sb)
	case levelBenchmark:
		err = runBenchmarkLevel(ctx, workspaceDir, pkgs, args.Run, &sb)
	case levelComplete:
		err = runBasicLevel(ctx, workspaceDir, pkgs, args.Run, &sb)
		if err == nil {
			runMutationLevel(ctx, workspaceDir, pkgs, &sb)
		}
	case levelBasic:
		fallthrough
	default:
		err = runBasicLevel(ctx, workspaceDir, pkgs, args.Run, &sb)
	}

	isError := err != nil
	return result(sb.String(), isError), nil, nil
}

func parsePackages(pkgs string) []string {
	if strings.TrimSpace(pkgs) == "" {
		return []string{allPackagesPattern}
	}
	normalized := strings.ReplaceAll(pkgs, ",", " ")
	fields := strings.Fields(normalized)
	if len(fields) == 0 {
		return []string{allPackagesPattern}
	}
	return fields
}

func runFastLevel(ctx context.Context, workspaceDir, pkgs, runFilter string, sb *strings.Builder) error {
	sb.WriteString("### ⚡ Fast Test Pass\n\n")

	pkgList := parsePackages(pkgs)
	testArgs := make([]string, 0, 3+len(pkgList))
	testArgs = append(testArgs, "test", "-v")
	if runFilter != "" {
		testArgs = append(testArgs, "-run="+runFilter)
	}
	testArgs = append(testArgs, pkgList...)

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

	tmpFile, err := os.CreateTemp("", "godoctor-coverage-*.out")
	var covFile string
	if err == nil {
		covFile = tmpFile.Name()
		_ = tmpFile.Close()
		defer func() {
			_ = os.Remove(covFile)
		}()
	} else {
		covFile = filepath.Join(workspaceDir, "coverage.out")
		defer func() {
			_ = os.Remove(covFile)
		}()
	}

	pkgList := parsePackages(pkgs)
	testArgs := make([]string, 0, 4+len(pkgList))
	testArgs = append(testArgs, "test", "-v", "-coverprofile="+covFile)
	if runFilter != "" {
		testArgs = append(testArgs, "-run="+runFilter)
	}
	testArgs = append(testArgs, pkgList...)

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

	pkgList := parsePackages(pkgs)
	benchArgs := make([]string, 0, 4+len(pkgList))
	benchArgs = append(benchArgs, "test", "-bench="+benchPattern, "-benchmem", "-run=NONE")
	benchArgs = append(benchArgs, pkgList...)
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
	if _, err := CommandRunner.LookPath(seleneTool); err == nil {
		seleneCmd = seleneTool
		seleneArgs = parsePackages(pkgs)
	} else {
		seleneCmd = "go"
		seleneArgs = append([]string{"run", "github.com/danicat/selene/cmd/selene@latest"}, parsePackages(pkgs)...)
	}

	out, err := CommandRunner.RunWithOutput(ctx, workspaceDir, seleneCmd, seleneArgs...)
	filtered := filterNoise(out)

	switch {
	case err != nil:
		sb.WriteString("⚠️ **Surviving Mutants Detected**:\n\n")
		sb.WriteString(formatOutput(filtered))
	case filtered == "":
		sb.WriteString("✅ **All Mutations Caught by Test Suite**\n\n")
	default:
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
		if covStr == "[no" {
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

	if _, err := CommandRunner.LookPath(testqueryTool); err == nil {
		tqCmd = testqueryTool
		tqArgs = []string{cmdBuild, flagPkg, pkgs, flagOutput, dbFile}
	} else if _, err := CommandRunner.LookPath(tqTool); err == nil {
		tqCmd = tqTool
		tqArgs = []string{cmdBuild, flagPkg, pkgs, flagOutput, dbFile}
	} else {
		tqCmd = "go"
		tqArgs = []string{"run", "github.com/danicat/testquery@latest", cmdBuild, flagPkg, pkgs, flagOutput, dbFile}
	}

	_, _ = CommandRunner.RunWithOutput(ctx, workspaceDir, tqCmd, tqArgs...)
}

func formatFailures(out string) string {
	lines := strings.Split(out, "\n")
	var failures []string
	inFailBlock := false

	for _, line := range lines {
		if strings.Contains(line, "--- FAIL:") ||
			strings.HasPrefix(line, "FAIL\t") ||
			strings.HasPrefix(line, "FAIL ") ||
			line == "FAIL" {
			inFailBlock = true
		} else if inFailBlock && isTestBoundary(line) {
			inFailBlock = false
		}

		if inFailBlock {
			failures = append(failures, line)
		}
	}

	if len(failures) > 0 {
		return "```text\n" + strings.TrimSpace(strings.Join(failures, "\n")) + "\n```\n"
	}
	return formatOutput(out)
}

func isTestBoundary(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(line, "=== RUN") ||
		strings.HasPrefix(line, "--- PASS:") ||
		strings.HasPrefix(line, "--- SKIP:") ||
		strings.HasPrefix(trimmed, "ok\t") ||
		strings.HasPrefix(trimmed, "ok ") ||
		trimmed == "PASS" {
		return true
	}
	return false
}

func formatTestSummary(out string) string {
	lines := strings.Split(out, "\n")
	var summary []string
	for _, line := range lines {
		if strings.HasPrefix(line, "ok ") ||
			strings.HasPrefix(line, "ok\t") ||
			strings.HasPrefix(line, "FAIL") ||
			strings.HasPrefix(line, "PASS") {
			summary = append(summary, line)
		}
	}
	if len(summary) > 0 {
		return "- " + strings.Join(summary, "\n- ") + "\n\n"
	}
	return ""
}

var benchLineRe = regexp.MustCompile(
	`^(Benchmark[a-zA-Z0-9_/-]+)\s+(\d+)\s+([0-9.]+\s+ns/op)(?:\s+([0-9.]+\s+B/op))?(?:\s+([0-9.]+\s+allocs/op))?`,
)

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
