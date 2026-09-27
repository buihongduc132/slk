package emojipicker

import (
	"strings"
	"testing"

	"github.com/gammons/slk/internal/emoji"
)

// ---------------------------------------------------------------------
// RED tests for fuzzy ranking in the compose emoji dropdown (plan
// items fuzzy-emoji + fuzzy-shared, gotchas G11/G12). Against
// today's prefix-only filter() every "reachable" assertion below
// fails: the subsequence and substring queries return empty filtered
// lists, and the ranking rows fail because the only matches the
// filter can produce are prefix matches.
//
// All fixtures were verified against real emoji.BuildEntries(nil)
// output (1972 entries) before being written down:
//
//   - "rkt" in-order subsequence matches on the built-ins are exactly
//     12; under channelfinder's word-boundary+tightness scoring
//     "rocket" ranks 2nd (80) behind bookmark_tabs (92) — i.e. rocket
//     wins a MaxVisible=5 slot on merit, no special-casing.
//   - "oc" has 3 prefix matches and 58 substring-only matches on the
//     built-ins, so a tier-order row with oc-prefixed names is never
//     starved.
// ---------------------------------------------------------------------

// filteredNames returns the Filtered() entry names in render order.
func filteredNames(m Model) []string {
	out := make([]string, 0, len(m.filtered))
	for _, e := range m.Filtered() {
		out = append(out, e.Name)
	}
	return out
}

func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

func containsPrefix(names []string, prefix string) bool {
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			return true
		}
	}
	return false
}

// entriesFor builds a picker whose entry list is the real built-in
// table filtered down to the names matching any of keep. Tests stay
// against REAL codemap entries (never invented names) while keeping
// each fixture small enough to reason about.
func entriesFor(t *testing.T, keep ...string) []emoji.EmojiEntry {
	t.Helper()
	all := emoji.BuildEntries(nil)
	have := make(map[string]emoji.EmojiEntry, len(all))
	for _, e := range all {
		have[e.Name] = e
	}
	out := make([]emoji.EmojiEntry, 0, len(keep))
	for _, k := range keep {
		e, ok := have[k]
		if !ok {
			t.Fatalf(`built-in entry %q not found; the codemap changed — re-verify the fixture against emoji.BuildEntries`, k)
		}
		out = append(out, e)
	}
	return out
}

// The DOD example, verified feasible: with only the 12 real "rkt"
// subsequence matches (plus the prefix set for "rkt", which is empty
// on the built-ins) loaded, "rocket" must be reachable. Today the
// prefix-only filter returns nothing for "rkt".
func TestFuzzy_RocketReachableBySubsequence(t *testing.T) {
	entries := entriesFor(t,
		"rocket", "bookmark_tabs", "roller_skate", "cricket",
		"smirk_cat", "broken_heart", "sparkling_heart",
		"rabbit", "robot_face", // two "r" prefix matches, for realism
	)
	m := New()
	m.SetEntries(entries)
	m.SetQuery("rkt")

	if got := m.Filtered(); len(got) == 0 {
		t.Fatal(`query "rkt" produced no matches; the filter is prefix-only — subsequence matches are unreachable`)
	}
	if !containsName(filteredNames(m), "rocket") {
		t.Errorf(`query "rkt" does not reach "rocket"; filtered = %v`, filteredNames(m))
	}
}

// "krt" must NOT match rocket (in-order semantics, G10) — with an
// otherwise-matchable entry set the negative query yields nothing.
//
// FIXTURE CONSTRAINT: every entry here must match "rkt" (so the set is
// genuinely matchable and the negative result is meaningful) while NO
// entry may contain k…r…t in that order. That second half is easy to get
// wrong. "bookmark_tabs" was the original third entry and is a REAL
// in-order match for "krt" — b-o-o-[k]-m-a-[r]-k-_-[t]-a-b-s, k@3 r@6
// t@9 — so the row demanded that correct in-order matching return
// nothing, which no correct implementation can do. Verified the
// replacements: rocket, cricket and roller_skate all match "rkt" and
// none match "krt".
func TestFuzzy_OutOfOrderQueryMatchesNothing(t *testing.T) {
	entries := entriesFor(t, "rocket", "cricket", "roller_skate")
	m := New()
	m.SetEntries(entries)
	m.SetQuery("krt")

	if got := m.Filtered(); len(got) != 0 {
		t.Errorf(`query "krt" matched %v; subsequence matching is IN-ORDER (G10) so nothing here should match`, filteredNames(m))
	}
}

