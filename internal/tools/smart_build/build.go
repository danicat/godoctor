// Package smartbuild implements the smart_build tool.
package smartbuild

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danicat/godoctor/internal/roots"
	"github.com/danicat/godoctor/internal/safeshell"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register registers the tool with the server.
func Register(server *mcp.Server) {
	//nolint:lll
	mcp.AddTool(server, &mcp.Tool{
		Name:        "smart_build",
		Title:       "Smart Build",
		Description: "GoDoctor's specialized build pipeline: Tidy -> Modernize -> Format -> Build -> Test -> Lint -> Deadcode. Runs `go mod tidy` -> modernization -> `gofmt` -> `go build` -> `go test` -> linter -> deadcode to verify workspace health.",
	}, Handler)
}

// Params defines the input parameters.
type Params struct {
	//nolint:lll
	Dir      string `json:"dir,omitempty" jsonschema:"The absolute directory path to build in. Always pass absolute paths in multi-root workspaces."`
	Packages string `json:"packages,omitempty" jsonschema:"Packages to build (default: ./...)"`
}

// Runner defines the interface for running commands.
type Runner interface {
	Run(ctx context.Context, dir, name string, args ...string) error
	RunWithOutput(ctx context.Context, dir, name string, args ...string) (string, error)
	LookPath(file string) (string, error)
}

// stdRunner implements standard command running.
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

