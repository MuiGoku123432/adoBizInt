---
status: testing
phase: 07-guarded-tui-writes
source: [07-VERIFICATION.md]
started: 2026-10-16T14:04:11Z
updated: 2026-10-16T14:04:11Z
---

## Current Test

number: 1
name: Live SIGTERM during a real TUI write (07-09 coverage D5, sub-truth 12f). Start the TUI, confirm a write with y, then kill -TERM <pid> while 'Writing #<id>...' is shown
expected: |
  Exactly one new audit line for that write with "actor":"tui" and outcome unknown (or executed if it finished first). The process exits 0 and the terminal is restored. The ADO item should be checked by hand, as the unknown outcome asks
awaiting: user response

## Tests

### 1. Live SIGTERM during a real TUI write (07-09 coverage D5, sub-truth 12f). Start the TUI, confirm a write with y, then kill -TERM <pid> while 'Writing #<id>...' is shown
expected: Exactly one new audit line for that write with "actor":"tui" and outcome unknown (or executed if it finished first). The process exits 0 and the terminal is restored. The ADO item should be checked by hand, as the unknown outcome asks
result: [pending]

### 2. PRs tab against a test PR you can approve: press a, then n; then press c on an active PR
expected: Rounded 'Confirm #<id>' modal shows 'reviewer.vote  0 → 10' and '[y] write  [n] discard  [esc] stay'. n changes nothing in ADO. c shows 'status  active → completed' with no old 'Merge PR ... [y] confirm' status prompt. At 80x24 the 66-column modal shows title and key hint unclipped
result: [pending]

### 3. Releases tab: open a pending approval, type a comment, submit. Pipelines tab: press t, pick a definition and branch, submit, then esc
expected: A second modal 'Confirm #<approval id>' shows 'approval.status  pending → approved' before anything is sent. The pipeline confirm shows 'queued_build  (none) → <branch>'. esc queues nothing in ADO
result: [pending]

### 4. Work Items: press n on an Active bug, then down, esc, down, n. Repeat at 80x24
expected: Actions cell shows the whole 'staged → Resolved' in amber. down opens 'Confirm #<id>' with 'System.State  Active → Resolved'. esc keeps the item staged. n discards and moves. At 80x24 the 18-column Actions column does not clip the row or status hint
result: [pending]

### 5. Close an Active Task with n: leave Original Estimate blank, set Remaining 0 and Completed > 0, submit, move down, press y
expected: One confirm modal lists the state and the two effort changes, with no Original Estimate line. In ADO History, Original Estimate is unchanged. ~/.config/adoBizInt/audit.jsonl gains two lines with "actor":"tui"
result: [pending]

### 6. Stage a state change on Work Items, press 3, then esc, then q, then n
expected: The confirm modal appears before the PRs tab. esc stays on Work Items with the row still staged. q shows the modal again. n exits the app without writing
result: [pending]

### 7. On an 80-column terminal, trigger a long server error (e.g. a 409 on PR complete) and an unknown outcome
expected: The sticky line and status line stay readable and are not clipped by the help bar
result: [pending]

### 8. Decide the reading of ROADMAP SC1 'one audit record per changed field' versus SC2 'two audit records' for a resolve
expected: Confirm that bundling a resolve's effort fields into ONE record is acceptable. Today Remaining and Completed share one record with changes[] holding both, and each changed field appears in exactly one record. Phase 16 SC3 reuses the 'one audit record per changed field' wording
result: [pending]

### 9. Judgment-tier prohibition (07-04): 'Must NOT retry a write automatically after a failed or unknown outcome'
expected: Human sign-off. Non-authoritative LLM verdict: holds. confirm.Execute runs each op at most once and breaks on the first outcome that is neither executed nor no_op. Execute has one call site (flow.go:107), and WriteMsg clears the modal so a second y is ignored. Flow.settle never calls Start again. 07-09 adds test-tier evidence for the interrupted case: TestExecuteInterruptedMidMutateIsAuditedUnknown asserts Mutate ran exactly once
result: [pending]

## Summary

total: 9
passed: 0
issues: 0
pending: 9
skipped: 0
blocked: 0

## Gaps
