// internal/cache/customemoji.go
//
// Per-workspace custom emoji persistence for plan item emoji-cache-table:
// the custom_emoji(team_id, name, value, updated_at) table declared in
// migrate() (pattern: `workspaces`, db.go:79), read as a whole set, replaced
// as a whole set in one team-scoped transaction.
//
// Read contract, relied on by the cmd/slk seed: a cache MISS is an empty,
// non-nil map with a NIL error. That is deliberate at this boundary — the UI
// never has to surface a cache error. It is also a trap one layer up: an empty
// map published into a WorkspaceContext CLEARS whatever is there, so the seed
// must check emptiness rather than just `err == nil` (B17).
//
// (This header described a RED-phase stub with zero-logic bodies until the
// feature landed; it outlived that by several commits.)
package cache

import "time"

// CustomEmoji returns teamID's cached custom emoji set (name ->
// URL-or-"alias:target"). A cache miss is an EMPTY, non-nil map and
// a nil error — never an error the UI has to surface.
func (db *DB) CustomEmoji(teamID string) (map[string]string, error) {
	rows, err := db.conn.Query("SELECT name, value FROM custom_emoji WHERE team_id = ?", teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]string)
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		res[name] = value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

// UpsertCustomEmoji replaces teamID's cached set with emojis in one
// team-scoped transaction (DELETE the team's rows, INSERT the new
// set). An empty map empties the team's set.
func (db *DB) UpsertCustomEmoji(teamID string, emojis map[string]string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM custom_emoji WHERE team_id = ?", teamID); err != nil {
		return err
	}

	// updated_at is written from ONE timestamp taken before the loop, so every
	// row in a team's set shares it and a reader can treat the set as a unit.
	// It was previously omitted (B21): the column is NOT NULL DEFAULT 0, so
	// every row read back as 0 forever and nothing could distinguish a set
	// cached minutes ago from months ago. Every other table in this schema
	// writes it -- channels.go, users.go, thread_subscriptions.go.
	now := time.Now().Unix()
	for name, value := range emojis {
		if _, err := tx.Exec("INSERT INTO custom_emoji (team_id, name, value, updated_at) VALUES (?, ?, ?, ?)", teamID, name, value, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}
