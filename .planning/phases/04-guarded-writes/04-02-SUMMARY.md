---
phase: 04-guarded-writes
plan: 02
subsystem: cli
tags: [go, json-patch, azure-devops, nullable, flag-presence, write-safety]

requires:
  - phase: 04-guarded-writes
    provides: "04-01's guarded-write path — runWrite, diffWorkItem's effort branches, writeFlags' *float64 fields, writeSupport.effort, the writeClient seam and its fake"
  - phase: 01-test-seam-for-internal-ado
    provides: unexported workitemAPI seam and nil-closure struct-literal fakes
provides:
  - "`UpdateWorkItemStateWithEffort(ctx, id, state, originalEstimate, remaining, completed *float64)` — nullable effort parameters; nil omits the operation entirely"
  - "`--original-estimate`, `--remaining`, `--completed` on `workitems set-state`"
  - "`writeFlags.promoteEffort(fs)` — fs.Visit presence promotion, and `writeFlags.hasEffort()`"
  - "`UpdateWorkItemStateWithEffort` on the `writeClient` seam and on `fakeWriteClient`"
  - "`effortLogValue` — renders a nullable effort value as the number or \"omitted\" for structured logging"
affects: [04-03, 04-04, 05-hardening]

actuals:
  tokens: 34000
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Flag presence via fs.Visit promoted into *float64, rather than value comparison or omitempty — the only shape in which omitted and zero stay distinguishable"
    - "A nil check gates the append into a JSON-Patch document, never the Value field"
    - "Pointer parameters taken from local copies at the call site, so a message struct never aliases into a call"

key-files:
  created: []
  modified:
    - internal/ado/workitems.go
    - internal/ado/workitems_test.go
    - internal/ui/workitems/workitems.go
    - cmd/adoBizInt/flags.go
    - cmd/adoBizInt/write.go
    - cmd/adoBizInt/write_test.go
    - cmd/adoBizInt/dispatch.go
    - cmd/adoBizInt/fakes_test.go

key-decisions:
  - "The nil check gates the `append`, never the `Value` field — a nil *float64 in Value serializes as \"value\":null, which CLEARS the field rather than omitting it"
  - "fs.Visit, not fs.VisitAll and not a value comparison: only the set of flags actually set can distinguish --remaining 0 from an omitted --remaining"
  - "Route through UpdateWorkItemStateWithEffort only when at least one effort pointer is non-nil; an effort-free write still goes through UpdateWorkItemState"
  - "effort.go left untouched: parseFloat returning 0 for blank input means the TUI genuinely sends 0 today, and making blank mean nil would be a TUI behaviour change nothing asked for"
  - "Added effortLogValue rather than logging the pointers — logging a *float64 prints an address, hiding both the value and the omitted/supplied distinction"
  - "Added UpdateWorkItemStateWithEffort to the writeClient seam (not only to *ado.Client), so the CLI effort path is exercised through the same fake as every other write guarantee"

patterns-established:
  - "Operation-COUNT assertions as the null-vs-omission proof: an appended \"value\":null keeps the document at four operations, so only a count assertion catches it"
  - "Structural flag-registration guards extended to effort: TestEffortFlagsNotRegisteredOnNonEffortVerbs mirrors the --confirm guard in both directions"

requirements-completed: [SAFE-03]

coverage:
  - id: D1
    description: "An omitted effort flag appends no JSON-Patch operation for its path — no \"value\":null is ever emitted"
    requirement: SAFE-03
    verification:
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestUpdateWorkItemStateWithEffortOmitsNilFields/only_remaining_supplied"
        status: pass
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestUpdateWorkItemStateWithEffortOmitsNilFields/no_effort_supplied_leaves_only_the_state_operation"
        status: pass
    human_judgment: false
  - id: D2
    description: "`--remaining 0` is a real zero: a non-nil pointer to 0.0 producing a RemainingWork operation, distinct from omitting the flag"
    requirement: SAFE-03
    verification:
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestUpdateWorkItemStateWithEffortOmitsNilFields/explicit_zero_remaining_is_a_real_zero,_not_an_omission"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestExplicitZeroEffortIsDistinctFromOmitted"
        status: pass
    human_judgment: false
  - id: D3
    description: "The four-operation document the TUI produces is byte-identical to the one it produced before the signature change (D-17 regression guard)"
    requirement: SAFE-03
    verification:
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestUpdateWorkItemStateWithEffortIssuesExactPatch"
        status: pass
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestUpdateWorkItemStateWithEffortOmitsNilFields/all_three_supplied"
        status: pass
    human_judgment: false
  - id: D4
    description: "Effort flags are registered on exactly the verbs whose writeSupport.effort is true, and using one elsewhere is a parse error exiting 3"
    requirement: SAFE-03
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestEffortFlagsNotRegisteredOnNonEffortVerbs"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestEffortFlagOnNonEffortVerbIsAParseError"
        status: pass
    human_judgment: false
  - id: D5
    description: "The `changes` entries for an effort write are emitted in the fixed order OriginalEstimate, RemainingWork, CompletedWork, then System.State"
    requirement: SAFE-03
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestEffortChangesAreOrderedBeforeState"
        status: pass
    human_judgment: false
  - id: D6
    description: "An effort-free write still routes through UpdateWorkItemState rather than handing the effort method three nils"
    requirement: SAFE-03
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestOmittedEffortRoutesThroughTheNoEffortMethod"
        status: pass
    human_judgment: false
  - id: D7
    description: "The TUI's effort flow behaves identically after the signature change when driven interactively against a real Azure DevOps organisation"
    verification: []
    human_judgment: true
    rationale: "The emitted patch document is pinned byte-for-byte by TestUpdateWorkItemStateWithEffortIssuesExactPatch, and go test ./internal/ui/... is green, but no test drives the Bubble Tea effort modal end to end. That the modal still closes a Task with the effort the user typed is an interactive fact the seam cannot establish. 04-04's human-verify step covers it."

