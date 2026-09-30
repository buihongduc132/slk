# Gotcha-coverage — BATCH 6 (rulings cross-cut) — RERUN

> Repo: /home/bhd/Documents/Projects/bhd/slk (read-only) · Plan: flow/plans/2026-10-01-launch-pin-zoom.md
> Note: first spawn emitted only tool-reminder echo (no deliverable); re-spawned. This is the re-run's deliverable.
> Env/flag double-read class pre-check: repo-wide search for SLK_CHANNEL / SLK_THREAD / SLK_FULLSCREEN across ALL file types = 4 hits, all in the plan document itself. No collision exists today. SLK_DEBUG is the only SLK_* env precedent and doesn't collide.

## Rank 4

**A. R9's subdomain→workspace match has no data source it can actually match against**
- What: R9 says "a permalink carrying a subdomain selects the matching configured workspace." But `config.Workspace` (`internal/config/config.go:171-190`) has NO Domain/subdomain field — fields are TeamID, Theme, Order, SidebarWidth, UseSlackSections, Sections, VersionTS. The TOML key is `Slugify(TeamName)` (`cmd/slk/onboarding.go:108`), which almost never equals the subdomain ("Acme Inc" → `acme-inc` vs subdomain `acmecorp`). Subdomains exist only in: the token store (`slackclient.Token.Domain`, `internal/slack/auth.go:18`, on disk under `xdgData()/tokens`), the cache `workspaces` table (`internal/cache/workspaces.go` Domain), and `Client.TeamSubdomain()` post-connect. The plan's `target-workspace-route` item and probe never say which store the resolver matches against.
- Why missed: the ruling was written as if "configured workspace" implies a subdomain key; nobody opened the config struct. An implementer who matches against config slugs gets silent always-unmatched → every permalink degrades to default + notice, R9 dead on arrival.
- Severity: significant correctness.
- Mitigation: pin the match source in the plan: token-store `Domain` (readable pre-connect; resolver lives in cmd/slk so no I/O-boundary issue), with cache `Domain` as fallback; probe must use two workspaces whose slugs differ from their subdomains.
- Rank: 4

**B. Pin-workspace ≠ InitialActive-workspace has no routing mechanism; `land-channel` assumes they coincide**
- What: active-workspace selection today is `defaultTeamID` from `general.default_workspace`, else first-to-connect via `sync.Once` (`cmd/slk/main.go:1386-1491`); `InitialActive` is unique per run by design (`internal/ui/msgs.go:396-402`). With `default_workspace = A` configured and a permalink for workspace B, InitialActive is A — but the pin targets B's channel. Nothing in the plan overrides the default/first-connect selection or performs a post-ready switch; the sync.Once + atomic-router uniqueness makes "just send a second InitialActive" impossible by construction.
- Why missed: R9 specifies the hint; the items specify the landing; the wiring between them is in neither.
- Severity: significant correctness — multi-workspace + non-default-workspace permalink lands on the wrong workspace or needs an unplanned mechanism.
- Mitigation: add an item: the pin's TeamID hint feeds the `defaultTeamID` resolution before the connect loop (pin overrides `default_workspace`), so B claims InitialActive and first-connect `sync.Once` only fires when no pin and no default exist.
- Rank: 4

## Rank 3

**C. The one `GetConversationInfo` can block startup pre-TUI; "bounded by context" is no bound**
- What: R11's raw-ID miss path makes one network call. Resolution must complete before the pinned first paint (`land-ordering`), i.e. inside the workspace startup goroutine before `WorkspaceReadyMsg` sends. The app context has no deadline; on a hung/slow call the terminal shows no TUI at all — worse than the normal launch it's meant to fall back to.
- Mitigation: wrap in `context.WithTimeout` (~2–3s, stated in the plan) and fall back on expiry; probe asserts fallback fires under a fake slow client. Rank: 3

**D. `--thread` + `--fullscreen` with thread resolved: which pane is zoomed is unpinned**
- What: R3 enumerates only failure modes. In the success case `enterZoom` promotes whichever pane `zoomFrontIsThread()` says — depends on focus at entry. Whether launch-pin zoom lands on the zoomed thread or the zoomed channel is nowhere decided; DOD-1's "thread open, pane already zoomed" is ambiguous.
- Mitigation: thread resolved + `--fullscreen` → focus thread, zoom promotes thread (consistent with "landed into it"); pin with `assertZoomedFrame` for both channel-zoom and thread-zoom cases. Rank: 3

**E. stdlib `fs.Usage` / `ErrHelp` path undesigned: two divergent help texts and a -h exit-code split**
- What: the plan updates printHelp only. The FlagSet has its own surface: unknown flag → stdlib's PrintDefaults format; `-h`/`--help` after other flags reaches the FlagSet, which under ExitOnError prints stdlib usage and exits 0. `slk --help` ≠ `slk --channel X --help` in text and provenance, and the exit-2 rule coexists with an exit-0 help path the plan never mentions.
- Mitigation: set `fs.Usage` to a wrapper over the same content as printHelp; decide ErrHelp → exit 0 explicitly; probes for `--help` at argv[1] and after a flag. Rank: 3

