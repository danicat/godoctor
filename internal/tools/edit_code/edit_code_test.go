package edit_code

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEditCode_InvalidGoEdit(t *testing.T) {
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	file := filepath.Join(dir, "test.go")
	initialContent := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello world\")\n}\n"
	if err := os.WriteFile(file, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create initial file: %v", err)
	}

	params := &mcp.CallToolParamsFor[EditCodeParams]{
		Arguments: EditCodeParams{
			FilePath: file,
			Edits: []Edit{
				{
					OldString: "fmt.Println(\"hello world\")",
					NewString: "fmt.Undefined(\"hello world\")",
				},
			},
		},
	}

	result, err := editCodeHandler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("editCodeHandler returned an unexpected error: %v", err)
	}

	if !result.IsError {
		t.Fatal("expected an error result, but got a successful one")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("expected text content")
	}

	if !strings.Contains(textContent.Text, "Edit code replacement resulted in invalid Go code") {
		t.Errorf("unexpected error message: got %q", textContent.Text)
	}

	fileContent, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(fileContent) != initialContent {
		t.Errorf("file content was not reverted: got %q", string(fileContent))
	}
}

func TestEditCode_UnformattedGoEdit(t *testing.T) {
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
	if err := os.WriteFile(file, []byte(unformattedContent), 0644); err != nil {
		t.Fatalf("failed to create initial file: %v", err)
	}

	expectedContent := "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello gopher\") }\n"

	params := &mcp.CallToolParamsFor[EditCodeParams]{
		Arguments: EditCodeParams{
			FilePath: file,
			Edits: []Edit{
				{
					OldString: "world",
					NewString: "gopher",
				},
			},
		},
	}

	_, err = editCodeHandler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("editCodeHandler returned an unexpected error: %v", err)
	}

	fileContent, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(fileContent) != expectedContent {
		t.Errorf("file content mismatch:\ngot:  %q\nwant: %q", string(fileContent), expectedContent)
	}
}

func TestEditCode_EditsExistingFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "test.txt")
	initialContent := "hello world"
	if err := os.WriteFile(file, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create initial file: %v", err)
	}

	params := &mcp.CallToolParamsFor[EditCodeParams]{
		Arguments: EditCodeParams{
			FilePath: file,
			Edits: []Edit{
				{
					OldString: "world",
					NewString: "gopher",
				},
			},
		},
	}

	_, err = editCodeHandler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("editCodeHandler returned an unexpected error: %v", err)
	}

	fileContent, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(fileContent) != "hello gopher" {
		t.Errorf("file content mismatch: got %q, want %q", string(fileContent), "hello gopher")
	}
}

func TestEditCode_FailsIfNotExist(t *testing.T) {
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "test.txt")

	params := &mcp.CallToolParamsFor[EditCodeParams]{
		Arguments: EditCodeParams{
			FilePath: file,
			Edits: []Edit{
				{
					OldString: "world",
					NewString: "gopher",
				},
			},
		},
	}

	result, err := editCodeHandler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("editCodeHandler returned an unexpected error: %v", err)
	}

	if !result.IsError {
		t.Fatal("expected an error result, but got a successful one")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("expected text content")
	}

	if !strings.Contains(textContent.Text, "file does not exist") {
		t.Errorf("unexpected error message: got %q", textContent.Text)
	}
}

func TestEditCode_FuzzyMatchSuggestions(t *testing.T) {
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "test.txt")
	initialContent := "This is a test file.\nAnother line of text.\nAnd one more line.\n"
	if err := os.WriteFile(file, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create initial file: %v", err)
	}

	params := &mcp.CallToolParamsFor[EditCodeParams]{
		Arguments: EditCodeParams{
			FilePath: file,
			Edits: []Edit{
				{
					OldString: "tst file", // Fuzzy match for "test file"
					NewString: "new content",
				},
			},
		},
	}

	result, err := editCodeHandler(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("editCodeHandler returned an unexpected error: %v", err)
	}

	if !result.IsError {
		t.Fatal("expected an error result, but got a successful one")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("expected text content")
	}

	expectedErrorPart := "old_string not found in file. No changes were made.\n\nDid you mean:\n  - This is a test file. (line 1)"
	if !strings.Contains(textContent.Text, expectedErrorPart) {
		t.Errorf("expected error message to contain %q, got %q", expectedErrorPart, textContent.Text)
	}

	// Ensure the file content remains unchanged
	fileContent, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(fileContent) != initialContent {
		t.Errorf("file content was unexpectedly changed: got %q, want %q", string(fileContent), initialContent)
	}
}
