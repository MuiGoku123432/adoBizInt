# Phase 2: Honest reads - Context

**Gathered:** 2026-09-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Make the eight existing aggregate reads in `internal/ado` truthful: a caller can
distinguish an empty result from a failed one, auth failures are classified
rather than rendered as emptiness, every call carries a deadline, exported
structs carry stable JSON field names, and the TUI shows failures instead of a
silently empty table.

**In scope:** the return contract of the eight aggregate reads
(`GetWorkItems`, `GetPullRequests`, `GetPipelineRuns`, `GetPipelineDefinitions`,
`GetReleases`, `GetPendingApprovals`, `GetBranches`, `GetDashboardCounts`),
failure classification, timeout enforcement and its config keys, `json:` tags on
exported `ado` types, and the TUI call sites and message types that consume
these reads.

**Not in scope:** CLI subcommands, `--json` output, exit codes (Phase 3);
dry-run, audit and write guards (Phase 4); attachment, path-containment, WIQL
and nil-guard hardening (Phase 5); rate-limit backoff and N+1 restructuring
(deferred, CONCERNS.md MED3 and PLAT-03).

</domain>

<decisions>
## Implementation Decisions

### Partial-result contract

- **D-01:** All eight aggregate reads return a **uniform result envelope**
  carrying the items, a failure list, and an explicit completeness flag. One
  shared shape reused across all eight rather than eight near-identical structs,
  so Phase 3 writes its JSON handling once. This introduces the first generic
  type in a codebase that has none; it is justified by the same rule-5 carve-out
  CONVENTIONS.md granted the Phase 1 interfaces, and by eight call sites being
  well past the rule of three.
  — **Reversibility:** costly — the envelope is the return type of every
  aggregate read and is consumed by all six view packages; changing its shape
  later touches every read and every consumer.
- **D-02:** Each failure entry names the **project and the operation** that
  failed (for example WIQL query, PR fetch, build count, per-repo branch fetch)
  plus the underlying error. Sub-request failures inside an otherwise-succeeding
  project are reported too, so a project that answered partially is never marked
  complete. This covers the batch skip at `workitems.go:158` and the three
  independent dashboard failures at `dashboard.go:102`, `:116`, `:127`.
- **D-03:** `GetDashboardCounts` returns the **sum of the projects that
  answered**, with failures listed and completeness false. A partial total is
  usable as long as it is labelled understated; zeroing it would discard good
  data from healthy projects.
- **D-04:** `json:` tags on the exported `ado` structs use **snake_case**
  (`repository_name`, `approval_status`, `is_draft`, `created_on`). This becomes
  the CLI field contract Phase 3 serializes.
  — **Reversibility:** one-way — once Phase 3 ships these names as CLI output,
  renaming them breaks every agent and script parsing that JSON.

### Fatal auth behavior

- **D-05:** A `401` or `403` **does not abort the remaining projects**. The read
  continues through every configured project and records the auth failure in the
  failure list. Chosen against the recommendation to abort early.
- **D-06:** Failure entries carry a **classified kind**, so a caller branches on
  a field rather than string-matching an error message. This is what Phase 3's
  auth-failure exit code will key on.
- **D-07:** The defined kinds are **`auth`, `timeout`, `rate_limit`, `other`**.
  These are the classes CONCERNS.md identifies as compounding: an auth outage, a
  hang, and throttling each currently render as emptiness.
- **D-08:** **No short-circuit and no call-bounding mechanism in this phase.**
  Every project is called on every refresh even after an auth failure is known.
  The user explicitly rejected short-circuiting after the cost was raised.

  **Recorded cost, for the planner to surface rather than silently absorb:** on
  a four-project config with a rejected token, each 30-second refresh spends
  four failing round trips instead of one. CONCERNS.md MED3 notes there is no
  backoff, retry, or rate-limit handling anywhere, so this adds load in exactly
  the scenario most likely to already be throttled. TRUST-02 is still satisfied
  because the failure is classified and never renders as an empty list — the
  cost is efficiency, not correctness.

