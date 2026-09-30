package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/gammons/slk/internal/ui/workspace"
)

// ---------------------------------------------------------------------
// RED layer for the fullscreen-pane plan (slk-fullscreen-emoji-fuzzy,
// items fs-* ; gotchas G1-G3, G5-G8, G12-G17).
//
// Harness rules this file obeys (AGENTS.md helper table + G3):
//   - EVERY esc/precedence/toast row rides the REAL reducer chain via
//     updateAndRender / a.Update. runKeyCases is never used for esc —
//     it calls dispatchModeKey directly and bypasses the chain, so an
//     esc row there can pass while production esc is broken.
//   - 'z' toggle rows also ride the chain so the binding, not just the
//     handler, is under test (fs-binding).
//   - No new App symbols are referenced: zoom state is observed
//     through FRAMES (structure: status row / sidebar presence) so
//     this file compiles against today's tree and fails as
//     assertions.
//
// RED anchors, per test, are the assertions that fail TODAY: 'z' is
// unbound (verified: no WithKeys("z") in keys.go), so every "frame
// changed on z" / "status row absent" / "suppression toast" assertion
// fires.
// ---------------------------------------------------------------------

// zoomGoldenOpts is the deterministic base fixture: golden channels +
// messages, active C1, 120x24 (stacked-friendly). Same shape
// goldenFixtureOpts uses, one size smaller so the thread pane and the
// channel pane stack.
func zoomGoldenOpts() []testOpt {
	return []testOpt{
		withSize(120, 24),
		withChannels(goldenChannels()...),
		withMessages(goldenMessages()...),
		withActiveChannel("C1"),
	}
}

// zoomedStatusToken is a string only the status row renders; its
// absence from a frame is the observable for "status row hidden".
// The statusbar always renders the current mode string, and NORMAL is
// the mode of every fixture here.
const zoomedStatusToken = "NORMAL"

// sidebarOnlyTokens are strings only the sidebar renders (the same
// set TestGolden_NoSidebarActuallyHidesIt uses; they cannot be
// satisfied by the messages pane or statusbar).
func sidebarOnlyTokens() []string {
	return []string{"▾ Channels", "▾ DMs", "● bob", "○ carol", "# muted-noise"}
}

// zoomedScrolledApp builds the fs-restore-eq fixture: more messages
// than fit, scrolled mid-history, with an active text selection — the
// state a bare frame-equality assertion is meaningless without.
//
// Positioning goes through the SELECTION cursor, not ScrollDown:
// View()'s snap step re-clamps a raw yOffset around the selected
// message, so a ScrollDown-set offset does not survive the first
// render under this (chain-driven) harness. SelectByIndex(80) + a
// render leaves the viewport settled strictly inside the history.
func zoomedScrolledApp(t *testing.T) *App {
	t.Helper()
	msgs := testMessageItems(200)
	a := newTestApp(t,
		withSize(120, 24),
		withChannels(goldenChannels()...),
		withMessages(msgs...),
		withActiveChannel("C1"),
		withRender(),
	)
	focusMessages(t, a)
	a.messagepane.SelectByIndex(80)
	_ = a.View()
	off := a.messagepane.YOffset()
	if off <= 0 {
		t.Fatalf("precondition: fixture did not scroll (yOffset = %d); mid-history state missing", off)
	}
	// A held drag across three visible rows (same shape as
	// goldenDragApp; no release, so no clipboard write).
	x := a.layout.sidebarEnd + 11
	_, _ = a.Update(tea.MouseClickMsg{X: x, Y: 7, Button: tea.MouseLeft})
	_, _ = a.Update(tea.MouseMotionMsg{X: x + 25, Y: 9, Button: tea.MouseLeft})
	_, _ = a.Update(motionFlushTickMsg{})
	_ = a.View()
	if !a.messagepane.HasSelection() {
		t.Fatal("precondition: drag produced no selection; the restore assertion would be vacuous")
	}
	if got := a.messagepane.YOffset(); got != off {
		t.Fatalf("precondition: the drag scrolled the pane (yOffset %d -> %d); fixture must be stable", off, got)
	}
	return a
}

