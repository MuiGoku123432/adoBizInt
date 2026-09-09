---
phase: 03-read-cli
plan: 01
subsystem: cli
tags: [go, cobra-free, stdlib-flag, json, exit-codes, tdd]

requires:
  - phase: 02-read-correctness
    provides: ado.Result[T] envelope, Failure, FailureKind, classified per-project failures
provides:
  - "run() int owning every exit path in package main"
  - "readClient consumer-side interface as the CLI test seam"
  - "noun-verb dispatch table with prs list registered"
  - "exit-code mapper with auth-wins precedence (0/1/2/3)"
  - "injectable stdout/stderr writers in cmd/adoBizInt"
  - "newResult pre-allocates Items and Failures so both marshal as []"
affects: [03-02, 03-03, 03-04, 03-05]

actuals:
  tokens: 9000
  tasks: 3
  commits: 4

tech-stack:
  added: []
  patterns:
    - "single json.Marshal + single os.Stdout.Write, never a streaming encoder"
    - "consumer-side unexported interface in package main as the CLI test seam"
    - "package-level writer variables swapped by tests to assert stdout purity"

key-files:
  created:
    - cmd/adoBizInt/dispatch.go
    - cmd/adoBizInt/exit.go
    - cmd/adoBizInt/main_test.go
    - cmd/adoBizInt/fakes_test.go
  modified:
    - cmd/adoBizInt/main.go
    - internal/ado/result.go
    - internal/ado/json_test.go

key-decisions:
  - "The dispatch table is keyed on the two-token `noun verb` string and stores one closure per command over an unexported readClient interface declared in package main (D-P1, D-P2)"
  - "The exit mapper scans for FailureAuth before consulting the completeness flag, so a co-occurring timeout can never mask a credential rejection (D-07)"
  - "Startup failures (config load, validate, client construction) return 3 rather than 1 — the read never ran, so its outcome is unknown rather than incomplete (D-08)"
  - "newResult pre-allocates both slices; failures was nil on every successful read, so \"failures\":null was the happy-path output"
  - "An unrecognised token or a noun with no verb prints usage to stderr and returns 3, never falling through to the TUI"
  - "Task 1 shipped a provisional complete-only exit mapper inside dispatch.go so the tracer could build; Task 2's RED test failed against it and Task 3's GREEN moved the real mapper to exit.go"
  - "A minimal tab-separated plain-text fallback was added for prs list without --json so the shipped verb is not broken between this plan and 03-03"

patterns-established:
  - "Decision-referencing comments: state the behavior and the rejected alternative with a D-number, mirroring internal/ado/branches.go"
  - "Fake methods fail by interface and method name when their closure is unset, mirroring internal/ado/branches_test.go"
  - "Diagnostics use fmt.Fprintf into the injected stderrWriter, never fmt.Print or os.Stdout"

requirements-completed: [CLI-01, CLI-02, OUT-01, OUT-03, OUT-04]

coverage:
  - id: D1
    description: "`adoBizInt prs list --json` writes exactly one newline-terminated JSON document to stdout and nothing else"
    requirement: OUT-01
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestDispatchEmptyReadEmitsEmptyArrays"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestLoggingInitFailureKeepsStdoutClean"
        status: pass
    human_judgment: false
  - id: D2
    description: "A read whose failures include any ado.FailureAuth exits 2 regardless of what else failed"
    requirement: OUT-04
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestExitCodeFor/auth_among_three_timeouts_wins"
        status: pass
    human_judgment: false
  - id: D3
    description: "A clean read exits 0, an incomplete read with no auth failure exits 1, and startup failures exit 3"
    requirement: OUT-04
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestExitCodeFor"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestDispatchUsageErrors"
        status: pass
    human_judgment: false
  - id: D4
    description: "An empty read emits {\"items\":[],\"failures\":[],\"complete\":true} — never null for either key"
    requirement: OUT-01
    verification:
      - kind: unit
        ref: "internal/ado/json_test.go#TestNewResultEmitsEmptyArraysNotNull"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestDispatchEmptyReadEmitsEmptyArrays"
        status: pass
    human_judgment: false
  - id: D5
    description: "Two invocations of the same read verb against the same fixture emit byte-identical stdout"
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestDispatchIsDeterministic"
        status: pass
    human_judgment: false
  - id: D6
    description: "main() contains exactly one statement, os.Exit(run()), so no path skips a deferred function"
    verification:
      - kind: other
        ref: "grep -c 'os\\.Exit' cmd/adoBizInt/main.go == 1"
        status: pass
    human_judgment: false
  - id: D7
    description: "`adoBizInt` with zero arguments still launches the TUI with no behavior change (CLI-01)"
    verification: []
    human_judgment: true
    rationale: "The TUI path requires a live PAT, a real Azure DevOps organisation and an interactive terminal. No automated test in this repo constructs a bubbletea program against a real client; the branch is verified by inspection here and belongs in the phase UAT."

duration: 34 min
completed: 2026-09-09
status: complete
---

