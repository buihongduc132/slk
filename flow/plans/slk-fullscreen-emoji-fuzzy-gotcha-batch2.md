# Gotcha Coverage — slk-fullscreen-emoji-fuzzy (batch 2: fuzzy matcher + its 4 consumers)

> Source: `flow/plans/slk-fullscreen-emoji-fuzzy.md`
> Mode: plan
> Command: `cmd-pallet/gotcha-coverage`, batch 2 of 4
> Sub-agent: general-purpose (delegated; 8 of 10 tool calls used, 4m15s)
> Units reviewed: `(fuzzy-shared)`, `(fuzzy-emoji)`, `(fuzzy-inorder-subseq)`,
> `(fuzzy-rkt-verified)`, `(fuzzy-reactionpicker-migration)`
> Verification: all 7 findings independently re-checked against
> `slkfz/integration` @ `894c209`. **7 of 7 confirmed**, two with corrections
> recorded inline (B25's blast radius, B26's count — the latter is worse than
> reported, not better).

Numbering continues B1–B23. Per the command: per-batch appendix, never merged;
originals immutable; Rank 3+ raised in the plan's `## Open Threads`.

## Findings (ranked)

### Rank 4 (Significant — impacts correctness)

- **B24 `emojipicker` has no recent/frecent tier at all, so `fuzzy-emoji` is
  marked `[x]` with half of it unimplemented.** The item promises ranking
  "recent (frecent) > prefix > substring > subsequence" and that "compose
  autocomplete (`:` trigger) and reaction picker get the same ranking". Only the
  reaction picker got it.
  - Verified: `emojipicker.Model` has seven fields and none is frecent;
    `SetFrecentEmoji` is declared **only** on `reactionpicker`
    (`reactionpicker/model.go:147`); both call sites are
    `a.reactionPicker.SetFrecentEmoji(...)` (`app.go:1224`, `:1242`); the string
    `frecent` appears **0 times** in `emojipicker` against 5 in
    `reactionpicker`. emojipicker's comparator has three terms (tier,
    subsequence score, input index), reactionpicker's has four, led by the
    recent tier.
  - Why missed: the item bundles four independent claims behind one checkbox.
    Three hold in reactionpicker, so it reads as done. And the DOD's proof
    obligation (`:thumbs` → `+1` reachable) is satisfiable with no recent tier
    at all, so the criterion could not catch it.
  - Severity: the mandated class in its worst form — **the same ranking rule
    implemented twice, divergently, after a consolidation whose whole purpose
    was one implementation.** The two emoji surfaces answer the same query in
    different orders, and `MaxVisible = 5` puts the disagreement in the top
    rows rather than the tail.
  - Same shape as B13: a plan item marked `[x]` whose named proof cannot see the
    missing half. **Not fixed here** — wiring frecent into emojipicker changes
    ranking users see, so it wants its own RED test and commit.
  - Mitigation: move the *comparator* into `internal/fuzzy`, not just the
    scorer — a `Candidate{Tier, Score, Rank, Idx}` plus `fuzzy.Less` — delete
    emojipicker's copy, and wire `SetFrecentEmoji` from the same two `app.go`
    sites. Until then the "compose autocomplete gets the same ranking" clause
    should read `[ ]`.

- **B25 `channelfinder` re-derives tier numbers with `int(tier) - 1` against a
  3-tier assumption, and the enum has 5 — with unreviewed model reasoning left
  in the production file.** `channelfinder/model.go:423-429`:
  ```go
  matches = append(matches, match{idx: i, tier: int(tier) - 1, score: score})
  // wait, Match returns TierPrefix (1), TierSubstring (2), TierSubsequence (3).
  // channelfinder originally used 0 for prefix, 1 for substring, 2 for subsequence.
  // so tier - 1 is perfect.
  ```
  The enum is `TierNone 0, TierPrefix 1, TierWordPrefix 2, TierSquashedPrefix 3,
  TierSubstring 4, TierSubsequence 5`. The comment's parenthetical numbers are
  wrong, and the local `match` struct still documents `tier int // 0 prefix,
  1 substring, 2 subsequence` (`model.go:416`).
  - **Correction to the finding, which the delegate got right and is worth
    keeping explicit: nothing is live-broken.** `int(tier) - 1` is monotone in
    tier and the sort only compares those values, so ordering is preserved. The
    defects are (a) a false comment, (b) a false struct-field doc, (c) tier
    identity living in two numberings, so reordering the enum silently corrupts
    channelfinder's local scheme with nothing failing — which is exactly what
    B30 leaves unpinned.
  - The separate, real behaviour question the finding raises: growing the tier
    set means names that previously landed in substring/subsequence can now land
    in `TierWordPrefix`/`TierSquashedPrefix` and outrank true substring hits,
    and squashed-style hits that used to arrive as subsequence matches with a
    *positive* score now carry score 0 (`Match` returns 0 for every tier except
    `TierSubsequence`), collapsing that sort step to a tie. Plausible and
    untested; needs a test per boundary rather than an assertion here.
  - Why missed: `fuzzy-shared` promised consumer "`_test.go` expectations
    unchanged" and read green tests as proof the contract held. **Unchanged
    expectations prove only that nothing pinned changed.** The plan also never
    enumerated the merged tier set — `fuzzy-reactionpicker-migration` says
    "4 tiers" where there are five plus `TierNone`, so the two tiers that cause
    the reordering are absent from the plan's own model of its change.