// Handler executes the smart_build tool.
func Handler(ctx context.Context, req *mcp.CallToolRequest, args Params) (*mcp.CallToolResult, any, error) {
	var session *mcp.ServerSession
	if req != nil {
		session = req.Session
	}
	projectDir := args.Dir
	if projectDir == "" {
		projectDir = "."
	}
	workspaceDir, err := roots.Global.Validate(session, projectDir)
	if err != nil {
		return result(err.Error(), true), nil, nil
	}
	pkgs := args.Packages
	if pkgs == "" {
		pkgs = "./..."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Smart Build Report (`%s`)\n\n", pkgs)

	runAutoFix(ctx, workspaceDir, pkgs, &sb)

	if err := runBuild(ctx, workspaceDir, pkgs, &sb); err != nil {
		//nolint:nilerr // Returning a JSON formatted tool error rather than an actual Go error
		return result(sb.String(), true), nil, nil
	}

	if err := runTestsPhase(ctx, workspaceDir, pkgs, &sb); err != nil {
		//nolint:nilerr // Returning a JSON formatted tool error rather than an actual Go error
		return result(sb.String(), true), nil, nil
	}

	if err := runLinterPhase(ctx, workspaceDir, pkgs, &sb); err != nil {
		//nolint:nilerr // Returning a JSON formatted tool error rather than an actual Go error
		return result(sb.String(), true), nil, nil
	}

	return result(sb.String(), false), nil, nil
}

func runAutoFix(ctx context.Context, workspaceDir, pkgs string, sb *strings.Builder) {
	sb.WriteString("### 🔧 Auto-Fix & Modernize:\n")

	if err := CommandRunner.Run(ctx, workspaceDir, "go", "mod", "tidy"); err != nil {
		fmt.Fprintf(sb, "  - ❌ Go Mod Tidy: FAILED (%v)\n", err)
	} else {
		sb.WriteString("  - ✅ Go Mod Tidy: SUCCESS\n")
	}

	const cmdRun = "run"
	const cmdDeadcode = "deadcode"

	var modCmd string
	var modArgs []string
	if _, err := CommandRunner.LookPath("modernize"); err == nil {
		modCmd = "modernize"
		modArgs = []string{"-fix", pkgs}
	} else {
		modCmd = "go"
		modArgs = []string{cmdRun, "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest", "-fix", pkgs}
	}

	out, err := CommandRunner.RunWithOutput(ctx, workspaceDir, modCmd, modArgs...)
	if err != nil {
		if strings.Contains(err.Error(), "exit status 3") {
			sb.WriteString("  - ✅ Go Modernizer: SUCCESS (Issues found and auto-fixed)\n")
		} else {
			fmt.Fprintf(sb, "  - ❌ Go Modernizer: FAILED (%v)\n    %s\n", err, strings.TrimSpace(out))
		}
	} else {
		sb.WriteString("  - ✅ Go Modernizer: SUCCESS (No issues found)\n")
	}

	if err := CommandRunner.Run(ctx, workspaceDir, "gofmt", "-w", "."); err != nil {
		fmt.Fprintf(sb, "  - ❌ Go Code Formatter: FAILED (%v)\n", err)
	} else {
		sb.WriteString("  - ✅ Go Code Formatter: SUCCESS\n")
	}

	var deadCmd string
	var deadcodeArgs []string
	if _, err := CommandRunner.LookPath(cmdDeadcode); err == nil {
		deadCmd = cmdDeadcode
		deadcodeArgs = strings.Fields(pkgs)
	} else {
		deadCmd = "go"
		deadcodeArgs = append([]string{cmdRun, "golang.org/x/tools/cmd/deadcode@latest"}, strings.Fields(pkgs)...)
	}
	deadOut, deadErr := CommandRunner.RunWithOutput(ctx, workspaceDir, deadCmd, deadcodeArgs...)
	if deadErr != nil {
		fmt.Fprintf(sb, "  - ❌ Deadcode Analysis: FAILED (%v)\n", deadErr)
	} else {
		trimmed := strings.TrimSpace(deadOut)
		if trimmed == "" {
			sb.WriteString("  - ✅ Deadcode Analysis: SUCCESS (No unreachable code found)\n")
		} else {
			fmt.Fprintf(sb, "  - ⚠️ Deadcode Analysis: Unreachable functions detected:\n%s\n", formatOutput(deadOut))
		}
	}
	sb.WriteString("\n")
}

func runBuild(ctx context.Context, workspaceDir, pkgs string, sb *strings.Builder) error {
	sb.WriteString("### 🛠  Build: ")
	buildOut, buildErr := CommandRunner.RunWithOutput(ctx, workspaceDir, "go", "build", pkgs)
	if buildErr != nil {
		sb.WriteString("❌ FAILED\n\n")
		sb.WriteString(formatOutput(buildOut))
		sb.WriteString(getDocHintFromOutput(buildOut))
		return buildErr
	}
	sb.WriteString("✅ PASS\n\n")
	return nil
}

func runTestsPhase(ctx context.Context, workspaceDir, pkgs string, sb *strings.Builder) error {
	sb.WriteString("### 🧪 Tests: ")

	// Create a temporary file for coverage
	covFile := "coverage.out"
	defer func() {
		_ = os.Remove(covFile)
	}()

	// -v for verbose, -coverprofile for coverage
	testArgs := []string{"test", "-v", "-coverprofile=" + covFile, pkgs}
	testOut, testErr := CommandRunner.RunWithOutput(ctx, workspaceDir, "go", testArgs...)

	if testErr != nil {
		sb.WriteString("❌ FAILED\n\n")
		sb.WriteString(formatOutput(testOut))
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
	return nil
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
		if !strings.Contains(line, "coverage:") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 4 || parts[0] != "ok" {
			continue
		}
		pkg := parts[1]
		if pkg == "coverage:" || strings.HasPrefix(pkg, "coverage") || seenPkgs[pkg] {
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
		if part == "coverage:" {
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
	for _, file := range configFiles {
		path := filepath.Join(workspaceDir, file)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func parseConfigVersion(configPath string) string {
	versionStr := "2" // Default to v2 if not found or parse fails
	//nolint:gosec
	content, err := os.ReadFile(configPath)
	if err != nil {
		return versionStr
	}
	lines := strings.SplitSeq(string(content), "\n")
	for line := range lines {
		trimmed := strings.TrimSpace(line)
		isVersionKey := strings.HasPrefix(trimmed, "version:") ||
			strings.HasPrefix(trimmed, "version=") ||
			strings.Contains(trimmed, "\"version\"")
		if isVersionKey {
			var digits strings.Builder
			for _, char := range trimmed {
				if char >= '0' && char <= '9' {
					digits.WriteRune(char)
				}
			}
			if digits.Len() > 0 {
				versionStr = digits.String()
				break
			}
		}
	}
	return versionStr
}

func runLinterPhase(ctx context.Context, workspaceDir, pkgs string, sb *strings.Builder) error {
	sb.WriteString("### 🧹 Lint: ")

	configPath := findConfigFile(workspaceDir)

	// If no .golangci file is found, fallback to go vet
	if configPath == "" {
		lintCmd := "go"
		lintArgs := []string{"vet", pkgs}
		sb.WriteString("(using `go vet`) ")

		lintOut, lintErr := CommandRunner.RunWithOutput(ctx, workspaceDir, lintCmd, lintArgs...)
		if lintErr != nil {
			sb.WriteString("⚠️ ISSUES FOUND\n\n")
			sb.WriteString(formatOutput(lintOut))
			return lintErr
		}
		sb.WriteString("✅ PASS\n")
		return nil
	}

	var lintCmd string
	var lintArgs []string

	if _, err := CommandRunner.LookPath("golangci-lint"); err == nil {
		lintCmd = "golangci-lint"
		lintArgs = []string{"run", "-c", configPath, pkgs}
		sb.WriteString("(using local `golangci-lint`) ")
	} else {
		versionStr := parseConfigVersion(configPath)

		// Select the appropriate major version of golangci-lint based on parsed version
		var linterPkg string
		if strings.HasPrefix(versionStr, "1") {
			linterPkg = "github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.5"
			sb.WriteString("(using `golangci-lint v1`) ")
		} else {
			linterPkg = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2"
			if versionStr != "2" {
				fmt.Fprintf(sb, "(using `golangci-lint v%s`) ", versionStr)
			}
		}

		lintCmd = "go"
		const cmdRun = "run"
		lintArgs = []string{cmdRun, linterPkg, cmdRun, "-c", configPath, pkgs}
	}

	lintOut, lintErr := CommandRunner.RunWithOutput(ctx, workspaceDir, lintCmd, lintArgs...)
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

var (
	undefinedPkgRe = regexp.MustCompile(`undefined:\s+([a-zA-Z0-9_]+)\.`)
	importErrorRe  = regexp.MustCompile(`(?:could not import|package)\s+([a-zA-Z0-9_./-]+)`)
)

func getDocHintFromOutput(msg string) string {
	if matches := undefinedPkgRe.FindStringSubmatch(msg); len(matches) > 1 {
		pkgName := matches[1]
		//nolint:lll
		return fmt.Sprintf("\n\n**HINT:** usage of '%s' failed. Try calling `go_docs` on that package to see the correct API.", pkgName)
	}
	if matches := importErrorRe.FindStringSubmatch(msg); len(matches) > 1 {
		pkgPath := matches[1]
		//nolint:lll
		return fmt.Sprintf("\n\n**HINT:** import '%s' failed. Try calling `go_docs` on \"%s\" to verify the package path and exports.", pkgPath, pkgPath)
	}
	return ""
}
