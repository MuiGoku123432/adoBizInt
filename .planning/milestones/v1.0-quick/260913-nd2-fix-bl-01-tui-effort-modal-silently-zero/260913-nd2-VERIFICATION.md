---
phase: quick-260913-nd2
verified: 2026-09-13T16:58:00Z
status: human_needed
score: 6/7 must-haves verified
behavior_unverified: 1
overrides_applied: 0
behavior_unverified_items:
  - truth: "Closing a real Task through the TUI with Original Estimate blank leaves the stored estimate unchanged in Azure DevOps."
    test: "Close a live Task through the TUI effort modal with Original Estimate blank; inspect the work item History in Azure DevOps."
    expected: "No OriginalEstimate revision appears; the prior value is intact. Then repeat with an explicit 0 and confirm a real zero revision IS recorded."
    why_human: "The original bug was found by live TUI interaction against a real work item. No test in this repo issues a real Azure DevOps API call, so nothing here re-runs the failing scenario end to end."
human_verification:
  - test: "Live TUI round-trip: close a Task with Original Estimate blank, then another with Original Estimate typed as 0."
    expected: "Blank leaves the stored estimate unchanged (no History revision for OriginalEstimate); explicit 0 writes a real zero."
    why_human: "Requires a live Azure DevOps work item; covered only by unit tests today (SUMMARY coverage item D6)."
---

# Quick 260913-nd2: BL-01 TUI Effort Modal Verification Report

**Goal:** Closing a work item through the Bubble Tea effort modal with an effort field left blank
must leave that field UNCHANGED on the work item, instead of overwriting it with 0.
**Base commit:** `e005de0` **Head:** `266b88b`
**Status:** human_needed (all static/unit evidence verified; live round-trip outstanding)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Blank field yields nil all the way into `ado.UpdateWorkItemStateWithEffort`, which omits the op | ✓ VERIFIED | Full chain read, see trace below |
| 2 | Explicit `0` yields non-nil ptr to 0.0 and a real zero patch op | ✓ VERIFIED | `effort.go:204-212`; `effort_test.go:67-76,157`; `ado/workitems_test.go` subtest "explicit zero remaining is a real zero, not an omission" PASS |
| 3 | Blank Completed still rejected by greater-than-zero rule | ✓ VERIFIED | `effort.go:151-154` (`completed == nil \|\| *completed <= 0`); `effort_test.go:87-94` |
| 4 | `internal/ado` untouched | ✓ VERIFIED | `git diff --stat e005de0 HEAD -- internal/ado/` → empty |
| 5 | D-17 guard `TestUpdateWorkItemStateWithEffortIssuesExactPatch` byte-identical and passing | ✓ VERIFIED | `git diff e005de0 HEAD -- internal/ado/workitems_test.go` → empty; `go test ./internal/ado/ -run TestUpdateWorkItemStateWithEffort -v` → PASS |
| 6 | BL-01 part 2 not attempted, still open in BACKLOG | ✓ VERIFIED | `git diff --stat e005de0 HEAD -- .planning/BACKLOG.md` → empty; BACKLOG remediation item 2 (`runWrite` routing) intact; no `runWrite` wiring in the TUI diff |
| 7 | The fix actually prevents the empirically-observed live data loss | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | Code present + wired + unit-tested; no test issues a real ADO call. See Human Verification. |

**Score:** 6/7 truths verified (1 present, behavior-unverified)

### Chain Trace (read from source, not from SUMMARY)

1. `effort.go:203-213` — `parseEffort` trims; `s == ""` → `return nil, nil`; otherwise `ParseFloat` then `return &v, nil`. `"0"` therefore returns a non-nil pointer to 0.0. Correct both directions.
2. `effort.go:139-141` — the `enter` branch calls `parseEffort` for all three fields.
3. `effort.go:151-154` — nil Completed rejected (required field preserved).
4. `effort.go:156-167` — the three `*float64` values assigned straight into `EffortSubmitMsg` (fields are `*float64` at `effort.go:27-29`). No address-of a zero local anywhere.
5. `workitems.go` both branches — `effortMsg.OriginalEstimate/.Remaining/.Completed` passed directly; the three local copies and the false "ALWAYS non-nil" comments are deleted (diff confirmed).
6. `ado/workitems.go:315-335` — `if originalEstimate != nil { append }` etc. A nil pointer emits no patch operation. Unmodified.

