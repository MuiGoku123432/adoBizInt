---
status: partial
phase: 01-test-seam-for-internal-ado
source: [01-VERIFICATION.md]
started: 2026-09-06T06:19:31Z
updated: 2026-09-07T01:03:57Z
---

# Phase 01 -- User Acceptance Testing

## Current Test

[testing paused -- 1 blocked item]

## Tests

### 1. NewClient eager identity failure and recovery
expected: Construction remains non-fatal, the warning describes retry behavior, successful eager lookup caches AuthenticatedUser.Id, and subsequent approval can recover through the retained provider.
result: blocked
reported: "With Wi-Fi disabled, ./adoBizInt exits before opening: Error connecting to Azure DevOps: Options https://dev.azure.com/windstream/_apis: dial tcp: lookup dev.azure.com: no such host"
blocked_by: third-party
reason: "Turning off Wi-Fi prevents mandatory Azure DevOps SDK OPTIONS service discovery before the optional identity lookup is reached. Requires a controlled transport/endpoint that permits service discovery while failing only GetConnectionData."

## Summary

total: 1
passed: 0
issues: 0
pending: 0
skipped: 0
blocked: 1

## Gaps

No code gap recorded. The attempted offline test did not reach the identity-recovery path; see `.planning/debug/g-01-1-offline-startup-block.md`.
