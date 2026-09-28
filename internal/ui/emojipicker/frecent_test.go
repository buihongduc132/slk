package emojipicker

import (
	"testing"

	"github.com/gammons/slk/internal/core"
)

// B24: the compose emoji dropdown implemented four of the DOD's five
// ranking tiers. `fuzzy-emoji` promises "recent (frecent) > prefix >
// substring > subsequence" for BOTH the compose autocomplete and the
// reaction picker; reactionpicker had the recent tier, emojipicker had
// no frecent field at all, so the two emoji surfaces disagreed in their
// top rows under a MaxVisible=5 cap.
//
// FIXTURE TIERS, measured against emoji.BuildEntries(nil) for query
// "ice" (not guessed — the probe printed fuzzy.Match for each):
//
//	ice_cream          TierPrefix       (1)
//	shaved_ice         TierWordPrefix   (2)
//	ear_of_rice        TierSubstring    (4)
//	articulated_lorry  TierSubsequence  (5), score 51
//
// The recency assertions below make a WORSE-tier entry frecent and
// require it to outrank a better-tier one. That direction matters: a
// test that made the already-winning entry frecent would pass without
// any recent tier existing.

// A frecent entry must outrank a better-tier non-frecent entry.
// ear_of_rice is TierSubstring and ice_cream is TierPrefix, so ordering
// ear_of_rice first is only possible if recency is consulted BEFORE
// tier. Input order lists ice_cream first, so the documented
// input-order tie-break also favours it — the frecent tier has to beat
// both.
func TestFrecent_OutranksBetterTierMatch(t *testing.T) {
	entries := entriesFor(t, "ice_cream", "shaved_ice", "ear_of_rice")
	m := New()
	m.SetEntries(entries)
	m.SetFrecentEmoji([]core.EmojiEntry{{Name: "ear_of_rice", Unicode: "x"}})
	m.SetQuery("ice")

	names := filteredNames(m)
	if len(names) != 3 {
		t.Fatalf(`query "ice" returned %d rows (%v), want 3`, len(names), names)
	}
	if names[0] != "ear_of_rice" {
		t.Errorf(`query "ice": rows are %v, want ear_of_rice first. ear_of_rice is only `+
			`TierSubstring but it IS frecent, and ice_cream is TierPrefix but is not. `+
			`The DOD orders "recent > prefix > substring > subsequence", so recency `+
			`must be consulted before tier (B24).`, names)
	}
}

// Among several frecent matches the recent tier orders by the frecency
// list's own order (most-frecent first), not by tier and not by the
// alphabetical input order. LoadFrecent already returns rows sorted by
// the cache's use_count/recency decay, so the picker must preserve that
// sequence rather than re-sorting it.
func TestFrecent_OrdersByFrecencyRankAmongItself(t *testing.T) {
	entries := entriesFor(t, "ice_cream", "shaved_ice", "ear_of_rice")
	m := New()
	m.SetEntries(entries)
	// Deliberately the reverse of both tier order and input order.
	m.SetFrecentEmoji([]core.EmojiEntry{
		{Name: "ear_of_rice", Unicode: "x"},
		{Name: "shaved_ice", Unicode: "x"},
	})
	m.SetQuery("ice")

	names := filteredNames(m)
	want := []string{"ear_of_rice", "shaved_ice", "ice_cream"}
	if len(names) != len(want) {
		t.Fatalf(`query "ice" returned %d rows (%v), want %d`, len(names), names, len(want))
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("query \"ice\": row %d = %q, want %q (order = %v, want %v). The two "+
				"frecent entries must lead in FRECENCY order, then the non-frecent "+
				"prefix match.", i, names[i], want[i], names, want)
			break
		}
	}
}

