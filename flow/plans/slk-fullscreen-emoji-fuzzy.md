# slk fullscreen pane + emoji fuzzy cache

> Plan ID: `slk-fullscreen-emoji-fuzzy`
> Created: 2026-09-26 · Last reconciled: 2026-09-26
> Status: pending
> Branch: main (2b43b29) — implement lanes on fork branches, PR per lane
> Location: flow/plans/slk-fullscreen-emoji-fuzzy.md
> Items: 26 total (0 implemented, 26 pending) — 11 original + 15 gotcha-appended (G1–G19 consolidated; see gotcha doc)

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
- [ ] `z` toggles the front content pane (thread or messages) fullscreen — rail, sidebar and status row hidden; `esc` (and `z`) exit; prior pane state restored. Proven by golden frames + key-table tests in CI (`go test ./internal/ui -race`).
- [ ] Workspace custom emoji survive restart: cold start renders custom emoji from SQLite with zero network wait; `emoji.list` refresh updates the cache. Proven by `internal/cache` + `cmd/slk` tests using `fakeEmojiLister` (existing, `customemoji_test.go`).
- [ ] Emoji autocomplete finds emojis by substring and out-of-order subsequence, ranked (recent > prefix > substring > subsequence), accent/case-folded. Proven by emojipicker tests (`:thumbs` → `+1` reachable, `rkt` → `rocket`).
- [ ] All feature tests use existing harnesses only — `compareGolden`, `runKeyCases`, `newTestApp`/`newGoldenApp`, fakes in `services_helpers_test.go`/`customemoji_test.go`; `go vet ./...`, `gofmt -l .` empty, full `-race` suite green.

## Tasks

### Fullscreen pane (a)

- [ ] fs-esc-pecking-order: esc peel order while zoomed is explicit and test-pinned: modal modes own esc (zoom persists; help/finder/confirm rows included) > insert protective arms (upload-toast, edit-cancel) > zoom-exit > picker-close/insert-exit/chord-cancel/reaction-nav/thread-close; ctrl+w-chord esc-cancel (`windows_chord_test.go`) and reaction-nav esc behavior unchanged when NOT zoomed. [gotcha G1, G15]
- [ ] fs-statusrow-math: zoomed pane spans the FULL terminal height — statusHeight threading or height-override reaches `panelLayout.Compute` AND `PanelAt` AND every parallel mouse router (wheel/click End-accessor chains) AND sixel/kitty placement bounds; golden shows content on the last row. [gotcha G2, G17]
- [ ] fs-zoom-invariant: fullscreen auto-clears when the zoomed pane closes or the view switches (q-close, ThreadClosedMsg, workspace switch app.go:2194, channel jump, ctrl+a); Enter/click thread-open while zoomed flips zoom to the thread (never opens invisibly); under zoom threadFront derives from stackFront only (Tab/click does not flip the zoomed pane; matches `threadInFront()` :898). [gotcha G5, G15]
- [ ] fs-toast-zoomed: toasts and chord hints remain visible while zoomed (transient overlay strip or temporarily un-hidden status row); suppression toast is asserted visible in a zoomed frame. [gotcha G6]
- [ ] fs-insert-z-types: `z` in insert mode types 'z' into compose (zoomed and unzoomed) — binding lives in the normal-mode map only. [gotcha G8]
- [ ] fs-esc-test-layering: esc/precedence/toast proofs ride the REAL reducer chain — esc rows via `updateAndRender`/`a.Update` (never `runKeyCases`, which bypasses the chain per its own doc); toast rows execute the returned `tea.Cmd` (snooze-test pattern); `z` toggle rows may use `runKeyCases`. Comment in the table points at the bypass caveat. [gotcha G3]
- [ ] fs-restore-eq: `frame(z,z) == frameBefore` byte-equal assertion on a fixture with more messages than fit, scrolled mid-history, with a selection; zoomed golden set includes thread-zoomed + messages-zoomed-with-thread-open-behind + resize-while-zoomed-then-exit. [gotcha G14]
- [ ] fs-zoom-cache-keys: screen memo/panel-cache keys include `fullscreen` (no stale non-zoomed composite, no dead width-keyed entries); the zoomed pane's own render stays cached at zoom width. [gotcha G16]
- [ ] fs-supp-set: the one suppression rule covers BOTH surfaces — keys (ctrl+b, ctrl+], ctrl+w chord suppressed at prefix-arm, 1-9, ctrl+a, ctrl+t, /, :) and command mode (`:sp/:vsp/:only/:q`); `:q`=close window vs `q`=close thread vs `Q`=quit documented; suppressed-at-prefix means the chord never arms while zoomed. [gotcha G7]

- [ ] fs-binding: `z` is bound in `internal/ui/keys.go` (`FullscreenToggle`, help "zoom pane in/out") and listed in the help view (`help.FromKeyMap` picks it up; no other mode claims bare `z`).
- [ ] fs-state: `App` carries `fullscreen bool`; toggling flips it and requests a render; `esc` while fullscreen clears it BEFORE any other Escape arm in every mode that can see a fullscreen pane (normal + insert; insert must NOT treat it as picker-close/insert-exit while zoomed).
- [ ] fs-layout: with `fullscreen`, `computeFrame()` passes `railWidth=0`, `sidebarVisible=false`, `threadFront=(stackFront==PanelThread||focusedPanel==PanelThread)` and `View()` skips the status row — the zoomed pane spans the full terminal; mouse hit-test bands (`a.layout`) match what was drawn; panel-render caches (rail/status/sidebar) are not consulted while zoomed and resume unchanged on exit.
- [ ] fs-scope: fullscreen zooms the FRONT pane whatever it is (`stackFront`), not thread-only; toggling `ctrl+b` sidebar / `ctrl+]` thread / window ops while zoomed either stay suppressed with a toast ("unavailable while zoomed") or exit zoom first — one rule, applied consistently; `q`/`Q` quit paths unaffected.
- [ ] fs-golden: `internal/ui/testdata/golden/` gains fullscreen-on frames (thread zoomed, and messages zoomed) + the zoomed-exit frame restored; `go test ./internal/ui -run TestGolden` green; `runKeyCases` table covers `z` toggle, `z` again, `esc` exit, esc-precedence with an open emoji picker, resize while zoomed (`tea.WindowSizeMsg` keeps pane zoomed at new size).

