package ui

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// B46: the zoomed pane's last row is DRAWN but not CLICKABLE.
//
// One rule -- "where does the status row start" -- is implemented three times,
// and only one of them knows about zoom:
//
//	panelLayout.Compute      statusHeight := 1; if zoomed { statusHeight = 0 }
//	panelLayout.PanelAt      if y >= height-1 { ... }   <- no zoomed parameter
//	reduceMouseClick         statusHeight := 1          <- its own literal
//
// Because Compute is zoom-aware, ContentHeight == height while zoomed and the
// pane genuinely renders on the final terminal row. Because the other two are
// not, both hit-test paths classify that row as status bar and drop the event.
// Clicks, drag-selection anchors and reaction hit-testing are all dead on the
// row the user is most likely looking at -- zoom exists to show more history.
//
// Why no existing test caught it: fs-statusrow-math's proof is a golden frame,
// and a golden cannot fail on a hit-test. The render half was proven and the
// routing half assumed. Same meta-defect as B13 and B45: an assertion whose
// subject the defect cannot reach.
//
// These tests live in their own file because internal/ui/fullscreen_red_test.go
// is a hash-pinned gate oracle (gates/zoom/oracle.sha256), and OT17 records
// what happened the one time I edited one of those in place.

// The load-bearing assertion, on the shared hit-test path (PanelAt). While
// zoomed the last row belongs to a content pane, because Compute gave the pane
// the whole terminal height.
func TestZoom_LastRowIsHitTestableWhileZoomed(t *testing.T) {
	a := newTestApp(t, withSize(100, 30), withMessages(testMessageItems(60)...), withRender())

	// Sanity, and a guard against the opposite failure: unzoomed, the last row
	// IS the status bar. Without this the assertion below would also pass
	// against a PanelAt that had simply stopped rejecting anything.
	if _, _, _, ok := a.panelAt(50, a.height-1); ok {
		t.Fatalf("unzoomed: last row (y=%d) hit-tests as a pane, want status bar", a.height-1)
	}

	a.enterZoom()

	// If Compute does not actually give the pane the full height, the premise
	// is broken rather than the hit-test, and the message should say so.
	frame := a.computeFrame()
	if frame.ContentHeight != a.height {
		t.Fatalf("zoomed ContentHeight = %d, want %d (the whole terminal); premise broken, not the hit-test",
			frame.ContentHeight, a.height)
	}

	panel, _, _, ok := a.panelAt(50, a.height-1)
	if !ok {
		t.Errorf("zoomed: last row (y=%d) hit-tests as status bar, but no status row is drawn while zoomed -- "+
			"the pane renders there and every click on it is discarded (B46). "+
			"PanelAt takes no zoomed parameter and app.go's panelAt passes none.", a.height-1)
	}
	if ok && panel != PanelMessages {
		t.Errorf("zoomed: last row resolved to panel %v, want PanelMessages", panel)
	}
}

// The three views of the one rule must agree at every zoom state, by
// construction rather than by coincidence. This is what fails if a fourth copy
// of the rule is ever added, and it is the reason the fix is "store it once"
// rather than "add a zoomed parameter to PanelAt".
func TestZoom_StatusRowHeightHasOneSourceOfTruth(t *testing.T) {
	a := newTestApp(t, withSize(100, 30), withMessages(testMessageItems(60)...), withRender())

	for _, zoomed := range []bool{false, true} {
		if zoomed {
			a.enterZoom()
		} else {
			a.clearZoom()
		}

		frame := a.computeFrame()
		// Compute's answer, derived: rows withheld from content.
		computeReserved := a.height - frame.ContentHeight
		// PanelAt's answer, probed: is the last row a pane or not?
		_, _, _, lastRowIsPane := a.panelAt(50, a.height-1)
		panelAtReserved := 0
		if !lastRowIsPane {
			panelAtReserved = 1
		}

		if computeReserved != panelAtReserved {
			t.Errorf("zoomed=%v: one rule, two answers -- Compute reserved %d row(s), PanelAt %d. "+
				"Both must read the same value (B46: delete the literal, store it on panelLayout).",
				zoomed, computeReserved, panelAtReserved)
		}
	}
}

