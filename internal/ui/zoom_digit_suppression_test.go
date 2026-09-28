package ui

// B53 oracle: the "a digit switches workspace" rule must exist ONCE, and the
// zoom suppression must swallow exactly the digits that would actually do
// something.
//
// This file is the gate oracle for lane-digits and is HASH-PINNED. Do not edit
// it to make it pass. If you think it asks the wrong question, say so in a report
// and leave the gate red; that is a successful outcome for this lane.
//
// ---------------------------------------------------------------------------
// THREE VERIFIED FINDINGS
//
// 1. ONE RULE, TWO IMPLEMENTATIONS (the mandated duplicate-value class, third
//    instance in this plan after B35 and B41). The character-class test
//    `len(s) == 1 && s[0] >= '1' && s[0] <= '9'` appears at BOTH
//    mode_normal.go:357 (the handler) and reducer_zoom.go:146 (the suppressor).
//    Resolution for the class is DELETE ONE, NEVER ALIAS.
//
// 2. THEY ALREADY DISAGREE, and not hypothetically. The handler acts only when
//    `idx < len(a.workspaceItems)` and the target differs from the current
//    selection; the suppressor swallows all nine digits unconditionally. So with
//    two workspaces configured, pressing `3`..`9` while zoomed is CONSUMED, while
//    unzoomed it is a harmless no-op. Combined with B45 -- the suppression toast
//    was invisible until 2bc0cb5 -- the user pressed a key, nothing happened, and
//    nothing said why. (This is why the appendix rated B53 as depending on
//    "whether normal mode uses counts": it does not, but it does not need to.
//    The asymmetry is already there without counts.)
//
// 3. `alt+1`..`alt+9` HAS NO HANDLER AT ALL. Verified by searching the whole
//    tree: no binding and no handler anywhere outside the suppression list
//    itself. So reducer_zoom.go:149 suppresses a key combination that does
//    nothing, and the comment above it -- "bare 1-9 and alt+1-9 both switch
//    workspace" -- is false for the alt half.
//
// ---------------------------------------------------------------------------
// WHAT THIS ORACLE DOES *NOT* DECIDE
//
// Finding 3 has two legitimate answers and this file takes neither:
//
//   (a) delete the alt clause, because nothing handles alt+N; or
//   (b) add the alt+N binding, because it was clearly intended.
//
// The assertion is only that suppression and handling AGREE. Both answers
// satisfy it. Pick one, and say which in the commit message.
//
// The same shape as B41's oracle: pin the agreement, not the choice.

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestZoomDigitSuppression_MatchesWhatNormalModeActuallyDoes is the behavioural
// half.
//
// For each bare digit it observes BOTH sides and requires them to agree:
//
//	handled    — does normal mode return a command for this key? Observed by
//	             dispatching it through the real handler, not by re-deriving the
//	             condition. (A test that recomputed `idx < len(items)` would be
//	             comparing the rule against itself; see OT27.)
//	suppressed — does a.zoomSuppresses report it?
//
// Two workspaces are seeded, so `1` is the already-active guard, `2` switches,
// and `3`..`9` are out of range. Today `3`..`9` are suppressed while doing
// nothing, which is the RED.
//
// THE ALREADY-ACTIVE DIGIT IS DELIBERATELY UNASSERTED. The first draft of this
// test required suppression to equal "normal mode returned a command", which
// made digit `1` fail too -- and that is not a defect, it is a contested
// contract: suppressing the already-active digit is defensible on intent even
// though it acts on nothing. Forcing my reading of it into a pinned oracle would
// have made a product decision look like a test failure. The switch below
// asserts only what BOTH readings agree on, and logs the contested cell.
func TestZoomDigitSuppression_MatchesWhatNormalModeActuallyDoes(t *testing.T) {
	for _, d := range "123456789" {
		digit := d
		t.Run(string(digit), func(t *testing.T) {
			// Fresh App per digit: dispatching a switch mutates state.
			a := newTestApp(t, workspaceOpts()...)
			wireSwitcher(t, a)

			msg := keyPress(digit)

			suppressed := a.zoomSuppresses(msg)

			// Observe the handler. Unzoomed normal mode is the surface the
			// suppression is protecting against, so that is what gets asked.
			a.zoomed = false
			a.mode = ModeNormal
			handled := dispatchModeKey(a, msg) != nil

			idx := int(digit - '1')
			inRange := idx < len(a.workspaceItems)
			alreadyActive := inRange && a.workspaceItems[idx].ID == a.workspaceRail.SelectedID()

			switch {
			case !inRange:
				// No configured workspace behind this digit, so neither contract
				// below has any reason to swallow it. Today it IS swallowed --
				// this is the RED.
				if suppressed {
					t.Errorf("digit %q is suppressed while zoomed, but there are only %d "+
						"workspace(s) so it does nothing at all (normal mode handled=%v).\n"+
						"  Suppressing it consumes the key AND raises a toast for a keystroke\n"+
						"  that would have been a harmless no-op. Match the real binding\n"+
						"  instead of the character class '1'..'9'.",
						string(digit), len(a.workspaceItems), handled)
				}

			case alreadyActive:
				// DELIBERATELY UNASSERTED -- this is the one contested cell.
				//
				// Two defensible contracts disagree here and this oracle refuses
				// to pick for you:
				//   (A) suppress keys that would ACT. Pressing the already-active
				//       digit acts on nothing (mode_normal.go guards on
				//       SelectedID), so it should not be suppressed.
				//   (B) suppress keys whose INTENT is a workspace switch. The
				//       digit means "go to workspace N" regardless of where the
				//       rail happens to be, so it should be suppressed, and the
				//       toast correctly explains why nothing moved.
				//
				// (B) is the more stable rule -- its answer does not change as the
				// selection moves -- but (A) is what "suppress what would act"
				// literally says. Asserting either would smuggle a product
				// decision into a refactor. Say which you chose in the commit
				// message; both pass this test.
				t.Logf("digit %q is the already-active workspace: suppressed=%v handled=%v "+
					"(intentionally unasserted -- see the comment; contracts (A) and (B) differ here)",
					string(digit), suppressed, handled)

			default:
				// This digit switches to a different configured workspace. Both
				// contracts agree it must be suppressed while zoomed.
				if !suppressed {
					t.Errorf("digit %q switches to a different configured workspace "+
						"(normal mode handled=%v) but is NOT suppressed while zoomed.\n"+
						"  That defeats the suppression rule: the zoom would be torn down\n"+
						"  by a workspace switch the user did not aim at the zoomed pane.",
						string(digit), handled)
				}
			}
		})
	}
}