- **B26 `fuzzy.Match` re-folds strings that are already folded, reinstating the
  hot-loop cost issue #165 removed.** Verified by counting whole function
  bodies: `WordPrefix` contains **2** `text.Fold` calls and `SquashedPrefix`
  **2**, while `Match` folds both arguments and then passes `foldedName,
  foldedQuery` into them. `SubsequenceScore` correctly contains 0.
  - **The count is worse than reported.** Per candidate on the deepest path the
    name is folded 3× and the query 3× — and all three consumers already fold
    the query once outside the loop (`emojipicker/model.go:138`,
    `reactionpicker/model.go:245`, `channelfinder/model.go:397`) and pass the
    folded value in, so that outer fold is immediately undone by a re-fold per
    candidate. The delegate said "up to four times… name up to three"; it is 3
    and 3 inside `Match` plus the wasted outer one.
  - **Not a correctness bug**: folding is idempotent, so every path returns the
    same answer. This is purely cost — which is why no test can see it.
  - `internal/text/fold.go` documents why this shape is wrong, in its own
    comment: *"`Fold` is called once per candidate inside the filter loop of ten
    pickers … reaction picker 271 ms / 1.12 GB / 768k allocs -> 2.96 ms / 611 B
    / 1 alloc. See issue #165."* The ASCII fast path keeps the common case
    cheap; non-ASCII candidates (accented custom emoji, display names like
    `Mélanie`) take the allocating NFD/NFC chain on every repeat, per keystroke.
  - Why missed: `fuzzy-shared` specifies the API by the tiers it exposes and
    never by where folding happens. No item or DOD clause mentions cost, and
    `e2e-suite-green` gates on whole-suite `-race` wall time, where a picker
    filter is invisible.
  - Mitigation: split the API — keep `Match(name, query)` for one-shot use, add
    a `Matcher` built once per query that folds it exactly once, and make
    `Match` call unexported no-fold variants. Then delete either the
    caller-side fold or the internal one, per AGENTS.md: delete one, never keep
    both. Guard with a benchmark over `BuildEntries(nil)` and a non-ASCII query.

### Rank 3 (Moderate — needs explicit handling)

- **B27 "What is a word boundary" has four answers, three of them inside
  `internal/fuzzy`.** Verified verbatim:

  | Site | Separator set |
  |---|---|
  | `fuzzy.isSeparator` (feeds `SubsequenceScore`) | `-` `_` `.` space `/` `:` |
  | `fuzzy.WordPrefix` | `" "` `"-"` `"_"` `"."` |
  | `fuzzy.SquashedPrefix` (`FieldsFunc`) | space `-` `_` `.` |
  | `mentionpicker.isSeparator` (`match.go:28`) | space `-` `_` `.`, byte-level |

  So `/` and `:` are word boundaries for subsequence scoring **and nothing
  else** — four definitions, two distinct sets. For emoji names, which lean on
  `-` and `_` and whose trigger character is `:`, the tier a candidate lands in
  depends on which of three definitions that tier's predicate happens to use.
  - Why missed: the plan derived the API from two sources and merged their
    *predicates* verbatim rather than their *notion of a separator*. "Extract
    the substrate, not the widget" was applied to the scoring functions and not
    to the one value underneath all of them.
  - Mitigation: one exported `fuzzy.IsSeparator(rune)`, all four sites routed
    through it, `mentionpicker.isSeparator` deleted. Decide `/` and `:`
    deliberately and write the decision as a test, not a comment.

