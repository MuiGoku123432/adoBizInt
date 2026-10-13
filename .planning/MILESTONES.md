# Milestones

## v1.0 Agent CLI (Shipped: 2026-10-12)

**Delivered:** A non-interactive `noun verb` CLI with structured JSON output, so an AI agent can drive the same Azure DevOps operations as the TUI without mistaking a broken call for an empty result.

**Phases completed:** 6 phases (1-6), 33 plans, 85 tasks
**Timeline:** 2026-09-04 → 2026-10-12 (283 commits, 75 non-planning files, +17,828 / -492)
**Requirements:** 29/29 v1 requirements checked off

**Key accomplishments:**

- **Test seam for `internal/ado`** — narrow interfaces and fail-fast fakes; exact SDK payload tests cover all seven destructive writes with no network access.
- **Honest reads** — a shared `Result[T]` envelope and status-based failure classifier; every aggregate read and all six TUI views now distinguish a rejected token, a partial answer and a genuinely empty backlog.
- **Read CLI** — `noun verb` dispatch over eight read commands, single JSON document on stdout, 0/1/2/3 exit contract with auth-wins precedence, `--timeout` and signal cancellation.
- **Guarded writes** — six write verbs behind dry-run default with a real pre-read diff, `--confirm`, a pre-flight append-only JSONL audit record, and exit codes 4/5 for partial and unknown outcomes.
- **Hardening** — `--project` allow-list, WIQL `@project` parameterization, 0700/0600 config and log permissions, a PAT-never-logged test, and SHA-pinned CI (`build`, `vet`, `test -race`, gofmt).
- **Core-value cleanup** — help tokens recognized only in flag position, empty project config fails closed, every read discloses its server-side `scope`, and a panic after a sent write exits 5 with an `unknown` audit record.

**Closeout:** override_closeout

- Known verification overrides: 1 newly acknowledged, 0 carried forward from a prior close (see STATE.md Deferred Items)
- Stale verifications at close: Phases 2, 4 and 5 passed verification but predate later code changes (largely Phase 6). Phase 2 scored 5/6 with re-verification pending; Phase 5 SC5 awaits an observed GitHub Actions run.
- Milestone audit (`milestones/v1.0-MILESTONE-AUDIT.md`, status `tech_debt`) predates Phase 6, which closed its W1-W4 warnings.
- Remaining tech debt and follow-ups are tracked in `.planning/BACKLOG.md` (BL-01 TUI write path bypasses Phase 4 guards; BL-02 open review warnings).

---
