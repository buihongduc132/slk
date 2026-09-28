// internal/ui/zoom_focus_drawn_pane_test.go
//
// FOCUS must not point at a content pane that zoom does not draw.
//
// THE DEFECT, measured at 200x30 with the thread open and focused, then
// `z`:
//
//	focus=PanelThread  zoomed=true  threadDrawnAlone=false
//	frame: MsgWidth=198  ThreadWidth=0
//	after `i` then "wombat": threadCompose="wombat", absent from the frame
//
// The sibling defect (threadDrawnAlone answering for a frame other than
// the one drawn) is fixed and pinned by zoom_insert_drawn_pane_test.go.
// That fix is correct and did NOT close this one: threadDrawnAlone
// reports false here, correctly, and the keystroke still vanishes --
// because mode_normal.go's `i` arm is an || chain whose FIRST clause is
// `focusedPanel == PanelThread`, which short-circuits before the helper
// is ever consulted. Focus alone routes typing into a compose box with
// zero width that no frame renders.
//
// So the two files are deliberately complementary and neither subsumes
// the other:
//
//   - zoom_insert_drawn_pane_test.go presses Tab to move focus to the
//     SIDEBAR, precisely so the first two clauses are false and
//     threadDrawnAlone is the only thing under test.
//   - this file leaves focus ON THE THREAD, so the first clause decides
//     and threadDrawnAlone is irrelevant.
//
// AND IT IS NOT ONLY `i`. Eight production sites route on
// `focusedPanel == PanelThread` (mode_normal's `i`, reaction picker and
// reaction-nav arms; mode_insert's send/upload arms; reducer_io's paste
// arms; editor.go's external-editor target). Every one of them is aimed
// at an undrawn pane in this state. The fix is therefore to stop the
// state existing rather than to teach eight call sites to double-check,
// and the reducer_zoom.go comment on threadFocusable says why.
//
// WIDTHS ARE RE-MEASURED, NOT INHERITED. With the shared fixture's 6-col
// rail and 30-col sidebar (+2 border) the side-by-side branch first has
// room at 162 cols. Measured MsgWidth/ThreadWidth after Enter then `z`,
// thread focused, BEFORE this fix:
//
//	w=80  0/78    w=120 0/118   w=160 0/158   w=161 0/159   <- thread promoted
//	w=162 160/0   w=200 198/0                               <- messages promoted
//
// The rows below use 120 (stacked), 200 (side by side) and the 161/162
// boundary pair, so a fix that hardcodes either answer fails a row.
//
// OBSERVES, NEVER RECOMPUTES. Every assertion reads published state: the
// rendered frame via stripANSI(a.View().Content), the drawn bands via
// assertFront (stored by the last real render), and compose.Value() /
// threadCompose.Value(). Nothing here re-derives what focusedPanel
// "should" be.
package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// typeToken enters insert mode and types tok one rune at a time through
// the real Update chain, rendering after each.
func typeToken(t *testing.T, a *App, tok string) {
	t.Helper()
	if before := stripANSI(a.View().Content); strings.Contains(before, tok) {
		t.Fatalf("precondition: %q is already in the frame before typing", tok)
	}
	updateAndRender(t, a, keyPress('i'))
	if a.mode != ModeInsert {
		t.Fatalf("precondition: 'i' left mode=%v, want ModeInsert", a.mode)
	}
	for _, r := range tok {
		updateAndRender(t, a, keyPress(r))
	}
}

// assertTypedVisible is the whole point of the file: whatever was typed
// must be in the compose belonging to the pane the frame draws, and must
// be on screen.
func assertTypedVisible(t *testing.T, a *App, tok string, wantThreadCompose bool) {
	t.Helper()
	gotChannel, gotThread := a.compose.Value(), a.threadCompose.Value()
	wantIn, wantEmpty, drawn, hidden := gotChannel, gotThread, "channel", "thread"
	if wantThreadCompose {
		wantIn, wantEmpty, drawn, hidden = gotThread, gotChannel, "thread", "channel"
	}
	if wantIn != tok || wantEmpty != "" {
		t.Errorf("typed %q went to the wrong compose: channel=%q thread=%q.\n"+
			"The frame draws the %s pane, so typing must land in the %s compose; "+
			"the %s compose has zero width here and anything typed into it is "+
			"silently lost.", tok, gotChannel, gotThread, drawn, drawn, hidden)
	}
	if plain := stripANSI(a.View().Content); !strings.Contains(plain, tok) {
		t.Errorf("typed %q is absent from the rendered frame -- it landed in a compose "+
			"box the user cannot see.\n%s", tok, plain)
	}
}

