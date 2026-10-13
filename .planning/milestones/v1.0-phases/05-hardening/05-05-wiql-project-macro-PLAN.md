---
phase: 05-hardening
plan: 05
type: execute
wave: 2
depends_on: ["05-01"]
files_modified:
  - internal/ado/client.go
  - internal/ado/workitems.go
  - internal/ado/dashboard.go
  - internal/ado/wiql_test.go
autonomous: true
requirements: [HARD-04]

estimate:
  tokens: 45000
  raw_tokens: 45000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "Both WIQL sites filter on `[System.TeamProject] = @project`. The project name never enters the query text. It travels only in `QueryByWiqlArgs.Project`, which the SDK places in the request route, where `@project` resolves (HARD-04, D-10 revised)."
    - "For a project named `O'Brien`, the query captured at each site contains `@project` and contains neither `O'Brien` nor `O''Brien`, and the captured `args.Project` is `O'Brien` (D-10 revised)."
    - "Iteration-path values, which are server-supplied, still enter the query only through `iterationClause`, which doubles every single quote. `iterationClause(nil)` and `iterationClause([]string{})` return an empty string. `iterationClause` preserves input order (D-10 revised; edge-probe HARD-04/empty and ordering)."
    - "Neither site logs the assembled query. `Executing WIQL query` and `Executing dashboard WIQL query` carry only the `project` and `iterations` (count) attributes, and captured log output from both reads contains no `FROM WorkItems` (D-11)."
    - "`TestWIQLInterpolationStructural` fails the build's tests in any of these cases: a `+` chain containing a `FROM WorkItems` literal has an operand other than a string literal or a call to `iterationClause`; a chain lacks `@project`; a chain contains `[System.TeamProject] = '`; a `FROM WorkItems` literal sits outside a checked chain, which catches the `fmt.Sprintf` and split-literal evasions; or the site count is not exactly 2 (D-12, RESEARCH OQ2)."
    - "Behaviour is otherwise unchanged. When `GetRecentIterations` fails, the work-item read still warns and drops the clause, and the dashboard still records `opDashboardIterations`. The existing `workitems_test.go` and `dashboard_test.go` suites pass unedited."
    - statement: "For one configured project, `workitems list --json` item count and `dashboard counts --json` work_items count from a binary built at this plan's base commit equal those from a binary built after it, both reads reporting complete: true. PHASE-BLOCKING: Phase 5 does not count as done until this holds, whatever the automated gates say (D-10 revised). Checked by the Task 2 human-check."
      verification: backstop
  prohibitions:
    - statement: "The assembled WIQL query text must never be written to the log, and the project name must never be concatenated into query text at any current or future WIQL site (D-10 revised, D-11)."
      status: resolved
      verification: test
  artifacts:
    - path: "internal/ado/client.go"
      provides: "iterationClause, the single escaping path for iteration-path values"
      contains: "func iterationClause("
    - path: "internal/ado/wiql_test.go"
      provides: "Both site tests, the iterationClause unit test, and the D-12 structural test"
      contains: "TestWIQLInterpolationStructural"
    - path: "internal/ado/workitems.go"
      provides: "Work-item WIQL using @project"
      contains: "[System.TeamProject] = @project"
    - path: "internal/ado/dashboard.go"
      provides: "Dashboard WIQL using @project"
      contains: "[System.TeamProject] = @project"
  key_links:
    - from: "internal/ado/workitems.go"
      to: "internal/ado/client.go"
      via: "the WIQL + chain calls iterationClause(iterations) inline; no bare identifier operand"
      pattern: "\\+ iterationClause\\(iterations\\)"
    - from: "internal/ado/dashboard.go"
      to: "internal/ado/client.go"
      via: "the WIQL + chain calls iterationClause(iterations) inline"
      pattern: "\\+ iterationClause\\(iterations\\)"
    - from: "internal/ado/workitems.go"
      to: "SDK QueryByWiql route"
      via: "Project: &project stays on QueryByWiqlArgs; it is what @project resolves from"
      pattern: "Project: &project"
