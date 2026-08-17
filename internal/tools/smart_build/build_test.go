package smartbuild

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/danicat/godoctor/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type commandCall struct {
	Dir  string
	Name string
	Args []string
}

func (c commandCall) String() string {
	if len(c.Args) == 0 {
		return c.Name
	}
	return c.Name + " " + strings.Join(c.Args, " ")
}

type mockRunner struct {
	mu           sync.Mutex
	calls        []commandCall
	outputs      map[string]string
	errors       map[string]error
	lookPathMap  map[string]string
	lookPathErr  map[string]error
	runFunc      func(ctx context.Context, dir, name string, args ...string) error
	runOutFunc   func(ctx context.Context, dir, name string, args ...string) (string, error)
	lookPathFunc func(file string) (string, error)
}

func (r *mockRunner) recordCall(dir, name string, args ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, commandCall{
		Dir:  dir,
		Name: name,
		Args: append([]string(nil), args...),
	})
}

func (r *mockRunner) getCalls() []commandCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := make([]commandCall, len(r.calls))
	copy(res, r.calls)
	return res
}

func (r *mockRunner) matchOutput(cmd string) string {
	var bestKey string
	var bestOut string
	for k, v := range r.outputs {
		if strings.Contains(cmd, k) && len(k) > len(bestKey) {
			bestKey = k
			bestOut = v
		}
	}
	return bestOut
}

func (r *mockRunner) matchError(cmd string) error {
	var bestKey string
	var bestErr error
	for k, v := range r.errors {
		if strings.Contains(cmd, k) && len(k) > len(bestKey) {
			bestKey = k
			bestErr = v
		}
	}
	return bestErr
}

func (r *mockRunner) Run(ctx context.Context, dir, name string, args ...string) error {
	r.recordCall(dir, name, args...)
	if r.runFunc != nil {
		return r.runFunc(ctx, dir, name, args...)
	}
	cmd := name + " " + strings.Join(args, " ")
	return r.matchError(cmd)
}

func (r *mockRunner) RunWithOutput(ctx context.Context, dir, name string, args ...string) (string, error) {
	r.recordCall(dir, name, args...)
	if r.runOutFunc != nil {
		return r.runOutFunc(ctx, dir, name, args...)
	}
	cmd := name + " " + strings.Join(args, " ")
	return r.matchOutput(cmd), r.matchError(cmd)
}

func (r *mockRunner) LookPath(file string) (string, error) {
	if r.lookPathFunc != nil {
		return r.lookPathFunc(file)
	}
	if r.lookPathErr != nil {
		if err, ok := r.lookPathErr[file]; ok {
			return "", err
		}
	}
	if r.lookPathMap != nil {
		if path, ok := r.lookPathMap[file]; ok {
			return path, nil
		}
		return "", fmt.Errorf("executable %q not found in mock LookPath", file)
	}
	return "/usr/bin/" + file, nil
}

func TestParsePackages(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty string", "", []string{"./..."}},
		{"whitespace only", "   \t\n", []string{"./..."}},
		{"single package", "./pkg/...", []string{"./pkg/..."}},
		{"space separated", "./cmd/... ./internal/...", []string{"./cmd/...", "./internal/..."}},
		{"comma separated", "./cmd/...,./internal/...", []string{"./cmd/...", "./internal/..."}},
		{"comma and space mixed", "./cmd/...,  ./pkg1, ./pkg2", []string{"./cmd/...", "./pkg1", "./pkg2"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePackages(tc.input)
			if len(got) != len(tc.expected) {
				t.Fatalf("parsePackages(%q) length = %d, want %d (%v)", tc.input, len(got), len(tc.expected), got)
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Errorf("parsePackages(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.expected[i])
				}
			}
		})
	}
}