// TestZoomFocus_ThreadFocusedTypingLandsInTheDrawnPane is the reported
// defect. No Tab, no seeded state: Enter (open and focus the thread), z,
// i, text.
func TestZoomFocus_ThreadFocusedTypingLandsInTheDrawnPane(t *testing.T) {
	for _, tc := range []struct {
		name string
		w    int
		// The panes the ZOOMED frame draws, asserted before typing.
		wantChannelDrawn, wantThreadDrawn bool
	}{
		{"stacked: zoom promotes the thread", stackedWidth, false, true},
		{"161 stacks: zoom promotes the thread", firstSideBySideWidth - 1, false, true},
		{"162 fits both: zoom promotes messages", firstSideBySideWidth, true, false},
		{"side by side: zoom promotes messages", sideBySideWidth, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := stackedApp(t, tc.w)
			updateAndRender(t, a, keyCode(tea.KeyEnter))
			if !a.threadVisible || a.focusedPanel != PanelThread {
				t.Fatalf("precondition: threadVisible=%v focus=%v, want the thread open "+
					"and focused", a.threadVisible, a.focusedPanel)
			}

			updateAndRender(t, a, keyPress('z'))
			if !a.zoomed {
				t.Fatal("precondition: 'z' did not enter zoom")
			}
			assertFront(t, a, tc.wantChannelDrawn, tc.wantThreadDrawn)

			typeToken(t, a, insertToken)
			assertTypedVisible(t, a, insertToken, tc.wantThreadDrawn)
		})
	}
}

// TestZoomFocus_ThreadOpenedWhileZoomedTypingLandsInTheDrawnPane is the
// second path into the same state, and the reason the fix cannot live in
// enterZoom alone: here zoom is already on when the thread opens, so
// enterZoom never runs. `z` (zoom the channel), Enter (open a thread --
// openThreadPanel sets focus to PanelThread unconditionally), i, text.
func TestZoomFocus_ThreadOpenedWhileZoomedTypingLandsInTheDrawnPane(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		w                                 int
		wantChannelDrawn, wantThreadDrawn bool
	}{
		{"stacked: the opened thread is promoted", stackedWidth, false, true},
		{"side by side: messages stays promoted", sideBySideWidth, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := stackedApp(t, tc.w)
			updateAndRender(t, a, keyPress('z')) // zoom with no thread open
			if !a.zoomed || a.threadVisible {
				t.Fatalf("precondition: zoomed=%v threadVisible=%v, want zoomed with no "+
					"thread", a.zoomed, a.threadVisible)
			}

			updateAndRender(t, a, keyCode(tea.KeyEnter)) // open a thread WHILE zoomed
			if !a.threadVisible {
				t.Fatal("precondition: Enter did not open the thread")
			}
			if !a.zoomed {
				t.Fatal("precondition: opening a thread dropped zoom")
			}
			assertFront(t, a, tc.wantChannelDrawn, tc.wantThreadDrawn)

			typeToken(t, a, insertToken)
			assertTypedVisible(t, a, insertToken, tc.wantThreadDrawn)
		})
	}
}

// TestZoomFocus_TabDuringZoomNeverStrandsTyping covers constraint 2: Tab
// moves focus while zoomed, and threadInFront reads focus, so a fix that
// only normalises at the zoom transition reopens the hole on the next
// Tab. Measured before the fix at w=200: focus cycles
// thread -> sidebar -> messages -> thread, and the third Tab is back on
// the undrawn thread with zoom still promoting messages.
//
// Each row is a fresh App so the count of Tabs is the only variable;
// typing and then escaping would exit zoom (insert mode's esc arm), which
// would make the later rows test something else.
func TestZoomFocus_TabDuringZoomNeverStrandsTyping(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		w                                 int
		wantChannelDrawn, wantThreadDrawn bool
	}{
		{"side by side: messages stays the zoomed pane", sideBySideWidth, true, false},
		{"stacked: the thread stays the zoomed pane", stackedWidth, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for tabs := 0; tabs <= 4; tabs++ {
				a := stackedApp(t, tc.w)
				updateAndRender(t, a, keyCode(tea.KeyEnter))
				updateAndRender(t, a, keyPress('z'))
				if !a.zoomed {
					t.Fatal("precondition: 'z' did not enter zoom")
				}
				for i := 0; i < tabs; i++ {
					updateAndRender(t, a, keyCode(tea.KeyTab))
				}
				if !a.zoomed {
					t.Fatalf("%d Tab(s) dropped zoom; Tab must not change the zoom state (G15)", tabs)
				}

				// G15 generalised: Tab moves focus, never which pane zoom
				// promotes -- at EITHER width.
				assertFront(t, a, tc.wantChannelDrawn, tc.wantThreadDrawn)

				typeToken(t, a, insertToken)
				if plain := stripANSI(a.View().Content); !strings.Contains(plain, insertToken) {
					t.Errorf("after %d Tab(s) while zoomed, typed %q is absent from the frame: "+
						"channel compose=%q thread compose=%q. Tab left focus on a pane the "+
						"zoomed frame does not draw.\n%s",
						tabs, insertToken, a.compose.Value(), a.threadCompose.Value(), plain)
				}
			}
		})
	}
}