---

<objective>
Close CONCERNS.md H5 in the library. Both WIQL queries stop concatenating the project
name and use the `@project` macro, resolved from the project the SDK already sends in the
request route. They stop logging the assembled query. A go/ast test keeps every current
and future WIQL site in that shape.

Purpose: Uniti's rule is parameterized queries, never string concatenation (D-10 revised,
chosen by the user on 2026-09-30). Escaping would still concatenate. Plan 05-04 is the
cmd policy layer. This plan makes `internal/ado` safe even for callers, such as the TUI,
that never run that check.
Output: `iterationClause`, two rewritten query sites, and `internal/ado/wiql_test.go`.
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
@.planning/phases/05-hardening/05-01-SUMMARY.md
</context>

## Artifacts this phase produces

Phase 05 creates or changes these symbols. None of them exists yet. **This plan owns the 05-05 rows.**

| Plan | Symbol / file | Kind |
|---|---|---|
| 05-01 | `internal/ado/attachments.go`; `DownloadAttachment(ctx, id uuid.UUID, destDir, name string) error`; `attachmentPath(name, destDir string) (string, error)`; `Attachment.ID`; `workItemAPI.GetAttachmentContent`; `fakeWorkItemAPI.GetAttachmentContentFunc` | new / replaced |
| 05-01 | removed: `Client.connection`, `AttachmentDownloadedMsg.Path`, URL-based `DownloadAttachment` | deletions |
| 05-01 | six attachment tests incl. `TestNoRawHTTPInADO` | new tests |
| 05-02 | `internal/ado/nilguard_test.go` with three tests | new tests |
| 05-03 | `.github/workflows/ci.yml` | new file |
| 05-04 | `var configuredProjects []string`; `--project` via `fs.Func`; `withConfiguredProjects`; `TestUnknownProjectRejected` | new |
| **05-05** | **`func iterationClause(paths []string) string` in `internal/ado/client.go`** | **new, unexported (two uses; the D-12-forced exception)** |
| **05-05** | **`internal/ado/wiql_test.go`: `TestGetWorkItemsUsesProjectMacro`, `TestDashboardUsesProjectMacro`, `TestIterationClauseEscapesQuotes`, `TestWIQLInterpolationStructural`** | **new tests** |
| 05-06 | `TestInitSetsOwnerOnlyPermissions`; `TestPATNeverLogged` | new tests |

## Planner decisions recorded in this plan

| Item | Decision | Rationale |
|---|---|---|
| D-10 | **`@project` macro** (REVISED 2026-09-30; RESEARCH OQ1 resolved) | The organization's parameterized-query rule. `QueryByWiql` already puts `args.Project` in the route (`routeValues["project"]`), and both sites already pass `Project: &project`. |
| RESEARCH OQ2: what satisfies D-12's "call result" rule | **Allow-list the callee.** The only permitted non-literal operand is a call to `iterationClause`. | A rule that accepts "any call" passes a call that escapes nothing, such as `strings.TrimSpace(project)`. With `@project` there is no project operand left, so the allow-list is the single escaper. This tightens D-12 without contradicting it. |
| `iterationClause` extraction | **One private function in `client.go`, beside `GetRecentIterations`, called inline from both sites.** Carries a `// note:` comment. | Two uses is below the rule of three, but locked D-12 forces it: a bare `iterationFilter` identifier fails the structural rule (the RESEARCH Pitfall 4 prototype, re-run at planning time, reproduced four bare operands at `workitems.go:149` and `dashboard.go:142`). Recorded as a justified exception, like D-08. `client.go` pairs it with the function whose output it consumes. |
| The work-item `Using iteration filter` log line | **Kept** | It logs the server-supplied paths, not query text, so D-11 does not touch it and removing it is not required. |
| Log-capture code in `wiql_test.go` | **Duplicated in the two site tests, with a `// note:` comment**, not extracted | Two occurrences stay inline under the user's rule of three. |
| Live-org check | **One PHASE-BLOCKING `<human-check>` on Task 2 that covers both sites** (end-of-phase UAT) | D-10 revised requires one live read per site before the phase counts as done, so the check is marked phase-blocking in its text, not left as advisory UAT. It sits on Task 2 because Task 2 is an `auto` task, and the verifier harvests `<human-check>` only from `auto` tasks. On the Task 1 tracer, the check would halt an interactive run mid-flight and would be skipped in auto mode, so it would never reach UAT. The before/after binaries are built once, after both sites have changed. CI has no credentials (D-17). `human_verify_mode` is `end-of-phase`, so no mid-flight checkpoint is used. |

