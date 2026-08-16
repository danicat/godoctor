package selene

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
		expectedCmd string
	}{
		{
			name: "selene in PATH",
			lookPaths: map[string]string{
				"selene": "/usr/bin/selene",
			},
			expectedCmd: "selene ./...",
		},
		{
			name: "selene not in PATH fallback to go run",
			lookPaths: map[string]string{
				"selene": "",
			},
			expectedCmd: "go run github.com/danicat/selene/cmd/selene@latest ./...",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldRunner := CommandRunner
			defer func() { CommandRunner = oldRunner }()

			mock := &mockRunner{
				outputs: map[string]string{
					"selene": "mutations checked",
				},
				lookPaths: tc.lookPaths,
			}
			CommandRunner = mock

			res, _, err := Handler(context.Background(), nil, Params{
				Dir: "/absolute/path",
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

func TestHandler_Success_EmptyOutput(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"selene": "",
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir: "/absolute/path",
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
			"selene": "100% mutation score: 15/15 killed",
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir: "/absolute/path",
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
			"selene": "",
		},
		errors: map[string]error{
			"selene": fmt.Errorf("command not found"),
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir: "/absolute/path",
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
			"selene": "Mutant survived: pkg/calc.go:25: replaced + with -",
		},
		errors: map[string]error{
			"selene": fmt.Errorf("exit status 1"),
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir: "/absolute/path",
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

func TestFilterNoise(t *testing.T) {
	input := `go: downloading github.com/danicat/selene v0.1.0
mutations generated: 12
mutations killed: 12
exit status 1
summary: all tests passed`

	expected := "mutations generated: 12\nmutations killed: 12\nsummary: all tests passed"
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
