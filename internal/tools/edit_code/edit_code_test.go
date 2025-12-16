package edit_code

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		_, _, err := editCodeHandler(ctx, nil, params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
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
}
