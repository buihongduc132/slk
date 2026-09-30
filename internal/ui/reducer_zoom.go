// internal/ui/reducer_zoom.go
//
// Fullscreen pane zoom (plan items fs-*).
//
// Owns three things:
//
//   - the `z` toggle (normal mode only, so `z` still types in insert —
//     fs-insert-z-types),
//   - the suppression rule: while zoomed, keys that would change the
//     layout or navigate away are swallowed with a toast (fs-supp-set),
//   - zoom-exit on esc, at its place in the esc peel order.
//
// ESC PEEL ORDER (fs-esc-pecking-order). The reducer chain runs BEFORE
// dispatchModeKey, so a reducer that claims esc unconditionally would
// outrank every per-mode Escape arm. The order the plan pins is:
//
//	modal modes own esc (zoom persists)
//	  > insert protective arms (upload-toast, edit-cancel)
//	    > zoom-exit
//	      > picker-close / insert-exit / chord-cancel / reaction-nav /
//	        thread-close
//
// This reducer therefore claims esc ONLY in ModeNormal. That yields the
// top two tiers for free: in a modal mode or in insert mode the reducer
// passes, and the mode handler runs. Zoom-exit's precedence over
// insert-mode's own picker-close / insert-exit arms is expressed where
// those arms live, in handleInsertMode — its zoom arm sits after the
// upload/edit guards and before picker-close.
//
// Because every claim is gated on a.zoomed, behaviour when NOT zoomed
// is unchanged (the guard windows_chord_test.go's esc-cancel relies on).
package ui

