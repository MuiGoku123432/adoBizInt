---
phase: 08-platform-upgrade-and-pacing
plan: 08
subsystem: testing
tags: [live-smoke, plat-05, plat-02, azuredevops-v7, waiver, d-13, d-15, d-16]

# Dependency graph
requires:
  - phase: 08-platform-upgrade-and-pacing
    provides: "08-04: scripts/smoke.sh and the 08-SMOKE.md record template; 08-01 to 08-07: v7.1.0 SDK, go1.27.2, govulncheck CI gate, org-bound transport, pacer, fail-fast, read retries, throttle summary"
provides:
  - "08-SMOKE.md closed (status complete, Blocks Phase 9 empty) on TUI-only live evidence from a v7.1.0 / go1.27.2 build, with smoke.sh, checklist 1, 2, 3, 5 and CI WAIVED by the user and quoted verbatim"
  - "BACKLOG.md BL-02: deferred live checks marked still open, TF401019 branch-stats WARN noise recorded as pre-existing, every waived 08-08 item carried open"
  - "WINDOWS.md entries 2 and 3 (unrun live smoke, unconfirmed CI) recorded and marked waived with the user's words"
affects: [phase-09, phase-08-verification, phase-07-verification]

# Actuals (#2632)
actuals:
  tokens: 8100
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "A waived live check is recorded as 'not run, waived by user' with the user's verbatim words, never as PASS and never with executor-drafted per-row waiver lines"

key-files:
  created:
    - .planning/phases/08-platform-upgrade-and-pacing/08-08-SUMMARY.md
  modified:
    - .planning/phases/08-platform-upgrade-and-pacing/08-SMOKE.md
    - .planning/BACKLOG.md
    - .planning/WINDOWS.md

key-decisions:
  - "08-08: the user overrode the Task 2 Phase 7 precondition on 2026-10-21 (verbatim: \"lets just move on\"); 07-VERIFICATION.md is still status human_needed with 9 pending UAT rows"
  - "08-08: SC5 (live smoke) and SC1's CI half were WAIVED by the user, not met; 08-SMOKE.md status is complete only because Blocks Phase 9 is empty once the blanket waiver covers every unrun row"
  - "08-08: PLAT-05 and PLAT-02 stay Pending in REQUIREMENTS.md: the plan accepts only per-row user waivers for dry-run SKIPs and has no waiver path for the CI check"
  - "08-08: the TF401019 branch-stats WARN for UQUAL and uqual-service-df is pre-existing D-06b behaviour (seen under b5 since 2026-08-17), non-blocking, carried in BL-02"

patterns-established:
  - "Evidence for a waived smoke comes from local, read-only sources only: go version -m on the binaries that ran, counts and message names from adoBizInt.log, audit.jsonl mtime"

requirements-completed: []  # Plan declares [PLAT-05, PLAT-02]; neither is met. SC5 and SC1's CI half were waived, not met. Both stay Pending (see Requirements).

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "08-SMOKE.md filled with real evidence only: go version -m provenance of both binaries that ran (azuredevops/v7 v7.1.0, go1.27.2), TUI live-read evidence, all five checklist slots resolved, CI row, the user's waiver quoted verbatim, findings classified, status complete with Blocks Phase 9 empty"
    requirement: "PLAT-05"
    verification:
      - kind: other
        ref: "grep -c 'result: \\[pending\\]' 08-SMOKE.md -> 0; provenance grep azuredevops/v7[[:space:]]+v7\\.1\\.0 matches; ^status: (complete|blocked) matches; no Basic token, no credential header line; dormant present; 0 non-ASCII; 0 '- waiver:' lines"
        status: pass
      - kind: other
        ref: "08-08 Task 3 awk record gate on 08-SMOKE.md: exit 1, 'missing read or identity row' x9 and 'expected 9 read and identity rows and 6 distinct dry-run rows, found , , 0' (no script output exists); status/Blocks consistency check passes"
        status: fail
    human_judgment: true
    rationale: "The record gate cannot pass without scripts/smoke.sh output, which the user waived; the verifier must decide whether the blanket waiver is acceptable for SC5"
  - id: D2
    description: "One live read per sub-client with a call site (D-16) on the v7 build: workitemtracking, git, build, release (vsrm) and location identity all succeeded in the user's TUI sessions with failure_count=0, 0 ERROR lines, 0 host-rule refusals"
    requirement: "PLAT-05"
    verification:
      - kind: manual_procedural
        ref: "~/.config/adoBizInt/adoBizInt.log lines dated 2026-10-21: Dashboard counts fetched, Work items fetched, Pull requests fetched, Branches fetched, Pipeline runs fetched, Pipeline definitions fetched, Releases fetched, Pending approvals fetched all failure_count=0; 0 'Could not cache current user ID'"
        status: pass
    human_judgment: true
    rationale: "Evidence comes from the user's TUI sessions, not the smoke.sh CLI rows; the CLI read path (exit codes, --json envelope, complete flag) was not exercised live"
  - id: D3
    description: "A dry run of each of the six write verbs against the real org (D-13)"
    requirement: "PLAT-05"
    verification: []
    human_judgment: true
    rationale: "Not run; waived by the user on 2026-10-21 (\"lets just move on\")"
  - id: D4
    description: "Live scratch-item writes and TUI checks: CLI set-state with the confirm flag, TUI two-PATCH Task close (IN-01), TUI attachment download (A3), confirm-modal dismissals"
    requirement: "PLAT-05"
    verification: []
    human_judgment: true
    rationale: "Not run; waived by the user. audit.jsonl unchanged since 2026-09-12, so no write ran"
  - id: D5
    description: "govulncheck step green in CI on a pushed dev branch (PLAT-02 SC1)"
    requirement: "PLAT-02"
    verification:
      - kind: other
        ref: "Local go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./... in 08-08 Task 1 (bf5ad85): No vulnerabilities found"
        status: pass
    human_judgment: true
    rationale: "The CI half was not run (dev not pushed, 120 commits ahead of origin/dev); waived by the user"
  - id: D6
    description: "BACKLOG.md BL-02 updated: deferred live checks still open, TF401019 branch-stats WARN recorded as pre-existing, waived 08-08 items carried open with the user's words"
    verification:
      - kind: other
        ref: "grep -c 'TF401019' .planning/BACKLOG.md >= 1; grep -c 'lets just move on' .planning/BACKLOG.md >= 1; grep -c 'Still open after 08-SMOKE.md' .planning/BACKLOG.md = 1"
        status: pass
    human_judgment: false