// mustEnterZoom presses 'z' through the chain and fails the test
// unless the rendered frame actually left the unzoomed layout. This
// is the shared RED anchor: today 'z' is unbound and the frame does
// not change.
func mustEnterZoom(t *testing.T, a *App, frameBefore string) string {
	t.Helper()
	updateAndRender(t, a, keyPress('z'))
	zoomed := a.View().Content
	if zoomed == frameBefore {
		t.Fatalf("'z' did not change the frame — fullscreen zoom is not wired (binding + handler + layout)")
	}
	return zoomed
}

// assertZoomedFrame pins the structural properties of a zoomed frame:
// status row hidden, sidebar hidden, frame height unchanged, and the
// zoomed pane's own content still present.
func assertZoomedFrame(t *testing.T, a *App, zoomed string, contentToken string) {
	t.Helper()
	plain := stripANSI(zoomed)
	if strings.Contains(plain, zoomedStatusToken) {
		t.Errorf("zoomed frame still renders the status row (%q present); the zoomed pane must span the full height", zoomedStatusToken)
	}
	for _, tok := range sidebarOnlyTokens() {
		if strings.Contains(plain, tok) {
			t.Errorf("zoomed frame still renders the sidebar row %q", tok)
		}
	}
	if got := strings.Count(zoomed, "\n") + 1; got != a.height {
		t.Errorf("zoomed frame has %d lines, want the terminal height %d", got, a.height)
	}
	if contentToken != "" && !strings.Contains(plain, contentToken) {
		t.Errorf("zoomed frame lost the zoomed pane's content (%q missing):\n%s", contentToken, plain)
	}
}

func TestFullscreen_ZTogglesAndZAgainRestoresByteEqualFrame(t *testing.T) {
	a := zoomedScrolledApp(t)
	frameBefore := a.View().Content // second render; View is idempotent

	zoomed := mustEnterZoom(t, a, frameBefore)
	assertZoomedFrame(t, a, zoomed, "msg-")

	updateAndRender(t, a, keyPress('z'))
	after := a.View().Content
	if after != frameBefore {
		t.Errorf("second 'z' did not restore the pre-zoom frame byte-for-byte (fs-restore-eq): %s",
			firstLineDiff(stripANSI(frameBefore), stripANSI(after)))
	}
	// The restore includes pane state, not just layout: the selection
	// cursor survives the round trip, and the viewport stays inside
	// the mid-history region (the snap step re-derives the exact
	// offset from the cursor, so cursor equality is the durable
	// assertion; the offset check guards a scroll-to-top regression).
	if got := a.messagepane.SelectedIndex(); got != 80 {
		t.Errorf("selected index after zoom round-trip = %d, want 80 (scroll position must be restored)", got)
	}
	if got := a.messagepane.YOffset(); got <= 0 {
		t.Errorf("yOffset after zoom round-trip = %d, want the pane still scrolled mid-history", got)
	}
	if !a.messagepane.HasSelection() {
		t.Error("text selection did not survive the zoom round-trip")
	}
}

func TestFullscreen_EscExitsZoom(t *testing.T) {
	// esc rows MUST ride the chain (G3): dispatchModeKey would bypass
	// the reducer where the esc peel order lives.
	a := zoomedScrolledApp(t)
	frameBefore := a.View().Content

	_ = mustEnterZoom(t, a, frameBefore)
	updateAndRender(t, a, keyCode(tea.KeyEscape))

	after := a.View().Content
	if after != frameBefore {
		t.Errorf("esc did not exit zoom back to the pre-zoom frame: %s",
			firstLineDiff(stripANSI(frameBefore), stripANSI(after)))
	}
}

func TestFullscreen_ZoomSpansFullHeightAndHidesRailSidebarStatus(t *testing.T) {
	a := newGoldenApp(t, zoomGoldenOpts()...)
	frameBefore := a.View().Content
	zoomed := mustEnterZoom(t, a, frameBefore)

	plain := stripANSI(zoomed)
	lines := strings.Split(plain, "\n")
	last := lines[len(lines)-1]

	// The zoomed pane owns the LAST terminal row. Today that row is
	// the status bar (it renders the mode string), so this fires.
	if strings.Contains(last, zoomedStatusToken) {
		t.Errorf("the last terminal row is still the status row (%q); the zoomed pane must use every row", zoomedStatusToken)
	}
	if strings.TrimSpace(last) == "" {
		t.Errorf("the last terminal row is blank; the zoomed pane must draw content on it: %q", last)
	}
	assertZoomedFrame(t, a, zoomed, "wrapping behaviour")
}

