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
- ~~**WR-05:** a flag value of `-h` (e.g. `--comment -h`) prints help and exits 0 even with `--confirm`. Nothing is written.~~ — **closed** by Phase 6 plan 06-01 (D-08): a help token is recognised only in flag position, so --comment -h is a comment value
- **WR-06:** a stdout write failure after the mutation (e.g. piping into `head`) returns exit 3 "nothing ran", although the write happened and was audited.
- **WR-01:** `hoistLeadingPositionals` has no tests, and its comment claims flag-first ordering works when it does not (currently fails with exit 3).
- **WR-04:** `audit.go:15-17` comment wrongly says Phase 5 covers the audit file's permissions (see T-05-26).

Follow-ups found during Phase 5 and the 2026-10-12 live smoke test:

- ~~`branches list` always exits 1 on empty repositories~~ — **closed 2026-10-12** by 02-07 (D-06a, `a40606a`) and D-06b (`f73cfa4`); live run exits 0, `complete: true`.
- `audit.jsonl` and `priorities.yaml` are created 0644, and the audit sink can create the config dir 0755 if `logging.Init` fails (T-05-26, IN-07).
- `--project P,P` duplicates pass the allow-list and both copies reach the read.
- ~~`go.mod` still marks `google/uuid` as `// indirect` (IN-01)~~ — **closed 2026-10-20** by 08-01 (`81058bd`): `go mod tidy` with the v7 swap moved `google/uuid` and `atotto/clipboard` to direct; CI `persist-credentials` default (IN-02) still open.
- `~/.config/adoBizInt/adoBizInt.log` is never rotated (426 MB on 2026-10-12).
- TUI `Error:` lines render raw server/SDK error text (`internal/ui/workitems/detail.go:118`).
- Deferred live checks for the next milestone's first smoke test: one TUI attachment download, one `set-state --confirm` with the project in the PATCH route, and the Phase 1 eager-identity failure/recovery path. **Still open after 08-SMOKE.md (2026-10-21):** the 08-08 live smoke ran only TUI reads; set-state with the confirm flag and the TUI attachment download were not run (waived by the user), and eager-identity recovery remains uncovered (it needs a controlled GetConnectionData failure). **Still open after the 08-12 decision (2026-10-21):** the re-run that was to close set-state with the confirm flag and the attachment download was waived by the user as well (see the 08-12 bullet below); nothing here is struck through.
- A fully rejected PAT exits 3 at startup, not 2: resource-area discovery is the first authenticated call, so the 401 surfaces as a client-construction error before any read or write runs (08-RESEARCH Pitfall 3, Open Question 1). Unchanged by the v7 swap and kept by user decision (08-CONTEXT SC2 clarification, 2026-10-19); exit 2 is proven on the read and write paths by `TestV7AuthFailureExitsAuth`. Offline startup (`g-01-1`) is likewise unchanged by v7 and carried forward.

- **From the v1.0 milestone audit (2026-10-12), core-value warnings:** W2 help tokens inside flag values (`--search help`) exit 0 with empty stdout on reads; W3 an empty `projects` list passes validation and reads report `complete: true`; W4 `workitems list` silently excludes Done/Closed/Removed and older iterations. Also W1 panic after a sent write exits 3 with no audit record. Full list in `.planning/v1.0-MILESTONE-AUDIT.md`.
- Pipeline reads (`pipelines runs`, `pipelines definitions`) return an empty, `complete: true` result when no pipelines are configured — same class as audit W3 (Phase 6 D-16 deferred it here).
- ADO pull-request API may cap `prs list` at a default page size (~100) when no limit is set — unverified (Phase 6 research).
- Branch-stats WARN noise for disabled repositories (08-SMOKE.md, 2026-10-21, pre-existing, non-blocking under 08-CONTEXT D-15): every branches read logs `Failed to get branch stats` with TF401019 for repositories UQUAL and uqual-service-df in project Process (44 lines under b5 from 2026-08-17 to 2026-10-12, 4 on v7). The read is correct: they are skipped as disabled repositories (D-06b, 404 GitRepositoryNotFoundException) and not counted as failures (branches failure_count=0). The issues are the WARN level for an expected skip and the stale "Empty repositories" comment at `internal/ado/branches.go:143`; v7's `GitRepository.IsDisabled` field could replace the D-06b error match.
- 08-08 live smoke items waived by the user on 2026-10-21, carried open. The user's words, verbatim: "I ran the tui it seems to be wokring?  https://dev.azure.com/Windstream", "I ran the new one so verify it looks good", "lets just move on". Waived and not run: `scripts/smoke.sh` (the eight CLI `--json` read rows, the identity row, and all six dry runs: `dry-run workitems set-state`, `dry-run prs approve`, `dry-run prs complete`, `dry-run releases approve`, `dry-run releases reject`, `dry-run pipelines queue`); 08-SMOKE.md checklist 1 (CLI set-state with the confirm flag), 2 (TUI two-PATCH Task close, 07-REVIEW IN-01), 3 (TUI attachment download, 08-RESEARCH A3) and 5 (confirm-modal dismissals); and the govulncheck step in CI on a pushed dev branch (PLAT-02 SC1). No write has gone over the v7 wire yet. Rerun the script and the checklist before the first Phase 9 write verb or on the next SDK or toolchain bump. **Not closed by 08-12:** the re-run was waived (next bullet), so every item in this bullet stays open.
- 08-12 live smoke re-run on the go1.27.3, host-rule-hardened build waived by the user on 2026-10-21, carried open. The user's words, verbatim: "I am willing to move to phase 9 as is". Waived and not run: the dev push and its CI run with the govulncheck step (PLAT-02 SC1 CI half; dev is 148 commits ahead of origin/dev per the local remote-tracking ref); `scripts/smoke.sh` on the fixed build (the eight CLI `--json` read rows, the identity row, and all six dry runs: `dry-run workitems set-state`, `dry-run prs approve`, `dry-run prs complete`, `dry-run releases approve`, `dry-run releases reject`, `dry-run pipelines queue`); 08-SMOKE.md checklist 1 (a, CLI set-state with the confirm flag), 2 (b, TUI two-PATCH Task close, 07-REVIEW IN-01), 3 (c, TUI attachment download under the hardened host rule, 08-REVIEW IN-05) and 5 (e, confirm-modal dismissals). Still unproven live: every write over v7 (none has run; audit.jsonl unchanged since 2026-09-12), every dry run, the attachment download under the 08-10 host rule, the 08-10 host rule and the 08-11 send budget and hold cap against real ADO, and live pacing (ADO has never throttled a v7 run). PLAT-05 and PLAT-02 stay Pending in REQUIREMENTS.md. Recommended: make the first Phase 9 live run double as the host-rule smoke (run `scripts/smoke.sh` and checklist 3 on the Phase 9 build before its first write verb ships, and treat any refused host or path as a host-rule regression); the push and CI check costs almost nothing (`git push origin dev`, then confirm the govulncheck step) and can be done at any time.
