---
phase: 02-honest-reads
verified: 2026-10-12T16:08:14Z
status: passed
score: 5/6 success criteria verified
covered_files:

  - .planning/REQUIREMENTS.md
  - .planning/phases/02-honest-reads/02-01-SUMMARY.md
  - .planning/phases/02-honest-reads/02-01-tracer-honest-read-PLAN.md
  - .planning/phases/02-honest-reads/02-02-SUMMARY.md
  - .planning/phases/02-honest-reads/02-02-expand-aggregate-reads-PLAN.md
  - .planning/phases/02-honest-reads/02-03-SUMMARY.md
  - .planning/phases/02-honest-reads/02-03-dashboard-partial-counts-PLAN.md
  - .planning/phases/02-honest-reads/02-04-SUMMARY.md
  - .planning/phases/02-honest-reads/02-04-json-field-contract-PLAN.md
  - .planning/phases/02-honest-reads/02-05-SUMMARY.md
  - .planning/phases/02-honest-reads/02-05-tui-failure-surfacing-PLAN.md
  - .planning/phases/02-honest-reads/02-06-PLAN.md
  - .planning/phases/02-honest-reads/02-06-SUMMARY.md
  - .planning/phases/02-honest-reads/02-07-PLAN.md
  - .planning/phases/02-honest-reads/02-07-SUMMARY.md
  - internal/ado/branches.go
  - internal/ado/branches_test.go
  - internal/ado/client.go
  - internal/ado/dashboard.go
  - internal/ado/pipelines.go
  - internal/ado/pipelines_test.go
  - internal/ado/pullrequests.go
  - internal/ado/releases.go
  - internal/ado/releases_test.go
  - internal/ado/result.go
  - internal/ado/workitems.go
  - internal/ado/workitems_test.go
  - internal/config/config.go
  - internal/ui/dashboard/dashboard.go
  - internal/ui/fetchctx/fetchctx.go
  - internal/ui/styles/styles.go

covered_digest: "v1:sha256:08bf70abc65e383eca1b6f619971273c12a24212ff43e0e47355515abbf3329c"
behavior_unverified: 1
overrides_applied: 0
verdict: HUMAN-NEEDED
requirements:
  TRUST-01: satisfied # per-definition GetBuilds/GetReleases/GetDefinition failures recorded (02-06); iteration lookup recorded (02-07)
  TRUST-02: satisfied # per-definition 401/403 classified auth via classifyFailure (value and pointer forms tested)
  TRUST-03: satisfied # complete:false over any failed sub-request; branches no longer complete:false over empty/disabled repos (D-06a/D-06b, live 886 items complete:true)
  TRUST-04: satisfied # D-12 pre-flight now inside all three N+1 loops
  OUT-02: satisfied   # TestJSONFieldNames green; no struct/tag changes in 02-06/02-07
re_verification:
  previous_status: gaps_found
  previous_score: 3/6
  previous_verified: 2026-10-12T14:59:29Z
  gaps_closed:
    - "Every aggregate read reports which projects failed and why, rather than dropping errors with continue (SC1) / A partial result is explicitly flagged incomplete (SC3): per-definition N+1 swallows in GetPipelineRuns, GetReleases and GetPipelineDefinitions now recorded (02-06)"
    - "An empty repository is not reported as a branch failure: recognised by 400 + VS403403: (D-06a), disabled repositories by 404 + typeKey GitRepositoryNotFoundException (D-06b) (02-07 + f73cfa4)"
    - "Sub-request failures inside an otherwise-succeeding project are reported, including a degraded iteration filter (T-02-05): opGetWorkItemsIterations recorded (02-07)"
  gaps_remaining: []
  regressions: []
prohibitions:

  - statement: "02-06: Must NOT stop issuing per-definition calls after one definition returns an auth failure (D-08)"
    verification: judgment
    status: verified
    flagged: false
    evidence: "All three per-definition error branches append and continue (pipelines.go:219-220, :373-374; releases.go:167-168); no early return on auth"
  - statement: "02-06: Must NOT add a field, json tag, or new Status value to PipelineRun, Release or PipelineDefinition"
    verification: judgment
    status: verified
    flagged: false
    evidence: "git diff 428ea39..HEAD on pipelines.go/releases.go adds no struct field, tag or status literal; TestJSONFieldNames 17/17"
  - statement: "02-06: Must NOT add a nil-Id guard to the GetPipelineDefinitions loop; nilguard_test.go not edited"
    verification: judgment
    status: verified
    flagged: false
    evidence: "Loop still guards d.Name only (pipelines.go:331-334); nilguard_test.go absent from git diff --stat 428ea39..HEAD; TestCollectedDefinitionsGuardNameAndID passes"
  - statement: "02-07: Must NOT classify, suppress or recognise failures outside the branch-stats call by message text"
    verification: judgment
    status: verified
    flagged: false
    evidence: "The only err.Error() match in non-test internal/ado code is branches.go:82 (unreadableRepository)"
  - statement: "02-07: Must NOT change result.go, classifyFailure, statusCodeOf, the WIQL text, iterationClause, GetRecentIterations, or the dashboard"
    verification: judgment
    status: unverified
    flagged: true
    evidence: "Breached in letter, not in effect, by the orchestrator's post-plan D-06b commit f73cfa4 (not by 02-07 itself): statusCodeOf was split into wrappedErrorOf + statusCodeOf. classifyFailure is untouched and TestClassifyFailure (16) passes. One theoretical difference: if an error chain held a value-form WrappedError with nil StatusCode AND a pointer-form one with a status, the old code fell through to the pointer and the new code does not. WrappedError has no Unwrap, so this needs a custom multi-error chain; not reachable from the SDK. Non-authoritative LLM verdict: acceptable. Human acknowledgement recommended."
