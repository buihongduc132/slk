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

---

# Appendix B — findings from IMPLEMENTATION + LIVE verification

> Added 2026-09-28 · append-only, originals above untouched
> Source: two gated-dev lanes driven to green + live capability test against
> workspace `dy-swarm` (T0934284S1J), 399 custom emoji
> Merged head verified: `5cb4bfa` — both gates exit 0, 60 pkgs green under
> `-race`, gofmt/vet clean, go.mod unchanged vs main
> Evidence class: every item below was observed by the orchestrator's OWN
> re-run, not reported by a delegate (gated-dev-start step 9)

## B-Rank 5 — invalidates a mechanism as written

- **B1 `TestFuzzy_OutOfOrderQueryMatchesNothing` was an UNSATISFIABLE oracle,
  and never an F2P test.** The fixture `{rocket, bookmark_tabs, cricket}`
  asserted nothing matches `krt`. But `krt` IS a genuine in-order subsequence
  of `bookmark_tabs` — b-o-o-[k]-m-a-[r]-k-_-[t]-a-b-s, k@3 r@6 t@9. The row
  therefore demanded that CORRECT in-order matching return nothing, which no
  correct implementation can do. Lane-fuzzy burned two iterations on it and
  began probing a `leadingSkip > 2` heuristic that would have rejected
  legitimate matches whose first hit sits deeper than 2 runes in — i.e. it was
  about to break the feature to satisfy a broken test.
  - Fixed in `4112bf2`: `bookmark_tabs` → `roller_skate`. Verified rocket,
    cricket and roller_skate all match `rkt`; none match `krt`.
  - **Vacuity consequence (vacuity.md fifth case):** with the corrected
    fixture the row passes against the BASE implementation, so it has zero
    discriminating power. `internal/fuzzy`'s in-order matching was correct all
    along. Lane-fuzzy's only real defect was the frecent-ranking row.
  - Lesson: G11 warned the DOD's *examples* were unverified empirical claims.
    The same applies to a test's *distractor set* — a negative fixture needs
    its non-matches verified as rigorously as its matches.

- **B2 Two toast helpers with different cmd contracts caused 6 of the 8
  fullscreen failures AND a 10-minute package hang.** `toastWithClear`
  (`reducer_io.go:88`) sets the toast EAGERLY and returns a bare clear tick;
  `App.uploadToastCmd` (`app.go:4131`) returns `tea.Batch(setter, tick)` so the
  toast is not applied until the batch runs. `reducer_zoom.go` called the
  batched one while the suppression rows read `statusbarText` straight after
  `a.Update`, so every row saw an empty status bar. The prior attempt "fixed"
  this by making `uploadToastCmd` eager, which (a) broke 14 production call
  sites — one at `app.go:3586` builds it INSIDE a `tea.Batch`, where an eager
  setter fires before the runtime executes it — and (b) made `firstBatchCmd`
  block on a bare `tea.Tick`, hanging `internal/ui` until its 10-minute
  `-timeout` panic.
  - This is precisely the "same value under two spellings" class `AGENTS.md`
    names as the repo's worst recurring defect, at the level of a *contract*
    rather than a constant. Resolution per that rule is DELETE one, never
    alias — the two helpers should be one with an explicit eager/deferred
    parameter. NOT done here (out of lane scope); recorded as Open Thread 5.

