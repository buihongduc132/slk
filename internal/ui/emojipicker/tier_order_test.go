package emojipicker

import (
	"testing"
)

// B33: emojipicker had ZERO ordering coverage for TierWordPrefix and
// TierSquashedPrefix -- two of the five real tiers.
//
// B30 pinned the tier CONSTANTS by value (internal/fuzzy/tier_constants_test.go).
// That stops a renumber; it is not the same property as "this consumer ranks by
// them". A consumer could collapse two tiers, or ignore one, and every constant
// would still be correct. The only tier references in this package's tests were
// inside TestFuzzy_TierOrderPrefixBeatsSubstringBeatsSubsequence and
// TestFuzzy_SubsequenceTierIsScoreAware, neither of which names the two middle
// tiers -- so a change that mis-tiered a word-boundary match reordered the picker
// for every user and failed nothing.
//
// This matters most for CUSTOM emoji, whose names are hyphenated by convention
// (`party-parrot`, `cmd-pallet-worktree`), making WordPrefix the common real case
// rather than an exotic one.
//
// ---------------------------------------------------------------------------
// HOW THESE ASSERT, and why the first draft of this file was worthless
// ---------------------------------------------------------------------------
//
// The first version of these tests called fuzzy.Match itself to compute each
// row's tier, then asserted the rows were in non-decreasing tier order. That is
// vacuous: it compares the matcher against itself. Collapsing TierWordPrefix and
// TierSquashedPrefix into TierSubstring *inside model.filter* left all four tests
// GREEN, because the helper never observed what the model computed.
//
// These assert on row ORDER instead -- the only thing the model actually
// publishes. The fixtures exploit the model's documented tie-break
// (`matches[i].idx < matches[j].idx`, "preserve input order") by listing the
// WORSE-tier entry FIRST. Correct tiering promotes the better-tier entry past
// it; a collapse ties them, input order wins, and the assertion fails. Verified
// RED against exactly that mutation.
//
// Fixtures come from entriesFor, i.e. the real shipped codemap, and every tier
// claim below was measured against emoji.BuildEntries rather than guessed.

// WordPrefix must outrank Substring in the published order.
//
// Query "ice": shaved_ice is TierWordPrefix ("ice" follows "_"), ear_of_rice is
// TierSubstring ("ice" is interior, no boundary). ear_of_rice is listed FIRST so
// the tie-break favours it if the tiers ever stop differing.
func TestFilter_WordPrefixOutranksSubstring_ByRowOrder(t *testing.T) {
	entries := entriesFor(t, "ear_of_rice", "shaved_ice")
	m := New()
	m.SetEntries(entries)
	m.SetQuery("ice")

	names := filteredNames(m)
	if len(names) != 2 {
		t.Fatalf(`query "ice" returned %d rows (%v), want both fixtures; cannot compare order`, len(names), names)
	}
	if names[0] != "shaved_ice" {
		t.Errorf(`query "ice": rows are %v, want shaved_ice first. shaved_ice is TierWordPrefix and `+
			`ear_of_rice is TierSubstring, so the word-boundary match must rank ahead. Getting `+
			`ear_of_rice first means the two tiers are being treated as equal and the input-order `+
			`tie-break decided it (B33).`, names)
	}
}

// Prefix must outrank WordPrefix, with the same construction: the word-prefix
// entries are listed first, so a collapse would leave one of them leading.
func TestFilter_PrefixOutranksWordPrefix_ByRowOrder(t *testing.T) {
	entries := entriesFor(t, "no_bell", "bell")
	m := New()
	m.SetEntries(entries)
	m.SetQuery("bell")

	names := filteredNames(m)
	if len(names) != 2 {
		t.Fatalf(`query "bell" returned %d rows (%v), want both fixtures`, len(names), names)
	}
	if names[0] != "bell" {
		t.Errorf(`query "bell": rows are %v, want bell first. "bell" is a whole-name prefix `+
			`(TierPrefix) and "no_bell" is only a word prefix (TierWordPrefix).`, names)
	}
}

// Substring must outrank Subsequence. Guards the boundary on the other side of
// the two middle tiers, so a collapse in either direction is caught.
//
// Query "corn": popcorn is TierSubstring (contiguous), accordion is
// TierSubsequence (c..o..r..n scattered). accordion is listed first.
func TestFilter_SubstringOutranksSubsequence_ByRowOrder(t *testing.T) {
	entries := entriesFor(t, "accordion", "popcorn")
	m := New()
	m.SetEntries(entries)
	m.SetQuery("corn")

	names := filteredNames(m)
	if len(names) != 2 {
		t.Fatalf(`query "corn" returned %d rows (%v), want both fixtures`, len(names), names)
	}
	if names[0] != "popcorn" {
		t.Errorf(`query "corn": rows are %v, want popcorn first. popcorn is TierSubstring and `+
			`accordion is only TierSubsequence.`, names)
	}
}

// The full chain on one query, as published order. This is what fails if any
// tier is dropped or collapsed by a future change.
//
// Query "ice", measured against the real codemap:
//
//	ice_cream    TierPrefix
//	shaved_ice   TierWordPrefix
//	ear_of_rice  TierSubstring
//	lorry-ish    TierSubsequence  (articulated_lorry: i..c..e in order)
//
// Listed in reverse of the expected order, so every pairwise relation has to be
// established by tiering rather than by input position.
func TestFilter_FullTierChain_ByRowOrder(t *testing.T) {
	entries := entriesFor(t, "articulated_lorry", "ear_of_rice", "shaved_ice", "ice_cream")
	m := New()
	m.SetEntries(entries)
	m.SetQuery("ice")

	names := filteredNames(m)
	want := []string{"ice_cream", "shaved_ice", "ear_of_rice", "articulated_lorry"}
	if len(names) != len(want) {
		t.Fatalf(`query "ice" returned %d rows (%v), want %d`, len(names), names, len(want))
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("query \"ice\": row %d = %q, want %q. Full order was %v, want %v. "+
				"The fixtures are listed in REVERSE of this order, so any pair appearing "+
				"reversed means those two tiers are being treated as equal (B33).",
				i, names[i], want[i], names, want)
			break
		}
	}
}
