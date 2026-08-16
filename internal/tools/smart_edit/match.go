package smartedit

import (
	"strings"
	"unicode/utf8"

	"github.com/danicat/godoctor/internal/text"
)

// findBestMatch locates the best match for 'search' within 'content' ignoring whitespace and newlines.
// It returns the start and end byte offsets in the original content and a similarity score (0-1).
func findBestMatch(content, search string) (int, int, float64) {
	normSearch := text.Normalize(search)
	if normSearch == "" {
		return 0, 0, 0
	}

	type charMap struct {
		char   rune
		offset int
	}
	var mapped []charMap
	for offset, char := range content {
		if !text.IsWhitespace(char) {
			mapped = append(mapped, charMap{char, offset})
		}
	}
	if len(mapped) == 0 {
		return 0, 0, 0
	}

	normContentRunes := make([]rune, len(mapped))
	for i, cm := range mapped {
		normContentRunes[i] = cm.char
	}
	normContent := string(normContentRunes)

	if before, _, ok := strings.Cut(normContent, normSearch); ok {
		runeIdx := utf8.RuneCountInString(before)
		searchRuneCount := utf8.RuneCountInString(normSearch)
		start := mapped[runeIdx].offset
		lastMapped := mapped[runeIdx+searchRuneCount-1]
		end := lastMapped.offset + utf8.RuneLen(lastMapped.char)
		return start, end, 1.0
	}

	searchRunes := []rune(normSearch)
	searchLen := len(searchRunes)
	contentLen := len(normContentRunes)

	if searchLen > contentLen {
		score := text.Similarity(normSearch, normContent)
		return 0, len(content), score
	}

	candidates := collectCandidates(normContent, normContentRunes, searchRunes, searchLen)
	bestScore, bestStartIdx, bestEndIdx := evaluateCandidates(normContentRunes, normSearch, searchLen, candidates)

	if bestScore > 0 {
		start := mapped[bestStartIdx].offset
		lastMapped := mapped[bestEndIdx-1]
		end := lastMapped.offset + utf8.RuneLen(lastMapped.char)
		return start, end, bestScore
	}

	return 0, 0, 0
}

func collectCandidates(normContent string, normContentRunes, searchRunes []rune, searchLen int) map[int]int {
	seedLen := 16
	step := 8

	switch {
	case searchLen < 4:
		seedLen = 1
		step = 1
	case searchLen < 8:
		seedLen = 2
		step = 1
	case searchLen < 16:
		seedLen = 4
		step = 2
	case searchLen < 64:
		seedLen = 8
		step = 4
	}

	// Map byte offset in normContent to rune index in normContentRunes.
	byteToNormRune := make([]int, len(normContent)+1)
	runeIdx := 0
	for byteOffset := range normContent {
		byteToNormRune[byteOffset] = runeIdx
		runeIdx++
	}
	byteToNormRune[len(normContent)] = runeIdx

	candidates := make(map[int]int)

	checkSeed := func(offset int) {
		seed := string(searchRunes[offset : offset+seedLen])
		startSearch := 0
		for {
			idx := strings.Index(normContent[startSearch:], seed)
			if idx == -1 {
				break
			}
			realIdx := startSearch + idx
			runePos := byteToNormRune[realIdx]
			projectedStart := runePos - offset
			if projectedStart >= 0 && projectedStart <= len(normContentRunes)-searchLen {
				candidates[projectedStart]++
			}
			startSearch = realIdx + 1
			if startSearch >= len(normContent) {
				break
			}
		}
	}

	for i := 0; i <= searchLen-seedLen; i += step {
		checkSeed(i)
	}

	if searchLen >= seedLen {
		tailOffset := searchLen - seedLen
		if tailOffset%step != 0 {
			checkSeed(tailOffset)
		}
	}

	// Fallback for short searches or small files if no seeds matched
	if len(candidates) == 0 && (searchLen <= 8 || len(normContentRunes) <= 256) {
		for i := 0; i <= len(normContentRunes)-searchLen; i++ {
			candidates[i] = 1
		}
	}

	return candidates
}

func evaluateCandidates(
	normContentRunes []rune,
	normSearch string,
	searchLen int,
	candidates map[int]int,
) (float64, int, int) {
	bestScore := 0.0
	bestStartIdx := 0
	bestEndIdx := 0

	for startIdx := range candidates {
		endIdx := min(startIdx+searchLen, len(normContentRunes))

		window := string(normContentRunes[startIdx:endIdx])
		score := text.Similarity(normSearch, window)

		if score > bestScore {
			bestScore = score
			bestStartIdx = startIdx
			bestEndIdx = endIdx
		}
	}
	return bestScore, bestStartIdx, bestEndIdx
}
