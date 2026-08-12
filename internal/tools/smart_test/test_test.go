package smarttest

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mockRunner struct {
	outputs map[string]string
	errors  map[string]error
}

func (m *mockRunner) Run(ctx context.Context, dir, name string, args ...string) error {
	key := name + " " + strings.Join(args, " ")
	return m.errors[key]
}

func (m *mockRunner) RunWithOutput(ctx context.Context, dir, name string, args ...string) (string, error) {
	key := name + " " + strings.Join(args, " ")
	if err, ok := m.errors[key]; ok && err != nil {
		return m.outputs[key], err
	}
	if out, ok := m.outputs[key]; ok {
		return out, nil
	}
	return "ok\tpackage/test\t0.010s\tcoverage: 80.0% of statements", nil
}

func (m *mockRunner) LookPath(file string) (string, error) {
	return "/usr/local/bin/" + file, nil
}

func TestSmartTest_FastLevel(t *testing.T) {
	mock := &mockRunner{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir:      ".",
		Packages: "./...",
		Level:    "fast",
	})

	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if res == nil || len(res.Content) == 0 {
		t.Fatalf("expected call result content")
	}

	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Fast Test Pass") {
		t.Errorf("expected Fast Test Pass in output, got:\n%s", text)
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
