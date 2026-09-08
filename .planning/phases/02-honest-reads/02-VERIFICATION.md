---
phase: 02-honest-reads
verified: 2026-09-07T23:10:00Z
status: gaps_found
score: 5/6 success criteria verified
behavior_unverified: 1
verdict: PASS-WITH-GAPS
requirements:
  TRUST-01: satisfied
  TRUST-02: satisfied-with-known-gap
  TRUST-03: satisfied-with-defect
  TRUST-04: satisfied
  OUT-02: satisfied
gaps:
  - truth: "A partial result is explicitly flagged incomplete, and flagged truthfully"
    status: partial
    reason: "The dashboard's partial banner hardcodes the headline 'totals understated' even when the only failure is a degraded iteration lookup, which the code's own comment states skews the total UPWARD. For that case the tool states the opposite of the truth."
    artifacts:
      - path: "internal/ui/dashboard/dashboard.go:166"
        issue: "parts := []string{\"totals understated\"} is unconditional; the degraded/unanswered split computed immediately above is not used to choose the headline."
    missing:
      - "Choose the headline from the failure split: 'totals understated' only when unanswered failures exist; a distinct 'totals overstated' (or neutral 'totals unreliable') headline when only opDashboardIterations degraded."
      - "A reducer test asserting the headline for an iterations-only failure."
  - truth: "REQUIREMENTS.md traceability reflects phase completion"
    status: failed
    reason: "Traceability row 222 still reads Pending for TRUST-01..TRUST-04 while all four checkboxes are [x]. The range label cannot be matched by requirements.mark-complete."
    artifacts:
      - path: ".planning/REQUIREMENTS.md:222"
        issue: "| TRUST-01 .. TRUST-04 | Phase 2 | Pending |"
    missing:
      - "Split the range row into four per-ID rows (or mark the range row Complete) so the table stops contradicting the checkboxes."
behavior_unverified_items:
  - truth: "The TUI renders failures rather than silently showing an empty table"
    test: "Run the tool with a deliberately invalid/expired PAT that is valid for at least one configured project and rejected for another; open each of the six views."
    expected: "An auth banner replaces the table for the rejected project; partial warnings appear where some projects answered; no view shows a bare empty table."
    why_human: "View() returns a lipgloss-styled string. Substring assertions exist in all six packages but a substring assertion is not a visual check. 02-VALIDATION.md marks this row NOT DONE."
---

# Phase 2: Honest Reads — Verification Report

**Phase Goal:** A caller can distinguish an empty result from a failed one
**Verified:** 2026-09-07
**Verdict:** PASS-WITH-GAPS

## Success Criteria

| # | Criterion | Status | Evidence |
|---|-----------|--------|----------|
| 1 | Every aggregate read reports which projects failed and why | ✓ VERIFIED | Targeted scan of all five aggregate read files: every `if err != nil { ... continue }` is preceded by `appendFailure`. The residual `continue` statements are nil-guards, repo/name filters, or the D-12 pre-flight `ctx.Err()` record — none drop an error. 10 `TestGet*ReportsProjectFailure` tests run. |
| 2 | An expired PAT produces a distinct fatal error, never an empty list | ⚠️ PARTIAL | `classifyFailure` → `auth` on 401/403 confirmed; 7 `TestGet*ClassifiesRejectedToken` tests. Known unreachable path documented (see Scrutiny 2). |
| 3 | A partial result is explicitly flagged incomplete | ⚠️ DEFECT | `Complete` derived centrally in `newResult`; all six views set a warning from `Failures`. But the dashboard's headline text is wrong for one failure class (see Scrutiny 4). |
| 4 | Every ADO call carries a deadline and `connection.Timeout` is set | ✓ VERIFIED | `TestNewClientSetsTimeout`, `TestLoadAppliesTimeoutDefaults` (4), `TestFetchTimeoutMarksUnreached` (6 reads). `internal/ui/fetchctx` bounds every read fan-out. |
| 5 | Exported `ado` structs carry `json:` tags with stable lower-cased names | ✓ VERIFIED | `TestJSONFieldNames` — 17 tests across 11 structs. |
| 6 | The TUI renders failures rather than silently showing an empty table | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | All six views wire `authBanner` + `partialWarning`; 6 `TestUpdateAuthFailureBlocksThe*` tests pass. No human has looked at rendered output. |

**Score:** 5/6 verified (1 behavior-unverified). One criterion (3) carries a real defect.

## Scrutiny Findings

### 1. `statusCodeOf` value + pointer — CONFIRMED CORRECT

`internal/ado/result.go` tries `azuredevops.WrappedError` (value) first, then `*azuredevops.WrappedError`, each with a `StatusCode != nil` guard. The doc comment cites the four SDK call sites that motivate it. A pointer-only implementation would indeed have degraded a rejected PAT to `FailureOther` silently; that has not happened. `TestClassifyFailure` runs 16 cases including both forms.

### 2. Startup-fatal PAT rejection — HONESTLY LOGGED, TRUST-02 defensible

02-01-SUMMARY.md records this in three places: a deviation entry ("Issue / Resolution: logged, not fixed"), the scope-call list, and a Follow-up Items table row targeting "Phase 02 expansion or Phase 03 CLI". This is not a buried caveat.