// TestZoomDigitSuppression_AltDigitsAgreeWithTheirHandler covers finding 3.
//
// Decision-free by construction: it only requires the two sides to agree, so
// deleting the alt clause and adding the alt binding both satisfy it.
func TestZoomDigitSuppression_AltDigitsAgreeWithTheirHandler(t *testing.T) {
	for _, d := range "123456789" {
		digit := d
		t.Run("alt+"+string(digit), func(t *testing.T) {
			a := newTestApp(t, workspaceOpts()...)
			wireSwitcher(t, a)

			msg := keyMod(digit, tea.ModAlt)

			// Sanity: the harness really does render this as "alt+N". If this
			// ever stops holding, the suppressor's string test is dead code and
			// this test would pass vacuously -- so it is checked, not assumed.
			if got := msg.String(); got != "alt+"+string(digit) {
				t.Fatalf("precondition: keyMod rendered %q, want %q -- the suppressor "+
					"matches on the STRING form, so this test means nothing if it differs",
					got, "alt+"+string(digit))
			}

			suppressed := a.zoomSuppresses(msg)

			a.zoomed = false
			a.mode = ModeNormal
			handled := dispatchModeKey(a, msg) != nil

			if suppressed != handled {
				t.Errorf("alt+%s: zoomSuppresses=%v but normal mode handled=%v\n"+
					"  Nothing in the tree binds or handles alt+N (verified), so either\n"+
					"  delete the alt clause from zoomSuppresses or add the binding.\n"+
					"  Both are acceptable; they must just agree.",
					string(digit), suppressed, handled)
			}
		})
	}
}

// TestZoomDigitSuppression_OneDigitRule is the structural half, and it is here
// for the same reason B41's structural test is: no behavioural test can tell
// "one rule" from "two rules that currently agree", which is how this drifted in
// the first place.
//
// It greps for the character-class shape rather than parsing, because the shape
// is what gets copy-pasted. Comments are stripped first so that prose quoting
// the old expression -- including the comments in THIS file's own package --
// cannot make it pass or fail spuriously.
func TestZoomDigitSuppression_OneDigitRule(t *testing.T) {
	files := []string{"mode_normal.go", "reducer_zoom.go"}

	var found []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			code := line
			if idx := strings.Index(code, "//"); idx >= 0 {
				code = code[:idx]
			}
			// The shape: a byte compared against '1' and against '9'.
			if strings.Contains(code, `>= '1'`) && strings.Contains(code, `<= '9'`) {
				found = append(found, fmt.Sprintf("%s:%d", f, i+1))
			}
		}
	}

	if len(found) > 1 {
		t.Errorf("the digit-range test appears in %d places: %v\n"+
			"  One rule, one implementation. Extract a single predicate (something\n"+
			"  like workspaceSwitchIndex(msg) (int, bool)) and have both the handler\n"+
			"  and the suppressor call it. DELETE ONE, NEVER ALIAS -- a second site\n"+
			"  that delegates still encodes the range twice.", len(found), found)
	}
}
