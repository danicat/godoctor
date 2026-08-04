package shared_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/danicat/godoctor/internal/tools/shared"
)

func TestGetLineOffsets(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5"

	tests := []struct {
		name       string
		startLine  int
		endLine    int
		wantStart  int
		wantEnd    int
		wantErr    bool
	}{
		{"full file default", 0, 0, 0, len(content), false},
		{"line 1 to 2", 1, 2, 0, 12, false},
		{"line 2 to 3", 2, 3, 6, 18, false},
		{"line out of bounds", 10, 15, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := shared.GetLineOffsets(content, tt.startLine, tt.endLine)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetLineOffsets() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if start != tt.wantStart || end != tt.wantEnd {
					t.Errorf("GetLineOffsets() = (%d, %d), want (%d, %d)", start, end, tt.wantStart, tt.wantEnd)
				}
			}
		})
	}
}

func TestGetSnippet(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5"
	snippet := shared.GetSnippet(content, 3)
	if !strings.Contains(snippet, "-> 3 | line3") {
		t.Errorf("GetSnippet() missing target line, got:\n%s", snippet)
	}

	outOfBounds := shared.GetSnippet(content, 100)
	if outOfBounds != "" {
		t.Errorf("GetSnippet() out of bounds expected empty string, got %q", outOfBounds)
	}
}

func TestExtractErrorSnippet(t *testing.T) {
	content := "line1\nline2\nline3"
	err := errors.New("file.go:2:5: syntax error")
	snippet := shared.ExtractErrorSnippet(content, err)
	if !strings.Contains(snippet, "-> 2 | line2") {
		t.Errorf("ExtractErrorSnippet() missing snippet, got:\n%s", snippet)
	}

	noLineErr := errors.New("generic error")
	noLineSnippet := shared.ExtractErrorSnippet(content, noLineErr)
	if !strings.Contains(noLineSnippet, "Could not determine error line") {
		t.Errorf("ExtractErrorSnippet() expected fallback msg, got %q", noLineSnippet)
	}
}

func TestGetLineFromOffset(t *testing.T) {
	content := "line1\nline2\nline3"
	if line := shared.GetLineFromOffset(content, 0); line != 1 {
		t.Errorf("GetLineFromOffset(0) = %d, want 1", line)
	}
	if line := shared.GetLineFromOffset(content, 7); line != 2 {
		t.Errorf("GetLineFromOffset(7) = %d, want 2", line)
	}
	if line := shared.GetLineFromOffset(content, -1); line != 0 {
		t.Errorf("GetLineFromOffset(-1) = %d, want 0", line)
	}
}
