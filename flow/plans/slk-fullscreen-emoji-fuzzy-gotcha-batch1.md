# Gotcha Coverage — slk-fullscreen-emoji-fuzzy (batch 1: emoji cache persistence)

> Source: `flow/plans/slk-fullscreen-emoji-fuzzy.md`
> Mode: plan
> Command: `cmd-pallet/gotcha-coverage`, batch 1 of 4
> Sub-agent: general-purpose (delegated; 15 tool calls)
> Units reviewed: `(emoji-seed-teamtag)`, `(emoji-upsert-txn)`, `(emoji-cache-table)`,
> `(emoji-cache-wiring)`, `(emoji-seed-order-test)`
> Verification: all 8 findings independently re-checked against
> `slkfz/integration` @ `59cb245` before recording. **8 of 8 confirmed.**

Numbering continues the main gotcha doc (B1–B16 taken). Per the command:
per-batch appendix, never one merged file; originals immutable; Rank 3+ raised
in the plan's `## Open Threads`.

## Findings (ranked)

### Rank 4 (Significant — impacts correctness)

- **B17 The seed clobbers the bootstrap emoji subset with an empty map, so a
  cold start plus a failed `emoji.list` renders every custom emoji as literal
  `:name:`.**
  - What: `connect.go:235` publishes the `conversations.view` subset as a
    deliberate head start (`if len(res.Emojis) > 0 { wctx.SetCustomEmoji(...) }`).
    `seedCustomEmojiFromCache` then runs unconditionally and publishes whatever
    the cache returned. On a first-ever launch that is an **empty map with a nil
    error** — `cache.CustomEmoji` does `res := make(map[string]string)` and
    returns it (`internal/cache/customemoji.go:23-35`), so the
    `err == nil && set != nil` branch is taken and overwrites the subset. The
    `else` branch does the same thing explicitly. If `emoji.list` then fails,
    `fetchWorkspaceEmojiIntoCache` returns early without publishing
    (`customemojiseed.go:28-31`), and the workspace renders every custom emoji
    as literal `:name:` — including in the restored channel that rendered
    correctly a moment earlier.
  - Why missed: the plan framed the cache as strictly additive ("a head start"),
    so it only asked what happens when the cache **has** rows. Unit 3 specifies
    "cache-miss → empty map, never an error surface to the UI", which is correct
    at the `internal/cache` boundary. Unit 4 then wired that empty map straight
    into `wctx` as a **publication**, where empty means "clear what is there".
    Two locally correct decisions compose into a clear.
  - Severity: user-visible regression of the exact behaviour
    `customemoji.go:23-30`'s comment exists to prevent.
  - Mitigation: make the seed non-destructive — skip the publish when the
    cached set is empty. Add the missing test (prime `wctx` with a subset, seed
    against an empty cache, assert the subset survives) and rewrite
    `TestSeedCustomEmojiFromCache_MissPublishesEmpty` to assert non-clobbering.
    That test currently pins the broken behaviour and uses a bare
    `&WorkspaceContext{}` with no prior subset, so the collision is invisible to
    it (`customemojiseed_test.go:89-99`).

- **B18 The startup goroutine races `WorkspaceReadyMsg.CustomEmoji`, so the
  first painted frame's emoji set is nondeterministic — and `-race` cannot see
  it.**
  - What: `main.go:1565` launches `go runStartupEmoji(...)`; `main.go:1579`
    reads `wctx.CustomEmoji()` in the very next statement to build
    `WorkspaceReadyMsg`. Access is via `atomic.Pointer`
    (`cmd/slk/workspace.go:115`), so this is a **value** race, not a data race:
    the race detector stays green permanently. Depending on scheduling the
    message carries the bootstrap subset, the cached set, B17's empty map, or
    the freshly fetched set. `reduceWorkspaceReady` feeds it to
    `a.SetCustomEmoji` (`internal/ui/reducer_workspace.go:222`), which rebuilds
    the entry list, so first paint and the initial picker contents vary run to
    run.
  - Why missed: unit 4's requirement is an ordering claim ("seeds from the cache
    BEFORE network") and unit 5 tests exactly that — but **synchronously**
    (`customemojiseed_test.go:131`), where seed-before-fetch holds trivially.
    The production call is `go`. The ordering that actually matters at startup
    is seed-vs-`WorkspaceReadyMsg`, which no unit states and no test covers.
  - Severity: nondeterministic first paint; also hides B17 half the time, which
    is why B17 was not caught by ordinary use.
  - Mitigation: call `seedCustomEmojiFromCache` **synchronously** before
    constructing `WorkspaceReadyMsg` and background only the fetch. That makes
    the message deterministic, gives the seed the "before first paint" effect
    unit 4 wants, and discharges B20 as a side effect. The inline comment
    "replaced by the goroutine below" is stale twice over: the goroutine is
    above it, and it may already have run.

