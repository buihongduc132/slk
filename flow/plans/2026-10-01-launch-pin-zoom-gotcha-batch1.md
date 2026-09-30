## Gotcha-coverage — BATCH 1 (CLI/ENV surface, U1–U5)

Grounded against `main` @ 7daa989 (read-only; no files touched). Plan edge cases already listed (fallback, precedence chain, single-read-site intent, exit-2 rules) are excluded below.

---

### Rank 4

**1. R10 validation scope is undefined across the flags/ENV merge — false exit-2 or silent violation**
- **What:** R10 says `--thread <bare-ts>` without `--channel` → exit 2, and U2 places that check at flag-parse time. But the effective target is flags-over-ENV (U3/R1). If R10 runs on flags alone, `SLK_CHANNEL=C1 slk --thread 123.456` exits 2 despite a perfectly resolvable target. If it runs post-merge, then `SLK_THREAD=123.456 slk` (env-only violation) also hard-exits 2 before the TUI — a fail-closed death for an env misconfig, which contradicts R2's "env is the soft pin" spirit. The plan never says which layer owns the check or which source combination triggers it.
- **Why missed:** U2's probe table (`missing value / unknown flag / bare --thread without --channel`) tests flags only; U3's precedence probe tests resolution, not validation. Nobody asked *when* R10 evaluates relative to the ENV merge.
- **Severity:** breaks the advertised ENV workflow (user exports SLK_CHANNEL, later passes `--thread`) or makes env-only misconfig lethal.
- **Mitigation:** specify: R10 runs on the **merged** target, after flag+ENV resolution; flag-form violations exit 2; ENV-form violations degrade to R2 fallback+notice (env is soft). Add both mixed cases to U2's probe table.
- **Rank:** 4

**2. Cross-source field merging can produce an incoherent target (thread whose channel ≠ channel)**
- **What:** "flags over ENV" is undefined at field granularity. `--channel C2` + `SLK_THREAD=<permalink into C1>` or `--thread <permalink-to-C1>` + `SLK_CHANNEL=C2` merges into channel=C2 + thread(C1) — a thread ts that doesn't exist in the selected channel. Same conflict exists purely in flags (`--channel C2 --thread <permalink>`); the plan's target-pure-parse table lists forms but no conflict rule between a permalink-carried channel and an explicit `--channel`.
- **Why missed:** the precedence ruling (R1) was written as a ranking, not a merge semantics; the probe lists "flag+ENV" as one case assuming the *same* field conflicts.
- **Severity:** lands on C2 with a dead thread ts → confusing R2 "thread not found" fallback that looks like a bug.
- **Mitigation:** pin one rule: a `--thread` permalink carries its channel and **wins** over any explicit channel from a different source (or: conflict → usage error exit 2). Assert it in target-pure-parse's table with a cross-source conflict row.
- **Rank:** 4

**3. Launch-pin workspace (R9) never reaches the `defaultTeamID` gate — first-connect race decides the active workspace**
- **What:** `cmd/slk/main.go:1479-1491`: with no `default_workspace` configured, the **first** workspace to connect claims `InitialActive` and `router.Set`. A permalink pinning workspace B while A connects first means A's `WorkspaceReadyMsg` (with `InitialActive: true`, main.go:1588) fires the reducer's channel-selection block (reducer_workspace.go:232-252) against A's channel list — the pinned channel isn't in it → spurious fallback or wrong-pane flash before B is ready. The plan's only seam is "the WorkspaceReadyMsg send site opens the pinned channel"; no item wires R9's subdomain choice into the isInitial gating.
- **Why missed:** the plan treats workspace routing (R9) and landing (land-channel) as separate items; the connect-goroutine race between them is invisible unless you read main.go:1448-1491.
- **Severity:** multi-workspace users (the R9 use case!) get nondeterministic landing.
- **Mitigation:** resolved pin sets `defaultTeamID = <pinned workspace's team>` for this run, reusing the existing gate verbatim; test with two workspaces where the pinned one connects second. *(Flagged for the landing batch too — dedupe on merge.)*
- **Rank:** 4

### Rank 3

**4. U3's single-read-site probe counts files, not reads — and directly collides with U4**
- **What:** U3's probe (`rg 'SLK_(CHANNEL|THREAD|FULLSCREEN)'` → "exactly 1 non-test file") cannot catch the defect class R8/U5 exist to prevent: two `os.Getenv("SLK_CHANNEL")` calls **inside one file** pass; and U4 *requires* printHelp to contain the literal strings `SLK_CHANNEL`/`SLK_THREAD`/`SLK_FULLSCREEN`, so if the accessor lives in a new `cmd/slk/launch.go` while help stays in `main.go`, the probe fails a *correct* implementation — pushing the implementer to co-locate code wrongly or weaken the probe. Repo precedent for the right shape exists (`internal/image/tmux_single_read_test.go:121-137` asserts zero raw `os.Getenv` *call sites* via AST/scan; `internal/image/capability.go:35` uses `var getenv = os.Getenv` for injection).
- **Why missed:** probe written as a filename grep, not a read-site check; U3 and U4 drafted without cross-checking that U4 forces the var names into a second location.
- **Severity:** the one enforcement mechanism for the multi-read-site class (my mandated gotcha class) is structurally unable to detect it.
- **Mitigation:** pin the probe to the read form — e.g. `rg 'os\.Getenv\("SLK_(CHANNEL|THREAD|FULLSCREEN)"\)' --type go` returning exactly one match, or an AST-based test mirroring `tmux_single_read_test.go`; assert help-text mentions separately in U4.
- **Rank:** 3

