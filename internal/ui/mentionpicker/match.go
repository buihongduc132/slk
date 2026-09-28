package mentionpicker

import (
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

// matchName ranks name against an already-folded query.
//
// B28: this used to take a third parameter, squashedQuery, with the comment
// "squashedQuery is ignored now" — dead since the shared-matcher migration
// moved squashing inside fuzzy.SquashedPrefix. The parameter was still computed
// per filter() call by a local squash(), which was still backed by a local
// isSeparator(), and all three were still tested. Deleting the parameter
// deletes both helpers with it, and with them one of the four competing
// definitions of "word separator" that B27 catalogues: this one treated
// ' ' '-' '_' '.' as separators, where fuzzy.isSeparator also counts '/' and
// ':'. Squashing behaviour now has exactly one implementation, inside
// internal/fuzzy, covered by that package's TestSquashedPrefix_* tests.
func matchName(name, query string) matchRank {
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
func rankUser(u User, query string) matchRank {
	r := matchName(u.DisplayName, query)
	if ru := matchName(u.Username, query); ru < r {
		r = ru
	}
	return r
}
