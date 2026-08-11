package text_test

import (
	"testing"

	"github.com/danicat/godoctor/internal/text"
)

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		s1, s2   string
		expected int
	}{
		{"kitten", "sitting", 3},
		{"hello", "hello", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"gopher", "go", 4},
	}

	for _, tt := range tests {
		got := text.Levenshtein(tt.s1, tt.s2)
		if got != tt.expected {
			t.Errorf("Levenshtein(%q, %q) = %d; want %d", tt.s1, tt.s2, got, tt.expected)
		}
	}
}

func TestGetLineOffsets(t *testing.T) {
	content := "line 1\nline 2\nline 3\nline 4"

	// Start at line 1, end at line 2
	s, e, err := text.GetLineOffsets(content, 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != 0 || e != 14 {
		t.Errorf("expected 0..14, got %d..%d (%q)", s, e, content[s:e])
	}

	// Line out of bounds
	_, _, err = text.GetLineOffsets(content, 10, 0)
	if err == nil {
		t.Error("expected error for line beyond length")
	}
}

func TestGetLineFromOffset(t *testing.T) {
	content := "line 1\nline 2\nline 3"
	if line := text.GetLineFromOffset(content, 0); line != 1 {
		t.Errorf("expected line 1 for offset 0, got %d", line)
	}
	if line := text.GetLineFromOffset(content, 7); line != 2 {
		t.Errorf("expected line 2 for offset 7, got %d", line)
	}
	if line := text.GetLineFromOffset(content, -1); line != 0 {
		t.Errorf("expected 0 for invalid offset, got %d", line)
	}
}

func TestGetSnippet(t *testing.T) {
	content := "l1\nl2\nl3\nl4\nl5\nl6\nl7"
	snip := text.GetSnippet(content, 3)
	if snip == "" {
		t.Error("expected non-empty snippet")
	}
	if text.GetSnippet(content, 99) != "" {
		t.Error("expected empty snippet for out of bounds line")
	}
}

func TestNormalizeAndIsWhitespace(t *testing.T) {
	raw := "  hello \t world \n "
	norm := text.Normalize(raw)
	if norm != "helloworld" {
		t.Errorf("expected 'helloworld', got %q", norm)
	}
}

func TestSimilarity(t *testing.T) {
	if sim := text.Similarity("hello", "hello"); sim != 1.0 {
		t.Errorf("expected 1.0 similarity, got %f", sim)
	}
	if sim := text.Similarity("a", "b"); sim >= 1.0 {
		t.Errorf("expected < 1.0 similarity for 'a' and 'b', got %f", sim)
	}
}
