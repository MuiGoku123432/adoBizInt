---
phase: 06-core-value-cleanup
verified: 2026-10-12T00:00:00Z
status: passed
score: 50/50 must-haves verified (4/4 roadmap success criteria, 46/46 plan truths)
covered_files:
  - .planning/REQUIREMENTS.md
  - .planning/phases/06-core-value-cleanup/06-01-SUMMARY.md
  - .planning/phases/06-core-value-cleanup/06-01-help-token-position-PLAN.md
  - .planning/phases/06-core-value-cleanup/06-02-SUMMARY.md
  - .planning/phases/06-core-value-cleanup/06-02-reject-empty-projects-PLAN.md
  - .planning/phases/06-core-value-cleanup/06-03-SUMMARY.md
  - .planning/phases/06-core-value-cleanup/06-03-scope-in-envelope-PLAN.md
  - .planning/phases/06-core-value-cleanup/06-04-SUMMARY.md
  - .planning/phases/06-core-value-cleanup/06-04-scope-disclosure-cli-PLAN.md
  - .planning/phases/06-core-value-cleanup/06-05-SUMMARY.md
  - .planning/phases/06-core-value-cleanup/06-05-write-panic-outcome-unknown-PLAN.md
  - cmd/adoBizInt/crash.go
  - cmd/adoBizInt/dispatch.go
  - cmd/adoBizInt/flags.go
  - cmd/adoBizInt/main.go
  - cmd/adoBizInt/render.go
  - cmd/adoBizInt/usage.go
  - cmd/adoBizInt/write.go
  - internal/ado/client.go
  - internal/ado/dashboard.go
  - internal/ado/pullrequests.go
  - internal/ado/releases.go
  - internal/ado/result.go
  - internal/ado/workitems.go
  - internal/config/config.go
covered_digest: "v1:sha256:392117988898d4a83d15cf61cb311c7cb8cdcb533f4ae330f69206e4061bcac6"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 6: Core-value cleanup Verification Report

**Phase Goal:** The four v1.0 audit warnings that let an agent mistake a non-read, a misconfigured or filtered read, or an in-flight write for a complete answer are closed.
**Verified:** 2026-10-12 (HEAD c03bf0c, branch dev)
**Status:** passed
**Re-verification:** No. This is the first verification.

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A help token used as a flag value (for example `--search help`) is the value, not a help request. Only a help token in flag position prints help (W2). | VERIFIED | `usage.go` `handleUsage` parses a value-inert copy of the verb's FlagSet (`helpProbeValue`, `Set` is a no-op, `IsBoolFlag` forwarded) over `hoistLeadingPositionals(args[2:])`. Only `errors.Is(err, flag.ErrHelp)` selects help, and help is printed from a fresh `newFlagSet`. `TestHelpTokenAsFlagValueIsNotHelp` (16 rows, including the read running once for `--search help` and the write-side `--comment -h` / `--branch -h` rows) and `TestHelpTokenInFlagPositionIsHelp` (8 rows) pass. Offline binary run: `workitems list --search help` with no config falls through to config load and exits 3 ("pat: ... required"), so it is not help. `workitems list --search x -h` with no PAT exits 0 with empty stdout. Live orchestrator evidence: `workitems list --search help --json` exited 0 with complete:true and 13 items. |
| 2 | A configuration with no projects is rejected at startup with exit 3, so no read can report `complete: true` over zero projects (W3). | VERIFIED | `config.go` `Validate` returns `ConfigError{Field:"projects"}` for `len==0` and for any `strings.TrimSpace(p)==""` entry. These checks come after the pat and org_url checks. `run()` checks `cfg.Validate()` before `dispatch` and before `ui.NewModel` and returns `exitUnknown`; this ordering is pinned by the go/ast test `TestValidateGuardsCLIAndTUI`. `TestValidate` (15 rows) and `TestLoadRejectsBlankProjects` (8 load paths) pass. Offline binary runs: no args (TUI path) with no projects exits 3 with "Invalid config: projects: at least one project is required" and the ADO_PROJECTS hint. `ADO_PROJECTS=" , " workitems list --json` exits 3 with empty stdout. |
| 3 | The fixed server-side scope of `workitems list` (excluded states, recent iterations) and `prs list` (active only) is disclosed in help and in the JSON envelope (W4). | VERIFIED | `result.go`: `Result.Scope *Scope` is tagged `json:"scope"` with no omitempty. It is set on every return path of GetWorkItems (2 returns), GetDashboardCounts (2), GetPullRequests (1) and GetPendingApprovals (1), and left nil on the other four reads. PR and approval values come from the same SDK constants the requests send. The WIQL NOT IN literal is unchanged, and `TestGetWorkItemsScope` / `TestGetDashboardCountsScope` tie it to the scope. `usage.go` `verbScopes` adds a `Scope:` paragraph to help for the 4 scoped verbs. `dispatch.go` calls `renderScope(stderrWriter, result.Scope)` only in human mode. Live evidence: `workitems list --json` scope `{"excluded_states":["Done","Closed","Removed"],"iterations":"recent"}`, `prs list` stderr "scope: active pull requests only", `branches list --json` scope null, `releases approvals` `{"approval_status":"pending"}`, `dashboard counts` includes `pr_status:"active"`. |
| 4 | A panic after a write's request was issued exits 5 (outcome unknown) with a best-effort audit record, not exit 3 (W1). | VERIFIED | `write.go` `runWrite` has a named result `code`. A deferred recover sits immediately before `op.mutate` and after `defer sink.Close()`. If the normal append was not reached (`!audited`), it appends an `outcomeUnknown` record and then calls `handlePanic(..., exitOutcomeUnknown)`. `crash.go` `handlePanic(r any, code int) int`. `main.go` `guard` passes `exitUnknown`. Tests driven through `guard(dispatch(...))` pass: `TestPanicAfterWriteIssuedExitsOutcomeUnknown`, `TestPanicBeforeWriteIssuedExitsThree`, the go/ast pin `TestWriteRecoverPrecedesMutate`, and `TestPanicAfterWriteIssuedIsAudited` (4 rows: mutate panics, append fails, panic after normal append gives one record, cancelled parent). |