- **D-06a (ADDED 2026-10-12, gap closure — narrow exception to D-06):** An empty
  repository on the branch-stats call is recognised by **HTTP 400 AND a message
  that begins with the ADO error code `VS403403:`**, and is suppressed rather than
  recorded as a failure. Live capture on 2026-10-12 against the Windstream org
  (repository "Pricing Engine") returned status 400, `typeKey: ArgumentException`,
  `errorCode: 0`, message `VS403403: Cannot find any branches for the Pricing
  Engine repository.` No structural field distinguishes it: `ArgumentException`
  is generic and `errorCode` is 0. Matching the leading ADO error identifier is a
  documented code, not free text; anything else on that call — any other 400, a
  401/403 (auth), a timeout — is still recorded. This replaces assumption A4
  (empty repo yields 404), which the live evidence falsified. Chosen by the user.
  — **Reversibility:** reversible — internal to `internal/ado/branches.go`.
- **D-06b (ADDED 2026-10-12, found during 02-07 live check):** A **disabled
  repository** is skipped on the branch-stats call when the answer is **HTTP 404
  AND the structured `typeKey` is `GitRepositoryNotFoundException`** (no message
  matching). Narrowing to D-06a surfaced 130 failures, all disabled repositories
  (2 Kinetic, 128 Process) that `GetRepositories` still lists; the old blanket
  404 rule had been hiding them. The 2020 SDK's `GitRepository` has no
  `isDisabled` field; after the v2 SDK upgrade (PLAT-02) prefer skipping by that
  field before the call. Chosen by the user.
  — **Reversibility:** reversible — internal to `internal/ado/branches.go`.

### Timeout policy

- **D-09:** Deadlines are enforced at **two points**: `connection.Timeout` bounds
  any single HTTP call, and a context deadline created at each fetch bounds the
  whole fan-out. The second matters because the N+1 reads
  (`pipelines.go:137`, `releases.go:103`, `branches.go:57`) can otherwise run for
  many multiples of a per-request budget.
- **D-10:** Both budgets are **config values**, not hardcoded constants. The user
  reversed an initial hardcoded-defaults answer to reach this.
- **D-11:** Two keys: **`request_timeout`** (per-call) and **`fetch_timeout`**
  (whole fan-out). Follow the existing `poll_interval` precedent exactly —
  `time.Duration` fields on `config.Config` with defaults applied in
  `config.Load()`, so `30s` and `2m` parse natively from YAML. One knob per
  enforcement point; no derived-and-invisible budget.
- **D-12:** When `fetch_timeout` expires mid-fan-out, the read returns the
  projects that already answered, with a `timeout`-kind failure for each project
  not reached and completeness false. Consistent with D-01 and D-07.

### TUI failure display

- **D-13:** On **partial success**, the view renders the rows that came back plus
  a persistent warning line naming the failed projects and why. The user keeps
  working with real data and can see what is missing. Not a transient status-line
  message — that channel is already used for action feedback and is easy to miss
  across a 30-second refresh.
- **D-14:** On **auth failure**, the view shows a distinct blocking banner
  stating the token was rejected, replacing the table. Auth failure is not
  partial data, and a warning line above a nearly-empty table invites the user to
  keep trusting it.
- **D-15:** When a **refresh fails after an earlier successful load**, the last
  good rows stay on screen and are marked stale, with the failure named. A single
  transient blip must not wipe a working view every 30 seconds, and staleness is
  stated rather than implied. Applies to the three polling views (`prs`,
  `pipelines`, `releases`).
- **D-16:** The view messages **carry the result envelope** in place of their
  bare slices (`PRsMsg`, `WorkItemsMsg`, `PipelinesMsg`, `DefinitionsMsg`,
  `ReleasesMsg`, `ApprovalsMsg`, `BranchesMsg`, `CountsMsg`), so failure and
  completeness travel with the payload in one consistent shape rather than as
  three fields repeated per message type.

### the agent's Discretion

