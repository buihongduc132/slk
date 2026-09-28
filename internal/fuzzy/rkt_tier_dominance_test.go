package fuzzy

import "testing"

// OT6: why the plan's `rkt` -> `rocket` DOD example is unreachable, pinned as
// behaviour rather than left as prose in the plan.
//
// These tests do NOT assert that rocket wins. They assert the two mechanical
// facts that make it LOSE, so that a future attempt to "fix" the example by
// reintroducing a length/density penalty fails here and has to read this
// comment first.
//
// Measured (see OT6 in flow/plans/slk-fullscreen-emoji-fuzzy.md):
//
//	Match("rocket",              "rkt") -> TierSubsequence(5), score 80
//	Match("worktree",            "rkt") -> TierSubstring(4),   score 0
//	Match("cmd-pallet-worktree", "rkt") -> TierSubstring(4),   score 0
//
// Both consumers (emojipicker.filter, reactionpicker.filter) compare tier
// FIRST and consult score only when tiers are equal AND the tier is
// TierSubsequence. So a substring match at tier 4 beats a subsequence match at
// tier 5 before score is ever read -- which is exactly what the hash-pinned
// TestMatch_TierOrderingBeatsScore codifies.
//
// Why this file exists as a tripwire: the plan offered a density penalty as the
// alternative to dropping the example. It was implemented and measured, and it
// does not work -- see OT6 for the two independent reasons. Do not re-derive it.

// The tier split is the whole mechanism. `rkt` is contiguous inside
// w-o-[rkt]-r-e-e, and only scattered inside r-o-c-k-e-t.
func TestOT6_RktTiersAgainstWorktreeAndRocket(t *testing.T) {
	for _, c := range []struct {
		name string
		want Tier
		why  string
	}{
		{"rocket", TierSubsequence, "r..k..t are scattered: r-o-c-k-e-t"},
		{"worktree", TierSubstring, "rkt is contiguous: w-o-rkt-r-e-e"},
		{"cmd-pallet-worktree", TierSubstring, "same contiguity, real custom-emoji shape"},
	} {
		got, _, ok := Match(c.name, "rkt")
		if !ok {
			t.Errorf("Match(%q, \"rkt\") did not match at all; want %v (%s)", c.name, c.want, c.why)
			continue
		}
		if got != c.want {
			t.Errorf("Match(%q, \"rkt\") tier = %v, want %v (%s). "+
				"A demotion rule that moves a long incidental substring out of "+
				"TierSubstring lands here. Read OT6 before changing this: the "+
				"demotion was measured and does NOT rescue rocket, because the "+
				"demoted name then TIES rocket on score and wins the "+
				"alphabetical input-order tie-break anyway.",
				c.name, got, c.want, c.why)
		}
	}
}

// The score channel cannot separate them. Both names score IDENTICALLY as
// subsequences, so demoting worktree into rocket's tier produces a tie, not a
// win -- and the picker's final tie-break is input order, which is
// alphabetical, where cmd-pallet-* precedes rocket.
//
// The magnitude is deliberately not pinned (the hash-pinned oracle states the
// score is an internal ranking signal); only the EQUALITY is, because the
// equality is the load-bearing fact.
func TestOT6_SubsequenceScoreCannotSeparateWorktreeFromRocket(t *testing.T) {
	rocket, okR := SubsequenceScore("rocket", "rkt")
	worktree, okW := SubsequenceScore("worktree", "rkt")
	if !okR || !okW {
		t.Fatalf("both must be in-order subsequence matches: rocket=%v worktree=%v", okR, okW)
	}
	if rocket != worktree {
		t.Errorf("SubsequenceScore for \"rkt\": rocket = %d, worktree = %d; OT6 measured these EQUAL. "+
			"If you have just made them differ, you are part-way through the density penalty OT6 "+
			"rejected -- and breaking this tie is necessary but NOT sufficient, because bare "+
			"\"worktree\" still outranks rocket on TIER. Re-read OT6.", rocket, worktree)
	}
}
