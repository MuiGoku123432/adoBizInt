---
phase: 08-platform-upgrade-and-pacing
plan: 09
subsystem: infra
tags: [go-toolchain, govulncheck, GO-2026-6809, azuredevops-v7, go-ast, retry, testing]

# Dependency graph
requires:
  - phase: 08-platform-upgrade-and-pacing
    provides: "08-01 go directive pin and CI govulncheck gate; 08-01 SC2 proof test; 08-07 retry decorators and TestRetryNeverWrapsWrites"
provides:
  - "go.mod go directive at 1.27.3 (lowest govulncheck-clean 1.27 patch), no toolchain line, go.sum unchanged"
  - "TestV7AuthFailureExitsAuth repeatable under -count=N via a per-invocation org (WR-05)"
  - "TestRetryNeverWrapsWrites two-sided read/write classification with readMethods (WR-04)"
affects: [08-10, 08-11, 08-12, phase-09]

# Actuals (#2632)
actuals:
  tokens: 1959
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-invocation org suffix (package-level atomic.Int64) for tests that assert the SDK's OPTIONS discovery request"
    - "Two-sided method classification: every narrow-interface method listed in exactly one of readMethods or writeMethods"

key-files:
  created: []
  modified:
    - go.mod
    - .planning/PROJECT.md
    - cmd/adoBizInt/v7auth_test.go
    - internal/ado/retry_test.go

key-decisions:
  - "08-09: go directive pinned at 1.27.3, the lowest govulncheck-clean 1.27 patch (GO-2026-6809 crypto/x509 closed); set with go mod edit -go, no toolchain line, go.sum unchanged; CI and smoke.sh follow go.mod with no edit"
  - "08-09: TestV7AuthFailureExitsAuth org is unique per invocation (name plus v7AuthSeq atomic counter), not per name; the exact OPTIONS-then-data-route assertion stays"
  - "08-09: TestRetryNeverWrapsWrites requires every interface method in exactly one of readMethods (16) or writeMethods (5); an unclassified method fails with text naming both lists and never an override; missing-override errors fire only for classified reads"

patterns-established:
  - "Per-invocation unique org: tests asserting the SDK OPTIONS request append an atomic sequence to the org so the process-global location cache cannot answer"
  - "Unclassified-first guard: a structural test refuses to infer read-vs-write from absence in a list"

requirements-completed: []

coverage:
  - id: D1
    description: "go directive raised to 1.27.3, govulncheck v1.9.0 clean locally on that toolchain, full gate and Phase 7 locks green"
    requirement: "PLAT-02"
    verification:
      - kind: other
        ref: "go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./... -> No vulnerabilities found. (go1.27.3 darwin/arm64)"
        status: pass
      - kind: other
        ref: "Task 1 verify block: directive regex, go version, no toolchain line, go mod tidy -diff, build, vet, test -race, gofmt, locks 7/1, PROJECT.md grep"
        status: pass
    human_judgment: false
  - id: D2
    description: "SC1 CI half: the pushed dev branch shows the govulncheck step green on go1.27.3"
    requirement: "PLAT-02"
    verification: []
    human_judgment: true
    rationale: "Requires the user to push dev and paste the CI run; executor never pushes (user decision 2026-10-21). Recorded in 08-12."
  - id: D3
    description: "SC2 exit-code proof repeatable: TestV7AuthFailureExitsAuth passes 12/12 under -count=3 with the exact OPTIONS assertion and both exitAuth checks intact"
    requirement: "PLAT-02"
    verification:
      - kind: unit
        ref: "go test -count=3 -run TestV7AuthFailureExitsAuth -v ./cmd/adoBizInt"
        status: pass
      - kind: unit
        ref: "go test -count=2 ./..."
        status: pass
    human_judgment: false
  - id: D4
    description: "TestRetryNeverWrapsWrites classifies every narrow-interface method both ways; unclassified methods fail without suggesting an override"
    requirement: "PLAT-03"
    verification:
      - kind: unit
        ref: "go test -count=1 -run 'TestRetryNeverWrapsWrites|TestRetryWritePassesThroughOnce|TestRetryRead$' -v ./internal/ado"
        status: pass
      - kind: other
        ref: "go test -overlay mutation probes: dropped write, dropped read, method in both lists, stale read entry each fail with the intended message only"
        status: pass
    human_judgment: false

# Metrics
duration: 10min
completed: 2026-10-21
status: complete
---

# Phase 8 Plan 09: Toolchain Bump and Proof-Test Hardening Summary

**go directive raised to 1.27.3 so govulncheck v1.9.0 is clean again (GO-2026-6809 closed), the SC2 v7 auth exit proof made repeatable under -count=N, and the never-retry-a-write structural test made two-sided so it can never steer a developer toward retrying a write**

