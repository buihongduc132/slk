package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/core"
	"github.com/gammons/slk/internal/ids"
)

// B24 wiring: the emojipicker recent tier is only worth anything if the
// frecent list actually reaches it, and internal/ui does no I/O of its
// own — the data has to arrive through core.ReactionService.LoadFrecent.
//
// The reaction picker reloads per-open (openPickerFrom* call LoadFrecent
// directly). The compose dropdown cannot: it is opened by compose's own
// `:` trigger in maybeOpenEmojiPicker, which never reaches App. So App
// PUSHES via refreshComposeFrecent, and these tests pin that push at
// both call sites.

// frecentReactionService builds a ReactionService whose LoadFrecent
// returns the named emoji in order, and which records RecordFrecent
// calls into got.
func frecentReactionService(names []string, recorded *[]string) core.ReactionService {
	entries := make([]core.EmojiEntry, 0, len(names))
	for _, n := range names {
		entries = append(entries, core.EmojiEntry{Name: n, Unicode: "x"})
	}
	return core.NewReactionService(
		func(_ ids.ChannelID, _ ids.MessageTS, _ string) error { return nil },
		func(_ ids.ChannelID, _ ids.MessageTS, _ string) error { return nil },
		func(int) []core.EmojiEntry { return entries },
		func(emoji string) {
			if recorded != nil {
				*recorded = append(*recorded, emoji)
			}
		},
	)
}

// typeIntoCompose sends s to the channel compose one key at a time
// through the real Update path, so maybeOpenEmojiPicker runs exactly as
// it does in production. The textarea only accepts keys while focused,
// which in production is ModeInsert's job (mode_normal.go:86).
func typeIntoCompose(a *App, s string) {
	a.compose.Focus()
	for _, r := range s {
		a.compose, _ = a.compose.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// dropdownOrder returns the emoji names the compose dropdown renders, in
// render order, by parsing the `:name:` tokens out of the frame. Reading
// the FRAME (not a filtered-list getter, which compose does not expose)
// also proves the order survives all the way to what the user sees.
func dropdownOrder(t *testing.T, a *App) []string {
	t.Helper()
	frame := a.compose.EmojiPickerView(60)
	if frame == "" {
		t.Fatal("compose emoji dropdown rendered empty; the picker is not open")
	}
	var out []string
	for _, line := range strings.Split(frame, "\n") {
		// Each row ends with the shortcode `:name:`; take the last
		// colon-delimited token on the line.
		first := strings.Index(line, ":")
		last := strings.LastIndex(line, ":")
		if first < 0 || last <= first {
			continue
		}
		name := line[first+1 : last]
		if name == "" || strings.ContainsAny(name, " \t") {
			continue
		}
		out = append(out, name)
	}
	if len(out) == 0 {
		t.Fatalf("parsed no emoji rows out of the dropdown frame:\n%s", frame)
	}
	return out
}

// The frecent list reaches the compose dropdown and outranks a
// better-tier match there. "ear_of_rice" is only a TierSubstring match
// for "ice" (ice_cream is TierPrefix), so it leading is only possible if
// LoadFrecent's data arrived through the seam AND the recent tier is
// consulted before tier.
func TestComposeFrecent_ReachesDropdownAndOutranksTier(t *testing.T) {
	a := newTestApp(t, withSize(100, 30))
	a.SetCustomEmoji(nil) // full built-in entry set
	a.SetReactionService(frecentReactionService([]string{"ear_of_rice"}, nil))

	typeIntoCompose(a, ":ice")

	if !a.compose.IsEmojiActive() {
		t.Fatal("precondition: typing \":ice\" did not open the compose emoji dropdown")
	}
	order := dropdownOrder(t, a)
	if order[0] != "ear_of_rice" {
		t.Errorf("compose dropdown order = %v, want ear_of_rice first. It is only a "+
			"TierSubstring match for \"ice\" while ice_cream is TierPrefix, so leading "+
			"requires the frecent list to have arrived via "+
			"core.ReactionService.LoadFrecent and been ranked ahead of tier (B24).", order)
	}
}

// With no usage history the dropdown is byte-identical to the frame
// produced with no reaction service wired at all. This is the state of
// every new user's first run and of every test that does not opt in.
func TestComposeFrecent_EmptyHistoryLeavesDropdownUnchanged(t *testing.T) {
	baseline := newTestApp(t, withSize(100, 30))
	baseline.SetCustomEmoji(nil)
	typeIntoCompose(baseline, ":ice")
	want := baseline.compose.EmojiPickerView(60)
	if want == "" {
		t.Fatal("precondition: baseline dropdown is empty")
	}

	withService := newTestApp(t, withSize(100, 30))
	withService.SetCustomEmoji(nil)
	// A real service whose frecent list is empty — the cold-cache case.
	withService.SetReactionService(frecentReactionService(nil, nil))
	typeIntoCompose(withService, ":ice")
	got := withService.compose.EmojiPickerView(60)

	if got != want {
		t.Errorf("empty-frecency dropdown differs from the no-service dropdown.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// Recording a reaction refreshes the compose dropdown's recent tier.
// Without the refreshComposeFrecent call in the RecordFrecent arm, an
// emoji just used via the reaction picker would not influence the
// compose dropdown until the next restart.
func TestComposeFrecent_RefreshedAfterReactionRecordsUse(t *testing.T) {
	a := newTestApp(t, append(reactionPickerOpts(false), withSize(100, 30))...)
	a.SetCustomEmoji(nil)
	a.SetCurrentUserID(reactionTestUserID)

	// LoadFrecent starts empty, then reports ear_of_rice once a use has
	// been recorded — the cache's own behaviour, faked at the port.
	var recorded []string
	var frecent []core.EmojiEntry
	a.SetReactionService(core.NewReactionService(
		func(_ ids.ChannelID, _ ids.MessageTS, _ string) error { return nil },
		func(_ ids.ChannelID, _ ids.MessageTS, _ string) error { return nil },
		func(int) []core.EmojiEntry { return frecent },
		func(emoji string) {
			recorded = append(recorded, emoji)
			frecent = []core.EmojiEntry{{Name: "ear_of_rice", Unicode: "x"}}
		},
	))

	// Before: no history, so the prefix match leads.
	typeIntoCompose(a, ":ice")
	if before := dropdownOrder(t, a); before[0] != "ice_cream" {
		t.Fatalf("precondition: dropdown order = %v, want ice_cream first with empty history", before)
	}
	a.compose.CloseEmoji()
	a.compose.SetValue("")

	// Drive the production record path: the real mode handler, which is
	// where the RecordFrecent arm (and its refreshComposeFrecent call)
	// lives.
	a.openPickerFromMessage()
	if !a.reactionPicker.IsVisible() {
		t.Fatal("precondition: openPickerFromMessage did not open the reaction picker")
	}
	a.SetMode(ModeReactionPicker)
	for _, r := range "ear_of_rice" {
		handleReactionPickerMode(a, keyPress(r))
	}
	handleReactionPickerMode(a, keyCode(tea.KeyEnter))
	if len(recorded) == 0 {
		t.Fatal("precondition: RecordFrecent was never called")
	}

	// After: the recorded emoji now leads despite its worse tier.
	typeIntoCompose(a, ":ice")
	after := dropdownOrder(t, a)
	if after[0] != "ear_of_rice" {
		t.Errorf("dropdown order after recording a use = %v, want ear_of_rice first. "+
			"The RecordFrecent arm must call refreshComposeFrecent, or the compose "+
			"dropdown keeps a stale recent tier until restart.", after)
	}
}
