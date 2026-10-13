---
status: complete
phase: 05-hardening
source: [05-VERIFICATION.md]
started: 2026-09-30T18:33:11Z
updated: 2026-10-12T14:45:01Z
---

## Current Test

[testing complete]

## Tests

### 1. PHASE-BLOCKING — green GitHub Actions ci run (05-03 T2)
expected: Push dev to origin; the ci run for HEAD is green across build, vet, race tests and gofmt; the run log shows no secrets or ADO_* variables. Optional: a throwaway branch with an unformatted .go file goes red at the gofmt step.
result: pass
note: "ci run 36798351937 on HEAD be36596 (push, 2026-10-01) concluded success; build, vet, go test -race and gofmt steps all green; first Linux run. Log shows only the runner's standard 'Secret source: Actions' header, no secrets used."

### 2. PHASE-BLOCKING — live @project before/after comparison (05-05 T2)
expected: With your own PAT, `workitems list --json` and `dashboard counts --json` for one configured project return the same counts from a binary built at 191b403 (pre-05-05) and one built at HEAD, and both report complete:true.
result: pass
note: "Live run 2026-10-12: workitems list --json identical ID sets before (191b403) and after (HEAD): Kinetic 1651, Process 6316, complete:true, 0 failures. dashboard counts drifts with live data for both binaries (7967/7965/7966); a back-to-back pair returned 7967 from each."

### 3. Real TUI attachment download by ID (05-01 T3)
expected: Downloading a real attachment writes a file byte-identical to the ADO web UI copy under ~/.config/adoBizInt/attachments (dir 0700, file 0600); a repeat download gets " (1)"; esc cancels a slow download.
result: skipped
reason: "Deferred follow-up: Record as deferred, close now. Deferred to the v2 milestone's first smoke test. Interactive TUI download not run; download path covered by attachments_test.go and detail_test.go (WR-01..WR-04 fixes)."

### 4. Live set-state with project in the update route (CR-01 fix)
expected: `workitems set-state` dry run and one --confirm against a work item in a configured project succeed; a work item from another project is refused with exit 3 and no audit record. Note the project match is case-sensitive against System.TeamProject.
result: skipped
reason: "Deferred follow-up: Record as deferred, close now. Partially verified live 2026-10-12: dry run previewed Active->Resolved with executed:false (exit 0), same-state dry run exited 4, Process item with --project Kinetic refused with exit 3, audit.jsonl unchanged. The real --confirm write was deferred by user to the v2 milestone's first smoke test."

## Summary

total: 4
passed: 2
issues: 0
pending: 0
skipped: 2
blocked: 0

## Deferred Follow-Ups

- test: 3
  idea: "Record as deferred, close now — run a real TUI attachment download in the v2 milestone's first smoke test"
  deferred_at: 2026-10-12
- test: 4
  idea: "Record as deferred, close now — run one live set-state --confirm (project in PATCH route) in the v2 milestone's first smoke test"
  deferred_at: 2026-10-12

## Gaps
