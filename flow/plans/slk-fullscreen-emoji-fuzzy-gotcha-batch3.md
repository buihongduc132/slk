# Gotcha Coverage — slk-fullscreen-emoji-fuzzy (batch 3: DOD criteria)

> Source: `flow/plans/slk-fullscreen-emoji-fuzzy.md`
> Mode: plan (detected `## Tasks` + `## DOD`, not `## Declarative Items` — see the
> structure note in `-gotcha.md`)
> Sub-agent: batch 3 of 4, delegated per `cmd-pallet/gotcha-coverage`
> Units reviewed: dod-1 (zoom), dod-2 (emoji cache), dod-3 (fuzzy ranking),
> dod-4 (harness discipline)
> Numbering: B32–B44, continuing from batch 2's B31
> Appendix only. The plan's own DOD text is unmodified.

## Provenance, and why this batch took four attempts

This batch and batch 4 were dispatched four times. The first three attempts
returned nothing and I recorded them as "lost". That diagnosis was **wrong**, and
the correction matters more than the findings:

The fourth pair of agents **completed successfully** — 6 tool calls each (inside
an 8-call budget), 103 and 118 lines of report, both in the requested schema. What
failed was *delivery of the completion notification*. `ListAgents` showed zero
in-process agents while two finished transcripts sat on disk. I only found them
by checking the transcript files directly instead of trusting the agent listing.

So the lesson is not "delegates flood their context on large packages" (my
hypothesis for attempts 1–3, which the 8-call budget was supposed to test and
which the evidence now refutes — these agents used 6 calls and still did not
report). The lesson is: **an absent completion notification is not evidence of an
absent result.** Check `tasks/<id>.output` for a final assistant text block before
concluding an agent died. Extraction that does not blow up the parent's context:

```
jq -r 'select(.type=="assistant") | .message.content[]?
        | select(.type=="text") | .text' <id>.output > report.md
```

Every finding below was **re-verified by me against the tree** before being
recorded. Where my check disagreed with or sharpened the delegate's claim, the
finding says so. Two of the delegate's claims were sharpened; none were refuted.

## Findings (ranked)

### Rank 5 (Sophisticated — fundamental gap, invalidates or limits the unit)

- **B32 — dod-3's headline proof is asserted IMPOSSIBLE by the test file cited as
  its proof**
  - Unit: dod-3
  - What: the criterion reads "Proven by emojipicker tests (`:thumbs` → `+1`
    reachable…)". `internal/ui/emojipicker/fuzzy_test.go:135-137` is a **negative
    guard** asserting the opposite: `// Negative guard: "+1" matches no tier of
    "humbs".` and `if containsName(filteredNames(m), "+1") { t.Errorf(…no tier
    applies to it) }`. `TestFuzzy_ThumbsStillReachesThumbsEmoji:236` has fixture
    `entriesFor(t, "thumbsup", "thumbsdown")` — `+1` is not in it. No alias table
    exists anywhere in the package (verified: zero case-insensitive matches for
    `alias` under `internal/ui/emojipicker/`). `+1` shares no character with
    `thumbs`, so no character-level tier can reach it; only an alias map could.
  - **My correction to the delegate**: this is not undetected drift. The test
    *documents the substitution explicitly* at :230-235 — "G11's documented
    substitution: the DOD's `:thumbs -> +1` cannot be won on name merit… so the
    plan's example is pinned here as the REACHABLE pair instead". The
    implementer noticed, wrote it down, and substituted deliberately. Only the
    DOD sentence was never corrected. That lowers the "nobody knew" severity and
    raises the process severity: the knowledge exists in the test and never
    propagated to the criterion it invalidates.
  - Severity: a criterion marked proven whose named example is a documented
    non-feature. A future contributor "fixing" `+1` reachability is blocked by a
    *passing* test that forbids it — the worst kind of obstacle, because it looks
    like a specification.
  - Mitigation: decide which is authoritative. Either add an alias map to the
    entry source and invert the :135-137 guard, or strike `+1` from the DOD and
    use the pair the test already pins (`:thumbs` → `thumbsup`). The test's own
    comment is the decision record; the DOD should quote it.
  - Evidence: `internal/ui/emojipicker/fuzzy_test.go:124-140,228-247`; alias
    absence verified by `grep -rin alias internal/ui/emojipicker/` → no output.