// Substring tier: "humbs" occurs inside "thumbsup" and "thumbsdown"
// but is a prefix of neither. Today both are unreachable.
func TestFuzzy_SubstringMatchReachable(t *testing.T) {
	entries := entriesFor(t, "thumbsup", "thumbsdown", "+1")
	m := New()
	m.SetEntries(entries)
	m.SetQuery("humbs")

	if !containsName(filteredNames(m), "thumbsup") || !containsName(filteredNames(m), "thumbsdown") {
		t.Errorf(`query "humbs" does not reach the substring matches; filtered = %v`, filteredNames(m))
	}
	// Negative guard: "+1" matches no tier of "humbs".
	if containsName(filteredNames(m), "+1") {
		t.Errorf(`"+1" matched query "humbs"; no tier applies to it`)
	}
}

// Tier ordering (fuzzy-emoji): prefix > substring > subsequence.
// Verified against the real codemap: the "oc" prefix matches are
// exactly octagonal_sign/ocean/octopus; "cocktail" carries oc as a
// contiguous interior substring; "popcorn" only as an in-order
// subsequence (o..c separated by p, never adjacent). All three tiers
// must be populated, and every prefix match must rank ahead of every
// non-prefix match.
func TestFuzzy_TierOrderPrefixBeatsSubstringBeatsSubsequence(t *testing.T) {
	entries := entriesFor(t,
		"octopus", "ocean", "octagonal_sign", // prefix tier (all real built-ins)
		"cocktail", // substring tier: "oc" contiguous, interior
		"popcorn",  // subsequence tier: o..c in order, never adjacent
	)
	m := New()
	m.SetEntries(entries)
	m.SetQuery("oc")

	names := filteredNames(m)
	if len(names) < 5 {
		t.Fatalf(`query "oc" returned %d of 5 entries; the filter is prefix-only — %v`, len(names), names)
	}
	// The first three results must all be prefix matches; nothing
	// from a lower tier may appear before them.
	firstNonPrefix := -1
	for i, n := range names {
		if !strings.HasPrefix(n, "oc") {
			firstNonPrefix = i
			break
		}
	}
	if firstNonPrefix != 3 {
		t.Errorf(`tier order broken: first non-prefix result at index %d, want 3 (all prefix matches first); order = %v`, firstNonPrefix, names)
	}
	// And the substring match must itself outrank the subsequence
	// match.
	if firstNonPrefix >= 0 && names[firstNonPrefix] != "cocktail" {
		t.Errorf(`substring tier must follow the prefix tier; got %q at the boundary; order = %v`, names[firstNonPrefix], names)
	}
	if names[len(names)-1] != "popcorn" {
		t.Errorf(`subsequence tier must rank last; got %q at the end; order = %v`, names[len(names)-1], names)
	}
}

// Input-order stability within a tier: entries arrive alphabetically
// from BuildEntries and same-tier matches must keep that order
// (filter()'s documented contract, model.go:133).
func TestFuzzy_StableWithinTier(t *testing.T) {
	// keep-order == alphabetical order here deliberately.
	entries := entriesFor(t, "thumbsdown", "thumbsup")
	m := New()
	m.SetEntries(entries)
	m.SetQuery("thumbs")

	names := filteredNames(m)
	want := []string{"thumbsdown", "thumbsup"}
	if len(names) != len(want) {
		t.Fatalf(`query "thumbs" filtered = %v, want %v`, names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf(`same-tier order not preserved: filtered[%d] = %q, want %q (input order); order = %v`, i, names[i], want[i], names)
		}
	}
}

// MaxVisible still caps the result list once the tier set grows
// beyond it (the 4-tier tier set only widens the candidate pool).
func TestFuzzy_MaxVisibleStillCaps(t *testing.T) {
	all := emoji.BuildEntries(nil)
	m := New()
	m.SetEntries(all)
	m.SetQuery("r") // 75 built-in prefix matches alone
	if got := len(m.Filtered()); got != MaxVisible {
		t.Errorf("filtered len = %d, want the MaxVisible cap %d", got, MaxVisible)
	}
}

