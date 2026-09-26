package fuzzy

import (
	"testing"
)

// ---------------------------------------------------------------------
// RED tests for the shared fuzzy matcher (plan item fuzzy-shared,
// gotchas G10 + G11). Every test here fails today against the stub:
// the stub returns TierNone/0/false, so "want ok" rows fail as
// assertions and the ordering rows fail because every tier is 0.
// ---------------------------------------------------------------------

// The tier constants themselves must keep the channelfinder ordering:
// prefix ranks better (numerically lower) than substring, which ranks
// better than subsequence. Consumers (channelfinder.filter, the
// emoji/reaction pickers) sort by tier ascending; a reorder here
// reorders every picker at once.
func TestTierConstants_OrderPrefixBeatsSubstringBeatsSubsequence(t *testing.T) {
	if !(TierNone < TierPrefix && TierPrefix < TierSubstring && TierSubstring < TierSubsequence) {
		t.Fatalf("tier ordering broken: none=%d prefix=%d substring=%d subsequence=%d",
			TierNone, TierPrefix, TierSubstring, TierSubsequence)
	}
}

func TestMatch_Prefix(t *testing.T) {
	tier, _, ok := Match("engineering", "eng")
	if !ok {
		t.Fatal(`Match("engineering", "eng") did not match; want a prefix match`)
	}
	if tier != TierPrefix {
		t.Errorf(`Match("engineering", "eng") tier = %v, want TierPrefix`, tier)
	}
}

// "tomo" is the substring example straight out of channelfinder's doc
// comment (model.go:384): not a prefix of "ext-automote", but it
// occurs contiguously inside.
func TestMatch_Substring(t *testing.T) {
	tier, _, ok := Match("ext-automote", "tomo")
	if !ok {
		t.Fatal(`Match("ext-automote", "tomo") did not match; want a substring match`)
	}
	if tier != TierSubstring {
		t.Errorf(`Match("ext-automote", "tomo") tier = %v, want TierSubstring`, tier)
	}
}

// G10: the subsequence tier is IN-ORDER. "csp" matches
// "cs-product-triage" because c, s, p appear in order.
func TestMatch_SubsequenceInOrder(t *testing.T) {
	tier, _, ok := Match("cs-product-triage", "csp")
	if !ok {
		t.Fatal(`Match("cs-product-triage", "csp") did not match; want an in-order subsequence match`)
	}
	if tier != TierSubsequence {
		t.Errorf(`Match("cs-product-triage", "csp") tier = %v, want TierSubsequence`, tier)
	}
}

// G10 negative test: "krt" must NOT match "rocket". r appears before
// k in "rocket", so an in-order subsequence walk dies on the k. A
// bag-of-characters matcher (the DOD's original "out-of-order"
// wording) would pass this row — that is the exact ambiguity the
// gotcha exists to kill.
func TestMatch_OutOfOrderIsNotASubsequence(t *testing.T) {
	_, _, ok := Match("rocket", "krt")
	if ok {
		t.Fatal(`Match("rocket", "krt") matched; subsequence semantics are IN-ORDER (G10) — a match here means bag-of-chars matching`)
	}
}

// rkt -> rocket, verified against real emoji.BuildEntries output
// (internal/emoji probe, 1972 entries): "rocket" IS one of the 12
// in-order subsequence matches for "rkt".
func TestMatch_RocketMatchesRKT(t *testing.T) {
	_, _, ok := Match("rocket", "rkt")
	if !ok {
		t.Fatal(`Match("rocket", "rkt") did not match; want an in-order subsequence match`)
	}
}

// Case- and accent-folding is part of the matcher's contract
// (channelfinder folds both sides via text.Fold before matching).
func TestMatch_FoldsCaseAndAccents(t *testing.T) {
	for _, tc := range []struct{ name, query string }{
		{"Engineering", "ENG"}, // case only
		{"Mélanie", "melanie"}, // accent on the name side
		{"MÉLANIE", "melanie"}, // both, via the fold
	} {
		if _, _, ok := Match(tc.name, tc.query); !ok {
			t.Errorf("Match(%q, %q) did not match; want a folded prefix match", tc.name, tc.query)
		}
	}
}

