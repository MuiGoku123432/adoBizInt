---
phase: 02-honest-reads
plan: 04
subsystem: api
tags: [json, encoding/json, struct-tags, contract, snake_case, go-generics]

requires:
  - phase: 02-01
    provides: the Result[T] envelope and the Failure record that this plan tags
  - phase: 02-02
    provides: the rewritten propagating bodies of six characterization tests
  - phase: 02-03
    provides: the dashboard aggregate on a one-element Items slice in Result[DashboardCounts]
provides:
  - Eleven exported ado structs carrying explicit lower snake_case json tags
  - json tags on the shared Result envelope and Failure record, so all eight reads serialize under one contract
  - A Failure.MarshalJSON that flattens the error interface to its message string (T-02-01 structural leak guard)
  - internal/ado/json_test.go pinning every key set, the embedded-inlining decision, the duration shape, and assumption A3
  - ado.Client explicitly excluded from tagging, on the record with a comment
  - Eight tests renamed from SwallowsProjectFailure to ReportsProjectFailure; the Phase 1 countdown gate now reads zero
affects: [phase-03-cli-json-output, phase-03-out-03-stdout-stderr-split]

actuals:
  tokens: 21000
  tasks: 3
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Exact-key-set JSON assertions over per-field assertions — an exact set is what catches a MISSING tag"
    - "Custom MarshalJSON to flatten an error interface to a string, preventing structural leakage of the dynamic value's exported fields"
    - "Initialisms mapped deliberately (URL -> url, PipelineID -> pipeline_id, PRs -> prs) rather than by mechanical camel-to-snake conversion"

key-files:
  created:
    - internal/ado/json_test.go
  modified:
    - internal/ado/result.go
    - internal/ado/client.go
    - internal/ado/workitems.go
    - internal/ado/pullrequests.go
    - internal/ado/pipelines.go
    - internal/ado/releases.go
    - internal/ado/branches.go
    - internal/ado/dashboard.go
    - internal/ado/workitems_test.go
    - internal/ado/pullrequests_test.go
    - internal/ado/pipelines_test.go
    - internal/ado/releases_test.go
    - internal/ado/branches_test.go
    - internal/ado/dashboard_test.go

key-decisions:
  - "PipelineRun.Duration serializes as encoding/json's default integer nanosecond count, under the key duration_ns — a locked one-way-door contract"
  - "Timestamps stay value semantics; a zero time.Time emits the year-one RFC 3339 sentinel rather than being omitted"
  - "The duration JSON key carries an _ns suffix because the unit is not otherwise discoverable from the output"
  - "Failure.Err is json:\"-\" and re-emitted as a plain string by a custom MarshalJSON, so no error implementation's exported fields can reach the output"
  - "The machine-readable failure message is intentionally untruncated, unlike the terminal rendering from 02-01"
  - "The embedded WorkItem in WorkItemDetail is left untagged so its fields inline into one flat object"
  - "ado.Client is the one exported struct deliberately left untagged; the exclusion is stated in a comment on the declaration"

patterns-established:
  - "Zero-value marshalling in tests: every tagged field must appear regardless of data, so consumers see a stable key set"
  - "A leak-guard test that asserts forbidden substrings (token, request URL) are absent from marshalled output"

requirements-completed: [OUT-02]

