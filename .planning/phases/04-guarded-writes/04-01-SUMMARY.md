---
phase: 04-guarded-writes
plan: 01
subsystem: cli
tags: [go, cli, write-safety, audit, jsonl, exit-codes, dry-run]

requires:
  - phase: 01-test-seam-for-internal-ado
    provides: unexported -API interfaces and struct-literal fake injection, used by every guard test
  - phase: 02-honest-reads
    provides: Result[T], FailureKind, classifyFailure/statusCodeOf, fetchctx budgets
  - phase: 03-read-cli
    provides: noun-verb dispatch table, newFlagSet single registration site, exit codes 0-3, emitJSON, crashEnvelope
provides:
  - "`workitems set-state` end to end: dry-run default, --confirm, pre-read diff, pre-flight audit, outcome classification"
  - "exitNoOp=4 and exitOutcomeUnknown=5 on the published exit contract"
  - "writeEnvelope / writeRecord / writeTarget / fieldChange — the `write` top-level JSON key"
  - "auditRecord + append-only JSONL sink at ~/.config/adoBizInt/audit.jsonl"
  - "writeExitCodeFor — write outcome mapper beside exitCodeFor"
  - "writeClient seam + combined client interface; writeSupportByCommand capability map"
  - "ado.StatusCodeOf and ado.ClassifyFailure exported; nullable effort fields on WorkItemDetail"
affects: [04-02, 04-03, 04-04, 05-hardening]

actuals:
  tokens: 16500
  tasks: 3
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Write envelope nested under a single `write` key, mirroring crashEnvelope's `error` key"
    - "Pre-flight audit sink open before the mutating call (D-13) — position is the guarantee"
    - "Capability map (writeSupportByCommand) keyed on the same `noun verb` string as the dispatch table"
    - "Nil-closure fake fields as the no-mutation assertion, in place of call counters"

key-files:
  created:
    - cmd/adoBizInt/write.go
    - cmd/adoBizInt/audit.go
    - cmd/adoBizInt/write_test.go
    - cmd/adoBizInt/audit_test.go
  modified:
    - cmd/adoBizInt/dispatch.go
    - cmd/adoBizInt/flags.go
    - cmd/adoBizInt/exit.go
    - cmd/adoBizInt/fakes_test.go
    - internal/ado/workitems.go
    - internal/ado/result.go

key-decisions:
  - "Ratified P-01: exitNoOp=4, exitOutcomeUnknown=5 — ascending severity, distinct from 0/1/2/3"
  - "Ratified P-02: verb names workitems set-state, prs approve, prs complete, releases approve, releases reject, pipelines queue — one spelling each"
  - "Ratified the write envelope schema: {write:{command,target,changes,executed,outcome}} with outcome in dry_run|no_op|executed|failed|unknown"
  - "Ratified the audit record contract: exactly timestamp, command, target, changes, outcome — one JSON object per line, newline-terminated, single Write"
  - "Exported ado.ClassifyFailure alongside StatusCodeOf so the write mapper keys on FailureKind rather than duplicating the cancellation-is-not-a-timeout rule"
  - "Added hoistLeadingPositionals: stdlib flag stops at the first non-flag argument, so the documented target-first spelling needed argument reordering before Parse"
  - "fakeReadClient embeds *fakeWriteClient so one fake satisfies the combined client interface without churning 52 existing dispatch call sites"

patterns-established:
  - "D-13 ordering: openAuditSink is called before the ADO mutation, and the ordering is pinned by a test that sets the mutating closure and asserts it is never invoked"
  - "Write outcome vocabulary lives in exit.go beside the exit codes it maps to, as named constants rather than string literals at call sites"
  - "Drift guards updated rather than bypassed: TestDispatchTableMatchesD02, TestFilterFlagsRegisteredOnSupportedVerbsOnly and TestJSONFieldNames all fired on this change and each expectation table was widened deliberately"

requirements-completed: [CLI-03, SAFE-01, SAFE-02, SAFE-04]