// The recent tier is frecent INTERSECT matches (gotcha G11: "recent =
// matching entries ∩ frecent only"). frecent_emoji is a global cache
// table with no team column, so it names emoji that need not match the
// query — those must not be injected as rows.
func TestFrecent_NonMatchingFrecentIsExcluded(t *testing.T) {
	entries := entriesFor(t, "ice_cream", "shaved_ice")
	m := New()
	m.SetEntries(entries)
	m.SetFrecentEmoji([]core.EmojiEntry{{Name: "rocket", Unicode: "x"}})
	m.SetQuery("ice")

	names := filteredNames(m)
	for _, n := range names {
		if n == "rocket" {
			t.Fatalf(`frecent entry "rocket" appeared for query "ice" (rows = %v); the `+
				`recent tier is the INTERSECTION with query matches, not a prepended list`, names)
		}
	}
	if len(names) != 2 || names[0] != "ice_cream" {
		t.Errorf(`query "ice" rows = %v, want the two matching entries with ice_cream first`, names)
	}
}

// A frecent name the entry list does not carry must not become a row.
// emojipicker renders from its own entries (built-ins + THIS workspace's
// customs); a global frecent row naming another workspace's custom has
// no Display glyph here and, unlike reactionpicker, inserting it into
// compose would produce a shortcode that does not resolve in this
// workspace. Skipped rather than drawn.
func TestFrecent_UnknownFrecentNameIsNotInjected(t *testing.T) {
	entries := entriesFor(t, "ice_cream")
	m := New()
	m.SetEntries(entries)
	m.SetFrecentEmoji([]core.EmojiEntry{{Name: "some_other_workspace_custom", Unicode: "x"}})
	m.SetQuery("ice")

	names := filteredNames(m)
	if len(names) != 1 || names[0] != "ice_cream" {
		t.Errorf(`rows = %v, want only ice_cream; a frecent name absent from the entry `+
			`list must not be injected as a row`, names)
	}
}

// ---------------------------------------------------------------------
// NO-OP DEFAULT. Every existing test in this package, and every new
// user's first run, has empty frecency. Ranking there must be
// byte-identical to the pre-frecent behaviour, which is what makes this
// change safe for tier_order_test.go's fixtures (they list the
// WORSE-tier entry first specifically so a tier collapse flips the
// answer — an empty recent tier must not flip anything).
// ---------------------------------------------------------------------

// With no frecency history the full documented tier chain is unchanged.
// Fixtures are listed in REVERSE of the expected order, so every
// pairwise relation is established by tiering rather than input
// position.
func TestFrecent_EmptyHistoryIsNoOp(t *testing.T) {
	want := []string{"ice_cream", "shaved_ice", "ear_of_rice", "articulated_lorry"}

	for _, tc := range []struct {
		name string
		set  func(m *Model)
	}{
		{"never set", func(m *Model) {}},
		{"set nil", func(m *Model) { m.SetFrecentEmoji(nil) }},
		{"set empty", func(m *Model) { m.SetFrecentEmoji([]core.EmojiEntry{}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := entriesFor(t, "articulated_lorry", "ear_of_rice", "shaved_ice", "ice_cream")
			m := New()
			m.SetEntries(entries)
			tc.set(&m)
			m.SetQuery("ice")

			names := filteredNames(m)
			if len(names) != len(want) {
				t.Fatalf("rows = %v, want %v", names, want)
			}
			for i := range want {
				if names[i] != want[i] {
					t.Errorf("row %d = %q, want %q (order = %v, want %v). With no frecency "+
						"history ranking must be identical to the tier-only behaviour.",
						i, names[i], want[i], names, want)
					break
				}
			}
		})
	}
}

// The empty-history no-op must hold for the rendered frame too, not just
// the row order: the recent tier must add no chrome, no marker and no
// reordering that View() could surface.
func TestFrecent_EmptyHistoryRendersIdentically(t *testing.T) {
	entries := entriesFor(t, "articulated_lorry", "ear_of_rice", "shaved_ice", "ice_cream")

	base := New()
	base.SetEntries(entries)
	base.Open("ice")
	wantFrame := base.View(40)

	withEmpty := New()
	withEmpty.SetEntries(entries)
	withEmpty.SetFrecentEmoji([]core.EmojiEntry{})
	withEmpty.Open("ice")
	gotFrame := withEmpty.View(40)

	if gotFrame != wantFrame {
		t.Errorf("empty-frecency frame differs from the no-frecency frame.\ngot:\n%s\nwant:\n%s", gotFrame, wantFrame)
	}
}