Blank → nil → no patch op. Explicit 0 → &0.0 → real zero op. Both requirements hold; the fix did not trade one silent bug for another.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| Build | `go build ./...` | clean | ✓ PASS |
| Full suite | `go test ./...` | all packages `ok`, no FAIL | ✓ PASS |
| Vet | `go vet ./...` | clean | ✓ PASS |
| Format | `gofmt -l .` | no output | ✓ PASS |
| D-17 guard | `go test ./internal/ado/ -run TestUpdateWorkItemStateWithEffort -v` | PASS (both tests, all subtests) | ✓ PASS |

### AST Guard Strength (independently re-tested)

I did not accept the executor's account. I applied my own **compiling** revert to the Task-branch
call site:

```go
var originalEstimate float64
if effortMsg.OriginalEstimate != nil { originalEstimate = *effortMsg.OriginalEstimate }
... &originalEstimate ...
```

`go build ./...` → COMPILES. Guard result:

```
--- FAIL: TestUpdateWorkItemStateWithEffortCallSitesForwardPointers
    workitems_test.go:168: call site 1 arg 4 is an address-of expression, which reintroduces BL-01
```

Reverted; `git diff` on the file clean. **The guard has real teeth against the exact bug shape.**

It also asserts the **positive** shape, not only the negative: `workitems_test.go` requires exactly
2 call sites, 6 args each, and args 4-6 to be `*ast.SelectorExpr` whose `X` is ident `effortMsg` and
whose `Sel` names `OriginalEstimate`/`Remaining`/`Completed` in order. A non-`&` regression (e.g. a
`*ast.StarExpr` or a renamed local) is caught by the selector assertion. AST-based, so a comment
containing `&originalEstimate` cannot defeat it.

**Minor honesty note:** the SUMMARY (line 139/156) claims the revert experiment produced a
`*ast.StarExpr` caught by the positive assertion. My compiling revert produced a `*ast.UnaryExpr`
caught by the negative assertion. Both branches work; the SUMMARY's narrative detail differs from
what I reproduced. Not a defect — recorded for accuracy.

### Anti-Patterns Found

None. No TODO/FIXME/TBD/HACK/placeholder introduced in the four changed files. Working tree clean
apart from untracked `.gsd/` and `.planning/quick/`.

### Scope Check

BL-01 part 2 was not attempted. No `runWrite`, dry-run, `--confirm`, or `audit.jsonl` wiring appears
anywhere in the diff (`internal/ui/workitems/` only, 4 files). `.planning/BACKLOG.md` is byte-identical
to its pre-task state, so BL-01 remains open with its part-2 remediation intact.

### Human Verification Required

**1. Live TUI round-trip against a real Azure DevOps Task**

- **Test:** Close a Task through the TUI effort modal with Original Estimate left blank. Then close
  another with Original Estimate typed as `0`.
- **Expected:** Blank → work item History shows no OriginalEstimate revision, prior value intact.
  Explicit `0` → History shows a real zero write.
- **Why human:** The original failure (2026-09-12) was found by live TUI interaction. This fix is
  covered **only by unit tests**; nothing in the repo issues a real API call. The static chain is
  sound and the patch-document behaviour is pinned at the `ado` boundary, but the end-to-end path
  that actually failed has not been re-run.

### Gaps Summary

No gaps. The implemented fix is correct, wired, formatted, vetted, and guarded in both directions.
**BL-01 part 1 should not be marked closed until the live round-trip above is performed** — that is
the single outstanding item, and it is the same class of evidence that originally surfaced the bug.

---

_Verified: 2026-09-13_
_Verifier: gsd-verifier (goal-backward, adversarial stance)_
