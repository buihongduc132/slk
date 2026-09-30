package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/ui/messages"
	"github.com/gammons/slk/internal/ui/sidebar"
)

// TestWorkspaceReadyPinChannelWinsOverLastChannel tests that PinChannelID wins over LastChannelID (land-channel).
func TestWorkspaceReadyPinChannelWinsOverLastChannel(t *testing.T) {
	selectOnReady := func(pinID, lastID string) (ChannelSelectedMsg, bool) {
		a := NewApp()
		_, _ = a.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
		_, cmd := a.Update(WorkspaceReadyMsg{
			TeamID:        "T1",
			InitialActive: true,
			Channels: []sidebar.ChannelItem{
				{ID: "C1", Name: "general", Type: "channel"},
				{ID: "C2", Name: "random", Type: "channel"},
				{ID: "C3", Name: "dev", Type: "channel"},
			},
			LastChannelID: lastID,
			PinChannelID:  pinID,
		})
		if cmd == nil {
			return ChannelSelectedMsg{}, false
		}
		return findChannelSelected(cmd())
	}

	// 1. PinChannelID wins over LastChannelID
	if sel, ok := selectOnReady("C3", "C2"); !ok || sel.ID != "C3" {
		t.Errorf("PinChannelID=C3 LastChannelID=C2: want C3, got ok=%v id=%q", ok, sel.ID)
	}

	// 2. No pin -> restores LastChannelID
	if sel, ok := selectOnReady("", "C2"); !ok || sel.ID != "C2" {
		t.Errorf("no pin LastChannelID=C2: want C2, got ok=%v id=%q", ok, sel.ID)
	}

	// 3. Stale/unresolvable PinChannelID -> falls back to LastChannelID
	if sel, ok := selectOnReady("CGONE", "C2"); !ok || sel.ID != "C2" {
		t.Errorf("stale pin LastChannelID=C2: want C2, got ok=%v id=%q", ok, sel.ID)
	}

	// 4. Stale pin and stale LastChannelID -> falls back to Channels[0]
	if sel, ok := selectOnReady("CGONE", "CGONE2"); !ok || sel.ID != "C1" {
		t.Errorf("stale pin and stale last: want C1, got ok=%v id=%q", ok, sel.ID)
	}
}

// TestWorkspaceReadyPinThread tests landing on a channel with thread open (land-thread, R6).
func TestWorkspaceReadyPinThread(t *testing.T) {
	a := newTestApp(t, withSize(120, 30))
	channels := []sidebar.ChannelItem{
		{ID: "C1", Name: "general", Type: "channel"},
	}

	// Send WorkspaceReadyMsg with thread pin
	_, cmdReady := a.Update(WorkspaceReadyMsg{
		TeamID:        "T1",
		InitialActive: true,
		Channels:      channels,
		PinChannelID:  "C1",
		PinThreadTS:   "1700000000.000100",
	})
	if cmdReady == nil {
		t.Fatal("WorkspaceReadyMsg returned nil cmd")
	}

	// ChannelSelectedMsg is dispatched
	_, _ = a.Update(ChannelSelectedMsg{ID: "C1", Name: "general", Type: "channel"})

	// Now ThreadRepliesLoadedMsg arrives
	parent := messages.MessageItem{TS: "1700000000.000100", Text: "parent thread message"}
	reply1 := messages.MessageItem{TS: "1700000000.000200", Text: "first reply"}
	_, _ = a.Update(ThreadRepliesLoadedMsg{
		ThreadTS: "1700000000.000100",
		Replies:  []messages.MessageItem{parent, reply1},
	})

	if !a.threadVisible {
		t.Errorf("thread panel not visible after ThreadRepliesLoadedMsg")
	}
	if a.focusedPanel != PanelThread {
		t.Errorf("focusedPanel = %v, want PanelThread", a.focusedPanel)
	}
}

// TestWorkspaceReadyPinZoomed tests that with PinFullscreen, the first content frame is zoomed (land-zoomed, land-ordering).
func TestWorkspaceReadyPinZoomed(t *testing.T) {
	a := newTestApp(t, withSize(120, 30))
	channels := []sidebar.ChannelItem{
		{ID: "C1", Name: "general", Type: "channel"},
	}

	// Ready message with PinFullscreen
	_, _ = a.Update(WorkspaceReadyMsg{
		TeamID:        "T1",
		InitialActive: true,
		Channels:      channels,
		PinChannelID:  "C1",
		PinFullscreen: true,
	})

	// ChannelSelectedMsg arrives (Tier 3 cold start -> empty messages, spinner)
	_, _ = a.Update(ChannelSelectedMsg{ID: "C1", Name: "general", Type: "channel"})

	// Before messages load, app should NOT be zoomed on an empty pane
	if a.zoomed {
		t.Errorf("app zoomed before messages landed (empty pane snapshot poison)")
	}

	// Messages land
	msg := messages.MessageItem{TS: "1700000000.000001", Text: "pinned channel content"}
	_, _ = a.Update(MessagesLoadedMsg{
		ChannelID: "C1",
		Messages:  []messages.MessageItem{msg},
	})

	if !a.zoomed {
		t.Fatalf("app not zoomed after MessagesLoadedMsg")
	}

	zoomedFrame := a.View().Content
	assertZoomedFrame(t, a, zoomedFrame, "pinned channel content")

	// Pressing 'z' exits zoom
	_, _ = a.Update(keyPress('z'))
	if a.zoomed {
		t.Errorf("pressing 'z' did not exit zoom")
	}
}