coverage:
  - id: D1
    description: "Dry run is the default: `workitems set-state` without --confirm reports the diff, exits 0, and makes no mutating ADO call"
    requirement: SAFE-01
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestDryRunIssuesNoMutatingCall"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestDryRunEnvelopeReportsFieldLevelDiff"
        status: pass
    human_judgment: false
  - id: D2
    description: "A failed or empty pre-read is fatal: nothing on stdout, non-zero exit, no mutation"
    requirement: SAFE-01
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestFailedPreReadEmitsNoDryRunReport"
        status: pass
    human_judgment: false
  - id: D3
    description: "A change already applied is refused with exit 4, outcome no_op and changes:[] on both the dry-run and --confirm paths"
    requirement: SAFE-01
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestRefusedNoOpExitsNoOpCodeOnBothPaths"
        status: pass
    human_judgment: false
  - id: D4
    description: "Write outcomes classify into five distinct outcome strings mapped to five exit codes, with timeout/5xx/no-status routing to unknown rather than failed"
    requirement: SAFE-04
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestWriteOutcomeClassification"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestExitCodesArePairwiseDistinct"
        status: pass
    human_judgment: false
  - id: D5
    description: "A mid-write interrupt reports exit 5 with a write envelope, never exit 3 and never a crash record"
    requirement: SAFE-04
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestInterruptedWriteReportsOutcomeUnknownNotNothingRan"
        status: pass
    human_judgment: false
  - id: D6
    description: "The audit sink is opened for append before the ADO call; an unwritable sink prevents the mutation entirely"
    requirement: SAFE-02
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/audit_test.go#TestUnwritableAuditSinkPreventsMutation"
        status: pass
    human_judgment: false
  - id: D7
    description: "Executed and outcome-unknown writes each append exactly one five-key JSONL record; dry runs and refused no-ops append none"
    requirement: SAFE-02
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/audit_test.go#TestExecutedWriteAppendsAuditRecord"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/audit_test.go#TestOutcomeUnknownWriteIsAudited"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/audit_test.go#TestDryRunAndNoOpAppendNoAuditRecord"
        status: pass
    human_judgment: false
  - id: D8
    description: "The audit record carries no argv, actor, url, headers or token at any level"
    requirement: SAFE-02
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/audit_test.go#TestAuditRecordOmitsArgvAndCredentialFields"
        status: pass
    human_judgment: false
  - id: D9
    description: "--confirm is registered on exactly the verbs writeSupportByCommand names, and on no read verb"
    requirement: CLI-03
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestConfirmRegisteredOnEveryWriteVerbAndNoReadVerb"
        status: pass
    human_judgment: false
  - id: D10
    description: "Usage errors — missing/duplicate target, zero or two --project values, missing --state — exit 3 with no client call"
    requirement: CLI-03
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestWriteParseErrorsExitUnknown"
        status: pass
    human_judgment: false
  - id: D11
    description: "The write path behaves correctly against a live Azure DevOps organisation — a real 409, a real permission denial, a real network timeout"
    verification: []
    human_judgment: true
    rationale: "Every guarantee above is proven against the Phase 1 fake seam with no socket opened, which is what ROADMAP SC6 requires. Whether the SDK surfaces a real ADO 409 and a real mid-flight timeout in the shapes the mapper keys on is an integration fact no in-process fake can establish. 04-04's human-verify step covers it."

duration: 42 min
completed: 2026-09-12
status: complete
---

# Phase 4 Plan 01: Guarded-write tracer Summary

**`workitems set-state` end to end — dry-run default with a real pre-read diff, `--confirm` gate, pre-flight append-only JSONL audit sink, five-outcome classification across two new exit codes, all pinned by fourteen seam tests that open no sockets.**

## Performance

- **Duration:** 42 min
- **Tasks:** 3 (1 ratified checkpoint, 1 tracer, 1 test plan)
- **Files modified:** 15 (4 created, 11 modified)

## Accomplishments

- One production-quality vertical slice proves the whole guarded-write architecture before any of the other six mutations exist. The dry-run/confirm/audit/outcome contract that 04-03 expands onto is now real code with a green test suite, not a design.
- The two load-bearing orderings are enforced structurally rather than by comment. `openAuditSink` runs before the ADO call, and `TestUnwritableAuditSinkPreventsMutation` was confirmed to FAIL when the open is moved below the mutate — verified by actually moving it, running the test red, and reverting.
- The refused no-op precedes the `--confirm` branch, so D-03's idempotency-by-refusal holds on both paths. This is the only thing in the phase that touches CONCERNS.md C2's idempotency half.
- Outcome-unknown is genuinely distinct from "nothing ran" at every layer: exit 5 not 3, `outcome:"unknown"` not `"failed"`, a write envelope on stdout not a crash record, and an audit line rather than silence.
- Three pre-existing drift guards fired on this change (`TestDispatchTableMatchesD02`, `TestFilterFlagsRegisteredOnSupportedVerbsOnly`, `TestJSONFieldNames/WorkItemDetail`). Each was widened deliberately with the new surface rather than relaxed — they did exactly the job they were written for.

