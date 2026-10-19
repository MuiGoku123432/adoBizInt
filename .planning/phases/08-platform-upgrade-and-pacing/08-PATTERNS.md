# Phase 8: Platform Upgrade and Pacing - Pattern Map

**Mapped:** 2026-10-19
**Files analyzed:** 27 new or modified file groups (plus 26 mechanical import sites)
**Analogs found:** 23 / 27 (4 have no in-repo analog: transport RoundTripper, synctest tests, real-SDK fake-RT tests, smoke script)

## Read This First: Locks and Structural Tripwires

These constraints decide what a plan may touch. They come from tests that already exist, so violating them fails CI rather than review.

| Constraint | Source | Effect on Phase 8 plans |
|-----------|--------|-------------------------|
| Phase 7 sha256 lock on 7 CLI test files | `.planning/phases/07-guarded-tui-writes/07-cli-tests.sha256` (fakes_test.go, filter_test.go, main_test.go, patleak_test.go, usage_test.go, write_test.go, writeverbs_test.go) | **PLAT-02 changes two locked files:** `cmd/adoBizInt/write_test.go:13` and `cmd/adoBizInt/patleak_test.go:11` (import path only). After the swap, `shasum -a 256 -c 07-cli-tests.sha256` reports 2 mismatches. The PLAT-02 summary must record `git diff -U0` limited to those import lines as the proof. The other 5 locked files must not change at all. New CLI tests go in a **new** file (`cmd/adoBizInt/throttle_test.go`), never in a locked one. Execution waits for Phase 7 verification (`07-VERIFICATION.md` status is `human_needed`) |
| `07-cli-audit-test.sha256` | `cmd/adoBizInt/audit_test.go` | No SDK import; not touched by Phase 8 |
| `TestNoRawHTTPInADO` body byte-identical (D-11) | `internal/ado/attachments_test.go:418-449` | Only the import path in that file changes (PLAT-02). **No non-test file in `internal/ado` may import `net/http`**, so `ado.NewClient` cannot take a `*http.Client` parameter. It must take the transport type (`*transport.Transport` or similar) and obtain the client by type inference (`hc := tr.HTTPClient()`). It also forbids any `*azuredevops.Connection` struct field (`isConnectionPointer`, `attachments_test.go:452-463`) |
| `TestValidateGuardsCLIAndTUI` (locked file) | `cmd/adoBizInt/main_test.go:944-1034` | In `run()`, `Validate`, `dispatch` and `ui.NewModel` must each appear in **exactly one** top-level statement, Validate first, and its `if` must return `exitUnknown` and name `ADO_PROJECTS`. Do not add a second `.Validate` selector call in `run()` (any selector named `Validate` matches). Keep the throttle-summary print inside the same `if len(os.Args) > 1 {` statement as `dispatch` |
| `TestNoUIFileCallsADOMutation` | `internal/guard/uibypass_test.go:20-65` | Any exported `*ado.Client` method whose name does not start with `Get`, `Current`, `Fetch`, `Download` (or is not `Projects`/`Pipelines`/`ReleaseDefinitions`) is classified a mutation and forbidden in `internal/ui`. A new retry-count accessor is harmless (UI never calls it, D-07) but its name decides the classification; prefer a read prefix or accept it is in the forbidden set |
| `TestGuardNeverPrintsOrExits` | `internal/guard/structural_test.go:108-208` | `guard.go` edits must not add `fmt.Print*`, `os.Stdout`, `os.Exit`. The same `forbiddenOutput` checker is the analog for a transport "never writes stdout/stderr" test |
| Existing fixtures under D-12 | `https://dev.azure.com/example`, `/fixture`, `/fabrikam` (config_test.go:100, :221, :279; client_test.go:20; main_test.go:903; crash_test.go:173) | All already satisfy D-12, so the locked `main_test.go:906` `cfg.Validate()` call keeps passing |

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `go.mod`, `go.sum` | config | build | `go.mod` (self) | exact (mechanical) |
| `.github/workflows/ci.yml` | config (CI) | batch | `.github/workflows/ci.yml` (self) | exact |
| 26 SDK import sites (incl. 2 Phase-7-locked) | mixed | n/a | mechanical rewrite | exact |
| `.planning/PROJECT.md` (lines 20, 104) | docs | n/a | self | exact |
| `internal/ado/result.go` (doc refs, then not-sent branch) | utility (classifier) | transform | self `classifyFailure` l.170-201 | exact |
| `internal/ado/result_test.go`, `internal/ado/fakes_test.go` (comment refs, new rows) | test | transform | self `TestClassifyFailure` l.22-111 | exact |
| `internal/ado/v7_test.go` (NEW) | test | request-response (real SDK, fake RT) | `internal/ado/result_test.go` + `fakes_test.go` helpers | partial (no RoundTripper analog) |
| `internal/transport/transport.go` (NEW) | middleware (RoundTripper) | request-response + shared pacing state | `internal/fetchctx/fetchctx.go` (package shape), `internal/guard/guard.go` (contract doc), `config.ConfigError` (typed error) | partial |
| `internal/transport/transport_test.go` (NEW) | test | request-response | `internal/guard/guard_test.go`, `cmd/adoBizInt/patleak_test.go`, `internal/guard/structural_test.go` | partial |
| `internal/ado/client.go` (`NewClient` rewrite) | service (constructor) | request-response | self l.44-138 + `FetchTimeout` accessor l.140-145 | exact |
| `internal/ado/client_test.go` (rewrite timeout test) | test | n/a | self l.10-51 | exact |
| `internal/ado/interfaces.go` (comment reword only) | model (interfaces) | n/a | self l.13-17 | exact |
| `internal/ado/retry.go` (NEW) | service (decorator) | request-response | `internal/ado/fakes_test.go` (one type per narrow interface) + `interfaces.go` | role-match |
| `internal/ado/retry_test.go` (NEW) | test + structural | request-response | `fakes_test.go` fail-fast fakes; `uibypass_test.go` `adoMutations` method-set walk | role-match |
| `internal/ado/construct_test.go` (NEW) | test (structural, go/ast) | n/a | `attachments_test.go` `TestNoRawHTTPInADO` + `nilguard_test.go` `parsePackageFiles` | exact |
| `internal/guard/guard.go` (`classify`) | utility (classifier) | transform | self l.237-286 | exact |
| `internal/guard/guard_test.go` (429 row, `TestRunNotSent`) | test | n/a | self `TestRunClassifiesOutcomes` l.187-224 | exact |
| `internal/config/config.go` (`Validate` org_url shape, org accessor) | config | validation | self `Validate` l.171-198, `ConfigError` l.200-207 | exact |
| `internal/config/config_test.go` (org_url table) | test | n/a | self `TestValidate` l.96-213 | exact |
| `cmd/adoBizInt/main.go` (build transport, pass to NewClient, summary) | controller (entry) | request-response | self `run()` l.68-172 | exact |
| `cmd/adoBizInt/throttle.go` (NEW, or inline in main.go) | utility (renderer) | stderr write | `cmd/adoBizInt/render.go` `renderScope` l.105-129 | exact |
| `cmd/adoBizInt/throttle_test.go` (NEW) | test | n/a | `main_test.go` `captureOutput` l.56-71, `TestRenderFailuresToStderrOnly` l.597-621 | exact |
| `scripts/smoke.sh` (NEW) | script | batch | none in repo | none |
| `.planning/phases/08-.../08-SMOKE.md` (NEW) | docs (record + checklist) | n/a | `.planning/phases/07-guarded-tui-writes/07-UAT.md` | role-match |
| `.planning/BACKLOG.md` (append findings) | docs | n/a | self §BL-02 l.73-98 | exact |

