---
phase: 04-guarded-writes
plan: 03
subsystem: cli
tags: [go, cli, azure-devops, guarded-writes, dry-run, audit, go-ast]

requires:
  - phase: 04-01
    provides: runWrite's ten-step order, the write envelope, writeExitCodeFor, hoistLeadingPositionals, ado.ClassifyFailure, the audit sink
  - phase: 04-02
    provides: UpdateWorkItemStateWithEffort's nullable effort signature on the writeClient seam
  - phase: 01-test-seam-for-internal-ado
    provides: the unexported -API interfaces and struct-literal fake injection every pre-read test drives
provides:
  - "internal/ado/preread.go: GetPullRequestDetail, GetReleaseApprovalDetail, GetPipelineDefinitionDetail — by-id pre-reads that error rather than returning (nil, nil) for a missing target"
  - "result types PullRequestDetail, ReleaseApprovalDetail, PipelineDefinitionDetail"
  - "five new CLI write verbs: prs approve, prs complete, releases approve, releases reject, pipelines queue"
  - "writeOp: the per-verb half of a guarded write, so all six verbs share ONE runWrite body"
  - "--comment (releases verbs), --branch (pipelines queue, required), write --repo (prs verbs, arity one)"
  - "three-level --help covering every write verb plus the full 0-5 exit-code contract"
affects: [05-hardening, agent-capability-tiers, any future write verb]

actuals:
  tokens: 19000
  tasks: 3
  commits: 3

tech-stack:
  added: ["go/ast + go/parser (stdlib, test-only structural assertion)"]
  patterns:
    - "writeOp closure struct: per-verb differences supplied by the dispatch entry, guard order kept in one shared body"
    - "AST-based prohibition assertions instead of grep-negative criteria"
    - "nil fake closure as the 'no mutating call' assertion, extended across six verbs"

key-files:
  created:
    - internal/ado/preread.go
    - internal/ado/preread_test.go
    - cmd/adoBizInt/writeverbs_test.go
  modified:
    - cmd/adoBizInt/write.go
    - cmd/adoBizInt/dispatch.go
    - cmd/adoBizInt/flags.go
    - cmd/adoBizInt/usage.go
    - cmd/adoBizInt/fakes_test.go
    - cmd/adoBizInt/usage_test.go
    - cmd/adoBizInt/filter_test.go
    - cmd/adoBizInt/main_test.go

key-decisions:
  - "No interface entry was added to releaseAPI: the SDK has no by-id approval endpoint, so GetReleaseApprovalDetail calls the existing GetApprovals and filters by id, keeping the 'no uncalled interface entry' rule (ROADMAP SC4) satisfied by construction rather than by discipline."
  - "GetApprovals is called with statusFilter 'pending,approved,rejected' rather than the endpoint's pending-only default, because an already-approved approval is exactly the case D-03 must refuse as a no-op and would otherwise be invisible."
  - "runWrite was generalised over a writeOp struct (resolve / plan / mutate) rather than branching per verb inside the body, so the ten-step guard order exists once for all six verbs."
  - "pipelines queue emits one unconditional fieldChange (queued_build) because a queue is not an idempotent state change; its diff is never empty, so exit 4 is unreachable for it by construction."
  - "prs complete treats both 'completed' and 'abandoned' as terminal and refuses as a no-op, rather than sending a doomed request to ADO."
  - "The write --repo is listed in TestFilterFlagsRegisteredOnSupportedVerbsOnly's table for the two prs verbs: that test can only see the FlagSet, and the two --repo flags are the same spelling with different meanings."

patterns-established:
  - "writeOp: a new write verb is a struct literal plus a dispatch entry; it cannot opt out of a guard."
  - "TestDryRunIssuesNoMutatingCallAllVerbs carries a drift guard — a verb in writeSupportByCommand with no fixture fails the test."
  - "Structural prohibitions are asserted with go/ast, never with a grep-returns-nothing criterion."

requirements-completed: [CLI-03, SAFE-01, SAFE-02]

