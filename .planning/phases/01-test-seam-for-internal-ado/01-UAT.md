---
status: complete
phase: 01-test-seam-for-internal-ado
source: [01-VERIFICATION.md]
started: 2026-09-06T06:19:31Z
updated: 2026-09-07T04:27:32Z
---

# Phase 01 -- User Acceptance Testing

## Current Test

[testing complete]

## Tests

### 1. NewClient eager identity failure and recovery
expected: Construction remains non-fatal, the warning describes retry behavior, successful eager lookup caches AuthenticatedUser.Id, and subsequent approval can recover through the retained provider.
result: skipped
reported: "With Wi-Fi disabled, ./adoBizInt exits before opening: Error connecting to Azure DevOps: Options https://dev.azure.com/windstream/_apis: dial tcp: lookup dev.azure.com: no such host"
reason: "Deferred follow-up: Requires a controlled transport/endpoint that permits mandatory SDK service discovery while selectively failing and recovering GetConnectionData. The Wi-Fi-off attempt did not reach this behavior, and no code defect was found."

## Summary

total: 1
passed: 0
issues: 0
pending: 0
skipped: 1
blocked: 0

## Gaps

No code gap recorded. The attempted offline test did not reach the identity-recovery path; see `.planning/debug/g-01-1-offline-startup-block.md`.

## Deferred Follow-Ups

- test: 1
  idea: "Add a controlled transport/endpoint harness for NewClient identity failure and recovery when that infrastructure is justified."
  deferred_at: 2026-09-07
