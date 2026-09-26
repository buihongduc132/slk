# Gotcha Coverage — slk-fullscreen-emoji-fuzzy

> Source: flow/plans/slk-fullscreen-emoji-fuzzy.md
> Mode: plan
> Sub-agents: 3 parallel batches (fullscreen / emoji / e2e+DOD), delegated per gotcha-coverage
> Units reviewed: 11 task items + 4 DOD criteria (U1–U5 per batch)
> Date: 2026-09-26 · Rank-3+ findings appended to plan as new items (15); originals untouched

## Machine verification of the top claims (planner, against main @ 2b43b29)

| Claim | Verdict | Evidence |
|---|---|---|
| statusHeight=1 hardcoded in Compute; PanelAt routes y≥height-1 to status | CONFIRMED | `internal/ui/panellayout.go:82,88,149` |
| runKeyCases bypasses reducer chain (esc rows = false green) | CONFIRMED | `internal/ui/modekeys_test.go` + AGENTS.md helper table: "bypasses the reducer chain and the ctrl+c / bootstrap / scroll-flush gates" |
| subsequenceScore is IN-order, not out-of-order | CONFIRMED | `internal/ui/channelfinder/model.go:476` "every rune of q appears in name in order" |
| confirmprompt claims bare `z` (decline) | CONFIRMED | `internal/ui/confirmprompt/model_test.go:56` |
| workspace switch resets stackFront | CONFIRMED | `internal/ui/app.go:2194 a.stackFront = PanelMessages`; `threadInFront()` at :898 uses stackFront only |
| CustomEmojisLoadedMsg carries TeamID (seed must be tagged) | CONFIRMED | `internal/ui/msgs.go:415-418` |
| DSN forces foreign_keys(ON) | CONFIRMED | `internal/cache/db.go:37 dsnPragmas` |
| frecent_emoji is global, no team column | CONFIRMED | `internal/cache/db.go:151-156` + `cache/frecent.go` |
| reactionpicker filter = prefix + buffered substring, cap 50 (tier set differs) | CONFIRMED | `internal/ui/reactionpicker/model.go:237-259` |

## Findings (ranked, consolidated)

### Rank 5 (invalidates a mechanism as written)
- **G1 esc-pecking-order breaks existing esc consumers** (batch1 U2): esc is consumed in ≥6 non-switch places — pendingWinCmd chord cancel (pinned by `windows_chord_test.go`), pendingTop `g`-chord, reaction-nav, insert upload-toast, edit-cancel, picker-close, plus modal modes own esc. Blanket "zoom-exit BEFORE any other Escape arm in every mode" breaks pinned tests and steals modal esc. → superseding item `fs-esc-pecking-order`.
- **G2 status-row math makes full-terminal span impossible with the listed inputs** (batch1 U3): `Compute` does `contentHeight := height - statusHeight` unconditionally and `PanelAt` hardcodes `y >= height-1` as status bar. "View skips the status row" alone ⇒ blank bottom row + misrouted clicks on last content row. → `fs-statusrow-math`.
- **G3 runKeyCases is the wrong harness for esc rows** (batch3 U1): it drives `dispatchModeKey` directly, bypassing the reducer chain where the esc rule most naturally lives — key-table esc rows pass while production esc is broken. Split: `z` rows → runKeyCases; esc rows → `updateAndRender`/`a.Update`; toast rows execute the returned cmd (snooze-test pattern). → `fs-esc-test-layering`.
- **G4 cold-start seed is team-untagged and switch-unsafe** (batch3 U4): seed must be TeamID-tagged and re-read on WorkspaceSwitchedMsg, else workspace B renders A's customs until B's fetch lands; existing test `TestFetchWorkspaceEmoji_SendsLoadedMsgForItsTeam` pins exactly this hazard. → `emoji-seed-teamtag`.
- ~~"fuzzy matcher not extracted yet" (batch2 U3, rank 5)~~ — **INVALID as a gotcha**: that is the plan's own pending item `fuzzy-shared`, not a missed risk. Every plan item is unimplemented at planning time.
- ~~"frecent never seeded from customs" (batch2 U2, rank 5)~~ — **INVALID**: plan never promised seeding frecent from emoji.list (that would forge usage counts). Valid residue — "recent tier" must be defined as *matching entries ∩ frecent* — folded into `fuzzy-rkt-verified`.

