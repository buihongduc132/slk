// internal/ui/zoom_sidebar_hidden_tab_test.go
//
// ZOOM WITH THE SIDEBAR ALREADY HIDDEN: the Tab ring must collapse to the
// one pane zoom draws.
//
// This is the sidebar-hidden corner of the invariant 1faa1fb established
// for zoom generally -- focus must never point at a content pane zoom does
// not draw. It was asserted to be "correct by construction" and left
// untested, which is the condition AGENTS.md exists to prevent: every
// existing zoom-focus test (zoom_focus_drawn_pane_test.go,
// zoom_front_pane_test.go, zoom_chord_tab_test.go,
// zoom_insert_drawn_pane_test.go) runs with the sidebar VISIBLE, so the
// `!a.sidebarVisible` branch of FocusNext/FocusPrev (app.go) -- a separate
// early-return branch with its own two predicate calls, not shared with
// the three-pane switch below it -- had no coverage at all while zoomed.
//
// WIDTHS ARE MEASURED, AND THE MEASUREMENT IS THE POINT. The sidebar's
// presence is an INPUT to the branch that decides whether the panes stack
// (panelLayout.Compute subtracts the sidebar's 30 cols + 2 border from the
// area it then tests against minMsgWidth+minThreadW). So hiding it moves
// the boundary, and the constants in zoom_front_pane_test.go do not carry
// over. Measured MsgWidth/ThreadWidth, thread open and focused:
//
//	sidebar SHOWN    129 -> 0/119   161 -> 0/121   162 -> 72/80   (branch at 162)
//	sidebar HIDDEN   129 -> 0/121   130 -> 40/80                  (branch at 130)
//
// That leaves 130..161 as a band where the sidebar ALONE decides the
// branch, and therefore flips which pane zoom promotes. Measured at 150,
// thread open and focused, then `z`:
//
//	sidebar SHOWN   zoomed Msg=  0 Thr=148   zoomFrontIsThread=true
//	sidebar HIDDEN  zoomed Msg=148 Thr=  0   zoomFrontIsThread=false
//
// so the rows below use 120 (stacked either way), 200 (side by side either
// way) and 150 (the band where hiding the sidebar flips the answer). A
// test that only used the inherited 120/200 pair would never exercise the
// flip.
//
// OBSERVES THE FRAME, NOT JUST THE FIELD. A field holding focus while the
// screen shows nothing is the defect class this work has been chasing, so
// every row asserts the drawn bands (assertFront) and then types through
// the real Update chain and requires the text on screen
// (assertTypedVisible, zoom_focus_drawn_pane_test.go). focusedPanel is
// checked too, but never alone.
package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// firstSideBySideWidthNoSidebar is firstSideBySideWidth's sidebar-hidden
// twin: the first width at which Compute has room for both content panes
// once the 30-col sidebar and its 2-col border are gone.
// firstSideBySideWidthNoSidebar-1 stacks.
//
// ASSERTED, not merely declared, by
// TestZoomSidebarHidden_BoundaryWidthMovesWhenTheSidebarGoes. It has to
// be: Go does not flag an unused package-level constant, so a geometry
// number nothing references is a comment maintained by hand -- and this
// file's header leans on it to argue the sidebar-shown constants do not
// carry over. If minMsgWidth, minThreadW, the sidebar's 30 cols or the
// border widths change, that test fails instead of this number silently
// becoming a lie. Its sibling firstSideBySideWidth is asserted the same
// way, by TestZoomFrontPane_G15_BoundaryWidth.
const firstSideBySideWidthNoSidebar = 130

// sidebarDecidesWidth sits inside the 130..161 band, where the panes stack
// with the sidebar shown and fit side by side without it. It is the only
// width at which hiding the sidebar changes which pane zoom promotes.
const sidebarDecidesWidth = 150

// hideSidebar presses ctrl+b through the real Update chain and asserts the
// sidebar actually went away. Used for the "hide, THEN zoom" order; the
// reverse order is suppressed (see
// TestZoomSidebarHidden_CtrlBIsSuppressedWhileZoomed).
func hideSidebar(t *testing.T, a *App) {
	t.Helper()
	if !a.sidebarVisible {
		t.Fatal("precondition: the sidebar is already hidden")
	}
	updateAndRender(t, a, keyMod('b', tea.ModCtrl|tea.ModAlt))
	if a.sidebarVisible {
		t.Fatal("ctrl+b did not hide the sidebar")
	}
}