coverage:
  - id: D1
    description: "Eleven exported ado structs carry explicit lower snake_case json tags, with ado.Client deliberately excluded"
    requirement: "OUT-02"
    verification:
      - kind: unit
        ref: "internal/ado/json_test.go#TestJSONFieldNames"
        status: pass
      - kind: other
        ref: "grep -c 'json:\"' internal/ado/client.go returns 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "Initialism-bearing field names render as single lower-case words, not letter-split garbage"
    requirement: "OUT-02"
    verification:
      - kind: unit
        ref: "internal/ado/json_test.go#TestJSONFieldNames_InitialismsAreSingleWords"
        status: pass
    human_judgment: false
  - id: D3
    description: "The embedded WorkItem inside WorkItemDetail inlines into a flat object rather than nesting"
    requirement: "OUT-02"
    verification:
      - kind: unit
        ref: "internal/ado/json_test.go#TestJSONFieldNames_WorkItemDetailEmbedsFlat"
        status: pass
    human_judgment: false
  - id: D4
    description: "The pipeline run duration marshals as an integer nanosecond count under duration_ns — a recorded, deliberately chosen contract"
    requirement: "OUT-02"
    verification:
      - kind: unit
        ref: "internal/ado/json_test.go#TestJSONDurationIsIntegerNanoseconds"
        status: pass
    human_judgment: false
  - id: D5
    description: "The Result envelope and the Failure record carry the same snake_case convention, uniformly across all eight reads"
    requirement: "OUT-02"
    verification:
      - kind: unit
        ref: "internal/ado/json_test.go#TestJSONFieldNames/Result_uniform_across_reads"
        status: pass
      - kind: unit
        ref: "internal/ado/json_test.go#TestJSONFieldNames/Failure"
        status: pass
    human_judgment: false
  - id: D6
    description: "The Failure record cannot structurally leak a request URL, header, or token into serialized output (T-02-01)"
    verification:
      - kind: unit
        ref: "internal/ado/json_test.go#TestJSONFailureErrIsFlattenedString"
        status: pass
    human_judgment: false
  - id: D7
    description: "No aggregate read test in internal/ado still carries a name asserting the pre-inversion swallowing behavior"
    verification:
      - kind: other
        ref: "go test ./internal/ado/ -v -run TestGet | grep -cE '^=== RUN   TestGet[A-Za-z]*SwallowsProjectFailure$' returns 0"
        status: pass
      - kind: other
        ref: "go test ./internal/ado/ -v -run 'TestGet.*ReportsProjectFailure' | grep -cE '^=== RUN   TestGet[A-Za-z]*ReportsProjectFailure$' returns 8"
        status: pass
    human_judgment: false
  - id: D8
    description: "Field ordering in the marshalled object follows struct declaration order, with the embedded struct's fields at the position of the embedding"
    verification: []
    human_judgment: true
    rationale: "Recorded in the plan as a backstop truth, not a primary assertion. Ordering is encoding/json's documented behavior and the key-set tests are order-insensitive by design; a human should confirm the rendered ordering is acceptable when Phase 3 first emits --json."

duration: 16 min
completed: 2026-09-07
status: complete
---

# Phase 02 Plan 04: JSON Field Contract Summary

**Eleven exported `ado` types plus the shared `Result`/`Failure` envelope now carry lower snake_case JSON tags pinned by exact-key-set tests, with `Failure.Err` flattened to a string to block structural leakage, and the Phase 1 characterization-test countdown gate driven to zero.**

## Performance

- **Duration:** 16 min
- **Tasks:** 3 (one blocking decision checkpoint, two implementation tasks)
- **Files modified:** 14 modified, 1 created

## Accomplishments

- Tagged eleven serializable structs across six files in lower snake_case per D-04, handling initialisms deliberately rather than mechanically.
- Tagged the `Result[T]` envelope and `Failure` record in the same convention, so Phase 3 serializes the whole read response under one contract — including 02-03's one-element `Items` slice in `Result[DashboardCounts]`.
- Closed the T-02-01 structural-leak hole that tagging would otherwise have opened, via a custom `Failure.MarshalJSON`.
- Created `internal/ado/json_test.go` with 5 top-level tests and 15 key-set subtests covering every tagged type.
- Retired all eight Phase 1 characterization test names in one sweep; the anchored countdown gate reads zero.

## Task Commits

1. **Task 1: Duration shape decision** — no commit (blocking decision checkpoint; outcome recorded in code comments and this summary)
2. **Task 2: Tag eleven structs + envelope, assert every field name** — `d9bc52b` (feat)
3. **Task 3: Retire eight characterization test names** — `27e1187` (test)

## Files Created/Modified

- `internal/ado/json_test.go` — **created.** Exact-key-set assertions for all 13 tagged types, plus the embedded-inlining pin, the initialism guard, the duration contract test, the assumption A3 confirmation, and the T-02-01 leak guard.
- `internal/ado/result.go` — tags on `Result[T]` (`items`/`failures`/`complete`) and `Failure` (`project`/`operation`/`kind`/`err`); `Err` marked `json:"-"` with a `MarshalJSON` that flattens it; `encoding/json` import added.
- `internal/ado/client.go` — comment on the `Client` declaration recording the deliberate tagging exclusion. No tags.
- `internal/ado/workitems.go` — tags on `WorkItem`, `WorkItemDetail`, `Attachment`; comments on the embedded-inlining decision and the `URL` initialism.
- `internal/ado/pipelines.go` — tags on `PipelineRun`, `PipelineDefinition`; the duration and timestamp contract recorded in a doc comment.
- `internal/ado/pullrequests.go`, `internal/ado/releases.go`, `internal/ado/branches.go`, `internal/ado/dashboard.go` — tags on `PullRequest`, `Release`, `ReleaseEnvironment`, `ReleaseApproval`, `Branch`, `DashboardCounts`.
- Six `internal/ado/*_test.go` files — eight test renames plus rewritten doc comments.

