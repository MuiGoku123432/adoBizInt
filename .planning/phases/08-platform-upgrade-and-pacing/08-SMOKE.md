---
status: testing
phase: 08-platform-upgrade-and-pacing
source: [08-04-PLAN.md, 08-08-PLAN.md, 08-12-PLAN.md]
started: 2026-10-20T17:03:00Z
updated: 2026-10-21T20:41:18Z
sc5_live_smoke: re-run pending (08-12)
sc1_ci_on_push: pending user push (08-12)
---

# Phase 8: Live Smoke Record (PLAT-05)

This is the record of the live smoke run against the real organization (08-CONTEXT D-13 to D-16). Unit tests run against SDK-typed fakes and cannot see the 5.1 to 7.1-preview wire change, so this run is the gate before Phase 9.

How it runs: the user exports the SMOKE_* targets documented in the header comment of `scripts/smoke.sh`, runs the script from the repo root with their normal ADO config, works through the manual checklist below, and pastes the results. The script never runs in CI and never passes the confirm flag. Phase 9 is blocked only by regressions that the v7 upgrade, the pacer or the host rule caused, or by an unwaived dry-run SKIP (D-13, D-15).

Never paste the PAT, the credential header line, or the config file into this record.

## Outcome (re-run)

pending: filled from the 08-12 re-run

## Binary provenance

pending: the two provenance lines scripts/smoke.sh prints on the re-run

## Live evidence actually gathered (TUI sessions, 2026-10-21)

Source: the 167 lines dated 2026-10-21 in ~/.config/adoBizInt/adoBizInt.log (counts and message names only; no PAT or identity value was read into this record).

| Session | Window (local, -05:00) | Startup | Reads logged (count, failure_count) |
|---------|------------------------|---------|-------------------------------------|
| 1, `./adoBizInt` | 11:25:54 to 11:35:41 | Config loaded, Current user identity, ADO client created successfully, Starting TUI | dashboard counts (7,883 work items, 12 PRs, 61 pipelines, 0); work items (7,883, 0); branches (566, 0) |
| 2, go run build | 11:35:49 onward | Config loaded, Current user identity, ADO client created successfully, Starting TUI | dashboard counts (7,885, 12, 61, 0); work items (7,885, 0); pull requests (12, 0); branches (566, 0); pipeline runs (11, 0); pipeline definitions (11, 0); releases (5, 0); pending approvals (61, 0) |

Across both sessions: 0 ERROR lines, 0 host-rule refusals, 0 VS403 or api-version rejections, 0 pacing, rate-limit or retry messages, 0 "Could not cache current user ID" lines, and 4 WARN "Failed to get branch stats" lines (TF401019, see Findings). "ADO client created successfully" in both sessions means resource-area discovery and the D-10a every-area host check passed at startup. The release reads succeeding means the vsrm host was allowed by the host rule.

## Script output

Paste the full script output inside the fence, verbatim, every line starting at column 0 as the script prints it, including any `throttled by ADO:` detail. Reads pass on exit 0 with complete true. Dry runs pass on exit 0 with outcome dry_run; NODIFF means exit 4 (the pre-read ran and found nothing to change); SKIP means the row's SMOKE_* target was unset and nothing ran.

```
(pending: 08-12 re-run)
```

## Sub-client coverage (D-16)

One live read per ADO sub-client that has a call site. Fill Result from the matching rows of the script output.

| Sub-client | Live check in scripts/smoke.sh | Result |
|------------|--------------------------------|--------|
| workitemtracking | `workitems list`, `dashboard counts` | Script rows not run (waived). Live TUI read on the v7 binary: PASS. Dashboard counts and work items, failure_count=0 in both sessions (7,883 then 7,885 work items) |
| git | `prs list`, `branches list` | Script rows not run (waived). Live TUI read: PASS. Pull requests 12 and branches 566, failure_count=0 |
| build | `pipelines runs`, `pipelines definitions` | Script rows not run (waived). Live TUI read: PASS. Pipeline runs 11 and definitions 11, failure_count=0 |
| release (vsrm host) | `releases list`, `releases approvals` | Script rows not run (waived). Live TUI read: PASS. Releases 5 and pending approvals 61, failure_count=0; vsrm host allowed, no refusal |
| location (identity) | `identity`: GetConnectionData runs at client construction; the log lines this run appended must not contain "Could not cache current user ID" | Script row not run (waived). Same criterion applied to the log lines of both TUI sessions: PASS. "Current user identity" logged at construction in both, 0 "Could not cache current user ID" lines dated 2026-10-21 |
| work | none: the work client is dormant | Named gap, not a miss (D-16). The work client is constructed through v7 resource-area discovery, the host rule and the startup location check, but it has no call site, so it gets no read until Phase 9 adds its first verb. Both 2026-10-21 sessions passed that construction ("ADO client created successfully"). |