## Performance

- **Duration:** 10 min
- **Started:** 2026-10-21T19:54:39Z
- **Completed:** 2026-10-21T20:04:57Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments

- Pinned patch: `go 1.27.3` (set with `go mod edit -go=1.27.3`; no toolchain line; `go mod tidy -diff` exit 0; go.sum untouched). GOTOOLCHAIN=auto downloaded go1.27.3 from the proxy; `go version` reports `go1.27.3 darwin/arm64`.
- govulncheck output line: `No vulnerabilities found.` (`go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...`, exit 0). 1.27.3 was clean on the first run, so no higher patch was needed.
- `go test -count=2 ./...`: exit 0, every package ok. `go test -race ./...`, `go build`, `go vet`, `gofmt -l .` (empty) all green on go1.27.3. Lock checks: 08-cli-tests.sha256 7/7 OK, 07-cli-audit-test.sha256 1/1 OK.
- ci.yml (`go-version-file: go.mod`) and scripts/smoke.sh (`GOTOOLCHAIN=go${want_go}` from the go directive) pick up 1.27.3 with no edit. PROJECT.md Tech stack names Go 1.27.3.
- TestV7AuthFailureExitsAuth: `-count=3` gives 12/12 subtest passes (was 0/4 on the second iteration). The org is now the subtest name plus `v7AuthSeq.Add(1)`.
- TestRetryNeverWrapsWrites: `readMethods` (16) beside `writeMethods` (5). Every interface method must be in exactly one list. Stale-entry check covers both lists. A missing-override error fires only for classified reads.

## Task Commits

1. **Task 1: Raise the go directive to the govulncheck-clean 1.27 patch** - `b9b3cfe` (build)
2. **Task 2: Make the SC2 exit-code proof repeatable (WR-05)** - `c5ff75a` (test)
3. **Task 3: Classify every narrow-interface method both ways (WR-04)** - `fea5a9c` (test)

**Plan metadata:** recorded in the docs(08-09) commits that follow this file.

## Files Created/Modified

- `go.mod` - go directive 1.27.2 -> 1.27.3, nothing else
- `.planning/PROJECT.md` - Tech stack line: Go 1.27.3
- `cmd/adoBizInt/v7auth_test.go` - `v7AuthSeq atomic.Int64`, appended to each subtest org; comment rewritten (unique per invocation, not per name)
- `internal/ado/retry_test.go` - `readMethods`, two-sided classification loop, stale check over both lists, missing-override only for reads, doc comments updated

## Decisions Made

- 1.27.3 is the lowest clean 1.27 patch; it is pinned, not a later one (08-CONTEXT "Toolchain pin").
- The SC2 fix keeps the exact OPTIONS-then-data-route assertion (review option 1) rather than asserting only the last request (option 2), so the proof that the 401 comes from the data route is unchanged.
- Unclassified-method message: `<iface>.<method> is unclassified: add it to readMethods or writeMethods in retry_test.go before deciding whether it may retry; a write must never be retried (D-03)`. It contains no "override".

## Deviations from Plan

None - plan executed exactly as written.

Tracer gate note: auto mode is off, but `workflow.human_verify_mode` is unset (default end-of-phase, #3309) and the tracer's verify is fully automated. The Task 1 verify block was re-run end to end after the commit and passed, then Tasks 2 and 3 ran. No mid-flight human checkpoint was raised.

## Issues Encountered

- `internal/ado/retry_test.go` is now 407 lines (352 before). It was already over the 300-line convention, and the plan places `readMethods` in this file. Not split here; out of scope.
- Mutation probes for Task 3 used `go test -overlay` on temporary copies in the session temp directory. No repository file was changed by the probes, and the copies were deleted.

## Known Stubs

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Ready for 08-10, which runs next on the same tree. No Go or module changes are left uncommitted.
- SC1 CI half is still open. The user pushes dev and pastes the run in 08-12. PLAT-02 stays Pending in REQUIREMENTS.md until then.
- The 08-SMOKE.md checklist build line moves to go1.27.3 in 08-12.

## Self-Check: PASSED

- FOUND: go.mod, .planning/PROJECT.md, cmd/adoBizInt/v7auth_test.go, internal/ado/retry_test.go
- FOUND commits: b9b3cfe, c5ff75a, fea5a9c
- Plan verification re-run on the final tree: build, vet, `go test -count=2 ./...`, `go test -race ./...`, gofmt, govulncheck (No vulnerabilities found.), `go mod tidy -diff`, locks 7/7 and 1/1 all pass

---
*Phase: 08-platform-upgrade-and-pacing*
*Completed: 2026-10-21*
