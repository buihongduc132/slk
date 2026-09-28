package messages

import "testing"

// B5 regression cover for ClickAt's trailing-spacer guard.
//
// renderMessageEntry appends a blank trailing spacer row to every message
// entry except the last (`i < len(m.messages)-1`), and that row counts
// toward viewEntry.height. ClickAt excludes it from the entry's hit range:
//
//	hitEnd := currentLine + entry.height
//	if i < len(m.cache)-1 {
//		hitEnd-- // trailing spacer row is dead space
//	}
//
// Both halves of that conditional carry behaviour, and neither was pinned:
//
//   - drop the `hitEnd--` and spacer rows start selecting the message above
//     them, so a click in the gap between two messages moves the selection.
//   - drop the `i < len(m.cache)-1` condition and always decrement, and the
//     final message's last row becomes dead space even though it has no
//     spacer to skip.
//
// The two guards agree on which entry is exempt even though one counts
// messages and the other counts cache entries (the cache also holds date
// separators and the "new messages" landmark): buildCache only ever appends
// a separator *before* a message, so the last cache entry is always the
// last message's entry.

const (
	// Width is narrow enough that each fixture message wraps onto several
	// rows; height is tall enough that the whole cache fits on screen, so
	// View() leaves yOffset at 0.
	spacerClickWidth  = 40
	spacerClickHeight = 60
)

// spacerClickModel returns a Model whose cache holds a date separator
// followed by three multi-row message entries.
//
// The cache is populated through the real View() path rather than by
// assigning m.cache directly, because the trailing spacer rows under test
// are appended by renderMessageEntry during a build -- a hand-assigned
// cache would not have them, and the test would assert nothing.
func spacerClickModel(t *testing.T) *Model {
	t.Helper()
	m := New([]MessageItem{
		{TS: "1700000000.000100", UserName: "alice", UserID: "U1", Timestamp: "1:00 PM",
			Text: "one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen"},
		{TS: "1700000001.000200", UserName: "bob", UserID: "U2", Timestamp: "1:01 PM",
			Text: "aaa bbb ccc ddd eee fff ggg hhh iii jjj kkk lll mmm nnn ooo ppp qqq rrr sss ttt"},
		{TS: "1700000002.000300", UserName: "carol", UserID: "U3", Timestamp: "1:02 PM",
			Text: "zzz yyy xxx www vvv uuu ttt sss rrr qqq ppp ooo nnn mmm lll kkk"},
	}, "general")
	_ = m.View(spacerClickHeight, spacerClickWidth)
	if m.yOffset != 0 {
		t.Fatalf("fixture precondition: want yOffset 0 (nothing scrolled), got %d", m.yOffset)
	}
	if len(m.cache) < 2 {
		t.Fatalf("fixture precondition: want at least 2 cache entries, got %d", len(m.cache))
	}
	return &m
}

// paneY converts an absolute cache line into the pane-local y that ClickAt
// expects (App.panelAt space: chrome occupies 0..chromeHeight-1).
func paneY(m *Model, abs int) int { return abs - m.yOffset + m.chromeHeight }

// messageEntryIdxs returns the cache indices of real message entries,
// skipping separators (msgIdx < 0).
func messageEntryIdxs(m *Model) []int {
	var out []int
	for i, e := range m.cache {
		if e.msgIdx >= 0 {
			out = append(out, i)
		}
	}
	return out
}

// otherMsgIdx returns a message index that is not want, for use as a
// sentinel selection that a spurious hit would visibly overwrite.
func otherMsgIdx(m *Model, want int) int {
	for _, i := range messageEntryIdxs(m) {
		if m.cache[i].msgIdx != want {
			return m.cache[i].msgIdx
		}
	}
	return want
}

// TestClickAtSpacer_FixtureShape pins the layout the other three tests
// reason about: a leading separator, multi-row message entries, a real
// spacer row ending every entry but the last, and no spacer on the last.
// Without this the assertions below could silently degenerate (e.g. every
// entry one row tall, where "last row" and "content row" coincide).
func TestClickAtSpacer_FixtureShape(t *testing.T) {
	m := spacerClickModel(t)

	idxs := messageEntryIdxs(m)
	if len(idxs) < 2 {
		t.Fatalf("want at least 2 message entries, got %d", len(idxs))
	}
	if idxs[len(idxs)-1] != len(m.cache)-1 {
		t.Fatalf("want the last cache entry to be a message entry, got msgIdx %d at %d of %d",
			m.cache[len(m.cache)-1].msgIdx, len(m.cache)-1, len(m.cache))
	}

	for _, i := range idxs {
		e := m.cache[i]
		if e.height < 3 {
			t.Fatalf("entry %d (msgIdx %d): want height >= 3 so content rows and the "+
				"last row are distinct, got %d", i, e.msgIdx, e.height)
		}
		last := e.linesNormal[e.height-1]
		isSpacer := last == m.cacheSpacer
		wantSpacer := i < len(m.cache)-1
		if isSpacer != wantSpacer {
			t.Errorf("entry %d (msgIdx %d): last row spacer = %v, want %v",
				i, e.msgIdx, isSpacer, wantSpacer)
		}
	}
}