**5. U1's switch inventory is incomplete and its "byte-identical table test" is not implementable as specced**
- **What:** three sub-flaws: (a) U1's list omits the bare-word aliases `"version"` and `"help"` (main.go:76,81) — a table test built from U1's list wouldn't protect them; (b) `--add-workspace`/`--remove-workspace` are **interactive** (help text: "interactive") — no argv-vector table can assert byte-identical output without a pty; (c) `--list-workspaces`/`--dump-*` output depends on the machine's real config → goldens must be captured under a hermetic `XDG_CONFIG_HOME` (precedent: `cmd/slk/xdg_single_source_test.go`), and captured **before** the refactor against 7daa989, not reconstructed afterward.
- **Why missed:** the probe assumed all switches are pure-print.
- **Severity:** the compat safety net has holes exactly where regressions hide (dropped bare aliases; environment-dependent goldens flaking in CI).
- **Mitigation:** extend the list with `version`/`help`; scope the byte-identical assertion to non-interactive switches, asserting only handler-reached for interactive ones; capture goldens under `t.Setenv("XDG_CONFIG_HOME", …)` in a pre-refactor characterization commit.
- **Rank:** 3

**6. Stdlib `flag` silently stops at the first non-flag token — positional garbage is swallowed, flags after it ignored**
- **What:** `slk foo --channel C1` reaches the FlagSet (argv[1] matches no switch), `flag.Parse` stops at `foo`, and `--channel C1` lands in `flag.Args()` — **never parsed, never an error, TUI launches unpinned**. Same for `--fullscreen false` (bool flags don't consume the next token → `false` is positional) and `--` terminators. The plan defines behavior only for *unknown flags* (exit 2), not for unexpected *positionals*.
- **Why missed:** R12 chose stdlib `flag` for its parser, but the plan's error taxonomy (`missing value / unknown flag`) maps to flag's error paths and ignores its silent-success path.
- **Severity:** a typo'd launch (`slk gen --channel x`) degrades to an unpinned launch with zero signal — the exact "thought I pinned it" failure the feature exists to kill.
- **Mitigation:** after parse, reject non-empty `fs.Args()` with usage on stderr + exit 2; add `slk foo --channel C1` and `--fullscreen false` rows to U2's probe.
- **Rank:** 3

**7. Two divergent help surfaces: stdlib `fs.Usage` vs `printHelp`, and `-h`'s special exit path**
- **What:** `slk --channel C1 --help` never reaches the main.go switch (argv[1] is `--channel`); stdlib flag special-cases undefined `-h`/`-help` → prints **fs's own usage** and exits **0** (ErrHelp path), while unknown flags exit 2. Unless `fs.Usage` is explicitly wired to the same text as `printHelp`, there are two help outputs that will drift — and U4 only tests one of them. Also note the asymmetry: plan says "usage → stderr, exit 2", but the `-h` path legitimately exits 0.
- **Why missed:** U4 assumes `--help` is always handled by the argv[1] switch; the second entry point via the FlagSet is invisible without knowing stdlib's ErrHelp behavior.
- **Severity:** the documented ENV/precedence/fallback text (U4's whole point) is missing from the help path users hit mid-command.
- **Mitigation:** set `fs.Usage = func(){ printHelp() }` (to stderr or stdout — pick one, pin it) so both entry points emit identical text; add `slk --channel C1 --help` to U4's probe.
- **Rank:** 3

**8. `SLK_FULLSCREEN` truthiness undefined — `=0` and `=false` are non-empty**
- **What:** U3 pins "empty/unset indistinguishable from absent" but never defines what non-empty values mean. `SLK_FULLSCREEN=0` and `=false` are the universal shell idiom for "disable", yet under any-non-empty-is-true they **zoom**. Same question for the flag form `--fullscreen=false` (stdlib parses it as false — good — but the ENV form has no parser).
- **Why missed:** bool ENV vars need a truthiness vocabulary; the plan treated ENV as a mirror of the flag's *presence*.
- **Severity:** inverted behavior for the most conventional disable spelling.
- **Mitigation:** define the accepted set explicitly (e.g. true/1/yes/on vs everything-else-false, or: any non-empty = true and document `unset` as the off switch) and table-test `0`, `false`, `TRUE`, `1`, garbage.
- **Rank:** 3

**9. Channel-name prefix stripping (`#general`, `@user`) unspecified — the most natural input always falls back**
- **What:** users write `--channel '#general'` and `SLK_CHANNEL=#general`. Slack channel names are unprefixed; R11 resolves names against the local list via `text.Fold`. Unless the resolver strips a leading `#` (and handles `@` for DMs), the pin silently fails → R2 fallback with a notice blaming "channel not found" for input any Slack user would consider correct.
- **Why missed:** target-pure-parse's form table lists "channel name" without a normalization rule; nobody role-played the shell command a user actually types.
- **Severity:** feature appears broken for the primary name-form use case.
- **Mitigation:** pure-resolver rule: strip one leading `#` from name forms (decide `@`-prefix policy for DM targets: strip+resolve as user, or reject with a targeted usage hint); add `#name` rows to the probe.
- **Rank:** 3

**10. Malformed bare ts *with* a channel: exit-2 vs fallback classification missing**
- **What:** R10 covers structurally-meaningless combos; R2/R11 cover unresolvable-but-well-formed ones. `slk --channel C1 --thread notats` is neither: well-formed argv, garbage ts. Exit 2 (fail fast on malformed input) and R2 fallback+notice are both defensible; the plan picks neither, so the test will be written ad hoc and the behavior will be accidental.
- **Why missed:** the parse-time/resolve-time boundary was drawn for structure, not syntax validation of values.
- **Severity:** inconsistent failure UX; a bad ts looks identical to a deleted thread (or kills the launch entirely, depending on which way it falls out).
- **Mitigation:** classify: ts-shape validation at parse time → exit 2 with usage; shape-valid but unresolvable → R2. One table row fixes it.
- **Rank:** 3

**11. Persisted ENV pin is invisible — no notice distinguishes "pinned via SLK_CHANNEL" from normal launch**
- **What:** requirement (a) invites users to `export SLK_CHANNEL=…` in their shell profile. Every subsequent launch then overrides last-visited state silently; the plan's only notice fires on *failure* (R2). A user who forgot the export (or inherited it from a wrapper/alias) experiences "slk keeps opening #general" with no discoverable cause.
- **Why missed:** notices were designed as an error channel, not a provenance channel.
- **Severity:** support-vector confusion exactly matching the feature's target audience (launcher-wrapper users).
- **Mitigation:** on a successful pin, emit the same non-blocking notice naming the source ("pinned to #x via SLK_CHANNEL"); costs nothing, turns silent state into visible state.
- **Rank:** 3

### Rank 2

**12. `--channel --fullscreen` silently consumes the flag as a value** — stdlib non-bool flags take the next token unconditionally; the "missing value" error fires only when `--channel` is the last argv. Pin and test `slk --channel --fullscreen` (expect: value `--fullscreen` → unresolvable → R2 notice, *not* exit 2 — or pre-validate that values don't start with `-`; either, but decide). *Rank 2*

**13. `--threads` divergence (user verbatim) — keep singular, never alias** — the user wrote `--threads`; plan standardized to `--thread`. Fail-closed exit 2 makes the divergence loud on day one, which is acceptable; adding a `--threads` alias would create the exact two-spellings defect class U5/R8 forbids (delete-one rule). Note it in help text ("`--thread`", singular) so the usage error is self-explanatory. *Rank 2*

**14. U4's probe forgets `SLK_THREAD`** — plan line 65 asserts help contains `SLK_CHANNEL`, `SLK_FULLSCREEN`… but not `SLK_THREAD`, while U4's text says "the three ENV vars". A regression dropping SLK_THREAD docs passes the probe. One-line probe fix. *Rank 2*

**15. Exit-2 paths are untestable in-process under `ExitOnError`** — U2's "unit test table" requires the parse function to *return* errors (ContinueOnError) with `os.Exit` at exactly one call site in `main`; ExitOnError makes every case a process kill. State it as an implementation constraint so the probe is buildable. *Rank 2*

**16. Flag-layer position vs the pre-probe config load unpinned** — parse must sit between the switch (main.go:115) and the preCfg load/probe (main.go:126-151); otherwise a usage error pays config load + protocol detection first and stderr interleaves with probe logs. *Rank 2*

**17. `--channel ""` (explicitly empty) undefined** — should be treated as unset (mirroring U3's empty-ENV rule) rather than "unresolvable target → fallback notice". One probe row. *Rank 2*

---

**Zero-modification confirmation:** read-only respected — no files created, modified, or committed.

**Files grepped/read for grounding:** `flow/plans/2026-10-01-launch-pin-zoom.md` (full); `cmd/slk/main.go` (lines 1-220, 1420-1494, 1550-1599); `internal/ui/boundary_test.go` (full); `internal/ui/editor.go` (full); `internal/slackurl/slackurl.go` (full); `internal/ui/reducer_workspace.go` (195-274); repo-wide greps: `os.Getenv|os.LookupEnv` (*.go), `SLK_[A-Z]+` (all files), `os.Args` (*.go), `LastChannelID` (*.go), `isInitial` (cmd/slk). Confirmed: no `SLK_CHANNEL/THREAD/FULLSCREEN` reads exist anywhere yet; the only argv consumer today is `cmd/slk/main.go:74-115`.