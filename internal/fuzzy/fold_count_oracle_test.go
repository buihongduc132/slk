package fuzzy

// B26 oracle: Match must fold each of its arguments ONCE, not once per tier.
//
// This file is the gate oracle for lane-fold and is HASH-PINNED. Do not edit it
// to make it pass. If you believe it asks the wrong question, say so in a report
// and leave the gate red; that is a successful outcome for this lane.
//
// ---------------------------------------------------------------------------
// THE DEFECT
//
// Match folds both arguments, then passes the FOLDED values into WordPrefix and
// SquashedPrefix -- each of which folds both arguments again (2 text.Fold calls
// apiece). On the deepest path the name is folded 3x and the query 3x per
// candidate. SubsequenceScore is correct and folds 0 times.
//
// internal/text/fold.go records what this costs and why it was removed once
// already (issue #165): the reaction picker went 271 ms / 1.12 GB / 768k allocs
// -> 2.96 ms / 611 B / 1 alloc when Fold stopped being called redundantly in the
// filter loop. This is that regression coming back in a different place.
//
// ---------------------------------------------------------------------------
// WHAT THE BATCH-2 APPENDIX GOT WRONG, MEASURED
//
// The appendix says the repeat folds hurt "non-ASCII candidates (accented custom
// emoji, display names like `Mélanie`)". That is the one case where they are
// FREE, because Fold's output for accented Latin is ASCII, so every re-fold hits
// the isASCII fast path. Measured with testing.AllocsPerRun(200):
//
//	input                   Fold   re-fold of its own output   Match total
//	engineering-platform      0              0                      0
//	Mélanie-Dupont            9              0  (output is ASCII)    9
//	日本語チャンネル             6              6  (output non-ASCII)  38
//	party_parrot_🦜            6              6                     12
//	Straße-Team               7              6                     13
//
// The waste appears only when the FOLDED FORM IS STILL NON-ASCII -- CJK, emoji,
// and ß, which Fold deliberately does not decompose (NFD, not NFKD). And it
// scales with tier depth, because each tier that runs folds twice more: the CJK
// row reaches TierSubstring, so both WordPrefix and SquashedPrefix run, and 38
// allocations is about six folds against a floor of two. The emoji and ß rows
// stop at TierWordPrefix and so show only ~2x.
//
// Worst case is therefore a MISS on a CJK/emoji/ß name, since a miss runs every
// tier -- and misses are most candidates on every keystroke, which is exactly
// the hot loop #165 was about.
//
// ---------------------------------------------------------------------------
// WHY THE ASSERTION IS SHAPED LIKE THIS
//
// The floor is MEASURED IN THE TEST, not written down as a constant: fold the
// name once and the query once via text.Fold and count that. Then Match must
// come within a small margin of it. Both sides are observed at run time, so the
// test cannot be satisfied by aliasing and does not rot when Go's allocator or
// x/text changes -- the ratio is what is being pinned, not any absolute number.
//
// This matters because an absolute bound ("Match must allocate <= 14") would
// have to be re-blessed on every toolchain bump, and the first person to do that
// re-blessing would have no way to tell a real regression from a drift.

import (
	"testing"

	"github.com/gammons/slk/internal/text"
)

// foldOracleCorpus holds inputs whose FOLDED form is still non-ASCII, which is
// the only situation where a redundant fold costs anything. An ASCII corpus
// would make this test vacuous -- every fold is 0 allocations there, so 1 fold
// and 3 folds are indistinguishable and the test would pass unfixed.
var foldOracleCorpus = []struct {
	name  string
	query string
	why   string
}{
	{"日本語チャンネル", "ネル", "CJK: no decomposition, folded form stays non-ASCII"},
	{"日本語チャンネル", "zzq", "CJK MISS: runs every tier, the real worst case"},
	{"party_parrot_🦜", "parrot", "emoji in the name, folded form stays non-ASCII"},
	{"party_parrot_🦜", "zzq", "emoji MISS: runs every tier"},
	{"Straße-Team", "team", "ß is deliberately not expanded (NFD, not NFKD)"},
	{"Straße-Team", "zzq", "ß MISS: runs every tier"},
}

