package main

import (
	"os"
	"strings"
	"testing"
)

// B18: the emoji cache seed must be SYNCHRONOUS and must precede the
// WorkspaceReadyMsg send; the fetch must be a goroutine AFTER it.
//
// This is a source-structural oracle, in the same spirit as the toast
// consolidation oracle: the property is a property of the CALL SITE, and no
// behavioural test of the two functions can pin it. A test that calls seed then
// fetch itself merely imposes the order it then asserts — which is exactly how
// the original slipped through. TestStartupEmojiOrder_SeedBeforeFetch had a real
// probe and still could not see this, because it called a production helper that
// ran both halves synchronously while production ran that helper under `go`.
//
// So this reads main.go and asserts the three statements' relative positions.
// It fails if someone moves the seed back into a goroutine, moves it after the
// message send, or drops the `go` from the fetch and blocks first paint.
//
// Reading source in a test is unusual and worth the cost here: the alternative
// is restructuring the whole connect block to be callable from a test, which
// would be a much larger change than the defect warrants.
func TestStartupEmojiCallSite_SeedBeforeReadyMsg(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading main.go: %v", err)
	}
	s := string(src)

	const (
		seed  = "seedCustomEmojiFromCache(wctx, db, wctx.TeamID)"
		ready = "p.Send(ui.WorkspaceReadyMsg{"
		fetch = "go fetchWorkspaceEmojiIntoCache(ctx, wctx, wctx.Client, p, wctx.TeamID, db)"
	)

	iSeed := strings.Index(s, seed)
	iReady := strings.Index(s, ready)
	iFetch := strings.Index(s, fetch)

	// V2/V3: prove each anchor matched before comparing positions. A missing
	// anchor must fail loudly, not silently satisfy an ordering comparison —
	// strings.Index returns -1, and -1 < anything is trivially "in order".
	if iSeed < 0 {
		t.Fatalf("no synchronous seed call found in main.go (looked for %q). If it was renamed, update this test; if it moved into a goroutine, that is B18 regressing.", seed)
	}
	if iReady < 0 {
		t.Fatalf("no WorkspaceReadyMsg send found in main.go (looked for %q)", ready)
	}
	if iFetch < 0 {
		t.Fatalf("no backgrounded emoji fetch found in main.go (looked for %q). The fetch must stay a goroutine after the message send, or it blocks first paint.", fetch)
	}

	if iSeed > iReady {
		t.Errorf("the emoji cache seed appears AFTER the WorkspaceReadyMsg send (seed at byte %d, send at %d). The message reads wctx.CustomEmoji(), so the seed must precede it or first paint is nondeterministic (B18).", iSeed, iReady)
	}
	if iFetch < iReady {
		t.Errorf("the emoji fetch appears BEFORE the WorkspaceReadyMsg send (fetch at byte %d, send at %d). It makes a network round trip; it must not gate first paint.", iFetch, iReady)
	}

	// The seed must not be under `go`. Checked by looking at what precedes it
	// on its own line, so an added `go ` is caught even though the call text is
	// otherwise identical.
	lineStart := strings.LastIndex(s[:iSeed], "\n") + 1
	if prefix := strings.TrimSpace(s[lineStart:iSeed]); prefix != "" {
		t.Errorf("the seed call is prefixed with %q; it must be a plain synchronous statement. Under `go` the message send races it, and because wctx.customEmoji is an atomic.Pointer that race is a VALUE race -- invisible to -race, with first paint varying run to run (B18).", prefix)
	}
}
