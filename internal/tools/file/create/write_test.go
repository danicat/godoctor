package create

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danicat/godoctor/internal/graph"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestWrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "write-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module write-test\n\ngo 1.24\n"), 0644)
	graph.Global = graph.NewManager()

	filePath := filepath.Join(tmpDir, "lib.go")

	// 1. Overwrite (Initial create)
	res, _, _ := toolHandler(nil, nil, Params{
		Name:    filePath,
		Content: "package lib\n\nfunc A() {}",
	})
	if res.IsError {
		t.Fatalf("Initial write failed: %v", res.Content[0].(*mcp.TextContent).Text)
	}

	content, _ := os.ReadFile(filePath)
	if !strings.Contains(string(content), "func A()") {
		t.Errorf("expected func A() in file, got: %s", string(content))
	}
}

func TestWrite_Validation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "write-val-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module val-test\n\ngo 1.24\n"), 0644)
	
	filePath := filepath.Join(tmpDir, "main.go")

	// Write code with missing import/symbol
	res, _, _ := toolHandler(nil, nil, Params{
		Name:    filePath,
		Content: "package main\n\nfunc main() { fmt.Println(NonExistent) }",
	})

	output := res.Content[0].(*mcp.TextContent).Text
	// imports.Process should have added "fmt"
	// but graph.Load should report "undefined: NonExistent"
	if !strings.Contains(output, "**WARNING:**") || !strings.Contains(output, "undefined: NonExistent") {
		t.Errorf("expected warning about NonExistent, got: %s", output)
	}
}
