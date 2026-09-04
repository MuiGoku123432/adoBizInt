---
phase: 01-test-seam-for-internal-ado
plan: 02
subsystem: testing
tags: [go, azure-devops, interfaces, fakes, identity]

requires:
  - phase: 01-test-seam-for-internal-ado/01-01
    provides: Work-item API seam, fail-fast fake convention, and network-free test injection pattern
provides:
  - Four narrow package-local ADO sub-client interfaces with exactly 20 consumed methods
  - Cached authenticated-user ID for network-free pull request approval tests
  - Compile-checked fail-fast fakes for work item, Git, build, and release APIs
affects: [01-03, 01-04, 01-05, phase-4-guarded-writes]

actuals:
  tokens: 3350
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - SDK interface narrowing through existing NewClient assignments
    - Construction-time identity caching from AuthenticatedUser.Id
    - Test-only function-field fakes with compile-time interface assignments

key-files:
  created: []
  modified:
    - internal/ado/interfaces.go
    - internal/ado/client.go
    - internal/ado/pullrequests.go
    - internal/ado/fakes_test.go

key-decisions:
  - "Cache AuthenticatedUser.Id, not AuthorizedUser.Id, because delegated access can distinguish the authenticated and authorized identities."
  - "Keep identity resolution non-fatal during NewClient construction, but warn that pull request approval will fail when the ID is unavailable."
  - "Keep workClient concrete and intentionally dormant until the v2 scrum requirements consume it."

patterns-established:
  - "Interface ceiling: the four package-local interfaces expose only the 5/7/4/4 SDK methods internal/ado actually calls."
  - "Fake conformance: each test fake is assigned to its narrow interface and fails by interface and method name for every unstubbed call."

requirements-completed: [SEAM-01, SEAM-02]

coverage:
  - id: D1
    description: "All four live ADO sub-clients use narrow package-local interfaces totaling exactly 20 consumed methods."
    requirement: SEAM-01
    verification:
      - kind: integration
        ref: "go build ./... && go vet ./..."
        status: pass
      - kind: other
        ref: "structural interface count check: 4 interfaces, 5/7/4/4 methods"
        status: pass
    human_judgment: false
  - id: D2
    description: "Pull request approval reads a cached AuthenticatedUser.Id and no longer constructs a location client per approval."
    requirement: SEAM-02
    verification:
      - kind: integration
        ref: "go build ./... && go vet ./..."
        status: pass
      - kind: other
        ref: "executable-code location reference count in pullrequests.go equals zero"
        status: pass
    human_judgment: false
  - id: D3
    description: "Work item, Git, build, and release fakes compile against their interfaces and fail by name on all 20 unstubbed methods."
    requirement: SEAM-02
    verification:
      - kind: unit
        ref: "go test ./internal/ado/... -cover"
        status: pass
      - kind: other
        ref: "go vet ./... plus 20 Fatalf guard count"
        status: pass
    human_judgment: false

duration: 9 min
completed: 2026-09-04
status: complete
---

# Phase 1 Plan 2: Complete ADO Test Seam Summary

**Four narrow ADO interfaces, authenticated-user ID caching, and compile-checked fail-fast fakes make every existing read and write path reachable without network construction.**

## Performance

- **Duration:** 9 min
- **Started:** 2026-09-04T21:56:37Z
- **Completed:** 2026-09-04T22:05:37Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments

- Expanded the proven work-item seam to Git, build, and release clients with exactly the 20 SDK methods used by `internal/ado`.
- Cached `AuthenticatedUser.Id` during `NewClient` construction while preserving the existing `AuthorizedUser.ProviderDisplayName` read and non-fatal startup behavior.
- Removed the per-approval location-client round trip so `ApprovePullRequest` can run from a struct-literal `Client` containing only `currentUserID` and `gitClient`.
- Added Git, build, and release fakes matching the existing work-item fake style, with compile-time interface checks and one fail-fast guard per method.

## Task Commits

Each task was committed atomically:

1. **Task 1: Complete the interface set and retype the remaining three fields** -- `2eada6a` (feat)
2. **Task 2: Resolve the current user ID once and remove the approval location call** -- `3325366` (refactor)
3. **Task 3: Add Git, build, and release fakes** -- `95e2033` (test)

## Files Created/Modified

- `internal/ado/interfaces.go` -- Defines the 7-method Git, 4-method build, and 4-method release interfaces alongside the existing 5-method work-item interface.
- `internal/ado/client.go` -- Retypes all four live sub-clients, preserves the dormant work client, and caches the authenticated-user ID.
- `internal/ado/pullrequests.go` -- Uses the cached user ID and preserves the existing empty-ID error text.
- `internal/ado/fakes_test.go` -- Adds compile-checked Git, build, and release function-field fakes with fail-fast guards.

## Verification

- `gofmt -l internal/` -- passed with no output.
- `go build ./...` -- passed.
- `go vet ./...` -- passed.
- `go test ./... -race -cover` -- passed; `internal/ado` remains at 2.1% pending the write tests in plan 01-03.
- Structural interface check -- passed with four interfaces and method split `5/7/4/4`, total 20.
- Fake guard check -- passed with 20 `Fatalf` calls and fake method split `5/7/4/4`.
- Scope check -- passed; no `internal/ui/`, `cmd/`, `go.mod`, or `go.sum` changes.
- Identity check -- passed; display name still reads `AuthorizedUser`, cached ID reads `AuthenticatedUser.Id`, and the warning contains no credential reference.

## Decisions Made

- Cached `AuthenticatedUser.Id` specifically rather than `AuthorizedUser.Id`; this preserves the identity used by the previous approval implementation under delegated access.
- Preserved non-fatal construction when connection-data lookup fails. The new warning exposes the lost approval capability without logging credentials.
- Left `workClient` on the SDK interface and documented its intentional dormancy because it has no current call sites and is reserved for v2 scrum work.

## Deviations from Plan

None -- plan executed exactly as written.

## Issues Encountered

- Context7 was unavailable. The pinned Azure DevOps SDK signatures were taken from the phase research, which had verified them against the local module source, and were confirmed by the Go compiler and vet.
- Gortex's generic review gate flagged the changed production methods as lacking direct tests. This is the intentional phase split: plan 01-02 establishes the remaining seams and fakes, while plan 01-03 adds the write-path tests. All verification required by this plan passed.

## Known Stubs

None.

## User Setup Required

None -- no external service configuration required.

## Next Phase Readiness

- All four production sub-clients and their test fakes are ready for plan 01-03's exact-call write tests.
- `ApprovePullRequest` is now testable with a nil connection while preserving its prior empty-identity error.
- Phase 1 remains in progress; three plans remain after this one.

## Self-Check: PASSED

- Modified files exist: `internal/ado/interfaces.go`, `internal/ado/client.go`, `internal/ado/pullrequests.go`, and `internal/ado/fakes_test.go`.
- Task commits exist: `2eada6a`, `3325366`, and `95e2033`.
- All task acceptance criteria and plan-level build, vet, formatting, race, and test commands passed.
- No planned source mutation remains uncommitted; only the explicitly excluded orchestration artifacts are untracked.

---
*Phase: 01-test-seam-for-internal-ado*
*Completed: 2026-09-04*
