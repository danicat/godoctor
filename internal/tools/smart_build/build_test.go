package smartbuild

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

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

func TestHandler_Success(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	CommandRunner = &mockRunner{
		outputs: map[string]string{
			"go build": "",
			"go test":  "PASS",
		},
	}

	res, _, err := Handler(context.Background(), nil, Params{
		Dir: "/path/to/workspace",
	})
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}
	if res.IsError {
		t.Error("Expected success, got error result")
	}

	out := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(out, "Build: ✅ PASS") {
		t.Errorf("Expected build success in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Tests: ✅ PASS") {
		t.Errorf("Expected test success in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Deadcode Analysis: SUCCESS") {
		t.Errorf("Expected deadcode pass in output, got:\n%s", out)
	}
}

func TestHandler_OutputTarget(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	runner := &mockRunner{
		outputs: map[string]string{
			"go build": "",
			"go test":  "PASS",
		},
	}
	CommandRunner = runner

	res, _, err := Handler(context.Background(), nil, Params{
		Dir:    "/path/to/workspace",
		Output: "bin/godoctor",
	})
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}
	if res.IsError {
		t.Error("Expected success, got error result")
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

	res, _, err := Handler(context.Background(), nil, Params{
		Dir: "/path/to/workspace",
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

func TestHandler_Deadcode_UnreachableCode(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	CommandRunner = &mockRunner{
		outputs: map[string]string{
			"go build": "",
			"go test":  "PASS",
			"deadcode": "main.go:10:6: unreachable func: UnusedFunc",
		},
	}

	res, _, err := Handler(context.Background(), nil, Params{
		Dir: "/path/to/workspace",
	})
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	out := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(out, "Deadcode Analysis: Unreachable functions detected") {
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
			"go build": "",
			"go test":  "PASS",
			"deadcode": "deadcode: packages contain errors",
		},
		errors: map[string]error{
			"deadcode": fmt.Errorf("exit status 1"),
		},
	}

	res, _, err := Handler(context.Background(), nil, Params{
		Dir: "/path/to/workspace",
	})
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	out := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(out, "Deadcode Analysis: FAILED") {
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
			msg:      "./main.go:12:3: undefined: helper.DoWork",
			contains: []string{"HINT:", "usage of 'helper' failed", "`read_docs`", "helper"},
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

//nolint:funlen
func TestRunBuild(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("build success", func(t *testing.T) {
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
	})

	t.Run("build success with output target", func(t *testing.T) {
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
	})

	t.Run("build failure with undefined symbol hint", func(t *testing.T) {
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
	})

	t.Run("build failure with could not import hint", func(t *testing.T) {
		runner := &mockRunner{
			outputs: map[string]string{
				"go build": "main.go:3:8: could not import github.com/user/pkg (not found)",
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
		if !strings.Contains(out, "import 'github.com/user/pkg' failed. Try calling `read_docs`") {
			t.Errorf("expected doc hint for github.com/user/pkg, got:\n%s", out)
		}
	})
}

//nolint:funlen,gocognit
func TestRunAutoFix(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("tidy failure modernize fail gofmt fail deadcode fail", func(t *testing.T) {
		runner := &mockRunner{
			errors: map[string]error{
				"go mod tidy": fmt.Errorf("tidy network error"),
				"modernize":   fmt.Errorf("exit status 1"),
				"gofmt":       fmt.Errorf("gofmt permission denied"),
				"deadcode":    fmt.Errorf("deadcode compile error"),
			},
			outputs: map[string]string{
				"modernize": "error detail on line 5",
			},
		}
		CommandRunner = runner

		var sb strings.Builder
		runAutoFix(context.Background(), "/workspace", "./...", &sb)
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
		if !strings.Contains(out, "❌ Deadcode Analysis: FAILED (deadcode compile error)") {
			t.Errorf("expected deadcode failed, got:\n%s", out)
		}
	})

	t.Run("lookpath fallbacks for modernize and deadcode with exit status 3 and unreachable code", func(t *testing.T) {
		runner := &mockRunner{
			lookPathErr: map[string]error{
				"modernize": fmt.Errorf("not found"),
				"deadcode":  fmt.Errorf("not found"),
			},
			errors: map[string]error{
				"passes/modernize/cmd/modernize@latest": fmt.Errorf("exit status 3: modernized 2 files"),
			},
			outputs: map[string]string{
				"cmd/deadcode@latest": "pkg/foo.go:10:2: unreachable func DeadFunc",
			},
		}
		CommandRunner = runner

		var sb strings.Builder
		runAutoFix(context.Background(), "/workspace", "./...", &sb)
		out := sb.String()

		if !strings.Contains(out, "✅ Go Mod Tidy: SUCCESS") {
			t.Errorf("expected mod tidy success, got:\n%s", out)
		}
		if !strings.Contains(out, "✅ Go Modernizer: SUCCESS (Issues found and auto-fixed)") {
			t.Errorf("expected modernize exit 3 success, got:\n%s", out)
		}
		if !strings.Contains(out, "✅ Go Code Formatter: SUCCESS") {
			t.Errorf("expected gofmt success, got:\n%s", out)
		}
		if !strings.Contains(out, "⚠️ Deadcode Analysis: Unreachable functions detected:") {
			t.Errorf("expected deadcode warning, got:\n%s", out)
		}
		if !strings.Contains(out, "DeadFunc") {
			t.Errorf("expected DeadFunc in deadcode output, got:\n%s", out)
		}

		calls := runner.getCalls()
		var foundModFallback, foundDeadFallback bool
		for _, c := range calls {
			str := c.String()
			if strings.Contains(str, "golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest") {
				foundModFallback = true
			}
			if strings.Contains(str, "golang.org/x/tools/cmd/deadcode@latest") {
				foundDeadFallback = true
			}
		}
		if !foundModFallback {
			t.Errorf("expected fallback command for modernize, calls were: %v", calls)
		}
		if !foundDeadFallback {
			t.Errorf("expected fallback command for deadcode, calls were: %v", calls)
		}
	})

	t.Run("clean deadcode and modernize no issues", func(t *testing.T) {
		runner := &mockRunner{
			outputs: map[string]string{
				"deadcode": "   \n\n  ",
			},
		}
		CommandRunner = runner

		var sb strings.Builder
		runAutoFix(context.Background(), "/workspace", "./...", &sb)
		out := sb.String()

		if !strings.Contains(out, "✅ Go Modernizer: SUCCESS (No issues found)") {
			t.Errorf("expected modernize no issues, got:\n%s", out)
		}
		if !strings.Contains(out, "✅ Deadcode Analysis: SUCCESS (No unreachable code found)") {
			t.Errorf("expected deadcode clean success, got:\n%s", out)
		}
	})
}

//nolint:funlen
func TestSyncTestQueryDB(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("testquery in LookPath", func(t *testing.T) {
		runner := &mockRunner{
			lookPathMap: map[string]string{
				"testquery": "/usr/local/bin/testquery",
			},
		}
		CommandRunner = runner

		syncTestQueryDB(context.Background(), "/workspace", "./...")

		calls := runner.getCalls()
		if len(calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(calls))
		}
		if calls[0].Name != "testquery" || strings.Join(calls[0].Args, " ") != "build --pkg ./... --output testquery.db" {
			t.Errorf("unexpected command call: %v", calls[0])
		}
	})

	t.Run("tq in LookPath", func(t *testing.T) {
		runner := &mockRunner{
			lookPathErr: map[string]error{
				"testquery": fmt.Errorf("not found"),
			},
			lookPathMap: map[string]string{
				"tq": "/usr/local/bin/tq",
			},
		}
		CommandRunner = runner

		syncTestQueryDB(context.Background(), "/workspace", "./pkg/...")

		calls := runner.getCalls()
		if len(calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(calls))
		}
		if calls[0].Name != "tq" || strings.Join(calls[0].Args, " ") != "build --pkg ./pkg/... --output testquery.db" {
			t.Errorf("unexpected command call: %v", calls[0])
		}
	})

	t.Run("neither in LookPath fallback to go run", func(t *testing.T) {
		runner := &mockRunner{
			lookPathErr: map[string]error{
				"testquery": fmt.Errorf("not found"),
				"tq":        fmt.Errorf("not found"),
			},
		}
		CommandRunner = runner

		syncTestQueryDB(context.Background(), "/workspace", "./...")

		calls := runner.getCalls()
		if len(calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(calls))
		}
		expectedArgs := "run github.com/danicat/testquery@latest build --pkg ./... --output testquery.db"
		if calls[0].Name != "go" || strings.Join(calls[0].Args, " ") != expectedArgs {
			t.Errorf("unexpected fallback command call: %v", calls[0])
		}
	})
}

//nolint:funlen
func TestParseTotalCoverage(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	tests := []struct {
		name     string
		output   string
		err      error
		expected string
	}{
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
			name:     "total prefix but fewer than 3 fields",
			output:   "total: 100%",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &mockRunner{
				outputs: map[string]string{
					"go tool cover": tc.output,
				},
			}
			if tc.err != nil {
				runner.errors = map[string]error{
					"go tool cover": tc.err,
				}
			}
			CommandRunner = runner

			got := parseTotalCoverage(context.Background(), "/workspace", "coverage.out")
			if got != tc.expected {
				t.Errorf("parseTotalCoverage() = %q, want %q", got, tc.expected)
			}
		})
	}
}

//nolint:funlen
func TestParsePackagesCoverage(t *testing.T) {
	tests := []struct {
		name     string
		testOut  string
		contains []string
		omits    []string
	}{
		{
			name: "valid package coverage lines",
			testOut: strings.Join([]string{
				"=== RUN   TestFoo",
				"--- PASS: TestFoo (0.01s)",
				"PASS",
				"coverage: 85.5% of statements",
				"ok  \tgithub.com/danicat/godoctor/pkg1\t0.05s\tcoverage: 85.5% of statements",
				"ok  \tgithub.com/danicat/godoctor/pkg2\t0.12s\tcoverage: 92.0% of statements",
			}, "\n"),
			contains: []string{
				"- **Packages**:",
				"  - `github.com/danicat/godoctor/pkg1`: 85.5%",
				"  - `github.com/danicat/godoctor/pkg2`: 92.0%",
			},
			omits: []string{},
		},
		{
			name: "skips 0.0 coverage and no test files and duplicate packages",
			testOut: strings.Join([]string{
				"ok  \tgithub.com/danicat/godoctor/pkg0\t0.01s\tcoverage: 0.0% of statements",
				"ok  \tgithub.com/danicat/godoctor/pkgno\t0.01s\tcoverage: [no test files]",
				"ok  \tgithub.com/danicat/godoctor/pkg1\t0.05s\tcoverage: 70.0% of statements",
				"ok  \tgithub.com/danicat/godoctor/pkg1\t0.05s\tcoverage: 70.0% of statements",
			}, "\n"),
			contains: []string{
				"- **Packages**:",
				"  - `github.com/danicat/godoctor/pkg1`: 70.0%",
			},
			omits: []string{
				"pkg0",
				"pkgno",
			},
		},
		{
			name: "skips lines with coverage in pkg position or without ok status",
			testOut: strings.Join([]string{
				"FAIL\tgithub.com/danicat/godoctor/pkgfail\t0.01s\tcoverage: 50.0% of statements",
				"ok  \tcoverage: 100.0% of statements",
				"ok  \tcoverage_foo\t0.01s\tcoverage: 80.0% of statements",
				"ok  \tpkgshort",
				"ok  \tpkgend\t0.01s\tcoverage:",
			}, "\n"),
			contains: []string{},
			omits: []string{
				"- **Packages**:",
				"pkgfail",
				"coverage_foo",
				"pkgshort",
				"pkgend",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var sb strings.Builder
			parsePackagesCoverage(tc.testOut, &sb)
			out := sb.String()

			for _, sub := range tc.contains {
				if !strings.Contains(out, sub) {
					t.Errorf("expected output to contain %q, got:\n%s", sub, out)
				}
			}
			for _, sub := range tc.omits {
				if strings.Contains(out, sub) {
					t.Errorf("expected output to NOT contain %q, got:\n%s", sub, out)
				}
			}
		})
	}
}

func TestFindConfigFile(t *testing.T) {
	configs := []string{
		".golangci.yml",
		".golangci.yaml",
		".golangci.toml",
		".golangci.json",
	}

	for _, cfg := range configs {
		t.Run("finds "+cfg, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, cfg)
			if err := os.WriteFile(target, []byte("version: 2\n"), 0o600); err != nil {
				t.Fatalf("failed to create %s: %v", cfg, err)
			}

			got := findConfigFile(dir)
			if got != target {
				t.Errorf("findConfigFile(%q) = %q, want %q", dir, got, target)
			}
		})
	}

	t.Run("finds config in parent repo root directory", func(t *testing.T) {
		rootDir := t.TempDir()
		subDir := filepath.Join(rootDir, "internal", "mypkg")
		if err := os.MkdirAll(subDir, 0o750); err != nil {
			t.Fatalf("failed to create subdir: %v", err)
		}
		cfgPath := filepath.Join(rootDir, ".golangci.yaml")
		if err := os.WriteFile(cfgPath, []byte("version: 2\n"), 0o600); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}
		if err := os.WriteFile(filepath.Join(rootDir, "go.mod"), []byte("module example.com/pkg\n"), 0o600); err != nil {
			t.Fatalf("failed to write go.mod: %v", err)
		}

		got := findConfigFile(subDir)
		if got != cfgPath {
			t.Errorf("findConfigFile(%s) = %q, want %q", subDir, got, cfgPath)
		}
	})

	t.Run("none found in empty dir", func(t *testing.T) {
		dir := t.TempDir()
		got := findConfigFile(dir)
		if got != "" {
			t.Errorf("findConfigFile(%q) = %q, want empty string", dir, got)
		}
	})
}

