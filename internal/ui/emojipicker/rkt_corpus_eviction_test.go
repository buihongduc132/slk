package emojipicker

import (
	"testing"

	"github.com/gammons/slk/internal/emoji"
)

// OT6: the DOD's `rkt` -> `rocket` example, characterised at the PICKER level
// against a corpus shaped like the real workspace.
//
// The existing fuzzy_test.go row for this example passes because its fixture
// holds 4 curated built-in entries. That is a true statement about a 4-entry
// corpus, not about a workspace. This test records what the live workspace
// actually produces (399 custom emoji, five `cmd-pallet-*worktree*` names):
// rocket is evicted, and the matcher is CORRECT to evict it.
//
// It asserts the eviction rather than the DOD example, because the eviction is
// what is true. See OT6 in flow/plans/slk-fullscreen-emoji-fuzzy.md for the
// measurement and for why the density-penalty alternative was rejected.
func TestOT6_RktEvictsRocketOnRealCorpusShape(t *testing.T) {
	// Five customs, as in the live workspace. BuildEntries sorts
	// alphabetically, so every `cmd-pallet-*` name precedes `rocket` -- and
	// input order is the picker's documented final tie-break.
	customs := map[string]string{
		"cmd-pallet-worktree":      "https://example.invalid/a.png",
		"cmd-pallet-worktree-list": "https://example.invalid/b.png",
		"cmd-pallet-worktree-new":  "https://example.invalid/c.png",
		"cmd-pallet-worktree-open": "https://example.invalid/d.png",
		"cmd-pallet-worktree-rm":   "https://example.invalid/e.png",
	}

	keep := map[string]bool{"rocket": true}
	for n := range customs {
		keep[n] = true
	}
	var entries []emoji.EmojiEntry
	for _, e := range emoji.BuildEntries(customs) {
		if keep[e.Name] {
			entries = append(entries, e)
		}
	}
	if len(entries) != len(keep) {
		t.Fatalf("fixture built %d entries, want %d; BuildEntries dropped a name", len(entries), len(keep))
	}
	if entries[len(entries)-1].Name != "rocket" {
		t.Fatalf("fixture order = %v; rocket must sort LAST for the tie-break premise to hold", entries)
	}

	m := New()
	m.SetEntries(entries)
	m.SetQuery("rkt")
	got := filteredNames(m)

	if len(got) != MaxVisible {
		t.Fatalf(`query "rkt" returned %d rows (%v), want MaxVisible=%d; the eviction premise needs a full window`, len(got), got, MaxVisible)
	}
	if containsName(got, "rocket") {
		t.Errorf(`query "rkt" surfaced rocket (rows %v). OT6 measured it EVICTED: the five `+
			`cmd-pallet-*worktree* names are TierSubstring(4) because "rkt" is contiguous inside `+
			`"worktree", while rocket is only TierSubsequence(5), and MaxVisible=%d fills up first. `+
			`If this now passes, the shared ranking rule changed -- confirm that was intended and `+
			`that channelfinder/mentionpicker/reactionpicker were re-checked, then update OT6.`,
			got, MaxVisible)
	}
}
