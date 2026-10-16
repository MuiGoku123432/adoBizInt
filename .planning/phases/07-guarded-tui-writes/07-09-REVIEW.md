---
phase: 07-guarded-tui-writes
plan: 09
reviewed: 2026-10-16T13:54:20Z
depth: standard
diff_base: 18e898c28b8fc013333bf676d75b069a8933caae
files_reviewed: 4
files_reviewed_list:
  - cmd/adoBizInt/main.go
  - cmd/adoBizInt/tuiexit_test.go
  - internal/ui/confirm/batch.go
  - internal/ui/confirm/inflight_test.go
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 7 Plan 09: Code Review Report (CR-01 gap closure, scoped)

**Reviewed:** 2026-10-16T13:54:20Z
**Depth:** standard
**Files Reviewed:** 4
**Status:** issues_found

## Summary

Scope: `git diff 18e898c..HEAD` for the four files above, plus the code they directly touch (`internal/guard/guard.go` Run/classify, `internal/ui/confirm/flow.go` cmd closures, `internal/fetchctx`, and bubbletea v1.3.10 `tea.go` signal, command and panic handling). Accepted residuals R1-R4 from 07-09-PLAN.md and the WaitGroup-reuse note in 07-09-SUMMARY.md are not re-raised.

The production change is correct. No blocker was found:

| Check | Result |
|-------|--------|
| `writeParent` / `SetParent` race safety | Safe in production. `SetParent` (main.go:157) runs on the main goroutine before `p.Run()`. Every reader runs in a cmd goroutine spawned transitively by `go` statements inside `Run`, so the write happens-before every read. No test in package `confirm` uses `t.Parallel`. Both new tests restore the parent in `t.Cleanup`. `go test -race -count=5` and `-shuffle=on -count=3` pass. |
| WaitGroup Add/Wait | `Add(1)` and the deferred `Done()` are the first two statements of `Execute`, so every Add has a Done, including on a Plan panic. Add is never at the cmd-dispatch site, so a dropped cmd cannot leave an unmatched Add. The only Add-concurrent-with-Wait window is R1. |
| Can `Wait` block forever | Not in practice. Each `guard.Run` is bounded by `fetchctx.NewWithParent`, and a cancelled parent ends Mutate at once. See IN-01 for the doc wording. |
| Exit codes, normal path | Unchanged. `_, err = p.Run()` overwrites the stale `err` from `config.LoadPriorities` unconditionally. Exit is still 0, or 1 on a Run error. SIGTERM still reaches bubbletea as `QuitMsg`, so the exit stays 0 because no external context is attached. |
| CLI branch | Byte-identical (lines 128-138 are untouched in the diff). |
| New tests | `inflight_test.go` is not vacuous. A missing Add fails line 69. A root-context parent fails line 107 or TestCancelledParentSendsNothing. The 100 ms check only asserts that Wait has not returned, so a slow machine cannot make it fail when the code is correct. All goroutines exit on the passing path. |
| Raw goroutines that could panic outside bubbletea's recover | None in `internal/` or `cmd/` (non-test). |
| `tea.ExecProcess` / `ReleaseTerminal` (bubbletea ignores signals while these run, but NotifyContext would still cancel the parent) | Not used, so a signal cannot leave a running TUI with a cancelled write parent outside R2. |

The defects are in the regression guard. `TestTUIWaitsForInflightWriteBeforeExit` is meant to pin the CR-01 wiring, but it passes for three wirings that reopen CR-01 (WR-01). It also does not pin design decision G-03 at all (WR-02). I proved both by mutating a scratch copy of the repo outside the working tree, not by reasoning alone.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: The exit-wiring AST pin passes for wirings that skip the wait or never install the parent

**File:** `cmd/adoBizInt/tuiexit_test.go:73-96, 98-123` (doc claims at lines 17-25)
**Issue:** `only` accepts any top-level statement whose subtree contains the call. It never checks that the statement *is* the call, so a conditional, a goroutine, or a dead branch all satisfy rules 2 and 3. I applied each mutation below to `main.go` in a scratch copy of the repo outside the working tree. With each one, `TestTUIWaitsForInflightWriteBeforeExit` still reports `ok`:

| Variant | main.go change | Effect | Test |
|---------|----------------|--------|------|
| A | `confirm.Wait()` moved inside `if err != nil { ... }` | SIGTERM ends Run through `QuitMsg` with a nil error, so the wait is skipped on the main CR-01 path | passes |
| B | `go confirm.Wait()` | No wait at all | passes |
| C | `if false { confirm.SetParent(tuiSignalCtx) }` | Writes run under the background context again, and an interrupt is never classified unknown | passes |

