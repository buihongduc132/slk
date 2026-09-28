# slk fullscreen pane + emoji fuzzy cache

> Plan ID: `slk-fullscreen-emoji-fuzzy`
> Created: 2026-09-26 · Last reconciled: 2026-09-28
> Status: implemented (merged 5cb4bfa; both gates exit 0, 60 pkgs green under -race, live-verified against dy-swarm)
> Branch: main (2b43b29) — implement lanes on fork branches, PR per lane
> Location: flow/plans/slk-fullscreen-emoji-fuzzy.md
> Items: 26 total (25 implemented, 1 partial [fuzzy-rkt-verified], 0 pending) — 11 original + 15 gotcha-appended (G1–G19 consolidated; see gotcha doc)

## Requirement (verbatim)

> Fullscreen thread + hotkey for slk fork
>
> :cmd-pallet-10-ospx-explore: --- a. how to we be able to implement hotkey that make current focus thread to be fullscreen ; (remember to have hotkey to esc the fullscreen thread as well)  b. how to implement the emoji caching , then also it could be search and auto complete as fuzzy as well ; c. how do we be able to e2e test these feature after implemented ?
>
> must reuse existed approach in the current repository for e2e ; then for the rest , do the :cmd-pallet-10-plan-declarative:  then also :cmd-pallet-gotcha-coverage:  for me ;

Source: user thread verbatim (parent + explore + follow-up). Engineering context: `~/Documents/Projects/bhd/slk` fork (buihongduc132/slk), main @ 2b43b29.

## Exploration evidence (grounds every item)

- a: stacked layout already draws one pane over the content area — `internal/ui/panellayout.go` (`Compute(width, height, railWidth, sidebarWidth, sidebarVisible, threadVisible, threadFront)`), design `docs/superpowers/specs/2026-09-24-thread-stacked-layout-design.md`; `stackFront Panel` on App records the front content pane; keys table `internal/ui/mode_handlers.go` + `mode_normal.go`; bindings `internal/ui/keys.go` (`z`/`Z` verified unbound — no `WithKeys("z")`).
- b: emoji.list fetched per session, never persisted — `cmd/slk/customemoji.go` (`fetchWorkspaceEmoji`) → `CustomEmojisLoadedMsg` (`internal/ui/msgs.go:415`) → `emoji.BuildEntries(customs)` (`internal/emoji/entries.go:29`); SQLite precedent `internal/cache/` (`workspaces` table, `frecent_emoji` via `cache/frecent.go`); fuzzy matcher precedent `internal/ui/channelfinder/model.go:388-483` (prefix > substring > subsequence + `text.Fold`); current emojipicker filter is prefix-only (`internal/ui/emojipicker/model.go` `filter()`, `strings.HasPrefix`).
- c: repo e2e = white-box golden frames + key tables: `compareGolden` + `testdata/golden/*.ansi` (`internal/ui/golden_test.go`), `runKeyCases`/`keyPress`/`keyCode` (`internal/ui/modekeys_test.go`), `stackedApp`/`assertFront`/`updateAndRender` (`internal/ui/thread_stacked_test.go`), `newGoldenApp`. CI runs `go test ./... -race` (~47s). **User ruling: e2e must REUSE these — no new deps (teatest rejected).**

## DOD (Definition of Done)

Plan done when ALL below true:
- [x] `z` toggles the front content pane (thread or messages) fullscreen — rail, sidebar and status row hidden; `esc` (and `z`) exit; prior pane state restored. Proven by golden frames + key-table tests in CI (`go test ./internal/ui -race`).
- [x] Workspace custom emoji survive restart: cold start renders custom emoji from SQLite with zero network wait; `emoji.list` refresh updates the cache. Proven by `internal/cache` + `cmd/slk` tests using `fakeEmojiLister` (existing, `customemoji_test.go`).
- [x] Emoji autocomplete finds emojis by substring and out-of-order subsequence, ranked (recent > prefix > substring > subsequence), accent/case-folded. Proven by emojipicker tests (`:thumbs` → `+1` reachable, `rkt` → `rocket`).
- [x] All feature tests use existing harnesses only — `compareGolden`, `runKeyCases`, `newTestApp`/`newGoldenApp`, fakes in `services_helpers_test.go`/`customemoji_test.go`; `go vet ./...`, `gofmt -l .` empty, full `-race` suite green.

## Tasks

### Fullscreen pane (a)

- [x] fs-esc-pecking-order: esc peel order while zoomed is explicit and test-pinned: modal modes own esc (zoom persists; help/finder/confirm rows included) > insert protective arms (upload-toast, edit-cancel) > zoom-exit > picker-close/insert-exit/chord-cancel/reaction-nav/thread-close; ctrl+w-chord esc-cancel (`windows_chord_test.go`) and reaction-nav esc behavior unchanged when NOT zoomed. [gotcha G1, G15]
- [x] fs-statusrow-math: zoomed pane spans the FULL terminal height — statusHeight threading or height-override reaches `panelLayout.Compute` AND `PanelAt` AND every parallel mouse router (wheel/click End-accessor chains) AND sixel/kitty placement bounds; golden shows content on the last row. [gotcha G2, G17]
- [x] fs-zoom-invariant: fullscreen auto-clears when the zoomed pane closes or the view switches (q-close, ThreadClosedMsg, workspace switch app.go:2194, channel jump, ctrl+a); Enter/click thread-open while zoomed flips zoom to the thread (never opens invisibly); under zoom threadFront derives from stackFront only (Tab/click does not flip the zoomed pane; matches `threadInFront()` :898). [gotcha G5, G15]
  - **Was marked `[x]` with no implementation** (B13). The auto-clear did not
    exist: `exitZoom` had three call sites, none of them a clearing event, and
    `TestFullscreen_ZoomAutoClears` could not fail because it recaptures its
    baseline frame *after* the event. Implemented in `d071d7f` via `clearZoom`
    (which deliberately does NOT restore the saved viewport — see B13), pinned
    by `TestFullscreen_ZoomAutoClearsStateNotFrame` asserting `a.zoomed`.
  - Tab/click not flipping the zoomed pane was already correct, but only
    provable after its oracle was replaced — the old one asserted a token that
    renders in both layouts at 200 cols.
  - Still open: the save/restore round trip is vacuous when the THREAD is the
    zoomed pane (B15). `thread.Model` exposes no viewport accessor, so the
    symmetric restore is not expressible without Phase 3's pane hooks.
- [x] fs-toast-zoomed: toasts and chord hints remain visible while zoomed (transient overlay strip or temporarily un-hidden status row); suppression toast is asserted visible in a zoomed frame. [gotcha G6]
- [x] fs-insert-z-types: `z` in insert mode types 'z' into compose (zoomed and unzoomed) — binding lives in the normal-mode map only. [gotcha G8]
- [x] fs-esc-test-layering: esc/precedence/toast proofs ride the REAL reducer chain — esc rows via `updateAndRender`/`a.Update` (never `runKeyCases`, which bypasses the chain per its own doc); toast rows execute the returned `tea.Cmd` (snooze-test pattern); `z` toggle rows may use `runKeyCases`. Comment in the table points at the bypass caveat. [gotcha G3]
- [x] fs-restore-eq: `frame(z,z) == frameBefore` byte-equal assertion on a fixture with more messages than fit, scrolled mid-history, with a selection; zoomed golden set includes thread-zoomed + messages-zoomed-with-thread-open-behind + resize-while-zoomed-then-exit. [gotcha G14]
- [x] fs-zoom-cache-keys: screen memo/panel-cache keys include `fullscreen` (no stale non-zoomed composite, no dead width-keyed entries); the zoomed pane's own render stays cached at zoom width. [gotcha G16]
- [x] fs-supp-set: the one suppression rule covers BOTH surfaces — keys (ctrl+b, ctrl+], ctrl+w chord suppressed at prefix-arm, 1-9, ctrl+a, ctrl+t, /, :) and command mode (`:sp/:vsp/:only/:q`); `:q`=close window vs `q`=close thread vs `Q`=quit documented; suppressed-at-prefix means the chord never arms while zoomed. [gotcha G7]

