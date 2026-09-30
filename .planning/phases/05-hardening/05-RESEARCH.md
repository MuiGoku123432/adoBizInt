# Phase 5: Hardening - Research

**Researched:** 2026-09-30
**Domain:** Go security hardening: credential scoping, filesystem containment, query-injection escaping, nil-safety, log permissions, GitHub Actions CI
**Confidence:** HIGH (nearly every claim comes from reading repo source, the vendored SDK source, or the installed Go toolchain docs this session)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Attachment fetch and write (HARD-01, HARD-02)

- **D-01:** `DownloadAttachment` takes an attachment identifier (`uuid.UUID`),
  not a URL. It calls the SDK's `GetAttachmentContent` through `c.connection`,
  and the hand-rolled `http.NewRequestWithContext` + `AuthorizationString`
  header + `http.DefaultClient` path at `internal/ado/workitems.go:612-643` is
  deleted outright. The PAT can then only reach the configured organization
  host structurally — there is no URL parameter left to abuse. This also
  inherits the Phase 2 `connection.Timeout`, which resolves the TRUST-04
  exclusion Phase 2 deferred to this phase. Adds one entry
  (`GetAttachmentContent`) to the Phase 1 `workitemAPI` interface; the entry is
  called, so the Phase 4 SC4 no-uncalled-interface-entry criterion holds.
  — **Reversibility:** costly — undoing means reinstating a raw HTTP path and
  re-deriving the header, which is the vulnerability itself.

- **D-02:** Phase 5 fixes the internal path and its single existing caller
  (`internal/ui/workitems/detail.go:191`) only. No CLI attachment download verb
  ships in this phase. Phase 3 defined the read CLI surface and deliberately
  excluded attachments; adding a verb here is a new capability, and HARD-01 and
  HARD-02 are both satisfied without it. `--output-dir` containment gets
  designed when the verb is actually planned.

- **D-03:** Path containment is enforced in `internal/ado`, not at the call
  site. The signature becomes `(ctx, id, destDir, name)`; the method itself does
  `filepath.Base`, rejects empty / `.` / `..` / absolute results, and performs a
  post-join prefix check against the resolved destination directory — all before
  any network call. Every future caller, including the deferred CLI verb,
  inherits the guard and cannot forget it. This is the same reasoning that put
  the Phase 4 write guards in a shared `runWrite` body rather than in six copies.
  — **Reversibility:** costly — the `destPath` → `destDir` + `name` signature
  change touches the call site and the interface.

- **D-04:** No download size cap in this phase. Attachments come from the
  operator's own ADO organization, the TUI download is human-initiated and
  one-at-a-time, so disk-fill is not a realistic path until the deferred CLI
  verb enables bulk iteration. The cap is deferred with the verb. This overrides
  the "size cap" wording in the ROADMAP's 05-01 plan sketch.

- **D-05:** `Attachment` gains an `ID` field (uuid string); `URL` is retained as
  informational only. Nothing in the code path consumes `URL` after D-01 — it
  stays for a human pasting it into a browser. `WorkItemDetail` is JSON-tagged
  but there is no `workitems show` verb in the dispatch table, so `Attachment`
  is not yet published output and this shape change is cheap now and expensive
  after such a verb ships.

- **D-06:** When the GUID cannot be parsed out of a relation URL, the attachment
  is not appended to `detail.Attachments` — the same rule as the existing
  `attachment.Name != ""` guard at `internal/ado/workitems.go:492`. It cannot be
  selected in the TUI, so it cannot fail at download time. No `Failure` is
  recorded; this matches the Phase 2 D-07 precedent of suppressing rather than
  naming a non-call anomaly.

- **D-07:** A containment rejection surfaces through the existing
  `AttachmentDownloadedMsg{Err: ...}` path as a plain error, indistinguishable
  from a network failure. A server-crafted traversal name is an attack, not a
  user mistake, and the operator's action is the same either way.

