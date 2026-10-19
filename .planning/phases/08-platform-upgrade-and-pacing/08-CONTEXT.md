# Phase 8: Platform Upgrade and Pacing - Context

**Gathered:** 2026-10-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Move the tool onto a supported SDK and toolchain and make it slow down when Azure
DevOps signals throttling, without ever retrying a write. Requirements PLAT-02,
PLAT-03, PLAT-05.

- **PLAT-02:** one atomic change from `azuredevops v1.0.0-b5` to `azuredevops/v7`
  v7.1.0, Go 1.27, patched `golang.org/x/sys`, and a `govulncheck` gate in CI that
  fails on any reachable vulnerability. A test proves a v7 401 still classifies as
  `auth`.
- **PLAT-03:** a header-driven pacing transport, a bounded service-layer retry for
  reads only, and a PAT host rule bound to the configured organization.
- **PLAT-05:** a live smoke run against the real org, recorded before Phase 9
  starts.

Locked by ROADMAP/REQUIREMENTS and the v2.0 roadmap decisions (not re-discussed):
- The transport paces but never retries. Reads retry in the service layer. Writes
  are never retried automatically.
- PLAT-02 lands as one commit before PLAT-03. There is no window with both SDK
  versions in `go.mod`, because b5 and v7 `WrappedError` are different types and
  `classifyFailure` would silently stop matching 401s (PITFALLS Pitfall 13).
- No new module dependencies. Pacer, retry and host check use the stdlib only.
- CI's Go version matches `go.mod` (v1.0 D-16). No credentials in CI (v1.0 D-17).
- Update the "Go 1.25.4" constraint in `.planning/PROJECT.md`.
- `g-01-1` (offline startup, mandatory resource-area discovery) is NOT fixed by v7.
  Carry it forward.
- Smoke-run findings that overlap the deferred v1.0 live checks are recorded
  without reopening v1.0.

Not in this phase: request-shape caps and fan-out concurrency caps (PLAT-04,
Phase 9), the `core` sub-client (Phase 9), any new read or write verb.

**Sequencing note:** Phase 7 has 9/9 plans executed, but re-verification after
07-09 has not run. Phase 8 can be planned now. Do not start execution until
Phase 7 verifies, because the smoke run exercises the Phase 7 TUI guard path.

</domain>

<decisions>
## Implementation Decisions

### Throttle vs deadline
- **D-01:** If ADO's requested wait (`Retry-After`, or `X-RateLimit-Delay` on a
  200) is longer than the time left before the request's deadline, the pacer
  does not sleep. It fails fast with an error that classifies as
  `ado.FailureRateLimit`, not `FailureTimeout`. Waiting into a timeout hides the
  real cause and pushes agents to retry harder (PITFALLS Pitfall 9). When the
  wait fits, the pacer waits and then sends.
- **D-02:** A write that provably never left the process is reported as outcome
  `failed`, not `unknown`. That covers two cases: the pacer refused it under
  D-01, or the deadline or cancellation hit while it was held in the pacer. It
  also covers a write refused by the host rule (D-09). The pacer and host check
  return a typed "not sent" error. `guard.classify`
  (`internal/guard/guard.go:252`) must check for that error **before** its
  `parentErr` (`ctx.Err()`) branch. Today that branch turns every
  deadline-or-cancel into `unknown`. Under the existing rules (D-14/D-16),
  `failed` writes get no audit record. The exit code is the existing write-failure
  mapping in `writeExitCodeFor` (`cmd/adoBizInt/exit.go:54`): `failed` exits 1,
  or 2 if the error is auth. **Correction recorded:** during discussion this
  option was labelled "exit 4". Exit 4 is `exitNoOp` (`dispatch.go:28`), so it
  must not be used for this. The user's choice was failed rather than unknown,
  with no audit record. A guard test pins the new branch.
  — **Reversibility:** costly — the exit codes are a published one-way contract
  (dispatch.go:20-21), and agents will come to read a 1 on these paths as "never
  sent, safe to retry later"
- **D-03:** A read that fails as `rate_limit` (HTTP 429) is retried at most **2
  times**, so 3 attempts in total. The limit is a fixed constant, not config.
  Each retry waits as the pacer instructs and only goes out if D-01 allows it,
  meaning the wait fits the deadline. A D-01 fail-fast is not retried. Writes are
  never retried; a 429 on a write is `failed` under D-02.