//nolint:funlen
func TestRunLinterPhase(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("no config file fallback to go vet pass", func(t *testing.T) {
		dir := t.TempDir()
		runner := &mockRunner{
			outputs: map[string]string{
				"go vet": "",
			},
		}
		CommandRunner = runner

		var sb strings.Builder
		err := runLinterPhase(context.Background(), dir, "./...", &sb)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		out := sb.String()
		if !strings.Contains(out, "(using `go vet`) ✅ PASS") {
			t.Errorf("expected go vet pass, got:\n%s", out)
		}
	})

	t.Run("no config file fallback to go vet fail", func(t *testing.T) {
		dir := t.TempDir()
		runner := &mockRunner{
			outputs: map[string]string{
				"go vet": "main.go:5:2: unreachable code",
			},
			errors: map[string]error{
				"go vet": fmt.Errorf("exit status 1"),
			},
		}
		CommandRunner = runner

		var sb strings.Builder
		err := runLinterPhase(context.Background(), dir, "./...", &sb)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		out := sb.String()
		if !strings.Contains(out, "(using `go vet`) ⚠️ ISSUES FOUND") {
			t.Errorf("expected go vet issues found, got:\n%s", out)
		}
		if !strings.Contains(out, "unreachable code") {
			t.Errorf("expected vet error message, got:\n%s", out)
		}
	})

	t.Run("config present with local golangci-lint pass", func(t *testing.T) {
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

		var sb strings.Builder
		err := runLinterPhase(context.Background(), dir, "./...", &sb)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		out := sb.String()
		if !strings.Contains(out, "(using local `golangci-lint`) ✅ PASS") {
			t.Errorf("expected local golangci-lint pass, got:\n%s", out)
		}
	})

	t.Run("config present with local golangci-lint fail", func(t *testing.T) {
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

		var sb strings.Builder
		err := runLinterPhase(context.Background(), dir, "./...", &sb)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		out := sb.String()
		if !strings.Contains(out, "(using local `golangci-lint`) ⚠️ ISSUES FOUND") {
			t.Errorf("expected lint issues found, got:\n%s", out)
		}
	})

	t.Run("config present without local golangci-lint fallback to v2.12.2", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, ".golangci.yml")
		_ = os.WriteFile(cfgPath, []byte("version: 2\n"), 0o600)

		runner := &mockRunner{
			lookPathErr: map[string]error{
				"golangci-lint": fmt.Errorf("not found"),
			},
			outputs: map[string]string{
				"golangci-lint@v2.12.2": "",
			},
		}
		CommandRunner = runner

		var sb strings.Builder
		err := runLinterPhase(context.Background(), dir, "./...", &sb)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		out := sb.String()
		if !strings.Contains(out, "(using `golangci-lint v2.12.2`) ✅ PASS") {
			t.Errorf("expected golangci-lint v2.12.2 pass, got:\n%s", out)
		}
	})
}

