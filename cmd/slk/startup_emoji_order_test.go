package main

// B40: "cold start renders custom emoji from SQLite with zero network wait" is a
// TIMING claim, and until this file nothing asserted the timing.
//
// ---------------------------------------------------------------------------
// WHAT THE EXISTING TEST ACTUALLY COVERS, AND WHY IT IS LESS THAN IT LOOKS
//
// TestStartupEmojiOrder_SeedBeforeFetch (customemojiseed_test.go:134) calls
// runStartupEmoji, which is declared at customemojiseed_test.go:111 -- IN A TEST
// FILE. It is:
//
//	func runStartupEmoji(...) {
//	        seedCustomEmojiFromCache(wctx, db, teamID)
//	        fetchWorkspaceEmojiIntoCache(ctx, wctx, client, sender, teamID, db)
//	}
//
// So the sequence whose order that test pins is a sequence THE TEST WROTE. It
// proves the two functions behave correctly when composed that way; it cannot
// prove production composes them that way, because production does not: it runs
// the fetch in a GOROUTINE, after a p.Send, which this helper does neither of.
//
// That is a distinct vacuity shape from the ones OT27 lists, and worth naming:
// not "the test recomputes the value" but "the test AUTHORS ITS OWN SUBJECT".
// A production change to the startup order leaves it green.
//
// ---------------------------------------------------------------------------
// THE THREE PROPERTIES THAT ACTUALLY DELIVER "ZERO NETWORK WAIT"
//
// At cmd/slk/main.go (workspace-ready block), in this order:
//
//	1. seedCustomEmojiFromCache(...)          -- synchronous, so the set exists
//	2. p.Send(ui.WorkspaceReadyMsg{...})      -- first paint's data, reads
//	                                             wctx.CustomEmoji()
//	3. go fetchWorkspaceEmojiIntoCache(...)   -- async AND after the send
//
// Ordering alone is not enough, which is exactly the gap B40 describes: a
// seed-before-fetch that still AWAITED the fetch before the send would satisfy
// every existing test and reintroduce a network-blocked cold start. All three
// properties are load-bearing:
//
//	drop the `go`         -> first paint waits on emoji.list
//	move the fetch up      -> ditto, even with `go`, if it were awaited
//	move the seed down     -> WorkspaceReadyMsg carries an empty set
//
// ---------------------------------------------------------------------------
// WHY THIS TEST IS STRUCTURAL
//
// The sequence lives inline inside a large function in main.go that builds a
// live Slack client, a SQLite handle and a tea.Program. There is no seam to
// inject a blocking lister into without refactoring startup, and refactoring the
// startup path to make this assertable is a real change with real risk -- see the
// Open Thread this lands with.
//
// Between "no assertion at all" and "an assertion about program text", the
// second is worth having: the property IS structural (a `go` keyword and a
// statement order), and the alternative has already let the claim go unchecked
// through an entire plan. It is written to fail loudly and explain itself rather
// than to be clever.
//
// PROVEN BY MUTATION, not merely written: with `go ` deleted from the fetch call
// this test fails; with the seed moved after the p.Send it fails; and
// TestStartupEmojiOrder_SeedBeforeFetch stays GREEN under both. That contrast is
// the entire justification for this file existing.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestStartupEmoji_ProductionOrderIsSeedSendThenAsyncFetch reads main.go and
// pins the three properties above.
func TestStartupEmoji_ProductionOrderIsSeedSendThenAsyncFetch(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading main.go: %v", err)
	}
	lines := strings.Split(string(src), "\n")

	// Comments are stripped before matching: main.go discusses all three of
	// these calls in prose around them (B18's note, and the "replaced by the
	// goroutine below" aside), and prose must not be able to satisfy or break
	// this test.
	code := make([]string, len(lines))
	for i, l := range lines {
		if idx := strings.Index(l, "//"); idx >= 0 {
			l = l[:idx]
		}
		code[i] = l
	}

	find := func(re string) []int {
		rx := regexp.MustCompile(re)
		var hits []int
		for i, l := range code {
			if rx.MatchString(l) {
				hits = append(hits, i+1)
			}
		}
		return hits
	}

	seeds := find(`\bseedCustomEmojiFromCache\(`)
	sends := find(`p\.Send\(ui\.WorkspaceReadyMsg\{`)
	fetches := find(`\bfetchWorkspaceEmojiIntoCache\(`)

	if len(seeds) != 1 || len(sends) != 1 || len(fetches) != 1 {
		t.Fatalf("expected exactly one call site each in main.go, got "+
			"seed=%v send=%v fetch=%v\n"+
			"  If startup grew a second path, this test needs to know about it --\n"+
			"  that is the point of failing here rather than silently checking the\n"+
			"  first one.", seeds, sends, fetches)
	}
	seed, send, fetch := seeds[0], sends[0], fetches[0]

	// Property 1: the seed precedes the send, so WorkspaceReadyMsg's
	// CustomEmoji field is populated rather than empty.
	if seed >= send {
		t.Errorf("seedCustomEmojiFromCache is at line %d but WorkspaceReadyMsg is sent at %d.\n"+
			"  The seed must come FIRST: the message carries wctx.CustomEmoji(), so a\n"+
			"  seed after the send makes the first paint render with an empty custom\n"+
			"  emoji set. (B18 also records that seeding in a goroutine made this an\n"+
			"  atomic.Pointer value race that -race cannot see.)", seed, send)
	}

	// Property 2: the fetch follows the send, so nothing about the network call
	// can order itself ahead of first paint.
	if fetch <= send {
		t.Errorf("fetchWorkspaceEmojiIntoCache is at line %d, at or before the "+
			"WorkspaceReadyMsg send at %d.\n"+
			"  The fetch must come AFTER the send. dod-2 claims cold start renders "+
			"with ZERO NETWORK WAIT; a fetch ahead of the send puts emoji.list on the "+
			"first-paint path.", fetch, send)
	}

	// Property 3: and it is launched asynchronously. This is the one that a
	// seed-before-fetch ordering test cannot see, and the one B40 is about.
	if !regexp.MustCompile(`\bgo\s+fetchWorkspaceEmojiIntoCache\(`).MatchString(code[fetch-1]) {
		t.Errorf("line %d calls fetchWorkspaceEmojiIntoCache WITHOUT the `go` keyword:\n"+
			"    %s\n"+
			"  A synchronous fetch here blocks startup on emoji.list even though it is\n"+
			"  correctly ordered after the send, which is exactly the regression dod-2\n"+
			"  exists to prevent and which every ordering-only test permits.",
			fetch, strings.TrimSpace(lines[fetch-1]))
	}
}

