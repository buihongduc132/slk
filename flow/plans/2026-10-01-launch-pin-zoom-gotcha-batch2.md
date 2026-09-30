READ-ONLY review complete. Batch 2 (launch-target resolution, U1–U5) findings, grounded in repo reads; plan edge cases (cid-param ignored, R10 bare-ts error, precedence flags>ENV, fallback-on-miss) excluded as already covered.

---

## Rank 4 — significant correctness

**1. Pin-to-workspace (R9) is not wired into the InitialActive selection — race + failure semantics undefined**
- **What:** R9 says a permalink subdomain "selects the matching configured workspace," and land-channel says "the InitialActive workspace opens the pinned channel." But `InitialActive` is claimed by `defaultTeamID` match or a `firstReady` sync.Once first-to-connect race (`cmd/slk/main.go:1479-1489`, `1408-1413`; contract at `internal/ui/msgs.go:396-408`). With no `default_workspace` configured, an unrelated workspace winning the connect race becomes InitialActive while the pin targets another — the pin then silently doesn't apply, or applies nondeterministically run-to-run. Also unspecified: the pinned workspace connects *slowly* (user stares at loading overlay while another is ready) or *fails* (`WorkspaceFailedMsg`, main.go:1468) — neither "wait with timeout then fall back" nor "land default, switch later" is decided.
- **Why missed:** U2 treats workspace routing as pure name-matching; the landing units treat InitialActive as given. The interaction (three competing selectors for one slot) is in no unit.
- **Severity:** high — wrong-workspace or nondeterministic landing; potential indefinite loading.
- **Mitigation:** Add a ruling: pinned-workspace-teamID, when resolvable from config, overrides `defaultTeamID` and disables the `firstReady` race; pinned workspace gets a bounded connect grace (reuse the same failure path as `WorkspaceFailedMsg` → R2 fallback + notice). Pin it with a two-workspace test where the non-pinned one connects first.

**2. "Resolved" ≠ "in the sidebar": the GetConversationInfo path lands channels with no list entry**
- **What:** R11's one-`GetConversationInfo` fallback fires exactly for channels *missing from the local list* — and the local list deliberately excludes archived channels and IMs (`cmd/slk/bootstrap_adapters.go:417,445`) and closed MPIMs (`:420-422`, `dropClosedMPIMs:487`). `conversations.info` happily returns archived channels, so "resolution succeeds" for the most plausible pin target (old thread in an archived channel) while `wctx.Channels`/`FinderItems` have no entry — no `sidebar.ChannelItem` is built (nothing routes the info result through `buildChannelItem`, `cmd/slk/channelitem.go:46`), so the pane header/statusbar render a bare `#`, the sidebar can't highlight it, and compose addressing is undefined. Same hole for non-member public channels and closed MPIMs resolved via raw ID.
- **Why missed:** U3 ends at "resolves"; no unit covers *landing* a resolved-but-unlisted channel, and no unit decides whether `IsArchived`/`!IsMember` in the info result counts as found or as an R2 fallback.
- **Severity:** high — the flagship use case (deep-linking an old thread) lands in a half-rendered channel.
- **Mitigation:** Decide explicitly: info-result `IsArchived`/non-member → either R2 fallback+notice, or synthesize a `sidebar.ChannelItem`+finder item (via `buildChannelItem`) and append to `wctx.Channels` before `WorkspaceReadyMsg` is sent. Pin with a test using an archived-channel permalink.

## Rank 3 — moderate, plausible, needs handling

**3. No configured field holds a workspace subdomain — and the one that exists may be empty or miscased**
- **What:** U2 matches "the matching configured workspace," but `[workspaces.*]` config carries only slug/TeamID (`internal/config/config.go:305-334`); the subdomain lives in the token store (`internal/slack/auth.go:18`), which can be empty (`cmd/slk/remint.go:37-40` re-mints precisely when `Domain == ""`), in `cache.Workspace.Domain`, and live via `Client.TeamSubdomain()` (post-connect only). Plan doesn't say which source. Additionally `slackurl.Parse` does not lowercase the host (`internal/slackurl/slackurl.go:48-52` — `net/url` preserves case), so `https://MyTeam.slack.com/...` yields `Subdomain:"MyTeam"` and a case-sensitive compare against `"myteam"` misses → silent default-workspace detour.
- **Why missed:** U2 assumes a domain↔workspace mapping exists in config; it doesn't, and the pure-parse unit never considers host case.
- **Severity:** medium — degrades to wrong workspace + fallback notice, looks like "feature broken."
- **Mitigation:** Specify the match source (token-store `Domain`, falling back to cache) and normalize both sides (`text.Fold` or `strings.ToLower`) in the workspace-route unit; test with empty-Domain token and mixed-case URL.

