package godoc

import (
	"context"
	"strings"
	"testing"
)

func TestGetDocumentation_VanityAndModules(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	tests := []struct {
		name          string
		pkgPath       string
		expectNote    bool
		expectContent []string
	}{
		{
			name:       "Vanity Import Redirect (adk-go)",
			pkgPath:    "github.com/google/adk-go",
			expectNote: true,
			expectContent: []string{
				"Redirected from github.com/google/adk-go",
				"Module root for google.golang.org/adk",
			},
		},
		{
			name:       "Standard External Package (uuid)",
			pkgPath:    "github.com/google/uuid",
			expectNote: false,
			expectContent: []string{
				"package uuid",
			},
		},
		{
			name:       "Module Root with Subpackages (genkit)",
			pkgPath:    "github.com/firebase/genkit/go",
			expectNote: false,
			expectContent: []string{
				"Module root for github.com/firebase/genkit/go",
				"sub-packages exist",
			},
		},
		{
			name:       "Parent Fallback (Non-existent Subpackage)",
			pkgPath:    "github.com/google/uuid/foo/bar",
			expectNote: true, // Should show fallback note
			expectContent: []string{
				"Could not find `github.com/google/uuid/foo/bar`",
				"Showing documentation for parent module `github.com/google/uuid` instead",
				"package uuid",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := LoadWithFallback(ctx, tt.pkgPath, "")
			if err != nil {
				t.Fatalf("LoadWithFallback failed: %v", err)
			}
			doc := Render(d)
			runVanityTestAssertions(t, tt.expectNote, tt.expectContent, doc)
		})
	}
}

func runVanityTestAssertions(t *testing.T, expectNote bool, expectContent []string, doc string) {
	hasNote := strings.Contains(doc, "> **Note:** Redirected from") ||
		strings.Contains(doc, "> ℹ️ **Note:** Could not find")
	if hasNote != expectNote {
		t.Errorf("Note presence mismatch: expected %v, got %v\nDoc:\n%s", expectNote, hasNote, doc)
	}

	for _, content := range expectContent {
		if !strings.Contains(doc, content) {
			t.Errorf("Documentation missing expected content: %q", content)
		}
	}
}
