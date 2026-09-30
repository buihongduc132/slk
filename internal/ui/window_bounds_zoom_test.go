// internal/ui/window_bounds_zoom_test.go
//
// B48 / OT21, second half: windowBounds (windows.go) passes a hardcoded
// threadFront=false to Compute while forwarding a.zoomed live. The
// asymmetry looks like a bug. Measured, it is not -- and these are the
// checks that keep it not-a-bug.
//
// VERDICT: safe, for two separate reasons depending on zoom state.
//
//  1. UNZOOMED it is the documented contract: windows are only drawn when
//     the channel is in front, so the rectangle the window tree
//     subdivides is always the channel-in-front area, even while a
//     stacked thread is the pane actually on screen. Already pinned by
//     TestStacked_WindowSplitWithThreadInFront.
//
//  2. ZOOMED there is exactly one state where the hardcoded false
//     disagrees with the frame on screen: stacked (< 162 cols) with the
//     thread promoted. Measured at 120x30, windowBounds returns W=120
//     (the messages-zoomed area) while the live frame has MsgWidth=0 and
//     ThreadWidth=118. That state is UNREACHABLE: windowBounds has two
//     callers, splitWindow and navigateWindow, and every route to them --
//     the ctrl+w chord and the :sp / :vsp commands -- is closed while
//     zoomed. zoomSuppresses swallows WindowPrefix and CommandMode, and
//     enterZoom calls disarmPendingChords so an armed chord cannot
//     survive the transition.
//
// So the fix is a check, not a code change: TestWindowBounds_RectMatches
// pins the rectangle against the frame in every reachable state, and
// TestWindowBounds_ZoomedThreadPromotedUnreachable pins the reachability
// argument that licenses the one divergence. Removing WindowPrefix or
// CommandMode from zoomSuppresses fails the second test, which is the
// signal to revisit the first.
package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/ui/wintree"
)

// TestWindowBounds_RectMatches asserts windowBounds agrees with the
// messages rectangle the live frame actually draws, in every state a
// window command can be issued from. Where the frame draws no messages
// pane at all (thread stacked in front) there is nothing to agree with,
// and the channel-in-front contract applies instead.
func TestWindowBounds_RectMatches(t *testing.T) {
	for _, tc := range []struct {
		name  string
		w     int
		setup func(t *testing.T, a *App)
	}{
		{"stacked, messages in front", stackedWidth, openThreadThenFocusMessages},
		{"stacked, thread in front", stackedWidth, openThread},
		{"side by side", sideBySideWidth, openThread},
		{"side by side, no thread", sideBySideWidth, nil},
		{"zoomed, messages promoted", stackedWidth, func(t *testing.T, a *App) {
			openThreadThenFocusMessages(t, a)
			updateAndRender(t, a, keyPress('z'))
		}},
		{"zoomed, no thread", sideBySideWidth, func(t *testing.T, a *App) {
			updateAndRender(t, a, keyPress('z'))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := stackedApp(t, tc.w)
			if tc.setup != nil {
				tc.setup(t, a)
			}

			bands := *a.layout
			frame := a.computeFrame()
			got := a.windowBounds()

			if frame.MsgWidth == 0 {
				// The thread is the pane on screen. windowBounds must
				// still describe the channel-in-front area, because that
				// is the geometry the windows fill once the channel comes
				// back to the front.
				if !a.threadDrawnAlone() {
					t.Fatalf("frame draws no messages pane but threadDrawnAlone() = false (frame %+v)", frame)
				}
				want := wintree.Rect{W: tc.w - testRailW - testSidebarW - 2, H: frame.ContentHeight}
				if got != want {
					t.Errorf("windowBounds = %+v, want %+v (the channel-in-front area)", got, want)
				}
				return
			}

			want := wintree.Rect{W: frame.MsgWidth + frame.MsgBorder, H: frame.ContentHeight}
			if got != want {
				t.Errorf("windowBounds = %+v, want %+v -- the rect the window tree "+
					"subdivides must match the messages region renderWindowsRegion draws",
					got, want)
			}

			// The scratch layout must not disturb the stored hit-test bands.
			if *a.layout != bands {
				t.Errorf("windowBounds overwrote the hit-test bands: %+v -> %+v", bands, *a.layout)
			}
		})
	}
}

