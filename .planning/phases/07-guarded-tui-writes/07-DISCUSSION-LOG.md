# Phase 7: Guarded TUI Writes - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-13
**Phase:** 07-guarded-tui-writes
**Areas discussed:** Confirm step UX, Single-shot writes, Failure & outcome display, audit.jsonl 0644 risk

---

## Confirm step UX

| Option | Description | Selected |
|--------|-------------|----------|
| Small modal | Overlay with field diffs and y/n, matches existing modals | ✓ |
| Status-line prompt | One line in statusMsg; truncates multi-field diffs | |
| You decide | | |

| Option (what counts as moving on) | Description | Selected |
|--------|-------------|----------|
| Any focus change | Row, tab, open item, quit all prompt | ✓ |
| Explicit key only | Stays staged with pending marker | |
| Cursor move only | Tab/quit discards with warning | |

| Option (answer 'n') | Description | Selected |
|--------|-------------|----------|
| Discard and move on | Nothing sent, nothing audited | ✓ |
| Cancel the move | n keeps edits staged | |
| Three choices | y / n / esc | |

| Option (audit dry runs) | Description | Selected |
|--------|-------------|----------|
| No, match the CLI | Only confirmed writes are audited | ✓ |
| Yes, audit discards too | Outcome 'declined' | |

**Notes:** The modal preview showed `[esc]`, so esc was taken to mean "cancel the move, keep the edits staged". The user accepted this.

---

## Single-shot writes

| Option | Description | Selected |
|--------|-------------|----------|
| Same confirm modal | One confirm UI; replaces confirmComplete | ✓ |
| Modal only for destructive | Approve without prompt | |
| Keep status-line y/n | Extend confirmComplete | |

| Option (input modals) | Description | Selected |
|--------|-------------|----------|
| Input then confirm | Input, then pre-read, then diff modal | ✓ |
| Input modal is the confirm | No second prompt | |
| You decide | | |

| Option (no-op) | Description | Selected |
|--------|-------------|----------|
| Status line, no modal | "No change" message, nothing sent | ✓ |
| Modal with empty diff | Dismiss only | |

---

## Failure & outcome display

| Option (unknown) | Description | Selected |
|--------|-------------|----------|
| Sticky warning + refresh | Stays until dismissed; re-fetch item | ✓ |
| Treat like failed | Hides the may-have-landed distinction | |

| Option (resolve partial) | Description | Selected |
|--------|-------------|----------|
| Report both, no rollback | Two audit records show what happened | ✓ |
| Offer re-try of effort | Re-open modal pre-filled | |
| Effort first, then state | Reverse order | |

| Option (audit sink fails) | Description | Selected |
|--------|-------------|----------|
| Refuse the write | Same as CLI | ✓ |
| Block all writes for session | Banner until restart | |

---

## audit.jsonl 0644 risk

| Option | Description | Selected |
|--------|-------------|----------|
| Pull in: new 0600/0700 | Applies to newly created files only | ✓ |
| Pull in + tighten existing | Also chmod existing files | |
| Accepted risk | Leave in BL-02 | |

**Notes:** Checked that no CLI test asserts the file mode, so GUARD-06 holds.

## Claude's Discretion

- Modal layout and wording, project resolution for work item writes, per-write context budget, `user_approved` field naming, structural test mechanism.

## Deferred Ideas

- Retry affordance for the failed second write of a resolve
- chmod of existing audit files
- Remaining BL-02 items
- Checking ADO History for earlier unaudited TUI effort zeroing