//nolint:funlen
func TestRunTestsPhase(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("tests pass with coverage and testquery sync", func(t *testing.T) {
		runner := &mockRunner{
			outputs: map[string]string{
				"go test":       "ok  \tgithub.com/danicat/godoctor/pkg\t0.02s\tcoverage: 80.0% of statements",
				"go tool cover": "total: (statements) 80.0%",
			},
		}
		CommandRunner = runner

		var sb strings.Builder
		err := runTestsPhase(context.Background(), "/workspace", "./...", &sb)
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
		if !strings.Contains(out, "*Indexed test run to `testquery.db`*") {
			t.Errorf("expected testquery indexing notice, got:\n%s", out)
		}
	})

	t.Run("tests fail with hint and testquery sync", func(t *testing.T) {
		runner := &mockRunner{
			outputs: map[string]string{
				"go test": "--- FAIL: TestBroken (0.01s)\n    broken_test.go:10: assert failed",
			},
			errors: map[string]error{
				"go test": fmt.Errorf("exit status 1"),
			},
		}
		CommandRunner = runner

		var sb strings.Builder
		err := runTestsPhase(context.Background(), "/workspace", "./...", &sb)
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
	})
}

//nolint:funlen
func TestHandler_Phases(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("tests phase failure aborts pipeline", func(t *testing.T) {
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

		res, _, err := Handler(context.Background(), nil, Params{
			Dir: "/workspace",
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
	})

	t.Run("linter phase failure aborts pipeline", func(t *testing.T) {
		runner := &mockRunner{
			outputs: map[string]string{
				"go build": "",
				"go test":  "PASS",
				"go vet":   "vet warning",
			},
			errors: map[string]error{
				"go vet": fmt.Errorf("exit status 1"),
			},
		}
		CommandRunner = runner

		res, _, err := Handler(context.Background(), nil, Params{
			Dir:      "/workspace",
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
		if !strings.Contains(out, "🧹 Lint: (using `go vet`) ⚠️ ISSUES FOUND") {
			t.Errorf("expected lint failure in report, got:\n%s", out)
		}
	})
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
