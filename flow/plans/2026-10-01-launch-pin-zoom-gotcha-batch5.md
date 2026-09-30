All reads done; worktree existence verified (`/home/bhd/Documents/Projects/bhd/slk-wt/launchpin-zoom` @ 7daa989 on branch `launchpin-zoom` via `git worktree list`). Findings below — rank 3+ first, per required format.

---

## Rank 4

**1. DOD-5/R3: "resolvable" for a *thread* is not decidable at launch — async thread-fetch failure has no defined behavior, and R3 forbids the state it produces**
- **What:** R3 promises "never a partially-zoomed or zoom-on-wrong-pane state," but R6's own landing semantics (`openThreadForPermalink`, `internal/ui/reducer_links.go:155-178`) open the thread **optimistically** — minimal stub parent, backfilled when `ThreadRepliesLoadedMsg` lands. For `--channel X --thread <ts> --fullscreen` where X resolves and the thread *looks* resolvable, zoom is applied to the thread pane at first frame; if the replies fetch then 404s/errors, the app sits zoomed on an empty/backfilling thread pane — exactly the wrong-pane state R3 forbids. The plan's DOD-5 test only covers the two pre-launch-decidable branches (channel-ok/thread-miss; neither) and never drives a late `ThreadRepliesLoadedMsg` failure while zoomed. There is also no ruling for which recovery applies (`clearZoom` vs `exitZoom` — deliberately different, `internal/ui/reducer_zoom.go:177-192`: restoring a saved viewport onto replaced content "is a bug, not a courtesy").
- **Why missed:** R3 treats resolution as a pre-TUI predicate; R6's stub+backfill makes thread-open inherently async. The two rulings compose into an uncovered window.
- **Severity:** significant — a user-visible dead state (fullscreen empty thread) that the plan's own invariant prohibits.
- **Mitigation:** add a ruling: late thread-landing failure while zoomed-on-thread → `clearZoom` + notice naming the thread + stay on the channel; pin with a test driving the failure msg after the zoomed landing frame. Extend DOD-5's probe to include it.
- **Rank:** 4

**2. U2's probe is structurally insensitive to the failure mode it guards — `TestAGENTSMD_EveryTableRowNamesAnythingThatExists` can never catch an unregistered helper**
- **What:** The probe ("test passes with the new rows") validates table→exists only. `internal/ui/agentsmd_registration_test.go:208-214` documents the unchecked direction by name: "a *reusable helper that was never given a row* still passes." A green probe is identical whether the launch-target resolver and ENV accessor got rows or not — and both are new helpers the codebase's #1 historical defect (fourth reimplementation) predicts will be re-implemented.
- **Why missed:** The plan cites the test as if it enforced registration; it enforces only de-registration staleness.
- **Severity:** significant — the item self-verifies as done while its substance (same-commit registration) is unverifiable by the chosen probe.
- **Mitigation:** replace/complement the probe with the grep pattern the plan already uses elsewhere: `rg -c 'launchTarget|<accessorName>' AGENTS.md` ≥1 per new helper, run in the same commit; the AGENTSMD test then guards the rows' longevity.
- **Rank:** 4

## Rank 3

**3. U2 has no registration target: `cmd/slk` has no helper table (documented scope gap)**
- **What:** "Registered in the AGENTS.md shared-code tables" is unactionable for cmd/slk helpers — AGENTS.md:281-291 explicitly documents that cmd/slk's 103 unexported functions are tracked nowhere and calls the sweep "an open item, not something this pass closes." There is no table a `cmd/slk` ENV accessor or launch resolver can be added to; wedging them into the internal/ui "Test helpers" table would be wrong-package.
- **Why missed:** the plan assumes a table that the repo documents as absent.
- **Mitigation:** U2 should explicitly open a minimal `cmd/slk` table (2–3 rows: resolver, ENV accessor) and note it partially discharges the documented gap — or relocate the helpers to a package that has a table.
- **Rank:** 3

