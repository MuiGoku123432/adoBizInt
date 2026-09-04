---
schema_version: 1
open_count: 1
waived_count: 0
fixed_count: 0
total_count: 1
last_updated: 2026-09-04T22:50:01.377Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | 01 | deviation | .planning/phases/01-test-seam-for-internal-ado/01-04-PLAN.md |  | GetWorkItems continue-deletion mutation was behaviorally inert; mutation proof used continue-to-return error propagation instead. | open |  | 2026-09-04T22:50:01.377Z |  |

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
  }
]
````
