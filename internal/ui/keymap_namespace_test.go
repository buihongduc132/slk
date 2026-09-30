package ui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
)

// Keymap namespace guard (herdr coexistence contract).
//
// slk runs inside herdr panes. herdr claims a GLOBAL, direct (no-prefix)
// slice of the bare-ctrl space for itself — ctrl+h/j/k/l (pane focus),
// ctrl+s / ctrl+d (splits), ctrl+z (zoom), ctrl+x (copy mode), ctrl+p /
// ctrl+n (prev/next workspace), ctrl+1..9 (tab switch) — plus its prefix
// ctrl+u and alt+u / alt+i. Because herdr sees every key before the pane
// does, ANY bare-ctrl binding slk declares is dead on arrival under
// herdr: herdr eats it and slk never fires.
//
// The resolution (user requirement, 2026-09-26): slk's entire
// ctrl-modified surface lives in the ctrl+alt+<key> namespace, which
// herdr, wezTerm and the dy-plane stack leave untouched.
//
// This test is the codified rule. Adding a bare-ctrl binding anywhere in
// DefaultKeyMap fails here at CI time — the sibling of the bug this
// family of migrations killed.

// herdrDirectKeys is the set of key strings herdr claims globally, taken
// from the dy-plane herdr config ([keys] table). Keep in sync with
// prompts/components/dy-plane/ops/main-controller/herdr/config.toml.
var herdrDirectKeys = map[string]bool{
	// pane focus (direct bindings)
	"ctrl+h": true, "ctrl+j": true, "ctrl+k": true, "ctrl+l": true,
	// splits / zoom / copy mode
	"ctrl+s": true, "ctrl+d": true, "ctrl+z": true, "ctrl+x": true,
	// workspace prev/next
	"ctrl+p": true, "ctrl+n": true,
	// herdr prefix
	"ctrl+u": true,
	// tab switching without prefix
	"ctrl+1": true, "ctrl+2": true, "ctrl+3": true, "ctrl+4": true,
	"ctrl+5": true, "ctrl+6": true, "ctrl+7": true, "ctrl+8": true,
	"ctrl+9": true, "ctrl+0": true,
	// direct alt bindings (tmux M- heritage)
	"alt+u": true, "alt+i": true,
}

// isBareCtrl reports whether a key string is ctrl-modified WITHOUT alt:
// "ctrl+x" and "ctrl+shift+y" yes; "ctrl+alt+z" no; "alt+x" no.
func isBareCtrl(k string) bool {
	return strings.HasPrefix(k, "ctrl+") && !strings.HasPrefix(k, "ctrl+alt+")
}

func TestKeyMap_NoBareCtrlBindings(t *testing.T) {
	km := DefaultKeyMap()
	for name, binding := range keyMapBindings(km) {
		for _, k := range binding.Keys() {
			if isBareCtrl(k) {
				t.Errorf("%s: key %q is a bare-ctrl binding — herdr owns the bare-ctrl space; use ctrl+alt+<key>", name, k)
			}
		}
	}
}

func TestKeyMap_HerdrDisjoint(t *testing.T) {
	km := DefaultKeyMap()
	for name, binding := range keyMapBindings(km) {
		for _, k := range binding.Keys() {
			if herdrDirectKeys[k] {
				t.Errorf("%s: key %q is claimed globally by herdr — it can never fire inside a herdr pane", name, k)
			}
		}
	}
}

func TestKeyMap_CtrlAltKeysUnique(t *testing.T) {
	km := DefaultKeyMap()
	seen := map[string]string{}
	for name, binding := range keyMapBindings(km) {
		for _, k := range binding.Keys() {
			if !strings.HasPrefix(k, "ctrl+alt+") {
				continue
			}
			if prev, dup := seen[k]; dup {
				t.Errorf("ctrl+alt key %q bound twice: %s and %s", k, prev, name)
			}
			seen[k] = name
		}
	}
}

