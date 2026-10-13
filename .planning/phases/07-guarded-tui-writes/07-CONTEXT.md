# Phase 7: Guarded TUI Writes - Context

**Gathered:** 2026-10-13
**Status:** Ready for planning

<domain>
## Phase Boundary

Every write made from the TUI is guarded and audited exactly like a CLI write,
through one guard package both surfaces share (`internal/guard`, extracted from
`cmd/adoBizInt/write.go`, `audit.go`, `exit.go`). Covers the 9 existing TUI write
call sites: work item state cycle, compound resolve (state + effort), PR approve,
PR complete, release approve, release reject, pipeline queue. No new write
capabilities. Requirements GUARD-01..06.

Already locked by REQUIREMENTS.md (not re-discussed): stage-per-item and confirm
once on move-on with one audit record per changed field (GUARD-03); `actor` key
(`tui` / `cli-human` / `agent`) plus a `--user-approved` field on every audit
record (GUARD-04); blank effort = absent, not zero (GUARD-05); guard never writes
stdout or exits; CLI write tests pass unchanged (GUARD-06). Compliance: SOX ITGC
audit evidence, so the TUI path must not be weakened to confirm-only.

</domain>

<decisions>
## Implementation Decisions

### Confirm step UX (work items)
- **D-01:** Staged diffs are confirmed in a small modal overlay listing each field change (`System.State  Active → Closed`, `RemainingWork 3 → 0`) with keys `[y] write  [n] discard  [esc] stay`. Same visual family as the existing effort and trigger modals.
- **D-02:** Any focus change triggers the confirm for staged edits on an item: moving to another row, switching tabs, opening another item, or quitting the app. Nothing can stay staged silently.
- **D-03:** `n` discards the staged edits and the navigation proceeds. `esc` cancels the navigation and leaves you on the item with the edits still staged.
- **D-04:** Declined or discarded TUI edits produce no audit record. This matches the CLI, where an unconfirmed dry run is not audited, so the shared guard behaves the same on both surfaces.

### Single-shot writes (PRs, releases, pipelines)
- **D-05:** PR approve, PR complete, release approve, release reject and pipeline queue all go through the same confirm modal as D-01, showing the pre-read diff (e.g. `reviewer.vote 0 → 10`, PR status `active → completed`). This replaces the ad-hoc `confirmComplete` status-line prompt in `internal/ui/prs/prs.go`. — **Reversibility:** reversible
- **D-06:** Where an input modal already exists (release approve/reject comment, pipeline branch in `trigger_modal.go`), submitting the input runs the guard pre-read, then shows the diff confirm modal. That makes two steps: input, then confirm.
- **D-07:** A no-op from the pre-read (e.g. PR already approved by you, release already approved) skips the modal. The status line shows `No change: #<id> already <state>` and nothing is sent, matching the CLI no-op refusal.

### Failure and outcome display
- **D-08:** An `unknown` outcome shows as a sticky error-styled status line (`#<id>: outcome unknown — verify in ADO`) that stays until dismissed. The view then re-fetches the item so the real state is visible. The audit records `unknown`.
- **D-09:** Compound resolve stays state write first, effort write second. If the second write fails or is unknown, both results are reported (`#<id> closed; effort update FAILED`) and the item is refreshed. There is no automatic rollback, because a compensating write would be an unguarded guess. The two audit records show exactly what happened.
- **D-10:** If the audit sink cannot be opened, the TUI refuses that write exactly as the CLI does. An error status line shows (`write blocked: audit log not writable: <path>`) and nothing is sent. Writes are not disabled for the whole session.

### Audit file permissions (pulled in from BL-02 / T-05-26)
- **D-11:** When the audit opener moves into `internal/guard`, newly created audit files use mode `0600` and the directory uses `0700`, replacing `0644`/`0755` (`cmd/adoBizInt/audit.go:77-80`). Existing files keep their current mode: there is no chmod on open. Verified: no CLI test asserts the file mode, so GUARD-06 is unaffected. Reason: v2.0 starts writing agent-supplied text to this log.