// Empty query matches nothing through Match. (Callers treat the
// empty query as "show everything / recents" before ever consulting
// the tiers; the matcher must not silently turn "" into a prefix
// match against every name.)
func TestMatch_EmptyQueryIsNotAMatch(t *testing.T) {
	if _, _, ok := Match("rocket", ""); ok {
		t.Fatal(`Match("rocket", "") matched; the empty query must be TierNone`)
	}
}

// Empty name is a candidate no real list produces but a robust
// matcher must survive (worst-first): no tier can match it.
func TestMatch_EmptyNameIsNotAMatch(t *testing.T) {
	if _, _, ok := Match("", "rkt"); ok {
		t.Fatal(`Match("", "rkt") matched; an empty name must be TierNone`)
	}
}

// A query LONGER than the name can never be a subsequence.
func TestMatch_QueryLongerThanNameIsNotAMatch(t *testing.T) {
	if _, _, ok := Match("rk", "rocket"); ok {
		t.Fatal(`Match("rk", "rocket") matched; a longer query can never be a subsequence`)
	}
}

// Unicode both ways: a query of multibyte runes must fold and match
// against a multibyte name. emoji shortcodes are ASCII, but custom
// emoji names are workspace-defined and Slack permits some non-ASCII.
func TestMatch_UnicodeQueryRoundTrip(t *testing.T) {
	if _, _, ok := Match("日本語チャンネル", "日本"); !ok {
		t.Error(`Match("日本語チャンネル", "日本") did not match; want a folded prefix match`)
	}
}

// Prefix always outranks substring, which always outranks
// subsequence — regardless of score. This is the ordering the emoji
// tier tests rely on; pinning it here keeps it testable without a
// picker.
func TestMatch_TierOrderingBeatsScore(t *testing.T) {
	// "rocket" as a prefix of itself must classify as TierPrefix...
	if tier, _, ok := Match("rocket", "rocket"); !ok || tier != TierPrefix {
		t.Fatalf(`Match("rocket", "rocket") = tier %v ok %v, want TierPrefix/true (exact match is a prefix)`, tier, ok)
	}
	// ...and "rocket" inside "rock-et" is a subsequence (the '-'
	// breaks contiguity) even though its raw score would be high.
	if tier, _, ok := Match("rock-et", "rocket"); !ok || tier != TierSubsequence {
		t.Fatalf(`Match("rock-et", "rocket") = tier %v ok %v, want TierSubsequence/true`, tier, ok)
	}
}

// SubsequenceScore, pinned on the examples channelfinder's own doc
// comment uses. Exact values are deliberately NOT pinned — the score
// is an internal ranking signal — but the two properties that make
// it score-aware (G11) are: word-boundary hits beat interior hits,
// and tighter spans beat looser ones.
func TestSubsequenceScore_WordBoundaryBeatsInterior(t *testing.T) {
	// "rock": r on the start-of-name boundary (+25) and k three rows
	// later; "mirak": the same r..k span lands interior. Channelfinder
	// semantics: 10+25+10 + 50*2/4 = 70 vs 10+10 + 50*2/3 = 53.
	boundary, okB := SubsequenceScore("rock", "rk")
	interior, okI := SubsequenceScore("mirak", "rk")
	if !okB || !okI {
		t.Fatalf("SubsequenceScore ok flags: boundary=%v interior=%v; both must match", okB, okI)
	}
	if boundary <= interior {
		t.Errorf("word-boundary subsequence score %d must beat interior %d", boundary, interior)
	}
}

