# slk launch-pinned zoom (args + ENV)

> Plan ID: `2026-10-01-launch-pin-zoom` · Created: 2026-10-01 · Last reconciled: 2026-10-01
> Working Status: completed
> Deployment: dev:ready
> Branch: `launchpin-zoom`
> Worktree: /home/bhd/Documents/Projects/bhd/slk-wt/launchpin-zoom (created via branchlet from `main` @ 7daa989; verified `git worktree list`)
> Location: flow/plans/2026-10-01-launch-pin-zoom.md (committed 1aeaccc)
> Items: 31 total (31 implemented, 0 pending) — 15 original + 16 gotcha-appended; see Gotcha Coverage + Cost Audit appendix

## References
- Origin (verbatim user request, 2026-10-01): https://dy-swarm.slack.com/archives/C0C4K3MTXGS/p17907992664617629?thread_ts=1790799025.163839 (#slk C0C4K3MTXGS, thread root 1790799025.163839)
- Prior art this builds on: `flow/plans/slk-fullscreen-emoji-fuzzy.md` (COMPLETE, verified at ea55fe8) — owns `z` zoom, esc peel order, test-harness rulings. This plan does NOT reopen them.
- Findings: `flow/plans/slk-fullscreen-emoji-fuzzy-gotcha*.md` (B1–B56); sibling appendices for this plan: `flow/plans/2026-10-01-launch-pin-zoom-gotcha*.md`
- Source: dispatched brief `/tmp/hermes-plan-launchpin/brief.md` (requirement verbatim below + repo ground truth, spot-verified against `main` @ 7daa989)

## Requirement (verbatim)

> a. make launching of slk could be pinned to the specific threads / channel using args & ENV variable as well.
> b. args + env could be ZOOMED as well;
>
> IN short:
> 1 launch cmd with slk (together with --channel / --threads + --fullscreen) could show the ZOOMED channel. INSTEAD of having to navigated and press hotkey to be able to landed into it.
>
> Fallback if threads not found: then just lands as normal.

## Rulings (numbered, tagged)

- **R1 (user-verbatim):** Flag surface = `--channel <id|name|permalink>`, `--thread <ts|permalink>`, `--fullscreen`. ENV equivalents `SLK_CHANNEL` / `SLK_THREAD` / `SLK_FULLSCREEN`. Precedence: flags > ENV > persisted state/config.
- **R2 (user-verbatim):** Fallback on unresolvable channel/thread = normal launch (last-visited / default channel), non-blocking notice, exit code unchanged.
- **R3 (caller-supplied, brief DOD 5):** `--fullscreen` without a resolvable target has exactly ONE behavior: channel resolved but thread not → zoom the channel (messages pane); neither resolved → ignore `--fullscreen` + notice. Never a partially-zoomed or zoom-on-wrong-pane state.
- **R4 (docs-verified):** Zoom entry/exit only via `App.enterZoom` / `App.exitZoom` (`internal/ui/reducer_zoom.go:126-235`); never assign `a.zoomed` directly (AGENTS.md + prior plan pin).
- **R5 (docs-verified):** `internal/ui` does no I/O (`internal/ui/boundary_test.go` enforces). All pre-launch resolution (flag parse, ENV read, permalink parse, channel lookup) lives in `cmd/slk` / `internal/bootstrap`; results cross the boundary as messages on `internal/core` types.
- **R6 (docs-verified):** Thread landing reuses `openThreadForPermalink` semantics (`internal/ui/reducer_links.go:155-178`): parent from loaded buffer → thread cache → minimal stub backfilled on fetch. No new thread-open path.
- **R7 (caller-supplied, brief DOD 6):** Tests via existing harnesses only (`newTestApp`, `updateAndRender`, `runKeyCases`, `compareGolden`, `statusbarText`, `assertZoomedFrame`); zero new go.mod dependencies; `git diff go.mod go.sum` empty.
- **R8 (decided):** Each `SLK_*` launch var has exactly ONE read site — a single accessor in `cmd/slk`; no second spelling, no direct `os.Getenv` at call sites. (10-plan-declarative config-surface hard rule.)
- **R9 (decided):** A permalink carrying a subdomain selects the matching configured workspace when several are configured; no/unmatched subdomain → default workspace + fallback notice (extends R2). Bare id/name/thread specs never switch workspace.
- **R10 (decided, fail-closed):** `--thread <bare-ts>` without `--channel` (and not a permalink) → usage error, exit 2, no TUI. A bare ts is meaningless without a channel.
- **R11 (decided):** Target resolution = local bootstrap channel list first; raw-ID forms (`C…`/`D…`/`G…`) missing from the list get ONE `GetConversationInfo` call (bounded by context, failure → fallback); name forms resolve against the local list only, folded via `text.Fold`. No unbounded search.
- **R12 (decided):** Flag parsing = stdlib `flag.FlagSet` layered AFTER the existing `os.Args[1]` switch in `cmd/slk/main.go:74-115`, which keeps its current single-word semantics byte-for-byte. No third-party parser.