coverage:
  - id: D1
    description: "Three by-id pre-reads that return a real error for a missing target, never (nil, nil)"
    requirement: "SAFE-02"
    verification:
      - kind: unit
        ref: "internal/ado/preread_test.go#TestPreReadNotFoundReturnsError"
        status: pass
      - kind: unit
        ref: "internal/ado/preread_test.go#TestPreReadNeverReturnsNilNilStructural"
        status: pass
    human_judgment: false
  - id: D2
    description: "All five remaining mutations reachable through exactly one CLI verb each, with the tracer's full guard set"
    requirement: "CLI-03"
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/writeverbs_test.go#TestDryRunIssuesNoMutatingCallAllVerbs"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/main_test.go#TestDispatchTableMatchesD02"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/writeverbs_test.go#TestNoPrsMergeAlias"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every write verb dry-runs by default and mutates only with --confirm"
    requirement: "SAFE-01"
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/write_test.go#TestConfirmRegisteredOnEveryWriteVerbAndNoReadVerb"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/writeverbs_test.go#TestDryRunIssuesNoMutatingCallAllVerbs"
        status: pass
    human_judgment: false
  - id: D4
    description: "pipelines queue with no --branch is a parse error, exit 3, with no client call"
    requirement: "CLI-03"
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/writeverbs_test.go#TestDryRunIssuesNoMutatingCallAllVerbs/pipelines_queue/parse_errors_exit_3_with_no_client_call/missing_branch"
        status: pass
    human_judgment: false
  - id: D5
    description: "Three-level --help lists every write verb, states the dry-run default, and documents exit codes 4 and 5"
    verification:
      - kind: unit
        ref: "cmd/adoBizInt/usage_test.go#TestEveryWriteVerbAppearsInNounUsage"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/usage_test.go#TestWriteVerbUsageDocumentsConfirm"
        status: pass
      - kind: unit
        ref: "cmd/adoBizInt/usage_test.go#TestRootUsageDocumentsTheWriteExitCodes"
        status: pass
    human_judgment: false
  - id: D6
    description: "The published CLI wording of the six write verbs and their help text reads correctly to a human operator"
    verification: []
    human_judgment: true
    rationale: "Verb names and help prose are a one-way door (D-23); no test can judge whether the wording is what an operator would reach for."

duration: 26 min
completed: 2026-09-12
status: complete
---

# Phase 04 Plan 03: Guarded-write expansion Summary

**Five remaining mutations wired to `prs approve` / `prs complete` / `releases approve` / `releases reject` / `pipelines queue`, each sharing one `runWrite` body via a `writeOp` closure struct, backed by three new by-id pre-reads that error rather than returning `(nil, nil)` for a missing target.**

## Performance

- **Duration:** 26 min
- **Started:** 2026-09-12T02:21Z
- **Completed:** 2026-09-12T02:47Z
- **Tasks:** 3
- **Files modified:** 11 (3 created, 8 modified)

## Accomplishments

- **`internal/ado/preread.go`** — a new file, not 190 more lines on the already-643-line `workitems.go`. Three by-id pre-reads whose defining deviation from `GetWorkItemDetail` is that a missing target is an **error**, closing threat T-04-13: a caller can never read a missing PR/approval/definition as an empty diff and a silently refused no-op.
- **No new `-API` interface entry.** `GetPullRequest`, `GetDefinition` and `GetApprovals` already existed; the SDK exposes no by-id approval endpoint, so `GetReleaseApprovalDetail` filters the list. The "no uncalled interface entry" rule (ROADMAP SC4) holds without a new fake method.
- **`runWrite` generalised over `writeOp`** (`resolve` / `plan` / `mutate`). The ten-step order — `openAuditSink` before the pre-read, empty-diff refusal before the `--confirm` branch — now exists **once** for six verbs. A guard in a shared body cannot be forgotten on verb seven.
- **`TestPreReadNeverReturnsNilNilStructural`** parses `preread.go` with `go/parser` and walks the three function bodies with `ast.Inspect`, failing on any `return nil, nil`. This replaces the fragile grep-negative criterion STATE.md records being defeated three times in Phase 3 — comments and string literals are not in the AST, so no prose can satisfy or defeat it.
- **`TestDryRunIssuesNoMutatingCallAllVerbs`** drives six verbs × six guard rows from one fixture table, with a drift guard that fails if a verb in `writeSupportByCommand` has no fixture.
- **Help now teaches the contract**: root usage documents all six exit codes including 4 and 5; every write verb's noun-level and verb-level text says it previews by default and writes only with `--confirm`.

## Task Commits

1. **Task 1: By-id pre-reads** — `f5c63f1` (feat)
2. **Task 2: Register the five remaining write verbs on `runWrite`** — `6459391` (feat)
3. **Task 3: Three-level `--help` coverage** — `1e6e3b6` (docs)

## Files Created/Modified

