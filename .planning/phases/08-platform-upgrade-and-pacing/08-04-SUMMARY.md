---
phase: 08-platform-upgrade-and-pacing
plan: 04
subsystem: testing
tags: [bash, smoke-test, jq, provenance, plat-05, d-13, d-14, d-15, d-16]

# Dependency graph
requires:
  - phase: 08-platform-upgrade-and-pacing
    provides: "08-01: azuredevops/v7 v7.1.0 and the go 1.27.2 directive; 08-02: org-bound transport on every sub-client; 08-03: config.Validate refuses a non-cloud org_url with exit 3"
provides:
  - "scripts/smoke.sh: fresh pinned-toolchain build into mktemp, go version -m provenance gate, eight --json reads, identity row from this run's log bytes, six dry runs with SKIP/NODIFF, printf-aligned table, totals line, exit 1 on any FAIL"
  - ".planning/phases/08-platform-upgrade-and-pacing/08-SMOKE.md: record template with provenance and output slots, D-16 sub-client map naming the dormant work client, Dry-run waivers (D-13) slot, five pending manual checks, CI row, split findings tables, Not covered"
affects: [08-08, phase-09]

# Actuals (#2632)
actuals:
  tokens: 4268
  tasks: 2
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Smoke rows are formatted once by printf '%-32s %-6s %s\\n' into a results file and printed as the final table; tool stderr for a FAIL is printed as an indented '-- NAME stderr (exit=N):' block so it never parses as a row"
    - "Every non-row line the script prints is shaped so the 08-08 record gate cannot read it as a row: provenance lines carry no result word, stderr blocks start with '-' or spaces, the totals line uses PASS=n"
    - "Identity is judged only on log bytes appended by this run (wc -c before, tail -c +N after, to a file so pipefail cannot hide a grep -q match)"

key-files:
  created:
    - scripts/smoke.sh
    - .planning/phases/08-platform-upgrade-and-pacing/08-SMOKE.md
  modified: []

key-decisions:
  - "Identity row is FAIL, not PASS, when the client was built but no log file exists: absence of the warning cannot be proven, and reporting PASS would be the lie Pitfall 11 forbids"
  - "The first provenance line is printed with the temp directory stripped (adoBizInt: go1.27.2), so the pasted record carries no machine-specific path"
  - "08-SMOKE.md never places azuredevops/v7 and v7.1.0 adjacent in prose, so 08-08's provenance grep cannot pass on the unfilled template"
  - "SKIP detail names the verb's target id first (set SMOKE_PR_ID before set SMOKE_PROJECT), so each SKIP row points at what that verb specifically lacks"

patterns-established:
  - "Plumbing test without network: a throwaway copy of the script swaps the provenance-checked binary for a stub emitting canned envelopes; never committed"

requirements-completed: [PLAT-05]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "smoke.sh builds a fresh binary with GOTOOLCHAIN pinned to the go.mod directive and stops with FAIL provenance before any ADO call unless go version -m shows that toolchain and azuredevops/v7 v7.1.0; it never runs a binary from PATH"
    requirement: "PLAT-05"
    verification:
      - kind: e2e
        ref: "Task 1 <verify> (isolated HOME, no credentials): rc=1, two provenance lines, workitems list FAIL exit=3; re-run on the committed tree as the tracer gate (TRACER_VERIFY_PASS)"
        status: pass
      - kind: other
        ref: "Tripwire copies (never committed): wrong SDK version and wrong toolchain both print FAIL provenance, exit 1, and the binary never ran (no log created)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Eight reads PASS only on exit 0 with complete true; identity row from this run's log bytes (SKIP client not built on exit 3); six dry-run rows named dry-run <noun> <verb>: PASS on exit 0 with outcome dry_run, NODIFF on exit 4, SKIP naming the missing SMOKE_* variable, else FAIL; totals line; exit 1 on any FAIL"
    requirement: "PLAT-05"
    verification:
      - kind: e2e
        ref: "Task 2 <verify> (sentinel ADO_PAT, no org_url, isolated HOME): rc=1, 8 FAIL exit=3 rows, 6 SKIP set SMOKE_ rows, identity SKIP client not built"
        status: pass
      - kind: other
        ref: "Stub plumbing test: PASS, FAIL (complete=false), throttled-by-ADO detail, PASS/NODIFF/FAIL/SKIP dry runs, identity PASS with an older warning in the log, identity FAIL when this run logs the warning; stub argv shows no confirm flag"
        status: pass
    human_judgment: false
  - id: D3
    description: "The script never prints the PAT and never passes the confirm flag (T-08-14, T-08-15)"
    requirement: "PLAT-05"
    verification:
      - kind: e2e
        ref: "Task 2 <verify>: smoke-sentinel-pat-7d1e absent from combined stdout/stderr and from the isolated adoBizInt.log"
        status: pass
      - kind: other
        ref: "grep -v '^[[:space:]]*#' scripts/smoke.sh | grep -c -- '--confirm' -> 0; set -[a-z]*x -> 0"
        status: pass
    human_judgment: false
  - id: D4
    description: "08-SMOKE.md template: provenance and script-output slots, D-16 map with the dormant work client named, Dry-run waivers (D-13) section with only 'No waivers recorded.', five pending manual checks, CI row, header-and-separator-only findings tables, Not covered; parses under the 08-08 record gate"
    requirement: "PLAT-05"
    verification:
      - kind: other
        ref: "Task 2 acceptance greps (pending=5, headings, waiver lines=0, table lines 2 and 2, no em dash, no non-ASCII)"
        status: pass
      - kind: other
        ref: "08-08 Task 3 gate awk run on a scratch filled copy: exit 0; with the unwaived SKIP's Blocks row removed: exit 1 naming it"
        status: pass
    human_judgment: false

# Metrics
duration: 10min
completed: 2026-10-20
status: complete
---

# Phase 8 Plan 04: Live Smoke Harness and Record Template Summary

**`scripts/smoke.sh` builds a fresh go1.27.2 binary, refuses to touch ADO unless `go version -m` proves azuredevops/v7 at v7.1.0, then runs the eight reads, an identity check on this run's log bytes and six confirm-free dry runs into one printf-aligned PASS/NODIFF/SKIP/FAIL table; `08-SMOKE.md` is the record 08-08 fills, including the user-only Dry-run waivers slot**

## Performance

- **Duration:** 10 min
- **Started:** 2026-10-20T16:06:18Z
- **Completed:** 2026-10-20T16:16:44Z
- **Tasks:** 2
- **Files modified:** 2 (both created)

## Accomplishments

- Provenance gate before any ADO call: `GOTOOLCHAIN=go<directive>` build into `mktemp -d`, then the first `go version -m` line must be exactly that toolchain and some line must link azuredevops/v7 at v7.1.0; both tripwires fail closed with the binary never run
- One read per sub-client with a call site (D-16): workitemtracking, git, build, release (vsrm host), plus an `identity` row for location that scans only the log bytes this run appended for "Could not cache current user ID"
- Six dry runs under predictable `dry-run <noun> <verb>` names; an unset target is SKIP naming the variable, exit 4 is NODIFF, nothing passes the confirm flag
- Output safety: no xtrace, no env/config/header echo; the sentinel PAT never appears in output or log; tool stderr appears only for a FAIL, indented so it never reads as a row
- 08-SMOKE.md template checked against the actual 08-08 record gate: a correctly filled copy passes, an unwaived SKIP is caught

## Task Commits

1. **Task 1: Smoke harness skeleton: fresh build, provenance gate, one read row, table and exit code** - `4c54151` (feat, tracer)
2. **Task 2: Full read and dry-run coverage with SKIP targets, identity check, and the 08-SMOKE.md record template** - `6a7f368` (feat)

**Plan metadata:** recorded in the `docs(08-04): complete live smoke harness plan` commit

## Files Created/Modified

- `scripts/smoke.sh` - executable, bash 3.2 (251 lines): header documents every SMOKE_* target and the waiver rule; `row`, `run_read`, `run_dry`, `show_stderr`, `throttle_note`
- `.planning/phases/08-platform-upgrade-and-pacing/08-SMOKE.md` - record template, frontmatter `status: pending`

## Verification Runs

| Run | Result |
|-----|--------|
| Task 1 verify (no credentials, isolated HOME) | rc=1, 2 provenance lines, `workitems list FAIL exit=3` |
| Tracer gate: Task 1 verify on the committed tree | TRACER_VERIFY_PASS |
| Tripwire: expected SDK version v7.1.9 | FAIL provenance, exit 1, no log created (binary never ran) |
| Tripwire: expected toolchain go1.99.0 | FAIL provenance, exit 1, no log created |
| Task 2 verify (sentinel PAT, no org_url) | rc=1, 8 read FAIL exit=3, 6 SKIP set SMOKE_, identity SKIP, sentinel absent from output and log |
| Stub plumbing scenario A | 11 PASS, 1 NODIFF, 1 SKIP (set SMOKE_BRANCH), 2 FAIL; throttle line in detail; old identity warning ignored |
| Stub plumbing scenario B | identity FAIL when this run logs the warning |
| 08-08 record gate on a filled scratch copy | exit 0; removing the unwaived SKIP's Blocks row gives exit 1 |
| `go build ./...`, `go vet ./...`, `go test ./...`, gofmt | all ok, no Go file touched |

