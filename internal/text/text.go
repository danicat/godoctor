// Package text provides string, line, and edit-distance text manipulation functions.
package text

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Levenshtein computes the Levenshtein edit distance between two strings.
// It operates on runes to correctly handle multi-byte characters.
func Levenshtein(s1, s2 string) int {
	if s1 == s2 {
		return 0
	}
	r1, r2 := []rune(s1), []rune(s2)
	n, m := len(r1), len(r2)
	if n == 0 {
		return m
	}
	if m == 0 {
		return n
	}
	if n > m {
		r1, r2 = r2, r1
		n, m = m, n
	}
	previousRow := make([]int, n+1)
	currentRow := make([]int, n+1)
	for i := 0; i <= n; i++ {
		previousRow[i] = i
	}
	for i := 1; i <= m; i++ {
		currentRow[0] = i
		for j := 1; j <= n; j++ {
			add, del, change := previousRow[j]+1, currentRow[j-1]+1, previousRow[j-1]
			if r1[j-1] != r2[i-1] {
				change++
			}
			minVal := min(change, min(del, add))
			currentRow[j] = minVal
		}
		previousRow, currentRow = currentRow, previousRow
	}
	return previousRow[n]
}

// GetLineOffsets returns the start and end byte offsets for the given 1-based line range in content.
func GetLineOffsets(content string, startLine, endLine int) (int, int, error) {
	if endLine > 0 && endLine < startLine {
		return 0, 0, fmt.Errorf("end_line %d cannot be less than start_line %d", endLine, startLine)
	}

	currentLine := 1
	startOffset := 0
	endOffset := len(content)
	foundStart := false

	if startLine <= 1 {
		startOffset = 0
		foundStart = true
	}

	for i, char := range content {
		if char == '\n' {
			currentLine++
			if !foundStart && currentLine == startLine {
				startOffset = i + 1
				foundStart = true
			}
			if endLine > 0 && currentLine > endLine {
				endOffset = i + 1
				break
			}
		}
	}

	if startLine > currentLine && startLine > 1 {
		return 0, 0, fmt.Errorf("start_line %d is beyond file length (%d lines)", startLine, currentLine)
	}

	return startOffset, endOffset, nil
}

// GetLineFromOffset returns the 1-based line number for a given byte offset in content.
func GetLineFromOffset(content string, offset int) int {
	if offset < 0 || offset > len(content) {
		return 0
	}
	return strings.Count(content[:offset], "\n") + 1
}

// GetSnippet returns a snippet centered around lineNum in content.
func GetSnippet(content string, lineNum int) string {
	lines := strings.Split(content, "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return ""
	}

	start := max(lineNum-5, 1)
	end := min(lineNum+5, len(lines))

	var sb strings.Builder
	for i := start; i <= end; i++ {
		prefix := "  "
		if i == lineNum {
			prefix = "-> "
		}
		fmt.Fprintf(&sb, "%s%d | %s\n", prefix, i, lines[i-1])
	}
	return sb.String()
}

// IsWhitespace reports whether a rune is a standard space, tab, newline, or unicode whitespace character.
func IsWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r':
		return true
	}
	return unicode.IsSpace(r)
}

// Normalize strips all whitespace characters from a string.
func Normalize(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if !IsWhitespace(r) {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// Similarity calculates the normalized similarity score (0.0 to 1.0) between two strings.
func Similarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	d := Levenshtein(s1, s2)
	maxLen := utf8.RuneCountInString(s1)
	if l2 := utf8.RuneCountInString(s2); l2 > maxLen {
		maxLen = l2
	}
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - float64(d)/float64(maxLen)
}
