package smarttest

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mockRunner struct {
	outputs   map[string]string
	errors    map[string]error
	lookPaths map[string]string
	calls     []string
}

func (m *mockRunner) Run(_ context.Context, _, name string, args ...string) error {
	cmd := name + " " + strings.Join(args, " ")
	m.calls = append(m.calls, cmd)
	for k, v := range m.errors {
		if strings.Contains(cmd, k) {
			return v
		}
	}
	return nil
}

func (m *mockRunner) RunWithOutput(_ context.Context, _, name string, args ...string) (string, error) {
	cmd := name + " " + strings.Join(args, " ")
	m.calls = append(m.calls, cmd)
	output := ""
	for k, v := range m.outputs {
		if strings.Contains(cmd, k) {
			output = v
		}
	}
	var err error
	for k, v := range m.errors {
		if strings.Contains(cmd, k) {
			err = v
		}
	}
	return output, err
}

func (m *mockRunner) LookPath(file string) (string, error) {
	if p, ok := m.lookPaths[file]; ok {
		if p == "" {
			return "", fmt.Errorf("executable not found: %s", file)
		}
		return p, nil
	}
	return "/usr/local/bin/" + file, nil
}

func TestRegister(_ *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test-server",
		Version: "1.0.0",
	}, nil)
	Register(server)
}

func TestSmartTest_RelativePathRejected(t *testing.T) {
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
				Dir:   tc.dir,
				Level: "fast",
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

func TestSmartTest_FastLevel(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"go test -v": "=== RUN   TestSample\n--- PASS: TestSample (0.00s)\n" +
				"PASS\nok  \tgithub.com/danicat/godoctor/pkg\t0.010s",
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir:      "/path/to/workspace",
		Packages: "./...",
		Level:    "fast",
		Run:      "TestSample",
	})

	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success result, got error")
	}

	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "⚡ Fast Test Pass") {
		t.Errorf("expected Fast Test Pass in output, got:\n%s", text)
	}
	if !strings.Contains(text, "✅ **All Tests Passed**") {
		t.Errorf("expected All Tests Passed in output, got:\n%s", text)
	}

	sawRunFilter := false
	for _, call := range mock.calls {
		if strings.Contains(call, "-run=TestSample") {
			sawRunFilter = true
		}
	}
	if !sawRunFilter {
		t.Error("expected -run=TestSample in command invocation")
	}
}

func TestSmartTest_FastLevel_Failure(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"go test -v": "=== RUN   TestFail\n--- FAIL: TestFail (0.00s)\n" +
				"    fail_test.go:10: assertion failed\nFAIL\nFAIL\tgithub.com/danicat/godoctor/pkg\t0.010s",
		},
		errors: map[string]error{
			"go test -v": fmt.Errorf("exit status 1"),
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir:   "/path/to/workspace",
		Level: "fast",
	})

	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error result on test failure")
	}

	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "❌ **Tests Failed**") {
		t.Errorf("expected Tests Failed header, got:\n%s", text)
	}
	if !strings.Contains(text, "--- FAIL: TestFail") {
		t.Errorf("expected failure output details, got:\n%s", text)
	}
}

func TestSmartTest_BasicLevel_Success(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"go test -v -coverprofile": "=== RUN   TestOK\n--- PASS: TestOK (0.00s)\n" +
				"PASS\nok  \tgithub.com/danicat/godoctor/pkg\t0.010s\tcoverage: 85.0% of statements",
			"go tool cover -func": "github.com/danicat/godoctor/pkg/file.go:10: Func 85.0%\ntotal:\t(statements)\t85.0%",
			"build":               "built testquery db",
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir:   "/path/to/workspace",
		Level: "basic",
	})

	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success result")
	}

	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "🧪 Test & Coverage Analysis") {
		t.Errorf("expected Test & Coverage Analysis, got:\n%s", text)
	}
	if !strings.Contains(text, "* **Total Coverage**: `85.0%`") {
		t.Errorf("expected Total Coverage 85.0%%, got:\n%s", text)
	}
	if !strings.Contains(text, "`github.com/danicat/godoctor/pkg`: `85.0%`") {
		t.Errorf("expected package coverage detail, got:\n%s", text)
	}
	if !strings.Contains(text, "Indexed test run to `testquery.db`") {
		t.Errorf("expected testquery indexing note, got:\n%s", text)
	}
}

