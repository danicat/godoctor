package scalpel

import (
	"os"
	"testing"
)

func TestReadOperation(t *testing.T) {
	// Create a temporary file with some content
	content := []byte("line 1\nline 2\nline 3")
	tmpfile, err := os.CreateTemp("", "testfile.*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test reading the entire file
	params := &ScalpelParams{
		Operation: "read",
		FilePath:  tmpfile.Name(),
	}
	result, err := Execute(params)
	if err != nil {
		t.Fatalf("Read operation failed: %v", err)
	}
	expected := "1: line 1\n2: line 2\n3: line 3\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}

	// Test reading a fragment of the file
	params = &ScalpelParams{
		Operation: "read",
		FilePath:  tmpfile.Name(),
		Start:     &Position{Line: 2, Column: 1},
		End:       &Position{Line: 3, Column: 5},
	}
	result, err = Execute(params)
	if err != nil {
		t.Fatalf("Read operation failed: %v", err)
	}
	expected = "1: line 2\n2: line\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}
