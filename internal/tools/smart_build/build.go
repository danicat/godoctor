// Package smartbuild implements the smart_build tool.
package smartbuild

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danicat/godoctor/internal/config"
	"github.com/danicat/godoctor/internal/safeshell"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	allPackagesPattern = "./..."
	cmdBuild           = "build"
	cmdRun             = "run"
	cmdDeadcode        = "deadcode"
	cmdModernize       = "modernize"
	cmdTestQuery       = "testquery"
	cmdGolangCILint    = "golangci-lint"
	flagPkg            = "--pkg"
	flagOutput         = "--output"
	tokenCoverageColon = "coverage:"
	defaultTestQueryDB = "testquery.db"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:  "smart_build",
		Title: "Smart Build",
		Description: "GoDoctor's specialized build pipeline: Tidy -> Modernize -> Format -> " +
			"Build -> Test -> Lint -> Deadcode. Runs `go mod tidy` -> modernization -> " +
			"`gofmt` -> `go build` -> `go test` -> linter -> deadcode to verify workspace health.",
	}, Handler)
}

// Params defines the input parameters.
type Params struct {
	Dir      string `json:"dir" jsonschema:"The absolute directory path to build in. Required."`
	Packages string `json:"packages,omitempty" jsonschema:"Packages to build (default: ./...)"`
	Output   string `json:"output,omitempty" jsonschema:"The build output binary target path (-o)."`
}

// Runner defines the interface for running commands.
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

// parsePackages splits comma- or whitespace-separated packages into a slice.
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

