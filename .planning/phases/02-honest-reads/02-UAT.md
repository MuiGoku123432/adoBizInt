---
status: complete
phase: 02-honest-reads
source: [02-VERIFICATION.md]
started: 2026-10-12T16:41:14Z
updated: 2026-10-12T16:41:14Z
---

## Current Test

[testing complete]

## Tests

### 1. TUI renders failures rather than an empty table (all six views)
expected: With a PAT valid for one configured project and rejected for another, each of the six views shows an auth banner for the rejected project and a partial warning where some projects answered; no view shows a bare empty table.
result: skipped
reason: "Deferred follow-up: complete with debt. Needs a PAT rejected for exactly one project; recorded as verification debt by the user on 2026-10-12. Substring assertions cover all six views; the read-side behaviour was verified live on 2026-10-12 (all reads complete:true, 0 failures)."

## Summary

total: 1
passed: 0
issues: 0
pending: 0
skipped: 1
blocked: 0

## Deferred Follow-Ups

- test: 1
  idea: "complete with debt — run the six-view TUI auth-banner check in the next milestone's first smoke test"
  deferred_at: 2026-10-12

## Gaps
