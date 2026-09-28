package main

import (
	"context"
	"log"

	"github.com/gammons/slk/internal/cache"
	"github.com/gammons/slk/internal/ui"
)

// seedCustomEmojiFromCache publishes teamID's cached custom emoji set so the
// first paint renders custom emoji with no network round trip.
//
// NON-DESTRUCTIVE, and that is the point (B17). Bootstrap may already have
// published the conversations.view subset as a head start (connect.go), and a
// cache miss must not take it away: cache.CustomEmoji returns an empty map with
// a NIL error when there are no rows, so an unconditional publish would clear
// the subset on every first-ever launch. If emoji.list then failed, every
// custom emoji rendered as literal `:name:` — including in the channel that had
// just rendered correctly. So an empty or unreadable cache leaves whatever is
// already published alone, and only a non-empty cached set replaces it.
//
// Called SYNCHRONOUSLY, before WorkspaceReadyMsg is built, so the message
// carries a determinate emoji set. It does no I/O beyond one indexed SQLite
// read. Do NOT move it back into a goroutine: the message reads
// wctx.CustomEmoji() one statement later, and because that field is an
// atomic.Pointer the resulting race is a VALUE race, not a data race — the
// race detector stays green while first paint varies run to run (B18).
func seedCustomEmojiFromCache(wctx *WorkspaceContext, db *cache.DB, teamID string) {
	if db == nil {
		return
	}
	set, err := db.CustomEmoji(teamID)
	if err != nil {
		log.Printf("custom emoji cache read for %s failed: %v (first paint will use the bootstrap subset)", teamID, err)
		return
	}
	if len(set) == 0 {
		return
	}
	wctx.SetCustomEmoji(set)
}

// fetchWorkspaceEmojiIntoCache publishes the workspace's full custom emoji set,
// persists it for the next cold start, and tells the UI to re-render with it.
//
// It runs UNCONDITIONALLY, and that is the whole point of it. Bootstrap or the
// cache seed may already have published a map, but conversations.view returns
// only the emoji the restored conversation uses — a per-conversation subset,
// not the workspace's set. An earlier version skipped emoji.list whenever that
// subset was non-empty, which left every channel other than the restored one
// rendering custom emoji as literal `:name:`.
// TestFetchWorkspaceEmoji_RunsEvenWhenBootstrapPublishedASubset pins that so
// the short-circuit cannot come back.
//
// Best-effort: on error nothing is published and nothing is sent, so the
// bootstrap subset (or the cache seed, or the built-ins) stays in place rather
// than being cleared. An EMPTY success is treated the same way, for the reason
// below.
//
// Intended to be run in a goroutine, AFTER WorkspaceReadyMsg, so it never
// blocks first paint. The seed above is the part that must precede the message;
// this part must not.
func fetchWorkspaceEmojiIntoCache(ctx context.Context, wctx *WorkspaceContext, client customEmojiLister, sender teaSender, teamID string, db *cache.DB) {
	emojis, err := client.ListCustomEmoji(ctx)
	if err != nil {
		return
	}
	// An empty SUCCESS is "no news", not "the workspace has no custom emoji"
	// (B37). Slack documents `{"ok": true}` as a minimal success body; an absent
	// `emoji` field decodes to a nil map with a nil error, and ListCustomEmoji
	// normalises that nil to an empty map (internal/slack/client.go) — still a
	// nil error. Gating only on `err != nil` therefore lets a bodyless success
	// through to db.UpsertCustomEmoji, which is a wholesale replace (DELETE the
	// team's rows, INSERT the argument), so it EMPTIES the team's cached set.
	// The next cold start then seeds nothing and every custom emoji in the
	// workspace renders as literal `:name:`.
	//
	// The asymmetry decides it. Skipping the write when an admin really did
	// delete every custom emoji leaves one stale set until the next fetch: a few
	// dead shortcodes. Honouring a spurious empty destroys a known-good set. And
	// a workspace that genuinely has none is unaffected — its cache is already
	// empty, so the skipped write was a no-op, and CustomEmoji() returns
	// empty-non-nil through the accessor regardless.
	//
	// Scoped to EMPTY deliberately. B37 originally alleged a TRUNCATED page
	// overwriting a full cache; that mechanism does not exist, because
	// emoji.list does not page (no cursor/limit/has_more in the method, and
	// customEmojiLister returns a bare map — a partial page is unrepresentable).
	// So there is no "implausibly smaller" case to defend against, and a ratio
	// heuristic would only misfire on legitimate bulk deletions.
	//
	// Same trap and same answer as the seed's emptiness check above (B17), one
	// layer out.
	if len(emojis) == 0 {
		return
	}
	wctx.SetCustomEmoji(emojis)
	if db != nil {
		// Not best-effort-silent: this write decides what the NEXT cold start
		// renders, so a failure past the busy_timeout has to be visible or the
		// cache degrades to permanently stale with no signal (B22).
		if err := db.UpsertCustomEmoji(teamID, emojis); err != nil {
			log.Printf("custom emoji cache write for %s failed: %v (next cold start will seed a stale set)", teamID, err)
		}
	}
	if sender != nil {
		sender.Send(ui.CustomEmojisLoadedMsg{
			TeamID:      teamID,
			CustomEmoji: emojis,
		})
	}
}