// fs-esc-pecking-order case 1: modal modes own esc — zoom persists.
func TestFullscreen_EscPeckingOrder_ModalOwnsEsc_ZoomPersists(t *testing.T) {
	a := newGoldenApp(t, zoomGoldenOpts()...)
	frameBefore := a.View().Content
	_ = mustEnterZoom(t, a, frameBefore)

	updateAndRender(t, a, keyPress('?')) // help modal over the zoomed pane
	if a.mode != ModeHelp {
		t.Fatalf("precondition: '?' did not open help (mode = %v)", a.mode)
	}
	updateAndRender(t, a, keyCode(tea.KeyEscape))
	if a.mode != ModeNormal {
		t.Fatalf("esc in help left mode = %v, want ModeNormal (the modal must consume esc)", a.mode)
	}
	// Zoom must still be active: the frame still hides the status row
	// and sidebar. If the esc had fallen through to zoom-exit, the
	// status row would be back.
	plain := stripANSI(a.View().Content)
	if strings.Contains(plain, zoomedStatusToken) {
		t.Error("esc in a modal exited zoom too; modal modes own esc and zoom must persist (G1 peel order)")
	}
	for _, tok := range sidebarOnlyTokens() {
		if strings.Contains(plain, tok) {
			t.Errorf("esc in a modal un-hid the sidebar (%q); zoom must persist", tok)
		}
	}
}

// fs-esc-pecking-order case 2: insert protective arms own esc — the
// upload toast fires and zoom persists (mode stays insert).
func TestFullscreen_EscPeckingOrder_UploadArmBeatsZoomExit(t *testing.T) {
	a := zoomedScrolledApp(t)
	frameBefore := a.View().Content
	_ = mustEnterZoom(t, a, frameBefore)

	updateAndRender(t, a, keyPress('i'))
	if a.mode != ModeInsert {
		t.Fatalf("precondition: 'i' did not enter insert mode (mode = %v)", a.mode)
	}
	a.compose.SetUploading(true)

	// Not updateAndRender: this row needs the returned cmd. The upload
	// arm toasts via uploadToastCmd, whose setter lives INSIDE a
	// tea.Batch (app.go:4131), so the status bar stays empty until the
	// batch's first member runs. firstBatchCmd runs exactly that member
	// and skips the 2s tick.
	_, cmd := a.Update(keyCode(tea.KeyEscape))
	if cmd == nil {
		t.Fatal("esc while uploading returned no cmd: the esc was consumed " +
			"by something that does not toast -- zoom-exit claiming it " +
			"before the insert upload arm is exactly the G1 defect " +
			"fs-esc-pecking-order pins (protective arms outrank zoom-exit)")
	}
	firstBatchCmd(t, cmd)
	_ = a.View()

	if a.mode != ModeInsert {
		t.Errorf("esc while uploading left mode = %v, want ModeInsert (the protective arm must consume esc before zoom-exit)", a.mode)
	}
	if got := statusbarText(a); !strings.Contains(got, "Upload in progress") {
		t.Errorf("status bar = %q, want the upload toast — protective arms outrank zoom-exit (G1)", got)
	}
}

// fs-esc-pecking-order case 3: zoom-exit beats picker-close and
// insert-exit — ONE esc exits zoom only, leaving insert mode (and an
// open compose picker) intact.
func TestFullscreen_EscPeckingOrder_ZoomExitBeatsInsertExit(t *testing.T) {
	a := zoomedScrolledApp(t)
	frameBefore := a.View().Content
	_ = mustEnterZoom(t, a, frameBefore)

	updateAndRender(t, a, keyPress('i'))
	if a.mode != ModeInsert {
		t.Fatalf("precondition: 'i' did not enter insert mode")
	}
	updateAndRender(t, a, keyCode(tea.KeyEscape))

	// RED today: esc (with zoom a no-op) exits insert mode, so the
	// mode assertion below fails against the current tree — which is
	// exactly the peel order being pinned: while zoomed, the esc is
	// consumed by zoom-exit and insert survives.
	if a.mode != ModeInsert {
		t.Errorf("esc while zoomed left mode = %v, want ModeInsert — zoom-exit must consume esc BEFORE insert-exit (fs-esc-pecking-order)", a.mode)
	}
}

