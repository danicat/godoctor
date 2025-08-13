package scribble

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestScribble_InvalidGo(t *testing.T) {
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	file := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc main() {\n\tfmt.Println(\"hello world\")\n}\n"

	params := &mcp.CallToolParamsFor[ScribbleParams]{
		Arguments: ScribbleParams{
			FilePath: file,
			Content:  content,
		},
	}

	_, err = scribbleHandler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("scribbleHandler returned an unexpected error: %v", err)
	}

	fileContent, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(fileContent) != content {
		t.Errorf("file content mismatch: got %q, want %q", string(fileContent), content)
	}
}

func TestScribble_UnformattedGo(t *testing.T) {
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	file := filepath.Join(dir, "test.go")
	unformattedContent := "package main\nimport \"fmt\"\nfunc main() {fmt.Println(\"hello world\")}"
	expectedContent := "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello world\") }\n"

	params := &mcp.CallToolParamsFor[ScribbleParams]{
		Arguments: ScribbleParams{
			FilePath: file,
			Content:  unformattedContent,
		},
	}

	_, err = scribbleHandler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("scribbleHandler returned an unexpected error: %v", err)
	}

	fileContent, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(fileContent) != expectedContent {
		t.Errorf("file content mismatch: got %q, want %q", string(fileContent), expectedContent)
	}
}

func TestScribble_CreatesNewFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "test.txt")
	content := "hello world"

	params := &mcp.CallToolParamsFor[ScribbleParams]{
		Arguments: ScribbleParams{
			FilePath: file,
			Content:  content,
		},
	}

	_, err = scribbleHandler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("scribbleHandler returned an unexpected error: %v", err)
	}

	fileContent, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(fileContent) != content {
		t.Errorf("file content mismatch: got %q, want %q", string(fileContent), content)
	}
}
