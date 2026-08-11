package smartedit

import (
	"strings"

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
	normContentRunes := make([]rune, len(mapped))
	for i, cm := range mapped {
		normContentRunes[i] = cm.char
	}
	normContent := string(normContentRunes)

	if before, _, ok := strings.Cut(normContent, normSearch); ok {
		runeIdx := len([]rune(before))
		start := mapped[runeIdx].offset
		end := mapped[runeIdx+len([]rune(normSearch))-1].offset + 1
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
		end := mapped[bestEndIdx-1].offset + 1
		return start, end, bestScore
	}

	return 0, 0, 0
}

func collectCandidates(normContent string, normContentRunes, searchRunes []rune, searchLen int) map[int]int {
	seedLen := 16
	step := 8

	if searchLen < 64 {
		seedLen = 8
		step = 4
	}
	if searchLen < seedLen {
		seedLen = 4
		step = 2
	}

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
			projectedStart := realIdx - offset
			if projectedStart >= 0 && projectedStart <= len(normContentRunes)-searchLen {
				candidates[projectedStart]++
			}
			startSearch = realIdx + 1
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