The user delegated no decision — every question was answered directly. The
following are open and the planner should decide or raise them:

- Where the envelope type lives (a new file in `internal/ado`, or beside its
  first consumer). CONVENTIONS.md sets a 300-line file standard that
  `internal/ado` already violates at 2,015 lines.
- Whether the envelope's items field is generic or the envelope is embedded in
  per-read types that keep concrete slices. D-01 fixes the contract shape, not
  the Go mechanism.
- How the eight Phase 1 characterization tests (`Test*SwallowsProjectFailure`)
  are inverted — rewritten in place, or replaced by new tests that assert the
  propagating behavior. The anchored gate pattern
  `^=== RUN   TestGet.*SwallowsProjectFailure$` from Phase 1 exists specifically
  so all eight are found together.
- Exact default values for `request_timeout` and `fetch_timeout`.
- Whether the warning line and stale marker are shared rendering helpers or
  per-view code. Six views need the same treatment, which is past the rule of
  three.
- Whether `DownloadAttachment`'s `http.DefaultClient` (`workitems.go:528`) gets a
  timeout here under TRUST-04, or waits for Phase 5 where the rest of that
  method's hardening lives.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase-defining analysis

- `.planning/codebase/CONCERNS.md` — **Highest-value read for this phase.**
  Finding C1 is the origin of this phase and contains the verified table of all
  eight swallow sites with exact file:line and the sole `return x, nil` for each.
  Finding H2 documents the 22 `context.Background()` call sites, the unset
  `connection.Timeout`, and that the SDK supports it. MED3 records the N+1
  fan-outs and the total absence of retry, backoff, and 429 handling — directly
  relevant to the cost recorded under D-08. MED4 names the nil-dereference sites
  deferred to Phase 5.
- `.planning/codebase/INTEGRATIONS.md` §"Error Handling Patterns" — the three
  inconsistent error patterns (log-and-continue, propagate, wrap with `%w`) and
  the full list of log-and-continue sites this phase inverts. §"Complete Method
  Inventory" gives the SDK calls each read issues.
- `.planning/codebase/ARCHITECTURE.md` §"Suitability for a non-TUI CLI surface"
  — records that none of the 12 exported `ado` structs carry `json:` tags and
  that tags are additive and non-breaking (OUT-02). §"Error Handling" describes
  the `err` banner plus `statusMsg`/`statusErr` split the TUI decisions build on.
- `.planning/codebase/CONVENTIONS.md` — rule 5 (no interface or abstraction
  purely for testability unless that is the task), rule 6 (justify dependencies),
  the rule of three, and the 300-line file standard. D-01's shared envelope and
  the shared-rendering question under discretion are both governed by these.
- `.planning/codebase/TESTING.md` — the test-file naming convention and the
  characterization-test style Phase 1 established, which Phase 2 inverts.

### Prior phase decisions

- `.planning/phases/01-test-seam-for-internal-ado/01-CONTEXT.md` — D-14 through
  D-16 explain why the eight `Test*SwallowsProjectFailure` characterization tests
  exist and state outright that Phase 2 inverts them. D-05 through D-09 define
  the unexported `-API` interfaces and the struct-literal injection seam these
  tests use. The deferred list records that `connection.Timeout` was left to
  TRUST-04 in this phase.

### Requirements and scope

- `.planning/REQUIREMENTS.md` §"Read Trustworthiness" — TRUST-01 through
  TRUST-04. §"Structured Output" OUT-02. §"Traceability" maps all five to
  Phase 2.
- `.planning/ROADMAP.md` §"Phase 2" — goal, the six success criteria, and the
  four provisional plan titles (02-01 through 02-04). §"Phase 3" for the
  downstream exit-code contract D-06 and D-07 feed.
- `.planning/PROJECT.md` §"Core Value" and §"Key Decisions" — the locked decision
  that read error propagation is fixed before any read verb ships, and the
  constraint that the no-argument TUI must not change behavior.

### Source files this phase modifies

