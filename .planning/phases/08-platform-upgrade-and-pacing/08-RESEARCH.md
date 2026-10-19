# Phase 8: Platform Upgrade and Pacing - Research

**Researched:** 2026-10-19
**Domain:** Go SDK migration (azure-devops-go-api b5 to v7), Go toolchain and vuln gate, stdlib `http.RoundTripper` pacing and credential host binding, live smoke run
**Confidence:** HIGH for SDK, toolchain, govulncheck and transport mechanics (verified this session by compiling, testing and probing the real v7 SDK in a scratch copy of the repo). MEDIUM for Azure DevOps service behavior (official docs plus one live anonymous probe). LOW items are marked inline and listed in the Assumptions Log.

## Summary

The PLAT-02 swap is mechanical and was re-verified today on a scratch copy of HEAD (`da3bdef`): a path rewrite across **26 files / 56 import lines / 9 SDK packages**, `go mod tidy`, `go 1.27.2`, `golang.org/x/sys v0.47.0`. `go build`, `go vet`, `go test -race ./...` and `gofmt -l` all pass with zero code changes. `govulncheck@v1.9.0` reports **"No vulnerabilities found."** with the directive at `go 1.27.2`, and **8 stdlib findings** (GO-2026-6603..6617, net/http, fixed in go1.27.2) with the directive at `go 1.27.0` on the local go1.27.1. The directive must therefore be the patch level, `go 1.27.2`, not `1.27.0`: it is what makes both local `GOTOOLCHAIN=auto` and CI `go-version-file` run the clean toolchain.

PLAT-03 was prototyped against the real v7 SDK with a fake base `RoundTripper`. Verified: `azuredevops.NewClientWithOptions(conn, url, azuredevops.WithHTTPClient(hc))` routes every request (including the SDK's `OPTIONS {base}/_apis` location discovery) through the injected transport; a real v7 401 JSON body arrives as a bare-value `WrappedError` and classifies `auth`; `Retry-After` on an HTTP 200 delays the next request by exactly the instructed time under `testing/synctest`; `http.Client.Timeout` is visible to the pacer as `req.Context().Deadline()`, so D-01's "wait does not fit" check works with only `request_timeout` set; a refusal returned from `RoundTrip` arrives at `internal/ado` wrapped in `*url.Error`, `errors.Is` reaches it, and today's `classifyFailure` labels it `other` (a new branch is required for D-01's `rate_limit`). Redirect hops are distinguishable (`req.Response != nil`), and Go forwards the `Authorization` header on redirects to the same host and to subdomains, so `dev.azure.com/acme` redirecting to `dev.azure.com/otherorg` would carry the PAT without D-10's per-hop check.

Two findings correct the v2.0 research. (1) The `azuredevops.SessionId` race is **not real**: the SDK looks up the non-canonical key `"X-TFS-Session"` in a header map that Go always canonicalizes to `"X-Tfs-Session"`, so the global is never written after init. Proven with `-race` probes. Do not add the header deletion. (2) A **fully** rejected PAT fails inside `ado.NewClient` during resource-area discovery and exits **3** today (v1.0 02-01 deviation 2); SC2 ("still exits with the auth-failure code") holds on the read and write paths, which is where a v7 401 must keep mapping to exit 2.

**Primary recommendation:** Land PLAT-02 as one commit (import swap, `go 1.27.2`, x/sys v0.47.0, `go mod tidy`, CI `go-version-file` plus a pinned `go run ...govulncheck@v1.9.0` step, and a real-SDK v7 401 test). Then build `internal/transport` (host rule plus header-driven pacer, stdlib only), rewrite `ado.NewClient` to construct every sub-client explicitly on the injected client after a startup `LocationUrl` check, add a read-only 429 retry decorator at the narrow-interface seam, and teach `classifyFailure` and `guard.classify` the typed not-sent error. Gate Phase 9 on the scripted smoke run.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

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

#### Throttle vs deadline
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

#### Throttle visibility
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

#### PAT host trust
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

#### Live smoke run (PLAT-05)
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

### Deferred Ideas (OUT OF SCOPE)
- A machine-readable `retry_after_s` / throttle field in the JSON envelope or `Failure`. Considered and not chosen (D-05, D-08). Revisit if agents need to schedule their own retries.
- A TUI status-line hint while a fetch is paced (D-07). Not chosen.
- Proactive pacing on `X-RateLimit-Remaining` (D-04). Not chosen.
- Configurable read-retry count (D-03). Not chosen.
- On-prem Azure DevOps Server support, dropped by D-12.
- Executing PR, release and pipeline writes live (D-13). Not chosen, because those writes are not cleanly reversible.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PLAT-02 | SDK `v1.0.0-b5` to `azuredevops/v7` v7.1.0 in one change, Go 1.27, patched `golang.org/x/sys`; CI runs `govulncheck` and fails on any reachable vulnerability; a test proves a v7 401 still classifies as auth | Swap verified (26 files, build/vet/race/gofmt green on go1.27.2); govulncheck clean only at `go 1.27.2`; real-SDK 401 probe returns value-form `WrappedError` -> `auth`; CI `go-version-file` semantics confirmed (Pattern 1, Code Examples 1-2) |
| PLAT-03 | Pacing transport honours `Retry-After` and rate-limit headers on any response incl. 200; reads retry on rate-limit a bounded number of times within their deadline; writes never retried; PAT only sent to hosts validated against the configured org | Pacer, host rule and NotSent prototypes verified against the real v7 SDK (Patterns 2-6, Code Examples 3-6); header semantics from MS Learn; redirect-hop and deadline mechanics read from Go 1.27.2 source |
| PLAT-05 | One live read per ADO sub-client and one dry run per write verb pass against the real org before later phases build on v7 | Read-to-sub-client map, dry-run target needs, stale-binary trap, bash 3.2 constraint, TUI checklist content (Pattern 7, Pitfalls 11-13) |
</phase_requirements>

## Project Constraints (from AGENTS.md)

No project-level `AGENTS.md` or `CLAUDE.md` exists. The global `~/.config/opencode/AGENTS.md` and `.planning/codebase/CONVENTIONS.md` apply:

| Directive | Source | Planner check |
|-----------|--------|---------------|
| Conventional commits `<type>(<scope>): <description>`, imperative, lowercase, no trailing period, max 72 chars, never `Co-Authored-By` | global AGENTS.md, CONVENTIONS.md:291-301 | Every task commit message |
| Never commit feature work to `master`/`main`; current branch is `dev`; do not create, rename or switch branches | global AGENTS.md, orchestrator note | No branch operations in plans |
| No emojis; no em dashes in prose | global AGENTS.md | Docs, comments, 08-SMOKE.md |
| Do not add features, refactors or improvements beyond what was asked; no error handling for impossible scenarios; no abstractions for one-time operations | global AGENTS.md | Rejects the `X-TFS-Session` deletion (Pitfall 1) and any token bucket |
| Single-use code stays inline; rule of three before extracting; no speculative generality; no exported symbol without a current external caller | CONVENTIONS.md:248-254 | Transport API stays minimal; the generic retry helper is justified by 16 interface read methods plus the identity func |
| No new module dependency without justification and confirmation; no new `internal/` layer without the same | CONVENTIONS.md:278-282 | `internal/transport` is approved by D-11; zero new modules |
| Structural guarantees asserted with `go/ast`, never grep | STATE.md Phase 4/5 decisions, CONTEXT | D-11 test, retry-decorator test |
| Classification is structural, never by error text | result.go:164-169 | New not-sent branches use `errors.Is`/`errors.As` |
| stdout carries parseable output only; diagnostics to stderr or log; transport never writes stdout/stderr | OUT-03, CONTEXT | Summary line printed by `cmd`, not transport |
| If a task uses paid API calls or credits, ask before running | global AGENTS.md | Smoke run is user-run against their org; govulncheck and proxy are free |

## Architectural Responsibility Map

This is a CLI/TUI, not a web stack; tiers are the repo's own layers.

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| SDK version, toolchain, vuln gate | Build / CI (`go.mod`, `.github/workflows/ci.yml`) | -- | The toolchain, not the SDK, decides govulncheck pass/fail |
| `org_url` shape validation (D-12) | `internal/config` (`Validate`) | `cmd` (exit 3 path already exists) | Validation runs before both CLI and TUI (`TestValidateGuardsCLIAndTUI`) |
| Host rule (D-09) and per-request refusal (D-10b) | `internal/transport` | -- | Only a `RoundTripper` sees every request and every redirect hop before bytes leave |
| Header-driven pacing (D-01, D-04) | `internal/transport` | -- | The SDK discards response headers on success and failure; only the transport sees them |
| Startup `LocationUrl` check (D-10a) | `internal/ado` (`NewClient`) | `internal/transport` (the one rule function) | Discovery results live in `NewClient`; the rule lives in transport |
| Explicit sub-client construction on the injected client (D-11) | `internal/ado` (`NewClient`) | -- | `connection.GetClientByUrl` cannot take options |
| Read-only 429 retry (D-03) | `internal/ado` (narrow-interface seam) | -- | Transport cannot tell read POSTs (WIQL) from write POSTs (queue build) |
| Failure kind for not-sent errors | `internal/ado` (`classifyFailure`) | -- | Single classifier; CLI and TUI both consume it |
| Write outcome for not-sent errors (D-02) | `internal/guard` (`classify`) | `cmd` (`writeExitCodeFor`, unchanged) | Outcome vocabulary lives in the guard |
| Throttle summary line (D-05/D-06) | `cmd/adoBizInt` | -- | Only `cmd` writes stderr; TUI logs only (D-07) |
| Live smoke run (PLAT-05) | `scripts/` + human | `.planning/.../08-SMOKE.md` | Never in CI (D-17) |

## Standard Stack

### Core (changes only)

