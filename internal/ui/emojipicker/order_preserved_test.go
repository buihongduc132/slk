package emojipicker

import (
	"testing"

	"github.com/gammons/slk/internal/emoji"
)

// filter()'s doc comment used to assert, in prose, that "callers must pass
// alphabetically-sorted entries (emoji.BuildEntries already does); the
// picker preserves that order." The first half is a precondition on the
// caller; the second half is a property of this package, and it was
// unchecked. These tests check it.
//
// Deliberately NOT a duplicate of internal/emoji's sort tests: nothing
// here calls BuildEntries. The entries are hand-ordered into a sequence
// that is neither raw-byte nor fold sorted, so the assertions fail if the
// picker ever re-derives an order of its own instead of preserving the one
// it was given.

// scrambledOrder is the input order under test. Names are absent from the
// bundled codemap, so no built-in can be confused for one of them, and
// every name shares the "zz" prefix so a "zz" query puts all of them in
// fuzzy.TierPrefix with score 0 — leaving input order as the comparator's
// only remaining discriminator.
//
// The order is neither fold-sorted (that would be zzAlpha, zzbravo,
// zzCharlie, zzdelta, zzEcho, ...) nor raw-byte sorted (zzAlpha,
// zzCharlie, zzEcho, zzGolf, zzbravo, ...).
func scrambledOrder() []emoji.EmojiEntry {
	return []emoji.EmojiEntry{
		{Name: "zzEcho", Display: "1"},
		{Name: "zzdelta", Display: "2"},
		{Name: "zzCharlie", Display: "3"},
		{Name: "zzbravo", Display: "4"},
		{Name: "zzAlpha", Display: "5"},
		{Name: "zzfoxtrot", Display: "6"},
		{Name: "zzGolf", Display: "7"},
	}
}

// gotOrder returns the filtered rows' names, in row order.
func gotOrder(m *Model) []string {
	var out []string
	for _, e := range m.Filtered() {
		out = append(out, e.Name)
	}
	return out
}

func assertOrder(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d rows %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row order not preserved:\n got %v\nwant %v", got, want)
		}
	}
}

func TestOrderPreserved_EmptyQueryKeepsGivenOrder(t *testing.T) {
	m := New()
	m.SetEntries(scrambledOrder())
	m.Open("")

	// MaxVisible=5 of 7 entries: the first five, in the order given.
	assertOrder(t, gotOrder(&m), []string{
		"zzEcho", "zzdelta", "zzCharlie", "zzbravo", "zzAlpha",
	})
}

func TestOrderPreserved_SameTierMatchesKeepGivenOrder(t *testing.T) {
	m := New()
	m.SetEntries(scrambledOrder())
	// Every name is prefixed "zz", so all seven land in TierPrefix with
	// score 0 and no frecent history is set. Tier, score and recency all
	// tie, so the comparator falls through to input index.
	m.Open("zz")

	assertOrder(t, gotOrder(&m), []string{
		"zzEcho", "zzdelta", "zzCharlie", "zzbravo", "zzAlpha",
	})
	if len(m.Filtered()) != MaxVisible {
		t.Errorf("expected the MaxVisible cap to bite: got %d rows, want %d",
			len(m.Filtered()), MaxVisible)
	}
}

// The cap must take a prefix of the given order, not an arbitrary subset:
// with exactly MaxVisible matches the full sequence has to come through
// unreordered.
func TestOrderPreserved_UncappedMatchesKeepGivenOrder(t *testing.T) {
	m := New()
	m.SetEntries(scrambledOrder()[:MaxVisible])
	m.Open("zz")

	assertOrder(t, gotOrder(&m), []string{
		"zzEcho", "zzdelta", "zzCharlie", "zzbravo", "zzAlpha",
	})
}

// SetQuery re-runs filter() on an already-open picker. Narrowing the query
// must not reshuffle the survivors.
func TestOrderPreserved_AcrossSetQuery(t *testing.T) {
	m := New()
	m.SetEntries(scrambledOrder())
	m.Open("zz")
	m.SetQuery("zz") // Same match set, re-filtered.

	assertOrder(t, gotOrder(&m), []string{
		"zzEcho", "zzdelta", "zzCharlie", "zzbravo", "zzAlpha",
	})
}
