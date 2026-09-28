package reactionpicker

import (
	"strings"
	"testing"

	"github.com/gammons/slk/internal/core"
)

// ---------------------------------------------------------------------
// RED tests for the reaction picker consuming the shared fuzzy
// matcher (plan items fuzzy-reactionpicker-migration + fuzzy-shared,
// gotchas G12 + G10). Today filter() (model.go:237-259) is prefix +
// buffered substring with a hard 50 cap, so:
//
//   - every subsequence row here fails (returns 0 results);
//   - the tier-order rows fail because substrings are appended AFTER
//     prefix matches only by accident of the two-buffer append — the
//     50-cap interacts with it (see the cap test);
//   - the negative in-order row would fail if the migration drifted
//     to bag-of-chars.
//
// The tier-set change is a behavior change in its own commit (G12);
// these are the tests that commit lands against.
// ---------------------------------------------------------------------

// typeQuery types query into the picker one printable rune at a time,
// the way HandleKey's default arm does.
func typeQuery(m *Model, query string) {
	for _, ch := range query {
		m.HandleKey(string(ch))
	}
}

// filteredNames returns the current filtered list's names in render
// order.
func filteredNames(m *Model) []string {
	out := make([]string, 0, len(m.filtered))
	for _, e := range m.filtered {
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

// G12's named case: a subsequence query reaches rocket in the
// reaction picker too. "rkt" is an in-order subsequence of "rocket";
// today's filter finds nothing for it.
func TestFuzzyReaction_SubsequenceQueryReachesRocket(t *testing.T) {
	m := New()
	// Narrow the list so the row is about matching, not scrolling:
	// keep only the entries whose names contain r, k and t in order
	// plus a couple of prefix-tier distractors.
	m.SetCustomEmoji(map[string]string{
		"rk_tracer":    "https://example.test/tracer.png", // prefix tier for "rk"
		"rocket_alias": "alias:rocket",                    // substring tier
	})
	m.Open("C1", "1.0", nil)
	typeQuery(m, "rkt")

	if len(m.filtered) == 0 {
		t.Fatal(`query "rkt" produced no matches; filter() has no subsequence tier — G12 migration missing`)
	}
	// The built-in rocket must be reachable through it.
	if !containsName(filteredNames(m), "rocket") {
		t.Errorf(`query "rkt" does not reach "rocket"; filtered = %v`, filteredNames(m))
	}
}

// G10 negative: "krt" must not match rocket, in this picker too.
func TestFuzzyReaction_OutOfOrderQueryDoesNotMatchRocket(t *testing.T) {
	m := New()
	m.Open("C1", "1.0", nil)
	typeQuery(m, "krt")
	if containsName(filteredNames(m), "rocket") {
		t.Error(`query "krt" matched "rocket"; subsequence semantics are IN-ORDER (G10)`)
	}
}

// Tier order, pinned on a fixture where all three tiers are
// populated: customs give precise control over name shape.
//   - "oc..." prefix tier: octari (custom)
//   - substring tier: "cockpit" (custom; oc interior, contiguous)
//   - subsequence tier: the built-in "popcorn" (o..c never adjacent)
func TestFuzzyReaction_TierOrderPrefixSubstringSubsequence(t *testing.T) {
	m := New()
	m.SetCustomEmoji(map[string]string{
		"octari":  "https://example.test/octari.png",  // prefix
		"cockpit": "https://example.test/cockpit.png", // substring
	})
	m.Open("C1", "1.0", nil)
	typeQuery(m, "oc")

	names := filteredNames(m)
	if !containsName(names, "octari") {
		t.Fatalf(`prefix-tier entry "octari" not matched by "oc"; filtered = %v`, names)
	}
	if !containsName(names, "cockpit") {
		t.Fatalf(`substring-tier entry "cockpit" not matched by "oc"; filtered = %v`, names)
	}
	if !containsName(names, "popcorn") {
		t.Fatalf(`subsequence-tier built-in "popcorn" not matched by "oc"; filtered = %v`, names)
	}
	// Ordering: octari before cockpit before popcorn.
	idx := func(name string) int {
		for i, n := range names {
			if n == name {
				return i
			}
		}
		return -1
	}
	if !(idx("octari") < idx("cockpit") && idx("cockpit") < idx("popcorn")) {
		t.Errorf(`tier order broken: octari@%d cockpit@%d popcorn@%d; want octari < cockpit < popcorn; order = %v`,
			idx("octari"), idx("cockpit"), idx("popcorn"), names)
	}
}

// The frecent (recent) tier outranks everything while it matches
// (fuzzy-emoji: recent = frecent ∩ matches). reactionpicker already
// has frecent; what changes is that a matching frecent entry must
// outrank a matching prefix entry.
func TestFuzzyReaction_FrecentOutranksPrefixMatch(t *testing.T) {
	m := New()
	m.SetCustomEmoji(map[string]string{
		"aaa_frec": "https://example.test/frec.png", // prefix match, NOT frecent
	})
	m.SetFrecentEmoji([]core.EmojiEntry{
		{Name: "zzz_frec_recent", Unicode: "x"}, // matches "frec" as substring, is frecent
	})
	m.Open("C1", "1.0", nil)
	typeQuery(m, "frec")

	names := filteredNames(m)
	if !containsName(names, "zzz_frec_recent") {
		t.Fatalf(`frecent entry matching the query is not in the results; filtered = %v`, names)
	}
	if !containsName(names, "aaa_frec") {
		t.Fatalf(`prefix-matching entry missing from results; filtered = %v`, names)
	}
	if names[0] != "zzz_frec_recent" {
		t.Errorf(`recent tier must outrank prefix: first result = %q; order = %v`, names[0], names)
	}
}

// Stale frecent entries that do NOT match the query must be skipped,
// not shown (fuzzy-rkt-verified: "recent = matching entries ∩
// frecent only").
func TestFuzzyReaction_NonMatchingFrecentIsExcluded(t *testing.T) {
	m := New()
	m.SetFrecentEmoji([]core.EmojiEntry{
		{Name: "zzz_unrelated", Unicode: "x"}, // frecent but matches nothing
	})
	m.Open("C1", "1.0", nil)
	typeQuery(m, "rock")

	for _, n := range filteredNames(m) {
		if n == "zzz_unrelated" {
			t.Fatalf(`non-matching frecent entry %q appeared in results for "rock"; recent tier must be the INTERSECTION with matches`, n)
		}
	}
	if !containsName(filteredNames(m), "rocket") {
		t.Errorf(`query "rock" does not reach "rocket"; filtered = %v`, filteredNames(m))
	}
}

// The 50-entry cap semantics (G11): today the cap is
// len(filtered)+len(substringMatches) >= 50 checked DURING the walk,
// which truncates the PREFIX tier itself — for query "a" the walk
// stops at "arrows_counterclockwise" (alphabetical index 53) and 11
// a-prefixed entries later in the alphabet ("art", "artist",
// "astronaut", ...) are unreachable FOREVER (verified against real
// BuildEntries output: 60 'a' prefix matches, walk stops at 50
// entries). Under the fuzzy migration the cap must keep the prefix
// tier intact: whatever the eviction rule, a prefix match must
// survive ahead of lower tiers (plan: "cap semantics defined
// (eviction documented or top prefix match guaranteed to survive)").
func TestFuzzyReaction_CapKeepsPrefixMatchesReachable(t *testing.T) {
	m := New()
	m.Open("C1", "1.0", nil)
	typeQuery(m, "a")

	names := filteredNames(m)
	if len(names) == 0 {
		t.Fatal(`query "a" produced no matches; the picker cannot match its own prefix tier`)
	}
	// HARD-RED today: "astronaut" is an a-prefix match that the
	// during-the-walk cap silently truncates away (it sits past the
	// 50-entry break point).
	if !containsName(names, "astronaut") {
		t.Errorf(`"astronaut" (an "a" prefix match) is unreachable; the 50-cap truncates the prefix tier mid-walk; order = %v`, names)
	}
	// Every visible row must genuinely match the query; a cap that
	// admits stale rows fails here.
	for _, n := range names {
		if !strings.HasPrefix(n, "a") && !strings.Contains(n, "a") {
			t.Errorf(`row %q does not match "a" in any tier; order = %v`, n, names)
		}
	}
}

// Regression guard for behavior the migration must NOT change
// (TestSubstringSearchSelectsCustomEmoji already pins selection;
// this pins that substring matching keeps working through
// HandleKey -> enter once the tier set grows).
func TestFuzzyReaction_SubstringCustomEmojiStillSelectable(t *testing.T) {
	m := New()
	m.SetCustomEmoji(map[string]string{
		"custom_needle": "https://emoji.example.com/custom_needle.gif",
	})
	m.Open("C123", "1234.5678", nil)
	typeQuery(m, "needle")

	result := m.HandleKey("enter")
	if result == nil {
		t.Fatal("substring search produced no selectable result")
	}
	if result.Emoji != "custom_needle" {
		t.Fatalf("selected emoji = %q, want %q", result.Emoji, "custom_needle")
	}
}

// Query folding through the shared matcher (reactionpicker already
// folds via text.Fold; the migration must keep it).
func TestFuzzyReaction_QueryFoldsCase(t *testing.T) {
	m := New()
	m.Open("C1", "1.0", nil)
	typeQuery(m, "ROCKET")
	if !containsName(filteredNames(m), "rocket") {
		t.Errorf(`query "ROCKET" does not reach "rocket" (case fold broken); filtered = %v`, filteredNames(m))
	}
}