func TestSmartTest_BasicLevel_Failure(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"go test -v -coverprofile": "=== RUN   TestFail\n--- FAIL: TestFail (0.01s)\nFAIL",
		},
		errors: map[string]error{
			"go test -v -coverprofile": fmt.Errorf("exit status 1"),
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir: "/path/to/workspace",
		// Default level should be basic
	})

	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error result on test failure")
	}

	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "❌ **Tests Failed**") {
		t.Errorf("expected Tests Failed in output, got:\n%s", text)
	}

	sawSync := false
	for _, call := range mock.calls {
		if strings.Contains(call, "build") {
			sawSync = true
		}
	}
	if !sawSync {
		t.Error("expected syncTestQueryDB to be called on test failure")
	}
}

func TestSmartTest_BenchmarkLevel_Success_Table(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"go test -bench": "goos: darwin\nBenchmarkSample-8   1000000   1050 ns/op   128 B/op   2 allocs/op\nPASS",
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir:   "/path/to/workspace",
		Level: "benchmark",
		Run:   "BenchmarkSample",
	})

	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success result")
	}

	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "🚀 Benchmark Results") {
		t.Errorf("expected Benchmark Results header, got:\n%s", text)
	}
	if !strings.Contains(text, "| `BenchmarkSample-8` | 1000000 | `1050 ns/op` | `128 B/op` | `2 allocs/op` |") {
		t.Errorf("expected markdown benchmark table, got:\n%s", text)
	}
}

func TestSmartTest_BenchmarkLevel_Success_Fallback(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"go test -bench": "no standard benchmark rows\nPASS",
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir:   "/path/to/workspace",
		Level: "benchmark",
	})

	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success result")
	}

	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "no standard benchmark rows") {
		t.Errorf("expected raw fallback output, got:\n%s", text)
	}
}

func TestSmartTest_BenchmarkLevel_Failure(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"go test -bench": "benchmark panic or compilation error",
		},
		errors: map[string]error{
			"go test -bench": fmt.Errorf("exit status 1"),
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir:   "/path/to/workspace",
		Level: "benchmark",
	})

	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true on benchmark failure")
	}

	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "❌ **Benchmark Execution Failed**") {
		t.Errorf("expected benchmark failed header, got:\n%s", text)
	}
}

func TestSmartTest_CompleteLevel_Pass(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"go test -v -coverprofile": "PASS\nok  \tpkg\t0.010s\tcoverage: 90.0% of statements",
			"go tool cover -func":      "total:\t(statements)\t90.0%",
			"selene":                   "",
		},
		lookPaths: map[string]string{
			"selene": "/usr/local/bin/selene",
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir:   "/path/to/workspace",
		Level: "complete",
	})

	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success result")
	}

	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "🧬 Selene AST Mutation Testing") {
		t.Errorf("expected Selene section, got:\n%s", text)
	}
	if !strings.Contains(text, "✅ **All Mutations Caught by Test Suite**") {
		t.Errorf("expected all mutations caught, got:\n%s", text)
	}
}

func TestSmartTest_CompleteLevel_MutantsSurvive(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"go test -v -coverprofile": "PASS\nok  \tpkg\t0.010s\tcoverage: 90.0% of statements",
			"go tool cover -func":      "total:\t(statements)\t90.0%",
			"selene":                   "Mutant survived at calc.go:12",
		},
		errors: map[string]error{
			"selene": fmt.Errorf("exit status 1"),
		},
		lookPaths: map[string]string{
			"selene": "/usr/local/bin/selene",
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir:   "/path/to/workspace",
		Level: "complete",
	})

	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "⚠️ **Surviving Mutants Detected**:") {
		t.Errorf("expected surviving mutants warning, got:\n%s", text)
	}
	if !strings.Contains(text, "Mutant survived at calc.go:12") {
		t.Errorf("expected mutant output, got:\n%s", text)
	}
}