- **B33 — dod-3's ranking chain names one tier that is not wired and omits two
  that are; no emojipicker test pins the real tier order**
  - Unit: dod-3
  - What: the claim is "ranked (recent > prefix > substring > subsequence)" —
    four names for a six-constant enum. "recent"/frecency is not wired into
    emojipicker at all (that is B24). `TierWordPrefix` and `TierSquashedPrefix`
    are real and sit *between* prefix and substring, and in emojipicker they have
    **zero ordering coverage**: the only tier references in that package's tests
    are inside `TestFuzzy_TierOrderPrefixBeatsSubstringBeatsSubsequence` and
    `TestFuzzy_SubsequenceTierIsScoreAware`, neither of which names them.
  - Relationship to B30, which is already fixed: B30 was the same gap at the
    `internal/fuzzy` layer and is now closed (`tier_constants_test.go` pins all
    six by value). **This is the consumer half and is still open.** Pinning the
    constants stops a renumber; it does not prove emojipicker *ranks* by them.
  - Severity: a regression that mis-tiers a word-boundary or squashed-prefix
    match — `party-parrot` matched on `parrot` is the common real case, since
    custom emoji names are hyphenated — reorders the picker for every user and
    fails no test. Two of five tiers unguarded at the consumer.
  - Mitigation: add emojipicker ordering cases for WordPrefix and
    SquashedPrefix, or amend the DOD to the three tiers actually pinned and drop
    "recent" until B24 lands. Do not leave the DOD naming a superset of what is
    tested.
  - Evidence: `internal/fuzzy/fuzzy.go:14-15,31,34`;
    `internal/ui/emojipicker/fuzzy_test.go:141,148,256`.

### Rank 4 (Significant)

- **B34 — dod-4's "existing harnesses only" is false, and three of the new
  helpers are already duplicated across packages**
  - Unit: dod-4
  - What: dod-4 enumerates the permitted harnesses. The feature tests add eight
    helpers on none of those lists, **none** of which appear in the AGENTS.md
    Test helpers table (verified individually): `zoomedScrolledApp`,
    `mustEnterZoom`, `assertZoomedFrame` (`internal/ui/fullscreen_red_test.go`),
    `firstLineDiff`, `stripANSI` (`internal/ui/golden_test.go`), `entriesFor`,
    `filteredNames`, `containsName` (the picker packages).
  - **My sharpening of the delegate's claim** — it flagged `stripANSI`
    duplication as an "unverified suspicion". It is confirmed, and it is not
    alone. Three of the eight are already declared **twice**:
    - `stripANSI` — `internal/ui/golden_test.go:131` **and**
      `internal/ui/statusbar/model_test.go:353`
    - `filteredNames` — `internal/ui/emojipicker/fuzzy_test.go:31` **and**
      `internal/ui/reactionpicker/fuzzy_test.go:37`
    - `containsName` — `internal/ui/emojipicker/fuzzy_test.go:39` **and**
      `internal/ui/reactionpicker/fuzzy_test.go:45`
    So this is not a documentation gap. It is AGENTS.md's stated top defect mode
    — "the same logic implemented a fourth time because the author did not know
    the first three existed" — happening *inside the feature whose DOD claims it
    did not happen*. The picker pair is the sharper case: two packages that the
    plan explicitly set out to unify behind one matcher each grew their own copy
    of the same two assertions.
  - Severity: dod-4 cannot be verified true; it is falsified by the files it
    covers. And the next zoom-adjacent author greps the table, finds nothing, and
    writes a ninth copy.
  - Mitigation: register all eight in the AGENTS.md table in the same commit as
    any further test work (the convention the file states about itself). Then
    reword dod-4 to "existing harnesses, plus newly registered X/Y/Z". For the
    three duplicated ones, delete one copy and share it — `stripANSI` is a
    one-line wrapper over `ansi.Strip` and needs no local copy at all.
  - Evidence: eight `grep -c` against `AGENTS.md` (all 0) plus `grep -rn "func
    <name>"` per helper, output in the session transcript.