func TestHandler_Success(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	dir := t.TempDir()

	CommandRunner = &mockRunner{
		outputs: map[string]string{
			"go build":          "",
			"go test":           "PASS",
			"golangci-lint run": "",
		},
	}

	res, _, err := Handler(context.Background(), nil, Params{
		Dir: dir,
	})
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}
	if res.IsError {
		t.Errorf("Expected success, got error result: %v", res.Content[0].(*mcp.TextContent).Text)
	}

	out := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(out, "Build: ✅ PASS") {
		t.Errorf("Expected build success in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Tests: ✅ PASS") {
		t.Errorf("Expected test success in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Deadcode Analysis: ✅ PASS") {
		t.Errorf("Expected deadcode pass in output, got:\n%s", out)
	}
}

func TestHandler_OutputTarget(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		outputs: map[string]string{
			"go build":          "",
			"go test":           "PASS",
			"golangci-lint run": "",
		},
	}
	CommandRunner = runner

	dir := t.TempDir()
	res, _, err := Handler(context.Background(), nil, Params{
		Dir:    dir,
		Output: "bin/godoctor",
	})
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}
	if res.IsError {
		t.Errorf("Expected success, got error result: %v", res.Content[0].(*mcp.TextContent).Text)
	}

	calls := runner.getCalls()
	var foundBuildWithO bool
	for _, call := range calls {
		if call.Name == "go" && len(call.Args) >= 3 &&
			call.Args[0] == "build" && call.Args[1] == "-o" && call.Args[2] == "bin/godoctor" {
			foundBuildWithO = true
			break
		}
	}
	if !foundBuildWithO {
		t.Errorf("Expected go build -o bin/godoctor in calls, got: %+v", calls)
	}
}

func TestHandler_RelativePathRejected(t *testing.T) {
	testCases := []struct {
		name string
		dir  string
	}{
		{"empty dir", ""},
		{"current dir dot", "."},
		{"relative subpath", "./subpkg"},
		{"relative plain", "subpkg"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res, _, err := Handler(context.Background(), nil, Params{
				Dir: tc.dir,
			})
			if err != nil {
				t.Fatalf("unexpected handler error: %v", err)
			}
			if !res.IsError {
				t.Errorf("expected error for non-absolute dir %q", tc.dir)
			}
			text := res.Content[0].(*mcp.TextContent).Text
			if !strings.Contains(text, "dir is required and must be an absolute path") {
				t.Errorf("expected absolute path error message, got:\n%s", text)
			}
		})
	}
}

func TestHandler_BuildFail(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	CommandRunner = &mockRunner{
		outputs: map[string]string{
			"go build": "syntax error",
		},
		errors: map[string]error{
			"go build": fmt.Errorf("exit status 1"),
		},
	}

	dir := t.TempDir()
	res, _, err := Handler(context.Background(), nil, Params{
		Dir: dir,
	})
	if err != nil {
		t.Fatalf("Handler returned unexpected error: %v", err)
	}

	if !res.IsError {
		t.Error("Expected error result for build failure")
	}

	out := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(out, "Build: ❌ FAILED") {
		t.Errorf("Expected build failure in output, got:\n%s", out)
	}
}

func TestHandler_ContextCancelled(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	CommandRunner = &mockRunner{}

	dir := t.TempDir()
	res, _, err := Handler(ctx, nil, Params{
		Dir: dir,
	})
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}
	if !res.IsError {
		t.Error("Expected error result for canceled context")
	}

	out := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(out, "Canceled") {
		t.Errorf("Expected cancellation message in output, got:\n%s", out)
	}
}

func TestHandler_Deadcode_UnreachableCode(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	CommandRunner = &mockRunner{
		outputs: map[string]string{
			"go build":          "",
			"go test":           "PASS",
			"golangci-lint run": "",
			"deadcode":          "main.go:10:6: unreachable func: UnusedFunc",
		},
	}

	dir := t.TempDir()
	res, _, err := Handler(context.Background(), nil, Params{
		Dir: dir,
	})
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	out := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(out, "Deadcode Analysis: ⚠️ Unreachable functions detected") {
		t.Errorf("Expected deadcode warning in output, got:\n%s", out)
	}
	if !strings.Contains(out, "UnusedFunc") {
		t.Errorf("Expected unreachable function in output, got:\n%s", out)
	}
}

func TestHandler_Deadcode_Fail(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	CommandRunner = &mockRunner{
		outputs: map[string]string{
			"go build":          "",
			"go test":           "PASS",
			"golangci-lint run": "",
			"deadcode":          "deadcode: packages contain errors",
		},
		errors: map[string]error{
			"deadcode": fmt.Errorf("exit status 1"),
		},
	}

	dir := t.TempDir()
	res, _, err := Handler(context.Background(), nil, Params{
		Dir: dir,
	})
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	out := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(out, "Deadcode Analysis: ❌ FAILED") {
		t.Errorf("Expected deadcode failure in output, got:\n%s", out)
	}
}