behavior_unverified_items:

  - truth: "The TUI renders failures rather than silently showing an empty table"
    test: "Run the tool with a deliberately invalid/expired PAT that is valid for at least one configured project and rejected for another; open each of the six views."
    expected: "An auth banner replaces the table for the rejected project; partial warnings appear where some projects answered; no view shows a bare empty table."
    why_human: "View() returns a lipgloss-styled string. Substring assertions exist in all six packages but a substring assertion is not a visual check. 02-VALIDATION.md marks this row NOT DONE. The user has chosen to record it as verification debt."
human_verification:

  - test: "TUI failure rendering (verification debt, user-acknowledged): run with a PAT valid for one configured project and rejected for another; open all six views."
    expected: "Auth banner replaces the table for the rejected project; partial warnings where some projects answered; no bare empty table. A per-definition 401/403 now shows the pipelines/releases auth banner rather than a grey never-run/no-releases row."
    why_human: "View() output is a lipgloss-styled string; substring tests are not a visual check."
  - test: "Acknowledge the result.go refactor in f73cfa4 (wrappedErrorOf extracted from statusCodeOf) against 02-07's prohibition on changing result.go."
    expected: "Accept as a behavior-preserving extraction made under D-06b, or ask for it to be reverted into branches.go."
    why_human: "Judgment-tier prohibition breached in letter by a post-plan orchestrator commit; the verifier's verdict is non-authoritative."
---

# Phase 2: Honest Reads — Verification Report

**Phase Goal:** A caller can distinguish an empty result from a failed one
**Verified:** 2026-10-12T16:08:14Z (re-verification 2 on `dev` @ `bd3f348`)
**Status:** human_needed
**Re-verification:** Yes (second today), after gap-closure plans 02-06 and 02-07 plus the orchestrator's D-06b follow-up. All three gaps from the first 2026-10-12 re-verification are closed in code and tests. The only open item is the user-acknowledged TUI visual check.

The original 2026-09-07 report, its Gap Closure section, and the first 2026-10-12 re-verification are preserved unchanged below, followed by Re-verification 2.

---

## Prior verification (2026-09-07), preserved

**Verified:** 2026-09-07
**Verdict:** PASS-WITH-GAPS

### Success Criteria

| # | Criterion | Status | Evidence |
|---|-----------|--------|----------|
| 1 | Every aggregate read reports which projects failed and why | ✓ VERIFIED | Targeted scan of all five aggregate read files: every `if err != nil { ... continue }` is preceded by `appendFailure`. The residual `continue` statements are nil-guards, repo/name filters, or the D-12 pre-flight `ctx.Err()` record — none drop an error. 10 `TestGet*ReportsProjectFailure` tests run. |
| 2 | An expired PAT produces a distinct fatal error, never an empty list | ⚠️ PARTIAL | `classifyFailure` → `auth` on 401/403 confirmed; 7 `TestGet*ClassifiesRejectedToken` tests. Known unreachable path documented (see Scrutiny 2). |
| 3 | A partial result is explicitly flagged incomplete | ⚠️ DEFECT | `Complete` derived centrally in `newResult`; all six views set a warning from `Failures`. But the dashboard's headline text is wrong for one failure class (see Scrutiny 4). |
| 4 | Every ADO call carries a deadline and `connection.Timeout` is set | ✓ VERIFIED | `TestNewClientSetsTimeout`, `TestLoadAppliesTimeoutDefaults` (4), `TestFetchTimeoutMarksUnreached` (6 reads). `internal/ui/fetchctx` bounds every read fan-out. |
| 5 | Exported `ado` structs carry `json:` tags with stable lower-cased names | ✓ VERIFIED | `TestJSONFieldNames` — 17 tests across 11 structs. |
| 6 | The TUI renders failures rather than silently showing an empty table | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | All six views wire `authBanner` + `partialWarning`; 6 `TestUpdateAuthFailureBlocksThe*` tests pass. No human has looked at rendered output. |

**Score:** 5/6 verified (1 behavior-unverified). One criterion (3) carries a real defect.

> **2026-10-12 correction to row 1:** the claim "every `if err != nil { ... continue }` is preceded by `appendFailure`" is false for `pipelines.go:198-203` and `releases.go:148-153`. See the re-verification section, gap 1.

### Scrutiny Findings

#### 1. `statusCodeOf` value + pointer — CONFIRMED CORRECT

`internal/ado/result.go` tries `azuredevops.WrappedError` (value) first, then `*azuredevops.WrappedError`, each with a `StatusCode != nil` guard. The doc comment cites the four SDK call sites that motivate it. A pointer-only implementation would indeed have degraded a rejected PAT to `FailureOther` silently; that has not happened. `TestClassifyFailure` runs 16 cases including both forms.

#### 2. Startup-fatal PAT rejection — HONESTLY LOGGED, TRUST-02 defensible

02-01-SUMMARY.md records this in three places: a deviation entry ("Issue / Resolution: logged, not fixed"), the scope-call list, and a Follow-up Items table row targeting "Phase 02 expansion or Phase 03 CLI". This is not a buried caveat.

The per-project case — a PAT valid for one project and rejected for another — IS covered: 7 `TestGet*ClassifiesRejectedToken` tests plus the six view reducer tests. That is the case the phase goal actually names ("distinguish an empty result from a failed one" is a per-project distinction).

**Judgment: TRUST-02 can honestly be called met.** The totally-rejected-PAT case is loud and fatal today — it produces `Failed to create ADO client`, which is exactly "not an empty list". It is ugly and unclassified, not silent. The requirement text is "HTTP 401/403 are fatal and never render as an empty result set"; both paths satisfy it.

#### 3. T-02-01 leak guard — HOLDS