- **B35 — `os.Getenv("TMUX")` is read three times, and one read bypasses its own
  package's injectable accessor** *(the mandated gotcha class)*
  - Unit: dod-1, and global
  - What: `TMUX` is resolved independently at `cmd/slk/main.go:416`,
    `internal/ui/app.go:791`, and `internal/image/kitty.go:65`. The third is the
    defect: `internal/image/capability.go:35` declares `var getenv = os.Getenv`
    with the comment "overridable in tests", and `kitty.go` — *the same
    package* — calls `os.Getenv` directly instead.
  - **Worse than the delegate reported**: `capability.go:26` already reads
    `TMUX` through that accessor (`TMUX: getenv("TMUX")`). So this is not merely
    "an accessor exists and was bypassed" — it is the **same variable read twice
    within one package, once injectable and once not**. That is the canonical
    shape in its purest form.
  - Severity: any test injecting a fake `getenv` to simulate tmux gets an
    inconsistent view — `capability.go` sees the fake, `kitty.go` sees the real
    environment, and image-capability decisions disagree inside one package.
    Lands on dod-1 because a zoom toggle forces a repaint path
    (`forceSixelRepaint`), so a zoomed pane with images is decided partly by one
    answer and partly by the other, and no zoom golden can see it.
  - Mitigation: **delete** the direct call — `kitty.go:65` becomes
    `return getenv("TMUX") != ""`. Never alias. Separately, `internal/ui` should
    receive tmux state through the `inTmux` field it already has rather than
    `cmd/slk` re-reading the variable at `main.go:416`.
  - Evidence: `internal/image/capability.go:26,34-35`;
    `internal/image/kitty.go:65`; `internal/ui/app.go:791`;
    `cmd/slk/main.go:416`. All four re-verified by grep.

- **B36 — dod-2's stated proof points at two files that contain no cold-start
  test; the real proof lives in two files the DOD does not name**
  - Unit: dod-2
  - What: dod-2 says "Proven by `internal/cache` + `cmd/slk` tests using
    `fakeEmojiLister` (existing, `customemoji_test.go`)". Neither cited file
    exercises startup: `cmd/slk/customemoji_test.go` holds
    `TestWorkspaceContextCustomEmoji{DefaultsEmpty,Replaces,ConcurrentAccess}`
    plus four `TestFetchWorkspaceEmoji_*`; `internal/cache/customemoji_test.go`
    holds ten pure upsert/read/table-shape round trips. The restart proof is in
    `cmd/slk/customemojiseed_test.go` and
    `cmd/slk/customemojiseed_nonclobber_test.go` — **neither named by dod-2**.
    Those are the files created for B17/B18, i.e. the criterion predates its own
    evidence and was never re-pointed.
  - Severity: a reviewer auditing dod-2 opens the cited files, finds no restart
    coverage, and must either reject a well-covered criterion or take it on
    faith. Worse, the real proof files are unprotected: deleting
    `customemojiseed_test.go` breaks nothing any DOD claims to depend on.
  - Mitigation: re-point dod-2 at `customemojiseed_test.go` and
    `customemojiseed_nonclobber_test.go` by name, keeping
    `internal/cache/customemoji_test.go` for the persistence half.
  - Evidence: delegate enumerated both cited files by test name and line; I
    confirmed the seed files exist and carry the startup-order oracle (they are
    the ones added under B17/B18 and `startup_order_test.go`).

- **B37 — replace-not-merge means a SUCCESSFUL but truncated `emoji.list`
  destroys the cached set**
  - Unit: dod-2
  - What: the cache contract is destructive replace
    (`TestCustomEmoji_UpsertReplacesNotMerges`,
    `TestCustomEmoji_EmptyUpsertClearsTheTeam`). The covered failure paths key on
    an **error** (`TestFetchWorkspaceEmoji_ErrorKeepsBootstrapSubset`,
    `…ErrorLeavesCacheUntouched`). A 200 response carrying a partial set is not
    an error, and flows into the same replace path, clearing every emoji absent
    from that page. dod-2 says only "`emoji.list` refresh updates the cache",
    which reads as additive.
  - Severity: on a workspace large enough to paginate, one refresh silently
    drops most of the cache; the next cold start then renders `:shortcode:`
    literals — precisely the regression dod-2 exists to prevent. Degrades over
    time, worst on the largest workspaces.
  - Mitigation: have the lister signal completeness (a `complete bool` or an
    exhausted-pagination sentinel); replace only on a complete fetch, otherwise
    merge. Add a test with a deliberately partial *successful* response
    asserting the prior set survives.
  - Evidence: **partially verified.** Test names confirmed; the delegate stated
    plainly that it read names and not bodies, and did not verify the lister's
    pagination behaviour. Whether `emoji.list` as called here can return a
    partial success is **unverified** — that is the first thing to check before
    acting, and it is why this is Rank 4 and not 5.