Variant A reopens CR-01 for its headline case. The doc comment says rule 2 makes "the wait ... reachable on every path out of Run", and the test does not enforce that. `hasReturn` also treats only `return` as a terminator. A `panic(...)`, `log.Fatal`, `os.Exit` or `runtime.Goexit` placed between Run and Wait would pass too.

**Fix:** Require the matched statements to be the bare calls, not merely to contain them:

```go
// bare reports whether stmt is exactly `call` as an expression statement,
// so the call runs unconditionally, on this goroutine, at this point.
bare := func(stmt ast.Stmt, call *ast.CallExpr) bool {
	es, ok := stmt.(*ast.ExprStmt)
	return ok && es.X == call
}

runAt, runCall := only("rule 1", "", "Run")
if as, ok := stmts[runAt].(*ast.AssignStmt); !ok || len(as.Rhs) != 1 || as.Rhs[0] != runCall {
	t.Errorf("rule 1: statement %d must be `_, err = p.Run()`, got %T", runAt, stmts[runAt])
}

waitAt, waitCall := only("rule 2", "confirm", "Wait")
if !bare(stmts[waitAt], waitCall) {
	t.Errorf("rule 2: statement %d is %T; confirm.Wait must be a bare top-level call, not inside a branch, go or defer", waitAt, stmts[waitAt])
}

setAt, setCall := only("rule 3", "confirm", "SetParent")
if !bare(stmts[setAt], setCall) {
	t.Errorf("rule 3: statement %d is %T; confirm.SetParent must be a bare top-level call", setAt, stmts[setAt])
}
```

Keep the existing "no return between" loop. Optionally, extend `hasReturn` to flag `panic` calls and `os.Exit` / `log.Fatal*` selectors as terminators too. After the fix, re-run variants A-C and record that each one fails, as the SUMMARY did for the original vacuity check.

### WR-02: G-03 (the signal context is not handed to the bubbletea program) has no lasting regression guard

**File:** `cmd/adoBizInt/tuiexit_test.go:33-161`; `cmd/adoBizInt/main.go:142, 155`
**Issue:** The plan rates T-07-42 medium with disposition `mitigate`: handing the signal context to bubbletea hangs shutdown with a raw terminal and turns a SIGTERM exit from 0 into 1. The only mitigation is a one-time grep in the 07-09 Task 1 `<verify>` block (`grep -c 'WithContext' = 0`), and no persisted test enforces it. In the scratch copy I moved the NotifyContext pair above `tea.NewProgram` and changed that statement to `tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(tuiSignalCtx))`. The package built, and both `TestTUIWaitsForInflightWriteBeforeExit` and the locked `TestValidateGuardsCLIAndTUI` passed. That is exactly the change CR-01's original fix suggestion in 07-REVIEW.md proposed, so a later maintainer following that text would reintroduce the hang with no test failing.

**Fix:** Add a sixth rule to the same test. The SetParent call must be the only use of the signal context identifier in `run`:

```go
// rule 6 (G-03): the signal context reaches only confirm.SetParent. Handing it
// to the bubbletea program lets its signal goroutine block forever on a send
// the cancelled event loop never receives.
uses := 0
ast.Inspect(run.Body, func(n ast.Node) bool {
	if id, ok := n.(*ast.Ident); ok && id.Name == parent.Name {
		uses++
	}
	return true
})
// one definition (rule 4) plus one argument to SetParent (rule 3)
if uses != 2 {
	t.Errorf("rule 6: %s is referenced %d times in run, want 2 (its definition and confirm.SetParent); it must not reach the bubbletea program", parent.Name, uses)
}
```

## Info

### IN-01: `Wait`'s "it cannot hang" doc overstates the bound

**File:** `internal/ui/confirm/batch.go:30-36`
**Issue:** The comment says Wait "cannot hang: every guard.Run is bounded by its budget through fetchctx". Three things are outside that bound:
- `guard.Run` opens the audit sink and appends the record (`guard.go:136, 210`) as plain file I/O with no context, so a stalled filesystem is not bounded by the budget.
- The bound holds only because every Plan and Mutate honours its context.
- With a live parent, which is the recovered-panic path (R4), one Execute can take up to `len(b.Ops) x budget`, because each `guard.Run` gets a fresh `fetchctx` budget. R4 in the plan describes this as "up to that write's budget".

None of these is a hang in practice. But the doc is the contract main.go relies on, and it should not claim more than the code guarantees.

**Fix:** Reword, for example: "Wait blocks until every Execute in flight has returned, which means each record is appended or its failure reported. It is bounded in practice: each guard.Run's requests are capped by its budget through fetchctx (at most len(Ops) budgets per Execute while the parent is live), and a cancelled parent ends an in-flight Mutate at once. Audit file I/O is not context-bounded."

---

_Reviewed: 2026-10-16T13:54:20Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
