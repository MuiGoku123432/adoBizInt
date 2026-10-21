---
schema_version: 1
open_count: 1
waived_count: 4
fixed_count: 0
total_count: 5
last_updated: 2026-10-21T20:58:14.119Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | 01 | deviation | .planning/phases/01-test-seam-for-internal-ado/01-04-PLAN.md |  | GetWorkItems continue-deletion mutation was behaviorally inert; mutation proof used continue-to-return error propagation instead. | open |  | 2026-09-04T22:50:01.377Z |  |
| 2 | 08 | unrun-verify | scripts/smoke.sh |  | 08-08 Task 2 live smoke not run: no smoke.sh read/identity rows, none of the six dry runs, checklist 1, 2, 3 and 5 (CLI set-state confirm, TUI two-PATCH close, attachment download, modal dismissals); 08-08 Task 3 record gate fails on row counts as a result. Only TUI reads ran on v7 (08-SMOKE.md) | waived | User waiver 2026-10-21, verbatim: "lets just move on" | 2026-10-21T16:50:28.003Z | 2026-10-21T16:50:34.900Z |
| 3 | 08 | unrun-verify | .github/workflows/ci.yml |  | PLAT-02 SC1 CI half unconfirmed: dev not pushed, govulncheck step never observed green in CI (local govulncheck v1.9.0 clean) | waived | User waiver 2026-10-21, verbatim: "lets just move on" | 2026-10-21T16:50:28.110Z | 2026-10-21T16:50:35.111Z |
| 4 | 08 | unrun-verify | scripts/smoke.sh |  | 08-12 live smoke re-run on the go1.27.3 build not run: no smoke.sh read/identity rows, none of the six dry runs, checklist 1, 2, 3 and 5 (a, b, c, e); 08-12 Task 3 record gate fails on row counts as a result. No live call from the fixed tree; 08-10 host rule and 08-11 pacer unexercised live | waived | User waiver 2026-10-21, verbatim: "I am willing to move to phase 9 as is" | 2026-10-21T20:58:06.987Z | 2026-10-21T20:58:14.013Z |
| 5 | 08 | unrun-verify | .github/workflows/ci.yml |  | 08-12 PLAT-02 SC1 CI half still unconfirmed: dev not pushed (148 commits ahead of origin/dev per local ref), govulncheck step never observed green in CI on go1.27.3 (local govulncheck v1.9.0 clean) | waived | User waiver 2026-10-21, verbatim: "I am willing to move to phase 9 as is" | 2026-10-21T20:58:07.113Z | 2026-10-21T20:58:14.119Z |

````json
[
  {
    "id": 1,
    "kind": "deviation",
    "phase": "01",
    "file": ".planning/phases/01-test-seam-for-internal-ado/01-04-PLAN.md",
    "line": null,
    "description": "GetWorkItems continue-deletion mutation was behaviorally inert; mutation proof used continue-to-return error propagation instead.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-04T22:50:01.377Z",
    "resolved_at": null
  },
  {
    "id": 2,
    "kind": "unrun-verify",
    "phase": "08",
    "file": "scripts/smoke.sh",
    "line": null,
    "description": "08-08 Task 2 live smoke not run: no smoke.sh read/identity rows, none of the six dry runs, checklist 1, 2, 3 and 5 (CLI set-state confirm, TUI two-PATCH close, attachment download, modal dismissals); 08-08 Task 3 record gate fails on row counts as a result. Only TUI reads ran on v7 (08-SMOKE.md)",
    "status": "waived",
    "reason": "User waiver 2026-10-21, verbatim: \"lets just move on\"",
    "recorded_at": "2026-10-21T16:50:28.003Z",
    "resolved_at": "2026-10-21T16:50:34.900Z"
  },
  {
    "id": 3,
    "kind": "unrun-verify",
    "phase": "08",
    "file": ".github/workflows/ci.yml",
    "line": null,
    "description": "PLAT-02 SC1 CI half unconfirmed: dev not pushed, govulncheck step never observed green in CI (local govulncheck v1.9.0 clean)",
    "status": "waived",
    "reason": "User waiver 2026-10-21, verbatim: \"lets just move on\"",
    "recorded_at": "2026-10-21T16:50:28.110Z",
    "resolved_at": "2026-10-21T16:50:35.111Z"
  },
  {
    "id": 4,
    "kind": "unrun-verify",
    "phase": "08",
    "file": "scripts/smoke.sh",
    "line": null,
    "description": "08-12 live smoke re-run on the go1.27.3 build not run: no smoke.sh read/identity rows, none of the six dry runs, checklist 1, 2, 3 and 5 (a, b, c, e); 08-12 Task 3 record gate fails on row counts as a result. No live call from the fixed tree; 08-10 host rule and 08-11 pacer unexercised live",
    "status": "waived",
    "reason": "User waiver 2026-10-21, verbatim: \"I am willing to move to phase 9 as is\"",
    "recorded_at": "2026-10-21T20:58:06.987Z",
    "resolved_at": "2026-10-21T20:58:14.013Z"
  },
  {
    "id": 5,
    "kind": "unrun-verify",
    "phase": "08",
    "file": ".github/workflows/ci.yml",
    "line": null,
    "description": "08-12 PLAT-02 SC1 CI half still unconfirmed: dev not pushed (148 commits ahead of origin/dev per local ref), govulncheck step never observed green in CI on go1.27.3 (local govulncheck v1.9.0 clean)",
    "status": "waived",
    "reason": "User waiver 2026-10-21, verbatim: \"I am willing to move to phase 9 as is\"",
    "recorded_at": "2026-10-21T20:58:07.113Z",
    "resolved_at": "2026-10-21T20:58:14.119Z"
  }
]
````