## Decisions Made

### The duration shape (Task 1, decided by the user)

`PipelineRun.Duration` serializes as `encoding/json`'s default for `time.Duration`: an **integer nanosecond count**. A one-hour run emits `3600000000000`. No custom marshaller, no derived field.

Rationale as recorded at decision time:

- This project's consumers are agents and shell pipelines. An integer is trivially handled by `jq` and by an LLM; a Go-format duration string (`"1h0m0s"`) is not natively parseable outside Go and would have been the highest-friction option for exactly the audience this project serves.
- Nanosecond integers stay numerically sortable and comparable without parsing.
- Accepted cost: raw JSON shows a large opaque integer, and unfinished runs emit a year-one sentinel date rather than omitting the field. Both are documented consequences, not oversights.
- **This is a locked contract.** Phase 3's `--json` makes the value type a one-way door. Changing an integer to a string later is a breaking change that will not even fail loudly — a consumer doing arithmetic on the value silently gets nonsense. `TestJSONDurationIsIntegerNanoseconds` exists to make a later change fail as a test rather than in production.

**Naming choice within the chosen shape:** the JSON key is `duration_ns`, not `duration`. The user explicitly delegated this. The unit is not discoverable from the value or from a bare `duration` key, and a consumer who guesses milliseconds is wrong by six orders of magnitude. The suffix makes the unit part of the contract instead of part of the documentation.

**Timestamps:** no field was converted to a pointer, per the decision. Every `time.Time` stays a value, so a zero timestamp marshals as the year-one RFC 3339 sentinel.

### Assumption A3: CONFIRMED

RESEARCH recorded as unverified that `,omitempty` does not suppress a zero `time.Time`. Confirmed by `TestJSONZeroTimestampIsSentinelDate`: a zero `time.Time` value field marshals to `"0001-01-01T00:00:00Z"` and is not omitted. A struct is never "empty" to `encoding/json`, so omission would require a pointer field or a custom marshaller. The test is named for the behavior so the confirmation is a recorded finding rather than an incidental pass, and it fails loudly with an "assumption A3 REFUTED" message if the standard library ever changes.

### The `Failure.Err` serialization (T-02-01)

Recorded separately below as a deviation, because it is the one place this plan added code beyond tags.

### The `WorkItemDetail` embedding

The embedded `WorkItem` is left untagged, so `encoding/json` inlines its fields into the same object. A detail record is therefore one flat object whose key set is a superset of the list record's. This is stated in a comment on the embedding and pinned by `TestJSONFieldNames_WorkItemDetailEmbedsFlat`, which asserts the absence of both a `WorkItem` and a `work_item` nesting key.

### The `ado.Client` exclusion

`Client` is the twelfth exported struct and the only one not tagged. Every field is unexported and it holds live SDK sub-clients plus a live connection. The exclusion is stated in a comment on the declaration so a future contributor does not read it as an oversight, and `grep -c 'json:"' internal/ado/client.go` returning `0` is an asserted criterion.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] `Failure.Err` would have structurally leaked the SDK error's exported fields**

- **Found during:** Task 2
- **Issue:** The plan directed tagging "exactly the four fields 02-01 declared". Tagging `Err error` naively lets `encoding/json` marshal the *dynamic* value behind the interface — for an `azuredevops.WrappedError` that means its exported fields, which can carry server-controlled response detail. Threat T-02-01 forbids exactly this, and the incoming brief flagged `Failure.Err` for deliberate audit rather than letting `encoding/json` decide by default. A plain tag would have satisfied the letter of the plan and violated its security constraint.
- **Fix:** `Err` tagged `json:"-"` and a `MarshalJSON` added on `Failure` that emits exactly the four contracted keys, with `err` flattened to `Err.Error()` (or `null`). The key set is unchanged at four — no field was added, so the T-02-01 mitigation's "tags exactly the four fields and adds none" holds.
- **Files modified:** `internal/ado/result.go`, `internal/ado/json_test.go`
- **Verification:** `TestJSONFailureErrIsFlattenedString` marshals a `Failure` carrying a stand-in error with an exported `RequestURL` field containing a token, and asserts none of `RequestURL`, `request_url`, `SECRET`, or `dev.azure.com` appears anywhere in the output. `TestJSONFieldNames/Failure` pins the four-key set.
- **Committed in:** `d9bc52b`