- `Failure.Err` is tagged `json:"-"` and a value-receiver `MarshalJSON` re-emits a flattened `*string`. The struct carries no URL/header/token field.
- `TestJSONFailureErrIsFlattenedString` is a genuine guard, not a shape check: it plants a `leakyError{RequestURL: "https://dev.azure.com/o/_apis?token=SECRET"}` and asserts the raw bytes contain none of `RequestURL`, `request_url`, `SECRET`, `dev.azure.com`. That is the real assertion asked for.
- Truncation is centralised, not per-view: `internal/ui/styles` defines `maxFailureMessage = 120` and applies `utils.Truncate` in all four renderers (`RenderAuthBanner`, `RenderPartialWarning`, `RenderUnderstatedWarning`, and via them `SummarizeFailures`). All six views route their server-controlled text through those renderers. **Better than the "prs only" concern implied** — no view formats failure text itself.

#### 4. Dashboard render + the understated/overstated label — REAL DEFECT

**Render wiring: correct.** `internal/ui/dashboard/dashboard.go` reads `result.Failures`, sets `authBanner` (line 147) or `partialWarning` (151), and `View()` emits them (316, 322). Because `partialWarning` is set whenever any non-auth failure exists, a partial total cannot render unflagged.

**Minor smell:** `m.complete` (line 89) is assigned at 136 and **never read**. Correctness rests entirely on `partialWarning` being derived from `Failures`. Functionally equivalent, but the field is dead and invites a future reader to think it is load-bearing.

**The defect (the item flagged for scrutiny is a genuine inconsistency):**

```go
parts := []string{"totals understated"}   // line 166 — unconditional
if len(unanswered) > 0 { ... }
if len(degraded)   > 0 { parts = append(parts, "counted unfiltered by iteration: "+ ...) }
```

The function's own doc comment says degraded iteration lookups "skew the total upward, not downward". 02-03 found the same: a failed iteration lookup drops the `IN (...)` clause and **overstates** the work-item total. Yet when the *only* failure is `opDashboardIterations`, the user is shown:

> `totals understated; counted unfiltered by iteration: <project> (other: dashboard iterations)`

The headline states the opposite of the truth. The cause clause is present and correct, so a careful reader can work it out — but this is the phase whose entire point is that the tool does not lie about its numbers. Stating "understated" over an overstated number is exactly the failure mode Phase 2 exists to remove.

**Severity: WARNING, not BLOCKER.** The number is still explicitly flagged as unreliable and the cause is named; no silent partial. But it should be fixed before the phase is called done.

#### 5. Self-contradictory acceptance criteria — RESOLVED HONESTLY, NOT GAMED

- **`continue` counts (02-02):** the criterion was reported unsatisfiable rather than met. Verified independently: the surviving `continue` statements are structurally benign (nil guards, filters, D-12 pre-flight records). No error is dropped anywhere in the five aggregate read files. The count criterion was wrong; the behavior is right.
- **`context.Background()` counts (02-05):** resolved by extracting `internal/ui/fetchctx`, a real behavioral change — every read fan-out now runs under a `WithTimeout` with a non-zero fallback (`defaultBudget = 60s`, applied when budget ≤ 0, so an unconfigured budget cannot mean unbounded). Not a grep dodge.

**Residual note on TRUST-04:** 13 `context.Background()` call sites remain in `internal/ui`. Most are write paths (Phase 4 scope, correctly deferred). Three are *reads* — `GetWorkItemDetail`, `GetChildWorkItems`, `DownloadAttachment`. These are single-target reads, not aggregates, and are bounded at the transport layer by `connection.Timeout`, so TRUST-04's literal text still holds. They carry no *fan-out* budget and no cancellation. Worth a Phase 5 line item; not a Phase 2 gap.

#### 6. `internal/ui/fetchctx` untested — ACCEPTABLE

The package is 33 lines, one exported function, one branch (`budget <= 0`). It is exercised indirectly by every view test. **Acceptable coverage gap**, but the zero/negative-budget fallback is the one behavior worth pinning, since regressing it silently restores the unbounded context the package was created to remove. Recommend a three-line test; not a blocker.

#### 7. 02-VALIDATION.md honesty — VERIFIED, ALL COMMANDS MATCH REAL TESTS

Every automated command in the verification map was re-run independently. All return non-zero test counts matching the recorded numbers:

| Command pattern | Tests matched | Recorded |
|---|---|---|
| `TestGet.*ReportsProjectFailure` | 10 | 10 ✓ |
| `TestGetWorkItemsReportsBatchFailure` | 2 | 2 ✓ |
| `TestGetBranchesReportsRepoFailure` | 3 | 3 ✓ |
| `TestGet(Project\|Dashboard)Counts.*Partial\|...ReportsProjectFailure` | 7 | 7 ✓ |
| `TestClassifyFailure` | 16 | 16 ✓ |
| `TestGet.*ClassifiesRejectedToken` | 7 | 7 ✓ |
| `TestResultCompleteness` | 4 | 4 ✓ |
| `TestNewClientSetsTimeout` | 1 | 1 ✓ |
| `TestLoadAppliesTimeoutDefaults` | 4 | 4 ✓ |
| `TestFetchTimeoutMarksUnreached` | 6 | 6 ✓ |
| `TestJSONFieldNames` | 17 | 17 ✓ |
| `TestUpdateAuthFailureBlocksThe*` (ui) | 6 | green ✓ |

Phase 1 countdown gate re-run independently: returns **0**. Confirmed.

The two originally-zero-matching commands were corrected to real test names rather than left to pass vacuously, and the correction is disclosed in the sign-off block. The manual row "Rendered TUI output actually shows the failure banner" is marked **NOT DONE, and deliberately not claimed**, with a correct explanation of why a substring assertion is not a substitute. **This validation file is honest.**