// fs-esc-pecking-order case 3b, with an open compose emoji picker:
// zoom-exit outranks picker-close, so the picker survives the esc
// that exits zoom.
func TestFullscreen_EscPeckingOrder_ZoomExitBeatsPickerClose(t *testing.T) {
	a := newTestApp(t,
		withSize(120, 24),
		withChannels(goldenChannels()...),
		withMessages(testMessageItems(20)...),
		withActiveChannel("C1"),
		withRender(),
	)
	focusMessages(t, a)
	frameBefore := a.View().Content
	_ = mustEnterZoom(t, a, frameBefore)

	updateAndRender(t, a, keyPress('i'))
	// Open the compose emoji picker the way production does: ":sm".
	updateAndRender(t, a, keyPress(':'))
	updateAndRender(t, a, keyPress('s'))
	updateAndRender(t, a, keyPress('m'))
	if !a.compose.IsEmojiActive() {
		t.Fatal("precondition: ':sm' did not open the emoji picker")
	}

	updateAndRender(t, a, keyCode(tea.KeyEscape))

	// RED today: esc closes the picker outright, so the assertion
	// below fails on the current tree. The pinned order: the esc
	// exits zoom first, and the picker stays open for a second esc.
	if !a.compose.IsEmojiActive() {
		t.Error("esc closed the compose emoji picker while zoomed; zoom-exit must consume the esc before picker-close (fs-esc-pecking-order)")
	}
	if a.mode != ModeInsert {
		t.Errorf("mode after esc = %v, want ModeInsert (insert survives zoom-exit)", a.mode)
	}
}

// G1's unchanged-when-not-zoomed guard: the ctrl+w chord's esc-cancel
// behavior must not change once the peel order lands. Chain-driven.
func TestFullscreen_ChordEscCancelUnchangedWhenNotZoomed(t *testing.T) {
	a := zoomedScrolledApp(t)
	updateAndRender(t, a, keyPress('i'))
	updateAndRender(t, a, keyCode(tea.KeyEscape)) // back to normal, not zoomed

	updateAndRender(t, a, keyMod('w', tea.ModCtrl|tea.ModAlt))
	if !a.pendingWinCmd {
		t.Fatal("precondition: ctrl+w did not arm the window chord")
	}
	updateAndRender(t, a, keyCode(tea.KeyEscape))
	if a.pendingWinCmd {
		t.Error("esc did not cancel the armed chord (behavior must be unchanged when not zoomed)")
	}
}

// fs-insert-z-types: the binding lives in the normal-mode map only.
// Rows are guards: they pin that whatever GREEN adds must not steal
// 'z' from the compose. Both pass today (z unbound) and must KEEP
// passing.
func TestFullscreen_InsertZTypesIntoCompose(t *testing.T) {
	typeIntoCompose := func(zoomFirst bool) func(t *testing.T) {
		return func(t *testing.T) {
			a := zoomedScrolledApp(t)
			updateAndRender(t, a, keyPress('i'))
			if a.mode != ModeInsert {
				t.Fatalf("precondition: 'i' did not enter insert mode")
			}
			if zoomFirst {
				// Enter zoom BEFORE insert; z must still type.
				updateAndRender(t, a, keyCode(tea.KeyEscape)) // insert -> normal
				_ = a.View().Content
				updateAndRender(t, a, keyPress('z')) // zoom (no-op today; RED anchors live in the other tests)
				updateAndRender(t, a, keyPress('i'))
			}
			updateAndRender(t, a, keyPress('z'))
			if got := a.compose.Value(); got != "z" {
				t.Errorf("compose value = %q, want %q — 'z' in insert mode must type, not toggle zoom (fs-insert-z-types)", got, "z")
			}
		}
	}
	t.Run("unzoomed", typeIntoCompose(false))
	t.Run("zoomed", typeIntoCompose(true))
}

