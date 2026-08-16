package smartedit

import (
	"testing"
	"unicode/utf8"

	"github.com/danicat/godoctor/internal/text"
)

// FuzzFindBestMatch checks for panics and basic invariants.
func FuzzFindBestMatch(f *testing.F) {
	f.Add("func main() {}", "func main")
	f.Add("some long content with newlines\nand tabs\t", "content")
	f.Add("func greet() { fmt.Println(\"こんにちは\") }", "fmt.Println(\"こんにちは\")")
	f.Add("rocket := \"🚀\"", "rocket")
	f.Add("", "")

	f.Fuzz(func(t *testing.T, content, search string) {
		// 1. Should not panic
		start, end, score := findBestMatch(content, search)

		// 2. Invariants
		if score < 0.0 || score > 1.0 {
			t.Errorf("Score out of range: %f", score)
		}
		if start < 0 || end < 0 {
			t.Errorf("Negative bounds: %d-%d", start, end)
		}
		if start > end {
			t.Errorf("Inverted bounds: %d-%d", start, end)
		}
		// If score > 0, bounds must be within content length (in bytes)
		if score > 0 {
			if end > len(content) {
				t.Errorf("End %d > ContentLen %d", end, len(content))
			}
			// If original content was valid UTF-8, the matched slice MUST be valid UTF-8
			if utf8.ValidString(content) && start <= end && end <= len(content) {
				matched := content[start:end]
				if !utf8.ValidString(matched) {
					t.Errorf("Matched slice %x from valid UTF-8 content is invalid UTF-8", matched)
				}
			}
		}
	})
}

// FuzzFindBestMatch_Exact checks that exact substrings are ALWAYS found.
func FuzzFindBestMatch_Exact(f *testing.F) {
	f.Add("prefix", "target", "suffix")
	f.Add("prefix ", "こんにちは", " suffix")
	f.Add("let a = ", "🚀", " in main")

	f.Fuzz(func(t *testing.T, prefix, target, suffix string) {
		// Normalize inputs to ensure we are testing the matching logic, not whitespace logic
		normTarget := text.Normalize(target)
		if normTarget == "" {
			return
		}

		content := prefix + target + suffix

		start, end, score := findBestMatch(content, target)
		if score < 0.99 { // Float epsilon
			t.Errorf("Failed to find exact match.\nContent: %q\nSearch: %q\nScore: %f", content, target, score)
		}

		if utf8.ValidString(content) && start <= end && end <= len(content) {
			matched := content[start:end]
			if !utf8.ValidString(matched) {
				t.Errorf("Exact matched slice %x is invalid UTF-8", matched)
			}
		}
	})
}