**Score:** 4/4 roadmap SCs. 46/46 plan must-have truths (06-01: 12, 06-02: 7, 06-03: 10, 06-04: 8, 06-05: 9) checked against code and named passing tests. 0 truths are present but behavior-unverified.

### Plan truths: notable checks beyond the SCs

- **06-01 D-17:** `runRead` returns `exitUnknown` on `len(fs.Args()) > 0` before any fetch and names the token with `%q`. `TestReadRejectsStrayPositional` covers all 8 read verbs, with the stray word first and last, plus `workitems list help --json`. Live evidence: `workitems list help` exits 3 with the expected message. Write-verb arity and `hoistLeadingPositionals` are unchanged.
- **06-01 D-09:** help detection still runs before `config.Load` in `run()`. The probe never applies values, so the `--project` allow-list is enforced only by dispatch's real parse (row "unconfigured project is still refused").
- **06-01 D-08:** `.planning/BACKLOG.md:81` shows WR-05 struck through as closed by 06-01.
- **06-03:** `TestNewResultScopeIsNull` and `TestUnscopedReadsReportNullScope` pin null on 4 reads. The source comments now say 30 days, not 90, matching `client.go:190` (`AddDate(0,0,-30)`). No "90 day" text remains in non-test source.
- **06-04:** the `--hide-done` text reads "also drop Resolved items client-side; Done, Closed and Removed are always excluded by Azure DevOps". This matches `ado.doneStates` = {Done, Closed, Resolved, Removed}. The filter itself is unchanged. `TestScopeLineOnStderrInHumanMode` (6 rows, including zero rows and scope before failures) and `TestScopeRidesInJSONEnvelopeOnly` pass.
- **06-05 D-14:** the `main.go` guard comment now reads "exit 3 (nothing ran, D-08) ... a panic inside a write after its request was issued ... exits 5 (D-12)". No comment in main.go or crash.go still calls 3 outcome-unknown.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/adoBizInt/usage.go` | stdlib help probe, `helpProbeValue`, `verbScopes` | VERIFIED | Substantive and wired (`handleUsage` is called from `run()`, and `usageVerb` prints `verbScopes[name]`) |
| `cmd/adoBizInt/dispatch.go` | stray-positional refusal; `renderScope` in the human branch | VERIFIED | Both present at the specified positions |
| `cmd/adoBizInt/render.go` | `renderScope` | VERIFIED | Writes nothing when scope is nil; writes one line otherwise |
| `cmd/adoBizInt/flags.go` | accurate `--hide-done` text | VERIFIED | Line 228-229 |
| `cmd/adoBizInt/write.go` | deferred recover before `op.mutate` | VERIFIED | Pinned by go/ast test |
| `cmd/adoBizInt/crash.go` | `handlePanic(r any, code int) int` | VERIFIED | Called from guard and from runWrite |
| `cmd/adoBizInt/main.go` | `handlePanic(r, exitUnknown)`; ADO_PROJECTS hint | VERIFIED | Present |
| `internal/config/config.go` | empty and blank projects rejected | VERIFIED | `Load` is unchanged, as the diff from ada2cb1 shows |
| `internal/ado/result.go` | `Scope` type; `Result.Scope` without omitempty | VERIFIED | Present |
| `internal/ado/{workitems,dashboard,pullrequests,releases}.go` | Scope set on every return | VERIFIED | Every return path checked |
| Tests (usage_test, writeverbs_test, main_test, crash_test, config_test, json_test, wiql_test, pullrequests_test, releases_test) | named tests | VERIFIED | All 22 named tests are listed by `go test -list` and pass |
| `.planning/BACKLOG.md` | WR-05 closed | VERIFIED | Line 81 |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `handleUsage` | `newFlagSet` | probe FlagSet, `errors.Is(err, flag.ErrHelp)` | WIRED |
| `handleUsage` help branch | `usageVerb` | fresh `newFlagSet(name)` | WIRED |
| `runRead` | `exitUnknown` | `len(fs.Args()) > 0` before fetch | WIRED |
| `run()` | `cfg.Validate()` | before `dispatch` and `ui.NewModel`, returns `exitUnknown` | WIRED (go/ast pinned) |
| `Validate` | `ConfigError{projects}` | `strings.TrimSpace(p) == ""` | WIRED |
| `GetWorkItems` / `GetDashboardCounts` / `GetPullRequests` / `GetPendingApprovals` | `Result.Scope` | `.Scope = ` before each return | WIRED |
| `GetPullRequests` scope | `git.PullRequestStatusValues.Active` | same constant as request | WIRED |
| `GetPendingApprovals` scope | `release.ApprovalStatusValues.Pending` | same constant as request | WIRED |
| `runRead` human branch | `renderScope` | `renderScope(stderrWriter, result.Scope)` | WIRED |
| `usageVerb` | `verbScopes` | `verbScopes[name]` | WIRED |
| `runWrite` deferred recover | `handlePanic` | `handlePanic(..., exitOutcomeUnknown)` | WIRED |
| `runWrite` deferred recover | `appendAuditRecord` | `Outcome: outcomeUnknown` when `!audited` | WIRED |
| `guard` | `handlePanic` | `handlePanic(r, exitUnknown)` | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `--json` envelope | `result.Scope` | Set by the ado read methods from code and SDK constants, then marshalled unwrapped by `emitJSON(result)` | Yes. Live evidence shows scope objects on 3 reads and null on branches | FLOWING |
| human stderr scope line | `result.Scope` | Same result, passed to `renderScope` | Yes. Live `prs list` stderr shows "scope: active pull requests only" | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full gate | `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"` | All packages ok; GATE_PASS, exit 0 | PASS |
| TUI refuses to start with no projects (VALIDATION manual item 2) | built binary, temp HOME, `ADO_PAT=dummy ADO_ORG_URL=https://example.invalid`, no args | exit 3, empty stdout, "Invalid config: projects: at least one project is required", hint names ADO_PROJECTS | PASS |
| Separator-only ADO_PROJECTS rejected for the CLI | `ADO_PROJECTS=" , " adoBizInt workitems list --json` | exit 3, empty stdout | PASS |
| Flag-position help needs no PAT and shows Scope | `adoBizInt workitems list --search x -h` (no config) | exit 0, empty stdout, one `Scope:` paragraph | PASS |
| Value-position help token is not help | `adoBizInt workitems list --search help` (no config) | Proceeds to config load, then exit 3 for the missing PAT | PASS |
| Live reads (orchestrator, read-only, Windstream) | `--search help --json`, `list help`, `prs list`, `branches list --json`, `releases approvals --json`, `dashboard counts --json` | Matches SC1 and SC3 expectations (see truths) | PASS (cited) |

