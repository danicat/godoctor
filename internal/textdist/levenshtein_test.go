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
