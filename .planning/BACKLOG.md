# Backlog

Items deferred out of a phase because they require a new decision, not a correction
within the phase that surfaced them.

---

## BL-01 — TUI write path bypasses every Phase 4 guard

**Surfaced:** Phase 4 (guarded-writes), live verification 2026-09-12
**Requirements affected:** SAFE-01, SAFE-02, SAFE-03
**Severity:** High — confirmed silent data loss against a live work item

### What happens

Closing a work item through the Bubble Tea effort modal silently overwrites any effort
field left blank with `0`. Confirmed empirically: a Task closed with Original Estimate
left blank had its Original Estimate set to 0. No preview, no confirmation prompt, and
no audit record — the change left no trace in `audit.jsonl`.

### Why

| Location | Problem |
|---|---|
| `internal/ui/workitems/effort.go:193-199` | `parseFloat("")` returns `0, nil` — blank becomes `0.0`, not "absent" |
| `internal/ui/workitems/effort.go:18-27` | `EffortSubmitMsg` carries plain `float64`, so the modal has no representation for "leave unchanged" |
| `internal/ui/workitems/workitems.go:338-347, 362-374` | All three values are always passed as non-nil pointers. In-source comment: *"All three are ALWAYS non-nil here — TUI behaviour is unchanged."* |
| — | The TUI path does not route through `cmd/adoBizInt`'s `runWrite`, so it inherits none of the dry-run / `--confirm` / audit / outcome-classification guards |

### Why it was not fixed in Phase 4

`04-CONTEXT.md` **D-17** locked "preserve TUI behaviour exactly" — which is why
`TestUpdateWorkItemStateWithEffortIssuesExactPatch` asserts a byte-identical
four-operation patch document. Plan 04-02 deliberately plumbed the nullable `*float64`
signature to the `ado` boundary and stopped there. The CLI path got the fix; the TUI
kept its prior behaviour by design.

Changing it therefore reverses a locked decision. That is a new decision, not a
Phase 4 defect fix.

### Proposed remediation (needs a discuss-phase)

1. Widen `EffortSubmitMsg` to `*float64` for the three effort fields; `parseFloat("")`
   returns `nil` rather than `0`.
2. Route TUI writes through the same guarded path as the CLI so they inherit the audit
   record and outcome classification. Dry run and `--confirm` need a TUI-native
   equivalent — a confirmation step in the modal — rather than a literal flag.
3. Add a test that drives the modal end to end. It is currently the only write surface
   with no automated coverage.

Decide explicitly whether the TUI should gain a confirmation step. D-04 framed
`--confirm` as "an accident guard, not an authorization boundary"; a modal that already
required deliberate keystrokes may not need a second gate, but a silent effort
overwrite is exactly the accident the guard exists to catch.

### Open question

Whether prior TUI-driven closes have already zeroed effort fields on real work items.
None of them were audited, so there is no local record to check. Azure DevOps work item
History is the only source of truth.