## Edge-probe and prohibition-probe disposition (spec-less fallback)

| Probe item | Disposition |
|---|---|
| HARD-04 / empty (no iterations) | resolved (explicit). `iterationClause` returns an empty string for a nil or empty slice, so the query equals the pre-change no-iteration form apart from `@project` (truth 3). |
| HARD-04 / ordering (iteration paths) | resolved (explicit). `iterationClause` emits the paths in input order (truth 3). |
| HARD-04 / adjacency | Covered by 05-04 (exact-match allow-list). Not applicable to the macro, which has no value adjacency. |
| Prohibition recall | One kept (see `must_haves.prohibitions`). WIQL injection itself is canon, covered by `/gsd-secure-phase`. |

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1: The work-item WIQL uses @project, escapes iterations through one call, and logs no query</name>
  <reversibility rating="reversible">D-10 revised: query text is internal to internal/ado; no published contract changes.</reversibility>
  <files>internal/ado/client.go, internal/ado/workitems.go, internal/ado/wiql_test.go</files>
  <read_first>
    - internal/ado/workitems.go (`getProjectWorkItems` at :131-175: the iteration loop at :135-146, the WIQL literal at :149, `Project: &project` at :150-153, and the query log line at :155)
    - internal/ado/client.go (`GetRecentIterations` at :169-255, which returns nil on error and on nil nodes; `strPtr` at :283)
    - internal/ado/workitems_test.go:85-137 (the `fakeWorkItemAPI` with `GetClassificationNodeFunc` and `QueryByWiqlFunc` to copy)
    - internal/logging/logging.go:36-43 (`Logger()` falls back to `slog.Default()` when uninitialised, which is the log-capture seam)
    - .planning/phases/05-hardening/05-RESEARCH.md § Pitfall 4 and § "D-12 structural test skeleton"
    - .planning/phases/05-hardening/05-PATTERNS.md § "workitems.go and dashboard.go — WIQL" and § "wiql_test.go"
    - .planning/phases/05-hardening/05-01-SUMMARY.md (confirm what 05-01 changed in `workitems.go` and `client.go`)
  </read_first>
  <behavior>
    - `TestIterationClauseEscapesQuotes`: `iterationClause(nil)` returns `""`. `iterationClause([]string{})` returns `""`. Given the two paths `Proj\Sprint 1` and `O'Brien\Sprint 2`, it returns ` AND [System.IterationPath] IN ('Proj\Sprint 1', 'O''Brien\Sprint 2')`: a leading space, input order, and the single quote doubled.
    - `TestGetWorkItemsUsesProjectMacro`: `&Client{projects: []string{"O'Brien"}, workitemClient: fake}`. `GetClassificationNodeFunc` returns `(nil, nil)`. `QueryByWiqlFunc` captures `*args.Wiql.Query` and `*args.Project` and returns an empty result. After `GetWorkItems(ctx, nil)`:
      - the query contains `[System.TeamProject] = @project`;
      - the query contains neither `O'Brien` nor `O''Brien`;
      - `args.Project` is `O'Brien`;
      - the captured `slog` output contains `Executing WIQL query` and no `FROM WorkItems`.
  </behavior>
  <action>
    Write both tests in the new `internal/ado/wiql_test.go` first and see them fail. `iterationClause` does not exist, and the query still carries the name. Then implement.

    **`internal/ado/client.go`**: directly after `GetRecentIterations`, add `func iterationClause(paths []string) string`. Return `""` for an empty slice. Otherwise, wrap each path in single quotes after `strings.ReplaceAll(path, "'", "''")`, join the quoted paths with `, `, and return ` AND [System.IterationPath] IN (` + joined + `)`. This moves the existing five-line loop that sits at `workitems.go:141-145` and `dashboard.go:133-137`.

    Its doc comment says three things. `// note:` two call sites, below the rule of three, extracted because D-12's structural test forbids a bare identifier operand in a WIQL chain. It is the only escaping path for iteration values (D-10 revised). It is the one callee `TestWIQLInterpolationStructural` allow-lists (RESEARCH OQ2).

    **`internal/ado/workitems.go`** in `getProjectWorkItems`:
    - Delete the `iterationFilter` variable and the escaping loop.
    - Keep the `GetRecentIterations` call. Keep its warn branch unchanged. Keep the `Using iteration filter` info line inside an `else if len(iterations) > 0` branch.
    - In the WIQL literal, replace `[System.TeamProject] = '" + project + "'` with `[System.TeamProject] = @project` inside the literal, so the chain is literal, then `iterationClause(iterations)`, then the literal ` ORDER BY [System.Id] DESC`.
    - Keep `Project: &project` on `QueryByWiqlArgs`. It is what `@project` resolves from.
    - Change the `Executing WIQL query` log call to carry `"project", project, "iterations", len(iterations)` and no query text (D-11).

    **`internal/ado/wiql_test.go`**: capture logs by saving `slog.Default()`, installing `slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))`, and restoring in `t.Cleanup`. `logging.Logger()` returns `slog.Default()` because no test in this package calls `logging.Init`. Do not add `t.Parallel()`.
  </action>
  <verify>
    <automated>go test -race ./internal/ado -run 'TestIterationClauseEscapesQuotes|TestGetWorkItemsUsesProjectMacro|TestGetWorkItems' -v && go vet ./...</automated>
    <fails_when>non-zero exit, or `no tests to run`, or the -v output lacks `--- PASS: TestGetWorkItemsUsesProjectMacro` or `--- PASS: TestIterationClauseEscapesQuotes`</fails_when>
  </verify>
  <acceptance_criteria>
    - `internal/ado/client.go` declares `func iterationClause(paths []string) string`, with a doc comment containing `note:` and citing D-12.
    - `internal/ado/workitems.go` no longer declares an `iterationFilter` variable, and its WIQL chain operands are a literal, `iterationClause(iterations)`, and a literal.
    - `TestIterationClauseEscapesQuotes`, `TestGetWorkItemsUsesProjectMacro` and every pre-existing `TestGetWorkItems*` test pass under `-race`, with no edit to `workitems_test.go`.
    - `gofmt -l .` prints nothing.
  </acceptance_criteria>
  <done>The work-item read queries by `@project` and logs no query text. Committed.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: The dashboard WIQL uses @project, and a go/ast test pins both sites</name>
  <files>internal/ado/dashboard.go, internal/ado/wiql_test.go</files>
  <read_first>
    - internal/ado/dashboard.go (`getProjectCounts` at :115-195: the iteration block with `appendFailure(..., opDashboardIterations, err)` at :121-139, the WIQL literal at :142, and the query log line at :148)
    - internal/ado/dashboard_test.go:177-232 (`dashboardClientFor`, and the `QueryByWiqlFunc` capture point at :202-212)
    - internal/ado/preread_test.go:272-330 (go/ast scaffolding, the `seen` non-vacuity check, and the doc-comment rationale)
    - internal/ado/wiql_test.go (as written in Task 1)
  </read_first>
  <behavior>
    - `TestDashboardUsesProjectMacro`: a `&Client{projects: []string{"O'Brien"}, ...}` whose `QueryByWiqlFunc` captures the query and project. `GetPullRequestsByProject` returns an empty list, and `pipelines` is empty, so `GetBuilds` is not called. The same four assertions as `TestGetWorkItemsUsesProjectMacro` hold, with the log message `Executing dashboard WIQL query`.
    - `TestWIQLInterpolationStructural`, over every non-test `.go` file in the package, parsed with `parser.SkipObjectResolution`:
      - Each outermost `+` chain that contains a string literal holding `FROM WorkItems` is a site.
      - Every operand of a site is an `*ast.BasicLit` or an `*ast.CallExpr` whose `Fun` is the identifier `iterationClause`.
      - Some literal in each site contains `@project`, and no literal in it contains `[System.TeamProject] = '`.
      - The total count of `FROM WorkItems` string literals in the package equals the count found inside sites.
      - The number of sites is exactly 2.
  </behavior>
  <action>
    Write `TestDashboardUsesProjectMacro` first and see it fail. Then edit **`internal/ado/dashboard.go`** in `getProjectCounts`:
    - Delete the `iterationFilter` variable and the escaping loop. Keep the `GetRecentIterations` call and its entire error branch, including the T-02-05 comment and `appendFailure`, unchanged.
    - Rewrite the WIQL literal to `[System.TeamProject] = @project`, followed by `+ iterationClause(iterations)`.
    - Keep `Project: &project`.
    - Change the `Executing dashboard WIQL query` log call to `"project", project, "iterations", len(iterations)` (D-11).

    Then add `TestWIQLInterpolationStructural` to `internal/ado/wiql_test.go`, implementing the rules in `<behavior>`:
    - Enumerate files with `filepath.Glob("*.go")`, skipping `_test.go` names. Do not use the deprecated `parser.ParseDir`.
    - Flatten `+` chains recursively over `token.ADD` binary expressions.
    - Return `false` from `ast.Inspect` after handling a site, so its sub-chains are not re-counted.
    - Count literals in a separate pass, so a literal in a `fmt.Sprintf` argument, a composite literal, or a standalone assignment makes the totals disagree. That disagreement is the evasion check (RESEARCH Pitfall 4, defeat vector 1).
    - Report violations with `fset.Position`.
    - Write a doc comment that states the rules, cites D-10 revised, D-12 and OQ2, and copies the grep-defeat rationale from `preread_test.go:283-288`.
    - Explain in one sentence why the count is exactly 2: a third WIQL site must fail until it is reviewed (D-12).
  </action>
  <verify>
    <automated>go test -race ./internal/ado -run 'TestDashboardUsesProjectMacro|TestWIQLInterpolationStructural|TestGetDashboardCounts|TestGetWorkItemsUsesProjectMacro|TestIterationClauseEscapesQuotes' -v && go test -race ./... && go vet ./...</automated>
    <fails_when>non-zero exit, or `no tests to run`, or the -v output lacks `--- PASS: TestWIQLInterpolationStructural` or `--- PASS: TestDashboardUsesProjectMacro`</fails_when>
    <human-check>
      <test>PHASE-BLOCKING (D-10 revised). Run this once both tasks have landed. Use your own configured PAT, on this machine only, and never paste it into the session. Pick one configured project P, then build a base-commit binary and a HEAD binary:
        `L=$(git rev-list --grep='(05-05)' HEAD) && B=$(printf '%s\n' "$L" | tail -1) && D=$(mktemp -d) && git worktree add --detach "$D/before" "$B^" && (cd "$D/before" && go build -o "$D/adoBizInt-before" ./cmd/adoBizInt) && go build -o "$D/adoBizInt-after" ./cmd/adoBizInt`. Here `B` is this plan's oldest commit, so `$B^` is the plan's base.
        Run these four commands back to back, in this order:
        - `"$D/adoBizInt-before" workitems list --project P --json | jq '[.complete, (.items|length)]'`
        - the same command with `"$D/adoBizInt-after"`
        - `"$D/adoBizInt-before" dashboard counts --project P --json | jq '[.complete, .items[0].work_items]'`
        - the same command with `"$D/adoBizInt-after"`
        Finish with `git worktree remove "$D/before"`.</test>
      <expected>
        - Each read prints `[true, N]` from both binaries, with the same N from both for that read.
        - Every run exits 0.
        - In `~/.config/adoBizInt/adoBizInt.log`, the last `Executing WIQL query` line and the last `Executing dashboard WIQL query` line are the ones the `-after` runs wrote. For P, each carries `iterations=` and contains no `FROM WorkItems` text. The `-before` lines still show the query, which is expected.
      </expected>
      <why_human>PHASE-BLOCKING. D-10 revised makes one live read per WIQL site a precondition for Phase 5 completion. If either read differs or fails, the phase is not done, even when every automated gate is green. `@project` has not been exercised against the live org (RESEARCH A3), and CI has no credentials (D-17). Both reads sit on this `auto` task, not on the Task 1 tracer, because the end-of-phase harvest collects human-check blocks only from `auto` tasks.</why_human>
    </human-check>
  </verify>
  <acceptance_criteria>
    - `TestDashboardUsesProjectMacro` and `TestWIQLInterpolationStructural` pass under `-race`, and every pre-existing dashboard test passes with no edit to `dashboard_test.go`.
    - Temporarily restoring `'" + project + "'` at either site makes `TestWIQLInterpolationStructural` fail and name that file and line. Revert after confirming.
    - Temporarily wrapping one site's WIQL in `fmt.Sprintf` makes the test fail on the literal-count mismatch. Revert after confirming.
    - Neither `workitems.go` nor `dashboard.go` declares `iterationFilter`.
    - `go test -race ./...` and `go vet ./...` exit 0, and `gofmt -l .` prints nothing.
  </acceptance_criteria>
  <done>Both WIQL sites are parameterized by `@project`, log no query text, and are held in that shape by an AST test.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Caller-supplied project name → `internal/ado` read | The TUI and CLI both pass project names, and the TUI never runs 05-04's check |
