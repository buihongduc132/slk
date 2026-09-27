// internal/ui/view_status.go
//
// Status row renderer for App.View (Phase 6d).
//
// The status row spans the bottom of the screen, composed of a
// rail-colored spacer matching the workspace rail's width plus
// the statusbar widget rendered at (a.width - railWidth). The
// composed row is cached on (statusbar.Version, statusWidth,
// themeVer) so a render-only keystroke (typing into compose) is
// a single cache hit rather than a re-join + style-walk.
package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/gammons/slk/internal/ui/styles"
)

// renderStatusRow returns the composed status row (rail-spacer +
// statusbar), reading-or-storing through the panel-render cache.
// themeVer is passed in so the call site stays the canonical
// source of theme freshness (App.View captures it once at the
// top of the render path).
func (a *App) renderStatusRow(railWidth, statusWidth int, themeVer int64) string {
	c := &a.renderCache.status
	if c.hit(a.statusbar.Version(), statusWidth, 1, themeVer) {
		return c.output
	}
	railSpacer := lipgloss.NewStyle().
		Width(railWidth).
		Background(styles.RailBackground).
		Render("")
	out := lipgloss.JoinHorizontal(lipgloss.Center, railSpacer, a.statusbar.View(statusWidth))
	c.store(out, a.statusbar.Version(), statusWidth, 1, themeVer)
	return out
}

// overlayZoomToast paints a live toast onto the LAST row of an already-composed
// zoomed frame, returning the frame unchanged when there is nothing to show.
//
// Why this exists (B45): while zoomed, View() sets `status := ""` and never
// calls renderStatusRow, and Compute sets statusHeight = 0 so the pane owns the
// full terminal height. A toast set by the suppression path therefore reached no
// pixels at all — every suppressed key was a silent no-op, indistinguishable
// from a dropped keystroke. Its only oracle was statusbarText(a), which renders
// the statusbar MODEL in isolation, so the assertion's subject was a field set
// unconditionally and could not vary with whether the row was composited.
//
// Why the last row specifically, rather than reserving a row: the zoomed pane's
// final row holds its bottom BORDER, not content (measured: at height 30 the
// pane spans rows 0..29 with the border on 29, and content ends at 28). So
// overwriting it costs no content and no reflow. Reserving a row instead would
// shrink ContentHeight and make the zoomed frame no longer full-height, which
// fs-statusrow-math and its golden both pin.
//
// The width is the full terminal: while zoomed railWidth is 0, so there is no
// rail spacer to match.
func (a *App) overlayZoomToast(screen string) string {
	toast := a.statusbar.Toast()
	if !a.zoomed || toast == "" {
		return screen
	}
	lines := strings.Split(screen, "\n")
	if len(lines) == 0 {
		return screen
	}
	row := lipgloss.NewStyle().
		Width(a.width).
		Foreground(styles.Accent).
		Background(styles.SurfaceDark).
		Bold(true).
		Render(ansi.Truncate(toast, a.width, "…"))
	lines[len(lines)-1] = row
	return strings.Join(lines, "\n")
}