| Library / Tool | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/microsoft/azure-devops-go-api/azuredevops/v7` | **v7.1.0** (replaces `azuredevops v1.0.0-b5`) | All ADO calls | Newest tag; `@latest` on proxy.golang.org is v7.1.0, published 2023-04-17 [VERIFIED: proxy.golang.org]. Adds `NewClientWithOptions` / `WithHTTPClient` [VERIFIED: v7 `client.go:67-82`, `client_options.go:10-15`] |
| Go toolchain (`go` directive) | **`go 1.27.2`** (from `go 1.25.4`) | Build and vuln gate | go.dev/dl lists `go1.27.2` and `go1.26.9` as current [VERIFIED: go.dev/dl JSON, 2026-10-19]. `go 1.27.2` + v7 + x/sys v0.47.0 scans clean; `go 1.27.0` on go1.27.1 has 8 stdlib findings [VERIFIED: govulncheck run this session] |
| `golang.org/x/sys` (indirect) | **v0.47.0** (from v0.38.0) | Clears GO-2026-5024 (fixed in 0.44.0) | v0.47.0 declares `go 1.25.0`; v0.48.0 and v0.49.0 declare `go 1.26.0` [VERIFIED: proxy.golang.org .mod files]. GO-2026-5024 = "integer overflow in NewNTUnicodeString in golang.org/x/sys/windows", fixed 0.44.0 [VERIFIED: vuln.go.dev/ID/GO-2026-5024.json] |
| `golang.org/x/vuln/cmd/govulncheck` | **v1.9.0**, run via `go run ...@v1.9.0`, never in `go.mod` | CI gate | Published 2026-10-11, module requires `go 1.26.0` [VERIFIED: proxy.golang.org]. Exits non-zero when code is affected [CITED: pkg.go.dev/golang.org/x/vuln/cmd/govulncheck] |

### Supporting (stdlib only, already available)

| Library | Purpose | When to Use |
|---------|---------|-------------|
| `net/http` (`RoundTripper`, `ParseTime`, `Request.Response`) | Pacer, host rule, redirect-hop detection | `internal/transport` only |
| `net/url` | Parse `org_url` (D-12) and `LocationUrl` (D-10a) | `internal/config`, `internal/transport`; `internal/ado` may import it (only `net/http` is forbidden) |
| `sync` / `sync/atomic` | `notBefore` mutex, wait and retry counters | TUI runs fetches in concurrent Bubble Tea commands, so counters must be race-safe |
| `testing/synctest` | Deterministic pacer tests, virtual clock | Verified working with `http.Client{Timeout: N}` and a fake base RT inside the bubble this session |
| `go/ast`, `go/parser` | D-11 structural test, retry-decorator structural test | Reuse `parsePackageFiles` in `internal/ado/nilguard_test.go:139` |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` | `golang/govulncheck-action` | A new third-party action to SHA-pin; locked decision already chose `go run` |
| Read-retry decorator at the narrow interfaces | `retryRead(...)` wrapper at each of about 27 read call sites | Call-site wrapping touches 8 files and a missed site silently never retries; the decorator is exhaustive by interface (Pattern 5) |
| `http.DefaultTransport` as the pacer's base | `http.DefaultTransport.(*http.Transport).Clone()` | Clone changes connection pooling vs b5; shared DefaultTransport is behaviour-preserving |

**Installation (PLAT-02, one commit):**
```bash
grep -rl 'azure-devops-go-api/azuredevops' --include='*.go' . \
  | xargs sed -i '' 's#github.com/microsoft/azure-devops-go-api/azuredevops#github.com/microsoft/azure-devops-go-api/azuredevops/v7#g'
go mod edit -droprequire github.com/microsoft/azure-devops-go-api/azuredevops
go get github.com/microsoft/azure-devops-go-api/azuredevops/v7@v7.1.0
go mod edit -go=1.27.2
go get golang.org/x/sys@v0.47.0
go mod tidy
go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```
Verified on a scratch copy this session. Expected `go.mod` diff: `go 1.27.2`; `azuredevops v1.0.0-b5` replaced by `azuredevops/v7 v7.1.0`; `golang.org/x/sys v0.38.0` to `v0.47.0`; `github.com/google/uuid v1.1.1` and `github.com/atotto/clipboard v0.1.4` move from `// indirect` to direct. The last two are a pre-existing untidiness (`go mod tidy -diff` on HEAD shows the same two moves) and close BACKLOG BL-02 IN-01 as a side effect. No `toolchain` line is written when the directive equals the switched-to toolchain [VERIFIED: scratch go.mod].

**Version verification:** `proxy.golang.org/<module>/@latest` and `/@v/<ver>.info` queried this session; checksums match `sum.golang.org/lookup` for v7.1.0, x/sys v0.47.0 and x/vuln v1.9.0.

## Package Legitimacy Audit

The `package-legitimacy` seam supports npm/pypi/crates only (`--ecosystem go` is rejected). Go modules were verified manually against the Go module proxy and checksum database, which is the authoritative source for Go.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `github.com/microsoft/azure-devops-go-api/azuredevops/v7` v7.1.0 | proxy.golang.org + sum.golang.org | 3.5 yrs (2023-04-17) | n/a (Go has no download counts) | github.com/microsoft/azure-devops-go-api (Origin in `.info`) | OK (manual) | Approved; same project as b5, new major path |
| `golang.org/x/sys` v0.47.0 | proxy.golang.org + sum.golang.org | 3.5 mo (2026-06-30) | n/a | go.googlesource.com/sys | OK (manual) | Approved; version bump of an existing indirect dep |
| `golang.org/x/vuln` v1.9.0 | proxy.golang.org + sum.golang.org | 8 days (2026-10-11) | n/a | go.googlesource.com/vuln | OK (manual) | Approved; CI tool only, never in `go.mod`, pinned by version and verified by sumdb on `go run` |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none. x/vuln v1.9.0 is new but is the Go team's own tool on the official host; if the planner prefers an older pin, v1.8.0 also needs Go >= 1.26 [CITED: .planning/research/STACK.md:190].

## Architecture Patterns

### System Architecture Diagram

```
 config.Load ──► config.Validate (D-12: https + cloud shape ──► {org}, org host)
                        │ bad: exit 3 "org_url: ..."
                        ▼
 main: tr := transport.New(org, cfg.RequestTimeout, http.DefaultTransport)
                        │
                        ▼
 ado.NewClient(cfg, tr) ─ conn := NewPatConnection(org_url, PAT)   (BaseUrl lowercased)
   │  root := NewClientWithOptions(conn, BaseUrl, WithHTTPClient(tr.HTTPClient()))
   │  areas := root.GetResourceAreas(ctx) ──► OPTIONS {base}/_apis, GET {base}/_apis/resourceAreas
   │  for id in {wit, git, build, release, work}:
   │      url := normalize(LocationUrl(id))  ── transport rule ── refused: exit 3 ConfigError{org_url, host}
   │  build &X.ClientImpl{Client: *NewClientWithOptions(conn, url, WithHTTPClient(hc))}
   │  wrap read seams in retry decorators (D-03)          location client on BaseUrl
   ▼
 CLI dispatch / TUI view ──► ado.Client method ──► retry decorator (reads only; 429 → re-issue ≤2)
                                                         │
                                                         ▼
                                                SDK Send (builds *http.Request, bytes.Reader body)
                                                         │  http.Client{Timeout: request_timeout}
                                                         ▼
                                  ┌──────────── transport.RoundTrip (every hop) ────────────┐
                                  │ 1 host rule(org, req.URL)  refused → NotSent / redirect │
                                  │ 2 wait := notBefore - now                               │
                                  │   wait > deadline-left → NotSent(rate-limited) fail-fast│
                                  │   else sleep (ctx done → NotSent wrapping ctx.Err())    │
                                  │ 3 base.RoundTrip(req)  ──► dev.azure.com / vsrm.dev...  │
                                  │ 4 read Retry-After / X-RateLimit-Delay → notBefore      │
                                  │   log X-RateLimit-* (file only); count waits            │
                                  └─────────────────────────────────────────────────────────┘
                                                         │ error path
                                                         ▼
 classifyFailure: rate-limited NotSent → rate_limit │ 401/403 → auth │ 429 → rate_limit │ ...
 guard.classify (writes): NotSent → failed (before parentErr) → writeExitCodeFor → 1 (2 if auth)
 cmd after dispatch: if waits+retries > 0 → one stderr line "throttled by ADO: ..."
```

### Recommended Project Structure

```
internal/
├── transport/            # NEW (D-11): stdlib only, the only production net/http importer
│   ├── transport.go      # Transport type: RoundTrip, host rule, pacing, NotSent errors, stats
│   └── transport_test.go # host-rule table, synctest pacing, redirect hop, PAT-never-logged
├── ado/
│   ├── client.go         # NewClient rewrite: discovery, startup host check, explicit construction
│   ├── retry.go          # NEW: read-only 429 retry decorators over workItemAPI/gitAPI/buildAPI/releaseAPI
│   ├── result.go         # classifyFailure gains the rate-limited NotSent branch; v7 line refs
│   ├── v7_test.go        # NEW: real v7 SDK via fake base RT (401 → auth, discovery, startup refusal)
│   ├── construct_test.go # NEW: D-11 go/ast structural test
│   └── retry_test.go     # NEW: retry counts, fail-fast not retried, writes pass through once
├── guard/guard.go        # classify: NotSent → failed before parentErr (D-02)
└── config/config.go      # Validate: org_url shape (D-12); org parser shared with main
cmd/adoBizInt/
├── main.go               # build transport, pass to ado.NewClient, print summary after dispatch
└── throttle.go           # NEW (or inline in main.go): summary line renderer + its test
scripts/smoke.sh          # NEW (D-14): reads + dry runs, pass/fail table, bash 3.2 compatible
.planning/phases/08-.../08-SMOKE.md  # NEW (D-15): pasted results + TUI checklist outcome
```

### Pattern 1: Atomic import swap with a real-SDK auth test (PLAT-02)
**What:** One commit rewrites every import, bumps toolchain and x/sys, tidies, updates CI, adds the v7 401 test, updates `result.go` line references and PROJECT.md.
**When to use:** PLAT-02 only. No constructor change in this commit; the existing `X.NewClient(ctx, connection)` calls still compile on v7.
**Key facts:**
- 26 files carry SDK imports (STACK.md's "24" predates Phase 7's `internal/guard/guard_test.go` and `internal/ui/workitems/resolve_test.go`) [VERIFIED: grep this session].
- `result.go`'s doc comment cites b5 lines `client.go:366, :403` (pointer) and `:389, :411` (value). v7 `UnwrapError` returns `&WrappedError{...}` at `client.go:375` and `:412`, and `WrappedError` values at `:398` and `:419` [VERIFIED: v7 `client.go:372-420`]. STACK.md's "374/398/403/414" is wrong; use 375/398/412/419. `UnwrapError` is byte-identical between b5 and v7 apart from line offset [VERIFIED: diff].
- The existing `TestClassifyFailure` (`internal/ado/result_test.go:22`) builds `WrappedError` from the same package `result.go` imports, so it cannot detect a half-migration. The new test must obtain the error **from a real v7 sub-client** (Code Example 2), which is the only shape that would fail if `result.go` and the clients ever imported different SDK modules.

