package textdist

import "testing"

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		name   string
		s1, s2 string
		want   int
	}{
		{"identical", "abc", "abc", 0},
		{"one sub", "abc", "abd", 1},
		{"classic", "kitten", "sitting", 3},
		{"empty left", "", "abc", 3},
		{"empty right", "abc", "", 3},
		{"both empty", "", "", 0},
		{"unicode", "café", "cafe", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Levenshtein(tt.s1, tt.s2); got != tt.want {
				t.Errorf("Levenshtein(%q, %q) = %d, want %d", tt.s1, tt.s2, got, tt.want)
			}
		})
	}
}

func BenchmarkLevenshtein(b *testing.B) {
	benchmarks := []struct {
		name   string
		s1, s2 string
	}{
		{"short", "kitten", "sitting"},
		{"medium", "The quick brown fox jumps over the lazy dog", "The fast brown fox leaped over a lazy dog"},
		{"long", "Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
			"Lorem ipsum dolor sit amet, sed do eiusmod tempor incididunt."},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = Levenshtein(bm.s1, bm.s2)
			}
		})
	}
}
