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
	"strings"
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

// enterZoom snapshots the front pane's viewport so exitZoom can put it
// back, then zooms. Callers must already have checked !a.zoomed.
func (a *App) enterZoom() {
	a.zoomSavedYOffset = a.messagepane.YOffset()
	a.zoomSavedSelectedIndex = a.messagepane.SelectedIndex()
	a.zoomed = true
	a.invalidateZoomCaches()
}

// exitZoom leaves zoom and restores the pane state captured by
// enterZoom. The single exit path: every place that drops zoom (the `z`
// toggle, esc here, insert mode's esc arm, and the auto-clear events)
// goes through it, so none of them can forget the cache invalidation or
// the viewport restore.
func (a *App) exitZoom() {
	if !a.zoomed {
		return
	}
	a.zoomed = false
	a.invalidateZoomCaches()
	a.messagepane.SetViewport(a.zoomSavedYOffset, a.zoomSavedSelectedIndex)
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
	// Workspace numbers: bare 1-9 and alt+1-9 both switch workspace.
	s := msg.String()
	if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		return true
	}
	return strings.HasPrefix(s, "alt+") && len(s) == 5 && s[4] >= '1' && s[4] <= '9'
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

	// Suppression. toastWithClear, NOT uploadToastCmd: the toast must be
	// applied EAGERLY, because the status bar is read straight after
	// Update without running the returned cmd. uploadToastCmd hides its
	// setter inside a tea.Batch and 14 production call sites depend on
	// that shape, so it must not be made eager.
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