func TestSubsequenceScore_TighterBeatsLooser(t *testing.T) {
	// Both matches start on the same boundary rune; only the span
	// differs (2 vs 10): 45+50 = 95 vs 45+10 = 55.
	tight, okT := SubsequenceScore("rkkkk", "rk")
	loose, okL := SubsequenceScore("rxxxxxxxxk", "rk")
	if !okT || !okL {
		t.Fatalf("SubsequenceScore ok flags: tight=%v loose=%v; both must match", okT, okL)
	}
	if tight <= loose {
		t.Errorf("tight subsequence score %d must beat loose %d", tight, loose)
	}
}

func TestSubsequenceScore_OutOfOrderReturnsFalse(t *testing.T) {
	if _, ok := SubsequenceScore("rocket", "krt"); ok {
		t.Fatal(`SubsequenceScore("rocket", "krt") = ok; in-order semantics mean no match (G10)`)
	}
}

func TestSubsequenceScore_EmptyQueryReturnsFalse(t *testing.T) {
	if _, ok := SubsequenceScore("rocket", ""); ok {
		t.Fatal(`SubsequenceScore("rocket", "") = ok; an empty query must not be a subsequence match`)
	}
}

// WordPrefix / SquashedPrefix — the mentionpicker ranks
// (match.go:15-22), exposed so the picker consumes one matcher
// instead of its private copy.

func TestWordPrefix_MatchesWordInsideName(t *testing.T) {
	// matchName's own doc example: "widg" matches "eng-widgets".
	if !WordPrefix("eng-widgets", "widg") {
		t.Error(`WordPrefix("eng-widgets", "widg") = false; want true`)
	}
}

func TestWordPrefix_WholeNamePrefixIsNotAWordPrefix(t *testing.T) {
	// "eng" is a prefix of the WHOLE name; the word rank exists to
	// rank it BELOW the prefix rank, so WordPrefix itself must answer
	// false (Match already reported the prefix tier for it).
	if WordPrefix("eng-widgets", "eng") {
		t.Error(`WordPrefix("eng-widgets", "eng") = true; a whole-name prefix belongs to the prefix rank, not the word rank`)
	}
}

func TestWordPrefix_FoldsCase(t *testing.T) {
	if !WordPrefix("Eng-Widgets", "WIDG") {
		t.Error(`WordPrefix("Eng-Widgets", "WIDG") = false; want true (folded)`)
	}
}

func TestWordPrefix_NoMatch(t *testing.T) {
	if WordPrefix("eng-widgets", "dget") { // "dget" is not a word prefix
		t.Error(`WordPrefix("eng-widgets", "dget") = true; want false (not a word boundary start)`)
	}
}

func TestSquashedPrefix_MatchesWithSeparatorsRemoved(t *testing.T) {
	// matchName's own doc example: "engwidgets" matches "eng-widgets".
	if !SquashedPrefix("eng-widgets", "engwidgets") {
		t.Error(`SquashedPrefix("eng-widgets", "engwidgets") = false; want true`)
	}
}

func TestSquashedPrefix_FoldsCase(t *testing.T) {
	if !SquashedPrefix("eng_widgets", "ENGWIDGETS") {
		t.Error(`SquashedPrefix("eng_widgets", "ENGWIDGETS") = false; want true (folded)`)
	}
}

func TestSquashedPrefix_NoMatch(t *testing.T) {
	if SquashedPrefix("eng-widgets", "widgetseng") {
		t.Error(`SquashedPrefix("eng-widgets", "widgetseng") = true; want false (not a squashed prefix)`)
	}
}

func TestSquashedPrefix_EmptyQueryIsFalse(t *testing.T) {
	if SquashedPrefix("eng-widgets", "") {
		t.Error(`SquashedPrefix("eng-widgets", "") = true; want false`)
	}
	if WordPrefix("eng-widgets", "") {
		t.Error(`WordPrefix("eng-widgets", "") = true; want false`)
	}
}