## Task Commits

1. **Task 1: Ratify the one-way published-contract decisions** — no commit (checkpoint). User selected `ratify`; P-01, P-02, the envelope schema and the audit record contract were adopted verbatim with no amendments and implemented as ratified in Task 2.
2. **Task 2: End-to-end guarded write for `workitems set-state`** — `f5638d1` (feat)
3. **Task 3: Seam tests pinning the tracer's guarantees** — `ac497bb` (test, includes the Rule 1 positional-parsing fix)

## Files Created/Modified

- `cmd/adoBizInt/write.go` — `runWrite` and the write envelope types; the step order is the contract
- `cmd/adoBizInt/audit.go` — `auditPath`, `openAuditSink`, `appendAuditRecord`, the five-key `auditRecord`
- `cmd/adoBizInt/write_test.go` — eight write tests plus the structural `--confirm` registration guard
- `cmd/adoBizInt/audit_test.go` — five audit tests including the D-13 ordering guarantee
- `cmd/adoBizInt/dispatch.go` — `exitNoOp`/`exitOutcomeUnknown`, `writeClient`, combined `client`, the `workitems set-state` entry
- `cmd/adoBizInt/flags.go` — `writeSupport`/`writeSupportByCommand`/`writeFlags`, `--confirm` and `--state` behind the capability map
- `cmd/adoBizInt/exit.go` — `writeExitCodeFor` and the five outcome constants
- `cmd/adoBizInt/fakes_test.go` — `fakeWriteClient` with nil-closure guards, embedded into `fakeReadClient`
- `internal/ado/workitems.go` — three `*float64` effort fields plus their extraction blocks
- `internal/ado/result.go` — exported `StatusCodeOf` and `ClassifyFailure`
- `cmd/adoBizInt/{usage.go,usage_test.go,filter_test.go,main_test.go}`, `internal/ado/json_test.go` — `newFlagSet` arity and widened drift-guard expectation tables

## Decisions Made

Beyond the four ratified in Task 1:

- **Exported `ado.ClassifyFailure` as well as `StatusCodeOf`.** The plan named only `StatusCodeOf`, but `writeExitCodeFor` is required to key on `ado.FailureKind` rather than error text, and `classifyFailure` was private. Re-implementing it in `cmd` would have put the "a cancellation must not look like a server timeout" rule in two places and let them drift — the exact failure mode `classifyFailure`'s own doc comment warns about.
- **`fakeReadClient` embeds `*fakeWriteClient`.** The plan required re-typing `commandFunc`'s second parameter to a combined interface, which would have broken all 52 existing `dispatch(...)` test call sites. Embedding a nil-able pointer keeps every existing helper returning `*fakeReadClient` valid; a read test that somehow reached a mutation panics on the nil receiver, which is still a hard failure.
- **`wantCommands` was split rather than extended.** `TestDispatchResolvesEveryVerb` drives each key with a bare `--json` invocation expecting exit 0, which a guarded write correctly rejects. A new `wantWriteCommands` list plus an `allCommands()` union keeps `TestDispatchTableMatchesD02` pinning the full published surface without weakening the resolution test.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Positional target was silently swallowing every flag**

- **Found during:** Task 3 (the RED phase — all eight write tests failed identically)
- **Issue:** Go's stdlib `flag` stops parsing at the first non-flag argument. D-23 puts the target in positional form, so the documented spelling `workitems set-state 4821 --project P --state Closed` left `--project` and `--state` unset and every write failed as a usage error. The bug was invisible to Task 2's acceptance criteria, which are structural (file contents, symbol presence, call ordering) and never invoked the command end to end. Task 3's tests are what found it, which is the tracer feedback loop working as designed.
- **Fix:** Added `hoistLeadingPositionals`, which moves the arguments preceding the first flag to the end of the slice before `fs.Parse`. It moves rather than extracts, so the `len(fs.Args()) != 1` arity check still sees a second target and D-25 still holds; and a token is relocated only when it precedes the first flag, so a flag's own value can never be hoisted.
- **Files modified:** `cmd/adoBizInt/write.go`
- **Verification:** All eight write tests and five audit tests go green; `TestWriteParseErrorsExitUnknown`'s "two positionals" row confirms the arity check still fires.
- **Committed in:** `ac497bb`

**2. [Rule 3 - Blocking] `classifyFailure` was unreachable from `cmd`**

