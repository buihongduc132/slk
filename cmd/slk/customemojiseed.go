package main

import (
	"context"

	"github.com/gammons/slk/internal/cache"
	"github.com/gammons/slk/internal/ui"
)

func seedCustomEmojiFromCache(wctx *WorkspaceContext, db *cache.DB, teamID string, _ customEmojiLister) {
	if db == nil {
		return
	}
	set, err := db.CustomEmoji(teamID)
	if err == nil && set != nil {
		wctx.SetCustomEmoji(set)
	} else {
		wctx.SetCustomEmoji(map[string]string{})
	}
}

func runStartupEmoji(ctx context.Context, wctx *WorkspaceContext, db *cache.DB, client customEmojiLister, sender teaSender, teamID string) {
	seedCustomEmojiFromCache(wctx, db, teamID, client)
	fetchWorkspaceEmojiIntoCache(ctx, wctx, client, sender, teamID, db)
}

func fetchWorkspaceEmojiIntoCache(ctx context.Context, wctx *WorkspaceContext, client customEmojiLister, sender teaSender, teamID string, db *cache.DB) {
	emojis, err := client.ListCustomEmoji(ctx)
	if err != nil {
		return
	}
	wctx.SetCustomEmoji(emojis)
	if db != nil {
		_ = db.UpsertCustomEmoji(teamID, emojis)
	}
	if sender != nil {
		sender.Send(ui.CustomEmojisLoadedMsg{
			TeamID:      teamID,
			CustomEmoji: emojis,
		})
	}
}
