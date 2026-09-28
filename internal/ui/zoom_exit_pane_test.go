package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/ui/messages"
)

// B49: enterZoom/exitZoom saved and restored the MESSAGES pane
// unconditionally, even when zoom had promoted the THREAD pane.
//
// zoomFrontIsThread (app.go) already reports which pane zoom promotes:
// the thread, exactly when the unzoomed layout would have stacked and
// left the messages pane undrawn. At the widths these tests use that
// answer was MEASURED, not assumed -- the cutover is width 162 with the
// thread focused (161 and below: thread promoted; 162 and above: both
// panes fit, so messages is promoted). 120 is therefore a thread-zoomed
// width and is what stackedApp's stacked cases already use.
//
// Neither test calls zoomFrontIsThread to decide what to expect. The
// precondition for "which pane did zoom promote" is read from the
// layout bands the last View() published, via assertFront -- the same
// bands the mouse routers hit-test against. Asserting against the
// helper the production code itself calls would make the test agree
// with the bug by construction.

// zoomExitApp is the shared fixture: a stacked-width App with 40
// messages, the messages pane focused on index 10, and a render so the
// pane's yOffset is consistent with that selection.
//
// The render is load-bearing. newTestApp's withRender (inside
// normalOpts, via stackedApp) runs BEFORE this selection is applied, at
// which point SetMessages has left the selection at the bottom -- so
// without a second render the pane carries selection 10 alongside the
// bottom's yOffset, a pair production never holds. enterZoom would then
// snapshot that inconsistent pair and the post-exit render would
// re-derive the selection from the offset, which is an artifact of the
// fixture and not of anything under test.
func zoomExitApp(t *testing.T) *App {
	t.Helper()
	a := stackedApp(t, 120, withMessages(testMessageItems(40)...))
	focusMessageAt(t, a, 10)
	_ = a.View()
	return a
}

// zoomExitInbound is a top-level message from another user in the active
// channel, which is the event messages.Model.AppendMessage autoscrolls
// to ("Always scroll to the newest message", messages/model.go). Routed
// through the real NewMessageMsg reducer rather than by calling
// AppendMessage directly, so the fan-out gates (self-send dedup,
// edit-echo, thread-reply routing) are all exercised as in production.
func zoomExitInbound() NewMessageMsg {
	return NewMessageMsg{
		ChannelID: "C1",
		Message: messages.MessageItem{
			TS:        "9999.0",
			UserID:    "U9",
			UserName:  "zoe",
			Text:      "inbound while the thread is zoomed",
			Timestamp: "1:00 PM",
		},
	}
}