// TestWindowBounds_ZoomedThreadPromotedUnreachable pins the reachability
// argument in this file's header: in the one state where windowBounds'
// hardcoded threadFront=false disagrees with the frame, no window command
// can be issued. If this test fails, windowBounds needs to pass
// layoutThreadFront() (or the callers need a guard) before whatever
// opened the route can ship.
func TestWindowBounds_ZoomedThreadPromotedUnreachable(t *testing.T) {
	// zoomedThreadApp is the divergent state itself: stacked, thread
	// promoted, zoomed.
	zoomedThreadApp := func(t *testing.T) *App {
		t.Helper()
		a := stackedApp(t, stackedWidth)
		openThread(t, a)
		updateAndRender(t, a, keyPress('z'))
		if !a.zoomed || !a.zoomFrontIsThread() {
			t.Fatalf("precondition: zoomed=%v zoomFrontIsThread=%v, want both true",
				a.zoomed, a.zoomFrontIsThread())
		}
		if frame := a.computeFrame(); frame.MsgWidth != 0 {
			t.Fatalf("precondition: frame draws a messages pane (%+v); this test is "+
				"about the state where it does not", frame)
		}
		return a
	}

	t.Run("the suppression set covers both routes", func(t *testing.T) {
		a := zoomedThreadApp(t)
		for _, tc := range []struct {
			name string
			key  tea.KeyMsg
		}{
			{"ctrl+alt+w (chord prefix -> splitWindow / navigateWindow)", keyMod('w', tea.ModCtrl|tea.ModAlt)},
			{": (command mode -> :sp / :vsp)", keyPress(':')},
		} {
			if !a.zoomSuppresses(tc.key) {
				t.Errorf("zoomSuppresses(%s) = false: this opens a route to windowBounds "+
					"while the thread is the zoomed pane, where it returns the "+
					"messages-zoomed rect for a frame that draws no messages pane",
					tc.name)
			}
		}
	})

	t.Run("ctrl+w v does not split", func(t *testing.T) {
		a := zoomedThreadApp(t)
		updateAndRender(t, a, keyMod('w', tea.ModCtrl))
		if a.pendingWinCmd {
			t.Error("ctrl+w armed the window chord while zoomed")
		}
		updateAndRender(t, a, keyPress('v'))
		if a.wins.Len() != 1 {
			t.Errorf("wins = %d, want 1: a split reached windowBounds while the thread "+
				"was the zoomed pane", a.wins.Len())
		}
	})

	t.Run("colon does not open command mode", func(t *testing.T) {
		a := zoomedThreadApp(t)
		updateAndRender(t, a, keyPress(':'))
		if a.mode != ModeNormal {
			t.Errorf("mode = %v, want ModeNormal: :sp / :vsp would reach windowBounds", a.mode)
		}
	})

	// Insert mode is the gap in the suppression rule -- zoomSuppresses is
	// gated on ModeNormal -- so it gets its own row. It is safe for a
	// different reason: handleInsertMode has no window-chord arm at all,
	// so ctrl+w never reaches handleWindowChord and ':' just types.
	t.Run("insert mode is not a bypass", func(t *testing.T) {
		a := zoomedThreadApp(t)
		updateAndRender(t, a, keyPress('i'))
		if a.mode != ModeInsert {
			t.Fatalf("precondition: mode = %v, want ModeInsert", a.mode)
		}
		updateAndRender(t, a, keyMod('w', tea.ModCtrl))
		updateAndRender(t, a, keyPress('v'))
		if a.wins.Len() != 1 {
			t.Errorf("wins = %d, want 1: ctrl+w in insert mode while zoomed reached "+
				"windowBounds", a.wins.Len())
		}
	})
}

// openThread opens the thread on the selected message, leaving it
// focused (and, when stacked, in front).
func openThread(t *testing.T, a *App) {
	t.Helper()
	updateAndRender(t, a, keyCode(tea.KeyEnter))
	if !a.threadVisible {
		t.Fatal("precondition: Enter did not open the thread")
	}
}

// openThreadThenFocusMessages opens the thread and hands focus back to
// the channel pane, so the messages pane is the one in front.
func openThreadThenFocusMessages(t *testing.T, a *App) {
	t.Helper()
	openThread(t, a)
	updateAndRender(t, a, keyMod(tea.KeyTab, tea.ModShift))
	if a.focusedPanel != PanelMessages || !a.threadVisible {
		t.Fatalf("precondition: focus=%v threadVisible=%v, want channel focused with the "+
			"thread still open", a.focusedPanel, a.threadVisible)
	}
}
