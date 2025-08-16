package write_code

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestWriteCode_InvalidGo(t *testing.T) {
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	file := filepath.Join(dir, "test.go")
	// This content is invalid because fmt is not imported.
	content := "package main\n\nfunc main() {\n\tfmt.Println(\"hello world\")\n}\n"

	params := &mcp.CallToolParamsFor[WriteCodeParams]{
		Arguments: WriteCodeParams{
			FilePath: file,
			Content:  content,
		},
	}

	result, err := writeCodeHandler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("writeCodeHandler returned an unexpected error: %v", err)
	}

	if !result.IsError {
		t.Fatal("expected an error result, but got a successful one")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("expected text content")
	}

	if !strings.Contains(textContent.Text, "Write code resulted in invalid Go code") {
		t.Errorf("unexpected error message: got %q", textContent.Text)
	}

	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("expected file to be deleted, but it still exists")
	}
}

func TestWriteCode_UnformattedGo(t *testing.T) {
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

	params := &mcp.CallToolParamsFor[WriteCodeParams]{
		Arguments: WriteCodeParams{
			FilePath: file,
			Content:  unformattedContent,
		},
	}

	_, err = writeCodeHandler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("writeCodeHandler returned an unexpected error: %v", err)
	}

	fileContent, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(fileContent) != expectedContent {
		t.Errorf("file content mismatch: got %q, want %q", string(fileContent), expectedContent)
	}
}

func TestWriteCode_CreatesNewFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "test.txt")
	content := "hello world"

	params := &mcp.CallToolParamsFor[WriteCodeParams]{
		Arguments: WriteCodeParams{
			FilePath: file,
			Content:  content,
		},
	}

	_, err = writeCodeHandler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("writeCodeHandler returned an unexpected error: %v", err)
	}

	fileContent, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(fileContent) != content {
		t.Errorf("file content mismatch: got %q, want %q", string(fileContent), content)
	}
}
