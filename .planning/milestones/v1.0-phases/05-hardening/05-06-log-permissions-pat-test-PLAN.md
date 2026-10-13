---
phase: 05-hardening
plan: 06
type: execute
wave: 2
depends_on: ["05-04"]
files_modified:
  - internal/logging/logging.go
  - internal/logging/logging_test.go
  - cmd/adoBizInt/patleak_test.go
autonomous: true
requirements: [HARD-06]

estimate:
  tokens: 35000
  raw_tokens: 35000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "On a fresh install, `logging.Init` creates `~/.config/adoBizInt` with mode `0700` and `adoBizInt.log` with mode `0600` (HARD-06)."
    - "On an existing install whose directory is `0755` and whose log is `0644`, one `logging.Init` call leaves them `0700` and `0600`. `Init` chmods both on every call, not only at creation (D-20)."
    - "`Init` is idempotent. A second call on an already-tightened install succeeds and leaves `0700` and `0600`, so two processes starting at once converge on the same modes (edge-probe HARD-06/concurrency)."
    - "A failed chmod is returned from `Init`, and the opened file is closed first. `cmd/adoBizInt/main.go`'s `initLogging` already treats any `Init` error as a non-fatal stderr warning and falls back to `slog.Default()`, so it is unchanged (D-21)."
    - "Log level, rotation and size cap are unchanged (D-18). `cmd/adoBizInt/audit.go` still creates `audit.jsonl` with mode `0644`, but inside the now-`0700` directory, so other local users cannot reach it. Phase 4 transferred T-04-04 to this directory fix, and the file's own mode is deferred."
    - "`TestPATNeverLogged` drives a representative read (`prs list --json`), an executed write (`workitems set-state --confirm`) and a forced panic through `guard`, all through the cmd fakes, with `slog` captured into a buffer and `ADO_PAT` set to the fixture. Neither the raw fixture, nor `azuredevops.CreateBasicAuthHeaderValue(\"\", fixture)`, nor its base64 part appears anywhere in the captured bytes (HARD-06, D-19; edge-probe HARD-06/encoding)."
    - "`TestPATNeverLogged` is non-vacuous. The same buffer contains `Read complete`, `Write complete` and `panic recovered`, which proves each of the three paths logged into the capture (edge-probe HARD-06/empty)."
    - statement: "The test is a tripwire for a future slog attribute, not a detector of a present leak. Go renders stack-frame arguments as hex words, and no cmd dispatch path holds the PAT today (D-19, RESEARCH Pitfall 10). No acceptance criterion asserts that the stack currently leaks."
      verification: backstop
  prohibitions:
    - statement: "The PAT-absence test must never use a real PAT, and must never read the developer's real config or ADO_PAT. It sets the fixture value and points HOME at an empty temporary directory."
      status: resolved
      verification: test
  artifacts:
    - path: "internal/logging/logging.go"
      provides: "0700 directory, 0600 file, chmod on every Init"
      contains: "Chmod"
    - path: "internal/logging/logging_test.go"
      provides: "First tests in the package: fresh, loose-existing and repeat Init permission assertions"
      contains: "TestInitSetsOwnerOnlyPermissions"
    - path: "cmd/adoBizInt/patleak_test.go"
      provides: "PAT-absence regression across read, write and panic"
      contains: "TestPATNeverLogged"
  key_links:
    - from: "internal/logging/logging.go"
      to: "cmd/adoBizInt/main.go"
      via: "Init's returned chmod error flows into initLogging's existing non-fatal warning (D-21)"
      pattern: "logging\\.Init\\(\\)"
    - from: "cmd/adoBizInt/patleak_test.go"
      to: "cmd/adoBizInt/crash.go"
      via: "loggingReady = true makes handlePanic write the panic stack into the captured slog.Default()"
      pattern: "loggingReady = true"
---

