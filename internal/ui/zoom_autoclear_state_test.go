package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/ui/workspace"
)

// fs-zoom-invariant, pinned as STATE rather than as a frame delta.
//
// TestFullscreen_ZoomAutoClears (fullscreen_red_test.go) intends to pin the
// same rule but cannot fail. It captures frameIdle BEFORE zoom, then
// REASSIGNS it after the clearing event, and finally asserts that a 'z'
// press produces a different frame. Both outcomes satisfy that:
//
//	zoom cleared correctly -> frameIdle is unzoomed, 'z' ENTERS zoom -> differs -> pass
//	zoom leaked            -> frameIdle is zoomed,   'z' EXITS  zoom -> differs -> pass
//
// The only way to fail it is 'z' being inert, which is a different defect.
// Its own error string names the leak branch as something it detects, but
// the comparison it performs cannot distinguish the two.
//
// a.zoomed is the observable that actually discriminates, so these rows read
// it directly. Asserting on it is what makes the row falsifiable: nothing in
// the production tree clears zoom on any of these events -- exitZoom has
// exactly three call sites (the 'z' toggle, esc, and insert-mode's esc arm)
// and neither reducer_channels.go nor reducer_workspace.go mentions zoom at
// all -- so every row below fails until the auto-clear is implemented.
func TestFullscreen_ZoomAutoClearsStateNotFrame(t *testing.T) {
	// enterZoomState presses 'z' and fails unless zoom is actually on, so a
	// row can never "pass" by never having zoomed in the first place.
	enterZoomState := func(t *testing.T, a *App) {
		t.Helper()
		updateAndRender(t, a, keyPress('z'))
		if !a.zoomed {
			t.Fatal("precondition: 'z' did not enter zoom (a.zoomed is false)")
		}
	}

	t.Run("q closes the thread", func(t *testing.T) {
		a := newTestApp(t,
			withWindowSize(120, 24),
			withChannels(goldenChannels()...),
			withMessages(testMessageItems(30)...),
			withActiveChannel("C1"),
		)
		focusMessages(t, a)
		updateAndRender(t, a, keyCode(tea.KeyEnter))
		if !a.threadVisible {
			t.Fatal("precondition: Enter did not open the thread")
		}
		enterZoomState(t, a)

		updateAndRender(t, a, keyPress('q'))
		if a.threadVisible {
			t.Fatal("precondition: 'q' did not close the thread")
		}
		if a.zoomed {
			t.Error("zoom survived the thread close: the zoomed pane is gone but " +
				"a.zoomed is still true, stranding the user in a fullscreen view of " +
				"a closed pane with no nav affordances (G5 stale zoom)")
		}
	})

	t.Run("workspace switch", func(t *testing.T) {
		a := newTestApp(t,
			withSize(120, 24),
			withChannels(goldenChannels()...),
			withMessages(testMessageItems(30)...),
			withActiveChannel("C1"),
			withActiveTeam("T1"),
			withWorkspaces(
				workspace.WorkspaceItem{ID: "T1", Name: "alpha", Initials: "AL"},
				workspace.WorkspaceItem{ID: "T2", Name: "beta", Initials: "BE"},
			),
			withRender(),
		)
		focusMessages(t, a)
		enterZoomState(t, a)

		updateAndRender(t, a, WorkspaceSwitchedMsg{TeamID: "T2", TeamName: "beta", Channels: nil})
		if a.zoomed {
			t.Error("zoom survived the workspace switch: the switch resets stackFront, " +
				"so zoom must clear with it (G5)")
		}
	})

	t.Run("channel jump", func(t *testing.T) {
		a := newTestApp(t,
			withWindowSize(120, 24),
			withChannels(goldenChannels()...),
			withMessages(testMessageItems(30)...),
			withActiveChannel("C1"),
		)
		focusMessages(t, a)
		updateAndRender(t, a, keyCode(tea.KeyEnter))
		if !a.threadVisible {
			t.Fatal("precondition: Enter did not open the thread")
		}
		enterZoomState(t, a)

		updateAndRender(t, a, ChannelSelectedMsg{ID: "C2", Name: "engineering", Type: "channel"})
		if a.zoomed {
			t.Error("zoom survived the channel jump: exitZoom also restores the saved " +
				"viewport unconditionally, so a later exit stamps the old channel's " +
				"offset and selected index onto the new channel's pane (G5)")
		}
	})
}
