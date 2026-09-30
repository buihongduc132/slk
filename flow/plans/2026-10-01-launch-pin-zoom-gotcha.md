# Gotcha Coverage — 2026-10-01-launch-pin-zoom

> Source: flow/plans/2026-10-01-launch-pin-zoom.md
> Mode: plan
> Sub-agents: 6 parallel batches (CLI/ENV · target-resolution · landing/zoom · fallback/DOD · test-gates · rulings), delegated per gotcha-coverage; batches 4 and 6 first spawns failed (markup echo / tool-reminder spam), re-spawned — deliverables are the re-runs'. Plus one separate cite-strip-cost-audit agent (see plan's Cost Audit appendix).
> Units reviewed: 15 declarative items + 6 DOD + rulings R1/R3/R8/R9/R10/R11/R12 (≤5 units/batch, full text, find-MISSED-only, rank-all)
> Date: 2026-10-01 · Per-batch appendix docs: `-gotcha-batch1..6.md` (traceability); originals untouched.

## Batch stats

| Batch | Units | Gotchas | R4 | R3 | R2 | Notes |
|---|---|---|---|---|---|---|
| 1 CLI/ENV surface | U1–U5 (cli-switch-compat, cli-flags-parse, cli-env-equivalents, cli-help-doc, R8) | 17 | 3 | 8 | 6 | |
| 2 target-resolution | U1–U5 (target-pure-parse, target-workspace-route, target-channel-resolve, target-thread-forms, R11) | 15 | 2 | 10 | 3 | summary says 4+10+3=17 counted items incl. sub-clauses; 15 titled |
| 3 landing/zoom | U1–U5 (land-channel, land-thread, land-zoomed, land-ordering, R3) | 12 | 4 | 4 | 4 | |
| 4 fallback/DOD | U1–U5 (fallback-normal, DOD-1..4) | 16 | 2 | 9 | 5 | re-run |
| 5 test-gates/conventions | U1–U5 (gates-green, agentsmd-index, DOD-5, DOD-6, R10) | 16 | 2 | 10 | 4 | |
| 6 rulings cross-cut | R9, R10, R11, R12, R1 | 15 | 2 | 9 | 4 | re-run |
| **total** | 30 units | **91 raw** | **15 R4** | **50 R3** | **26 R2** | |

Cross-batch duplicates are heavy (six batches independently found the width-dependent zoom pane, the thread-not-found runtime case, R10×ENV, R9×InitialActive). This doc consolidates to unique findings; per-batch docs retain the originals verbatim.

## Machine verification of top claims (planner, against main @ 7daa989)