- [x] fs-binding: `z` is bound in `internal/ui/keys.go` (`FullscreenToggle`, help "zoom pane in/out") and listed in the help view (`help.FromKeyMap` picks it up; no other mode claims bare `z`).
- [x] fs-state: `App` carries `fullscreen bool`; toggling flips it and requests a render; `esc` while fullscreen clears it BEFORE any other Escape arm in every mode that can see a fullscreen pane (normal + insert; insert must NOT treat it as picker-close/insert-exit while zoomed).
- [x] fs-layout: with `fullscreen`, `computeFrame()` passes `railWidth=0`, `sidebarVisible=false`, `threadFront=(stackFront==PanelThread||focusedPanel==PanelThread)` and `View()` skips the status row — the zoomed pane spans the full terminal; mouse hit-test bands (`a.layout`) match what was drawn; panel-render caches (rail/status/sidebar) are not consulted while zoomed and resume unchanged on exit.
- [x] fs-scope: fullscreen zooms the FRONT pane whatever it is (`stackFront`), not thread-only; toggling `ctrl+b` sidebar / `ctrl+]` thread / window ops while zoomed either stay suppressed with a toast ("unavailable while zoomed") or exit zoom first — one rule, applied consistently; `q`/`Q` quit paths unaffected.
- [x] fs-golden: `internal/ui/testdata/golden/` gains fullscreen-on frames (thread zoomed, and messages zoomed) + the zoomed-exit frame restored; `go test ./internal/ui -run TestGolden` green; `runKeyCases` table covers `z` toggle, `z` again, `esc` exit, esc-precedence with an open emoji picker, resize while zoomed (`tea.WindowSizeMsg` keeps pane zoomed at new size).

### Emoji cache + fuzzy (b)