<objective>
Close CONCERNS.md MED2's permission finding and the Phase 3 threat T-03-15. The log file
becomes `0600` and its directory, which also holds the PAT-bearing config file, becomes
`0700` on every start. A regression test asserts that the PAT, raw or in its Basic-auth
form, never appears in anything logged.

Purpose: every existing install has a world-readable directory and log file today, and a
mode passed only at creation would never repair them.
Output: a three-line change to `logging.Init`, the package's first test file, and
`cmd/adoBizInt/patleak_test.go`.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/05-hardening/05-CONTEXT.md
@.planning/phases/05-hardening/05-RESEARCH.md
@.planning/phases/05-hardening/05-PATTERNS.md
@.planning/phases/05-hardening/05-04-SUMMARY.md
</context>

## Artifacts this phase produces

Phase 05 creates or changes these symbols. None of them exists yet. **This plan owns the 05-06 row.**

| Plan | Symbol / file | Kind |
|---|---|---|
| 05-01 | `internal/ado/attachments.go`; `DownloadAttachment(ctx, id uuid.UUID, destDir, name string) error`; `attachmentPath(name, destDir string) (string, error)`; `Attachment.ID`; `workItemAPI.GetAttachmentContent`; `fakeWorkItemAPI.GetAttachmentContentFunc` | new / replaced |
| 05-01 | removed: `Client.connection`, `AttachmentDownloadedMsg.Path`, URL-based `DownloadAttachment` | deletions |
| 05-01 | six attachment tests incl. `TestNoRawHTTPInADO` | new tests |
| 05-02 | `internal/ado/nilguard_test.go` with three tests | new tests |
| 05-03 | `.github/workflows/ci.yml` | new file |
| 05-04 | `var configuredProjects []string`; `--project` via `fs.Func`; `withConfiguredProjects`; `TestUnknownProjectRejected` | new |
| 05-05 | `func iterationClause(paths []string) string`; `internal/ado/wiql_test.go` with four tests | new |
| **05-06** | **`internal/logging/logging_test.go`: `TestInitSetsOwnerOnlyPermissions`; `cmd/adoBizInt/patleak_test.go`: `TestPATNeverLogged`** | **new tests (`logging.Init` changes behaviour; no new production symbol)** |

## Planner decisions recorded in this plan

| Item | Decision | Rationale |
|---|---|---|
| `cmd/adoBizInt/audit.go:77,80`, where the audit directory is created `0755` and the file `0644` (pattern-mapper follow-up) | **Deferred. Not in this plan.** | D-18 locks the scope to exactly SC6: the log file, its directory, and the test. Phase 4's T-04-04 transferred the audit-file exposure to this phase's directory fix, and that transfer is discharged here: the file sits inside `~/.config/adoBizInt`, which `Init` now chmods to `0700` on every run, before any write verb can open the audit sink. The residual gap is that `openAuditSink`'s `MkdirAll(..., 0755)` could create the directory if `Init` failed entirely. That is accepted as T-05-26 and recorded as a follow-up for the plan summary and STATE.md. |
| PAT sentinel | **Reuse `patFixture`** (`cmd/adoBizInt/crash_test.go:163`) | It is already a distinctive, never-real value in the same package, and `panickingClient` already closes over a config that carries it. A second constant would duplicate it. |
| Write path in the PAT test | **An executed write** (`--confirm`), not a dry run | A dry run logs nothing, so the write path would be vacuous in the capture. `Write complete` is logged only on an executed write. |
| Where the test lives | `cmd/adoBizInt` only | The panic handler and the cmd dispatch paths live there. `internal/ado` holds no credential after 05-01 deletes `Client.connection`, so an `internal/ado` twin would be vacuous. D-19's "through the Phase 1 fake" is read as the cmd-level fakes built on that seam. |
| Directory chmod target | Only `~/.config/adoBizInt` | `MkdirAll(..., 0o700)` also creates `~/.config` as `0700` when it is missing, which is more restrictive and harmless. The explicit `Chmod` touches only the app's own directory, never a shared parent. |