// TestZoomExit_ThreadZoomedDoesNotClobberMessagesPane is the B49
// regression. With the THREAD zoomed, a message arriving in the channel
// autoscrolls the messages pane behind the zoom. Leaving zoom must not
// yank that pane back to a viewport the user never asked to return to:
// they never zoomed it, so there is no navigation of theirs to undo.
func TestZoomExit_ThreadZoomedDoesNotClobberMessagesPane(t *testing.T) {
	a := zoomExitApp(t)

	// Open the thread on the selected message and focus it. At width 120
	// that stacks the layout with the thread in front, so the messages
	// pane is already undrawn before zoom is involved at all.
	updateAndRender(t, a, keyCode(tea.KeyEnter))
	if !a.threadVisible || a.focusedPanel != PanelThread {
		t.Fatalf("precondition: Enter left threadVisible=%v focus=%v, want the thread open and focused",
			a.threadVisible, a.focusedPanel)
	}
	assertFront(t, a, false, true)

	preZoomSel := a.messagepane.SelectedIndex()
	preZoomYOff := a.messagepane.YOffset()
	if preZoomSel != 10 {
		t.Fatalf("precondition: selection = %d, want 10 (the fixture's)", preZoomSel)
	}

	updateAndRender(t, a, keyPress('z'))
	if !a.zoomed {
		t.Fatal("precondition: 'z' did not enter zoom")
	}
	// The zoomed frame draws the thread and not the messages pane. This
	// is the observation that establishes "zoom promoted the thread",
	// read from the published bands rather than from zoomFrontIsThread.
	assertFront(t, a, false, true)

	updateAndRender(t, a, zoomExitInbound())
	if !a.zoomed {
		t.Fatal("precondition: the inbound message dropped zoom; it must not")
	}
	if got := len(a.messagepane.Messages()); got != 41 {
		t.Fatalf("precondition: message count = %d, want 41 (40 fixture + 1 inbound)", got)
	}
	autoSel := a.messagepane.SelectedIndex()
	autoYOff := a.messagepane.YOffset()
	// 40 is the last index of 41 messages: the fixture count plus the one
	// inbound, written out rather than read back off the model, so a
	// model that failed to autoscroll cannot define its own expectation.
	if autoSel != 40 {
		t.Fatalf("precondition: the inbound message did not autoscroll the messages pane: selection = %d, want 40", autoSel)
	}
	if autoSel == preZoomSel {
		t.Fatalf("precondition: autoscroll left the selection at its pre-zoom value %d, so this test cannot discriminate", preZoomSel)
	}

	// Exit zoom WITHOUT rendering, so the assertion sees exactly what
	// exitZoom did rather than what a subsequent re-snap made of it.
	_, _ = a.Update(keyCode(tea.KeyEscape))
	if a.zoomed {
		t.Fatal("precondition: esc did not exit zoom")
	}

	if got := a.messagepane.SelectedIndex(); got != autoSel {
		t.Errorf("exiting a THREAD zoom moved the messages pane's selection %d -> %d "+
			"(back to its pre-zoom %d): the user zoomed the thread, never the messages "+
			"pane, so exitZoom must not restore a viewport for it -- doing so silently "+
			"discards the autoscroll to the newest message (B49)",
			autoSel, got, preZoomSel)
	}
	if got := a.messagepane.YOffset(); got != autoYOff {
		t.Errorf("exiting a THREAD zoom moved the messages pane's yOffset %d -> %d (pre-zoom was %d)",
			autoYOff, got, preZoomYOff)
	}

	// And it survives the frame the runtime draws next: the newest
	// message stays selected, which is what the user sees.
	_ = a.View()
	if got := a.messagepane.SelectedIndex(); got != 40 {
		t.Errorf("after the post-exit render the selection = %d, want 40 (the newest message)", got)
	}
}

// TestZoomExit_MessagesZoomedStillRestoresViewport guards the behaviour
// B49's fix must NOT change. exitZoom's contract is deliberate: on the
// user-initiated exit path the pane still holds the content it held at
// enterZoom, "so putting the viewport back is what the user expects".
// That applies to the pane zoom actually promoted -- including undoing
// an autoscroll that happened inside it.
//
// This is the same scenario as the test above with one thing varied,
// the pane zoom promotes, so a fix that simply deleted the restore
// instead of scoping it to that pane fails here.
func TestZoomExit_MessagesZoomedStillRestoresViewport(t *testing.T) {
	a := zoomExitApp(t)

	// No thread: at any width the messages pane is the one drawn, so
	// zoom promotes it.
	if a.threadVisible {
		t.Fatal("precondition: the fixture opened a thread; this case needs none")
	}
	assertFront(t, a, true, false)

	preZoomSel := a.messagepane.SelectedIndex()
	preZoomYOff := a.messagepane.YOffset()
	if preZoomSel != 10 {
		t.Fatalf("precondition: selection = %d, want 10 (the fixture's)", preZoomSel)
	}

	updateAndRender(t, a, keyPress('z'))
	if !a.zoomed {
		t.Fatal("precondition: 'z' did not enter zoom")
	}
	// The zoomed frame draws the messages pane and not the thread.
	assertFront(t, a, true, false)

	updateAndRender(t, a, zoomExitInbound())
	autoSel := a.messagepane.SelectedIndex()
	autoYOff := a.messagepane.YOffset()
	if autoSel != 40 {
		t.Fatalf("precondition: the inbound message did not autoscroll the messages pane: selection = %d, want 40", autoSel)
	}
	if autoYOff == preZoomYOff {
		t.Fatalf("precondition: autoscroll left yOffset at its pre-zoom value %d, so the restore is unobservable", preZoomYOff)
	}

	updateAndRender(t, a, keyCode(tea.KeyEscape))
	if a.zoomed {
		t.Fatal("precondition: esc did not exit zoom")
	}

	if got := a.messagepane.SelectedIndex(); got != preZoomSel {
		t.Errorf("selection after exiting a MESSAGES zoom = %d, want the pre-zoom %d restored: "+
			"exitZoom's restore is a product contract for the pane zoom promoted, and B49's "+
			"fix must scope that restore to the promoted pane, not remove it", got, preZoomSel)
	}
	if got := a.messagepane.YOffset(); got != preZoomYOff {
		t.Errorf("yOffset after exiting a MESSAGES zoom = %d, want the pre-zoom %d restored", got, preZoomYOff)
	}
}