### Emoji cache + fuzzy (b)

- [ ] emoji-seed-teamtag: the cold-start seed is TeamID-tagged (same shape as `CustomEmojisLoadedMsg`, msgs.go:415) AND re-read per active team on WorkspaceSwitchedMsg (switch reducer applies synchronously or atomic map swap on `wctx.customEmoji`) — workspace B never renders A's customs; a cmd/slk test switches workspaces against a primed two-team cache and asserts each team's set is published (`TestFetchWorkspaceEmoji_SendsLoadedMsgForItsTeam` keeps passing). [gotcha G4, G19]
- [ ] emoji-upsert-txn: custom_emoji write is one team-scoped transaction (DELETE by team_id + INSERT) safe against the foreign_keys(ON) DSN (db.go:37 — no FK to workspaces, or workspace row upserted first); two-team replace test; emoji-before-workspace-row order tested. [gotcha G9]
- [ ] emoji-seed-order-test: cmd/slk wiring test asserts recorded call order = seed-from-cache → fetchWorkspaceEmoji, and the seed path never calls the lister (`callCount 0` at seed time). [gotcha G13]
- [ ] fuzzy-inorder-subseq: DOD/examples use "non-contiguous, IN-order subsequence" (matching `subsequenceScore` semantics); negative test `krt` does NOT match `rocket`. [gotcha G10]
- [ ] fuzzy-rkt-verified: `rkt→rocket` and `:thumbs→+1` verified against real `emoji.BuildEntries` output before being enshrined in tests; subsequence tier is score-aware (word-boundary/tightness, not plain alphabetical) so the examples win on merit; "recent" = matching entries ∩ frecent only (stale/global frecent names skipped — frecent_emoji has no team column); cap semantics defined (eviction documented or top prefix match guaranteed to survive). [gotcha G11]
- [ ] fuzzy-reactionpicker-migration: reactionpicker + emojipicker expectation updates are behavior changes in their own commit (their tier set grows: prefix+substring → 4 tiers); "expectations unchanged" holds ONLY for channelfinder/mentionpicker; reactionpicker test coverage includes a subsequence query (`rkt→rocket` there too). [gotcha G12]
- [ ] emoji-cache-table: `internal/cache` persists custom emoji per workspace (table `custom_emoji(team_id, name, value, updated_at)`, pattern: `workspaces` in `cache/db.go`); upsert replaces the workspace's set on `emoji.list` success; read returns the full map; cache-miss → empty map, never an error surface to the UI.
- [ ] emoji-cache-wiring: startup seeds `BuildEntries` from the cache BEFORE network (`cmd/slk` connect path, replacing today's build-in-`app.go:829` with customs-from-cache), `fetchWorkspaceEmoji` upserts on success and re-publishes `CustomEmojisLoadedMsg`; existing bootstrap-subset/failure semantics preserved (`customemoji_test.go` assertions keep passing); `frecent_emoji` feeds a "recent" tier. Cache seeding stays OUT of `NewApp`/`newTestApp` (no golden perturbation). [gotcha G18]
- [ ] fuzzy-shared: ONE extracted matcher package (e.g. `internal/fuzzy` or `internal/text`) exposing folded prefix/substring/subsequence scoring — API derived from `channelfinder/model.go:388-483` (`filter` tiers + `subsequenceScore`) and `mentionpicker/match.go` word/squash ranks; `channelfinder` + `mentionpicker` + `emojipicker` + `reactionpicker` all consume it (their `_test.go` expectations unchanged, moved only if the helper moves); AGENTS.md helper table updated in the same commit.
- [ ] fuzzy-emoji: `emojipicker.Model.filter()` ranks: recent (frecent) > prefix > substring > subsequence, case/accent-folded, keeping `MaxVisible` cap and input-order stability within a tier; compose autocomplete (`:` trigger) and reaction picker get the same ranking.

### E2E / verification (c — existing repo approach only)

- [ ] e2e-harness-reuse: every new behavior proven with existing harnesses — golden frames (`compareGolden`/`newGoldenApp`), key tables (`runKeyCases` via real `dispatchModeKey`), reducer-chain driver (`updateAndRender`), service fakes (`fakeEmojiLister`, `services_helpers_test.go`); NO new test dependencies in `go.mod` (teatest/pty rejected by user ruling); `git diff --exit-code go.mod go.sum` clean on every feature branch (ruling gated, not memory-checked). [gotcha: go.mod freeze]
- [ ] e2e-suite-green: `go build ./...`, `go vet ./...`, `gofmt -l .` empty, `go test ./... -race` green (~47s shape, `internal/ui` remains the dominant package); expected-diff list pre-enumerated (help-entry churn from FullscreenToggle via `help.FromKeyMap`; reactionpicker/emojipicker expectation updates in their own commit) — no other golden re-bless (`-update` package-local only). [gotcha G18]

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
