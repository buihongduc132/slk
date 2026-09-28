// internal/ui/zoom_insert_drawn_pane_test.go
//
// threadDrawnAlone must answer for the frame that is actually DRAWN.
//
// THE DEFECT. threadDrawnAloneAt built its scratch frame from
// threadInFront() -- which reads focus -- while the real frame
// (computeFrame) resolves through layoutThreadFront(), which while zoomed
// defers to zoomFrontIsThread(). Those two diverge in exactly one state:
// zoomed at a side-by-side width with the thread "in front" by focus.
// There zoom promotes MESSAGES (G15), so the frame has ThreadWidth == 0,
// yet threadDrawnAlone() reported true -- "the thread is the only pane on
// screen" -- about a pane with no width at all.
//
// WHY THAT LOSES A KEYSTROKE. mode_normal.go's `i` arm consults
// threadDrawnAlone() to route insert mode to whichever content pane the
// user can actually see. Believing the invisible thread was the lone pane,
// it focused threadCompose, and the typed character went into a compose box
// that is not on screen -- the exact failure that call site exists to
// prevent.
//
// REACHABLE BY KEYSTROKES ALONE, with no seeded state: Enter (open and
// focus the thread) -> z (zoom; promotes messages) -> Tab. Tab walks
// PanelThread -> PanelSidebar directly (app.go FocusNext), skipping
// PanelMessages, and App.Update only records stackFront for the two content
// panes -- so stackFront stays PanelThread and threadInFront() keeps
// returning true while focus sits on the sidebar.
//
// Focus on the SIDEBAR is what makes this test discriminating. The `i` arm
// is an || chain whose first two arms are `focusedPanel == PanelThread` and
// the Threads view; from the sidebar both are false, so threadDrawnAlone()
// is the sole reason the keystroke is routed, and the sole thing this test
// can be failing on.
//
// WIDTHS ARE MEASURED. With the shared fixture's 6-col rail and 30-col
// sidebar (+2 border) the side-by-side branch first has room at 162 cols
// (161: MsgWidth=0/ThreadWidth=121; 162: 40/80; 200: 78/80). Hence 200 for
// the divergent row and 120 for the stacked control.
//
// OBSERVES, NEVER RECOMPUTES. Every assertion reads published state -- the
// rendered frame via stripANSI(a.View().Content), the hit-test bands via
// assertFront, and compose.Value() / threadCompose.Value(). Nothing here
// re-derives what threadDrawnAloneAt "should" return from the inputs it
// already uses, which is the failure mode that let the original divergence
// hide.
package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// insertToken is typed into whichever compose the `i` arm chooses. Absent
// from the fixture and from the chrome, and free of 'z' so no row can be
// confused with the zoom toggle.
const insertToken = "wombat"

// TestZoomInsert_TypingLandsInTheDrawnPane pins the invariant both ways:
// the pane insert mode routes to is the pane the frame draws. The rows are
// the two sides of the zoom-promotion branch, so a "fix" that hardcodes
// either answer while zoomed fails one of them.
func TestZoomInsert_TypingLandsInTheDrawnPane(t *testing.T) {
	for _, tc := range []struct {
		name string
		w    int
		// wantChannelDrawn / wantThreadDrawn are the panes the zoomed
		// frame draws, asserted before anything is typed.
		wantChannelDrawn bool
		wantThreadDrawn  bool
		// wantThreadCompose is where the keystrokes must land.
		wantThreadCompose bool
	}{
		{
			// THE DEFECT. Side by side unzoomed, so zoom promotes
			// messages and the thread is not drawn. Typing must land in
			// the channel compose. Before the fix this row put the token
			// in threadCompose, off screen.
			name:              "zoomed side by side: messages drawn, typing goes to the channel compose",
			w:                 sideBySideWidth,
			wantChannelDrawn:  true,
			wantThreadDrawn:   false,
			wantThreadCompose: false,
		},
		{
			// The control, and the reason the fix cannot simply force
			// threadDrawnAlone false while zoomed. Stacked with the
			// thread in front, so zoom promotes the THREAD; it is the
			// pane on screen and typing must land in its compose.
			name:              "zoomed stacked: thread drawn, typing goes to the thread compose",
			w:                 stackedWidth,
			wantChannelDrawn:  false,
			wantThreadDrawn:   true,
			wantThreadCompose: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := stackedApp(t, tc.w)
			updateAndRender(t, a, keyCode(tea.KeyEnter)) // open + focus the thread
			if !a.threadVisible || a.focusedPanel != PanelThread {
				t.Fatalf("precondition: threadVisible=%v focus=%v, want the thread open and focused",
					a.threadVisible, a.focusedPanel)
			}

			updateAndRender(t, a, keyPress('z'))
			if !a.zoomed {
				t.Fatal("precondition: 'z' did not enter zoom")
			}

			updateAndRender(t, a, keyCode(tea.KeyTab)) // thread -> sidebar
			if a.focusedPanel != PanelSidebar {
				t.Fatalf("precondition: focus=%v, want PanelSidebar -- with focus on a content "+
					"pane the `i` arm's earlier branches decide the route and this row "+
					"would not be testing threadDrawnAlone at all", a.focusedPanel)
			}

			// Which panes the frame actually draws, read from the bands
			// the last real render stored.
			assertFront(t, a, tc.wantChannelDrawn, tc.wantThreadDrawn)

			// Guards against a vacuous pass: the token must not already
			// be on screen before it is typed.
			if before := stripANSI(a.View().Content); strings.Contains(before, insertToken) {
				t.Fatalf("precondition: %q is already in the frame before typing", insertToken)
			}

			updateAndRender(t, a, keyPress('i'))
			if a.mode != ModeInsert {
				t.Fatalf("precondition: 'i' left mode=%v, want ModeInsert", a.mode)
			}
			for _, r := range insertToken {
				updateAndRender(t, a, keyPress(r))
			}

			// The published observation: the keystrokes are in the
			// compose belonging to the pane that is drawn, and they are
			// visible on screen.
			gotChannel, gotThread := a.compose.Value(), a.threadCompose.Value()
			wantIn, wantEmpty := &gotChannel, &gotThread
			drawnPane, hiddenPane := "channel", "thread"
			if tc.wantThreadCompose {
				wantIn, wantEmpty = &gotThread, &gotChannel
				drawnPane, hiddenPane = "thread", "channel"
			}
			if *wantIn != insertToken || *wantEmpty != "" {
				t.Errorf("typed %q went to the wrong compose: channel=%q thread=%q.\n"+
					"The frame draws the %s pane, so insert mode must focus the %s "+
					"compose; the %s compose is not on screen and anything typed into "+
					"it is silently lost.",
					insertToken, gotChannel, gotThread, drawnPane, drawnPane, hiddenPane)
			}
			if plain := stripANSI(a.View().Content); !strings.Contains(plain, insertToken) {
				t.Errorf("typed %q is absent from the rendered frame -- it landed in a compose "+
					"box the user cannot see.\n%s", insertToken, plain)
			}
		})
	}
}