| ADO server → `GetRecentIterations` → WIQL | Iteration paths are server-supplied and concatenated into query text |
| `internal/ado` → log file | Log lines persist on disk |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-05-19 | Tampering / Information Disclosure | Project name concatenated into WIQL at both sites (H5) | high | mitigate | `[System.TeamProject] = @project`, with the name only in the SDK route (D-10 revised). Pinned by both `UsesProjectMacro` tests and by `TestWIQLInterpolationStructural` (no bare operand, no `= '` literal, exactly 2 sites, Sprintf evasion caught). |
| T-05-20 | Tampering | A server-supplied iteration path containing a quote | medium | mitigate | `iterationClause` doubles single quotes and is the only allow-listed non-literal operand. Pinned by `TestIterationClauseEscapesQuotes`. |
| T-05-21 | Information Disclosure | Assembled query text persisted to the log (H5 item 3, part of MED2) | medium | mitigate | Log attributes reduced to `project` and the iteration count (D-11). Both site tests assert captured logs contain no `FROM WorkItems`. |
| T-05-22 | Denial of Service (functional) | `@project` resolving differently on the live org, so reads return no items | medium | mitigate | A live before-and-after comparison of both sites is a phase-blocking human-check on Task 2 (D-10 revised, RESEARCH A3). `Project: &project` is retained in the route. |
| T-05-SC | Tampering | npm/pip/cargo installs | high | mitigate | Not applicable. No package install occurs. |
</threat_model>

<verification>
- `go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"`
- `go test ./internal/ado -run 'ProjectMacro|IterationClause|WIQLInterpolation' -v` shows four passing tests.
- End of phase: the Task 2 human-check, which covers both live reads, is harvested into UAT. It is PHASE-BLOCKING. Phase 5 is not complete until both reads match, whatever the automated gates say (D-10 revised).
</verification>

<success_criteria>
- The library half of ROADMAP Phase 5 SC4 holds: WIQL project interpolation is gone rather than merely escaped, per the organization's parameterized-query rule.
- The 05-VALIDATION.md HARD-04 structural and site rows are green. The live-read row is queued as a phase-blocking human-check, and it must pass before Phase 5 is marked complete.
</success_criteria>

<output>
Create `.planning/phases/05-hardening/05-05-SUMMARY.md` when done
</output>