#### 8. Requirement marking — DISCREPANCY PERSISTS

`.planning/REQUIREMENTS.md:222` still reads `| TRUST-01 .. TRUST-04 | Phase 2 | Pending |` while lines 15–18 are all `[x]`. Line 223 (`OUT-02`) correctly reads Complete. 02-02's prediction was accurate and the row was never fixed. Bookkeeping only, but the file now contradicts itself.

*(Unrelated observation: `SEAM-04` at line 27 — "aggregate read failure paths from TRUST-01 are covered by tests" — is still `[ ]` but is now arguably satisfied by this phase's 10+7+6 failure-path tests. Phase 1 scope; flagged for the milestone audit, not this phase.)*

### Anti-Patterns

No `TODO`, `FIXME`, `XXX`, or `HACK` markers in the phase's source files. `gofmt`, `go vet`, `go build`, `go test ./... -count=1` all green (regression gate re-confirmed by the module-wide test runs above).

### Verdict

**PASS-WITH-GAPS.**

The phase goal — a caller can distinguish an empty result from a failed one — is achieved. The result envelope is real, every aggregate read populates it, the classifier handles both SDK error shapes, the JSON leak guard is a genuine adversarial test, and all six views surface failures. The execution artifacts are unusually honest: two unsatisfiable acceptance criteria were reported as unsatisfiable rather than gamed, a known unreachable auth path was logged rather than buried, and two vacuous validation commands were fixed rather than allowed to pass proving nothing.

**Nothing here blocks the phase, but one item should be fixed before it is signed off:**

- 🟡 **Dashboard says "totals understated" over an overstated number** (`dashboard.go:166`). The one place in this phase where the tool states something untrue about its own data. Small fix, high symbolic cost if left.
- 🟡 **REQUIREMENTS.md:222 traceability row contradicts its own checkboxes.**
- ⚪ Recommended: a `fetchctx` zero-budget test; delete or use the dead `m.complete` field.
- ⚪ Open: human visual check of the six views against an invalid PAT — the correct subject for `/gsd-verify-work`.

_Verified by gsd-verifier. Read-only apart from this file._

### Gap Closure

Both blocking gaps closed on 2026-09-07, on `dev`, TDD with atomic commits.

| Gap | Commits | Outcome |
|-----|---------|---------|
| Dashboard headline asserted the wrong skew direction (`dashboard.go:166`) | `93acdf2` (RED tests), `6f8a98b` (fix) | Headline now derived from the degraded/unanswered split |
| REQUIREMENTS.md:222 range row contradicted its checkboxes | `762f03e` | Range split into four per-ID rows, all `Complete` |

**Headline wording now produced by `summarizeCountFailures`:**

| Failure mix | Headline |
|-------------|----------|
| Unanswered projects only | `totals understated` |
| Degraded iteration lookups only | `totals overstated` |
| Both present | `totals unreliable, direction unknown` |

The existing cause clauses (`did not answer: ...`, `counted unfiltered by iteration: ...`) are unchanged. `TestSummarizeCountFailuresHeadlineMatchesSkewDirection/degraded_iteration_lookups_alone_are_overstated` asserts the string does NOT contain `understated`, pinning the inversion so it cannot be reintroduced silently.

**`m.complete`:** left in place. It is unread by production code, but `dashboard_test.go:137` reads it, so removal ripples outside `dashboard.go`. Deferred rather than widened into this gap-closure scope.

**Suite:** `go build ./...`, `go vet ./...`, `gofmt -l .` (clean), `go test ./... -count=1` all pass. `internal/ado/result.go` untouched.

**Still open:** the human visual check of the six views against an invalid PAT (behavior-unverified item above) remains the subject for `/gsd-verify-work`.

---

## Re-verification (2026-10-12)

**Scope:** HEAD of `dev` (`ccba07b`), after Phases 3, 4 (partial) and 5 touched the same read paths. Every roadmap success criterion and the gap-closure claims were re-checked against code, not against the SUMMARY files. No live ADO calls were made. The live results cited below are the user's 2026-10-12 run.

### Gate

| Check | Result |
|-------|--------|
| `go build ./...` | pass |
| `go vet ./...` | pass |
| `go test -race -count=1 ./...` | pass: 13 packages ok, 724 `--- PASS`, 0 `--- FAIL` |
| `test -z "$(gofmt -l .)"` | pass (empty) |

### Gap-closure check

| Prior gap | Status on HEAD | Evidence |
|-----------|----------------|----------|
| Dashboard headline asserted the wrong skew direction | ✓ CLOSED | `6f8a98b` is an ancestor of HEAD. `summarizeCountFailures` (`internal/ui/dashboard/dashboard.go:163-189`) chooses `totals understated` / `totals overstated` / `totals unreliable, direction unknown` from the degraded/unanswered split. `TestSummarizeCountFailuresHeadlineMatchesSkewDirection` and its 3 subtests pass under `-race`. The op label `"get dashboard iterations"` matches in both `internal/ado/dashboard.go:25` and `internal/ui/dashboard/dashboard.go:124`. |
| REQUIREMENTS.md traceability contradicted its checkboxes | ✓ CLOSED | `.planning/REQUIREMENTS.md:226-230`: TRUST-01, TRUST-02, TRUST-03, TRUST-04 and OUT-02 are each a separate row reading `Phase 2 / Complete`, consistent with the `[x]` checkboxes at lines 15-18. |

### Observable truths (roadmap success criteria) on HEAD

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Every aggregate read reports which projects failed and why, rather than dropping errors with `continue` | ✗ FAILED | Project-level swallows are all inverted (10 `TestGet*ReportsProjectFailure` pass). But two per-definition N+1 calls still drop their error with `continue` and record nothing: `internal/ado/pipelines.go:198-203` (GetBuilds per definition) and `internal/ado/releases.go:148-153` (GetReleases per definition). Probe evidence is below. |
| 2 | An expired PAT produces a distinct fatal error, never an empty list | ✓ VERIFIED | `classifyFailure` 401/403 → `auth` (`TestClassifyFailure` 16, `TestGet*ClassifiesRejectedToken` 7). An expired PAT fails the first per-project call (definitions, repositories, WIQL), which is recorded. Caveat: a 401/403 on the *per-definition* call alone renders as `neverRun`/`noReleases`, which is covered by gap 1. |
| 3 | A partial result is explicitly flagged incomplete | ✗ FAILED | `newResult` derives `Complete` from failures, and the dashboard headline is now truthful. But because of gap 1, GetPipelineRuns/GetReleases return `complete:true` over a failed sub-request, and the CLI exits 0 (`exitCodeFor`, `cmd/adoBizInt/exit.go:19-27`). Same root cause as #1. |
| 4 | Every ADO call carries a deadline and `connection.Timeout` is set | ✓ VERIFIED | `newConnection` sets `connection.Timeout` before any sub-client is built (`internal/ado/client.go:54-61`). Defaults are 30s/2m (`internal/config/config.go:106-110`), and `Validate` requires `fetch_timeout > poll_interval`. All eight aggregate reads in the TUI run under `fetchctx.New(budget)` (6 views, 8 sites). `TestNewClientSetsTimeout`, `TestLoadAppliesTimeoutDefaults` (4), `TestFetchTimeoutMarksUnreached` (6) and the new `TestNewWithParentZeroBudgetStillHasDeadline` pass. The prior recommendation for a fetchctx zero-budget test is closed. Note that the D-12 *recording* of mid-loop expiry is broken inside the per-definition loops. That is part of gap 1; the deadline itself is present. |
| 5 | Exported `ado` structs carry `json:` tags with stable lower-cased names | ✓ VERIFIED | `TestJSONFieldNames` 17/17 and `TestJSONFailureErrIsFlattenedString` 4/4 pass. The T-02-01 four-key `Failure` contract is unchanged. |
| 6 | The TUI renders failures rather than silently showing an empty table | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | All six views still wire `authBanner`/`partialWarning`, and `TestUpdateAuthFailureBlocksThe*` (6) passes. No human visual check has been done; the user recorded this as verification debt. Gap 1 also makes the pipelines/releases views show a dim-grey "never run"/"no releases" row for a failed fetch (`internal/ui/pipelines/pipelines.go:584`, `internal/ui/releases/releases.go:590`). |

**Score:** 3/6 verified (1 present, behavior-unverified; 2 failed, both from one root cause).

### Phase 3-5 regression check on Phase 2 guarantees

- **Phase 5 WIQL `@project` change** (`933466b`, `df05d1e`): `getProjectCounts` still records `opDashboardIterations`, `opDashboardWorkItems`, `opDashboardPullRequests` and `opDashboardBuilds` failures, and still returns partial counts (D-03). `getProjectWorkItems` still records batch failures (`opGetWorkItemsBatch`). Removing the WIQL-text logging did not remove any failure recording. No regression.
- **Phase 3 CLI:** `exitCodeFor` maps auth → 2 and `!complete` → 1, so it consumes the envelope correctly. It can only be as honest as the envelope, which gap 1 undermines for two verbs.
- **D-07 kind set:** still four kinds (`result.go`). The `StatusCodeOf`/`ClassifyFailure` exports added later are thin wrappers that do not change classification.

### Behavioral evidence (verifier probe, working tree untouched)

A throwaway test was injected with `go test -overlay` from the session scratchpad. It used the package's own fakes and was not written to the repo. `git status` was unchanged afterwards.

| Probe | Injected error | Result | Status |
|-------|----------------|--------|--------|
| GetPipelineRuns, per-definition GetBuilds | `WrappedError{StatusCode:401}` | `items=1 status="neverRun" failures=0 complete=true` | ✗ FAIL: an auth failure renders as an empty state |
| GetReleases, per-definition GetReleases | `context.DeadlineExceeded` | `items=1 status="noReleases" failures=0 complete=true` | ✗ FAIL: a timeout renders as an empty state |
| GetBranches, per-repo GetBranches | VS403403 message with status 404 | `failures=0 complete=true` | suppressed (as designed) |
| GetBranches, per-repo GetBranches | VS403403 message with status 400 / 409 | `failures=1 kind="other" complete=false` | recorded: matches the live 5-failure result |
| GetWorkItems, GetRecentIterations | `WrappedError{StatusCode:500}` | `failures=0 complete=true` | ⚠️ silent degradation (gap 3) |

### Live evidence assessment (user's 2026-10-12 run)

- `workitems` / `dashboard` / `prs` / `pipelines` / `releases`: `complete:true`, 0 failures. This is consistent with Phase 2 but does not exercise gap 1, which needs a per-definition failure to show.
- `branches list`: `complete:false`, 5 VS403403 failures for empty repositories, the same on the pre-Phase-5 build. **This conflicts with the Phase 2 decision.** 02-02 chose to suppress empty repositories by status 404 exactly (to keep a 403 classified as auth). The SDK always populates `StatusCode` (`azuredevops@v1.0.0-b5/client.go:366,388,394,405`), and the 404 suppression demonstrably works (probe row 3). So ADO answers VS403403 with a status other than 404, and assumption A4 is falsified. **Judgment: recorded as a gap (status partial), not a follow-up.** It falsifies a written 02-02 must-have truth with live evidence. It also makes `branches list` exit 1 on every run against this org. The noise errs in the safe direction (no failure is lost), so it ranks below gap 1. The fix needs one captured response (status plus `TypeKey`/`ErrorCode`) to suppress structurally, not by message text (D-06).

### Requirements coverage

| Requirement | Status | Evidence |
|-------------|--------|----------|
| TRUST-01 | ✗ BLOCKED | Two per-definition swallows remain (gap 1) |
| TRUST-02 | ✓ SATISFIED | Project-level classification verified; per-definition case folded into gap 1 |
| TRUST-03 | ✗ BLOCKED | `complete:true` over a failed sub-request in two reads (gap 1); branches `complete:false` over a complete read (gap 2) |
| TRUST-04 | ✓ SATISFIED | Both budgets set and wired; the mid-loop D-12 recording defect is tracked under gap 1 |
| OUT-02 | ✓ SATISFIED | `TestJSONFieldNames` 17/17 |

REQUIREMENTS.md marks TRUST-01 and TRUST-03 `[x]`/Complete. That is now ahead of the code and should revert to Pending until gap 1 closes. The verifier did not edit REQUIREMENTS.md.

### Deferred check

Later phases: Phase 3 (complete), Phase 4 (Guarded writes; write paths only), Phase 5 (complete). None names per-definition read failures, empty-repository classification, or the work-item iteration degradation. No gap is deferred.

### Anti-patterns and info

| File | Line | Finding | Severity |
|------|------|---------|----------|
| `internal/ado/pipelines.go` | 198-203 | error swallowed with `continue`, logged at Debug only | 🛑 Blocker (gap 1) |
| `internal/ado/releases.go` | 148-153 | error swallowed with `continue`, logged at Debug only | 🛑 Blocker (gap 1) |
| `internal/ado/workitems.go` | 138-141 | degraded filter not recorded | ⚠️ Warning (gap 3) |
| `internal/ui/dashboard/dashboard.go` | 127-128 | `applyResult` doc comment still says the warning "states they are understated"; it is now direction-aware | ℹ️ Info |
| `internal/ui/dashboard/dashboard.go` / `internal/ado/dashboard.go` | 124 / 25 | the `"get dashboard iterations"` label is duplicated across packages, and no test links them. If the ado label is renamed, the headline silently falls back to "understated". | ℹ️ Info |
| `internal/ui/dashboard/dashboard.go` | 89 | `m.complete` is still unread by production code (carried from 2026-09-07) | ℹ️ Info |
| `internal/ui/workitems/detail.go`, `workitems.go` | 80, 252 | single-target reads on `context.Background()` (carried; bounded only by `connection.Timeout`) | ℹ️ Info |

No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK` markers in the phase's source files.

### Human verification (verification debt, user-acknowledged)

1. **TUI failure rendering:** run with a PAT that is valid for one configured project and rejected for another, then open all six views. An auth banner should replace the table for the rejected project, partial warnings should appear where some projects answered, and no view should show a bare empty table. This needs a human because `View()` is a lipgloss-styled string and substring tests are not a visual check.

### Gaps summary

Both 2026-09-07 gaps are closed in code, tests and REQUIREMENTS.md, and Phases 3-5 did not regress any Phase 2 guarantee. The phase goal still fails because of a defect that was present from the start. **GetPipelineRuns and GetReleases make a second, per-definition call whose failure is silently converted into a "never run" / "no releases" row under `complete:true`.** That is exactly the empty-versus-failed confusion the phase exists to remove, and it reaches the CLI as exit 0. The 02-RESEARCH swallow table misclassified both sites as data filters, and the 2026-09-07 verification repeated that classification. Closing it is two `appendFailure` calls, a per-definition `ctx.Err()` pre-flight, a decision on how a failed definition row renders, and two tests.

Two smaller gaps share the theme of a sub-request outcome that is not stated accurately. First, the live org shows that empty-repository suppression keyed on 404 does not match ADO's VS403403 status, so `branches list` reports partial (exit 1) on every run. Second, the work-item list silently drops its iteration filter, a case the dashboard already records.

---

_Re-verified: 2026-10-12T14:59:29Z_
_Verifier: Claude (gsd-verifier). Read-only apart from this file; not committed._

---

## Re-verification 2 (2026-10-12, after 02-06/02-07)

**Scope:** HEAD of `dev` (`bd3f348`). This covers 02-06 (`ae99ba8`..`a1d29ad`), 02-07 (`07546af`..`0c67c57`) and the orchestrator's D-06b follow-up (`8995e2e`, `f73cfa4`). The three gaps above were re-checked in code and tests, not against the SUMMARY files. The verifier made no live ADO calls. The live results cited here are the orchestrator's read-only run against the Windstream org on HEAD.

### Gate

| Check | Result |
|-------|--------|
| `go build ./...` | pass |
| `go vet ./...` | pass |
| `go test -race -count=1 ./...` | pass: 12 packages ok, 750 `--- PASS`, 0 `--- FAIL` (was 724) |
| `test -z "$(gofmt -l .)"` | pass (empty) |

### Gap-closure check

| Prior gap | Status on HEAD | Evidence |
|-----------|----------------|----------|
| 1. Per-definition N+1 failures were dropped with `continue` in GetPipelineRuns and GetReleases (SC1, SC3) | ✓ CLOSED | `pipelines.go:214-221` and `releases.go:162-169` now call `appendFailure(failures, project, opGetLatestBuild/opGetLatestRelease+" for "+defName, err)` and `continue` without appending the row. The same class of swallow in GetPipelineDefinitions' `GetDefinition` call (`pipelines.go:368-375`), which the first re-verification missed, is closed the same way. A D-12 `ctx.Err()` pre-flight now sits inside all three per-definition loops (`pipelines.go:185`, `:343`; `releases.go:137`). Tests: `TestGetPipelineRunsReportsDefinitionFailure`, `TestGetReleasesReportsDefinitionFailure` and `TestGetPipelineDefinitionsReportsDetailFailure` (auth and timeout subtests each; value-form 401 and pointer-form 403) assert that the broken definition is absent from Items, that exactly one Failure names it with the right kind, and that `Complete == false`. The three `...MarksUnreachedDefinitions` tests assert one SDK call and a timeout Failure after a 100ms deadline. All pass under `-race`. |
| 2. Empty-repository suppression keyed on 404 contradicted live VS403403 evidence | ✓ CLOSED | `unreadableRepository` (`branches.go:75-87`) suppresses only on (a) status 400 AND `strings.HasPrefix(err.Error(), "VS403403:")` (D-06a), or (b) status 404 AND `TypeKey == "GitRepositoryNotFoundException"` (D-06b). Anything else is recorded with the repository named. `TestGetBranchesReportsRepoFailure` has 14 rows. They cover the live capture in value and pointer form, the disabled repository in value and pointer form, and the negatives: other 400, VS403403 under 404, VS403403 with no SDK status, a 404 with another typeKey, the not-found typeKey under 400, 401, VS403403 or the not-found typeKey on a 403 (stays auth), a timeout, and a 500. Live, on HEAD: `branches list` exit 0, `complete:true`, 0 failures, 886 items (was `complete:false`, 5 failures, then 130 after D-06a alone). |
| 3. Work-item iteration lookup degradation was silent | ✓ CLOSED | `getProjectWorkItems` records `opGetWorkItemsIterations` (`workitems.go:155`) and returns `failures` on all four return paths (`:174`, `:179`, `:194`, `:274`). `GetWorkItems` appends `projectFailures` before its own err check (`:125`), so the degradation survives a QueryByWiql failure too. `TestGetWorkItemsReportsIterationFailure` (2 subtests) asserts the degraded items are kept, there is one Failure with the right project, operation and kind, `Complete == false`, and the same holds when the query returns nothing. |

### Observable truths (roadmap success criteria) on HEAD

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Every aggregate read reports which projects failed and why, rather than dropping errors with `continue` | ✓ VERIFIED | Gap 1 and gap 3 closed (above). A rescan of every `if err != nil` in the six aggregate read files (`branches`, `dashboard`, `pipelines`, `pullrequests`, `releases`, `workitems`) found no error branch without `appendFailure` or a `return`. The only branch-stats suppressions are the two structural shapes in `unreadableRepository`. |
| 2 | An expired PAT produces a distinct fatal error, never an empty list | ✓ VERIFIED | The per-definition caveat from the first re-verification is gone. A 401/403 on the N+1 call is now recorded as `auth` (value-form 401 in the pipelines test, pointer-form 403 in the releases test). `exitCodeFor` returns `exitAuth` on any auth Failure (`cmd/adoBizInt/exit.go:19-29`). `TestClassifyFailure` (16) and `TestGet*ClassifiesRejectedToken` (7) still pass. |
| 3 | A partial result is explicitly flagged incomplete | ✓ VERIFIED | `newResult` derives `Complete: len(failures) == 0` (`result.go:121`). The three reads that returned `complete:true` over a failed sub-request now return `complete:false`. In the other direction, `branches list` no longer reports `complete:false` over a complete read (live, 886 items, `complete:true`). Legitimate empty states are unchanged: a verifier `go test -overlay` probe with GetBuilds/GetReleases answering zero results gave `status=neverRun` / `status=noReleases`, `failures=0`, `complete=true`. |
| 4 | Every ADO call carries a deadline and `connection.Timeout` is set | ✓ VERIFIED | Unchanged from the first re-verification, plus the D-12 pre-flight now runs inside the three N+1 loops, so a mid-loop expiry records timeouts instead of issuing doomed calls (3 `...MarksUnreachedDefinitions` tests). `TestFetchTimeoutMarksUnreached*` (6 reads) pass. |
| 5 | Exported `ado` structs carry `json:` tags with stable lower-cased names | ✓ VERIFIED | `TestJSONFieldNames` passes. The diff since `428ea39` adds no struct field, tag or `Status` literal to `PipelineRun`, `Release` or `PipelineDefinition`. |
| 6 | The TUI renders failures rather than silently showing an empty table | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | All six views still wire `authBanner`/`partialWarning`, and `TestUpdateAuthFailureBlocksThe*` (6) passes. The grey "never run"/"no releases" row for a failed fetch, noted under the first re-verification, can no longer occur because the row is omitted. No human visual check has been done; the user records it as verification debt. |

**Score:** 5/6 verified (1 present, behavior-unverified; 0 failed).

### Key links (manual; `verify.key-links` cannot parse prose `from:` fields)

| From | To | Status |
|------|----|--------|
| GetBuilds error branch | `appendFailure(... opGetLatestBuild+" for "+defName, err)` | ✓ WIRED (`pipelines.go:186`, `:219`) |
| GetReleases error branch | `appendFailure(... opGetLatestRelease+" for "+defName, err)` | ✓ WIRED (`releases.go:138`, `:167`) |
| GetDefinition error branch | `appendFailure(... opGetDefinitionDetail+" for "+*d.Name, err)` | ✓ WIRED (`pipelines.go:344`, `:373`) |
| Non-empty Failures | `exitCodeFor` / view banners | ✓ WIRED (`result.go:121`, `exit.go:19-29`) |
| Branch-stats error branch | `unreadableRepository` → `wrappedErrorOf` | ✓ WIRED (`branches.go:82`, `:147`). The 02-07 plan named `statusCodeOf`; after f73cfa4 both share `wrappedErrorOf`. |
| Iteration error branch | `GetWorkItems` failure aggregation | ✓ WIRED (`workitems.go:155` → `:125`) |

All 10 plan artifacts (6 from 02-06, 4 from 02-07) pass `verify.artifacts`.

### Behavioral evidence

| Probe | Result | Status |
|-------|--------|--------|
| Named gap tests above (run in the single full `-race` suite) | all PASS | ✓ |
| Overlay probe: GetBuilds and GetReleases answer zero results | `neverRun` / `noReleases` rows, `failures=0`, `complete=true` | ✓ legitimate empty state preserved (the 02-06 plan truth has no committed test; probe only, scratchpad overlay, `git status` unchanged) |
| Live, orchestrator, HEAD: `branches list` | exit 0, `complete:true`, 0 failures, 886 items | ✓ |
| Live, orchestrator, HEAD: `pipelines runs`, `pipelines definitions`, `releases list`, `workitems list`, `dashboard counts` | all exit 0, `complete:true`, 0 failures | ✓ (no per-definition failure occurred live, so the failure path rests on the unit tests) |

### Prohibitions

Four of the five plan prohibitions hold (see frontmatter). One is flagged. 02-07 says "Must NOT change result.go ... statusCodeOf", and the orchestrator's D-06b commit `f73cfa4` extracted `wrappedErrorOf` from `statusCodeOf`. `classifyFailure` is untouched and its 16 cases pass. The only behavioral difference is a chain holding both a status-less value-form and a status-bearing pointer-form `WrappedError`, which the SDK cannot produce (`WrappedError` has no `Unwrap`). The verifier's verdict is that this is acceptable, but the verdict is non-authoritative and listed for human acknowledgement.

### Requirements coverage

| Requirement | Status | Evidence |
|-------------|--------|----------|
| TRUST-01 | ✓ SATISFIED | Gaps 1 and 3 closed; no remaining error-swallowing `continue` in aggregate reads |
| TRUST-02 | ✓ SATISFIED | Per-definition 401/403 classified `auth` (value and pointer forms) |
| TRUST-03 | ✓ SATISFIED | `complete:false` over every failed sub-request; `complete:true` over empty and disabled repositories (live 886 items) |
| TRUST-04 | ✓ SATISFIED | D-12 pre-flight inside the N+1 loops |
| OUT-02 | ✓ SATISFIED | `TestJSONFieldNames`; JSON contract untouched |

REQUIREMENTS.md already marks all five `[x]`/Complete (lines 15-18, 41, 226-230), and that is now consistent with the code. No orphaned Phase 2 requirement IDs.

### Anti-patterns and info

| File | Line | Finding | Severity |
|------|------|---------|----------|
| `internal/ado/branches.go` | 124-147 | The per-repository loop has no in-loop `ctx.Err()` pre-flight, unlike the three N+1 loops 02-06 fixed. After a mid-loop expiry, each remaining repository still issues a call. The call fails fast on the done context and is recorded as `timeout` (no `WrappedError`, so `unreadableRepository` is false), so nothing is lost; it is only extra doomed calls. | ℹ️ Info |
| `internal/ado/branches.go` | 60-70 | D-06b treats any 404 + `GitRepositoryNotFoundException` as disabled. A repository deleted between `GetRepositories` and the branch-stats call is also skipped silently. That matches the user's decision and is reversible. | ℹ️ Info |
| `internal/ado/pipelines_test.go`, `releases_test.go` | n/a | No committed test pins the legitimate `neverRun`/`noReleases` empty state (02-06 truth 9); the verifier probe confirmed it | ℹ️ Info |
| `internal/ui/dashboard/dashboard.go` | 126-128, 89 | Carried from earlier: the `applyResult` doc comment still says "understated", and `m.complete` is unread by production code | ℹ️ Info |

No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK` markers in `branches.go`, `pipelines.go`, `releases.go`, `workitems.go` or `result.go`.

### Deferred check

Nothing to defer: no gaps remain.

### Human verification

1. **TUI failure rendering (verification debt, user-acknowledged).** Run with a PAT valid for one configured project and rejected for another, then open all six views. Expect an auth banner in place of the table for the rejected project, partial warnings where some projects answered, and no bare empty table. A per-definition 401/403 now raises the pipelines/releases auth banner rather than a grey row. This needs a human because `View()` is a lipgloss-styled string.
2. **Acknowledge the `result.go` refactor in `f73cfa4`** against 02-07's prohibition. Either accept it as a behavior-preserving extraction under D-06b, or ask for `wrappedErrorOf` to move into `branches.go`.

### Summary

All three gaps from the first 2026-10-12 re-verification are closed in code and tests on `bd3f348`, and the full gate is green (750 passes, 0 failures under `-race`). A failed per-definition lookup in GetPipelineRuns, GetReleases or GetPipelineDefinitions is now named, classified and excluded from Items, with `complete:false`. Empty and disabled repositories are recognised by narrow structural shapes, so `branches list` is `complete:true` live. The work-item list flags a dropped sprint filter. Within the automated reach, the phase goal (a caller can distinguish an empty result from a failed one) is achieved. Status is `human_needed` only for the user-acknowledged TUI visual check and one flagged judgment-tier prohibition.

---

_Re-verified (2): 2026-10-12T16:08:14Z_
_Verifier: Claude (gsd-verifier). Read-only apart from this file; not committed._

### Override accepted (2026-10-12)

- **02-07 prohibition "Must NOT change result.go … statusCodeOf"** — broken in letter by the orchestrator's D-06b follow-up `f73cfa4`, which extracted `wrappedErrorOf` from `statusCodeOf` so `unreadableRepository` reuses the value/pointer `WrappedError` extraction instead of duplicating it. `classifyFailure` is untouched and its 16 cases pass; the only behavioural delta needs an error chain carrying both SDK error shapes, which the SDK does not produce. **Accepted by the user on 2026-10-12** (reuse over a second implementation, per the user's engineering standards).
