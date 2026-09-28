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
		prevWasSep = IsSeparator(r)
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

func IsSeparator(r rune) bool {
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
	for i, r := range foldedName {
		if IsSeparator(r) {
			if strings.HasPrefix(foldedName[i+len(string(r)):], foldedQuery) {
				return true
			}
		}
	}
	return false
}

// SquashedPrefix reports whether the query is a prefix of the name with
// separators removed — "engwidgets" matches "eng-widgets" — but a join is
// only allowed across a boundary whose words are both at least 3 runes.
// Short words do not squash: Match("cs-product-triage", "csp") and
// Match("rock-et", "rocket") must fall through to TierSubsequence (pinned
// by TestMatch_SubsequenceInOrder / TestMatch_TierOrderingBeatsScore),
// while SquashedPrefix("eng-widgets", "engwidgets") stays true.
func SquashedPrefix(name, query string) bool {
	if query == "" {
		return false
	}
	foldedName := text.Fold(name)
	foldedQuery := text.Fold(query)
	words := strings.FieldsFunc(foldedName, func(r rune) bool {
		return IsSeparator(r)
	})
	if len(words) == 0 {
		return false
	}
	var b strings.Builder
	b.Grow(len(foldedName))
	for i, w := range words {
		if i > 0 {
			prev := words[i-1]
			if len([]rune(prev)) < 3 || len([]rune(w)) < 3 {
				// Short word: the boundary stays hard, so no
				// query can prefix-match across it.
				b.WriteByte('-')
			}
		}
		b.WriteString(w)
	}
	return strings.HasPrefix(b.String(), foldedQuery)
}
