---
status: testing
phase: 05-hardening
source: [05-VERIFICATION.md]
started: 2026-09-30T18:33:11Z
updated: 2026-09-30T18:33:11Z
---

## Current Test

number: 1
name: PHASE-BLOCKING — green GitHub Actions ci run (05-03 T2)
expected: |
  After pushing dev, the ci workflow run for HEAD is green with all four gates (build, vet, go test -race, gofmt) and uses no secrets.
awaiting: user response

## Tests

### 1. PHASE-BLOCKING — green GitHub Actions ci run (05-03 T2)
expected: Push dev to origin; the ci run for HEAD is green across build, vet, race tests and gofmt; the run log shows no secrets or ADO_* variables. Optional: a throwaway branch with an unformatted .go file goes red at the gofmt step.
result: [pending]

### 2. PHASE-BLOCKING — live @project before/after comparison (05-05 T2)
expected: With your own PAT,  and  for one configured project return the same counts from a binary built at 191b403 (pre-05-05) and one built at HEAD, and both report complete:true.
result: [pending]

### 3. Real TUI attachment download by ID (05-01 T3)
expected: Downloading a real attachment writes a file byte-identical to the ADO web UI copy under ~/.config/adoBizInt/attachments (dir 0700, file 0600); a repeat download gets " (1)"; esc cancels a slow download.
result: [pending]

### 4. Live set-state with project in the update route (CR-01 fix)
expected:  dry run and one --confirm against a work item in a configured project succeed; a work item from another project is refused with exit 3 and no audit record. Note the project match is case-sensitive against System.TeamProject.
result: [pending]

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps
