package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module clitest\n\ngo 1.26.0\n"), 0600); err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(tmpDir, "main.go")
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
