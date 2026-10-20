---
status: testing
phase: 08-platform-upgrade-and-pacing
source: [08-04-PLAN.md]
started: 2026-10-20T17:03:00Z
updated: 2026-10-20T17:03:00Z
---

# Phase 8: Live Smoke Record (PLAT-05)

This is the record of the live smoke run against the real organization (08-CONTEXT D-13 to D-16). Unit tests run against SDK-typed fakes and cannot see the 5.1 to 7.1-preview wire change, so this run is the gate before Phase 9.

How it runs: the user exports the SMOKE_* targets documented in the header comment of `scripts/smoke.sh`, runs the script from the repo root with their normal ADO config, works through the manual checklist below, and pastes the results. The script never runs in CI and never passes the confirm flag. Phase 9 is blocked only by regressions that the v7 upgrade, the pacer or the host rule caused, or by an unwaived dry-run SKIP (D-13, D-15).

Never paste the PAT, the credential header line, or the config file into this record.

## Binary provenance

The two `provenance:` lines `scripts/smoke.sh` prints before any ADO call. They must name the go.mod toolchain (go1.27.2) and the azuredevops/v7 module at v7.1.0, which proves the run exercised the fresh v7 build and not an older binary on PATH (08-RESEARCH Pitfall 12).

```
(not yet run)
```

## Script output

Paste the full script output inside the fence, verbatim, every line starting at column 0 as the script prints it, including any `throttled by ADO:` detail. Reads pass on exit 0 with complete true. Dry runs pass on exit 0 with outcome dry_run; NODIFF means exit 4 (the pre-read ran and found nothing to change); SKIP means the row's SMOKE_* target was unset and nothing ran.

```
(not yet run)
```

## Sub-client coverage (D-16)

One live read per ADO sub-client that has a call site. Fill Result from the matching rows of the script output.

| Sub-client | Live check in scripts/smoke.sh | Result |
|------------|--------------------------------|--------|
| workitemtracking | `workitems list`, `dashboard counts` | pending |
| git | `prs list`, `branches list` | pending |
| build | `pipelines runs`, `pipelines definitions` | pending |
| release (vsrm host) | `releases list`, `releases approvals` | pending |
| location (identity) | `identity`: GetConnectionData runs at client construction; the log lines this run appended must not contain "Could not cache current user ID" | pending |
| work | none: the work client is dormant | Named gap, not a miss (D-16). The work client is constructed through v7 resource-area discovery, the host rule and the startup location check, but it has no call site, so it gets no read until Phase 9 adds its first verb. |

## Dry-run waivers (D-13)

D-13 requires a dry run of every one of the six write verbs, so each `dry-run <noun> <verb>` row in the script output must read PASS or NODIFF. A SKIP row is accepted only with a waiver, one per SKIP row, written by the user and never drafted, suggested or completed by the executor. A waiver uses the exact form 08-08 Task 2 defines, `- waiver: dry-run <noun> <verb>: <reason>`, written as its own list line, where the text between the prefix and the next colon is the SKIP row's CHECK name exactly as the script prints it. The 08-08 record gate reads every list line in this section that begins with that prefix as a user waiver and matches it to a SKIP row by name, which is why this template holds no example line. A SKIP row that no waiver names is listed under Blocks Phase 9, and the record status becomes blocked.

No waivers recorded.

## Manual checklist

Run every item with a binary built from this tree, never the one on PATH (08-RESEARCH Pitfall 12). The script deletes its own binary on exit, so build one from the repo root with `GOTOOLCHAIN=go1.27.2 go build -o /tmp/adoBizInt-v7 ./cmd/adoBizInt` and confirm that `go version -m /tmp/adoBizInt-v7` shows the same two lines as Binary provenance.

### 1. CLI: workitems set-state on the scratch item (SMOKE_WORKITEM_ID) to SMOKE_STATE, first as a dry run, then with the confirm flag
expected: The dry run exits 0 with outcome dry_run. The confirmed run exits 0 with outcome executed, the state change appears in the item's ADO history, and ~/.config/adoBizInt/audit.jsonl gains one record. The write goes through the PATCH route that carries the project, which closes the BL-02 deferred check
result: [pending]

### 2. TUI: close the scratch Task with effort (Remaining 0, Completed above 0), move on, and confirm with y
expected: One confirm modal lists the state change and the effort changes. Both PATCHes succeed, state first and then effort (07-REVIEW IN-01): no process rule rejects the state write, and the effort write is not refused as changed since preview. ~/.config/adoBizInt/audit.jsonl gains two records with actor tui. If the item is already closed, reopen it first so the two-PATCH path runs
result: [pending]

### 3. TUI: download an attachment from a work item that has one
expected: The file is saved and no host-rule refusal appears; attachment content is assumed to be served from the org's own host (08-RESEARCH A3). If it is refused, record the refused host from the error
result: [pending]

### 4. TUI: visit every view
expected: Every view loads its data with no failure banner
result: [pending]

### 5. TUI: open each confirm modal (PR approve, PR complete, release approve, release reject, pipeline queue) and dismiss it once with n and once with esc
expected: Nothing changes in ADO and no audit record is appended
result: [pending]

## CI

| Check | Run URL | Conclusion |
|-------|---------|------------|
| govulncheck step, CI run for the pushed dev branch | pending | pending |

## Findings

Each problem goes in exactly one of the two tables below (D-15). The first column names the check: the CHECK name from the script table in backticks, or checklist N for a manual item. A table with no finding keeps only its header and separator lines.

### Blocks Phase 9 (v7, pacer or host-rule regression, or an unwaived dry-run SKIP)

| Check | Observed | Evidence |
|-------|----------|----------|

### Pre-existing, recorded in BACKLOG.md (does not block)

| Check | Observed | Evidence |
|-------|----------|----------|

## Not covered

- Eager-identity recovery (the Phase 1 path that retries the identity lookup when the startup GetConnectionData call failed): exercising it needs a controlled GetConnectionData failure that a manual run cannot produce, because taking the network away fails resource-area discovery first (g-01-1). It stays carried in BACKLOG.md BL-02 (08-RESEARCH Open Question 8).