func TestSmartTest_CompleteLevel_TestFailure_SkipsMutation(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"go test -v -coverprofile": "--- FAIL: TestA\nFAIL",
		},
		errors: map[string]error{
			"go test -v -coverprofile": fmt.Errorf("exit status 1"),
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir:   "/path/to/workspace",
		Level: "complete",
	})

	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result")
	}

	text := res.Content[0].(*mcp.TextContent).Text
	if strings.Contains(text, "Selene AST Mutation Testing") {
		t.Error("expected mutation testing to be skipped on test failure")
	}
}

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
			name:     "valid total",
			output:   "pkg/file.go:1: fn 100.0%\ntotal:\t(statements)\t78.9%",
			expected: "78.9%",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
		{
			name:     "missing total line",
			output:   "pkg/file.go:1: fn 100.0%\nother text",
			expected: "",
		},
		{
			name:     "short total line",
			output:   "total: 50%",
			expected: "",
		},
		{
			name:     "cover tool error",
			err:      fmt.Errorf("file not found"),
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockRunner{}
			if tc.err != nil {
				mock.errors = map[string]error{"go tool cover -func": tc.err}
			} else {
				mock.outputs = map[string]string{"go tool cover -func": tc.output}
			}
			CommandRunner = mock
			cov := parseTotalCoverage(context.Background(), "/dir", "coverage.out")
			if cov != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, cov)
			}
		})
	}
}

func TestParsePackageCoverage(t *testing.T) {
	testOutput := `=== RUN   TestA
--- PASS: TestA (0.01s)
PASS
ok  	github.com/danicat/godoctor/pkg1	0.012s	coverage: 85.0% of statements
ok  	github.com/danicat/godoctor/pkg2	0.015s	coverage: 0.0% of statements
ok  	github.com/danicat/godoctor/pkg3	0.002s	coverage: [no test files]
ok  	github.com/danicat/godoctor/pkg1	0.012s	coverage: 85.0% of statements
ok  	github.com/danicat/godoctor/pkg4	0.020s	coverage: 92.5% of statements
FAIL	github.com/danicat/godoctor/pkg5	0.010s	coverage: 50.0% of statements`

	var sb strings.Builder
	parsePackageCoverage(testOutput, &sb)
	result := sb.String()

	if !strings.Contains(result, "* **Package Details**:") {
		t.Errorf("expected Package Details header, got:\n%s", result)
	}
	if !strings.Contains(result, "  * `github.com/danicat/godoctor/pkg1`: `85.0%`") {
		t.Errorf("expected pkg1 coverage in output, got:\n%s", result)
	}
	if !strings.Contains(result, "  * `github.com/danicat/godoctor/pkg4`: `92.5%`") {
		t.Errorf("expected pkg4 coverage in output, got:\n%s", result)
	}
	if strings.Contains(result, "pkg2") {
		t.Error("expected pkg2 (0.0%) to be omitted")
	}
	if strings.Contains(result, "pkg3") {
		t.Error("expected pkg3 ([no test files]) to be omitted")
	}
	if strings.Contains(result, "pkg5") {
		t.Error("expected pkg5 (FAIL) to be omitted")
	}
	if strings.Count(result, "pkg1") != 1 {
		t.Errorf("expected duplicate pkg1 to be omitted, got count: %d", strings.Count(result, "pkg1"))
	}
}

func TestSyncTestQueryDB_ToolSelection(t *testing.T) {
	tests := []struct {
		name        string
		lookPaths   map[string]string
		expectedCmd string
	}{
		{
			name: "testquery in PATH",
			lookPaths: map[string]string{
				"testquery": "/usr/bin/testquery",
			},
			expectedCmd: "testquery build --pkg ./... --output testquery.db",
		},
		{
			name: "tq in PATH",
			lookPaths: map[string]string{
				"testquery": "",
				"tq":        "/usr/bin/tq",
			},
			expectedCmd: "tq build --pkg ./... --output testquery.db",
		},
		{
			name: "fallback to go run",
			lookPaths: map[string]string{
				"testquery": "",
				"tq":        "",
			},
			expectedCmd: "go run github.com/danicat/testquery@latest build --pkg ./... --output testquery.db",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldRunner := CommandRunner
			defer func() { CommandRunner = oldRunner }()

			mock := &mockRunner{
				outputs:   map[string]string{},
				lookPaths: tc.lookPaths,
			}
			CommandRunner = mock

			syncTestQueryDB(context.Background(), "/workspace", "./...")

			if len(mock.calls) == 0 || mock.calls[0] != tc.expectedCmd {
				t.Errorf("expected cmd %q, got %q", tc.expectedCmd, mock.calls[0])
			}
		})
	}
}