func TestFindCoverageIndex(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected int
	}{
		{
			name:     "at index 0",
			parts:    []string{"coverage:", "85.5%"},
			expected: 0,
		},
		{
			name:     "in middle index",
			parts:    []string{"ok", "github.com/danicat/godoctor", "0.05s", "coverage:", "85.5%", "of", "statements"},
			expected: 3,
		},
		{
			name:     "at end index",
			parts:    []string{"ok", "pkg", "coverage:"},
			expected: 2,
		},
		{
			name:     "not found in slice",
			parts:    []string{"ok", "pkg", "0.05s", "no_match"},
			expected: -1,
		},
		{
			name:     "empty slice",
			parts:    []string{},
			expected: -1,
		},
		{
			name:     "similar word without colon",
			parts:    []string{"ok", "pkg", "coverage", "85.5%"},
			expected: -1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := findCoverageIndex(tc.parts)
			if got != tc.expected {
				t.Errorf("findCoverageIndex(%v) = %d, want %d", tc.parts, got, tc.expected)
			}
		})
	}
}

func TestGetDocHintFromOutput(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		contains []string
		empty    bool
	}{
		{
			name:     "undefined symbol in package",
			msg:      "./main.go:12:3: undefined: logger.Log",
			contains: []string{"HINT:", "usage of 'logger' failed", "`read_docs`", "logger"},
		},
		{
			name: "could not import package",
			msg:  "./main.go:4:2: could not import github.com/danicat/godoctor/pkg (cannot find package)",
			contains: []string{
				"HINT:",
				"import 'github.com/danicat/godoctor/pkg' failed",
				"`read_docs`",
				"\"github.com/danicat/godoctor/pkg\"",
			},
		},
		{
			name:     "package error format",
			msg:      "./main.go:4:2: package example.com/tools/foo is not in std",
			contains: []string{"HINT:", "import 'example.com/tools/foo' failed", "`read_docs`", "\"example.com/tools/foo\""},
		},
		{
			name:  "generic syntax error",
			msg:   "./main.go:10:1: syntax error: unexpected semicolon",
			empty: true,
		},
		{
			name:  "empty message",
			msg:   "",
			empty: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := getDocHintFromOutput(tc.msg)
			if tc.empty {
				if got != "" {
					t.Errorf("expected empty doc hint, got: %q", got)
				}
				return
			}
			for _, sub := range tc.contains {
				if !strings.Contains(got, sub) {
					t.Errorf("expected doc hint to contain %q, got:\n%s", sub, got)
				}
			}
		})
	}
}

func TestRunBuild_SinglePackage(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		outputs: map[string]string{
			"go build": "",
		},
	}
	CommandRunner = runner

	var sb strings.Builder
	err := runBuild(context.Background(), "/workspace", "./...", "", &sb)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if !strings.Contains(sb.String(), "### 🛠  Build: ✅ PASS\n\n") {
		t.Errorf("expected build pass output, got:\n%s", sb.String())
	}
}

func TestRunBuild_MultiplePackages(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		outputs: map[string]string{
			"go build ./cmd/... ./pkg/...": "",
		},
	}
	CommandRunner = runner

	var sb strings.Builder
	err := runBuild(context.Background(), "/workspace", "./cmd/..., ./pkg/...", "", &sb)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	calls := runner.getCalls()
	if len(calls) == 0 {
		t.Fatal("expected runner calls")
	}
	lastCall := calls[len(calls)-1]
	if len(lastCall.Args) != 3 || lastCall.Args[0] != "build" || lastCall.Args[1] != "./cmd/..." || lastCall.Args[2] != "./pkg/..." {
		t.Errorf("expected build args with spread packages, got: %v", lastCall.Args)
	}
}

