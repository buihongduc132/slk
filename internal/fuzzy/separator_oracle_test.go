package fuzzy

import (
	"os"
	"strings"
	"testing"
)

// B27 ORACLE. "Word boundary" has three definitions inside this package:
//
//	isSeparator      '-' '_' '.' ' ' '/' ':'      (subsequence scoring)
//	WordPrefix       ' ' '-' '_' '.'              (inline, no '/' or ':')
//	SquashedPrefix   ' ' '-' '_' '.' via FieldsFunc
//
// So '/' and ':' are word boundaries for subsequence scoring and for nothing
// else. For emoji names -- which lean on '-' and '_' and whose trigger character
// is ':' -- which tier a candidate lands in depends on which definition that
// tier's predicate happens to use. The plan merged the two sources' PREDICATES
// verbatim without merging their notion of a separator: "extract the substrate,
// not the widget" was applied to the scoring functions but not to the value
// underneath them.
//
// B28 already deleted the fourth definition (mentionpicker's own isSeparator,
// which counted only four of the six). Three remain, all here.
//
// WHAT DONE LOOKS LIKE
//
//  1. ONE exported func IsSeparator(rune) bool.
//  2. WordPrefix and SquashedPrefix route through it. No inline character lists
//     and no second FieldsFunc closure with its own set.
//  3. The '/' and ':' question is answered DELIBERATELY, in a test, not left to
//     whichever predicate a caller happens to hit.
//
// ON THAT THIRD POINT -- READ BEFORE CHANGING BEHAVIOUR
//
// Unifying the definition necessarily CHANGES behaviour for one of the three
// sites: '/' and ':' become word boundaries for WordPrefix and SquashedPrefix
// where they were not. That is a user-visible ranking change, not a refactor.
//
// If routing through one definition turns an existing test red, STOP. Do not
// edit that test and do not weaken this one. A red pre-existing test here is
// the signal that the '/'-and-':' decision needs a human, and the correct
// output is a written report saying which test, which input, and what the two
// candidate answers are. Leaving this gate red with that report is a SUCCESSFUL
// outcome; silently reinterpreting someone's ranking is not.

// The single definition must exist and be exported.
func TestIsSeparator_ExistsAndCoversTheFullSet(t *testing.T) {
	for _, r := range []rune{'-', '_', '.', ' ', '/', ':'} {
		if !IsSeparator(r) {
			t.Errorf("IsSeparator(%q) = false, want true; this is the set the subsequence "+
				"scorer has always used, and unifying must not shrink it", r)
		}
	}
	for _, r := range []rune{'a', 'Z', '0', '+', 'é'} {
		if IsSeparator(r) {
			t.Errorf("IsSeparator(%q) = true, want false", r)
		}
	}
}

// Structural: the duplicate definitions must be GONE, not merely shadowed.
// A behavioural test cannot distinguish "routed through one definition" from
// "three definitions that happen to agree today", and the whole point of B27 is
// that they do NOT agree. So this reads the source, the same instrument used for
// B18 and B46.
func TestFuzzy_HasExactlyOneSeparatorDefinition(t *testing.T) {
	src, err := os.ReadFile("fuzzy.go")
	if err != nil {
		t.Fatalf("read fuzzy.go: %v", err)
	}
	body := string(src)

	// Prove the anchors exist before concluding anything from an absence --
	// a renamed function would otherwise make every check below pass vacuously.
	for _, anchor := range []string{"func WordPrefix(", "func SquashedPrefix(", "func IsSeparator("} {
		if !strings.Contains(body, anchor) {
			t.Fatalf("anchor %q not found in fuzzy.go; this oracle no longer describes the code", anchor)
		}
	}

	// The unexported twin must be gone.
	if strings.Contains(body, "func isSeparator(") {
		t.Errorf("fuzzy.go still declares unexported isSeparator alongside IsSeparator. " +
			"Delete one, never alias (AGENTS.md).")
	}

	// No inline separator list may survive. This is the literal set WordPrefix
	// used; if it is still present, WordPrefix is still deciding for itself.
	for _, lit := range []string{
		`r == ' ' || r == '-' || r == '_' || r == '.'`,
		`== ' ' || `,
	} {
		if strings.Contains(body, lit) {
			t.Errorf("fuzzy.go still contains an inline separator list (%q). Both WordPrefix and "+
				"SquashedPrefix must call IsSeparator instead.", lit)
		}
	}

	// Every separator decision should reach IsSeparator. Three call sites is the
	// floor: the subsequence scorer, WordPrefix, SquashedPrefix.
	if n := strings.Count(body, "IsSeparator("); n < 4 {
		t.Errorf("IsSeparator appears %d times in fuzzy.go (declaration + %d call sites); "+
			"want the declaration plus at least 3 -- the subsequence scorer, WordPrefix and "+
			"SquashedPrefix must all route through it", n, n-1)
	}
}

// The deliberate decision, written down. Whichever way it goes, it must be
// ASSERTED rather than emergent.
//
// Current behaviour, measured: "a:b" splits on ':' for subsequence scoring but
// NOT for WordPrefix. After unification these must agree. This test pins the
// unified answer as "':' and '/' ARE word boundaries everywhere" -- the
// direction that preserves the older, wider set rather than narrowing it.
//
// If that direction turns out to be wrong for real emoji or channel names, the
// correct response is to change THIS test deliberately, in a commit that says
// why, and to say so in the plan's Open Threads. Not to special-case a caller.
func TestWordPrefix_TreatsColonAndSlashAsBoundaries(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  bool
		why   string
	}{
		{"skin:tone", "tone", true, "':' must start a new word for WordPrefix, as it already does for subsequence scoring"},
		{"a/b-thing", "b", true, "'/' must start a new word"},
		{"plain_name", "name", true, "'_' was already a boundary; unification must not lose it"},
		{"nobreakhere", "here", false, "no separator, so 'here' is interior and not a word prefix"},
	}
	for _, c := range cases {
		if got := WordPrefix(c.name, c.query); got != c.want {
			t.Errorf("WordPrefix(%q, %q) = %v, want %v -- %s", c.name, c.query, got, c.want, c.why)
		}
	}
}
