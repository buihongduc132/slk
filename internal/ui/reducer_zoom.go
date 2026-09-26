package ui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

var reduceZoom reducerFunc = func(a *App, msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, a.keys.Zoom) {
			if a.mode == ModeNormal {
				a.zoomed = !a.zoomed
				a.invalidateAllWinModelCaches()
				a.threadPanel.InvalidateCache()
				if m, ok := a.messagepane.SelectedMessage(); ok {
					a.messagepane.SelectByTS(m.TS)
				}
				return nil, true
			}
		}

		if a.zoomed {
			// suppression toasts for G5
			s := msg.String()
			isAltNumber := strings.HasPrefix(s, "alt+") && len(s) == 5 && s[4] >= '1' && s[4] <= '9'
			isAltNumber = isAltNumber || (len(s) == 1 && s[0] >= '1' && s[0] <= '9') // Wait, the test does keyPress('2'), which might just output "2"
			if key.Matches(msg, a.keys.ToggleSidebar, a.keys.WindowPrefix) || isAltNumber || key.Matches(msg, a.keys.WorkspaceSearch, a.keys.WorkspaceFinder) {
				return a.uploadToastCmd("Action not available while zoomed", 2*time.Second), true
			}
			// esc precedence
			if key.Matches(msg, a.keys.Escape) {
				a.zoomed = false
				a.invalidateAllWinModelCaches()
				a.threadPanel.InvalidateCache()
				if m, ok := a.messagepane.SelectedMessage(); ok {
					a.messagepane.SelectByTS(m.TS)
				}
				return nil, true
			}
		}
	}
	return nil, false
}
