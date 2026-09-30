// internal/ui/keys.go
package ui

import "charm.land/bubbles/v2/key"

// KeyMap holds every user-facing binding. The ctrl-modified surface
// lives entirely in the ctrl+alt+<key> namespace: herdr (the terminal
// workspace manager slk runs inside) claims the bare-ctrl space
// globally — ctrl+h/j/k/l pane focus, ctrl+s/ctrl+d splits, ctrl+z
// zoom, ctrl+x copy mode, ctrl+p/ctrl+n workspace prev/next, ctrl+1..9
// tab switch — and its prefix is ctrl+u, so a bare-ctrl binding here
// could never fire under herdr. ctrl+alt+<key> is free in herdr,
// wezTerm and the dy-plane stack. TestKeyMap_NoBareCtrlBindings in
// keymap_namespace_test.go is the codified guard; keep the field list
// there in sync when adding fields to this struct.
type KeyMap struct {
	Up                  key.Binding
	Down                key.Binding
	Left                key.Binding
	Right               key.Binding
	Enter               key.Binding
	Escape              key.Binding
	InsertMode          key.Binding
	CommandMode         key.Binding
	SearchMode          key.Binding
	SearchNext          key.Binding
	SearchPrev          key.Binding
	WorkspaceSearch     key.Binding
	Tab                 key.Binding
	ShiftTab            key.Binding
	ToggleSidebar       key.Binding
	SidebarGrow         key.Binding
	SidebarShrink       key.Binding
	ToggleThread        key.Binding
	FuzzyFinder         key.Binding
	FuzzyFinderAlt      key.Binding
	Top                 key.Binding
	Bottom              key.Binding
	PageUp              key.Binding
	PageDown            key.Binding
	HalfPageUp          key.Binding
	HalfPageDown        key.Binding
	Quit                key.Binding
	QuitConfirm         key.Binding
	CloseThreadView     key.Binding
	Reaction            key.Binding
	ReactionNav         key.Binding
	Edit                key.Binding
	Delete              key.Binding
	CopyMessage         key.Binding
	CopyPermalink       key.Binding
	ForwardMessage      key.Binding
	OpenPreview         key.Binding
	OpenLink            key.Binding
	DownloadFile        key.Binding
	MarkUnread          key.Binding
	NextUnread          key.Binding
	PrevUnread          key.Binding
	ActivityView        key.Binding
	ActivityUnread      key.Binding
	WorkspaceFinder     key.Binding
	NewMessage          key.Binding
	ThemeSwitcher       key.Binding
	ThemeSwitcherGlobal key.Binding
	PresenceMenu        key.Binding
	ToggleSection       key.Binding
	NavBack             key.Binding
	NavForward          key.Binding
	Help                key.Binding
	SaveThread          key.Binding
	ListReactions       key.Binding
	WindowPrefix        key.Binding
	WinSplit            key.Binding
	WinVSplit           key.Binding
	WinNavigate         key.Binding
	WinCycle            key.Binding
	WinClose            key.Binding
	WinOnly             key.Binding
	ToggleBroadcast     key.Binding
	OpenInEditor        key.Binding
	Zoom                key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:              key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/up", "up")),
		Down:            key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/down", "down")),
		Left:            key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/left", "left")),
		Right:           key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/right", "right")),
		Enter:           key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open/confirm")),
		Escape:          key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		InsertMode:      key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "insert mode")),
		CommandMode:     key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command mode")),
		SearchMode:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		SearchNext:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next match")),
		SearchPrev:      key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "prev match")),
		WorkspaceSearch: key.NewBinding(key.WithKeys("ctrl+alt+f"), key.WithHelp("ctrl+alt+f", "search workspace")),
		Tab:             key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next panel")),
		ShiftTab:        key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev panel")),
		ToggleSidebar:   key.NewBinding(key.WithKeys("ctrl+alt+b"), key.WithHelp("ctrl+alt+b", "toggle sidebar")),
		SidebarGrow:     key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "widen sidebar")),
		SidebarShrink:   key.NewBinding(key.WithKeys("["), key.WithHelp("[", "narrow sidebar")),
		ToggleThread:    key.NewBinding(key.WithKeys("ctrl+alt+]"), key.WithHelp("ctrl+alt+]", "toggle thread")),
		FuzzyFinder:     key.NewBinding(key.WithKeys("ctrl+alt+t"), key.WithHelp("ctrl+alt+t", "switch channel")),
		FuzzyFinderAlt:  key.NewBinding(key.WithKeys("ctrl+alt+p"), key.WithHelp("ctrl+alt+p", "switch channel")),
		Top:             key.NewBinding(key.WithKeys("g"), key.WithHelp("gg", "top")),
		Bottom:          key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		PageUp:          key.NewBinding(key.WithKeys("pgup"), key.WithHelp("PgUp", "page up")),
		PageDown:        key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("PgDn", "page down")),
		HalfPageUp:      key.NewBinding(key.WithKeys("ctrl+alt+u"), key.WithHelp("ctrl+alt+u", "half page up")),
		HalfPageDown:    key.NewBinding(key.WithKeys("ctrl+alt+d"), key.WithHelp("ctrl+alt+d", "half page down")),
		// ctrl+alt+c and `Q` both route through the quit-confirm
		// prompt; `Q` (capital) keeps working everywhere as before.
		Quit:            key.NewBinding(key.WithKeys("ctrl+alt+c"), key.WithHelp("ctrl+alt+c / Q", "quit (confirm)")),
		QuitConfirm:     key.NewBinding(key.WithKeys("Q"), key.WithHelp("Q", "quit (confirm)")),
		CloseThreadView: key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "close thread view")),
		Reaction:        key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "add reaction")),
		ReactionNav:     key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "navigate reactions")),
		Edit:            key.NewBinding(key.WithKeys("E"), key.WithHelp("E", "edit message")),
		Delete:          key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete message")),
		CopyMessage:     key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy message")),
		CopyPermalink:   key.NewBinding(key.WithKeys("Y", "C"), key.WithHelp("Y/C", "copy permalink")),
		ForwardMessage:  key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "forward message")),
		OpenPreview:     key.NewBinding(key.WithKeys("O", "v"), key.WithHelp("O/v", "open image preview")),
		OpenLink:        key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open link in message")),
		DownloadFile:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "download file in message")),
		MarkUnread:      key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "mark unread")),
		NextUnread:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "next unread channel")),
		PrevUnread:      key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "prev unread channel")),
		ActivityView:    key.NewBinding(key.WithKeys("ctrl+alt+a"), key.WithHelp("ctrl+alt+a", "open activity")),
		ActivityUnread:  key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "Activity: toggle unread-only")),
		// Keyless: no chord is bound; :ws (and 1-9 directly) switch
		// workspaces (window-management design §4). The keyless
		// binding keeps the help-overlay entry pointing at :ws.
		WorkspaceFinder:     key.NewBinding(key.WithHelp(":ws", "switch workspace")),
		NewMessage:          key.NewBinding(key.WithKeys("ctrl+alt+n"), key.WithHelp("ctrl+alt+n", "new message")),
		ThemeSwitcher:       key.NewBinding(key.WithKeys("ctrl+alt+y"), key.WithHelp("ctrl+alt+y", "switch theme (per workspace)")),
		ThemeSwitcherGlobal: key.NewBinding(key.WithKeys("ctrl+alt+shift+y"), key.WithHelp("ctrl+alt+shift+y", "set default theme")),
		PresenceMenu:        key.NewBinding(key.WithKeys("ctrl+alt+s"), key.WithHelp("ctrl+alt+s", "set status")),
		ToggleSection:       key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle section")),
		NavBack:             key.NewBinding(key.WithKeys("ctrl+alt+h"), key.WithHelp("ctrl+alt+h", "navigate back")),
		NavForward:          key.NewBinding(key.WithKeys("ctrl+alt+k"), key.WithHelp("ctrl+alt+k", "navigate forward")),
		Help:                key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "show keybindings")),
		SaveThread:          key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "save thread")),
		ListReactions:       key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "list reactions")),
		// Window commands (design §4). WindowPrefix is the only real
		// binding; the Win* entries are keyless help-only bindings
		// (same trick as WorkspaceFinder above) — actual dispatch of
		// the chord key happens in handleWindowChord.
		WindowPrefix: key.NewBinding(key.WithKeys("ctrl+alt+w"), key.WithHelp("ctrl+alt+w", "window commands")),
		WinSplit:     key.NewBinding(key.WithHelp("ctrl+alt+w s / :sp", "split window")),
		WinVSplit:    key.NewBinding(key.WithHelp("ctrl+alt+w v / :vsp", "vertical split window")),
		WinNavigate:  key.NewBinding(key.WithHelp("ctrl+alt+w h/j/k/l", "focus window in direction")),
		WinCycle:     key.NewBinding(key.WithHelp("ctrl+alt+w w", "cycle windows")),
		WinClose:     key.NewBinding(key.WithHelp("ctrl+alt+w q / :q", "close window")),
		WinOnly:      key.NewBinding(key.WithHelp("ctrl+alt+w o / :only", "close other windows")),
		// Insert mode, thread compose only: toggles Slack's
		// "Also send to #channel" checkbox for the next reply.
		// Alt+Enter sends and broadcasts in a single keystroke.
		ToggleBroadcast: key.NewBinding(key.WithKeys("ctrl+alt+o"), key.WithHelp("ctrl+alt+o / alt+enter", "also send reply to channel")),
		// Shadows the textarea's own ctrl+e (LineEnd); "End" still works.
		OpenInEditor: key.NewBinding(key.WithKeys("ctrl+alt+e"), key.WithHelp("ctrl+alt+e", "edit message in $EDITOR")),
		Zoom:         key.NewBinding(key.WithKeys("z"), key.WithHelp("z", " zoom pane")),
	}
}