// TestStartupEmoji_SeedIsSynchronousAtItsCallSite pins the narrower half of B18
// that the ordering test also cannot see: the seed must NOT be backgrounded.
//
// Separate from the test above so a failure names one cause. If someone
// "optimises" startup by wrapping the seed in a goroutine, the seed/send line
// ordering still holds and only this test objects.
func TestStartupEmoji_SeedIsSynchronousAtItsCallSite(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading main.go: %v", err)
	}
	for i, l := range strings.Split(string(src), "\n") {
		if idx := strings.Index(l, "//"); idx >= 0 {
			l = l[:idx]
		}
		if !strings.Contains(l, "seedCustomEmojiFromCache(") {
			continue
		}
		if regexp.MustCompile(`\bgo\s+seedCustomEmojiFromCache\(`).MatchString(l) {
			t.Errorf("line %d launches the cache seed in a goroutine:\n    %s\n"+
				"  It must be synchronous. WorkspaceReadyMsg reads wctx.CustomEmoji()\n"+
				"  immediately after, so a backgrounded seed makes the first paint depend\n"+
				"  on a scheduling coin flip -- an atomic.Pointer VALUE race, invisible to\n"+
				"  -race, with output that varies run to run (B18).",
				i+1, strings.TrimSpace(l))
		}
	}
}