func TestFormatBenchmarkTable(t *testing.T) {
	raw := `goos: darwin
goarch: arm64
pkg: github.com/danicat/godoctor/internal/text
BenchmarkFormat-12    	 1000000	      1052 ns/op	     128 B/op	       4 allocs/op
PASS
ok  	github.com/danicat/godoctor/internal/text	1.234s`

	table := formatBenchmarkTable(raw)
	if !strings.Contains(table, "`BenchmarkFormat-12`") {
		t.Errorf("expected BenchmarkFormat-12 in formatted table, got:\n%s", table)
	}
	if !strings.Contains(table, "1052 ns/op") {
		t.Errorf("expected 1052 ns/op in formatted table, got:\n%s", table)
	}
}

func TestFormatFailures(t *testing.T) {
	t.Run("with fail block", func(t *testing.T) {
		out := "=== RUN TestX\n--- FAIL: TestX (0.01s)\n    x_test.go:5: fail msg\n\nPASS"
		formatted := formatFailures(out)
		if !strings.Contains(formatted, "--- FAIL: TestX") {
			t.Errorf("expected fail block, got:\n%s", formatted)
		}
	})

	t.Run("without fail block", func(t *testing.T) {
		out := "some random failure"
		formatted := formatFailures(out)
		if !strings.Contains(formatted, "some random failure") {
			t.Errorf("expected fallback output, got:\n%s", formatted)
		}
	})
}

func TestFormatTestSummary(t *testing.T) {
	out := "ok  \tpkg1\t0.01s\nFAIL\tpkg2\t0.02s\nPASS\nsome extra line"
	summary := formatTestSummary(out)
	if !strings.Contains(summary, "- ok  \tpkg1\t0.01s") {
		t.Errorf("expected ok in summary, got:\n%s", summary)
	}
	if !strings.Contains(summary, "- FAIL\tpkg2\t0.02s") {
		t.Errorf("expected FAIL in summary, got:\n%s", summary)
	}
	if !strings.Contains(summary, "- PASS") {
		t.Errorf("expected PASS in summary, got:\n%s", summary)
	}
	if strings.Contains(summary, "some extra line") {
		t.Errorf("expected extra line omitted from summary, got:\n%s", summary)
	}

	emptySummary := formatTestSummary("unrelated text only")
	if emptySummary != "" {
		t.Errorf("expected empty string summary, got %q", emptySummary)
	}
}

func TestFilterNoise(t *testing.T) {
	input := "go: downloading github.com/foo/bar v1.0\nnormal line\nexit status 1\nanother line"
	expected := "normal line\nanother line"
	got := filterNoise(input)
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatOutput(t *testing.T) {
	if formatOutput("") != "" {
		t.Error("expected empty string for empty input")
	}
	out := formatOutput("test text")
	if !strings.Contains(out, "```text\ntest text\n```") {
		t.Errorf("expected wrapped text block, got %q", out)
	}
}

func TestResult(t *testing.T) {
	res := result("test content", true)
	if !res.IsError {
		t.Error("expected IsError=true")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if text != "test content" {
		t.Errorf("expected 'test content', got %q", text)
	}
}

func TestParsePackages(t *testing.T) {
	cases := []struct {
		input    string
		expected []string
	}{
		{"", []string{"./..."}},
		{"   ", []string{"./..."}},
		{"./pkg1/...", []string{"./pkg1/..."}},
		{"./pkg1/...,./pkg2/...", []string{"./pkg1/...", "./pkg2/..."}},
		{"./pkg1/... ./pkg2/...", []string{"./pkg1/...", "./pkg2/..."}},
		{"./pkg1/..., ./pkg2/... , ./pkg3/...", []string{"./pkg1/...", "./pkg2/...", "./pkg3/..."}},
	}

	for _, tc := range cases {
		got := parsePackages(tc.input)
		if len(got) != len(tc.expected) {
			t.Fatalf("for input %q, expected len %d, got %d (%v)", tc.input, len(tc.expected), len(got), got)
		}
		for i := range got {
			if got[i] != tc.expected[i] {
				t.Errorf("for input %q at %d: expected %q, got %q", tc.input, i, tc.expected[i], got[i])
			}
		}
	}
}