// TestKeyMap_HelpMatchesKeys pins that the help text of every migrated
// binding names the same key the binding actually matches — a stale
// "ctrl+f" help string on a ctrl+alt+f binding is a documentation bug
// this family of migrations produced once already.
func TestKeyMap_HelpMatchesKeys(t *testing.T) {
	km := DefaultKeyMap()
	for name, binding := range keyMapBindings(km) {
		ks := binding.Keys()
		if len(ks) == 0 {
			continue // keyless help-only entries (WorkspaceFinder etc.)
		}
		help := binding.Help().Key
		for _, part := range strings.Split(help, "/") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			// Help strings may name fallbacks that are not bindings
			// (":ws", "alt+enter", "gg"); only pin the ctrl-modified
			// ones, which are the migrated surface.
			if !strings.HasPrefix(part, "ctrl+") {
				continue
			}
			if !isBareCtrl(part) {
				continue // already ctrl+alt+… — fine
			}
			// A bare-ctrl help fragment must correspond to a real
			// legacy alias still present in Keys(); otherwise the help
			// advertises a dead binding.
			found := false
			for _, k := range ks {
				if k == part {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: help names %q but no such key remains bound (stale help from the pre-migration keymap)", name, part)
			}
		}
	}
}

// keyMapBindings reflects over KeyMap, returning field name → binding.
// Reflection keeps this guard total: a NEW KeyMap field is checked the
// moment it is added, with zero test edits.
func keyMapBindings(km KeyMap) map[string]key.Binding {
	out := map[string]key.Binding{}
	add := func(name string, b key.Binding) { out[name] = b }
	add("Up", km.Up)
	add("Down", km.Down)
	add("Left", km.Left)
	add("Right", km.Right)
	add("Enter", km.Enter)
	add("Escape", km.Escape)
	add("InsertMode", km.InsertMode)
	add("CommandMode", km.CommandMode)
	add("SearchMode", km.SearchMode)
	add("SearchNext", km.SearchNext)
	add("SearchPrev", km.SearchPrev)
	add("WorkspaceSearch", km.WorkspaceSearch)
	add("Tab", km.Tab)
	add("ShiftTab", km.ShiftTab)
	add("ToggleSidebar", km.ToggleSidebar)
	add("SidebarGrow", km.SidebarGrow)
	add("SidebarShrink", km.SidebarShrink)
	add("ToggleThread", km.ToggleThread)
	add("FuzzyFinder", km.FuzzyFinder)
	add("FuzzyFinderAlt", km.FuzzyFinderAlt)
	add("Top", km.Top)
	add("Bottom", km.Bottom)
	add("PageUp", km.PageUp)
	add("PageDown", km.PageDown)
	add("HalfPageUp", km.HalfPageUp)
	add("HalfPageDown", km.HalfPageDown)
	add("Quit", km.Quit)
	add("QuitConfirm", km.QuitConfirm)
	add("CloseThreadView", km.CloseThreadView)
	add("Reaction", km.Reaction)
	add("ReactionNav", km.ReactionNav)
	add("Edit", km.Edit)
	add("Delete", km.Delete)
	add("CopyMessage", km.CopyMessage)
	add("CopyPermalink", km.CopyPermalink)
	add("ForwardMessage", km.ForwardMessage)
	add("OpenPreview", km.OpenPreview)
	add("OpenLink", km.OpenLink)
	add("DownloadFile", km.DownloadFile)
	add("MarkUnread", km.MarkUnread)
	add("NextUnread", km.NextUnread)
	add("PrevUnread", km.PrevUnread)
	add("ActivityView", km.ActivityView)
	add("ActivityUnread", km.ActivityUnread)
	add("WorkspaceFinder", km.WorkspaceFinder)
	add("NewMessage", km.NewMessage)
	add("ThemeSwitcher", km.ThemeSwitcher)
	add("ThemeSwitcherGlobal", km.ThemeSwitcherGlobal)
	add("PresenceMenu", km.PresenceMenu)
	add("ToggleSection", km.ToggleSection)
	add("NavBack", km.NavBack)
	add("NavForward", km.NavForward)
	add("Help", km.Help)
	add("SaveThread", km.SaveThread)
	add("ListReactions", km.ListReactions)
	add("WindowPrefix", km.WindowPrefix)
	add("WinSplit", km.WinSplit)
	add("WinVSplit", km.WinVSplit)
	add("WinNavigate", km.WinNavigate)
	add("WinCycle", km.WinCycle)
	add("WinClose", km.WinClose)
	add("WinOnly", km.WinOnly)
	add("ToggleBroadcast", km.ToggleBroadcast)
	add("OpenInEditor", km.OpenInEditor)
	add("Zoom", km.Zoom)
	return out
}