# Metrics
duration: 12min
completed: 2026-10-21
status: complete
---

# Phase 8 Plan 08: Live Smoke Record Summary

**08-SMOKE.md is closed on TUI-only live evidence from a v7.1.0 / go1.27.2 build (every sub-client read succeeded, no refusals, no errors); scripts/smoke.sh, all six dry runs, every live write, the attachment and modal checks, and CI on push were WAIVED by the user, not met, so PLAT-05 and PLAT-02 stay Pending**

## Performance

- **Duration:** 12 min for this continuation (2026-10-21T16:42:56Z to about 16:55Z); the plan spans 2026-10-20T17:03Z (Task 1) to 2026-10-21 across the human checkpoint
- **Started:** 2026-10-20T17:03:00Z
- **Completed:** 2026-10-21
- **Tasks:** 3 (Task 1 pre-flight; Task 2 checkpoint resolved by a partial run and a user waiver; Task 3 record)
- **Files modified:** 3 (08-SMOKE.md, BACKLOG.md, WINDOWS.md), plus this SUMMARY

## Outcome in one table

| Item | Result |
|------|--------|
| Binary that ran | `./adoBizInt` and a `go run` build, both go1.27.2 with azuredevops/v7 v7.1.0 (`go version -m`, executor-run) |
| Live reads per sub-client (D-16) | Done through the TUI, not smoke.sh: workitemtracking, git, build, release (vsrm), location identity all failure_count=0 |
| scripts/smoke.sh (8 CLI reads, identity, 6 dry runs) | Not run. WAIVED |
| Checklist 1 (CLI set-state with confirm) | Not run. WAIVED |
| Checklist 2 (TUI two-PATCH close, IN-01) | Not run. WAIVED |
| Checklist 3 (TUI attachment, A3) | Not run. WAIVED |
| Checklist 4 (TUI views) | Pass (log evidence plus the user's report) |
| Checklist 5 (confirm-modal dismissals) | Not run. WAIVED |
| CI govulncheck on pushed dev (SC1) | Not run (dev not pushed). WAIVED. Local govulncheck clean |
| Regressions from v7, pacer or host rule | None observed in what ran |
| 08-SMOKE.md status | `complete`; Blocks Phase 9 table empty |
| SC5 | WAIVED, not met |
| PLAT-05, PLAT-02 | Pending |

The user's words, verbatim and in order (2026-10-21): "I ran the tui it seems to be wokring?  https://dev.azure.com/Windstream", "I ran the new one so verify it looks good", "lets just move on".

## Accomplishments

- Verified, read-only, that both binaries the user ran link azuredevops/v7 v7.1.0 with go1.27.2, and that the old `~/go/bin/adoBizInt` (go1.26.5, b5) was not used.
- Pulled live-read evidence from the 167 log lines dated 2026-10-21: both sessions passed config, identity, resource-area discovery and the D-10a every-area host check; session 2 read dashboard (7,885 work items, 12 PRs, 61 pipelines), work items, pull requests, branches, pipeline runs and definitions, releases and pending approvals, all failure_count=0; 0 ERROR, 0 refusals, 0 VS403 or api-version rejections, 0 pacing or retry messages, 0 identity-cache warnings.
- Filled 08-SMOKE.md with that evidence only: an Outcome section stating plainly that SC5 and SC1's CI half were waived, not met, and listing what remains unproven; the user's waiver quoted verbatim; every unrun item recorded as "not run, waived by user".
- Classified the one finding: 4 WARN "Failed to get branch stats" (TF401019, repositories UQUAL and uqual-service-df, project Process) is pre-existing D-06b behaviour, logged the same way under b5 since 2026-08-17 (44 lines before v7, 4 on v7). Not a regression; recorded in BL-02.
- Inferred from the same lines that v7's WrappedError still carries the status and typeKey the D-06b skip matches (the warnings did not become failures).
- Carried every waived item into BL-02 and into the broken-windows ledger (entries 2 and 3, status waived with the user's words).

## Task Commits

1. **Task 1: Pre-flight (full suite, govulncheck, lock hashes, harness plumbing)** - `bf5ad85` (docs: start live smoke record)
2. **Task 2: User live smoke run (checkpoint)** - no commit; resolved by the user's partial TUI run and waiver. Orchestrator state note `988d95d`
3. **Task 3: Record results, classify findings, update BACKLOG** - `4765d81` (docs: record live smoke results)

**Plan metadata:** the `docs(08-08): add live smoke plan summary` commit (this file and WINDOWS.md), then a separate state and roadmap commit.

## Files Created/Modified

- `.planning/phases/08-platform-upgrade-and-pacing/08-SMOKE.md` - the filled record: Outcome, provenance, live evidence, D-16 table, verbatim waiver, checklist results, CI row, findings, status complete
- `.planning/BACKLOG.md` - BL-02: deferred live checks annotated as still open; two new bullets (TF401019 WARN noise; waived 08-08 items)
- `.planning/WINDOWS.md` - entries 2 (unrun live smoke) and 3 (unconfirmed CI), both waived with the user's words
- `.planning/phases/08-platform-upgrade-and-pacing/08-08-SUMMARY.md` - this summary

## Verification

| Check | Result |
|-------|--------|
| `go build ./...` | pass |
| `go test ./...` | pass (all packages ok) |
| Phase 7 lock hashes | 7/7 and 1/1 OK (no Go file touched in this plan) |
| `grep -c 'result: \[pending\]' 08-SMOKE.md` | 0 (pass) |
| Provenance grep `azuredevops/v7[[:space:]]+v7\.1\.0` | pass (real `go version -m` line) |
| `^status: (complete\|blocked)` | pass (`complete`) |
| No Basic token, no credential header line | pass |
| `dormant` present | pass (1) |
| No `- waiver:` lines written by the executor | pass (0) |
| Task 3 awk record gate | **FAIL**: 9 x "missing read or identity row" and "expected 9 read and identity rows and 6 distinct dry-run rows, found , , 0". There is no script output to parse. The status and Blocks Phase 9 consistency check inside the gate passes |
| CI run on pushed dev (Task 2 acceptance) | not run (waived) |

## Decisions Made

- Applied the user's blanket waiver (message 3) to every row that did not run, as relayed by the orchestrator, and quoted it verbatim rather than drafting per-row waiver lines in the user's voice.
- Set 08-SMOKE.md `status: complete` under the plan's own rule (complete exactly when Blocks Phase 9 is empty) and added two frontmatter keys, `sc5_live_smoke` and `sc1_ci_on_push`, both "waived by user, not met", so the status cannot be read as SC5 passed.
- Left PLAT-05 and PLAT-02 Pending (see Requirements).
- Classified the TF401019 warnings as pre-existing and non-blocking, with dated evidence that they predate v7.

## Requirements

- **PLAT-05: Pending.** SC5 needs one live read per sub-client and one dry run per write verb against the real org. The reads are evidenced through the TUI. The dry runs did not run. The plan closes a missing dry run only with a per-row user waiver naming the row; the blanket waiver is not in that form, so by the plan's own rules the dry-run half of SC5 is unmet.
- **PLAT-02: Pending.** SC1's CI half (govulncheck green on a pushed run) was not observed, and the plan has no waiver path for it. The rest of PLAT-02 (v7 swap, toolchain, x/sys, real-v7 401 test) was delivered and verified in 08-01.
- `requirements.mark-complete` was not run for either ID.

## SC5 exceptions (D-15)

None. The Pre-existing table holds no FAIL row of the script: its one row is a log-level issue on a branches read that succeeded. SC5 is not met because of the waiver, not because of a D-15 exception.

## Deviations from Plan

### User decisions

**1. Phase 7 precondition overridden**
- **Found during:** Task 2
- **Issue:** Task 2's precondition requires 07-VERIFICATION.md `status: passed`; it is still `human_needed` (9 pending UAT rows).
- **Resolution:** The user's message 3, "lets just move on", recorded as the override. No Phase 7 artifact was touched. Recorded as a STATE.md decision.

**2. Live smoke replaced by a partial TUI run plus a blanket waiver**
- **Found during:** Task 2
- **Issue:** The resume signal required a CI run URL, the full smoke.sh output, per-row waivers for any SKIP, and one line per checklist item. The user ran only the TUI (twice) and then waived the rest.
- **Resolution:** Recorded what ran from local, read-only evidence; recorded everything else as "not run, waived by user". No smoke.sh output, provenance line or waiver line was fabricated.

### Plan rules that could not be met

**3. Task 3 record gate fails on row counts**
- **Issue:** The awk gate requires 9 read and identity rows and 6 dry-run rows inside `## Script output`. There are none, because the script did not run.
- **Resolution:** Not fixed and not fabricated. Every other Task 3 check passes. Logged in WINDOWS.md entry 2 (waived by the user).

**4. Status under a strict reading**
- **Issue:** The plan treats a dry run as closed only by a user waiver naming that exact row. Without the blanket waiver, all six dry runs would be listed under Blocks Phase 9 and the status would be `blocked`.
- **Resolution:** The status is `complete` because the orchestrator relayed the user's explicit decision to waive every unrun row. The verifier should know this is the one point where the record depends on the blanket waiver rather than the plan's per-row form.

**5. Provenance from the executor, not from smoke.sh**
- **Issue:** The Binary provenance slot expects smoke.sh's two `provenance:` lines.
- **Resolution:** Filled with `go version -m` lines the executor ran on the two binaries that actually ran, labelled as such, with the go run temp directory elided.

**6. BACKLOG strike-throughs not applied**
- **Issue:** Task 3 step 5 strikes through deferred checks that passed live.
- **Resolution:** None passed live (none ran), so nothing was struck through; the bullet is annotated as still open instead.

### Evidence corrections

**7. Occurrence count**
- The orchestrator's prompt cited 1,202 occurrences of the branch-stats warning. The executor's own count of the full log: 1,245 "Failed to get branch stats" lines, 1,200 of them TF401019, 48 of those for UQUAL and uqual-service-df (44 before v7, 4 on 2026-10-21). The record uses the executor's counts.

---

**Total deviations:** 2 user decisions, 4 plan rules not met (recorded, not worked around), 1 evidence correction. **Impact:** Phase 9 is not blocked by this record, but SC5 and SC1's CI half are unmet and both requirements stay Pending.

## Issues Encountered

- `go run`-built binaries carry no vcs stamp, so session 2's build revision is inferred (no code commit after 988d95d; working tree has no modified Go files).
- The log cannot show which TUI tabs were opened on screen; session 2's reads all land in its first 22 seconds, consistent with startup loads.

## Known Stubs

None in code. 08-SMOKE.md's `## Script output` fence holds a "not run" note instead of script output; that is the waived gap, recorded in WINDOWS.md entry 2.

## Threat Flags

None. No code changed. T-08-30: the record was scanned for a credential header line, a Basic-scheme token and config content before commit; none present. The org URL appears in the record; it was already in STATE.md.

## User Setup Required

None.

## Next Phase Readiness

- Phase 8 has all 8 summaries; it is ready for phase verification, which should treat SC5 and SC1's CI half as waived, not met.
- Phase 7 still needs `/gsd-verify-work 7` (9 pending UAT rows) to reach `status: passed`.
- To retire the waivers: push dev and confirm the CI govulncheck step; run `scripts/smoke.sh` with the SMOKE_* targets and the 08-SMOKE.md checklist 1, 2, 3 and 5, then rerun the 08-08 Task 3 record gate. Best done before Phase 9 adds its first write verb, since no write has gone over the v7 wire yet.
- Phase 9 planning is not blocked by 08-SMOKE.md.

## Self-Check: PASSED

- FOUND: .planning/phases/08-platform-upgrade-and-pacing/08-SMOKE.md, .planning/BACKLOG.md, .planning/WINDOWS.md, this SUMMARY
- FOUND commits: bf5ad85, 988d95d, 4765d81
- Every claim in this summary matches the files on disk. The summary does not claim the live smoke ran; the one failing plan check (the Task 3 record gate) is reported above, not hidden, and is a direct consequence of the user's waiver.

---
*Phase: 08-platform-upgrade-and-pacing*
*Completed: 2026-10-21*
