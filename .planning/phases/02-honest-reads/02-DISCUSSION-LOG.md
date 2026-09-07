# Phase 2: Honest reads - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-07
**Phase:** 2-Honest reads
**Areas discussed:** Partial-result contract, Fatal auth behavior, Timeout policy, TUI failure display

---

## Partial-result contract

### Return shape when some projects succeed and others fail

| Option | Description | Selected |
|--------|-------------|----------|
| Uniform result envelope | One shared type carrying items, failures, completeness, reused by all eight reads. Single contract, written once for Phase 3 JSON. | ✓ |
| Per-read result types | Eight explicit structs each with items, failures, completeness. No generics, but eight near-identical definitions. | |
| Data plus aggregate error | Slice plus a rich error value. Smallest change, but callers that return early on error discard partial successes. | |

**User's choice:** Uniform result envelope
**Notes:** The user first selected "Per-read result types", then asked for the question to be re-asked because they were unsure they had answered correctly. It was re-presented with a comparison table covering signature shape and cost per option, and the answer changed to the uniform envelope. The later answer stands.

### Failure entry detail

| Option | Description | Selected |
|--------|-------------|----------|
| Project plus operation detail | Names project, failing operation, and underlying error; sub-request failures inside a succeeding project reported too. | ✓ |
| Project-level only | One entry per project. A partially-answering project looks identical to a fully-answering one. | |
| Project, operation, and resource | Adds the specific repository, definition, or batch range. Most precise, most plumbing. | |

**User's choice:** Project plus operation detail
**Notes:** Chosen with the batch skip at `workitems.go:158` and the three separate dashboard failure sites in view.

### Dashboard counts on partial failure

| Option | Description | Selected |
|--------|-------------|----------|
| Sum of successes, flagged incomplete | Return the sum of projects that answered, labelled understated. | ✓ |
| Zero the counts on any failure | No partial number can be mistaken for a total; loses usable data. | |
| Per-project count breakdown | Caller sees which projects contributed. Larger change to the dashboard shape. | |

**User's choice:** Sum of successes, flagged incomplete

### JSON tag naming

| Option | Description | Selected |
|--------|-------------|----------|
| snake_case | `repository_name`, `approval_status`, `is_draft`. Conventional for CLI JSON and jq-friendly. | ✓ |
| lowerCamelCase | `repositoryName`, `approvalStatus`. Matches the Azure DevOps REST API shape. | |
| Flat lowercase | `repositoryname`, `approvalstatus`. Simplest rule, hard to read. | |

**User's choice:** snake_case
**Notes:** Flagged as one-way — Phase 3 ships these as the CLI field contract.

---

## Fatal auth behavior

### Behavior when 401/403 arrives mid-fan-out

| Option | Description | Selected |
|--------|-------------|----------|
| Abort immediately, error only | Stop remaining projects, return the auth failure as an error, discard partial data. Recommended. | |
| Abort but keep partial data | Stop remaining projects, hand back what succeeded with the auth failure recorded. | |
| Continue, mark fatal in failures | Let every project run and report the auth failure among the failures. | ✓ |

**User's choice:** Continue, mark fatal in failures
**Notes:** Chosen against the recommendation. The cost was raised explicitly afterward: N-1 additional round trips per 30-second refresh against a rejected token, compounding CONCERNS.md MED3 which records no backoff, retry, or 429 handling anywhere.

### Distinguishing auth failure from ordinary failure

| Option | Description | Selected |
|--------|-------------|----------|
| Classified failure kind | Entries carry a kind field; callers branch on it rather than string-matching. Feeds the Phase 3 exit code. | ✓ |
| Dedicated auth flag on the envelope | Separate top-level flag. Hard to miss, but used by only one error class. | |
| No special marking | Caller inspects the wrapped error. Pushes classification onto every consumer. | |

**User's choice:** Classified failure kind

### Failure kind set

| Option | Description | Selected |
|--------|-------------|----------|
| auth, timeout, rate_limit, other | Covers the compounding failure classes CONCERNS.md names, kept small. | ✓ |
| auth and other only | Minimal set matching TRUST-02 today; timeout and throttling stay generic. | |
| Add not_found for missing projects | Adds a class for misconfigured projects. | |

**User's choice:** auth, timeout, rate_limit, other

### Bounding wasted calls after an auth failure

| Option | Description | Selected |
|--------|-------------|----------|
| Short-circuit remaining projects after first auth failure | Skip remaining projects for that invocation, still recording an entry per skipped project. Recommended. | |
| Always call every project | Call every project regardless, accepting N auth round trips per refresh. | ✓ |
| You decide | Let the planner choose a bounding mechanism from CONCERNS.md MED3. | |

**User's choice:** Always call every project
**Notes:** Short-circuiting was explicitly rejected after the cost was stated. Recorded in CONTEXT.md D-08 so the planner does not reintroduce it as an optimization.

---

## Timeout policy

### Where deadlines are enforced

