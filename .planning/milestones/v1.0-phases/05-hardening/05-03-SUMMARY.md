---
phase: 05-hardening
plan: 03
subsystem: infra
tags: [ci, github-actions, gofmt, go-vet, race-detector, supply-chain]

requires:
  - phase: 01
    provides: Fake SDK seam, so the whole suite runs with no ADO credentials
provides:
  - ".github/workflows/ci.yml: workflow `ci`, job `check`. Runs go build, go vet, go test -race and a gofmt -l gate on every push and pull_request"
affects: [05-VALIDATION HARD-05 row, /gsd-secure-phase T-05-10..T-05-13, /gsd-verify-work phase gate]

actuals:
  tokens: 203
  tasks: 2
  commits: 1
plan_head_before: 8cdd2a8d9bece2e973d57be62adb5f887321c940

tech-stack:
  added: [GitHub Actions (actions/checkout v7.0.1, actions/setup-go v7.0.0, both SHA-pinned)]
  patterns:
    - "Third-party actions pinned to 40-hex commit SHAs, with the release tag in a trailing comment"
    - "The workflow token is read-only (permissions: contents: read)"
    - "CI gets no credentials. A missing PAT is the signal that a test escaped the fake seam (D-17)"

key-files:
  created:
    - .github/workflows/ci.yml
  modified: []

key-decisions:
  - "The action SHAs were re-verified at execution with gh api (both tags point directly at commits), even though the RESEARCH validity date (2026-10-30) had not passed"
  - "Task 2 is proof-only and changed no file, so it has no commit. Both proofs passed on the first run"

patterns-established:
  - "CI gofmt gate lists offending files before exit 1, rather than a bare test -z"

requirements-completed: [HARD-05]

coverage:
  - id: D1
    description: "ci.yml is structurally correct: push and pull_request with no filter, contents: read, two SHA-pinned actions, go-version '1.25.4', all four gates present, and no secrets, ADO_* or token reference"
    requirement: HARD-05
    verification:
      - kind: other
        ref: "Ruby YAML structural check from 05-03-PLAN Task 1 <verify> -> 'ci.yml structure ok'"
        status: pass
    human_judgment: false
  - id: D2
    description: "The four gates pass locally on the current tree"
    requirement: HARD-05
    verification:
      - kind: other
        ref: "go build ./... && go vet ./... && go test -race ./... && test -z \"$(gofmt -l .)\""
        status: pass
    human_judgment: false
  - id: D3
    description: "The gofmt step's own run script exits 1 and names bad.go in a directory holding an unformatted file, and exits 0 at the repo root"
    requirement: HARD-05
    verification:
      - kind: other
        ref: "05-03-PLAN Task 2 <verify> #1 -> 'gofmt step: fails on bad.go, passes on repo'"
        status: pass
    human_judgment: false
  - id: D4
    description: "The full race suite passes with ADO_PAT, ADO_ORG_URL, ADO_PROJECTS and ADO_PROJECT unset and an empty HOME (D-17)"
    requirement: HARD-05
    verification:
      - kind: other
        ref: "05-03-PLAN Task 2 <verify> #2 (env -u ADO_* HOME=$(mktemp -d) go test -race -count=1 ./...) -> exit 0, zero FAIL lines"
        status: pass
    human_judgment: false
  - id: D5
    description: "PHASE-BLOCKING: a push of the phase branch triggers the ci workflow on GitHub and the run for HEAD is green. Optional: a throwaway branch with an unformatted .go file goes red at the gofmt step"
    requirement: HARD-05
    verification: []
    human_judgment: true
    rationale: "Only a real push proves the workflow triggers on GitHub, and pushing is the operator's decision. The executor does not push. 05-VALIDATION.md requires one green GitHub Actions run on the phase branch before Phase 5 is complete. Run this after all Phase 5 plans, including wave 2, have landed."

duration: 1min
completed: 2026-09-30
status: complete
---

# Phase 05 Plan 03: CI Workflow Summary

**This plan closes MED6. `.github/workflows/ci.yml` runs `go build`, `go vet`, `go test -race` and a gofmt gate that lists offending files, on every push and pull request. It uses a read-only token and SHA-pinned `actions/checkout` v7.0.1 and `actions/setup-go` v7.0.0. Go is pinned to `'1.25.4'` to match `go.mod`. There are no credentials and no skip guards.**

## Performance