- **B19 `fetchWorkspaceEmoji` and `fetchWorkspaceEmojiIntoCache` are the same
  function under two spellings; production calls only the second, so the
  invariant documented on the first is unpinned and four tests guard dead code.**
  - What: two implementations of "call `emoji.list`, publish into `wctx`, send
    `CustomEmojisLoadedMsg`". The original (`cmd/slk/customemoji.go:38`) carries
    the 20-line comment that is the **only** record of why the fetch must run
    unconditionally. The copy (`cmd/slk/customemojiseed.go:27`) is that body plus
    the cache upsert, and is what `runStartupEmoji` calls.
    `fetchWorkspaceEmoji` has **zero non-test callers** — verified: the only
    non-test grep hit is its own declaration.
  - Why missed: unit 4 says "existing bootstrap-subset/failure semantics
    preserved (`customemoji_test.go` assertions keep passing)". They do keep
    passing — because they test the dead copy.
    `TestFetchWorkspaceEmoji_RunsEvenWhenBootstrapPublishedASubset` and
    `_ErrorKeepsBootstrapSubset` (`customemoji_test.go:109,132`) are regression
    guards over code nobody runs, and unit 1's
    `TestFetchWorkspaceEmoji_SendsLoadedMsgForItsTeam` (`:151`) is in the same
    position. "Keeps passing" was read as "still protected"; the tests were
    pinned to the wrong symbol.
  - Severity: the live path has no coverage of the invariant its own comment
    says is essential, and the next person to edit will likely edit the dead
    copy.
  - Mitigation: **delete one, never alias** (the command's resolution rule).
    Delete `fetchWorkspaceEmoji`, move its explanatory comment onto
    `fetchWorkspaceEmojiIntoCache`, repoint the four `customemoji_test.go` tests
    at the survivor. The signatures differ only by a trailing `db` param, so the
    change is mechanical. Also delete the orphaned comment at `main.go:1586-1589`,
    whose `go func` body was removed — it still ends "see fetchWorkspaceEmoji for
    why the bootstrap subset must not be treated as an answer" and is followed by
    the *usergroups* comment.

### Rank 3 (Moderate — needs explicit handling)

- **B20 Nothing rebuilds `BuildEntries` from the seed, so unit 4's "seed before
  network" is unobservable when the fetch fails.** The seed publishes into
  `wctx` only. The UI learns about emoji through exactly three messages
  (`reducer_workspace.go:159,222,362`), and the seed sends none of them — only
  `fetchWorkspaceEmojiIntoCache` sends `CustomEmojisLoadedMsg`, and only on
  success. With B18's race, the cached set reaches `App.SetCustomEmoji` only by
  winning a scheduling coin flip. `App` is built with `emoji.BuildEntries(nil)`
  (`app.go:832`), so a lost race plus a failed fetch leaves built-ins only, with
  a populated cache unused on disk — the offline-start failure mode. Unit 4
  conflates "seeds `BuildEntries` from the cache" (a UI-side rebuild) with the
  `wctx` store the seed actually writes; those coincide only because
  `WorkspaceReadyMsg` happens to read `wctx`, which is an accident of ordering,
  not a mechanism. Fixing B18 discharges this.

- **B21 `updated_at` is declared, defaulted, and never written.** The schema has
  `updated_at INTEGER NOT NULL DEFAULT 0` (`internal/cache/db.go:204`) and the
  INSERT omits it (`internal/cache/customemoji.go:52`), so every row is `0`
  forever and no reader can distinguish a set cached minutes ago from months
  ago. Every other table in the schema writes it (`channels.go:21`,
  `users.go:42`, `thread_subscriptions.go` in four places). Unit 3 names the
  column in the table shape and separately specifies the write as "DELETE by
  team_id + INSERT"; neither sentence says the write populates it, and
  `customemoji_test.go:251` asserts only that the column **exists** via
  `PRAGMA table_info`. **A schema assertion is not a write assertion.**
  Mitigation: write `strftime('%s','now')` and assert non-zero — or drop the
  column, noting the plan's "recent tier" and any future revalidation will want
  it, and adding it later means a migration.

- **B22 The upsert's error is discarded, so a full disk or lock timeout silently
  degrades the cache to permanently stale.** `customemojiseed.go:34` is
  `_ = db.UpsertCustomEmoji(teamID, emojis)`. Atomicity itself is fine —
  unit 2's concern is genuinely met: the DELETE+INSERT pair is one transaction
  with `defer tx.Rollback()`, and concurrent refreshes serialize on SQLite's
  single writer with `busy_timeout(5000)`. What is missing is observability: a
  failure past the timeout leaves the previous set in place with no log line, so
  the next cold start seeds a set one or more refreshes behind, indefinitely.
  Every comparable best-effort path here logs — see the usergroups fetch,
  `main.go:1598`. Mitigation: one `log.Printf` naming the team and the
  consequence.

### Rank 2 (Minor)

- **B23 `seedCustomEmojiFromCache`'s lister parameter is blanked, making unit
  5's `callCount 0` assertion unfalsifiable.** The signature is
  `func seedCustomEmojiFromCache(wctx *WorkspaceContext, db *cache.DB, teamID string, _ customEmojiLister)`
  — discarded in the **declaration**. So
  `TestSeedCustomEmojiFromCache_NeverCallsTheLister`
  (`customemojiseed_test.go:44-59`) asserts something the compiler already
  guarantees; it cannot fail for any body. Unit 5's "the seed path never calls
  the lister (`callCount 0` at seed time)" is satisfied vacuously.
  `TestStartupEmojiOrder_SeedBeforeFetch` (`:122`) does carry real signal via
  `orderProbingLister.snapshotAtCall`, so the coverage is misattributed rather
  than absent — hence Rank 2. Mitigation: drop the unused parameter and let the
  order test carry G13 alone.
  - This is the **fifth** vacuous test found in this plan's suite (with B1, B13,
    the Tab row, and the frame-delta oracle). The recurring shape: *an assertion
    whose subject cannot vary*.

