// Package textdist provides text distance algorithms for fuzzy matching.
package textdist

// Levenshtein computes the Levenshtein edit distance between two strings.
// It operates on runes to correctly handle multi-byte characters.
func Levenshtein(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	n, m := len(r1), len(r2)
	if n > m {
		r1, r2 = r2, r1
		n, m = m, n
	}
	currentRow := make([]int, n+1)
	for i := 0; i <= n; i++ {
		currentRow[i] = i
	}
	for i := 1; i <= m; i++ {
		previousRow := currentRow
		currentRow = make([]int, n+1)
		currentRow[0] = i
		for j := 1; j <= n; j++ {
			add, del, change := previousRow[j]+1, currentRow[j-1]+1, previousRow[j-1]
			if r1[j-1] != r2[i-1] {
				change++
			}
			min := add
			if del < min {
				min = del
			}
			if change < min {
				min = change
			}
			currentRow[j] = min
		}
	}
	return currentRow[n]
}
