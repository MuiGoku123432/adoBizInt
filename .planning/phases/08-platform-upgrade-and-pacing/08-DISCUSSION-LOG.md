# Phase 8: Platform Upgrade and Pacing - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md -- this log preserves the alternatives considered.

**Date:** 2026-10-16
**Phase:** 08-platform-upgrade-and-pacing
**Areas discussed:** Throttle vs deadline, Throttle visibility, PAT host trust, Live smoke run

---

## Throttle vs deadline

**Q1: ADO's requested wait is longer than the time left before a read's deadline.**

| Option | Description | Selected |
|--------|-------------|----------|
| Fail fast as rate_limit | Return a rate_limit failure now; true cause, fast exit | ✓ |
| Wait, then time out | Sleep until the deadline, fail as timeout | |
| Wait what fits, then send | Partially honour the delay, send anyway | |

**Q2: A write never left the process (held in the pacer past its deadline, or refused for its host).**

| Option | Description | Selected |
|--------|-------------|----------|
| Failed / exit 4 | Typed not-sent error checked before ctx.Err() in guard.classify; failed, no audit | ✓ |
| Unknown / exit 5 (as today) | Leave guard.classify alone | |
| Failed + audit not_sent | New audit outcome value (one-way door) | |

**Notes:** Correction made after the discussion. The option label said "exit 4", but exit 4 is `exitNoOp` (`cmd/adoBizInt/dispatch.go:28`). Per `writeExitCodeFor`, outcome `failed` exits 1 (or 2 for auth). The choice that was made (failed rather than unknown, no audit record) stands, recorded with the correct exit code in CONTEXT D-02. The user was told about the correction when the CONTEXT was written.

**Q3: Read retries after a 429.**

| Option | Description | Selected |
|--------|-------------|----------|
| 2 retries, fixed | 3 attempts max, hardcoded | ✓ |
| 2 retries, configurable | Config key | |
| Until deadline | No count | |

**Q4: Pacer policy.**

| Option | Description | Selected |
|--------|-------------|----------|
| Obey explicit delays only | Retry-After / X-RateLimit-Delay; log Remaining/Limit | ✓ |
| Also slow down proactively | Invented threshold on Remaining | |

---

## Throttle visibility

**Q1: A CLI read was paced or retried and still succeeded.**

| Option | Description | Selected |
|--------|-------------|----------|
| Log + stderr line | Log file + one stderr line, JSON unchanged | ✓ |
| Log file only | Invisible to the caller | |
| Log + JSON field | New envelope field (one-way door) | |

**Q2: TUI.**

| Option | Description | Selected |
|--------|-------------|----------|
| Nothing new in TUI | Log only; existing failure banner covers rate_limit | ✓ |
| Status-line hint | "ADO throttling: waiting 12s" | |

**Q3: rate_limit failure detail.**

| Option | Description | Selected |
|--------|-------------|----------|
| Say it in the message | Requested wait vs budget left, or retries exhausted | ✓ |
| Add retry_after_s field | Failure contract change | |
| Kind only | Generic message | |

**Q4: Several throttle events in one run.**

| Option | Description | Selected |
|--------|-------------|----------|
| One summary at end | Single stderr line | ✓ |
| One line per event | Burst of lines | |

---

## PAT host trust

**Q1: Hosts allowed to receive the PAT.**

| Option | Description | Selected |
|--------|-------------|----------|
| Bound to configured org | org_url host; *.dev.azure.com with /{org}/ path; {org}[.*].visualstudio.com | ✓ |
| Microsoft suffix allow-list | Any *.dev.azure.com / *.visualstudio.com | |
| org_url host only | Breaks vsrm release client | |

**Q2: Enforcement point.**

| Option | Description | Selected |
|--------|-------------|----------|
| Both: startup + per request | Startup exit 3 on bad LocationUrl; transport refuses per request incl. redirects | ✓ |
| Per request only | | |
| Startup only | Misses redirects; weaker than SC4 | |

**Q3: Transport location vs TestNoRawHTTPInADO.**

| Option | Description | Selected |
|--------|-------------|----------|
| New package, test untouched | internal/ado receives a ready *http.Client | ✓ |
| internal/ado + amend test | Weakens a Phase 5 security assertion | |

**Q4: org_url validation.**

| Option | Description | Selected |
|--------|-------------|----------|
| https + cloud shape | dev.azure.com/{org} or {org}.visualstudio.com, else exit 3 | ✓ |
| https only, any host | Two rule paths incl. on-prem | |
| No new validation | http:// would send PAT in cleartext | |

---

## Live smoke run

**Q1: Writes executed live.**

| Option | Description | Selected |
|--------|-------------|----------|
| Dry runs + scratch item | 6 dry runs + CLI set-state --confirm + TUI Task close on one scratch item | ✓ |
| Dry runs only | Exactly SC5 | |
| Execute every verb | Includes non-reversible PR/release/pipeline writes | |

**Q2: Form.**

| Option | Description | Selected |
|--------|-------------|----------|
| Script + TUI checklist | scripts/smoke.sh with pass/fail table + manual TUI checklist; never in CI | ✓ |
| Go test, build tag | `live` tag in the test tree | |
| Manual checklist only | | |

**Q3: What blocks Phase 9.**

| Option | Description | Selected |
|--------|-------------|----------|
| Block on regressions only | Only v7/pacer/host-rule regressions block; v1.0 issues recorded | ✓ |
| Block on any failure | Pulls BL-02 into Phase 8 | |
| Record, judge later | No written rule | |

**Q4: Dormant work client vs "one live read per sub-client".**

| Option | Description | Selected |
|--------|-------------|----------|
| Real call sites only | wit, git, build, release, location; work gap recorded; no core client | ✓ |
| Add smoke-only work read | Code with no product caller | |
| Also build core client now | Pulls Phase 9 scope forward | |

---

## Claude's Discretion

- Toolchain patch pin, x/sys v0.47.0, govulncheck v1.9.0 via `go run`, CI `go-version-file`
- Plan split and commit granularity within the locked order
- Transport package name and API, pacer state scope, any cap on a single Retry-After
- How the CLI gathers counters for the stderr summary
- Classification of a per-request host refusal on a read (must not be auth or timeout)
- Whether the pacer deletes `X-TFS-Session` (SessionId race)
- Whether the TUI checklist adds attachment download and eager-identity recovery
- Updating b5 line references in result.go comments

## Deferred Ideas

- JSON throttle field / `retry_after_s` on Failure
- TUI throttle status-line hint
- Proactive pacing on X-RateLimit-Remaining
- Configurable read-retry count
- On-prem Azure DevOps Server support
- Live execution of PR/release/pipeline writes