- **D-04:** The pacer only obeys explicit server instructions: `Retry-After`,
  given as integer seconds or as an HTTP-date via `http.ParseTime`, and
  `X-RateLimit-Delay`. These count on any response status, including 200.
  `X-RateLimit-Remaining`, `X-RateLimit-Limit` and `X-RateLimit-Resource` are
  logged and never acted on. There is no proactive slowdown, no token bucket and
  no invented threshold.

### Throttle visibility
- **D-05:** When a CLI run was paced or retried and still succeeded, the details
  go to `adoBizInt.log`: resource, wait duration, retry count, and the
  X-RateLimit-* values. One summary line also goes to stderr in both human and
  `--json` modes, for example `throttled by ADO: 4 waits, 31s total, 1 retry`.
  stdout and the JSON envelope do not change, because the result is complete and
  correct.
- **D-06:** That stderr summary is a single line printed when the run ends, not
  one line per event. Multi-team fan-outs from Phase 9 onward will be throttled
  in bursts, and one line is what agents parse.
- **D-07:** The TUI shows nothing new. TUI pacing is logged to the file only,
  because stderr would garble the alt-screen. A read that ends as `rate_limit`
  already surfaces through the existing Phase 2 failure banner.
- **D-08:** A `rate_limit` failure explains its cause in the existing
  `Failure.Message` text: either "ADO asked for 45s, 12s left in budget" (the
  D-01 fail-fast) or "rate limited after 2 retries" (D-03 exhausted). `Kind`
  stays `rate_limit`. No new JSON field and no schema change; Phase 9 already
  changes the Failure contract for `team`.

### PAT host trust
- **D-09:** The PAT may be sent only to hosts bound to the configured
  organization `{org}`. The allowed set is:
  - the exact `org_url` host
  - `dev.azure.com` and `*.dev.azure.com` (vsrm, vssps and the rest), **only
    when the URL path starts with `/{org}/`**
  - `{org}.visualstudio.com` and `{org}.*.visualstudio.com`

  Every other host is refused. Refusing an unexpected but legitimate host is the
  safe way to fail, and the smoke run will surface any such case. The release
  client's `vsrm.dev.azure.com/{org}` must pass.
- **D-10:** The rule is enforced in two places. (a) **Startup:** after resource-area
  discovery, every `LocationUrl` is checked before any sub-client is built. A bad
  one stops the tool with exit 3 and a named config error that gives the
  offending host. (b) **Every request** in the transport, including each
  redirect hop: a refused request returns the typed "not sent" error (D-02)
  before the credential leaves the process. One rule function serves both.
- **D-11:** The pacer and host check live in a **new package** (e.g.
  `internal/transport`, name at discretion). It hands `internal/ado` a ready
  `*http.Client`. `internal/ado` never imports `net/http`, so `TestNoRawHTTPInADO`
  (`internal/ado/attachments_test.go:418`) stays byte-identical. A new
  structural test asserts that every SDK sub-client is built with the injected
  client: no `GetClientByUrl`, no typed `NewClient(ctx, connection)` and no
  default transport.
- **D-12:** `config.Validate()` requires `org_url` to be `https` and to match a
  cloud shape: `https://dev.azure.com/{org}` or `https://{org}.visualstudio.com`.
  Anything else is exit 3 with a named `org_url` config error. That makes the
  `{org}` the host rule binds to unambiguous, and prevents sending the PAT over
  cleartext. On-prem Azure DevOps Server is no longer supported; nothing
  configures it today. — **Reversibility:** reversible — relaxing a validator
  later breaks no caller

### Live smoke run (PLAT-05)
- **D-13:** Every one of the six write verbs gets a dry run. In addition, real
  writes go against **one scratch work item** the user designates:
  - a CLI `workitems set-state --confirm`, which closes the deferred v1.0 check
    "set-state --confirm with the project in the PATCH route"
  - a TUI Task close with effort, which exercises the two-PATCH path flagged in
    07-REVIEW IN-01 (a process rule that requires RemainingWork on Closed, or one
    that rewrites effort on close)

  PR approve/complete, release approve/reject and pipeline queue stay dry-run.
- **D-14:** The smoke run is a **checked-in script** (e.g. `scripts/smoke.sh`)
  plus a short **manual TUI checklist** beside it. The script runs every CLI read
  and dry run against the configured org, checks exit codes and `complete`, and
  prints a pass/fail table. The user runs it and pastes the output into the
  record. It never runs in CI (D-17), and it can be rerun on later SDK or
  toolchain bumps. The script must never print the PAT.