No live ADO call was made: every run had `ADO_ORG_URL` unset and an isolated `HOME`, so startup stopped at config validation (exit 3) or the stub stood in for the binary.

## Decisions Made

- Identity row is FAIL when the client was built but no log file exists, because the warning's absence cannot be proven.
- The first provenance line drops the temp directory (`adoBizInt: go1.27.2`).
- The template never puts azuredevops/v7 and v7.1.0 side by side in prose, so 08-08's provenance grep needs the real pasted line.
- SKIP detail names the verb's target id before SMOKE_PROJECT.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] jq preflight**
- **Found during:** Task 1
- **Issue:** Without jq every row would read `complete=none` and fail with a misleading cause.
- **Fix:** One `command -v jq` check up front prints `FAIL jq` and exits 1.
- **Files modified:** scripts/smoke.sh
- **Committed in:** 4c54151

**2. [Rule 2 - Missing critical] Temp binary cleanup on interrupt**
- **Found during:** Task 1
- **Issue:** The EXIT trap alone may not run when the script is killed by a signal, leaving the built binary behind.
- **Fix:** `trap 'exit 130' INT` and `trap 'exit 143' TERM` route signals through the EXIT trap.
- **Files modified:** scripts/smoke.sh
- **Committed in:** 4c54151

**3. [Rule 2 - Missing critical] Identity with no log file**
- **Found during:** Task 2
- **Issue:** The plan defines PASS/FAIL by the warning's presence; with no log file the absence check would silently PASS.
- **Fix:** That case records FAIL with "no log file to check".
- **Files modified:** scripts/smoke.sh
- **Committed in:** 6a7f368

### Process

**4. [Tracer gate] No mid-flight human-verify halt after the tracer**
- Auto mode is not active, `human_verify_mode` is at its end-of-phase default, and the tracer verify is fully automated; it was re-run on the committed tree and passed. Same handling as 08-01 to 08-03.

**5. [Live smoke] Not run here**
- Per the user decision of 2026-10-20 and the plan's scope, this plan only authors the harness and template. The run against the real org is 08-08 (human checkpoint).

---

**Total deviations:** 3 auto-fixed (Rule 2), 2 process. **Impact:** small safety and honesty additions inside the plan's intent; no scope change.

## Known Stubs

None that block the goal. The `(not yet run)` fences, `pending` table cells, `result: [pending]` rows and `No waivers recorded.` in 08-SMOKE.md are the intended empty slots of the record template; 08-08 Task 3 fills them.

## Issues Encountered

- `PIPESTATUS` printed empty in one zsh-hosted test wrapper; the script's own exit code was confirmed through the totals line and the direct runs.
- Plan estimate was 45000 tokens; the realized files are about 4300 (chars/4).

## User Setup Required

None now. For 08-08: export the SMOKE_* targets listed in the header of `scripts/smoke.sh`, and build `/tmp/adoBizInt-v7` as 08-SMOKE.md's Manual checklist preamble says before the TUI checks.

## Next Phase Readiness

- 08-08 Task 1 can run the Task 2 verify command of this plan as its harness plumbing check.
- `throttled by ADO:` lines are already captured into row detail; they appear once 08-05/08-06 add the summary line.
- PLAT-05 is shared with later Phase 8 plans; the shared-ID gate decides when it is marked complete.

## Self-Check: PASSED

- FOUND: scripts/smoke.sh (mode 100755 in the index), .planning/phases/08-platform-upgrade-and-pacing/08-SMOKE.md
- FOUND commits: 4c54151, 6a7f368
- key_links: `go version -m` (3) and `\.write\.outcome` (2) in scripts/smoke.sh; 08-SMOKE.md contains "work"
- All Task 1 and Task 2 acceptance criteria re-run and pass on the committed tree

---
*Phase: 08-platform-upgrade-and-pacing*
*Completed: 2026-10-20*