## Categories that yielded nothing

- **The mandated env-var / flag duplication hunt: clean.** Every name is read in
  exactly one non-test file — `TMUX` ×3 and `XDG_DATA_HOME` ×2 but each within a
  single file, plus `XDG_CONFIG_HOME`, `XDG_CACHE_HOME`, `WAYLAND_DISPLAY`,
  `SLK_DEBUG` once each. No config field with an env-var twin in this area, and
  no second spelling of an emoji setting. **The class still fired** — it
  surfaced as a duplicated *function* (B19) with a live copy and a dead copy,
  same failure shape (behaviour silently differing between call sites) and same
  resolution rule (delete one, never alias).
- **Migration on an existing DB: clean.** `custom_emoji` is
  `CREATE TABLE IF NOT EXISTS` inside the single `migrate()` Exec
  (`db.go:200-206`), needs no `addColumnIfMissing` probe since the whole table is
  new, and has no FK to `workspaces` — so unit 2's G9 ordering concern is
  structurally moot rather than merely tested.
- **Half-applied DELETE+INSERT visible to a reader: not reachable.** One
  transaction, `defer tx.Rollback()`, and WAL gives readers a consistent
  snapshot.
- **Team isolation on switch: clean.** `WorkspaceSwitchedMsg.CustomEmoji` reads
  the active `wctx` (`main.go:1371`), `reduceWorkspaceSwitched` applies it
  synchronously (`reducer_workspace.go:362`), and `CustomEmojisLoadedMsg` is
  guarded by `m.TeamID == a.activeTeamID` (`reducer_workspace.go:157-159`).
  Unit 1's core claim holds.
- **Cold start seeding only the connecting workspace: fine.** `runStartupEmoji`
  is called per workspace inside the connect loop.

## Cross-references

- B17 and B18 compound: the race hides the clobber roughly half the time, which
  is why ordinary use did not surface it.
- B18's fix discharges B20.
- B19 is the same class as B2 (two toast helpers, one contract) — resolved in
  this plan by consolidation — and as the `AGENTS.md` headline rule that the
  repo's worst recurring defect is the same logic under two spellings.
- B23 joins B1, B13 and the Tab-row oracle as vacuous assertions.
