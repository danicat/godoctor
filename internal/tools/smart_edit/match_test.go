package smartedit

import (
	"testing"
	"unicode/utf8"

	"github.com/danicat/godoctor/internal/text"
)

const mainFunc = "func main() {}"

var findBestMatchTests = []struct {
	name        string
	content     string
	search      string
	expectMatch bool
	minScore    float64
}{
	{
		name:        "Exact Match",
		content:     "func main() {\n\tfmt.Println(\"Hello\")\n}",
		search:      "fmt.Println(\"Hello\")",
		expectMatch: true,
		minScore:    1.0,
	},
	{
		name:        "Whitespace Normalization (Tabs vs Spaces)",
		content:     "func main() {\tfmt.Println(\"Hello\")\n}",
		search:      "func main() { fmt.Println(\"Hello\") }",
		expectMatch: true,
		minScore:    1.0,
	},
	{
		name:        "Typo (1 char)",
		content:     "func main() {\n\tfmt.Println(\"Hello\")\n}",
		search:      "fmt.Prontln(\"Hello\")",
		expectMatch: true,
		minScore:    0.8,
	},
	{
		name:        "Long Block with Typo (Seeding)",
		content:     "func long() {\n\tline1()\n\tline2()\n\tline3()\n\tline4()\n}",
		search:      "func long() { line1() line2() line3-typo() line4() }",
		expectMatch: true,
		minScore:    0.85,
	},
	{
		name:        "Short String (< 16 chars)",
		content:     "var x = 10",
		search:      "var x = 10",
		expectMatch: true,
		minScore:    1.0,
	},
	{
		name:        "No Match (Garbage)",
		content:     mainFunc,
		search:      "completely different string",
		expectMatch: false,
	},
	{
		name:        "Empty File",
		content:     "",
		search:      "func main()",
		expectMatch: false,
	},
	{
		name:        "Search Larger Than File",
		content:     "short",
		search:      "longer search string",
		expectMatch: false,
	},
	{
		name:        "Unicode Support",
		content:     "func main() { fmt.Println(\"こんにちは\") }",
		search:      "fmt.Println(\"こんにちは\")",
		expectMatch: true,
		minScore:    1.0,
	},
	{
		name:        "Unicode Fuzzy Typo (Japanese)",
		content:     "func greet() { fmt.Println(\"こんにちは世界\") }",
		search:      "fmt.Println(\"こんには世界\")", // 1 missing rune
		expectMatch: true,
		minScore:    0.85,
	},
	{
		name:        "Unicode Fuzzy Typo (Cyrillic)",
		content:     "func greet() { return \"Привет мир\" }",
		search:      "return \"Привед мир\"", // 1 typo in Cyrillic
		expectMatch: true,
		minScore:    0.9,
	},
	{
		name:        "Unicode Emoji Fuzzy Typo",
		content:     "const rocket = \"🚀 launch sequence\"",
		search:      "rocket = \"🚀 luanch sequence\"",
		expectMatch: true,
		minScore:    0.9,
	},
	{
		name:        "Short Expression with Typo (< 4 chars)",
		content:     "x = 10\ni++\ny = 20",
		search:      "i+-", // 3 chars with 1 typo
		expectMatch: true,
		minScore:    0.6,
	},
	{
		name:        "Typos at Start",
		content:     "func main() { body }",
		search:      "fync main() { body }", // typo in first seed
		expectMatch: true,
		minScore:    0.9,
	},
	{
		name:        "Typos at End",
		content:     "func main() { body }",
		search:      "func main() { budy }", // typo in last seed
		expectMatch: true,
		minScore:    0.9,
	},
	{
		name:        "Partial Match (Substring)",
		content:     "prefix func target() {} suffix",
		search:      "func target() {}",
		expectMatch: true,
		minScore:    1.0,
	},
}

func TestFindBestMatch(t *testing.T) {
	for _, tt := range findBestMatchTests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, score := findBestMatch(tt.content, tt.search)

			if !tt.expectMatch {
				if score > 0.6 {
					t.Errorf("expected no match, got score %.2f at %d-%d", score, start, end)
				}
				return
			}

			if score < tt.minScore {
				t.Errorf("score %.2f < minScore %.2f. Bounds: %d-%d", score, tt.minScore, start, end)
			}

			if start > end {
				t.Errorf("invalid bounds start > end: %d-%d", start, end)
			}

			if start < 0 || end > len(tt.content) {
				t.Errorf("bounds out of range: %d-%d (content len: %d)", start, end, len(tt.content))
			}

			// Validate UTF-8 slice boundary
			matchedSlice := tt.content[start:end]
			if !utf8.ValidString(matchedSlice) {
				t.Errorf("matched slice %q is not valid UTF-8", matchedSlice)
			}
		})
	}
}

func TestFindBestMatch_UTF8_Boundary(t *testing.T) {
	// Search target ends with multi-byte rune 'は'
	content := "prefix こんにちは suffix"
	search := "こんにちは"

	start, end, score := findBestMatch(content, search)
	if score != 1.0 {
		t.Fatalf("expected 1.0 score, got %f", score)
	}

	got := content[start:end]
	if got != "こんにちは" {
		t.Errorf("expected %q, got %q (bytes: %d..%d)", "こんにちは", got, start, end)
	}
	if !utf8.ValidString(got) {
		t.Errorf("expected valid UTF-8 slice, got invalid: %x", got)
	}
}

func TestNormalize(t *testing.T) {
	s := "  a \t b \n c "
	got := text.Normalize(s)
	want := "abc"
	if got != want {
		t.Errorf("normalize(%q) = %q, want %q", s, got, want)
	}
}

func TestLevenshtein(t *testing.T) {
	if d := text.Levenshtein("abc", "abd"); d != 1 {
		t.Errorf("Levenshtein(abc, abd) = %d, want 1", d)
	}
	if d := text.Levenshtein("abc", "abc"); d != 0 {
		t.Errorf("Levenshtein(abc, abc) = %d, want 0", d)
	}
}

func BenchmarkFindBestMatch_Exact(b *testing.B) {
	content := "package main\n\nfunc ProcessData(items []Item) error {\n\tfor _, item := range items {\n\t\titem.Process()\n\t}\n\treturn nil\n}\n"
	search := "for _, item := range items {\n\titem.Process()\n}"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		findBestMatch(content, search)
	}
}

func BenchmarkFindBestMatch_Fuzzy(b *testing.B) {
	content := "package main\n\nfunc ProcessData(items []Item) error {\n\tfor _, item := range items {\n\t\titem.ExecuteActions()\n\t}\n\treturn nil\n}\n"
	search := "for _, item := range items {\n\titem.ExecuteAction()\n}"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		findBestMatch(content, search)
	}
}