---

## Pattern Assignments

### PLAT-02: `go.mod`, `go.sum` (config, build)

**Analog:** current `go.mod`

Current lines that change (verbatim):
```
go 1.25.4
...
	github.com/microsoft/azure-devops-go-api/azuredevops v1.0.0-b5
...
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/google/uuid v1.1.1 // indirect
	golang.org/x/sys v0.38.0 // indirect
```
Expected after `go mod tidy` (RESEARCH Installation): `go 1.27.2`, `github.com/microsoft/azure-devops-go-api/azuredevops/v7 v7.1.0` in the direct block, `golang.org/x/sys v0.47.0 // indirect`, `uuid` and `clipboard` move to direct (closes BL-02 IN-01). No `toolchain` line. Use the exact command sequence in RESEARCH "Installation (PLAT-02, one commit)". No new module.

---

### PLAT-02: `.github/workflows/ci.yml` (config, CI)

**Analog:** self. Current (lines 1-2, 17-31):
```yaml
# Gates are exactly ROADMAP Phase 5 SC5: build, vet, race tests, gofmt.
# D-15 scope (no linters), D-16 go-version matches go.mod, D-17 no credentials.
...
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version: '1.25.4'
      - run: go build ./...
      - run: go vet ./...
      - run: go test -race ./...
      - name: gofmt
        run: |
          ...
```
Change: `go-version: '1.25.4'` to `go-version-file: go.mod`; append the step from RESEARCH Code Example 1 (`name: govulncheck`, `run: go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...`); update the header comment to list the new gate. Keep job name `check`, the SHA pins, `permissions: contents: read`, and `timeout-minutes: 15` unchanged.

---

### PLAT-02: the 26 SDK import sites (mechanical)

Files (from `grep -rl 'azure-devops-go-api/azuredevops' --include='*.go'`):

| Group | Files |
|-------|-------|
| `internal/ado` non-test (11) | attachments.go, branches.go, client.go, dashboard.go, interfaces.go, pipelines.go, preread.go, pullrequests.go, releases.go, result.go, workitems.go |
| `internal/ado` tests (11) | attachments_test.go, branches_test.go, dashboard_test.go, fakes_test.go, nilguard_test.go, pipelines_test.go, preread_test.go, pullrequests_test.go, releases_test.go, wiql_test.go, workitems_test.go |
| other tests (2) | internal/guard/guard_test.go, internal/ui/workitems/resolve_test.go |
| **Phase-7-locked (2)** | **cmd/adoBizInt/write_test.go:13, cmd/adoBizInt/patleak_test.go:11** |

Import block convention to preserve (gofmt groups stdlib, then third-party, then `sentinovo.ai/bizInt/...`), from `internal/ado/client.go:3-18`:
```go
import (
	"context"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
	...

	"sentinovo.ai/bizInt/internal/config"
	"sentinovo.ai/bizInt/internal/logging"
)
```
After the rewrite the paths become `.../azuredevops/v7` and `.../azuredevops/v7/build`; package names (`azuredevops`, `build`, ...) are unchanged, so no call site changes.

---

### PLAT-02: `.planning/PROJECT.md` (docs)

Lines to edit (verbatim today):
- l.20: `... exit codes 0-5 distinguish success, partial, auth failure, usage/config error, write failure and unknown outcome` -- mislabels 4; exit 4 is `exitNoOp` (`cmd/adoBizInt/dispatch.go:28`). Correct it in the same edit (CONTEXT Specifics).
- l.104: `- **Tech stack**: Go 1.25.4, Charmbracelet Bubbletea. No new dependencies without stated justification` -- change to Go 1.27.2 and the v7 SDK.

---

### `internal/ado/result.go` (utility, transform) -- PLAT-02 doc refs, then PLAT-03 branch

**Analog:** self.