### Pattern 2: Explicit sub-client construction on the injected client (D-11)
**What:** Build every sub-client with `NewClientWithOptions` on the area's `LocationUrl`, never via `X.NewClient(ctx, connection)` or `GetClientByUrl`.
**Why:** v7 `NewClient(connection, baseUrl)` builds `&http.Client{}` and only copies `TlsConfig`/`Timeout` [VERIFIED: v7 `client.go:55-65`]; `connection.GetClientByUrl` always calls it [VERIFIED: v7 `connection.go:74-82`]; every typed `X.NewClient(ctx, connection)` goes through `GetClientByResourceAreaId` -> `GetClientByUrl` [VERIFIED: e.g. v7 `workitemtracking/client.go:216-224`]; `location.NewClient` calls `GetClientByUrl(connection.BaseUrl)` [VERIFIED: v7 `location/client.go:48-53`].
**Rules:**
- `hc.Timeout = cfg.RequestTimeout` must be set on the injected client; `WithHTTPClient` bypasses `connection.Timeout` entirely [VERIFIED: v7 `client.go:67-82`]. A zero budget keeps today's meaning (no per-call bound), matching `newConnection`'s nil-for-zero rule (`internal/ado/client.go:52-60`).
- Normalize each `LocationUrl` exactly as the SDK did: `strings.ToLower(strings.TrimRight(url, "/"))` [VERIFIED: v7 `connection.go:55-57`]. Live `LocationUrl`s carry a trailing slash (`https://dev.azure.com/{org}/`), and the SDK's global location cache is keyed by the exact base URL string [VERIFIED: v7 `client.go:209-244`]; without normalization each sub-client pays an extra `OPTIONS` and wire URLs change case vs b5.
- Resource area IDs used (verbatim, v7): `workitemtracking/client.go:25` `var ResourceAreaId, _ = uuid.Parse("5264459e-e5e0-4bd8-b118-0985e68a4ec5")`; `git/client.go:26` `"4e080c62-fa21-4fbc-8fef-2a10a2b38049"`; `build/client.go:25` `"965220d5-5bb9-42cf-8d67-9b146df2a5a4"`; `release/client.go:24` `"efc2f575-36ef-48e9-b672-0c6fb4a48ac5"`; `work/client.go:23` `"1d4f49f9-02b9-4e26-b826-2cdb6195f2a9"` [VERIFIED: v7 source]. Reference them as `workitemtracking.ResourceAreaId` etc., never as literals.
- Missing-area semantics: mirror the SDK. A non-empty area list that lacks a needed ID is an error (`ResourceAreaIdNotRegisteredError`); an empty list falls back to `connection.BaseUrl` [VERIFIED: v7 `connection.go:84-109`]. BaseUrl is the org_url host, so the fallback always passes the host rule.
- `GetResourceAreas` itself sends `api-version=5.1-preview.1` even in v7 [VERIFIED: v7 `client.go:422-433`]; the v1 tool already depends on it working at every startup.
- `interfaces.go`'s comment says "The SDK constructors return interfaces, so their assignments in NewClient are already compile-time drift checks. Do not add concrete ClientImpl assertions." Assigning `&git.ClientImpl{...}` to a `gitAPI` field is still a compile-time check; reword the comment rather than add assertions.

### Pattern 3: One host rule for startup and every hop (D-09, D-10)
**What:** A single function decides whether a URL may receive the PAT. `ado.NewClient` calls it on each used `LocationUrl`; `RoundTrip` calls it on every request.
**Rule (case-insensitive host and path; https only; no userinfo; no explicit port):**
1. Host `dev.azure.com` or ending in `.dev.azure.com` (label boundary): allowed only if the cleaned path is `/{org}` or starts with `/{org}/`. This applies **even when `org_url` is `https://dev.azure.com/{org}`**: do not let the "exact org_url host" clause short-circuit the path check, or `dev.azure.com/otherorg` passes.
2. Host `{org}.visualstudio.com`, or `{org}.<label>.visualstudio.com` (prefix `{org}.`, suffix `.visualstudio.com`): allowed, any path.
3. Everything else refused.
**Why both points:** Go forwards `Authorization` on a redirect when the destination host equals or is a subdomain of the initial host [VERIFIED: Go 1.27.2 `net/http/client.go:1017-1062`], so a same-host cross-org redirect would carry the PAT. A probe confirmed a 307 from `dev.azure.com/acme/...` to `vsrm.dev.azure.com/...` reaches the transport with `Authorization` set and `req.Response != nil` [VERIFIED: probe this session].
**Startup scope:** check the five areas the tool builds plus BaseUrl, not all 239 areas ADO returns (Open Question 3).

### Pattern 4: Header-driven pacer with typed not-sent errors (D-01, D-02, D-04)
**What:** A `RoundTripper` holding one process-wide, mutex-guarded `notBefore`. Before sending: host rule, then wait for `notBefore` if it fits the request deadline. After receiving: advance `notBefore` from `Retry-After` and `X-RateLimit-Delay` (take the max), log `X-RateLimit-*` to the file.
**Facts that shape it:**
- Deadline source: for a non-stdlib `RoundTripper`, `http.Client` installs its `Timeout` as a context deadline on the request before calling `RoundTrip` (`setRequestCancel`, custom-transport branch) [VERIFIED: Go 1.27.2 `net/http/client.go:358-381`]. A probe with no caller deadline and `hc.Timeout = 10s` failed fast on `Retry-After: 45` [VERIFIED: probe]. Every CLI/TUI read and write also carries a `fetchctx` deadline (`const defaultBudget = 60 * time.Second`, `internal/fetchctx/fetchctx.go:28`).
- `RoundTrip` must close `req.Body` on every error path and must not modify the request [CITED: Go `net/http/client.go:119-143` RoundTripper doc]. Modifying the **response** headers is allowed but unnecessary (Pitfall 1).
- Error surface: the client wraps the transport's error in `*url.Error`; `errors.Is` / `errors.As` reach the typed error through it [VERIFIED: probe]. `*url.Error` implements `net.Error`, and `url.Error.Timeout()` delegates to the wrapped error, so the not-sent type must **not** implement `Timeout()` or `classifyFailure` maps it to `timeout` (`result.go:186-188`).
- Three not-sent reasons, one sentinel: refused host, D-01 fail-fast, ctx done while held. Only the fail-fast maps to `rate_limit`. The held-then-cancelled form should wrap `ctx.Err()` so reads keep today's cancel/timeout classification, while `errors.Is(err, ErrNotSent)` still lets `guard.classify` call it `failed`.
- A redirect hop (`req.Response != nil`) is **not** "never left the process": the first hop already reached ADO. A refusal or hold-cancel on a redirect hop must return an error that is not `ErrNotSent`, so a write classifies `unknown` (P-13 no-status path), not `failed` (Pitfall 4).
- `X-RateLimit-Delay` is decimal seconds up to 3 places; `Retry-After` is integer seconds or HTTP-date [CITED: learn.microsoft.com rate-limits]. D-04 parses `Retry-After` with `strconv.Atoi` then `http.ParseTime`, and `X-RateLimit-Delay` with `strconv.ParseFloat`. Ignore negative, unparseable and past-date values (log them).
- Process-wide `notBefore` is correct: ADO throttles per identity on a global 200 TSTU / 5-minute budget, not per resource [CITED: learn.microsoft.com rate-limits].

### Pattern 5: Read-only retry as an embedding decorator (D-03)
**What:** In `internal/ado/retry.go`, one small type per narrow interface embeds the real client and overrides only the read methods; writes fall through the embedded field untouched.
```go
type retryingGit struct {
	gitAPI                  // writes (CreatePullRequestReviewer, UpdatePullRequest) fall through: never retried
	retries *atomic.Int64
}

func (r retryingGit) GetPullRequest(ctx context.Context, a git.GetPullRequestArgs) (*git.GitPullRequest, error) {
	return retryRead(ctx, r.retries, func() (*git.GitPullRequest, error) { return r.gitAPI.GetPullRequest(ctx, a) })
}
```
**Why this placement:** The four interfaces in `internal/ado/interfaces.go` declare 21 methods: 16 reads and 5 writes (`UpdateWorkItem`, `CreatePullRequestReviewer`, `UpdatePullRequest`, `QueueBuild`, `UpdateReleaseApproval`) [VERIFIED: `internal/ado/interfaces.go`]. The decorator never touches the about 27 read call sites (counted by grep over non-test files), existing fakes assigned directly to `c.gitClient` etc. are unaffected, and the safe default for any un-overridden method, including one Phase 9 adds, is "not retried". Make it exhaustive with one `go/ast` test: the set of overridden methods must equal the interface methods minus the five writes, so a new read without an override, or any write override, fails the test.
**`retryRead` rules:** retry only when `StatusCodeOf(err) == 429` (a raw server 429), never on the D-01 fail-fast (it has no status), never when `ctx.Err() != nil`; at most 2 retries; on exhaustion wrap the last error as `fmt.Errorf("rate limited after 2 retries: %w", err)` so `classifyFailure` still sees the 429 (D-08). The re-issued SDK call re-marshals its body into a fresh `bytes.NewReader`, so no `GetBody` replay is involved (Pitfall 8).
**Also a read:** wrap `getConnectionData` (identity) and, optionally, the startup `GetResourceAreas` call with the same helper. Pre-reads (`GetPullRequestDetail`, `GetReleaseApprovalDetail`, `GetPipelineDefinitionDetail`, `GetWorkItemDetail`) are reads and are covered automatically because they call the decorated interfaces.

