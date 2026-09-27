package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// F3: a zoom transition must disarm a pending ctrl+w chord.
//
// SetMode disarms it (app.go:2082, "a global intercept must not strand it
// armed"), but a zoom transition is NOT a mode change -- `z` and esc-to-exit
// both stay in ModeNormal -- so that guard never fires. And reduceZoom sits
// in the reducer chain (app.go:998) while pendingWinCmd is consumed in
// handleNormalMode (mode_normal.go:42), which runs AFTER the chain, so every
// arm of reduceZoom that returns true starves the chord.
//
// The trap is worse than a stranded flag: the chord's "ctrl+w …" hint lives
// in the status row, which zoom HIDES. So the chord is armed with no visible
// affordance, and the user's next keystroke is silently eaten as a window
// command.
//
// ctrl+w cannot be armed while zoomed (WindowPrefix is in zoomSuppresses), so
// the only reachable order is arm-then-zoom.
func TestFullscreen_ZoomTransitionDisarmsWindowChord(t *testing.T) {
	newApp := func(t *testing.T) *App {
		t.Helper()
		a := newTestApp(t,
			withWindowSize(120, 24),
			withChannels(goldenChannels()...),
			withMessages(testMessageItems(30)...),
			withActiveChannel("C1"),
		)
		focusMessages(t, a)
		return a
	}

	armChord := func(t *testing.T, a *App) {
		t.Helper()
		updateAndRender(t, a, keyMod('w', tea.ModCtrl))
		if !a.pendingWinCmd {
			t.Fatal("precondition: ctrl+w did not arm the chord")
		}
	}

	t.Run("entering zoom disarms it", func(t *testing.T) {
		a := newApp(t)
		armChord(t, a)

		updateAndRender(t, a, keyPress('z'))
		if !a.zoomed {
			t.Fatal("precondition: 'z' did not enter zoom")
		}
		if a.pendingWinCmd {
			t.Error("the ctrl+w chord is still armed after entering zoom: its " +
				"\"ctrl+w …\" hint lives in the status row, which zoom hides, so " +
				"the chord is armed with no visible affordance and the next " +
				"keystroke is silently eaten as a window command")
		}
	})

	t.Run("exiting zoom via z disarms it", func(t *testing.T) {
		a := newApp(t)
		updateAndRender(t, a, keyPress('z'))
		if !a.zoomed {
			t.Fatal("precondition: 'z' did not enter zoom")
		}
		// Leave zoom, arm, then re-enter and leave: the disarm must hold on
		// the exit edge too, not just the entry edge.
		updateAndRender(t, a, keyPress('z'))
		armChord(t, a)
		updateAndRender(t, a, keyPress('z'))
		updateAndRender(t, a, keyPress('z'))
		if a.zoomed {
			t.Fatal("precondition: second 'z' did not exit zoom")
		}
		if a.pendingWinCmd {
			t.Error("the ctrl+w chord survived a zoom enter/exit round trip")
		}
	})

	t.Run("exiting zoom via esc disarms it", func(t *testing.T) {
		a := newApp(t)
		armChord(t, a)
		updateAndRender(t, a, keyPress('z'))
		updateAndRender(t, a, keyCode(tea.KeyEscape))
		if a.zoomed {
			t.Fatal("precondition: esc did not exit zoom")
		}
		if a.pendingWinCmd {
			t.Error("the ctrl+w chord is still armed after esc exited zoom; " +
				"reducer_zoom's esc arm returns true, so handleNormalMode's " +
				"chord arm never runs")
		}
	})

	// The `g` chord is consumed in the same post-chain block and cleared in
	// the same SetMode guard, so it starves identically. One asymmetry: `g`
	// is NOT in zoomSuppresses, so unlike ctrl+w it can also be armed WHILE
	// zoomed -- both orders are reachable, and both are covered.
	t.Run("g chord: armed then zoom", func(t *testing.T) {
		a := newApp(t)
		updateAndRender(t, a, keyPress('g'))
		if !a.pendingTop {
			t.Fatal("precondition: 'g' did not arm the gg chord")
		}
		updateAndRender(t, a, keyPress('z'))
		if !a.zoomed {
			t.Fatal("precondition: 'z' did not enter zoom")
		}
		if a.pendingTop {
			t.Error("the gg chord is still armed after entering zoom; its " +
				"\"g …\" hint is in the hidden status row, so the next key is " +
				"consumed by the chord with no affordance shown")
		}
	})

	t.Run("g chord: armed while zoomed, then exit", func(t *testing.T) {
		a := newApp(t)
		updateAndRender(t, a, keyPress('z'))
		if !a.zoomed {
			t.Fatal("precondition: 'z' did not enter zoom")
		}
		updateAndRender(t, a, keyPress('g'))
		if !a.pendingTop {
			t.Fatal("precondition: 'g' did not arm the gg chord while zoomed " +
				"(g is not in zoomSuppresses, so it must reach handleNormalMode)")
		}
		updateAndRender(t, a, keyCode(tea.KeyEscape))
		if a.zoomed {
			t.Fatal("precondition: esc did not exit zoom")
		}
		if a.pendingTop {
			t.Error("the gg chord survived esc-exit from zoom")
		}
	})
}

// F4: TestFullscreen_TabWhileZoomedDoesNotFlipTheZoomedPane asserts only
// strings.Contains(plain, "wrapping behaviour"). At 200 cols both panes render
// side-by-side unzoomed -- the sibling golden subtest says so explicitly as its
// reason for choosing that width -- so that token is present whether Tab left
// the zoom alone, flipped it to the thread, or dropped zoom entirely. Its own
// comment concedes it "passes today (z is a no-op…)": it was written as a
// base-passing guard and never acquired a zoom-specific observable, unlike
// every other row in that file, which checks zoomedStatusToken.
//
// This row keeps the same intent and adds the two observables that
// discriminate: zoom is still ON, and the pane it is zooming is still
// messages (so the thread, which renders beside messages at 200 cols
// unzoomed, must be absent from the zoomed frame).
func TestFullscreen_TabWhileZoomedKeepsMessagesZoomed(t *testing.T) {
	a := newTestApp(t,
		withWindowSize(200, 24),
		withChannels(goldenChannels()...),
		withMessages(goldenMessages()...),
		withActiveChannel("C1"),
	)
	focusMessages(t, a)
	updateAndRender(t, a, keyCode(tea.KeyEnter))
	if !a.threadVisible {
		t.Fatal("precondition: Enter did not open the thread")
	}
	updateAndRender(t, a, keyPress('z')) // zoom the messages pane (front)
	if !a.zoomed {
		t.Fatal("precondition: 'z' did not enter zoom")
	}

	updateAndRender(t, a, keyCode(tea.KeyTab))

	if !a.zoomed {
		t.Error("Tab while zoomed dropped zoom entirely (G15); Tab must not " +
			"change the zoom state")
	}
	plain := stripANSI(a.View().Content)
	if !strings.Contains(plain, "wrapping behaviour") {
		t.Errorf("Tab while zoomed flipped the zoomed pane away from the messages content (G15):\n%s", plain)
	}
	// The discriminating half: at 200 cols the thread renders BESIDE messages
	// when unzoomed, so its absence is what proves messages is still the
	// zoomed pane rather than the frame having fallen back to the split.
	if strings.Contains(plain, zoomedStatusToken) {
		t.Errorf("frame still renders the status row (%q) after Tab, so it is "+
			"no longer a zoomed frame at all", zoomedStatusToken)
	}
}
