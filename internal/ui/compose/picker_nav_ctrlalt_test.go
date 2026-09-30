package compose

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/ui/channelpicker"
	"github.com/gammons/slk/internal/ui/mentionpicker"
)

// ctrl+alt picker navigation (herdr coexistence, rework round 1).
//
// herdr owns bare ctrl+p / ctrl+n globally (workspace prev/next), so the
// legacy bare-ctrl picker nav is dead inside a herdr pane. These tests
// pin that every compose overlay picker — mention, #channel, :emoji —
// navigates with ctrl+alt+p / ctrl+alt+n through the real Update
// dispatch (not the helper in isolation). If a picker's handler adds a
// new raw-dispatch nav path, add the matching row here.

func ctrlAltP() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl | tea.ModAlt} }
func ctrlAltN() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl | tea.ModAlt} }

func TestMentionNavigate_CtrlAltPN(t *testing.T) {
	m := New("general")
	m.SetUsers([]mentionpicker.User{
		{ID: "U1", DisplayName: "Alice", Username: "alice"},
		{ID: "U2", DisplayName: "Bob", Username: "bob"},
		{ID: "U3", DisplayName: "Carol", Username: "carol"},
	})
	m.SetWidth(80)
	_ = m.Focus()
	m, _ = m.Update(tea.KeyPressMsg{Code: '@', Text: "@"})
	if !m.IsMentionActive() {
		t.Fatal("precondition: mention picker should be open")
	}

	// ctrl+alt+n moves down, ctrl+alt+p moves back up.
	m, _ = m.Update(ctrlAltN())
	if m.mentionPicker.Selected() != 1 {
		t.Fatalf("ctrl+alt+n: selection = %d, want 1", m.mentionPicker.Selected())
	}
	m, _ = m.Update(ctrlAltP())
	if m.mentionPicker.Selected() != 0 {
		t.Fatalf("ctrl+alt+p: selection = %d, want 0", m.mentionPicker.Selected())
	}
	// Legacy bare-ctrl still works (plain terminals).
	m, _ = m.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	if m.mentionPicker.Selected() != 1 {
		t.Fatalf("bare ctrl+n: selection = %d, want 1", m.mentionPicker.Selected())
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	if m.mentionPicker.Selected() != 0 {
		t.Fatalf("bare ctrl+p: selection = %d, want 0", m.mentionPicker.Selected())
	}
}

func TestChannelNavigate_CtrlAltPN(t *testing.T) {
	m := New("general")
	m.SetChannels([]channelpicker.Channel{
		{ID: "C111", Name: "general", Type: "channel"},
		{ID: "C222", Name: "general-help", Type: "channel"},
		{ID: "C333", Name: "engineering", Type: "channel"},
	})
	m.SetWidth(80)
	_ = m.Focus()
	m, _ = m.Update(tea.KeyPressMsg{Code: '#', Text: "#"})
	if !m.IsChannelActive() {
		t.Fatal("precondition: channel picker should be open")
	}

	m, _ = m.Update(ctrlAltN())
	if m.channelPicker.Selected() != 1 {
		t.Fatalf("ctrl+alt+n: selection = %d, want 1", m.channelPicker.Selected())
	}
	m, _ = m.Update(ctrlAltP())
	if m.channelPicker.Selected() != 0 {
		t.Fatalf("ctrl+alt+p: selection = %d, want 0", m.channelPicker.Selected())
	}
}

func TestEmojiNavigate_CtrlAltPN(t *testing.T) {
	m := New("general")
	m.SetEmojiEntries(sampleEmojiEntries())
	_ = m.Focus()
	m = typeChars(t, m, ":ro")
	if !m.IsEmojiActive() {
		t.Fatal("precondition: emoji picker should be open on ':ro'")
	}

	m, _ = m.Update(ctrlAltN())
	if sel := m.emojiPicker.Selected(); sel != 1 {
		t.Fatalf("ctrl+alt+n: selection = %d, want 1", sel)
	}
	m, _ = m.Update(ctrlAltP())
	if sel := m.emojiPicker.Selected(); sel != 0 {
		t.Fatalf("ctrl+alt+p: selection = %d, want 0", sel)
	}
}
