---
phase: 01-test-seam-for-internal-ado
plan: 01
subsystem: testing
tags: [go, azure-devops, interfaces, fakes, mutation-testing]

requires: []
provides:
  - Package-local workItemAPI seam with the five SDK methods internal/ado calls
  - Fail-fast hand-written work-item fake for network-free tests
  - Exact JSON Patch regression test for UpdateWorkItemState
  - Mutation evidence proving the exact-path assertion detects source drift
affects: [01-02, 01-03, 01-04, 01-05, phase-4-guarded-writes]

actuals:
  tokens: 1404
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - Narrow package-local consumer interface assigned from an SDK interface
    - Hand-written function-field fake that fails by method name on unexpected calls
    - Literal SDK argument comparison with reflect.DeepEqual

key-files:
  created:
    - internal/ado/interfaces.go
    - internal/ado/fakes_test.go
    - internal/ado/workitems_test.go
  modified:
    - internal/ado/client.go

key-decisions:
  - "Use the assignable unexported Client.workitemClient field as the injection seam; add no constructor or adapter."
  - "Rely on NewClient's SDK-interface assignment as compile-time conformance proof; add no ClientImpl assertion."
  - "Require each fake method to fail by interface and method name when its function field is unset."

patterns-established:
  - "Consumer interface: declare only methods internal/ado actually calls, keeping the SDK boundary narrow."
  - "Mutation evidence: capture both the deliberate source diff and failing test output, then restore with git checkout and rerun green."

requirements-completed: [SEAM-01, SEAM-02, SEAM-03]

coverage:
  - id: D1
    description: "A package-local five-method workItemAPI seam accepts the SDK client in production and a fake in tests."
    requirement: SEAM-01
    verification:
      - kind: integration
        ref: "go build ./... && go vet ./..."
        status: pass
    human_judgment: false
  - id: D2
    description: "A Client struct literal with only workitemClient set drives UpdateWorkItemState without credentials or network access."
    requirement: SEAM-02
    verification:
      - kind: unit
        ref: "internal/ado/workitems_test.go#TestUpdateWorkItemStateIssuesExactPatch"
        status: pass
    human_judgment: false
  - id: D3
    description: "UpdateWorkItemState emits the exact literal JSON Patch operation, and a mutated path makes the test fail."
    requirement: SEAM-03
    verification:
      - kind: unit
        ref: "go test ./internal/ado/ -run TestUpdateWorkItemStateIssuesExactPatch"
        status: pass
      - kind: other
        ref: "mutation check: /fields/System.State -> /fields/System.State.MUTATED"
        status: pass
    human_judgment: false

duration: 4 min
completed: 2026-09-04
status: complete
---

# Phase 1 Plan 1: Work Item API Seam Tracer Summary

**A five-method work-item interface, fail-fast fake, and mutation-proven exact JSON Patch test establish the network-free ADO write-test seam.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-09-04T21:48:36Z
- **Completed:** 2026-09-04T21:52:35Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Narrowed `Client.workitemClient` to the unexported five-method `workItemAPI` while preserving the production `NewClient` assignment as compile-time SDK conformance proof.
- Added a hand-written `fakeWorkItemAPI` whose five methods fail by interface and method name when unstubbed.
- Added `TestUpdateWorkItemStateIssuesExactPatch`, which injects the fake through a `Client` struct literal and asserts the complete `UpdateWorkItemArgs` value without credentials or network access.
- Raised `internal/ado` coverage from no test files to 2.1% and proved the assertion detects a changed state-field path.

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end seam slice -- workItemAPI wired from interface to green test** -- `2bf658d` (test)
2. **Task 2: Mutation sanity check -- prove the tracer test is not vacuous** -- `d825903` (test, evidence-only empty commit after source restoration)

## Files Created/Modified

- `internal/ado/interfaces.go` -- Defines the narrow package-local work-item SDK boundary and documents its method ceiling and conformance mechanism.
- `internal/ado/client.go` -- Retypes `workitemClient` from the full SDK interface to `workItemAPI`.
- `internal/ado/fakes_test.go` -- Provides the five-method fail-fast hand-written fake.
- `internal/ado/workitems_test.go` -- Captures and compares the exact state-update patch arguments.