### Rank 4
- **G5 stale zoom**: fullscreen survives the zoomed pane disappearing — `q` closes thread but leaves zoom on messages; ThreadClosedMsg, workspace switch (app.go:2194), channel jump, ctrl+a Activity all strand a zoom with no nav affordances; Enter/click opens a thread *invisibly* while zoomed on messages. → `fs-zoom-invariant`.
- **G6 toasts invisible while zoomed**: suppression feedback + upload toasts + chord hints all render in the hidden status row. → `fs-toast-zoomed`.
- **G7 command-mode bypass**: `:sp/:vsp/:only/:q` reach window ops under zoom with no rule; `:q` (close window) ≠ `q` (close thread) ≠ `Q` (quit). → `fs-supp-set`.
- **G8 insert-mode `z` must type 'z'**: binding must live in the normal-mode map only; rows for insert-z zoomed and unzoomed. → `fs-insert-z-types`.
- **G9 custom_emoji upsert hazards**: FK failure if fetch lands before workspace row (foreign_keys ON, db.go:37); DELETE+INSERT must be one transaction, team-scoped, or workspace B's rows die when A refreshes. → `emoji-upsert-txn`.
- **G10 "out-of-order subsequence" spec ambiguity**: the mandated matcher is in-order (`subsequenceScore`); "out-of-order" reads as bag-of-chars. Fix DOD wording + negative test `krt ↛ rocket`. → `fuzzy-inorder-subseq`.
- **G11 rkt→rocket / :thumbs→+1 are empirical claims**: ~3.9k-alias generated table + alphabetical-within-tier + MaxVisible=5 + recent tier can displace the DOD's own examples (5 matching recents evict `+1`). Verify examples against real `BuildEntries`; make subsequence tier score-aware; define cap/eviction semantics. → `fuzzy-rkt-verified`.
- **G12 DOD-3 proof scope**: names emojipicker tests only while the task also migrates reactionpicker + compose — criterion goes green with 2 of 3 surfaces un-migrated; and reactionpicker/emojipicker expectation changes are *behavior changes*, so fuzzy-shared's "expectations unchanged" can only hold for channelfinder/mentionpicker. → `fuzzy-reactionpicker-migration` (+ Open Thread on DOD-3 wording).

### Rank 3
- **G13 cmd/slk wiring seam uncovered** (batch3 U1): cold-start ordering (seed-before-fetch, seed never calls lister) needs an explicit recorded-call-order test in cmd/slk. → `emoji-seed-order-test`.
- **G14 restore must be byte-equal, fixture-dependent**: `frame(z,z) == frameBefore` assertion with a scrolled-mid-history fixture + selection, else golden proves nothing; resize-while-zoomed-then-exit case. → `fs-restore-eq`.
- **G15 zoom + focus flip**: `threadFront=(stackFront==PanelThread||focusedPanel==PanelThread)` makes Tab/click change the zoomed pane — use `threadInFront()`-style stackFront-only under zoom; tab-while-zoomed row. Folded into `fs-statusrow-math` + `fs-zoom-invariant`.
- **G16 cache/memo keys**: include fullscreen in screen-memo/cache keys; keep caching the zoomed pane at zoom width; exit frame byte-equal. → `fs-zoom-cache-keys`.
- **G17 mouse routers split**: PanelAt is not the only hit-test — parallel End-accessor chains in wheel/click handlers each carry status-row assumptions; sixel/kitty placement math consumes frame widths too. Folded into `fs-statusrow-math`.
- **G18 help-table churn + golden stability**: FullscreenToggle flows into `help.FromKeyMap`; help-entry diffs expected; cache seeding must stay out of `NewApp`/`newTestApp` or goldens re-bless wholesale. Doc note + covered by `fs-restore-eq`/`e2e-suite-green` expectations.
- **G19 bootstrap subset vs fetch race**: fold into `emoji-seed-teamtag` (synchronous apply under switch reducer = no race; atomic map swap).

### Rank ≤2 (doc-only, no item)
- `Z` left unbound (bind for consistency or drop from evidence note).
- "messages zoomed" golden ambiguous — add frame with thread open behind (folded into `fs-restore-eq` fixture matrix).
- go.mod/go.sum freeze not gated — add `git diff --exit-code go.mod go.sum` to the PR checklist (encodes the user ruling; cheap, recommended).
- Modal-over-zoom frame composition (help/finder/reactions) — one key row "open help while zoomed, esc → still zoomed" (covered by `fs-esc-pecking-order`).

## Process notes
- Plan items were NOT rewritten (append-only). Invalidated *mechanism clauses* are recorded as Open Threads + superseding items rather than flipping items to blocked, because each affected item's end-state survives with a corrected mechanism — deviation noted here.
- Seam note: this plan lives in `flow/plans/` per the invoked cmd. The slk repo's own convention is `docs/superpowers/specs + plans` with tests-first (README workflow); implementation lanes should still follow the repo convention — carry this plan as the requirement source.