### Pattern 6: Not-sent writes are `failed` (D-02)
`guard.classify` today (verbatim, `internal/guard/guard.go:252-258`):
```go
func classify(err, parentErr error) string {
	if parentErr != nil {
		return OutcomeUnknown
	}
	if err == nil {
		return OutcomeExecuted
	}
```
Insert `if errors.Is(err, <ErrNotSent>) { return OutcomeFailed }` as the **first** statement. Outcome constants (verbatim, `internal/guard/guard.go:27-31`): `OutcomeDryRun = "dry_run"`, `OutcomeNoOp = "no_op"`, `OutcomeExecuted = "executed"`, `OutcomeFailed = "failed"`, `OutcomeUnknown = "unknown"`. Audit runs only for executed/unknown (`guard.go:207` `if outcome == OutcomeExecuted || outcome == OutcomeUnknown {`), so a not-sent write gets no record, as D-02 requires. `writeExitCodeFor` (`cmd/adoBizInt/exit.go:54-66`) then maps `failed` to `exitPartial` (1) or `exitAuth` (2) unchanged. `runWrite` (`cmd/adoBizInt/write.go:166-225`) has no separate `ctx.Err()` branch, so the guard's outcome is the CLI's outcome [VERIFIED: read this session]. A write 429 already classifies `failed` today through the P-08 `status < 500` path (`guard.go:271-285`); the only new risk is the retry decorator wrapping a write.

### Pattern 7: Smoke run as a reproducible script (PLAT-05)
**Reads (exit 0 and `.complete == true`)**, mapped to D-16's sub-clients:

| CLI read | Sub-client exercised |
|----------|---------------------|
| `workitems list --json`, `dashboard counts --json` | workitemtracking (WIQL POST, batch GET, classification nodes) |
| `prs list --json`, `branches list --json` | git |
| `pipelines runs --json`, `pipelines definitions --json` | build |
| `releases list --json`, `releases approvals --json` | release (vsrm host) |
| identity at startup (`GetConnectionData`) | location: assert the log has no `Could not cache current user ID` line for this run (`internal/ado/client.go:112-114`) |
| (none) | work: dormant, construction only; name the gap in 08-SMOKE.md (D-16) |

**Dry runs (no `--confirm`):** `workitems set-state`, `prs approve`, `prs complete`, `releases approve`, `releases reject`, `pipelines queue`. Each needs a real target, supplied by env vars (for example `SMOKE_PROJECT`, `SMOKE_WORKITEM_ID`, `SMOKE_STATE`, `SMOKE_REPO`, `SMOKE_PR_ID`, `SMOKE_APPROVAL_ID`, `SMOKE_DEFINITION_ID`, `SMOKE_BRANCH`). PASS = exit 0 with `.write.outcome == "dry_run"`; record exit 4 (`no_op`) as "pre-read exercised, no diff" rather than a failure; anything else FAIL. A verb with no available target is SKIP with the reason, never silently PASS.
**Manual, outside the script:** the D-13 `workitems set-state --confirm` on the scratch item, the TUI Task close with effort (IN-01), TUI view loads, TUI confirm modals dismissed with n/esc, and one TUI attachment download (Open Question 8).

### Anti-Patterns to Avoid
- **Retrying inside the transport:** it cannot tell `QueryByWiql` (POST, read) from `QueueBuild` (POST, write) [VERIFIED: method table below]; a transport retry of a write double-mutates with one audit record.
- **Checking every one of the ~239 resource areas at startup:** turns an unused area's host into a startup outage (Open Question 3).
- **Typing a redirect-hop refusal as not-sent:** reports a write `failed` after its first hop already reached ADO (Pitfall 4).
- **Implementing `Timeout()` on the not-sent error:** `*url.Error` forwards it and `classifyFailure` turns D-01 into `timeout`.
- **Running `adoBizInt` from `PATH` in the smoke script:** `~/go/bin/adoBizInt` (2026-08-17) and the repo-root binary (2026-10-12) are b5 builds (Pitfall 12).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP-date parsing for `Retry-After` | A custom date parser | `http.ParseTime` | Handles all three RFC 7231 date formats |
| Body replay for redirects | Buffering and re-attaching `req.Body` in the transport | Nothing: `http.NewRequest` sets `GetBody` for `*bytes.Reader`, which every SDK call uses | [VERIFIED: Go 1.27.2 `net/http/request.go:933-955`; v7 sub-clients pass `bytes.NewReader(body)`, e.g. `workitemtracking/client.go:2987`] |
| Retry of a failed read | Body replay or request cloning in the transport | Re-invoke the SDK method from the decorator | The SDK re-marshals and builds a fresh request every call |
| Credential stripping on cross-domain redirect | Custom `CheckRedirect` header scrubbing | Stdlib already strips `Authorization` for non-subdomain targets; add the per-hop host rule for the rest | [VERIFIED: Go `client.go:1017-1062`] |
| Deterministic time in pacer tests | Injectable clocks, sleeps, `time.Now` seams | `testing/synctest.Test` with a fake base `RoundTripper` | Verified this session: a 7s `Retry-After` gap measures exactly 7s, zero wall time |
| Rate limiting | `golang.org/x/time/rate`, retry libraries | The header-driven pacer | ADO publishes no fixed rate; libraries are new dependencies and retry writes by default [CITED: .planning/research/STACK.md:166-170] |
| `govulncheck` install | `go install` into the runner, a third-party action, or a `go.mod` tool dependency | `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` | Version-pinned, sumdb-verified, nothing added to `go.mod` (locked) |
| Structural "no default client" check | A grep for `NewClient(` | `go/ast` walk via `parsePackageFiles` (`internal/ado/nilguard_test.go:139`) | STATE.md records three grep criteria defeated by comments |

**Key insight:** every piece of this phase already exists in the stdlib or the SDK. The work is wiring (which client gets which `http.Client`) and classification (which error means "never sent"), not new mechanisms.

## Runtime State Inventory

Included because PLAT-02 is a dependency migration.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `audit.jsonl` records carry `command`, `target`, `changes`, `outcome`, `actor`, `user_approved`: no SDK, API-version or toolchain strings. `priorities.yaml` and `config.yaml` hold no version strings. The SDK's location cache is in-memory only. | None. Verified by reading `internal/guard/guard.go` record construction and `internal/config/config.go`. |
| Live service config | ADO org: no server-side configuration changes; the tool only reads resource areas. GitHub: the CI workflow lives in git (`.github/workflows/ci.yml`); the job name `check` is unchanged, so any branch-protection required check keeps matching. | Code edit to `ci.yml` only. |
| OS-registered state | None: no launchd, cron or Task Scheduler entries reference the tool (none created by any prior phase). | None. |
| Secrets / env vars | `ADO_PAT`, `ADO_ORG_URL`, `ADO_PROJECTS` names unchanged. **D-12 tightens `org_url`**: the user's value must be `https://dev.azure.com/{org}` or `https://{org}.visualstudio.com`. Evidence it already is: g-01-1 debug output shows `Options "https://dev.azure.com/windstream/_apis"` (`.planning/debug/g-01-1-offline-startup-block.md`). | Confirm during smoke that startup passes validation; no rename. |
| Build artifacts | `/adoBizInt` in the repo root (gitignored, built 2026-10-12 with go1.27.1 and b5) and `~/go/bin/adoBizInt` (2026-08-17), which is first on `PATH`. Go module cache holds both SDK versions and the go1.27.2 toolchain. | Smoke script must build a fresh binary to a temp path and assert `go version -m` shows `azuredevops/v7 v7.1.0` and `go1.27.2`. Optionally rebuild or `go install` after the phase. |

## Common Pitfalls

### Pitfall 1: Fixing the `SessionId` race that does not exist
**What goes wrong:** Adding `resp.Header.Del("X-TFS-Session")` to the pacer, as STACK.md recommends, to stop a data race on the package global `azuredevops.SessionId`.
**Why it happens:** v7 writes `SessionId` from `resp.Header[headerKeySession]` (`client.go:133-136`, `:259-262`) and reads it in `CreateRequestMessage` (`:190-194`), with `headerKeySession = "X-TFS-Session"` (`:32`). That looks racy. But the lookup is a raw map index with a **non-canonical** key, and Go canonicalizes every response header key to `X-Tfs-Session` for HTTP/1 (`net/textproto/reader.go:563`) and HTTP/2 (`net/http/internal/http2/transport.go:2222`). Real ADO responses are HTTP/2 and carry `x-tfs-session` [VERIFIED: live anonymous probe]. The write branch can never execute.
**Evidence:** `-race` probe with 4 goroutines x 50 calls each through the real v7 client: no race with canonical headers; `WARNING: DATA RACE` at `client.go:261` vs `:193` only when the test forces the non-canonical key into the map. `SessionId` unchanged after responses carrying the header [VERIFIED: probes this session].
**How to avoid:** Do not add the deletion (it is dead-code defense against an impossible case). Optionally add one regression test asserting `azuredevops.SessionId` is unchanged after a real-client call whose response carries the header, so a future SDK fix that canonicalizes the lookup is noticed.

### Pitfall 2: govulncheck passes or fails on the toolchain that runs it
**What goes wrong:** Setting `go 1.27.0` (as STACK.md suggested) leaves local go1.27.1 and any CI that installs 1.27.0 with 8 stdlib findings.
**Why it happens:** Source mode analyzes "the Go version specified by the go command found on the PATH" [CITED: pkg.go.dev govulncheck]. The directive only matters because `GOTOOLCHAIN=auto` (default, verified locally) switches up to it and `setup-go` installs it.
**How to avoid:** `go 1.27.2` exactly; CI `go-version-file: go.mod`; no `toolchain` line. Re-run the gate at implementation time and raise the patch if a newer 1.27.x security release has shipped. Document: CI can turn red later with no code change when a new stdlib advisory lands; the fix is a directive bump. A developer with `GOTOOLCHAIN=local` and an older Go gets "go.mod requires go >= 1.27.2".
**Warning signs:** `Your code is affected by N vulnerabilities from the Go standard library` with `Fixed in: ...@go1.27.x`.

### Pitfall 3: A fully rejected PAT exits 3, not 2, at startup
**What goes wrong:** Writing SC2's test as "run the binary with a bad PAT and expect exit 2". It exits 3 today and will on v7.
**Why it happens:** Resource-area discovery is the first authenticated request; a well-formed but rejected PAT gets 401 there, `ado.NewClient` returns the error, and `main.run` prints `Error connecting to Azure DevOps` and returns `exitUnknown` (3) [VERIFIED: `cmd/adoBizInt/main.go:111-116`; behavior observed live in v1.0, `.planning/milestones/v1.0-phases/02-honest-reads/02-01-SUMMARY.md:240-245`, logged and deferred].
**How to avoid:** Prove SC2 where a 401 is classified: a real v7 client returning 401 on a data call yields `Failure.Kind == "auth"` (read path, `exitCodeFor` -> 2) and `failed` + `FailureAuth` (write path, `writeExitCodeFor` -> 2). Record the startup case as unchanged pre-existing behavior (Open Question 1).

