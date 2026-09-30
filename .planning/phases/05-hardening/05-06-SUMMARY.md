---
phase: 05-hardening
plan: 06
subsystem: logging
tags: [slog, file-permissions, chmod, credentials, regression-test]

requires:
  - phase: 05-hardening
    provides: "05-04 withConfiguredProjects helper and the fs.Func --project check (the write path in the PAT test needs --project P allowed)"
  - phase: 05-hardening
    provides: "05-01 deletion of Client.connection, so no dispatch-reachable value holds the PAT"
provides:
  - "logging.Init leaves ~/.config/adoBizInt 0700 and adoBizInt.log 0600 on every call, repairing existing 0755/0644 installs"
  - "internal/logging/logging_test.go (first test file in the package): TestInitSetsOwnerOnlyPermissions"
  - "cmd/adoBizInt/patleak_test.go: TestPATNeverLogged, a raw/Basic-header/base64 PAT tripwire across read, executed write and panic"
affects: [audit-file-mode-follow-up, logging, crash-handling]

actuals:
  tokens: 2110
  tasks: 2
  commits: 3
plan_head_before: 1e1b131fa128df9c4d27ee793e4a556988694d44

tech-stack:
  added: []
  patterns:
    - "Explicit chmod after MkdirAll/OpenFile whenever a mode matters for an existing path; file.Chmod on the open fd, not a path chmod"
    - "Log-capture tests must assert per-path presence markers so an absence check cannot pass vacuously"

key-files:
  created:
    - internal/logging/logging_test.go
    - cmd/adoBizInt/patleak_test.go
  modified:
    - internal/logging/logging.go

key-decisions:
  - "05-06: Init chmods the log dir to 0700 and the log to 0600 on every call (file.Chmod on the fd); a chmod failure closes the file and returns, and initLogging's existing non-fatal warning handles it (D-20, D-21)"
  - "05-06: TestPATNeverLogged reuses patFixture and drives an executed write (--confirm), because a dry run logs nothing; it is a tripwire, not a present-leak detector"
  - "05-06: audit.go still creates audit.jsonl 0644 (and its MkdirAll uses 0755); deferred as T-05-26 follow-up, mitigated by the 0700 directory"

patterns-established:
  - "Owner-only modes for anything under ~/.config/adoBizInt, enforced on every start rather than only at creation"

requirements-completed: [HARD-06]

coverage:
  - id: D1
    description: "Log directory 0700 and log file 0600 after Init on a fresh install, an existing 0755/0644 install, and a repeated Init"
    requirement: HARD-06
    verification:
      - kind: unit
        ref: "internal/logging/logging_test.go#TestInitSetsOwnerOnlyPermissions"
        status: pass
    human_judgment: false
  - id: D2
    description: "The PAT (raw, Basic header, base64) never appears in captured log output across a read, an executed write and a recovered panic, with per-path non-vacuity markers"
    requirement: HARD-06
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/patleak_test.go#TestPATNeverLogged"
        status: pass
    human_judgment: false

duration: 4min
completed: 2026-09-30
status: complete
---

# Phase 5 Plan 06: Log Permissions and PAT Test Summary

**`logging.Init` now tightens `~/.config/adoBizInt` to `0700` and `adoBizInt.log` to `0600` on every start. It uses an explicit chmod, so existing world-readable installs get repaired too, not just new ones. `TestPATNeverLogged` fails the suite if the fixture PAT ever reaches the log in raw, Basic-header or base64 form, on the read, write or panic path.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-09-30T17:35:38Z
- **Completed:** 2026-09-30T17:39:25Z
- **Tasks:** 2
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments
- Closes CONCERNS MED2's permission finding and Phase 3 threat T-03-15 (T-05-23). The directory also holds the PAT-bearing `config.yaml`, so the directory chmod does the most good. `MkdirAll` and `OpenFile` now pass `0o700` and `0o600`. `os.Chmod(logDir, 0o700)` and `file.Chmod(0o600)` run after them on every call, because modes passed at creation never change an existing path.
- Error handling follows D-21. If `file.Chmod` fails, the file is closed and the error returned. `cmd/adoBizInt/main.go`'s `initLogging` is unchanged and already treats this as a non-fatal stderr warning. The signature is still `func Init() error`.
- `internal/logging` now has its first test file. Its three subtests cover fresh, loose-existing and repeat `Init` (the repeat case stands in for two processes starting at once). Each runs under a `t.TempDir()` HOME and resets the package `logger` in `t.Cleanup`.
- `TestPATNeverLogged` (T-05-24) covers `prs list --json`, `workitems set-state ... --confirm` and a panic through `guard`, with `slog` captured and `loggingReady = true`. It searches for the three credential forms, and it checks that `Read complete`, `Write complete` and `panic recovered` appear, so an empty capture cannot pass.