- **D-08:** Containment is verified by extracting a pure function
  `(name, destDir) -> (path, error)` and table-testing the hostile set directly:
  `../`, `..\`, absolute paths, empty, `.`, `..`, embedded NUL, all-separator
  names. Plus one integration test through the Phase 1 fake proving
  `DownloadAttachment` refuses before any `GetAttachmentContent` call is made.
  This is a two-use extraction (download now, deferred CLI verb later), short of
  the CONVENTIONS.md rule of three — justified explicitly because the guard is
  the feature.

#### WIQL and project validation (HARD-04)

- **D-09:** An unknown `--project` value is rejected upfront in `cmd/` right
  after `config.Load`, before any read runs, and exits `3`. Nothing ran, so
  exit 3 is the accurate fact and matches the Phase 3 D-08 rationale. Fail
  closed: the whole invocation is rejected even if two of three supplied
  projects were valid. Rejected the alternative of recording a per-project
  `Failure` and exiting `1` — an agent that retries a `1` expecting transience
  would loop forever against a permanently bogus project name, which is the
  exact trap D-07 cites for auth.
  — **Reversibility:** one-way — the exit-code vocabulary is a published
  contract an agent branches on (Phase 3 D-06/D-07/D-08); moving this mapping
  later breaks callers already written against it.

- **D-10:** Both WIQL interpolation sites are escaped with
  `strings.ReplaceAll(project, "'", "''")` — `internal/ado/workitems.go:149` and
  `internal/ado/dashboard.go:142` — matching the escaping already applied to
  iteration paths a few lines above each. The allow-list does not remove this:
  it is a `cmd/` policy check the TUI never runs, and `internal/ado` is a library
  that must not assume its caller validated anything. Escaping is the structural
  guarantee; validation is the policy layer; they live in different packages on
  purpose.

- **D-11:** The interpolated query is no longer logged. `internal/ado/workitems.go`
  and `internal/ado/dashboard.go` log the project name, iteration filter and
  result count as structured `slog` attributes instead of the assembled query
  string. This closes the H5 remediation item 3 and part of MED2, and it lands
  with the escaping change rather than with the logging work.

- **D-12:** Escaping is kept fixed by a `go/ast` test that walks `internal/ado`,
  finds every string literal containing `FROM WorkItems`, and asserts that no
  concatenation operand in that expression is a bare identifier — every
  interpolated value must be a call result. A third WIQL site added later fails
  the test, and a prose comment cannot defeat it. This follows the Phase 4
  precedent established after three Phase 3 incidents where a grep-negative
  criterion was satisfied and then defeated by an explanatory comment.

#### Nil guards (HARD-03)

- **D-13:** HARD-03 covers the two MED4 sites only — `internal/ado/pipelines.go:171`
  and `internal/ado/releases.go:125` — each gaining `|| d.Id == nil` at the
  collection guard. Plus a `go/ast` test pinning the specific shape: any loop
  that collects SDK definitions into a slice and later dereferences `.Id` must
  guard both `.Name` and `.Id` at the collection point. Rejected a repo-wide
  sweep of all 423 deref expressions in `internal/ado`: a flow-insensitive
  checker cannot see the enclosing guard, so most hits would be false positives,
  and CONCERNS.md itself calls these two sites outliers rather than a systemic
  pattern. `go vet`'s `nilness` analyzer does not catch this class — SDK pointer
  fields are not provably nil, which is why `go vet ./...` passes clean today.
  The `handlePanic` recover in `cmd/adoBizInt/crash.go` remains the backstop for
  everything else.

- **D-14:** When the guard fires on a nil `Id` it does a silent `continue`,
  matching the existing `Name == nil` guard immediately above it. A definition
  the server returned without an `Id` is malformed server data, not a failed
  call, and Phase 2 D-07 already set the precedent of suppressing rather than
  naming a non-call anomaly.

#### CI (HARD-05)

- **D-15:** CI is exactly the ROADMAP SC5 set and nothing more: GitHub Actions
  on push and PR running `go build ./...`, `go vet ./...`, `go test -race ./...`,
  and a `gofmt -l` check that fails on any output. `govulncheck` and
  `golangci-lint` are deferred — `golangci-lint` on a 7,600-line brownfield repo
  with this comment-heavy house style produces a triage backlog before the gate
  can be made blocking, and that backlog is not what this phase is for.

- **D-16:** The workflow pins `go-version: 1.25.4`, matching `go.mod` exactly.
  CI failures then reflect the code rather than toolchain drift the developer
  did not choose, and the one-line bump on a Go upgrade becomes an explicit
  commit.

- **D-17:** CI gets no ADO credentials, and that absence is itself the
  assertion. Every test runs against the Phase 1 fake seam; a test that reaches
  real ADO fails on a missing PAT, which is the correct signal that a test
  escaped the seam. Rejected adding explicit skip guards — that converts the
  escape signal into silence, which is the failure mode this project exists to
  avoid. Planning must confirm no current test reads `ADO_PAT`.

#### Log hardening (HARD-06)

- **D-18:** Exactly SC6 lands here: `0600` file, `0700` directory, and the
  PAT-absence regression test. Configurable log level, rotation and size cap are
  CONCERNS.md MED2 items the ROADMAP deliberately did not promote to a success
  criterion — deferred. (The interpolated-WIQL log line is already handled by
  D-11.)

- **D-19:** The regression test points the logger at an in-memory buffer, sets a
  distinctive sentinel PAT, drives a representative read, a write, and a forced
  panic through the Phase 1 fake, and asserts the sentinel appears nowhere in
  the captured bytes. The panic path is included because it is where HARD-06
  originated (Phase 3 threat T-03-15): `handlePanic` at
  `cmd/adoBizInt/crash.go:96` deliberately writes the full `debug.Stack()` to
  the log file. Recorded honestly for planning: Go renders stack frame arguments
  as hex words rather than dereferenced strings, so the stack is unlikely to leak
  the PAT literally — the realistic regression path this test defends is a future
  `slog` attribute, not the stack.

- **D-20:** `logging.Init` explicitly `chmod`s both the log file to `0600` and
  the directory to `0700` on every call, not just at creation. `os.OpenFile`'s
  mode and `MkdirAll`'s mode apply only at creation, so a mode-only change would
  fix nothing for any existing install. The directory also holds the
  PAT-bearing config file, which makes tightening it the higher-value half.

- **D-21:** A failed `chmod` returns the error from `Init`. `cmd/adoBizInt/main.go:17-19`
  already treats an `Init` failure as a non-fatal warning and falls back to
  `slog.Default()` on stderr, so the existing policy applies unchanged and a
  `--json` consumer discards the diagnostics. No new failure mode is introduced.

### Claude's Discretion

None — every question in this discussion was answered with an explicit choice.

### Deferred Ideas (OUT OF SCOPE)

- **CLI attachment download verb** (`workitems download-attachment`) with
  `--output-dir` containment and a config-tunable byte cap. Deferred out of
  Phase 5 as a new capability (D-02, D-04). Note it needs an output contract for
  a verb that writes a file rather than emitting JSON — a shape the Phase 3
  envelope does not cover.
- **`govulncheck ./...` in CI.** CONCERNS.md MED5 asks for it specifically
  before scoping an SDK migration off the 2020 beta. Deferred from D-15; pairs
  naturally with the v2 SDK-upgrade item.
- **`golangci-lint` with a `.golangci.yml`.** Deferred from D-15 — needs a
  triage pass over the existing 7,600 lines before the gate can be blocking.
- **Configurable log level defaulting to Info** (CONCERNS.md MED2). Deferred
  from D-18.
- **Log rotation or a size cap** (CONCERNS.md MED2). Deferred from D-18; would
  need a new dependency or hand-rolled logic, against the PROJECT.md constraint.
- **Attachment download byte cap.** Deferred with the CLI verb (D-04).
- **Routing TUI writes through `runWrite`** (BACKLOG.md BL-01 part 2) — already
  carried in STATE.md as needing its own discuss-phase. Not touched here.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| HARD-01 | Attachment download takes an attachment identifier, not a caller-supplied URL, so the PAT cannot be sent to an arbitrary host | SDK `GetAttachmentContent` signature and routing verified in vendored source (§ SDK facts); relation-URL GUID extraction format from MS docs; structural "no net/http in internal/ado" test (Pattern 5) |
| HARD-02 | Attachment writes are contained to the intended directory | `filepath.Base` / `IsLocal` behaviour on the full D-08 hostile set probed this session (Pitfall 2 table); pure-function shape (Pattern 2) |
| HARD-03 | Nil-dereference guards on the ADO SDK pointer fields identified in CONCERNS.md MED4 | Both sites located at current line numbers; go/ast prototype run this session finds exactly those two loops package-wide (Pattern 4) |
| HARD-04 | WIQL project interpolation is escaped and validated against the configured project list | Both interpolation sites located; D-12 go/ast prototype run this session shows `iterationFilter` also trips the rule (Pitfall 4); `--project` is parsed per-verb, not in main (Pitfall 5); ~30 existing tests pass `--project` (Pitfall 6) |
| HARD-05 | CI runs build, vet, test with race detection, and gofmt on every push | Remote is GitHub; no `.github/` today; full gate green under Go 1.25.4 and 1.27.1 locally; suite passes with no ADO env and an empty `$HOME`; action versions and SHAs fetched |
| HARD-06 | The log file is opened `0600` and its directory `0700`, and a regression test asserts the PAT string never appears in captured log output | `logging.Init` source read; `Logger()` falls back to `slog.Default()` when uninitialised (the capture seam); `loggingReady` gate; base64 form of the PAT (Pitfall 8) |
</phase_requirements>

## Project Constraints (from CLAUDE.md and organization instructions)

There is no project `./CLAUDE.md` and no `.claude/skills` or `.agents/skills` directory `[VERIFIED: ls this session]`. These directives come from the user's global CLAUDE.md and the organization instructions:

- **Simplest code, YAGNI, no speculative generality.** No new interfaces, managers, or config knobs unless the task requires them.
- **Single-use code stays inline. Rule of three.** Two occurrences stay duplicated with a `// note:` comment, unless a locked decision says otherwise (D-08 is one such exception, recorded explicitly).
- **No new dependencies without stated justification.** This phase adds no module. `github.com/google/uuid` is already in `go.mod` (listed `// indirect`) and is already imported by `internal/ado/branches_test.go:10` and `internal/ado/pullrequests_test.go:10` `[VERIFIED: grep + go.mod read]`.
- **Files under 300 lines per module.** `internal/ado/workitems.go` is already 643 lines `[VERIFIED: wc -l]`. New attachment code should not make it longer (see Recommended Project Structure).
- **Follow existing folder and test conventions:** in-package closure fakes that `t.Fatalf` on an unexpected call, go/ast structural tests with a non-vacuity check, `withX(t, …)` helpers that restore package state in `t.Cleanup`.
- **Organization rule: "When generating SQL, always use parameterized queries — never string concatenation."** WIQL is a SQL-like query language, and this rule bears directly on D-10. See Open Question 1. **The planner must bring it up with the user.**
- **Organization rule: OWASP Top 10, least privilege.** For CI this means `permissions: contents: read`, no secrets, and actions pinned to commit SHAs.
- **Organization rule: never repeat secrets.** The PAT test uses a sentinel value, never a real token.
- **Gortex MCP tools** are preferred for code navigation. They were not available to this subagent, so the research used direct reads of repo and SDK source.

## Summary

Every fix in this phase is small, local, and uses the stdlib or an SDK method that is already vendored. No new module is needed. The attachment fix (HARD-01) removes code rather than adding it. The vendored SDK's `GetAttachmentContent` takes `Id *uuid.UUID`, sends the request through the same `azuredevops.Client` as every other call (same resolved org host, same `Authorization` header, same `connection.Timeout`), and returns the raw `resp.Body` as an `io.ReadCloser` for the caller to stream and close. It also maps any non-2xx status to an error. Once the raw `http` path is deleted, `internal/ado` has no `net/http` import and no reader of `AuthorizationString`, and a go/ast test can assert that structurally. The GUID has to be parsed from the path of the relation URL, because the relation's `attributes.id` is an integer revision ID, not the attachment GUID.

