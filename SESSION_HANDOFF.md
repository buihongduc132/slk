# Session handoff — slkfz (fullscreen zoom / emoji cache / fuzzy matcher)

> **Superseded.** Current status lives in
> `flow/plans/slk-fullscreen-emoji-fuzzy.md`. This file is kept only for the
> operational map at the bottom, which is not recorded anywhere else.
>
> The work described here merged to `main`; the branch and commits this file used
> to cite (`slkfz/integration` @ `ebe42ad`) are history. Every item it listed as
> "ready to implement" is closed: B41 `73c1c76`, B27 `3a6d285`, B26 `9255c8b`,
> B33, B40 (by coverage), B47 (first half refuted), B49 `2727a6d` + `ab38b2a`,
> B34, with B50 and B15 settled as won't-fix and deferred.
>
> Two things this file got wrong, both of which propagated for a full session
> before anyone checked them:
>
> - It cited `app.go:931-945` for the G15 zoom contract. Those are bare comment
>   lines. The rule is `zoomFrontIsThread` at `internal/ui/app.go:1008`, contract
>   in its doc comment at `:980-1007`.
> - It described `## Open Threads` as items "1–26". That heading *is* the 65-item
>   working log, appended out of order (19–26 follow 37).
>
> Its capability-suite figure (25/25) is also unreachable: the count is
> state-dependent, 24 or 28, never 25.

## Procedural rule that still binds

**Never edit a hash-pinned oracle** (`~/.local/state/slkfz/gates/*/oracle.sha256`).
It happened once and is recorded as OT17. The gate's own error text names the
case: *"If a test is genuinely wrong, STOP and say so; do not edit it."* Add a new
unpinned file alongside — additive is sanctioned and needs no decision. **Never
re-pin a hash to match your own edit**; that disarms the check permanently and is
worse than either the weak test or the violation.

Open gap nobody has closed: there is no sanctioned channel for "this oracle is
too weak." The only compliant move leaves the weak test running forever.

## Where things live

```
~/.local/state/slkfz/
  verify-commit.sh          run every gate against ONE commit, from clean checkouts
  gates/<lane>/             gate.sh + oracle.sha256 + p2p.txt per lane (7 lanes)
  gates/p2p_skip.txt        tests excluded from every P2P run, with reasons
  deploy-slk-dev.sh         isolated slk-dev install, real creds, read-only from live
  capability-test.sh        live checks incl. B13 auto-clear and B45 toast
  agyralph-patched/         scratch fixes for the user's agyralph (NOT applied)
  salvage/                  the recovered batch-3 and batch-4 reports
  verify/                   per-commit gate logs
  lane-*-prompt.md          how each lane was briefed (the pattern for new lanes)
```

`slk-dev` runs via `slk-dev-run`; its XDG roots are
`~/.local/state/slk-dev-root/{config,data,cache}`. The live install at
`~/.config/slk` and `~/.local/share/slk` is **read only** — `cache.db` is copied
with `sqlite3 .backup` (WAL-safe against the live writer), never `cp`. Documented
caveat: that read-only connection can update the live file's *mtime* via a
checkpoint on close; contents verified unchanged (integrity `ok`, row counts
identical).

`verify-commit.sh` does **not** advance `~/.worktrees/slkfz-int`, which
`deploy-slk-dev.sh` defaults to. Check the source worktree's SHA before deploying
or you ship a tree no gate measured.