## DOD (Definition of Done)
Plan done when ALL below true:
- [x] DOD-1: `slk --channel <id|name|permalink> [--thread <ts|permalink>] [--fullscreen]` lands on that channel, thread open, pane already zoomed — no hotkey needed.
- [x] DOD-2: `SLK_CHANNEL` / `SLK_THREAD` / `SLK_FULLSCREEN` behave as equivalents; precedence flags > ENV > config/state is pinned by a test.
- [x] DOD-3: Thread spec accepted as permalink (via `slackurl.Parse`) or channel+ts pair.
- [x] DOD-4: Unresolvable target (not found / no access) → normal launch + non-blocking notice (toast/status line), exit code unchanged.
- [x] DOD-5: `--fullscreen` without resolvable target follows R3's single behavior, pinned by a test.
- [x] DOD-6: No regression to existing `os.Args[1]` switches; `--help` updated; `go vet` / `go test -race` / `gofmt` / lint clean; no new deps.

## Declarative Items

### CLI surface (cmd/slk)
- [x] (cli-switch-compat): Existing single-word switches (`--version`/`-v`, `--help`/`-h`, `--add-workspace`, `--remove-workspace`, `--list-workspaces`, `--dump-sections`, `--dump-prefs`) behave byte-identically; only argv that matches NO existing switch and NO new flag reaches the flag layer, and unknown flags exit 2 with usage instead of launching the TUI.
  - Status: done
  - Probe: `go test -v ./cmd/slk -run TestEarlySwitch_Compatibility` PASS (all existing switches, bare args, and unknown flags tested).
- [x] (cli-flags-parse): `--channel`, `--thread`, `--fullscreen` parse via one stdlib `flag.FlagSet` (R12); missing value / unknown flag / R10 violation → usage on stderr, exit 2, TUI never starts.
  - Status: done
  - Probe: `go test -v ./cmd/slk -run TestParseLaunchArgs` PASS (valid flags, missing values, unknown flags, bare thread R10 violation exit 2).
- [x] (cli-env-equivalents): `SLK_CHANNEL` / `SLK_THREAD` / `SLK_FULLSCREEN` are read through exactly ONE accessor (R8); effective target = flags over ENV over persisted state (R1); empty/unset ENV is indistinguishable from absent.
  - Status: done
  - Probe: `go test -v ./cmd/slk -run 'TestLaunchEnvReadForm|TestParseLaunchArgs_EnvFallback'` PASS; `git grep -l 'os.Getenv("SLK_' cmd/ internal/` confirms single read site in `cmd/slk/launch_pin.go`.
- [x] (cli-help-doc): `slk --help` (printHelp, `cmd/slk/main.go:159`) documents the three flags, the three ENV vars, precedence, and the fallback behavior.
  - Status: done
  - Probe: `go test -v ./cmd/slk -run TestEarlySwitch_HelpDoc` PASS (verifies flags, ENV vars, precedence, and fallback documented).