| Option | Description | Selected |
|--------|-------------|----------|
| Both connection and per-fetch context | `connection.Timeout` bounds a single call; a context deadline bounds the whole fan-out. | ✓ |
| Connection timeout only | One line, but a fan-out over 40 repositories runs for 40 times the per-request budget. | |
| Per-fetch context deadline only | Bounds total work, leaves the HTTP client itself unbounded. | |

**User's choice:** Both connection and per-fetch context

### Source of the timeout values

| Option | Description | Selected |
|--------|-------------|----------|
| Hardcoded defaults, no config key | Sensible constants now; Phase 3 adds `--timeout` where the requirement lives. Recommended. | |
| Configurable via config.yaml | Operators tune both budgets. Adds config surface this phase's requirements do not ask for. | ✓ |
| Derive from PollInterval | Reuses an existing knob; couples two unrelated concerns. | |

**User's choice:** Configurable via config.yaml
**Notes:** The user initially selected "Hardcoded defaults, no config key", then reversed mid-area with "Actually make that a config value please". The reversal stands.

### Config key shape

| Option | Description | Selected |
|--------|-------------|----------|
| Two keys: request_timeout and fetch_timeout | One knob per enforcement point, `time.Duration` with defaults in `Load()` like `poll_interval`. | ✓ |
| One key, derive the fan-out budget | Fewer knobs; the derived budget is invisible to the operator. | |
| One key applied to both | Simplest to explain; a wide fan-out gets a single call's budget. | |

**User's choice:** Two keys: request_timeout and fetch_timeout
**Notes:** `internal/config/config.go` was read mid-question to confirm the `poll_interval` precedent — a `time.Duration` field with a `30 * time.Second` default applied in `Load()`.

### Behavior when fetch_timeout expires mid-fan-out

| Option | Description | Selected |
|--------|-------------|----------|
| Return partial with timeout failures | Projects that answered, plus a timeout-kind failure for each not reached, completeness false. | ✓ |
| Discard everything on expiry | Unambiguous, but throws away projects that answered inside the budget. | |
| Return partial marked complete | Listed only for contrast; reproduces the false-completeness this phase removes. | |

**User's choice:** Return partial with timeout failures
**Notes:** This question was dismissed on first presentation, then the user asked for the timeout source to become a config value. It was re-asked after the config-key question was settled.

---

## TUI failure display

### Partial success rendering

| Option | Description | Selected |
|--------|-------------|----------|
| Show rows plus a warning line | Render returned rows with a persistent warning naming failed projects and why. | ✓ |
| Block the table on any failure | Impossible to overlook, but one flaky project blocks three healthy ones. | |
| Rows plus transient status message | Least intrusive, easiest to miss across a 30-second refresh. | |

**User's choice:** Show rows plus a warning line

### Auth failure rendering

| Option | Description | Selected |
|--------|-------------|----------|
| Blocking auth banner | Distinct blocking banner replacing the table; auth failure is not partial data. | ✓ |
| Same warning line, auth named | Uniform treatment, but the most consequential failure looks like the least. | |
| Warning line plus global header indicator | Most persistent, more surface to build. | |

**User's choice:** Blocking auth banner
**Notes:** The asymmetry with the partial-success treatment is intentional.

### Refresh failure after a successful load

| Option | Description | Selected |
|--------|-------------|----------|
| Keep last good rows, marked stale | Nothing useful disappears on a blip; staleness is stated rather than implied. | ✓ |
| Clear the table on refresh failure | Nothing outdated is shown, but a flaky refresh wipes a working view every 30 seconds. | |
| Keep rows, no staleness marker | Least disruptive, but fresh and stale data become indistinguishable. | |

**User's choice:** Keep last good rows, marked stale

### Plumbing failure data into the views

| Option | Description | Selected |
|--------|-------------|----------|
| Messages carry the envelope | Each message carries the result envelope instead of a bare slice; one shape across six views. | ✓ |
| Add failure fields to existing messages | Smaller diff per message, but three fields repeated across every message type. | |
| Keep messages unchanged | Smallest change, and cannot represent partial success at all. | |

**User's choice:** Messages carry the envelope

---

## the agent's Discretion

The user delegated no decision. Every question received a direct answer, including
the two that were revisited. Open items the planner must decide are listed under
"the agent's Discretion" in CONTEXT.md — envelope type placement, the Go mechanism
for the envelope's items field, how the eight characterization tests are inverted,
default timeout values, whether the warning and stale rendering is shared, and
whether `DownloadAttachment`'s missing HTTP timeout is closed here or in Phase 5.

## Deferred Ideas

No scope creep was raised during discussion — every answer stayed inside the phase
boundary. The deferred list in CONTEXT.md is composed of rejected alternatives and
downstream-phase work surfaced by the questions themselves: bounding wasted auth
calls, rate-limit backoff and 429 handling, N+1 restructuring and `Top` caps, a
`not_found` failure kind, per-project dashboard count breakdown, the Phase 3
`--timeout` flag and exit-code mapping, `DownloadAttachment` hardening, and
promoting the Phase 1 fakes to an exported package.