### Pitfall 4: Treating a redirect-hop refusal as "never sent"
**What goes wrong:** A write whose first hop reached ADO, then got a 307/308 to a refused host, is reported `failed` and not audited, although ADO may have acted.
**Why it happens:** `RoundTrip` runs once per hop; the refusal looks the same unless the hop is distinguished.
**How to avoid:** `req.Response != nil` marks a redirect hop [VERIFIED: Go `net/http/request.go:325-328`; probe]. Return a refusal error that does **not** satisfy `errors.Is(err, ErrNotSent)` on hops, so the guard's no-status rule yields `unknown`. Go converts 301/302/303 on non-GET to GET without body, and replays the body on 307/308 via `GetBody` [VERIFIED: Go `client.go:514-542`].

### Pitfall 5: The fail-fast error classifies as `other` or `timeout`
**What goes wrong:** D-01's error shows up as `kind: "other"` (today's fall-through) or `timeout`.
**Why it happens:** `classifyFailure` knows only context errors, `net.Error.Timeout()` and HTTP status (`internal/ado/result.go:170-201`). The probe confirmed today's code returns `other` for the wrapped not-sent error.
**How to avoid:** Add a branch for the transport's rate-limited not-sent error that returns `FailureRateLimit`. The fail-fast must not wrap `context.DeadlineExceeded`, and the error type must not implement `Timeout()`. Keep the existing `context.Canceled` and deadline branches for the hold-then-cancelled form.

### Pitfall 6: The SDK's process-global location cache leaks between tests
**What goes wrong:** A test asserting "OPTIONS then GET" passes alone and fails in the package run, or a second test sees one fewer request.
**Why it happens:** `apiResourceLocationCache` is a package global keyed by base URL [VERIFIED: v7 `client.go:43-45`, `:233-244`]. The probe's second test received no `OPTIONS` because the first had cached `https://dev.azure.com/acme`.
**How to avoid:** Fake base RTs always answer `OPTIONS` with a location list and never count it; or give each test a unique org (for example derived from `t.Name()`). Never run tests that share an org URL in parallel with different location fixtures.

### Pitfall 7: Case-sensitive org binding
**What goes wrong:** `https://dev.azure.com/Windstream/` from discovery is refused because the configured org is `windstream`, or vice versa.
**Why it happens:** `NewPatConnection` lowercases `BaseUrl` [VERIFIED: v7 `connection.go:18-26`, `:55-57`]; the resource-area API preserves the org's own casing in `locationUrl` [CITED: learn.microsoft.com work-with-urls, example `https://dev.azure.com/Fabrikam`].
**How to avoid:** Compare host and the `/{org}` path segment case-insensitively; normalize `LocationUrl` with the SDK's own `ToLower(TrimRight(u, "/"))` before building clients.

### Pitfall 8: Consuming or replaying the request body in the transport
**What goes wrong:** Reading `req.Body` to "inspect" a POST, or attempting a retry with `GetBody`, breaks the request or double-sends a write.
**How to avoid:** The transport never reads the body and never retries. On refusal it must `req.Body.Close()` (RoundTripper contract). Stdlib uses `GetBody` only for 307/308. Service-layer read retries call the SDK again, which builds a new body.

### Pitfall 9: v7 sends deprecated preview versions for every call
**What goes wrong:** Assuming v7 "moves off preview APIs". It does the opposite for most calls: b5 sent released `5.1`; v7 sends `7.1-preview.N` (table below). Microsoft's policy: once a version is released, "its preview version ... is deprecated and can be deactivated after 12 weeks" [CITED: learn.microsoft.com rest-api-versioning].
**How to avoid:** Accept it (the upgrade is locked), and let the smoke run be the gate. Evidence that ADO does not enforce deactivation broadly: the tool's startup has called `resourceAreas` at `5.1-preview.1` successfully through 2026-10-12 [VERIFIED: v7/b5 `client.go` GetResourceAreas; live v1.0 smoke in BACKLOG BL-02]. Record any `VS403...`/`api-version` rejection in 08-SMOKE.md as a v7 regression that blocks Phase 9 (D-15).

### Pitfall 10: Misreading `X-RateLimit-Delay`
**What goes wrong:** Treating it as a future instruction equivalent to `Retry-After` and expecting the server to stop delaying.
**Why it happens:** Microsoft defines it as "How long the request is delayed", that is, the server-side delay already applied to this response [CITED: learn.microsoft.com rate-limits]. D-04 locks honoring it as a client-side wait, which is the conservative direction (slower, never faster).
**How to avoid:** Implement D-04 as written; take `max(Retry-After, X-RateLimit-Delay)` rather than the sum; do not "fix" it later without a decision.

### Pitfall 11: A smoke script that leaks or lies
**What goes wrong:** `set -x`, `env`, or echoing the config prints the PAT; a missing target is reported PASS; bash 4 syntax fails on macOS.
**How to avoid:** No `set -x`; never print env, config or headers; print only command name, exit code, `complete`/`outcome` and the tool's own stderr (which never carries the PAT: `TestPATNeverLogged`, `cmd/adoBizInt/patleak_test.go`). Use `/bin/bash` 3.2 syntax (no associative arrays, no `mapfile`); `jq` 1.8.2 is available locally. Unset targets are SKIP with a reason.

### Pitfall 12: Smoke-testing the wrong binary
**What goes wrong:** `adoBizInt` on `PATH` resolves to `~/go/bin/adoBizInt` from 2026-08-17 (b5), so the smoke "passes" without exercising v7.
**How to avoid:** `go build -o "$tmp/adoBizInt" ./cmd/adoBizInt`, then assert `go version -m "$tmp/adoBizInt"` contains `github.com/microsoft/azure-devops-go-api/azuredevops/v7	v7.1.0` and `go1.27.2`, and record that line in 08-SMOKE.md.

### Pitfall 13: Phase 7 locks and "byte-identical" claims
**What goes wrong:** The import swap changes `cmd/adoBizInt/write_test.go` and `cmd/adoBizInt/patleak_test.go`, both listed in `.planning/phases/07-guarded-tui-writes/07-cli-tests.sha256`, and `internal/ado/attachments_test.go`, which holds `TestNoRawHTTPInADO`.
**How to avoid:** Execute only after Phase 7 verifies (CONTEXT sequencing). In the PLAT-02 summary, record that the only diff in those files is the import path (`git diff --stat` plus `git diff -U0` limited to `import` lines), and that `TestNoRawHTTPInADO`'s function body is byte-identical (D-11 means the test, not the whole file).

### Pitfall 14: The release pre-read is heavy
**What goes wrong:** The `releases approve/reject` dry run is slow or trips throttling.
**Why it happens:** `GetReleaseApprovalDetail` pages pending, approved and rejected approvals, up to 50 pages of 200 each per status, until it finds the id (`internal/ado/preread.go:149-200`).
**How to avoid:** Choose a recent pending approval for `SMOKE_APPROVAL_ID`. If the run is paced, the D-06 summary line is the evidence the pacer works live; record it.

### Pitfall 15: Logging every response's cost header
**What goes wrong:** ADO sends `x-ratelimit-cost` on ordinary 200s [VERIFIED: live anonymous probe]; logging it per request grows an already-unrotated log (426 MB on 2026-10-12, BACKLOG BL-02).
**How to avoid:** Log a pacing line only when `Retry-After`, `X-RateLimit-Delay` or `X-RateLimit-Remaining` is present, or when a wait or refusal happens. Never log the URL query, request headers or body.

### Pitfall 16: Pre-read rate limiting exits 3, not 1
**What goes wrong:** A plan asserts every rate-limited write exits 1.
**Why it happens:** A rate-limited **pre-read** surfaces as `Report.PreReadErr`, which `runWrite` maps to `exitUnknown` (3): nothing was attempted (`cmd/adoBizInt/write.go:180-183`). Only a rate-limited **mutation** is `failed` -> 1.
**How to avoid:** Write SC3's write test against the mutate step (pre-read succeeds, mutate gets 429 or fail-fast).

## Code Examples

Prototypes below compiled and passed against v7.1.0 on go1.27.2 in a scratch copy this session. Names inside `internal/transport` are recommendations, not existing symbols.

### 1. CI workflow (PLAT-02)
```yaml
# Source: current .github/workflows/ci.yml:17-31 plus setup-go README (go-version-file)
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
      - run: go build ./...
      - run: go vet ./...
      - run: go test -race ./...
      - name: gofmt
        run: |
          unformatted=$(gofmt -l .)
          if [ -n "$unformatted" ]; then
            echo "gofmt needs to be run on:"
            echo "$unformatted"
            exit 1
          fi
      - name: govulncheck
        run: go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```
Update the header comment (`ci.yml:1-2`) to list the new gate. setup-go v6+ uses the `toolchain` directive when present, else the `go` directive [CITED: github.com/actions/setup-go README].

### 2. A v7 401 from the real SDK classifies as auth (SC2)
```go
// Source: verified probe, internal/ado test file (test files may import net/http)
type rtFunc func(*http.Request) (*http.Response, error)
func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestV7RejectedTokenClassifiesAuth(t *testing.T) {
	base := rtFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodOptions { // SDK location discovery; may be cached (Pitfall 6)
			return jsonResp(r, 200, locationsFixture), nil
		}
		return jsonResp(r, 401, `{"$id":"1","message":"TF400813: The user is not authorized to access this resource.","typeKey":"UnauthorizedRequestException","errorCode":0,"eventId":3000}`), nil
	})
	conn := azuredevops.NewPatConnection("https://dev.azure.com/"+uniqueOrg(t), "fixture-pat")
	hc := &http.Client{Transport: base} // PLAT-02 commit; PLAT-03 swaps in the transport
	c := &git.ClientImpl{Client: *azuredevops.NewClientWithOptions(conn, conn.BaseUrl, azuredevops.WithHTTPClient(hc))}
	p := "P"
	_, err := c.GetRepositories(context.Background(), git.GetRepositoriesArgs{Project: &p})
	if got := classifyFailure(err); got != FailureAuth {
		t.Fatalf("classifyFailure(real v7 401) = %q, want %q", got, FailureAuth)
	}
}
```
Also cover the pointer form: a 401 with `ContentLength: 0` takes v7 `client.go:373-379`. The probe observed `errors.As(err, &value) == true` for the JSON form, matching v1.0's live A1 evidence. `FailureAuth` is verbatim `FailureAuth FailureKind = "auth"` (`internal/ado/result.go:28`).

