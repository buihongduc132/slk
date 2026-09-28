package peerstatus

import (
	"strings"
	"testing"
	"time"

	"github.com/gammons/slk/internal/emoji"
)

var testNow = time.Unix(1700000000, 0)

func TestGlyph_ResolvesShortcodesAndFallsBack(t *testing.T) {
	calendar := emoji.CodeMap()[":calendar:"]
	thumbs := emoji.CodeMap()[":+1:"]
	if calendar == "" || thumbs == "" {
		t.Fatal("test fixtures missing from the emoji code map")
	}
	cases := []struct {
		name string
		st   Status
		want string
	}{
		{"standard shortcode", Status{Emoji: ":calendar:"}, calendar},
		{"skin tone is dropped", Status{Emoji: ":+1::skin-tone-3:"}, thumbs},
		{"custom workspace emoji", Status{Emoji: ":company-logo-not-unicode:"}, FallbackGlyph},
		{"text without emoji", Status{Text: "Heads down"}, FallbackGlyph},
		{"nothing set", Status{}, ""},
		{"expired", Status{Emoji: ":calendar:", Expires: testNow}, ""},
		{"expires later", Status{Emoji: ":calendar:", Expires: testNow.Add(time.Minute)}, calendar},
	}
	for _, c := range cases {
		if got := c.st.Glyph(testNow); got != c.want {
			t.Errorf("%s: Glyph = %q; want %q", c.name, got, c.want)
		}
	}
}

func TestInDND(t *testing.T) {
	if (Status{DND: true}).InDND(testNow) != true {
		t.Error("DND with unknown end must count as in DND")
	}
	if (Status{DND: true, DNDEnd: testNow}).InDND(testNow) {
		t.Error("DND whose end is now must not count as in DND")
	}
	if (Status{DNDEnd: testNow.Add(time.Hour)}).InDND(testNow) {
		t.Error("an end time without DND on must not count as in DND")
	}
}

func TestSummary(t *testing.T) {
	end := testNow.Add(time.Hour)
	st := Status{Emoji: ":calendar:", Text: "In a meeting", DND: true, DNDEnd: end}
	want := emoji.CodeMap()[":calendar:"] + " In a meeting · " + DNDGlyph + " Do not disturb until " + end.In(testNow.Location()).Format("15:04")
	if got := st.Summary(testNow, "15:04"); got != want {
		t.Errorf("Summary = %q; want %q", got, want)
	}
	if got := (Status{}).Summary(testNow, "15:04"); got != "" {
		t.Errorf("empty Summary = %q; want empty", got)
	}
	if got := (Status{DND: true}).Summary(testNow, "15:04"); got != DNDGlyph+" Do not disturb" {
		t.Errorf("DND without end Summary = %q", got)
	}
}

func TestExpiredAndClear(t *testing.T) {
	st := Status{
		Emoji: ":calendar:", Text: "In a meeting", Expires: testNow.Add(-time.Second),
		DND: true, DNDEnd: testNow.Add(time.Hour),
	}
	if !st.Expired(testNow) {
		t.Fatal("a passed status expiry must report Expired")
	}
	cleared := st.Clear(testNow)
	if cleared.Emoji != "" || cleared.Text != "" || !cleared.Expires.IsZero() {
		t.Errorf("Clear kept the expired status: %+v", cleared)
	}
	if !cleared.DND || !cleared.DNDEnd.Equal(st.DNDEnd) {
		t.Errorf("Clear dropped DND that has not ended: %+v", cleared)
	}
	if cleared.Expired(testNow) {
		t.Error("a cleared status must not still report Expired")
	}
	if (Status{Emoji: ":calendar:"}).Expired(testNow) {
		t.Error("a status that never expires must not report Expired")
	}
}

func TestWithDNDOffClearsEnd(t *testing.T) {
	st := Status{}.WithDND(true, testNow).WithDND(false, testNow)
	if st.DND || !st.DNDEnd.IsZero() {
		t.Errorf("WithDND(false) = %+v; want DND off with no end", st)
	}
}

// TestSummary_FormatsDNDEndInTheClocksZone pins that Summary formats DNDEnd in
// the zone of the `now` it is handed, not in the process-global time.Local.
//
// WHY THIS IS NOT ASSERTED BY COMPARING AGAINST A ZONE CONSTANT. Two earlier
// attempts at this assertion were vacuous, both for the same reason, and the
// second one was mine:
//
//   - `want: end.Local()` — matches production's own `.Local()` call, so it
//     agrees with the bug it is supposed to catch.
//   - `want: end.In(testNow.Location())` — testNow is time.Unix(...), and
//     time.Unix returns a Time in the LOCAL zone, so testNow.Location() IS
//     time.Local and the expectation collapses to the first case.
//   - `testNow` re-based onto FixedZone("TESTZONE", 7*3600) — this machine's
//     local offset is +0700, so the formatted strings were byte-identical.
//     Measured: reverting production to .Local() still passed.
//
// Any assertion naming one zone can accidentally name the ambient one. So this
// compares two EXPLICIT zones against EACH OTHER and never against time.Local:
// the same instant, handed to Summary as two clocks 9 hours apart, must format
// two different wall-clock strings. That holds on every machine regardless of
// TZ, and it fails whenever Summary ignores its argument's zone -- which is
// exactly what `.Local()` does.
func TestSummary_FormatsDNDEndInTheClocksZone(t *testing.T) {
	const layout = "15:04"
	zoneA := time.FixedZone("ZONE_A", -5*3600)
	zoneB := time.FixedZone("ZONE_B", 4*3600)

	instant := time.Unix(1700000000, 0)
	end := instant.Add(time.Hour)
	st := Status{DND: true, DNDEnd: end}

	gotA := st.Summary(instant.In(zoneA), layout)
	gotB := st.Summary(instant.In(zoneB), layout)

	if gotA == gotB {
		t.Fatalf("Summary ignored its clock's zone: both zones rendered %q. "+
			"DNDEnd must be formatted in the zone of the `now` argument, so two "+
			"clocks 9h apart cannot agree. A .Local() call here would read the "+
			"process-global zone and produce exactly this equality.", gotA)
	}

	// And each must be the correct wall clock for its own zone, so the test
	// fails on a wrong zone rather than merely on any difference.
	if want := end.In(zoneA).Format(layout); !strings.Contains(gotA, want) {
		t.Errorf("zone A: Summary = %q, want it to contain %q", gotA, want)
	}
	if want := end.In(zoneB).Format(layout); !strings.Contains(gotB, want) {
		t.Errorf("zone B: Summary = %q, want it to contain %q", gotB, want)
	}
}