Path containment (HARD-02) is a pure function built from `filepath.Base` plus explicit rejects. A probe this session showed three behaviours the table test has to expect. First, `Base` turns `../../.ssh/authorized_keys` and `/etc/passwd` into safe child names rather than errors. Second, `Base("")` is `"."`. Third, on Linux and macOS `..\` is a literal filename, not a traversal. The table therefore has to assert "error OR direct child of destDir", or reject backslash explicitly.

Two findings change how HARD-04 has to be planned. First, the D-12 go/ast rule ("no bare-identifier concatenation operand") fails on `iterationFilter` as well as `project` at both sites. The prototype run this session confirmed it, so the iteration clause has to become a call expression. Second, `--project` is parsed inside each verb's `runRead` / `runWrite`, not in `main`, and about 30 existing cmd tests pass `--project` values (`P`, `Q`, `one`, `alpha`, `beta`, `gamma`) that no configured list contains. The allow-list therefore needs a configured-projects source that tests can set.

Separately, WIQL has a documented `@project` macro that resolves server-side from the project in the route. The SDK already sends that route value. Using it would remove the project interpolation entirely, which is the parameterized form the organization rule requires. It conflicts with locked D-10, so it goes to Open Question 1 for the user rather than being decided here.

**Primary recommendation:** Build five plans in three waves: attachments (HARD-01+02 together), nil guards, and CI in wave 1; WIQL plus project validation in wave 2; logging in wave 3. Every "must never happen" criterion is a go/ast test with a non-vacuity count, and every behaviour criterion is a fake-driven test that fails if an unexpected SDK call is made.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Credential scoping for attachment fetch | `internal/ado` (API client library) | vendored SDK `azuredevops.Client` | The SDK owns URL construction and the auth header. `internal/ado` must only pass an ID. |
| Filename containment | `internal/ado` (D-03) | TUI caller supplies `destDir` only | The guard lives in the shared body so every caller inherits it. |
| WIQL escaping | `internal/ado` (D-10) | — | The library must not trust callers, because the TUI never runs the cmd check. |
| Project allow-list | `cmd/adoBizInt` (D-09) | `internal/config` provides `cfg.Projects` | Policy belongs in the CLI layer, and exit code 3 is a cmd contract. |
| Nil guards on SDK data | `internal/ado` | `cmd` `handlePanic` backstop | Malformed server data is filtered where it is collected. |
| Log file permissions | `internal/logging` | `cmd` `initLogging` warning policy | `Init` owns file and directory creation. |
| PAT-absence regression | `cmd/adoBizInt` tests (panic path, dispatch) | `internal/ado` tests (optional) | The panic handler and the cfg-bearing code live in cmd. |
| CI gate | GitHub Actions (`.github/workflows/`) | — | The remote is GitHub (`git@github.com:MuiGoku123432/adoBizInt.git`) `[VERIFIED: git remote -v]`. |

## Standard Stack

### Core (all already present, nothing to install)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go toolchain | 1.25.4 (`go.mod`: `go 1.25.4`) | Language/runtime | `[VERIFIED: go.mod read; GOTOOLCHAIN=go1.25.4 go version → go1.25.4 darwin/arm64]` |
| `github.com/microsoft/azure-devops-go-api/azuredevops` | v1.0.0-b5 | `workitemtracking.Client.GetAttachmentContent` | Already the project SDK `[VERIFIED: go.mod]` |
| `github.com/google/uuid` | v1.1.1 | `uuid.UUID` / `uuid.Parse`. `UUID` implements `MarshalText`, so it JSON-encodes as a string | Required by the SDK's `GetAttachmentContentArgs.Id *uuid.UUID`. Already in the module graph `[VERIFIED: $GOMODCACHE/github.com/google/uuid@v1.1.1/marshal.go:10,17; uuid.go:43]` |
| stdlib `path/filepath`, `net/url`, `path`, `go/ast`, `go/parser`, `go/token`, `log/slog`, `os` | Go 1.25 | Containment, relation-URL parsing, structural tests, log capture, chmod | stdlib |

### Supporting (CI)

| Action | Version / SHA | Purpose |
|--------|---------------|---------|
| `actions/checkout` | v7.0.1, `3d3c42e5aac5ba805825da76410c181273ba90b1` (published 2026-07-20) | Check out the repo `[VERIFIED: gh api repos/actions/checkout/tags + releases/latest]` |
| `actions/setup-go` | v7.0.0, `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` (published 2026-07-16); runs on `node24`; `cache` input defaults to `true` | Install Go 1.25.4 and cache modules and the build cache `[VERIFIED: gh api repos/actions/setup-go/tags, releases/latest, action.yml]` |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `filepath.Base` + explicit rejects (D-03) | `os.OpenRoot(destDir)` + `root.Create(name)` (Go 1.24+) | `os.Root` also refuses symlink escapes. D-03 locks the lexical approach, and a symlink plant needs local write access to `~/.config/adoBizInt`, which is outside this threat model. Not recommended as an addition. Mention it in the deferred CLI verb's design. `[VERIFIED: go doc os.OpenRoot]` |
| `strings.ReplaceAll(project, "'", "''")` (D-10) | `[System.TeamProject] = @project` macro | Removes the interpolation entirely. See Open Question 1. |
| Package-level `configuredProjects` var | Add `Projects() []string` to the cmd `readClient`/`writeClient` interfaces (`*ado.Client` already has `func (c *Client) Projects() []string` at `client.go:149`) | The interface route changes both fakes and every fake constructor. The package var mirrors the existing `readBudget = cfg.FetchTimeout` precedent at `main.go:105`. **Use the package var.** |
| GitHub Actions | Azure Pipelines | The remote is GitHub, and D-15 locks Actions. |

**Installation:** none. `go mod tidy -diff` today already wants `github.com/google/uuid` and `github.com/atotto/clipboard` moved from `// indirect` to direct `[VERIFIED: go mod tidy -diff output]`. Running `go mod tidy` in plan 05-01 is optional hygiene: `go build` does not require it, and CI does not check it.

## Package Legitimacy Audit

No external packages are installed in this phase. The only newly-direct import in non-test code is `github.com/google/uuid` v1.1.1. It was already resolved in `go.mod`/`go.sum`, pinned by the SDK's own `go.mod`, and imported by two existing test files. No registry fetch or new code enters the module graph.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| github.com/google/uuid v1.1.1 | Go module proxy | already in go.sum | n/a | github.com/google/uuid | not re-checked (already a transitive dependency of the SDK) | Approved: no change to the module graph |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

The two GitHub Actions are pinned to commit SHAs (above). They are the official `actions/*` org repos.

## SDK facts (vendored source read this session)

`$GOMODCACHE/github.com/microsoft/azure-devops-go-api/azuredevops@v1.0.0-b5/workitemtracking/client.go:826-862`:

```go
// Downloads an attachment.
func (client *ClientImpl) GetAttachmentContent(ctx context.Context, args GetAttachmentContentArgs) (io.ReadCloser, error) {
	routeValues := make(map[string]string)
	if args.Project != nil && *args.Project != "" {
		routeValues["project"] = *args.Project
	}
	if args.Id == nil {
		return nil, &azuredevops.ArgumentNilError{ArgumentName: "args.Id"}
	}
	routeValues["id"] = (*args.Id).String()
	...
	resp, err := client.Client.Send(ctx, http.MethodGet, locationId, "5.1", routeValues, queryParams, nil, "", "application/octet-stream", nil)
	if err != nil {
		return nil, err
	}

	return resp.Body, err
}

// Arguments for the GetAttachmentContent function
type GetAttachmentContentArgs struct {
	// (required) Attachment ID
	Id *uuid.UUID
	// (optional) Project ID or project name
	Project *string
	// (optional) Name of the file
	FileName *string
	// (optional) If set to <c>true</c> always download attachment
	Download *bool
}
```

The interface method is declared at `client.go:61` as `GetAttachmentContent(context.Context, GetAttachmentContentArgs) (io.ReadCloser, error)`. `uuid` is `github.com/google/uuid` (`client.go:15`) `[VERIFIED]`.