**4. Thread existence is undetectable at resolve time — the user's verbatim "fallback if threads not found" has no launch-time mechanism**
- **What:** `slackurl.Parse` returns `ThreadTS` verbatim from the query with zero validation (`slackurl.go:66`) — `?thread_ts=garbage` parses ok, and a syntactically-valid ts pointing at a nonexistent/deleted thread is indistinguishable from a real one until the fetch lands. Per R6/land-thread the thread opens as stub+backfill *after* first paint and after zoom is applied; if replies come back empty, nothing reverts to the R2 "lands as normal" behavior the user asked for — you get a zoomed empty thread.
- **Why missed:** U4 enumerates forms, not validity; resolution (U3) only validates *channels*. The plan's fallback is defined at channel-resolution time only.
- **Mitigation:** Add a ruling for the post-landing case: `ThreadRepliesLoadedMsg` with zero replies + stub parent whose fetch also found nothing → close thread panel, drop zoom or re-zoom channel per R3, emit the same non-blocking notice. Pin with a test.

**5. `--channel` vs `--thread`-permalink channel conflict has no precedence rule**
- **What:** R10 handles `--thread <bare-ts>` without `--channel`. Nothing handles `--channel C1 --thread <permalink→C2>` (or the ENV mix `SLK_CHANNEL=C1` + flag permalink): three defensible behaviors (usage error, permalink wins, `--channel` wins). U4's "no fourth behavior" claim is about landing forms, not input conflicts — this is an unspecified input, and R1's per-flag precedence doesn't resolve cross-flag consistency.
- **Severity:** medium — implementer's guess becomes the contract.
- **Mitigation:** One ruling: when both are present and disagree → usage error exit 2 (deterministic, matches R10's fail-closed posture); when they agree, no-op. Table-test it.

**6. Name-form matching is undefined for DMs and MPIMs — the local list stores display names, not handles**
- **What:** R11 says name forms match the local list via `text.Fold`, but the list's `Name` for DMs is the *display name* from `UserNames`, falling back to the raw userID when unresolved (`cmd/slk/channelitem.go:65-71`; `UnresolvedDM` exists because this happens at boot, `cmd/slk/workspace.go:16-20`), and MPIMs get `FormatMPDMName` participant lists (`:72-76`). So `--channel alice` matches only if the cached display name happens to be "alice"; `GetConversationInfo` can't help (it takes an ID). Unresolved-DM timing makes this vary between runs.
- **Severity:** medium — DM pinning by name is unreliable; failure looks like a bug, not a fallback.
- **Mitigation:** Either scope name-form to channel-type entries only (documented), or match DMs by peer handle via `UserNamesByHandle` as a second pass. Decide in U3; test with an unresolved DM.

**7. The "bounded by context" GetConversationInfo runs on the app root context — an unbounded call stalls first paint**
- **What:** R11 bounds the call "by context," but the resolution site (the connect goroutine, before `WorkspaceReadyMsg` at `main.go:1573-1590`) inherits `run()`'s root context (`main.go:1448-1453`) — no deadline; the HTTP client sets no timeout (`internal/bootstrap` docs the same class of hazard). A wedged proxy/captive portal pins the call indefinitely = infinite loading overlay, worse than the R2 fallback it was meant to protect.
- **Severity:** medium-high under bad networks.
- **Mitigation:** Specify `context.WithTimeout` (~3–5s) at the resolver call site in U3's item text; unit-test the timeout-expiry path yields fallback.

**8. `#`-prefixed names are not stripped — the natural way users write channels fails**
- **What:** `--channel #general` folds to "#general" ≠ "general" → miss → fallback notice quoting a target that *looks* correct. The requirement's own origin thread writes "#slk C0C4K3MTXGS" — this notation is native to the user.
- **Severity:** low-medium but high frequency.
- **Mitigation:** Strip a single leading `#` in the pure resolver (U1) before classification; test row in U1's table.

**9. Name-collision nondeterminism: first match over an unordered list**
- **What:** Channel names are unique among channels, but sidebar `Name` is display-name for DMs and formatted lists for MPIMs — "eng" as a channel and a DM display name "eng" collide. R11 says "folded match" with no tie-break; a map-iteration or slice-order first-match makes the pick vary run to run (sidebar order also shifts with sections).
- **Severity:** medium — nondeterministic landing target.
- **Mitigation:** Define precedence (channels > private > group_dm > dm) and error/notice on ambiguity rather than first-wins; or determinism via the sidebar's own order.

**10. `SLK_FULLSCREEN=0`/`false` — truthiness unspecified, non-empty-is-on footgun**
- **What:** cli-env-equivalents pins only "empty/unset ENV is indistinguishable from absent." Any wrapper exporting `SLK_FULLSCREEN=0` (or `false`, `no`) to *disable* zoom will enable it under a naive `os.Getenv != ""` read.
- **Severity:** medium for scripted callers, which is exactly the ENV audience.
- **Mitigation:** Define the truthy set (`1/true/yes/on`, case-folded; everything else = off) in R1's item; test `0`, `false`, `TRUE`.

**11. Classification of a `--thread` value that fails `slackurl.Parse`: exit 2 vs R2 fallback unspecified**
- **What:** A `--thread` value that *looks* like a URL but fails Parse (e.g. `slack://archive?p=...`, `https://team.slack.com/archives/C1` channel-only link — rejected by the regex, `slackurl.go:37,56-59`, test `:45`) is neither a valid ts nor a valid permalink. cli-flags-parse exits 2 for parse errors; DOD-4 says unresolvable → normal launch + notice. Which regime governs is unstated; U1 only says the *pure* function returns ok=false.
- **Severity:** medium — wrong choice either breaks scripts (exit 2 on a typo'd URL) or swallows errors (launch with a garbage ThreadTS string passed downstream).
- **Mitigation:** Ruling: syntactically-flagged-as-permalink (matches `^https?://` or contains `.slack.com`) but Parse-fails → R2 fallback + notice; anything else non-ts → exit 2. Test in U1's table.

**12. Permalink to a thread *root* without `thread_ts` lands select-message, not thread-open**
- **What:** U4 maps message-ts-without-thread_ts to select-message. But Slack web opens the *thread* when a /p link points at a message with replies, and `--thread` semantics elsewhere in slk (openThreadForSelectedMessage) do too. The root's reply-count is only known after `FetchAround` lands (`internal/ui/reducer_links.go:140-150`), so the resolver can't distinguish at parse time.
- **Severity:** medium behavioral surprise, not a crash.
- **Mitigation:** Decide explicitly and document: either accept select-message (test pins it, help text says so), or add the post-fetch upgrade (if selected row `ThreadTS != ""` → open thread panel) as a named behavior within U4's two forms.

## Rank 2 — minor

**13. Explicitly empty flag value defeats ENV** — `--channel "$MAYBE_UNSET"` passes a set-but-empty flag; `flag.Visit` sees it as set, so flags>ENV wins and the empty string becomes the target → ok=false → fallback while `SLK_CHANNEL` is ignored. Mitigation: treat empty flag value as unset in the resolver; test row. (U1)

**14. Archived/closed conversations resolvable by ID but never by name** — asymmetric by construction of R11 (names match local list only, which excludes archived at `bootstrap_adapters.go:417,445`). Document in help; optionally notice text should say "not in your joined channels." (U3)

**15. Host edge forms silently rejected** — `sub.slack.com:443` (port defeats `HasSuffix`, `slackurl.go:49`), `x.enterprise.slack.com` (subdomain "x.enterprise" won't match stored "x"). Both degrade to fallback; fine, but worth one test row each so it's pinned as intended. (U1/U2)

---

**Files read/grepped:** `flow/plans/2026-10-01-launch-pin-zoom.md`; `internal/slackurl/slackurl.go`, `slackurl_test.go`; `internal/bootstrap/bootstrap.go`; `cmd/slk/main.go` (60–160, 1370–1649), `cmd/slk/workspace.go`, `cmd/slk/connect.go` (32–151), `cmd/slk/bootstrap_adapters.go` (400–489), `cmd/slk/channelitem.go`; `internal/ui/msgs.go` (380–419), `internal/ui/reducer_links.go` (60–179); `internal/slack/client.go` (545–575); `internal/config/config.go` (300–369); `internal/text/fold.go`; `internal/slack/auth.go` + `cmd/slk/remint.go` (grep). Greps: GetConversationInfo, UnresolvedDM, isInitial, default_workspace/TeamSubdomain, Domain, SelectByTS/FetchAround, buildChannelItem, IsArchived, sidebar.ChannelItem.

**Summary:** 15 gotchas (3× rank 4 territory collapsed into 2 rank-4 + 10 rank-3/2 items above; ranked 4,4,3×10,2×3). Top two: the R9 workspace-selection/InitialActive race and the resolved-but-unlisted-channel landing (archived-channel permalinks being the canonical pin target). No files modified.