// fs-scope + G5: zoom auto-clears when the zoomed pane's world goes
// away. Each case uses the same probe: after the clearing event, 'z'
// must ENTER zoom (frame changes away from the idle post-event
// frame). If the zoom leaked across the event, 'z' would instead EXIT
// zoom — restoring the idle frame — and the probe fails.
func TestFullscreen_ZoomAutoClears(t *testing.T) {
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
		frameIdle := a.View().Content
		_ = mustEnterZoom(t, a, frameIdle)

		updateAndRender(t, a, keyPress('q'))
		if a.threadVisible {
			t.Fatal("precondition: 'q' did not close the thread")
		}
		frameIdle = a.View().Content
		updateAndRender(t, a, keyPress('z'))
		if a.View().Content == frameIdle {
			t.Error("'z' after 'q' did not enter zoom; zoom survived the thread close (G5 stale zoom) — or exited a leaked zoom back to the idle frame")
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
		frameIdle := a.View().Content
		_ = mustEnterZoom(t, a, frameIdle)

		updateAndRender(t, a, WorkspaceSwitchedMsg{TeamID: "T2", TeamName: "beta", Channels: nil})
		frameIdle = a.View().Content
		updateAndRender(t, a, keyPress('z'))
		if a.View().Content == frameIdle {
			t.Error("'z' after a workspace switch did not enter zoom; zoom survived the switch (G5 — app.go resets stackFront there, zoom must clear too)")
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
		frameIdle := a.View().Content
		_ = mustEnterZoom(t, a, frameIdle)

		updateAndRender(t, a, ChannelSelectedMsg{ID: "C2", Name: "engineering", Type: "channel"})
		frameIdle = a.View().Content
		updateAndRender(t, a, keyPress('z'))
		if a.View().Content == frameIdle {
			t.Error("'z' after a channel jump did not enter zoom; zoom survived the jump (G5 stale zoom)")
		}
	})
}

// fs-zoom-invariant: Enter while zoomed on messages flips the zoom to
// the thread — the thread must become visible IN the zoomed frame,
// never open invisibly behind it.
func TestFullscreen_EnterWhileZoomedFlipsZoomToThread(t *testing.T) {
	a := newTestApp(t,
		withWindowSize(120, 24),
		withChannels(goldenChannels()...),
		withMessages(testMessageItems(30)...),
		withActiveChannel("C1"),
	)
	focusMessages(t, a)
	frameBefore := a.View().Content
	_ = mustEnterZoom(t, a, frameBefore)

	updateAndRender(t, a, keyCode(tea.KeyEnter))
	if !a.threadVisible {
		t.Fatal("precondition: Enter did not open the thread")
	}
	plain := stripANSI(a.View().Content)
	// The thread is now the zoomed pane: its breadcrumb is on screen
	// AND the frame is still zoomed (no status row).
	if !strings.Contains(plain, "Thread") {
		t.Errorf("the thread opened while zoomed is not visible in the zoomed frame:\n%s", plain)
	}
	if strings.Contains(plain, zoomedStatusToken) {
		t.Error("Enter un-zoomed the pane instead of flipping zoom to the thread; the frame must stay zoomed")
	}
}

// fs-scope: z zooms the FRONT pane whatever it is. With the thread
// open and focused, the zoomed frame shows the thread, not the
// channel.
func TestFullscreen_ZoomZoomsTheFrontPane(t *testing.T) {
	a := newTestApp(t,
		withWindowSize(120, 24),
		withChannels(goldenChannels()...),
		withMessages(testMessageItems(30)...),
		withActiveChannel("C1"),
	)
	focusMessages(t, a)
	updateAndRender(t, a, keyCode(tea.KeyEnter))
	if !a.threadVisible || a.focusedPanel != PanelThread {
		t.Fatalf("precondition: Enter did not focus the thread (visible=%v focus=%v)", a.threadVisible, a.focusedPanel)
	}
	frameBefore := a.View().Content

	updateAndRender(t, a, keyPress('z'))
	plain := stripANSI(a.View().Content)
	if a.View().Content == frameBefore {
		t.Fatal("'z' with the thread in front did not change the frame — front-pane zoom is not wired")
	}
	if !strings.Contains(plain, "Thread") {
		t.Errorf("zooming with the thread in front shows no thread content; z must zoom the FRONT pane (fs-scope):\n%s", plain)
	}
	if strings.Contains(plain, zoomedStatusToken) {
		t.Error("thread zoom still renders the status row")
	}
}