| Claim | Verdict | Evidence |
|---|---|---|
| `config.Workspace` has no Domain/subdomain field; subdomain lives in token store | CONFIRMED | `internal/config/config.go:171-190`; `internal/slack/auth.go:18 Token.Domain` |
| InitialActive claimed by defaultTeamID-match else first-to-connect sync.Once | CONFIRMED | `cmd/slk/main.go:1386-1392, 1479-1491`; contract `internal/ui/msgs.go:396-402` |
| zoomFrontIsThread probes unzoomed layout; at ≥162 cols zoom promotes messages (B48) | CONFIRMED | `internal/ui/app.go:1008-1010`; `internal/ui/panellayout.go:134-138`; pinned by zoom_front_pane_test.go |
| `flag` stops at first positional; `--channel --fullscreen` consumes flag as value | CONFIRMED (stdlib semantics) | Go `flag` package Parse contract |
| main_scope_test allow-list pins exactly 7 names in main.go | CONFIRMED | `cmd/slk/main_scope_test.go:25-33` |
| startup_order_test string-anchors the WorkspaceReadyMsg send site | CONFIRMED | `cmd/slk/startup_order_test.go:34-55` |
| make test/lint omit `go vet ./...` and go.mod diff | CONFIRMED | `Makefile:9-13` |
| TestAGENTSMD_EveryTableRowNamesSomethingThatExists exists | CONFIRMED — **corrects cost-audit's "fabricated test" claim AND the plan's own probe, which misspelled it "…AnythingThatExists"** | `internal/ui/agentsmd_registration_test.go:65` |
| AGENTSMD test checks table→exists only; unregistered helpers pass | CONFIRMED (doc'd direction gap) | `agentsmd_registration_test.go:208-214` per batch 5 |
| No SLK_CHANNEL/THREAD/FULLSCREEN reads exist anywhere today | CONFIRMED (batch 6 pre-check; planner grep agreed) | repo-wide rg, all file types: 4 hits, all in the plan doc itself |

## Consolidated findings (unique, ranked)

### Rank 4 — 7 unique after dedupe
1. **Width-dependent zoom pane (batch3 #1, batch4 #1, batch6 D)** — DOD-1 "thread open, pane already zoomed" is internally unsatisfiable at side-by-side widths (≥162 cols): zoom promotes messages, thread open-but-invisible until esc. Rule: thread target + `--fullscreen` → promote thread at any width; width-tabled probe. → items (lp-zoom-threadpane), R3 amended.
2. **Runtime "thread not found" undetectable pre-TUI (batch3 #2, batch2 #4, batch4 #2, batch5 #1)** — bogus/deleted thread_ts lands a stub thread that never fills, zoomed, silent — the user's verbatim fallback never fires. → item (lp-thread-notfound-runtime) + R6 amended.
3. **R9 workspace routing not wired into InitialActive + no match source (batch1 #3, batch2 #1, batch3 #3, batch6 A+B)** — three sub-gaps: (a) no configured field holds a subdomain (token-store Domain is the only pre-connect source); (b) pin-workspace must override defaultTeamID at `main.go:1479-1491`, not the send site; (c) pinned workspace failing to connect (WorkspaceFailedMsg) must release the claim or no workspace lands at all. → items (lp-workspace-domain-source), (lp-workspace-initialactive); R9 amended.
4. **Resolved-but-unlisted channel lands on Channels[0] and zooms it (batch3 #4, batch2 #2, batch4 #11)** — local list excludes archived/IM/closed-MPIM (bootstrap_adapters.go:417,445); GetConversationInfo succeeds for them; nothing hydrates the sidebar → bare `#` header, or worse. → item (lp-resolved-unlisted-hydrate); R11 amended.
5. **R10 vs ENV merge (batch1 #1, batch6 G, batch5 #9)** — R10 evaluated flags-only kills `SLK_CHANNEL=C1 slk --thread ts`; evaluated post-merge kills env-only misconfig. → R10 amended (merged-target rule; flag→exit2, ENV→fallback+notice).
6. **Cross-source channel conflict (batch1 #2, batch2 #5, batch6 H)** — `--thread <permalink→C1>` + `--channel C2` (or ENV mix) merges incoherent. → R13 ruling: permalink's channel wins; explicit conflicting `--channel` from another source → notice. → item (lp-cross-source-conflict).
7. **argv probe harness doesn't exist (batch6 J, batch5 #6)** — table tests over argv vectors require extracting a pure dispatcher from `main()` first; unbudgeted. → item (lp-argv-seam-extract).

### Rank 3 — consolidated into items (13 unique)
8. **Single-read-site probe wrong shape (batch1 #4, batch6 M, batch5 #10)** — file-count grep can't see same-file double reads and collides with cli-help-doc; t.Setenv test files break the naive form. Fix: probe read form `os.Getenv("SLK_…")` occurrences = 1, `-g '!*_test.go'`. → (cli-env-equivalents) probe amended (append-only note).
9. **SLK_FULLSCREEN truthiness (batch1 #8, batch2 #10, batch3 #11, batch4 #5, batch6 K)** — `=0`/`=false` must not zoom. → (lp-env-truthiness).
10. **Positional garbage swallowed (batch6 F, batch1 #6)** — `slk foo --channel X` silently unpinned; `--channel --fullscreen` value-swallow. → (lp-positional-reject).
11. **fs.Usage/ErrHelp second help surface (batch1 #7, batch6 E)** — `slk --channel X --help` prints stdlib usage, exit 0. → (lp-fsusage-wiring).
12. **Unbounded GetConversationInfo stall (batch2 #7, batch6 C)** — no deadline on root ctx. → (lp-resolve-timeout).
13. **First-content-frame flash (batch3 #6)** — spinner frames precede any select; zoom must enter synchronously in the landing reducer arm. → (land-ordering) amended.
14. **Launch-zoom not cancelled by user navigation (batch3 #7)** — enter zoom from the same arm that completes nav. → (land-ordering) amended.
15. **enterZoom empty-pane snapshot (batch3 #8)** — snapshot after content lands, not panel-open. → (land-ordering) amended.
16. **Double ChannelSelectedMsg race via OpenLinkMsg (batch3 #5)** — arm pendingLinkNav inside reduceWorkspaceReady InitialActive branch, never OpenLinkMsg; routeLink browser fallback hazard. → (land-thread) amended.
17. **OpenThreadForPermalink via OpenLinkMsg is wrong seam (batch3 #5/#10)** — domain re-check diverges from R9. → (land-thread) amended (same fix).
18. **Notice oracle wrong while zoomed (batch4 #3)** — statusbarText renders statusbar in isolation; zoomed notices only visible via composed frame. → (fallback-normal) probe amended.
19. **Notice composition/timing (batch4 #9, #10)** — multiple failures → one composed toast; fire after first WorkspaceReadyMsg; longer/sticky window vs 3s auto-clear and bootstrap overlay. → (lp-notice-compose).
20. **R3-partial silent (batch4 #4)** — channel-ok/thread-miss fires no notice. → folded into (lp-thread-notfound-runtime) + R3 amended.
21. **AGENTS.md registration has no cmd/slk table (batch5 #3) + test name misspelled in plan probe (planner)** — open a minimal cmd/slk table or note the gap; correct test name to TestAGENTSMD_EveryTableRowNamesSomethingThatExists. → (agentsmd-index) amended.
22. **Gates probe gaps (batch5 #4, #5, #15)** — run the six commands literally (make omits vet + go.mod diff); `git diff 7daa989 -- go.mod go.sum` not worktree diff; record golangci-lint version + -count=1. → (gates-green) amended.
23. **main.go scope gate + startup_order anchors (batch5 #7, #8)** — new decls go in cmd/slk/launch*.go, never extend mainGoAllowed; edits stay inside the composite literal; update both oracles if reshaping. → (cli-switch-compat) amended.
24. **DOD-5 two-layer pin (batch5 #11)** — resolver outcome table (cmd/slk) + zoomed frame (ui) with non-empty content token. → (lp-zoom-decision-two-layer).
25. **DM/MPIM name matching unreliable (batch2 #6, #9)** — display names, not handles; scope name-form to channels or second-pass handle match; ambiguity → notice, not first-match. → (target-channel-resolve) amended.
26. **Empty/whitespace values flag+ENV (batch1 #17, batch4 #6, batch2 #13, batch6 O)** — trim-then-empty ≡ unset for both flags and ENV. → (lp-empty-trim).
27. **Archived/closed by name never resolvable (batch2 #14, batch1 #9 sub-clause)** — document; strip `#` prefix (batch2 #8, batch1 #9). → (target-pure-parse) amended.
28. **Success-pin provenance notice (batch1 #11)** — "pinned via SLK_CHANNEL" toast on success too. → (lp-pin-provenance).
29. **Onboarding drop rule (batch6 I)** — zero-workspace run drops the pin. → (lp-onboarding-drop).
30. **`--threads` divergence (batch1 #13)** — user wrote plural; keep singular, never alias, document in help. → (cli-help-doc) amended.

### Rank 2 — recorded, not itemized (11 unique)
31. Permalink-to-reply-without-thread_ts no-op (batch4 #12, batch2 #12). 32. Malformed `--thread` ts with channel → shape check (batch1 #10, batch4 #13). 33. Channels[0] empty-list case (batch4 #14). 34. Notice truncation/sanitization (batch4 #15). 35. DOD-2 phantom third precedence tier (batch4 #16). 36. `-channel` single-dash free alias (batch6 N). 37. Early-switch eats trailing flags (batch6 L). 38. a.zoomed diff gate context-line FP (batch3 #9). 39. PinChannelID distinct field / RecordVisit side effect (batch3 #12). 40. make lint version pin (batch5 #15). 41. runKeyCases wrong harness for landing/zoom (batch5 #13) — folded into R7 note.

## Resolution mapping — every Rank 3+ finding → plan disposition

| Finding(s) | Disposition |
|---|---|
| 1 width-zoom-pane | appended item (lp-zoom-threadpane) |
| 2 thread-notfound-runtime | appended item (lp-thread-notfound-runtime) |
| 3 workspace routing (a,b,c) | appended items (lp-workspace-domain-source), (lp-workspace-initialactive) |
| 4 resolved-unlisted | appended item (lp-resolved-unlisted-hydrate) |
| 5 R10×ENV | appended item (lp-r10-env-merge) |
| 6 cross-source conflict | appended item (lp-cross-source-conflict) |
| 7 argv seam | appended item (lp-argv-seam-extract) |
| 8 probe shape | appendix note on (cli-env-equivalents) |
| 9 truthiness | appended item (lp-env-truthiness) |
| 10 positional | appended item (lp-positional-reject) |
| 11 fs.Usage | appended item (lp-fsusage-wiring) |
| 12 timeout | appended item (lp-resolve-timeout) |
| 13–15 flash/cancel/snapshot | appendix note on (land-ordering) |
| 16–17 OpenLinkMsg seam | appendix note on (land-thread) |
| 18 notice oracle | appendix note on (fallback-normal) |
| 19 notice compose/timing | appended item (lp-notice-compose) |
| 20 R3-partial silent | folded into (lp-thread-notfound-runtime) note |
| 21 AGENTS.md | appendix note on (agentsmd-index) |
| 22 gates gaps | appendix note on (gates-green) |
| 23 scope gate | appendix note on (cli-switch-compat) |
| 24 two-layer pin | appended item (lp-zoom-decision-two-layer) |
| 25 DM/MPIM names | appendix note on (target-channel-resolve) |
| 26 empty/trim | appended item (lp-empty-trim) |
| 27 archived + `#` strip | appendix note on (target-pure-parse) |
| 28 provenance | appended item (lp-pin-provenance) |
| 29 onboarding | appended item (lp-onboarding-drop) |
| 30 `--threads` doc | appendix note on (cli-help-doc) |
| Rank 2 (31–41) | recorded in this doc; no items (below action threshold) |

## Cross-references
- Cost audit (separate agent): verdict STRIP-COST-NOW on [OE1] R9 workspace routing, [OE2] select-message third form, [OE3] land-ordering standalone, [OE5] agentsmd-index — see plan `## Cost Audit appendix` for the ruling taken (items kept in degraded-scope form because three independent R4 findings mandate the mechanisms; the cost question is deferred to the user).
- Batch 5 #16 corrects R5's wording: the landing seam is `WorkspaceReadyMsg` fields, not `internal/core` types; `os.Getenv` in internal/ui would NOT breach boundary_test.go — R8 is the operative rule.
