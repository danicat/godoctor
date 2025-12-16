package edit_code

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEditCode(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")

	// Helper to write file
	writeFile := func(content string) {
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Helper to read file
	readFile := func() string {
		b, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	ctx := context.Background()

	t.Run("Overwrite File (Valid)", func(t *testing.T) {
		params := EditCodeParams{
			FilePath:   filePath,
			NewContent: "package main\n\nfunc main() {}",
			Strategy:   "overwrite_file",
		}
		_, _, err := editCodeHandler(ctx, nil, params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := readFile()
		if !strings.Contains(got, "package main") {
			t.Errorf("expected file to contain 'package main', got %q", got)
		}
	})

	t.Run("Single Match (Exact)", func(t *testing.T) {
		writeFile("package main\n\nfunc old() {}\n")
		params := EditCodeParams{
			FilePath:      filePath,
			SearchContext: "func old() {}",
			NewContent:    "func new() {}",
			Strategy:      "single_match",
		}
		_, _, err := editCodeHandler(ctx, nil, params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := readFile()
		if !strings.Contains(got, "func new() {}") {
			t.Errorf("replace failed, got %q", got)
		}
	})

	t.Run("Single Match (Fuzzy Whitespace)", func(t *testing.T) {
		writeFile("package main\n\nfunc old() {\n\tprintln(\"hi\")\n}\n")
		// Search context has different indentation (spaces instead of tabs)
		params := EditCodeParams{
			FilePath:      filePath,
			SearchContext: "func old() {\n  println(\"hi\")\n}",
			NewContent:    "func new() {}",
			Strategy:      "single_match",
			Threshold:     0.8, // Allow some fuzziness
		}
		result, _, err := editCodeHandler(ctx, nil, params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			content := ""
			if len(result.Content) > 0 {
				content = result.Content[0].(*mcp.TextContent).Text
			}
			t.Fatalf("tool returned error: %s", content)
		}
		got := readFile()
		if !strings.Contains(got, "func new() {}") {
			t.Errorf("fuzzy replace failed, got %q", got)
		}
	})

	t.Run("Syntax Error Rejection", func(t *testing.T) {
		writeFile("package main\n\nfunc main() {}\n")
		params := EditCodeParams{
			FilePath:   filePath,
			NewContent: "package main\n\nfunc main() { (((( }", // Syntax error
			Strategy:   "overwrite_file",
		}
		result, _, _ := editCodeHandler(ctx, nil, params)
		if !result.IsError {
			t.Fatal("expected error result for syntax error")
		}
		// Content should NOT have changed
		got := readFile()
		if strings.Contains(got, "((((") {
			t.Error("file was modified despite syntax error")
		}
	})

	t.Run("Replace All", func(t *testing.T) {
		writeFile("package main\nimport \"fmt\"\nfunc main() {\n\tfmt.Println(\"foo\")\n\tfmt.Println(\"foo\")\n}\n")
		params := EditCodeParams{
			FilePath:      filePath,
			SearchContext: "fmt.Println(\"foo\")",
			NewContent:    "fmt.Println(\"bar\")",
			Strategy:      "replace_all",
		}
		result, _, err := editCodeHandler(ctx, nil, params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("tool returned error: %s", result.Content[0].(*mcp.TextContent).Text)
		}
		got := readFile()
		if strings.Count(got, "fmt.Println(\"bar\")") != 2 {
			t.Errorf("replace_all failed, expected 2 occurrences, got content:\n%q", got)
		}
	})

	t.Run("Feedback Best Match", func(t *testing.T) {
		writeFile("package main\n\nfunc correct() {\n\tprintln(\"hello\")\n}\n")
		// Major typo/garbage in search context
		params := EditCodeParams{
			FilePath:      filePath,
			SearchContext: "func correct() {\n  xxxxxx(\"hello\")\n}",
			NewContent:    "func new() {}",
			Strategy:      "single_match",
		}
		result, _, err := editCodeHandler(ctx, nil, params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected error for mismatch")
		}
		text := result.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, "Best candidate found at line 3") {
			t.Errorf("expected best match feedback, got: %s", text)
		}
		if !strings.Contains(text, "Diff:") {
			t.Errorf("expected diff in feedback, got: %s", text)
		}
	})
}