// The last CONTENT row must be clickable while zoomed. This is the property
// fs-statusrow-math actually cares about, and it is the honest version of the
// assertion the first draft of this file got wrong -- see the geometry note
// below.
//
// GEOMETRY, measured rather than assumed (height 30):
//
//	unzoomed  rows 0..28 = pane (border at 0 and 28), row 29 = status row
//	zoomed    rows 0..29 = pane (border at 0 and 29), no status row
//
// So while zoomed the final terminal row holds the pane's bottom BORDER, not
// content, and content ends at row 28. My first draft asserted that a click on
// row 29 must move the selection, and reported it as a live user-visible defect
// ("a full row of content is mouse-dead"). That was WRONG: clicking a border
// correctly selects nothing, and no content row was ever unreachable. B46's
// duplicate-rule mechanism is real and worth fixing; its severity claim was not.
// The correction is recorded in the batch-4 appendix.
func TestZoom_LastContentRowIsClickableWhileZoomed(t *testing.T) {
	a := newTestApp(t, withSize(100, 30), withMessages(testMessageItems(60)...), withRender())
	a.enterZoom()
	_ = a.computeFrame()
	a.View() // populate the layout bands and pane caches the click path reads

	// Park at the top: a fixture already sitting at the bottom cannot
	// discriminate, because the click would select the already-selected item
	// and both outcomes would look identical.
	a.messagepane.SetViewport(0, 0)

	lastContent := a.height - 2 // height-1 is the pane border while zoomed
	_, _ = a.Update(tea.MouseClickMsg{X: 50, Y: lastContent, Button: tea.MouseLeft})
	if got := a.messagepane.SelectedIndex(); got == 0 {
		t.Errorf("zoomed: click on the last content row (y=%d) did not move the selection off 0; "+
			"the zoomed pane's bottom content row must be clickable", lastContent)
	}
}

// The third implementation of the status-row rule lived in reduceMouseClick as
// its own `statusHeight := 1`. Fixing PanelAt alone would have left it, and no
// behavioural test can catch that: while zoomed the only row the two answers
// disagree about is the pane's bottom border, where a click correctly selects
// nothing either way. There is no observable difference, so a behavioural
// oracle would be vacuous by construction.
//
// So this is pinned structurally instead, the same shape as
// cmd/slk/startup_order_test.go (B18): read the source and assert the literal
// is gone. A structural oracle is the honest instrument when the invariant is
// "one source of truth" rather than "this output changes".
func TestZoom_MouseClickHasNoOwnStatusRowLiteral(t *testing.T) {
	src, err := os.ReadFile("reducer_mouse.go")
	if err != nil {
		t.Fatalf("read reducer_mouse.go: %v", err)
	}
	body := string(src)

	// Prove the anchor exists before drawing a conclusion from its absence --
	// a renamed file or function would otherwise make this pass vacuously.
	if !strings.Contains(body, "func reduceMouseClick(") {
		t.Fatalf("reduceMouseClick not found in reducer_mouse.go; this oracle no longer describes the code")
	}
	if !strings.Contains(body, "a.layout.statusRows()") {
		t.Errorf("reduceMouseClick does not consult a.layout.statusRows(); the status-row height must have "+
			"ONE source of truth (B46). Found no call in:\n%s", firstLinesContaining(body, "status"))
	}
	if strings.Contains(body, "statusHeight := 1") {
		t.Errorf("reducer_mouse.go still declares its own `statusHeight := 1`. Delete it and read " +
			"a.layout.statusRows() instead -- AGENTS.md's rule for this class is delete one, never alias.")
	}
}

// firstLinesContaining is a diagnostic for the assertion above: it shows the
// lines that mention the concept so a failure names what IS there, not just
// what is missing.
func firstLinesContaining(body, needle string) string {
	var out []string
	for _, ln := range strings.Split(body, "\n") {
		if strings.Contains(strings.ToLower(ln), needle) {
			out = append(out, "    "+strings.TrimSpace(ln))
			if len(out) == 6 {
				break
			}
		}
	}
	if len(out) == 0 {
		return "    (no matching lines)"
	}
	return strings.Join(out, "\n")
}
