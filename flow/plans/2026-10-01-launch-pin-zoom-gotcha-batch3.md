# Gotcha-coverage BATCH 3 — landing + zoom sequencing

Read plan first; all findings are MISSED items (nothing re-reviewed from the plan's own list). Rank 3+ first.

---

**1. No ruling for which pane zooms when a thread target lands with `--fullscreen` — behavior silently flips on terminal width**
- **What**: U3/U5/R3 rule only the *fallback* cases. In the success case (channel+thread resolved+zoom), `enterZoom` promotes whichever pane `zoomFrontIsThread()` picks (`internal/ui/reducer_zoom.go:156`, `internal/ui/app.go:1008-1010` → `threadInFront()`, focus-based). At stacked widths that's the thread; at side-by-side widths (≥162 cols, `internal/ui/panellayout.go:138`) zoom **always promotes the messages pane** (pinned by `zoom_front_pane_test.go` / B48) — thread open (`threadVisible=true`) but **not drawn** (ThreadWidth=0, `panellayout.go:134-135`) and **unfocusable** (`threadFocusable`, `reducer_zoom.go:74-79`; focus normalized off it, `:119-124`). DOD-1 ("thread open, pane already zoomed") is internally unsatisfiable at wide widths: the user lands zoomed on the channel with the thread they asked for invisible until esc. The probe `assertZoomedFrame` will silently encode one arbitrary, width-dependent answer.
- **Why missed**: U5 enumerates only the two failure branches of R3; nobody asked what zoom promotes when both targets resolve.
- **Severity**: Primary success path behaves differently on a 140-col vs 200-col terminal; contradicts "never zoom-on-wrong-pane" in spirit and the user's "land INTO it" intent.
- **Mitigation**: Add a ruling: thread target + `--fullscreen` → zoom promotes the thread at any width (explicitly overriding `zoomFrontIsThread` for launch-zoom, e.g. focus the thread before enterZoom AND force the stacked-front answer), or explicitly document "zoom the channel, thread behind". Pin with a test at both a stacked and a side-by-side width.
- **Rank**: **4**

**2. "Thread not found" is undetectable before the TUI starts — R3's mid-branch and the user's explicit fallback are unimplementable as specified**
- **What**: R3's "channel resolved but thread not → zoom the channel" and the requirement's "Fallback if threads not found: then just lands as normal" both require knowing a thread doesn't exist at launch. Nothing in the plan performs a thread-existence check: R11's one bounded `GetConversationInfo` covers *channels* only; a bogus `thread_ts` is indistinguishable pre-TUI from a slow one. In the TUI, `ThreadRepliesLoadedMsg` failure returns nil Replies = "network failure, keep cached render" (`internal/ui/reducer_channels.go` analog at `internal/ui/reducer_threads.go:156-163`) — never "not found". Net: a valid-looking ts for a nonexistent/deleted thread lands a **stub-parent thread panel that never fills**, zoomed (per gotcha 1's width coin-flip), with no notice and no fallback to zoomed-channel.
- **Why missed**: R11 was written for channels; the thread resolution path was assumed to be symmetric.
- **Severity**: The user's stated fallback ("threads not found → lands as normal") cannot fire; silent dead pane instead.
- **Mitigation**: Either (a) define "thread not found" as permalink/parse/channel-resolution failure only and document that a bogus ts lands an empty thread panel, or (b) add one bounded `conversations.replies` existence probe at launch (R11-analog for threads) before the TUI starts.
- **Rank**: **4**

**3. Workspace routing (R9) vs first-connect-wins `InitialActive` — and the failed-pinned-workspace strand**
- **What**: `InitialActive` is decided by `default_workspace`, else `firstReady.Do` first-to-connect (`cmd/slk/main.go:1479-1491`). U1's seam is the WorkspaceReadyMsg *send site* (`main.go:1573-1590`), which only *labels* `isInitial`; it does not decide it. If a permalink's subdomain selects workspace B (R9) but workspace A connects first and no `default_workspace` is configured, A claims `InitialActive` and the pin never lands on B. The likely fix — force `defaultTeamID` to the pinned team for the run — has its own hole: with `defaultTeamID != ""`, non-matching workspaces **never** claim (`main.go:1480-1485`), so a pinned workspace that fails to connect (`WorkspaceFailedMsg`, `main.go:1468`) leaves **no active workspace at all**: empty sidebar, "no active workspace" from every service closure — the exact Grid-#5 failure mode described at `main.go:1455-1460` — not R2's "lands as normal".
- **Why missed**: R9 ("selects the matching configured workspace") was ruled without connecting it to the `isInitial` computation or the failed-connect path; `fallback-normal` only covers *resolution* failure, not connect failure of a resolved target.
- **Severity**: Multi-workspace users get either wrong-workspace landing or a dead session.
- **Mitigation**: The launch target must override the `defaultTeamID` input at `main.go:1479-1491` (not the send site), AND `WorkspaceFailedMsg` for the pinned team must re-arm the first-ready fallback (release the claim) so another workspace lands normally + notice.
- **Rank**: **4**

**4. Channel resolved via `GetConversationInfo` but absent from the sidebar list → lands on `Channels[0]` and zooms it (R3's wrong-pane ban directly reachable)**
- **What**: R11 lets raw-ID channels missing from the local list resolve via one `GetConversationInfo` (e.g. a public channel the user never joined). But U1's landing mechanism is the `LastChannelID` loop, which only finds channels present in `m.Channels` (`internal/ui/reducer_workspace.go:236-244`) — and U1 mandates "reducer logic unchanged", forbidding the fix. Resolution *succeeds* (cmd/slk), so per U4 the zoom fires "after resolution completes" — zooming whatever `Channels[0]` happened to be. That is exactly R3's "never zoom-on-wrong-pane", caused by the *success* path.
- **Why missed**: R11's resolution layer and U1's landing layer were designed independently; nobody checked a resolved channel can be unlistable.
- **Severity**: Silent wrong-channel zoomed landing, no fallback notice (resolution "succeeded").
- **Mitigation**: Either restrict launch resolution to the local list (drop `GetConversationInfo` for landing), or spec how an off-list resolved channel enters `m.Channels`/gets a `ChannelSelectedMsg` — acknowledging that changes reducer-adjacent behavior and updating U1's "reducer logic unchanged" claim.
- **Rank**: **4**

**5. Double `ChannelSelectedMsg` race if the thread target is armed via `OpenLinkMsg` — the duplicate select closes the just-opened thread**
- **What**: U2 never names the seam that arms `pendingLinkNav`. The tempting path is `OpenLinkMsg`→`routeLink`. `routeLink` queues its own `ChannelSelectedMsg` when `channelID != activeChannelID` (`internal/ui/reducer_links.go:94-102`); `reduceWorkspaceReady` independently queues one for the pinned channel (`internal/ui/reducer_workspace.go:249-251`) as a *returned cmd* (goroutine). The two race: if `OpenLinkMsg` is processed first (its `p.Send` from the connect goroutine can beat the batch-cmd result), the app gets **two** `ChannelSelectedMsg`s. The first opens the thread (`completePendingLinkNav` → `openThreadForPermalink`); the second runs `a.clearZoom()` + `a.CloseThread()` (`internal/ui/reducer_channels.go:331,356`), and its completion hook dies on the `p.threadOpened` guard (`internal/ui/reducer_links.go:125-129`) — thread closed at first paint, never reopened, nav silently dropped. Bonus: `routeLink`'s guards (`activeWorkspaceDomain()==""` → browser, `:81`; `Lookup` miss → browser, `:84-87`) mean arming before `WorkspaceReadyMsg` processes opens the **OS browser**.
- **Why missed**: "openThreadForPermalink semantics (R6)" was read as "reuse the pipeline" without specifying arming order relative to the InitialActive select.
- **Severity**: Race-dependent thread-closed-on-landing; only reproduces outside `updateAndRender` (real bubbletea cmd scheduling), so the planned probe won't catch it.
- **Mitigation**: Arm the launch thread target *synchronously inside* the InitialActive branch of `reduceWorkspaceReady` (new `WorkspaceReadyMsg` field), never via `OpenLinkMsg`; pin with a test driving both message orders.
- **Rank**: **3** (4 if the OpenLinkMsg route is chosen)

**6. "First content frame already zoomed" is unsatisfiable as a literal claim — spinner frames + async cmd gap**
- **What**: `reduceWorkspaceReady` renders loading frames before any select (`reducer_workspace.go:246-247`), and tier-3 cold channels (likely for a rarely-visited pinned channel) render more spinner frames until `MessagesLoadedMsg` (`reducer_channels.go:450-458`). Zoom entering "after resolution completes" therefore always follows ≥1 unzoomed content frame. Worse, even sequenced messages don't guarantee frame coalescing: a zoom message returned as a cmd runs on a goroutine and the renderer can paint between Updates — "enforced by message sequencing, not by sleeps" doesn't hold in production; the `updateAndRender` probe (synchronous) structurally cannot detect the flash.
- **Why missed**: U3 and U4 were treated as compatible; "content frame" never defined; bubbletea's async cmd/render model assumed sequential.
- **Severity**: One-to-several-frame unzoomed flash in the real app; probe green regardless.
- **Mitigation**: Define "first content frame" = first frame after message content lands; make zoom enter *synchronously inside the same reducer arm* that completes the landing (not a follow-up message), so no Update boundary intervenes.
- **Rank**: **3**

**7. Launch zoom is not covered by the user-navigated-first cancellation — zoom-on-wrong-pane after mid-flight navigation**
- **What**: U2 leans on the `pendingLinkNav` stale-guard for cancellation, but U3/U4's zoom has no equivalent: if the user switches channel between the landing and the zoom message (input is live — the bootstrap overlay dismisses as soon as *one* workspace is ready, `internal/ui/bootstrap.go:124-132`, while others still connect), the standalone zoom message enters zoom on the user's *new* channel. `reduceChannelSelected`'s `clearZoom` only runs on the select itself (`reducer_channels.go:331`); nothing cancels a pending launch-zoom.
- **Why missed**: Cancellation was specified for the thread nav only, not the zoom trigger.
- **Severity**: Direct R3 violation ("never zoom-on-wrong-pane") via a reachable race.
- **Mitigation**: Enter zoom only from within the same arm that completes the pending nav (so the stale-guard at `reducer_links.go:119-124` covers zoom too); add a probe: navigate away mid-landing → no zoom.
- **Rank**: **3**

**8. `enterZoom` snapshots an empty/loading pane — `exitZoom` then yanks the user to the top**
- **What**: `enterZoom` saves `YOffset`/`SelectedIndex` whenever zoom promotes messages (`reducer_zoom.go:156-160`); `exitZoom` restores them (`:200-202`). At launch, "the target lands" earliest at thread-panel-open time (stub parent, `internal/ui/app.go:2120-2148`) — which for tier-2/3 channels is **before** `MessagesLoadedMsg` fills the pane, and for select-message permalinks before `FetchAround` re-anchors the buffer (`reducer_links.go:144-151`). Zoom entering then snapshots offset 0/selection 0 of an empty pane; when the user later presses esc, the restore stamps 0 onto settled content — top-of-channel instead of the bottom. (Autoscroll interplay — `AppendMessage` always scrolls to newest, `internal/ui/messages/model.go:688-691` — makes the divergence larger, not smaller.)
- **Why missed**: "AFTER the target lands" was never mapped against the actual landing order: thread-open (sync, stub) → channel messages (async) → thread replies (async).
- **Severity**: Visible viewport jump on first esc after a launch-zoomed session.
- **Mitigation**: Define "target lands" as *content* landed (MessagesLoadedMsg / ThreadRepliesLoadedMsg applied), not panel-opened; or skip the viewport snapshot when the pane is loading/empty at enterZoom.
- **Rank**: **3**

**9. Probe gate `git diff | grep -c "a.zoomed *=" == 0` false-positives on context lines**
- **What**: `git diff` emits unchanged context lines (space-prefixed) around every hunk; `a.zoomed = true/false` appears at `reducer_zoom.go:161,197,239` and will match as *context* in any diff touching nearby lines. The gate as written fails on harmless diffs and passes nothing extra.
- **Mitigation**: `git diff -U0 | grep -c '^+.*a.zoomed *='`.
- **Rank**: **2**

**10. `routeLink`'s domain re-check diverges from R9's cmd/slk-level routing**
- **What**: If gotcha 5's `OpenLinkMsg` route is taken, `routeLink` independently re-checks `activeWorkspaceDomain()` (`reducer_links.go:80-83`): a workspace whose `Domain` is empty (subdomain unknown/custom domain) or a case-mismatched subdomain falls to the **OS browser** at launch even though cmd/slk already routed correctly per R9.
- **Mitigation**: Same as gotcha 5 — bypass `routeLink`; arm `pendingLinkNav` directly with the pre-resolved target.
- **Rank**: **2**

**11. `SLK_FULLSCREEN` truthiness undefined**
- **What**: `cli-env-equivalents` rules "empty/unset ENV is indistinguishable from absent" but never defines `SLK_FULLSCREEN=0`/`false`/`no` — by the item's letter they're non-empty ⇒ set ⇒ zoom.
- **Mitigation**: One ruling line: exactly `1`/`true` (folded) is truthy; anything else non-empty is a usage notice or false — pick one, pin it.
- **Rank**: **2**

**12. "Pin wins over LastChannelID" probe is vacuous if implemented by overwriting `LastChannelID` at the send site**
- **What**: U1 mandates "reducer logic unchanged", whose cheapest reading is `LastChannelID: pin` at `main.go:1589` — then pin vs LastChannelID is one field and the probe's precedence assertion cannot fail. Side effect worth deciding: after landing, `RecordVisit` (`reducer_channels.go:371`) rewrites the persisted last-visited to the pinned channel, so the *next* unpinned launch lands on the pin, not the user's prior channel.
- **Mitigation**: Add a distinct `PinChannelID` field so precedence is observable; accept or suppress the `RecordVisit` side effect explicitly.
- **Rank**: **2**

Zero-gotcha check: none — 12 found (4×rank 4, 4×rank 3, 4×rank 2).

**Files grepped/read**: `flow/plans/2026-10-01-launch-pin-zoom.md`; `internal/ui/reducer_zoom.go`; `internal/ui/reducer_links.go`; `internal/ui/reducer_workspace.go`; `internal/ui/msgs.go` (:360-430); `internal/ui/reducer_channels.go`; `internal/ui/reducer_threads.go`; `internal/ui/app.go` (threadInFront/zoomFrontIsThread/layoutThreadFront :906-1029, WindowSizeMsg :1106-1116, openThreadPanel :2120-2148, CloseThread :2332-2353, thread-mark plumbing :4030-4050); `internal/ui/panellayout.go` (:100-199); `internal/ui/bootstrap.go`; `internal/ui/view_early.go`; `internal/ui/messages/model.go` (AppendMessage :668-692); `cmd/slk/main.go` (:1390-1628); greps across `internal/ui` for `WorkspaceReadyMsg|InitialActive|LastChannelID|pendingLinkNav|ThreadRepliesLoadedMsg|normalizeZoomFocus|clearZoom|FetchAround|autoscroll|MsgWidth|IsLoading`.