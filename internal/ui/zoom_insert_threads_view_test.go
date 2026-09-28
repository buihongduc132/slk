// internal/ui/zoom_insert_threads_view_test.go
//
// The Threads-view arm of the `i` router must respect drawn-ness.
//
// THE DEFECT. mode_normal.go's `i` arm is an || chain whose second clause
// was `a.view == ViewThreads && a.threadVisible`. That consults neither
// focus nor drawn-ness, so it fired even when zoom had promoted a pane
// that is not the thread -- overriding normalizeZoomFocus, which had just
// moved focus OFF the undrawn thread one keystroke earlier. Measured at
// 200x30 in ViewThreads: after `z` the frame is MsgWidth=198
// ThreadWidth=0 and focus is PanelMessages; pressing `i` put focus back
// on PanelThread and the typed text into threadCompose, a box with no
// width in any frame.
//
// WHY THAT IS WORSE THAN ONE LOST KEYSTROKE. reducer_zoom.go enumerates
// eight production sites that route a keypress or a paste on
// `focusedPanel == PanelThread` (the reaction picker and reaction-nav
// arms, three insert-mode send/upload arms, two paste arms, the external
// editor target). The invariant commit 1faa1fb established is that the
// invalid STATE is unreachable, so all eight stay correct without knowing
// zoom exists. This clause re-created that state, so `i` re-armed every
// one of them at an undrawn pane. The fix reuses 1faa1fb's own predicate,
// threadFocusable(), rather than adding a fourth notion of drawn-ness.
//
// THE ROWS ARE THE FOUR MEASURED STATES, and they matter as a set: three
// of them must keep routing to threadCompose, so a "fix" that simply
// stops trusting the Threads view, or that hardcodes either answer while
// zoomed, fails one of them.
//
// EVERY ROW TABS TO THE SIDEBAR FIRST. That is what makes the rows
// discriminating: with focus on a content pane the chain's FIRST clause
// (`focusedPanel == PanelThread`) would decide the route, and the rows
// would not be testing the Threads-view clause at all. It is asserted,
// not assumed.
//
// a.view IS ASSERTED. withThreadsView seeds summaries but does not set
// the view; without withView(ViewThreads) every row would silently
// exercise ViewChannels and the defect would look absent.
//
// OBSERVES, NEVER RECOMPUTES. Assertions read published state only -- the
// frame via stripANSI(a.View().Content), the hit-test bands via
// assertFront, compose.Value() / threadCompose.Value(), and
// focusedPanel. Nothing here calls threadFocusable() or
// threadDrawnAlone(), which are the functions under test.
package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/cache"
)

// threadsInsertToken is typed into whichever compose the `i` arm picks.
// Absent from the fixtures and the chrome, and free of 'z' so no row can
// be confused with the zoom toggle.
const threadsInsertToken = "badger"

// threadsNoComposeToast is the exact text `i` raises in the state this file's
// defect row reaches: a thread IS open (Enter was pressed) but zoom at a
// side-by-side width promoted messages, so the reply box is drawn on no frame.
// The remedy is therefore "exit zoom", not "open a thread" -- the two
// conditions have different remedies and separate strings.
//
// Deliberately a literal copy rather than a reference to the production
// constant: if this referenced the same symbol, a production reword would
// change both sides at once and the assertion would keep passing while
// asserting nothing. Verified by rewording production to "Nope" and watching
// this row fail.
const threadsNoComposeToast = "Zoom hides the reply box — press z or Esc"

func threadsViewSummaries() []cache.ThreadSummary {
	return []cache.ThreadSummary{
		{ChannelID: "C1", ThreadTS: "1.0", ParentTS: "1.0", ParentText: "first", ReplyCount: 1, ChannelName: "general"},
		{ChannelID: "C1", ThreadTS: "2.0", ParentTS: "2.0", ParentText: "second", ReplyCount: 2, ChannelName: "general"},
	}
}