## Task Commits

1. **Task 1: logging.Init leaves the directory 0700 and the log 0600 on every start (tracer)**: `df20b53` (test, RED) and `1cd6d24` (feat, GREEN)
2. **Task 2: The PAT never appears in captured log output across a read, a write and a panic**: `f11c2da` (test)

## TDD and Tracer Gates

- **Task 1 RED:** At `df20b53` all three subtests failed on the mode assertions: the directory was `0755` and the log `0644`. `gsd-tools check tdd-red-evidence` returned `RED_EVIDENCE_OK` (`target_test_failed`) on a TAP record built from `go test -json`.
- **Task 1 GREEN:** At `1cd6d24` all three subtests pass under `-race`.
- **Task 1 mutation:** I removed only the `os.Chmod(logDir, 0o700)` call. The "existing loose install" subtest then failed (dir `0755`, want `0700`) and the other two still passed. Then I restored the call.
- **Tracer gate:** Auto mode was off, `human_verify_mode` was end-of-phase, and the tracer's `<verify>` is automated-only. I re-ran it end to end against the commit: 3 subtest PASS lines, `go test -race ./cmd/adoBizInt/...` and `go vet ./...` green. Then I moved on to Task 2 with no checkpoint.
- **Task 2:** This is a tripwire with no production change, so the test passes as soon as it is written. That is expected: after 05-01 no dispatch path holds the PAT. The RED evidence comes from mutations instead:
  - I added `"pat", patFixture` to the `Read complete` call at `dispatch.go:307`. The test failed with `log output contains the raw form of the PAT`, and `check tdd-red-evidence` returned `RED_EVIDENCE_OK`. Restored.
  - I deleted the `loggingReady = true` line. The test failed with `capture is vacuous: no "panic recovered" line was logged`. Restored.
- REFACTOR: none needed.

## Verification Results

| Check | Result |
|---|---|
| `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"` | pass |
| `go test ./internal/logging ./cmd/adoBizInt -run 'TestInitSetsOwnerOnlyPermissions\|TestPATNeverLogged' -v` | both PASS, no SKIP on darwin |
| `TestLoggingInitFailureKeepsStdoutClean` | pass |
| `go doc ./internal/logging Init` | `func Init() error` |
| `cmd/adoBizInt/main.go` diff | empty |
| `patleak_test.go` declares no second PAT-like constant | confirmed (only `patFixture`) |
| Real `~/.config/adoBizInt` untouched by tests | confirmed: still `0755`, mtime Sep 12 |

## Files Created/Modified
- `internal/logging/logging.go`: owner-only modes at creation plus a chmod on every `Init`, with a doc comment explaining why (D-20, D-21).
- `internal/logging/logging_test.go`: `TestInitSetsOwnerOnlyPermissions` with three subtests, skipped on Windows.
- `cmd/adoBizInt/patleak_test.go`: `TestPATNeverLogged`.

## Decisions Made
- Followed the plan as written: reuse `patFixture`, test an executed write rather than a dry run, and chmod only `~/.config/adoBizInt`, never a shared parent.
- I did not update `05-VALIDATION.md`. Plans 05-01 to 05-05 did not touch it either, and `/gsd-validate-phase` owns it. Both HARD-06 rows (05-06 T1, T2) are green by the run above.

## Deviations from Plan

None. The plan executed as written.

## Deferred (follow-up, T-05-26)
- `cmd/adoBizInt/audit.go` still creates `audit.jsonl` with `0644`, and its `MkdirAll` still uses `0755`. D-18 keeps this out of scope. The risk is reduced: `Init` chmods the containing directory to `0700` before any write verb opens the audit sink. Two gaps remain:
  - The file's own mode is still `0644`.
  - If `Init` fails entirely, `openAuditSink` could create the directory as `0755`.

## Issues Encountered
None.

## User Setup Required
None. An existing install is repaired automatically the next time `adoBizInt` starts.

## Human-check items (end of phase)
- 05-03's human-check still applies at the phase gate: the pushed phase branch needs a green `ci` run on GitHub. The full suite is green locally.

## Next Phase Readiness
- This is the last plan in Phase 5 (6 of 6). The phase is ready for verification (`/gsd-verify-work 5`) and validation (`/gsd-validate-phase 5`).

## Self-Check: PASSED
- FOUND: internal/logging/logging.go, internal/logging/logging_test.go, cmd/adoBizInt/patleak_test.go
- FOUND commits: df20b53, 1cd6d24, f11c2da
- TDD gate commits present: `test(05-06)` then `feat(05-06)`

---
*Phase: 05-hardening*
*Completed: 2026-09-30*