// G15 guard: Tab while zoomed must not flip WHICH pane is zoomed
// (threadFront derives from stackFront only under zoom). Guard row —
// passes today (z is a no-op and both panes render at 200 cols), and
// must keep passing under GREEN.
func TestFullscreen_TabWhileZoomedDoesNotFlipTheZoomedPane(t *testing.T) {
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

	updateAndRender(t, a, keyCode(tea.KeyTab))
	plain := stripANSI(a.View().Content)
	if !strings.Contains(plain, "wrapping behaviour") {
		t.Errorf("Tab while zoomed flipped the zoomed pane away from the messages content (G15):\n%s", plain)
	}
}

// fs-supp-set: the ONE suppression rule, key surface. While zoomed,
// each key is suppressed with a visible toast.
//
// HARNESS NOTE — the repo has TWO toast helpers with DIFFERENT cmd
// contracts, and picking the wrong one breaks this test:
//
//	toastWithClear(a, text, d)   reducer_io.go:88   sets the toast
//	                                                EAGERLY, returns a
//	                                                bare clear tick.
//	a.uploadToastCmd(text, dur)  app.go:4131        returns
//	                                                tea.Batch(setter,
//	                                                tick) -- the toast
//	                                                is NOT applied until
//	                                                the batch runs.
//
// The rows below read statusbarText straight after a.Update, so the
// suppression path MUST use toastWithClear. If it uses uploadToastCmd
// the toast is still sitting unapplied inside an unexecuted batch and
// every row here fails with an empty-looking status bar. Do not "fix"
// that by making uploadToastCmd eager: 14 production call sites and
// firstBatchCmd depend on its batched shape (one of them builds it
// INSIDE a tea.Batch at app.go:3586, where an eager setter fires before
// the runtime ever executes the cmd).
func TestFullscreen_SuppressedKeysToastWhileZoomed(t *testing.T) {
	suppressed := []struct {
		name string
		key  tea.KeyMsg
		// state asserts the key's NORMAL effect did not happen.
		state func(t *testing.T, a *App, cmd tea.Cmd)
	}{
		{
			name: "ctrl+b does not toggle the sidebar",
			key:  keyMod('b', tea.ModCtrl|tea.ModAlt),
			state: func(t *testing.T, a *App, _ tea.Cmd) {
				if !a.sidebarVisible {
					t.Error("ctrl+b toggled the sidebar while zoomed; it must be suppressed")
				}
			},
		},
		{
			name: "ctrl+w chord never arms",
			key:  keyMod('w', tea.ModCtrl|tea.ModAlt),
			state: func(t *testing.T, a *App, _ tea.Cmd) {
				if a.pendingWinCmd {
					t.Error("ctrl+w armed the window chord while zoomed; suppression must happen at the prefix arm (fs-supp-set)")
				}
			},
		},
		{
			name: "ctrl+a does not open Activity",
			key:  keyMod('a', tea.ModCtrl|tea.ModAlt),
			state: func(t *testing.T, a *App, _ tea.Cmd) {
				if a.view == ViewActivity {
					t.Error("ctrl+a opened the Activity view while zoomed; it must be suppressed")
				}
			},
		},
		{
			name: "ctrl+t does not open the finder",
			key:  keyMod('t', tea.ModCtrl|tea.ModAlt),
			state: func(t *testing.T, a *App, _ tea.Cmd) {
				if a.channelFinder.IsVisible() {
					t.Error("ctrl+t opened the channel finder while zoomed; it must be suppressed")
				}
			},
		},
		{
			name: "':' does not enter command mode",
			key:  keyPress(':'),
			state: func(t *testing.T, a *App, _ tea.Cmd) {
				if a.mode == ModeCommand {
					t.Error("':' entered command mode while zoomed; it must be suppressed")
				}
			},
		},
		{
			name: "'/' does not enter search mode",
			key:  keyPress('/'),
			state: func(t *testing.T, a *App, _ tea.Cmd) {
				if a.mode == ModeSearch {
					t.Error("'/' entered search mode while zoomed; it must be suppressed")
				}
			},
		},
	}

	for _, tc := range suppressed {
		t.Run(tc.name, func(t *testing.T) {
			a := zoomedScrolledApp(t)
			frameBefore := a.View().Content
			_ = mustEnterZoom(t, a, frameBefore)

			_, cmd := a.Update(tc.key)
			_ = a.View()

			// RED anchors: the effect did not happen AND a toast
			// naming zoom is visible (G6: toasts stay visible while
			// zoomed).
			tc.state(t, a, cmd)
			if got := statusbarText(a); !strings.Contains(strings.ToLower(got), "zoom") {
				t.Errorf("status bar = %q, want a suppression toast mentioning zoom (fs-supp-set / G6)", got)
			}
		})
	}
}

