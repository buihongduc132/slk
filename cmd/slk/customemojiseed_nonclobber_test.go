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

// B37: an EMPTY-but-successful emoji.list must not destroy a non-empty cached
// set.
//
// WHAT B37 CLAIMED, AND WHAT IS ACTUALLY THERE. B37 described a "truncated
// success" -- a PARTIAL page overwriting a full cache. That mechanism is
// refuted: emoji.list has no paging. Slack's reference documents exactly two
// arguments (token, include_categories) and no cursor/limit/page; slack-go's
// GetEmojiContext (emoji.go) posts only a token, never loops, and returns
// response.Emoji alone. slk's own seam cannot express a page either --
// customEmojiLister.ListCustomEmoji returns (map[string]string, error), with no
// cursor and no has_more, so "half a list plus a cursor" is unrepresentable by
// construction. There are no pages to lose.
//
// The DESTRUCTIVE HALF of B37 is real, and reachable by a different trigger.
// cache.UpsertCustomEmoji is a wholesale replace, not a merge: it DELETEs the
// team's rows then INSERTs the argument (internal/cache/customemoji.go:47), so
// an empty map empties the team -- deliberate, and pinned by
// TestCustomEmoji_EmptyUpsertClearsTheTeam. Meanwhile
// fetchWorkspaceEmojiIntoCache gates that replace on `err != nil` and nothing
// else. And empty-with-a-NIL-error is a producible state, not a hypothetical:
// Slack documents `{"ok": true}` as a minimal success body, an absent `emoji`
// field decodes to a nil map, GetEmojiContext returns it with a nil error, and
// slk's ListCustomEmoji (internal/slack/client.go:858-860) converts that nil to
// an empty map -- still nil error. So a success carrying no emoji at all walks
// straight through the error gate and clears the team's rows.
//
// The asymmetry is what makes the guard right rather than merely cautious. If
// an admin genuinely deleted every custom emoji, skipping the write leaves one
// stale set until the next fetch -- a few dead shortcodes. If a spurious empty
// success is honoured, the cache is emptied, the next cold start seeds nothing,
// and EVERY custom emoji in the workspace renders as literal `:name:`. A
// workspace that truly has none is unaffected either way: its cache is already
// empty, so the skipped write would have been a no-op.
//
// Scoped to EMPTY on purpose. No "implausibly smaller" ratio heuristic: that
// would guard against the truncation this test's header just refuted, while
// misfiring on a legitimate bulk deletion. Empty is the one case with a
// demonstrated trigger and no false positive.
//
// This mirrors the emptiness check seedCustomEmojiFromCache already makes for
// B17 (customemojiseed.go:38-40). Same trap, same answer, one layer out.
func TestFetchWorkspaceEmoji_EmptySuccessDoesNotClearTheCache(t *testing.T) {
	// nil and empty-non-nil are both worth covering: the fake can return a
	// nil map directly, which is what slack-go hands back for a bodyless
	// success, while the real client normalises it to empty-non-nil first.
	for _, tc := range []struct {
		name   string
		result map[string]string
	}{
		{"empty non-nil map, as slk's client normalises it", map[string]string{}},
		{"nil map, as slack-go returns it for a bodyless ok:true", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wctx := &WorkspaceContext{}
			db := newSeedTestDB(t)
			good := map[string]string{
				"party_parrot": "https://emoji.test/parrot.gif",
				"shipit":       "alias:squirrel",
			}
			if err := db.UpsertCustomEmoji("T1", good); err != nil {
				t.Fatalf("priming the cache: %v", err)
			}
			// A subset is published too, so the in-memory half is observable.
			wctx.SetCustomEmoji(map[string]string{"party_parrot": good["party_parrot"]})

			lister := &fakeEmojiLister{result: tc.result}
			sender := &captureEmojiSender{}

			fetchWorkspaceEmojiIntoCache(context.Background(), wctx, lister, sender, "T1", db)

			if got := lister.callCount(); got != 1 {
				t.Fatalf("ListCustomEmoji calls = %d, want 1; the guard must skip the WRITE, not the fetch", got)
			}

			cached, err := db.CustomEmoji("T1")
			if err != nil {
				t.Fatalf("CustomEmoji(T1): %v", err)
			}
			if len(cached) != len(good) {
				t.Errorf("cache after an empty success = %v (len %d), want the %d-entry set intact; "+
					"an ok:true carrying no emoji emptied the team's rows, so the next cold start "+
					"seeds nothing and every custom emoji renders as literal :name: (B37)",
					cached, len(cached), len(good))
			}
			for k, v := range good {
				if cached[k] != v {
					t.Errorf("cache[%q] = %q, want %q", k, cached[k], v)
				}
			}

			// The same empty response must not clear what is already published,
			// nor tell the UI to repaint with nothing.
			if got := wctx.CustomEmoji()["party_parrot"]; got != good["party_parrot"] {
				t.Errorf("CustomEmoji()[party_parrot] = %q, want %q; an empty success must not "+
					"clear the published set either", got, good["party_parrot"])
			}
			if msgs := sender.snapshot(); len(msgs) != 0 {
				t.Errorf("sent %d messages for an empty success, want 0; there is no new set to "+
					"repaint with, and an empty CustomEmojisLoadedMsg would strip the UI's", len(msgs))
			}
		})
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