// TestZoomExit_MessagesZoomedUndoesAGWhileZoomed pins the OTHER arm of
// exitZoom's stated contract. Its doc comment claims the restore undoes
// "an autoscroll or a `G` that happened inside the zoomed pane"; the
// test above pins the autoscroll arm only, driving the movement with an
// inbound message. The two arms are not the same event:
//
//   - autoscroll is the CONTENT moving under the user, from
//     messages.Model.AppendMessage;
//   - `G` is the USER navigating deliberately, through
//     a.keys.Bottom -> handleGoToBottom -> messagepane.GoToBottom.
//
// "Undo what the user just asked for" is the more surprising of the two
// claims, and is the one a future author is likelier to talk themselves
// out of, so the comment carrying it alone was the gap. `G` is also NOT
// in zoomSuppresses -- verified in that function, which lists the layout
// and navigation keys plus the workspace numbers -- so unlike ctrl+b it
// genuinely reaches the pane while zoomed, and there is a real movement
// here to undo.
//
// If this test ever fails, the question to settle FIRST is which of the
// two is wrong: the contract or the code. This test asserts the
// documented contract, not a preference.
func TestZoomExit_MessagesZoomedUndoesAGWhileZoomed(t *testing.T) {
	a := zoomExitApp(t)

	if a.threadVisible {
		t.Fatal("precondition: the fixture opened a thread; this case needs none")
	}
	assertFront(t, a, true, false)

	preZoomSel := a.messagepane.SelectedIndex()
	preZoomYOff := a.messagepane.YOffset()
	if preZoomSel != 10 {
		t.Fatalf("precondition: selection = %d, want 10 (the fixture's)", preZoomSel)
	}

	updateAndRender(t, a, keyPress('z'))
	if !a.zoomed {
		t.Fatal("precondition: 'z' did not enter zoom")
	}
	assertFront(t, a, true, false)

	// G through the real reducer chain. The two existing G cases in
	// mode_normal_keys_test.go go through runKeyCases, which calls
	// dispatchModeKey directly and so bypasses reduceZoom entirely --
	// neither of them can observe the suppression rule or the restore.
	updateAndRender(t, a, keyPress('G'))

	// Preconditions, not assertions: if G moved nothing, everything below
	// passes trivially, which is how this test would rot into a tautology.
	gSel := a.messagepane.SelectedIndex()
	if gSel == preZoomSel {
		t.Fatalf("precondition: G did not move the selection off %d -- either it was suppressed "+
			"while zoomed (it is not in zoomSuppresses) or the fixture was already at the bottom", preZoomSel)
	}
	if got := a.messagepane.YOffset(); got == preZoomYOff {
		t.Fatalf("precondition: G left yOffset at its pre-zoom value %d, so the restore is unobservable", preZoomYOff)
	}

	updateAndRender(t, a, keyCode(tea.KeyEscape))
	if a.zoomed {
		t.Fatal("precondition: esc did not exit zoom")
	}

	if got := a.messagepane.SelectedIndex(); got != preZoomSel {
		t.Errorf("selection after exiting a MESSAGES zoom that a G moved = %d, want the pre-zoom %d: "+
			"exitZoom's doc states the restore undoes \"an autoscroll or a G that happened inside the "+
			"zoomed pane\". If the product now means to KEEP a deliberate G, change the doc and this "+
			"test together -- do not let them disagree", got, preZoomSel)
	}
	if got := a.messagepane.YOffset(); got != preZoomYOff {
		t.Errorf("yOffset after exiting a MESSAGES zoom that a G moved = %d, want the pre-zoom %d restored",
			got, preZoomYOff)
	}
}