// TestZoomInsert_ThreadsViewRoutesToTheDrawnPane drives real keys through
// the reducer chain: Enter opens the selected thread, `z` zooms where the
// row asks for it, Tab parks focus on the sidebar, `i` picks a compose.
func TestZoomInsert_ThreadsViewRoutesToTheDrawnPane(t *testing.T) {
	for _, tc := range []struct {
		name string
		w    int
		zoom bool
		// channel / thread: which panes the frame draws, asserted from
		// the bands before anything is typed. At a stacked width the
		// threads-list pane is dropped entirely when the thread is in
		// front, so the channel band is drawn only side by side.
		channel bool
		thread  bool
		// wantThreadCompose: the typed text must land in threadCompose.
		wantThreadCompose bool
		// wantInFrame: assert the text is visible on screen. False only
		// where NO compose box is drawn at all -- see the row's comment.
		wantInFrame bool
		// wantInsertMode: does `i` enter insert mode at all? True
		// everywhere a compose box is drawn. False in the one state where
		// none is, because `i` there now raises a toast and stays in
		// ModeNormal rather than focusing a box on no frame. This was a
		// bare precondition asserting ModeInsert unconditionally until the
		// toast landed; it is per-row now so the exception is visible in
		// the table instead of being buried in a t.Fatalf.
		wantInsertMode bool
	}{
		{
			name:              "unzoomed side by side: thread drawn, types into the thread compose",
			w:                 sideBySideWidth,
			channel:           true,
			thread:            true,
			wantThreadCompose: true,
			wantInFrame:       true,
			wantInsertMode:    true,
		},
		{
			// THE DEFECT. Side by side unzoomed, so zoom promotes the
			// threads LIST and the thread pane has zero width. Before the
			// fix this row put the token in threadCompose, off screen,
			// and left focus on PanelThread.
			//
			// wantInFrame is false, and that is NOT this test blessing an
			// invisible keystroke. ViewThreads renders no compose box at
			// all -- renderThreadsViewPanel has neither a typing row nor
			// a compose, measured true unzoomed and with no thread open --
			// so in this state there is no drawn compose for `i` to reach.
			// What `i` should ideally do when the promoted pane is the
			// threads list is an open product question. This row pins only
			// what is determined: the keystroke must not be routed to the
			// undrawn thread compose, and focus must not be left on the
			// pane the frame does not draw. Revisit the in-frame
			// expectation when that question is settled.
			name:              "zoomed side by side: thread NOT drawn, must not type into the thread compose",
			w:                 sideBySideWidth,
			zoom:              true,
			channel:           true,
			thread:            false,
			wantThreadCompose: false,
			wantInFrame:       false,
			wantInsertMode:    false,
		},
		{
			// The control that forbids "never trust the Threads view":
			// zoom promotes the THREAD here, so its compose is the one on
			// screen and the token must land there.
			name:              "zoomed stacked: thread drawn, types into the thread compose",
			w:                 stackedWidth,
			zoom:              true,
			channel:           false,
			thread:            true,
			wantThreadCompose: true,
			wantInFrame:       true,
			wantInsertMode:    true,
		},
		{
			name:              "unzoomed stacked: thread drawn, types into the thread compose",
			w:                 stackedWidth,
			channel:           false,
			thread:            true,
			wantThreadCompose: true,
			wantInFrame:       true,
			wantInsertMode:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp(t, append(normalOpts(),
				withWindowSize(tc.w, 30),
				withView(ViewThreads),
				withThreadsView(threadsViewSummaries()),
			)...)
			if a.view != ViewThreads {
				t.Fatalf("precondition: view=%v, want ViewThreads -- withThreadsView seeds "+
					"summaries but does not set the view, so without withView this row "+
					"would be testing ViewChannels", a.view)
			}
			// The threads list lives in the messages region; Enter only
			// opens the selected thread from there.
			a.focusedPanel = PanelMessages
			updateAndRender(t, a, keyCode(tea.KeyEnter))
			if !a.threadVisible {
				t.Fatalf("precondition: Enter did not open a thread (threadVisible=%v)", a.threadVisible)
			}

			if tc.zoom {
				updateAndRender(t, a, keyPress('z'))
				if !a.zoomed {
					t.Fatal("precondition: 'z' did not enter zoom")
				}
			}

			updateAndRender(t, a, keyCode(tea.KeyTab))
			if a.focusedPanel != PanelSidebar {
				t.Fatalf("precondition: focus=%v, want PanelSidebar -- with focus on a content "+
					"pane the `i` arm's first clause decides the route and this row would "+
					"not be testing the Threads-view clause", a.focusedPanel)
			}

			// The frame draws exactly the panes the row names.
			assertFront(t, a, tc.channel, tc.thread)

			if before := stripANSI(a.View().Content); strings.Contains(before, threadsInsertToken) {
				t.Fatalf("precondition: %q is already in the frame before typing", threadsInsertToken)
			}

			updateAndRender(t, a, keyPress('i'))
			if got := a.mode == ModeInsert; got != tc.wantInsertMode {
				t.Fatalf("'i' left mode=%v (insert=%v), want insert=%v", a.mode, got, tc.wantInsertMode)
			}

			// The token is only typed where a compose box is drawn. In the
			// toast row `i` stays in ModeNormal, and every rune of
			// "badger" is a live normal-mode binding there -- 'd' delete,
			// 'e' edit, 'r' react among them -- so typing it would fire
			// real side effects and the assertions below would be measuring
			// those, not the insert route.
			if tc.wantInsertMode {
				for _, r := range threadsInsertToken {
					updateAndRender(t, a, keyPress(r))
				}
			} else {
				// Nothing was typed, so "the compose is empty" proves
				// nothing here. What this row pins instead: the keystroke
				// was refused VISIBLY, and focus was not parked on the pane
				// the frame does not draw.
				if plain := stripANSI(a.View().Content); !strings.Contains(plain, threadsNoComposeToast) {
					t.Errorf("`i` was swallowed with no feedback: toast %q is absent from the frame.\n"+
						"ViewThreads draws no compose box in this state, so `i` must say so "+
						"rather than focusing a box on no frame.\n%s", threadsNoComposeToast, plain)
				}
				if a.compose.Value() != "" || a.threadCompose.Value() != "" {
					t.Errorf("no key was typed after `i`, yet a compose holds text: channel=%q thread=%q",
						a.compose.Value(), a.threadCompose.Value())
				}
			}

			gotChannel, gotThread := a.compose.Value(), a.threadCompose.Value()
			if tc.wantThreadCompose {
				if gotThread != threadsInsertToken || gotChannel != "" {
					t.Errorf("typed %q went to the wrong compose: channel=%q thread=%q.\n"+
						"The frame draws the thread pane, so insert mode must focus the "+
						"thread compose.", threadsInsertToken, gotChannel, gotThread)
				}
			} else {
				if gotThread != "" {
					t.Errorf("typed %q landed in the thread compose (channel=%q thread=%q), but the "+
						"frame does not draw the thread pane -- ThreadWidth is 0, so the box "+
						"is on no frame and the keystroke is silently lost.",
						threadsInsertToken, gotChannel, gotThread)
				}
			}

			// Focus must never be parked on a pane the frame does not draw,
			// in EITHER branch. This is checked outside the typing split
			// because it is the invariant 1faa1fb established and it holds
			// whether or not `i` was accepted -- eight production sites
			// route on focusedPanel == PanelThread, and none of them care
			// how focus got there.
			if !tc.thread && a.focusedPanel == PanelThread {
				t.Errorf("focus=%v after 'i', but the frame does not draw the thread pane. "+
					"Eight production sites route on focusedPanel == PanelThread; "+
					"leaving focus here re-arms every one of them at an undrawn pane.",
					a.focusedPanel)
			}

			if tc.wantInFrame {
				if plain := stripANSI(a.View().Content); !strings.Contains(plain, threadsInsertToken) {
					t.Errorf("typed %q is absent from the rendered frame -- it landed in a "+
						"compose box the user cannot see.\n%s", threadsInsertToken, plain)
				}
			}
		})
	}
}
