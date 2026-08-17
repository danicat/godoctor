package selene

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/danicat/godoctor/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	testAbsPath = "/absolute/path"
	pkg1Pattern = "./pkg1/..."
	pkg2Pattern = "./pkg2/..."
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

func TestHandler_RelativePathRejected(t *testing.T) {
	testCases := []struct {
		name string
		dir  string
	}{
		{"empty dir", ""},
		{"dot dir", "."},
		{"relative subpath", "./pkg"},
		{"relative plain", "pkg"},
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
				t.Errorf("expected error result for dir %q", tc.dir)
			}
			text := res.Content[0].(*mcp.TextContent).Text
			if !strings.Contains(text, "dir is required and must be an absolute path") {
				t.Errorf("expected absolute path error message, got: %s", text)
			}
		})
	}
}

func TestHandler_ToolPathResolution(t *testing.T) {
	tests := []struct {
		name        string
		lookPaths   map[string]string
		wantErr     bool
		expectedErr string
		expectedCmd string
	}{
		{
			name: "selene in PATH",
			lookPaths: map[string]string{
				toolName: "/usr/bin/selene",
			},
			expectedCmd: fmt.Sprintf("selene -workers %d --db .godoctor/testquery.db ./...", runtime.GOMAXPROCS(0)),
		},
		{
			name: "selene not in PATH returns clear error",
			lookPaths: map[string]string{
				toolName: "",
			},
			wantErr:     true,
			expectedErr: `selene binary ("selene") not found in PATH`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldRunner := CommandRunner
			defer func() { CommandRunner = oldRunner }()

			mock := &mockRunner{
				outputs: map[string]string{
					toolName: "mutations checked",
				},
				lookPaths: tc.lookPaths,
			}
			CommandRunner = mock

			res, _, err := Handler(context.Background(), nil, Params{
				Dir: testAbsPath,
			})
			if err != nil {
				t.Fatalf("unexpected handler error: %v", err)
			}
			if tc.wantErr {
				if !res.IsError {
					t.Fatalf("expected error result, got success")
				}
				text := res.Content[0].(*mcp.TextContent).Text
				if !strings.Contains(text, tc.expectedErr) {
					t.Errorf("expected error message to contain %q, got %q", tc.expectedErr, text)
				}
				return
			}
			if res.IsError {
				t.Fatalf("expected success result")
			}
			if len(mock.calls) == 0 || mock.calls[0] != tc.expectedCmd {
				t.Errorf("expected cmd %q, got %q", tc.expectedCmd, mock.calls[0])
			}
		})
	}
}

func TestHandler_Success_EmptyOutput(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			toolName: "",
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir: testAbsPath,
	})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success result")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "✅ All mutations were caught by tests.") {
		t.Errorf("expected all mutations caught message, got: %s", text)
	}
}

func TestHandler_Success_WithOutput(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			toolName: "100% mutation score: 15/15 killed",
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir: testAbsPath,
	})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success result")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "✅ Mutation testing results:") {
		t.Errorf("expected header, got: %s", text)
	}
	if !strings.Contains(text, "100% mutation score") {
		t.Errorf("expected output content, got: %s", text)
	}
}

func TestHandler_Failure_EmptyOutput(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			toolName: "",
		},
		errors: map[string]error{
			toolName: fmt.Errorf("command not found"),
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir: testAbsPath,
	})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "mutation testing failed to run: command not found") {
		t.Errorf("expected failure message, got: %s", text)
	}
}

func TestHandler_Failure_SurvivingMutants(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			toolName: "Mutant survived: pkg/calc.go:25: replaced + with -",
		},
		errors: map[string]error{
			toolName: fmt.Errorf("exit status 1"),
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir: testAbsPath,
	})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true when mutants survive")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "🧬 Mutation testing results:") {
		t.Errorf("expected mutant results header, got: %s", text)
	}
	if !strings.Contains(text, "Mutant survived: pkg/calc.go:25") {
		t.Errorf("expected surviving mutant details, got: %s", text)
	}
}

func TestHandler_PackagesParameter(t *testing.T) {
	tests := []struct {
		name        string
		packages    string
		lookPaths   map[string]string
		expectedCmd string
	}{
		{
			name:     "custom packages with local binary",
			packages: "./pkg1/..., ./pkg2/...",
			lookPaths: map[string]string{
				toolName: "/usr/bin/selene",
			},
			expectedCmd: fmt.Sprintf("selene -workers %d --db .godoctor/testquery.db ./pkg1/... ./pkg2/...", runtime.GOMAXPROCS(0)),
		},
		{
			name:     "single custom package with local binary",
			packages: "./pkg/calc",
			lookPaths: map[string]string{
				toolName: "/usr/bin/selene",
			},
			expectedCmd: fmt.Sprintf("selene -workers %d --db .godoctor/testquery.db ./pkg/calc", runtime.GOMAXPROCS(0)),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldRunner := CommandRunner
			defer func() { CommandRunner = oldRunner }()

			mock := &mockRunner{
				outputs: map[string]string{
					toolName: "mutations checked",
				},
				lookPaths: tc.lookPaths,
			}
			CommandRunner = mock

			res, _, err := Handler(context.Background(), nil, Params{
				Dir:      testAbsPath,
				Packages: tc.packages,
			})
			if err != nil {
				t.Fatalf("unexpected handler error: %v", err)
			}
			if res.IsError {
				t.Fatalf("expected success result")
			}
			if len(mock.calls) == 0 || mock.calls[0] != tc.expectedCmd {
				t.Errorf("expected cmd %q, got %q", tc.expectedCmd, mock.calls[0])
			}
		})
	}
}

