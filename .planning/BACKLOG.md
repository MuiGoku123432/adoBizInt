# Backlog

Items deferred out of a phase because they require a new decision, not a correction
within the phase that surfaced them.

---

## BL-01 — TUI write path bypasses every Phase 4 guard

**Surfaced:** Phase 4 (guarded-writes), live verification 2026-09-12
**Requirements affected:** SAFE-01, SAFE-02, SAFE-03
**Severity:** High — confirmed silent data loss against a live work item

**Part 1 (blank field zeroing) — FIXED AND CLOSED 2026-09-13.** Quick task `260913-nd2`
(commits `b2f2ae2`, `266b88b`). Unit-verified, then confirmed by a live TUI re-test on
2026-09-13: a Task closed through the modal with a blank effort field left that field
unchanged in work item History. SAFE-03 is now satisfied on both the CLI and TUI paths.

**Part 2 (no dry run / no confirmation / no audit on the TUI write path) — STILL OPEN.**
This is now the whole of BL-01. The sections below still apply to it; the `parseFloat` /
`EffortSubmitMsg` rows in the table are historical and describe the fixed part 1.

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

---

## BL-02 — Open v1.0 review warnings and follow-ups

**Surfaced:** Phase 4 verification and Phase 5 review, live CLI smoke test, 2026-10-12
**Severity:** Low to medium. Each fails loudly or safely; none allows a silent mutation.

Phase 4 review findings still open on HEAD (`04-REVIEW.md`):

- **WR-02:** with the user identity unresolved at startup, `prs approve` cannot detect an existing approval. The dry run shows `reviewer.vote: 0 -> 10` for an already-approved PR, and `--confirm` re-casts the vote and audits it as a change.
- **WR-05:** a flag value of `-h` (e.g. `--comment -h`) prints help and exits 0 even with `--confirm`. Nothing is written.
- **WR-06:** a stdout write failure after the mutation (e.g. piping into `head`) returns exit 3 "nothing ran", although the write happened and was audited.
- **WR-01:** `hoistLeadingPositionals` has no tests, and its comment claims flag-first ordering works when it does not (currently fails with exit 3).
- **WR-04:** `audit.go:15-17` comment wrongly says Phase 5 covers the audit file's permissions (see T-05-26).

Follow-ups found during Phase 5 and the 2026-10-12 live smoke test:

- `branches list` always exits 1: five empty repositories return `VS403403: Cannot find any branches`, which Phase 2's empty-repository rule (HTTP 404 only) does not suppress.
- `audit.jsonl` and `priorities.yaml` are created 0644, and the audit sink can create the config dir 0755 if `logging.Init` fails (T-05-26, IN-07).
- `--project P,P` duplicates pass the allow-list and both copies reach the read.
- `go.mod` still marks `google/uuid` as `// indirect` (IN-01); CI `persist-credentials` default (IN-02).
- `~/.config/adoBizInt/adoBizInt.log` is never rotated (426 MB on 2026-10-12).
- TUI `Error:` lines render raw server/SDK error text (`internal/ui/workitems/detail.go:118`).
- Deferred live checks for the next milestone's first smoke test: one TUI attachment download, one `set-state --confirm` with the project in the PATCH route, and the Phase 1 eager-identity failure/recovery path.
