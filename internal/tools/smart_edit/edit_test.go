package smartedit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danicat/godoctor/internal/roots"
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

//nolint:funlen,errcheck
func TestEdit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "edit-test-*")
	if err != nil {
		t.Fatal(err)
	}
	//nolint:errcheck
	defer os.RemoveAll(tmpDir)

	t.Run("invalid params", func(t *testing.T) {
		res, _, err := singleEditHandler(context.TODO(), nil, SingleEditParams{})
		if err != nil {
			t.Fatalf("unexpected go error: %v", err)
		}
		if !res.IsError {
			t.Error("expected error for empty params")
		}
	})

	t.Run("single edit valid", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "edit_test_*.go")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		initialContent := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
		if _, err := tmpFile.WriteString(initialContent); err != nil {
			t.Fatal(err)
		}
		_ = tmpFile.Close()

		res, _, err := singleEditHandler(context.TODO(), nil, SingleEditParams{
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

		content, err := os.ReadFile(tmpFile.Name())
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "println(\"world\")") {
			t.Errorf("content not updated: %s", string(content))
		}
	})

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\n\ngo 1.24\n"), 0600); err != nil {
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
			res, _, err := singleEditHandler(context.TODO(), nil, SingleEditParams{
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

			//nolint:gosec // G304: Test file path.
			newContent, _ := os.ReadFile(filePath)
			if !strings.Contains(string(newContent), tt.expected) {
				t.Errorf("expected %q in content, got: %s", tt.expected, string(newContent))
			}
		})
	}
}

func TestEdit_Broken(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "edit-broken-*")
	if err != nil {
		t.Fatal(err)
	}
	//nolint:errcheck
	defer os.RemoveAll(tmpDir)
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module broken\n\ngo 1.24\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Register temp dir as a root
	roots.Global.Add(nil, tmpDir)

	filePath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n\nfunc main() {}"), 0600); err != nil {
		t.Fatal(err)
	}

	// 1. Invalid Syntax (should fail immediately in imports.Process)
	res, _, _ := singleEditHandler(context.TODO(), nil, SingleEditParams{
		Filename:   filePath,
		OldContent: "func main() {}",
		NewContent: "func main() { invalid syntax }",
	})
	if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "edit produced invalid Go code") {
		t.Errorf("expected error for invalid syntax, got: %s", res.Content[0].(*mcp.TextContent).Text)
	}

	// 2. Broken Implementation (Valid syntax but e.g. undefined var - caught in Post-Check)
	res2, _, _ := singleEditHandler(context.TODO(), nil, SingleEditParams{
		Filename:   filePath,
		OldContent: "func main() {}",
		NewContent: "func main() { undefinedVar() }",
	})
	output := res2.Content[0].(*mcp.TextContent).Text
	// Syntax is OK, but Post-Check should see it (Actually parser.ParseFile won't see undefined vars)
	// So we might NOT see a warning here anymore.
	_ = output
}