**F. `flag.Parse` stops at the first positional: `slk foo --channel X` silently drops the pin**
- What: stdlib flag stops at the first non-flag argument. `slk --channel X extra`, `slk extra --channel X`, `slk -- --channel X` all launch normally with no pin and no error. Also `slk --channel --fullscreen` consumes `--fullscreen` as the VALUE of `--channel`.
- Mitigation: after `fs.Parse`, reject non-empty `fs.Args()` with usage + exit 2; probe both argv shapes. Rank: 3

**G. R10 is flag-literal: ENV-sourced thread without any channel — exit 2 or R2 fallback? And `--thread` ts + `SLK_CHANNEL` (no `--channel` flag)**
- What: (1) `SLK_THREAD` set, no `SLK_CHANNEL`, no flags — a stale exported var now refuses to launch the TUI (fail-closed applied to input the user didn't type). (2) `--thread <ts>` with channel supplied only via `SLK_CHANNEL` — the effective channel exists per R1's equivalence, but R10's literal wording demands exit 2, contradicting R1. The two rulings collide and neither is marked to win.
- Mitigation: restate R10 over *effective* channel (flag or ENV): ENV-only thread with no effective channel → R2 fallback + notice (not exit 2); flag-supplied bare `--thread` with no effective channel stays exit 2. Rank: 3

**H. `--thread <permalink>` + `--channel <other>`: conflicting channel sources, no winner**
- What: a thread permalink carries its own ChannelID; `--channel` may name a different one. R1 precedence is per-variable, not per-fact, so it cannot resolve this. DOD-3 accepts both forms but never says which channel wins.
- Mitigation: decide once (permalink's embedded channel wins; mismatched `--channel` → notice; or hard error) and add to the `target-pure-parse` table. Rank: 3

**I. Zero-workspace / onboarding interplay with a pin is unhandled**
- What: with no workspaces configured, `slk --channel X` runs onboarding. After the user adds a workspace and it connects, does the stale pin apply? The plan has no item touching onboarding.
- Mitigation: rule: pin applies only if a workspace was configured at parse time; onboarding path drops the pin silently (or applies it — decide) and note in `cli-help-doc`. Rank: 3

**J. The cmd/slk argv probes are untestable without a main() extraction that no item includes**
- What: `cli-switch-compat` and `cli-flags-parse` demand table tests over argv vectors asserting outputs/exit codes "as main @ 7daa989" — but `main()` reads process-global `os.Args` and calls `os.Exit` directly (`cmd/slk/main.go:74-115`). The probes require first extracting a `parseEarlyArgs(argv []string) (…)`-shaped function; that refactor is a prerequisite item that doesn't exist.
- Mitigation: add an item: extract the argv[1] switch + flag layer into a pure-ish function returning (action, exitCode, error) with injected output writer, `main()` reduced to a shim. Rank: 3

**K. `SLK_FULLSCREEN` truthiness is unspecified — `SLK_FULLSCREEN=0` likely turns it ON**
- What: only empty/unset was pinned; `SLK_DEBUG` precedent (`internal/debuglog/debuglog.go:58`) is any-non-empty = on. Then `=0` / `false` / `no` — natural off spellings — enable fullscreen.
- Mitigation: pick one rule (recommend `strconv.ParseBool`, parse failure = unset + debug-log) and pin with `0`, `false`, `TRUE` cases. Rank: 3

## Rank 2

**L. Existing-switch argv[1] silently eats trailing new flags (`slk --version --channel X`)** — argv[1] match wins and returns; `--channel` silently ignored with exit 0. Document "early switches consume the whole command line" in printHelp, or make it a usage error. Rank 2.

**M. The `rg` single-read probe collides with the help-text item** — `cli-env-equivalents` gates on rg "SLK_(…)" returning exactly 1 non-test file; `cli-help-doc` requires printHelp output to contain SLK_CHANNEL. If accessor and printHelp are in different files, the probe fails a correct implementation. Mitigation: probe the read FORM (`os.Getenv("SLK_…")` call sites), and have printHelp format env names from the accessor's constants. Rank 2.

**N. stdlib gives `-channel` for free — an undeletable second spelling** — Go's flag package treats `-channel` ≡ `--channel`; cannot be disabled. Accept and document; probes use canonical double-dash only. Rank 2.

**O. Empty-string flag value: `--channel ""`** — empty ≡ absent stated for ENV only; extend to flag values; one table case. Rank 2.

---

**Files actually read:** flow/plans/2026-10-01-launch-pin-zoom.md (full); cmd/slk/main.go (L60-189, L1545-1624, grep windows L1359-1491); internal/slackurl/slackurl.go (full); internal/ui/reducer_zoom.go (L120-234); internal/ui/msgs.go (L388-412); internal/config/config.go (L160-234 + grep); internal/config/workspaces.go + _test.go; cmd/slk/onboarding.go (L42-111); cmd/slk/remove_workspace.go, cmd/slk/remint.go/_test.go; internal/slack/auth.go, internal/slack/client.go, internal/cache/workspaces.go, internal/slackdesktop/workspaces.go; repo-wide greps: SLK_* (all file types), os.Getenv|LookupEnv, flag.*, os.Args, type Workspace struct, type Token struct, Slugify(, Domain, isInitial|TeamIDForDefaultWorkspace. No files modified.