duration: 24 min
completed: 2026-09-12
status: complete
---

# Phase 4 Plan 02: Nullable effort fields Summary

**`UpdateWorkItemStateWithEffort` takes three `*float64` so a field the caller never named produces no JSON-Patch operation at all, with `fs.Visit` presence promotion on the CLI side keeping `--remaining 0` a real zero — closing a live blind-overwrite bug that the daily-use TUI was also hitting.**

## Performance

- **Duration:** 24 min
- **Tasks:** 3
- **Files modified:** 8 (0 created, 8 modified)

## Accomplishments

- The fix lives at the `internal/ado` signature rather than in the CLI, so every caller gets it. `UpdateWorkItemStateWithEffort` sent `Add` for all three effort paths on every call before this plan, which meant the TUI blind-overwrote OriginalEstimate and CompletedWork every time a user closed a Task with only remaining work in mind. That is a real bug being fixed, not churn imposed on working code for the CLI's benefit.
- The null-vs-omission trap is closed by construction and proven by an operation-COUNT assertion. A nil `*float64` assigned to `JsonPatchOperation.Value` would serialize as `"value":null` — an explicit CLEAR — and would still produce a four-operation document that a field-by-field comparison could be written to accept. `TestUpdateWorkItemStateWithEffortOmitsNilFields` asserts the count first and the full document second, so that mistake fails loudly.
- `--remaining 0` survives end to end. The presence map comes from `fs.Visit`, which walks only the flags actually set, so zero-because-asked and zero-by-default never collapse. No `float64` with `omitempty` exists anywhere between the flag and the patch builder — the inverse bug, which would silently drop the primary use case of closing a task.
- The D-17 regression guard held without being weakened. `TestUpdateWorkItemStateWithEffortIssuesExactPatch` still `reflect.DeepEqual`s the same four-operation slice literal; only the three arguments became pointers.
- The TUI blast radius was exactly two lines, not four. 04-RESEARCH Q7's warning was correct: `workitems.go:266` and `:347` call `UpdateWorkItemState`, which takes no effort parameters and was untouched. Verified by reading before editing.

## Task Commits

