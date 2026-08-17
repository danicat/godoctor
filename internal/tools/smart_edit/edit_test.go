package smartedit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var editTests = []struct {
	name     string
	search   string
	replace  string
	expected string
}{
	{
		"Simple Replace",
		"fmt.Println(\"Hello\")",
		"fmt.Println(\"Goodbye\")",
		"fmt.Println(\"Goodbye\")",
	},
	{
		"Whitespace Agnostic",
		"func main() {\n\tfmt.Println(\"Goodbye\")\n}",
		"func main() { fmt.Println(\"Modified\") }",
		"fmt.Println(\"Modified\")",
	},
}

func TestEdit_InvalidParams(t *testing.T) {
	testCases := []string{"", "main.go", "./pkg/file.go", "internal/server.go"}
	for _, fn := range testCases {
		res, _, err := Handler(context.TODO(), nil, SingleEditParams{Filename: fn})
		if err != nil {
			t.Fatalf("unexpected go error: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected error for non-absolute filename %q", fn)
		}
		text := res.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "filename is required and must be an absolute path") {
			t.Errorf("expected absolute path error message, got: %s", text)
		}
	}
}

func TestEdit_SingleEditValid(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n\ngo 1.26.0\n"), 0600); err != nil {
		t.Fatal(err)
	}

	tmpFile, err := os.CreateTemp(tmpDir, "edit_test_*.go")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	initialContent := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	if _, err := tmpFile.WriteString(initialContent); err != nil {
		t.Fatal(err)
	}
	_ = tmpFile.Close()

	res, _, err := Handler(context.TODO(), nil, SingleEditParams{
		Filename:   tmpFile.Name(),
		OldContent: "println(\"hello\")",
		NewContent: "println(\"world\")",
	})
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error: %s", res.Content[0].(*mcp.TextContent).Text)
	}

	content, err := os.ReadFile(filepath.Clean(tmpFile.Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "println(\"world\")") {
		t.Errorf("content not updated: %s", string(content))
	}
}

func TestEdit_TableDriven(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n\ngo 1.26.0\n"), 0600); err != nil {
		t.Fatal(err)
	}

	content := `package main
import "fmt"

func main() {
	fmt.Println("Hello")
}
`
	filePath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	for _, tt := range editTests {
		t.Run(tt.name, func(t *testing.T) {
			res, _, err := Handler(context.TODO(), nil, SingleEditParams{
				Filename:   filePath,
				OldContent: tt.search,
				NewContent: tt.replace,
			})
			if err != nil {
				t.Fatalf("singleEditHandler failed: %v", err)
			}
			if res.IsError {
				t.Fatalf("Tool returned error: %v", res.Content[0].(*mcp.TextContent).Text)
			}

			newContent, _ := os.ReadFile(filepath.Clean(filePath))
			if !strings.Contains(string(newContent), tt.expected) {
				t.Errorf("expected %q in content, got: %s", tt.expected, string(newContent))
			}
		})
	}
}

func TestEdit_Broken(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module broken\n\ngo 1.26.0\n"), 0600); err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n\nfunc main() {}"), 0600); err != nil {
		t.Fatal(err)
	}

	// 1. Invalid Syntax (should fail immediately in imports.Process)
	res, _, _ := Handler(context.TODO(), nil, SingleEditParams{
		Filename:   filePath,
		OldContent: "func main() {}",
		NewContent: "func main() { invalid syntax }",
	})
	if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "edit produced invalid Go code") {
		t.Errorf("expected error for invalid syntax, got: %s", res.Content[0].(*mcp.TextContent).Text)
	}

	// 2. Broken Implementation (Valid syntax but undefined symbol - caught in Post-Check go vet)
	res2, _, err := Handler(context.TODO(), nil, SingleEditParams{
		Filename:   filePath,
		OldContent: "func main() {}",
		NewContent: "func main() { undefinedVar() }",
	})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if res2 == nil || !res2.IsError {
		t.Fatalf("expected error from post-edit compiler check, got success")
	}
	output := res2.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(output, "Post-edit diagnostics check failed") {
		t.Errorf("expected diagnostics check failure, got: %s", output)
	}

	// Verify rollback restored original content
	restored, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		t.Fatalf("failed to read file after rollback: %v", err)
	}
	if string(restored) != "package main\n\nfunc main() {}" {
		t.Errorf("expected content to be rolled back to original, got: %q", string(restored))
	}
}
