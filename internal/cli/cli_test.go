package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danicat/godoctor/internal/config"
	smartbuild "github.com/danicat/godoctor/internal/tools/smart_build"
	"github.com/danicat/godoctor/internal/versioncheck"
)

func TestRun_NoArgs_PrintsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), "1.0.0", []string{}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error with no args, got: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "Usage:") || !strings.Contains(output, "godoctor") {
		t.Errorf("expected help output, got: %s", output)
	}
	if !strings.Contains(output, "mcp") || !strings.Contains(output, "call") || !strings.Contains(output, "list") {
		t.Errorf("expected subcommands in help output, got: %s", output)
	}
}

func TestRun_HelpAndVersion(t *testing.T) {
	testCases := []struct {
		args     []string
		expected string
	}{
		{[]string{"help"}, "Usage:"},
		{[]string{"--help"}, "Usage:"},
		{[]string{"-h"}, "Usage:"},
		{[]string{"version"}, "1.2.3"},
		{[]string{"--version"}, "1.2.3"},
		{[]string{"-v"}, "1.2.3"},
	}

	for _, tc := range testCases {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := Run(context.Background(), "1.2.3", tc.args, nil, &stdout, &stderr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(stdout.String(), tc.expected) {
				t.Errorf("expected output to contain %q, got: %q", tc.expected, stdout.String())
			}
		})
	}
}

func TestRun_List(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), "dev", []string{"list"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error on list: %v", err)
	}
	out := stdout.String()
	expectedTools := []string{"edit", "build", "test", "docs", "selene", "tq"}
	for _, tool := range expectedTools {
		if !strings.Contains(out, tool) {
			t.Errorf("expected tool %q in list output, got:\n%s", tool, out)
		}
	}
}

func TestRun_Init_GeneratesConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. First run creates .godoctor.yaml with full master template
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), "dev", []string{"init", "--dir", tmpDir}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error on init: %v", err)
	}

	configPath := filepath.Clean(filepath.Join(tmpDir, ".godoctor.yaml"))
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read created config file: %v", err)
	}

	if !strings.Contains(string(data), "GoDoctor") || !strings.Contains(string(data), `version: "1"`) {
		t.Errorf("expected standard template content in %s, got:\n%s", configPath, string(data))
	}
	hasSafeShell := strings.Contains(string(data), "safeshell:")
	hasInstructions := strings.Contains(string(data), "instructions:")
	hasMatching := strings.Contains(string(data), "matching:")
	if !hasSafeShell || !hasInstructions || !hasMatching {
		t.Errorf("expected all RFC-0001 sections in %s, got:\n%s", configPath, string(data))
	}

	// 2. Second run without --force should fail
	stdout.Reset()
	stderr.Reset()
	err = Run(context.Background(), "dev", []string{"init", "--dir", tmpDir}, nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when config file already exists and --force is not set")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}

	// 3. Third run with --force should succeed
	stdout.Reset()
	stderr.Reset()
	err = Run(context.Background(), "dev", []string{"init", "--dir", tmpDir, "--force"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error on init with --force: %v", err)
	}
	if !strings.Contains(stdout.String(), "Created") {
		t.Errorf("expected success message in stdout, got: %s", stdout.String())
	}

	// 4. Test minimal config template
	minimalDir := t.TempDir()
	stdout.Reset()
	stderr.Reset()
	err = Run(context.Background(), "dev", []string{"init", "-d", minimalDir, "-m"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error on init --minimal: %v", err)
	}
	minData, err := os.ReadFile(filepath.Clean(filepath.Join(minimalDir, ".godoctor.yaml")))
	if err != nil {
		t.Fatalf("failed to read minimal config file: %v", err)
	}
	if !strings.Contains(string(minData), "Minimal Configuration") {
		t.Errorf("expected minimal template, got:\n%s", string(minData))
	}
}

func TestRun_Check(t *testing.T) {
	// 1. Standard table output
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), "dev", []string{"check"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error on check: %v", err)
	}
	tableOut := stdout.String()
	if !strings.Contains(tableOut, "GoDoctor Environment & Tool Diagnostic Check") {
		t.Errorf("expected diagnostic check title, got:\n%s", tableOut)
	}
	if !strings.Contains(tableOut, "golangci-lint") || !strings.Contains(tableOut, "modernize") {
		t.Errorf("expected tool names in check output, got:\n%s", tableOut)
	}

	// 2. JSON output
	stdout.Reset()
	stderr.Reset()
	err = Run(context.Background(), "dev", []string{"check", "--json"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error on check --json: %v", err)
	}
	var statuses []versioncheck.ToolStatus
	if err := json.Unmarshal(stdout.Bytes(), &statuses); err != nil {
		t.Fatalf("failed to unmarshal JSON output from check --json: %v\nOutput was: %s", err, stdout.String())
	}
	if len(statuses) == 0 {
		t.Errorf("expected non-empty statuses list in JSON output")
	}

	// 3. No-cache flag
	stdout.Reset()
	stderr.Reset()
	err = Run(context.Background(), "dev", []string{"check", "--no-cache"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error on check --no-cache: %v", err)
	}

	// 4. With explicit workspace directory
	stdout.Reset()
	stderr.Reset()
	tmpDir := t.TempDir()
	err = Run(context.Background(), "dev", []string{"check", "-d", tmpDir}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error on check with dir: %v", err)
	}
}

func TestRun_Check_Strict(t *testing.T) {
	// If any tool is missing or outdated in the local environment, --strict should return an error
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), "dev", []string{"check", "--strict"}, nil, &stdout, &stderr)
	// We check if output contains table or json and error is handled cleanly
	if err != nil && !strings.Contains(err.Error(), "strict check failed") {
		t.Errorf("unexpected strict check error format: %v", err)
	}
}

func TestRun_Check_WithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, ".godoctor.yaml")
	cfgContent := `version: "1"
tools:
  golangci_lint:
    command: "golangci-lint"
    recommended_version: "v2.12.2"
`
	if err := os.WriteFile(filepath.Clean(cfgFile), []byte(cfgContent), 0600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), "dev", []string{"--config", cfgFile, "check"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error on check with --config: %v", err)
	}
}