Behaviour that matters for planning `[VERIFIED: azuredevops@v1.0.0-b5/client.go:53-73, 84-90, 146-170]`:
- **It streams.** It returns `resp.Body` directly and does not buffer. The caller must `defer body.Close()` and `io.Copy` into the file.
- **It uses `connection.Timeout`.** `NewClient` sets `client.Timeout = *connection.Timeout` on the `http.Client`. Per `go doc net/http.Client`: *"The timeout includes connection time, any redirects, and reading the response body. The timer remains running after Get, Head, Post, or Do return and will interrupt reading of the Response.Body."* `[CITED: go doc net/http.Client, Go 1.27.1 toolchain]`. The default `RequestTimeout` is `30 * time.Second` (`internal/config/config.go:87`) `[VERIFIED]`. See Pitfall 7.
- **Non-2xx is an error.** `SendRequest` sets `err = client.UnwrapError(resp)` when `resp.StatusCode < 200 || resp.StatusCode >= 300`. The existing `if resp.StatusCode != http.StatusOK` check therefore disappears along with `net/http`.
- **Auth and host.** `CreateRequestMessage` does `req.Header.Add(headerKeyAuthorization, client.authorization)`. The base URL is the one the SDK resolved for the work-item-tracking resource area from the configured org URL (`connection.go:59-74`), which is the same trust boundary as every other call in the app.
- **Redirects.** The SDK's `http.Client{}` uses the default `CheckRedirect`. Per `go doc net/http.Client`, sensitive headers such as `Authorization` "will be ignored when following a redirect to a domain that is not a subdomain match or exact match of the initial domain" `[CITED: go doc net/http.Client]`. A redirect to blob storage does not carry the PAT.
- **Auth string.** `NewPatConnection` → `CreateBasicAuthHeaderValue("", pat)` = `"Basic " + base64.StdEncoding.EncodeToString([]byte(":" + pat))` (`connection.go:18-53`) `[VERIFIED]`. The PAT test must search for this encoded form as well as the raw string (Pitfall 8).

**Relation URL format.** From the Microsoft REST docs example for an `AttachedFile` relation `[CITED: learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-items/update?view=azure-devops-rest-7.1]`:
```
"rel": "AttachedFile",
"url": "https://dev.azure.com/fabrikam/_apis/wit/attachments/098a279a-60b9-40a8-868b-b7fd00c0a439?fileName=Spec.txt",
```
and in the response:
```
"url": "https://dev.azure.com/fabrikam/_apis/wit/attachments/098a279a-60b9-40a8-868b-b7fd00c0a439",
"attributes": { ..., "id": 65274, ... }
```
The GUID is the **last path segment**. The URL may carry a `?fileName=` query, and **`attributes.id` is an integer, not the GUID**. The Attachments Get endpoint documents `id` as `string (uuid)`, "Attachment ID" `[CITED: learn.microsoft.com/en-us/rest/api/azure/devops/wit/attachments/get?view=azure-devops-rest-7.1]`.

## Architecture Patterns

### System Architecture Diagram

```
TUI key press (detail modal)
   │  att.ID (uuid.UUID), att.Name (server string), destDir = ~/.config/adoBizInt/attachments
   ▼
ado.Client.DownloadAttachment(ctx, id, destDir, name)
   │
   ├─► attachmentPath(name, destDir) ──reject──► error ──► AttachmentDownloadedMsg{Err}   (no network call)
   │        │ ok: path = destDir/<base>
   ▼        ▼
workitemClient.GetAttachmentContent({Id:&id})   ◄── fake in tests (t.Fatalf if called on reject path)
   │  SDK: resolved org host + Authorization header + connection.Timeout; non-2xx → error
   ▼
io.ReadCloser ──io.Copy──► os.Create(path) (after MkdirAll(destDir, 0700)) ──► nil / error ──► msg


CLI invocation: adoBizInt <noun> <verb> --project X ...
   │
   main.run: config.Load → cfg.Validate → configuredProjects = cfg.Projects (beside readBudget)
   ▼
dispatch → runRead / runWrite → fs.Parse
   │
   ├─► every --project value ∈ configuredProjects?  no ──► stderr msg, exit 3 (nothing ran)
   ▼ yes
fetch(projects) ─► ado.GetWorkItems / GetDashboardCounts ─► WIQL built with escaped project
                                                           (or @project, per Open Question 1)
                                                           log attrs: project, iteration count (no query text)

logging.Init: MkdirAll(dir,0700) → Chmod(dir,0700) → OpenFile(log,0600) → file.Chmod(0600) → slog handler
```

### Recommended Project Structure (files touched)

```
.github/workflows/ci.yml            # NEW (05-04)
internal/ado/attachments.go         # NEW: DownloadAttachment (moved) + attachmentPath pure func (05-01)
internal/ado/attachments_test.go    # NEW: table test, refusal-before-fetch, happy path, no-net/http structural (05-01)
internal/ado/workitems.go           # Attachment.ID, GUID extraction; delete old DownloadAttachment (05-01); WIQL escape + log attrs (05-03)
internal/ado/interfaces.go          # + GetAttachmentContent on workItemAPI (05-01)
internal/ado/fakes_test.go          # + GetAttachmentContentFunc (05-01)
internal/ado/json_test.go           # Attachment key set gains "id" (05-01)
internal/ado/pipelines.go, releases.go   # nil guards (05-02)
internal/ado/nilguard_test.go       # NEW: structural + behavioural (05-02)
internal/ado/dashboard.go           # WIQL escape + log attrs (05-03)
internal/ado/wiql_test.go           # NEW: structural D-12 test + escaping behaviour (05-03)
internal/ui/workitems/detail.go     # caller: handle UserHomeDir error, pass ID/destDir/name (05-01)
cmd/adoBizInt/main.go, dispatch.go, write.go, flags.go  # configuredProjects + check (05-03)
cmd/adoBizInt/*_test.go             # withConfiguredProjects helper + new rejection tests (05-03)
internal/logging/logging.go         # 0700/0600 + explicit chmod (05-05)
internal/logging/logging_test.go    # NEW: permission test (05-05)
cmd/adoBizInt/patleak_test.go       # NEW: PAT-absence regression (05-05)
```

Putting `DownloadAttachment` and the pure function in a new `attachments.go` keeps `workitems.go` (643 lines) from growing, and follows a real seam: attachment I/O is a different concern from work-item reads. After the move, `workitems.go` no longer needs the `net/http`, `io`, `os` or `path/filepath` imports. The compiler will flag each unused one, so let it guide the cleanup.

### Pattern 1: Download through the SDK by ID (HARD-01)

```go
// internal/ado/attachments.go — shape only; names other than SDK/stdlib are recommendations
func (c *Client) DownloadAttachment(ctx context.Context, id uuid.UUID, destDir, name string) error {
	path, err := attachmentPath(name, destDir) // refuses BEFORE any network call (D-03)
	if err != nil {
		return err
	}
	body, err := c.workitemClient.GetAttachmentContent(ctx, workitemtracking.GetAttachmentContentArgs{Id: &id})
	if err != nil {
		return err
	}
	defer body.Close()

	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return err
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, body); err != nil {
		out.Close()
		return err
	}
	return out.Close() // a Close error on a written file is a real write failure; don't defer-and-drop it
}
```
The interface addition in `interfaces.go` (current block is `type workItemAPI interface {` at line 17, last entry `GetClassificationNode(...)` at line 22):
```go
GetAttachmentContent(context.Context, workitemtracking.GetAttachmentContentArgs) (io.ReadCloser, error)
```
Note the interface name is **`workItemAPI`** (capital I) `[VERIFIED: internal/ado/interfaces.go:17]`. CONTEXT.md writes it as `workitemAPI`, which is a typo.

### Pattern 2: Pure containment function (HARD-02, D-03/D-08)

```go
func attachmentPath(name, destDir string) (string, error) {
	if !filepath.IsAbs(destDir) { // covers the UserHomeDir-failure "" case structurally
		return "", fmt.Errorf("attachment destination %q is not an absolute directory", destDir)
	}
	if strings.ContainsRune(name, 0) {
		return "", errors.New("attachment name contains a NUL byte")
	}
	base := filepath.Base(name) // Base("") == "."
	if base == "." || base == ".." || filepath.IsAbs(base) {
		return "", fmt.Errorf("attachment name %q is not a usable file name", name)
	}
	dir := filepath.Clean(destDir)
	path := filepath.Join(dir, base)
	if filepath.Dir(path) != dir { // post-join containment: exactly one level below destDir
		return "", fmt.Errorf("attachment name %q escapes %s", name, dir)
	}
	return path, nil
}
```
`filepath.Dir(path) != dir` is the prefix check D-03 asks for, in its exact-child form. It avoids the `strings.HasPrefix(p, dir+"/")` edge case when `dir == "/"`. Use `%q` for server-supplied names in error text so control characters cannot inject into the terminal.

### Pattern 3: GUID extraction (D-05/D-06)