- **B28 `mentionpicker.squashedQuery` is a dead parameter with live tests, and
  `rankSquashed`'s semantics changed underneath it.** Verified:
  `match.go:48-49` says "squashedQuery is ignored now" while the parameter
  survives on `matchName` and `rankUser` (`:68-70`), `model.go:155` still
  computes `sq := squash(q)`, and `squash`/`isSeparator` remain declared with
  3 live test references in `match_test.go`.
  - The substantive half: old `rankSquashed` compared a fully squashed name
    against a fully squashed **query**; `fuzzy.SquashedPrefix` compares a
    *selectively* squashed name (its short-word rule re-inserts `-` when either
    adjacent word is under 3 runes) against the **unsquashed** query. So
    `matchName("a-bc-def", "abcdef")` matched before and cannot now — a
    different predicate under the same name.
  - Why missed: same as B25 — "expectations unchanged" was read as a verified
    property. It held because no mentionpicker test exercised a short-word
    boundary, the only input class where the two predicates disagree.
  - This is B2's and B19's class verbatim: one idea in two spellings, tests
    pinning the dead copy.

- **B29 Both candidate pools sort on RAW `Name` while every comparison
  downstream runs folded, and emojipicker's input-order precondition is prose.**
  Verified: `internal/emoji/entries.go:68` is
  `sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })`,
  and `emojipicker/model.go:134-136` reads "Callers must pass
  alphabetically-sorted entries (`emoji.BuildEntries` already does); the picker
  preserves that order" with no assertion anywhere.
  - Consequence: a custom emoji named `Rocket` sorts before every lowercase
    name (`R` < `a`), so it wins every within-tier tie against `rocket`. Slack
    lowercases on upload, but these rows now arrive from the `custom_emoji`
    cache table and nothing in that write path normalizes case.
  - AGENTS.md names the second half exactly: a comment standing in for a check.
  - Mitigation: sort on `text.Fold(Name)` with raw `Name` as the final
    tie-break in both pools, normalize on the cache write path, add a
    mixed-case custom-emoji fixture, and replace the prose precondition with a
    test.
  - Useful negative result from the same probe, worth recording so nobody
    re-checks it: **Go map iteration order does not reach the output.** Both
    pools sort by unique `Name` after their map walk, both dedup first, the
    within-tier tie-break is a unique `idx`, and `frecentRank` is lookup-only
    with that property deliberately documented
    (`reactionpicker/model.go:249-251`). Clean, and clean on purpose.

- **B30 The tier-order test pins 3 of the 5 real tiers, so reordering the two
  new constants passes the entire suite.** `internal/fuzzy/fuzzy_test.go:19-24`
  asserts only
  `TierNone < TierPrefix && TierPrefix < TierSubstring && TierSubstring < TierSubsequence`.
  `TierWordPrefix` and `TierSquashedPrefix` are absent, so swapping them — or
  moving either across `TierSubstring` — keeps this test and every consumer test
  green while silently reranking channelfinder and reactionpicker. Compounded by
  B25: because channelfinder stores `int(tier) - 1`, a constant reorder also
  corrupts its local numbering with nothing failing.
  - Why missed: the plan's own count is wrong (four tiers claimed, five plus
    `TierNone` implemented), so **the two unpinned constants are the two the
    plan does not know exist.**
  - Mitigation: assert the full chain in one expression, add `Tier.String()`,
    table-test all six constants by name and value.

### Rank 2 (Minor)

- **B31 Frecent entries absent from `allEmoji` land at `idx = len(allEmoji)+j`**
  (`reactionpicker/model.go:290-296`), so a frecent-but-unknown emoji loses
  every within-tier tie to every `allEmoji` entry. Harmless while the recent
  tier dominates the comparator; becomes a permanent last-place bucket if that
  tier is ever made score-aware.

## Categories that yielded nothing

- **Go map iteration order reaching the result** — see B29's note. All three map
  walks are sorted by a unique key afterward or lookup-only.
- **`os.Getenv` in >1 non-test file** across this feature's surface
  (`internal/fuzzy`, the four pickers, `internal/emoji`) — none.
- **A tier/cap constant duplicated across packages** — `MaxVisible` is 5 in
  `emojipicker`, 5 in `channelpicker`, 7 in `mentionpicker`, and
  `emojipicker/model.go:19` explicitly declares independence ("Independent of
  mentionpicker.MaxVisible"). Genuinely separate values, correctly annotated.
  **Not** a finding — recorded so the next pass does not re-raise it.

## Cross-references

- B24 is B13's class: a plan item marked `[x]` whose named proof cannot see the
  unimplemented half.
- B25, B28 and B30 compound: the plan undercounts its own tier set, one consumer
  re-derives tier numbers from the old count, another carries a dead parameter
  from the old predicate, and the test pins only the tiers the old count knew
  about.
- B26 is the counterpart to B21's lesson in the performance register: a closed,
  measured, documented fix regressed by a refactor that never named it as an
  invariant, and no test can see it because it is cost rather than behaviour.
- B24, B27, B28 are all the AGENTS.md headline defect — the same logic under two
  spellings — inside the package created to end exactly that.