func TestRunBuild_WithOutputTarget(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		outputs: map[string]string{
			"go build -o bin/app ./...": "",
		},
	}
	CommandRunner = runner

	var sb strings.Builder
	err := runBuild(context.Background(), "/workspace", "./...", "bin/app", &sb)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if !strings.Contains(sb.String(), "### 🛠  Build: ✅ PASS\n\n") {
		t.Errorf("expected build pass output, got:\n%s", sb.String())
	}
	calls := runner.getCalls()
	if len(calls) == 0 {
		t.Fatal("expected calls to runner")
	}
	lastCall := calls[len(calls)-1]
	if len(lastCall.Args) < 3 || lastCall.Args[0] != "build" ||
		lastCall.Args[1] != "-o" || lastCall.Args[2] != "bin/app" {
		t.Errorf("expected go build -o bin/app args, got: %v", lastCall.Args)
	}
}

func TestRunBuild_FailureWithUndefinedSymbolHint(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		outputs: map[string]string{
			"go build": "main.go:15:2: undefined: logger.Log",
		},
		errors: map[string]error{
			"go build": fmt.Errorf("exit status 1"),
		},
	}
	CommandRunner = runner

	var sb strings.Builder
	err := runBuild(context.Background(), "/workspace", "./...", "", &sb)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	out := sb.String()
	if !strings.Contains(out, "### 🛠  Build: ❌ FAILED\n\n") {
		t.Errorf("expected build fail output, got:\n%s", out)
	}
	if !strings.Contains(out, "undefined: logger.Log") {
		t.Errorf("expected build output in report, got:\n%s", out)
	}
	if !strings.Contains(out, "usage of 'logger' failed. Try calling `read_docs`") {
		t.Errorf("expected doc hint for logger, got:\n%s", out)
	}
}

func TestRunAutoFix_FeatureDisabled(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	cfg.Features.Autofix = false

	var sb strings.Builder
	runAutoFix(context.Background(), "/workspace", "./...", cfg, &sb)

	if sb.Len() != 0 {
		t.Errorf("expected no output when Autofix is disabled, got: %q", sb.String())
	}
	if len(runner.getCalls()) != 0 {
		t.Errorf("expected no calls when Autofix is disabled, got: %v", runner.getCalls())
	}
}

func TestRunAutoFix_Failures(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		errors: map[string]error{
			"go mod tidy": fmt.Errorf("tidy network error"),
			"modernize":   fmt.Errorf("exit status 1"),
			"gofmt":       fmt.Errorf("gofmt permission denied"),
		},
		outputs: map[string]string{
			"modernize": "error detail on line 5",
		},
	}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	var sb strings.Builder
	runAutoFix(context.Background(), "/workspace", "./...", cfg, &sb)
	out := sb.String()

	if !strings.Contains(out, "❌ Go Mod Tidy: FAILED (tidy network error)") {
		t.Errorf("expected mod tidy failed, got:\n%s", out)
	}
	if !strings.Contains(out, "❌ Go Modernizer: FAILED (exit status 1)") {
		t.Errorf("expected modernize failed, got:\n%s", out)
	}
	if !strings.Contains(out, "error detail on line 5") {
		t.Errorf("expected modernize output detail, got:\n%s", out)
	}
	if !strings.Contains(out, "❌ Go Code Formatter: FAILED (gofmt permission denied)") {
		t.Errorf("expected gofmt failed, got:\n%s", out)
	}
}

func TestRunAutoFix_ModernizeExitStatus3(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		lookPathMap: map[string]string{
			"modernize": "/usr/local/bin/modernize",
		},
		errors: map[string]error{
			"modernize": fmt.Errorf("exit status 3: modernized 2 files"),
		},
		outputs: map[string]string{
			"modernize": "pkg/foo.go: replaced min with built-in min",
		},
	}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	var sb strings.Builder
	runAutoFix(context.Background(), "/workspace", "./...", cfg, &sb)
	out := sb.String()

	if !strings.Contains(out, "✅ Go Mod Tidy: SUCCESS") {
		t.Errorf("expected mod tidy success, got:\n%s", out)
	}
	if !strings.Contains(out, "✅ Go Modernizer: SUCCESS (Issues found and auto-fixed)") {
		t.Errorf("expected modernize exit 3 success, got:\n%s", out)
	}
	if !strings.Contains(out, "replaced min with built-in min") {
		t.Errorf("expected modernize details in report, got:\n%s", out)
	}
	if !strings.Contains(out, "✅ Go Code Formatter: SUCCESS") {
		t.Errorf("expected gofmt success, got:\n%s", out)
	}
}