None of these spot-checks made a live ADO call. Every offline check fails validation or prints help before `ado.NewClient` is reached.

### Probe Execution

Step 7c: not applicable. No probe scripts are declared, and no `scripts/*/tests/probe-*.sh` exist.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CLI-05 | 06-01, 06-04 | Every command responds to `--help` | SATISFIED | Help in flag position still works without config or a PAT. Scoped verbs state their scope. |
| CLI-06 | 06-01, 06-03, 06-04 | Agent filtering available as flags | SATISFIED | `--search help` filters as a search. `--hide-done` text is accurate. Scope is disclosed. |
| OUT-04 | 06-01, 06-02, 06-03, 06-05 | Exit codes distinguish success, partial, auth, outcome-unknown | SATISFIED | A stray positional or empty projects exits 3. A post-request panic exits 5. |
| OUT-05 | 06-05 | A panic emits structured JSON | SATISFIED | The crash record stays on stdout with the three-key shape, one document. |
| TRUST-03 | 06-02, 06-03, 06-04 | A partial result is marked incomplete | SATISFIED | Zero projects is rejected. Scope never changes Complete (tested). |
| SAFE-02 | 06-05 | Every executed write appends an audit record | SATISFIED | A best-effort unknown record is written on a post-request panic, never two records. |
| SAFE-04 | 06-05 | A timed-out write reports outcome-unknown | SATISFIED | A post-request panic, including one under a cancelled parent, exits 5 with an unknown record. |