## Mutation Evidence

The transient production mutation was captured before restoration:

```diff
diff --git a/internal/ado/workitems.go b/internal/ado/workitems.go
index 185a94d..c54a6d8 100644
--- a/internal/ado/workitems.go
+++ b/internal/ado/workitems.go
@@ -216,7 +216,7 @@ func (c *Client) UpdateWorkItemState(ctx context.Context, itemID int, newState s
 	log := logging.Logger()
 
 	op := webapi.OperationValues.Add
-	path := "/fields/System.State"
+	path := "/fields/System.State.MUTATED"
 	document := []webapi.JsonPatchOperation{
 		{
 			Op:    &op,
```

Observed failing output with that mutation present:

```text
=== RUN   TestUpdateWorkItemStateIssuesExactPatch
2026/09/04 16:51:50 INFO Work item state updated id=42 state=Active
    workitems_test.go:43: UpdateWorkItem args = {Document:0x2cc7459302a0 Id:0x2cc745932810 Project:<nil> ValidateOnly:<nil> BypassRules:<nil> SuppressNotifications:<nil> Expand:<nil>}, want {Document:0x2cc745930300 Id:0x2cc745932870 Project:<nil> ValidateOnly:<nil> BypassRules:<nil> SuppressNotifications:<nil> Expand:<nil>}
--- FAIL: TestUpdateWorkItemStateIssuesExactPatch (0.00s)
FAIL
FAIL	sentinovo.ai/bizInt/internal/ado	0.537s
FAIL
```

After `git checkout -- internal/ado/workitems.go`, the targeted test, build, and vet all passed. `git status --porcelain internal/ado/workitems.go` and `git diff --exit-code -- internal/ado/workitems.go` were empty, proving no deliberate mutation remains.

## Verification

- `gofmt -l internal/` -- passed with no output.
- `go build ./... && go vet ./...` -- passed.
- `go test ./internal/ado/... -count=1 -cover` -- passed at 2.1% coverage.
- `go test ./... -count=1` -- passed.
- Targeted test with `AZURE_DEVOPS_*` credentials removed -- passed at 2.1% coverage.
- Interface count: 1; interface method signatures: 5; fake methods: 5; `Fatalf` guards: 5.
- No `ClientImpl` assertion, `logging.Init`, network construction, `internal/ui/`, or `cmd/` change was introduced.

## Decisions Made

- Used the unexported struct field as the complete injection seam. A constructor would export implementation detail, add unused public API, and still encounter `NewClient` network I/O.
- Kept the SDK constructor assignment as the conformance check because the SDK constructor returns an interface. A concrete `ClientImpl` assertion would prove the wrong production relationship.
- Committed Task 2 as an intentional evidence-only empty commit because its required mutation was transient and had to be fully reverted before completion.

## Deviations from Plan

None -- plan executed exactly as written.

## Issues Encountered

- Context7 was unavailable in this runtime. The pinned SDK contract was verified with `go doc` and the already-resolved module source; no dependency or API guesswork was used.

## Known Stubs

None.

## Threat Surface

No unplanned security-relevant surface was introduced. The test client contains no PAT, connection, HTTP client, or network constructor, and the fake terminates calls before the ADO REST boundary.

## User Setup Required

None -- no external service configuration required.

## Next Phase Readiness

- The tracer seam and fake convention are proven for expansion in plan 01-02.
- The mutation protocol has a concrete red/green evidence baseline for the remaining write tests.
- Phase 1 remains in progress; four plans remain.

## Self-Check: PASSED

- Created files exist: `internal/ado/interfaces.go`, `internal/ado/fakes_test.go`, and `internal/ado/workitems_test.go`.
- Task commits exist: `2bf658d` and `d825903`.
- All task acceptance criteria and plan-level verification commands passed.
- `internal/ado/workitems.go` is clean relative to the Task 1 commit; no deliberate mutation remains.

---
*Phase: 01-test-seam-for-internal-ado*
*Completed: 2026-09-04*
