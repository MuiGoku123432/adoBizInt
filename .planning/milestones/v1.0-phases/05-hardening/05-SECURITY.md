---
phase: "05"
slug: "hardening"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-10-12"
---

# Phase 05 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Client → Azure DevOps org | All ADO calls go through the SDK connection built from `cfg.OrgURL` | PAT (Basic auth), work item / PR / release data |
| Server response → local filesystem | Attachment names and bytes from ADO written under `~/.config/adoBizInt/attachments` | Untrusted file names and content |
| Agent argv → CLI | `--project` and write targets supplied by an agent or script | Untrusted identifiers that select read and write scope |
| Server / internal error → terminal | Names and error text rendered in the TUI or on stderr | Untrusted strings that may carry control bytes |
| Process → log file | `slog` output persisted to `~/.config/adoBizInt/adoBizInt.log` | Operational data; must never include the PAT |
| GitHub Actions runner → repository | CI workflow executes repo code on push and pull request | `GITHUB_TOKEN` (read-only); no ADO secrets |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-05-01 | Information Disclosure | `DownloadAttachment` sending the PAT to a relation-URL host (C3) | high | mitigate | ID-only `DownloadAttachment(uuid.UUID)` via SDK `GetAttachmentContent` (`internal/ado/attachments.go:96,102`); no `net/http` in non-test code; `TestNoRawHTTPInADO` | closed |
| T-05-02 | Tampering / EoP | Attachment name → file path (H6) | high | mitigate | `attachmentPath` rejects abs/control/`.`/`..`, exact-child check, runs before fetch (`attachments.go:34-55,97`); `TestAttachmentPath`, `TestDownloadAttachmentRefusesBeforeFetch` | closed |
| T-05-03 | Tampering | Truncated file left after mid-body failure | low | mitigate | Temp file always removed; final name only via `os.Link` after successful copy (`attachments.go:119-142`) | closed |
| T-05-04 | Denial of Service | Unbounded attachment size | low | accept | See accepted risks (D-04) | closed |
| T-05-05 | Information Disclosure | Redirect carrying PAT to blob storage | medium | accept | See accepted risks | closed |
| T-05-06 | EoP | Symlink planted in attachments dir | low | accept | See accepted risks | closed |
| T-05-07 | Tampering | Control bytes in server names reaching the terminal | low | mitigate | `%q` on all `attachmentPath` errors; control runes refused and stripped from TUI list (`attachments.go:37-41`, `ui/workitems/detail.go:283-289`) | closed |
| T-05-08 | Denial of Service | `*def.Id` dereference (MED4) | medium | mitigate | `Name == nil \|\| Id == nil` guards (`pipelines.go:157`, `releases.go:104`); `TestCollectedDefinitionsGuardNameAndID` | closed |
| T-05-09 | Denial of Service | Other unguarded SDK dereferences | low | accept | See accepted risks (D-13) | closed |
| T-05-10 | Tampering | Action tags re-pointed upstream | high | mitigate | Actions pinned to 40-hex SHAs (`.github/workflows/ci.yml:17-18`), verified against upstream tags | closed |
| T-05-11 | EoP | Workflow `GITHUB_TOKEN` scope | medium | mitigate | `permissions: contents: read` (`ci.yml:9-10`) | closed |
| T-05-12 | Information Disclosure | ADO credentials exposed to CI / fork PRs | high | mitigate | No secrets, `ADO_*` or env in `ci.yml`; tests use fixtures only; CI run 36798351937 green without secrets | closed |
| T-05-13 | Tampering | Fork PR runs untrusted test code | low | accept | See accepted risks | closed |
| T-05-14 | Tampering / Info Disclosure | Agent `--project` reaching WIQL and write targets (H5) | high | mitigate | Allow-list in single `newFlagSet` (`cmd/adoBizInt/flags.go:198-215`), exit 3; CR-01 set-state ownership check and project in PATCH route (`write.go:320-321`, `internal/ado/workitems.go:282,350,414`); verified live 2026-10-12 | closed |
| T-05-15 | EoP | Case-folded match redirecting a write | medium | mitigate | Exact `slices.Contains` (`flags.go:204`); case-mismatch test row | closed |
| T-05-16 | Tampering | Escape bytes in echoed project value | low | mitigate | `%q` in rejection text (`flags.go:208,210`) | closed |
| T-05-17 | Information Disclosure | Rejection text on stdout (OUT-03) | low | mitigate | `fs.SetOutput(stderrWriter)` (`flags.go:186`); empty-stdout assertions | closed |
| T-05-18 | Information Disclosure | Stderr lists configured project names | low | accept | See accepted risks | closed |
| T-05-19 | Tampering / Info Disclosure | Project concatenated into WIQL (H5) | high | mitigate | `@project` macro at both sites with project in the route (`workitems.go:148,151`, `dashboard.go:136,139`); `TestWIQLInterpolationStructural`, `TestWIQLCheckCatchesBypasses` | closed |
| T-05-20 | Tampering | Iteration path containing a quote | medium | mitigate | `iterationClause` doubles `'` (`internal/ado/client.go:289-298`) | closed |
| T-05-21 | Information Disclosure | Query text persisted to the log | medium | mitigate | Logs carry only project and iteration count (`workitems.go:155`, `dashboard.go:143`) | closed |
| T-05-22 | DoS (functional) | `@project` resolving differently on the live org | medium | mitigate | Live before/after comparison passed 2026-10-12 (05-UAT test 2): identical ID sets, Kinetic 1651 / Process 6316 | closed |
| T-05-23 | Information Disclosure | World-readable config dir and log (MED2, T-03-15) | high | mitigate | Dir 0700 / log 0600 set on every `Init` (`internal/logging/logging.go:26-38`); live install confirmed 0700/0600 | closed |
| T-05-24 | Information Disclosure | slog attribute rendering the PAT | medium | mitigate | `TestPATNeverLogged` (raw, Basic, base64 forms, non-vacuous); `Config` `LogValue`/`String`/`GoString` redaction (`internal/config/config.go:60-72`); live 426 MB log scanned 2026-10-12 with zero PAT occurrences | closed |
| T-05-25 | Denial of Service | chmod failure breaking startup | low | accept | See accepted risks (D-21) | closed |
| T-05-26 | Information Disclosure | `audit.jsonl` created 0644 | low | accept | See accepted risks | closed |
| T-05-SC | Tampering | Dependency installs (declared in all six plans) | high | mitigate | `go.mod`/`go.sum` unchanged across the phase; new imports stdlib only; CI actions official and SHA-pinned | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-05-01 | T-05-04 | Downloads are human-initiated, one at a time, from the operator's own org; byte cap deferred with the CLI download verb (D-04) | Plan 05-01 / CONTEXT D-04 | 2026-09-28 |
| AR-05-02 | T-05-05 | The SDK's default `http.Client` drops `Authorization` on a redirect to a non-matching domain | Plan 05-01 | 2026-09-30 |
| AR-05-03 | T-05-06 | Planting a symlink needs local write access to `~/.config/adoBizInt` (now 0700), outside the threat model; `os.Root` recorded for the deferred CLI verb | Plan 05-01 | 2026-09-30 |
| AR-05-04 | T-05-09 | Repo-wide dereference sweep rejected (D-13); `handlePanic` recover is the backstop | Plan 05-02 / CONTEXT D-13 | 2026-09-28 |
| AR-05-05 | T-05-13 | Fork PRs get a read-only token and there are no secrets to take | Plan 05-03 | 2026-09-30 |
| AR-05-06 | T-05-18 | Stderr lists the operator's own configured project names; no credential | Plan 05-04 | 2026-09-30 |
| AR-05-07 | T-05-25 | chmod failure falls back to the existing non-fatal warning and `slog.Default()` (D-21) | Plan 05-06 / CONTEXT D-21 | 2026-09-28 |
| AR-05-08 | T-05-26 | `audit.jsonl` 0644 sits under the 0700 config dir; residual: if `Init` fails entirely the audit sink could create the dir 0755. Follow-up carried to the next milestone | Plan 05-06 | 2026-09-30 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-10-12 | 27 | 27 | 0 | gsd-security-auditor (ASVS L1, block_on high) |

Informational observations from the 2026-10-12 audit (not registered threats, non-blocking):

- `internal/ui/workitems/detail.go:118` renders raw server/SDK error text in the TUI; candidate follow-up threat alongside BL-01.
- Live `--confirm` write with the project in the PATCH route (CR-01) and the live TUI attachment download were deferred by the user to the v2 milestone's first smoke test; cross-project refusal was verified live.
- TUI writes pass an empty project (unscoped route); deferred with BL-01 part 2.
- Review info items IN-02 (`persist-credentials` default) and IN-07 (`priorities.go` 0644/0755) remain deferred.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-10-12