- **B38 — dod-3 claims out-of-order subsequence matching; the implementation is
  in-order and a test pins it that way**
  - Unit: dod-3
  - What: the criterion reads "finds emojis by substring and **out-of-order**
    subsequence". `TestFuzzy_OutOfOrderQueryMatchesNothing:113` asserts the
    opposite, with the comment at :101 unambiguous — `"krt" must NOT match rocket
    (in-order semantics, G10)`. Out-of-order is a deliberate non-feature with a
    guard test, described in the DOD as delivered.
  - Severity: likely just wording — "out-of-order" probably meant
    "non-contiguous", which is what `rkt` → `rocket` shows. Rank 4 not 5 for
    that reason. The cost is review integrity: two of dod-3's three factual
    assertions (`+1`, out-of-order) contradict its own oracle, so the criterion
    cannot be checked by reading it.
  - Mitigation: reword to "non-contiguous in-order subsequence", matching G10
    and the guard test.
  - Evidence: `internal/ui/emojipicker/fuzzy_test.go:101,113` — re-verified.

### Rank 3 (Moderate)

- **B39 — dod-1 states one restore contract for two exit paths that deliberately
  differ**
  - Unit: dod-1
  - What: "prior pane state restored" is unconditional. `exitZoom` restores the
    viewport (`reducer_zoom.go:79`); `clearZoom` deliberately does not
    (:94-112). The test name admits the weaker path is unpinned:
    `TestFullscreen_ZoomAutoClearsStateNotFrame` — "StateNotFrame" says in its
    own name that the rendered frame is not compared on the auto-clear path.
  - Severity: a maintainer reads the DOD, assumes auto-clear restores, and
    "fixes" the deliberate behaviour — or files it as a bug. No test contradicts
    either move, because that oracle checks state, not frame.
  - Mitigation: split the sentence — user-initiated `esc`/`z` restores pane
    state byte-for-byte; auto-clear restores layout only and intentionally leaves
    the viewport alone. The reasoning already exists in `clearZoom`'s doc
    comment; the DOD should not contradict it.
  - Evidence: `internal/ui/reducer_zoom.go:72-80,94-112` — re-verified.

- **B40 — "zero network wait" is a timing claim with no timing oracle**
  - Unit: dod-2
  - What: dod-2 asserts "cold start renders custom emoji from SQLite with **zero
    network wait**". The closest oracle is `TestStartupEmojiOrder_SeedBeforeFetch`,
    which pins *call ordering*. Ordering is a different property: seed-before-
    fetch still permits the seed to sit behind a startup barrier, and nothing
    asserts the first **render** is unblocked by the `emoji.list` round trip. No
    test constructs a slow or never-returning lister and asserts a frame appears
    anyway.
  - Severity: a future change that awaits the fetch before first paint satisfies
    every existing test while reintroducing a network-blocked cold start — the
    exact regression dod-2 exists to prevent.
  - Mitigation: add a test with a `fakeEmojiLister` that blocks until released;
    assert the seeded set is published and a frame renders before release, then
    release and assert the refresh lands.
  - Evidence: `cmd/slk/customemojiseed_test.go:134,159` — names verified by the
    delegate, **bodies unread**, so whether `SeedBeforeFetch` additionally
    asserts non-blocking is unverified.