## Dry-run waivers (D-13)

D-13 requires a dry run of every one of the six write verbs, so each `dry-run <noun> <verb>` row in the script output must read PASS or NODIFF. A SKIP row is accepted only with a waiver, one per SKIP row, written by the user and never drafted, suggested or completed by the executor. A waiver uses the exact form 08-08 Task 2 defines, `- waiver: dry-run <noun> <verb>: <reason>`, written as its own list line, where the text between the prefix and the next colon is the SKIP row's CHECK name exactly as the script prints it. The 08-08 record gate reads every list line in this section that begins with that prefix as a user waiver and matches it to a SKIP row by name, which is why this template holds no example line. A SKIP row that no waiver names is listed under Blocks Phase 9, and the record status becomes blocked.

No waivers recorded.

## Manual checklist

Run every item with a binary built from this tree, never the one on PATH (08-RESEARCH Pitfall 12). The script deletes its own binary on exit, so build one from the repo root with `GOTOOLCHAIN=go1.27.3 go build -o /tmp/adoBizInt-v7 ./cmd/adoBizInt` and confirm that `go version -m /tmp/adoBizInt-v7` shows the same two lines as Binary provenance.

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
result: pass. The user reports "I ran the tui it seems to be wokring?" and asked for the second run to be verified. Session 2's log shows every view's read completing with failure_count=0 (dashboard, work items, pull requests, branches, pipeline runs, pipeline definitions, releases, pending approvals) and 0 ERROR lines; session 1 shows the same for dashboard, work items and branches. The log cannot show which tabs were opened on screen (session 2's reads all land in its first 22 seconds, consistent with startup loads). The 4 branch-stats WARN lines are pre-existing (Findings). Kept from the 2026-10-21 sessions on the go1.27.2 build by user decision (2026-10-21) and not re-run; it predates the 08-10 host-rule hardening, so the post-hardening proof for every sub-client's reads is the re-run's script read rows

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

No regression caused by the v7 upgrade, the pacer or the host rule was observed in what ran, and every row that did not run is under the user's blanket waiver, so this table is empty. That is not evidence that the unrun paths are regression-free (see Outcome).

| Check | Observed | Evidence |
|-------|----------|----------|

### Pre-existing, recorded in BACKLOG.md (does not block)

| Check | Observed | Evidence |
|-------|----------|----------|
| checklist 4 | Branch stats for repositories UQUAL and uqual-service-df in project Process fail with TF401019 and are logged at WARN as "Failed to get branch stats", twice per branches read. The read itself is correct: they are not recorded as failures (branches 566, failure_count=0), which matches the D-06b disabled-repository skip (404 GitRepositoryNotFoundException), so the v7 error still carries the status and typeKey that skip needs | adoBizInt.log 2026-10-21 11:26:11 (session 1) and 11:35:59 (session 2); the same two repositories logged the same warning under the b5 builds from 2026-08-17T09:05:09 through 2026-10-12 (44 lines before v7, 4 on v7); BACKLOG.md BL-02 |

## Not covered

- Eager-identity recovery (the Phase 1 path that retries the identity lookup when the startup GetConnectionData call failed): exercising it needs a controlled GetConnectionData failure that a manual run cannot produce, because taking the network away fails resource-area discovery first (g-01-1). It stays carried in BACKLOG.md BL-02 (08-RESEARCH Open Question 8).
- Everything listed under Outcome as still unproven, waived by the user on 2026-10-21 and carried in BACKLOG.md BL-02.

## Prior attempt: 2026-10-21, waived (superseded by the 08-12 re-run)

From the former Outcome (2026-10-21) section:

**The live smoke was not run as planned. SC5 (live smoke) and SC1's CI half were WAIVED by the user, not met.**

- What ran: two TUI sessions on 2026-10-21 against https://dev.azure.com/Windstream (projects Kinetic and Process), both on a v7.1.0 / go1.27.2 binary built from this tree. Every TUI read completed with no failure (see Live evidence below).
- What did not run: `scripts/smoke.sh` (no CLI read rows, no identity row, none of the six dry runs), manual checklist items 1, 2, 3 and 5, and the CI run on a pushed dev branch. No write of any kind went over the v7 wire.
- Status is `complete` because, under the 08-08 Task 3 rule, it is `complete` exactly when Blocks Phase 9 is empty: no v7, pacer or host-rule regression was observed in what ran, and the user's blanket waiver (quoted verbatim under Dry-run waivers) covers every row that did not run. `complete` here means "Phase 9 is not blocked by this record", not "SC5 passed".
- The Phase 7 precondition of 08-08 Task 2 (07-VERIFICATION.md `status: passed`) was not met (still `human_needed`, 9 pending UAT rows). The user's message 3 below overrides it.

Still unproven after this record:

- The CLI read path live: exit codes, the `--json` envelope and `complete: true` for the eight read verbs, and the `throttled by ADO:` stderr line.
- Every write verb's dry-run pre-read and plan on v7 (set-state, PR approve, PR complete, release approve, release reject, pipeline queue).
- Any executed write on v7: the set-state PATCH with the project in the route (BL-02 deferred check), the TUI two-PATCH Task close (07-REVIEW IN-01), and an audit record written by the v7 build.
- TUI attachment download under the host rule (08-RESEARCH A3).
- The TUI confirm modals dismissed with n and esc against the live org.
- The govulncheck step passing in CI on a pushed dev branch (local govulncheck v1.9.0 was clean in 08-08 Task 1).
- Live pacing: ADO did not throttle either session, so the pacer, the read retries and the throttle summary were not exercised live (they are unit-tested).
- Eager-identity recovery (unchanged; see Not covered).

From the former Binary provenance section:

`scripts/smoke.sh` was not run, so there are no `provenance:` lines from it. These are `go version -m` lines the executor took on 2026-10-21 from the two binaries that actually ran (temp directory of the go run build elided). Both link azuredevops/v7 at v7.1.0 with go1.27.2, which rules out the old binary on PATH (08-RESEARCH Pitfall 12): `~/go/bin/adoBizInt` is go1.26.5 with azuredevops v1.0.0-b5 and was not used.

```
./adoBizInt: go1.27.2
	dep	github.com/microsoft/azure-devops-go-api/azuredevops/v7	v7.1.0	h1:mmJCWLe63QvybxhW1iBmQWEaCKdc4SKgALfTNZ+OphU=
	build	vcs.revision=988d95d9842d61493b60865cb14aed45876537fe
	build	vcs.modified=true
<go-build temp dir>/b001/exe/adoBizInt: go1.27.2
	dep	github.com/microsoft/azure-devops-go-api/azuredevops/v7	v7.1.0	h1:mmJCWLe63QvybxhW1iBmQWEaCKdc4SKgALfTNZ+OphU=
```

- `./adoBizInt` (repo root, git-ignored, built 2026-10-21 11:25 local) ran session 1. Its vcs.revision is HEAD at the time (988d95d); 988d95d and bf5ad85 change only planning docs, so the Go code is the 08-07 tree. vcs.modified=true reflects untracked planning files only (milestone.lock, state.json, research cache); no Go source was modified.
- The go run build (`go run ./cmd/adoBizInt`, no vcs stamp) ran session 2 and was still running when this record was written.

From the former Dry-run waivers (D-13) section, below its rules paragraph:

No per-row waivers recorded. The script did not run, so there are no SKIP rows; none of the six dry runs executed.

Instead the user gave one blanket waiver. Their three messages of 2026-10-21, verbatim and in order:

> I ran the tui it seems to be wokring?  https://dev.azure.com/Windstream

> I ran the new one so verify it looks good

> lets just move on

Message 3 is recorded as the user's decision to (a) override the 08-08 Task 2 Phase 7 precondition and (b) waive every smoke row and checklist item that was not run, plus the CI confirmation. The executor applied it to: all eight script read rows, the identity row, all six dry runs (`dry-run workitems set-state`, `dry-run prs approve`, `dry-run prs complete`, `dry-run releases approve`, `dry-run releases reject`, `dry-run pipelines queue`), checklist items 1, 2, 3 and 5, and the CI row. No per-row reason was given and none is supplied here.