func TestRun_GlobalFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), "dev", []string{"--quiet", "-V", "list"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error with global flags: %v", err)
	}
}

func TestRun_Call_UnknownTool(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), "dev", []string{"call", "non_existent_tool"}, nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("expected unknown tool error, got: %v", err)
	}
}

func TestRun_Call_MissingToolName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), "dev", []string{"call"}, nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing tool name")
	}
	if !strings.Contains(err.Error(), "missing tool name") {
		t.Errorf("expected missing tool name error, got: %v", err)
	}
}

func TestRun_Call_ReadDocs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	args := []string{"call", "docs", `{"import_path": "fmt", "symbol_name": "Println"}`}
	err := Run(context.Background(), "dev", args, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error calling docs: %v", err)
	}
	if !strings.Contains(stdout.String(), "Println") {
		t.Errorf("expected doc output for Println, got:\n%s", stdout.String())
	}
}

func TestRun_Call_Docs_StdinJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader(`{"import_path": "fmt", "symbol_name": "Sprintf"}`)
	err := Run(context.Background(), "dev", []string{"call", "docs"}, stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error calling docs with stdin: %v", err)
	}
	if !strings.Contains(stdout.String(), "Sprintf") {
		t.Errorf("expected doc output for Sprintf, got:\n%s", stdout.String())
	}
}

func TestRun_Call_RelativePathRejected(t *testing.T) {
	var stdout, stderr bytes.Buffer
	for _, toolName := range []string{"selene", "mutation_test", "build", "test", "tq", "testquery"} {
		t.Run(toolName, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			cmdArgs := []string{"call", toolName, `{"dir": ".", "query": "SELECT 1"}`}
			err := Run(context.Background(), "dev", cmdArgs, nil, &stdout, &stderr)
			if err == nil {
				t.Fatalf("expected error rejecting relative path for tool %s", toolName)
			}
			combined := stderr.String() + " " + err.Error()
			if !strings.Contains(combined, "must be an absolute path") {
				t.Errorf("expected absolute path error message, got: %s", combined)
			}
		})
	}
}

func TestRun_Call_SmartEdit_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Clean(filepath.Join(tmpDir, "go.mod")), []byte("module clitest\n\ngo 1.26.0\n"), 0600); err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Clean(filepath.Join(tmpDir, "main.go"))
	initialCode := "package main\n\nfunc main() {\n\tprintln(\"before\")\n}\n"
	if err := os.WriteFile(filePath, []byte(initialCode), 0600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	jsonArg := fmt.Sprintf(
		`{"filename": %q, "old_content": "println(\"before\")", "new_content": "println(\"after\")"}`,
		filePath,
	)
	err := Run(context.Background(), "dev", []string{"call", "edit", jsonArg}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error calling edit: %v (stderr: %s)", err, stderr.String())
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "println(\"after\")") {
		t.Errorf("expected file to be updated, got: %s", string(data))
	}
}