// fs-supp-set, the workspace-number surface: 1-9 must not switch
// workspaces while zoomed, and must toast instead. Observable today:
// '2' returns the workspace-switch cmd; the row pins cmd == nil.
func TestFullscreen_SuppressedWorkspaceNumberWhileZoomed(t *testing.T) {
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
	// The switcher RECORDS instead of just answering: "was the switch
	// scheduled" cannot be read off the returned cmd, because the
	// suppression path returns a non-nil cmd of its own (the toast's
	// clear tick). Asserting cmd == nil would make the row
	// unsatisfiable -- it would demand both a toast and no cmd. The
	// observable that actually separates suppressed from not-suppressed
	// is whether the workspace service was ever invoked.
	var switches []string
	a.setWorkspaceSwitcherForTest(func(teamID string) tea.Msg {
		switches = append(switches, teamID)
		return WorkspaceSwitchedMsg{TeamID: teamID, TeamName: "beta", Channels: nil}
	})
	focusMessages(t, a)
	frameBefore := a.View().Content
	_ = mustEnterZoom(t, a, frameBefore)

	_, cmd := a.Update(keyPress('2'))
	// Run the cmd the key produced: a workspace switch does its work
	// inside the cmd, so the recorder only sees it once the cmd is
	// executed. A suppressed key's cmd is just a toast tick, which
	// records nothing. Bounded, because an unsuppressed switch may
	// return a tick that would otherwise block the test goroutine.
	if cmd != nil {
		_, _ = cmdMsgWithin(t, cmd, time.Second)
	}
	_ = a.View()

	if len(switches) != 0 {
		t.Errorf("'2' while zoomed switched workspace (switcher called with %v); 1-9 must be suppressed with a toast", switches)
	}
	if a.activeTeamID != "T1" {
		t.Errorf("active team = %q, want it unchanged at T1 while zoomed", a.activeTeamID)
	}
	if got := statusbarText(a); !strings.Contains(strings.ToLower(got), "zoom") {
		t.Errorf("status bar = %q, want a suppression toast mentioning zoom", got)
	}
}

// fs-zoom-cache-keys (G16): render, zoom, render — the zoomed frame
// must differ (no stale non-zoomed composite from the screen memo);
// a second zoomed render must be stable; and exit must restore the
// pre-zoom frame byte-for-byte (no stale zoomed composite either).
func TestFullscreen_PanelCacheAndScreenMemoKeysIncludeFullscreen(t *testing.T) {
	a := zoomedScrolledApp(t)
	frameBefore := a.View().Content

	zoomed := mustEnterZoom(t, a, frameBefore)
	again := a.View().Content
	if again != zoomed {
		t.Error("consecutive zoomed renders differ; the screen memo is unstable across the zoom transition (G16)")
	}

	updateAndRender(t, a, keyPress('z'))
	if after := a.View().Content; after != frameBefore {
		t.Errorf("exiting zoom did not restore the pre-zoom frame byte-for-byte (stale zoomed composite, G16): %s",
			firstLineDiff(stripANSI(frameBefore), stripANSI(after)))
	}
}

