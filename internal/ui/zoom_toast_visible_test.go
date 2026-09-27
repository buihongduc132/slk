package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// B45: the suppression toast is written to a row that zoom does not draw.
//
// reduceZoom reports suppression with a.uploadToastCmd(zoomSuppressedToast, …,
// toastEager), which resolves to a.statusbar.SetToast(text). But View() sets
// `status := ""` while zoomed (app.go:3384-3386) and never calls
// renderStatusRow, and Compute sets statusHeight = 0. So pressing ctrl+b,
// ctrl+], ctrl+t, `:`, `/` or a workspace digit while zoomed produces no action
// AND no feedback: indistinguishable from a dropped keystroke or a hung app.
//
// Why the existing oracle cannot see it: per AGENTS.md the only observer of a
// toast is statusbarText(a), which is stripANSI(a.statusbar.View(120)) -- the
// statusbar MODEL rendered in isolation, not the frame. SetToast is called
// unconditionally, so that subject cannot vary with whether the row reaches the
// screen. Third instance of this meta-defect in one feature, after B13 (a
// frame-delta recaptured after the event) and B46 (a golden asked to prove a
// hit-test).
//
// These assert on the composed frame, which is the channel the user actually
// observes. In their own file because fullscreen_red_test.go is a pinned gate
// oracle (OT17).

// The load-bearing test: a suppressed key must produce visible feedback in the
// frame while zoomed.
func TestZoom_SuppressionToastIsVisibleInTheZoomedFrame(t *testing.T) {
	a := newTestApp(t, withSize(100, 30), withMessages(testMessageItems(60)...), withRender())
	a.enterZoom()

	// Control: the toast text can reach a frame at all. Set it directly and
	// render UNZOOMED, where the status row is composited. Without this leg the
	// assertion below could be satisfied by text that renders nowhere, and the
	// test would be pinning the wrong thing.
	//
	// Note the control cannot press ctrl+b: unzoomed, ctrl+b is not suppressed
	// at all -- it toggles the sidebar -- so there would be no toast to see.
	// The suppression path exists only while zoomed, which is exactly why its
	// only oracle was a model getter.
	a.clearZoom()
	a.statusbar.SetToast(zoomSuppressedToast)
	if got := frameText(a); !strings.Contains(got, suppressToastProbe) {
		t.Fatalf("unzoomed: a directly-set toast is not visible in the frame either, so this test "+
			"cannot discriminate. Looked for %q.", suppressToastProbe)
	}
	a.statusbar.SetToast("")

	// The subject.
	a.enterZoom()
	_, _ = a.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	if got := frameText(a); !strings.Contains(got, suppressToastProbe) {
		t.Errorf("zoomed: ctrl+b is suppressed but its toast is INVISIBLE (B45). The frame contains no %q.\n"+
			"reduceZoom sets it on a.statusbar, but View() sets status := \"\" while zoomed and never "+
			"composites the status row, so the user gets no action and no feedback.\nlast frame row: %q",
			suppressToastProbe, lastFrameRow(a))
	}
}

// The memo hazard, which is why this fix is not a one-liner. The screen memo is
// keyed on (panels, status, width, height), and while zoomed `status` is always
// "". So a toast appearing changes NO memo input: a fix that writes the toast
// into the composed frame after stackContentStatus would be served the stale
// pre-toast frame on the very next View().
//
// Sequence: render a clean zoomed frame (populating the memo), then suppress a
// key, then render again. The second frame must differ.
func TestZoom_ToastDefeatsTheScreenMemoWhileZoomed(t *testing.T) {
	a := newTestApp(t, withSize(100, 30), withMessages(testMessageItems(60)...), withRender())
	a.enterZoom()
	a.statusbar.SetToast("")

	clean := frameText(a)
	if strings.Contains(clean, suppressToastProbe) {
		t.Fatalf("premise broken: the clean zoomed frame already contains the toast text")
	}
	// Render twice so the memo is definitely populated and hot.
	if again := frameText(a); again != clean {
		t.Fatalf("premise broken: two identical renders differ, so a frame delta cannot prove anything here")
	}

	_, _ = a.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	withToast := frameText(a)

	if withToast == clean {
		t.Errorf("zoomed: the frame is byte-identical before and after a suppressed key (B45). "+
			"The screen memo is keyed on (panels, status, w, h) and status is always \"\" while zoomed, "+
			"so a live toast changes no memo input and the stale frame is served.\nlast row: %q",
			lastFrameRow(a))
	}
}

// Symmetry: the toast must not survive the zoom exit as a painted artifact, and
// the unzoomed path must keep working unchanged.
func TestZoom_ToastRowIsGoneAfterExitingZoom(t *testing.T) {
	a := newTestApp(t, withSize(100, 30), withMessages(testMessageItems(60)...), withRender())
	a.enterZoom()
	_, _ = a.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	_ = frameText(a)

	a.statusbar.SetToast("")
	a.exitZoom()
	got := frameText(a)
	if strings.Contains(got, suppressToastProbe) {
		t.Errorf("after exiting zoom with the toast cleared, the frame still shows %q", suppressToastProbe)
	}
	// And the real status row is back: this is the string the unzoomed status
	// row always carries, so its absence means the exit path broke the row.
	if !strings.Contains(got, "NORMAL") {
		t.Errorf("after exiting zoom the status row is missing (no %q in the frame)", "NORMAL")
	}
}

// suppressToastProbe is a substring of zoomSuppressedToast chosen to avoid the
// em dash, which lipgloss may render or truncate differently at narrow widths.
const suppressToastProbe = "Not available while zoomed"

// frameText renders one real frame and strips ANSI. This is the observation
// channel the user has, as opposed to statusbarText(a), which renders the
// statusbar model in isolation and is the channel B45 hid behind.
func frameText(a *App) string {
	return stripANSI(a.View().Content)
}

func lastFrameRow(a *App) string {
	lines := strings.Split(frameText(a), "\n")
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}
