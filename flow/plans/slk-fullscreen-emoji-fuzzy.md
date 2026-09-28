# slk fullscreen pane + emoji fuzzy cache

> Plan ID: `slk-fullscreen-emoji-fuzzy` · Created 2026-09-26 · Condensed 2026-09-28
> Status: **complete and verified at `ea55fe8`.** Nothing left to implement; seven
> decisions open (bottom). Branch `main`, 142 ahead of `origin/main` (`2b43b29`), no PR.

## Requirement (verbatim)

> Fullscreen thread + hotkey for slk fork
>
> :cmd-pallet-10-ospx-explore: --- a. how to we be able to implement hotkey that make current focus thread to be fullscreen ; (remember to have hotkey to esc the fullscreen thread as well)  b. how to implement the emoji caching , then also it could be search and auto complete as fuzzy as well ; c. how do we be able to e2e test these feature after implemented ?
>
> must reuse existed approach in the current repository for e2e ; then for the rest , do the :cmd-pallet-10-plan-declarative:  then also :cmd-pallet-gotcha-coverage:  for me ;

This replaces a 2,659-line working log (the appendices below are kept in full).
Conclusions only; the log is `git show ea55fe8:flow/plans/slk-fullscreen-emoji-fuzzy.md`.

## What shipped

The log records 30 of 30 DOD items ticked and the gates below pass. Exactly what
that is worth: two were hand-verified because I doubted them (frecent wiring; the
`rkt`→`rocket` example — Open decisions 2 and 5); the other 28 rest on the log's
ticks plus the suite, not an item-by-item re-audit.

**(a) Fullscreen pane zoom.** `z` toggles the front content pane fullscreen —
rail, sidebar and status row hidden; `esc` or `z` exits, restoring the prior pane
state. Zoom auto-clears when the zoomed pane closes. Layout and navigation keys
are suppressed while zoomed, with a toast: `ctrl+b`, `ctrl+]`, `ctrl+w`,
`ctrl+a`, `ctrl+t`, `ctrl+p`, `:`, `/`, `ctrl+f`, and bare `1`-`9`.
`App.enterZoom` / `App.exitZoom` own the transition — **never assign `a.zoomed`
directly**; they also invalidate the zoom-keyed caches and save/restore the
viewport.

**(b) Emoji cache + fuzzy autocomplete.** Custom emoji survive restart: startup
seeds `emoji.BuildEntries` from SQLite *before* any network call, and the
`emoji.list` refresh runs in a goroutine *after* first paint, so cold start has
zero network wait. Ranking is four tiers — recent (frecent) > prefix > substring >
subsequence — case- and accent-folded, `MaxVisible` capped, input-order stable
within a tier. The same ranking serves the compose `:` dropdown, the emoji picker
and the reaction picker. One matcher does it: `fuzzy.Match(name, query)` →
`(Tier, score, ok)`, folding both sides itself. Subsequence scoring is
word-boundary and tightness aware, so examples win on merit rather than by
special-casing.

**(c) e2e.** Existing harnesses as ruled, no new dependencies: `compareGolden` +
`testdata/golden/*.ansi`, `runKeyCases` over the real `dispatchModeKey`,
`updateAndRender` through the real reducer chain, `stackedApp`/`assertFront`, and
the fakes in `services_helpers_test.go`. `git diff go.mod go.sum` stays clean.

## Verification

At `9c05241`: seven lane gates GREEN from clean checkouts, each `head=`-matched with
`dirty=0`; `go build`, `go vet`, `gofmt` (0 of 676), `golangci-lint` **0 issues**,
`go test ./... -race` (60 packages, 0 FAIL). Everything after it is markdown, so that
carries. All 14 pins verify (see decision 7 for what they are).

`slk-dev` deployed from `9c05241`, real credentials, own XDG roots. Both isolation
controls hold (dev roots resolve a workspace, an empty root resolves nothing), live
`config.toml` byte-identical, capability suite **24 PASS / 0 FAIL** — that count is
state-dependent, 24 or 28, never 25. Three live probes green.

Run gates only via `~/.local/state/slkfz/verify-commit.sh <commit>`: each `gate.sh`
hardcodes its own lane worktree, so a bare run measures whatever that tree holds. It
does **not** advance `~/.worktrees/slkfz-int`, which `deploy-slk-dev.sh` defaults to —
check the source worktree's SHA first, or you ship a tree no gate measured. For what a
deployed binary *is*, use `go version -m ~/.local/bin/slk-dev`, not `DEPLOYED-FROM`.

