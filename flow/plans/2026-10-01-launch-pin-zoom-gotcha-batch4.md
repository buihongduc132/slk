# Gotcha-coverage — BATCH 4 (fallback + user-visible DOD) — RERUN

> Repo: /home/bhd/Documents/Projects/bhd/slk (read-only) · Plan: flow/plans/2026-10-01-launch-pin-zoom.md
> Note: first spawn of this batch died emitting tool-call markup as narration (no deliverable); re-spawned with explicit anti-narration instruction. This is the re-run's deliverable.
> Files actually read: flow/plans/2026-10-01-launch-pin-zoom.md; internal/ui/reducer_new_message.go; internal/ui/reducer_workspace_lastchannel_test.go; internal/ui/reducer_zoom.go; internal/ui/view_status.go; internal/ui/statusbar/model.go; internal/ui/reducer_io.go; internal/ui/app.go (zoom/layout/View/composite/toast sections); internal/ui/reducer_links.go; internal/ui/reducer_channels.go (clearZoom); internal/ui/reducer_workspace.go (InitialActive/LastChannelID); internal/ui/msgs.go; internal/ui/bootstrap.go; internal/slackurl/slackurl.go; cmd/slk/main.go (argv switch, workspace-connect/ready-send). Plus ripgrep surveys of toast/zoom/thread usage across internal/ui.

## Rank 4

**1. Zoomed pane for a pinned *thread* is terminal-width-dependent — DOD-1's "thread open, pane already zoomed" hides the thread on wide terminals**
- What: `App.enterZoom` promotes whichever pane `zoomFrontIsThread()` picks, and that probes the **unzoomed** layout (`app.go:1008`, `threadDrawnAloneAt(a.threadInFront(), false)`). At side-by-side widths the answer is "messages pane" — B48-pinned intended behavior ("z zooms the messages pane even when the thread is focused", closed in favor of the code, `zoom_front_pane_test.go`). So `slk --channel C --thread T --fullscreen` on a 200-col terminal enters zoom with the **messages** pane promoted: `threadVisible=true` but not drawn, focus normalized off `PanelThread`. The user pinned a thread and the first frame is the channel; the thread is invisible until they press `z` — exactly the hotkey the requirement exists to remove. On a narrow terminal the same command zooms the thread. Landing varies with terminal width.
- Why missed: The plan treats zoom as a boolean (`land-zoomed`: "first frame already zoomed, via enterZoom"). R4 pins *how* to enter zoom, never *which pane* `--fullscreen` should promote when the target is a thread. R3 only rules failure cases.
- Severity: significant — user-visible intent violation in the headline scenario.
- Mitigation: Decide explicitly: for a thread target, `--fullscreen` should zoom the thread pane (requires focusing the thread / a thread-front entry path through the existing predicates, not new zoom state). Pin with a width-tabled test (narrow + wide) asserting which pane the first zoomed frame draws — `assertZoomedFrame` + `assertFront` cover this today.
- Rank: 4

**2. "Thread not found" is undetectable at resolution time for a valid channel — nonexistent/deleted TS lands as an open empty stub thread, violating the user's fallback line**
- What: R11's resolution verifies the *channel* (local list + one `GetConversationInfo`); nothing verifies the thread TS exists. `openThreadForPermalink` (`reducer_links.go:162`) opens the panel with a minimal stub parent and relies on `ThreadRepliesLoadedMsg` to backfill. For a wrong/deleted TS the fetch returns empty/failed and the user stays on an empty thread with no notice and no fallback — but the user verbatim said "Fallback if threads not found: then just lands as normal." U5's "not found / no access" is resolved as channel-level only; thread-level "not found" after landing is in no DOD.
- Why missed: The plan models resolution as pre-launch, but thread existence is only knowable post-landing (post-fetch). No item owns the runtime-empty-thread case at launch.
- Severity: significant — the fallback promise silently fails for the most common thread typo.
- Mitigation: Add a launch-scoped guard: if the initial thread fetch lands empty/failed for a *pinned* target (not a user-navigated one), close the thread, fire the R2 notice, and continue on the channel (zoomed per R3). Pin with a `newTestApp` test driving `ThreadRepliesLoadedMsg{Replies: nil}` for the pinned nav.
- Rank: 4