func TestBuildArgs(t *testing.T) {
	t.Run("default configuration uses GOMAXPROCS and testquery.db", func(t *testing.T) {
		cfg := config.NewDefaultConfig()
		args := BuildArgs("/workspace", []string{"./..."}, cfg)
		expected := []string{
			"-workers", fmt.Sprintf("%d", runtime.GOMAXPROCS(0)),
			"--db", ".godoctor/testquery.db",
			"./...",
		}
		if len(args) != len(expected) {
			t.Fatalf("expected %v, got %v", expected, args)
		}
		for i := range expected {
			if args[i] != expected[i] {
				t.Errorf("arg[%d]: expected %q, got %q", i, expected[i], args[i])
			}
		}
	})

	t.Run("custom workers and db path configured", func(t *testing.T) {
		cfg := config.NewDefaultConfig()
		cfg.Tools.Selene.Workers = 4
		cfg.Tools.Selene.DbPath = "custom/test.db"
		args := BuildArgs("/workspace", []string{"./pkg"}, cfg)
		expected := []string{
			"-workers", "4",
			"--db", "custom/test.db",
			"./pkg",
		}
		if strings.Join(args, " ") != strings.Join(expected, " ") {
			t.Errorf("expected %v, got %v", expected, args)
		}
	})

	t.Run("workers already present in custom args", func(t *testing.T) {
		cfg := config.NewDefaultConfig()
		cfg.Tools.Selene.Args = []string{"-workers", "2"}
		args := BuildArgs("/workspace", []string{"./..."}, cfg)
		// Should not duplicate -workers
		if strings.Count(strings.Join(args, " "), "-workers") != 1 {
			t.Errorf("expected single -workers flag, got %v", args)
		}
		if !strings.Contains(strings.Join(args, " "), "-workers 2") {
			t.Errorf("expected -workers 2, got %v", args)
		}
	})

	t.Run("disabled testquery compatibility", func(t *testing.T) {
		cfg := config.NewDefaultConfig()
		cfg.Tools.Selene.TestQueryCompat = false
		cfg.Features.TestQueryCompat = false
		args := BuildArgs("/workspace", []string{"./..."}, cfg)
		if strings.Contains(strings.Join(args, " "), "--db") {
			t.Errorf("expected no --db flag when testquery compat disabled, got %v", args)
		}
	})
}

func TestParsePackages(t *testing.T) {
	cases := []struct {
		input    string
		expected []string
	}{
		{"", []string{defaultPackages}},
		{"   ", []string{defaultPackages}},
		{pkg1Pattern, []string{pkg1Pattern}},
		{"./pkg1/...,./pkg2/...", []string{pkg1Pattern, pkg2Pattern}},
		{"./pkg1/... ./pkg2/...", []string{pkg1Pattern, pkg2Pattern}},
	}

	for _, tc := range cases {
		got := parsePackages(tc.input)
		if len(got) != len(tc.expected) {
			t.Fatalf("for %q: expected len %d, got %d (%v)", tc.input, len(tc.expected), len(got), got)
		}
		for i := range got {
			if got[i] != tc.expected[i] {
				t.Errorf("for %q at %d: expected %q, got %q", tc.input, i, tc.expected[i], got[i])
			}
		}
	}
}

func TestFilterNoise(t *testing.T) {
	input := `go: downloading github.com/danicat/selene v0.1.0
mutations generated: 12
mutations killed: 12
exit status 1
summary: all tests passed
assertion failure: expected exit status 0, got 1`

	expected := "mutations generated: 12\nmutations killed: 12\nsummary: all tests passed\n" +
		"assertion failure: expected exit status 0, got 1"
	got := filterNoise(input)
	if got != expected {
		t.Errorf("expected filtered noise:\n%s\ngot:\n%s", expected, got)
	}
}

func TestErrorResult(t *testing.T) {
	msg := "test error message"
	res := errorResult(msg)
	if !res.IsError {
		t.Error("expected IsError=true")
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected 1 content element, got %d", len(res.Content))
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if text != msg {
		t.Errorf("expected %q, got %q", msg, text)
	}
}
