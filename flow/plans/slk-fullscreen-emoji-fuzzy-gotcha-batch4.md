# Gotcha Coverage — slk-fullscreen-emoji-fuzzy (batch 4: zoom layout + suppression)

> Source: `flow/plans/slk-fullscreen-emoji-fuzzy.md`
> Mode: plan (same structure note as batch 3)
> Sub-agent: batch 4 of 4, delegated per `cmd-pallet/gotcha-coverage`
> Units reviewed: fs-statusrow-math, fs-layout, fs-supp-set, fs-scope,
> fs-zoom-cache-keys
> Numbering: B45–B55, continuing from batch 3's B44
> Appendix only. The plan's items are unmodified.

## Provenance

Same dispatch and same recovery as batch 3 — see that appendix for why three
apparent losses were actually delivery failures rather than dead agents, and for
the `jq` extraction that recovers a finished transcript without flooding the
parent's context. This agent used 6 of its 8 tool calls.

**Every finding below was re-verified by me against the tree.** This batch's two
Rank 5 findings are the highest-value results of the entire four-batch pass: both
are live defects in merged, deployed code, and both are the same class as B13 —
a unit marked `[x]` whose oracle observes a subject the defect cannot reach.

## Findings (ranked)

### Rank 5 (Sophisticated — fundamental gap, invalidates or limits the unit)

- **B45 — the suppression toast is written to a row that zoom does not draw, so
  the entire suppression UX is invisible, and its oracle cannot see that**
  - Unit: fs-supp-set (fs-layout is the cause)
  - What: `reduceZoom` reports suppression via
    `a.uploadToastCmd(zoomSuppressedToast, zoomToastDuration, toastEager)`
    (`reducer_zoom.go:167`), which resolves to `a.statusbar.SetToast(text)`
    (`app.go:4198-4200`). But fs-layout's whole point is that the status row is
    not drawn while zoomed, and I confirmed the exact mechanism the delegate
    inferred: `app.go:3384-3386` reads `status := ""; if !a.zoomed { status =
    a.renderStatusRow(…) }`, so `renderStatusRow` is **never called** while
    zoomed. `Compute` independently sets `statusHeight = 0`
    (`panellayout.go:81-83`). So pressing `ctrl+b`, `ctrl+]`, `ctrl+t`, `:`, `/`
    or a workspace digit while zoomed produces **no action and no feedback at
    all**.
  - Why missed: the author optimised for the test's read path, not the user's.
    The comment at `reducer_zoom.go:43-44` shows the oracle reads the statusbar
    **model** ("the suppression rows assert on that substring"), and per
    AGENTS.md the only observer available is `statusbarText(a)`, a model getter.
    The assertion's subject is a field that is set unconditionally — it cannot
    vary with whether the row is composited. That is the vacuous shape exactly:
    proof by a subject the defect cannot reach. Note batch 3 independently looked
    at these same tests and judged them "over-covered, the harmless direction";
    batch 4 is right and batch 3 is wrong, which is itself the argument for
    running overlapping batches.
  - Severity: every suppressed key is a silent no-op. `ctrl+b` while zoomed is
    indistinguishable from a dropped keystroke or a hung app. All 10+ suppressed
    keys, always, every terminal. This ships today.
  - Mitigation: while zoomed, either render the toast as a one-row overlay on the
    zoomed pane's last row, or reserve the status row while a toast is live, or
    drop the toast and have those keys exit zoom then act (which fs-scope already
    offers as its alternative). Whichever is chosen, **re-point the oracle at the
    rendered frame** — a substring assertion on `View()` output, or a golden —
    not at `statusbarText`.
  - **FIXED.** `App.overlayZoomToast` (`view_status.go`) paints a live toast onto
    the last row of an already-composed zoomed frame; `statusbar.Model` gained a
    `Toast()` getter so the App can read back what it set.
    - The **last row** rather than a reserved row, for the reason B46's severity
      correction established by measurement: that row holds the pane's bottom
      *border*, not content, so overwriting it costs no content and no reflow.
      Reserving a row would shrink `ContentHeight` and make the zoomed frame no
      longer full-height, which fs-statusrow-math and its golden both pin.
    - Applied BEFORE `applyOverlays` so a modal still draws over it.
    - **The memo was the trap, and it is the half a naive fix would miss.** The
      screen memo is keyed on `(panels, status, w, h)` and `status` is always `""`
      while zoomed — so a toast appearing changed no memo input and the stale
      pre-toast frame would be served, making the fix invisible in exactly the
      case it exists for. `View()` now keys on the toast text while zoomed.
      `TestZoom_ToastDefeatsTheScreenMemoWhileZoomed` was RED for this reason
      alone, with the overlay already written.
    - Pinned by `internal/ui/zoom_toast_visible_test.go`: the toast is visible in
      a zoomed frame, a suppressed key changes the frame bytes, and exiting zoom
      restores the real status row. All assert on `stripANSI(a.View().Content)` —
      the channel the user observes — never on `statusbarText`.
    - Note on the control leg, which was wrong in its first draft: it cannot press
      `ctrl+b` unzoomed to establish "a toast can render", because unzoomed
      `ctrl+b` is not suppressed at all, it toggles the sidebar. The suppression
      path exists *only* while zoomed, which is precisely why its only oracle was
      a model getter. The control now sets the toast directly and renders
      unzoomed.
  - Evidence: `internal/ui/reducer_zoom.go:42-45,161-167`;
    `internal/ui/app.go:3384-3386,4198-4200`; `internal/ui/panellayout.go:81-83`.
    All re-verified by me; the `app.go:3384` conditional is the link the delegate
    marked unverified.

