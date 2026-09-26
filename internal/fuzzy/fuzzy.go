// internal/fuzzy/fuzzy.go
//
// Shared fuzzy matcher (plan slk-fullscreen-emoji-fuzzy, item
// fuzzy-shared). RED-phase stub: signatures are FINAL, logic is not —
// every exported function returns its zero value so the tests in
// fuzzy_test.go fail as assertions, not build errors. The GREEN
// implementation derives its semantics from
// internal/ui/channelfinder/model.go:388-483 (prefix > substring >
// in-order subsequence, word-boundary + tightness scoring) and
// internal/ui/mentionpicker/match.go (word / squashed prefix ranks).
//
// The extracted package is the single implementation channelfinder,
// mentionpicker, emojipicker and reactionpicker consume.
package fuzzy

// Tier classifies how well a folded query matches a folded candidate
// name. Lower numeric values rank BETTER, mirroring the match tier
// ordering channelfinder.filter already uses (tier 0 prefix, 1
// substring, 2 subsequence).
type Tier int

const (
	// TierNone means the query does not match the name at all.
	TierNone Tier = iota
	// TierPrefix means the query is a prefix of the folded name
	// (e.g. "eng" matches "engineering").
	TierPrefix
	// TierSubstring means the query occurs contiguously inside the
	// folded name but not at the start (e.g. "tomo" matches
	// "ext-automote").
	TierSubstring
	// TierSubsequence means every rune of the folded query appears in
	// the folded name IN ORDER, non-contiguously (e.g. "csp" matches
	// "cs-product-triage"). Out-of-order queries ("krt" vs "rocket")
	// do NOT match — this is in-order subsequence semantics, per
	// gotcha G10.
	TierSubsequence
)

// Match reports the best tier for query against name, plus a score
// that ranks matches WITHIN the tier (only meaningful for
// TierSubsequence, where it rewards word-boundary hits and tightness;
// 0 otherwise). ok is false when the tier is TierNone.
//
// Both name and query are case- and accent-folded internally
// (text.Fold semantics), so callers pass raw strings.
func Match(name, query string) (tier Tier, score int, ok bool) {
	_ = name
	_ = query
	return TierNone, 0, false
}

// SubsequenceScore returns the subsequence score for an in-order
// match of query inside name, and whether query is a subsequence of
// name at all. Score components mirror channelfinder.subsequenceScore:
// +10 per matched rune, +25 per match landing on a word boundary
// (start of name or after one of '-', '_', '.', ' ', '/', ':'), plus a
// tightness bonus up to ~50 for minimal first-to-last span. Both
// inputs are expected already folded (matching the call shape the
// extract leaves behind in channelfinder).
func SubsequenceScore(name, query string) (int, bool) {
	_ = name
	_ = query
	return 0, false
}

// WordPrefix reports whether the folded query is a prefix of a whole
// word inside the folded name. Word boundaries are the
// mentionpicker separators: ' ', '-', '_', '.' (e.g. "widg" matches
// "eng-widgets"). An empty query is not a word prefix.
func WordPrefix(name, query string) bool {
	_ = name
	_ = query
	return false
}

// SquashedPrefix reports whether the folded query is a prefix of the
// folded name with every separator (' ', '-', '_', '.') removed
// (e.g. "engwidgets" matches "eng-widgets"). An empty query is not a
// squashed prefix.
func SquashedPrefix(name, query string) bool {
	_ = name
	_ = query
	return false
}
