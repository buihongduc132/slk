package cache

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------
// RED tests for the custom_emoji cache table (plan item
// emoji-cache-table, gotcha G9). They fail today because the table
// and its accessors do not exist; the two that peek at the schema
// fail as clean assertions, and the accessor tests fail as
// no-such-method build errors ONLY if the GREEN agent has not added
// them — per the task brief, these accessors are the FINAL signatures
// the plan's wiring test (cmd/slk) also depends on.
//
// Shape (per the plan, patterned on `workspaces` in db.go):
//
//	CREATE TABLE IF NOT EXISTS custom_emoji (
//		team_id    TEXT NOT NULL,
//		name       TEXT NOT NULL,
//		value      TEXT NOT NULL,
//		updated_at INTEGER NOT NULL DEFAULT 0,
//		PRIMARY KEY (team_id, name)
//	);
// ---------------------------------------------------------------------

// newCustomEmojiDB is the shared builder: an in-memory cache, which
// the DSN pragmas pin to a single connection so every test writes and
// reads the same database. foreign_keys(ON) rides the DSN (db.go:37),
// which is exactly the hazard G9 covers.
func newCustomEmojiDB(t *testing.T) *DB {
	t.Helper()
	db, err := New(":memory:")
	if err != nil {
		t.Fatalf("New(:memory:) failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustUpsertWorkspace(t *testing.T, db *DB, id, name string) {
	t.Helper()
	if _, err := db.conn.Exec(
		`INSERT INTO workspaces (id, name) VALUES (?, ?)
		 ON CONFLICT(id) DO UPDATE SET name = excluded.name`,
		id, name,
	); err != nil {
		t.Fatalf("upserting workspace %s: %v", id, err)
	}
}

// The table must exist after a plain New() — same guarantee every
// other table in migrate() carries. A missing table turns every
// read into an error the UI would have to surface, which
// emoji-cache-table explicitly forbids.
func TestCustomEmoji_TableExistsAfterMigrate(t *testing.T) {
	db := newCustomEmojiDB(t)
	var count int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM custom_emoji").Scan(&count); err != nil {
		t.Fatalf("custom_emoji table missing or unreadable after migrate: %v", err)
	}
	if count != 0 {
		t.Errorf("custom_emoji rows = %d on a fresh database, want 0", count)
	}
}

// Cache miss (no rows for the team) must yield an EMPTY MAP and NO
// error — never nil, never an error the UI has to handle. This is the
// cold-start contract the wiring test builds on.
func TestCustomEmoji_CacheMissReturnsEmptyMapNoError(t *testing.T) {
	db := newCustomEmojiDB(t)
	got, err := db.CustomEmoji("T_NONE")
	if err != nil {
		t.Fatalf("CustomEmoji(missing team) error = %v, want nil — a cache miss is not an error", err)
	}
	if got == nil {
		t.Fatal("CustomEmoji(missing team) = nil map, want an initialized empty map")
	}
	if len(got) != 0 {
		t.Errorf("CustomEmoji(missing team) = %v, want empty", got)
	}
}

// Round trip: upsert then read returns the full map verbatim.
func TestCustomEmoji_UpsertThenReadRoundTrips(t *testing.T) {
	db := newCustomEmojiDB(t)
	mustUpsertWorkspace(t, db, "T1", "alpha")

	want := map[string]string{
		"party-parrot": "https://example.test/parrot.gif",
		"meow":         "alias:cat",
	}
	if err := db.UpsertCustomEmoji("T1", want); err != nil {
		t.Fatalf("UpsertCustomEmoji(T1) failed: %v", err)
	}

	got, err := db.CustomEmoji("T1")
	if err != nil {
		t.Fatalf("CustomEmoji(T1) failed: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("CustomEmoji(T1) = %v (len %d), want %v (len %d)", got, len(got), want, len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("CustomEmoji(T1)[%q] = %q, want %q", k, got[k], v)
		}
	}
}

// Upsert REPLACES, not merges: emoji.list returns the workspace's
// whole set, so an entry dropped server-side must disappear from the
// cache (same rationale as WorkspaceContext.SetCustomEmoji's
// replace-not-merge, customemoji_test.go:23).
func TestCustomEmoji_UpsertReplacesNotMerges(t *testing.T) {
	db := newCustomEmojiDB(t)
	mustUpsertWorkspace(t, db, "T1", "alpha")

	if err := db.UpsertCustomEmoji("T1", map[string]string{
		"stale":  "https://example.test/stale.png",
		"common": "https://example.test/common.png",
	}); err != nil {
		t.Fatalf("first UpsertCustomEmoji: %v", err)
	}
	if err := db.UpsertCustomEmoji("T1", map[string]string{
		"common": "https://example.test/common.png",
		"fresh":  "https://example.test/fresh.png",
	}); err != nil {
		t.Fatalf("second UpsertCustomEmoji: %v", err)
	}

	got, err := db.CustomEmoji("T1")
	if err != nil {
		t.Fatalf("CustomEmoji(T1): %v", err)
	}
	if _, ok := got["stale"]; ok {
		t.Errorf("CustomEmoji(T1) still contains %q after the upsert; upsert must replace, not merge", "stale")
	}
	if got["fresh"] == "" {
		t.Errorf("CustomEmoji(T1)[%q] empty; the new entry must be present", "fresh")
	}
}

// Empty upsert = team's set emptied. emoji.list legitimately returns
// zero customs; the cache must reflect that rather than keep stale
// rows alive.
func TestCustomEmoji_EmptyUpsertClearsTheTeam(t *testing.T) {
	db := newCustomEmojiDB(t)
	mustUpsertWorkspace(t, db, "T1", "alpha")
	if err := db.UpsertCustomEmoji("T1", map[string]string{"a": "b"}); err != nil {
		t.Fatalf("UpsertCustomEmoji: %v", err)
	}
	if err := db.UpsertCustomEmoji("T1", map[string]string{}); err != nil {
		t.Fatalf("empty UpsertCustomEmoji: %v", err)
	}
	got, err := db.CustomEmoji("T1")
	if err != nil {
		t.Fatalf("CustomEmoji(T1): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("CustomEmoji(T1) = %v after an empty upsert, want empty", got)
	}
}

// G9 core: two teams in one database. Refreshing team A must leave
// team B's rows untouched — the DELETE inside the upsert is scoped to
// the team, not the table.
func TestCustomEmoji_TwoTeamReplaceLeavesOtherTeamIntact(t *testing.T) {
	db := newCustomEmojiDB(t)
	mustUpsertWorkspace(t, db, "T1", "alpha")
	mustUpsertWorkspace(t, db, "T2", "beta")

	teamA := map[string]string{"a-only": "https://example.test/a.png"}
	teamB := map[string]string{"b-only": "https://example.test/b.png"}
	if err := db.UpsertCustomEmoji("T1", teamA); err != nil {
		t.Fatalf("UpsertCustomEmoji(T1): %v", err)
	}
	if err := db.UpsertCustomEmoji("T2", teamB); err != nil {
		t.Fatalf("UpsertCustomEmoji(T2): %v", err)
	}
	// Refresh A with a different set.
	if err := db.UpsertCustomEmoji("T1", map[string]string{"a-new": "https://example.test/a2.png"}); err != nil {
		t.Fatalf("refresh UpsertCustomEmoji(T1): %v", err)
	}

	gotB, err := db.CustomEmoji("T2")
	if err != nil {
		t.Fatalf("CustomEmoji(T2): %v", err)
	}
	if len(gotB) != 1 || gotB["b-only"] == "" {
		t.Errorf("CustomEmoji(T2) = %v after T1's refresh; team B's rows must be untouched", gotB)
	}
	gotA, err := db.CustomEmoji("T1")
	if err != nil {
		t.Fatalf("CustomEmoji(T1): %v", err)
	}
	if _, ok := gotA["a-only"]; ok {
		t.Errorf("CustomEmoji(T1) = %v; the refresh must have replaced the old set", gotA)
	}
	if gotA["a-new"] == "" {
		t.Errorf("CustomEmoji(T1)[a-new] is empty; the refreshed entry must be present: %v", gotA)
	}
}

// G9's FK hazard: the DSN forces foreign_keys(ON) on every
// connection (db.go:37). The upsert must therefore succeed with NO
// workspace row at all (no FK from custom_emoji to workspaces — the
// plan's stated resolution), or upsert the workspace row first;
// either way, emoji-before-workspace-row order must not fail.
func TestCustomEmoji_UpsertWithoutWorkspaceRowSucceeds(t *testing.T) {
	db := newCustomEmojiDB(t)
	// Deliberately NO workspaces row for T_LATE: the emoji fetch can
	// land before the workspace row is written.
	if err := db.UpsertCustomEmoji("T_LATE", map[string]string{"x": "y"}); err != nil {
		t.Fatalf("UpsertCustomEmoji before any workspace row failed under foreign_keys(ON): %v", err)
	}
	got, err := db.CustomEmoji("T_LATE")
	if err != nil {
		t.Fatalf("CustomEmoji(T_LATE): %v", err)
	}
	if got["x"] != "y" {
		t.Errorf("CustomEmoji(T_LATE) = %v, want the round-tripped entry", got)
	}
}

// Column shape: the plan pins team_id/name/value/updated_at. A
// missing column or a rename breaks the wiring silently, so probe the
// schema directly like TestNewDBCreatesIndexes does.
func TestCustomEmoji_TableShape(t *testing.T) {
	db := newCustomEmojiDB(t)
	rows, err := db.conn.Query("PRAGMA table_info(custom_emoji)")
	if err != nil {
		t.Fatalf("PRAGMA table_info(custom_emoji) failed: %v (table missing?)", err)
	}
	defer rows.Close()

	got := map[string]string{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt *string // nullable, and irrelevant here
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scanning custom_emoji columns: %v", err)
		}
		got[name] = ctype
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating custom_emoji columns: %v", err)
	}
	for _, col := range []string{"team_id", "name", "value", "updated_at"} {
		if _, ok := got[col]; !ok {
			t.Errorf("custom_emoji has no %q column; shape per plan emoji-cache-table. Columns: %v", col, got)
		}
	}
}

// B21: updated_at must actually be WRITTEN, not merely declared.
//
// TestCustomEmoji_TableShape above probes PRAGMA table_info and asserts the
// column exists. It passed for the entire life of the bug: the column was
// declared NOT NULL DEFAULT 0 and omitted from the INSERT, so every row read
// back as 0 forever and no reader could distinguish a set cached minutes ago
// from one cached months ago. A schema assertion is not a write assertion.
//
// Three properties, each of which can fail independently:
//   - non-zero, which is the write itself;
//   - one distinct value per team's set, which is the invariant the upsert's
//     own comment claims ("ONE timestamp taken before the loop, so every row in
//     a team's set shares it and a reader can treat the set as a unit") -- a
//     comment asserting an invariant with no check is the thing AGENTS.md says
//     to replace with the check;
//   - plausibly now, in SECONDS. This is the unit trap: UnixMilli() would be
//     non-zero AND uniform, satisfying the first two while being 1000x off and
//     silently breaking any staleness arithmetic. The repo convention is
//     seconds (see thread_subscriptions.go, "Bumps updated_at to
//     time.Now().Unix()"). The window is deliberately generous -- it is
//     checking the unit, not the clock.
func TestCustomEmoji_UpsertWritesUpdatedAt(t *testing.T) {
	db := newCustomEmojiDB(t)

	before := time.Now().Unix()
	if err := db.UpsertCustomEmoji("TA", map[string]string{
		"a-emoji": "https://example.test/a.png",
		"b-emoji": "https://example.test/b.png",
		"c-emoji": "alias:a-emoji",
	}); err != nil {
		t.Fatalf("UpsertCustomEmoji: %v", err)
	}
	after := time.Now().Unix()

	rows, err := db.conn.Query("SELECT DISTINCT updated_at FROM custom_emoji WHERE team_id = ?", "TA")
	if err != nil {
		t.Fatalf("querying updated_at: %v", err)
	}
	defer rows.Close()

	var stamps []int64
	for rows.Next() {
		var ts int64
		if err := rows.Scan(&ts); err != nil {
			t.Fatalf("scanning updated_at: %v", err)
		}
		stamps = append(stamps, ts)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating updated_at: %v", err)
	}

	// V3: prove the query matched something. Zero rows would make every
	// assertion below vacuously true.
	if len(stamps) == 0 {
		t.Fatal("no custom_emoji rows for TA after an upsert of three emoji; the write did not land")
	}
	if len(stamps) != 1 {
		t.Errorf("updated_at has %d distinct values across one team's set (%v), want 1 -- the whole set must share a single timestamp so a reader can treat it as a unit", len(stamps), stamps)
	}

	got := stamps[0]
	if got == 0 {
		t.Fatal("updated_at = 0, i.e. the column's DEFAULT: the INSERT is not writing it, so nothing can tell a fresh cached set from a months-old one (B21)")
	}
	// Generous bounds: this is a unit check, not a clock check. Unix() seconds
	// land inside [before, after]; UnixMilli() would overshoot by ~1000x.
	if got < before-60 || got > after+60 {
		t.Errorf("updated_at = %d, outside the plausible window [%d, %d]. If it is ~1000x too large the write is using UnixMilli(); the repo convention is time.Now().Unix() seconds.", got, before-60, after+60)
	}
}

// Idempotence: the same upsert twice must not duplicate rows or
// error (the PRIMARY KEY (team_id, name) is what enforces it).
func TestCustomEmoji_UpsertIsIdempotent(t *testing.T) {
	db := newCustomEmojiDB(t)
	set := map[string]string{"dup": "https://example.test/dup.png"}
	for i := 0; i < 2; i++ {
		if err := db.UpsertCustomEmoji("T1", set); err != nil {
			t.Fatalf("UpsertCustomEmoji call %d: %v", i+1, err)
		}
	}
	var n int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM custom_emoji WHERE team_id = 'T1'").Scan(&n); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	if n != 1 {
		t.Errorf("custom_emoji rows for T1 = %d after two identical upserts, want 1", n)
	}
}
