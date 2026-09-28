package fuzzy

import "testing"

// Tier constants pinned BY VALUE, not merely by relative order (gotcha B30).
//
// This lives in its own file, and not beside the tier test it strengthens,
// for a procedural reason worth recording: `fuzzy_test.go` is a hash-pinned
// gate oracle (`~/.local/state/slkfz/gates/fuzzy/oracle.sha256`). The gate's
// contract is that the RED tests ARE the specification -- an implementing
// agent may not edit them, and if a test is genuinely wrong the agent must
// stop and say so rather than "fix" it. B30 is exactly the genuinely-wrong
// case, so the weak assertion stays where it is, byte-identical, and the
// strong one is added alongside. Additive is allowed; editing the oracle is
// not; re-pinning the recorded hash to match an edit would be the agent
// rewriting the gate to suit its own code, which is worse than either.
//
// What B30 found: TestTierConstants_OrderPrefixBeatsSubstringBeatsSubsequence
// asserts
//
//	TierNone < TierPrefix && TierPrefix < TierSubstring && TierSubstring < TierSubsequence
//
// which names 4 of the 6 constants. TierWordPrefix and TierSquashedPrefix are
// absent, so swapping those two -- or moving either across TierSubstring --
// leaves that test AND every consumer test green while silently reranking
// channelfinder and both emoji pickers. The two unpinned constants are the two
// the plan did not know existed: it describes the merged set as "4 tiers" where
// there are five, plus TierNone.
//
// RED-proven, not assumed: with TierWordPrefix and TierSquashedPrefix swapped
// in fuzzy.go, the old assertion exits 0 -- it cannot see the reorder -- while
// this test exits 1 reporting `TierWordPrefix = 3, want 2` and
// `TierSquashedPrefix = 2, want 3`.
//
// Why by value and not just by order: consumers do arithmetic on these.
// channelfinder stored `int(tier) - 1` against a stale 3-tier numbering (B25,
// since deleted), which a renumber would have corrupted with nothing failing.
// Values are the contract precisely because a consumer can depend on them
// without any test naming the dependency.
func TestTierConstants_ExactValuesAndFullOrder(t *testing.T) {
	for _, c := range []struct {
		name string
		got  Tier
		want Tier
	}{
		{"TierNone", TierNone, 0},
		{"TierPrefix", TierPrefix, 1},
		{"TierWordPrefix", TierWordPrefix, 2},
		{"TierSquashedPrefix", TierSquashedPrefix, 3},
		{"TierSubstring", TierSubstring, 4},
		{"TierSubsequence", TierSubsequence, 5},
	} {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d. Renumbering a tier reranks every picker at once, and a consumer may do arithmetic on these values, so a renumber can corrupt it silently.", c.name, c.got, c.want)
		}
	}

	// Every adjacent pair, so no constant can cross another even if the
	// by-value block above is ever relaxed.
	chain := []struct {
		name string
		tier Tier
	}{
		{"TierNone", TierNone},
		{"TierPrefix", TierPrefix},
		{"TierWordPrefix", TierWordPrefix},
		{"TierSquashedPrefix", TierSquashedPrefix},
		{"TierSubstring", TierSubstring},
		{"TierSubsequence", TierSubsequence},
	}
	for i := 1; i < len(chain); i++ {
		if !(chain[i-1].tier < chain[i].tier) {
			t.Errorf("%s (%d) must rank better than %s (%d); lower is better and consumers sort ascending",
				chain[i-1].name, chain[i-1].tier, chain[i].name, chain[i].tier)
		}
	}
}