No requirement is orphaned. REQUIREMENTS.md maps no ID to Phase 6, and the union of the plans' `requirements` equals the 7 phase IDs.

### Prohibitions (plan `must_haves.prohibitions`)

The plans list prohibitions as plain strings, without the ADR-550 `{statement, status, verification}` shape, so tier routing does not apply. Each prohibition was checked against the code:

- **06-01:** Pass. No hand-scan of argv. `helpProbeValue.Set` returns nil, so no value is applied. Help detection still precedes `config.Load`. Write-verb arity and hoisting are unchanged.
- **06-02:** Pass. `Validate` only rejects; it does not trim. There is no bypass key, env var or flag. `Load` is unchanged by this phase (diff ada2cb1..HEAD touches only `Validate`).
- **06-03:** Pass. No widening. The WIQL literal stays in the query. `Scope` has no omitempty (`TestNewResultScopeIsNull`). Values are constants. Complete is never derived from scope.
- **06-04:** Pass. The scope line is not written to stdout or under `--json` (`TestScopeRidesInJSONEnvelopeOnly`). `emitJSON(result)` is unwrapped. `--hide-done` behavior is unchanged. No "90 days" text remains.
- **06-05:** Pass. The crash record stays on stdout with exactly [error]. The audit record is built only from command, target, changes and outcome, with the five keys asserted. The `audited` local prevents a second record. The signal is the recover's statement position, not a package-level variable. The recover is not moved into guard. The read-path panic still exits 3 (`TestPanicExitsThree`, `TestPanicBeforeWriteIssuedExitsThree`).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (all phase-modified source) | - | TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER | - | None found |
| `cmd/adoBizInt/write.go` | 254-274 | A panic in `finishWrite` after a write ADO refused (`outcome` failed, `audited` false) appends an `unknown` record | Info | Over-reports in the safe direction. Documented in the code comment, consistent with D-13 and RESEARCH A4. |
| `cmd/adoBizInt/write.go` | 254-274 | A panic raised inside the recover's own `appendAuditRecord` would escape to `guard` and exit 3 | Info | The append writes a marshalled struct to a file and has no realistic panic site. It is outside the must-haves. |
| `cmd/adoBizInt/flags.go` | 201-213 | `--project` values are trimmed, while `Validate` accepts padded configured names as written, so a configured `" A "` cannot be selected with `--project` | Info | Existed before this phase and lies outside W1–W4. Fails closed with exit 3, never with a wrong read. |

### Human Verification Required

None. Both manual-only items from 06-VALIDATION.md are covered. The live `--search help --json` read is covered by the orchestrator's read-only evidence, and the TUI-refuses-to-start check was reproduced offline with the built binary (exit 3 before `ui.NewModel`).

### Gaps Summary

None. All four audit warnings are closed in code on HEAD, each backed by named passing tests and, where applicable, offline binary checks or live evidence:

- **W2:** help is decided by a stdlib parse over the verb's FlagSet, so a help token in a flag's value position is that flag's value. Stray positionals on read verbs exit 3.
- **W3:** an empty or blank projects list is rejected by `Validate` before the CLI dispatch and before the TUI starts.
- **W4:** `scope` is on all 8 envelopes: an object on the 4 scoped reads, null on the other 4. A human-mode stderr line and the verb help state the same scope.
- **W1:** a panic after a write's request is issued exits 5 and writes a best-effort `unknown` audit record. The crash record stays on stdout.

---

_Verified: 2026-10-12_
_Verifier: Claude (gsd-verifier)_