func TestRunAutoFix_Clean(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		outputs: map[string]string{
			"modernize": "",
		},
	}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	var sb strings.Builder
	runAutoFix(context.Background(), "/workspace", "./...", cfg, &sb)
	out := sb.String()

	if !strings.Contains(out, "✅ Go Modernizer: SUCCESS (No issues found)") {
		t.Errorf("expected modernize no issues, got:\n%s", out)
	}
}

func TestRunDeadcodePhase_Clean(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		outputs: map[string]string{
			"deadcode": "   \n\n  ",
		},
	}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	var sb strings.Builder
	runDeadcodePhase(context.Background(), "/workspace", "./...", cfg, &sb)
	out := sb.String()

	if !strings.Contains(out, "### 🔍 Deadcode Analysis: ✅ PASS (No unreachable code found)") {
		t.Errorf("expected clean deadcode pass, got:\n%s", out)
	}
}

func TestRunDeadcodePhase_Disabled(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	cfg.Features.DeadcodeCheck = false

	var sb strings.Builder
	runDeadcodePhase(context.Background(), "/workspace", "./...", cfg, &sb)

	if sb.Len() != 0 {
		t.Errorf("expected no output when DeadcodeCheck is disabled, got: %q", sb.String())
	}
}

func TestRunDeadcodePhase_UnreachableDetected(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		outputs: map[string]string{
			"deadcode": "main.go:20:1: unreachable func DeadHelper",
		},
	}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	var sb strings.Builder
	runDeadcodePhase(context.Background(), "/workspace", "./...", cfg, &sb)
	out := sb.String()

	if !strings.Contains(out, "### 🔍 Deadcode Analysis: ⚠️ Unreachable functions detected:") {
		t.Errorf("expected deadcode warning, got:\n%s", out)
	}
	if !strings.Contains(out, "DeadHelper") {
		t.Errorf("expected DeadHelper in output, got:\n%s", out)
	}
}

func TestRunDeadcodePhase_Missing(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		lookPathErr: map[string]error{
			"deadcode": fmt.Errorf("not found"),
		},
	}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	var sb strings.Builder
	runDeadcodePhase(context.Background(), "/workspace", "./pkg1 ./pkg2", cfg, &sb)
	out := sb.String()

	if !strings.Contains(out, "### 🔍 Deadcode Analysis: ⏩ SKIPPED (`deadcode` binary not found in PATH)") {
		t.Errorf("expected deadcode skipped notice, got:\n%s", out)
	}
}

func TestSyncTestQueryDB_LookPathTestQuery(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		lookPathMap: map[string]string{
			"testquery": "/usr/local/bin/testquery",
		},
	}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	var sb strings.Builder
	syncTestQueryDB(context.Background(), "/workspace", "./...", cfg, &sb)

	calls := runner.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d (calls=%+v, sb=%q)", len(calls), calls, sb.String())
	}
	expectedArgs := fmt.Sprintf("build --pkg ./... --output %s", cfg.TestQuery.DatabasePath)
	if calls[0].Name != "testquery" || strings.Join(calls[0].Args, " ") != expectedArgs {
		t.Errorf("unexpected command call: %v (want %s)", calls[0], expectedArgs)
	}
}

func TestSyncTestQueryDB_LookPathTQ(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		lookPathErr: map[string]error{
			"testquery": fmt.Errorf("not found"),
		},
		lookPathMap: map[string]string{
			"tq": "/usr/local/bin/tq",
		},
	}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	var sb strings.Builder
	syncTestQueryDB(context.Background(), "/workspace", "./pkg/...", cfg, &sb)

	calls := runner.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	expectedArgs := fmt.Sprintf("build --pkg ./pkg/... --output %s", cfg.TestQuery.DatabasePath)
	if calls[0].Name != "tq" || strings.Join(calls[0].Args, " ") != expectedArgs {
		t.Errorf("unexpected command call: %v (want %s)", calls[0], expectedArgs)
	}
}

func TestSyncTestQueryDB_Missing(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		lookPathErr: map[string]error{
			"testquery": fmt.Errorf("not found"),
			"tq":        fmt.Errorf("not found"),
		},
	}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	var sb strings.Builder
	syncTestQueryDB(context.Background(), "/workspace", "./...", cfg, &sb)

	calls := runner.getCalls()
	if len(calls) != 0 {
		t.Fatalf("expected 0 calls when tool missing, got %d: %v", len(calls), calls)
	}
}