func TestRun_Call_InvalidJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), "dev", []string{"call", "docs", "{invalid json"}, nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for invalid JSON arguments")
	}
	if !strings.Contains(err.Error(), "invalid arguments") {
		t.Errorf("expected invalid arguments error, got: %v", err)
	}
}

func TestRun_Call_MissingJSONArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), "dev", []string{"call", "selene"}, nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing arguments")
	}
}

func TestRun_MCP_Cancellation(_ *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	var stdout, stderr bytes.Buffer
	// Running mcp with canceled context should exit cleanly
	_ = Run(ctx, "dev", []string{"mcp"}, nil, &stdout, &stderr)
}

func TestRun_MCP_InvalidTransport(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), "dev", []string{"mcp", "--transport", "unknown"}, nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for invalid transport")
	}
	if !strings.Contains(err.Error(), "unsupported transport") {
		t.Errorf("expected unsupported transport error, got: %v", err)
	}
}

type mockCLIRunner struct {
	calls [][]string
}

func (m *mockCLIRunner) Run(_ context.Context, _, _ string, _ ...string) error {
	return nil
}

func (m *mockCLIRunner) RunWithOutput(_ context.Context, _, name string, args ...string) (string, error) {
	call := append([]string{name}, args...)
	m.calls = append(m.calls, call)
	if name == "go" && len(args) > 0 && args[0] == "test" {
		return "PASS", nil
	}
	return "", nil
}

func (m *mockCLIRunner) LookPath(file string) (string, error) {
	return "/usr/bin/" + file, nil
}

func TestRun_Call_Build_OutputTarget(t *testing.T) {
	oldRunner := smartbuild.CommandRunner
	defer func() { smartbuild.CommandRunner = oldRunner }()

	runner := &mockCLIRunner{}
	smartbuild.CommandRunner = runner

	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Clean(filepath.Join(tmpDir, ".golangci.yml")), []byte("linters:\n  enable:\n    - errcheck\n"), 0600)

	var stdout, stderr bytes.Buffer
	args := []string{"call", "build", fmt.Sprintf(`{"dir": %q, "output": "bin/jsonbin"}`, tmpDir)}
	err := Run(context.Background(), "dev", args, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v (stderr: %s)", err, stderr.String())
	}

	var foundBuildWithO bool
	for _, call := range runner.calls {
		if len(call) >= 4 && call[0] == "go" && call[1] == "build" && call[2] == "-o" && call[3] == "bin/jsonbin" {
			foundBuildWithO = true
			break
		}
	}
	if !foundBuildWithO {
		t.Errorf("expected go build -o bin/jsonbin in runner calls, got: %+v", runner.calls)
	}
}

func TestPrintHelp(t *testing.T) {
	var buf bytes.Buffer
	err := Run(context.Background(), "1.0.0", []string{"--help"}, nil, &buf, &buf)
	if err != nil {
		t.Fatalf("unexpected error running help: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Usage:") || !strings.Contains(out, "godoctor") {
		t.Errorf("help output missing expected content: %s", out)
	}
}

func TestDefaultAndMinimalConfigTemplatesValid(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Test DefaultConfigFileTemplate loads and validates
	defaultPath := filepath.Clean(filepath.Join(tmpDir, "default.yaml"))
	if err := os.WriteFile(defaultPath, []byte(DefaultConfigFileTemplate), 0600); err != nil {
		t.Fatal(err)
	}
	cfgDefault, err := config.Load(defaultPath)
	if err != nil {
		t.Fatalf("DefaultConfigFileTemplate failed to load/validate: %v", err)
	}
	if cfgDefault.CLI.Timeout != 60*time.Second {
		t.Errorf("expected cli timeout 60s, got %v", cfgDefault.CLI.Timeout)
	}

	// 2. Test MinimalConfigFileTemplate loads and validates
	minimalPath := filepath.Clean(filepath.Join(tmpDir, "minimal.yaml"))
	if err := os.WriteFile(minimalPath, []byte(MinimalConfigFileTemplate), 0600); err != nil {
		t.Fatal(err)
	}
	cfgMinimal, err := config.Load(minimalPath)
	if err != nil {
		t.Fatalf("MinimalConfigFileTemplate failed to load/validate: %v", err)
	}
	if !cfgMinimal.Features.Autofix {
		t.Errorf("expected minimal autofix feature to be true")
	}
}
