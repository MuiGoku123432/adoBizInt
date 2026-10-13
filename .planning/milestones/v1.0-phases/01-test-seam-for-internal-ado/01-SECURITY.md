---
phase: 01
slug: test-seam-for-internal-ado
status: verified
# Only open threats at or above the configured high threshold count here.
threats_open: 0
asvs_level: 1
created: 2026-09-06
verified: 2026-09-06
---

# Phase 01 -- Security

> Per-phase security contract for the `internal/ado` test seam and identity-recovery gap closure.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Test process to ADO client seam | Package-local fakes terminate calls without live Azure DevOps access | Synthetic SDK argument values only |
| ADO location response to reviewer identity | The authenticated identity determines which account casts a pull-request vote | `AuthenticatedUser.Id` |
| Identity resolution to Git write | Approval must not write until a valid authenticated identity is available | Reviewer ID and vote request |

---

## Threat Register

| Threat IDs | Severity | Disposition | Mitigation | Status |
|------------|----------|-------------|------------|--------|
| T-01-01, T-01-12, T-01-17, T-01-23 | medium/high | mitigate | Tests contain no PAT, URL, connection, or HTTP construction; warnings and errors contain no credential or connection-data object. | closed |
| T-01-02, T-01-08, T-01-10, T-01-14, T-01-18, T-01-19, T-01-26 | medium/high | mitigate | Exact call assertions and recorded red/restore/green mutation checks prevent vacuous certification; production source is clean after each mutation. | closed |
| T-01-04, T-01-09, T-01-11, T-01-22, T-01-25 | high | mitigate | Reviewer identity comes only from `AuthenticatedUser.Id`; exact requests, release status divergence, and pre-Git identity guards are tested. | closed |
| T-01-05, T-01-15, T-01-20, T-01-21 | medium/high | mitigate | Failure behavior, structural exceptions, and the Phase 2 inversion contract are explicitly recorded and tested. | closed |
| T-01-16, T-01-24, T-01-27 | medium/high | mitigate | Non-empty fixtures prevent early-return false positives; cache hits bypass lookup; nil providers return safely without Git calls. | closed |
| T-01-03 | low | accept | The five-method work-item interface is package-local and bounded to methods actually consumed; it creates no runtime exposure. | closed |
| T-01-07 | low | accept | The complete interface surface is package-local and capped at the verified 5/7/4/4 method sets. | closed |
| T-01-13 | low | accept | The in-process, no-I/O test suite completes in seconds; no additional runtime mitigation is warranted. | closed |

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01-01 | T-01-03 | Package-local impersonation is limited to the five consumed work-item methods and has no production exposure. | Phase 01 plan | 2026-09-06 |
| AR-01-02 | T-01-07 | The 20-method aggregate seam is split into four narrow package-local interfaces and guarded by exact method-count checks. | Phase 01 plan | 2026-09-06 |
| AR-01-03 | T-01-13 | Test runtime is bounded, in-process, and free of network I/O. | Phase 01 plan | 2026-09-06 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-06 | 27 | 27 | 0 | gsd-security-auditor |

Evidence: credential-cleared targeted tests, `go test -race ./internal/ado`, `go build ./...`, `go vet ./...`, fail-fast guard inspection, mutation restoration checks, and source/log scans all passed.

---

## Sign-Off

- [x] All threats have a disposition
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-06