type totalCovTestCase struct {
	name     string
	output   string
	err      error
	expected string
}

func getTotalCoverageTestCases() []totalCovTestCase {
	return []totalCovTestCase{
		{
			name:     "valid coverage summary",
			output:   "github.com/danicat/godoctor/pkg/foo.go:10:\tDoSomething\t100.0%\ntotal:\t\t(statements)\t\t75.0%",
			expected: "75.0%",
		},
		{
			name:     "valid coverage with spaces",
			output:   "total: (statements) 82.4%",
			expected: "82.4%",
		},
		{
			name:     "error running cover tool",
			output:   "",
			err:      fmt.Errorf("exit status 1"),
			expected: "",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
		{
			name:     "last line missing total prefix",
			output:   "some/file.go:10: Func 100.0%\nother text 75.0%",
			expected: "",
		},
		{
			name:     "short parts on total line",
			output:   "total: (statements)",
			expected: "",
		},
	}
}

func TestParseTotalCoverage(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	for _, tc := range getTotalCoverageTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			runner := &mockRunner{
				outputs: map[string]string{
					"go tool cover": tc.output,
				},
				errors: map[string]error{
					"go tool cover": tc.err,
				},
			}
			CommandRunner = runner

			got := parseTotalCoverage(context.Background(), "/workspace", "cov.out")
			if got != tc.expected {
				t.Errorf("parseTotalCoverage() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestParsePackagesCoverage(t *testing.T) {
	tests := []struct {
		name     string
		testOut  string
		contains []string
		empty    bool
	}{
		{
			name: "single package pass with coverage",
			testOut: "=== RUN   TestFoo\n--- PASS: TestFoo (0.01s)\n" +
				"ok  \tgithub.com/danicat/godoctor/pkg/foo\t0.02s\tcoverage: 85.5% of statements\n",
			contains: []string{
				"- **Packages**:",
				"`github.com/danicat/godoctor/pkg/foo`: 85.5%",
			},
		},
		{
			name: "multiple packages with duplicates and no-test packages",
			testOut: "ok  \tgithub.com/danicat/godoctor/pkg/foo\t0.02s\tcoverage: 85.5% of statements\n" +
				"ok  \tgithub.com/danicat/godoctor/pkg/foo\t0.02s\tcoverage: 85.5% of statements\n" +
				"?   \tgithub.com/danicat/godoctor/pkg/notests\t[no test files]\n" +
				"ok  \tgithub.com/danicat/godoctor/pkg/bar\t0.05s\tcoverage: 92.0% of statements\n",
			contains: []string{
				"- **Packages**:",
				"`github.com/danicat/godoctor/pkg/foo`: 85.5%",
				"`github.com/danicat/godoctor/pkg/bar`: 92.0%",
			},
		},
		{
			name: "package without statement percentage (no statements)",
			testOut: "=== RUN   TestEmpty\n--- PASS: TestEmpty (0.00s)\n" +
				"ok  \tgithub.com/danicat/godoctor/pkg/empty\t0.01s\tcoverage: [no statements]\n",
			empty: true,
		},
		{
			name:    "no coverage strings at all",
			testOut: "=== RUN   TestFoo\n--- PASS: TestFoo (0.01s)\nPASS\n",
			empty:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var sb strings.Builder
			parsePackagesCoverage(tc.testOut, &sb)
			got := sb.String()

			if tc.empty {
				if got != "" {
					t.Errorf("expected empty coverage output, got: %q", got)
				}
				return
			}

			for _, expectedStr := range tc.contains {
				if !strings.Contains(got, expectedStr) {
					t.Errorf("expected output to contain %q, got:\n%s", expectedStr, got)
				}
			}
		})
	}
}

func TestRunLinterPhase_Pass(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".golangci.yml")
	_ = os.WriteFile(cfgPath, []byte("version: 2\n"), 0o600)

	runner := &mockRunner{
		lookPathMap: map[string]string{
			"golangci-lint": "/usr/local/bin/golangci-lint",
		},
		outputs: map[string]string{
			"golangci-lint run": "",
		},
	}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	var sb strings.Builder
	err := runLinterPhase(context.Background(), dir, "./pkg1 ./pkg2", cfg, &sb)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "(using `golangci-lint`) ✅ PASS") {
		t.Errorf("expected golangci-lint pass, got:\n%s", out)
	}
	calls := runner.getCalls()
	if len(calls) != 1 || calls[0].Args[0] != "run" {
		t.Errorf("expected lint args with run, got: %v", calls)
	}
}

