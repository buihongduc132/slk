# AGENTS.md

Orientation for anyone — human or agent — writing code in this repo.

**The single most important rule: search before you write.** The most common
defect in this codebase's history is not bugs, it is the same logic implemented
a fourth time because the author did not know the first three existed. See
[Shared code](#shared-code--check-here-before-writing-a-helper) below.

## Build, test, lint

```
go build ./...
go test ./...                 # ~10s
go test ./... -race           # ~47s; this is what CI runs
go vet ./...
golangci-lint run             # v2.13.1, config in .golangci.yml
gofmt -l .                    # must be empty; enforced in CI
```

Those two timings are wall clock with `-count=1` and a warm build cache on an
8-core Linux box, re-measured at the end of Phase 0 (they were `~21s`/`~44s`
before it, on unrecorded hardware — treat them as a shape, not a target).
The shape is what matters: under `-race`, four packages are ~95% of the run —
`internal/ui` 32s, `cmd/slk` 29s, `internal/cache` 19s, `internal/ui/messages`
16s (they overlap, hence a 47s wall). If you are iterating, run the one package
you are changing; `internal/ui` alone is now longer under `-race` than the whole
suite used to be.

Tests are plain `testing.T`, stdlib only. No testify, no gomock, no golden
libraries. White-box (`package ui`, not `package ui_test`) by convention.

## Architecture in one screen

```
cmd/slk/                composition root: wiring, workspace connection,
                        WebSocket event handling, message fetch/cache pipeline
internal/core/          the ports (service interfaces) the TUI calls, and the
                        values the TUI and cmd/slk exchange through them
internal/ui/            bubbletea App: reducers, mode key handlers, view regions
internal/ui/<widget>/   self-contained sub-models (messages, thread, sidebar,
                        compose, and 13 modal packages)
internal/slack/         Slack Web API + browser-protocol WebSocket client
internal/slack/edge/    edgeapi: conditional revalidation, server-side search
internal/bootstrap/     startup fetch orchestration
internal/cache/         SQLite cache (a cache, not a source of truth)
internal/config/        TOML config
```

**`wiki/Architecture.md` is stale by roughly 7× and describes a service layer
that no longer exists. Do not trust it.** Current structural documentation:

- `docs/superpowers/plans/2026-09-06-architecture-refactor.md` — the active
  refactor: measured baseline, known problems, phase sequence
- `docs/superpowers/plans/2026-05-23-app-go-solid-refactor.md` — the completed
  `app.go` decomposition; establishes the patterns still in use
- `docs/superpowers/specs/` — one design doc per feature

### Invariants worth knowing

- **`internal/ui` does no I/O of its own.** Slack, SQLite, the filesystem, the
  clipboard, the external editor and launching apps all go through the service
  ports in `internal/core`, which `cmd/slk` wires. No `internal/slack`,
  `slackhttp`, `cache`, `config`, `filedl`, `export`, `editor`, `net/http` or
  `os/exec`; `slack-go` only in `blockkit`, as the data it renders. `internal/ui/boundary_test.go`
  enforces this. The boundary is deliberate; do not breach it.
- **`App.Update` routes through a reducer chain**, not a switch. Add behavior by
  adding to a `reducer_*.go` file, not by extending `Update`. The only step
  outside the chain is the thin `Update` wrapper that records `stackFront`
  (which content pane — messages or thread — last had focus, for the stacked
  thread layout) after every message; do not add other post-chain logic
  there.
- **Per-mode key handling is a table**, `modeHandlers` in
  `internal/ui/mode_handlers.go`. One `mode_*.go` file per mode.
- **SQLite is a cache.** Slack remains authoritative.

## Shared code — check here before writing a helper

If you are about to write text wrapping, box drawing, list windowing,
scrollbars, date formatting, case folding, or ID formatting: it already exists.

### Text and rendering

| Need | Use |
|---|---|
| Word wrap to a width | `messages.WordWrap(s, limit)` |
| Plain-text line segmentation (grapheme-correct) | `messages.PlainLines`, `messages.DisplayWidthOfPlain`, `messages.SliceColumns` |
| Display width of a string (emoji-aware) | `emoji.Width(s)` |
| Case/accent-insensitive fold for matching | `text.Fold(s)` |
| Rank a candidate name against a typed query | `fuzzy.Match(name, query)` → `(Tier, score, ok)`; folds both sides itself, so pass raw strings |
| The individual match predicates behind it | `fuzzy.SubsequenceScore` (in-order walk + word-boundary/tightness score), `fuzzy.WordPrefix`, `fuzzy.SquashedPrefix` |
| Slack mrkdwn → plain text | `messages.FlattenMrkdwn`, `messages.FlattenMrkdwnWithUserGroups` |
| Slack mrkdwn → styled/rendered text, or → CommonMark | `messages.RenderSlackMarkdownWith(text, opts)` (`render.go:670`); `messages.SlackMrkdwnToCommonMark` / `SlackMrkdwnToCommonMarkWithUserGroups` (`render.go:916,922`) — the sibling of `FlattenMrkdwn` for callers that want CommonMark or styled output instead of plain text; used by `internal/export/markdown.go`, `internal/ui/app.go`, `internal/ui/thread/model.go`, `internal/ui/activityview`, `internal/ui/threadsview` |
| Which mrkdwn string a message should actually render | `messages.MessageTextSource(msg)` (`model.go:530`) — the block-vs-text-field resolution rule; called from 3 places outside its own package (`internal/export/markdown.go`, `internal/ui/app.go`, `internal/ui/thread/model.go`), so it belongs beside `BlocksCarryBody` above, not reimplemented at each call site |
| SGR background/foreground codes for a rendered line | `messages.BgANSI()` / `messages.FgANSI()` (`render.go:549,580`), `messages.ReapplyBgAfterResets(text, style)` (`render.go:274`) — `BgANSI` alone is referenced from 18 files, `ReapplyBgAfterResets` from 17, `FgANSI` from 11; despite that reach none of the three were in this table before now |
| Search-term highlighting (ANSI/OSC-safe) | `messages.HighlightSearchTerms`, `messages.SearchHighlightSGR` |
| Extract links from message text | `messages.ExtractLinks` |
| Does message text mention the current user? | `mention.InText(text, selfUserID)` |
| Reaction pill rendering | `messages.ReactionPillText` |
| Date label from a Slack ts | `messages.DateFromTS`, `messages.FormatDateSeparator` — but see the row above for two more parsers of the same ts format that should have called this instead |
| mpdm channel name → human name | `slackfmt.FormatMPDMName` |
| Channel-type glyph (`#` / `◆` / `●`) | `messages.ChannelGlyph(chType)` |
| Slack permalink parsing | `slackurl.Parse` |
| Emoji shortcode → glyph | `emoji.Sprint`, `emoji.CodeMap`, `emoji.StripSkinTone` |
| Frecent ("recent") emoji tier on an emoji surface | `SetFrecentEmoji([]core.EmojiEntry)` on **both** `reactionpicker.Model` and `emojipicker.Model`; data comes from `core.ReactionService.LoadFrecent` via `App` — never read the cache from inside `internal/ui` |
| Does Block Kit already render the message body? | `blockkit.RendersBody(blocks)`, `messages.BlocksCarryBody(msg)` |
| Display-width-aware truncate/pad inside a Block Kit render | `blockkit.truncateToWidth(s, width)` (`internal/ui/messages/blockkit/render.go:472`, ellipsizes on overflow) / `padRight(s, w)` (`:439`) — both go through `lipgloss.Width`, so they're columns-correct like `emoji.Width`/`SliceColumns` above, just living in a third location scoped to Block Kit rendering |
| Current DND state from a Slack API result | `slack.DNDStateFromStatus` |
| Peer custom status, DND and huddle rendering | `ui/peerstatus` (`Status`, glyph/expiry/summary methods); `messages.AuthorStatusSuffix` for author headers |
| Usergroup map helpers | `usergroups.Copy`, `usergroups.Equal`, `usergroups.Display` |
| Copy text to the clipboard | `App.clipboardWrite` / `SetClipboardWriter`; `cmd/slk/newClipboardWriter` selects local macOS `pbcopy` or terminal OSC 52 |

### UI chrome

| Need | Use |
|---|---|
| Scrollbar gutter on a rendered pane | `ui/scrollbar.Overlay`, `ui/scrollbar.Visible` |
| Centered modal over a dimmed backdrop | `ui/overlay.DimmedOverlay` |
| Text selection ranges and anchors | `ui/selection` (`Range`, `Anchor`, `LessOrEqual`) |
| Theme colors and styles | `ui/styles` (`Username`, `SelectionStyle`, `SearchHighlightStyle`, `MentionBadgeStyle`, `UserColor`) |
| Window tree geometry | `ui/wintree` |
| Modal geometry / row hit-testing | `boxedOverlay`, `clickableOverlay` in `internal/ui/reducer_modal_click.go` |
| Channel/DM destination picker for forwarding | `channelfinder.Model.OpenForForwarding()` (joined conversations only); `Open()` restores the normal switcher |
| Enter / leave fullscreen pane zoom | `App.enterZoom`, `App.exitZoom` (`internal/ui/reducer_zoom.go`). **Never assign `a.zoomed` directly** — `exitZoom` also invalidates the zoom-keyed caches and restores the saved viewport, and a second hand-rolled exit in `mode_insert.go` was doing neither |
| Which pane a Compute call should treat as in front | `App.layoutThreadFront()` (`internal/ui/app.go`) — resolves through `zoomFrontIsThread` while zoomed, `threadInFront` otherwise. `windowBounds` is the one deliberate exception (hardcodes `false`) |

### Test helpers

Everything here is unexported and lives in a `_test.go` file, so it is
reachable only from the package that declares it (all of these are
`package ui` unless noted). They are listed because the failure mode this
file exists to prevent — writing a sixteenth ad-hoc test-app builder — is
exactly what happened before Phase 0 consolidated them. Symbols are
greppable by name; no line numbers, because these files move.

| Need | Use |
|---|---|
| Build an `App` for a test or benchmark | `newTestApp(t, opts...)` (`internal/ui/testapp_test.go`) |
| The same without a `testing.TB` | `buildTestApp(opts...)` — only for the four legacy builders whose signatures must not change: `newPanelAtApp`, `sixelTestApp`, `makeBenchApp`, `makeWideScrollApp` |
| Size the App | `withSize(w, h)` (direct field assign) / `withWindowSize(w, h)` (real `tea.WindowSizeMsg` resize path). **Not interchangeable** — only the latter sets `forceSixelRepaint`. Mutually last-wins |
| Seed panes and data | `withMessages`, `withChannels`, `withWorkspaces`, `withThreadsView`, `withChannelFinderOpen` |
| Seed App-level state | `withMode`, `withView`, `withActiveChannel`, `withActiveTeam`, `withChannelService`, `withWindowSplit`. **`withActiveChannel` assigns only `a.activeChannelID`** — nothing derives a *name* from an ID, so the messages-pane header renders as a bare `#`, the statusbar as `#`, and the compose placeholder as `Message #...`. A test that asserts on any of those must push the name through the production trio itself; `newGoldenApp` does it via `nameGoldenActiveChannel` (`internal/ui/golden_test.go`) |
| Populate `a.layout` bands / pane caches (needed for mouse hit-testing) | `withRender()` |
| N plain message fixtures | `testMessageItems(n)` |
| Compare or bless a full-screen frame against `testdata/golden/<name>.ansi` | `compareGolden(t, name, got)` (`internal/ui/golden_test.go`) |
| Re-bless goldens | the package-local `-update` flag: `go test ./internal/ui -run TestGolden -update`. It is not defined repo-wide, so `go test ./... -update` fails |
| An `App` with every render nondeterminism pinned (theme, emoji mode, clock) | `newGoldenApp(t, opts...)`, with `goldenMessages()` / `goldenChannels()` as the fixtures |
| Drive a message through the real `Update` chain and render one frame | `updateAndRender(t, a, msg)` (`internal/ui/thread_breadcrumb_test.go`) |
| An App at a given width, resized via `WindowSizeMsg`, channel focused, for thread-layout tests | `stackedApp(t, w, extra...)` (`internal/ui/thread_stacked_test.go`) |
| Assert which of channel / thread the last frame drew | `assertFront(t, a, wantChannel, wantThread)` (same file) |
| Fake one service method on an `App` | `a.setChannelFetcherForTest(fn)` and its siblings, `setUploaderForTest`, `setClipboardReaderForTest`, `setReadStateReaderForTest`, `setDesktopForTest(func(*core.DesktopServiceFuncs))`, `setFilesystemForTest()`, `setEditorForTest()` (`internal/ui/services_helpers_test.go`). Calls for sibling methods of one service compose instead of replacing each other |
| Table-drive a mode handler's keys | `runKeyCases(t, mode, []keyCase{...})` (`internal/ui/modekeys_test.go`). Calls `dispatchModeKey` directly, so it **bypasses** the reducer chain and the `ctrl+c` / bootstrap / scroll-flush gates ahead of it |
| Build a key message for such a table | `keyPress(r)` printable rune, `keyCode(c)` special key, `keyMod(c, mod)` modified key |
| Count the rows a finder-style modal is showing | `modalRows(bs)` (`internal/ui/mode_workspace_finder_test.go`) — `h - 7`; floors at 1, and is **not** valid for `newmessagepicker` |
| Read the highlighted (`▌`) row out of a rendered modal box | `modalHighlightedRow(t, box)` (same file) |
| Compare two `color.Color` without `==` panicking on a non-comparable dynamic type | `colorEqual(a, b)` (`internal/ui/mode_theme_switcher_test.go`; `package styles` has its own twin in `styles/styles_test.go`) |
| Observe a status-bar toast (no getter exists) | `statusbarText(a)` (`internal/ui/mode_presence_snooze_test.go`) |
| Record status-setter invocations | the `statusCall` struct (same file) |
| A fixture with real unread channels either side of the active one | `unreadOpts()` + `seedUnreads(t, a)` (`internal/ui/mode_normal_keys_test.go`) |
| Establish a focused pane with an asserted selection | `focusMessages(t, a)`, `focusThreadPanel(t, a)` (same file) |
| Park the message viewport at an exact `yOffset` | `scrollTo(off)` (same file) |
| Make nav-history entries resolvable | `navLookupOpt()` (same file) |
| Run only the first command of a `tea.Batch` (skip a 2s tick) | `firstBatchCmd(t, cmd)` (`internal/ui/mode_insert_keys_test.go`) |
| Observe a compose cursor position or blur state (no getter exists) | `afterKeyValue(c, r)` (same file) |
| An `App` zoomed, scrolled mid-history, with a selection | `zoomedScrolledApp(t)` (`internal/ui/fullscreen_red_test.go`) |
| Enter zoom and assert the frame changed | `mustEnterZoom(t, a, frame)` (same file) |
| Assert a frame is/isn't the zoomed layout | `assertZoomedFrame(t, a, zoomed, marker)` (same file) |
| First differing line between two frames, for a diff message | `firstLineDiff(want, got)` (`internal/ui/golden_test.go`) |
| Strip ANSI from a rendered frame | `stripANSI(s)` (`internal/ui/golden_test.go`) — a one-line wrapper over `ansi.Strip`. **`internal/ui/statusbar` has a second copy** (`statusbar/model_test.go`); call `ansi.Strip` directly in new packages rather than adding a third |
| Emoji fixtures filtered from the real codemap | `entriesFor(t, names...)` (`internal/ui/emojipicker/fuzzy_test.go`) |
| Names of a picker's filtered rows | `filteredNames(m)` — **declared twice**, `emojipicker/fuzzy_test.go` (value receiver) and `reactionpicker/fuzzy_test.go` (pointer receiver). Unexported, so neither is reachable from the other; the signatures differ, so consolidating means moving both models' shared shape first |
| Is a name among a picker's rows? | `containsName(names, want)` — **declared twice**, same two files, identical bodies |
| Open the thread on the selected message, or open it and hand focus back to the channel pane | `openThread(t, a)`, `openThreadThenFocusMessages(t, a)` (`internal/ui/window_bounds_zoom_test.go`) — both drive real keys through the reducer chain, unlike `focusThreadPanel`, which assigns `threadVisible` directly |
| Assert zoom is on with MESSAGES as the promoted pane | `assertZoomedPaneIsMessages(t, a, why)` (`internal/ui/zoom_front_pane_test.go`) |
| The measured widths either side of the side-by-side layout branch | `stackedWidth` (120), `sideBySideWidth` (200), `firstSideBySideWidth` (162 — 161 stacks) (same file). With the shared fixture's 6-col rail and 30-col sidebar; re-measure if either changes |
| The same branch with the sidebar HIDDEN | `firstSideBySideWidthNoSidebar` (130 — 129 stacks), `sidebarDecidesWidth` (150) (`internal/ui/zoom_sidebar_hidden_tab_test.go`). The sidebar's 30+2 cols are an **input** to that branch, so `firstSideBySideWidth` does not carry over: 130–161 is a band where hiding the sidebar alone flips the layout from stacked to side-by-side, and with it which pane zoom promotes. Both boundary constants are **asserted**, not just declared — `TestZoomSidebarHidden_BoundaryWidthMovesWhenTheSidebarGoes` here, `TestZoomFrontPane_G15_BoundaryWidth` for 162. Go does not flag an unused constant, so a geometry number nothing references is a hand-maintained comment; if you add one, pin it |
| Hide the sidebar through the real `ctrl+b` path, asserting it went | `hideSidebar(t, a)` (same file). Only valid **unzoomed** — `ctrl+b` is in `zoomSuppresses`, so while zoomed it raises the toast and the sidebar stays put |
| Every sidebar-hidden zoomed configuration, with the one pane its Tab ring must hold | `sidebarHiddenRings()` + `zoomedSidebarHiddenApp(t, tc)` (same file) |
| Format a `uint8` for a hand-built SGR background code | `itoaU8(v)` + `fmtRGBBg(r, g, b)` — **declared four times**, byte-identical (`threadsview`'s copy is reformatted onto one line but is the same expression): `internal/ui/messages/model_test.go:1121,1135`, `internal/ui/thread/model_test.go:370,384`, `internal/ui/compose/model_test.go:943,960`, `internal/ui/threadsview/model_test.go:335,350`. Worse than any other "declared twice" row here, and not a simple delete: each is an unexported helper private to its own package, so consolidating means standing up a shared test-support package first, same shape as the `filteredNames`/`containsName` problem below |
| Type a string through the real key-`Update` loop | six independent helpers, none sharing a signature: `typeChars(t, m, s)` (`internal/ui/compose/model_test.go:468`), `typeText(m, s)` (`compose/channel_test.go:21`), `typeInto(t, c, s)` (`internal/ui/mode_insert_keys_test.go:71`), `typeIntoCompose(a, s)` (`compose_frecent_wiring_test.go:47`), `typeCommand(a, s)` (`mode_command_test.go:10`), `typeIntoFinder(a, s)` (`channel_finder_search_test.go:37`, returns the debounce messages fired). `mode_insert_keys_test.go` is a hash-pinned oracle — `typeInto` may be called or listed, but that file itself must not be edited |

The `stripANSI` / `entriesFor` / `filteredNames` / `containsName` rows are the
registration B34 found missing: all eight helpers
existed and none were listed, which is precisely the condition this table exists
to prevent. Three of them had already been written twice by the time anyone
noticed. Treat the "declared twice" notes as debt, not as license — the
`filteredNames`/`containsName` pair sits inside the two packages this repo's
fuzzy work set out to unify behind one matcher. `itoaU8`/`fmtRGBBg` is the same
problem one level worse: four packages, not two.

### Known duplication — do not add to it

These are tracked in the refactor plan and are being consolidated. Do not copy
them as templates:

- **11 `renderBox` implementations** and **7 `visibleWindow`**, mostly but not
  cleanly across the 13 modal packages: one of the 11 `renderBox`es is in
  `internal/ui/reactionsview/model.go:159`, which is not one of the 13; and
  `channelpicker` and `mentionpicker` are 2 of the 13 that contribute neither
  helper (they filter with plain prefix/substring matching instead). If you are
  building a modal, expect a shared chrome package to land (Phase 4); coordinate
  rather than adding a twelfth copy.
- **`messages.Model` and `thread.Model`** share approximately 377 verbatim
  lines (right order of magnitude; not exactly reproducible) and **55
  identically-named methods, 48 of them exported** — `func (m *Model)
  X(...)` declarations intersected between `internal/ui/messages/*.go` (97,
  excluding `_test.go`) and `internal/ui/thread/*.go` (82, same exclusion).
  `internal/ui/thread/lockstep_test.go` pins *render*
  parity in **one static state only**: 80×20 (`lockstepWidth`/`lockstepHeight`),
  chosen so neither pane scrolls — so no scroll offset, no scrollbar gutter, no
  search-term highlighting and no loading state is compared. The
  scroll/viewport/selection math, which is most of the 377 shared lines, is
  pinned *within* each model (`messages/scrollbar_test.go`,
  `messages/selection_test.go`, `thread/selection_test.go`,
  `thread/model_test.go`) but **not across** them: a change that breaks one
  model's scrolling and not the other's will not fail the lockstep test. Its doc
  comment enumerates **15 verified
  divergences** — that list is the specification Phase 3's pane hooks have to
  satisfy, not a wishlist. If you change one model, change both, and expect the
  lockstep test to tell you when you forgot — within the limits above. Note that
  `TestLockstep_ReactionHitTestFrames` is a tripwire that fires on
  *convergence*: Phase 3 must delete it, not satisfy it.
  One divergence the "identically-named" framing hides: `messages.Model.ClickAt(y
  int) bool` (`internal/ui/messages/model.go:2554`) returns whether the click
  landed, while `thread.Model.ClickAt(y int)` (`internal/ui/thread/model.go:1027`)
  returns nothing. Same name, same receiver shape, different signature — a
  consolidation that assumes signature parity across the 55 breaks here, and the
  lockstep test cannot catch it because it compares rendered frames, not
  signatures.
- **`convertAndCacheHistory` / `fetchChannelMessages` / `fetchThreadReplies`** in
  `cmd/slk/main.go` have ~85% duplicated bodies.
- **`internal/ui/newmessagepicker/filter.go:74`** hand-rolls a tiered matcher,
  `matchTier` (prefix/substring/subsequence, mirroring `fuzzy.Match`'s tier
  concept) plus its own `isSubsequence` at `:91`, both walking `strings.ToLower`
  instead of `text.Fold`. That's a behavioral gap, not just duplication:
  `strings.ToLower` does not fold accents, so this picker's matching diverges
  from every other surface's. Of the 9 picker/finder packages
  (`channelfinder`, `channelpicker`, `emojipicker`, `linkpicker`,
  `mentionpicker`, `newmessagepicker`, `reactionpicker`, `themeswitcher`,
  `workspacefinder`), 5 do not import `internal/fuzzy`. Do not read that as 5
  offenders: `linkpicker` has no filtering concept at all, and `channelpicker`,
  `themeswitcher` and `workspacefinder` do plain prefix-then-substring
  filtering through the correct `text.Fold` — they just don't need a scored
  matcher. `newmessagepicker` is the only one of the 5 that built a
  competing tiered matcher from scratch. (It is not, as has been claimed
  elsewhere, "the only package that skips `internal/fuzzy`" — four other
  packages also skip it, without the behavioral gap.)

**Suspected dead code — do not delete on sight.** `messages.RenderAttachments`
(`internal/ui/messages/render.go:137`) has zero call sites anywhere in the
repo, including its own package's tests. That is unusual enough to flag rather
than assume: either something that used to call it was removed and it was
missed, or a caller path exists that a plain grep for the identifier can't
see. Confirm before removing it.

**Scope gap: `cmd/slk` has no helper table.** `cmd/slk` carries ~103 unexported
top-level functions across 37 non-test files and, unlike every `internal/ui/*`
package, none of them are tracked anywhere above — the only `cmd/slk` entries
in this file are the composition-root description, the `-race` timing, the
I/O-boundary note, one clipboard-writer row, and the
`convertAndCacheHistory`/`fetchChannelMessages`/`fetchThreadReplies`
duplication. The three-way timestamp duplication and the `RenderAttachments`
gap above were both found by hand-checking claims about `internal/ui`; nobody
has done the equivalent sweep for `cmd/slk`. Treat this as an open item, not
something this pass closes.

## Conventions

**Adding a reusable helper?** Add it to the tables above in the same commit. An
unlisted helper gets re-implemented. When this file and the code disagree, the
code is right and this file is a bug — fix it.

**Two implementations that must stay parallel?** Express it as an interface with
a compile-time assertion (`var _ Chrome = (*Model)(nil)`) or a lockstep test.
Not a comment. The comment at `internal/ui/thread/model.go:35-36` — "this shape
mirrors `internal/ui/messages.viewEntry` exactly; keeping them in lockstep
means…" — was the counter-example: prose asking humans to maintain a 377-line
invariant by hand. Phase 0 discharged it. The invariant now has a test,
`internal/ui/thread/lockstep_test.go`, which asserts the shared render
behaviour in one static state and carries a documented 15-item divergence list.
Do the same: when
you find a comment standing in for a check, replace it with the check.

**Extract the substrate, not the widget.** Share the uniform part; leave the
divergent part alone. Forcing genuinely different behavior into a common shape
is worse than the duplication it removes.

**Refactoring?** Moving functions is free — two phases of the prior refactor
moved ~3,000 lines with zero test changes. Moving *state* is not: `internal/ui`
test files make **2,905 references to 142 distinct unexported `App` fields and
methods** (type-checked count, not a grep; measured at the end of Phase 0, up
from 1,995 / 121 before it — Phase 0's characterization tests added ~910 of
them). Budget roughly **220 mechanical test-line edits per 10 extractions**, and
expect five members to dominate: `messagepane` (243), `compose` (204),
`focusedPanel` (146), `mode` (129) and `activeChannelID` (125) are 847 of the
total between them. Method and per-member breakdown:
`docs/superpowers/plans/2026-09-06-architecture-refactor.md`, Phase 0 "Achieved".

**Found a bug while refactoring?** Record it, annotate it, raise it separately.
A refactor commit that also changes behavior cannot be reviewed.

## Workflow

Per the README: brainstorm the design first, write tests, then implement.
Designs go to `docs/superpowers/specs/YYYY-MM-DD-<topic>-design.md`,
implementation plans to `docs/superpowers/plans/`.

Before opening a PR: `go build ./...`, `go vet ./...`, `go test ./... -race`,
`gofmt -l .` empty.