The per-project case — a PAT valid for one project and rejected for another — IS covered: 7 `TestGet*ClassifiesRejectedToken` tests plus the six view reducer tests. That is the case the phase goal actually names ("distinguish an empty result from a failed one" is a per-project distinction).

**Judgment: TRUST-02 can honestly be called met.** The totally-rejected-PAT case is loud and fatal today — it produces `Failed to create ADO client`, which is exactly "not an empty list". It is ugly and unclassified, not silent. The requirement text is "HTTP 401/403 are fatal and never render as an empty result set"; both paths satisfy it.

### 3. T-02-01 leak guard — HOLDS

- `Failure.Err` is tagged `json:"-"` and a value-receiver `MarshalJSON` re-emits a flattened `*string`. The struct carries no URL/header/token field.
- `TestJSONFailureErrIsFlattenedString` is a genuine guard, not a shape check: it plants a `leakyError{RequestURL: "https://dev.azure.com/o/_apis?token=SECRET"}` and asserts the raw bytes contain none of `RequestURL`, `request_url`, `SECRET`, `dev.azure.com`. That is the real assertion asked for.
- Truncation is centralised, not per-view: `internal/ui/styles` defines `maxFailureMessage = 120` and applies `utils.Truncate` in all four renderers (`RenderAuthBanner`, `RenderPartialWarning`, `RenderUnderstatedWarning`, and via them `SummarizeFailures`). All six views route their server-controlled text through those renderers. **Better than the "prs only" concern implied** — no view formats failure text itself.

### 4. Dashboard render + the understated/overstated label — REAL DEFECT

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

### 5. Self-contradictory acceptance criteria — RESOLVED HONESTLY, NOT GAMED

- **`continue` counts (02-02):** the criterion was reported unsatisfiable rather than met. Verified independently: the surviving `continue` statements are structurally benign (nil guards, filters, D-12 pre-flight records). No error is dropped anywhere in the five aggregate read files. The count criterion was wrong; the behavior is right.
- **`context.Background()` counts (02-05):** resolved by extracting `internal/ui/fetchctx`, a real behavioral change — every read fan-out now runs under a `WithTimeout` with a non-zero fallback (`defaultBudget = 60s`, applied when budget ≤ 0, so an unconfigured budget cannot mean unbounded). Not a grep dodge.

**Residual note on TRUST-04:** 13 `context.Background()` call sites remain in `internal/ui`. Most are write paths (Phase 4 scope, correctly deferred). Three are *reads* — `GetWorkItemDetail`, `GetChildWorkItems`, `DownloadAttachment`. These are single-target reads, not aggregates, and are bounded at the transport layer by `connection.Timeout`, so TRUST-04's literal text still holds. They carry no *fan-out* budget and no cancellation. Worth a Phase 5 line item; not a Phase 2 gap.

### 6. `internal/ui/fetchctx` untested — ACCEPTABLE

The package is 33 lines, one exported function, one branch (`budget <= 0`). It is exercised indirectly by every view test. **Acceptable coverage gap**, but the zero/negative-budget fallback is the one behavior worth pinning, since regressing it silently restores the unbounded context the package was created to remove. Recommend a three-line test; not a blocker.

### 7. 02-VALIDATION.md honesty — VERIFIED, ALL COMMANDS MATCH REAL TESTS

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

### 8. Requirement marking — DISCREPANCY PERSISTS

`.planning/REQUIREMENTS.md:222` still reads `| TRUST-01 .. TRUST-04 | Phase 2 | Pending |` while lines 15–18 are all `[x]`. Line 223 (`OUT-02`) correctly reads Complete. 02-02's prediction was accurate and the row was never fixed. Bookkeeping only, but the file now contradicts itself.

*(Unrelated observation: `SEAM-04` at line 27 — "aggregate read failure paths from TRUST-01 are covered by tests" — is still `[ ]` but is now arguably satisfied by this phase's 10+7+6 failure-path tests. Phase 1 scope; flagged for the milestone audit, not this phase.)*

## Anti-Patterns

No `TODO`, `FIXME`, `XXX`, or `HACK` markers in the phase's source files. `gofmt`, `go vet`, `go build`, `go test ./... -count=1` all green (regression gate re-confirmed by the module-wide test runs above).

## Verdict

**PASS-WITH-GAPS.**

The phase goal — a caller can distinguish an empty result from a failed one — is achieved. The result envelope is real, every aggregate read populates it, the classifier handles both SDK error shapes, the JSON leak guard is a genuine adversarial test, and all six views surface failures. The execution artifacts are unusually honest: two unsatisfiable acceptance criteria were reported as unsatisfiable rather than gamed, a known unreachable auth path was logged rather than buried, and two vacuous validation commands were fixed rather than allowed to pass proving nothing.

**Nothing here blocks the phase, but one item should be fixed before it is signed off:**

- 🟡 **Dashboard says "totals understated" over an overstated number** (`dashboard.go:166`). The one place in this phase where the tool states something untrue about its own data. Small fix, high symbolic cost if left.
- 🟡 **REQUIREMENTS.md:222 traceability row contradicts its own checkboxes.**
- ⚪ Recommended: a `fetchctx` zero-budget test; delete or use the dead `m.complete` field.
- ⚪ Open: human visual check of the six views against an invalid PAT — the correct subject for `/gsd-verify-work`.

---

_Verified by gsd-verifier. Read-only apart from this file._