### Claude's Discretion
- Exact modal layout, styling and wording beyond the keys and content above.
- How the TUI resolves the real project for work item writes (call sites pass `""` today), and the bounded-context budget per write (reuse `fetchctx`, moving to `internal/fetchctx`).
- The name and type of the `user_approved` audit field. It is always `false` from the TUI and present on every record.
- How the structural test (GUARD-02) detects direct `ado` mutation calls from `internal/ui`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements and scope
- `.planning/REQUIREMENTS.md` §Guarded TUI Writes (BL-01) — GUARD-01..06
- `.planning/ROADMAP.md` §Phase 7 — success criteria, compliance note, 0644 note
- `.planning/BACKLOG.md` §BL-01 — history, D-17 reversal rationale, remediation outline; §BL-02 — audit file mode item now resolved by D-11
- `.planning/research/SUMMARY.md` — `internal/guard` shape (`guard.Run(ctx, op, confirm) Report`), decisions 4 and 9, Pitfall 14 (lock CLI write tests first; extraction must be byte-identical)

### Prior phase decisions
- `.planning/milestones/v1.0-phases/04-guarded-writes/04-CONTEXT.md` (D-01..D-24, esp. D-04 confirm-as-accident-guard, D-11 record shape, D-12/D-13 append-only audit opened before mutation, D-17 TUI-preservation now reversed)
- `.planning/codebase/CONVENTIONS.md` — single-use inline, rule of three, files < 300 lines

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/adoBizInt/write.go` — `writeOp` (resolve/plan/mutate), `runWrite`, `fieldChange`, `writeTarget`, `writeRecord`, `diffWorkItem`, `appendFloatChange`, per-verb op constructors (`setStateOp`, `prsApproveOp`, `prsCompleteOp`, `releaseApprovalOp`, `pipelinesQueueOp`). This is the body that moves to `internal/guard`.
- `cmd/adoBizInt/audit.go` — `auditRecord` (5 keys today, append-only, so new keys are a one-way door), `openAuditSink`, `appendAuditRecord`.
- `cmd/adoBizInt/exit.go` — outcome constants `dry_run`, `no_op`, `executed`, `failed`, `unknown`.
- `internal/ui/fetchctx/fetchctx.go` — bounded contexts. It moves to `internal/fetchctx`.
- `internal/ui/workitems/effort.go` — the effort modal, already nullable after quick task `260913-nd2`.

### Established Patterns
- TUI writes return `tea.Cmd` closures that emit result messages (`PRActionMsg`, `ApprovalResultMsg`, state-update msg) and set `statusMsg` / `statusErr`.
- Auto-refresh is suppressed while a confirm is pending (`prs.go:195-197`). The new modal must do the same in every view.

### Integration Points (current direct mutation calls, all `context.Background()`)
- `internal/ui/workitems/workitems.go:266, 336, 350, 360` — state cycle and resolve (state + effort)
- `internal/ui/prs/prs.go:361, 384` — approve and complete
- `internal/ui/releases/releases.go:206, 214` — approve and reject
- `internal/ui/pipelines/pipelines.go:205` — queue
- Prioritize/unprioritize (`p`/`u`) is local-only (`priorities.Save()`) and is not an ADO write, so it is out of guard scope.

</code_context>

<specifics>
## Specific Ideas

Confirm modal mock agreed in discussion:

```
┌ Confirm #12345 ─────────────┐
│ System.State  Active → Closed│
│ RemainingWork 3 → 0          │
│                              │
│ [y] write  [n] discard  [esc]│
└──────────────────────────────┘
```

</specifics>

<deferred>
## Deferred Ideas

- Retry affordance for a failed second write in compound resolve (re-open the effort modal pre-filled). Considered and not chosen. Could revisit with GROOM.
- Tightening the mode of pre-existing `audit.jsonl` files (chmod on open). Rejected for now because it silently mutates user files.
- The rest of BL-02 (approval detection with an unresolved identity, stdout failure exit code, log rotation) stays in the backlog.
- Open question from BL-01: whether earlier unaudited TUI closes zeroed effort fields on real work items. This can only be checked in ADO History and is not phase work.

</deferred>

---

*Phase: 07-guarded-tui-writes*
*Context gathered: 2026-10-13*