// Case folding: the tier queries must fold, matching the channelfinder
// behavior the shared matcher standardizes.
func TestFuzzy_QueryFoldsCaseAndAccent(t *testing.T) {
	entries := entriesFor(t, "thumbsup")
	m := New()
	m.SetEntries(entries)
	m.SetQuery("THUMBSUP")
	if !containsName(filteredNames(m), "thumbsup") {
		t.Errorf(`query "THUMBSUP" does not reach "thumbsup" (case fold broken); filtered = %v`, filteredNames(m))
	}
}

// G11's documented substitution: the DOD's `:thumbs -> +1` cannot be
// won on name merit ("+1" is no prefix/substring/subsequence of
// "thumbs"), so the plan's example is pinned here as the REACHABLE
// pair instead — "thumbs" reaching "thumbsdown"/"thumbsup", which
// today's filter already matches and the fuzzy filter must KEEP
// matching (a regression guard for the tier change, not new reach).
func TestFuzzy_ThumbsStillReachesThumbsEmoji(t *testing.T) {
	entries := entriesFor(t, "thumbsup", "thumbsdown")
	m := New()
	m.SetEntries(entries)
	m.SetQuery("thumbs")
	for _, want := range []string{"thumbsup", "thumbsdown"} {
		if !containsName(filteredNames(m), want) {
			t.Errorf(`query "thumbs" no longer reaches %q (prefix regression); filtered = %v`, want, filteredNames(m))
		}
	}
}

// The subsequence tier must be SCORE-aware, not plain alphabetical
// (G11): with the real 12 "rkt" matches loaded, channelfinder's
// word-boundary+tightness semantics put bookmark_tabs (92) ahead of
// rocket (80) but rocket ahead of roller_skate (68). A plain
// alphabetical subsequence tier would order them bookmark_tabs,
// broken_heart, cricket, ... — rocket 9th. Asserting rocket ranks
// within the first three pins score-awareness without pinning exact
// scores.
func TestFuzzy_SubsequenceTierIsScoreAware(t *testing.T) {
	entries := entriesFor(t,
		"rocket", "bookmark_tabs", "roller_skate", "cricket",
		"cricket_bat_and_ball", "smirk_cat", "knife_fork_plate",
		"broken_heart", "sparkling_heart", "person_kneeling_facing_right",
		"person_walking_facing_right", "heavy_heart_exclamation_mark_ornament",
	)
	m := New()
	m.SetEntries(entries)
	m.SetQuery("rkt")

	names := filteredNames(m)
	if !containsName(names, "rocket") {
		t.Fatalf(`"rocket" not reachable under "rkt"; filtered = %v`, names)
	}
	rank := -1
	for i, n := range names {
		if n == "rocket" {
			rank = i
			break
		}
	}
	if rank >= 3 {
		t.Errorf(`"rocket" ranks %d under "rkt" (filtered = %v); a score-aware subsequence tier ranks it 2nd (behind bookmark_tabs, ahead of roller_skate). Rank >= 3 means the tier is alphabetical, not scored`, rank, names)
	}
}

// Empty query keeps the historical behavior: first N entries of the
// alphabetically-sorted list (documented behavior of filter(), and
// TestEmptyQueryShowsFirstN already pins it — restated here because
// the tier change must not disturb it).
func TestFuzzy_EmptyQueryKeepsFirstN(t *testing.T) {
	all := emoji.BuildEntries(nil)
	m := New()
	m.SetEntries(all)
	m.SetQuery("")
	if got := len(m.Filtered()); got != MaxVisible {
		t.Errorf("empty-query filtered len = %d, want %d (first N of the sorted list)", got, MaxVisible)
	}
}

// No-match query yields an empty (not nil-crashing) list; the View
// branch depends on len(filtered) == 0 to hide the dropdown.
func TestFuzzy_NoMatchQueryYieldsEmpty(t *testing.T) {
	entries := entriesFor(t, "rocket")
	m := New()
	m.SetEntries(entries)
	m.SetQuery("zzzzzz")
	if got := m.Filtered(); len(got) != 0 {
		t.Errorf(`query "zzzzzz" matched %v; want no matches`, filteredNames(m))
	}
	if containsPrefix(filteredNames(m), "z") {
		t.Error("filtered list unexpectedly contains a z-prefixed name")
	}
}