## Rank 3

**3. `statusbarText(a)` is the wrong oracle for the notice in every zoomed variant (R3-partial, DOD-5) — the B45 trap replayed**
- What: The fallback-normal probe asserts the notice via `statusbarText(a)`. That renders the statusbar model in isolation; while zoomed, `View()` composites no status row and the toast only reaches pixels via `overlayZoomToast` painted on the pane's bottom border row (`view_status.go:61`, `app.go:3553-3556`). For R3's partial case (channel ok → **zoomed** + notice) a `statusbarText` assertion passes while the notice is invisible — the exact failure B45 documented and fixed.
- Why missed: The plan reuses the last-channel test's oracle for a feature whose failure mode is zoom-adjacent.
- Mitigation: For zoomed notice tests, assert on the composed frame (last row contains the notice), reusing the `zoom_toast_visible_test.go` pattern; keep `statusbarText` only for the unzoomed fallback case.
- Rank: 3

**4. R3's partial resolution (channel resolves, thread doesn't) specifies no notice**
- What: R3 says "channel resolved but thread not → zoom the channel". U1/U5's notice is attached only to "unresolvable target". Partial success is neither: the user asked for a thread, gets a zoomed channel, silently. Is that "lands as normal" per the user's fallback line? The plan ruled the *behavior* (R3) but never ruled the *notice* for it.
- Mitigation: One clause: partial resolution also fires the naming notice ("thread not found: <ts>"). Fold into the same single ToastMsg as other failures.
- Rank: 3

**5. ENV whitespace-only values and `SLK_FULLSCREEN` value forms unspecified**
- What: cli-env-equivalents pins "empty/unset indistinguishable from absent" but not whitespace (`SLK_CHANNEL=" "` from a shell profile → name lookup fails every launch → spurious fallback notice forever). Likewise `SLK_FULLSCREEN=0` / `false` / `yes`: if implemented as `!= ""`, `0` zooms.
- Mitigation: TrimSpace-then-empty = unset; define the truthy set (`1|true|yes`, case-folded) and treat everything else as unset (or warn). Pin both in the DOD-2 precedence table.
- Rank: 3

**6. Empty-string flag value (`--channel ""`) — set-but-empty wins precedence with a garbage target**
- What: `flag` happily accepts `--channel=` / `--channel ""`. By "flags > ENV > state" that's a *set* flag whose target is empty: resolution fails on `""` and toasts `not found: `. The empty-ENV rule has no flag twin.
- Mitigation: Same rule as ENV: empty-after-trim flag = unset, falls through to ENV/state. Add a row to the cli-frames-parse table.
- Rank: 3

**7. R10's exit-2 usage error, extended to ENV by "equivalents", bricks the TUI for persistent exports**
- What: DOD-2 makes `SLK_*` behave as equivalents; the natural reading applies R10 (`--thread` bare-ts without channel → exit 2) to `SLK_THREAD` bare-ts without `SLK_CHANNEL`. A stray exported pair in `.profile` then makes **every** launch die before the TUI — far more hostile than a mistyped flag, which is a one-off.
- Mitigation: Asymmetry is warranted: flag-form errors → exit 2; ENV-form errors → ignore the malformed var + launch-normal notice (exit 0). State it in DOD-2's test.
- Rank: 3

**8. `SLK_FULLSCREEN` with *no* target at all → R3's wording produces notice spam**
- What: R3: "--fullscreen without a resolvable target … neither resolved → ignore + notice." A user who permanently exports `SLK_FULLSCREEN=1` and launches bare `slk` has no target — by that wording, a "fullscreen ignored" toast on every single launch. R3 conflates "target given and failed" with "no target given".
- Mitigation: No target + fullscreen = silent ignore (or zoom the last-visited channel — decide); notice only when a target was supplied and failed. Add the no-target row to DOD-5's table.
- Rank: 3

