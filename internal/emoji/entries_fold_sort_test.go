package emoji

import (
	"testing"

	// Aliased: this package's tests declare a helper named `text`
	// (tokens_test.go), which shadows the bare package name. Follows the
	// repo's `slk`-prefix aliasing convention (cf. slkemoji in
	// internal/ui/reactionpicker).
	slktext "github.com/gammons/slk/internal/text"
)

// Slack custom emoji names are user-supplied and mixed case does occur.
// A raw byte sort puts every uppercase ASCII letter ahead of every
// lowercase one, so :Rocket: lands before :apple: instead of beside
// :rocket:. BuildEntries' doc comment promises "sorted alphabetically by
// name", and internal/ui/emojipicker's filter() consumes that order
// directly (empty query takes the first MaxVisible; a non-empty query
// tie-breaks on input index), so the defect is visible in the dropdown.
//
// These tests pin the fold-based order. The tie-break is a raw-Name
// comparison rather than sort.SliceStable: BuildEntries fills `out` by
// ranging over a map, so "input order" is already randomized and
// stability with respect to it would still be nondeterministic. Name is
// a map key, hence unique, so (fold(Name), Name) is a strict total order
// and the output is reproducible. TestBuildEntries_FoldSortIsDeterministic
// below pins that.

// namesInFixtureOrder returns the names of entries whose name is in want,
// in output order. Restricting to the fixture set keeps built-ins that
// fold-collide with a fixture name (e.g. the built-in :banana: against a
// custom :Banana:) from polluting the assertion.
func namesInFixtureOrder(entries []EmojiEntry, want map[string]bool) []string {
	var got []string
	for _, e := range entries {
		if want[e.Name] {
			got = append(got, e.Name)
		}
	}
	return got
}

func TestBuildEntries_MixedCaseCustomsSortCaseInsensitively(t *testing.T) {
	const url = "https://emoji.slack-edge.com/T1/x/abc.gif"
	customs := map[string]string{
		"Rocket": url,
		"rocket": url,
		"apple":  url,
		"Banana": url,
		"zebra":  url,
	}
	set := make(map[string]bool, len(customs))
	for n := range customs {
		set[n] = true
	}

	got := namesInFixtureOrder(BuildEntries(customs), set)
	// Fold keys: apple, banana, rocket, rocket, zebra. The Rocket/rocket
	// tie breaks on raw Name, and "Rocket" < "rocket" bytewise.
	want := []string{"apple", "Banana", "Rocket", "rocket", "zebra"}

	if len(got) != len(want) {
		t.Fatalf("got %d fixture entries %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("case-insensitive order wrong:\n got %v\nwant %v", got, want)
		}
	}
}

func TestBuildEntries_WholeListIsFoldSorted(t *testing.T) {
	const url = "https://emoji.slack-edge.com/T1/x/abc.gif"
	customs := map[string]string{
		"Rocket":     url,
		"Banana":     url,
		"ZZTop":      url,
		"aardvark":   url,
		"MixedCase":  url,
		"mixedcase":  url,
		"UPPERCASE":  url,
		"lowercase2": url,
	}

	entries := BuildEntries(customs)
	for i := 1; i < len(entries); i++ {
		prev, cur := entries[i-1].Name, entries[i].Name
		pf, cf := slktext.Fold(prev), slktext.Fold(cur)
		if pf > cf {
			t.Fatalf("not fold-sorted at %d: %q (fold %q) before %q (fold %q)",
				i, prev, pf, cur, cf)
		}
		if pf == cf && prev >= cur {
			t.Fatalf("fold tie at %d not broken by raw name: %q before %q", i, prev, cur)
		}
	}
}

// text.Fold strips diacritics as well as case (Fold("Über") == "uber"),
// so an accented custom name sorts with its unaccented base rather than
// after every ASCII name — which is where a raw byte sort puts it, since
// the leading rune encodes as 0xC3.
func TestBuildEntries_AccentedCustomSortsWithItsBase(t *testing.T) {
	const url = "https://emoji.slack-edge.com/T1/x/abc.gif"
	customs := map[string]string{
		"Über":  url,
		"uber":  url,
		"zebra": url,
	}
	set := map[string]bool{"Über": true, "uber": true, "zebra": true}

	got := namesInFixtureOrder(BuildEntries(customs), set)
	// Fold keys: uber, uber, zebra. The uber/Über tie breaks on raw Name,
	// and "uber" < "Über" bytewise: Ü is U+00DC, which UTF-8-encodes as
	// 0xC3 0x9C, so it sorts after ASCII "u" (0x75). (Not 0x55 — the
	// leading byte is not an ASCII "U".)
	//
	// The load-bearing assertion is the adjacency: Über sits next to uber
	// rather than after zebra, which is where the raw byte sort put it.
	want := []string{"uber", "Über", "zebra"}

	if len(got) != len(want) {
		t.Fatalf("got %d fixture entries %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("accented name misplaced:\n got %v\nwant %v", got, want)
		}
	}
}

// Regression guard for the tie-break choice, not for the fold itself: a
// fold-only comparator would leave fold-equal names ordered by map range,
// which differs run to run. This passes under the old raw sort too.
func TestBuildEntries_FoldSortIsDeterministic(t *testing.T) {
	const url = "https://emoji.slack-edge.com/T1/x/abc.gif"
	customs := map[string]string{}
	// Many fold-colliding pairs, so a nondeterministic tie-break is
	// overwhelmingly likely to show up across a handful of passes.
	for _, n := range []string{
		"Alpha", "alpha", "Bravo", "bravo", "Charlie", "charlie",
		"Delta", "delta", "Echo", "echo", "Foxtrot", "foxtrot",
	} {
		customs[n] = url
	}

	first := BuildEntries(customs)
	for pass := 0; pass < 8; pass++ {
		got := BuildEntries(customs)
		if len(got) != len(first) {
			t.Fatalf("pass %d: length %d, want %d", pass, len(got), len(first))
		}
		for i := range first {
			if got[i].Name != first[i].Name {
				t.Fatalf("pass %d: entry %d = %q, first pass had %q",
					pass, i, got[i].Name, first[i].Name)
			}
		}
	}
}