// Handler executes the smart_build tool.
func Handler(ctx context.Context, _ *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(args.Dir) == "" || !filepath.IsAbs(args.Dir) {
		return result("dir is required and must be an absolute path", true), nil, nil
	}
	workspaceDir := filepath.Clean(args.Dir)
	cfg, err := config.LoadFromWorkspace(workspaceDir)
	if err != nil || cfg == nil {
		cfg = config.NewDefaultConfig()
	}

	pkgs := args.Packages
	if strings.TrimSpace(pkgs) == "" {
		if cfg.Build.DefaultPackages != "" {
			pkgs = cfg.Build.DefaultPackages
		} else {
			pkgs = allPackagesPattern
		}
	}
	output := strings.TrimSpace(args.Output)
	if output == "" && cfg.Build.Output != "" {
		output = cfg.Build.Output
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Smart Build Report (`%s`)\n\n", pkgs)

	runAutoFix(ctx, workspaceDir, pkgs, cfg, &sb)

	var pipelineErr error
	if ctx.Err() != nil {
		pipelineErr = ctx.Err()
		sb.WriteString("\n⚠️ Canceled: context deadline exceeded or canceled\n")
	} else if errBuild := runBuild(ctx, workspaceDir, pkgs, output, &sb); errBuild != nil {
		pipelineErr = errBuild
	} else if ctx.Err() != nil {
		pipelineErr = ctx.Err()
		sb.WriteString("\n⚠️ Canceled: context deadline exceeded or canceled\n")
	} else if cfg.Build.RunTests {
		if errTests := runTestsPhase(ctx, workspaceDir, pkgs, cfg, &sb); errTests != nil {
			pipelineErr = errTests
		} else if ctx.Err() != nil {
			pipelineErr = ctx.Err()
			sb.WriteString("\n⚠️ Canceled: context deadline exceeded or canceled\n")
		}
	}

	if pipelineErr == nil {
		if errLint := runLinterPhase(ctx, workspaceDir, pkgs, cfg, &sb); errLint != nil {
			pipelineErr = errLint
		} else if ctx.Err() != nil {
			pipelineErr = ctx.Err()
			sb.WriteString("\n⚠️ Canceled: context deadline exceeded or canceled\n")
		}
	}

	if pipelineErr == nil {
		runDeadcodePhase(ctx, workspaceDir, pkgs, cfg, &sb)
	}

	return result(sb.String(), pipelineErr != nil), nil, nil
}

func runAutoFix(ctx context.Context, workspaceDir, pkgs string, cfg *config.Config, sb *strings.Builder) {
	if !cfg.Features.Autofix {
		return
	}
	sb.WriteString("### 🔧 Auto-Fix & Modernize:\n")

	if cfg.Build.AutoTidy && cfg.Autofix.ModTidy {
		runTidyStep(ctx, workspaceDir, sb)
	}
	if ctx.Err() != nil {
		return
	}

	if cfg.Build.AutoModernize && cfg.Autofix.Modernize {
		runModernizeStep(ctx, workspaceDir, pkgs, cfg, sb)
	}
	if ctx.Err() != nil {
		return
	}

	if cfg.Build.AutoFormat && cfg.Autofix.Gofmt {
		runFormatStep(ctx, workspaceDir, sb)
	}
	sb.WriteString("\n")
}

func runTidyStep(ctx context.Context, workspaceDir string, sb *strings.Builder) {
	if err := CommandRunner.Run(ctx, workspaceDir, "go", "mod", "tidy"); err != nil {
		fmt.Fprintf(sb, "  - ❌ Go Mod Tidy: FAILED (%v)\n", err)
	} else {
		sb.WriteString("  - ✅ Go Mod Tidy: SUCCESS\n")
	}
}

func runModernizeStep(ctx context.Context, workspaceDir, pkgs string, cfg *config.Config, sb *strings.Builder) {
	modCmd := cfg.Tools.Modernize.Command
	if modCmd == "" {
		modCmd = cmdModernize
	}

	if _, err := CommandRunner.LookPath(modCmd); err != nil && !filepath.IsAbs(modCmd) {
		fmt.Fprintf(sb, "  - ⏩ Go Modernizer: SKIPPED (%s binary not found in PATH)\n", modCmd)
		return
	}

	pkgList := parsePackages(pkgs)
	modArgs := make([]string, 0, len(cfg.Tools.Modernize.Args)+len(pkgList)+1)
	if len(cfg.Tools.Modernize.Args) > 0 {
		modArgs = append(modArgs, cfg.Tools.Modernize.Args...)
	} else {
		modArgs = append(modArgs, "-fix")
	}
	modArgs = append(modArgs, pkgList...)

	out, err := CommandRunner.RunWithOutput(ctx, workspaceDir, modCmd, modArgs...)
	if err != nil {
		var exitErr *exec.ExitError
		if (errors.As(err, &exitErr) && exitErr.ExitCode() == 3) || strings.Contains(err.Error(), "exit status 3") {
			sb.WriteString("  - ✅ Go Modernizer: SUCCESS (Issues found and auto-fixed)\n")
			if trimmed := strings.TrimSpace(out); trimmed != "" {
				sb.WriteString(formatOutput(trimmed))
			}
		} else {
			fmt.Fprintf(sb, "  - ❌ Go Modernizer: FAILED (%v)\n", err)
			if trimmed := strings.TrimSpace(out); trimmed != "" {
				sb.WriteString(formatOutput(trimmed))
			}
		}
	} else {
		sb.WriteString("  - ✅ Go Modernizer: SUCCESS (No issues found)\n")
	}
}

func runFormatStep(ctx context.Context, workspaceDir string, sb *strings.Builder) {
	if err := CommandRunner.Run(ctx, workspaceDir, "gofmt", "-w", "."); err != nil {
		fmt.Fprintf(sb, "  - ❌ Go Code Formatter: FAILED (%v)\n", err)
	} else {
		sb.WriteString("  - ✅ Go Code Formatter: SUCCESS\n")
	}
}

func runBuild(ctx context.Context, workspaceDir, pkgs, output string, sb *strings.Builder) error {
	sb.WriteString("### 🛠  Build: ")
	pkgList := parsePackages(pkgs)
	buildArgs := []string{cmdBuild}
	if output != "" {
		buildArgs = append(buildArgs, "-o", output)
	}
	buildArgs = append(buildArgs, pkgList...)
	buildOut, buildErr := CommandRunner.RunWithOutput(ctx, workspaceDir, "go", buildArgs...)
	if buildErr != nil {
		sb.WriteString("❌ FAILED\n\n")
		sb.WriteString(formatOutput(buildOut))
		sb.WriteString(getDocHintFromOutput(buildOut))
		return buildErr
	}
	sb.WriteString("✅ PASS\n\n")
	return nil
}

func runTestsPhase(ctx context.Context, workspaceDir, pkgs string, cfg *config.Config, sb *strings.Builder) error {
	sb.WriteString("### 🧪 Tests: ")

	// Create a temporary file for coverage in OS temp dir to avoid polluting workspace
	covTmp, err := os.CreateTemp("", "godoctor-coverage-*.out")
	if err != nil {
		fmt.Fprintf(sb, "❌ FAILED (cannot create temp coverage file: %v)\n\n", err)
		return err
	}
	covFile := covTmp.Name()
	_ = covTmp.Close()
	defer func() {
		_ = os.Remove(covFile)
	}()

	pkgList := parsePackages(pkgs)
	testArgs := make([]string, 0, 3+len(pkgList))
	testArgs = append(testArgs, "test", "-v", "-coverprofile="+covFile)
	testArgs = append(testArgs, pkgList...)
	testOut, testErr := CommandRunner.RunWithOutput(ctx, workspaceDir, "go", testArgs...)

	if testErr != nil {
		sb.WriteString("❌ FAILED\n\n")
		sb.WriteString(formatOutput(testOut))
		sb.WriteString("\n**HINT:** Run `test_query` (`tq`) to query failing tests via SQL: " +
			"`SELECT test, output FROM all_tests WHERE action='fail'`.\n\n")
		syncTestQueryDB(ctx, workspaceDir, pkgs, cfg, sb)
		return testErr
	}
	sb.WriteString("✅ PASS\n\n")

	// Process coverage
	sb.WriteString("#### 📊 Coverage\n")

	// 1. Get Total Coverage from go tool cover -func
	if totalCov := parseTotalCoverage(ctx, workspaceDir, covFile); totalCov != "" {
		fmt.Fprintf(sb, "✅ **Total Project Coverage**: %s\n", totalCov)
	}

	// 2. Parse per-package coverage from test output
	parsePackagesCoverage(testOut, sb)
	sb.WriteString("\n")

	// 3. Sync to testquery.db
	syncTestQueryDB(ctx, workspaceDir, pkgs, cfg, sb)
	return nil
}

func runDeadcodePhase(ctx context.Context, workspaceDir, pkgs string, cfg *config.Config, sb *strings.Builder) {
	if !cfg.Features.DeadcodeCheck || !cfg.Build.RunDeadcode {
		return
	}
	sb.WriteString("### 🔍 Deadcode Analysis: ")
	deadCmd := cfg.Tools.Deadcode.Command
	if deadCmd == "" {
		deadCmd = cmdDeadcode
	}
	if _, err := CommandRunner.LookPath(deadCmd); err != nil && !filepath.IsAbs(deadCmd) {
		fmt.Fprintf(sb, "⏩ SKIPPED (`%s` binary not found in PATH)\n\n", deadCmd)
		return
	}

	pkgList := parsePackages(pkgs)
	deadOut, deadErr := CommandRunner.RunWithOutput(ctx, workspaceDir, deadCmd, pkgList...)
	if deadErr != nil {
		fmt.Fprintf(sb, "❌ FAILED (%v)\n\n", deadErr)
		if trimmed := strings.TrimSpace(deadOut); trimmed != "" {
			sb.WriteString(formatOutput(trimmed))
		}
	} else {
		trimmed := strings.TrimSpace(deadOut)
		if trimmed == "" {
			sb.WriteString("✅ PASS (No unreachable code found)\n\n")
		} else {
			sb.WriteString("⚠️ Unreachable functions detected:\n")
			sb.WriteString(formatOutput(deadOut))
		}
	}
}

func syncTestQueryDB(ctx context.Context, workspaceDir, pkgs string, cfg *config.Config, sb *strings.Builder) {
	if !cfg.Features.TestQuerySync {
		return
	}
	tqCmd := cfg.Tools.TestQuery.Command
	if tqCmd == "" {
		tqCmd = cmdTestQuery
	}
	if _, err := CommandRunner.LookPath(tqCmd); err != nil && !filepath.IsAbs(tqCmd) {
		if _, errTQ := CommandRunner.LookPath("tq"); errTQ == nil {
			tqCmd = "tq"
		} else {
			sb.WriteString("*TestQuery sync skipped (`testquery` binary not found in PATH)*\n\n")
			return
		}
	}
	absDBPath := cfg.GetTestQueryDBPath(workspaceDir)
	relDBPath, err := filepath.Rel(workspaceDir, absDBPath)
	dbTarget := absDBPath
	if err == nil && !strings.HasPrefix(relDBPath, "..") {
		dbTarget = relDBPath
	}

	_ = os.MkdirAll(filepath.Dir(absDBPath), 0750)
	_ = os.Remove(absDBPath)
	_ = os.Remove(absDBPath + "-wal")
	_ = os.Remove(absDBPath + "-shm")

	tqArgs := []string{cmdBuild, flagPkg, pkgs, flagOutput, dbTarget}
	_, err = CommandRunner.RunWithOutput(ctx, workspaceDir, tqCmd, tqArgs...)
	if err != nil {
		fmt.Fprintf(sb, "*TestQuery sync warning: %v*\n\n", err)
	} else {
		sb.WriteString("*Indexed test run to `" + dbTarget + "`*\n\n")
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

func parsePackagesCoverage(testOut string, sb *strings.Builder) {
	lines := strings.Split(testOut, "\n")
	hasCoverage := false
	seenPkgs := make(map[string]bool)
	for _, line := range lines {
		if !strings.Contains(line, tokenCoverageColon) {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 4 || parts[0] != "ok" {
			continue
		}
		pkg := parts[1]
		if pkg == tokenCoverageColon || strings.HasPrefix(pkg, "coverage") || seenPkgs[pkg] {
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
		if !hasCoverage {
			sb.WriteString("- **Packages**:\n")
			hasCoverage = true
		}
		seenPkgs[pkg] = true
		fmt.Fprintf(sb, "  - `%s`: %s\n", pkg, covStr)
	}
}

func findCoverageIndex(parts []string) int {
	for i, part := range parts {
		if part == tokenCoverageColon {
			return i
		}
	}
	return -1
}

func findConfigFile(workspaceDir string) string {
	configFiles := []string{
		".golangci.yml",
		".golangci.yaml",
		".golangci.toml",
		".golangci.json",
	}
	curr := filepath.Clean(workspaceDir)
	for {
		for _, file := range configFiles {
			path := filepath.Join(curr, file)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
		if _, err := os.Stat(filepath.Join(curr, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return ""
}

func runLinterPhase(ctx context.Context, workspaceDir, pkgs string, cfg *config.Config, sb *strings.Builder) error {
	if !cfg.Build.RunLinter {
		return nil
	}
	sb.WriteString("### 🧹 Lint: ")

	lintCmd := cfg.Tools.GolangCILint.Command
	if lintCmd == "" {
		lintCmd = cmdGolangCILint
	}

	if _, err := CommandRunner.LookPath(lintCmd); err != nil && !filepath.IsAbs(lintCmd) {
		fmt.Fprintf(sb, "⏩ SKIPPED (`%s` binary not found in PATH)\n\n", lintCmd)
		return nil
	}

	configPath := cfg.Tools.GolangCILint.Config
	if configPath == "" {
		configPath = findConfigFile(workspaceDir)
	} else if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(workspaceDir, configPath)
	}

	if configPath != "" {
		if _, err := os.Stat(configPath); err != nil {
			configPath = findConfigFile(workspaceDir)
		}
	}

	pkgList := parsePackages(pkgs)
	lintArgs := make([]string, 0, 3+len(pkgList))
	lintArgs = append(lintArgs, cmdRun)
	if configPath != "" {
		lintArgs = append(lintArgs, "-c", configPath)
	}
	lintArgs = append(lintArgs, pkgList...)

	fmt.Fprintf(sb, "(using `%s`) ", lintCmd)
	lintOut, lintErr := CommandRunner.RunWithOutput(ctx, workspaceDir, lintCmd, lintArgs...)
	if lintErr != nil {
		sb.WriteString("⚠️ ISSUES FOUND\n\n")
		sb.WriteString(formatOutput(lintOut))
		return lintErr
	}
	sb.WriteString("✅ PASS\n\n")
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

var (
	undefinedPkgRe = regexp.MustCompile(`undefined:\s+([a-zA-Z0-9_]+)\.`)
	importErrorRe  = regexp.MustCompile(`(?:could not import|package)\s+([a-zA-Z0-9_./-]+)`)
)

func getDocHintFromOutput(msg string) string {
	if matches := undefinedPkgRe.FindStringSubmatch(msg); len(matches) > 1 {
		pkgName := matches[1]
		return fmt.Sprintf("\n\n**HINT:** usage of '%s' failed. "+
			"Try calling `read_docs` on that package to see the correct API.", pkgName)
	}
	if matches := importErrorRe.FindStringSubmatch(msg); len(matches) > 1 {
		pkgPath := matches[1]
		return fmt.Sprintf("\n\n**HINT:** import '%s' failed. "+
			"Try calling `read_docs` on \"%s\" to verify the package path and exports.", pkgPath, pkgPath)
	}
	return ""
}