**4. U1 probe ≠ U1 unit: the make targets omit `go vet ./...` and the go.mod/go.sum check entirely**
- **What:** Makefile:9-13: `test` = `go test ./... -v -race`; `lint` = `golangci-lint run ./...` (which does cover gofmt via `.golangci.yml:33-35` formatters); `build` compiles only `./cmd/slk` with ldflags. Nothing runs `go vet ./...` (go test runs only a vet *subset*), and nothing checks `git diff go.mod go.sum`. Gates-green's own text lists six checks; the probe verifies four.
- **Why missed:** probe was written from the Makefile rather than from the unit text / AGENTS.md:13-18 gate list.
- **Mitigation:** run the six commands as literally listed (or add a `vet:` target); treat `make` as convenience only.
- **Rank:** 3

**5. `git diff go.mod go.sum` is vacuous after any commit — the gate as written cannot detect a committed dependency change**
- **What:** U2 mandates same-commit registration; the plan's workflow commits as items complete. Post-commit, working-tree `git diff` in the worktree is empty regardless of go.mod changes vs `main`. The "no new deps" guarantee (also DOD-6) is unenforced by this probe.
- **Why missed:** diff-vs-worktree vs diff-vs-base confusion.
- **Mitigation:** `git diff 7daa989 -- go.mod go.sum` (or `main...HEAD`) in the probe.
- **Rank:** 3

