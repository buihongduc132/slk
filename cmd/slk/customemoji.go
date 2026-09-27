package main

import (
	"context"
)

// customEmojiLister is the one-method slice of *slackclient.Client that
// the emoji fetch needs. Mirrors reconnectClient in reconnect_sync.go:
// a narrow interface keeps the fetch testable without a live client,
// and fails compilation if the surface grows.
type customEmojiLister interface {
	// ListCustomEmoji is emoji.list: the workspace's entire custom
	// emoji set. Distinct from conversations.view's Emojis field,
	// which is only the emoji the fetched conversation happens to use.
	ListCustomEmoji(ctx context.Context) (map[string]string, error)
}

// The emoji fetch itself lives in customemojiseed.go as
// fetchWorkspaceEmojiIntoCache, together with the cache seed that must precede
// WorkspaceReadyMsg.
//
// There used to be a second copy here, fetchWorkspaceEmoji, identical except
// for the cache upsert. Production called only the copy in customemojiseed.go,
// so this one was dead — while carrying the only written record of why the
// fetch must run unconditionally, and while four tests pointed at it and
// "kept passing" over code nobody ran (B19). Deleted per the repo rule for the
// same logic under two spellings: DELETE one, never alias. Its comment moved
// onto the survivor.