// sidebarHiddenRing is one configuration of the sidebar-hidden zoomed
// state and the single pane the ring must collapse onto.
type sidebarHiddenRing struct {
	name string
	w    int
	// openThread opens a thread (and focuses it) BEFORE zooming.
	openThread bool
	// wantFocus is the pane the ring must hold, and wantThreadDrawn says
	// which pane the zoomed frame draws. They must agree -- that agreement
	// IS the invariant.
	wantFocus        Panel
	wantThreadDrawn  bool
	wantChannelDrawn bool
}

// sidebarHiddenRings enumerates every sidebar-hidden zoomed configuration:
// three widths x thread-absent / thread-present.
//
// Thread ABSENT is a ring of 1 trivially -- there is only one content pane
// to begin with. It is included because it is the control: if a mutation
// breaks it too, the mutation is not specific to the zoom rule.
//
// Thread PRESENT is the discriminating half. At 120 the thread is the pane
// drawn alone, so the ring must hold the THREAD; at 150 and 200 the panes
// fit side by side unzoomed, so zoom promotes MESSAGES (G15) and the ring
// must hold messages with the thread dropped from it.
func sidebarHiddenRings() []sidebarHiddenRing {
	return []sidebarHiddenRing{
		{"stacked, no thread", stackedWidth, false, PanelMessages, false, true},
		{"stacked, thread open: the thread is drawn alone", stackedWidth, true, PanelThread, true, false},
		{"sidebar decides the branch, no thread", sidebarDecidesWidth, false, PanelMessages, false, true},
		{"sidebar decides the branch, thread open: hiding it promotes messages",
			sidebarDecidesWidth, true, PanelMessages, false, true},
		{"side by side, no thread", sideBySideWidth, false, PanelMessages, false, true},
		{"side by side, thread open: zoom promotes messages", sideBySideWidth, true, PanelMessages, false, true},
	}
}

// zoomedSidebarHiddenApp reaches the state under test by the reachable
// order: hide the sidebar, optionally open a thread, THEN zoom.
func zoomedSidebarHiddenApp(t *testing.T, tc sidebarHiddenRing) *App {
	t.Helper()
	a := stackedApp(t, tc.w)
	hideSidebar(t, a)
	if tc.openThread {
		updateAndRender(t, a, keyCode(tea.KeyEnter))
		if !a.threadVisible {
			t.Fatal("precondition: Enter did not open the thread")
		}
	}
	updateAndRender(t, a, keyPress('z'))
	if !a.zoomed {
		t.Fatal("precondition: 'z' did not enter zoom")
	}
	if a.sidebarVisible {
		t.Fatal("precondition: zooming brought the sidebar back")
	}
	assertFront(t, a, tc.wantChannelDrawn, tc.wantThreadDrawn)
	return a
}

// TestZoomSidebarHidden_TabRingCollapsesToTheDrawnPane is the claim under
// test: with the sidebar hidden and zoom engaged, the Tab ring has length
// 1 and the pane it holds is the pane the frame draws.
//
// SIX presses, not one. A ring of length 1 and a ring that merely STARTS
// on the right pane are indistinguishable under a single Tab: both leave
// focus in the right place after the first press. Walking past a full
// cycle in both directions is what separates them -- if the ring had
// length 2, some press in this walk lands on the pane with a zero-width
// band, and both the focus assertion and the frame assertion fail on that
// press rather than at the end.
func TestZoomSidebarHidden_TabRingCollapsesToTheDrawnPane(t *testing.T) {
	for _, dir := range []struct {
		name string
		key  tea.KeyMsg
	}{
		{"tab", keyCode(tea.KeyTab)},
		{"shift+tab", keyMod(tea.KeyTab, tea.ModShift)},
	} {
		t.Run(dir.name, func(t *testing.T) {
			for _, tc := range sidebarHiddenRings() {
				t.Run(tc.name, func(t *testing.T) {
					a := zoomedSidebarHiddenApp(t, tc)
					if a.focusedPanel != tc.wantFocus {
						t.Fatalf("before any %s: focus=%v, want %v -- the zoom transition "+
							"itself left focus on the wrong pane", dir.name, a.focusedPanel, tc.wantFocus)
					}

					// A full cycle plus one, each press checked. Every press
					// must be a no-op: that is what "collapsed to one
					// pane" means operationally.
					for press := 1; press <= 6; press++ {
						updateAndRender(t, a, dir.key)
						if !a.zoomed {
							t.Fatalf("%s #%d dropped zoom; %s must move focus only (G15)",
								dir.name, press, dir.name)
						}
						if a.sidebarVisible {
							t.Fatalf("%s #%d brought the sidebar back", dir.name, press)
						}
						if a.focusedPanel != tc.wantFocus {
							t.Fatalf("%s #%d moved focus to %v, want %v held. With the sidebar "+
								"hidden and zoom drawing one pane, the ring must have length 1; "+
								"landing anywhere else points focus at a pane with a zero-width "+
								"band (drawn: channel=%v thread=%v).",
								dir.name, press, a.focusedPanel, tc.wantFocus,
								tc.wantChannelDrawn, tc.wantThreadDrawn)
						}
						// The frame, not just the field: the pane that holds
						// focus must still be the pane with a band.
						assertFront(t, a, tc.wantChannelDrawn, tc.wantThreadDrawn)
					}

					// Focus never moved, so one typing check at the end proves
					// the held pane is the one on screen. Typing enters insert
					// mode, which is why it comes after the whole walk.
					typeToken(t, a, insertToken)
					assertTypedVisible(t, a, insertToken, tc.wantThreadDrawn)
				})
			}
		})
	}
}