func TestRunLinterPhase_Fail(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".golangci.yml")
	_ = os.WriteFile(cfgPath, []byte("version: 2\n"), 0o600)

	runner := &mockRunner{
		lookPathMap: map[string]string{
			"golangci-lint": "/usr/local/bin/golangci-lint",
		},
		outputs: map[string]string{
			"golangci-lint run": "file.go:1: lint error",
		},
		errors: map[string]error{
			"golangci-lint run": fmt.Errorf("exit status 1"),
		},
	}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	var sb strings.Builder
	err := runLinterPhase(context.Background(), dir, "./...", cfg, &sb)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	out := sb.String()
	if !strings.Contains(out, "(using `golangci-lint`) ⚠️ ISSUES FOUND") {
		t.Errorf("expected lint issues found, got:\n%s", out)
	}
	if !strings.Contains(out, "file.go:1: lint error") {
		t.Errorf("expected lint error detail in output, got:\n%s", out)
	}
}

func TestRunLinterPhase_MissingBinary(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	dir := t.TempDir()
	runner := &mockRunner{
		lookPathErr: map[string]error{
			"golangci-lint": fmt.Errorf("not found"),
		},
	}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	var sb strings.Builder
	err := runLinterPhase(context.Background(), dir, "./...", cfg, &sb)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "⏩ SKIPPED (`golangci-lint` binary not found in PATH)") {
		t.Errorf("expected skipped notice, got:\n%s", out)
	}
}

func TestRunLinterPhase_Disabled(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	cfg.Build.RunLinter = false

	var sb strings.Builder
	err := runLinterPhase(context.Background(), "/workspace", "./...", cfg, &sb)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if sb.Len() != 0 {
		t.Errorf("expected no output when RunLinter is false, got: %q", sb.String())
	}
}

func TestRunTestsPhase_PassWithCoverage(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		outputs: map[string]string{
			"go test":       "ok  \tgithub.com/danicat/godoctor/pkg\t0.02s\tcoverage: 80.0% of statements",
			"go tool cover": "total: (statements) 80.0%",
		},
	}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	var sb strings.Builder
	err := runTestsPhase(context.Background(), "/workspace", "./pkg1 ./pkg2", cfg, &sb)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "### 🧪 Tests: ✅ PASS\n\n") {
		t.Errorf("expected test pass output, got:\n%s", out)
	}
	if !strings.Contains(out, "Total Project Coverage**: 80.0%") {
		t.Errorf("expected total coverage in output, got:\n%s", out)
	}
	if !strings.Contains(out, "`github.com/danicat/godoctor/pkg`: 80.0%") {
		t.Errorf("expected pkg coverage in output, got:\n%s", out)
	}
	if !strings.Contains(out, "*Indexed test run to `"+cfg.TestQuery.DatabasePath+"`*") {
		t.Errorf("expected testquery indexing notice, got:\n%s", out)
	}

	calls := runner.getCalls()
	var foundTestCall bool
	for _, c := range calls {
		if c.Name == "go" && len(c.Args) >= 4 && c.Args[0] == "test" {
			foundTestCall = true
			if !strings.Contains(c.Args[2], "-coverprofile=") {
				t.Errorf("expected -coverprofile arg, got: %v", c.Args)
			}
			if c.Args[3] != "./pkg1" || c.Args[4] != "./pkg2" {
				t.Errorf("expected spread package arguments in test call, got: %v", c.Args)
			}
		}
	}
	if !foundTestCall {
		t.Errorf("expected go test call, got calls: %v", calls)
	}
}

