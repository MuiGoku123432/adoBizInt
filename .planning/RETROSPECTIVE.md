# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — Agent CLI

**Shipped:** 2026-10-12
**Phases:** 6 | **Plans:** 33 | **Sessions:** --

### What Was Built
- A network-free test seam for `internal/ado`, with exact SDK payload tests over all seven destructive writes
- A `Result[T]` envelope that makes auth failure, partial answers and empty results distinguishable in both the TUI and the CLI
- A `noun verb` CLI: eight read verbs, six guarded write verbs, JSON on stdout, a 0-5 exit-code contract
- Dry-run-by-default writes with `--confirm` and a pre-flight JSONL audit record
- Hardening: `--project` allow-list, WIQL `@project`, 0700/0600 permissions, PAT-never-logged test, SHA-pinned CI

### What Worked
- Ordering correctness before surface area. Honest reads (Phase 2) landed before any read verb (Phase 3), so no CLI verb ever shipped able to render an auth failure as an empty list
- Mutation-proven characterization tests in Phase 1. Each "swallows failure" test was shown to fail when the swallow was removed, which made the Phase 2 inversion mechanical
- go/ast structural tests pinning invariants (WIQL shape, nil guards, validate-before-dispatch ordering) so future code cannot quietly regress them
- The milestone audit. It found four core-value warnings (W1-W4) that every phase verification had passed, and Phase 6 closed them

### What Was Inefficient
- Phase 2 needed two gap-closure plans (02-06, 02-07) after first verification, including `branches list` always exiting 1 on empty repositories
- Phases 2, 4 and 5 ended the milestone with stale verifications because Phase 6 touched code they had verified; nothing re-verified them before close
- The audit ran before Phase 6, so the close relied on a pre-cleanup audit plus a 50/50 Phase 6 verification instead of one current audit
- Live-ADO confirmation of the write path and one GitHub Actions run (Phase 5 SC5) were deferred rather than scheduled

### Patterns Established
- Every read returns `Result[T]` with `complete`, `failures` and `scope`; a filtered or partial answer must say so
- Writes go through one `runWrite` body: pre-read, diff, `--confirm` gate, audit record before the request, outcome classification
- Fail-fast fakes that fail by interface and method name when an unexpected ADO call is made
- Fail closed on configuration: an empty or blank `projects` list is an error, not an empty answer

### Key Lessons
1. Verification per phase does not catch cross-phase contract drift. Run the milestone audit, then re-audit after any cleanup phase it spawns
2. "Empty" is an output that needs proof. Each new way to produce zero rows (no projects, no pipelines, a server-side filter) needs an explicit disclosure or a failure
3. Guard parity matters: hardening the CLI write path left the TUI write path (BL-01) as the weaker door into the same mutations
4. Schedule deferred live checks as a concrete first task of the next milestone, or they stay deferred

### Cost Observations
- Model mix: not tracked
- Sessions: not tracked
- Notable: 283 commits over 39 days; tests (about 13,200 lines) now outweigh production code (about 11,700 lines)

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | -- | 6 | Correctness-first ordering; milestone audit spawned a cleanup phase |

### Cumulative Quality

| Milestone | Tests | Coverage | Zero-Dep Additions |
|-----------|-------|----------|-------------------|
| v1.0 | 252 test functions | -- | 0 new dependencies |

### Top Lessons (Verified Across Milestones)

1. (Pending a second milestone)