- **Found during:** Task 2 (implementing `writeExitCodeFor`)
- **Issue:** The plan requires the write mapper to key on `ado.FailureKind`, but only `StatusCodeOf` was listed for export. There was no exported classifier.
- **Fix:** Added `ado.ClassifyFailure` as a thin exported wrapper, documented with the same rationale as `StatusCodeOf`.
- **Files modified:** `internal/ado/result.go`
- **Verification:** `TestWriteOutcomeClassification`'s 403 and deadline rows both route correctly, which is only possible through the classifier.
- **Committed in:** `f5638d1`

**3. [Rule 3 - Blocking] Three drift-guard expectation tables required widening**

- **Found during:** Task 2
- **Issue:** `TestJSONFieldNames/WorkItemDetail` (key count 22 vs 19), `TestDispatchTableMatchesD02` (9 commands vs 8) and `TestFilterFlagsRegisteredOnSupportedVerbsOnly` (uncovered command) all failed. Each is a deliberate guard against silent surface growth.
- **Fix:** Added the three new effort keys, the new command key, and the new verb's empty filter-flag row. Each guard's assertion strength is unchanged — the tables grew, the checks did not relax.
- **Files modified:** `internal/ado/json_test.go`, `cmd/adoBizInt/main_test.go`, `cmd/adoBizInt/filter_test.go`
- **Verification:** `go test ./...` green with all three guards still asserting exact sets.
- **Committed in:** `f5638d1`

---

**Total deviations:** 3 auto-fixed (1 bug, 2 blocking)
**Impact on plan:** No scope creep. Deviation 1 is a genuine correctness bug the plan's own structure surfaced; 2 and 3 are mechanical consequences of the planned surface change. Every prohibition in `must_haves` remains kept — `runWrite` calls neither `emptyIfNil` nor `writeCrashRecord`, the `openAuditSink` line precedes `UpdateWorkItemState`, the envelope's single top-level key is `write`, and no write path prompts or branches on TTY detection.

## Issues Encountered

None beyond the deviations above.

## User Setup Required

None — no external service configuration required. The audit sink is created on first write at `~/.config/adoBizInt/audit.jsonl`, beside the existing log file, so Phase 5 HARD-06's `0600`/`0700` work will cover both.

## Known Stubs

None. The three effort branches in `diffWorkItem` are reachable-but-unexercised in 04-01 by design: their flags are not yet registered (`writeSupportByCommand` sets `effort: false`), so they cannot be reached from the CLI. They exist so the fixed field ordering — OriginalEstimate, RemainingWork, CompletedWork, then System.State — is a structural property of the function rather than a comment 04-03 must remember to honour. This is documented in the function's doc comment.

## Threat Flags

None. No new network endpoint, auth path or trust boundary beyond those already in the plan's threat register. `auditPath` is built from fixed literal segments with no caller-controlled component (T-04-03 mitigated by construction), and no dependency was added (T-04-SC not applicable).

## Next Phase Readiness

- The contract 04-02, 04-03 and 04-04 all build on is now implemented and green. 04-03's expansion to the remaining six verbs is additive: new `writeSupportByCommand` entries, new `writeClient` methods, new `commands` keys.
- Two things 04-03 must not miss, both currently true only for the tracer: `writeSupportByCommand` has one entry, and `diffWorkItem` is work-item-specific. `TestConfirmRegisteredOnEveryWriteVerbAndNoReadVerb` will fail loudly if a verb is registered in the dispatch table without a support entry, in either direction.
- `UpdateWorkItemStateWithEffort`'s `float64` -> `*float64` signature change (D-17) and the four TUI call sites it touches are NOT in this plan and remain outstanding for 04-02/04-03.
- `hoistLeadingPositionals` currently lives in `write.go` and is used by one verb. When 04-03 adds six more, it should stay one shared function rather than being copied.

## Self-Check: PASSED

- `cmd/adoBizInt/write.go` — FOUND
- `cmd/adoBizInt/audit.go` — FOUND
- `cmd/adoBizInt/write_test.go` — FOUND
- `cmd/adoBizInt/audit_test.go` — FOUND
- Commit `f5638d1` — FOUND
- Commit `ac497bb` — FOUND
- `go build ./... && go test ./... && go vet ./... && gofmt -l .` — all clean
- `go test -race ./cmd/adoBizInt/...` — ok
- D-13 ordering guard confirmed to fail when `openAuditSink` is moved below the mutate, then reverted

---
*Phase: 04-guarded-writes*
*Completed: 2026-09-12*
