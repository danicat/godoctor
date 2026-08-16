package testquery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

func TestValidateParams_RelativePathRejected(t *testing.T) {
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
			_, err := validateParams(nil, Params{
				Dir:   tc.dir,
				Query: "SELECT 1",
			})
			if err == nil {
				t.Fatalf("expected error for relative dir %q", tc.dir)
			}
			if !strings.Contains(err.Error(), "dir is required and must be an absolute path") {
				t.Errorf("expected absolute path error, got: %v", err)
			}
		})
	}
}

func TestValidateParams_EmptyQuery(t *testing.T) {
	_, err := validateParams(nil, Params{
		Dir:   "/absolute/path",
		Query: "",
	})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	if !strings.Contains(err.Error(), "query cannot be empty") {
		t.Errorf("expected query cannot be empty error, got: %v", err)
	}
}

func TestValidateParams_Valid(t *testing.T) {
	absDir, err := validateParams(nil, Params{
		Dir:   "/absolute/path",
		Query: "SELECT 1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if absDir != "/absolute/path" {
		t.Errorf("expected /absolute/path, got %s", absDir)
	}
}

func TestHandler_ValidationFailure(t *testing.T) {
	res, _, err := Handler(context.Background(), nil, Params{
		Dir:   "relative/path",
		Query: "SELECT * FROM all_tests",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for invalid dir")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "dir is required and must be an absolute path") {
		t.Errorf("expected validation message, got: %s", text)
	}
}

func TestHandler_ExistingDB_NoRebuild(t *testing.T) {
	tmpDir := t.TempDir()
	dbFilePath := filepath.Join(tmpDir, dbFile)
	if err := os.WriteFile(dbFilePath, []byte("fake db"), 0600); err != nil {
		t.Fatalf("failed to create fake db: %v", err)
	}

	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"query": "col1 | col2\nval1 | val2",
		},
		lookPaths: map[string]string{
			"testquery": "/usr/local/bin/testquery",
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir:     tmpDir,
		Query:   "SELECT * FROM all_tests",
		Rebuild: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error result")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "val1 | val2") {
		t.Errorf("expected query results, got: %s", text)
	}

	// Verify build was not called
	for _, call := range mock.calls {
		if strings.Contains(call, "build") {
			t.Errorf("expected build not to be called, but saw: %s", call)
		}
	}
}

func TestHandler_MissingDB_BuildsDB(t *testing.T) {
	tmpDir := t.TempDir()

	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"build": "building db...",
			"query": "query output",
		},
		lookPaths: map[string]string{
			"testquery": "/usr/local/bin/testquery",
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir:   tmpDir,
		Query: "SELECT * FROM all_tests",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error result")
	}

	sawBuild := false
	sawQuery := false
	for _, call := range mock.calls {
		if strings.Contains(call, "build") {
			sawBuild = true
		}
		if strings.Contains(call, "query") {
			sawQuery = true
		}
	}
	if !sawBuild {
		t.Error("expected build command to be executed")
	}
	if !sawQuery {
		t.Error("expected query command to be executed")
	}
}

func TestHandler_Rebuild_ForcesBuild(t *testing.T) {
	tmpDir := t.TempDir()
	dbFilePath := filepath.Join(tmpDir, dbFile)
	if err := os.WriteFile(dbFilePath, []byte("existing db"), 0600); err != nil {
		t.Fatalf("failed to create db: %v", err)
	}

	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"build": "rebuilding db...",
			"query": "results",
		},
		lookPaths: map[string]string{
			"testquery": "/usr/local/bin/testquery",
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir:     tmpDir,
		Query:   "SELECT * FROM all_tests",
		Rebuild: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error result")
	}

	sawBuild := false
	for _, call := range mock.calls {
		if strings.Contains(call, "build") {
			sawBuild = true
		}
	}
	if !sawBuild {
		t.Error("expected build command because Rebuild=true")
	}
}

func TestHandler_BuildDB_Error_MissingDB(t *testing.T) {
	tmpDir := t.TempDir()

	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"build": "compilation error in tests",
		},
		errors: map[string]error{
			"build": fmt.Errorf("exit status 1"),
		},
		lookPaths: map[string]string{
			"testquery": "/usr/local/bin/testquery",
		},
	}
	CommandRunner = mock

	res, _, err := Handler(context.Background(), nil, Params{
		Dir:   tmpDir,
		Query: "SELECT * FROM all_tests",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result when buildDB fails without db file")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "failed to build test database") {
		t.Errorf("expected failed to build test database message, got: %s", text)
	}
	if !strings.Contains(text, "compilation error in tests") {
		t.Errorf("expected build output in error message, got: %s", text)
	}
}

