package ui

import "testing"

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
		t.Errorf("App entered ModeInsert via E in ViewThreads!")
	}
}
