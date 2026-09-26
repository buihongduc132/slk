// internal/cache/customemoji.go
//
// RED-phase stub for plan slk-fullscreen-emoji-fuzzy item
// emoji-cache-table. Signatures are FINAL (the cmd/slk wiring seam
// in customemojiseed.go mirrors them); the bodies are zero-logic so
// customemoji_test.go fails as assertions, not build errors. The
// GREEN implementation adds the custom_emoji(team_id, name, value,
// updated_at) table to migrate() (pattern: `workspaces`, db.go:79)
// and makes the upsert one team-scoped transaction (DELETE by team +
// INSERT), safe under the foreign_keys(ON) DSN (db.go:37).
package cache

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

	for name, value := range emojis {
		if _, err := tx.Exec("INSERT INTO custom_emoji (team_id, name, value) VALUES (?, ?, ?)", teamID, name, value); err != nil {
			return err
		}
	}

	return tx.Commit()
}