func TestBuildDB_ToolPathResolution(t *testing.T) {
	tests := []struct {
		name        string
		lookPaths   map[string]string
		pkg         string
		expectedCmd string
	}{
		{
			name:        "testquery in PATH",
			lookPaths:   map[string]string{"testquery": "/usr/bin/testquery"},
			pkg:         "github.com/danicat/godoctor/...",
			expectedCmd: "testquery build --pkg github.com/danicat/godoctor/... --output testquery.db",
		},
		{
			name:        "tq in PATH",
			lookPaths:   map[string]string{"testquery": "", "tq": "/usr/bin/tq"},
			pkg:         "",
			expectedCmd: "tq build --pkg ./... --output testquery.db",
		},
		{
			name:        "neither in PATH fallback to go run",
			lookPaths:   map[string]string{"testquery": "", "tq": ""},
			pkg:         "my/pkg",
			expectedCmd: "go run github.com/danicat/testquery@latest build --pkg my/pkg --output testquery.db",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldRunner := CommandRunner
			defer func() { CommandRunner = oldRunner }()

			mock := &mockRunner{lookPaths: tc.lookPaths}
			CommandRunner = mock

			tmpDir := t.TempDir()
			dbPath := filepath.Join(tmpDir, dbFile)
			errRes := buildDB(context.Background(), tmpDir, Params{Pkg: tc.pkg}, dbPath)
			if errRes != nil {
				t.Fatalf("unexpected error result: %v", errRes)
			}

			if len(mock.calls) == 0 || mock.calls[0] != tc.expectedCmd {
				t.Errorf("expected cmd %q, got %v", tc.expectedCmd, mock.calls)
			}
		})
	}
}

func TestBuildDB_BuildFailure_DBExists(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, dbFile)
	if err := os.WriteFile(dbPath, []byte("pre-existing db"), 0600); err != nil {
		t.Fatalf("failed to create db: %v", err)
	}

	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"build": "some non-fatal build warning",
		},
		errors: map[string]error{
			"build": fmt.Errorf("exit status 1"),
		},
		lookPaths: map[string]string{
			"testquery": "/usr/bin/testquery",
		},
	}
	CommandRunner = mock

	errRes := buildDB(context.Background(), tmpDir, Params{}, dbPath)
	if errRes != nil {
		t.Errorf("expected error to be tolerated when db exists, got error result: %v", errRes)
	}
}

func TestRunQuery_ToolPathResolution(t *testing.T) {
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
			expectedCmd: "testquery query --db testquery.db --format table SELECT 1",
		},
		{
			name: "tq in PATH",
			lookPaths: map[string]string{
				"testquery": "",
				"tq":        "/usr/bin/tq",
			},
			expectedCmd: "tq query --db testquery.db --format table SELECT 1",
		},
		{
			name: "fallback to go run",
			lookPaths: map[string]string{
				"testquery": "",
				"tq":        "",
			},
			expectedCmd: "go run github.com/danicat/testquery@latest query --db testquery.db --format table SELECT 1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldRunner := CommandRunner
			defer func() { CommandRunner = oldRunner }()

			mock := &mockRunner{
				outputs: map[string]string{
					"query": "query result row",
				},
				lookPaths: tc.lookPaths,
			}
			CommandRunner = mock

			res, _, err := runQuery(context.Background(), "/workspace", "SELECT 1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
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

func TestRunQuery_EmptyOutput(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"query": "",
		},
		lookPaths: map[string]string{
			"testquery": "/usr/bin/testquery",
		},
	}
	CommandRunner = mock

	res, _, err := runQuery(context.Background(), "/workspace", "SELECT * FROM all_tests WHERE 1=0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected non-error for empty result set")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if text != "Query returned no results." {
		t.Errorf("expected 'Query returned no results.', got: %q", text)
	}
}

func TestRunQuery_ErrorWithEmptyOutput(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"query": "",
		},
		errors: map[string]error{
			"query": fmt.Errorf("syntax error in SQL statement"),
		},
		lookPaths: map[string]string{
			"testquery": "/usr/bin/testquery",
		},
	}
	CommandRunner = mock

	res, _, err := runQuery(context.Background(), "/workspace", "INVALID SQL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "test query failed:") {
		t.Errorf("expected 'test query failed:', got: %s", text)
	}
	if !strings.Contains(text, "**HINT:** Check SQL query syntax") {
		t.Errorf("expected hint, got: %s", text)
	}
}

func TestRunQuery_ErrorWithOutput(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	mock := &mockRunner{
		outputs: map[string]string{
			"query": "col1 | col2\nwarning: partial data only",
		},
		errors: map[string]error{
			"query": fmt.Errorf("exit status 1"),
		},
		lookPaths: map[string]string{
			"testquery": "/usr/bin/testquery",
		},
	}
	CommandRunner = mock

	res, _, err := runQuery(context.Background(), "/workspace", "SELECT * FROM all_tests")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true for query with warnings")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "⚠️ Query completed with warnings:") {
		t.Errorf("expected warning header, got: %s", text)
	}
	if !strings.Contains(text, "warning: partial data only") {
		t.Errorf("expected partial output in message, got: %s", text)
	}
}

func TestFilterNoise(t *testing.T) {
	input := `go: downloading github.com/danicat/testquery v1.0.0
table header
row 1 data
exit status 1
row 2 data`

	expected := "table header\nrow 1 data\nrow 2 data"
	got := filterNoise(input)
	if got != expected {
		t.Errorf("expected filtered noise:\n%s\ngot:\n%s", expected, got)
	}
}

func TestErrorResult(t *testing.T) {
	msg := "something went wrong"
	res := errorResult(msg)
	if !res.IsError {
		t.Error("expected IsError to be true")
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected 1 content element, got %d", len(res.Content))
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if text != msg {
		t.Errorf("expected text %q, got %q", msg, text)
	}
}