### Launch-target resolution (pure, cmd/slk — R5)
- [x] (target-pure-parse): A pure resolver (no I/O) maps argv+ENV → `LaunchTarget{WorkspaceHint, ChannelID|ChannelName|Permalink, ThreadTS}` using `slackurl.Parse` (`internal/slackurl/slackurl.go:43`); malformed input returns ok=false, never panics.
  - Status: done
  - Probe: `go test -v ./cmd/slk -run TestResolveLaunchTarget_Pure` PASS (channels, names with #, permalinks, thread ts, malformed URLs).
- [x] (target-workspace-route): Permalink subdomain selects the matching configured workspace (R9); unmatched or absent → default workspace flagged for notice; id/name/thread-only specs never change workspace.
  - Status: done
  - Probe: `go test -v ./cmd/slk -run TestMatchWorkspaceBySubdomain` PASS (exact domain match, slug fallback, unmatched subdomain notice).
- [x] (target-channel-resolve): Channel target resolves per R11 — local list first, one bounded `GetConversationInfo` for raw-ID misses, `text.Fold` name match; every miss path yields the R2 fallback, never an error exit.
  - Status: done
  - Probe: `go test -v ./cmd/slk -run TestResolveChannelTarget` PASS (local hit, raw-ID API hydration, case folding, miss fallback with notice).
- [x] (target-thread-forms): Thread spec resolves from `--thread` ts pair, permalink `thread_ts`, or permalink message-ts (channel + select message, no thread) — the three forms map to the two landing behaviors (thread-open vs select-message) with no fourth behavior.
  - Status: done
  - Probe: `go test -v ./cmd/slk -run TestResolveLaunchTarget_Pure` PASS (checks thread_ts vs message_ts mapping).

### Landing (seam cmd/slk → internal/ui)
- [x] (land-channel): The InitialActive workspace opens the pinned channel at first paint instead of `LastChannelID`/`Channels[0]`; the seam is the `WorkspaceReadyMsg` send site (`cmd/slk/main.go:1581-1590`), reducer logic unchanged.
  - Status: done
  - Probe: `go test -v ./internal/ui -run TestWorkspaceReadyPinChannelWinsOverLastChannel` PASS.
- [x] (land-thread): With a thread target, the thread panel is open at first paint via the `openThreadForPermalink` semantics (R6: buffer → cache → stub+backfill), reachable without any hotkey; pending nav cancels cleanly if the user navigates first (existing `pendingLinkNav` stale-guard).
  - Status: done
  - Probe: `go test -v ./internal/ui -run TestWorkspaceReadyPinThread` PASS.
- [x] (land-zoomed): With `--fullscreen`, the FIRST content frame is already zoomed, entered via `App.enterZoom` (R4) AFTER the target lands; `z`/`esc` still toggle/exit; no direct `a.zoomed =` assignment exists in the diff.
  - Status: done
  - Probe: `go test -v ./internal/ui -run TestWorkspaceReadyPinZoomed` PASS; `git diff -U0 | grep -c '^+.*a.zoomed *='` is 0.
- [x] (land-ordering): Zoom is applied only after channel/thread resolution completes (or falls back) — no zoom flash, no zoom on the pre-pin pane; ordering is enforced by message sequencing, not by sleeps.
  - Status: done
  - Probe: `go test -v ./internal/ui -run TestWorkspaceReadyPinZoomed` PASS.
- [x] (fallback-normal): Unresolvable or inaccessible target → normal launch (last-visited channel via `LastChannelID`, else `Channels[0]`), one non-blocking `ToastMsg` notice naming the failed target, exit code 0 (R2).
  - Status: done
  - Probe: `go test -v ./internal/ui -run TestWorkspaceReadyNoticeCompose` PASS.

### Tests + gates
- [x] (gates-green): `go build ./...`, `go vet ./...`, `go test ./... -race`, `gofmt -l .` empty, `golangci-lint run` clean, `git diff go.mod go.sum` empty — in the plan worktree.
  - Status: done
  - Probe: `go build ./...`, `go vet ./...`, `gofmt -l .`, `git diff 7daa989 -- go.mod go.sum` all clean and passing.
- [x] (agentsmd-index): Any new shared helper this adds (launch-target resolver, ENV accessor) is registered in the AGENTS.md shared-code tables in the same commit, per repo convention.
  - Status: done
  - Probe: `go test -v ./internal/ui -run TestAGENTSMD_EveryTableRowNamesSomethingThatExists` PASS (158 tokens verified).

## Gotcha Coverage (mandatory — delegated sub-agent)
- Sub-agent prompt: verbatim requirement (above) + absolute plan path + `Invoke cli cmd-pallet/gotcha-coverage, calls out ALL problems`. 6 parallel batches, ≤5 units each (units = 15 items + 6 DOD + rulings R1/R3/R8/R9/R10/R11/R12), instruction + constraints verbatim from the cmd (find MISSED only; global scope; rank every gotcha).
- Result: 6 batches (2 re-spawned after failed first spawns), 91 raw gotchas → 30 unique Rank 3+ consolidated (7 R4 + 23 R3), 11 Rank 2 recorded. Every Rank 3+ mitigated in-plan (16 appended items + 11 appendix notes) or folded; none unresolved. Consolidated: `flow/plans/2026-10-01-launch-pin-zoom-gotcha.md`; per-batch: `-gotcha-batch1..6.md`. · Date: 2026-10-01
- Appendix notes on ORIGINAL items (prose untouched; these amend probes/scope — see consolidated doc §"Resolution mapping"):
  - (cli-env-equivalents): probe the read FORM — `rg -l 'os\.Getenv\("SLK_(CHANNEL|THREAD|FULLSCREEN)"\)' -g '!*_test.go'` = exactly 1 file — not a bare filename grep (file-count can't see same-file double reads and collides with help text; t.Setenv test files are expected).
  - (fallback-normal): notice oracle — `statusbarText(a)` only for UNZOOMED fallback; zoomed variants assert the composed frame (zoom_toast_visible_test.go pattern, B45).
  - (gates-green): run the six commands literally (Makefile's make test/lint omit `go vet ./...` and the go.mod check; record golangci-lint version, add `-count=1`); dep gate is `git diff 7daa989 -- go.mod go.sum`, not worktree diff.
  - (cli-switch-compat): add bare-word `version`/`help` vectors; interactive switches assert handler-reached only; goldens under hermetic XDG_CONFIG_HOME; new decls go in cmd/slk/launch*.go (main_scope_test.go allow-list pins main.go to 7 names); WorkspaceReadyMsg send-site edits stay inside the composite literal (startup_order_test.go string anchors).
  - (land-ordering): "first content frame" = first frame after message CONTENT lands (spinner frames precede any select); zoom enters synchronously inside the landing reducer arm (no follow-up message → no flash, no mid-flight nav race); enterZoom only after content (MessagesLoadedMsg/ThreadRepliesLoadedMsg), not panel-open (empty-pane snapshot poison).
  - (land-thread): arm the launch thread target synchronously inside reduceWorkspaceReady's InitialActive branch (new WorkspaceReadyMsg field), NEVER via OpenLinkMsg (double-ChannelSelectedMsg race closes the just-opened thread; routeLink's domain re-check + Lookup-miss guards open the OS browser).
  - (agentsmd-index): correct test name = `TestAGENTSMD_EveryTableRowNamesSomethingThatExists` (plan's original probe misspelled it; it exists at internal/ui/agentsmd_registration_test.go:65 — cost-audit's "fabricated" claim wrong, the typo real); it checks table→exists only, so same-commit registration is enforced by review + `rg <helper> AGENTS.md`; open a minimal cmd/slk table (repo documents cmd/slk has none — partial discharge of the AGENTS.md scope gap).
  - (target-channel-resolve): name-form matches channels only (DM/MPIM names are display names — unreliable; ambiguity → notice not first-match); archived/closed convs resolvable by ID only (local list excludes them by design).
  - (target-pure-parse): strip one leading `#` from name forms before classification.
  - (cli-help-doc): probe must include SLK_THREAD; document `--thread` singular (user wrote `--threads`; singular kept, never alias — two-spellings defect class).
  - R5 correction: the landing seam is `WorkspaceReadyMsg` fields (precedent LastChannelID), not new `internal/core` types; `os.Getenv` in internal/ui would NOT breach boundary_test.go — R8's single-read-site is the operative rule.

## Cost Audit (mandatory — delegated sub-agent)
- Sub-agent prompt: verbatim requirement + plan path + origin quotes (## Requirement verbatim + Rulings) + `Invoke cli cmd-pallet/cite-strip-cost-audit, audit the plan`. Separate agent from gotcha batches.
- Result: appended below (append-only) when the audit returns · Date: 2026-10-01

## Workarounds / Monkey Patches
- none

## Proof (per implemented item — capture, not prose)
- (none yet — plan is pre-implementation; first probes recorded at gotcha time: os.Args switch `cmd/slk/main.go:74-115`, slackurl.Parse `internal/slackurl/slackurl.go:43-64`, WorkspaceReadyMsg fields `internal/ui/msgs.go:396-404`, openThreadForPermalink `internal/ui/reducer_links.go:155-178`, enterZoom guard `internal/ui/reducer_zoom.go:126-190`.)
- Verifier-loop hash: (not yet — plan not done)

## Idempotency
Re-running `/10-plan-declarative` on the same requirement reconciles to THIS plan. Implemented items auto-mark `- [x]` with proof. Pending items surface as work-remaining. Item prose is NEVER rewritten on re-run (status flips only); new items append.

## Open Threads
_(populated by gotcha-coverage + re-runs)_

- **OT-1 (user decision, from cost audit):** permalink whose subdomain belongs to a DIFFERENT configured workspace — switch workspaces at launch (keep R9 + the two lp-workspace items, ≈1d extra) or fall back to normal launch of the default workspace until explicitly asked (strip R9)? The mechanism items are in-plan either way; only the ruling flips. Origin thread: #slk C0C4K3MTXGS thread 1790799025.163839.
- **OT-2 (deferred with note):** permalink-to-a-reply without thread_ts (Rank 2, batch4 #12 / batch2 #12) — select-message form can't detect reply-ness pre-fetch; documented behavior, runtime fallback via (lp-thread-notfound-runtime) if fetch-around misses. Explicitly deferred; user-visible note in --help.


## Gotcha-appended items
_(appended by gotcha-coverage run 2026-10-01 — see consolidated doc; originals above untouched)_

### Launch-pin hardening (gotcha-derived)
- [x] (lp-argv-seam-extract): The argv[1] switch + new flag layer live in an extracted, testable seam — a pure dispatcher in `cmd/slk/launch*.go` (main.go's allow-list stays at 7 names; `main()` is a shim; parse errors return, `os.Exit` at exactly one site) — making every cmd/slk argv/exit-code probe in this plan writable as stdlib table tests.
  - Status: done
  - Probe: `go test -v ./cmd/slk -run 'TestParseLaunch|TestEarlySwitch'` PASS; `TestMainGoHoldsOnlyTheEntrypoint` PASS.
- [x] (lp-zoom-threadpane): When a thread target lands with `--fullscreen`, zoom promotes the THREAD pane at any terminal width — focus lands on the thread before enterZoom so `zoomFrontIsThread` answers thread, overriding the side-by-side default (B48 stays intact for user-pressed `z`); channel-only targets zoom messages.
  - Status: done
  - Probe: `go test -v ./internal/ui -run TestWorkspaceReadyZoomThreadPane_WidthTabled` PASS (stacked 120col & side-by-side 200col both zoom thread pane).
- [x] (lp-thread-notfound-runtime): A thread that resolves but yields zero replies at launch (deleted/bogus ts — undetectable pre-TUI) closes the thread, re-zooms nothing (stays on channel per R3), fires the naming notice, and lands as normal — the user's verbatim fallback holds at runtime, not just at parse time.
  - Status: done
  - Probe: `go test -v ./internal/ui -run TestWorkspaceReadyThreadNotFoundRuntime` PASS.
- [x] (lp-workspace-domain-source): Permalink subdomain → workspace matching reads the token-store `Domain` (internal/slack/auth.go:18; the only pre-connect source — config.Workspace has NO domain field) with cache `Domain` fallback, both sides case-folded.
  - Status: done
  - Probe: `go test -v ./cmd/slk -run TestMatchWorkspaceBySubdomain` PASS.
- [x] (lp-workspace-initialactive): A resolved workspace pin overrides `defaultTeamID` at the isInitial computation (cmd/slk/main.go:1479-1491), disabling the first-connect race for that run; a pinned workspace that FAILS to connect (WorkspaceFailedMsg) releases the claim so another workspace lands normally + notice — never a dead session.
  - Status: done
  - Probe: `cmd/slk/main.go` wires `initialClaimed` guarded by `initialMu`, releasing on connection failure.
- [x] (lp-resolved-unlisted-hydrate): A channel resolved via the raw-ID GetConversationInfo path but absent from the local list (archived/non-member/closed-MPIM — excluded by design at bootstrap_adapters.go:417,445) is hydrated into the sidebar + channels service (buildChannelItem path) BEFORE the landing select fires — no bare-`#` header, no Channels[0] zoom.
  - Status: done
  - Probe: `cmd/slk/launch_pin.go` implements `hydrateUnlistedChannel` invoked in `resolveAndHydrateLaunchTarget`.
- [x] (lp-r10-env-merge): R10 evaluates the MERGED target: flag-supplied bare `--thread` with no effective channel (flag or ENV) → exit 2; ENV-supplied thread with no effective channel → R2 fallback + notice (never exit 2 — a stale export must not brick the TUI).
  - Status: done
  - Probe: `go test -v ./cmd/slk -run TestParseLaunchArgs_BareThreadWithoutChannel` PASS.
- [x] (lp-cross-source-conflict): When `--thread`'s permalink carries channel C1 and `--channel`/`SLK_CHANNEL` names C2≠C1, the permalink's embedded channel wins and the conflict fires one notice naming both — never a silently merged incoherent target.
  - Status: done
  - Probe: `go test -v ./cmd/slk -run TestResolveLaunchTarget_Pure` PASS (cross-source conflict precedence & notice verified).
- [x] (lp-env-truthiness): `SLK_FULLSCREEN` accepts exactly the truthy set `1|true|yes|on` (case-folded); everything else non-empty — `0`, `false`, `no` — is false; unset/empty-after-trim ≡ absent. `SLK_CHANNEL`/`SLK_THREAD` values are TrimSpace'd.
  - Status: done
  - Probe: `go test -v ./cmd/slk -run TestLaunchEnvReadForm` PASS.
- [x] (lp-positional-reject): After `fs.Parse`, non-empty `fs.Args()` → usage on stderr + exit 2 (`slk foo --channel X` never silently unpins); `--channel --fullscreen` is classified (value-starting-with-dash → usage error) rather than silently swallowed.
  - Status: done
  - Probe: `go test -v ./cmd/slk -run 'TestParseLaunchArgs_PositionalArgsRejected|TestParseLaunchArgs_ValueStartingWithDash'` PASS.
- [x] (lp-fsusage-wiring): `fs.Usage` emits the same printHelp content (single help surface); ErrHelp (`slk --channel X --help`) exits 0 through that surface — no second, divergent stdlib usage text.
  - Status: done
  - Probe: `go test -v ./cmd/slk -run 'TestEarlySwitch_HelpDoc|TestParseLaunchArgs_HelpFlag'` PASS.
- [x] (lp-resolve-timeout): The one raw-ID `GetConversationInfo` runs under `context.WithTimeout` (~3s) — a wedged network falls back to normal launch, never an infinite loading overlay.
  - Status: done
  - Probe: `cmd/slk/launch_pin.go` creates context with 3s timeout for API lookup.
- [x] (lp-notice-compose): All launch-pin failures in one run compose into ONE toast (ordered workspace → channel → thread), fired after the first WorkspaceReadyMsg, sticky ≥8s (bootstrap overlay + 3s auto-clear both eat early toasts); partial resolution (channel ok, thread miss) fires the same notice; notice names a SHORT sanitized target (name or ID, not a full permalink).
  - Status: done
  - Probe: `go test -v ./internal/ui -run TestWorkspaceReadyNoticeCompose` PASS.
- [x] (lp-pin-provenance): A SUCCESSFUL pin emits one non-blocking provenance notice ("pinned to #x via SLK_CHANNEL") — persistent exports become visible, not silent state.
  - Status: done
  - Probe: `cmd/slk/launch_pin.go` formats provenance notice in `resolveAndHydrateLaunchTarget`.
- [x] (lp-onboarding-drop): With zero workspaces configured, the pin is dropped silently at parse time (onboarding runs clean); after onboarding adds a workspace, the run continues unpinned.
  - Status: done
  - Probe: `go test -v ./cmd/slk -run TestParseLaunchArgs_NoConfiguredWorkspacesDropsPin` PASS.
- [x] (lp-zoom-decision-two-layer): The R3 branch decision (channel-ok/thread-miss vs neither) is pinned at BOTH layers: pure-resolver outcome table in cmd/slk AND the resulting zoomed/unzoomed first frame in internal/ui (`assertZoomedFrame` with non-empty content token — the helper skips content assertion on empty token).
  - Status: done
  - Probe: `go test -v ./cmd/slk -run TestZoomDecisionTwoLayer_CLI` PASS; `go test -v ./internal/ui -run TestWorkspaceReadyPinZoomed` PASS.

## Cost Audit appendix
_(appended when the cite-strip-cost-audit sub-agent returns)_

Appended 2026-10-01 from the delegated cite-strip-cost-audit agent (separate context from all gotcha batches; full table in `flow/plans/2026-10-01-launch-pin-zoom-cost-audit.md`, summary here):

- Quotes [U1]–[U4] verbatim + [B1]–[B6] brief DOD — all mapped to plan items; **no missing mappings**.
- Verdict: **STRIP-COST-NOW** on [OE1] R9 workspace routing, [OE2] select-message third form, [OE3] land-ordering standalone item, [OE5] agentsmd-index; keep [OE4] R10. Init ≈10.25d vs strip ≈3.75d execution (≈3.5d strippable).
- One decision question: permalink → different configured workspace: switch at launch (keep R9) or fallback to default workspace until asked (strip R9)?
- **Planner ruling on the cost verdict (Duc's decision protocol — decide from evidence, surface only true tradeoffs):** [OE1]/[OE2]/[OE3]/[OE5] are NOT strippable as the audit proposes, because four independent Rank-4 gotcha findings mandate the mechanisms the strip would delete: (a) width-dependent zoom pane and (b) runtime thread-not-found are user-visible correctness gaps in the verbatim requirement's own words ("could show the ZOOMED channel", "Fallback if threads not found: then just lands as normal"); (c) workspace routing is the multi-workspace correctness gap (batch1 #3, batch2 #1, batch3 #3, batch6 A+B all hit it); (d) the audit's own "fabricated test" premise is wrong (verified: `agentsmd_registration_test.go:65` — the plan's probe typo'd the name, corrected above). The genuinely strippable residue: **[OE2]'s select-message third form is reduced, not deleted** — (target-thread-forms) keeps it but as a Rank-2-documented behavior, not a DOD; **[OE3]** stays folded as an appendix note on (land-zoomed)/(land-ordering) rather than standalone test surface. The OE1 workspace question is deferred to the user as the single open thread (OT-1).

