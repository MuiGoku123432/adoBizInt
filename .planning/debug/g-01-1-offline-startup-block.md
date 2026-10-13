---
status: diagnosed
trigger: 'G-01-1: "it did not open when I turned off wifi". Expected NewClient construction to remain non-fatal when eager identity lookup fails. Diagnose only; do not modify production code.'
created: 2026-09-07T00:55:57Z
updated: 2026-09-07T01:10:00Z
audit_acknowledged:
  milestone: v1.0
  at: 2026-10-13
  status: diagnosed
---

## Current Focus

hypothesis: The fatal OPTIONS request is mandatory SDK resource-area discovery performed by the first generated service-client constructor, before the optional location.GetConnectionData identity lookup; disabling Wi-Fi invalidates the UAT isolation because it disables both calls.
test: Inspect the pinned Azure DevOps Go SDK constructors and connection discovery implementation, trace NewClient call order, and compare the pre-Phase-1 NewClient revision.
expecting: workitemtracking.NewClient calls connection.GetClientByResourceArea, which issues OPTIONS to {OrgURL}/_apis and returns the DNS error before execution reaches location.NewClient or GetConnectionData; the same mandatory constructor existed before Phase 1.
next_action: Verify constructor-line provenance and dependency stability against the last pre-Phase-1 commit, then commit the diagnosis and UAT disposition to Resolution.
bug_class: bohrbug
known_pattern_candidate: none; semantic recall returned no matching prior resolution
candidate_causes:

  - code: Phase 1 may have moved optional GetConnectionData before mandatory service-client initialization or made identity failure fatal.
  - environment: Wi-Fi-off removes DNS/network access needed by mandatory Azure DevOps SDK resource-area discovery as well as optional identity lookup.

and_gate: no for regression classification; either baseline ordering changed or the UAT setup prevents baseline-required discovery.

## Symptoms

expected: NewClient construction remains non-fatal when eager identity lookup fails; the TUI opens even when identity lookup cannot reach Azure DevOps.
actual: The application did not open when Wi-Fi was turned off.
errors: No error message reported initially. Exact continuation stderr: Error connecting to Azure DevOps: Options "https://dev.azure.com/windstream/_apis": dial tcp: lookup dev.azure.com: no such host
reproduction: Turn off Wi-Fi, then launch the application; do not reproduce by disabling networking during this investigation.
started: Observed during Phase 01 UAT after implementation of the internal ADO test seam.

## Eliminated

## Evidence

- timestamp: 2026-09-07T00:55:57Z
  checked: User-provided UAT report and requested scope.
  found: Failure is observed only as non-opening while offline; no immediate error output or elapsed time was supplied.
  implication: Investigation must distinguish a fatal return from an unbounded or long-blocking request before the TUI starts.

- timestamp: 2026-09-07T01:10:00Z
  checked: User-provided continuation evidence.
  found: 'Exact stderr superseding the earlier absence of an error: Error connecting to Azure DevOps: Options "https://dev.azure.com/windstream/_apis": dial tcp: lookup dev.azure.com: no such host. It occurred after Wi-Fi was disabled.'
  implication: The failing HTTP method and URL match Azure DevOps SDK service/resource-area discovery, not the later GetConnectionData endpoint; source and history must confirm call order.

- timestamp: 2026-09-07T01:10:00Z
  checked: Current internal/ado/client.go NewClient implementation.
  found: NewClient creates workitemtracking, git, build, work, and release clients in sequence before constructing locationClient and invoking getConnectionData; every generated service constructor error returns immediately.
  implication: If the first service constructor performs network discovery, its offline error necessarily precedes and prevents the optional identity lookup.

- timestamp: 2026-09-07T01:10:00Z
  checked: Debug knowledge recall.
  found: Semantic recall returned no matching prior resolution; no knowledge-base file was present for keyword fallback.
  implication: Diagnose from source, pinned dependency code, and revision history rather than a prior pattern.