- **D-15:** Results are recorded in
  `.planning/phases/08-platform-upgrade-and-pacing/08-SMOKE.md`. Phase 9 is
  **blocked only by regressions** that the v7 upgrade, the pacer or the host rule
  caused. Examples: API-version response drift (5.1 to 7.1-preview), a legitimate
  host refused, a 401 misclassified, a TUI guard path broken on v7. Pre-existing
  v1.0 issues the run surfaces (BL-02 items, `g-01-1`) are recorded in
  `08-SMOKE.md` and `.planning/BACKLOG.md` and do not block.
- **D-16:** "One live read per ADO sub-client" (SC5) is **interpreted** as one
  read per sub-client that has a real call site: `workitemtracking`, `git`,
  `build`, `release`, and `location` (identity / `GetConnectionData`). The
  dormant `work` client's construction is still covered by v7 discovery, the
  host rule and the startup check. It gets no read until Phase 9 adds its first
  verb, and `08-SMOKE.md` names that gap explicitly so the verifier does not
  count it as a miss. The `core` client is not built in Phase 8, because a
  client with no caller is speculative generality.

### Claude's Discretion
- Toolchain pin. The `go` directive goes at the 1.27 patch level that makes
  `govulncheck` clean at implementation time; research found go1.27.2 clean and
  local go1.27.1 still has 8 stdlib findings. CI uses `go-version-file: go.mod`,
  which keeps D-16 automatically. `golang.org/x/sys` goes to v0.47.0, the newest
  that still declares go 1.25. `govulncheck` is pinned and run with
  `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...`, never added to
  `go.mod`. Handle the `toolchain` line as needed.
- Plan split and commit granularity, within the locked PLAT-02-then-PLAT-03 order
  and the single atomic import swap.
- Package name and API of the transport package. Pacer state (`notBefore`) is
  process-wide or per resource; the research default is a single mutex-guarded
  `notBefore`. Also any cap on a single honoured `Retry-After` that fits the
  deadline.
- How the CLI gathers pacer and retry counters for the D-05/D-06 summary line.
  The dispatch layer reads them after the run; the transport never writes stderr.
- How a per-request host refusal on a **read** classifies. It must not classify
  as `auth` or `timeout`, and its message must name the refused host.
- Whether the pacer deletes the `X-TFS-Session` response header. The research
  says to delete it because of the latent `azuredevops.SessionId` global race,
  which becomes real in Phase 9 fan-out. Decide during research and planning.
- Whether the TUI checklist also covers one attachment download and the Phase 1
  eager-identity recovery path. These are BL-02's other deferred live checks.
  Attachment download goes through the injected client and so exercises the host
  rule.
- Updating the b5 line references in the `internal/ado/result.go` doc comments
  (`client.go:366, :389, :403, :411`) to their v7 equivalents.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements and scope
- `.planning/REQUIREMENTS.md` §Platform: PLAT-02, PLAT-03, PLAT-05 (PLAT-04 is Phase 9)
- `.planning/ROADMAP.md` §Phase 8: success criteria 1-5, research focus, notes (PROJECT.md constraint update, `g-01-1` carry-forward, smoke overlap with v1.0 live checks)
- `.planning/PROJECT.md` §Constraints: the "Go 1.25.4" line to update; the PAT-never-to-caller-supplied-host rule; the no-new-dependencies rule

### Research (v2.0, 2026-10-13)
- `.planning/research/STACK.md`: SDK v7.1.0 facts, the govulncheck finding (the toolchain, not the SDK, fails the gate), x/sys pin, §Integration Points 1 (pacer design, explicit sub-client construction via `NewClientWithOptions` + `WithHTTPClient`, `hc.Timeout` must be set explicitly, URL normalisation), §Alternatives / What NOT to Use
- `.planning/research/PITFALLS.md`: Pitfall 9 (throttling shows up as 200-with-delay and timeouts; SDK hides headers), Pitfall 13 (clean compile hides API-version jumps; atomic swap; v7 401 test; live smoke is the gate)
- `.planning/research/ARCHITECTURE.md` §Pattern 6 and the build-order table: transport injection coupling to v7. **Superseded on location:** it places the transport at `internal/ado/transport.go`; D-11 moves it to a new package
- `.planning/research/SUMMARY.md` §Phase 2 (Platform): delivery list for this phase. **Correction:** STACK.md says "a 429 on a write maps to exit 4". That is wrong for this codebase; see D-02 (failed write = exit 1)