**9. Multiple simultaneous failures emit multiple ToastMsgs into a single-slot toast — last write wins**
- What: The toast is one string (`statusbar.SetToast` replaces; `reducer_io.go:141` handles each `ToastMsg` independently). Permalink with unmatched subdomain can fail R9 *and* R2 *and* the thread: up to three notices, user sees one. U1 says "one … notice" but no composition rule exists.
- Mitigation: Compose failures at the send site into one notice (ordered: workspace, channel, thread), or define precedence. Test: all-three-fail → exactly one toast naming all.
- Rank: composition rule + test.

**10. Notice timing vs the bootstrap loading overlay and the 3 s auto-clear**
- What: `ToastMsg` → `uploadToastCmd(…, 3*time.Second, toastEager)` — the clear tick starts when Update processes it. The R9 subdomain-mismatch decision needs only config and can fire while workspaces are still connecting; the bootstrap overlay draws **over** both the status row and `overlayZoomToast` (`applyOverlays` runs after it, `app.go:3557`). Notice painted under the overlay, cleared 3 s later, never seen. Same risk for any notice sent before first paint on slow terminals.
- Mitigation: Fire launch notices only after the first `WorkspaceReadyMsg` lands (resolution needs the channel list anyway, except R9 — defer that one too), and give launch notices a longer/sticky window (e.g. 8–10 s or until first keystroke).
- Rank: 3

**11. Channel resolved via the R11 `GetConversationInfo` path but absent from the sidebar/channels service — no hydration item**
- What: R11 lets raw-ID channels missing from the local list resolve via one API call. But `land-channel` only swaps the `WorkspaceReadyMsg` target; everything downstream (`retargetChannel` → `channels.Lookup` for the statusbar name, `sidebar.SelectByID`, history fetch) keys off the sidebar list. A resolvable-but-unlisted channel lands with a bare `#` statusbar, no sidebar selection — the same class of gap `hydrateNewConversation` (`reducer_new_message.go:90`) exists to patch for new DMs.
- # Mitigation: Add an item: resolved-but-unlisted channel is hydrated via `sidebar.UpsertItem` (+ channels service entry) before `ChannelSelectedMsg` fires, mirroring the DM path.
- Rank: 3

## Rank 2

**12. Permalink-to-a-reply without `thread_ts` silently no-ops** — DOD-3's "select-message" form assumes `MessageTS` ∈ channel history; a reply ts isn't. `SelectByTS` misses, `FetchAround` loads a window around a ts that exists only inside the thread → user lands nowhere near it, no notice. Mitigation: detect reply-ness is impossible pre-fetch; at minimum document, ideally treat fetch-around-miss as the gotcha-2 runtime fallback. Rank: 2

**13. Malformed `--thread` value with `--channel` present is validated nowhere** — R10 covers only missing channel; `--thread notats` becomes a stub-thread open with a garbage TS (falls into gotcha-2's empty-thread landing). A digits`.`digits check in the pure resolver is free. Rank: 2

**14. `Channels[0]` fallback-of-fallback undefined when the list is empty** — `reduceWorkspace` guards `len(m.Channels) > 0` (`reducer_workspace.go:232`); empty list → no channel selected at all, notice or not. Plan says "else Channels[0]" without the empty case. Rank: 2

**15. Notice text can be truncated into uselessness and is unsanitized** — the toast slot truncates (`overlayZoomToast` → `ansi.Truncate(toast, a.width, "…")`; statusbar `View` elides), so "naming the failed target" with a full permalink may render as `…`. ENV-sourced text can carry control characters. Mitigation: echo a short form (name or channel ID), sanitize. Rank: 2

**16. DOD-2's third precedence tier ("config/state") exists only for the channel var** — there is no persisted thread or zoom state to outrank; the precedence test can only meaningfully pin `SLK_CHANNEL` vs `LastChannelID`. Scope the DOD wording or the test will assert a phantom. Rank: 2