// TestZoomFocus_ExitRestoresThreadFocus covers constraint 3, the way out.
// enterZoom moves focus off the thread when zoom promotes messages; the
// USER-INITIATED exit (`z` again, esc) must hand it back, the way exitZoom
// already hands back the saved viewport. At 200 cols both panes are drawn
// once zoom is gone, so the token is on screen either way -- which compose
// holds it is the observation that discriminates.
func TestZoomFocus_ExitRestoresThreadFocus(t *testing.T) {
	for _, exit := range []struct {
		name string
		key  tea.KeyMsg
	}{
		{"z toggles back", keyPress('z')},
		{"esc exits zoom", keyCode(tea.KeyEscape)},
	} {
		t.Run(exit.name, func(t *testing.T) {
			a := stackedApp(t, sideBySideWidth)
			updateAndRender(t, a, keyCode(tea.KeyEnter))
			updateAndRender(t, a, keyPress('z'))
			assertFront(t, a, true, false) // precondition: zoom promoted messages

			updateAndRender(t, a, exit.key)
			if a.zoomed {
				t.Fatalf("precondition: %s did not leave zoom", exit.name)
			}
			if !a.threadVisible {
				t.Fatal("precondition: leaving zoom closed the thread")
			}
			assertFront(t, a, true, true) // both panes back

			typeToken(t, a, insertToken)
			assertTypedVisible(t, a, insertToken, true)
		})
	}
}

// TestZoomFocus_ExitKeepsFocusTheUserMoved is the other half of the
// restore: it is guarded, not unconditional. If the user Tabbed away
// while zoomed, leaving zoom must not yank focus back to the thread.
func TestZoomFocus_ExitKeepsFocusTheUserMoved(t *testing.T) {
	a := stackedApp(t, sideBySideWidth)
	updateAndRender(t, a, keyCode(tea.KeyEnter))
	updateAndRender(t, a, keyPress('z'))
	updateAndRender(t, a, keyCode(tea.KeyTab)) // focus away from the content panes
	if a.focusedPanel != PanelSidebar {
		t.Fatalf("precondition: focus=%v, want PanelSidebar", a.focusedPanel)
	}
	updateAndRender(t, a, keyPress('z'))
	if a.zoomed {
		t.Fatal("precondition: 'z' did not leave zoom")
	}
	if a.focusedPanel != PanelSidebar {
		t.Errorf("leaving zoom moved focus from the sidebar to %v: the restore must only "+
			"undo the move enterZoom itself made", a.focusedPanel)
	}
	// And typing from the sidebar with both panes drawn still goes to the
	// channel compose, as it does unzoomed.
	typeToken(t, a, insertToken)
	assertTypedVisible(t, a, insertToken, false)
}

// TestZoomFocus_AutoClearDoesNotRestoreThreadFocus is the clearZoom path:
// a channel jump closes the thread and drops zoom without restoring
// anything. The thread is gone, so there is no focus to give back and
// typing must go to the channel compose.
func TestZoomFocus_AutoClearDoesNotRestoreThreadFocus(t *testing.T) {
	a := stackedApp(t, sideBySideWidth)
	updateAndRender(t, a, keyCode(tea.KeyEnter))
	updateAndRender(t, a, keyPress('z'))
	assertFront(t, a, true, false)

	updateAndRender(t, a, ChannelSelectedMsg{ID: "C2", Name: "random", Type: "channel"})
	if a.threadVisible {
		t.Fatal("precondition: the channel jump left the thread open")
	}
	if a.zoomed {
		t.Fatal("precondition: the channel jump left zoom on (fs-zoom-invariant)")
	}
	assertFront(t, a, true, false)

	typeToken(t, a, insertToken)
	assertTypedVisible(t, a, insertToken, false)
}