- **Duration:** 1 min (timer started after the context reads)
- **Started:** 2026-09-30T17:12:31Z
- **Completed:** 2026-09-30T17:14:30Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments
- The repository has CI for the first time. The four ROADMAP SC5 gates run in a fixed order, and GitHub stops the job at the first failure, so a later gate never hides a build failure.
- Supply-chain and privilege rules hold. Both actions are pinned to commit SHAs, re-verified on GitHub at execution: checkout `3d3c42e5…` is v7.0.1 and setup-go `b7ad1dad…` is v7.0.0. The token scope is `contents: read`. The workflow has no `env:`, no `secrets` and no `ADO_*` reference.
- The failure direction is proven against the actual step text. The gofmt `run` script, taken from the parsed YAML, exits 1 in a directory holding a badly formatted `bad.go` and prints `gofmt needs to be run on:` / `bad.go`. At the repo root it exits 0.
- D-17 is confirmed. No test reads a real `ADO_PAT`. The only `ADO_*` reference in any `_test.go` file is `cmd/adoBizInt/usage_test.go:50-51` (`TestHelpNeedsNoCredentials`), and it blanks `ADO_PAT` and `ADO_ORG_URL`. The full race suite exits 0 with all four `ADO_*` variables unset and `HOME` set to an empty temp directory.

## Task Commits

1. **Task 1: A workflow that runs the four gates on push and pull request** - `091a866` (ci)
2. **Task 2: Prove the gates fail in the right direction and that the suite needs no credential** - no commit. The task is proof-only and found no defect, so it changed no file.

## Tracer Gate

Task 1 is a `tracer`. The run was interactive, `human_verify_mode` was `end-of-phase`, and the tracer's `<verify>` is automated-only. I re-ran the verify against the committed file, it passed (`ci.yml structure ok`, all four gates green), and I moved on to Task 2 without a checkpoint.

## Verification Results

| Check | Result |
|---|---|
| Ruby structural check (triggers, permissions, pins, go-version, gates, credential reference) | `ci.yml structure ok` |
| One job (`check`), six steps in the planned order | PASS: checkout, setup-go, build, vet, race test, gofmt |
| `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"` | exit 0 |
| gofmt step on an unformatted file, then at the repo root | `gofmt step: fails on bad.go, passes on repo` |
| Race suite with ADO_* unset and an empty HOME (`-count=1`) | exit 0, 0 `FAIL` lines |
| `git status --short` after the proofs | only the pre-existing untracked `.gsd/`, `.planning/milestone.lock` and `.planning/state.json`; no stray `bad.go` or temp directory |

## Files Created/Modified
- `.github/workflows/ci.yml`: workflow `ci`, job `check` (ubuntu-latest, 15-minute timeout). Two SHA-pinned actions and four gate steps.

## Decisions Made
- I re-verified both action SHAs with `gh api` before writing them, even though the RESEARCH validity date had not passed. The check is cheap and read-only, and it guards T-05-10.
- The YAML matches the RESEARCH example, except that the heading and the file list are printed with two separate `echo` lines rather than a single `;`-joined line. The behaviour is the same.

## Deviations from Plan

None. The plan was executed as written.

## Issues Encountered
- The local toolchain is Go 1.27.1, and CI uses 1.25.4 (D-16). Locally, the gates passed under 1.27.1. The first run on Linux under 1.25.4 happens only on GitHub (RESEARCH A4). If that run goes red, the failure is real signal.
- The interactive shell is zsh, so the first credential-free run could not capture its exit code through a pipe with bash `PIPESTATUS`. I re-ran the plan's exact command without the pipe, and it returned `RC=0`.

## User Setup Required

None. No secrets or repository settings are needed, because the workflow uses only the default read-only token.

## Pending Human Check (PHASE-BLOCKING)

- **What:** Push the phase branch (`dev`) to `origin` yourself. The executor does not push. Then open the repository's Actions tab and find the `ci` run for the pushed HEAD commit.
- **Expected:** The run is green, with build, vet, race test and gofmt all passing. Optionally, push a throwaway branch with a badly formatted `.go` file. That run should be red at the `gofmt` step, and its log should name the file. Delete the branch afterward.
- **When:** After all Phase 5 plans, including wave 2, have landed. Phase 5 is not complete until this check passes (05-VALIDATION.md phase gate).

## Next Phase Readiness
- The local half of the HARD-05 row in 05-VALIDATION.md is green. The GitHub half is the pending human check above.
- Plans 05-04 to 05-06 are not affected. Every later commit is now gated by CI once it is pushed.

---
*Phase: 05-hardening*
*Completed: 2026-09-30*

## Self-Check: PASSED
- FOUND: .github/workflows/ci.yml
- FOUND commit: 091a866