import (
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// zoomSuppressedToast is the status-bar text shown when a suppressed
// key is pressed while zoomed. Must mention "zoom" — the suppression
// rows assert on that substring.
const zoomSuppressedToast = "Not available while zoomed — esc or z to exit zoom"

// zoomToastDuration is how long a suppression toast stays up.
const zoomToastDuration = 2 * time.Second

// threadFocusable / messagesFocusable report whether that content pane
// may hold focus right now. Unzoomed both are true (visibility alone
// decides, as it always did); while zoomed only the pane zoom PROMOTES
// may, because the other one has a zero-width band and appears in no
// frame.
//
// WHY THE RULE LIVES HERE AND NOT AT THE ROUTING SITES. Eight production
// sites route a keypress or a paste on `focusedPanel == PanelThread`:
// mode_normal.go's `i`, reaction-picker and reaction-nav arms;
// mode_insert.go's send/upload arms (three); reducer_io.go's paste arms
// (two); editor.go's external-editor target. Every one of them is aimed
// at an undrawn pane while zoom promotes messages with focus on the
// thread, and `i` is merely the one whose loss is visible in the same
// frame. Teaching all eight to double-check would be eight copies of one
// rule -- the defect AGENTS.md names as this codebase's most-repeated --
// and each new thread-targeted key would have to remember the ninth. So
// the fix makes the invalid STATE unreachable instead: if focus never
// points at a pane zoom does not draw, every existing consumer stays
// correct without knowing zoom exists.
//
// The oracle is zoomFrontIsThread, untouched. Its deliberate probe of the
// UNZOOMED layout (G15) is what these predicates depend on: it is the one
// function that already knows which pane zoom promotes, and it answers
// without consulting a.zoomed, so asking it here cannot feed back into
// itself. Nothing in this file resolves through layoutThreadFront.
func (a *App) threadFocusable() bool {
	if !a.threadVisible {
		return false
	}
	return !a.zoomed || a.zoomFrontIsThread()
}

func (a *App) messagesFocusable() bool {
	return !a.zoomed || !a.zoomFrontIsThread()
}

// normalizeZoomFocus moves focus off a content pane that zoom does not
// draw, and records the move so exitZoom can undo it.
//
// Called from the two edges that can create the invalid state, because
// measurement says one is not enough:
//
//   - enterZoom -- `z` with the thread focused at a side-by-side width.
//     Measured at 200x30: frame MsgWidth=198 ThreadWidth=0 with focus
//     still on PanelThread.
//   - openThreadPanel -- a thread opened WHILE already zoomed. This path
//     never runs enterZoom, so a fix there alone leaves it broken;
//     measured identically at 200x30 via `z` then Enter.
//
// The third edge, Tab, is handled differently and deliberately: FocusNext
// and FocusPrev drop the unfocusable pane from the ring rather than
// landing on it and being corrected here. Correcting after the fact would
// have made Tab a no-op on that step (focus set to the thread, then moved
// straight back), swallowing a keypress to fix a swallowed keypress.
//
// Only PanelThread is corrected. The mirror case -- focus on PanelMessages
// while zoom promotes the thread -- is unreachable: zoomFrontIsThread
// resolves through threadInFront, which returns false whenever focus is on
// PanelMessages, so zoom cannot be promoting the thread while messages
// holds focus. messagesFocusable still exists because FocusNext/FocusPrev
// ask it from the SIDEBAR, where threadInFront falls through to stackFront
// and the answer can be false.
//
// Focus on the SIDEBAR is left alone even though zoom does not draw it
// either. That is pre-existing, pinned behaviour (Tab reaches the sidebar
// while zoomed -- zoom_insert_drawn_pane_test.go depends on it) and it is
// not a keystroke sink: the sidebar is not a compose target, so the `i`
// arm falls through to threadDrawnAlone and routes to whichever pane IS
// drawn. Widening this to the sidebar would change zoom's Tab behaviour
// for no defect.
func (a *App) normalizeZoomFocus() {
	if a.focusedPanel == PanelThread && !a.threadFocusable() {
		a.focusedPanel = PanelMessages
		a.zoomSavedThreadFocus = true
	}
}

// enterZoom snapshots the front pane's viewport so exitZoom can put it
// back, then zooms.
//
// The guard is a guard, not a comment (B52). This used to read "Callers
// must already have checked !a.zoomed", which both siblings express as
// actual code -- exitZoom and clearZoom each open with `if !a.zoomed`.
// A second enterZoom would overwrite zoomSavedYOffset with the
// already-scrolled offset and destroy the restore point permanently,
// silently, the first time a second entry path was added. AGENTS.md:
// when you find a comment standing in for a check, replace it with the
// check.
//
// The snapshot is taken ONLY when zoom promotes the messages pane
// (B49). zoomFrontIsThread reports which pane zoom promotes; when it is
// the thread, the messages pane is not the pane being zoomed and not
// even drawn, so there is nothing of the user's to put back -- and
// snapshotting it anyway is actively harmful. The pane keeps receiving
// live channel traffic behind the zoom, and messages.Model.AppendMessage
// autoscrolls to the newest message unconditionally; a restore on the
// way out would silently discard that autoscroll, reverting a viewport
// the user never navigated.
//
// zoomSavedMsgViewport pairs the save with the restore so the two halves
// cannot drift: exitZoom restores exactly when enterZoom saved.
// zoomSavedThreadFocus is the same pattern for focus -- one rule, one
// implementation, rather than a second mechanism.
func (a *App) enterZoom() {
	if a.zoomed {
		return
	}
	a.zoomSavedMsgViewport = !a.zoomFrontIsThread()
	if a.zoomSavedMsgViewport {
		a.zoomSavedYOffset = a.messagepane.YOffset()
		a.zoomSavedSelectedIndex = a.messagepane.SelectedIndex()
	}
	a.zoomed = true
	// Cleared before the normaliser can set it, so a previous zoom's
	// answer can never be read as this one's. Both save flags are written
	// unconditionally on every entry for that reason.
	a.zoomSavedThreadFocus = false
	// AFTER a.zoomed = true (the predicates are no-ops while unzoomed) and
	// after the viewport snapshot, whose zoomFrontIsThread call must see
	// the focus the user actually had.
	a.normalizeZoomFocus()
	// Zoom hides the status row, which is where the "ctrl+alt+w …" / "g …"
	// hints live. Leaving a chord armed behind a hidden hint means the
	// user's next keystroke is silently eaten as a window command.
	a.disarmPendingChords()
	a.invalidateZoomCaches()
}

// exitZoom leaves zoom and restores the pane state captured by
// enterZoom. This is the USER-INITIATED exit path -- the `z` toggle, esc
// here, and insert mode's esc arm -- where the pane still holds the same
// content it held at enterZoom, so putting the viewport back is what the
// user expects. That includes undoing an autoscroll or a `G` that
// happened inside the zoomed pane: intended, and pinned by
// TestZoomExit_MessagesZoomedStillRestoresViewport.
//
// It applies to the pane zoom actually PROMOTED, though, and only that
// one. The restore is therefore gated on zoomSavedMsgViewport rather
// than run unconditionally (B49) -- see enterZoom for why a thread zoom
// must leave the messages pane alone.
//
// The auto-clear events do NOT come through here; they use clearZoom.
// Restoring a saved offset onto a pane whose content has been replaced is
// a bug, not a courtesy.
func (a *App) exitZoom() {
	if !a.zoomed {
		return
	}
	a.zoomed = false
	a.disarmPendingChords()
	a.invalidateZoomCaches()
	if a.zoomSavedMsgViewport {
		a.messagepane.SetViewport(a.zoomSavedYOffset, a.zoomSavedSelectedIndex)
	}
	// Hand back the focus zoom took (normalizeZoomFocus), so a z-in /
	// z-out round trip leaves the user where they were. GUARDED, unlike
	// the viewport restore: the move being undone was mechanical, not
	// something the user asked for, so it is only undone while focus is
	// still where the normaliser parked it. If the user Tabbed away
	// meanwhile, that is a deliberate choice and outranks this.
	if a.zoomSavedThreadFocus && a.focusedPanel == PanelMessages && a.threadVisible {
		a.focusedPanel = PanelThread
	}
	a.zoomSavedThreadFocus = false
}

// clearZoom drops zoom WITHOUT restoring the saved viewport, for the
// auto-clear events: the zoomed pane's content is gone or replaced
// (thread closed, channel jumped, workspace switched), so the offset and
// selected index captured by enterZoom no longer refer to anything. On a
// channel jump the old channel's offset would be stamped onto the new
// channel's pane, and a shorter new channel makes the saved selected
// index out of range.
//
// The saved FOCUS is dropped here rather than restored, for the same
// reason and with one more: every auto-clear event either closes the
// thread or replaces the channel under it, so PanelThread is not a pane
// worth handing focus back to. CloseThread sets focus itself, right after
// calling this. Dropping the flag rather than leaving it set keeps the
// pairing honest -- exitZoom must never see a flag some other path left
// behind.
//
// fs-zoom-invariant is pinned by TestFullscreen_ZoomAutoClearsStateNotFrame,
// which asserts a.zoomed directly. Do not "simplify" these two into one
// call: the difference is the whole point, and the older frame-delta test
// in fullscreen_red_test.go passes either way.
func (a *App) clearZoom() {
	if !a.zoomed {
		return
	}
	a.zoomed = false
	a.zoomSavedThreadFocus = false
	a.disarmPendingChords()
	a.invalidateZoomCaches()
}

// invalidateZoomCaches drops the render caches that are keyed on the
// pane's width/height, which the zoom transition changes. The screen
// memo is invalidated too: its key is the panel strings, and a
// zoom transition can produce a panel set that compares equal to the
// stored one while the composite must differ (G16).
func (a *App) invalidateZoomCaches() {
	a.invalidateAllWinModelCaches()
	a.threadPanel.InvalidateCache()
	a.lastScreenValid = false
}

func (a *App) workspaceSwitchIndex(msg tea.KeyMsg) (int, bool) {
	s := msg.String()
	if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		idx := int(s[0] - '1')
		if idx < len(a.workspaceItems) {
			return idx, true
		}
	}
	return -1, false
}

