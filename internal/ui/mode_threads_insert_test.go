package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestInsertMode_ThreadsView(t *testing.T) {
	// "Test both a stacked width and a side-by-side width, zoomed and unzoomed — four states."
	cases := []struct {
		name   string
		width  int
		zoomed bool
	}{
		{"stacked_unzoomed", 120, false},
		{"side_by_side_unzoomed", 200, false},
		{"stacked_zoomed", 120, true},
		{"side_by_side_zoomed", 200, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := []testOpt{
				withWindowSize(tc.width, 30),
				withView(ViewThreads),
				withThreadsView(nil),
			}
			a := newTestApp(t, opts...)

			if tc.zoomed {
				// To enter zoom, we first need to ensure we can.
				a.zoomed = true
			}

			if a.view != ViewThreads {
				t.Fatal("precondition: view is not ViewThreads")
			}

			// We press 'i'. This should raise a toast and NOT enter insert mode, nor type 'i'.
			// wait, if we press 'i' as key press, it goes to insert mode if bug is present.
			// we want to ensure 'i' is swallowed and a toast is raised.
			// Let's also press 'x' after 'i' and ensure 'x' does not appear in compose.

			updateAndRender(t, a, keyPress('i'))
			updateAndRender(t, a, keyPress('x'))

			if a.mode == ModeInsert {
				t.Errorf("App entered ModeInsert, want ModeNormal")
			}

			screen := ansi.Strip(a.View().Content)
			if !strings.Contains(screen, "No message box in Threads view") {
				t.Errorf("Toast 'No message box in Threads view' not found in rendered frame")
			}

			chanComposeStr := a.compose.Value()
			threadComposeStr := a.threadCompose.Value()

			if strings.Contains(chanComposeStr, "x") || strings.Contains(threadComposeStr, "x") {
				t.Errorf("Keystroke 'x' leaked into a compose box! chan: %q, thread: %q", chanComposeStr, threadComposeStr)
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
