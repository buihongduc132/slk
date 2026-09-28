package main

import (
	"context"
	"testing"
)

// B17: the cache seed must not destroy the bootstrap emoji subset.
//
// connect.go publishes the conversations.view subset as a deliberate head
// start ("published as a head start, not an answer"), so the first channel's
// emoji resolve without waiting on a round trip. The seed then runs
// unconditionally. On a cache MISS, cache.CustomEmoji returns an empty map
// with a NIL error -- so the seed's `err == nil && set != nil` branch is taken
// with an empty map and overwrites the subset. If emoji.list then fails,
// fetchWorkspaceEmojiIntoCache returns early without publishing and the
// workspace renders every custom emoji as literal `:name:`, including in the
// channel that rendered correctly a moment earlier.
//
// That is exactly the regression the comment on the fetch exists to prevent:
// the bootstrap subset must not be treated as an answer, but it must also not
// be treated as garbage.
//
// NOTE on the sibling test: TestSeedCustomEmojiFromCache_MissPublishesEmpty
// asserts a non-nil empty map after a miss, and it keeps passing under the fix
// -- WorkspaceContext.CustomEmoji() returns map[string]string{} when the
// pointer is nil (workspace.go:177-182), so "publish nothing" and "publish
// empty" are indistinguishable through the accessor when there is no prior
// value. It only looks like it pins the clobber; it does not, because its
// fixture has no prior subset to clobber. That is precisely why this test
// needs a primed wctx.
func TestSeedCustomEmojiFromCache_MissKeepsBootstrapSubset(t *testing.T) {
	wctx := &WorkspaceContext{}
	subset := map[string]string{"party_parrot": "https://emoji.test/parrot.gif"}
	wctx.SetCustomEmoji(subset)

	db := newSeedTestDB(t)

	// T_UNKNOWN has no cached rows: a MISS.
	seedCustomEmojiFromCache(wctx, db, "T_UNKNOWN")

	got := wctx.CustomEmoji()
	if len(got) == 0 {
		t.Fatal("the seed cleared the bootstrap subset on a cache miss; a " +
			"failed emoji.list now leaves every custom emoji rendering as " +
			"literal :name: (B17)")
	}
	if got["party_parrot"] != subset["party_parrot"] {
		t.Errorf("party_parrot = %q, want %q; the bootstrap subset must survive a cache miss",
			got["party_parrot"], subset["party_parrot"])
	}
}

// The end-to-end shape of B17: bootstrap subset published, cache empty, and
// emoji.list fails. The user must still see their custom emoji.
func TestStartupEmoji_MissAndFetchFailureKeepsBootstrapSubset(t *testing.T) {
	wctx := &WorkspaceContext{}
	subset := map[string]string{"shipit": "https://emoji.test/shipit.png"}
	wctx.SetCustomEmoji(subset)

	db := newSeedTestDB(t)
	lister := &fakeEmojiLister{err: context.DeadlineExceeded}

	runStartupEmoji(context.Background(), wctx, db, lister, nil, "T_UNKNOWN")

	got := wctx.CustomEmoji()
	if got["shipit"] != subset["shipit"] {
		t.Errorf("shipit = %q, want %q -- cold start + cache miss + failed "+
			"emoji.list must not strip the bootstrap subset (B17)",
			got["shipit"], subset["shipit"])
	}
}