- timestamp: 2026-09-07T01:18:00Z
  checked: Pinned dependency and first constructor call.
  found: go.mod pins github.com/microsoft/azure-devops-go-api/azuredevops v1.0.0-b5. internal/ado/client.go calls workitemtracking.NewClient first. SDK workitemtracking/client.go:188-195 immediately calls connection.GetClientByResourceAreaId and propagates its error.
  implication: The first generated service-client constructor can fail before any location client or identity request is created.

- timestamp: 2026-09-07T01:18:00Z
  checked: Pinned SDK discovery implementation.
  found: azuredevops/connection.go:59-63 calls getResourceAreaInfo; lines 84-90 call base Client.GetResourceAreas on a cache miss. azuredevops/client.go:413-423 routes GetResourceAreas through Client.Send, whose empty location cache invokes getResourceLocationsFromServer; client.go:237-246 constructs http.MethodOptions to combineUrl(baseUrl, "_apis") and returns transport errors unchanged.
  implication: The exact `Options https://dev.azure.com/windstream/_apis: ... no such host` stderr originates in SDK service/resource-location discovery entered by workitemtracking.NewClient.

- timestamp: 2026-09-07T01:18:00Z
  checked: Pinned SDK identity path and application ordering.
  found: location.NewClient at location/client.go:48-53 performs no request; GetConnectionData at lines 85-104 later calls Client.Send with http.MethodGet. Application NewClient invokes it only after five generated service constructors succeed.
  implication: The observed OPTIONS failure occurs before, and prevents, the optional GetConnectionData call. It is not an identity-lookup failure.

- timestamp: 2026-09-07T01:18:00Z
  checked: Pre-Phase-1 baseline at commit 3aca6cb and immediate pre-gap baseline 7eb081a.
  found: Both baselines call the same five constructors in the same order before location.NewClient/GetConnectionData. Phase 1 changed client field interface types, cached AuthenticatedUser.Id, retained a bound identity provider, and changed a warning; it did not add, move, or make fatal workitemtracking.NewClient discovery. go.mod/go.sum remained unchanged.
  implication: Wi-Fi-off startup failure is baseline behavior, not a Phase 1 regression.

- timestamp: 2026-09-07T01:18:00Z
  checked: UAT method against verifier specification.
  found: Verification explicitly required a controlled identity endpoint that fails the eager GetConnectionData request and then recovers. Turning Wi-Fi off instead fails mandatory SDK OPTIONS discovery before identity lookup is reached.
  implication: The test setup cannot exercise the stated truth. The UAT result should be blocked pending a controlled endpoint/harness, not recorded as a product issue or pass.

- timestamp: 2026-09-07T01:23:00Z
  checked: Constructor provenance and dependency stability.
  found: git blame attributes workitemtracking.NewClient and its immediate fatal error return to the repository's initial commit 9c985df; location.NewClient/GetConnectionData also predate Phase 1. `git diff --exit-code 3aca6cb..HEAD -- go.mod go.sum` and the same check for cmd/adoBizInt/main.go both pass.
  implication: Phase 1 neither introduced the mandatory discovery call nor changed dependency/runtime error propagation at startup.

## Resolution

root_cause: The UAT disabled all network/DNS, so the first baseline-required generated SDK constructor, workitemtracking.NewClient, failed during Azure DevOps service/resource-location discovery. Its GetClientByResourceAreaId path issues OPTIONS to `{OrgURL}/_apis` before NewClient reaches the optional location.GetConnectionData identity GET. This is an invalid isolation setup, not a Phase 1 regression.
fix: No production change recommended. Reclassify UAT test 1 from issue/failed to blocked because the required controlled identity-failure/recovery harness is unavailable; rerun with service discovery succeeding while only GetConnectionData fails and then recovers. Do not mark skipped or pass because the identity behavior remains unexecuted.
verification: Source-proven call order in current application and pinned SDK; exact stderr method/URL matches SDK OPTIONS discovery; pre-Phase-1 baseline and git blame prove identical mandatory constructor ordering; dependency and main entrypoint unchanged.
files_changed: []