- **B46 — the height override reaches `Compute` and nothing else: the zoomed
  pane's last row is drawn but not clickable** *(mandated class — one dimension,
  three implementations)*
  - Unit: fs-statusrow-math (this is the unit's central claim, and it is false)
  - What: one rule, three independent implementations, two of them not
    zoom-aware:
    - `panelLayout.Compute` — zoom-aware: `statusHeight := 1; if zoomed {
      statusHeight = 0 }` (`panellayout.go:81-87`)
    - `panelLayout.PanelAt` — takes **no `zoomed` parameter at all**, and does
      `if y >= height-1 { return PanelWorkspace, 0, 0, false // status bar }`
      (`panellayout.go:157-159`). Its sole call site passes no zoom state:
      `a.layout.PanelAt(x, y, a.height, a.sidebarVisible, a.threadVisible)`
      (`app.go:1829`)
    - `reduceMouseClick` — declares its own `statusHeight := 1` and returns early
      on `m.Y >= a.height-statusHeight` (`reducer_mouse.go:192-193`)
    Because `ContentHeight = height - 0` while zoomed, the pane genuinely draws
    on the last terminal row — which the unit's golden confirms — and both
    hit-test paths classify that row as status bar and discard the event.
  - Why missed: the unit enumerates its consumers correctly ("`Compute` AND
    `PanelAt` AND every parallel mouse router") but the proof is a golden frame,
    and **a golden cannot fail on a hit-test**. The render half was proven; the
    routing half was assumed. The unit's text is therefore not wrong about what
    needed doing — only about what was done.
  - Severity as reported by the delegate, and as I first repeated it: "one full
    row of content is mouse-dead whenever zoomed — clicks, drag-selection
    anchors, reaction hit-testing — and it is the row the eye lands on."
  - **SEVERITY CORRECTION — that claim is false, and I verified it false by
    rendering the frame rather than reasoning about it.** At height 30 the
    measured geometry is:

    ```
    unzoomed  rows 0..28 = pane (border at 0 and 28), row 29 = status row
    zoomed    rows 0..29 = pane (border at 0 and 29), no status row
    ```

    So while zoomed the final terminal row holds the pane's bottom **border**
    (`╰─────`), not content; content ends at row 28 in both states. The row
    `PanelAt` was rejecting was never clickable content, clicking a border
    correctly selects nothing, and **no content row was ever unreachable**. I
    wrote a test asserting a click on row 29 must move the selection; it stayed
    red after the fix, which is what sent me to measure. There is no
    user-visible defect here.
  - Severity, corrected: **latent, not live.** The mechanism below is real — one
    rule, three implementations, two of them unaware of zoom — and it is a trap
    that fires the moment the pane's border or the status height changes, because
    two of the three readers would not follow. But nothing is broken for a user
    today, and this is not Rank 5 on impact. It is recorded at Rank 5 for the
    *class* (the mandated duplicate-rule shape, which AGENTS.md ranks as a real
    defect regardless of current blast radius) with the impact claim withdrawn.
  - Mitigation: **delete the two literals.** Have `Compute` store the fact on
    `panelLayout` beside the bands, and have both readers consult it. Do not add
    a `zoomed` parameter to `PanelAt` that callers can forget — that reproduces
    the class one level down.
  - **FIXED.** `panelLayout` gained a `zoomed bool` plus one derivation,
    `statusRows()`; `Compute`, `PanelAt` and `reduceMouseClick` all read it and
    both literals are gone.
    - A `bool` rather than a stored `statusHeight int`, for a reason worth
      keeping: tests seed the bands directly without calling `Compute`
      (`app_panelat_test.go:37-40`), so a zero-valued height field would have
      read as "reserve nothing" = "zoomed" and silently changed unrelated tests.
      The zero value has to be the safe one.
    - Pinned by `internal/ui/zoom_lastrow_hittest_test.go`, in its own file
      because `fullscreen_red_test.go` is a pinned gate oracle (OT17). Four
      tests: last row hit-tests to a pane while zoomed; `Compute` and `PanelAt`
      agree at both zoom states; the last *content* row is clickable while
      zoomed; and a **structural** assertion that `reducer_mouse.go` consults
      `a.layout.statusRows()` and declares no `statusHeight := 1`.
    - The structural oracle exists because no behavioural test can catch that
      third implementation: the only row the two answers disagree about is the
      border, where a click correctly selects nothing either way. A behavioural
      assertion there would be vacuous by construction — so the honest instrument
      for "one source of truth" is to read the source, the same shape as
      `cmd/slk/startup_order_test.go` (B18). RED-proven both directions:
      reintroducing the literal fires both of its assertions.
  - Evidence: `internal/ui/panellayout.go:80-87,157-159`; `internal/ui/app.go:1829`;
    `internal/ui/reducer_mouse.go:192-193`. All re-verified by me, including the
    absence of `zoomed` from the `PanelAt` signature.

### Rank 4 (Significant)

- **B47 — `:sp/:vsp/:only/:q` are not suppressed; only the `:` that reaches them
  is, so "one rule, both surfaces" is vacuously true**
  - Unit: fs-supp-set
  - What: `internal/ui/command.go` contains **zero** references to `zoomed` or
    `fullscreen` (verified: `grep -c` → 0). `reduceZoom` returns early unless
    `a.zoomed && a.mode == ModeNormal` (`reducer_zoom.go:157`), so once in command
    mode nothing is suppressed. The single surface actually covered is the `:`
    keypress in normal mode. Any other route into `ModeCommand` while zoomed — a
    future binding, a mode restored by a reducer, a synthesised `:` — executes
    split/only/close-window against a zoomed layout unguarded.
  - Compounding it: `windowBounds` calls `Compute` with a **hardcoded
    `threadFront=false`** and the live `a.zoomed` (`windows.go:53-55`), so it
    lands in the zoomed branch and returns `H = frame.ContentHeight` (full
    terminal height, no status row) with full-width `MsgWidth` and
    `railWidth=0`. A split created while zoomed is laid out against the zoomed
    rectangle and will not match the unzoomed layout it reappears in.
  - Why missed: blocking the door is indistinguishable from guarding the room
    while there is exactly one door. The unit lists command mode as covered
    because the observable behaviour today is right.
  - Severity: latent for the keys, breaks on the next mapping or mode-entry path
    added; the `windowBounds`-while-zoomed geometry is reachable **today** by any
    non-key caller of the window ops.
  - Mitigation: guard in `command.go` where the verbs execute — one check at the
    dispatch point, not per verb. And make `windowBounds` either refuse while
    zoomed or pass `zoomed=false`, so the rectangle describes the layout the
    window will actually live in.
  - Evidence: `internal/ui/reducer_zoom.go:157`; `internal/ui/windows.go:52-56`;
    `internal/ui/command.go` zoom-reference count 0. Re-verified.

- **B48 — `z` zooms the messages pane even when the thread is focused, at any
  width where both panes fit: fs-layout's and fs-scope's stated `threadFront`
  rule is not what ships**
  - Unit: fs-layout, fs-scope
  - What: the units state `threadFront=(stackFront==PanelThread||
    focusedPanel==PanelThread)` and "zooms the FRONT pane whatever it is
    (`stackFront`)". The code consults **neither**: `layoutThreadFront()` →
    `zoomFrontIsThread()` → `threadDrawnAloneAt(false)`, which returns
    `frame.MsgWidth == 0` in the *unzoomed* layout (`app.go:921-955`). At any
    width where messages and thread fit side by side, `MsgWidth > 0`, so zoom
    promotes MESSAGES regardless of focus or `stackFront`.
  - **My correction to the delegate**, which changes what to do about it: the
    delegate called the code's behaviour a defect against the units. It is not —
    `zoomFrontIsThread`'s doc comment (`app.go:931-945`) states the intent
    explicitly: *"Probing the unzoomed layout (rather than reading focusedPanel)
    is also what keeps Tab from flipping WHICH pane is zoomed (G15)."* So the
    code is deliberate and G15-motivated, and the **plan text is stale**. The
    defect is a documentation/spec divergence, not a logic error — which is why
    the mitigation is "make them agree", not "fix the code".
  - Severity: on a wide terminal a user reading a thread presses `z` and gets a
    fullscreen of the channel they were not looking at. Whether that is wrong
    depends on which contract holds — and nothing currently decides. fs-scope's
    "zooms the FRONT pane whatever it is" has no implementation for the
    side-by-side case.
  - Mitigation: decide, then make text and code agree. If G15's behaviour is
    right, rewrite both units to "zoom promotes the thread only when the unzoomed
    layout would have stacked". If the units are right, G15's Tab-stability
    problem needs a different fix — latch the zoomed pane at `enterZoom` instead
    of recomputing it per frame.
  - Evidence: `internal/ui/app.go:913-965` (read in full, including the G15 doc
    comment); `internal/ui/panellayout.go:105-107`.

### Rank 3 (Moderate)

- **B49 — `exitZoom` restores the messages viewport unconditionally, discarding
  movement that happened while zoomed**
  - Unit: fs-scope / fs-zoom-invariant
  - What: `a.messagepane.SetViewport(a.zoomSavedYOffset, a.zoomSavedSelectedIndex)`
    runs on every user exit (`reducer_zoom.go:79`) with no check on **which** pane
    was zoomed and no check on whether the pane moved for a legitimate reason.
    The justifying assumption — the pane still holds the content it held at
    `enterZoom` — is invalid: zoom does not stop ingestion, `cmd/slk` keeps
    pushing messages in, and any autoscroll, `G`, or search-jump during zoom is
    silently reverted on `z`/`esc`. When the *thread* is the zoomed pane it also
    stamps a stale offset onto a messages pane the user never touched.
  - Why missed: the `clearZoom`/`exitZoom` split was designed around content
    *replacement* (channel jump). Content *append* to the same channel was not
    considered — it does not invalidate the saved index, so it never tripped the
    reasoning that produced the split. The fix that introduced `clearZoom` (B13)
    is correct; this is the case neither branch covers.
  - Severity: a user who zooms, reads to the bottom of a busy channel, and exits
    is thrown back minutes. Silent.
  - Mitigation: restore only when the zoomed pane **is** the messages pane, and
    skip the restore if the pane's item count or bottom-anchored state changed
    since `enterZoom` — capture a cheap content-generation counter alongside the
    offset. Note this interacts with **B15**: the thread pane has no viewport
    accessor, so "which pane was zoomed" cannot currently be answered
    symmetrically.
  - Evidence: `internal/ui/reducer_zoom.go:50-61,72-80` — re-verified, including
    that the restore line has no surrounding condition.

- **B50 — the zoom cache-key bit and `invalidateZoomCaches` are two mechanisms
  for one rule; one is dead, and which one decides whether fs-zoom-cache-keys'
  second clause is even true** *(mandated class)*
  - Unit: fs-zoom-cache-keys
  - What: `invalidateZoomCaches` drops every win-model cache, the thread panel
    cache and `lastScreenValid` on **every** zoom transition
    (`reducer_zoom.go:108-112`) — and the zoomed bit is *also* baked into the
    layout keys (`view_messages.go:89`: `boolToInt(a.zoomed)<<3`;
    `view_thread.go:43`: `boolToInt(a.zoomed)<<2`). These cannot both be
    load-bearing. If the blanket invalidation is correct, no entry survives a
    transition, so the key bit can never discriminate and "no stale non-zoomed
    composite" is unfalsifiable by it. If the key bit is load-bearing, the blanket
    invalidation contradicts "the zoomed pane's own render stays cached at zoom
    width" across a toggle.
  - Why missed: belt-and-braces. Each satisfies a different clause of the same
    unit and neither author checked the other.
  - Severity: correctness is fine today — over-invalidation is safe. The defect is
    that the unit's proof is anchored to whichever half is dead, so a later
    "optimisation" removing the invalidation looks safe because the key bit
    appears to cover it. Note the screen memo (`lastScreenValid`, keyed on panel
    strings per G16) has **no** zoom bit at all, so it is the one piece the keys
    genuinely cannot express.
  - Mitigation: **delete one.** Keep the key bits and narrow
    `invalidateZoomCaches` to only what the keys cannot express (the screen memo),
    or keep the blanket invalidation and remove the bits. Then write a test that
    fails if the surviving mechanism is removed — otherwise the same ambiguity
    returns.
  - Evidence: `internal/ui/reducer_zoom.go:103-112`;
    `internal/ui/view_messages.go:89`; `internal/ui/view_thread.go:43`.
    Re-verified.

- **B51 — no evidence the zoom height override reaches sixel/kitty placement at
  all**
  - Unit: fs-statusrow-math (which names this consumer explicitly)
  - What: `internal/ui/sixelpaint.go` contains **zero** matches for `zoom`,
    `statusH`, `height - 1` or `height-1` (verified by count). Given that
    `PanelAt` and `reduceMouseClick` both turned out to carry their own
    un-zoom-aware status row (B46), the unit's claim that the override reaches
    "sixel/kitty placement bounds" is unsupported. Placement may derive from pane
    geometry that *is* zoom-aware, in which case it is fine — but nothing
    demonstrates it, and the failure mode (an image placed one row off, or clipped
    at the old content height) is exactly what a status-row change causes.
  - Severity: images in a zoomed pane potentially misplaced or clipped by one
    row; terminal-dependent, so it may not reproduce locally. **Unconfirmed as a
    defect** — this is an absence of evidence for a claim the unit makes, which is
    why it is Rank 3 rather than higher.
  - Mitigation: trace `sixelpaint`'s row origin to whichever value `Compute` ends
    up owning after B46, then add one assertion that the placement origin shifts
    by exactly one row across a zoom toggle. If it already derives from pane
    geometry, that assertion documents it and costs nothing.
  - Evidence: `grep -c 'zoom\|statusH\|height - 1\|height-1'
    internal/ui/sixelpaint.go` → 0. Mechanism **unverified** — neither the
    delegate nor I read the file.

### Rank 2 (Minor)

- **B52 — `enterZoom` is the only one of the three transitions with no guard**
  - Unit: fs-zoom-invariant
  - What: `exitZoom:73` and `clearZoom:95` both open with `if !a.zoomed {
    return }`. `enterZoom:52` has no guard and delegates it to a comment
    ("Callers must already have checked `!a.zoomed`"). A second `enterZoom`
    overwrites `zoomSavedYOffset` with the already-scrolled offset, destroying the
    restore point permanently.
  - Severity: none today — one call site, immediately adjacent to the `if
    a.zoomed` branch that protects it. A silent loss of restore state the first
    time a second entry path is added.
  - Mitigation: `if a.zoomed { return }` at the top, matching its siblings. This
    also deletes a comment-standing-in-for-a-check, which AGENTS.md explicitly
    prohibits ("when you find a comment standing in for a check, replace it with
    the check").
  - Evidence: `internal/ui/reducer_zoom.go:52,72-73,94-95` — re-verified; the
    asymmetry is exactly as described.

- **B53 — bare `1`-`9` are swallowed wholesale in normal mode while `0` is not**
  - Unit: fs-supp-set
  - What: `reducer_zoom.go:132-137` suppresses any single-rune `1`-`9` and
    `alt+1`-`alt+9` before the mode table sees it; `0` is excluded. The rule is
    expressed in the comment as "the workspace numbers" but implemented as "any
    digit 1-9", which is a superset. If normal mode has or gains numeric count
    prefixes, zoom changes their meaning asymmetrically — and the digits are
    consumed with an **invisible** toast (B45), so the user gets no signal at all.
  - Severity: depends on whether normal mode uses counts — **unverified**. The
    interaction with B45 is what makes it worth recording: a wrong suppression is
    survivable when it says so, and confusing when it is silent.
  - Mitigation: match against the actual workspace-switch bindings rather than a
    character-class test.
  - Evidence: `internal/ui/reducer_zoom.go:132-137` — re-verified.

### Rank 1 (YAGNI)

- **B54 — `a.keys.WorkspaceFinder` in the `key.Matches` list is a no-op that
  reads as coverage**
  - Unit: fs-supp-set
  - What: the entry is annotated "keyless; kept for help parity"
    (`reducer_zoom.go:128`). A keyless binding matches nothing, so the line
    contributes zero suppression while making the set look one item longer.
  - Severity: none functionally; it inflates the apparent suppression set during
    review, which is how B47 ("both surfaces covered") became believable.
  - Mitigation: drop it, or move help-parity concerns to the help table.
  - Evidence: `internal/ui/reducer_zoom.go:128` — re-verified.

- **B55 — the suppression comment names a helper that no longer exists** *(my
  finding, not the delegate's)*
  - Unit: fs-supp-set
  - What: `reducer_zoom.go:161-163` reads "Suppression. toastWithClear, NOT
    uploadToastCmd: the toast must be … uploadToastCmd hides its …" — but :167
    calls `a.uploadToastCmd(…, toastEager)`, and `toastWithClear` no longer
    exists: the lane-toast consolidation (OT5 / B2) deleted it and preserved its
    body as `toastEager`. So the comment now argues *against* the call directly
    beneath it.
  - Severity: none functionally. It matters because this comment is the reasoning
    a future reader will use when touching the very line B45 says must change, and
    it currently points at a deleted API.
  - Mitigation: rewrite to name `toastEager` and the reason (eagerness, so the
    statusbar is readable straight after `Update` without running the returned
    cmd). Fold into whichever commit fixes B45.
  - Evidence: `internal/ui/reducer_zoom.go:161-167`; `toastWithClear` absent from
    the tree.

## Duplicate-rule / duplicate-value class

Checked: the status-row height rule, the front-pane predicate, the
cache-invalidation rule, the `Compute` argument assembly, and `os.Getenv` in
`internal/ui`.

**Found — status-row height, implemented three times, two not zoom-aware.**
B46. This is the canonical shape the brief predicted: one dimension that must
reach several consumers, implemented once per consumer, now drifted. Resolution:
delete the two literals; store the value on `panelLayout` where `Compute` already
computes it.

**Found — one cache-correctness rule expressed twice.** B50. Resolution: delete
one.

**Examined and NOT ranked — the "which pane is front" question has three
spellings** with different answers: `threadInFront()`, `threadDrawnAlone()` /
`threadDrawnAloneAt(zoomed)`, and `zoomFrontIsThread()`, funnelled through a
fourth, `layoutThreadFront()` (`app.go:913-960`). Each asks a distinct,
documented question and `layoutThreadFront` is the single funnel every `Compute`
call site is meant to use, so this is the *good* version of the pattern —
`AGENTS.md`'s "extract the substrate, not the widget". The defect adjacent to it
is that `windowBounds` bypasses the funnel with a literal `false`
(`windows.go:55`), folded into B47.

Not found in `internal/ui`: no duplicated `os.Getenv`, no second zoom-toast
constant, no second suppression set. (The two environment-variable instances this
pass found are in `internal/image` and `internal/export` — B35 and B41, batch 3.)

## Consumers of the zoom flag

Observed non-test consumers: `reducer_zoom.go` (16 references), `app.go` (14 —
layout predicates, `computeFrame`, the `status := ""` conditional at 3385),
`panellayout.go` (4 — `Compute` only), `windows.go` (1 — `windowBounds`),
`mode_insert.go` (1 — the esc arm), `view_messages.go` (1 — cache key),
`view_thread.go` (1 — cache key), `reducer_workspace.go` (1).

Of the consumers fs-statusrow-math names, these could **not** be confirmed:

- **`PanelAt`** — confirmed it does *not* receive the override: no `zoomed`
  parameter, and `app.go:1829` passes none. **Unit claim false** (B46).
- **"every parallel mouse router"** — `reduceMouseClick` confirmed not
  zoom-aware, own literal (B46). `reduceMouseWheel` (`reducer_mouse.go:59`)
  showed no `a.height` guard in the lines examined, so it may be unaffected —
  **unverified**. `reducer_modal_click.go` does not mention `zoomed` at all.
- **sixel/kitty placement bounds** — no zoom/status/`height-1` reference in
  `sixelpaint.go`. **Unit claim unsupported** (B51).
- **command mode** — `command.go` has no zoom reference. **Unit claim vacuous**
  (B47).
- **"golden shows content on the last row"** — not verified; neither the delegate
  nor I opened `testdata/golden/`. Note B46 means this can be true while the row
  is still unclickable, so confirming it would not discharge the unit.

Consumer the units do **not** list at all: **`windowBounds`**
(`windows.go:52-56`), which calls `Compute` with `zoomed=true` and a hardcoded
`threadFront=false`, producing a window rectangle sized to the zoomed layout.

## Not reported (checked, held)

- `g`-prefixed chords arming while zoomed — already recorded.
- **B15** thread save/restore vacuity — already recorded. B49 is a *different*
  defect: the messages pane being clobbered by legitimate movement, not the
  thread pane being unobservable.
- `toastWithClear` vs `uploadToastCmd` as duplication — excluded by the brief as
  resolved. The stale *comment* is new and is B55.
- `PanelAt`'s `y - 1` top-border subtraction — still correct while zoomed, since
  `paneBorder` stays 2 in both zoom branches (`panellayout.go:105-107`).
- `disarmPendingChords` called in all three transitions — correct and deliberate,
  not duplication.
- Three spellings of the front-pane predicate — distinct questions behind a
  single funnel; see the duplicate-class section.
- `mode_normal.go:162,177` handling ToggleSidebar/ToggleThread with no zoom guard
  — correct: the reducer chain runs before `dispatchModeKey`, so suppression lands
  upstream. Hazard for test authors, though: `runKeyCases`/`dispatchModeKey`
  bypasses the chain per AGENTS.md, so a suppression test written that way would
  prove nothing.
- Insert mode's lack of a suppression arm — the layout-toggle keys are handled
  only in `mode_normal.go`, so there is nothing to suppress there.

## Cross-references

- **B45 and B46 are both B13's class** — `[x]` with an oracle whose subject the
  defect cannot reach. B13 was a frame-delta recaptured after the event; B45 is a
  model getter for a widget that is not composited; B46 is a golden asked to prove
  a hit-test. That is three instances of one meta-defect in one feature, and it is
  the strongest argument in this plan for the rule *name the observation channel
  before writing the assertion*.
- **B45 contradicts batch 3's "held" note** on the same suppression tests. Batch 4
  is right. Recorded in both appendices so the disagreement stays visible.
- **B46 supersedes fs-statusrow-math's own claim** and should be read before
  anyone marks that item verified.
- **B49 depends on B15**: "restore only when the zoomed pane is messages" needs a
  way to ask which pane was zoomed, which Phase 3's pane hooks would provide.
- **B47's `windowBounds` half** and **B48** both come from the same root: call
  sites that bypass `layoutThreadFront`, the funnel built to prevent exactly that.

