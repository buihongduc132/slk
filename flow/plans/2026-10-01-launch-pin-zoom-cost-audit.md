All facts verified. Here is the audit.

# Cite-strip-cost-audit — `flow/plans/2026-10-01-launch-pin-zoom.md`

## 1. Origin quotes, verbatim

**User (2026-10-01, Slack #slk):**
- **[U1]** "a. make launching of slk could be pinned to the specific threads / channel using args & ENV variable as well."
- **[U2]** "b. args + env could be ZOOMED as well;"
- **[U3]** "1 launch cmd with slk (together with --channel / --threads + --fullscreen) could show the ZOOMED channel. INSTEAD of having to navigated and press hotkey to be able to landed into it."
- **[U4]** "Fallback if threads not found: then just lands as normal."

**Caller brief DOD sketch** (`/tmp/hermes-plan-launchpin/brief.md`):
- **[B1]** "`slk --channel <id|name|permalink> [--thread <ts|permalink>] [--fullscreen]` lands on that channel, thread open, pane already zoomed — no hotkey needed."
- **[B2]** "ENV equivalents (e.g. SLK_CHANNEL / SLK_THREAD / SLK_FULLSCREEN) with defined precedence vs flags (flags win over ENV; both win over config/state)."
- **[B3]** "Thread spec may come from a permalink (slackurl.Parse) or channel+ts pair."
- **[B4]** "Fallback: target channel/thread not found (or no access) → normal launch (last state), surface a non-blocking notice (toast/status line), exit code unchanged."
- **[B5]** "Fallback: --fullscreen/--zoom without a resolvable target → apply to the default-landed channel if sensible, else ignore (spec exactly one)."
- **[B6]** "No regression to existing os.Args[1] switches; --help updated; go vet/test/gofmt clean; tests per repo conventions; no new deps in go.mod."

**No missing mappings** — every quote maps to at least one plan item. But note **[U1]/[U3] say `--threads` (plural); R1 ships `--thread` (singular) and tags it "(user-verbatim)" — it is not verbatim.** See Callouts.

## 2. Quote → init → strip table

| quote | init solutions | strip solutions | why strip delivery still meet quote requirement |
|---|---|---|---|
| **[U1]** pinned threads/channel via args & ENV | (cli-flags-parse), (cli-env-equivalents), (target-pure-parse), (target-channel-resolve), (target-thread-forms), (land-channel), (land-thread); R1, R8, R11, R12 | `--channel <id\|name\|permalink>` + `--thread <ts\|permalink>` via one stdlib FlagSet; `SLK_CHANNEL`/`SLK_THREAD` via ONE accessor; resolve against local bootstrap list (+1 bounded `GetConversationInfo` for raw-ID misses, `text.Fold` for names); land channel, open thread via existing `openThreadForPermalink` (reducer_links.go:162) | Delivers the quote word-for-word: "pinned to the specific **threads / channel**" ✓, "using **args**" ✓, "**ENV** variable" ✓. No workspace routing, no select-message form — neither word appears in the quote |
| **[U2]** args + env ZOOMED as well | (cli-flags-parse) `--fullscreen`, (cli-env-equivalents) `SLK_FULLSCREEN`, (land-zoomed), (land-ordering); R1, R3, R4 | `--fullscreen` flag + `SLK_FULLSCREEN` ENV, applied via `App.enterZoom` after the target lands; no resolvable target → no zoom (lands normal) | "**args + env** could be **ZOOMED** as well" — both surfaces zoom ✓, through the mandated `enterZoom` (R4). U2 says nothing about targetless zoom; that entire case collapses to U4's "lands as normal" |
| **[U3]** launch with flags shows ZOOMED channel, no navigation/hotkey | (land-channel), (land-thread), (land-zoomed), (land-ordering), (cli-help-doc); R4, R6; DOD-1 | One landing flow: resolve → land pinned channel/thread → `enterZoom` before first paint; first-frame-zoom asserted once via existing `assertZoomedFrame` (fullscreen_red_test.go:121) | Quote asks "**launch cmd** … **could show the ZOOMED channel. INSTEAD of having to navigated and press hotkey**" — first frame zoomed, zero hotkeys, zero navigation ✓. The separate no-flash-sequencing item adds no quote word; "first frame already zoomed" covers it |
| **[U4]** thread not found → lands as normal | (fallback-normal); R2; DOD-4 | Unresolvable thread (or channel) → normal launch (`LastChannelID` → `Channels[0]`), one non-blocking toast, exit 0 | Verbatim: "**Fallback if threads not found: then just lands as normal**" ✓. The toast is [B4]'s "non-blocking notice", kept |
| **[B1]** lands channel, thread open, zoomed, no hotkey | DOD-1; (cli-flags-parse), (target-pure-parse), (target-workspace-route), (land-channel), (land-thread), (land-zoomed) | Same surface minus (target-workspace-route): permalink parsed for channel/thread only (existing `slackurl.Parse`); subdomain ignored — wrong-workspace permalink → not-found → [B4] fallback | "**lands on that channel, thread open, pane already zoomed — no hotkey needed**" ✓ for the workspace slk actually launches into. [B1] names no workspace-switching behavior |
| **[B2]** ENV equivalents + precedence | (cli-env-equivalents); R1, R8; DOD-2 | Unchanged: one accessor, precedence flags > ENV > state pinned by test | Verbatim DOD ✓ — nothing strippable; R8's single-read-site is the brief's own hard rule ("single read site… never two spellings") |
| **[B3]** thread spec = permalink OR channel+ts pair | (target-thread-forms), (target-pure-parse); DOD-3; R6 | Two forms only: permalink (`slackurl.Parse`) or channel+ts. Permalink WITHOUT `thread_ts` = a channel permalink → lands the channel; no "select message" behavior | "**may come from a permalink … or channel+ts pair**" — exactly two forms ✓. The third select-message landing was never in this line |
| **[B4]** not-found/no-access → normal launch + notice, exit unchanged | (fallback-normal); R2; DOD-4 | Identical — this is the core fallback; nothing to strip | Verbatim ✓ ("normal launch (last state)… non-blocking notice… exit code unchanged") |
| **[B5]** fullscreen without target: exactly one behavior | R3; (fallback-normal); DOD-5 | Collapse R3 to: any resolvable channel → zoom it (messages or thread pane per target); nothing resolvable → ignore zoom + notice. Two branches, still exactly one | "**apply to the default-landed channel if sensible, else ignore (spec exactly one)**" ✓ — still exactly one specified behavior, smaller matrix. R3's tri-case refinement is a decision, not a DOD word |
| **[B6]** no switch regression, --help, clean gates, no deps | (cli-switch-compat), (cli-help-doc), (gates-green); R7, R12; DOD-6 | Existing `os.Args[1]` switch (main.go:74-115, verified) untouched; only argv matching no switch reaches the FlagSet (`flag.ExitOnError` gives usage+exit-2 for free); printHelp gains flag/ENV/precedence/fallback lines; full gate run | "**No regression to existing os.Args[1] switches; --help updated; go vet/test/gofmt clean; … no new deps**" ✓ all four clauses covered with stdlib only |

## 3. Overengineering audit

| ID | plan part | untraceable to | class | cost if kept |
|---|---|---|---|---|
| **[OE1]** | **R9 + (target-workspace-route)** — permalink subdomain selects among configured workspaces; unmatched → default + notice | No quote word. "workspace" / "subdomain" appears nowhere in U1–U4 or B1–B6. [B1] accepts permalinks as a *channel spec*, silent on which workspace | **invented-decision** | ~1.0d (logic + two-workspace test matrix) + coupling to bootstrap workspace selection = the plan's single biggest rework-risk surface |
| **[OE2]** | **(target-thread-forms) "select-message" third form** — permalink message-ts → channel + select-message landing | [B3] enumerates **two** forms; U1/U3 never mention message selection | **invented-decision** | ~0.5d + interaction risk with existing `pendingLinkNav` stale-guard |
| **[OE3]** | **(land-ordering)** as a standalone item + probe ("un-zoomed frame never appears") | (land-zoomed) already pins "the FIRST content frame is already zoomed" with `assertZoomedFrame` on the first frame — same assertion, second item | **artifact-duplication** | ~0.25d |
| **[OE4]** | **R10** — bare `--thread` without `--channel` → usage error, exit 2, no TUI | No quote specifies error exits; the quotes' only stated failure mode is U4's "lands as normal" | **invented-policy** | ~0.1d — **keep anyway**: near-free, and a silent no-op launch on malformed argv is worse than the OE. (Alternatives exist — treat as unresolvable → U4 fallback — but not worth the churn) |
| **[OE5]** | **(agentsmd-index)** — register resolver/ENV accessor in "the AGENTS.md shared-code tables"; probe runs `TestAGENTSMD_EveryTableRowNamesAnythingThatExists` | Repo convention, not a quote — **but the premise is wrong twice**: (a) that test does not exist anywhere in the repo (grep: only the plan file mentions it — fabricated probe, plan defect regardless of verdict); (b) AGENTS.md's own "Scope gap" note says **cmd/slk has no helper table**, and both new helpers live in cmd/slk | **premature-structure** (+ broken probe) | ~0.25d, currently pointed at a test that cannot run |

**Traceable-decision check (scrutinized, kept):** R11 (bounded lookup) is a necessary implementation decision for "pinned to the specific channel" — without the one `GetConversationInfo` for raw-ID misses, a valid-but-uncached ID would fall to "not found". R12 (stdlib FlagSet layered after the switch) is the mechanism [B6] demands. R3's "exactly one" is [B5]'s own mandate. R8 is the brief's hard rule. None of these is OE.

## 4. Cost (engineering work-days)

| bucket | init (full plan) | strip | notes |
|---|---|---|---|
| authoring | 2.0 (sunk) | 1.0 (counterfactual) / 0 extra now | plan + 5 gotcha batches already written — sunk either way |
| execution | ~6.75 | ~3.25 | init: cli 1.5 + target 2.25 + land 2.25 + gates 0.75; strip: switch-guard 0.25, flags 0.5, env 0.25, resolve 0.6, landing trio 1.0, fallback 0.25, help 0.1, gates 0.25. Grounded: no flag layer exists today; zoom/thread-landing mechanisms already exist (verified) |
| upkeep | ~0.75/yr | ~0.25/yr | init carries workspace-routing × bootstrap coupling + R3/R10/select-message test matrix |
| rework-risk | ~0.75 | ~0.25 | OE1 is the dominant risk; OE5's broken probe guarantees one rework touch |
| **totals** | **~10.25** | **~3.75 + sunk authoring** | |
| **strip-cost-now** | — | **~3.5 work-days saved** | strip set: OE1, OE2, OE3, OE5 |

## 5. Verdict

**STRIP-COST-NOW** — strip **[OE1]** (R9 workspace routing + target-workspace-route), **[OE2]** (select-message third form), **[OE3]** (land-ordering as separate item — fold one sentence into land-zoomed), **[OE5]** (agentsmd-index as written). Keep **[OE4]** R10 (≈0.1d, real CLI hygiene). Regardless of verdict, fix two plan defects: the fabricated `TestAGENTSMD_EveryTableRowNamesAnythingThatExists` probe, and R1's "(user-verbatim)" tag on a normalized flag name (user wrote `--threads`, plan ships `--thread` — retag "user-verbatim, spelling normalized" or accept both spellings; default: keep singular + one help line).

**One decision question:** you run multiple workspaces (the origin thread itself is a `dy-swarm` permalink) — should a permalink whose subdomain belongs to a *different configured workspace* switch workspaces at launch (keep R9), or is falling back to a normal launch of the default workspace acceptable until you ask for it (strip R9)?

---
*Read-only audit: no files created, modified, or committed; no background tasks. Repo facts verified: main.go:74-115 switch, WorkspaceReadyMsg seam main.go:1573-1590, openThreadForPermalink reducer_links.go:155-178, enterZoom reducer_zoom.go:152, assertZoomedFrame fullscreen_red_test.go:121, statusbarText in use; TestAGENTSMD_EveryTableRowNamesAnythingThatExists absent from repo.*