// TestClickAtSpacer_ContentRowSelects covers a click on a multi-row
// entry's real content rows: the first row, and the last row that is NOT
// the spacer.
func TestClickAtSpacer_ContentRowSelects(t *testing.T) {
	m := spacerClickModel(t)

	for _, i := range messageEntryIdxs(m) {
		e := m.cache[i]
		off := m.entryOffsets[i]

		// Last row owned by the entry that is real content: one short of
		// the spacer for every entry but the last, which has none.
		lastContent := off + e.height - 1
		if i < len(m.cache)-1 {
			lastContent--
		}

		for _, abs := range []int{off, lastContent} {
			m.selected = otherMsgIdx(m, e.msgIdx)
			y := paneY(m, abs)
			if !m.ClickAt(y) {
				t.Errorf("entry %d (msgIdx %d): ClickAt(y=%d, abs=%d) on a content row = false, want true",
					i, e.msgIdx, y, abs)
				continue
			}
			if m.selected != e.msgIdx {
				t.Errorf("entry %d: ClickAt(y=%d, abs=%d) selected %d, want %d",
					i, y, abs, m.selected, e.msgIdx)
			}
		}
	}
}

// TestClickAtSpacer_SpacerRowDoesNotSelect covers the `hitEnd--`: the
// trailing spacer row of every non-final entry is dead space, so a click
// there misses and leaves the selection alone.
func TestClickAtSpacer_SpacerRowDoesNotSelect(t *testing.T) {
	m := spacerClickModel(t)

	for _, i := range messageEntryIdxs(m) {
		if i == len(m.cache)-1 {
			continue // no trailing spacer; covered by the LastEntry test
		}
		e := m.cache[i]
		spacerAbs := m.entryOffsets[i] + e.height - 1
		if got := e.linesNormal[e.height-1]; got != m.cacheSpacer {
			t.Fatalf("entry %d: abs %d is not the spacer row; fixture drifted", i, spacerAbs)
		}

		sentinel := otherMsgIdx(m, e.msgIdx)
		m.selected = sentinel
		y := paneY(m, spacerAbs)
		if m.ClickAt(y) {
			t.Errorf("entry %d (msgIdx %d): ClickAt(y=%d, abs=%d) on the trailing spacer = true, want false",
				i, e.msgIdx, y, spacerAbs)
		}
		if m.selected != sentinel {
			t.Errorf("entry %d: spacer click changed selection to %d, want %d untouched",
				i, m.selected, sentinel)
		}
	}
}

// TestClickAtSpacer_LastEntryFinalRowIsClickable is the contrast case for
// the `i < len(m.cache)-1` condition. The final entry carries no spacer,
// so its last row is real content and must stay clickable. A test that
// only covered the two cases above would survive a mutation that drops
// the condition and always decrements.
func TestClickAtSpacer_LastEntryFinalRowIsClickable(t *testing.T) {
	m := spacerClickModel(t)

	i := len(m.cache) - 1
	e := m.cache[i]
	if e.msgIdx < 0 {
		t.Fatalf("fixture precondition: last cache entry is a separator (msgIdx %d)", e.msgIdx)
	}
	finalAbs := m.entryOffsets[i] + e.height - 1
	if got := e.linesNormal[e.height-1]; got == m.cacheSpacer {
		t.Fatalf("fixture precondition: last entry unexpectedly has a trailing spacer")
	}

	m.selected = otherMsgIdx(m, e.msgIdx)
	y := paneY(m, finalAbs)
	if !m.ClickAt(y) {
		t.Fatalf("last entry (msgIdx %d): ClickAt(y=%d, abs=%d) on its final row = false, want true "+
			"(the final entry has no trailing spacer to skip)", e.msgIdx, y, finalAbs)
	}
	if m.selected != e.msgIdx {
		t.Errorf("last entry: ClickAt(y=%d) selected %d, want %d", y, m.selected, e.msgIdx)
	}

	// One row past the last entry is off the end of the content: still a miss.
	sentinel := otherMsgIdx(m, e.msgIdx)
	m.selected = sentinel
	pastY := paneY(m, finalAbs+1)
	if m.ClickAt(pastY) {
		t.Errorf("ClickAt(y=%d, abs=%d) past the end of the cache = true, want false", pastY, finalAbs+1)
	}
	if m.selected != sentinel {
		t.Errorf("past-the-end click changed selection to %d, want %d untouched", m.selected, sentinel)
	}
}