### Prior decisions and findings
- `.planning/phases/07-guarded-tui-writes/07-CONTEXT.md`: guard shape, D-08 (unknown outcome display), D-09 (two-write resolve order)
- `.planning/phases/07-guarded-tui-writes/07-REVIEW.md` §IN-01: Task close as two PATCHes, to confirm live in PLAT-05 (D-13)
- `.planning/BACKLOG.md` §BL-02: deferred live checks (set-state --confirm with the project in the PATCH route, TUI attachment download, eager-identity recovery); the place to record non-blocking smoke findings (D-15)
- `.planning/milestones/v1.0-phases/04-guarded-writes/04-CONTEXT.md`: D-14/D-16 (which outcomes are audited), D-20/D-22 (timeout and interrupt are unknown), P-08/P-12/P-13 classification rules that D-02 extends
- `.planning/codebase/CONVENTIONS.md`: single-use inline, rule of three, files under 300 lines

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/ado/client.go` `newConnection` / `NewClient`: the constructor to rewrite. It builds five typed sub-clients via `X.NewClient(ctx, connection)` plus `location.NewClient`. Its doc comment explains that `connection.Timeout` must be set before any client is built; `WithHTTPClient` bypasses `connection.Timeout`, so the timeout must move onto the injected `http.Client` (`cfg.RequestTimeout`).
- `internal/ado/result.go` `classifyFailure` (l.170): already maps 429 to `FailureRateLimit`. It checks `context.DeadlineExceeded` before status, so the D-01 fail-fast error must not wrap `DeadlineExceeded`, or it must be checked first. `wrappedErrorOf` handles both value and pointer `WrappedError`; the behaviour is unchanged in v7.
- `internal/guard/guard.go` `classify` (l.252): the parentErr-first rule that D-02 amends. `appendAuditRecord` runs only for executed/unknown (l.207).
- `cmd/adoBizInt/exit.go` `writeExitCodeFor`: `failed` maps to 1 (2 for auth), anything else not executed maps to 5. `cmd/adoBizInt/dispatch.go:22-30`: the exit-code contract (4 = no-op).
- `config.Config.RequestTimeout` / `FetchTimeout` and `config.Validate()` (`internal/config/config.go:171`): where D-12 adds the org_url check.
- `stderrWriter` in `cmd/adoBizInt/dispatch.go`: the existing seam the D-05 summary line writes through.
- `testing/synctest` (Go 1.25+ GA): deterministic pacer tests with a fake base `RoundTripper`, no real sleeps.

### Established Patterns
- Structural guarantees are asserted with `go/ast`, not grep (`TestNoRawHTTPInADO`, `TestNoUIFileCallsADOMutation`). D-11's new structural test follows this pattern.
- Narrow interfaces with fail-fast fakes in `internal/ado`. Tests run against SDK-typed fakes, so passing tests prove nothing about wire behaviour after the API-version jump. That is why PLAT-05 is the gate.
- stdout carries parseable output only; diagnostics go to stderr or the log (OUT-03). The transport package must never write to stdout or stderr.
- Classification is structural, never by error text (D-06).

### Integration Points
- `go.mod`: currently `go 1.25.4`, `azuredevops v1.0.0-b5`. 24 import sites across 8 SDK sub-packages; the swap is a mechanical rewrite.
- `.github/workflows/ci.yml`: hardcodes `go-version: '1.25.4'`. Switch it to `go-version-file: go.mod` and add the govulncheck step.
- Release client discovery resolves to `vsrm.dev.azure.com/{org}`, a different host from `org_url`, which the D-09 rule must allow.
- `DownloadAttachment` (`internal/ado/attachments.go:96`) goes through the SDK `workitemtracking` client, so it inherits the injected client and the host rule.

</code_context>

<specifics>
## Specific Ideas

- Example stderr summary (D-06): `throttled by ADO: 4 waits, 31s total, 1 retry`
- Example rate_limit messages (D-08): `rate limited: ADO asked for 45s, 12s left in budget` / `rate limited after 2 retries`
- `.planning/PROJECT.md` §Current State describes exit codes 0-5 as "... usage/config error, write failure and unknown outcome", which mislabels 4 (it is no-op). Correct it in the same edit that updates the Go constraint.

</specifics>

<deferred>
## Deferred Ideas

- A machine-readable `retry_after_s` / throttle field in the JSON envelope or `Failure`. Considered and not chosen (D-05, D-08). Revisit if agents need to schedule their own retries.
- A TUI status-line hint while a fetch is paced (D-07). Not chosen.
- Proactive pacing on `X-RateLimit-Remaining` (D-04). Not chosen.
- Configurable read-retry count (D-03). Not chosen.
- On-prem Azure DevOps Server support, dropped by D-12.
- Executing PR, release and pipeline writes live (D-13). Not chosen, because those writes are not cleanly reversible.

</deferred>

---

*Phase: 08-platform-upgrade-and-pacing*
*Context gathered: 2026-10-16*
