package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"
)

// TestInsertMode_ThreadsView covers `i` in the Threads view across the two
// reachable no-reply-box conditions AND the states where `i` must still work.
//
// The original four rows set threadVisible=false in every one of them (no
// Enter, and withThreadsView(nil)), so the `zoomed` dimension never reached
// the branch it named: all four exercised the same condition and asserted the
// same string. Rows now say which toast they expect, with "" meaning `i` must
// be accepted -- so a change that toasts everywhere fails here instead of
// looking like broader coverage.
//
// Zoom is entered by pressing 'z' through the real chain. The original
// assigned a.zoomed = true directly, which AGENTS.md forbids: enterZoom also
// snapshots the viewport and normalises focus, and skipping it tests a state
// the app never actually reaches.
func TestInsertMode_ThreadsView(t *testing.T) {
	cases := []struct {
		name string
		// openThread presses Enter first, the route the 2026-04-28 spec
		// names as the only way to a reply box in this view.
		openThread bool
		width      int
		zoomed     bool
		// wantToast is the exact toast expected, or "" when `i` must be
		// accepted and enter insert mode.
		wantToast string
	}{
		{"no thread, stacked", false, stackedWidth, false, threadsNoThreadOpenToast},
		{"no thread, side by side", false, sideBySideWidth, false, threadsNoThreadOpenToast},
		{"no thread, zoomed", false, sideBySideWidth, true, threadsNoThreadOpenToast},
		// Thread open: zoom at a side-by-side width promotes MESSAGES, so the
		// reply box exists but is drawn on no frame. This is the only state
		// where the zoom remedy is the right one.
		{"thread open, side by side, zoomed", true, sideBySideWidth, true, threadsZoomHidesReplyToast},
		// And the states where `i` must NOT toast, which is what stops the
		// guard from being too broad.
		{"thread open, side by side, unzoomed", true, sideBySideWidth, false, ""},
		{"thread open, stacked, zoomed: zoom promotes the THREAD", true, stackedWidth, true, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp(t, append(normalOpts(),
				withWindowSize(tc.width, 30),
				withView(ViewThreads),
				withThreadsView(threadsViewSummaries()),
			)...)
			if a.view != ViewThreads {
				t.Fatalf("precondition: view=%v, want ViewThreads", a.view)
			}

			// The threads list lives in the messages region.
			a.focusedPanel = PanelMessages
			if tc.openThread {
				updateAndRender(t, a, keyCode(tea.KeyEnter))
				if !a.threadVisible {
					t.Fatal("precondition: Enter did not open a thread")
				}
			}
			if tc.zoomed {
				updateAndRender(t, a, keyPress('z'))
				if !a.zoomed {
					t.Fatal("precondition: 'z' did not enter zoom")
				}
			}
			if a.threadVisible != tc.openThread {
				t.Fatalf("precondition: threadVisible=%v, want %v -- the row's zoom "+
					"dimension only reaches the guard when this matches",
					a.threadVisible, tc.openThread)
			}

			updateAndRender(t, a, keyPress('i'))

			if tc.wantToast == "" {
				if a.mode != ModeInsert {
					t.Fatalf("`i` did not enter insert mode (mode=%v) in a state where a "+
						"reply box IS drawn -- the guard is too broad", a.mode)
				}
				return
			}

			if a.mode == ModeInsert {
				t.Errorf("`i` entered insert mode, want ModeNormal: there is no drawn " +
					"compose box in this state, so focusing one strands the keystroke")
			}
			screen := ansi.Strip(a.View().Content)
			if !strings.Contains(screen, tc.wantToast) {
				t.Errorf("toast %q absent from the frame.\nEach condition has its own "+
					"remedy -- naming the wrong one is worse than naming none.", tc.wantToast)
			}
			// The OTHER toast must be absent, or one string satisfying every row
			// would pass and the two remedies would be interchangeable.
			other := threadsZoomHidesReplyToast
			if tc.wantToast == threadsZoomHidesReplyToast {
				other = threadsNoThreadOpenToast
			}
			if strings.Contains(screen, other) {
				t.Errorf("the wrong remedy is on screen: got %q, want only %q", other, tc.wantToast)
			}

			// A following keystroke must not leak into either compose.
			updateAndRender(t, a, keyPress('x'))
			if c, th := a.compose.Value(), a.threadCompose.Value(); strings.Contains(c, "x") || strings.Contains(th, "x") {
				t.Errorf("keystroke 'x' leaked into a compose box: chan=%q thread=%q", c, th)
			}
		})
	}
}

func TestInsertMode_ChannelsViewControl(t *testing.T) {
	// "Add a control row asserting ViewChannels still enters insert mode normally"
	a := newTestApp(t, withWindowSize(200, 30), withView(ViewChannels))
	if a.view != ViewChannels {
		t.Fatal("precondition: view is not ViewChannels")
	}

	updateAndRender(t, a, keyPress('i'))
	if a.mode != ModeInsert {
		t.Errorf("App did not enter ModeInsert in ViewChannels, got %v", a.mode)
	}
}