## Edge-probe and prohibition-probe disposition (spec-less fallback)

| Probe item | Disposition |
|---|---|
| HARD-06 / empty (captured log is empty, so the test passes vacuously) | resolved (explicit). The per-path non-vacuity markers are truth 7. |
| HARD-06 / encoding (raw vs Basic-header vs base64 form of the PAT) | resolved (explicit). All three forms are searched (truth 6, RESEARCH Pitfall 8). |
| HARD-06 / concurrency (two processes run `Init` at once) | resolved (explicit). Chmod to fixed modes is idempotent, pinned by the repeat-Init subtest (truth 3). |
| Prohibition recall | One kept (no real PAT). Credential logging and log-file permissions are canon (OWASP logging), covered by `/gsd-secure-phase`. |

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1: logging.Init leaves the directory 0700 and the log 0600 on every start</name>
  <files>internal/logging/logging.go, internal/logging/logging_test.go</files>
  <read_first>
    - internal/logging/logging.go (whole file, 49 lines: `Init` at :11-34, `Logger` at :36-43, the package-level `logger` at :9)
    - cmd/adoBizInt/main.go:44-60 (`initLogging`, the D-21 caller that must not change)
    - cmd/adoBizInt/main_test.go:199-206 (the HOME-override idiom to invert)
    - .planning/phases/05-hardening/05-RESEARCH.md § "Log permissions test" and § Pitfall 9
  </read_first>
  <behavior>
    - `TestInitSetsOwnerOnlyPermissions`, skipped on Windows. Each subtest sets `HOME` to `t.TempDir()` and restores `logger` to nil in `t.Cleanup`.
      - "fresh install": no directory exists. After `Init()` returns nil, the directory's mode is `0700` and the log's is `0600`.
      - "existing loose install": the directory is pre-created and explicitly chmodded to `0755`, and the log is pre-created and chmodded to `0644`. After `Init()` returns nil, they are `0700` and `0600`.
      - "repeat init": `Init()` is called twice, both calls return nil, and the modes are `0700` and `0600`.
  </behavior>
  <action>
    Create `internal/logging/logging_test.go` as `package logging`. It is the package's first test file. Write the three subtests from `<behavior>` and see the "existing loose install" subtest fail, then implement.

    Pre-create the fixtures with an explicit `os.Chmod` after creating them, so the process umask cannot mask the loose modes. Assert with `Stat(...).Mode().Perm()`, and guard the whole test with `if runtime.GOOS == "windows" { t.Skip(...) }`, because POSIX modes are meaningless there.

    **`internal/logging/logging.go`** in `Init`:
    1. Change the `MkdirAll` mode to `0o700`.
    2. Immediately after it, call `os.Chmod(logDir, 0o700)` and return its error.
    3. Change the `OpenFile` mode to `0o600`.
    4. Immediately after a successful open, call `file.Chmod(0o600)`. Use the method on the open file rather than a second path-based chmod (RESEARCH Pitfall 9). On error, close the file, then return the chmod error.
    5. Leave the handler, level and everything else unchanged (D-18).

    Replace the one-line `Init` doc comment with a short "why" paragraph citing D-20 and D-21:
    - Modes passed to `MkdirAll` and `OpenFile` apply only at creation, so every existing install stays loose without an explicit chmod.
    - The directory also holds the PAT-bearing `config.yaml`, which makes it the higher-value half.
    - A chmod failure is returned, and `initLogging` treats it as the existing non-fatal warning.
  </action>
  <verify>
    <automated>go test -race -count=1 ./internal/logging -run TestInitSetsOwnerOnlyPermissions -v && go test -race ./cmd/adoBizInt/... && go vet ./...</automated>
    <fails_when>non-zero exit, or `no tests to run` or `[no test files]` for internal/logging, or fewer than three `--- PASS: TestInitSetsOwnerOnlyPermissions/` subtest lines, or a `--- SKIP` line on darwin or linux</fails_when>
  </verify>
  <acceptance_criteria>
    - `internal/logging/logging_test.go` exists, and all three subtests pass under `-race` on darwin.
    - Reverting only the added `os.Chmod(logDir, ...)` line makes the "existing loose install" subtest fail. Revert after confirming.
    - `logging.Init` still has the signature `func Init() error` (`go doc ./internal/logging Init`).
    - `cmd/adoBizInt/main.go` is unchanged by this plan.
    - `TestLoggingInitFailureKeepsStdoutClean` in `cmd/adoBizInt` still passes.
    - `gofmt -l .` prints nothing.
  </acceptance_criteria>
  <done>Every start tightens the log file and its directory to owner-only access. Committed.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: The PAT never appears in captured log output across a read, a write and a panic</name>
  <files>cmd/adoBizInt/patleak_test.go</files>
  <read_first>
    - cmd/adoBizInt/crash.go:70-99 (`loggingReady`, and `handlePanic` writing `panic recovered` plus `debug.Stack()` through `logging.Logger()`)
    - cmd/adoBizInt/crash_test.go:160-181 (`patFixture`, `panickingClient`) and :233-262 (`TestPanicRecordCarriesNoStackOrCredential`, the assert-absent shape to follow)
    - cmd/adoBizInt/fakes_test.go:101-130 (`allReadsFake`) and :147-160 (`fakeWriteClient`)
    - cmd/adoBizInt/write_test.go:20-65 (`tempAuditSink`, `preReadFake`, `setStateArgs`)
    - cmd/adoBizInt/main_test.go (`captureOutput` at :55, `withReadBudget` at :694, and `withConfiguredProjects` as added by 05-04)
    - cmd/adoBizInt/dispatch.go:307 (`Read complete`) and cmd/adoBizInt/write.go:273 (`Write complete`), the non-vacuity markers
    - .planning/phases/05-hardening/05-RESEARCH.md § "PAT-absence test (cmd), shape" and § Pitfall 8, § Pitfall 10
  </read_first>
  <behavior>
    - Set `ADO_PAT` to `patFixture`, set `HOME` to `t.TempDir()`, capture `slog` into a buffer, and set `loggingReady` to true (restored afterwards). Then:
      - `dispatch(ctx, allReadsFake(t), []string{"prs", "list", "--json"})` returns 0.
      - A `preReadFake(t, "Active")` whose `fakeWriteClient.UpdateWorkItemStateFunc` returns nil, driven with `setStateArgs("--confirm")`, returns 0.
      - `guard(func() int { return dispatch(ctx, panickingClient(t), []string{"prs", "list", "--json"}) })` returns 3.
    - The buffer contains none of `patFixture`, `azuredevops.CreateBasicAuthHeaderValue("", patFixture)`, or `base64.StdEncoding.EncodeToString([]byte(":" + patFixture))`.
    - The buffer contains `Read complete`, `Write complete` and `panic recovered`.
  </behavior>
  <action>
    Create `cmd/adoBizInt/patleak_test.go` with `TestPATNeverLogged`, as specified in `<behavior>`.

    Set-up, in order:
    1. `t.Setenv` for `ADO_PAT` and `HOME`.
    2. Save `slog.Default()` and install `slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))`, restoring in `t.Cleanup`. `slog.SetDefault` also redirects the stdlib `log` package, so restoring matters.
    3. Save `loggingReady`, set it to true, and restore it in `t.Cleanup`.
    4. Call `withReadBudget(t, 2*time.Minute)`, `withConfiguredProjects(t, "P")` (from 05-04), `tempAuditSink(t)` and `captureOutput(t)`.

    Do **not** call `initLogging()` or `logging.Init()`. A successful `Init` would point `logging.Logger()` at a file instead of `slog.Default()`, and the capture would silently go vacuous (RESEARCH Pitfall 8). The non-vacuity markers are what would catch that. Do not add `t.Parallel()`; no cmd test uses it.

    The doc comment must state three things honestly:
    - It is a tripwire for a future careless `slog` attribute (D-19).
    - Go renders stack-frame arguments as hex words, so the panic stack is not expected to contain the PAT today.
    - After 05-01 no dispatch-reachable value holds the PAT (RESEARCH Pitfall 10).

    It must not claim the test detects a present leak. It must also say why `ADO_PAT` and `HOME` are set: any future dispatch-reachable `config.Load` would carry the fixture rather than the developer's real token (organization rule: never a real secret in a test).

    Search all three credential forms, per Pitfall 8.
  </action>
  <verify>
    <automated>go test -race -count=1 ./cmd/adoBizInt -run 'TestPATNeverLogged' -v && go test -race ./... && go vet ./...</automated>
    <fails_when>non-zero exit, or `no tests to run`, or the -v output lacks `--- PASS: TestPATNeverLogged`</fails_when>
  </verify>
  <acceptance_criteria>
    - `TestPATNeverLogged` passes under `-race`.
    - Temporarily adding a `"pat"` attribute carrying `patFixture` to the `Read complete` log call at `dispatch.go:307` makes the test fail and name the raw-form needle. Revert after confirming.
    - Temporarily deleting the `loggingReady = true` line makes the test fail on the missing `panic recovered` marker. Revert after confirming; this proves the non-vacuity check bites.
    - The test file references `patFixture` and declares no second PAT-like constant.
    - `go test -race ./...` and `go vet ./...` exit 0, and `gofmt -l .` prints nothing.
  </acceptance_criteria>
  <done>A future log line carrying the PAT, raw or as a Basic header, fails the suite on every CI run.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Process → `~/.config/adoBizInt/` | The log, the audit file and the PAT-bearing `config.yaml` persist here |