// TestMatch_DoesNotRefoldItsArguments is the oracle.
//
// For each input it measures the unavoidable cost -- one fold of the name plus
// one fold of the query -- and then measures Match. Match does strictly more
// work than folding (it walks runes, compares prefixes), but that work does not
// ALLOCATE, so the allocation count of a correct Match should sit at the floor
// plus a small slack.
//
// Slack is 4 allocations, not 0: SubsequenceScore builds a []rune of the query
// when it runs, and the miss rows all reach it. The margin is deliberately far
// below the 2x-6x the redundant folds cost, so it cannot accidentally admit the
// defect.
func TestMatch_DoesNotRefoldItsArguments(t *testing.T) {
	const slack = 4

	for _, c := range foldOracleCorpus {
		name, query := c.name, c.query

		// The floor: exactly what Match cannot avoid doing.
		floor := testing.AllocsPerRun(200, func() {
			_ = text.Fold(name)
			_ = text.Fold(query)
		})

		got := testing.AllocsPerRun(200, func() {
			_, _, _ = Match(name, query)
		})

		if got > floor+slack {
			t.Errorf("Match(%q, %q) allocates %.0f, floor is %.0f (+%d slack) -- %s\n"+
				"  Match folds both arguments and then passes the FOLDED values into\n"+
				"  WordPrefix and SquashedPrefix, which fold them AGAIN. Fold the inputs\n"+
				"  once and give the tiers already-folded strings to work on.\n"+
				"  See internal/text/fold.go and issue #165 for what this cost last time.",
				name, query, got, floor, slack, c.why)
		}
	}
}

// TestMatch_ResultsAreUnchangedByTheFoldFix pins behaviour across the change.
//
// Folding is idempotent, so removing redundant folds MUST NOT alter a single
// answer. This is the regression half: it is GREEN before and must stay GREEN
// after. Without it, "fewer allocations" could be achieved by skipping a tier,
// which would be a silent ranking change dressed up as a performance fix.
//
// The expectations are the tiers Match returns TODAY, recorded by observation.
func TestMatch_ResultsAreUnchangedByTheFoldFix(t *testing.T) {
	cases := []struct {
		name     string
		query    string
		wantTier Tier
		wantOK   bool
	}{
		{"engineering-platform", "plat", TierWordPrefix, true},
		{"engineering-platform", "eng", TierPrefix, true},
		{"Mélanie-Dupont", "dup", TierWordPrefix, true},
		{"Mélanie-Dupont", "melanie", TierPrefix, true},
		{"日本語チャンネル", "ネル", TierSubstring, true},
		{"party_parrot_🦜", "parrot", TierWordPrefix, true},
		{"Straße-Team", "team", TierWordPrefix, true},
		{"cs-product-triage", "csp", TierSubsequence, true},
		{"rocket", "krt", TierNone, false},
		{"日本語チャンネル", "zzq", TierNone, false},
	}
	for _, c := range cases {
		tier, _, ok := Match(c.name, c.query)
		if ok != c.wantOK || tier != c.wantTier {
			t.Errorf("Match(%q, %q) = tier %d, ok %v; want tier %d, ok %v\n"+
				"  Removing a redundant fold cannot change an answer -- folding is\n"+
				"  idempotent. If this row moved, a tier stopped running.",
				c.name, c.query, tier, ok, c.wantTier, c.wantOK)
		}
	}
}

// BenchmarkMatch_NonASCIIMiss is the guard the appendix asked for.
//
// A miss on a CJK name is the worst case: every tier runs, and every tier that
// runs re-folds. Run it with -benchmem; allocs/op is the number that matters and
// the one that must not climb back.
func BenchmarkMatch_NonASCIIMiss(b *testing.B) {
	const name = "日本語チャンネル"
	const query = "zzq"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, _ = Match(name, query)
	}
}

// BenchmarkMatch_ASCIIMiss is the control. It should be ~0 allocs/op before and
// after: the ASCII fast path means redundant folds are free here, which is why
// no existing benchmark could see this defect.
func BenchmarkMatch_ASCIIMiss(b *testing.B) {
	const name = "engineering-platform"
	const query = "zzq"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, _ = Match(name, query)
	}
}