- **B41 — `XDG_DATA_HOME` is resolved independently in two packages** *(mandated
  class, second instance)*
  - Unit: global
  - What: `cmd/slk/paths.go:17` and `internal/export/markdown.go:71` each carry
    their own `if dir := os.Getenv("XDG_DATA_HOME"); dir != ""`, with independent
    fallbacks for the unset case.
  - Severity: the fallbacks can diverge, so exports can land outside the
    directory the rest of the app treats as its data root — user-visible as "my
    export isn't where my data is" — and it drifts silently the moment one
    fallback is edited. Note this is exactly the variable the slk-dev deployment
    relies on for isolation, so a divergence here also weakens that isolation.
  - Mitigation: **delete one.** Move the resolution into a low-level package
    both can import, or pass the resolved data dir into the export call.
    `internal/export` cannot import `cmd/slk`, which is why it was re-derived —
    so the shared home has to be a third package, not either of these two.
  - Evidence: `cmd/slk/paths.go:17`; `internal/export/markdown.go:69-71` —
    re-verified. `XDG_CONFIG_HOME` and `XDG_CACHE_HOME` appear once each
    (`paths.go:9,25`), so this is the only duplicated XDG variable.

- **B42 — dod-3 says "ranked" but its own example does not rank first, and the
  oracle deliberately avoids pinning position**
  - Unit: dod-3
  - What: `internal/ui/emojipicker/fuzzy_test.go:21-23` records that for the
    DOD's own query, `"rocket" ranks 2nd (80) behind bookmark_tabs (92)`. The
    scoring test asserts only top-three membership, by design, to avoid pinning
    brittle scores. And `TestFuzzy_RocketReachableBySubsequence` is
    fixture-constrained — ":104 FIXTURE CONSTRAINT: every entry here must match
    `rkt`" — so its presence assertion can only fail if the filter returns
    nothing at all; it cannot detect mis-ranking. Close to the vacuous shape but
    **not** vacuous: the prefix-only regression it guards would return empty and
    trip it.
  - Severity: a reader expects best-match-first for `rkt`. Ranking quality can
    degrade from 2nd to 3rd with no failure, and a future complaint that `rkt`
    should surface `rocket` first is not a regression against these tests.
  - Mitigation: state the real contract in the DOD — tier order is pinned;
    within-tier position is score-ordered but only top-3 membership is asserted
    for the example. Cross-reference OT6, which is the same example failing for a
    different reason (custom-emoji eviction).
  - Evidence: `internal/ui/emojipicker/fuzzy_test.go:21-23,104,249-256`.

### Rank 2 (Minor)

- **B43 — dod-1's "`esc` (and `z`) exit" is unconditional; four tests establish
  conditions where `esc` does not exit**
  - Unit: dod-1
  - What: `fullscreen_red_test.go` has
    `TestFullscreen_EscPeckingOrder_ModalOwnsEsc_ZoomPersists:206` and
    `_UploadArmBeatsZoomExit:235` — two cases where `esc` is consumed elsewhere
    and zoom survives — alongside `_ZoomExitBeatsInsertExit:272` and
    `_ZoomExitBeatsPickerClose:295` where it does exit. The behaviour is a peel
    order, not a rule.
  - Severity: documentation only; code and tests are correct and cover both
    directions. Cost is a wrong mental model of `esc`.
  - Mitigation: one clause — "`esc` exits zoom unless a modal or an armed upload
    owns it first."
  - Evidence: `internal/ui/fullscreen_red_test.go:206,235,272,295`.

### Rank 1 (YAGNI)

- **B44 — `fullscreen_zoom_exit_restored.ansi` is a re-blessable golden covering
  a property already proven by byte-equality**
  - Unit: dod-1
  - What: the restored frame has both a golden (`fullscreen_red_test.go:819`)
    and a stronger in-test oracle —
    `TestFullscreen_ZTogglesAndZAgainRestoresByteEqualFrame` compares against the
    *captured* pre-zoom frame. The golden is the weaker of the two and can be
    silently ratified by `-update` after a real regression.
  - Severity: negligible while the byte-equality test exists; matters only if
    that test is deleted.
  - Mitigation: none needed. Recorded so nobody treats the golden as the
    authoritative restore oracle. See also OT19: the *live* capability suite's
    frame comparison is informational for a related reason.
  - Evidence: `internal/ui/fullscreen_red_test.go:140-167,819`.

## Environment-variable / duplicate-value class

The mandated class was checked across every `os.Getenv` / `os.LookupEnv` in
non-test Go files — 15 `Getenv` sites, zero `LookupEnv` (count re-verified).
**Two new instances, both above:**

