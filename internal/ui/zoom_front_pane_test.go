// internal/ui/zoom_front_pane_test.go
//
// B48 / OT21: pins the contract in zoomFrontIsThread's doc comment
// (app.go) so it cannot silently regress.
//
// The plan text recorded B48 as a defect -- "`z` zooms the messages pane
// even when the thread is focused at side-by-side widths". The code is
// deliberate and the plan text was stale. zoom promotes the thread
// exactly when the UNZOOMED layout would have stacked and left the
// messages pane undrawn; where both panes fit, there is no pane in front
// of the other and zoom promotes messages. Probing the unzoomed layout
// rather than reading focusedPanel is what keeps Tab from flipping WHICH
// pane is zoomed (G15).
//
// That contract lived only in prose, which AGENTS.md forbids for an
// invariant two call sites must respect ("when you find a comment
// standing in for a check, replace it with the check"). This file is the
// check.
//
// WIDTHS ARE MEASURED, NOT ASSUMED. With the shared fixture's 6-col
// workspace rail and 30-col sidebar (+2 border), the side-by-side branch
// in panelLayout.Compute first has room at exactly 162 cols: 161 stacks,
// 162 draws both. The rows below use 200 (both fit, comfortably past the
// boundary), 120 (stacked) and the 161/162 boundary pair.
package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// sideBySideWidth / stackedWidth are the two measured sides of the
// panelLayout.Compute branch this file is about. firstSideBySideWidth is
// the exact boundary: firstSideBySideWidth-1 stacks.
const (
	stackedWidth         = 120
	sideBySideWidth      = 200
	firstSideBySideWidth = 162
)

// TestZoomFrontPane_G15_SideBySideAlwaysZoomsMessages is the
// discriminating half: at a width where both panes fit, zoom must
// promote MESSAGES no matter where focus sits. A zoomFrontIsThread that
// read focusedPanel would zoom the thread in both subtests here.
//
// Complements TestFullscreen_TabWhileZoomedKeepsMessagesZoomed
// (zoom_chord_tab_test.go), which presses Tab AFTER zooming. These rows
// move focus to the thread BEFORE zooming, which is the order the stale
// B48 report described.
func TestZoomFrontPane_G15_SideBySideAlwaysZoomsMessages(t *testing.T) {
	t.Run("thread opened and focused via Enter", func(t *testing.T) {
		a := stackedApp(t, sideBySideWidth)
		updateAndRender(t, a, keyCode(tea.KeyEnter))
		if !a.threadVisible || a.focusedPanel != PanelThread {
			t.Fatalf("precondition: threadVisible=%v focus=%v, want thread open and focused",
				a.threadVisible, a.focusedPanel)
		}
		assertFront(t, a, true, true) // precondition: both panes drawn unzoomed

		updateAndRender(t, a, keyPress('z'))
		assertZoomedPaneIsMessages(t, a, "thread focused via Enter")
	})

	t.Run("focus moved to the thread with Tab", func(t *testing.T) {
		a := stackedApp(t, sideBySideWidth)
		focusThreadPanel(t, a)                     // seed thread content
		a.focusedPanel = PanelMessages             // start on messages
		updateAndRender(t, a, keyCode(tea.KeyTab)) // messages -> thread
		if a.focusedPanel != PanelThread {
			t.Fatalf("precondition: Tab left focus on %v, want PanelThread", a.focusedPanel)
		}
		assertFront(t, a, true, true)

		updateAndRender(t, a, keyPress('z'))
		assertZoomedPaneIsMessages(t, a, "focus moved to the thread with Tab")

		// The strongest observable: at this width the thread renders
		// BESIDE messages when unzoomed, so its content being absent from
		// the zoomed frame is what proves messages is the zoomed pane.
		plain := stripANSI(a.View().Content)
		for _, tok := range []string{"reply-1", "reply-2"} {
			if strings.Contains(plain, tok) {
				t.Errorf("zoomed frame still shows thread content %q: zoom promoted the "+
					"THREAD because focus was there. zoomFrontIsThread must probe the "+
					"unzoomed layout, not focusedPanel (G15).\n%s", tok, plain)
			}
		}
	})
}