| Other local users → that directory | This is the disclosure boundary the modes defend |
| Any code path → `slog` | Any attribute logged persists to the file |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-05-23 | Information Disclosure | A world-readable config directory and log file on every existing install (MED2, T-03-15) | high | mitigate | `MkdirAll` and `OpenFile` modes become `0o700` and `0o600`, plus an explicit chmod of both on every `Init` (D-20). Pinned by all three `TestInitSetsOwnerOnlyPermissions` subtests. |
| T-05-24 | Information Disclosure | A future `slog` attribute rendering the PAT into the log | medium | mitigate | `TestPATNeverLogged` searches the raw, Basic-header and base64 forms across read, write and panic, with per-path non-vacuity markers (D-19). |
| T-05-25 | Denial of Service | A chmod failure breaking startup | low | accept | D-21. The error is returned, and the existing `initLogging` policy warns on stderr and continues with `slog.Default()`. This introduces no new failure mode. |
| T-05-26 | Information Disclosure | `audit.jsonl` created `0644` by `cmd/adoBizInt/audit.go` | low | accept | It sits inside the directory `Init` chmods to `0700` before any write verb runs, which discharges the T-04-04 transfer. The file's own mode is deferred under D-18 and recorded as a follow-up. |
| T-05-SC | Tampering | npm/pip/cargo installs | high | mitigate | Not applicable. No package install occurs. |
</threat_model>

<verification>
- `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"`
- `go test ./internal/logging ./cmd/adoBizInt -run 'TestInitSetsOwnerOnlyPermissions|TestPATNeverLogged' -v` shows both passing.
- Phase gate, after this plan and 05-05 land: the full-suite command is green locally, and the pushed phase branch gets a green `ci` run (05-03's human-check).
</verification>

<success_criteria>
- ROADMAP Phase 5 SC6 holds: log `0600`, directory `0700`, and a test asserting the PAT never appears in log output.
- The 05-VALIDATION.md HARD-06 rows are green.
- The plan summary and STATE.md record the `audit.go` file-mode follow-up as deferred.
</success_criteria>

<output>
Create `.planning/phases/05-hardening/05-06-SUMMARY.md` when done
</output>