// TestWorkspaceReadyZoomThreadPane_WidthTabled tests that thread zoom promotes the thread pane at both stacked and side-by-side widths (lp-zoom-threadpane).
func TestWorkspaceReadyZoomThreadPane_WidthTabled(t *testing.T) {
	widths := []struct {
		name string
		w    int
	}{
		{"stacked", 120},
		{"side-by-side", 180}, // >= 162
	}

	for _, tc := range widths {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp(t, withSize(tc.w, 30))
			channels := []sidebar.ChannelItem{
				{ID: "C1", Name: "general", Type: "channel"},
			}

			_, _ = a.Update(WorkspaceReadyMsg{
				TeamID:        "T1",
				InitialActive: true,
				Channels:      channels,
				PinChannelID:  "C1",
				PinThreadTS:   "1700000000.000100",
				PinFullscreen: true,
			})

			_, _ = a.Update(ChannelSelectedMsg{ID: "C1", Name: "general", Type: "channel"})

			// Thread replies land
			reply := messages.MessageItem{TS: "1700000000.000200", Text: "thread reply in zoom"}
			_, _ = a.Update(ThreadRepliesLoadedMsg{
				ThreadTS: "1700000000.000100",
				Replies:  []messages.MessageItem{reply},
			})

			if !a.zoomed {
				t.Fatalf("at width %d: app not zoomed", tc.w)
			}
			if !a.zoomFrontIsThread() {
				t.Fatalf("at width %d: zoomFrontIsThread() = false, want true", tc.w)
			}

			zoomedFrame := a.View().Content
			assertZoomedFrame(t, a, zoomedFrame, "thread reply in zoom")

			// Exit zoom
			_, _ = a.Update(keyPress('z'))
			if a.zoomed {
				t.Fatalf("at width %d: 'z' did not exit zoom", tc.w)
			}
			if a.launchPinThreadZoom {
				t.Fatalf("at width %d: launchPinThreadZoom not cleared on exit", tc.w)
			}
		})
	}
}

// TestWorkspaceReadyThreadNotFoundRuntime tests runtime fallback when a pinned thread fails to load (lp-thread-notfound-runtime, R2, R3).
func TestWorkspaceReadyThreadNotFoundRuntime(t *testing.T) {
	a := newTestApp(t, withSize(120, 30))
	channels := []sidebar.ChannelItem{
		{ID: "C1", Name: "general", Type: "channel"},
	}

	_, _ = a.Update(WorkspaceReadyMsg{
		TeamID:        "T1",
		InitialActive: true,
		Channels:      channels,
		PinChannelID:  "C1",
		PinThreadTS:   "1700000000.000999",
		PinFullscreen: true,
	})

	_, _ = a.Update(ChannelSelectedMsg{ID: "C1", Name: "general", Type: "channel"})

	// Thread fetch fails (nil Replies)
	_, cmd := a.Update(ThreadRepliesLoadedMsg{
		ThreadTS: "1700000000.000999",
		Replies:  nil,
	})

	// Thread panel should close
	if a.threadVisible {
		t.Errorf("threadVisible = true after thread not found, want false")
	}

	// Runtime thread miss re-zooms nothing, staying on channel normal
	if a.zoomed {
		t.Errorf("zoomed = true, want false (runtime thread miss re-zooms nothing)")
	}

	// A notice toast naming the failed thread should be returned
	if cmd == nil {
		t.Errorf("expected toast command for thread not found")
	}
}

// TestWorkspaceReadyNoticeCompose tests that launch notices are surfaced via uploadToastCmd (lp-notice-compose, fallback-normal).
func TestWorkspaceReadyNoticeCompose(t *testing.T) {
	a := newTestApp(t, withSize(120, 30))
	channels := []sidebar.ChannelItem{
		{ID: "C1", Name: "general", Type: "channel"},
	}

	noticeText := "workspace acme not configured; channel C_MISSING not found"
	_, _ = a.Update(WorkspaceReadyMsg{
		TeamID:        "T1",
		InitialActive: true,
		Channels:      channels,
		LaunchNotice:  noticeText,
	})

	sbText := statusbarText(a)
	if !strings.Contains(sbText, "workspace acme not configured") {
		t.Errorf("statusbar text = %q, want notice text %q", sbText, noticeText)
	}
}