func TestRunTestsPhase_FailWithHint(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		outputs: map[string]string{
			"go test": "--- FAIL: TestBroken (0.01s)\n    broken_test.go:10: assert failed",
		},
		errors: map[string]error{
			"go test": fmt.Errorf("exit status 1"),
		},
	}
	CommandRunner = runner

	cfg := config.NewDefaultConfig()
	var sb strings.Builder
	err := runTestsPhase(context.Background(), "/workspace", "./...", cfg, &sb)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	out := sb.String()
	if !strings.Contains(out, "### 🧪 Tests: ❌ FAILED\n\n") {
		t.Errorf("expected test fail output, got:\n%s", out)
	}
	if !strings.Contains(out, "broken_test.go:10: assert failed") {
		t.Errorf("expected test failure output details, got:\n%s", out)
	}
	if !strings.Contains(out, "**HINT:** Run `test_query` (`tq`) to query failing tests via SQL:") {
		t.Errorf("expected tq hint in output, got:\n%s", out)
	}
}

func TestHandler_Phases_TestFailureAborts(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		outputs: map[string]string{
			"go build": "",
			"go test":  "FAIL",
		},
		errors: map[string]error{
			"go test": fmt.Errorf("exit status 1"),
		},
	}
	CommandRunner = runner

	dir := t.TempDir()
	res, _, err := Handler(context.Background(), nil, Params{
		Dir: dir,
	})
	if err != nil {
		t.Fatalf("unexpected Handler error: %v", err)
	}
	if !res.IsError {
		t.Error("expected res.IsError == true when tests fail")
	}
	out := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(out, "Tests: ❌ FAILED") {
		t.Errorf("expected test failure in report, got:\n%s", out)
	}
	if strings.Contains(out, "### 🧹 Lint:") {
		t.Errorf("lint phase should not have run after test failure, got:\n%s", out)
	}
	if strings.Contains(out, "### 🔍 Deadcode Analysis:") {
		t.Errorf("deadcode phase should not have run after test failure, got:\n%s", out)
	}
}

func TestHandler_Phases_LinterFailureAborts(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		outputs: map[string]string{
			"go build":          "",
			"go test":           "PASS",
			"golangci-lint run": "lint warning",
		},
		errors: map[string]error{
			"golangci-lint run": fmt.Errorf("exit status 1"),
		},
	}
	CommandRunner = runner

	dir := t.TempDir()
	res, _, err := Handler(context.Background(), nil, Params{
		Dir:      dir,
		Packages: "./custom/...",
	})
	if err != nil {
		t.Fatalf("unexpected Handler error: %v", err)
	}
	if !res.IsError {
		t.Error("expected res.IsError == true when linter fails")
	}
	out := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(out, "# Smart Build Report (`./custom/...`)") {
		t.Errorf("expected custom packages in header, got:\n%s", out)
	}
	if !strings.Contains(out, "🧹 Lint: (using `golangci-lint`) ⚠️ ISSUES FOUND") {
		t.Errorf("expected lint failure in report, got:\n%s", out)
	}
	if strings.Contains(out, "### 🔍 Deadcode Analysis:") {
		t.Errorf("deadcode phase should not have run after linter failure, got:\n%s", out)
	}
}

func TestFormatOutput(t *testing.T) {
	if got := formatOutput(""); got != "" {
		t.Errorf("formatOutput(\"\") = %q, want empty", got)
	}
	want := "```text\nhello world\n```\n"
	if got := formatOutput("  hello world  \n"); got != want {
		t.Errorf("formatOutput() = %q, want %q", got, want)
	}
}

func TestRegister(_ *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	Register(server)
}

func TestStdRunner(t *testing.T) {
	runner := &stdRunner{}
	ctx := context.Background()
	dir := t.TempDir()

	path, err := runner.LookPath("go")
	if err != nil || path == "" {
		t.Fatalf("LookPath(\"go\") failed: %v", err)
	}

	if err := runner.Run(ctx, dir, "go", "version"); err != nil {
		t.Errorf("runner.Run(\"go\", \"version\") failed: %v", err)
	}

	out, err := runner.RunWithOutput(ctx, dir, "go", "version")
	if err != nil {
		t.Errorf("runner.RunWithOutput(\"go\", \"version\") failed: %v", err)
	}
	if !strings.Contains(out, "go version") {
		t.Errorf("expected output to contain 'go version', got: %s", out)
	}

	if err := runner.Run(ctx, dir, "go;echo", "version"); err == nil {
		t.Error("expected error for command with shell operator, got nil")
	}

	if _, err := runner.RunWithOutput(ctx, dir, "go|echo", "version"); err == nil {
		t.Error("expected error for command with shell operator, got nil")
	}
}