// TestZoomFrontPane_G15_StackedZoomsWhicheverPaneIsDrawn is the other
// side of the same branch: where the panes stack, zoom promotes whichever
// one the unzoomed layout was actually drawing. Focus decides that (via
// threadInFront), so here -- and only here -- zoom follows focus.
func TestZoomFrontPane_G15_StackedZoomsWhicheverPaneIsDrawn(t *testing.T) {
	t.Run("thread drawn alone: z zooms the thread", func(t *testing.T) {
		a := stackedApp(t, stackedWidth)
		updateAndRender(t, a, keyCode(tea.KeyEnter))
		assertFront(t, a, false, true) // precondition: thread alone, messages undrawn
		if !a.zoomFrontIsThread() {
			t.Fatal("precondition: zoomFrontIsThread() = false with the thread drawn alone")
		}

		updateAndRender(t, a, keyPress('z'))
		if !a.zoomed {
			t.Fatal("'z' did not enter zoom")
		}
		assertFront(t, a, false, true)
		if a.layout.threadEnd-a.layout.msgEnd != a.width {
			t.Errorf("thread band = %d cols, want the full %d: the thread was drawn alone "+
				"unzoomed, so zoom must promote it", a.layout.threadEnd-a.layout.msgEnd, a.width)
		}
	})

	t.Run("messages drawn alone: z zooms messages", func(t *testing.T) {
		a := stackedApp(t, stackedWidth)
		updateAndRender(t, a, keyCode(tea.KeyEnter))
		updateAndRender(t, a, keyMod(tea.KeyTab, tea.ModShift)) // thread -> messages
		assertFront(t, a, true, false)                          // precondition: messages alone
		if !a.threadVisible {
			t.Fatal("precondition: shift+tab closed the thread")
		}

		updateAndRender(t, a, keyPress('z'))
		assertZoomedPaneIsMessages(t, a, "messages drawn alone while stacked")
	})
}

// TestZoomFrontPane_G15_BoundaryWidth pins the branch edge itself: one
// column decides which pane zoom promotes, with focus held on the thread
// throughout. 161 stacks with the thread in front, so zoom takes the
// thread; 162 fits both, so zoom takes messages.
func TestZoomFrontPane_G15_BoundaryWidth(t *testing.T) {
	for _, tc := range []struct {
		width      string
		w          int
		wantThread bool
	}{
		{"161 stacks", firstSideBySideWidth - 1, true},
		{"162 fits both", firstSideBySideWidth, false},
	} {
		t.Run(tc.width, func(t *testing.T) {
			a := stackedApp(t, tc.w)
			updateAndRender(t, a, keyCode(tea.KeyEnter)) // thread open AND focused
			if a.focusedPanel != PanelThread {
				t.Fatalf("precondition: focus = %v, want PanelThread", a.focusedPanel)
			}
			if got := a.zoomFrontIsThread(); got != tc.wantThread {
				t.Fatalf("zoomFrontIsThread() at w=%d = %v, want %v", tc.w, got, tc.wantThread)
			}

			updateAndRender(t, a, keyPress('z'))
			if !a.zoomed {
				t.Fatal("'z' did not enter zoom")
			}
			assertFront(t, a, !tc.wantThread, tc.wantThread)
		})
	}
}

// assertZoomedPaneIsMessages asserts the App is zoomed with MESSAGES as
// the promoted pane: zoom is on, the messages band spans the terminal,
// the thread band is collapsed, and the status row is gone (a zoomed
// frame does not composite it).
func assertZoomedPaneIsMessages(t *testing.T, a *App, why string) {
	t.Helper()
	if !a.zoomed {
		t.Fatalf("%s: 'z' did not enter zoom", why)
	}
	if a.zoomFrontIsThread() {
		t.Errorf("%s: zoomFrontIsThread() = true, want false -- zoom must promote "+
			"messages here (G15)", why)
	}
	assertFront(t, a, true, false)
	if got := a.layout.msgEnd - a.layout.sidebarEnd; got != a.width {
		t.Errorf("%s: messages band = %d cols, want the full %d", why, got, a.width)
	}
	if plain := stripANSI(a.View().Content); strings.Contains(plain, zoomedStatusToken) {
		t.Errorf("%s: frame still renders the status row (%q), so it is not a zoomed frame",
			why, zoomedStatusToken)
	}
}
