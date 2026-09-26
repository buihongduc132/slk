package main

import (
	"context"
	"testing"

	"github.com/gammons/slk/internal/cache"
	"github.com/gammons/slk/internal/ui"
)

// ---------------------------------------------------------------------
// RED tests for the cold-start emoji cache seed and the emoji.list
// refresh path (plan items emoji-cache-wiring, emoji-seed-order-test,
// emoji-seed-teamtag; gotchas G13, G4, G18).
//
// The functions under test do not exist yet; customemojiseed.go
// carries the FINAL signatures as zero-logic stubs so this file
// compiles today and fails as assertions.
//
//   - seedCustomEmojiFromCache(wctx, db, teamID): reads the team's
//     cached set from SQLite and publishes it into wctx BEFORE any
//     network call. Never touches the lister (G13).
//   - fetchWorkspaceEmoji's success path additionally upserts the
//     fetched set into the cache.
//
// Existing semantics pinned by customemoji_test.go
// (bootstrap-subset, error-keeps-subset, team-tagged message, nil
// sender) are deliberately NOT re-tested here; those tests keep
// passing untouched.
// ---------------------------------------------------------------------

func newSeedTestDB(t *testing.T) *cache.DB {
	t.Helper()
	db, err := cache.New(":memory:")
	if err != nil {
		t.Fatalf("cache.New(:memory:) failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// G13: the seed must never hit the network. A lister that IS called
// fails the test — recorded, not assumed.
func TestSeedCustomEmojiFromCache_NeverCallsTheLister(t *testing.T) {
	wctx := &WorkspaceContext{}
	db := newSeedTestDB(t)
	if err := db.UpsertCustomEmoji("T1", map[string]string{
		"cached-parrot": "https://example.test/parrot.gif",
	}); err != nil {
		t.Fatalf("priming the cache: %v", err)
	}

	lister := &fakeEmojiLister{result: map[string]string{"net": "https://example.test/net.png"}}
	seedCustomEmojiFromCache(wctx, db, "T1", lister)

	if got := lister.callCount(); got != 0 {
		t.Errorf("ListCustomEmoji called %d times during the seed, want 0 — the seed reads SQLite only, never the network (G13)", got)
	}
}

// Cold start: customs come out of the cache and land in wctx before
// any fetch runs.
func TestSeedCustomEmojiFromCache_PublishesTheCachedSet(t *testing.T) {
	wctx := &WorkspaceContext{}
	db := newSeedTestDB(t)
	cached := map[string]string{
		"cached-parrot": "https://example.test/parrot.gif",
		"meow":          "alias:cat",
	}
	if err := db.UpsertCustomEmoji("T1", cached); err != nil {
		t.Fatalf("priming the cache: %v", err)
	}

	seedCustomEmojiFromCache(wctx, db, "T1", &fakeEmojiLister{})

	got := wctx.CustomEmoji()
	if len(got) != len(cached) {
		t.Fatalf("CustomEmoji() after seed = %v (len %d), want the cached set %v (len %d)", got, len(got), cached, len(cached))
	}
	for k, v := range cached {
		if got[k] != v {
			t.Errorf("CustomEmoji()[%q] = %q, want %q", k, got[k], v)
		}
	}
}

// A cache miss is an empty publication, not an error and not a
// stale-map leak: the zero-value map must stay empty-non-nil.
func TestSeedCustomEmojiFromCache_MissPublishesEmpty(t *testing.T) {
	wctx := &WorkspaceContext{}
	db := newSeedTestDB(t)

	seedCustomEmojiFromCache(wctx, db, "T_UNKNOWN", &fakeEmojiLister{})

	got := wctx.CustomEmoji()
	if got == nil || len(got) != 0 {
		t.Errorf("CustomEmoji() after a cache-miss seed = %v, want an initialized empty map", got)
	}
}

// G13: recorded call order at startup must be seed-from-cache FIRST,
// fetchWorkspaceEmoji SECOND. The probe is the lister itself: it
// snapshots wctx.CustomEmoji() at the moment emoji.list is called.
// If the seed ran first, that snapshot contains the CACHED set; if
// the fetch ran first (or no seed ran at all), it does not.
// runStartupEmoji is the composition seam (stub today; the GREEN
// body wires seed -> fetch the way the connect path calls them).
type orderProbingLister struct {
	wctx *WorkspaceContext

	called         bool
	snapshotAtCall map[string]string
	result         map[string]string
}

func (l *orderProbingLister) ListCustomEmoji(_ context.Context) (map[string]string, error) {
	l.called = true
	l.snapshotAtCall = l.wctx.CustomEmoji()
	return l.result, nil
}

func TestStartupEmojiOrder_SeedBeforeFetch(t *testing.T) {
	wctx := &WorkspaceContext{}
	db := newSeedTestDB(t)
	cached := map[string]string{"cached-parrot": "https://example.test/parrot.gif"}
	if err := db.UpsertCustomEmoji("T1", cached); err != nil {
		t.Fatalf("priming the cache: %v", err)
	}
	lister := &orderProbingLister{wctx: wctx, result: map[string]string{"net": "https://example.test/net.png"}}

	runStartupEmoji(context.Background(), wctx, db, lister, &captureEmojiSender{}, "T1")

	if !lister.called {
		t.Fatal("the startup pipeline never called emoji.list; the fetch half is missing")
	}
	if got := lister.snapshotAtCall["cached-parrot"]; got == "" {
		t.Errorf("wctx.CustomEmoji() at the moment of the FIRST lister call = %v; it must already hold the cached set — the seed must run BEFORE the fetch (G13)", lister.snapshotAtCall)
	}
	// And the fetch's fresher set must win afterwards (the seed is a
	// head start, not a veto).
	if got := wctx.CustomEmoji()["net"]; got == "" {
		t.Errorf("wctx.CustomEmoji() after the pipeline = %v; the fetched set must be published", wctx.CustomEmoji())
	}
}

// The pipeline fetches exactly once (the seed adds no network call).
func TestStartupEmojiPipeline_SingleListerCall(t *testing.T) {
	wctx := &WorkspaceContext{}
	db := newSeedTestDB(t)
	lister := &fakeEmojiLister{result: map[string]string{"a": "b"}}

	runStartupEmoji(context.Background(), wctx, db, lister, &captureEmojiSender{}, "T1")

	if got := lister.callCount(); got != 1 {
		t.Errorf("ListCustomEmoji calls across the startup pipeline = %d, want exactly 1 (the seed must not fetch)", got)
	}
}

// The pipeline still publishes CustomEmojisLoadedMsg for its team once
// the fetch lands (emoji-cache-wiring: upsert + re-publish).
func TestStartupEmojiPipeline_PublishesLoadedMsg(t *testing.T) {
	wctx := &WorkspaceContext{}
	db := newSeedTestDB(t)
	lister := &fakeEmojiLister{result: map[string]string{"party-parrot": "https://example.test/parrot.gif"}}
	sender := &captureEmojiSender{}

	runStartupEmoji(context.Background(), wctx, db, lister, sender, "T9")

	msgs := sender.snapshot()
	found := false
	for _, m := range msgs {
		if loaded, ok := m.(ui.CustomEmojisLoadedMsg); ok && loaded.TeamID == "T9" && loaded.CustomEmoji["party-parrot"] != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("the startup pipeline sent %d messages; none is a team-tagged CustomEmojisLoadedMsg carrying the fetched set", len(msgs))
	}
	// And the cache holds the fetched set for the next cold start.
	got, err := db.CustomEmoji("T9")
	if err != nil {
		t.Fatalf("CustomEmoji(T9): %v", err)
	}
	if got["party-parrot"] == "" {
		t.Errorf("cache after the pipeline = %v; the fetch must upsert", got)
	}
}

// emoji.list success must UPSERT into the cache so the next cold
// start has the fresh set (emoji-cache-wiring).
func TestFetchWorkspaceEmoji_UpsertsIntoCache(t *testing.T) {
	wctx := &WorkspaceContext{}
	db := newSeedTestDB(t)
	lister := &fakeEmojiLister{result: map[string]string{
		"party-parrot": "https://example.test/parrot.gif",
	}}

	fetchWorkspaceEmojiIntoCache(context.Background(), wctx, lister, &captureEmojiSender{}, "T1", db)

	got, err := db.CustomEmoji("T1")
	if err != nil {
		t.Fatalf("CustomEmoji(T1) after fetch: %v", err)
	}
	if got["party-parrot"] != "https://example.test/parrot.gif" {
		t.Errorf("cache after fetch = %v; the fetched set must be upserted so the next cold start uses it", got)
	}
}

// A FAILED emoji.list must leave the cache untouched (best-effort,
// same semantics as the in-memory publication: subset-or-built-ins
// stay in place).
func TestFetchWorkspaceEmoji_ErrorLeavesCacheUntouched(t *testing.T) {
	wctx := &WorkspaceContext{}
	db := newSeedTestDB(t)
	if err := db.UpsertCustomEmoji("T1", map[string]string{"stale": "https://example.test/stale.png"}); err != nil {
		t.Fatalf("priming the cache: %v", err)
	}
	lister := &fakeEmojiLister{err: errFakeList}

	fetchWorkspaceEmojiIntoCache(context.Background(), wctx, lister, &captureEmojiSender{}, "T1", db)

	got, err := db.CustomEmoji("T1")
	if err != nil {
		t.Fatalf("CustomEmoji(T1): %v", err)
	}
	if got["stale"] == "" {
		t.Errorf("cache was disturbed by a failed fetch: %v; best-effort means untouched", got)
	}
}

// G4/G19: two workspaces primed in one cache; each team's seed reads
// ONLY its own set. Workspace B must never render A's customs from a
// stale seed, no matter which order they connect in.
func TestSeedCustomEmojiFromCache_TwoTeamIsolation(t *testing.T) {
	db := newSeedTestDB(t)
	if err := db.UpsertCustomEmoji("TA", map[string]string{"a-emoji": "https://example.test/a.png"}); err != nil {
		t.Fatalf("priming TA: %v", err)
	}
	if err := db.UpsertCustomEmoji("TB", map[string]string{"b-emoji": "https://example.test/b.png"}); err != nil {
		t.Fatalf("priming TB: %v", err)
	}

	lister := &fakeEmojiLister{} // never called; see the G13 test
	for _, team := range []struct{ id, want string }{
		{"TA", "a-emoji"},
		{"TB", "b-emoji"},
	} {
		wctx := &WorkspaceContext{}
		seedCustomEmojiFromCache(wctx, db, team.id, lister)
		got := wctx.CustomEmoji()
		if got[team.want] == "" {
			t.Errorf("seed for %s did not publish %q; set = %v", team.id, team.want, got)
		}
		for _, leaked := range []string{"a-emoji", "b-emoji"} {
			if leaked != team.want && got[leaked] != "" {
				t.Errorf("seed for %s leaked the other team's emoji %q; set = %v (G4: team-untagged seed)", team.id, leaked, got)
			}
		}
	}
}

// The seed's publication is TeamID-tagged through WorkspaceContext
// per-team state; a UI-level re-read on WorkspaceSwitchedMsg is the
// App-side half of G4 and is pinned in internal/ui (the switch msg
// carries CustomEmoji from wctx). Here we pin the cmd/slk half: the
// switch path reads the ACTIVE team's context, whose customs were
// seeded from that team's cache rows. With the seed present, both
// teams' WorkspaceSwitchedMsgs carry distinct sets.
func TestSeedFeedsSwitchMsg_PerActiveTeam(t *testing.T) {
	db := newSeedTestDB(t)
	if err := db.UpsertCustomEmoji("TA", map[string]string{"a-emoji": "https://example.test/a.png"}); err != nil {
		t.Fatalf("priming TA: %v", err)
	}
	if err := db.UpsertCustomEmoji("TB", map[string]string{"b-emoji": "https://example.test/b.png"}); err != nil {
		t.Fatalf("priming TB: %v", err)
	}
	lister := &fakeEmojiLister{}

	build := func(teamID string) ui.WorkspaceSwitchedMsg {
		wctx := &WorkspaceContext{TeamID: teamID}
		seedCustomEmojiFromCache(wctx, db, teamID, lister)
		return ui.WorkspaceSwitchedMsg{TeamID: teamID, CustomEmoji: wctx.CustomEmoji()}
	}

	msgA := build("TA")
	msgB := build("TB")

	if msgA.CustomEmoji["a-emoji"] == "" {
		t.Errorf("switch msg for TA does not carry TA's customs: %v", msgA.CustomEmoji)
	}
	if msgB.CustomEmoji["a-emoji"] != "" {
		t.Errorf("switch msg for TB carries TA's customs: %v — workspace B must never render A's (G4)", msgB.CustomEmoji)
	}
	if msgB.CustomEmoji["b-emoji"] == "" {
		t.Errorf("switch msg for TB does not carry TB's customs: %v", msgB.CustomEmoji)
	}
}

// errFakeList keeps the error test free of an errors import cycle
// with the fixtures above.
var errFakeList = fakeListError{}

type fakeListError struct{}

func (fakeListError) Error() string { return "ratelimited" }