### 3. Host rule (D-09)
```go
// Source: verified prototype; org is the lowercase {org} parsed from org_url (D-12)
func Allowed(org string, u *url.URL) bool {
	if u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "dev.azure.com" || strings.HasSuffix(host, ".dev.azure.com") {
		p := strings.ToLower(path.Clean("/" + u.Path))
		return p == "/"+org || strings.HasPrefix(p, "/"+org+"/")
	}
	if host == org+".visualstudio.com" {
		return true
	}
	return strings.HasPrefix(host, org+".") && strings.HasSuffix(host, ".visualstudio.com")
}
```
D-09's "exact org_url host" clause needs no branch of its own: D-12 admits only `dev.azure.com/{org}` (covered, path-bound) and `{org}.visualstudio.com` (covered). `path.Clean` hardens against `/{org}/../other/` (the SDK path-escapes route values with `url.PathEscape`, which leaves `..` intact [VERIFIED: v7 `client.go:281-302`]). Trailing-dot hosts (`dev.azure.com.`) and non-ASCII look-alikes fail the exact comparisons, which is the safe direction.

### 4. Pacer `RoundTrip` skeleton (D-01, D-02, D-04, D-10b)
```go
// Source: verified prototype (simplified)
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	hop := req.Response != nil // redirect hop: the first hop already left
	if !Allowed(t.org, req.URL) {
		closeBody(req)
		return nil, t.refusal(hop, notSentHost, req.URL.Hostname(), nil)
	}
	if wait := t.untilNotBefore(); wait > 0 {
		if dl, ok := req.Context().Deadline(); ok && time.Until(dl) < wait {
			closeBody(req)
			return nil, t.refusal(hop, notSentRateLimited, "", nil) // "ADO asked for 45s, 12s left in budget"
		}
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
			t.countWait(wait)
		case <-req.Context().Done():
			timer.Stop()
			closeBody(req)
			return nil, t.refusal(hop, notSentHeld, "", req.Context().Err()) // wraps ctx.Err()
		}
	}
	resp, err := t.base.RoundTrip(req)
	if resp != nil {
		t.observe(resp.Header) // Retry-After / X-RateLimit-Delay -> notBefore (max); log X-RateLimit-*
	}
	return resp, err
}
```
After the timer fires, re-check `notBefore` once before sending if concurrent responses may have moved it (TUI fetches run concurrently).

### 5. `NewClient` construction skeleton (D-10a, D-11)
```go
// Source: verified prototype; internal/ado never names net/http (type inference only)
hc := tr.HTTPClient() // *http.Client{Transport: tr, Timeout: cfg.RequestTimeout}
conn := azuredevops.NewPatConnection(cfg.OrgURL, cfg.PAT)
opt := azuredevops.WithHTTPClient(hc)
root := azuredevops.NewClientWithOptions(conn, conn.BaseUrl, opt)
areas, err := root.GetResourceAreas(ctx)
if err != nil {
	return nil, err
}
witURL, err := areaURL(areas, workitemtracking.ResourceAreaId, conn.BaseUrl, tr) // host rule here
if err != nil {
	return nil, err // *config.ConfigError{Field: "org_url", Message: "... host vsrm.example ..."}
}
wit := &workitemtracking.ClientImpl{Client: *azuredevops.NewClientWithOptions(conn, witURL, opt)}
// ... git, build, release, work likewise; location on conn.BaseUrl
```
`config.ConfigError` is verbatim `type ConfigError struct { Field string; Message string }` with `Error()` returning `e.Field + ": " + e.Message` (`internal/config/config.go:200-207`). `main.run` already maps any `ado.NewClient` error to exit 3 (`cmd/adoBizInt/main.go:111-116`).

### 6. Deterministic pacing test
```go
// Source: verified probe (synctest bubble, fake base RT, real v7 client)
func TestRetryAfterOn200PacesNextRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var sent []time.Time
		base := rtFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method == http.MethodOptions {
				return jsonResp(r, 200, locationsFixture), nil
			}
			sent = append(sent, time.Now())
			return jsonRespWith(r, 200, `{"count":0,"value":[]}`, "Retry-After", "7"), nil
		})
		c := gitClientOn(t, base)
		for i := 0; i < 2; i++ {
			mustGetRepositories(t, c)
		}
		if gap := sent[1].Sub(sent[0]); gap != 7*time.Second {
			t.Fatalf("second request sent after %v, want 7s", gap)
		}
	})
}
```
Do not use `httptest.NewServer` inside a bubble (real network I/O) [CITED: .planning/research/STACK.md:30].

### 7. Throttle summary line (D-05, D-06)
```go
// cmd/adoBizInt: called once after dispatch returns; never from transport
func renderThrottleSummary(w io.Writer, waits int, waited time.Duration, retries int64) {
	if waits == 0 && retries == 0 {
		return
	}
	fmt.Fprintf(w, "throttled by ADO: %d %s, %s total, %d %s\n",
		waits, plural(waits, "wait", "waits"), waited.Round(time.Second), retries, plural(int(retries), "retry", "retries"))
}
```
`main.run` holds the concrete `*ado.Client` and transport, so the `client` interface in `dispatch.go` and the locked `cmd` fakes need no new method.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `azuredevops v1.0.0-b5` builds its own `http.Client`, no hook | v7 `NewClientWithOptions` + `WithHTTPClient` | v7.x (tag v7.1.0, 2023-04-17) | Enables the transport; every sub-client must be constructed explicitly |
| `go 1.25.4` (out of support) | `go 1.27.2` | go1.27.2 is current with go1.26.9 (2026-10-19) | Clears the stdlib findings; requires `GOTOOLCHAIN=auto` or Go >= 1.27.2 locally |
| govulncheck v1.7.0 (last that runs on 1.25) | v1.9.0 (needs Go >= 1.26) | 2026-10-11 | Must run after the toolchain bump |
| setup-go `go-version: '1.25.4'` literal | `go-version-file: go.mod` | setup-go v6+ honours `toolchain` then `go` directive | CI tracks `go.mod` automatically (v1.0 D-16) |

### API surface and version jump (verified for every SDK method this repo calls)

| Sub-client.Method | HTTP | b5 api-version | v7 api-version |
|---|---|---|---|
| workitemtracking.UpdateWorkItem | PATCH | 5.1 | 7.1-preview.3 |
| workitemtracking.GetWorkItems / GetWorkItem | GET | 5.1 | 7.1-preview.3 |
| workitemtracking.QueryByWiql | **POST** (read) | 5.1 | 7.1-preview.2 |
| workitemtracking.GetClassificationNode | GET | 5.1 | 7.1-preview.2 |
| workitemtracking.GetAttachmentContent | GET | 5.1 | 7.1-preview.3 |
| git.CreatePullRequestReviewer | PUT | 5.1 | 7.1-preview.1 |
| git.UpdatePullRequest | PATCH | 5.1 | 7.1-preview.1 |
| git.GetBranches / GetPullRequest / GetPullRequestsByProject / GetRefs / GetRepositories | GET | 5.1 | 7.1-preview.1 |
| build.QueueBuild | **POST** (write) | 5.1 | 7.1-preview.7 |
| build.GetBuilds / GetDefinition / GetDefinitions | GET | 5.1 | 7.1-preview.7 |
| release.UpdateReleaseApproval | PATCH | 5.1 | 7.1-preview.3 |
| release.GetApprovals | GET | 5.1 | 7.1-preview.3 |
| release.GetReleaseDefinitions | GET | 5.1 | 7.1-preview.4 |
| release.GetReleases | GET | 5.1 | 7.1-preview.8 |
| location.GetConnectionData | GET | 5.1-preview.1 | 7.1-preview.1 |
| (startup) Client.GetResourceAreas | GET | 5.1-preview.1 | 5.1-preview.1 |

[VERIFIED: `Send(ctx, http.Method..., "<version>"` literals extracted from both module-cache copies this session]

- Every `XArgs` struct this repo uses is identical between b5 and v7, except `build.QueueBuildArgs` gaining an optional `DefinitionId *int` (unused) [VERIFIED: struct diff].
- Response models used are additive only: `git.GitPullRequest.HasMultipleMergeBases`, `release.Release.CreatedFor`, `build.Build.AppendCommitMessageToRunName`/`TemplateParameters`, `git.GitRepository.IsDisabled`/`IsInMaintenance`, `git.IdentityRefWithVote.HasDeclined`/`IsFlagged` [VERIFIED: struct diff]. Wire-shape drift at 7.1 can therefore only show up live (D-15).
- PR completion sends `Status: completed`, `LastMergeSourceCommit`, `CompletionOptions{DeleteSourceBranch: false, MergeStrategy: NoFastForward}` (`internal/ado/pullrequests.go:211-262`); release approve/reject PATCH `Status` and `Comments` (`internal/ado/releases.go:322-370`). No documented 7.1 change to either body is known; both stay dry-run in the smoke (D-13), so their pre-reads (`GetPullRequest`, `GetApprovals`) are what the smoke exercises live.
- PR comments/threads and work-item comments have **no call site** in Phase 8; the `AddComment` vs `AddWorkItemComment` (Markdown) question belongs to Phase 14.