```go
if rel.Url != nil {
	attachment.URL = *rel.Url
	if u, err := url.Parse(*rel.Url); err == nil {
		if id, err := uuid.Parse(path.Base(u.Path)); err == nil {
			attachment.ID = id
			hasID = true
		}
	}
}
...
if attachment.Name != "" && hasID { // D-06: unparsable GUID → not appended
```
**Type choice:** declare the field as `ID uuid.UUID `json:"id"``. `uuid.UUID` has `MarshalText` (`uuid@v1.1.1/marshal.go:10`), so it serializes as the canonical GUID string, which satisfies D-05's "uuid string". The TUI passes `att.ID` straight to `DownloadAttachment(ctx, att.ID, ...)` without parsing again and without importing `uuid` into `internal/ui`. A `string` field would force a second `uuid.Parse` at the call site. Either way, `json_test.go:149` (`assertKeySet(t, Attachment{}, []string{"name", "url", "size"})`) must become `{"id", "name", "url", "size"}`. Use `path.Base` (URL paths are slash-separated) rather than `filepath.Base`.

### Pattern 4: go/ast nil-guard pin (HARD-03, D-13) — prototype run this session

The prototype scanned **all** non-test files in `internal/ado`. It found exactly two collecting loops, both unguarded today:
```
internal/ado/pipelines.go:156:3: collecting loop over d guards Name=true Id=false
internal/ado/releases.go:103:3: collecting loop over rd guards Name=true Id=false
```
Shape: for each `*ast.RangeStmt` whose value identifier `v` appears as the second argument of an `append(..., v)` call in its body, the **first** body statement must be an `*ast.IfStmt` whose body is a single `continue` and whose condition, flattened over `||`, contains `v.Name == nil` and `v.Id == nil`. Assert that at least two loops were found (non-vacuity, as in `preread_test.go`'s `seen` map). Guarded loops elsewhere, such as `branches.go:84` `if repo.Name == nil || repo.Id == nil {`, do not append the range value, so there is no false positive.

The fix itself:
- `pipelines.go:157` `if d.Name == nil {` → `if d.Name == nil || d.Id == nil {`
- `releases.go:104` `if rd.Name == nil {` → `if rd.Name == nil || rd.Id == nil {`

`[VERIFIED: read this session]`. The deref sites are `pipelines.go:171` `defID := *def.Id` and `releases.go:125` `defID := *def.Id`. Behaviour tests: feed `GetDefinitionsFunc` / `GetReleaseDefinitionsFunc` a definition with a non-nil `Name` and nil `Id` alongside a normal one, and assert no panic and exactly one row. `pipelineTestClient` (`pipelines_test.go:151`) and `releaseTestClient` (`releases_test.go:202`) already exist as scaffolding.

### Pattern 5: Structural HARD-01 assertion (recommended, not in CONTEXT)

A go/ast test over the non-test files of `internal/ado` asserting that (a) no file imports `"net/http"` and (b) no `*ast.SelectorExpr` has `Sel.Name == "AuthorizationString"`. Imports are AST nodes, so a comment cannot defeat this. Today the only `net/http` import and the only `AuthorizationString` read are in `workitems.go` (lines 7 and 618) `[VERIFIED: grep]`. This is the SC1 "PAT cannot reach an arbitrary host" criterion expressed structurally, following the project's go/ast-over-grep precedent.

### Anti-Patterns to Avoid
- **Grep-negative acceptance criteria** ("grep for DefaultClient returns nothing"). Write every one as a go/ast test. This is the project's own Phase 4 rule.
- **`defer out.Close()` on a written file.** It drops the error that reports a failed flush. Return `out.Close()`.
- **Validating `--project` after the audit sink opens or after the pre-read.** Put the check with the arity checks, straight after `fs.Parse`.
- **`EvalSymlinks` on one side of the containment comparison only.** On macOS, `t.TempDir()` is under `/var` → `/private/var` `[VERIFIED: probe this session]`, so a half-resolved comparison rejects legitimate paths.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Attachment URL + auth header | `http.NewRequest` + `AuthorizationString` | `workitemClient.GetAttachmentContent` | The SDK already owns the host, auth, API version, timeout and non-2xx mapping |
| GUID parsing | regex over the URL | `url.Parse` + `path.Base` + `uuid.Parse` | Handles `?fileName=` queries and every GUID form uuid accepts |
| Lexical path safety | custom `..` scanning | `filepath.Base` / `filepath.Clean` / `filepath.Dir` (optionally `filepath.IsLocal`) | Cross-platform separator semantics |
| Log capture in tests | a new exported `logging.SetOutput` | `slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))` | `logging.Logger()` already returns `slog.Default()` when `logger == nil` (`logging.go`), so no production change is needed |
| CI toolchain install and cache | manual `curl` of Go tarballs | `actions/setup-go` with `cache: true` (the default) | Keyed on `go.sum` |

**Key insight:** every deletion in this phase (raw HTTP, interpolated query logging) removes attack surface. The additions are guards and tests, not mechanisms.

## Common Pitfalls

### Pitfall 1: The UI loses the destination path
**What goes wrong:** `AttachmentDownloadedMsg{Name, Path, Err}` currently receives `Path: destPath`. With the D-03 signature, the UI no longer computes the final path.
**Facts:** `msg.Path` is never read. The handler at `detail.go:106-112` shows only `msg.Name` or `msg.Err` `[VERIFIED: grep]`.
**How to avoid:** Keep the D-03 signature (returns `error`), drop `Path:` from the literal, and delete the now-unset field in the same edit. Handle the `os.UserHomeDir()` error at `detail.go:189` (`home, _ := os.UserHomeDir()`) by returning `AttachmentDownloadedMsg{Name: att.Name, Err: err}`. The `filepath.IsAbs(destDir)` check in Pattern 2 is the structural backstop.

### Pitfall 2: The hostile-name table has platform-dependent answers (probed this session, darwin/arm64, same semantics as Linux)
| Input | `filepath.Base` | Correct outcome with Pattern 2 |
|-------|-----------------|--------------------------------|
| `"../"` | `".."` | reject |
| `` `..\` `` | `` `..\` `` (literal name on Unix) | **accepted as `destDir/..\`**: contained, but not an error |
| `"../../.ssh/authorized_keys"` | `"authorized_keys"` | **accepted as `destDir/authorized_keys`** |
| `` `..\..\evil` `` | unchanged | accepted, contained (literal name) |
| `"/etc/passwd"` | `"passwd"` | **accepted as `destDir/passwd`** |
| `""` | `"."` | reject |
| `"."`, `".."` | same | reject |
| `"a\x00b"` | unchanged | reject (explicit NUL check; `os.Create` would otherwise fail with `invalid argument` only after the network call) |
| `"///"` | `"/"` | reject (absolute) |
| `` `\\\` `` | unchanged | accepted, contained (literal name on Unix) |

**How to avoid:** Write the table as `{input, wantErr bool, wantPath string}` and assert the containment property on every row (`err != nil || filepath.Dir(got) == destDir`). If the team wants `..\` rejected on every OS, the stricter rule is `if base != name { reject }`, which rejects any name containing a separator. That is a small tightening of D-03. It is in Open Question 3, not assumed here.

### Pitfall 3: `attributes.id` is not the attachment GUID
It is an integer (`"id": 65274` in the MS example). Parse the GUID from the URL path only.

### Pitfall 4: The D-12 rule trips on `iterationFilter`, not only `project` (prototype run this session)
```
internal/ado/dashboard.go:142:79:  bare operand *ast.Ident project
internal/ado/dashboard.go:142:151: bare operand *ast.Ident iterationFilter
internal/ado/workitems.go:149:218: bare operand *ast.Ident project
internal/ado/workitems.go:149:290: bare operand *ast.Ident iterationFilter
sites: 2
```
**Consequence:** D-12 as written forces the iteration clause into a call expression. The two sites carry an identical five-line escaping loop today (`workitems.go:141-145`, `dashboard.go:133-137`). The minimal compliant change is one private helper in package `ado`, for example `iterationClause(paths []string) string`, that returns `""` for an empty slice or `" AND [System.IterationPath] IN ('a', 'b''c')"`. Call it inline at both sites. That is two uses, below the rule of three, and it is **forced by locked D-12**. Record it as a justified exception, as D-08 does. `GetRecentIterations` returns no paths on error, so passing its result straight through keeps behaviour identical. Keep the dashboard's `appendFailure` on the error branch unchanged.
**Defeat vectors the test must also close:**
1. **`fmt.Sprintf("...FROM WorkItems...'%s'", project)`.** The literal is a call argument, not in a `+` chain. Count every `FROM WorkItems` literal in the package and fail if any sits outside a `+` chain.
2. **Any call satisfies "is a call result".** `strings.TrimSpace(project)` passes but escapes nothing. Recommend an allow-list of callees: `strings.ReplaceAll` with args `"'"` and `"''"`, or the named helpers. Open Question 2.
3. **Non-vacuity.** Assert that at least two sites were found.

### Pitfall 5: `--project` is parsed per verb, not "right after config.Load"
`gf.projects` is filled by `fs.Parse` inside `runRead` (`dispatch.go:243`) and `runWrite` (`write.go:139`). `main.run` never sees it. **How to satisfy D-09's intent:** capture `configuredProjects = cfg.Projects` in `run()` beside `readBudget = cfg.FetchTimeout` (`main.go:105`, right after `config.Load`/`Validate`). Check every `gf.projects` value against it immediately after `fs.Parse` in both `runRead` and `runWrite`, before logging, context, audit sink or fetch. Exit `exitUnknown` (`= 3`, `dispatch.go:25`). One private function in `flags.go`, called from two places, is acceptable under the user's rule ("a private function within the same file is fine"). Print the rejected value with `%q` to stderr and never to stdout.

**Write verbs:** D-09 says "before any read runs". Writes also take `--project` (exactly one, `oneProject` at `write.go:116-121`). The allow-list should cover writes too, because fail-closed applies equally, and it sits beside the arity checks where exit 3 already means "nothing attempted". Confirm with the user if in doubt. Recommended default: include writes.

**Case sensitivity:** exact match is recommended (predictable and fail-closed). ADO project names are case-insensitive, so `--project kinetic` against configured `Kinetic` would be rejected. `[ASSUMED]` acceptable, low risk.

### Pitfall 6: About 30 existing cmd tests pass `--project` values no allow-list contains
Counts `[VERIFIED: grep]`: `write_test.go` 20, `writeverbs_test.go` 6, `filter_test.go` 2, `usage_test.go` 2. Values: `P` ×24, `Q` ×2, `gamma` ×2, `one`, `alpha,beta`, `alpha, beta`. A nil `configuredProjects` makes every one of them exit 3. **How to avoid:** add `withConfiguredProjects(t, names ...string)`, which saves the var, sets it and restores it in `t.Cleanup`, mirroring `withReadBudget` at `main_test.go:694-699`, and call it in each affected test. Do not hide it in a `TestMain` or in `captureOutput`. Run `go test ./cmd/adoBizInt` before and after to prove zero regressions.

### Pitfall 7: `connection.Timeout` now bounds the whole download, body included
With the 30s default `RequestTimeout`, a large attachment on a slow link fails mid-copy with `Client.Timeout exceeded while reading body` and leaves a partial file. D-01 accepts this inheritance, and it is the TRUST-04 resolution. **How to avoid:** accept it and document it in the plan. Optionally `os.Remove(path)` when the copy fails, so a truncated file never looks complete. That costs one line and is recommended, but it is the planner's call.

### Pitfall 8: A PAT test that looks only for the raw sentinel misses the header form
The credential the code actually holds is `"Basic " + base64(":" + PAT)`. **How to avoid:** assert that the captured bytes contain neither `sentinel` nor `azuredevops.CreateBasicAuthHeaderValue("", sentinel)` nor its base64 suffix alone. Also make the test non-vacuous: assert the buffer **does** contain a known line such as `panic recovered`. That proves capture worked. It matters because `logging.Logger()` only falls back to `slog.Default()` while `logging.logger == nil`, and a future test that calls `logging.Init()` successfully inside the cmd test binary would silently redirect logs away from the buffer. Today no cmd test succeeds at `Init`: both callers of `initLogging()` point `HOME` at a regular file to force failure (`main_test.go:200-206`, `crash_test.go:270-273`) `[VERIFIED]`.

### Pitfall 9: `chmod` via path, and fd leaks on the new error return
Use `file.Chmod(0o600)` on the opened `*os.File`, which has no path race, and `os.Chmod(logDir, 0o700)` for the directory. If either fails, `file.Close()` before returning the error (D-21). Mode assertions in tests are meaningless on Windows. Guard them with `if runtime.GOOS == "windows" { t.Skip(...) }`. CI runs on Ubuntu.

### Pitfall 10: Where the PAT actually lives, for the D-19 test
`cfg.PAT` is referenced only in `internal/config` and in `newConnection` (`client.go:55`). `c.connection` is read only by the `DownloadAttachment` being deleted (`workitems.go:618`) `[VERIFIED: grep "\.connection\b"]`. After 05-01, `Client.connection` is **written in `NewClient` and never read**. The cmd dispatch and panic paths never hold the PAT. The D-19 test is therefore a tripwire for a future careless `slog` attribute, exactly as D-19 says, and not a detector of a present leak. Do not write an acceptance criterion that implies otherwise. See Open Question 4 on deleting the dead `connection` field.

## Code Examples

### D-12 structural test skeleton (verified against current code this session)
```go
// Source: prototype run against internal/ado this session; mirrors preread_test.go's structure
func flattenAdd(e ast.Expr, out *[]ast.Expr) {
	if b, ok := e.(*ast.BinaryExpr); ok && b.Op == token.ADD {
		flattenAdd(b.X, out)
		flattenAdd(b.Y, out)
		return
	}
	*out = append(*out, e)
}
// For each non-test file: ast.Inspect; at each top-level ADD BinaryExpr, flatten;
// if any operand is a STRING BasicLit containing "FROM WorkItems", count a site and
// require every operand to be *ast.BasicLit or an allowed *ast.CallExpr; return false
// so inner sub-chains are not re-visited. Separately count all "FROM WorkItems"
// literals; fail if total != in-chain count (Sprintf / composite-literal evasion).
// Fail if sites < 2.
```

### PAT-absence test (cmd), shape
```go
func TestPATNeverLogged(t *testing.T) {
	const sentinel = "SENTINELPAT-7f3a9c-do-not-log"
	t.Setenv("ADO_PAT", sentinel)
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	prevReady := loggingReady
	loggingReady = true // so handlePanic writes the stack to the (captured) log
	t.Cleanup(func() { loggingReady = prevReady })
	withConfiguredProjects(t, "P")
	captureOutput(t)
	// read:  dispatch(ctx, allReadsFake(t), []string{"prs","list","--json"})
	// write: dispatch a dry-run write through wrapFake/preReadFake with --project P
	// panic: guard(func() int { return dispatch(ctx, panickingClient(t), ...) })
	for _, needle := range []string{sentinel, azuredevops.CreateBasicAuthHeaderValue("", sentinel)} {
		if bytes.Contains(buf.Bytes(), []byte(needle)) { t.Fatalf("log output contains credential form %q", needle) }
	}
	if !bytes.Contains(buf.Bytes(), []byte("panic recovered")) { t.Fatal("capture was vacuous: no panic line logged") }
}
```
`allReadsFake`, `panickingClient`, `wrapFake`, `preReadFake`, `captureOutput`, `guard` and `loggingReady` all exist in package `main` `[VERIFIED: grep of cmd/adoBizInt]`. Take care that `slog.SetDefault` also redirects the stdlib `log` package, and do not add `t.Parallel()` here. No cmd test uses `t.Parallel` today `[VERIFIED: grep]`.

### Log permissions test (internal/logging, new file)
```go
func TestInitTightensExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" { t.Skip("POSIX modes") }
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "adoBizInt")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "adoBizInt.log"), nil, 0o644) // pre-existing loose install (D-20)
	if err := Init(); err != nil { t.Fatal(err) }
	// assert Stat(dir).Mode().Perm() == 0o700 and Stat(log).Mode().Perm() == 0o600
}
```
Add a fresh-install subtest where the directory does not exist. `internal/logging` has no tests today `[VERIFIED: go test output "[no test files]"]`, so this test's package-level `logger` state is isolated from the other test binaries.

### CI workflow
```yaml
# .github/workflows/ci.yml
name: ci
on:
  push:
  pull_request:
permissions:
  contents: read
jobs:
  check:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version: '1.25.4'   # D-16: matches go.mod exactly
      - run: go build ./...
      - run: go vet ./...
      - run: go test -race ./...
      - name: gofmt
        run: |
          unformatted=$(gofmt -l .)
          if [ -n "$unformatted" ]; then
            echo "gofmt needs to be run on:"; echo "$unformatted"; exit 1
          fi
```
Quote `'1.25.4'` so YAML never reads it as a float. `test -z "$(gofmt -l .)"` also works, but it fails without listing the offending files.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Hand-rolled path sanitising | `filepath.IsLocal` (Go 1.20), `os.Root` (Go 1.24; `Root.MkdirAll` and friends in 1.25) | Go 1.20 / 1.24 | Available under `go 1.25.4`. `os.Root` is the right tool for the deferred bulk CLI verb. |
| Unpinned action tags | SHA-pinned actions + `permissions: contents: read` | GitHub hardening guidance | Supply-chain least privilege, as the org rule requires |
| Interpolated literal for project | `@project` macro with the project in the route | Long-standing WIQL feature | Removes the interpolation (Open Question 1) |

**Deprecated/outdated:** CONCERNS.md's line numbers (`workitems.go:520`, `:104`; `dashboard.go:92`; `pipelines.go:125/139`; `releases.go:84/105`) are stale. The current lines are in this document. ROADMAP plan titles 05-01 ("host allow-list, timeouts, size cap") and 05-04 ("linter config") are overtaken by D-01, D-04 and D-15. The planner should retitle them.

## Recommended Plan Split and Waves

| Plan | Requirements | Files | Wave | Why |
|------|--------------|-------|------|-----|
| 05-01 Attachment by ID + containment | HARD-01, HARD-02 | attachments.go (new), workitems.go, interfaces.go, fakes_test.go, json_test.go, detail.go | 1 | Both requirements change the same function signature and its one call site. Splitting them (as the ROADMAP sketch does) creates an intermediate signature and a same-file conflict. |
| 05-02 Nil guards | HARD-03 | pipelines.go, releases.go, nilguard_test.go | 1 | Disjoint files |
| 05-04 CI | HARD-05 | .github/workflows/ci.yml | 1 | Disjoint. Landing it first gates every later plan. |
| 05-03 WIQL + project allow-list | HARD-04 | workitems.go, dashboard.go, wiql_test.go, cmd/* | 2 | Shares `workitems.go` with 05-01 |
| 05-05 Log perms + PAT test | HARD-06 | logging.go, logging_test.go, cmd/.../patleak_test.go | 3 | Its write path needs `--project P`, and therefore 05-03's `withConfiguredProjects` |

## Runtime State Inventory

Not a rename phase, but two runtime-state items matter:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | Existing `~/.config/adoBizInt/` directories are `0755` and `adoBizInt.log` files are `0644` on every install `[VERIFIED: logging.go:19,24 modes]` | The code edit alone is not enough. D-20's per-`Init` chmod repairs them on the next run. |
| Live service config | None. No external service stores these names. | none |
| OS-registered state | None | none |
| Secrets/env vars | `ADO_PAT` is read only by `config.loadFromEnv`. CI provides none (D-17). The suite passes with `ADO_PAT`, `ADO_ORG_URL`, `ADO_PROJECTS` and `ADO_PROJECT` unset and an empty `$HOME` `[VERIFIED: env -u … go test -race ./... this session, all ok]` | none |
| Build artifacts | Committed-ignored binary `/adoBizInt` in the repo root (`.gitignore`). No stray `.go` files outside `cmd/` and `internal/` `[VERIFIED: find]` | none: `gofmt -l .` stays clean |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `''` doubling is the correct WIQL escape for a single quote inside a single-quoted literal. The MS WIQL page says string literals may use single or double quotes but does not document the escape explicitly. | Pitfall 4, Open Question 1 | Escaping could be incomplete. `@project` avoids the question for the project field. |
| A2 | Exact-case match is acceptable for the `--project` allow-list | Pitfall 5 | Minor UX friction: a user typing different case gets exit 3 |
| A3 | The `@project` macro resolves from the route project on `POST {org}/{project}/_apis/wit/wiql` (MS docs say so, but it has not been exercised against a live org this session) | Open Question 1 | If wrong, the query returns no items. It would need a live smoke test before being adopted. |
| A4 | No Linux-only behaviour difference in the current test suite. It passes on darwin under Go 1.25.4 and 1.27.1 and cross-vets clean for `GOOS=linux`, but was not executed on Linux (Docker daemon not running). | Environment | The first CI run is the Linux verification. Treat a red first run as expected-possible. |

## Open Questions

1. **D-10 escaping vs the organization's parameterized-query rule (`@project`)**
   - What we know: the organization instruction says "When generating SQL, always use parameterized queries — never string concatenation", and organization instructions take priority over individual preferences. WIQL documents `@project`, and its REST form needs the project in the URL. The SDK's `QueryByWiql` already puts `args.Project` into the route (`routeValues["project"] = *args.Project`), and both sites already pass `Project: &project` `[VERIFIED: SDK client.go QueryByWiql; workitems.go:150-153; dashboard.go:143-146]`.
   - What's unclear: whether the user regards WIQL as covered by the rule, and whether a live-org smoke test is available (A3).
   - Recommendation: the planner raises this with the user at a checkpoint. **If accepted:** use `[System.TeamProject] = @project` at both sites, keep `''` escaping for iteration paths (there is no macro for arbitrary lists), and keep D-12's test. It passes trivially at the project operand and still catches `iterationFilter`. **If D-10 is held:** implement escaping exactly as locked. Either way, D-09 validation and D-11 logging are unchanged.

2. **How strict the D-12 "call result" rule is.** Recommend allow-listing callees (the escaper and the iteration helper) rather than accepting any call, because otherwise `strings.TrimSpace(project)` passes. This tightens D-12 without contradicting it.

3. **`..\` and other names containing a separator.** Should they be rejected outright on every OS (`base != name` → reject) instead of accepted as a literal contained filename on Unix? Either choice satisfies HARD-02. Recommendation: accept D-03 as written and assert containment, unless the user prefers deterministic cross-OS rejection.

4. **Delete `Client.connection` after 05-01.** Once `DownloadAttachment` stops reading it, the field is written in `NewClient` and read nowhere. Removing it means no struct in `internal/ado` holds a credential-bearing value, which strengthens HARD-01 and HARD-06. Recommendation: delete it in 05-01 as a direct consequence of D-01, not as a "while I'm here" change. `newConnection` and its tests stay.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | all | ✓ | go1.27.1 local; go1.25.4 downloaded via `GOTOOLCHAIN=go1.25.4` and verified | — |
| Race detector (cgo) | `go test -race` | ✓ locally; Ubuntu runners ship gcc `[ASSUMED]` | — | — |
| gofmt | CI gate | ✓ (bundled) | — | — |
| gh CLI | fetching action SHAs | ✓ (intermittent TLS timeouts; retries succeeded) | — | — |
| Docker (to run the Linux suite locally) | optional pre-CI check | ✗ (daemon not running) | 29.8.0 client | The first CI run is the check |
| actionlint / act | workflow lint/dry-run | ✗ | — | Review the workflow by hand. It is 25 lines. |

**Baseline gate today** `[VERIFIED: run this session]`: `go build ./...` OK, `go vet ./...` OK, `gofmt -l .` empty, `go test -race -count=1 ./...` all ok in about 3.2s, under both Go 1.25.4 and 1.27.1. `GOOS=linux GOARCH=amd64 go vet ./...` is OK.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | stdlib `testing` (Go 1.25.4); no third-party assertion library |
| Config file | none |
| Quick run command | `go test -race ./internal/ado ./internal/logging ./cmd/adoBizInt ./internal/ui/workitems` |
| Full suite command | `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| HARD-01 | Download calls `GetAttachmentContent` with the exact ID and writes its bytes | unit (fake) | `go test -race ./internal/ado -run TestDownloadAttachmentFetchesByID` | ❌ Wave 0 (`attachments_test.go`) |
| HARD-01 | No `net/http` import or `AuthorizationString` read in `internal/ado` | structural (go/ast) | `go test ./internal/ado -run TestNoRawHTTPInADO` | ❌ Wave 0 |
| HARD-01 | GUID parsed from the relation URL (with `?fileName=`); unparsable GUID not appended (D-06) | unit (fake `GetWorkItemFunc`) | `go test ./internal/ado -run TestGetWorkItemDetailAttachmentID` | ❌ Wave 0 |
| HARD-01 | `Attachment` JSON keys `id,name,url,size` | unit | `go test ./internal/ado -run TestJSONFieldNames` | ✅ update `json_test.go:149` |
| HARD-02 | Hostile-name table (Pitfall 2 rows) | unit (pure) | `go test ./internal/ado -run TestAttachmentPath` | ❌ Wave 0 |
| HARD-02 | Rejected name → error and **no** `GetAttachmentContent` call (fake field nil, `t.Fatalf` on call) | unit (fake) | `go test ./internal/ado -run TestDownloadAttachmentRefusesBeforeFetch` | ❌ Wave 0 |
| HARD-03 | Collecting loops guard `Name` and `Id` (≥2 found) | structural | `go test ./internal/ado -run TestCollectedDefinitionsGuardNameAndID` | ❌ Wave 0 |
| HARD-03 | nil-`Id` definition skipped, no panic | unit (fake) | `go test -race ./internal/ado -run 'TestGetPipelineRunsSkipsNilID\|TestGetReleasesSkipsNilID'` | ❌ Wave 0 |
| HARD-04 | Every `FROM WorkItems` chain has literal/allowed-call operands only (≥2 sites, no Sprintf evasion) | structural | `go test ./internal/ado -run TestWIQLInterpolationStructural` | ❌ Wave 0 |
| HARD-04 | Project `O'Brien` yields `'O''Brien'` (or `@project`) in the captured query at both sites; logged attrs carry no query text | unit (fake `QueryByWiqlFunc`) | `go test ./internal/ado -run 'TestGetWorkItemsEscapesProject\|TestDashboardEscapesProject'` | ❌ Wave 0 |
| HARD-04 | Unknown `--project` (alone and mixed with a valid one) → exit 3, no fake call, stderr message; same for a write verb | unit (cmd fake) | `go test ./cmd/adoBizInt -run TestUnknownProjectRejected` | ❌ Wave 0 |
| HARD-04 | All existing `--project` tests still pass with `withConfiguredProjects` | regression | `go test -race ./cmd/adoBizInt` | ✅ (update ~30 sites) |
| HARD-05 | CI runs the four gates on push/PR and fails on regression | CI / manual-once | Push a branch and observe a green run. Optionally push a deliberately unformatted file to see a red run, then revert. | ❌ Wave 0 (`ci.yml`) |
| HARD-06 | Existing 0755/0644 install → 0700/0600 after `Init`; fresh install is created 0700/0600 | unit | `go test ./internal/logging -run TestInit` | ❌ Wave 0 (`logging_test.go`) |
| HARD-06 | Sentinel PAT (raw and Basic-header form) absent from captured log across read, write and panic; capture non-vacuous | unit (cmd fakes) | `go test ./cmd/adoBizInt -run TestPATNeverLogged` | ❌ Wave 0 (`patleak_test.go`) |