- `internal/ado/workitems.go` — `GetWorkItems` (:56), swallow site (:75-78),
  sole return (:83), batch skip (:155-158), `getProjectWorkItems` (:86).
- `internal/ado/pullrequests.go` — `GetPullRequests` (:34), swallow (:52-55),
  return (:65).
- `internal/ado/pipelines.go` — `GetPipelineRuns` (:91), swallow (:112-115),
  return (:227); `GetPipelineDefinitions` (:231), swallow (:251-254),
  return (:315).
- `internal/ado/releases.go` — `GetReleases` (:56), swallow (:71-74),
  return (:197); `GetPendingApprovals` (:201), swallow (:218-221),
  return (:268).
- `internal/ado/branches.go` — `GetBranches` (:32), swallow (:49-52),
  return (:126), per-repo swallow (:77-78).
- `internal/ado/dashboard.go` — `GetDashboardCounts` (:23), swallow (:49-52),
  return (:70); `getProjectCounts` (:74) with three in-place swallows at
  :102, :116, :127 and its `counts, nil` return at :141.
- `internal/ado/client.go` — `NewClient` (:34) and the `NewPatConnection` call
  (:35) where `connection.Timeout` is set for D-09.
- `internal/config/config.go` — `Config` struct (:38) and `Load()` for the
  `request_timeout` and `fetch_timeout` keys; `PollInterval` (:46) and its
  `30 * time.Second` default are the precedent to follow.
- `internal/ui/dashboard/dashboard.go` (`CountsMsg` :17),
  `internal/ui/workitems/workitems.go` (`WorkItemsMsg` :44),
  `internal/ui/prs/prs.go` (`PRsMsg` :33),
  `internal/ui/pipelines/pipelines.go` (`PipelinesMsg` :34, `DefinitionsMsg` :40),
  `internal/ui/releases/releases.go` (`ReleasesMsg` :37, `ApprovalsMsg` :43),
  `internal/ui/branches/branches.go` (`BranchesMsg` :32) — the message types
  D-16 changes and the reducers that render D-13 through D-15.

### Existing tests this phase inverts

- `internal/ado/workitems_test.go`, `pullrequests_test.go`, `pipelines_test.go`,
  `releases_test.go`, `branches_test.go`, `dashboard_test.go` — the eight
  `Test*SwallowsProjectFailure` characterization tests, plus
  `internal/ado/fakes_test.go` for the four fakes they drive.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **The Phase 1 seam is complete and directly usable.** Four unexported `-API`
  interfaces with hand-written fakes in `internal/ado/fakes_test.go`, and tests
  construct `&Client{workitemClient: fake, projects: []string{"P"}}` directly.
  Every failure path this phase creates is testable with no new infrastructure.
- **Eight characterization tests already pin the current swallow behavior**, each
  written to be inverted by this phase and findable with one anchored grep. The
  Phase 2 change lands as a visible test diff rather than an unverifiable
  rewrite.
- **`config.PollInterval` is the exact precedent for D-11** — a `time.Duration`
  field with a `yaml` tag and a default applied in `Load()`.
- **The SDK already supports the timeout D-09 needs.** `azuredevops.Connection`
  declares `Timeout *time.Duration` and applies it to the HTTP client; the code
  simply never sets it.
- **Every `ado` method is already context-first**, so no signature needs to change
  to thread deadlines — only the call sites that pass `context.Background()`.

### Established Patterns

- **`log := logging.Logger()` then log-and-return** at the top of each method.
  Failure entries should carry the same information already being logged, rather
  than introducing a second vocabulary.
- **Errors travel inside the result message, not by short-circuit.** Every UI
  message already carries an `Err error` beside its payload, and the reducer
  branches on it into `err` (banner) or `statusMsg`/`statusErr` (status line).
  D-13 through D-15 extend this split rather than replacing it.
- **Zero interfaces existed before Phase 1; four unexported ones exist now.**
  D-01's envelope is the next abstraction and should be justified in the plan on
  the same rule-5 grounds.