// zoomSuppresses reports whether msg is a key the suppression rule
// swallows while zoomed (fs-supp-set). Two surfaces: the layout /
// navigation keys, and the workspace numbers 1-9 (bare and alt+N).
func (a *App) zoomSuppresses(msg tea.KeyMsg) bool {
	if key.Matches(msg,
		a.keys.ToggleSidebar,   // ctrl+b
		a.keys.ToggleThread,    // ctrl+]
		a.keys.WindowPrefix,    // ctrl+w — suppressed AT the prefix arm
		a.keys.ActivityView,    // ctrl+a
		a.keys.FuzzyFinder,     // ctrl+t
		a.keys.FuzzyFinderAlt,  // ctrl+p
		a.keys.CommandMode,     // :
		a.keys.SearchMode,      // /
		a.keys.WorkspaceSearch, // ctrl+f
		a.keys.WorkspaceFinder, // keyless; kept for help parity
	) {
		return true
	}
	// Workspace numbers: bare 1-9 switch workspace.
	if _, ok := a.workspaceSwitchIndex(msg); ok {
		return true
	}
	return false
}

var reduceZoom reducerFunc = func(a *App, msg tea.Msg) (tea.Cmd, bool) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, false
	}

	// The `z` toggle. Normal mode only: in insert mode `z` must type
	// into the compose (fs-insert-z-types).
	if a.mode == ModeNormal && key.Matches(km, a.keys.Zoom) {
		if a.zoomed {
			a.exitZoom()
		} else {
			a.enterZoom()
		}
		return nil, true
	}

	if !a.zoomed || a.mode != ModeNormal {
		return nil, false
	}

	// Suppression. toastEager, NOT toastDeferred: the toast must be applied
	// EAGERLY, because the status bar is read straight after Update without
	// running the returned cmd. The deferred mode hides its setter inside a
	// tea.Batch, which 14 production call sites depend on, so that shape had
	// to survive -- hence one helper with two modes rather than two helpers.
	//
	// This comment used to argue for `toastWithClear, NOT uploadToastCmd`.
	// That helper no longer exists: the lane-toast consolidation (B2 / OT5)
	// deleted it and preserved its body as the toastEager mode, so the comment
	// was arguing against the call directly beneath it (B55).
	//
	// FIXED (B45). This comment used to say the toast was INVISIBLE and every
	// suppressed key a silent no-op. The premise still holds -- app.go's
	// compositor sets `status := ""` while zoomed, so the status ROW is not
	// composited -- but the conclusion no longer does: App.overlayZoomToast
	// (view_status.go:61) paints the live toast onto the last row of the
	// composed zoomed frame, which is the pane's bottom border, so it costs no
	// content row and forces no reflow. The screen memo keys on the toast text
	// while zoomed (app.go), without which the memo would serve a stale frame
	// and the toast would still never appear.
	//
	// Pinned by internal/ui/zoom_toast_visible_test.go, which asserts on
	// stripANSI(a.View().Content) and never on statusbarText(a) -- reading the
	// statusbar MODEL is what let the original defect hide, since that field is
	// set unconditionally whether or not anything renders it.
	//
	// Left as a comment rather than deleted because "the toast is invisible" was
	// a load-bearing belief in four places in the plan, and the next person to
	// read this line should see that it was measured and fixed, not merely
	// assumed away.
	if a.zoomSuppresses(km) {
		return a.uploadToastCmd(zoomSuppressedToast, zoomToastDuration, toastEager), true
	}

	// Zoom-exit. See the ESC PEEL ORDER note above for why this is
	// reachable only in ModeNormal.
	if key.Matches(km, a.keys.Escape) {
		a.exitZoom()
		return nil, true
	}
	return nil, false
}