**Deprecated/outdated:**
- `connection.Timeout` as the per-call budget: bypassed by `WithHTTPClient`; move the budget to `http.Client.Timeout`. `newConnection` and `TestNewClientSetsTimeout` (`internal/ado/client_test.go:18-50`) are rewritten accordingly.
- STACK.md's "a 429 on a write maps to exit 4" and "delete `X-TFS-Session`": both superseded (D-02; Pitfall 1).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | An org whose primary URL is legacy returns `LocationUrl`s of the form `https://{org}.visualstudio.com/` and `https://{org}.vsrm.visualstudio.com/` [ASSUMED] | Pattern 3 | A legitimate host is refused at startup (exit 3). Safe failure; the smoke run surfaces it. The user's org is dev.azure.com-primary (A4), so low impact |
| A2 | ADO does not currently deactivate `7.1-preview.N` requests despite the deprecation policy [ASSUMED; indirect evidence in Pitfall 9] | Pitfall 9 | A v7 call fails live; D-15 makes it a Phase 9 blocker |
| A3 | Work item attachment content is served from the org's own host, not redirected to a blob/CDN host [ASSUMED] | Open Question 8 | TUI attachment download is refused by the host rule after the upgrade; the TUI checklist row catches it |
| A4 | The user's org is dev.azure.com-primary and `org_url` already matches D-12 (`https://dev.azure.com/windstream`) [ASSUMED from g-01-1 stderr and CONTEXT's vsrm note] | Runtime State Inventory | Startup exits 3 with a named `org_url` error until config is corrected |
| A5 | A well-formed but rejected PAT gets 401 on resource-area discovery [CITED: v1.0 02-01-SUMMARY observation; MEDIUM] | Pitfall 3 | Only affects how SC2 is phrased; the read/write-path test stands either way |
| A6 | A 429 from ADO carries `Retry-After` [ASSUMED; RFC 6585 says MAY, MS docs do not state it for 429] | Pattern 5 | Without it, D-04 retries immediately (3 attempts back to back), then fails `rate_limit`. Bounded and honest |
| A7 | Org names are ASCII letters, digits and hyphens, so a D-12 pattern like `^[a-z0-9][a-z0-9-]*$` (case-insensitive) admits every real org [ASSUMED] | Open Question 6 | An unusual but valid org name fails validation (exit 3). Prefer structural parsing (one path segment, one host label) over a character class to avoid this |

## Open Questions (RESOLVED)

1. **SC2 and the startup path.** RESOLVED: SC2 says a rejected PAT "still" exits with the auth code. Today a fully rejected PAT exits 3 at startup (Pitfall 3) and 2 on any read or write where the 401 is classified. "Still" can only refer to the second, so the test proves a real v7 401 yields `auth` on both the read path (`Failure.Kind`) and the write path (`failed` + `FailureAuth`), which `exitCodeFor` and `writeExitCodeFor` map to 2. The startup exit code is unchanged pre-existing behavior; changing it would alter the published exit-code contract and is out of scope. Record it in 08-SMOKE.md if the user runs a bad-PAT check, and note it in BACKLOG BL-02 if not already listed.
2. **Delete `X-TFS-Session`?** RESOLVED: No. The race is unreachable with Go's canonical header keys (Pitfall 1, proven with `-race`). An optional regression test pins `SessionId` unchanged.
3. **"Every LocationUrl" at startup.** RESOLVED: check the `LocationUrl` of each area a sub-client is built on (wit, git, build, release, work) plus `BaseUrl` for root and location. A live anonymous probe of a dev.azure.com org returned 239 areas, all on `*.dev.azure.com/{org}/`, so the wider reading would also pass there; the narrow reading avoids turning an unused area on a legacy-primary org (A1) into a startup outage, and still checks every host a credential can reach.
4. **Where the read retry lives.** RESOLVED: embedding decorators over the four narrow interfaces in `internal/ado/retry.go` (Pattern 5), plus the identity func. Structural test forbids overriding the five write methods.
5. **Cap on a single honoured wait.** RESOLVED: no separate cap. The request deadline is the cap (D-01), and every CLI/TUI call has one (`fetchctx`, plus `request_timeout` 30s default per `internal/config/config.go:106-110`). The only unbounded case is startup discovery with `request_timeout: 0`, which is unbounded today too. A fixed cap would be the "invented threshold" D-04 rules out.
6. **`{org}` parsing for D-12.** RESOLVED: parse with `net/url`; require scheme `https`, no userinfo, no port, no query or fragment; accept host `dev.azure.com` with exactly one non-empty path segment (optional trailing slash), or host `<label>.visualstudio.com` with an empty or `/` path. The org is that segment or label, compared lowercase. Return `&ConfigError{Field: "org_url", Message: ...}`. All existing test fixtures (`https://dev.azure.com/fixture`, `/example`, `/fabrikam`) pass.
7. **Classification of a per-request host refusal on a read.** RESOLVED: `FailureOther` (it is neither auth nor timeout), with the refused host in the error text. The `Failure` JSON `err` string is the error's `Error()`, which for `*url.Error` also contains the request URL; transport errors already do this today (g-01-1 shows `Options "https://dev.azure.com/windstream/_apis": dial tcp ...`), and the PAT is never part of a URL.
8. **TUI checklist scope.** RESOLVED: include one attachment download (it is the most likely path to hit an unexpected redirect host, A3, and it exercises the injected client and `hc.Timeout` on body reads). Exclude the eager-identity recovery path: it needs a controlled `GetConnectionData` failure that a manual run cannot produce (g-01-1 showed Wi-Fi-off fails discovery first); carry it forward in BACKLOG.
9. **Counters for the summary line.** RESOLVED: the transport counts waits and total wait; the retry decorators count retries on an `atomic.Int64` owned by `ado.Client`; `main.run` reads both after `dispatch` returns and prints one line when either is non-zero. Printing it on failed runs too is the single simplest rule and never misleads; D-05's success case is a subset.
10. **`result.go` line references.** RESOLVED: update to v7 `client.go:375, :412` (pointer) and `:398, :419` (value); also update the `connection.Timeout` mention at `result.go:182-185` and the result_test.go comments that cite b5 lines.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build, tests | Yes (local go1.27.1; go1.27.2 auto-downloaded via `GOTOOLCHAIN=auto`) | go1.27.2 in module cache | -- |
| proxy.golang.org, sum.golang.org | `go get`, `go run ...@v1.9.0` | Yes | -- | -- |
| vuln.go.dev | govulncheck | Yes (scan ran this session) | -- | -- |
| GitHub Actions `actions/setup-go` | CI | Yes, already SHA-pinned (v7.0.0) | supports `go-version-file` | -- |
| `jq` | smoke script | Yes | 1.8.2 | `python3 -c 'import json'` |
| bash | smoke script | Yes | `/bin/bash` 3.2.57 | Write to 3.2 syntax |
| `shellcheck` | optional script lint | No | -- | `bash -n scripts/smoke.sh` |
| `gh` | optional CI run check | Yes | 2.100.0 | GitHub web UI |
| Live ADO org + valid PAT + scratch work item + dry-run targets | PLAT-05 | User-held, not available to the agent | -- | None: `checkpoint:human-action` |
| Phase 7 verification | Execution start (CONTEXT) | Pending (`07-VERIFICATION.md` status `human_needed`) | -- | None: plans may be written now; execution waits |

**Missing dependencies with no fallback:** live ADO access for PLAT-05 (human-run by design); Phase 7 sign-off before execution.

**Missing dependencies with fallback:** shellcheck (use `bash -n`).

## Validation Architecture

`workflow.nyquist_validation` is absent from `.planning/config.json`, so this section is enabled.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `testing/synctest` on go1.27.2; `go/ast` for structural tests |
| Config file | none (CI: `.github/workflows/ci.yml`) |
| Quick run command | `go test ./internal/transport/... ./internal/ado/... ./internal/guard/... ./internal/config/... ./cmd/adoBizInt/...` |
| Full suite command | `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)" && go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` |
| Current runtime | `go test -race ./...` about 2s per package on the scratch copy; synctest tests add no wall time |

### Phase Requirements to Test Map
| Req / SC | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PLAT-02 SC1 | One SDK module, v7 only; no b5 import | smoke (shell) | `test "$(go list -m all \| grep -c 'microsoft/azure-devops-go-api')" = 1 && go list -m github.com/microsoft/azure-devops-go-api/azuredevops/v7` | n/a (command) |
| PLAT-02 SC1 | Toolchain and CI pin | smoke (shell) | `grep -qx 'go 1.27.2' go.mod && grep -q 'go-version-file: go.mod' .github/workflows/ci.yml` | n/a |
| PLAT-02 SC1 | govulncheck clean, gate in CI | integration | `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` (exit 0) plus a green CI run on push | n/a; manual check of CI run |
| PLAT-02 SC2 | Real v7 401 (value and pointer forms) classifies `auth` | unit (real SDK, fake base RT) | `go test ./internal/ado -run 'TestV7.*Auth' -count=1` | Wave 0 (`internal/ado/v7_test.go`) |
| PLAT-02 SC2 | A read hit by a v7 401 records `Failure.Kind == "auth"`; a write's v7 401 is `FailureAuth` | unit | same file, `-run 'TestV7.*(Read\|Write)Auth'`; mapping to exit 2 already pinned by `TestExitCodeFor` (`cmd/adoBizInt/main_test.go:25`) | Wave 0 |
| PLAT-03 SC3 | `Retry-After` (seconds, HTTP-date) and `X-RateLimit-Delay` on a 200 delay the next request; max of both; `Remaining` alone does nothing | unit (synctest) | `go test ./internal/transport -run 'TestPace' -count=1` | Wave 0 (`internal/transport/transport_test.go`) |
| PLAT-03 SC3 | Wait longer than deadline fails fast, base not called, classifies `rate_limit` with D-08 text; `hc.Timeout` alone is a deadline | unit | `go test ./internal/transport -run 'TestFailFast'` and `go test ./internal/ado -run 'TestClassifyFailure'` (new rows) | Wave 0 / extend existing |
| PLAT-03 SC3 | Read 429 retried at most 2 times; fail-fast not retried; cancelled ctx not retried; exhaustion message | unit | `go test ./internal/ado -run 'TestRetry' -count=1` | Wave 0 (`internal/ado/retry_test.go`) |
| PLAT-03 SC3 | Write 429 never retried and exits 1 | unit + structural | `go test ./internal/ado -run 'TestRetryNeverWraps'` (AST: no write-method overrides) and `go test ./internal/guard -run 'TestRunClassifiesOutcomes'` (add a 429 row -> `failed`, 0 audit lines) | Wave 0 / extend |
| PLAT-03 (D-02) | Not-sent write with a cancelled parent is `failed`, unaudited; redirect-hop refusal is `unknown`, audited | unit | `go test ./internal/guard -run 'TestRunNotSent'` | Wave 0 |
| PLAT-03 SC4 | Host rule table: allowed and refused hosts, paths, case, port, scheme, look-alikes, traversal | unit (table) | `go test ./internal/transport -run 'TestAllowed'` | Wave 0 |
| PLAT-03 SC4 | Refused request never reaches the base RT; PAT absent from error text and log | unit | `go test ./internal/transport -run 'TestRefuse'` | Wave 0 |
| PLAT-03 SC4 | Startup refuses a bad `LocationUrl` with `ConfigError{Field: "org_url"}` naming the host, before any sub-client is built | unit (real SDK, fake RT) | `go test ./internal/ado -run 'TestNewClient.*LocationURL'` | Wave 0 |
| PLAT-03 SC4 (D-11) | Every sub-client built via `NewClientWithOptions` + `WithHTTPClient`; no `GetClientByUrl`, `GetClientByResourceAreaId`, typed `NewClient`, `azuredevops.NewClient`; `TestNoRawHTTPInADO` still passes | structural (`go/ast`) | `go test ./internal/ado -run 'TestSubClientsUseInjectedClient\|TestNoRawHTTPInADO'` | Wave 0 (`internal/ado/construct_test.go`) |
| PLAT-03 (D-12) | `org_url` shapes accepted/rejected | unit (table) | `go test ./internal/config -run 'TestValidateOrgURL'` | Wave 0 (extend `config_test.go`) |
| PLAT-03 (D-05/D-06) | One stderr line when counters > 0; nothing otherwise; stdout untouched | unit | `go test ./cmd/adoBizInt -run 'TestThrottleSummary'` | Wave 0 (new file; do not edit Phase 7-locked test files) |
| PLAT-05 SC5 | One live read per sub-client, one dry run per write verb, recorded | manual-only | `bash -n scripts/smoke.sh`; user runs `scripts/smoke.sh`, pastes table into `08-SMOKE.md` | Wave 0 (script) |
| PLAT-05 (D-13) | CLI `set-state --confirm` on scratch item; TUI Task close with effort; TUI attachment download | manual-only | TUI checklist in `08-SMOKE.md` | Wave 0 (checklist) |