## What the bug hunt found

56 catalogued items. B1–B55 sit in the five appendices indexed below; B56 was found
later and lives only in the working log (merged `976e55b`).

**There is no quotable coverage breakdown.** The audit's buckets sum to 70 against
56 items — seven doc/behaviour splits, seven double-listed — and log item 60 says in
terms that they "cannot be quoted as a coverage figure." An earlier draft of this
file quoted them anyway, from the summary instead of the source.

What *is* verified is the eleven it singled out. Of eight reported uncovered: three
real (B5 `e8c61da`, B29 `feb2d00`, B51 `eaab873`), one **refuted** (B31), two already
closed before the audit ran (B40; B49, `G` arm at `ab38b2a`), two not test gaps (B50's
premise was never established; B15 needs Phase 3's pane hooks). Three more closed
after: B35 `5665841`, B2 `0d3d80f`, B34-registration `6d7740c`.

**One live production defect in all of it.** `emoji.BuildEntries` sorted on raw bytes,
so every uppercase ASCII letter sorted ahead of every lowercase one and a custom
`:Rocket:` landed before `:apple:` instead of beside `:rocket:`. Now sorts on
`text.Fold(Name)`, raw `Name` as tie-break. `internal/emoji` must import
`internal/text` **aliased** — that package's tests declare a helper named `text`.

## Appendices — retained in full, 1,717 lines

Kept on instruction, not condensed. Each ranks findings 1–5 and ends in
cross-references; per-item mechanism and `file:line` live there and nowhere else, so
**read the appendix, not this summary, before reopening an item.**

| File | Scope | Items |
|---|---|---|
| `…-gotcha.md` (399) | planner findings vs `main@2b43b29`, + Appendix B from live verification | B1–B16 |
| `…-batch1.md` (203) | emoji cache persistence | B17–B23 |
| `…-batch2.md` (228) | fuzzy matcher + its 4 consumers | B24–B31 |
| `…-batch3.md` (412) | DOD criteria; carries a note on why it took four attempts | B32–B45 |
| `…-batch4.md` (475) | zoom layout + suppression; consumers-of-the-zoom-flag survey | B46–B55 |

Batches 3 and 4 carry a **"Not reported (checked, held)"** section — the negative results. No `docs/superpowers/` spec exists for this effort (OT3); convention wants one.

In the log (`git show ea55fe8:…`, 2,659 lines) the `## Open Threads` heading at line 91
*is* the 65 items, appended out of order (19–26 follow 37). OT6/OT12/OT21/OT24/OT37
index into it and still resolve.

## Lessons worth keeping

**Tests.** A passing test proves nothing until you have seen it fail for the
reason it exists; every guard here has a recorded mutation control. But **"the
mutation survived" is not sufficient grounds to write a test** — it shows no test
fails, not that a test *could* fail for the right reason, and those come apart
exactly when the code is dead (B31's tie-break is unreachable, so any test would
have passed for the wrong reason; the refutation was the deliverable). Assert
relationships, not observed numbers, reading the expected quantity back from the
leg a bug would leave untouched, and assert movement as a *precondition* before
asserting what undoes it. Vacuity shapes seen here: a test that recomputes the
value it should observe; one that **authors its own subject**, composing the
sequence itself so a production reordering stays green; a fixture so constrained
only total absence can fail it; a probe placed where process-global state has
already been restored; a verdict from an unset variable (zsh arrays are 1-indexed, so
`${PIPESTATUS[0]}` is empty and `[ "" -ne 0 ]` takes the else branch); and a passing
probe that never compiled in — settle that with a panicking `init()`. When a comment
asks humans to maintain an invariant, replace it with a check.

**Measurement.** Measure, do not reason, when the claim is about pixels: a
published severity — "a full row of content is mouse-dead" — died on inspecting
the rendered frame, where the row turned out to be the pane's border. A regex is
not a parser; a shell check reported a stale doc row declared 60 lines away,
because its pattern missed a standalone `const X = 130` and read only the first
backticked token per row. `gofmt -l .` is a plain file walker, unlike module-aware
`go build ./...` — with agent worktrees in-tree it scanned 7,845 foreign files
against the module's 676, so prune any directory carrying its own `go.mod`.

**Citations rot, and a summary launders them.** `app.go:931-945` for the G15 contract
was wrong — bare comment lines — and survived a handoff doc, an appendix, commits, a
cron prompt and my own repetition before anyone opened the file. Same shape: a
coverage breakdown the source forbade quoting; "Open Threads 9-26", a range that
described nothing; a `DEPLOYED-FROM` sidecar naming the wrong commit *and* sha256
because the binary was rebuilt after it. Each was true-sounding, inherited, cheap to
check. Prefer a record the artifact cannot drift from — `go version -m` over a
sidecar — and re-read the source you cite, especially when the summary is your own.

**Delegation.** A delegated report must name the commit it measured, and for an
absence claim must name the grep that came back empty — two items were reported
UNCOVERED while already closed in files the report cited by name.
`Agent(isolation: "worktree")` branches from `origin/<default-branch>`, not local HEAD,
which with unpushed work is silent staleness (117 commits of it); put the checkout in
the prompt as step 0, because steering a running agent does not work. Before concluding
an agent died, read its transcript — four completion notifications were lost in one
session and every one of those agents had finished:
`jq -r 'select(.type=="assistant") | .message.content[]? | select(.type=="text") | .text' <tasks-dir>/<id>.output`.
Take facts from delegates; re-derive severity yourself.

**The harness.** Six of my own tools produced a confident wrong verdict. A control
restores the **bytes it saved**, never `git checkout` to a ref — a pre-commit
control runs while the fix is uncommitted, so HEAD is the wrong baseline, and that
silently deleted a verified fix. A restore called both inline and from an `EXIT`
trap must be idempotent, or it invents a failure at the end of a success. Exit codes
must not collide: two scripts exited `1` for a usage error while documenting `1` as
a verdict (`SURVIVED`, `zoom leaked`); usage is now `64`. Refuse a verdict on a tree
that does not compile — that caught four would-be false results.

**This repo.** Search before you write: the `rkt`-eviction test I was about to add
already existed, and one grep for `TierSubstring` found it. `/usr/bin/go` only — the
mise Go's vendored `x/text` is corrupted and its failures are the toolchain lying.

## Open decisions — yours, not mine

None of these is blocked on work.

1. **The push.** 142 unpushed commits, no PR.
2. **B24 / OT12 — frecent ranking weight.** Frecent *is* wired and live
   (`app.go:3308` → `compose.SetFrecentEmoji` → `emojiPicker.SetFrecentEmoji`,
   fed real `LoadFrecent` data, pinned by three `TestComposeFrecent_*` tests).
   The open question is whether the recent tier should outrank prefix as strongly
   as it does; today's order is pinned by tests.
3. **B48 / OT21 — `z` with the thread focused at side-by-side widths.** It zooms
   messages, and that is settled, not open: `zoomFrontIsThread`
   (`internal/ui/app.go:1008`, contract at `:980-1007`) is deliberate, B48 closed in
   favour of the code, `zoom_front_pane_test.go` pins it (G15). Open only as a
   product preference — making `z` follow focus breaks a pinned contract.
4. **B37 / OT24 — `emoji.list` and the cache.** The original mechanism
   (truncated pagination) is refuted: that API cannot page. The real trigger, an
   empty `ok:true` wiping the cache, is fixed. Open only as "do more?".
5. **OT6 — the `rkt`→`rocket` example.** Unreachable where `*worktree*` custom emoji
   exist: `rkt` is contiguous inside `wo-rkt-ree`, so those are TierSubstring(4),
   `rocket` is TierSubsequence(5), and `MaxVisible=5` evicts it. DOD line already
   withdrawn, eviction pinned (`emojipicker/rkt_corpus_eviction_test.go`). Drop the
   example, or add a length/density penalty.
6. **agyralph B12/B7.** A circuit breaker disarms a correct gate. Patch is a
   scratch copy at `~/.local/state/slkfz/agyralph-patched/`; applying it to your
   `open-ralph-wiggum` repo is your call.
7. **Gate retirement.** Seven lanes (`digits fold fuzzy sep toast xdg zoom`) and 14
   pins armed, all verifying: 10 test-file oracles + 4 `gate.sh` self-pins (`fuzzy
   sep toast zoom` only). Inventory `~/.local/state/slkfz/gates/*/`.