1. **Task 1: Nullable effort parameters at the `internal/ado` signature** — `e650eb5` (fix; TDD — the pointer-argument test was compiled red against the old signature first)
2. **Task 2: Update the two TUI call sites** — `23b9a2d` (refactor)
3. **Task 3: CLI effort flags with `fs.Visit` presence detection** — `9e7f292` (feat; TDD — the fake's missing closure field was the red)

## Files Created/Modified

- `internal/ado/workitems.go` — `*float64` effort parameters; conditional `append` builder; `effortLogValue`
- `internal/ado/workitems_test.go` — pointer arguments in the exact-patch guard; new four-case `TestUpdateWorkItemStateWithEffortOmitsNilFields`
- `internal/ui/workitems/workitems.go` — the two `UpdateWorkItemStateWithEffort` call sites pass addresses of local copies
- `cmd/adoBizInt/flags.go` — `effort: true` for `workitems set-state`; the three `Float64Var` registrations; `promoteEffort` and `hasEffort`
- `cmd/adoBizInt/write.go` — `promoteEffort` called immediately after `Parse`; effort-aware routing of the confirmed write
- `cmd/adoBizInt/dispatch.go` — `UpdateWorkItemStateWithEffort` added to the `writeClient` seam
- `cmd/adoBizInt/fakes_test.go` — the matching nil-guarded fake closure
- `cmd/adoBizInt/write_test.go` — five new tests covering zero-vs-omitted, routing, diff order and flag registration

## Decisions Made

- **`effortLogValue` rather than logging the pointers.** The plan said to keep the log tail as-is, but the tail logged the three parameters directly and they are now pointers — `slog` would have printed `0xc000112233`, hiding both the value and the omitted/supplied distinction the whole plan exists to preserve. A nil renders as `"omitted"`, which is the fact worth logging.
- **`UpdateWorkItemStateWithEffort` added to the `writeClient` interface.** The plan named the method as the confirmed-write route but the seam only carried `UpdateWorkItemState`. Extending the interface (and the fake) keeps the CLI effort path provable through the same nil-closure seam as every other write guarantee, rather than reachable only against a real `*ado.Client`.
- **`TestOmittedEffortRoutesThroughTheNoEffortMethod` added beyond the plan's named tests.** `hasEffort()` gating is the other half of D-18 and nothing the plan listed would have caught an unconditional route through the effort method with three nils — which is harmless today but would quietly become the only path 04-03 maintains.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `writeClient` had no `UpdateWorkItemStateWithEffort`**

- **Found during:** Task 3
- **Issue:** The plan's Task 3 requires routing the confirmed write through `UpdateWorkItemStateWithEffort`, but 04-01 only put `UpdateWorkItemState` on the `writeClient` seam. The method existed on `*ado.Client` and was therefore unreachable through the interface the write path is handed.
- **Fix:** Added the method to `writeClient` with a doc comment stating the nil-means-omitted contract, and the matching nil-guarded closure to `fakeWriteClient`.
- **Files modified:** `cmd/adoBizInt/dispatch.go`, `cmd/adoBizInt/fakes_test.go`
- **Verification:** `TestExplicitZeroEffortIsDistinctFromOmitted` captures the pointers the mutating call receives, which is only possible through the seam.
- **Committed in:** `9e7f292`

**2. [Rule 1 - Bug] Structured log printed pointer addresses**

- **Found during:** Task 1
- **Issue:** The existing log tail logs `originalEstimate`, `remaining` and `completed` by value. After the signature change those are `*float64`, so every successful effort write would have logged three heap addresses — strictly less information than before, at the exact site where the omitted/supplied distinction matters most.
- **Fix:** Added `effortLogValue`, returning the dereferenced value or the string `"omitted"`.
- **Files modified:** `internal/ado/workitems.go`
- **Verification:** `go test ./internal/ado/... -run UpdateWorkItemState -v` shows `originalEstimate=omitted remaining=0 completed=omitted` on the explicit-zero case.
- **Committed in:** `e650eb5`

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** No scope creep. Deviation 1 is a mechanical consequence of the seam 04-01 left narrow; deviation 2 is a small correctness fix at a line the plan told me to leave alone, and leaving it alone would have degraded the logs. Every prohibition in `must_haves` remains kept — no pointer-typed expression is assigned to a `Value:` field, no `float64` with `omitempty` exists in the flag-to-patch path (verified with the comment-stripped grep), `Op` is `Add` on all four paths, surviving operations are not reordered, and `internal/ui/workitems/effort.go` is untouched.

## Issues Encountered

One self-inflicted edit went into the wrong block — the `promoteEffort` call landed inside `finishWrite`'s JSON branch because the anchor text `return exitUnknown\n\t}` was not unique in `write.go`. The compiler caught it immediately (`undefined: wf`), and the fix was to restore `finishWrite` verbatim and re-anchor on a longer string. Nothing reached a commit.

## User Setup Required

None — no external service configuration required.

## Known Stubs

None. Every branch added by this plan is reachable from the CLI: `writeSupportByCommand["workitems set-state"].effort` is now `true`, so all three flags are registered and all three `diffWorkItem` effort branches — which 04-01 documented as reachable-but-unexercised — are now exercised by `TestEffortChangesAreOrderedBeforeState`.

## Threat Flags

None. No new network endpoint, auth path, file access pattern or schema change at a trust boundary. The plan's register is fully mitigated: T-04-08 by the append-gating nil check plus the count assertion, T-04-09 by the comment-stripped source grep returning nothing, T-04-10 by the untouched `effort.go` and the unchanged exact-patch literal. T-04-SC is not applicable — no dependency was added.

## Next Phase Readiness

- SAFE-03 is closed and ROADMAP SC4 holds: omitting an effort field leaves it unchanged rather than zeroing it.
- 04-03 expands `writeSupportByCommand` to the remaining six verbs. `TestEffortFlagsNotRegisteredOnNonEffortVerbs` will fail in both directions if a new verb sets `effort: true` without the registrations, or registers them without the support entry — the same structural guard `--confirm` already has.
- `promoteEffort` lives on `writeFlags` and is called once, from `runWrite`. When 04-03 adds verbs it stays one call at one site; it must not be copied into per-verb bodies.
- D7 is the only human-judgment item: the Bubble Tea effort modal is not driven end to end by any test. 04-04's human-verify step should close a real Task through the TUI once.

## Self-Check: PASSED

- `internal/ado/workitems.go` — FOUND
- `internal/ui/workitems/workitems.go` — FOUND
- `cmd/adoBizInt/flags.go` — FOUND
- Commit `e650eb5` — FOUND
- Commit `23b9a2d` — FOUND
- Commit `9e7f292` — FOUND
- `go build ./... && go test ./... && go vet ./...` — all exit 0; `gofmt -l .` prints nothing
- `go test -race ./cmd/adoBizInt/... ./internal/ado/...` — ok
- `grep -rn 'fs.VisitAll(' cmd/adoBizInt` — no matches; `fs.Visit(` present at `flags.go:234`
- Comment-stripped `float64.*omitempty` grep excluding `*float64` — no matches
- `git diff --stat` lists no change to `internal/ui/workitems/effort.go`

---
*Phase: 04-guarded-writes*
*Completed: 2026-09-12*