Manual-only justification: PLAT-05 exists precisely because fakes cannot observe 7.1 wire behaviour; it needs the user's credential and org (no credentials in CI, v1.0 D-17).

### Sampling Rate
- **Per task commit:** `go test` on the touched packages (quick run command above).
- **Per wave merge:** `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"`.
- **Phase gate:** full suite including govulncheck, a green CI run on the pushed `dev` branch, and `08-SMOKE.md` filled with no v7/pacer/host-rule regression before `/gsd-verify-work` and before Phase 9.

### Wave 0 Gaps
- [ ] `internal/transport/transport_test.go`: host rule table, synctest pacing, fail-fast, hold-cancel, redirect hop, refusal-before-send, PAT-never-logged (capture `slog` as `cmd/adoBizInt/patleak_test.go` does)
- [ ] `internal/ado/v7_test.go`: shared `rtFunc`, JSON response helper, location-list fixture (must include the resource-areas location `e81700f7-3be2-46de-8624-2eb35882fcaa` and each data route used), unique org per test; 401 value and pointer forms; `NewClient` discovery and startup refusal
- [ ] `internal/ado/retry_test.go`: decorator behaviour plus the no-write-override AST assertion
- [ ] `internal/ado/construct_test.go`: D-11 AST assertion, non-vacuous (fails if `client.go` is not parsed or no `NewClientWithOptions` call is found)
- [ ] `internal/guard`: new test function for not-sent and redirect-hop outcomes; add a 429 row to the outcome table
- [ ] `internal/config/config_test.go`: `org_url` shape table
- [ ] `cmd/adoBizInt/throttle_test.go`: summary line
- [ ] `scripts/smoke.sh` and the TUI checklist (in `08-SMOKE.md` or beside the script)
- [ ] Rewrite `internal/ado/client_test.go` `TestNewClientSetsTimeout` to assert the injected client's `Timeout` (it currently asserts `connection.Timeout`, which v7 construction bypasses)

## Security Domain

`security_enforcement` is absent from config, so this section is enabled.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | PAT as Basic auth built by the SDK (`CreateBasicAuthHeaderValue`); never read `AuthorizationString` outside the SDK (`TestNoRawHTTPInADO`); never log it |
| V3 Session Management | no | Stateless PAT; the SDK's `X-TFS-Session` correlation id is not a security session |
| V4 Access Control | partial | Credential scope binding: the PAT may only reach hosts bound to `{org}` (D-09/D-10). Agent tiers are Phase 13 |
| V5 Input Validation | yes | `org_url` shape (D-12); server-supplied `LocationUrl` (D-10a); `Retry-After`/`X-RateLimit-Delay` parsed with stdlib and bounded by the deadline (D-01) |
| V6 Cryptography | yes (TLS only) | stdlib TLS via `http.DefaultTransport`; https required for `org_url` and every hop; nothing hand-rolled |
| V8 Data Protection | yes | PAT never in logs, errors, stdout or the summary line; transport logs only `X-RateLimit-*`/`Retry-After` values |
| V9 Communications | yes | https-only host allowlist checked on every request and redirect hop |
| V14 Configuration | yes | Versions pinned and sumdb-verified; `govulncheck` gate in CI; actions SHA-pinned (unchanged) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Server-supplied `LocationUrl` points the credential at a foreign host | Information Disclosure | Startup check on used areas (D-10a) and per-request rule (D-10b); refusal before `base.RoundTrip` |
| Redirect to another org on `dev.azure.com`, or to a subdomain, carries `Authorization` (stdlib forwards to same host and subdomains) | Information Disclosure | Per-hop host rule with org path binding; redirect-hop refusals typed so writes stay `unknown` |
| `org_url` over `http://` sends the PAT in cleartext | Information Disclosure | D-12 requires https; the rule refuses any non-https hop |
| Path traversal `/{org}/../other/` on an allowed host | Elevation of Privilege | `path.Clean` before the prefix check |
| Huge or hostile `Retry-After` stalls the tool | Denial of Service | D-01 fail-fast against the request deadline; every CLI/TUI call has a deadline |
| Server-controlled `X-RateLimit-Resource` text reaches a terminal | Tampering (terminal injection) | Log file only (slog quotes values); the stderr summary carries numbers only |
| Not-sent write reported `unknown`, or a sent write reported `failed` | Repudiation / Integrity | `ErrNotSent` only for first-hop refusals and pacer holds; `guard.classify` checks it first; audit unchanged for `unknown` |
| Compromised or vulnerable dependency | Tampering | No new modules; sumdb-verified pins; govulncheck fails CI on reachable vulns |

## Sources

### Primary (HIGH confidence)
- Module cache source, `github.com/microsoft/azure-devops-go-api/azuredevops/v7@v7.1.0`: `client.go` (NewClient/NewClientWithOptions 53-82, SendRequest 93-99, Send and SessionId 101-139, CreateRequestMessage 155-207, location cache 209-272, transformRouteTemplate 281-302, UnwrapError 372-420, GetResourceAreas 422-433), `client_options.go` 1-15, `connection.go` 1-157, `versionnegotiation.go`, per-area `client.go` (`ResourceAreaId`, `NewClient`, `Send` api-version literals), `models.go` (`WrappedError`, `ResourceAreaInfo`, `ApiResourceLocation`); diffed against `azuredevops@v1.0.0-b5`
- Go 1.27.2 source (`golang.org/toolchain@v0.0.1-go1.27.2`): `net/http/client.go` (RoundTripper contract 113-143, `setRequestCancel` 327-381, `redirectBehavior` 514-542, `shouldCopyHeaderOnRedirect`/`isDomainOrSubdomain` 1017-1062), `net/http/request.go` (`Response` 325-328, `GetBody` 933-955), `net/textproto/reader.go:563`, `net/http/internal/http2/transport.go:2222`
- Scratch-copy verification this session: import swap + `go mod tidy` + build/vet/`test -race`/gofmt; govulncheck v1.9.0 at `go 1.27.0` (8 findings) and `go 1.27.2` (clean); probes for v7 401 classification, synctest pacing, client-timeout deadline, not-sent wrapping, redirect-hop marking, SessionId race, resource-area parsing
- Repository files read this session: `internal/ado/{client,result,attachments,attachments_test,interfaces,preread,pullrequests,releases}.go`, `internal/guard/guard.go`, `internal/guard/guard_test.go`, `internal/ado/result_test.go`, `cmd/adoBizInt/{main,dispatch,exit,write}.go`, `internal/config/config.go`, `internal/fetchctx/fetchctx.go`, `internal/logging/logging.go`, `.github/workflows/ci.yml`, `go.mod`
- proxy.golang.org (`@latest`, `.info`, `.mod`), sum.golang.org lookups, go.dev/dl JSON, vuln.go.dev `GO-2026-5024`

### Secondary (MEDIUM confidence)
- Live anonymous `GET https://dev.azure.com/dnceng-public/_apis/resourceAreas` (2026-10-19): 239 areas, host and path pattern, used-area URLs, response headers (HTTP/2, `x-tfs-session`, `x-ratelimit-cost`)
- v1.0 phase records: `.planning/milestones/v1.0-phases/02-honest-reads/02-01-SUMMARY.md` (A1 live 401 evidence, startup auth deviation), `.planning/debug/g-01-1-offline-startup-block.md`, `.planning/BACKLOG.md` BL-02

### Tertiary (LOW confidence per classify-confidence seam; official sources fetched via webfetch)
- https://learn.microsoft.com/en-us/azure/devops/integrate/concepts/rate-limits (updated 2026-07-31): header names, units, Delay semantics, 200-before-429
- https://learn.microsoft.com/en-us/azure/devops/extend/develop/work-with-urls (updated 2026-07-23): resource areas, no-auth discovery, primary URL rule, vsrm host
- https://learn.microsoft.com/en-us/azure/devops/integrate/concepts/rest-api-versioning (updated 2026-07-23): preview deprecation after 12 weeks
- https://github.com/actions/setup-go README (main): `go-version-file`, toolchain-directive precedence (v6+)
- https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck (v1.9.0): toolchain-on-PATH rule, exit codes

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH. Versions, checksums and the vuln result were verified by running the tools this session.
- Architecture: HIGH for SDK and stdlib mechanics (read from source and proven by probes); MEDIUM for the startup-check scope and retry placement, which are recommendations within discretion.
- Pitfalls: HIGH for 1-8, 12-16 (source or probe evidence); MEDIUM for 9-11 (official docs plus indirect evidence).

**Research date:** 2026-10-19
**Valid until:** 2026-10-26 for the toolchain pin and govulncheck result (a new Go security release changes the answer; re-run the gate at implementation time). 2026-11-18 for everything else.