**Doc refs to update in PLAT-02** (l.182-185 and l.203-209 verbatim):
```go
	// Belt and braces: the deadline may come from context.WithTimeout (the
	// fan-out budget) or from http.Client.Timeout (the per-call budget via
	// connection.Timeout). Modern toolchains surface both as
	// context.DeadlineExceeded, but the net.Error check does not depend on that.
...
// azuredevops.Client.UnwrapError returns a *WrappedError on two paths
// (client.go:366, :403) and a bare WrappedError value on two others
// (client.go:389, :411). WrappedError.Error() has a value receiver, so both
```
v7 line numbers: pointer at `client.go:375, :412`; value at `:398, :419` (RESEARCH Pattern 1; STACK.md's numbers are wrong). The `connection.Timeout` mention becomes "via the injected http.Client.Timeout" once PLAT-03 lands. Same b5 citations also live in `result_test.go:34, :40` and `fakes_test.go:28, :34`.

**Classifier to extend in PLAT-03** (l.170-201, verbatim):
```go
func classifyFailure(err error) FailureKind {
	if err == nil {
		return ""
	}

	// Cancellation is checked before the timeout branch and deliberately falls
	// through to FailureOther. Phase 3's CLI-04 interrupt handling depends on a
	// user-initiated cancel not being reported as a server timeout.
	if errors.Is(err, context.Canceled) {
		return FailureOther
	}

	// Belt and braces: ...
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return FailureTimeout
	}

	if status, ok := statusCodeOf(err); ok {
		switch status {
		case 401, 403:
			return FailureAuth
		case 429:
			return FailureRateLimit
		}
	}

	return FailureOther
}
```
Rules for the new branch (RESEARCH Pattern 4, Pitfall 5):
- The D-01 fail-fast form (rate-limited not-sent) returns `FailureRateLimit`. It must not wrap `context.DeadlineExceeded` and its type must not implement `Timeout()` (`*url.Error.Timeout()` forwards to the wrapped error, so the `net.Error` check above would catch it).
- The held-then-cancelled form wraps `ctx.Err()` and keeps today's cancel/timeout classification; no new branch needed for it.
- A refused host on a read classifies `FailureOther` (Open Question 7), with the host in the error text.
- Use `errors.Is` / `errors.As` only. File header rule (l.164-169): "It classifies structurally, never by matching the error text (D-06)".
- `internal/ado` may import `internal/transport` and `net/url`; it may not import `net/http`.

**Exported-wrapper convention** (l.233-251) if `guard` needs a not-sent predicate without importing `transport` directly:
```go
// StatusCodeOf is the exported wrapper over statusCodeOf. The CLI's write exit
// mapper needs the HTTP status ... and duplicating the extraction there would
// duplicate the value-vs-pointer WrappedError subtlety documented above ...
func StatusCodeOf(err error) (int, bool) {
	return statusCodeOf(err)
}
```

---

### `internal/ado/result_test.go` (test) -- new rows

**Analog:** self `TestClassifyFailure` (l.22-111). Row shape to copy:
```go
	tests := []struct {
		name string
		err  error
		want FailureKind
	}{
		...
		{
			name: "status 429 is rate_limit",
			err:  wrappedErrorPointer(429, "too many requests"),
			want: FailureRateLimit,
		},
		...
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyFailure(tt.err); got != tt.want {
				t.Errorf("classifyFailure() kind = %v, want %v", got, tt.want)
			}
		})
	}
```
Add rows: fail-fast not-sent wrapped in `*url.Error` -> `rate_limit`; hold-cancel not-sent wrapping `context.DeadlineExceeded` -> `timeout`; wrapping `context.Canceled` -> `other`; refused host -> `other`; `fmt.Errorf("rate limited after 2 retries: %w", wrappedErrorPointer(429, ...))` -> `rate_limit` (D-08 message keeps the 429 reachable). The `fakeNetTimeout` type at l.11-20 is the model for proving a type does or does not satisfy `net.Error`.

---

### `internal/ado/v7_test.go` (NEW; test, real SDK over fake base RoundTripper)

**Analog:** no in-repo test drives a real SDK client (all tests use the narrow-interface fakes). Closest: `result_test.go` table + `fakes_test.go` helper-constructor style. Use RESEARCH Code Example 2 for the body.

**Helper-constructor style to copy** (`internal/ado/fakes_test.go:22-37`):
```go
// azuredevops.Client.UnwrapError returns a *WrappedError on two paths and a bare
// WrappedError value on two others, so tests that inject an SDK failure must be
// able to produce either shape. These constructors keep that detail in one place
// rather than in every expansion read's test file.

// wrappedErrorValue builds the bare-value error shape the SDK returns for a
// text/plain body (client.go:389) and for a normal JSON error body (client.go:411).
func wrappedErrorValue(statusCode int, message string) error {
	return azuredevops.WrappedError{Message: &message, StatusCode: &statusCode}
}
```
Rules:
- Test files may import `net/http` (`TestNoRawHTTPInADO` parses non-test files only, `nilguard_test.go:149`).
- Shared helpers live here once: `rtFunc`, a JSON response builder, a location-list fixture that includes resource-areas location `e81700f7-3be2-46de-8624-2eb35882fcaa` and every data route used, and a unique org per test from `t.Name()` (Pitfall 6: the SDK's location cache is a process global keyed by base URL).
- Cover both 401 forms: JSON body (value) and `ContentLength: 0` (pointer).
- Startup refusal test (`TestNewClient...LocationURL`): assert `*config.ConfigError` with `Field == "org_url"` naming the host, using the same assertion shape as `config_test.go:204-210` (see config section).
- `rtFunc` will also be needed in `internal/transport/transport_test.go`. Two copies in two packages is below the rule of three: keep both and add a `// note:` comment pointing at the twin (CONVENTIONS "Rule of three").

---

### `internal/transport/transport.go` (NEW; middleware, request-response with shared pacing state)

**Analogs:** no `RoundTripper`, `net/http`, or sentinel error exists in the repo today. Copy the package-level conventions from these:

**Package doc that states why the package exists and what it never does** (`internal/fetchctx/fetchctx.go:1-18`, `internal/guard/guard.go:1-9`):
```go
// Package guard is the single guarded-write sequence the CLI and the TUI share
// (GUARD-01). A guard that lives in one shared body cannot be forgotten on a
// surface; a guard copied per surface can.
//
// The package never writes stdout and never exits the process. Each surface maps
// the returned Report to its own output and exit code ...
package guard
```
Transport doc must state: stdlib only, the only production `net/http` importer (D-11), paces but never retries, never writes stdout or stderr (OUT-03), never logs the URL query, request headers or body (Pitfall 15).

**Small stdlib-only package shape** (`internal/fetchctx/fetchctx.go:20-50`): one constant, one or two exported funcs, each with a doc comment explaining the rule it enforces:
```go
import (
	"context"
	"time"
)

// defaultBudget bounds a fetch when no budget is configured. ...
const defaultBudget = 60 * time.Second

func NewWithParent(parent context.Context, budget time.Duration) (context.Context, context.CancelFunc) {
	if budget <= 0 {
		budget = defaultBudget
	}
	return context.WithTimeout(parent, budget)
}
```

**Typed error shape** (`internal/config/config.go:200-207`), the only custom error type in the repo:
```go
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Field + ": " + e.Message
}
```
For the not-sent error: one sentinel reachable via `errors.Is` (first-hop refusals and pacer holds only), a reason (host / rate-limited / held), the refused host for the message, and `Unwrap()` returning `ctx.Err()` for the held form. Must **not** implement `Timeout()`. Redirect-hop refusals (`req.Response != nil`) must return an error that does **not** satisfy `errors.Is(err, ErrNotSent)` (Pitfall 4). D-08 message text: `rate limited: ADO asked for 45s, 12s left in budget`.

**Logging call shape** (`internal/ado/client.go:94, :113-115`):
```go
	log := logging.Logger()
	...
	if currentUserID == "" {
		log.Warn("Could not cache current user ID; pull request approval will retry identity lookup")
	}
	log.Info("Current user identity", "displayName", currentUser, "email", currentUserEmail)
```
Import `sentinovo.ai/bizInt/internal/logging` (internal, not a new module). Key-value attributes only; log `Retry-After`, `X-RateLimit-Delay`, `X-RateLimit-Remaining`, `X-RateLimit-Limit`, `X-RateLimit-Resource` values plus the wait, and only when one of those headers is present or a wait/refusal happens (Pitfall 15).

**Shared mutable state** (`internal/ui/confirm/batch.go:13-15`, the repo's only `sync` user, and `internal/ui/confirm/inflight_test.go:32` `var mutates atomic.Int32`):
```go
// inflight counts the Execute calls in flight, so that process exit can wait
// for each one's audit append (CR-01, Phase 4 D-13).
var inflight sync.WaitGroup
```
For the pacer: hold `notBefore` behind a `sync.Mutex` on the `Transport` value (process-wide, RESEARCH Pattern 4), counters as `atomic` fields so concurrent TUI fetches are race-safe under `go test -race`.

**Core skeleton:** RESEARCH Code Example 3 (`Allowed`) and Code Example 4 (`RoundTrip`). Base transport is `http.DefaultTransport` (not `Clone()`, RESEARCH Alternatives). `HTTPClient()` returns `&http.Client{Transport: t, Timeout: requestTimeout}` with zero meaning no `Timeout` (matches `newConnection`'s nil-for-zero rule, `internal/ado/client.go:52-60`). Do not delete `X-TFS-Session` (Pitfall 1). Import graph: `transport` imports only stdlib and `internal/logging`; it must not import `ado`, `guard` or `config` (would cycle, since `ado` imports `transport`).

Keep the file near 150 lines; split `Allowed` into its own file only if it crosses that (CONVENTIONS "Structure and size").

---

### `internal/transport/transport_test.go` (NEW; test)

**Analogs:**

Table plus `t.Run` style (`internal/guard/guard_test.go:191-223`), used for the host-rule table (`TestAllowed`):
```go
	tests := []struct {
		name      string
		parent    context.Context
		mutateErr error
		want      string
		wantLines int
	}{
		{name: "nil", parent: context.Background(), mutateErr: nil, want: OutcomeExecuted, wantLines: 1},
		...
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			...
		})
	}
```

Fail-fast helper style (`internal/guard/guard_test.go:57-69`), for "refused request never reaches base RT":
```go
func mustNotMutate(t *testing.T) func(context.Context) error {
	return func(context.Context) error {
		t.Fatalf("Mutate was called")
		return nil
	}
}
```

slog capture plus non-vacuous marker check (`cmd/adoBizInt/patleak_test.go:41-46, :77-96`), for PAT-never-logged in the transport:
```go
	var buf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	// slog.SetDefault also redirects the stdlib log package, so restoring it
	// matters for every later test in the binary.
	t.Cleanup(func() { slog.SetDefault(prevLogger) })
	...
	for _, needle := range []struct{ form, value string }{
		{"raw", patFixture},
		{"basic header", azuredevops.CreateBasicAuthHeaderValue("", patFixture)},
		{"base64", base64.StdEncoding.EncodeToString([]byte(":" + patFixture))},
	} {
		if bytes.Contains(logged, []byte(needle.value)) {
			t.Errorf("log output contains the %s form of the PAT", needle.form)
		}
	}
```
Do not call `logging.Init` in the test (patleak_test.go:33-36: a successful Init redirects `logging.Logger()` away from `slog.Default()` and the capture goes empty). `transport_test.go` cannot import the SDK for `CreateBasicAuthHeaderValue` without reason; build the Basic header with `base64` directly or set `Authorization` on the request under test.

AST "never prints" check (`internal/guard/structural_test.go:165-208` `forbiddenOutput`) is the model if the plan adds a structural assertion that the transport never writes stdout/stderr; copy the function (different package) and its tripwire sub-test (l.132-162).

Deterministic pacing: `testing/synctest.Test` with a fake base RT (RESEARCH Code Example 6). No in-repo analog. Never use `httptest.NewServer` inside a bubble.

---

### `internal/ado/client.go` (service constructor; `NewClient` rewrite)

**Analog:** self. Current construction (l.44-91, abbreviated verbatim):
```go
func newConnection(cfg *config.Config) *azuredevops.Connection {
	connection := azuredevops.NewPatConnection(cfg.OrgURL, cfg.PAT)
	if cfg.RequestTimeout > 0 {
		timeout := cfg.RequestTimeout
		connection.Timeout = &timeout
	}
	return connection
}

func NewClient(cfg *config.Config) (*Client, error) {
	connection := newConnection(cfg)

	ctx := context.Background()

	workitemClient, err := workitemtracking.NewClient(ctx, connection)
	if err != nil {
		return nil, err
	}
	...
	releaseClient, err := release.NewClient(ctx, connection)
	if err != nil {
		return nil, err
	}

	// Fetch current user from PAT identity
	log := logging.Logger()
	locationClient := location.NewClient(ctx, connection)
```
Every `X.NewClient(ctx, connection)` and `location.NewClient` call is what D-11 forbids. Replace with RESEARCH Code Example 5: `areas := root.GetResourceAreas(ctx)`, normalize each used `LocationUrl` with `strings.ToLower(strings.TrimRight(u, "/"))`, check it with the transport's one rule function, return `&config.ConfigError{Field: "org_url", Message: ...host...}` on refusal, then build `&X.ClientImpl{Client: *azuredevops.NewClientWithOptions(conn, url, opt)}`. Reference area IDs as `workitemtracking.ResourceAreaId` etc., never literals. Check only wit, git, build, release, work plus BaseUrl (Open Question 3). `core` is not built (D-16).

Keep these parts unchanged: identity block (l.93-115, but `getConnectionData` gets wrapped by the read retry), `releaseDefinitions` fallback (l.117-121), struct literal (l.123-137). Signature becomes `NewClient(cfg *config.Config, tr *transport.Transport)` (name per plan); `main.go:111` is the only caller.

**Read-retry wiring:** wrap the four narrow-interface fields and `getConnectionData` in the decorators from `retry.go`. Existing tests assign fakes straight to `c.gitClient` etc. (e.g. `nilguard_test.go:20-39`) and stay unaffected.

**Accessor shape for the retry counter** (l.140-145, verbatim):
```go
// FetchTimeout reports the configured budget for one whole multi-project
// fan-out. Views read it here rather than having it threaded through six view
// constructors and app.go for a value that already travels with the client.
func (c *Client) FetchTimeout() time.Duration {
	return c.fetchTimeout
}
```
Name it with care (see `TestNoUIFileCallsADOMutation` in the locks table). The `client` interface in `cmd/adoBizInt/dispatch.go:83-86` and the locked cmd fakes need no new method: `main.run` holds the concrete `*ado.Client` (RESEARCH Code Example 7 note).

**`Client` struct doc** (l.20-25) says "every field is unexported, and it holds live SDK sub-clients" and forbids json tags. A retry counter field stays unexported and untagged. Never add a `*azuredevops.Connection` field (`TestNoRawHTTPInADO`).

---

### `internal/ado/client_test.go` (test; rewrite)

**Analog:** self (l.10-51). Both tests assert `newConnection(cfg).Timeout`, which v7 construction bypasses (`WithHTTPClient` ignores `connection.Timeout`). Rewrite to assert the injected `http.Client.Timeout` equals `cfg.RequestTimeout`, and zero stays zero (no deadline installed). The test file may name `net/http`. Keep the doc-comment style that explains why the assertion matters:
```go
// TestNewClientSetsTimeout asserts the connection carries the per-call budget.
//
// It exercises the connection helper rather than NewClient itself: NewClient
// performs six network round trips against Azure DevOps and cannot run as a unit
// test. ...
```
With the fake base RT from `v7_test.go`, `NewClient` itself becomes testable, so the "cannot run as a unit test" sentence is now false; update it.

---

### `internal/ado/interfaces.go` (comment reword only)

Current (l.13-17):
```go
// These package-local interfaces make destructive write methods testable without
// crossing the Azure DevOps REST boundary (SEAM-01). Each interface declares only
// methods this package calls; adding an uncalled method violates ROADMAP SC4.
// The SDK constructors return interfaces, so their assignments in NewClient are
// already compile-time drift checks. Do not add concrete ClientImpl assertions.
```
After D-11, `NewClient` assigns `&X.ClientImpl{...}` (wrapped in a decorator) to the interface fields, which is still a compile-time check. Reword the last two sentences; do not add `var _ gitAPI = (*git.ClientImpl)(nil)` assertions (RESEARCH Pattern 2). The interface method sets themselves do not change.

---

### `internal/ado/retry.go` (NEW; service decorator, request-response)

**Analog for "one type per narrow interface, one method per interface method"** (`internal/ado/fakes_test.go:15-20, :45-62`):
```go
var (
	_ workItemAPI = (*fakeWorkItemAPI)(nil)
	_ gitAPI      = (*fakeGitAPI)(nil)
	_ buildAPI    = (*fakeBuildAPI)(nil)
	_ releaseAPI  = (*fakeReleaseAPI)(nil)
)

type fakeWorkItemAPI struct {
	t *testing.T
	...
}

func (f *fakeWorkItemAPI) UpdateWorkItem(ctx context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
	f.t.Helper()
	...
	return f.UpdateWorkItemFunc(ctx, args)
}
```
Difference for the decorator: embed the interface so the 5 writes fall through untouched, and override only the 16 reads (RESEARCH Pattern 5 snippet). Method signatures to override come verbatim from `interfaces.go:18-49`:

| Interface | Reads to override | Writes that must fall through (never overridden) |
|-----------|-------------------|--------------------------------------------------|
| `workItemAPI` | GetWorkItems, GetWorkItem, QueryByWiql (POST read), GetClassificationNode, GetAttachmentContent | UpdateWorkItem |
| `gitAPI` | GetBranches, GetPullRequest, GetPullRequestsByProject, GetRefs, GetRepositories | CreatePullRequestReviewer, UpdatePullRequest |
| `buildAPI` | GetBuilds, GetDefinition, GetDefinitions | QueueBuild |
| `releaseAPI` | GetApprovals, GetReleaseDefinitions, GetReleases | UpdateReleaseApproval |

Plus `getConnectionData` (field of func type, `client.go:41`; used at `pullrequests.go:176-182`).

`retryRead` rules (generic over the return type is justified by 17 call shapes, RESEARCH Pattern 5): retry only when `statusCodeOf(err) == 429`; never on the D-01 fail-fast (no status); never when `ctx.Err() != nil`; at most 2 retries (fixed constant, D-03); on exhaustion `fmt.Errorf("rate limited after 2 retries: %w", err)`; increment the shared `*atomic.Int64` per retry. Types stay unexported (no exported symbol without an external caller).

Error-wrapping convention from `pullrequests.go:179-182`:
```go
		connData, err := c.getConnectionData(ctx, location.GetConnectionDataArgs{})
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}
```

---

### `internal/ado/retry_test.go` (NEW; behaviour + structural)

**Behaviour analog:** fail-fast fakes from `fakes_test.go` (an unset func field calls `t.Fatalf("unexpected call to ...")`, l.56-62) wrapped by the decorator; count calls with a closure counter as in `pullrequests_test.go:145-163`:
```go
	providerCalls := 0
	client := &Client{
		getConnectionData: func(_ context.Context, _ location.GetConnectionDataArgs) (*location.ConnectionData, error) {
			providerCalls++
			return &location.ConnectionData{AuthenticatedUser: &identity.Identity{}}, nil
		},
		gitClient: &fakeGitAPI{t: t},
	}
	...
	if providerCalls != 1 {
		t.Errorf("getConnectionData calls = %d, want 1", providerCalls)
	}
```
Cases: 429 then success (2 calls, 1 retry); three 429s (3 calls, message `rate limited after 2 retries`, still `FailureRateLimit`); fail-fast not-sent (1 call); cancelled ctx (1 call); 500 (1 call); a write method through the decorator (1 call, never retried).

**Structural analog** (method-set derivation by AST, `internal/guard/uibypass_test.go:32-65`):
```go
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 || !fn.Name.IsExported() {
				continue
			}
			star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			if id, ok := star.X.(*ast.Ident); ok && id.Name == "Client" && !readOnlyClientMethod(fn.Name.Name) {
				forbidden[fn.Name.Name] = true
			}
		}
```
Adapt: collect the method names of each interface type in `interfaces.go` (walk `*ast.TypeSpec` with `*ast.InterfaceType`), collect methods declared on each `retrying*` receiver in `retry.go` (value or pointer receiver), and assert overridden set == interface methods minus the five writes. Use `parsePackageFiles(t)` (`nilguard_test.go:139-159`) for the file map, and fail non-vacuously if `retry.go` or `interfaces.go` is missing from it (pattern: `attachments_test.go:446-448`).

---

### `internal/ado/construct_test.go` (NEW; structural go/ast, D-11)

**Analog:** `internal/ado/attachments_test.go:406-449` (verbatim core):
```go
func TestNoRawHTTPInADO(t *testing.T) {
	fset, files := parsePackageFiles(t)
	for _, file := range files {
		for _, imp := range file.Imports {
			if imp.Path.Value == `"net/http"` {
				t.Errorf("%s imports net/http; attachment and API traffic must go through the SDK", fset.Position(imp.Pos()))
			}
		}

		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if n.Sel.Name == "AuthorizationString" {
					t.Errorf("%s reads AuthorizationString; the PAT header belongs to the SDK", fset.Position(n.Pos()))
				}
			...
			}
			return true
		})
	}

	if files["attachments.go"] == nil {
		t.Errorf("parsed files do not include attachments.go; the structural assertion covered nothing")
	}
}
```
Doc-comment rationale to reuse (l.410-414): "This is an AST walk rather than a grep ... STATE.md records three Phase 3 incidents where a 'grep returns nothing' criterion was satisfied and then quietly defeated by a prose comment".

New assertions: no selector `GetClientByUrl` or `GetClientByResourceAreaId`; no call `<pkg>.NewClient(...)` where `<pkg>` is one of `azuredevops`, `workitemtracking`, `git`, `build`, `release`, `work`, `location`, `core`; at least one `azuredevops.NewClientWithOptions` call and every one of them passes an argument that is a `WithHTTPClient(...)` call or an identifier bound to one (non-vacuous: fail if `client.go` is not parsed or zero `NewClientWithOptions` calls are found). Selector matching resolves package names through the file's imports, as `forbiddenOutput` does (`internal/guard/structural_test.go:169-177`). Do **not** edit `TestNoRawHTTPInADO`; put the new test in its own file.

---

### `internal/guard/guard.go` (`classify`, utility)

**Analog:** self (l.237-286). Current head of the function:
```go
func classify(err, parentErr error) string {
	if parentErr != nil {
		return OutcomeUnknown
	}
	if err == nil {
		return OutcomeExecuted
	}

	switch ado.ClassifyFailure(err) {
	case ado.FailureTimeout:
		// D-20: ...
		return OutcomeUnknown
	case ado.FailureAuth:
		return OutcomeFailed
	}
```
Insert the not-sent check as the **first** statement (D-02), before `parentErr`. Update the doc comment (l.245-251 says "parentErr is consulted FIRST and is the only way to see a signal") to state the new precedence and why: a request that provably never left the process cannot have landed. Keep the reason-comment style used for each branch (`// P-13: ...`, `// P-12: ...`). Call site at l.200 (`outcome := classify(err, ctx.Err())`) and the audit rule at l.207 (`if outcome == OutcomeExecuted || outcome == OutcomeUnknown {`) stay unchanged, so a not-sent write gets no audit record. `guard` already imports `ado`; choose between importing `transport` directly or an `ado` exported wrapper (see result.go section). No stdout/exit (`TestGuardNeverPrintsOrExits`).

---

### `internal/guard/guard_test.go` (test)

**Analog:** `TestRunClassifiesOutcomes` (l.187-224). Add a 429 row to the existing table:
```go
		{name: "403", parent: context.Background(), mutateErr: wrappedError(403, "forbidden"), want: OutcomeFailed, wantLines: 0},
```
becomes a sibling row `{name: "429", ..., mutateErr: wrappedError(429, "too many requests"), want: OutcomeFailed, wantLines: 0}`. New function `TestRunNotSent` in the same table shape: not-sent with background parent -> `failed`, 0 lines; not-sent with a cancelled parent (`cancelled, cancel := context.WithCancel(...); cancel()`, l.188-189) -> `failed`, 0 lines; redirect-hop refusal (not `ErrNotSent`, no status) -> `unknown`, 1 line. Reuse `tempSink`, `auditLines`, `testOp`, `planOne`, `confirmed`, `wrappedError` (l.27-75). Assert `errors.Is(got.MutateErr, tt.mutateErr)` as l.216 does. This file is not in the Phase 7 lock.

---

### `internal/config/config.go` (`Validate`, config validation)

**Analog:** self (l.171-198). Validation style:
```go
func (c *Config) Validate() error {
	if c.PAT == "" {
		return &ConfigError{Field: "pat", Message: "Personal Access Token is required"}
	}
	if c.OrgURL == "" {
		return &ConfigError{Field: "org_url", Message: "Organization URL is required"}
	}
	// D-10: an empty list makes every read fan out over nothing and report
	// complete:true. This covers the CLI and the TUI alike, because run()
	// validates before both.
	if len(c.Projects) == 0 {
		return &ConfigError{Field: "projects", Message: "at least one project is required"}
	}
	// D-11: a blank name would reach the read as a project with no name. Names
	// are validated, never trimmed or rewritten, so what the operator configured
	// is what the read uses.
```
Insert the D-12 shape check directly after the empty-`org_url` check (so `pat` still reports first, mirroring the "missing pat with no projects" ordering row in `config_test.go:150-156`). Prefix with a `// D-12: ...` reason comment. Parse with `net/url` per RESEARCH Open Question 6 (https, no userinfo, no port, no query/fragment; `dev.azure.com` + exactly one path segment, optional trailing slash; or `<label>.visualstudio.com` with empty or `/` path). Validate only; never rewrite `c.OrgURL` (the D-11 "validated, never trimmed or rewritten" rule above). Return `&ConfigError{Field: "org_url", Message: ...}`.

The `{org}` value is needed by `main.go` to build the transport. Expose it once (for example a `Config` method parsing the same way) rather than parsing in two places; method-on-Config style matches `LogValue`/`String` (l.60-72). Keep `LogValue`/`String` untouched (they already render `org_url`, never the PAT).

---

### `internal/config/config_test.go` (test)

**Analog:** `TestValidate` (l.96-213). Copy the `base()` + `mutate` + `wantField` table and its assertion block verbatim:
```go
			err := cfg.Validate()
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}

			cfgErr, ok := err.(*ConfigError)
			if !ok {
				t.Fatalf("Validate() error = %v (%T), want *ConfigError", err, err)
			}
			if cfgErr.Field != tt.wantField {
				t.Errorf("Validate() ConfigError.Field = %v, want %v", cfgErr.Field, tt.wantField)
			}
```
Either add rows to `TestValidate` or a sibling `TestValidateOrgURL` (VALIDATION.md names the latter). Rows: accepted `https://dev.azure.com/example`, `.../example/`, `https://example.visualstudio.com`, mixed case; refused `http://`, userinfo, port, query, fragment, two path segments, empty segment, other host, `dev.azure.com` with no segment, `visualstudio.com` with a path.

---

### `cmd/adoBizInt/main.go` (controller entry)

**Analog:** self `run()` (l.94-139). Insertion points:
```go
	if err := cfg.Validate(); err != nil {
		...
		return exitUnknown
	}
	...
	client, err := ado.NewClient(cfg)
	if err != nil {
		log.Error("Failed to create ADO client", "error", err)
		fmt.Fprintf(stderrWriter, "Error connecting to Azure DevOps: %v\n", err)
		return exitUnknown
	}
	...
	if len(os.Args) > 1 {
		signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stopSignals()

		return dispatch(signalCtx, client, os.Args[1:])
	}
```
- Build the transport after `Validate` and before `ado.NewClient`; pass it in. The existing `NewClient` error branch already maps a startup host refusal to exit 3 (D-10a) with the `ConfigError` text naming the host.
- Summary line (D-05/D-06): capture `code := dispatch(...)`, call the renderer once with the transport's wait stats and the client's retry count on `stderrWriter`, then `return code`. Keep all of it inside the one `if len(os.Args) > 1 {` statement so `TestValidateGuardsCLIAndTUI`'s "exactly one statement calls dispatch" holds. Print on failed runs too (Open Question 9).
- TUI path: no stderr output (D-07); the transport logs to file only.
- All stderr writes go through `stderrWriter` (`dispatch.go:32-38`), never `os.Stderr` directly.

---

### `cmd/adoBizInt/throttle.go` (NEW or inline; renderer to stderr)

**Analog:** `cmd/adoBizInt/render.go:105-129` (`renderScope`), verbatim:
```go
// renderScope writes the read's fixed server-side scope as one line to the
// stderr writer (D-04), never to stdout. It prints even when the read returned
// zero rows: an empty result under an undisclosed filter is exactly audit W4.
// A nil scope (an unscoped read) writes nothing. Every value is a code constant
// from internal/ado, so no field needs sanitize. Under --json the envelope's
// scope object carries the same fact instead.
func renderScope(w io.Writer, s *ado.Scope) {
	if s == nil {
		return
	}
	var parts []string
	...
	_, _ = io.WriteString(w, "scope: "+strings.Join(parts, "; ")+"\n")
}
```
Copy: `io.Writer` parameter, early return when there is nothing to say, a single write, a doc comment naming the decision IDs (D-05, D-06) and why stdout is untouched. Body per RESEARCH Code Example 7 (`throttled by ADO: 4 waits, 31s total, 1 retry`). Numbers only, so no `sanitize` needed. If the renderer is called from exactly one place and stays a few lines, CONVENTIONS "Single-use code stays inline" allows it inside `main.go`; a separate file is justified only by its own test file.

---

### `cmd/adoBizInt/throttle_test.go` (NEW; test)

**Analog:** `TestRenderFailuresToStderrOnly` (`main_test.go:597-621`) and the `captureOutput` helper (`main_test.go:56-71`). Calling helpers defined in the locked `main_test.go` is fine; editing that file is not.
```go
func captureOutput(t *testing.T) (stdout, stderr *bytes.Buffer) {
	t.Helper()

	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}

	prevOut, prevErr := stdoutWriter, stderrWriter
	stdoutWriter, stderrWriter = stdout, stderr
	t.Cleanup(func() {
		stdoutWriter, stderrWriter = prevOut, prevErr
	})

	return stdout, stderr
}
```
For a pure renderer, writing into a local `bytes.Buffer` is enough. Table rows: zero waits and zero retries -> empty; waits only; retries only; singular vs plural; rounding to whole seconds. One row asserts nothing reaches the stdout buffer.

---

### `scripts/smoke.sh` (NEW; script, batch) -- no analog

No shell script exists in the repo (`git ls-files '*.sh'` is empty). Use RESEARCH Pattern 7 and Pitfalls 11, 12, 14:
- `/bin/bash` 3.2 syntax (no associative arrays, no `mapfile`); `jq` for `.complete` / `.write.outcome`.
- Build a fresh binary to a temp dir and assert `go version -m` shows `github.com/microsoft/azure-devops-go-api/azuredevops/v7	v7.1.0` and `go1.27.2`; never use `adoBizInt` from `PATH`.
- Reads (exit 0 and `.complete == true`): `workitems list`, `dashboard counts`, `prs list`, `branches list`, `pipelines runs`, `pipelines definitions`, `releases list`, `releases approvals`, all with `--json`.
- Dry runs (no `--confirm`): the six write keys in `cmd/adoBizInt/dispatch.go:150-167` (`workitems set-state`, `prs approve`, `prs complete`, `releases approve`, `releases reject`, `pipelines queue`), targets from `SMOKE_*` env vars; exit 4 recorded as "pre-read exercised, no diff"; unset target is SKIP with a reason.
- No `set -x`; never print env, config or headers; print only command, exit code, `complete`/`outcome`, and the tool's own stderr. Never run in CI (D-17). Validate with `bash -n scripts/smoke.sh`.

---

### `.planning/phases/08-platform-upgrade-and-pacing/08-SMOKE.md` (NEW; record + TUI checklist)

**Analog:** `.planning/phases/07-guarded-tui-writes/07-UAT.md` (frontmatter, then numbered tests each with `expected:` and `result: [pending]`):
```markdown
---
status: testing
phase: 07-guarded-tui-writes
source: [07-VERIFICATION.md]
started: 2026-10-16T14:04:11Z
updated: 2026-10-16T14:04:11Z
---
...
### 5. Close an Active Task with n: leave Original Estimate blank, set Remaining 0 and Completed > 0, submit, move down, press y
expected: One confirm modal lists the state and the two effort changes, with no Original Estimate line. ...
result: [pending]
```
Sections: binary provenance line (`go version -m` output), pasted script table, the D-16 sub-client map with the dormant `work` gap named explicitly, the manual checklist (CLI `workitems set-state --confirm` on the scratch item; TUI Task close with effort, IN-01; TUI attachment download; TUI confirm modals dismissed with n/esc), and a findings table split into "blocks Phase 9 (v7/pacer/host-rule regression)" vs "pre-existing, recorded in BACKLOG" (D-15). No em dashes, no emojis.

---

### `.planning/BACKLOG.md` (append non-blocking findings)

**Analog:** §BL-02 (l.73-98). Append bullets under "Follow-ups found during ..." using the existing style; strike through closed items as `~~item~~ -- **closed <date>** by <ref>` (l.88 uses that form). Candidates already known: the `go.mod` indirect item (l.91) closes as a PLAT-02 side effect; the startup bad-PAT exit 3 (Pitfall 3) and the eager-identity recovery check (Open Question 8) carry forward.

---

## Shared Patterns

### Structural classification, never error text
**Source:** `internal/ado/result.go:164-169`, `internal/guard/guard.go:239-244`, `cmd/adoBizInt/exit.go:18-21`
**Apply to:** `result.go`, `guard.go`, `retry.go`, `transport.go`
```go
// It classifies structurally, never by matching the error text (D-06):
// WrappedError.Error() returns the server-controlled, localisable message and
// can be an HTML fragment, so string matching would be both fragile and a
// spoofing surface.
```
Every new branch uses `errors.Is` / `errors.As` / `statusCodeOf`. Tests never assert on server text, only on kind/outcome and on code-built messages (D-08).

### Exit-code contract (unchanged)
**Source:** `cmd/adoBizInt/dispatch.go:20-30`, `cmd/adoBizInt/exit.go:54-66`
**Apply to:** guard and CLI plans (D-02)
```go
func writeExitCodeFor(outcome string, err error) int {
	switch outcome {
	case outcomeExecuted:
		return exitOK
	case outcomeFailed:
		if ado.ClassifyFailure(err) == ado.FailureAuth {
			return exitAuth
		}
		return exitPartial
	default:
		return exitOutcomeUnknown
	}
}
```
Not-sent writes reach exit 1 through `failed` with no change here. Never use exit 4 (`exitNoOp`) for throttling. A rate-limited **pre-read** is `Report.PreReadErr` -> exit 3 (`write.go:180-183`, Pitfall 16); SC3 write tests must target the mutate step.

### stdout purity and logging
**Source:** `cmd/adoBizInt/dispatch.go:32-38` (`stderrWriter`), `internal/logging/logging.go:50-57` (`Logger()`), `internal/guard/structural_test.go:165-208` (`forbiddenOutput`)
**Apply to:** `transport.go` (log file only), `main.go`/`throttle.go` (stderr via `stderrWriter`), `guard.go` (no new output)

### AST over grep for structural guarantees
**Source:** `internal/ado/nilguard_test.go:136-159` (`parsePackageFiles`), `internal/ado/attachments_test.go:406-449`
**Apply to:** `construct_test.go`, `retry_test.go`, any transport "never prints" test
```go
func parsePackageFiles(t *testing.T) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	names, err := filepath.Glob("*.go")
	...
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		...
		files[name] = file
	}
	return fset, files
}
```
Every structural test ends with a non-vacuity check (`if files["attachments.go"] == nil { t.Errorf(...) }`, `if found != 2 { ... }`, `if scanned < 3 { t.Fatalf(...) }`).

### Fail-fast fakes for the narrow interfaces
**Source:** `internal/ado/fakes_test.go:45-62`
**Apply to:** `retry_test.go`, any new `internal/ado` unit test that does not need the real SDK
```go
func (f *fakeWorkItemAPI) UpdateWorkItem(ctx context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
	f.t.Helper()
	if f.UpdateWorkItemFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.UpdateWorkItem")
	}
	return f.UpdateWorkItemFunc(ctx, args)
}
```

### Doc comments carry the decision ID and the reason
**Source:** every analog above (`// D-13. This is the load-bearing line ...`, `// P-13: a transport error carrying no HTTP status ...`)
**Apply to:** all new and modified code. Cite D-01..D-16 from 08-CONTEXT.md and the RESEARCH pitfall numbers. Prose: no emojis, no em dashes.

### Commits
**Source:** `git log --oneline` (`feat(07-09): ...`, `test(07-09): ...`, `docs(08): ...`), CONVENTIONS "Commits"
**Apply to:** every task. `<type>(<scope>): <description>`, lowercase, imperative, max 72 chars, no `Co-Authored-By`. Branch is `dev`; no branch operations. PLAT-02 is one commit (for example `build(deps): move to azuredevops v7 and go 1.27.2`).

---

## No Analog Found

| File | Role | Data Flow | Reason | Use Instead |
|------|------|-----------|--------|-------------|
| `internal/transport/transport.go` (RoundTripper core) | middleware | request-response | No `net/http` code exists in the repo; no sentinel errors exist | RESEARCH Code Examples 3-4, Patterns 3-4; package conventions from fetchctx/guard listed above |
| `internal/transport/transport_test.go` (synctest pacing) | test | request-response | No `testing/synctest` or RoundTripper test exists | RESEARCH Code Example 6 |
| `internal/ado/v7_test.go` (real SDK over fake RT) | test | request-response | Every existing ado test uses narrow-interface fakes, never a real `ClientImpl` | RESEARCH Code Example 2, Pitfall 6 |
| `scripts/smoke.sh` | script | batch | No shell scripts in the repo | RESEARCH Pattern 7, Pitfalls 11, 12, 14 |

## Metadata

**Analog search scope:** `cmd/adoBizInt/`, `internal/ado/`, `internal/guard/`, `internal/config/`, `internal/fetchctx/`, `internal/logging/`, `internal/ui/confirm/`, `internal/utils/`, `.github/workflows/`, `go.mod`, `.planning/phases/07-guarded-tui-writes/`, `.planning/BACKLOG.md`, `.planning/PROJECT.md`, `.planning/codebase/CONVENTIONS.md`
**Files scanned:** 34 read or grepped
**Pattern extraction date:** 2026-10-19
