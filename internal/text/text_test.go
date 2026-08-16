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
		{"こんにちは", "こんばんは", 2},
		{"🚀go", "🛸go", 1},
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

	// endLine < startLine
	_, _, err = text.GetLineOffsets(content, 3, 1)
	if err == nil {
		t.Error("expected error when endLine < startLine")
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

	// Non-breaking space and other unicode whitespace
	unicodeRaw := "foo\u00A0bar\u3000baz\u2002qux"
	unicodeNorm := text.Normalize(unicodeRaw)
	if unicodeNorm != "foobarbazqux" {
		t.Errorf("expected 'foobarbazqux', got %q", unicodeNorm)
	}

	if !text.IsWhitespace('\u00A0') {
		t.Error("expected non-breaking space (\\u00A0) to be whitespace")
	}
	if !text.IsWhitespace('\u3000') {
		t.Error("expected ideographic space (\\u3000) to be whitespace")
	}
	if text.IsWhitespace('a') {
		t.Error("expected 'a' not to be whitespace")
	}
}

func TestSimilarity(t *testing.T) {
	if sim := text.Similarity("hello", "hello"); sim != 1.0 {
		t.Errorf("expected 1.0 similarity, got %f", sim)
	}
	if sim := text.Similarity("a", "b"); sim >= 1.0 {
		t.Errorf("expected < 1.0 similarity for 'a' and 'b', got %f", sim)
	}
	if sim := text.Similarity("こんにちは", "こんにちは"); sim != 1.0 {
		t.Errorf("expected 1.0 for identical unicode strings, got %f", sim)
	}
	if sim := text.Similarity("こんにちは", "こんばんは"); sim != 0.6 {
		t.Errorf("expected 0.6 for 2 edit distance out of 5 runes, got %f", sim)
	}
}

func BenchmarkLevenshtein(b *testing.B) {
	s1 := "func CalculateTotal(items []Item) (int, error)"
	s2 := "func CalculateTotals(items []Item) (int, error)"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		text.Levenshtein(s1, s2)
	}
}

func BenchmarkSimilarity(b *testing.B) {
	s1 := "func CalculateTotal(items []Item) (int, error)"
	s2 := "func CalculateTotals(items []Item) (int, error)"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		text.Similarity(s1, s2)
	}
}

func BenchmarkNormalize(b *testing.B) {
	raw := "   func   main()   {\n\tfmt.Println(\"hello\")\n}\n"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		text.Normalize(raw)
	}
}
