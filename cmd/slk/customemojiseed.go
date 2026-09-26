// cmd/slk/customemojiseed.go
//
// RED-phase stubs for plan slk-fullscreen-emoji-fuzzy items
// emoji-cache-wiring / emoji-seed-order-test / emoji-seed-teamtag
// (gotchas G13, G4, G18). Signatures are FINAL; bodies are
// zero-logic so customemojiseed_test.go fails as assertions, not
// build errors. The GREEN implementation:
//
//   - seedCustomEmojiFromCache reads teamID's set from SQLite
//     (cache.DB.CustomEmoji) and publishes it into wctx BEFORE any
//     network call; it never touches the lister.
//   - fetchWorkspaceEmojiIntoCache is fetchWorkspaceEmoji plus the
//     cache upsert on success (best-effort: a failed list leaves the
//     cache untouched).
//
// Cache seeding stays OUT of ui.NewApp / newTestApp (G18) — it lives
// here, in the composition root.
package main

import (
	"context"

	"github.com/gammons/slk/internal/cache"
)

// seedCustomEmojiFromCache publishes teamID's cached custom emoji set
// into wctx so a cold start renders custom emoji with zero network
// wait. Never calls the lister (G13); a cache miss publishes an
// empty set.
func seedCustomEmojiFromCache(wctx *WorkspaceContext, db *cache.DB, teamID string, _ customEmojiLister) {
	_ = wctx
	_ = db
	_ = teamID
}

// runStartupEmoji is the connect path's emoji pipeline, in order:
// seed the WorkspaceContext from the SQLite cache (no network), then
// run the emoji.list fetch. Stub today — the GREEN body composes
// seedCustomEmojiFromCache + fetchWorkspaceEmojiIntoCache and is what
// main.go's connect path calls.
func runStartupEmoji(ctx context.Context, wctx *WorkspaceContext, db *cache.DB, client customEmojiLister, sender teaSender, teamID string) {
	_ = ctx
	_ = wctx
	_ = db
	_ = client
	_ = sender
	_ = teamID
}

// fetchWorkspaceEmojiIntoCache is fetchWorkspaceEmoji's success path
// plus the cache upsert: emoji.list result -> wctx publication,
// CustomEmojisLoadedMsg send, and db.UpsertCustomEmoji(teamID, set).
// A failed list returns without touching the cache. Stub today.
func fetchWorkspaceEmojiIntoCache(ctx context.Context, wctx *WorkspaceContext, client customEmojiLister, sender teaSender, teamID string, db *cache.DB) {
	_ = ctx
	_ = wctx
	_ = client
	_ = sender
	_ = teamID
	_ = db
}