- `internal/ado/preread.go` — the three by-id write pre-reads and their detail types
- `internal/ado/preread_test.go` — happy-path + not-found per method, plus the `go/ast` structural prohibition
- `cmd/adoBizInt/write.go` — `writeOp`, generalised `runWrite`, the six per-verb op constructors, `oneProject` / `oneRepo`
- `cmd/adoBizInt/dispatch.go` — five `commands` entries; `writeClient` grew the three pre-reads and five mutations
- `cmd/adoBizInt/flags.go` — five `writeSupportByCommand` entries; `--comment`, `--branch`, write `--repo` registration
- `cmd/adoBizInt/usage.go` — `verbSummaries`, exit-code section, write notice at verb level
- `cmd/adoBizInt/writeverbs_test.go` — the six-verb fixture table and the per-verb semantic tests
- `cmd/adoBizInt/fakes_test.go`, `usage_test.go`, `filter_test.go`, `main_test.go` — fake methods and expectation-table updates

## Decisions Made

See `key-decisions` in the frontmatter. The two with the longest reach:

- **`statusFilter` widened past the endpoint default.** `GetApprovals` defaults to `pending`; an approval already `approved` would have been absent from the list and the pre-read would have reported "not found" for a target that exists and is precisely the D-03 no-op case. Passing `"pending,approved,rejected"` (the REST API's flags enum, carried through as a string by the SDK) makes the refusal reachable.
- **`pipelines queue` has no no-op condition, stated in code rather than assumed.** A queue is not an idempotent state change, so its diff func emits one `queued_build` change unconditionally and exit 4 is unreachable for that verb. `TestQueueHasNoNoOpCondition` pins it so a later "optimisation" comparing the branch to the definition's default cannot silently turn a legitimate queue into a refusal.

## Deviations from Plan

None - plan executed exactly as written.

Two things worth recording that are *not* deviations, because the plan explicitly left them open:

- The plan said "if the SDK offers a by-id `GetApproval` ... add exactly that one entry; otherwise filter the list and add no interface entry at all." The SDK offers none (`release/client.go` has `GetApprovals` only), so the second branch was taken. `internal/ado/interfaces.go` is byte-identical to its pre-plan state.
- `cmd/adoBizInt/write.go` grew from 350 to ~470 lines, past CONVENTIONS.md's 300-line standard. The per-verb op constructors were kept in `write.go` rather than split out deliberately: Task 2's acceptance criterion "the `openAuditSink(` line number is still less than every mutate-call line number" is only meaningful while both live in one file, and it now holds (196 < 305, 337, 370, 405, 448). The package is already well past that standard (`main_test.go` is 842 lines); a split should be a considered pass over the whole `cmd/` package, not a side effect of this plan.

## Issues Encountered

- `TestFilterFlagsRegisteredOnSupportedVerbsOnly` and `TestDispatchTableMatchesD02` both failed on first run after the five verbs were registered. Both are **deliberate drift guards**, not breakage: their tables are written out longhand precisely so a new published verb cannot appear without a human writing it down. Both were updated with the new keys, and the `--repo` entry for the two `prs` verbs carries a comment explaining that the flag is the same spelling with a different meaning on a write.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- CLI-03 is satisfied: all seven `internal/ado` mutations are reachable, through six verbs (`UpdateWorkItemState` and `UpdateWorkItemStateWithEffort` share `workitems set-state` per D-19).
- ROADMAP SC1, SC2, SC3 and SC5 now hold for every write verb rather than only the tracer.
- Verification: `go build ./... && go test ./... && go vet ./... && gofmt -l .` all clean. No new `http.` reference in any test file — every guard drives the Phase 1 seam (ROADMAP SC6).
- **Carried into Phase 5 unchanged:** the audit file's `0600`/`0700` permissions (HARD-06), and the read-then-write race D-03 narrows but does not eliminate. Neither is new to this plan.
- **Open, pre-existing:** `GetWorkItemDetail` still returns `(nil, nil)` for a missing item. `setStateOp`'s `plan` closure checks the nil explicitly rather than trusting it. Aligning that method with `preread.go`'s contract would be a small, well-isolated follow-up.

## Self-Check: PASSED

- `internal/ado/preread.go`, `internal/ado/preread_test.go`, `cmd/adoBizInt/writeverbs_test.go` all present on disk.
- Commits `f5c63f1`, `6459391`, `1e6e3b6` all present in `git log`.
- All task-level `<acceptance_criteria>` re-run and passing, including the two source-level ones (`internal/ado/interfaces.go` and `internal/ado/workitems.go` unchanged by `git diff`; `openAuditSink(` at line 196 precedes every mutate call).
- Plan-level `<verification>` re-run: build, test, vet, gofmt clean; the `http.` grep returns only two pre-existing comment lines in `internal/ado/client_test.go`.

---
*Phase: 04-guarded-writes*
*Completed: 2026-09-12*