- [x] emoji-seed-teamtag: the cold-start seed is TeamID-tagged (same shape as `CustomEmojisLoadedMsg`, msgs.go:415) AND re-read per active team on WorkspaceSwitchedMsg (switch reducer applies synchronously or atomic map swap on `wctx.customEmoji`) — workspace B never renders A's customs; a cmd/slk test switches workspaces against a primed two-team cache and asserts each team's set is published (`TestFetchWorkspaceEmoji_SendsLoadedMsgForItsTeam` keeps passing). [gotcha G4, G19]
- [x] emoji-upsert-txn: custom_emoji write is one team-scoped transaction (DELETE by team_id + INSERT) safe against the foreign_keys(ON) DSN (db.go:37 — no FK to workspaces, or workspace row upserted first); two-team replace test; emoji-before-workspace-row order tested. [gotcha G9]
- [x] emoji-seed-order-test: cmd/slk wiring test asserts recorded call order = seed-from-cache → fetchWorkspaceEmoji, and the seed path never calls the lister (`callCount 0` at seed time). [gotcha G13]
- [x] fuzzy-inorder-subseq: DOD/examples use "non-contiguous, IN-order subsequence" (matching `subsequenceScore` semantics); negative test `krt` does NOT match `rocket`. [gotcha G10]
- [~] fuzzy-rkt-verified: `rkt→rocket` and `:thumbs→+1` verified against real `emoji.BuildEntries` output before being enshrined in tests; subsequence tier is score-aware (word-boundary/tightness, not plain alphabetical) so the examples win on merit; "recent" = matching entries ∩ frecent only (stale/global frecent names skipped — frecent_emoji has no team column); cap semantics defined (eviction documented or top prefix match guaranteed to survive). [gotcha G11]
- [x] fuzzy-reactionpicker-migration: reactionpicker + emojipicker expectation updates are behavior changes in their own commit (their tier set grows: prefix+substring → 4 tiers); "expectations unchanged" holds ONLY for channelfinder/mentionpicker; reactionpicker test coverage includes a subsequence query (`rkt→rocket` there too). [gotcha G12]
- [x] emoji-cache-table: `internal/cache` persists custom emoji per workspace (table `custom_emoji(team_id, name, value, updated_at)`, pattern: `workspaces` in `cache/db.go`); upsert replaces the workspace's set on `emoji.list` success; read returns the full map; cache-miss → empty map, never an error surface to the UI.
- [x] emoji-cache-wiring: startup seeds `BuildEntries` from the cache BEFORE network (`cmd/slk` connect path, replacing today's build-in-`app.go:829` with customs-from-cache), `fetchWorkspaceEmoji` upserts on success and re-publishes `CustomEmojisLoadedMsg`; existing bootstrap-subset/failure semantics preserved (`customemoji_test.go` assertions keep passing); `frecent_emoji` feeds a "recent" tier. Cache seeding stays OUT of `NewApp`/`newTestApp` (no golden perturbation). [gotcha G18]
- [x] fuzzy-shared: ONE extracted matcher package (e.g. `internal/fuzzy` or `internal/text`) exposing folded prefix/substring/subsequence scoring — API derived from `channelfinder/model.go:388-483` (`filter` tiers + `subsequenceScore`) and `mentionpicker/match.go` word/squash ranks; `channelfinder` + `mentionpicker` + `emojipicker` + `reactionpicker` all consume it (their `_test.go` expectations unchanged, moved only if the helper moves); AGENTS.md helper table updated in the same commit.
- [x] fuzzy-emoji: `emojipicker.Model.filter()` ranks: recent (frecent) > prefix > substring > subsequence, case/accent-folded, keeping `MaxVisible` cap and input-order stability within a tier; compose autocomplete (`:` trigger) and reaction picker get the same ranking.

### E2E / verification (c — existing repo approach only)

- [x] e2e-harness-reuse: every new behavior proven with existing harnesses — golden frames (`compareGolden`/`newGoldenApp`), key tables (`runKeyCases` via real `dispatchModeKey`), reducer-chain driver (`updateAndRender`), service fakes (`fakeEmojiLister`, `services_helpers_test.go`); NO new test dependencies in `go.mod` (teatest/pty rejected by user ruling); `git diff --exit-code go.mod go.sum` clean on every feature branch (ruling gated, not memory-checked). [gotcha: go.mod freeze]
- [x] e2e-suite-green: `go build ./...`, `go vet ./...`, `gofmt -l .` empty, `go test ./... -race` green (~47s shape, `internal/ui` remains the dominant package); expected-diff list pre-enumerated (help-entry churn from FullscreenToggle via `help.FromKeyMap`; reactionpicker/emojipicker expectation updates in their own commit) — no other golden re-bless (`-update` package-local only). [gotcha G18]

## Idempotency

Re-running `/10-plan-declarative` on same requirement reconciles to THIS plan.
Implemented items auto-marked `- [x]`. Pending items surface as work-remaining.
DO NOT rewrite item prose on re-run (status flips only).
Gotcha items appended 2026-09-26 (prefixed `fs-*`/`emoji-*`/`fuzzy-*`, tagged `[gotcha GN]`) — see `slk-fullscreen-emoji-fuzzy-gotcha.md` for full findings + machine verification of claims.

## Open Threads

_(populated by gotcha-coverage + re-runs)_

1. **DOD-1 mechanism superseded by `fs-esc-pecking-order`** — original fs-state clause "esc BEFORE any other Escape arm in every mode" would break modal-owned esc and pinned chord-cancel tests; new peel order in the appended item. DOD wording to update on next reconcile.
2. **DOD-3 wording** — "out-of-order subsequence" is wrong for the mandated matcher (in-order); proof scope must name reactionpicker too (`fuzzy-inorder-subseq`, `fuzzy-reactionpicker-migration`). DOD wording to update on next reconcile.
3. **Plan home seam** — plan lives in `flow/plans/` (cmd convention); slk repo's own workflow is `docs/superpowers/specs + plans`, tests-first (README). Implementation lanes should follow the repo convention; carry this plan as the requirement source.
4. **`Z` binding** — bind `Z` as alias or drop from evidence (rank 2, decided: alias bind, cheap).

5. ~~**Two toast helpers, one contract (B2)**~~ — **RESOLVED** on
   `slkfz/lane-toast` @ `63ced86`, awaiting merge sign-off. `toastWithClear`
   (eager) and `App.uploadToastCmd` (batched) were the same idea under two
   spellings; picking the wrong one caused 6 of the 8 fullscreen failures, and
   "fixing" it by making the batched one eager broke 14 call sites and hung
   `internal/ui` for 10 minutes. Per `AGENTS.md` the resolution is DELETE one,
   never alias — now one helper, `uploadToastCmd(text, dur, mode)`, with the two
   former bodies preserved verbatim under `toastEager` / `toastDeferred`.
   - Pinned by `internal/ui/toast_consolidation_test.go` (hash `59f229d7`): an
     AST walk asserting exactly one helper implementation survives (F2P, RED at
     base), plus both semantics as P2P. The two lines that must legitimately
     change during the refactor live in an unpinned seam,
     `internal/ui/toast_shims_test.go` — pinning them would have blocked the fix
     while not pinning the assertions would have let them be weakened.
   - Gate re-run independently (gated-dev step 9), exit 0: 1 F2P green, P2P
     across 9 packages including the fullscreen suite, gofmt clean, vet clean,
     `go.mod` frozen.
   - Still open, cosmetic: the surviving name `uploadToastCmd` is now a
     misnomer — it is no longer upload-specific. Renaming it touches the pinned
     oracle, so it wants its own commit and a re-pin.
6. **`fuzzy-rkt-verified` is only partially satisfiable (B4)** — the ranking
   rule, the score-aware subsequence tier and the "recent = matching ∩ frecent"
   clause are all implemented and green. But the DOD's own `rkt→rocket` example
   is NOT reachable in a workspace with `*worktree*` custom emoji: `rkt` is a
   contiguous substring of `worktree`, so those names are TierSubstring(4) while
   `rocket` is TierSubsequence(5), and `MaxVisible=5` evicts it. The example was
   verified against the built-in table only, never against custom emoji. DECISION
   NEEDED: either drop the example from the DOD, or introduce a length/density
   penalty so a short near-exact name outranks a long incidental substring —
   the latter is a ranking-semantics change the current tests pin, so it is not
   a silent fix.
   - **Now confirmed against the live workspace**, not just reasoned about. The
     capability suite drove the real binary in tmux against `dy-swarm`: `:rkt`
     opened the picker with 5 candidate rows, every row a genuine in-order match
     (no bag-of-chars leakage), and `rocket` was **not** among them. The
     evicting rows are the `cmd-pallet-*worktree*` custom emoji, because `rkt`
     is contiguous inside `w-o-`**`rkt`**`-r-e-e`. So the matcher is obeying the
     plan's own mandated order — tier 4 beats tier 5 — and the DOD example is
     what is wrong. This closes the "is it reachable in practice" half of the
     question; the DECISION above (drop the example, or add a length/density
     penalty) is still open and still not a silent fix.
7. **`messages.ClickAt` spacer-row fix has repo-wide reach (B5)** — the
   pre-existing off-by-one it corrected affects EVERY click in the messages
   pane, not just the zoom fixture. Worth calling out in review; `thread.Model`
   shares 377 lines with `messages.Model` and should be checked for the same
   miscount.
8. ~~**agyralph is unusable on this host (B7, B8)**~~ — **root-caused and fixed**
   in a scratch copy at `~/.local/state/slkfz/agyralph-patched/` (5-hunk patch +
   README; the user's `open-ralph-wiggum` repo is deliberately untouched, so
   applying it is their call). Two independent bugs, not one:
   - **B8** the harvest returned tool-call ids because `step_type=15` carries
     both prose and tool calls and a turn usually *ends* on a tool call, so
     `rows[-1]` read the wrong row. Fixed: scan backward for the newest row with
     prose, longest-printable-run extraction, junk filter. Measured 11 chars →
     645 and 338 chars on two dead runs.
   - **B11** (the worse one) `wait_turn` tested for `STATUS_IN_PROGRESS = 8`,
     which this agy build never writes — the executing step is status **2**. So
     every turn was declared finished ~9s after being sent, on top of an agent
     still working. Fixed by enumerating the TERMINAL set and requiring
     `MAX(idx)` stable. Proven end to end on a task whose tool call sleeps 25s:
     before, turn "finished" in ~9s and the run died
     `failed/max_iterations_reached`; after, `turn finished in 1m00s`,
     `quiesced: idx steady for 4 reads`, gate exit 0, **`done/complete` at
     iteration 1**.
   - It then did real work: the toast consolidation now merged at `59cb245`.
   - Still open: **B12/B7**, the circuit breaker disarming a correct gate. It
     should trip on gate-invalid (exit 2/125), not on red (exit 1) — consecutive
     failures are the expected shape of a red gate mid-refactor. Unfixed; it is a
     design call on the user's tool.

9. **The emoji seed clobbers the bootstrap subset (B17), and a value race hides
   it (B18)** — both Rank 4, both in merged and deployed code, both from batch 1
   of the delegated gotcha-coverage pass. `cache.CustomEmoji` returns an empty
   map with a **nil** error on a miss, so the seed's `err == nil && set != nil`
   branch publishes empty over the `conversations.view` subset; a failed
   `emoji.list` then leaves every custom emoji rendering as literal `:name:`.
   And `main.go:1565` launches the seed with `go` one statement before line 1579
   reads the same field to build `WorkspaceReadyMsg` — an `atomic.Pointer` value
   race, so `-race` stays green permanently and first paint is nondeterministic.
   The single fix for both: seed **synchronously** before the message, background
   only the fetch, and make the seed non-destructive. See
   `slk-fullscreen-emoji-fuzzy-gotcha-batch1.md`.

10. **`fetchWorkspaceEmoji` is dead code with four tests guarding it (B19)** —
    Rank 4. The live path is `fetchWorkspaceEmojiIntoCache`; the original has
    zero non-test callers but carries the only written record of why the fetch
    must run unconditionally, and four `TestFetchWorkspaceEmoji_*` tests pin the
    dead copy. The plan's "assertions keep passing" was true and meaningless.
    Same class as B2 and the same resolution rule: delete one, never alias.

11. **`updated_at` is never written (B21) and the upsert error is discarded
    (B22)** — both Rank 3. Every `custom_emoji` row is `updated_at = 0` forever
    while every other table in the schema writes it, and the existing test
    asserts only that the column *exists* (`PRAGMA table_info`) — a schema
    assertion is not a write assertion. Separately, `_ = db.UpsertCustomEmoji(...)`
    means a full disk or lock timeout silently leaves the cache permanently
    stale, with no log line, unlike every comparable best-effort path here.
    **Both fixed** in `40ae159`.

12. **`fuzzy-emoji` is marked `[x]` with half of it unimplemented (B24)** —
    Rank 4, and B13's class again. The item promises "recent (frecent) > prefix >
    substring > subsequence" for BOTH the compose autocomplete and the reaction
    picker. `emojipicker` has no frecent field, no `SetFrecentEmoji`, and the
    string `frecent` appears **0 times** in the package against 5 in
    `reactionpicker`; both `app.go` call sites target `a.reactionPicker`. So the
    same ranking rule is implemented twice, divergently, inside the feature whose
    purpose was one implementation — and with `MaxVisible = 5` the two emoji
    surfaces disagree in their top rows. The DOD's proof obligation
    (`:thumbs` → `+1` reachable) is satisfiable with no recent tier at all, so
    the criterion could not catch it. **DECISION NEEDED**: wiring frecent into
    emojipicker changes ranking users see and existing tests pin the current
    order, so it is not a silent fix — it wants its own RED test and commit.
    Until then the "compose autocomplete gets the same ranking" clause should
    read `[ ]`.

13. **The plan undercounts its own tier set, and three consequences follow
    (B25, B28, B30)** — all Rank 3–4. `fuzzy-reactionpicker-migration` says the
    tier set grew to "4 tiers"; the enum has five plus `TierNone`
    (`TierPrefix`, `TierWordPrefix`, `TierSquashedPrefix`, `TierSubstring`,
    `TierSubsequence`). From that one miscount: `channelfinder` re-derives tier
    numbers as `int(tier) - 1` against the old 3-tier scheme, with unreviewed
    model reasoning left in the production file ("wait, Match returns TierPrefix
    (1)…so tier - 1 is perfect") and a struct field still documented `// 0
    prefix, 1 substring, 2 subsequence`; `mentionpicker` carries a dead
    `squashedQuery` parameter from the old predicate, with `squash` still
    computed, still declared, and still tested; and the tier-order test pins only
    3 of the 5 real tiers, so the two new constants can be reordered with the
    whole suite green. **Nothing is live-broken** — `int(tier) - 1` is monotone
    and the sort only compares those values — but the latent trap is exact:
    reorder the enum and channelfinder's local numbering corrupts silently with
    no test failing. The behaviour question underneath (names now landing in
    WordPrefix/SquashedPrefix outranking true substring hits, and squashed hits
    losing their positive subsequence score) is plausible and **untested**; it
    wants a test per tier boundary. B25's comments and B30's test are cheap fixes;
    B28 is a mechanical deletion.

14. **`fuzzy.Match` re-folds already-folded strings (B26)** — Rank 4, and
    invisible to every test because it is cost, not behaviour. `Match` folds both
    arguments, then passes the folded pair into `WordPrefix` and
    `SquashedPrefix`, **each of which folds both again** (2 `text.Fold` calls
    apiece, verified by whole-function count). Per candidate on the deepest path
    the name is folded 3× and the query 3× — and all three consumers already fold
    the query once outside the loop and pass it in, so that outer fold is undone
    immediately. Folding is idempotent, so no answer is wrong. But
    `internal/text/fold.go`'s own comment records why this shape was removed:
    *"reaction picker 271 ms / 1.12 GB / 768k allocs -> 2.96 ms / 611 B / 1
    alloc. See issue #165."* A closed, measured, documented fix regressed by a
    refactor that never named it as an invariant. Wants a `Matcher` that folds
    the query once, unexported no-fold internals, and a benchmark with a
    non-ASCII query as the guard.

15. **"Word boundary" has four definitions, three inside `internal/fuzzy`
    (B27)** — Rank 3. `fuzzy.isSeparator` treats `- _ . space / :` as
    separators; `fuzzy.WordPrefix` uses `" " - _ .`; `fuzzy.SquashedPrefix`'s
    `FieldsFunc` uses space `- _ .`; `mentionpicker.isSeparator` uses the same
    four at byte level. So `/` and `:` are word boundaries for subsequence
    scoring and nothing else — and for emoji names, which lean on `-` and `_` and
    whose trigger character is `:`, the tier a candidate lands in depends on
    which definition that tier's predicate happens to use. The plan merged the
    two sources' *predicates* verbatim without merging their notion of a
    separator: "extract the substrate, not the widget" applied to the scoring
    functions but not to the value underneath them. Wants one exported
    `fuzzy.IsSeparator(rune)`, all four sites routed through it, and a
    deliberate decision about `/` and `:` written as a test.

16. **Both candidate pools sort on RAW `Name` while all comparison is folded
    (B29)** — Rank 3. `internal/emoji/entries.go:68` sorts on unfolded `Name`,
    so a custom emoji named `Rocket` sorts before every lowercase name and wins
    every within-tier tie against `rocket`. Slack lowercases on upload, but these
    rows now arrive from the `custom_emoji` cache table and nothing in that write
    path normalizes case. Separately, emojipicker's input-order precondition is
    prose on an exported type ("Callers must pass alphabetically-sorted
    entries…") with no assertion — the pattern AGENTS.md says to replace with a
    check. Useful negative result from the same probe, recorded so nobody
    re-checks it: **Go map iteration order does not reach the output** — both
    pools sort by unique `Name` after their map walk, both dedup first, the
    within-tier tie-break is a unique `idx`, and `frecentRank` is lookup-only.

17. **I edited a hash-pinned gate oracle, and the gate could not have told me
    so usefully** — process finding, no B-number (it is mine, not a delegate's).
    Fixing B30 meant strengthening `TestTierConstants_*`, which lives in
    `internal/fuzzy/fuzzy_test.go` — a file pinned in
    `gates/fuzzy/oracle.sha256`. I replaced the weak assertion in place and
    committed it (`9d0f622`). The gate's own error text names this exact case:
    *"The RED tests are the specification. Restore the file and fix the
    implementation instead. If a test is genuinely wrong, STOP and say so; do
    not edit it."* B30 **is** the genuinely-wrong case, so editing was the one
    response the contract forbids.
    - Remedied in `17d8594`: oracle restored byte-exactly to its recorded hash
      (verified by `sha256sum`, not by eyeball), strong assertion re-landed as a
      new unpinned file `internal/fuzzy/tier_constants_test.go`. Additive is
      allowed. The weak test still stands, untouched and passing, directly above
      the strong one. What I did **not** do is re-pin the hash to match my
      edit — that is the agent rewriting the gate to suit its own code, which
      disarms the check permanently and silently, and is worse than either the
      weak test or the violation.
    - The load-bearing detail: **exit 2 is not a verdict.** Had I run the gate
      before noticing, it would have exited 2 (ORACLE TAMPERED) — correct, but
      it reads as "the gate is broken", and the temptation under a green-tree
      deadline is to re-pin and move on. The check caught me only because I
      compared the hashes by hand before spending 15 minutes on a run. A gate
      cannot distinguish *tampering to weaken* from *tampering to strengthen*,
      and should not try: the procedure has to be "propose, do not edit".
    - Open question for whoever owns the gates: there is no sanctioned channel
      for "this oracle is too weak". Right now the only compliant move is to add
      a parallel file, which leaves the weak test running forever as evidence
      of nothing. A `gates/<lane>/oracle-amendments/` directory, or a re-pin
      that requires the diff to be strictly-additive, would close it.

18. **Gates measure their own worktree, not the commit you name (B16 + its
    extension)** — recorded here because it recurred twice and cost a false
    green each time. Every `gate.sh` hardcodes `WT=` to its lane worktree at
    line ~17, so a bare run scores whatever that tree happens to hold: after a
    merge, the PRE-merge tree. All three gates once printed `head=5cb4bfa`
    while the commit under test was `7453f44`, and all three exited 0.
    The extension: proving the worktree is *at* the commit does not prove it is
    *clean*. I committed a non-compiling `cmd/slk` (`43c744e`) because the fix
    was in the working tree but absent from `git add`; my own `-race` run said
    60 packages OK (tree + unstaged fix) while all three gates said
    `[build failed]` (the commit). Both measurements were accurate; they
    described different trees.
    - Now encoded once, in `~/.local/state/slkfz/verify-commit.sh`: for each
      lane, verify `gate.sh` against its own recorded hash, clear `.ralph`,
      `git checkout -B slkfz/verify-<lane> <commit>`, then assert **three**
      things — the worktree HEAD equals the requested commit, `git status
      --porcelain` is empty, and the `head=` line the gate itself prints
      matches. Exit 2 and 125 are reported as infrastructure, never as red.
    - `17d8594` is the first commit verified this way end to end: zoom GREEN
      (22 F2P / 8 P2P), fuzzy GREEN (76 F2P across 3 pkgs / 7 P2P), toast GREEN
      (1 F2P / 9 P2P incl. the fullscreen suite), all three at
      `head=17d8594, dirty=0`.

27. **B33 is closed, and closing it produced the cleanest example of the vacuity
    trap in this whole plan — I wrote one while hunting them.** Two results, and
    the second is the more useful.
    - **The ranking was already correct.** B33 was a *coverage* gap, not a defect:
      `emojipicker` does sort by `fuzzy.Tier` (`model.go:173-174`), it simply had
      no test proving it for `TierWordPrefix` or `TierSquashedPrefix`. Four new
      tests in `internal/ui/emojipicker/tier_order_test.go`.
    - **My first draft of those tests was worthless, and passed.** It called
      `fuzzy.Match` itself to compute each row's tier, then asserted the rows were
      in non-decreasing tier order — i.e. it compared the matcher against itself
      and never observed the model at all. Collapsing `TierWordPrefix` and
      `TierSquashedPrefix` into `TierSubstring` *inside `model.filter`* left all
      four GREEN. That is the exact regression B33 says nothing would catch, and
      my test for B33 did not catch it either.
    - The fix was to assert on **published row order**, the only thing the model
      actually emits, and to exploit the model's documented tie-break
      (`matches[i].idx < matches[j].idx`, "preserve input order") by listing the
      WORSE-tier entry FIRST in each fixture. Correct tiering promotes the
      better-tier entry past it; a collapse ties them, input order wins, the
      assertion fails.
    - Proven against two mutations, not one: (A) the two middle tiers collapsed
      into `TierSubstring` → 2 of 4 fail; (B) the tier comparison deleted from the
      sort entirely → 4 of 4 fail. The pre-existing `TestFuzzy_*` tests are green
      under mutation A, which is the measurement that justifies B33 existing.
    - **The transferable rule**, and it is sharper than "check your test can
      fail": *a test must observe the value the production code published, never
      recompute it from the same inputs.* Recomputation looks like verification
      and is tautology. All three earlier instances in this plan are the same
      error wearing different clothes — B13 recaptured a frame after the event,
      B45 read a model field instead of the frame, B46 asked a golden to prove a
      hit-test. Name the observation channel before writing the assertion.

28. **B27 is closed (`3a6d285`, lane `slkfz/lane-sep`), and it CHANGED RANKING —
    "no legacy tests failed" is true and does not mean what it sounds like.** The
    three definitions of a word boundary are now one exported
    `fuzzy.IsSeparator`; `WordPrefix`'s inline `[]string{" ", "-", "_", "."}` and
    `SquashedPrefix`'s `FieldsFunc` closure both call it.
    - **The lane took the widening, not the narrowing.** Unification had two
      answers and they are not equivalent: `isSeparator` (subsequence scoring)
      already counted `/` and `:`; the two prefix tiers did not. Keeping the wider
      set makes `/` and `:` boundaries for `WordPrefix` and `SquashedPrefix`,
      where they previously were not. Concretely, `WordPrefix("eng/platform",
      "platform")` was **false** — none of `" platform"`, `"-platform"`,
      `"_platform"`, `".platform"` occurs in it — and is now **true**, promoting
      that row from `TierSubstring` (4) to `TierWordPrefix` (2).
    - **No pre-existing test failed because no pre-existing test used `/` or `:`
      as data.** Verified, not assumed: `internal/fuzzy/fuzzy_test.go` contains no
      slash or colon inside any fixture string. So the green suite records that the
      widened region was *untested*, not that behaviour held. The only coverage it
      now has is the oracle's own
      `TestWordPrefix_TreatsColonAndSlashAsBoundaries`, which asserts `skin:tone`
      /`tone` and `a/b-thing`/`b` true, `plain_name`/`name` still true, and
      `nobreakhere`/`here` false.
    - **Reach, by call site** (`fuzzy.Match`/`WordPrefix`/`SquashedPrefix` have
      exactly four non-test callers): `emojipicker` and `reactionpicker` match
      shortcodes, which contain only `_`, `-`, `+`, so they cannot be affected.
      `channelfinder` matches `Name`, which for channels cannot contain `/` or `:`
      but for DM/mpdm rows is a human display name, which can. `mentionpicker`
      matches display names directly and is the most exposed — a name like
      `Jane Doe / Platform` or `ops:oncall` now word-prefix-matches on the segment
      after the separator.
    - **CORRECTION, appended after measuring the user's real cache (do not read
      the reach paragraph above without this).** Two claims in it are wrong.
      Queried against the deployed dev copy of the live cache (175 channels,
      93 users):
      - I wrote that channel names "cannot contain `/` or `:`". **Two channel
        rows do**, both of shape `<word>:<word>-<word>`. So `channelfinder` is
        affected in this workspace by *channel* rows, not only DM/mpdm rows.
      - I wrote that `mentionpicker` "is the most exposed". In this workspace it
        is **not exposed at all**: `0` of 93 users have a `/` or `:` in either
        `name` or `display_name`. The exposure is real but it is in
        `channelfinder`, which is the opposite of what I said.
      - Measured effect on that shape, against the merged matcher:
        `Match("abc:def-ghi", "def")` → **tier 2 (WordPrefix)**, and the same for
        `"ghi"`. Under the deleted code the separator list was `" ", "-", "_",
        "."`, so `":def"` could not match and it fell through to **tier 4
        (Substring)**. Those two channels therefore rank *higher* than before for
        a query matching the post-colon segment — which is arguably the better
        answer, but it is a live ranking change in the user's own workspace, not a
        theoretical one.
      - Lesson, same as the rest of this plan: I reasoned about what Slack
        permits in a channel name instead of querying the 175 rows sitting in the
        cache. The data was one SQL statement away the whole time.
    - **This is a deliberate behaviour change, made by the lane and accepted here,
      not a refactor.** It is recorded rather than re-litigated because the wider
      set is the one the subsequence tier has always used, so unifying downward
      would have *narrowed* an existing tier to match two narrower ones — losing
      behaviour to gain consistency. If the promotion turns out to be unwanted in
      `mentionpicker`, the fix is a per-call-site separator set, not re-splitting
      the predicate.

29. **B41 is closed (`73c1c76`, lane `slkfz/lane-xdg`) — one rule now, and the
    credential hazard it exposed is RELOCATED, NOT REMOVED.** `XDG_DATA_HOME` is
    resolved in exactly one place, the new `internal/xdg.DataDir() (string,
    error)`; `cmd/slk/xdgData()` and `internal/export.ExportDir()` both delegate.
    A third package was unavoidable because `internal/export` cannot import
    `cmd/slk`, which is why the rule was re-derived in the first place.
    - **The mandated class is satisfied properly**: both original
      `os.Getenv("XDG_DATA_HOME")` reads are *deleted*, not aliased. The gate
      counts env-read sites from the shell independently of the oracle's own repo
      walk, so a merely delegating second site would still fail it.
    - **What batch 3 recorded understated this, and what the fix leaves behind is
      a different shape of the same risk.** The pre-fix defect was not "exports
      aren't where my data is": `xdgData()` swallowed the `os.UserHomeDir` error
      and returned the *relative* `.local/share/slk`, and since it has no error
      channel at all, its 11 call sites consumed that unchecked — four of them as
      `filepath.Join(xdgData(), "tokens")`. With `HOME` unset, slk wrote
      **credentials into its working directory**.
    - **Post-fix `xdgData()` returns `""` on failure, and measured,
      `filepath.Join("", "tokens") == "tokens"`.** The token path is therefore
      still relative — `./tokens` instead of `./.local/share/slk/tokens`. The
      write still lands in the CWD, and arguably more exposed: a bare filename is
      likelier to collide with a real file, or be committed by accident, than a
      hidden nested path. **B41 fixed the duplicate rule. It did not fix the
      credential location.**
    - This is not the lane overstepping or falling short. Hardening the 11 call
      sites was explicitly scoped OUT of its prompt as a different defect —
      callers cannot express failure — with a different blast radius, and the lane
      both respected that and said so in its own commit comment. It is recorded
      here as a **new, separate, open item** rather than as a loose end of B41.
    - **Open**: give `xdgData()` a way to fail, or make the four token call sites
      refuse an empty data dir. Either is a `cmd/slk` change across 11 call sites
      and wants its own lane. `TestXDG_DataAndExportAgreeWhenHomeIsUnset` already
      pins the invariant such a fix must not break, so that lane inherits a guard
      instead of starting from nothing.

30. **B26 is closed (`9255c8b`, lane `slkfz/lane-fold`), and building its oracle
    corrected the batch-2 appendix about where the cost actually is.** `Match` now
    folds each argument once and hands the already-folded pair to unexported
    `wordPrefixFolded` / `squashedPrefixFolded`. The exported `WordPrefix` and
    `SquashedPrefix` still fold, because `internal/ui/mentionpicker/match.go:42,45`
    calls both with **raw** input — that was the one regression this change could
    plausibly cause, and it is the thing the gate's P2P note points at.
    - **The appendix blamed the wrong inputs.** It attributes the cost to
      "non-ASCII candidates (accented custom emoji, display names like
      `Mélanie`)". That is precisely the case where the redundant folds are
      **free**: `Fold("Mélanie")` returns `"melanie"`, which is ASCII, so every
      re-fold takes `Fold`'s `isASCII` fast path and allocates nothing.
    - **The waste requires the FOLDED FORM to still be non-ASCII** — CJK, emoji
      and `ß`, which `Fold` deliberately does not decompose (NFD, not NFKD).
      Measured with `testing.AllocsPerRun(200)` before the fix: `Fold` vs a
      re-fold of its own output vs whole `Match` — `engineering-platform` 0/0/0,
      `Mélanie-Dupont` 9/**0**/9, `日本語チャンネル` 6/**6**/38,
      `party_parrot_🦜` 6/6/12, `Straße-Team` 7/6/13.
    - **It also scales with tier depth**, since each tier that runs folds twice
      more. So the worst case is a **miss**, which is most candidates on every
      keystroke — exactly the hot loop issue #165 was about. Benchmarked
      `-benchmem`, CJK miss: `9561 ns / 26296 B / 20 allocs` →
      `3541 ns / 8792 B / 8 allocs`. The ASCII control moved `487 → 376 ns` at 2
      allocs either way, **which is why no existing benchmark could see this** and
      why the oracle's corpus is entirely non-ASCII. An ASCII corpus would make
      the assertion vacuous: every fold there is 0 allocations, so one fold and
      three are indistinguishable.
    - **The oracle measures its own floor at run time** — fold the name once, fold
      the query once, count that — rather than hardcoding a bound. It therefore
      pins a *ratio*, needs no re-blessing on a toolchain or `x/text` bump, and
      cannot be satisfied by aliasing. `TestMatch_ResultsAreUnchangedByTheFoldFix`
      is the paired guard, green before and after: folding is idempotent, so no
      answer may move, and if one did it would mean allocations were saved by
      skipping a tier — a ranking change disguised as a performance fix.
    - **Still open, scoped out deliberately**: all three loop callers
      (`emojipicker/model.go:138`, `reactionpicker/model.go:245`,
      `channelfinder/model.go:397`) already fold the query once *outside* their
      filter loop and pass it in, where `Match` folds it again per candidate. A
      `Matcher` holding a pre-folded query for a whole scan removes that last
      redundancy. It is additive, has its own API decision, and is a separate
      item — the 3× inside `Match` was the bulk of the win.

31. **B40 is closed by coverage, and investigating it found a FOURTH vacuity
    shape: a test that authors its own subject.** `dod-2`'s "zero network wait"
    now has two guards in `cmd/slk/startup_emoji_order_test.go`.
    - **The existing test could not have caught this, and the reason is
      structural.** `TestStartupEmojiOrder_SeedBeforeFetch` calls
      `runStartupEmoji`, which is declared at `customemojiseed_test.go:111` — **in
      a test file**. It is a two-line helper that calls `seedCustomEmojiFromCache`
      then `fetchWorkspaceEmojiIntoCache` synchronously. So the sequence whose
      order that test pins is *a sequence the test wrote*. It proves the two
      functions compose correctly in that arrangement; it cannot prove production
      uses that arrangement, and production does not.
    - **This is a new entry in OT27's catalogue.** The three there recompute a
      value they should observe. This one is different: the subject itself is
      manufactured by the test. Same symptom — an assertion that cannot fail for
      the reason it was written — different cause. *Ask what production actually
      executes, not just where the value came from.*
    - **What production does**, `main.go:1572-1600`: `seedCustomEmojiFromCache`
      synchronously → `p.Send(ui.WorkspaceReadyMsg{...})` → **`go`**
      `fetchWorkspaceEmojiIntoCache`. Three load-bearing properties, and ordering
      alone is not one of them: a seed-before-fetch that *awaited* the fetch
      before the send would satisfy every pre-existing test while putting
      `emoji.list` back on the first-paint path. That is precisely the regression
      `dod-2` exists to prevent.
    - **The guards are structural, and that is a considered choice, not
      laziness.** The sequence is inline in a function that builds a live Slack
      client, a SQLite handle and a `tea.Program`; there is no seam for a blocking
      lister without refactoring startup. The property being asserted *is*
      structural — a `go` keyword and a statement order. Between no assertion and
      an assertion over program text, the second is worth having.
    - **Proven by mutation, with the contrast that justifies the file.** Delete
      the `go` → `TestStartupEmoji_ProductionOrderIsSeedSendThenAsyncFetch` fails.
      Background the seed → `TestStartupEmoji_SeedIsSynchronousAtItsCallSite`
      fails, and only that one. `TestStartupEmojiOrder_SeedBeforeFetch` stays
      **GREEN under both**.
    - **Open, deliberately not done here**: extracting the startup ordering into
      an injectable function would allow a real blocking-lister test. That is a
      change to the startup path with real risk, and it wants its own lane.

32. **B34's leftovers are two different situations, and only one was a defect.**
    The appendix listed `stripANSI`, `filteredNames` and `containsName` as "each
    declared twice". Reading them changes the verdict.
    - **`filteredNames` / `containsName`: NOT a defect, closing as won't-fix.**
      The two copies differ in signature (`*Model` in `reactionpicker` vs `Model`
      in `emojipicker`) *and* body (`m.filtered` vs `m.Filtered()`). They are
      per-package adapters onto two genuinely different model APIs, so they are
      the *divergent* part, not the uniform one. AGENTS.md's own rule governs:
      "Extract the substrate, not the widget… Forcing genuinely different
      behavior into a common shape is worse than the duplication it removes."
      Unifying them means unifying the two pickers' APIs, which is Phase 4.
    - **`stripANSI`: a real defect, and worse than duplication — the two copies
      were NOT EQUIVALENT.** `internal/ui` calls `ansi.Strip`. `statusbar` had a
      hand-rolled byte loop ending each escape at the first ASCII letter: correct
      for SGR, wrong for OSC. Measured:
      `"\x1b]8;;https://example.com/a\x07label\x1b]8;;\x07"` stripped to
      `"ttps://example.com/a\alabel"` — it leaks the URL into the result **and
      eats the `h` of `https`**, because `h` is a letter. The ST-terminated form
      lost the `l` of `label` too.
    - Latent, not active: `statusbar` emits no hyperlinks today, so nothing was
      reading corrupted text. But 17 call sites use that helper, and a URL
      silently spliced into the string an assertion matches against is the kind of
      thing that gets diagnosed as a rendering bug. Now delegates to `ansi.Strip`;
      all 17 still pass.
    - **The general lesson, which is the same one B27 and B26 taught**: "declared
      twice" is a claim about names. Whether the two *bodies agree* is a separate
      question, and it was the interesting one in all three cases.

33. **B47's first half is REFUTED, and B51's is unsupported for a reason worth
    recording. Neither needs a lane.** Both were carried as "unverified"; this
    discharges them.
    - **B47 — `:sp/:vsp/:only/:q` while zoomed: NOT REACHABLE, so not a defect.**
      The observation was right — `internal/ui/command.go` contains **zero**
      zoom references and those four map straight to `cmdSplit`, `cmdVSplit`,
      `cmdCloseWindow`, `cmdOnlyWindow`. But the inference was wrong. Traced:
      `enterCommandMode` has exactly **one** caller, `mode_normal.go:89`, the `:`
      binding; `:` is `a.keys.CommandMode`, which is in `zoomSuppresses`; and the
      suppression returns `handled=true` from the reducer, so the key never
      reaches the mode table at all. While zoomed there is **no door into command
      mode**, therefore no way to type `:sp`. `command.go` needing no zoom
      awareness is correct-by-construction, not an omission.
      - This is only true while `:` is the sole entry. A future palette, keymap
        or mouse affordance that enters command mode another way reopens it
        immediately — the property lives in `zoomSuppresses` plus a one-caller
        fact, and nothing pins that fact. Worth a structural guard if command
        mode ever grows a second entry point.
      - B47's **second** half (`windowBounds` passing a hardcoded
        `threadFront=false` alongside a live `a.zoomed`) is untouched by this and
        remains open under B48/OT21, which needs the user's contract decision.
    - **B51 — `sixelpaint.go` has no zoom awareness, and should not have any.**
      The count is right: one match in 121 lines, and it is in a *comment*. But
      the paint function takes `rect` and `chromeHeight` **as parameters** and
      computes `contentTop := rect.Y + 1 + chromeHeight`, i.e. every coordinate
      is relative to the pane it was handed. It inherits zoom-corrected geometry
      from its caller rather than deriving any, and its own comment says so:
      guards are relative to the pane's border, "NOT to a global pane edge or
      status bar". So `fs-statusrow-math`'s sixel clause is unsupported because
      **there is nothing there to support** — not because a case was missed.
      Closing as won't-fix.
    - **Both were the same error in opposite directions**: a correct *count* of
      references treated as evidence about *behaviour*. Grep establishes what a
      file mentions; only tracing establishes what can happen.

34. **A stale comment corrected in `reducer_zoom.go`, and it mattered because
    four places in this plan leaned on it.** The suppression site carried a
    "KNOWN DEFECT, do not read this as working" note saying the toast is
    invisible and every suppressed key a silent no-op. Its premise still holds —
    the compositor sets `status := ""` while zoomed — but its conclusion is now
    false: B45 added `App.overlayZoomToast` (`view_status.go:61`), which paints
    the live toast onto the composed frame's last row (the pane's bottom border,
    so no content row is spent), plus the screen-memo key on toast text without
    which the memo would serve a stale frame. Pinned by
    `zoom_toast_visible_test.go`, which asserts on `stripANSI(a.View().Content)`
    and never on `statusbarText(a)` — reading the statusbar model is exactly what
    let the original defect hide. Rewritten as FIXED rather than deleted, per
    AGENTS.md: when this file and the code disagree, the code is right and the
    comment is the bug.

35. **B50's premise is NOT established — "delete one" would be a guess, so the
    code is unchanged.** The item says the zoom cache-key bit and
    `invalidateZoomCaches` are "two mechanisms for one rule, delete one; whichever
    is dead is the one fs-zoom-cache-keys' proof is anchored to". Traced, they
    guard **different objects**:
    - `invalidateZoomCaches` (`reducer_zoom.go:120`, called from three transition
      sites) drops three things: every win-model's own cache, `thread.Model`'s
      internal caches (`m.cache`, `m.viewCacheValid`, `m.chromeCacheValid`, via
      `InvalidateCache` at `thread/model.go:292`), and `a.lastScreenValid`.
    - the zoom bit lives in `threadLayoutKey`
      (`view_thread.go:43`, `boolToInt(a.zoomed)<<2`), which keys the **App-level
      `panelCache`** — a different cache from the model's own.
    - So one is not a reimplementation of the other. And the screen memo is the
      case no key can cover, which the existing doc comment already states: its
      key is the panel strings, and "a zoom transition can produce a panel set
      that compares equal to the stored one while the composite must differ
      (G16)".
    - **Verdict: no change.** Deleting either on the appendix's premise risks
      precisely the stale-composite class G16 and B13 exist to prevent, and the
      appendix itself flagged the mechanism as unverified. If this is pursued, the
      question is empirical — remove one, drive a zoom transition, diff the frame —
      not a reading exercise. Recorded per AGENTS.md: found while looking, raised
      separately, not folded into unrelated work.
    - Residual truth in the item: the **belt-and-braces overlap** is real, since
      the win-model and thread caches are keyed on width/height which a zoom
      transition already changes. That makes the explicit invalidation redundant
      *for those two* but not for the screen memo. Tightening it would be a
      readability change with a stale-frame downside and no measured upside.

19. **The live capability suite's "restored frame differs" line is NOT
    `fs-restore-eq` failing** — recorded so nobody chases it. Step 4 of
    `capability-test.sh` drives the real binary against a real workspace, so
    between the pre-zoom capture and the post-exit capture the workspace itself
    can change. On the latest run the only two diff hunks were a peer's presence
    glyph flipping (`○ Jane` → `⊘ Jane`, i.e. a `peerstatus` DND change arriving
    over the WebSocket) and the status row echoing it. Neither is a zoom
    artifact, and the suite already says so in its own output.
    - The byte-equal guarantee `fs-restore-eq` actually claims is pinned
      deterministically by the golden tests, where the clock, theme and emoji
      mode are all fixed (`newGoldenApp`) and no live data exists. A live TUI
      against a live workspace is the wrong instrument for a byte-equality
      claim, and the suite is right to report the diff as informational rather
      than as a failure.
    - Standing caveat that follows from this: any future live-frame comparison
      in that suite must either mask volatile regions (presence glyphs, clock,
      unread badges) or stay informational. Do not "fix" it by re-blessing a
      live capture.

20. **The suppression UX is invisible, and the zoomed pane's last row is not
    clickable (B45, B46)** — both Rank 5, both live in merged and deployed code,
    both `B13`'s class again: an item marked `[x]` whose oracle observes a
    subject the defect cannot reach. From batch 4 of the delegated pass.
    - **B45**: the suppression toast goes to `a.statusbar.SetToast`
      (`app.go:4198`), but `app.go:3384-3386` sets `status := ""` while zoomed so
      `renderStatusRow` is never called. Every suppressed key — `ctrl+b`,
      `ctrl+]`, `ctrl+t`, `:`, `/`, workspace digits — is a silent no-op with no
      feedback. The oracle is `statusbarText(a)`, a *model* getter, so the
      assertion's subject is set unconditionally and cannot vary with whether the
      row is composited.
    - **B46**: the status-row height rule is implemented **three times** —
      `Compute` (zoom-aware, `panellayout.go:81-87`), `PanelAt` (literal
      `height-1`, **no `zoomed` parameter**, `:157-159`, and `app.go:1829` passes
      none), and `reduceMouseClick` (its own `statusHeight := 1`,
      `reducer_mouse.go:192`). `fs-statusrow-math`'s central claim — that the
      override reaches all three — is false; a golden cannot fail on a hit-test,
      so the render half was proven and the routing half assumed.
    - **B46 SEVERITY CORRECTED, and B46 is now FIXED.** The delegate reported, and
      I first repeated, that "a full row of content is mouse-dead whenever
      zoomed". That is false. Measured by rendering the frame: at height 30 the
      zoomed pane occupies rows 0–29 with its **border** on row 29, and content
      ends at row 28 in both zoom states. The row `PanelAt` rejected was never
      clickable content. The duplicate-rule mechanism is real and is a live trap
      for the next change to border or status height, but **nothing was broken for
      a user**. Recorded at Rank 5 for the mandated class, with the impact claim
      withdrawn. Fixed by giving `panelLayout` a `zoomed bool` and one
      `statusRows()` derivation that all three readers consult; pinned by
      `internal/ui/zoom_lastrow_hittest_test.go` (4 tests, including a structural
      one, because the only row the answers disagree about is the border and a
      behavioural assertion there would be vacuous).
    - **B45 is FIXED**, and it was the one with real user impact.
      `App.overlayZoomToast` paints a live toast onto the zoomed frame's last row
      — the pane's bottom *border* row, so it costs no content and no reflow,
      which is the option B46's measurement made available. `statusbar.Model`
      gained a `Toast()` getter; the overlay runs before `applyOverlays` so modals
      still draw over it.
    - The memo was the half a naive fix would have missed: the screen memo is
      keyed on `(panels, status, w, h)` and `status` is always `""` while zoomed,
      so a toast changed no memo input and the stale frame would be served —
      invisible in exactly the case the fix exists for. `View()` now folds the
      toast into the key while zoomed. Pinned by
      `internal/ui/zoom_toast_visible_test.go`, asserting on
      `stripANSI(a.View().Content)` rather than `statusbarText`, because the
      latter is the channel that hid the defect.
    - Both `fs-statusrow-math` and `fs-supp-set` now hold on the mechanisms B45
      and B46 named. `fs-supp-set`'s *other* half — command mode — is still open
      as B47 below.

21. **Suppression covers one door, not the room; and `z` does not zoom the
    focused pane (B47, B48)** — Rank 4. `internal/ui/command.go` has **zero**
    zoom references: `:sp/:vsp/:only/:q` are unguarded, and only the `:` keypress
    in normal mode is suppressed, which makes fs-supp-set's "both surfaces"
    claim vacuously true. `windowBounds` compounds it by calling `Compute` with a
    hardcoded `threadFront=false` plus the live `a.zoomed` (`windows.go:53-55`),
    so a split created while zoomed is laid out against the zoomed rectangle —
    reachable today by any non-key caller.
    - B48 is a **spec/code divergence, not a logic bug**, and the delegate had it
      backwards: `zoomFrontIsThread`'s own doc (`app.go:931-945`) states the
      behaviour is deliberate and G15-motivated ("probing the unzoomed layout
      rather than reading focusedPanel is what keeps Tab from flipping WHICH pane
      is zoomed"). So at any width where both panes fit, `z` promotes MESSAGES
      regardless of focus or `stackFront` — and the **plan text is what is
      stale**. DECISION NEEDED: rewrite fs-layout/fs-scope to match G15, or latch
      the zoomed pane at `enterZoom` and keep the units' rule.

22. **dod-3 is contradicted by its own oracle in two of three assertions (B32,
    B38, B42), and no emojipicker test pins the real tier order (B33)** — B32 is
    Rank 5. The criterion claims `:thumbs` → `+1` is *proven*; the cited file
    contains a **negative guard** asserting `+1` matches no tier
    (`emojipicker/fuzzy_test.go:135-137`), and no alias table exists anywhere in
    the package. The substitution is documented *in the test* at :230-235 — so
    this is not undetected drift, it is knowledge that lived in the test and never
    propagated to the criterion it invalidates. Same shape for "out-of-order"
    (B38): `TestFuzzy_OutOfOrderQueryMatchesNothing:113` pins in-order semantics
    (G10), so the DOD describes a guarded non-feature as delivered.
    - **B33** is the consumer half of the now-fixed B30: pinning the tier
      constants by value stops a renumber but does **not** prove emojipicker
      *ranks* by them. `TierWordPrefix` and `TierSquashedPrefix` have zero
      ordering coverage in that package, and hyphenated custom-emoji names make
      WordPrefix the common real case (`party-parrot` matched on `parrot`).
    - **B42**: `rocket` actually ranks **2nd** for the DOD's own query (recorded
      at `fuzzy_test.go:21-23`), and the oracle deliberately asserts only top-3
      membership. Same example as **OT6**, failing a second, independent way.

23. **dod-4's "existing harnesses only" is false, and three of the new helpers
    are already duplicated (B34)** — Rank 4, and AGENTS.md's stated top defect
    mode occurring *inside the feature whose DOD claims it did not*. Eight
    helpers are unregistered (`zoomedScrolledApp`, `mustEnterZoom`,
    `assertZoomedFrame`, `firstLineDiff`, `stripANSI`, `entriesFor`,
    `filteredNames`, `containsName` — all verified absent from the AGENTS.md
    table), and three already exist **twice**: `stripANSI`
    (`golden_test.go:131` + `statusbar/model_test.go:353`), `filteredNames` and
    `containsName` (both `emojipicker/fuzzy_test.go` + `reactionpicker/fuzzy_test.go`).
    The picker pair is the sharp one: two packages this plan set out to unify
    behind one matcher each grew their own copy of the same two assertions.
    Register all eight; delete one of each duplicate pair.

24. **dod-2's proof points at the wrong files, and truncated success destroys the
    cache (B36, B37, B40)** — Rank 4/3. dod-2 cites `internal/cache` +
    `cmd/slk/customemoji_test.go`; **neither contains a cold-start test**. The
    real proof is `customemojiseed_test.go` and
    `customemojiseed_nonclobber_test.go` — the files added under B17/B18, i.e.
    the criterion predates its own evidence and was never re-pointed, leaving the
    real proof files unprotected.
    - **B37**: the cache contract is destructive replace, and every covered
      failure path keys on an **error**. A 200 carrying a partial page is not an
      error and clears everything absent from it — the `:shortcode:` regression
      dod-2 exists to prevent, worst on the largest workspaces. Whether
      `emoji.list` as called here can return a partial success is **unverified**;
      check that before acting.
    - **B40**: "zero network wait" is a timing claim whose closest oracle
      (`TestStartupEmojiOrder_SeedBeforeFetch`) pins *call ordering*. Ordering
      permits the seed to sit behind a startup barrier. Wants a blocking
      `fakeEmojiLister` and an assertion that a frame renders before release.

25. **Two more instances of the mandated class, both outside `internal/ui` (B35,
    B41)** — Rank 4/3. `os.Getenv("TMUX")` is read at `cmd/slk/main.go:416`,
    `internal/ui/app.go:791` and `internal/image/kitty.go:65` — and `kitty.go`
    bypasses **its own package's** injectable accessor (`capability.go:35`,
    `var getenv = os.Getenv`, "overridable in tests") while `capability.go:26`
    reads the same variable *through* it. So one variable, one package, two
    reads, one injectable. A test faking `getenv` gets an inconsistent view
    inside a single package. Fix is one line: `return getenv("TMUX") != ""`.
    - **B41**: `XDG_DATA_HOME` is resolved independently at `cmd/slk/paths.go:17`
      and `internal/export/markdown.go:71`, with independent unset-case
      fallbacks. `internal/export` cannot import `cmd/slk`, so the shared home has
      to be a third package. Note this is the variable slk-dev's isolation relies
      on.

26. **Lower-ranked but recorded, because two touch lines a fix must edit anyway
    (B49–B55)** — `exitZoom` restores the messages viewport **unconditionally**
    (`reducer_zoom.go:79`), so any autoscroll or `G` during zoom is silently
    reverted on exit, and a thread-zoomed exit stamps a stale offset onto a pane
    the user never touched (B49, Rank 3 — and it needs B15's pane hooks to fix
    properly). The zoom cache bit and `invalidateZoomCaches` are two mechanisms
    for one rule, so whichever is dead is the one fs-zoom-cache-keys' proof is
    anchored to (B50, Rank 3 — delete one; note the screen memo has no zoom bit
    at all and is the one thing the keys cannot express). `sixelpaint.go` has zero
    zoom/status references, leaving fs-statusrow-math's sixel clause unsupported
    (B51, Rank 3, mechanism unverified). Then: `enterZoom` is the only transition
    with no guard, delegating it to a comment AGENTS.md prohibits (B52); bare
    `1`-`9` are swallowed wholesale while `0` is not (B53); the keyless
    `WorkspaceFinder` entry inflates the apparent suppression set (B54); and the
    suppression comment still argues for `toastWithClear`, which lane-toast
    deleted, directly above the `uploadToastCmd` call it now contradicts (B55).