HARD-05 is manual-only for its "runs on GitHub" half, because only a real push can prove the workflow triggers. Its local half is the full-suite command above.

### Sampling Rate
- **Per task commit:** quick run command (under 5s)
- **Per wave merge:** full suite command
- **Phase gate:** full suite green locally plus one green GitHub Actions run on the phase branch before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/ado/attachments_test.go`: HARD-01/02
- [ ] `internal/ado/nilguard_test.go`: HARD-03
- [ ] `internal/ado/wiql_test.go`: HARD-04 structural + escaping
- [ ] `internal/logging/logging_test.go`: HARD-06 permissions (first tests in this package)
- [ ] `cmd/adoBizInt/patleak_test.go`: HARD-06 PAT absence
- [ ] `withConfiguredProjects` helper in `cmd/adoBizInt/main_test.go`, beside `withReadBudget`
- [ ] `GetAttachmentContentFunc` on `fakeWorkItemAPI` in `internal/ado/fakes_test.go`
- [ ] `.github/workflows/ci.yml`

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (PAT as service credential) | The credential reaches only the SDK-resolved org host. No code path attaches it by hand. |
| V3 Session Management | no | — |
| V4 Access Control | yes (project scope) | `--project` allow-list against `cfg.Projects`, fail-closed, exit 3 |
| V5 Input Validation | yes | WIQL escaping or `@project`; filename sanitisation; GUID parse via `uuid.Parse` |
| V6 Cryptography | no | — (base64 Basic auth is transport encoding handled by the SDK over HTTPS) |
| V7 Error Handling & Logging | yes | No credentials in logs (sentinel test); query text no longer logged; log 0600, directory 0700 |
| V12 Files & Resources | yes | Server-supplied filename never used directly as a path (V12.3.1-class control) |
| V14 Configuration / build | yes | CI gate, `permissions: contents: read`, SHA-pinned actions, no secrets |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Credential sent to an attacker host via a server- or agent-supplied URL (C3) | Information disclosure | ID-only API; SDK owns the host; go/ast "no net/http" test |
| Path traversal through an attachment name (H6) | Tampering / Elevation | `Base` + rejects + exact-child check before any I/O |
| WIQL injection via project name (H5) | Information disclosure (read-only query) | `@project` or `''` escaping in the library, plus a cmd allow-list |
| Nil-pointer panic on malformed server data (MED4) | Denial of service | Collection-point guards plus the existing `handlePanic` backstop |
| World-readable log and config directory (MED2/MED1-adjacent) | Information disclosure | chmod 0600/0700 on every `Init` |
| CI supply chain (compromised action tag) | Tampering | Pin to commit SHA; read-only token; no secrets in workflow |
| Terminal escape injection through echoed server or agent strings | Tampering | `%q` formatting for rejected names and projects in stderr messages |

Regulatory note: nothing here touches CPNI, SOX data, or customer records. This is operator tooling against the operator's own ADO org.

## Sources

### Primary (HIGH confidence)
- Repo source read this session: `internal/ado/{workitems,dashboard,pipelines,releases,interfaces,client,fakes_test,preread_test,json_test}.go`, `internal/logging/logging.go`, `internal/config/config.go`, `internal/ui/workitems/detail.go`, `cmd/adoBizInt/{main,crash,dispatch,exit,flags,write,main_test,crash_test,usage_test,fakes_test}.go`, `go.mod`
- Vendored SDK: `azuredevops@v1.0.0-b5/{client.go,connection.go,workitemtracking/client.go}`; `google/uuid@v1.1.1/{uuid.go,marshal.go}`
- `go doc` (Go 1.27.1 toolchain): `net/http.Client` (Timeout, redirect header stripping), `path/filepath.IsLocal`, `os.OpenRoot`
- Probes run this session: filepath hostile-name table; D-12 and D-13 go/ast prototypes against the real package; baseline gate under Go 1.25.4 and 1.27.1; credential-free, empty-HOME test run
- `gh api`: actions/checkout and actions/setup-go tags, SHAs, latest releases, setup-go `action.yml`

### Secondary (MEDIUM confidence)
- [Attachments - Get (REST 7.1)](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/attachments/get?view=azure-devops-rest-7.1): `id` is `string (uuid)`
- [Work Items - Update, Add an attachment (REST 7.1)](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-items/update?view=azure-devops-rest-7.1#add-an-attachment): relation URL format, integer `attributes.id`
- [WIQL syntax reference](https://learn.microsoft.com/azure/devops/boards/queries/wiql-syntax): `@Project` macro, quoting of string literals
- [Previous-versions WIQL REST doc (TFS 2017)](https://learn.microsoft.com/en-us/previous-versions/azure/devops/integrate/previous-apis/wit/wiql?view=tfs-2017): the `@project` macro requires the project in the URL (via search summary)

### Tertiary (LOW confidence)
- None relied upon.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH. No new dependencies. Versions read from `go.mod`, the module cache and the GitHub API.
- Architecture: HIGH. Every seam, call site and test helper was read. Both go/ast rules were prototyped against the real code.
- Pitfalls: HIGH for Pitfalls 1-10 (each reproduced or read). MEDIUM for the `@project` alternative (documented, not exercised live).

**Research date:** 2026-09-30
**Valid until:** 2026-10-30 (stable domain; re-check action SHAs if planning slips past that)