# Phase 3 Plan 01: Read CLI Tracer Slice Summary

**One production path from `os.Args` through a `noun verb` dispatch table, into the Phase 2 `Result[T]` envelope as a single JSON write on stdout, out through a 0/1/2/3 exit contract with auth-wins precedence.**

## Performance

- **Duration:** 34 min
- **Tasks:** 3
- **Files modified:** 7 (4 created, 3 modified)

## Accomplishments

- `main()` is now exactly `os.Exit(run())`. All four former `os.Exit(1)` sites became returns out of `run() int`, so the Plan 03-05 panic handler can be a `defer` inside it rather than dead code.
- Startup failures now return 3 instead of 1 (D-08): a config, validation or client-construction failure means the read never ran, which is a different fact from an incomplete read.
- `cmd/adoBizInt/dispatch.go` introduces the `readClient` consumer-side seam, the `noun verb` dispatch table, and injectable `stdoutWriter`/`stderrWriter` — the three things every later plan in this phase builds on.
- `prs list --json` emits the raw envelope with one `json.Marshal` and one `Write`, so a mid-marshal panic cannot leave a partial document ahead of the 03-05 crash record.
- The exit mapper scans for `FailureAuth` before the completeness flag. A caller that retries a 1 eventually succeeds; a caller that retries a 2 loops forever against a rejected credential, so auth must never be masked.
- `newResult` now pre-allocates both slices. `failures` was nil on *every* successful read, so `"failures":null` was the happy-path output and `.failures | length` errored on the ordinary success case.
- First test file in `cmd/` in the repo's history, including a process-level `os.Stdout` capture that discharges RESEARCH assumption A1 (`slog.Default()` writes to stderr) instead of leaving it assumed.

## Task Commits

1. **Task 1 (tracer): end-to-end `prs list --json`** — `6f3ab5b` (feat)
2. **Task 2 RED: failing exit-code table** — `9d099fb` (test)
3. **Task 2 GREEN: exit mapper with auth precedence** — `bbd3f8a` (feat)
4. **Task 3: stdout purity, empty-read and determinism tests** — `03f4b66` (test)

## Files Created/Modified

- `cmd/adoBizInt/main.go` — `main()` reduced to the single exit call; `run() int` owns every path; `initLogging()` extracted so tests can drive the init-failure branch
- `cmd/adoBizInt/dispatch.go` — exit-code constants, injectable writers, `readClient`, the dispatch table, `cmdPRsList`, `emitJSON`, `emitPlain`, `printUsage`
- `cmd/adoBizInt/exit.go` — `exitCodeFor`, the auth-precedence mapper
- `cmd/adoBizInt/main_test.go` — exit-code table, empty-read, determinism, usage-error and logging-init-failure tests
- `cmd/adoBizInt/fakes_test.go` — `fakeReadClient` with per-call closure fields
- `internal/ado/result.go` — `newResult` pre-allocates `Items` and `Failures`
- `internal/ado/json_test.go` — `TestNewResultEmitsEmptyArraysNotNull` regression assertion

## Decisions Made

Beyond the plan's D-P1..D-P4, three judgment calls were made during execution:

- **The provisional mapper.** Task 1's `dispatch.go` needed an exit code before Task 2's TDD cycle could run. Rather than inline the final logic (which would have made Task 2's RED phase impossible), Task 1 shipped a deliberately incomplete complete-only mapper. Task 2's table failed against it on exactly the four auth rows, which is the RED phase doing its job.
- **A plain-text fallback for `prs list` without `--json`.** The plan scopes the renderer to 03-03, but shipping a verb that produces nothing without a flag is a broken binary between plans. A five-line tab-separated fallback was added through the same single-write discipline, commented as 03-03's replacement target.
- **`fmt.Fprintln` was replaced with `fmt.Fprintf`** throughout `cmd/`. The plan's stdout-purity acceptance grep (`fmt\.Print\|println(`) matches `Fprintln(` on its `println(` alternation. Rather than weaken the guard, the call sites were changed — the guard is the contract that keeps OUT-03 held deliberately rather than by accident.

## Deviations from Plan

None — plan executed as written. The three items above are execution-level judgment calls within the plan's stated scope, not departures from it.

## Issues Encountered

None. All three tasks' acceptance criteria passed on the first verification run after implementation.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

Ready for 03-02, which widens `readClient` to all eight reads and registers the remaining seven `noun verb` entries. The seam, the exit contract, the writer injection and the fake convention are all in place and pinned by tests.

**Carried to phase UAT:** `adoBizInt` with zero arguments launching the TUI unchanged (coverage entry D7) requires a live PAT and an interactive terminal, so it is verified by a human rather than by a test.

## Self-Check: PASSED

- All four created files exist on disk.
- All four commit hashes resolve in `git log`.
- `go build ./... && go vet ./... && go test ./...` all green.
- `grep -c 'os\.Exit' cmd/adoBizInt/main.go` returns 1.

---
*Phase: 03-read-cli*
*Completed: 2026-09-09*