1. **B35 — `TMUX` read three times**, with `internal/image/kitty.go:65` bypassing
   its own package's `var getenv = os.Getenv` while `capability.go:26` uses it
   for the same variable. Resolution: delete the direct call.
2. **B41 — `XDG_DATA_HOME` resolved in two packages**. Resolution: delete one,
   thread the resolved dir through a package both can import.

Clean on inspection: `XDG_CONFIG_HOME` / `XDG_CACHE_HOME` once each
(`cmd/slk/paths.go:9,25`); `WAYLAND_DISPLAY` once
(`cmd/slk/clipboard_wayland.go:20`); `SLK_DEBUG` once
(`internal/debuglog/debuglog.go:58`). `cmd/slk/main.go:300` and
`internal/slackdesktop/configdir.go:62` pass `os.Getenv` as an injected function
rather than reading a named variable — the correct pattern, not duplication. Tier
constants are declared once (`internal/fuzzy/fuzzy.go`); after the B25 fix
`channelfinder` only names them in a comment.

Beyond the environment-variable shape, the same class appears in the **test
helpers** (B34): `stripANSI`, `filteredNames` and `containsName` are each
declared twice. Recorded there rather than duplicated here.

## Not reported (checked, held)

- **dod-1's goldens are not vacuous** — `md5sum` on all three is pairwise
  distinct (`e059e94a`, `c2e304d2`, `cd79e418`) and all three are referenced from
  `TestGolden_FullscreenFrames`. No orphan, no identical pair.
- **dod-1's key oracles ride the real chain** — `TestFullscreen_EscExitsZoom`
  uses `updateAndRender` with an explicit comment that `dispatchModeKey` would
  bypass the reducer where the peel order lives. The `runKeyCases` hazard was
  anticipated and avoided; the opposite of the defect being hunted.
- **The zoom round-trip assertions have varying subjects** —
  `SelectedIndex() != 80`, `YOffset() <= 0`, `!HasSelection()` on a fixture
  scrolled to mid-history; all three can fail. Not vacuous.
- **B15 not re-reported**, per the brief.
- **`TestFuzzy_OutOfOrderQueryMatchesNothing`'s fixture is sound** — `rocket`,
  `cricket`, `roller_skate` all match `rkt`, none match `krt`, so the
  empty-result assertion genuinely fails if in-order semantics break. Checked
  for vacuity specifically.
- **`gofmt -l .` verified empty**, so that third of dod-4 holds. `go vet` and the
  `-race` suite were not run by the delegate (budget) — but both were run by me
  this session and are green, so dod-4's tooling clause is fully verified.
- **`fakeEmojiLister` is genuinely pre-existing** as dod-2 claims
  (`cmd/slk/customemoji_test.go:64`, with `callCount()` at :78). No finding.
- **dod-1 omits key suppression while zoomed** — under-described in the DOD,
  over-covered in tests (`…SuppressedKeysToastWhileZoomed:563`,
  `…SuppressedWorkspaceNumberWhileZoomed:649`, `zoom_chord_tab_test.go`). The
  harmless direction. **But see batch 4's B45** — those tests observe the
  statusbar *model*, and the toast is never rendered while zoomed, so their
  subject cannot vary. The two batches independently reached opposite conclusions
  about the same tests, and batch 4 is right: this one is reported there.
- **Whether `TestGolden_FullscreenFrames` uses `newGoldenApp`** — not verified;
  judged more likely handled than not, since `newGoldenApp` exists for exactly
  that reason.

## Cross-references

- B33 is the consumer half of **B30** (fixed): pinning tier constants by value
  does not prove emojipicker ranks by them.
- B33 also depends on **B24** (open, decision needed): "recent" cannot be proven
  in emojipicker until frecency is wired there.
- B42 and **OT6** are the same DOD example failing two different ways — ranking
  looseness here, custom-emoji eviction there. Fixing one does not fix the other.
- B34's picker-helper duplication sits inside the feature whose stated purpose
  was to unify those two packages behind one matcher; see **B26/B27** for the
  production-code version of the same drift.
- B35 and B41 are the mandated class; **B2** (resolved) and **B19** are earlier
  instances of the same "delete one, never alias" rule.
