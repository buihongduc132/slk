package fuzzy

import (
	"strings"

	"github.com/gammons/slk/internal/text"
)

type Tier int

const (
	TierNone Tier = iota
	TierPrefix
	TierWordPrefix
	TierSquashedPrefix
	TierSubstring
	TierSubsequence
)

func Match(name, query string) (tier Tier, score int, ok bool) {
	if query == "" || name == "" {
		return TierNone, 0, false
	}
	foldedName := text.Fold(name)
	foldedQuery := text.Fold(query)

	if strings.HasPrefix(foldedName, foldedQuery) {
		return TierPrefix, 0, true
	}
	if WordPrefix(foldedName, foldedQuery) {
		return TierWordPrefix, 0, true
	}
	if SquashedPrefix(foldedName, foldedQuery) {
		return TierSquashedPrefix, 0, true
	}
	if strings.Contains(foldedName, foldedQuery) {
		return TierSubstring, 0, true
	}
	if score, ok := SubsequenceScore(foldedName, foldedQuery); ok {
		return TierSubsequence, score, true
	}
	return TierNone, 0, false
}

func SubsequenceScore(name, query string) (int, bool) {
	if query == "" {
		return 0, false
	}

	score := 0
	qi := 0
	qrunes := []rune(query)
	first, last := -1, -1
	prevWasSep := true // start of string counts as a word boundary
	for i, r := range name {
		if qi >= len(qrunes) {
			break
		}
		if r == qrunes[qi] {
			if first < 0 {
				first = i
			}
			last = i
			score += 10
			if prevWasSep {
				score += 25 // word-boundary bonus
			}
			qi++
		}
		prevWasSep = isSeparator(r)
	}
	if qi < len(qrunes) {
		return 0, false
	}
	// Tightness bonus
	span := last - first + 1
	if span > 0 {
		score += 50 * len(qrunes) / span
	}
	return score, true
}

func isSeparator(r rune) bool {
	switch r {
	case '-', '_', '.', ' ', '/', ':':
		return true
	}
	return false
}

func WordPrefix(name, query string) bool {
	if query == "" {
		return false
	}
	foldedName := text.Fold(name)
	foldedQuery := text.Fold(query)
	if strings.HasPrefix(foldedName, foldedQuery) {
		return false // whole-name prefix is not a word prefix
	}
	for _, sep := range []string{" ", "-", "_", "."} {
		if strings.Contains(foldedName, sep+foldedQuery) {
			return true
		}
	}
	return false
}

func SquashedPrefix(foldedName, foldedQuery string) bool {
	if foldedQuery == "" {
		return false
	}
	squashed := foldedName
	for _, sep := range []string{" ", "-", "_", "."} {
		squashed = strings.ReplaceAll(squashed, sep, "")
	}
	return strings.HasPrefix(squashed, foldedQuery)
}