// fs-zoom-cache-keys, resize half: a WindowSizeMsg while zoomed keeps
// the zoom (the pane re-lays out at the new size; the status row must
// not come back).
func TestFullscreen_ResizeWhileZoomedKeepsZoom(t *testing.T) {
	a := zoomedScrolledApp(t)
	frameBefore := a.View().Content
	_ = mustEnterZoom(t, a, frameBefore)

	updateAndRender(t, a, tea.WindowSizeMsg{Width: 100, Height: 20})
	plain := stripANSI(a.View().Content)
	if strings.Contains(plain, zoomedStatusToken) {
		t.Error("resize while zoomed dropped the zoom (status row is back); the pane must stay zoomed at the new size")
	}
	for _, tok := range sidebarOnlyTokens() {
		if strings.Contains(plain, tok) {
			t.Errorf("resize while zoomed un-hid the sidebar (%q)", tok)
		}
	}
	if got := strings.Count(a.View().Content, "\n") + 1; got != a.height {
		t.Errorf("zoomed frame after resize has %d lines, want the new terminal height %d", got, a.height)
	}
}

// fs-binding, behavioral half: the help overlay (driven by
// help.FromKeyMap over the same key map production uses) must list
// the fullscreen binding. Today no binding mentions zoom, so this is
// RED; the GREEN agent names the binding whatever they like as long
// as its help text mentions zoom.
func TestFullscreen_HelpListsTheZoomBinding(t *testing.T) {
	a := newGoldenApp(t, zoomGoldenOpts()...)
	updateAndRender(t, a, keyPress('?'))
	if a.mode != ModeHelp {
		t.Fatal("precondition: '?' did not open help")
	}
	plain := stripANSI(a.View().Content)
	if !strings.Contains(strings.ToLower(plain), "zoom") {
		t.Errorf("the help overlay does not mention a zoom/fullscreen binding:\n%s", plain)
	}
}

// fs-golden: golden frames for the zoomed layouts. Per the task
// brief, the golden FILES are blessed by the GREEN agent via the
// package-local -update flag; today compareGolden fails on the
// missing file (acceptable RED). The behavioral properties in
// assertZoomedFrame stay hard-RED independent of blessing.
func TestGolden_FullscreenFrames(t *testing.T) {
	t.Run("thread zoomed", func(t *testing.T) {
		a := newGoldenApp(t,
			withSize(120, 24),
			withChannels(goldenChannels()...),
			withMessages(goldenMessages()...),
			withActiveChannel("C1"),
		)
		focusMessages(t, a)
		updateAndRender(t, a, keyCode(tea.KeyEnter))
		if !a.threadVisible {
			t.Fatal("precondition: Enter did not open the thread")
		}
		frameBefore := a.View().Content
		zoomed := mustEnterZoom(t, a, frameBefore)
		assertZoomedFrame(t, a, zoomed, "Thread")
		compareGolden(t, "fullscreen_thread_zoomed", zoomed)
	})

	t.Run("messages zoomed with thread open behind", func(t *testing.T) {
		// 200 cols: both panes side-by-side unzoomed, so the zoomed
		// frame's "thread hidden" is a real difference, not an
		// artifact of stacking.
		a := newGoldenApp(t,
			withSize(200, 24),
			withChannels(goldenChannels()...),
			withMessages(goldenMessages()...),
			withActiveChannel("C1"),
		)
		focusMessages(t, a)
		updateAndRender(t, a, keyCode(tea.KeyEnter))
		if !a.threadVisible {
			t.Fatal("precondition: Enter did not open the thread")
		}
		frameBefore := a.View().Content
		zoomed := mustEnterZoom(t, a, frameBefore)
		assertZoomedFrame(t, a, zoomed, "wrapping behaviour")
		if strings.Contains(stripANSI(zoomed), "Thread from") {
			t.Error("the thread behind the zoomed messages pane is still drawn; only the front pane renders")
		}
		compareGolden(t, "fullscreen_messages_zoomed", zoomed)
	})

	t.Run("zoom exit restores", func(t *testing.T) {
		a := newGoldenApp(t, zoomGoldenOpts()...)
		frameBefore := a.View().Content
		_ = mustEnterZoom(t, a, frameBefore)
		updateAndRender(t, a, keyPress('z'))
		after := a.View().Content
		if after != frameBefore {
			t.Errorf("exit frame differs from the pre-zoom frame: %s",
				firstLineDiff(stripANSI(frameBefore), stripANSI(after)))
		}
		compareGolden(t, "fullscreen_zoom_exit_restored", after)
	})
}

// Keep ansi imported even if future edits drop its other uses (the
// stripANSI helper used throughout lives in golden_test.go).
var _ = ansi.Strip