**6. cmd/slk has no argv/stdout/exit-code test harness, yet U4/U5 probes assume one exists**
- **What:** The `os.Args[1]` switch is inline in `main()` with direct `os.Exit` calls (`cmd/slk/main.go:74-115`); `printHelp` writes straight to `os.Stdout` (`main.go:159-177`); no stdout-capture helper exists anywhere in cmd/slk (repo-wide, only `internal/image/probe_test.go` uses `os.Pipe`; no `TestPrintHelp` exists). U4's "table test over argv vectors asserting same outputs/exit codes" and U5's "usage error, exit 2, no TUI" both require a new seam first: extract a pure dispatcher + a `flag.FlagSet` with `ContinueOnError` returning `(handled, exitCode, err)`, and/or `printHelp(w io.Writer)`. That refactor is real, unbudgeted work the plan treats as free.
- **Why missed:** the plan lists probes as if main-package behavior were already table-testable.
- **Mitigation:** budget the extraction as its own item (it's also what makes R10 testable at all); "no TUI" is then asserted structurally — parse error returned before `run()` is reached.
- **Rank:** 3

**7. `TestMainGoHoldsOnlyTheEntrypoint` will fire the moment launch-parsing declarations land in main.go**
- **What:** R12 layers the FlagSet "in `cmd/slk/main.go:74-115`". `cmd/slk/main_scope_test.go:25-33` allows exactly seven names in main.go; any new top-level decl (`parseLaunchFlags`, a `LaunchTarget` type, the ENV accessor) declared there fails the test (`main_scope_test.go:46-61`), and stale-allow-list enforcement means even adding-and-removing is noisy.
- **Why missed:** plan never mentions the main.go scope gate.
- **Mitigation:** all new decls go in a new topical file (`cmd/slk/launch*.go`) — which AGENTS.md conventions and the test's own error message prescribe; never extend `mainGoAllowed`.
- **Rank:** 3

**8. Source-structural oracles anchor the exact region the land-channel seam edits**
- **What:** `cmd/slk/startup_order_test.go:34-55` and `startup_emoji_order_test.go` pin, by byte-position in main.go source, that `seedCustomEmojiFromCache(...) < p.Send(ui.WorkspaceReadyMsg{ < go fetchWorkspaceEmojiIntoCache(...)`. The land-channel item's seam *is* that send site (`main.go:1573-1590`). Additive edits inside the composite literal are safe, but hoisting the send into a helper (a natural refactor when adding 4–6 launch fields) breaks both oracles.
- **Why missed:** the plan cites the send-site line range without noting it is string-anchored test terrain.
- **Mitigation:** keep edits inside the literal; if reshaping, update both oracle tests in the same commit.
- **Rank:** 3

**9. R10 vs ENV: does `SLK_CHANNEL` satisfy "--thread without --channel"?**
- **What:** R10 is written flag-scoped ("`--thread <bare-ts>` without `--channel`"); R1 makes ENV an equivalent source. `SLK_CHANNEL=C1 slk --thread 123.456` — exit 2 (literal R10) or a valid merged target (R1/R11)? The cli-flags-parse probe tests only the flags-only violation.
- **Why missed:** R10 predates the precedence ruling's application to error cases.
- **Mitigation:** rule that R10 evaluates the *merged* channel spec (any source); add the ENV-channel+flag-thread row to the probe. (Same ruling answers SLK_CHANNEL+SLK_THREAD both-ENV.)
- **Rank:** 3

**10. The `rg SLK_ … "returns exactly 1 non-test file"` probe contradicts its own command and the precedence test it mandates**
- **What:** The precedence unit test must set `SLK_CHANNEL/SLK_THREAD/SLK_FULLSCREEN` via `t.Setenv` (precedent: `cmd/slk/clipboard_write_test.go:95`), so the literal probe command `rg -n "SLK_(CHANNEL|THREAD|FULLSCREEN)" --type go` returns ≥2 files (accessor + test file) and "fails" its own expectation. Worst-case response — removing `t.Setenv` from tests to satisfy the grep — degrades the tests. Also note `t.Setenv` forbids `t.Parallel` in those tests.
- **Why missed:** the parenthetical intent ("non-test file") never made it into the command.
- **Mitigation:** `rg -l 'SLK_(CHANNEL|THREAD|FULLSCREEN)' -g '!*_test.go'` must return exactly 1 file.
- **Rank:** 3

**11. DOD-5's "pinned by a test" needs a two-layer pin; one ui-side test leaves the cmd/slk decision untested**
- **What:** The R3 branch (channel-ok/thread-miss vs neither) is *decided* in cmd/slk (resolver outcome) but *observed* in internal/ui (first zoomed frame via `assertZoomedFrame`, `fullscreen_red_test.go:121-138`). A ui-side simulation alone can't distinguish a correct decision from a lucky fixture; a cmd/slk-side test alone can't see the frame. Also: `assertZoomedFrame` **skips the content assertion when `contentToken` is empty** (`fullscreen_red_test.go:135-137`) — an empty token yields a structural-only, near-vacuous pass.
- **Why missed:** DOD-5 says "a test" (singular).
- **Mitigation:** pin the decision as a pure-resolver outcome table (extends the target-* items) *and* the frame via `newTestApp` + `updateAndRender` + `assertZoomedFrame` with a non-empty content token.
- **Rank:** 3

**12. Fall-through argv behavior change is undecided and unprobed: stray argv previously launched the TUI, now exits 2**
- **What:** Today `slk junk` or `slk --channel` (missing value) matches no switch case and falls through to `run()` → TUI launches, argv ignored (`main.go:74-115` fall-through). After R12, anything reaching the flag layer that isn't a known flag exits 2. That's a user-visible hard-fail for existing stray-arg invocations (shell aliases, mis-expanded vars). The compat probe's vectors cover the 7 switches + `--nonsense` + bare `slk` — no non-flag positional junk, no legacy missing-value case.
- **Why missed:** "byte-for-byte switch semantics" was scoped to the switches, not the fall-through.
- **Mitigation:** decide explicitly (exit 2 is defensible under fail-closed R10, but say so), add `junk` and `--channel`-without-value vectors, and document the behavior in `--help`.
- **Rank:** 3

## Rank 2

**13. R7 lists `runKeyCases` among the harnesses for this feature — it's the wrong harness for landing/zoom assertions**
`runKeyCases` calls `dispatchModeKey` directly, bypassing the reducer chain and the ctrl+c/bootstrap/scroll-flush gates (`internal/ui/modekeys_test.go:298-301,338`); AGENTS.md documents this. All five landing items are reducer-message-level; `z`/esc regression must go through `updateAndRender` like `mustEnterZoom` (`fullscreen_red_test.go:108-116`). A `runKeyCases`-based zoom test asserts a dispatch path production may not take. Mitigation: scope R7's `runKeyCases` to mode-table regressions only. **Rank 2**

**14. Compat probe vector list omits the bare-word aliases `version` / `help`**
The switch cases include `"version"` and `"help"` bare words (`main.go:76,81`), not just `--`/`-` forms; the item prose and probe enumerate only the dashed forms. They're handled before the flag layer so no regression today — but nothing pins that the switch *retains* them once a flag layer exists behind it. Two cheap vector rows. **Rank 2**

**15. `make lint` verdict is version-relative; CI pins v2.13.1 and older binaries panic on go1.27**
`.github/workflows/ci.yml:15-20`; a local `make lint` with a drifted binary yields a "clean" that CI may not reproduce (or a panic). Also `make test` runs without `-count=1` — cached `(cached)` PASS lines blur the "actually executed" record when capturing gate-output SHAs (AGENTS.md timings assume `-count=1`; `-v` also makes the recorded output huge). Mitigation: record `golangci-lint version` and add `-count=1` to the probe invocation. **Rank 2**

**16. R5's "results cross the boundary as messages on `internal/core` types" misstates the actual seam**
The landing seam is fields on `internal/ui.WorkspaceReadyMsg` (`internal/ui/msgs.go:367-409`; precedent `LastChannelID`, tested by `reducer_workspace_lastchannel_test.go`), which cmd/slk already constructs directly (`main.go:1573`). Taking R5 literally invites inventing a `core` type/port the codebase doesn't use for this. Corrigendum to the batch context, also: `internal/ui` reading ENV would **not** breach `boundary_test.go` — `os.Getenv` is explicitly allowed there (`boundary_test.go:54-56`; precedent `internal/ui/editor.go:21-26`); the real constraint is R8's single-read-site rule. Mitigation: extend `WorkspaceReadyMsg` (or a sibling ui msg) with launch-target fields; keep the ENV accessor in cmd/slk per R8, and don't cite the boundary test where R8 is the operative rule. **Rank 2**

---

**Files read:** `flow/plans/2026-10-01-launch-pin-zoom.md`; `AGENTS.md` (full); `cmd/slk/main.go` (lines 60–180, 1540–1610); `cmd/slk/main_scope_test.go`; `cmd/slk/startup_order_test.go`; `internal/ui/boundary_test.go`; `internal/ui/agentsmd_registration_test.go` (55–189); `internal/ui/fullscreen_red_test.go` (60–139); `internal/ui/modekeys_test.go` (300–348); `internal/ui/thread_breadcrumb_test.go`; `internal/ui/reducer_workspace.go` (160–260); `internal/ui/reducer_workspace_lastchannel_test.go`; `internal/ui/reducer_links.go` (155–179); `internal/ui/reducer_threads.go` (72–101); `internal/ui/reducer_zoom.go` (120–199); `internal/ui/msgs.go` (360–419); `internal/ui/mode_presence_snooze_test.go` (50–79); `internal/slackurl/slackurl.go`; `Makefile`; `.golangci.yml`; `.github/workflows/ci.yml`; `.gitignore`.
**Grepped:** `SLK_[A-Z]+` (all Go), `os.Getenv|LookupEnv|Environ` (internal/ui), `os.Args` (Go), `flag.FlagSet|os.Args[1]` (cmd/slk tests), `printHelp|TestMain|capture|os.Pipe|stdout` (tests), `WorkspaceReadyMsg`, `pendingLinkNav`, `mainGoAllowed`, `SLK_` usage sites, Makefile/CI targets; `git worktree list` (worktree existence verified).

Zero files modified — read-only throughout.