- **B13 `fs-zoom-invariant` was marked `[x]` with NO implementation, and the
  test that should have caught it cannot fail.** Zoom survived the
  disappearance of the pane it was zooming: `exitZoom` had exactly three call
  sites (the `z` toggle, esc, insert-mode's esc arm), `a.zoomed` was assigned
  only in `enterZoom`/`exitZoom`, and neither `reducer_channels.go` nor
  `reducer_workspace.go` mentioned zoom at all — so a thread close, channel
  jump or workspace switch stranded the user fullscreen on a closed or
  replaced pane with no nav affordances (G5, exactly as predicted).
  - `TestFullscreen_ZoomAutoClears` intends to pin this and structurally
    cannot. It captures `frameIdle` **before** zoom, **reassigns** it after the
    clearing event, then asserts a `z` press yields a different frame. Cleared
    → `frameIdle` unzoomed, `z` enters, differs, pass. Leaked → `frameIdle`
    zoomed, `z` exits, differs, **also pass**. Only an inert `z` fails it. Its
    own error string names the leak branch as something it detects.
  - Proven on one tree, one run, at `6b4f672`: the new state-based test FAILED
    all three rows while the frame-delta test reported `ok`.
  - Fixed by adding `clearZoom` beside `exitZoom` rather than reusing it.
    `exitZoom` restores the viewport captured by `enterZoom`, which is right
    for a user-initiated exit and **wrong** for auto-clear: on a channel jump
    it stamps the old channel's offset and selected index onto the new
    channel's pane, and a shorter new channel puts that index out of range.
  - Generalizable, and the counterpart to B1: **a frame-delta oracle cannot
    pin a rule whose two outcomes both change the frame.** Assert the state
    that discriminates. This one hid behind a green suite, a green gate, and a
    21/21 live capability run — none of which covered it, because none of them
    asserted `a.zoomed` after a clearing event.

- **B14 Zoom transitions stranded both pending chords armed, behind a hidden
  hint.** `reduceZoom` is in the reducer chain (`app.go:998`) while
  `pendingWinCmd` and `pendingTop` are consumed in `handleNormalMode`
  (`mode_normal.go:42`, `:51`), which runs **after** the chain — and every arm
  of `reduceZoom` returns `true`, so it starved both. `SetMode` disarms them
  ("a global intercept must not strand it armed") but a zoom transition is not
  a mode change: `z` and esc-exit both stay in `ModeNormal`, so that guard
  never fired.
  - Worse than a stranded flag: the `ctrl+w …` / `g …` hints render in the
    status row, which zoom **hides**. The chord stayed armed with no
    affordance and the next keystroke was silently eaten as a window command.
  - Five rows RED at base, including `g` armed *while* zoomed — `g` is not in
    `zoomSuppresses`, unlike `ctrl+w`, so both orders are reachable. Fixed by
    extracting `SetMode`'s block as `App.disarmPendingChords` and calling it
    from `enterZoom`/`exitZoom`/`clearZoom` (extracted, not copied, per
    `AGENTS.md`).
  - Generalizable: **a reducer that claims a key starves every post-chain
    consumer.** Any state change that makes a chord's hint unreachable has to
    disarm the chord.

## B-Rank 4 — significant

- **B3 A shared test helper could hang its whole package instead of failing.**
  `firstBatchCmd` called `cmd()` on the test goroutine and type-asserted
  `tea.BatchMsg`. A non-batch cmd is almost always a bare `tea.Tick`, whose
  closure blocks for the tick's duration, so a contract break surfaced as a
  10-minute panic three packages away rather than as one failing row. Fixed by
  running `cmd()` off-goroutine with a 1s bound (`0f41b68`). **Generalizable:
  any test helper that invokes a `tea.Cmd` directly needs a timeout**, because
  the cmd's shape is production's choice, not the helper's.

- **B4 G11 CONFIRMED LIVE — and the DOD's example is unreachable in a real
  workspace.** With 399 custom emoji, `:rkt` yields five
  `cmd-pallet-*worktree*` rows and NO `rocket`. Mechanism: `worktree` contains
  `rkt` as a CONTIGUOUS substring (w-o-[r-k-t]-r-e-e), so those names are
  `TierSubstring(4)` while `rocket` is only `TierSubsequence(5)`. The plan's
  own mandated order (`recent > prefix > substring > subsequence`) means tier 4
  SHOULD win, and `MaxVisible=5` then evicts rocket. **The implementation is
  correct; the DOD's `rkt→rocket` example is not satisfiable against this
  corpus.** `fuzzy-rkt-verified` asked for the examples to be verified against
  real `BuildEntries` output — that verification was done against the built-in
  table only, not against a workspace's custom emoji, which is where it fails.
  - The unit test passes only because its fixture holds 4 curated entries.
  - Live-portable assertions used instead: `:rocke` (prefix tier, cannot be
    evicted) reaches rocket, and every `:rkt` row is a genuine in-order match.

- **B5 `messages.ClickAt` mis-counted the trailing spacer row — a PRE-EXISTING
  production bug the zoom fixture surfaced.** It treated the blank spacer row
  as part of the entry above it, so a click on dead space moved the
  selected-message cursor. That is what made the fs-restore-eq fixture's cursor
  land on 81 when it asked for 80 — the defect was in click hit-testing, not in
  zoom restore. Fixed in `2dbb038` by reusing the same
  `i < len(entries)-1 && msgIdx >= 0` guard `viewInternal` already applies when
  trimming the spacer off `selectedEndLine`, so the two now agree. Affects
  every click in the messages pane, zoomed or not.

- **B6 A golden file had been blessed from the WRONG frame.**
  `fullscreen_messages_zoomed.ansi` contained `"Thread from"`, contradicting
  both its own filename and its test's inline assertion. A golden that is
  wrong-but-stable is invisible: it passes forever and pins the wrong
  behaviour. Re-blessed in `2dbb038` with the reason stated; the other two
  goldens were untouched.

- **B15 The zoom save/restore round trip is vacuous whenever the THREAD is the
  zoomed pane.** `enterZoom` snapshots `a.messagepane.YOffset()` /
  `SelectedIndex()` and `exitZoom` restores onto `a.messagepane` — always,
  unconditionally. But `TestFullscreen_ZoomZoomsTheFrontPane` and the
  `fullscreen_thread_zoomed` golden both establish the thread as a legitimate
  zoom target. So when the thread is in front, the snapshot captures a pane the
  user is not zooming and cannot change while it is hidden, and the thread's own
  scroll position is never saved or restored at all.
  - `thread.Model` has 69 exported methods and **none** of `YOffset`,
    `SelectedIndex`, `SetViewport` or `SetSize`. It owns an internal viewport
    (`m.vp`, driven by `ScrollUp`/`ScrollDown`/`GoToTop`/`GoToBottom`) that
    `App` has no accessor to reach, so the symmetric restore is not merely
    missing — it is not expressible.
  - `fs-restore-eq` is therefore pinned for exactly the one pane where the code
    happens to be right: every restore assertion is on `a.messagepane`, via a
    fixture that never opens a thread.
  - NOT fixed here. It needs `YOffset`/`SetViewport` equivalents on
    `thread.Model`, which per `AGENTS.md` means changing both models in lockstep
    — Phase 3's pane-hooks territory, not a bug-fix branch. Raised separately
    rather than folded in.
  - Note the divergence is *asymmetric duplication*: the two models share 45
    identically-named methods, and this is one of the places they do not. The
    lockstep test cannot catch it, because it compares render output in one
    static non-scrolling state.

## B-Rank 3 — moderate

- **B7 agyralph's circuit breaker DISARMS a correct gate.** After 5 consecutive
  failures it auto-disabled the registered gate check (observed:
  `[check:645e1fee] CIRCUIT BREAKER: 5 consecutive failures -- auto-disabling`).
  For a flaky check that is sensible; for a gated-dev gate it is backwards — a
  valid gate is SUPPOSED to fail repeatedly until the work is done. Disarming
  it is how a loop manufactures a false green. The bridge doc should state that
  a gate check must be exempt from the breaker, or that the orchestrator must
  re-enable it every round.

- **B8 agyralph's conversation-DB harvest returns TRUNCATED step bodies, so the
  loop does nothing.** Two independent attempts (flash + `--sub 1`, then
  `--model pro --sub 0`): 14 iterations, zero commits, zero file changes. Main
  responses were 11-char tool-call tokens (`call_277455`) or fragments cut
  mid-word — `"$683929c6-6983-45de-a8` (a conversation id) and `his suggests
  the "frecent" entry is likely not being added to allEmoji, and filter`
  (missing its leading `T`). The agent was reasoning CORRECTLY; the harvest
  never delivered a complete response, so nothing executed. This is L-B1 in the
  bridge doc's own lessons list, reproduced. Both lanes were moved to
  `ralph --agent claude-code`, which worked first time.

- **B9 The post-merge gate re-run can silently measure the WRONG TREE.** In
  `merge-and-verify.sh`, `git checkout` of the merged commit aborted
  (`untracked working tree files would be overwritten: .ralph/...`) and the
  script still ran the gates and printed `exit=0 (on merged 5975b4c)` — a
  vacuous green in the one step designed to prevent vacuous greens. Fixed: the
  checkout failure is now fatal, the worktree HEAD is compared to the merged
  commit before the gate runs, and the gate's own printed `head=` is
  cross-checked against the merge. Generalizable: **a verification step must
  prove WHICH tree it measured**, not just report an exit code.

- **B10 Loop bookkeeping reached a commit.** ralph's iteration-1 auto-commit
  (`3a99b82`) captured four `.ralph/` files because the worktree's git exclude
  was added after that loop had been relaunched. Untracked in `5cb4bfa`. The
  stray-artifact sweep missed it because its pattern covered
  `scratch|patch_|fix_|.orig|.rej` but not `.ralph/` — a denylist only catches
  what it already knows.

## B-Rank ≤2 — doc-only

- The live capability test needed three fixes of its own before it measured
  anything real: a connection predicate that hardcoded `▾ Channels`/`▾ DMs`
  (invalid when `use_slack_sections` is on — this workspace renders `▾ Starred`,
  `▾ Direct messages`, etc.), a `custom_emoji` check ordered BEFORE the run that
  created the table (it reported "absent" while a later step reported 399 rows
  from the same file), and a row scraper that matched `:name:` anywhere in the
  frame, picking up shortcodes inside MESSAGE TEXT (`:wave:`, `:one:`, `:gear:`)
  as if they were picker candidates. **A smoke test's own predicates need the
  same scrutiny as the code under test** (V8/L-GS7, extended: assert you are
  reading the right thing, not merely that something was read).
- The compose emoji trigger requires `:` at column 0 or directly after
  whitespace (`compose/model.go:1140-1148`). A stale compose buffer therefore
  reads as "picker broken". Worth a line in the feature docs.
- `TestFullscreen_TabWhileZoomedDoesNotFlipTheZoomedPane` asserts only
  `strings.Contains(plain, "wrapping behaviour")`. At 200 cols both panes render
  side-by-side unzoomed — the sibling golden subtest says so explicitly as its
  reason for choosing that width — so the token is present whether Tab left the
  zoom alone, flipped it, or dropped it. Its own comment concedes it "passes
  today (`z` is a no-op…)": written as a base-passing guard, never given a
  zoom-specific observable, unlike every other row in the file. Replaced with a
  row that also asserts `a.zoomed` and the status row's absence. **Tab's
  behaviour was already correct** — the replacement passes at base, so it is
  P2P strengthening, not an F2P fix. Recorded so the next reader does not read
  it as a bug that was fixed.
- **Delegate findings need the same verification as delegate code.** Of six
  findings returned by the delegated gotcha-coverage pass on the zoom feature,
  four survived independent checking (B13, B14, B15, the Tab row above), one was
  **refuted**, and one was accurate but already annotated in place. The refuted
  one claimed two of `assertZoomedFrame`'s five sidebar tokens were vacuous
  because `● bob` / `○ carol` "do not appear in the rendered frame" — they
  appear at lines 11 and 12 of `fullscreen_zoom_exit_restored.ansi`, so
  asserting their absence from a zoomed frame is a real check. The delegate's
  own quoted evidence contradicted its conclusion. Its `file:line` references
  were otherwise accurate, including two I initially mis-refuted with a bad
  grep (`reduceChannelSelected`/`reduceWorkspaceSwitched` are package-level
  functions taking `a *App`, not methods, so a `^func (a \*App)` pattern misses
  them). **Verify both directions**: a delegate's finding can be wrong, and so
  can the check that dismisses it.