- **`internal/ado` is 2,015 lines across 7 files against a 300-line standard.**
  `workitems.go` alone is 551. Where the envelope and failure types live is a
  real decision, not a formality.
- **The SDK-to-domain conversion pattern** is an explicit nil-checked
  field-by-field copy. Adding `json:` tags does not disturb it.

### Integration Points

- **The eight aggregate read signatures** are the primary seam. Changing them
  forces every consumer to acknowledge failure, which is the point.
- **`getProjectCounts` (`dashboard.go:74`) is structurally different** from the
  other seven: it swallows three separate failures in place and its outer
  `continue` is unreachable until the helper propagates. Phase 1 recorded this
  explicitly — dashboard inversion is a helper-level change, not an
  outer-loop-only edit.
- **All six view packages hold `client *ado.Client` concretely** and each
  declares its own message types. There is no shared message vocabulary, so D-16
  touches eight message types across six packages independently.
- **`cmd/adoBizInt/main.go`** is where a fetch-level context would originate for
  any non-TUI caller; Phase 3 attaches `--timeout` here. Phase 2 only needs the
  TUI-side deadlines and the connection timeout.
- **`connection.Timeout` must be set before the sub-clients are constructed** in
  `NewClient`, since each construction performs a network call of its own.

</code_context>

<specifics>
## Specific Ideas

- The framing that drove the partial-result questions: *"three of four projects
  answered" must be distinguishable from "four projects, no work."* Every
  decision in the first area traces back to that sentence, and it is worth
  carrying into the plan's acceptance criteria verbatim.
- D-08 is a deliberate, informed acceptance of wasted calls. The plan should not
  quietly add a short-circuit because it looks like an obvious optimization — the
  user rejected it explicitly after the cost was stated.
- D-14's blocking auth banner and D-13's non-blocking warning line are
  intentionally asymmetric. The asymmetry is the feature: a user must not be able
  to read a rejected token as merely incomplete data.
- The dashboard's summed counts are the one place where a partial result is a
  single number with no visible seam. D-03 keeps the number but labels it, on the
  reasoning that a labelled understatement beats both a silent understatement and
  a discarded truth.

</specifics>

<deferred>
## Deferred Ideas

- **Bounding wasted calls after a known auth failure** — short-circuiting the
  remaining projects was offered and rejected under D-08. If refresh cost against
  a rejected token becomes a real problem, this is the first lever. Related to
  PLAT-03 (rate limit backoff and fan-out caps).
- **Rate-limit backoff and 429 handling** — the `rate_limit` failure kind (D-07)
  makes throttling *visible* in this phase; it does not make it *handled*.
  CONCERNS.md MED3, REQUIREMENTS.md PLAT-03.
- **N+1 fan-out restructuring and `Top` caps** — including the unbounded
  `GetBuilds` in the dashboard. These make timeouts more likely to fire, but
  fixing them is not TRUST-04. CONCERNS.md MED3.
- **`not_found` as a failure kind** for missing or misconfigured projects — the
  option was offered and the smaller four-kind set chosen. Add it only if
  misconfigured projects prove hard to diagnose as `other`.
- **Per-project count breakdown on the dashboard** — offered under D-03 and not
  chosen. Would change the dashboard's shape and its TUI consumer; revisit if a
  CLI consumer in Phase 3 actually needs per-project counts.
- **`--timeout` flag and interrupt handling** — CLI-04, Phase 3. This phase
  establishes the config-driven budgets those will override.
- **Exit code mapping for partial, auth, and outcome-unknown** — OUT-04, Phase 3.
  D-06 and D-07 exist to make that mapping mechanical.
- **`DownloadAttachment` hardening** — the host allow-list, path containment, and
  size cap are HARD-01 through HARD-03 in Phase 5. Only its missing HTTP timeout
  is arguably in scope here, and that call is left to the planner under
  the agent's discretion.
- **Promoting the Phase 1 fakes to an exported package** — still deferred to
  Phase 4, unchanged by this phase.

</deferred>

---

*Phase: 2-Honest reads*
*Context gathered: 2026-09-07*