// TestZoomSidebarHidden_CtrlBIsSuppressedWhileZoomed answers the second
// order, "zoom THEN hide", and the answer is that it does not exist.
//
// ctrl+b is in zoomSuppresses (reducer_zoom.go), so while zoomed in normal
// mode reduceZoom claims the key, raises the toast and returns before
// mode_normal.go's ToggleSidebar arm is reached. That arm is the ONLY
// production caller of ToggleSidebar (no command-mode verb, no mouse
// target -- the mouse routers gate on a.sidebarVisible), so the whole
// "zoom, then hide the sidebar" order is unreachable by user input.
//
// This is pinned rather than worked around. Forcing the state with a
// direct ToggleSidebar() call would be a test of a state no user can
// reach; if the suppression is ever lifted, THIS test fails and whoever
// lifts it has to come here and write the order-(b) rows deliberately.
func TestZoomSidebarHidden_CtrlBIsSuppressedWhileZoomed(t *testing.T) {
	for _, w := range []int{stackedWidth, sidebarDecidesWidth, sideBySideWidth} {
		t.Run(fmt.Sprintf("w=%d", w), func(t *testing.T) {
			a := stackedApp(t, w)
			updateAndRender(t, a, keyCode(tea.KeyEnter))
			updateAndRender(t, a, keyPress('z'))
			if !a.zoomed || !a.sidebarVisible {
				t.Fatalf("precondition: zoomed=%v sidebarVisible=%v", a.zoomed, a.sidebarVisible)
			}

			updateAndRender(t, a, keyMod('b', tea.ModCtrl|tea.ModAlt))
			if !a.sidebarVisible {
				t.Errorf("ctrl+b hid the sidebar while zoomed. It is in zoomSuppresses, so "+
					"the keypress must be swallowed; if that changed deliberately, the "+
					"zoom-then-hide order is now reachable and needs its own ring rows "+
					"alongside %s.", "TestZoomSidebarHidden_TabRingCollapsesToTheDrawnPane")
			}
			if !a.zoomed {
				t.Error("ctrl+b dropped zoom while zoomed")
			}
			if got := stripANSI(a.View().Content); !strings.Contains(got, "zoom") {
				t.Errorf("suppressed ctrl+b raised no visible zoom toast; frame:\n%s", got)
			}
		})
	}
}

// TestZoomSidebarHidden_HidingTheSidebarFlipsThePromotedPane is the
// measurement that makes sidebarDecidesWidth worth having, and the reason
// this file cannot reuse zoom_front_pane_test.go's constants.
//
// At 150 cols with the thread open and focused, the sidebar's presence
// decides panelLayout.Compute's branch, so it decides which pane zoom
// promotes -- and therefore how long the Tab ring is. Shown: the panes
// stack, the thread is drawn alone, zoom promotes the thread, and the ring
// is thread <-> sidebar (length 2). Hidden: the panes fit side by side,
// zoom promotes messages, and the ring collapses to messages alone.
//
// A change that made zoomFrontIsThread ignore sidebarVisible would pass
// every row of the main test above at 120 and 200 and fail here.
func TestZoomSidebarHidden_HidingTheSidebarFlipsThePromotedPane(t *testing.T) {
	for _, tc := range []struct {
		name           string
		hide           bool
		wantZoomThread bool
		wantRing       []Panel
	}{
		{"sidebar shown: stacked, zoom promotes the thread", false, true,
			[]Panel{PanelSidebar, PanelThread, PanelSidebar, PanelThread}},
		{"sidebar hidden: side by side, zoom promotes messages", true, false,
			[]Panel{PanelMessages, PanelMessages, PanelMessages, PanelMessages}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := stackedApp(t, sidebarDecidesWidth)
			if tc.hide {
				hideSidebar(t, a)
			}
			updateAndRender(t, a, keyCode(tea.KeyEnter))
			if a.focusedPanel != PanelThread {
				t.Fatalf("precondition: focus=%v, want PanelThread", a.focusedPanel)
			}
			// The unzoomed layout is what zoomFrontIsThread probes, so
			// assert it before zooming: this is where the sidebar's 32 cols
			// change the answer.
			assertFront(t, a, tc.hide, true)

			updateAndRender(t, a, keyPress('z'))
			if got := a.zoomFrontIsThread(); got != tc.wantZoomThread {
				t.Fatalf("zoomFrontIsThread() = %v, want %v at w=%d with sidebar hidden=%v",
					got, tc.wantZoomThread, sidebarDecidesWidth, tc.hide)
			}
			assertFront(t, a, !tc.wantZoomThread, tc.wantZoomThread)

			for i, want := range tc.wantRing {
				updateAndRender(t, a, keyCode(tea.KeyTab))
				if a.focusedPanel != want {
					t.Fatalf("tab #%d: focus=%v, want %v (ring %v)", i+1, a.focusedPanel, want, tc.wantRing)
				}
				// Whichever pane the ring is on, the CONTENT pane drawn must
				// never change under Tab (G15) -- including when the ring
				// passes through the sidebar, which zoom does not draw.
				assertFront(t, a, !tc.wantZoomThread, tc.wantZoomThread)
			}
		})
	}
}