**2. [Rule 3 - Blocking] The `Client` exclusion comment contained the literal string its own acceptance criterion greps for**

- **Found during:** Task 2 acceptance verification
- **Issue:** The first draft of the comment explaining why `Client` is untagged quoted the criterion command verbatim, which contains `json:"`. That made `grep -c 'json:"' internal/ado/client.go` return `1` instead of the required `0` — the comment falsified the criterion it was documenting.
- **Fix:** Reworded the comment to describe the constraint without reproducing the literal.
- **Files modified:** `internal/ado/client.go`
- **Verification:** `grep -c 'json:"' internal/ado/client.go` returns `0`.
- **Committed in:** `d9bc52b`

---

**Total deviations:** 2 auto-fixed (1 missing critical, 1 blocking)
**Impact on plan:** The security fix is the one behavioral addition beyond tags and is required by the plan's own threat register. Neither changed the field contract. No scope creep.

## Security Notes

- **T-02-01 (Information Disclosure, high):** `Failure` still carries exactly four fields — no URL, header, authorization value, or PAT. The exact-key-set assertion in `json_test.go` fails loudly if a fifth is ever added, and the `MarshalJSON` guard means a future error type cannot leak its internals through the `err` key either.
- **T-02-02 (Information Disclosure, medium):** The machine-readable failure error text is **intentionally untruncated**. The truncation applied for terminal rendering in 02-01 is a display concern; a machine-readable record is more useful at full fidelity. Keeping server-controlled text out of parseable output when it should be is Phase 3's OUT-03 stdout/stderr split, not this struct's job. Pinned by the "message is not truncated" subtest.
- **T-02-06 (Tampering / contract integrity, high):** The duration shape was chosen by a human before it was written down, and every field name is pinned by an exact-key-set test — so a later accidental rename is a failing test rather than a silent break.
- **T-02-SC:** Zero dependencies added. `git diff --stat go.mod go.sum` is empty.

## Issues Encountered

None. Both deviations were caught by the verification loop before the task commit.

## Verification Results

| Check | Result |
|---|---|
| `gofmt -l internal/ado` | empty |
| `go vet ./internal/ado/` | clean |
| `go test ./internal/ado/ -count=1` | ok |
| `go test ./internal/ado/ -run TestJSONFieldNames -count=1 -v` | pass, 15 subtests |
| Old gate: `TestGet[A-Za-z]*SwallowsProjectFailure$` | **0** (countdown complete) |
| New gate: `TestGet[A-Za-z]*ReportsProjectFailure$` | **8** |
| `git diff --stat go.mod go.sum` | empty |
| `git diff --stat -- . ':!internal/ado'` | empty (scope boundary held) |
| Repo-wide `go build ./... && go vet ./... && gofmt -l . && go test ./... -count=1` | green |

## Self-Check: PASSED

- `internal/ado/json_test.go` exists on disk.
- Commit `d9bc52b` (feat) and `27e1187` (test) both present in `git log`.
- All acceptance criteria for Tasks 2 and 3 re-run and passing (table above).
- No file under `internal/ui/` modified; `.planning/phases/02-honest-reads/02-VALIDATION.md` untouched.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- **Ready for 02-05**, the last plan in the phase. It owns `internal/ui/**` and the repo-wide phase-close gate. That gate was run here as a sequential-mode sanity check and is currently green, so 02-05 starts from a clean tree.
- The `internal/ado` JSON contract is complete and locked. Phase 3's `--json` should serialize `Result[T]` directly rather than defining a second envelope shape.
- **One item for Phase 3:** `Failure` now has a custom `MarshalJSON` but no matching `UnmarshalJSON`, so the record does not round-trip (`err` decodes to a string, not an `error`). This is correct for an output-only contract. If Phase 3 ever needs to read its own JSON back, that asymmetry has to be addressed deliberately.
- Deliverable D8 (field ordering) is flagged `human_judgment: true` — worth a look when Phase 3 first emits real `--json` output.

---
*Phase: 02-honest-reads*
*Completed: 2026-09-07*
