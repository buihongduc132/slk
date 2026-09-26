package mentionpicker

import (
	"strings"

	"github.com/gammons/slk/internal/fuzzy"
)

// matchRank describes how well a candidate name matches the query.
// Lower is better: the picker sorts by rank before name, so a whole-name
// prefix hit outranks a hit on a word in the middle of the name.
//
// The three matching modes mirror what Slack's own composer does — typing
// "widg" finds @eng-widgets, and so does "engwidgets".
type matchRank int

const (
	rankPrefix   matchRank = iota // query is a prefix of the whole name
	rankWord                      // query is a prefix of a word inside the name
	rankSquashed                  // query matches once separators are removed
	rankNone                      // no match
)

// isSeparator reports whether b separates words in a display name or
// handle. Slack handles allow "-", "_" and "."; display names add
// spaces. All are ASCII, so byte-level tests are safe on UTF-8 input —
// no continuation byte of a multi-byte rune can collide with them.
func isSeparator(b byte) bool {
	return b == ' ' || b == '-' || b == '_' || b == '.'
}

// squash removes every separator from s, so "eng-widgets" and
// "engwidgets" compare equal.
func squash(s string) string {
	if !strings.ContainsAny(s, " -_.") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if !isSeparator(s[i]) {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// matchName ranks name against an already-folded query. squashedQuery is ignored now.
func matchName(name, query, squashedQuery string) matchRank {
	if query == "" {
		return rankPrefix
	}
	tier, _, ok := fuzzy.Match(name, query)
	if ok && tier == fuzzy.TierPrefix {
		return rankPrefix
	}
	if fuzzy.WordPrefix(name, query) {
		return rankWord
	}
	if fuzzy.SquashedPrefix(name, query) {
		return rankSquashed
	}
	return rankNone
}

// rankUser returns the better of the user's display-name and username
// ranks.
func rankUser(u User, query, squashedQuery string) matchRank {
	r := matchName(u.DisplayName, query, squashedQuery)
	if ru := matchName(u.Username, query, squashedQuery); ru < r {
		r = ru
	}
	return r
}