// TestZoomSidebarHidden_BoundaryWidthMovesWhenTheSidebarGoes makes
// firstSideBySideWidthNoSidebar load-bearing.
//
// The number was previously declared and never referenced. Go does not
// flag an unused package-level constant, so nothing would ever have caught
// it going stale -- and this file's header cites it to argue that
// zoom_front_pane_test.go's constants do not carry over. A geometry claim
// maintained by hand is what AGENTS.md says to replace with a check.
//
// Two rows, one column apart, are the whole assertion: at
// firstSideBySideWidthNoSidebar-1 the panes stack with the sidebar hidden,
// and at firstSideBySideWidthNoSidebar they fit side by side. Each row
// then re-runs at the SAME width with the sidebar SHOWN, where both must
// stack -- which is what makes the constant's value matter rather than
// just its existence. 129 and 130 both stack with the sidebar shown, so a
// wrong constant cannot be rescued by the sidebar-shown half.
//
// The zoom consequence is asserted too, because the boundary is only
// interesting here for what it does to zoom: the stacked side promotes the
// THREAD (it is the pane drawn alone) and the side-by-side side promotes
// MESSAGES (G15).
func TestZoomSidebarHidden_BoundaryWidthMovesWhenTheSidebarGoes(t *testing.T) {
	for _, tc := range []struct {
		name string
		w    int
		// With the sidebar HIDDEN: do both panes get a band unzoomed, and
		// which pane does zoom then promote?
		wantBothDrawnHidden bool
	}{
		{"one below the boundary stacks", firstSideBySideWidthNoSidebar - 1, false},
		{"the boundary fits both", firstSideBySideWidthNoSidebar, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := stackedApp(t, tc.w)
			hideSidebar(t, a)
			updateAndRender(t, a, keyCode(tea.KeyEnter))
			if a.focusedPanel != PanelThread {
				t.Fatalf("precondition: focus=%v, want PanelThread", a.focusedPanel)
			}

			// Unzoomed, sidebar hidden: this IS the branch the constant names.
			assertFront(t, a, tc.wantBothDrawnHidden, true)

			// zoomFrontIsThread probes exactly this layout, so the boundary
			// decides which pane `z` promotes.
			wantZoomThread := !tc.wantBothDrawnHidden
			if got := a.zoomFrontIsThread(); got != wantZoomThread {
				t.Errorf("sidebar hidden at w=%d: zoomFrontIsThread() = %v, want %v",
					tc.w, got, wantZoomThread)
			}
			updateAndRender(t, a, keyPress('z'))
			assertFront(t, a, !wantZoomThread, wantZoomThread)

			// The other half of "the boundary MOVED": at this same width the
			// sidebar's 32 cols push the layout back to stacked, both sides
			// of the boundary. Without this, a constant set anywhere in
			// 121..161 would still satisfy the rows above.
			b := stackedApp(t, tc.w)
			updateAndRender(t, b, keyCode(tea.KeyEnter))
			if !b.sidebarVisible {
				t.Fatal("precondition: the sidebar-shown half started hidden")
			}
			assertFront(t, b, false, true)
			if !b.zoomFrontIsThread() {
				t.Errorf("sidebar shown at w=%d: zoomFrontIsThread() = false, want true -- "+
					"with the sidebar shown this width must still stack, which is what makes "+
					"%d the SIDEBAR-HIDDEN boundary rather than the shared one",
					tc.w, firstSideBySideWidthNoSidebar)
			}
		})
	}
}
