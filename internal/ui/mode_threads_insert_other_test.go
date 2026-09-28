package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestInsertMode_ThreadsView_Edit(t *testing.T) {
	opts := append(normalOpts(), withWindowSize(200, 30))
	a := newTestApp(t, opts...)
	msgs := normalMessages()
	msgs[0].UserID = "Uself"
	a.messagepane.SetMessages(msgs)
	a.messagepane.SelectByIndex(0)

	a.view = ViewThreads
	a.focusedPanel = PanelMessages
	a.currentUserID = "Uself"

	updateAndRender(t, a, keyPress('E'))
	if a.mode == ModeInsert {
		t.Errorf("App entered ModeInsert via E in ViewThreads: beginEditOfSelected " +
			"resolves through messagepane, which remembers the last viewed channel, " +
			"so this would edit an OFF-SCREEN message on an invisible compose")
	}

	// Mode alone is not enough: `E` doing nothing at all would also leave
	// ModeNormal and pass. The refusal has to be visible, and it has to name
	// the right remedy. No Enter was pressed here, so no thread is open and
	// the remedy is to open one -- not to exit zoom, which is not even on.
	screen := ansi.Strip(a.View().Content)
	if !strings.Contains(screen, threadsNoThreadOpenToast) {
		t.Errorf("`E` was refused with no feedback: toast %q absent from the frame",
			threadsNoThreadOpenToast)
	}
	if strings.Contains(screen, threadsZoomHidesReplyToast) {
		t.Errorf("wrong remedy on screen: zoom is off and no thread is open, so %q "+
			"is the wrong advice", threadsZoomHidesReplyToast)
	}

	// And the guard must not swallow `E` in the view where editing works.
	b := newTestApp(t, append(normalOpts(), withWindowSize(200, 30))...)
	bmsgs := normalMessages()
	bmsgs[0].UserID = "Uself"
	b.messagepane.SetMessages(bmsgs)
	b.messagepane.SelectByIndex(0)
	b.focusedPanel = PanelMessages
	b.currentUserID = "Uself"
	if b.view != ViewChannels {
		t.Fatalf("precondition: view=%v, want ViewChannels", b.view)
	}
	updateAndRender(t, b, keyPress('E'))
	if b.mode != ModeInsert {
		t.Errorf("`E` in ViewChannels did not enter insert mode (mode=%v) -- the "+
			"Threads-view guard must not reach this view", b.mode)
	}
}
