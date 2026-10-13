---
phase: 06-core-value-cleanup
plan: 03
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/ado/result.go
  - internal/ado/workitems.go
  - internal/ado/client.go
  - internal/ado/dashboard.go
  - internal/ado/pullrequests.go
  - internal/ado/releases.go
  - internal/ado/json_test.go
  - internal/ado/wiql_test.go
  - internal/ado/pullrequests_test.go
  - internal/ado/releases_test.go
autonomous: true
requirements: [TRUST-03, OUT-04, CLI-06]

estimate:
  tokens: 46000
  raw_tokens: 46000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "Every ado.Result[T] envelope marshals a \"scope\" key on all 8 reads: an object when the read applies a fixed server-side filter, JSON null when it does not; the key is never absent (D-01, D-02)"
    - "workitems list reports scope {\"excluded_states\":[\"Done\",\"Closed\",\"Removed\"],\"iterations\":\"recent\"} exactly, on every return path including the no-projects early return (D-01, RESEARCH Pitfall 6) [edge TRUST-03/empty]"
    - "dashboard counts reports scope {\"excluded_states\":[\"Done\",\"Closed\",\"Removed\"],\"iterations\":\"recent\",\"pr_status\":\"active\"} exactly, on both return paths (D-01)"
    - "prs list reports scope {\"pr_status\":\"active\"} and releases approvals reports scope {\"approval_status\":\"pending\"} exactly (D-01, D-15)"
    - "branches list, pipelines runs, pipelines definitions and releases list report \"scope\":null, because latest-per-definition is the verb's purpose rather than a hidden filter (D-02, D-16)"
    - "The scope cannot drift from the request: the work-item and dashboard tests assert the captured WIQL contains NOT IN (' + the scope's excluded_states joined in order + '), and the PR and approval tests assert the captured SearchCriteria.Status and StatusFilter equal the scope's value, which is derived from the same SDK constant the request sends (D-01) [edge TRUST-03/ordering]"
    - "Scope never changes Complete or Failures: a scoped read with no failure stays complete:true, and a scoped read whose project failed keeps a non-null scope alongside complete:false (D-01, RESEARCH Pitfall 8) [edge TRUST-03/adjacency]"
    - "Each result gets its own Scope value; two calls never share one pointer, so a caller mutating one envelope's scope cannot alter another (D-01) [edge TRUST-03/concurrency]"
    - "Inside a scope object a key appears only when that filter applies (the D-01 examples are per-verb objects); a fully populated Scope marshals exactly the four keys excluded_states, iterations, pr_status and approval_status, and an empty Scope marshals {} (D-01)"
    - "The source comments that say recent iterations means 90 days are corrected to 30 days, matching client.go's cutoff, so the code and the help text 06-04 writes state the same window (D-06)"
  prohibitions:
    - "Must NOT add a flag, widen a WIQL clause, or change any request's status filter; this plan discloses scope only (D-03)"
    - "Must NOT set, clear or derive Complete or Failures from scope on any read (RESEARCH Pitfall 8)"
    - "Must NOT build the WIQL NOT IN list from the scope slice; the literal stays in the query and a test ties the two (TestWIQLInterpolationStructural)"
    - "Must NOT put server-supplied text into scope; every value is a code constant or an SDK enum constant"
    - "Must NOT add omitempty to Result.Scope; null must be emitted when there is no scope (D-02)"
  artifacts:
    - path: "internal/ado/result.go"
      provides: "type Scope (excluded_states, iterations, pr_status, approval_status; omitempty) and Result.Scope *Scope tagged json:\"scope\" without omitempty"
      contains: "json:\"scope\""
    - path: "internal/ado/workitems.go"
      provides: "GetWorkItems sets Scope on both returns"
      contains: "Scope"
    - path: "internal/ado/dashboard.go"
      provides: "GetDashboardCounts sets Scope with PRStatus on both returns"
      contains: "PullRequestStatusValues.Active"
    - path: "internal/ado/pullrequests.go"
      provides: "GetPullRequests sets Scope{PRStatus}"
      contains: "PRStatus"
    - path: "internal/ado/releases.go"
      provides: "GetPendingApprovals sets Scope{ApprovalStatus} (D-15)"
      contains: "ApprovalStatus"
    - path: "internal/ado/json_test.go"
      provides: "scope in every Result key set; Scope key set; TestNewResultScopeIsNull"
      contains: "TestNewResultScopeIsNull"
    - path: "internal/ado/wiql_test.go"
      provides: "TestGetWorkItemsScope and TestGetDashboardCountsScope tying scope to the captured WIQL and PR status"
      contains: "TestGetWorkItemsScope"
    - path: "internal/ado/pullrequests_test.go"
      provides: "TestGetPullRequestsScope tying pr_status to SearchCriteria.Status"
      contains: "TestGetPullRequestsScope"
    - path: "internal/ado/releases_test.go"
      provides: "TestGetPendingApprovalsScope (D-15) and TestUnscopedReadsReportNullScope (D-16)"
      contains: "TestUnscopedReadsReportNullScope"
  key_links:
    - from: "internal/ado/workitems.go GetWorkItems"
      to: "internal/ado/result.go Result.Scope"
      via: "the result built by newResult gets Scope assigned before each return"
      pattern: "\\.Scope = "
    - from: "internal/ado/pullrequests.go GetPullRequests scope"
      to: "git.PullRequestStatusValues.Active"
      via: "PRStatus is string(git.PullRequestStatusValues.Active), the constant the request's SearchCriteria.Status points at"
      pattern: "string\\(git\\.PullRequestStatusValues\\.Active\\)"
    - from: "internal/ado/releases.go GetPendingApprovals scope"
      to: "release.ApprovalStatusValues.Pending"
      via: "ApprovalStatus is string(release.ApprovalStatusValues.Pending), the constant the request's StatusFilter points at"
      pattern: "string\\(release\\.ApprovalStatusValues\\.Pending\\)"
---

<objective>
Close the data half of audit warning W4: three reads apply a fixed server-side filter that nothing discloses, so an agent cannot tell "not in scope" from "does not exist".

- `workitems list` and `dashboard counts` exclude Done, Closed and Removed and limit to recent iterations.
- `prs list` and the dashboard PR count are active-only.
- `releases approvals` is pending-only (D-15).

After this plan every read envelope carries a `scope` key: a structured object for the four scoped reads, `null` for the other four (D-01, D-02, D-16). Each value is tied by test to the request that applies it. This plan discloses; it widens nothing (D-03).

Purpose: TRUST-03 (an agent can tell "outside the scope" from "absent"), OUT-04 and CLI-06 (the published envelope states the filter the flags sit on top of).
Output: `ado.Scope` on `Result[T]`, scope set on four reads, the corrected 30-day comments, and contract and tie tests. Plan 06-04 renders this field for humans and in help.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/phases/06-core-value-cleanup/06-CONTEXT.md
@.planning/phases/06-core-value-cleanup/06-RESEARCH.md
@.planning/phases/06-core-value-cleanup/06-PATTERNS.md
@internal/ado/result.go
</context>

<interfaces>
These already exist. Use them as they are and do not redefine them.

- `type Result[T any] struct { Items []T; Failures []Failure; Complete bool }` in internal/ado/result.go:98-102. Its tags are items, failures and complete.
- `func newResult[T any](items []T, failures []Failure) Result[T]` in result.go:111-123. It derives Complete from len(failures) == 0 and pre-allocates both slices. It does not change.
- Return paths that must carry scope:

  | Read | File | Returns |
  |------|------|---------|
  | GetWorkItems | workitems.go | the early return when there are no projects (~line 104) and the final return (~line 137) |
  | GetDashboardCounts | dashboard.go | the early return (~line 60) and the final return (~line 102) |
  | GetPullRequests | pullrequests.go | one final return (~line 82) |
  | GetPendingApprovals | releases.go | one final return (~line 314) |

- The WIQL literal at workitems.go (getProjectWorkItems) and dashboard.go (getProjectCounts) contains `[System.State] NOT IN ('Done', 'Closed', 'Removed')`, followed by `iterationClause(iterations)`. TestWIQLInterpolationStructural (wiql_test.go:195) forbids any non-literal operand other than iterationClause in that + chain.
- SDK enums: `git.PullRequestStatusValues.Active` is the string-typed value "active" (git/models.go:2591-2602), and `release.ApprovalStatusValues.Pending` is "pending" (release/models.go:190-205). Both are verified in 06-RESEARCH.md.
- The recency window: client.go GetRecentIterations uses `time.Now().AddDate(0, 0, -30)` as the cutoff (~line 190). The comments at client.go ~167, ~209 and ~231, workitems.go ~147 and dashboard.go ~120 wrongly say 90 days.
- Test fakes and helpers:
  - fakeWorkItemAPI (QueryByWiqlFunc, GetClassificationNodeFunc), fakeGitAPI (GetPullRequestsByProjectFunc) and fakeReleaseAPI (GetApprovalsFunc) in internal/ado/fakes_test.go
  - assertKeySet / keysOf in json_test.go:19-56
  - branchTestClient (branches_test.go:74), pipelineTestClient (pipelines_test.go:152) and releaseTestClient (releases_test.go:202)
  - The capture-closure shapes to copy, without the slog capture: TestGetWorkItemsUsesProjectMacro (wiql_test.go:27-74) and TestDashboardUsesProjectMacro (wiql_test.go:105-135)
</interfaces>

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1 (tracer): the envelope gains scope, and workitems list discloses its WIQL filter end to end into JSON (W4, D-01, D-02)</name>
  <reversibility rating="one-way">D-01/D-02 record this rating: scope and its key names (excluded_states, iterations) become part of the published --json envelope agents branch on; renaming or removing them later breaks callers. Decided by the user in 06-CONTEXT.md, so no checkpoint re-asks it.</reversibility>
  <files>internal/ado/result.go, internal/ado/workitems.go, internal/ado/client.go, internal/ado/json_test.go, internal/ado/wiql_test.go</files>
  <read_first>
    - internal/ado/result.go lines 1-130: the Failure MarshalJSON, the Result doc comment and type, and newResult
    - internal/ado/workitems.go lines 76-170: the op constants, GetWorkItems with both returns, and getProjectWorkItems' WIQL
    - internal/ado/client.go lines 160-245: GetRecentIterations, the 30-day cutoff, and the three 90-day comments
    - internal/ado/json_test.go lines 1-60 (keysOf, assertKeySet), 150-170 (the Result key-set rows) and 418-454 (TestNewResultEmitsEmptyArraysNotNull)
    - internal/ado/wiql_test.go lines 1-78 (imports, quoteProject, TestGetWorkItemsUsesProjectMacro) and 185-210 (TestWIQLInterpolationStructural and its rationale)
    - .planning/phases/06-core-value-cleanup/06-RESEARCH.md: Pattern 3 and Pitfalls 6-8
  </read_first>
  <behavior>
    json_test.go:
    - Every Result key set in TestJSONFieldNames ("Result" and "Result_uniform_across_reads", 8 assertions) becomes items, failures, complete, scope.
    - New subtest "Scope" in TestJSONFieldNames, with two assertions:
      - Scope with all four fields set marshals exactly the keys approval_status, excluded_states, iterations and pr_status.
      - Scope{} marshals no keys at all.
    - New TestNewResultScopeIsNull:
      - newResult[Branch](nil, nil) has a nil Scope.
      - Marshalled and decoded into a struct with `Scope json.RawMessage` tagged scope, the raw value is exactly "null". An absent key decodes as nil RawMessage and fails.

    New TestGetWorkItemsScope in wiql_test.go, with these subtests:
    - "scope matches the query". The client has projects ["P"] and a fakeWorkItemAPI:
      - GetClassificationNode returns nil, nil.
      - QueryByWiql records the query text and returns an empty WorkItemQueryResult.
      - Expect:
        - json.Marshal(result.Scope) == {"excluded_states":["Done","Closed","Removed"],"iterations":"recent"}
        - the captured query contains "[System.State] NOT IN ('" + strings.Join(result.Scope.ExcludedStates, "', '") + "')"
        - Complete is true and Failures is empty
        - json.Marshal(result) contains `"scope":{"excluded_states":["Done","Closed","Removed"],"iterations":"recent"}`
    - "early return carries the scope": &Client{} with no projects and no fakes. Scope marshals the same object.
    - "a failed project keeps the scope": QueryByWiql returns errors.New("query failed"). Complete is false, Failures has one entry, and Scope is non-nil with the same object.
    - "results do not share a scope": two GetWorkItems calls on &Client{} return Scope pointers that are not equal.

    RED: compile failure first, because Result has no Scope field and ado has no Scope type. Then, once the type exists with no read setting it, the workitems subtests fail on a nil Scope.
  </behavior>
  <action>
    RED first:
    - Write the json_test.go updates and TestGetWorkItemsScope as the behavior block states.
    - Do not copy the slog capture from TestGetWorkItemsUsesProjectMacro. This test needs no log assertion, and a copy would be its third use.
    - Run the tests and record the compile-failure RED in the SUMMARY.

    GREEN in internal/ado/result.go, per D-01 and D-02:
    1. Declare type Scope just above Result. It has four fields, each with omitempty:
       - ExcludedStates []string, tagged json:"excluded_states,omitempty"
       - Iterations string, tagged json:"iterations,omitempty"
       - PRStatus string, tagged json:"pr_status,omitempty"
       - ApprovalStatus string, tagged json:"approval_status,omitempty"
    2. Doc comment on Scope:
       - It discloses the fixed filter Azure DevOps applies server-side for a read, so a caller can tell "outside the scope" from "does not exist" (audit W4, D-01).
       - A key is present only when that filter applies. This deliberately departs from TestJSONFieldNames' stable-key-set rule, because a scope key's presence IS the fact (D-01's examples are per-verb objects).
       - "recent" means iterations starting or finishing after a cutoff 30 days back (GetRecentIterations), and a project with no such dated iteration is not limited by iteration.
       - Values are code or SDK constants, never server text.
       - It never affects Complete.
       - The key names are a published one-way contract (D-01). pr_status is used rather than status because dashboard counts carries both a work-item filter and a PR filter, and a bare status would be ambiguous there (D-15 note, RESEARCH A1).
    3. Add Scope *Scope to Result, tagged json:"scope" with NO omitempty (D-02: null is emitted when nil). Extend Result's doc comment with one sentence on scope. Leave newResult as it is.

    GREEN in internal/ado/workitems.go GetWorkItems:
    1. At the top of the function, build scope as a new &Scope with ExcludedStates Done, Closed, Removed (in the WIQL literal's order) and Iterations "recent". Building it inside the function gives every call its own value.
    2. Comment beside it: D-01. The states must match the NOT IN literal in getProjectWorkItems, which TestGetWorkItemsScope ties. Add a note: comment saying dashboard.go builds the same two fields. That is two uses, below the rule of three, so it is not extracted.
    3. On the early return and the final return, assign the newResult value to a local, set its Scope, then return it.
    4. Do not touch the WIQL string.

    Comment fixes, per D-06 accuracy: change "90 days" to "30 days" in:
    - workitems.go, the getProjectWorkItems comment above GetRecentIterations
    - client.go, the GetRecentIterations doc comment and the two in-body comments
    Change no code in client.go.

    Leave these untouched:
    - dashboard.go, pullrequests.go and releases.go (Tasks 2 and 3)
    - newResult, Failure, and every WIQL string
    - go.mod
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go test -race -count=1 -v ./internal/ado -run '^(TestJSONFieldNames|TestNewResultScopeIsNull|TestNewResultEmitsEmptyArraysNotNull|TestGetWorkItemsScope|TestGetWorkItemsUsesProjectMacro|TestWIQLInterpolationStructural|TestGetWorkItemsReportsProjectFailure|TestGetWorkItemsReportsIterationFailure)$'</automated>
    <fails_when>non-zero exit, any "--- FAIL" line, "testing: warning: no tests to run" in the output, fewer than 8 top-level "--- PASS: Test" lines, or fewer than 4 "--- PASS: TestGetWorkItemsScope/" subtest lines</fails_when>
  </verify>
  <acceptance_criteria>
    - TestJSONFieldNames passes:
      - all 8 Result key sets include scope
      - the Scope subtest asserts the four keys when populated and none when empty
    - TestNewResultScopeIsNull passes with the raw scope value exactly "null".
    - TestGetWorkItemsScope passes all 4 subtests:
      - the exact scope object, tied to the captured WIQL's NOT IN list
      - the early return carries the scope
      - a failed project keeps the scope with complete:false
      - two calls return distinct pointers
    - TestWIQLInterpolationStructural, TestGetWorkItemsUsesProjectMacro and the GetWorkItems failure tests pass unchanged.
    - The SUMMARY records the RED compile failure.
  </acceptance_criteria>
  <done>`ado.Result` publishes scope on every envelope, and `workitems list` states the exact WIQL filter it applies, on every return path, without touching completeness.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: dashboard counts and prs list disclose their filters, tied to the WIQL and the PR status the requests send (W4, D-01)</name>
  <reversibility rating="one-way">D-01 records this rating: the pr_status key joins the published envelope. Key name chosen under D-01's planner latitude and the D-15 note, so no checkpoint re-asks it.</reversibility>
  <files>internal/ado/dashboard.go, internal/ado/pullrequests.go, internal/ado/wiql_test.go, internal/ado/pullrequests_test.go</files>
  <read_first>
    - internal/ado/dashboard.go lines 40-165: GetDashboardCounts with both returns, getProjectCounts' WIQL and the PR request's Status
    - internal/ado/pullrequests.go lines 36-83: GetPullRequests and its single return
    - internal/ado/wiql_test.go: as left by Task 1, especially TestDashboardUsesProjectMacro at ~105-160 for the dashboard capture shape
    - internal/ado/pullrequests_test.go lines 330-362: TestGetPullRequestsReportsCompleteWhenEveryProjectAnswers, the analog
    - internal/ado/result.go: the Scope type as written in Task 1
  </read_first>
  <behavior>
    New TestGetDashboardCountsScope in wiql_test.go, with these subtests:
    - "scope matches the requests". The client has projects ["P"], plus:
      - a fakeWorkItemAPI whose GetClassificationNode returns nil, nil and whose QueryByWiql records the query and returns an empty result
      - a fakeGitAPI whose GetPullRequestsByProject records args.SearchCriteria.Status and returns an empty slice
      - no pipelines configured, so no build call is made
      - Expect:
        - json.Marshal(result.Scope) == {"excluded_states":["Done","Closed","Removed"],"iterations":"recent","pr_status":"active"}
        - the query contains the joined NOT IN list as in Task 1
        - string(*capturedStatus) == result.Scope.PRStatus
        - Complete is true
    - "early return carries the scope": &Client{} gives the same scope object.

    New TestGetPullRequestsScope in pullrequests_test.go, with these subtests:
    - "scope matches the request": two projects, and a fake recording args.SearchCriteria.Status. Expect:
      - json.Marshal(result.Scope) == {"pr_status":"active"}
      - string(*capturedStatus) == result.Scope.PRStatus
      - Complete is true and Failures is empty
    - "no projects still carries the scope": &Client{} gives {"pr_status":"active"}, with no git call.

    RED: every scope assertion fails on a nil Scope.
  </behavior>
  <action>
    RED first:
    - Add both tests as the behavior block states, each with a doc comment citing D-01 and audit W4.
    - Copy only the fake closure shapes. Do not copy the slog capture.
    - Record the RED output in the SUMMARY.

    GREEN, per D-01:
    1. In dashboard.go GetDashboardCounts:
       - At the top, build a new &Scope with ExcludedStates Done, Closed, Removed, Iterations "recent", and PRStatus string(git.PullRequestStatusValues.Active).
       - Set it on the early return and the final return, using the same local-then-assign shape as Task 1.
       - Comment: D-01. The states and iterations mirror getProjectCounts' WIQL, and PRStatus is derived from the constant the PR request sends, so the two cannot drift. Add a note: comment pointing at the identical work-item fields in GetWorkItems (two uses, not extracted).
       - Change the "90 days" comment above getProjectCounts' GetRecentIterations call to "30 days".
    2. In pullrequests.go GetPullRequests:
       - Set Scope on the single return to a new &Scope with PRStatus string(git.PullRequestStatusValues.Active).
       - Comment: D-01. Azure DevOps returns only active pull requests to this request, and the value is the same constant SearchCriteria.Status points at.

    Leave these untouched:
    - the PR request, the WIQL strings, and getProjectCounts' pipeline count (a config-driven client-side match, not a server filter, D-16 reasoning)
    - releases.go (Task 3)
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go test -race -count=1 -v ./internal/ado -run '^(TestGetDashboardCountsScope|TestGetPullRequestsScope|TestDashboardUsesProjectMacro|TestGetDashboardCountsReportsCompleteWhenEveryProjectAnswers|TestGetDashboardCountsReportsProjectFailure|TestGetPullRequestsReportsCompleteWhenEveryProjectAnswers|TestGetPullRequestsReportsProjectFailure|TestWIQLInterpolationStructural)$'</automated>
    <fails_when>non-zero exit, any "--- FAIL" line, "testing: warning: no tests to run" in the output, fewer than 8 top-level "--- PASS: Test" lines, or fewer than 2 subtest PASS lines under each of TestGetDashboardCountsScope and TestGetPullRequestsScope</fails_when>
  </verify>
  <acceptance_criteria>
    - TestGetDashboardCountsScope passes both subtests:
      - the exact three-key scope
      - the NOT IN list matches the captured WIQL
      - pr_status equals the captured SearchCriteria.Status
      - the early return carries the scope
    - TestGetPullRequestsScope passes both subtests:
      - {"pr_status":"active"}, equal to the captured Status
      - complete:true
      - present with no projects
    - The existing dashboard and pull-request tests and TestWIQLInterpolationStructural pass unchanged.
    - The SUMMARY records RED evidence.
  </acceptance_criteria>
  <done>The dashboard and the PR list each state the filter their requests apply, derived from the same constants the requests send.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: releases approvals discloses its pending-only filter, and the four latest-per-definition reads report null (D-15, D-16)</name>
  <reversibility rating="one-way">D-15 records this rating: approval_status joins the published envelope. Decided by the user in 06-CONTEXT.md, so no checkpoint re-asks it.</reversibility>
  <files>internal/ado/releases.go, internal/ado/releases_test.go</files>
  <read_first>
    - internal/ado/releases.go lines 236-315: GetPendingApprovals, the statusFilter at ~257 and its single return
    - internal/ado/releases_test.go lines 159-272: the inline GetApprovalsFunc closure shape, releaseTestClient at 202, and the two happy-path tests
    - internal/ado/branches_test.go lines 74-99: branchTestClient
    - internal/ado/pipelines_test.go lines 152-216: pipelineTestClient and the two happy-path tests
    - internal/ado/result.go: the Scope type as written in Task 1
  </read_first>
  <behavior>
    New TestGetPendingApprovalsScope in releases_test.go:
    - The client has two projects and a fakeReleaseAPI whose GetApprovals records args.StatusFilter and returns one approval with an Id.
    - Expect:
      - json.Marshal(result.Scope) == {"approval_status":"pending"}
      - string(*capturedFilter) == result.Scope.ApprovalStatus
      - Complete is true and Failures is empty

    New TestUnscopedReadsReportNullScope in releases_test.go, with one subtest per read:
    - "branches list" via branchTestClient(t, "project-one", "repository-one", nil)
    - "pipelines runs" via pipelineTestClient(t, []string{"project-one"}, func(string) error { return nil })
    - "pipelines definitions", the same client calling GetPipelineDefinitions
    - "releases list" via releaseTestClient(t, []string{"project-one"}, func(string) error { return nil })

    Each subtest asserts:
    - the result's Scope is nil
    - json.Marshal(result) contains `"scope":null`
    - Complete is true, which proves the fixture answered

    RED: TestGetPendingApprovalsScope fails on a nil Scope. TestUnscopedReadsReportNullScope already passes after Task 1 and is a D-16 pin that fails if anyone sets a scope on these reads.
  </behavior>
  <action>
    RED first:
    - Add both tests as the behavior block states.
    - TestUnscopedReadsReportNullScope's doc comment cites D-16: "latest run/release per definition" is the verb's purpose, not a hidden filter. The empty-but-complete result when no pipelines are configured is tracked in BL-02, not here.
    - Use a small closure local to the test for the shared nil-plus-JSON assertion. It is one function, so nothing is extracted.
    - Record the RED output in the SUMMARY.

    GREEN in releases.go GetPendingApprovals, per D-15:
    - On the single return, set Scope to a new &Scope with ApprovalStatus string(release.ApprovalStatusValues.Pending).
    - Comment: D-15. The request's StatusFilter already restricts the answer to pending approvals server-side, so D-02 makes the scope non-null, and the value is the same constant the request sends.

    Leave these untouched:
    - GetReleases, GetPipelineRuns, GetPipelineDefinitions and GetBranches. Their null scope is the point of D-16.
    - .planning/BACKLOG.md. The D-16 empty-pipelines follow-up is already recorded under BL-02.
  </action>
  <verify>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go test -race -count=1 -v ./internal/ado -run '^(TestGetPendingApprovalsScope|TestUnscopedReadsReportNullScope|TestGetPendingApprovalsHappyPathRowCountUnchanged|TestGetPendingApprovalsReportsProjectFailure|TestGetReleasesHappyPathRowCountUnchanged)$'</automated>
    <fails_when>non-zero exit, any "--- FAIL" line, "testing: warning: no tests to run" in the output, fewer than 5 top-level "--- PASS: Test" lines, or fewer than 4 "--- PASS: TestUnscopedReadsReportNullScope/" subtest lines</fails_when>
    <automated>cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"</automated>
    <fails_when>non-zero exit from any stage, any "FAIL" package line, or gofmt listing a file</fails_when>
  </verify>
  <acceptance_criteria>
    - TestGetPendingApprovalsScope passes: {"approval_status":"pending"}, equal to the captured StatusFilter, with complete:true.
    - TestUnscopedReadsReportNullScope passes all 4 subtests: branches list, pipelines runs, pipelines definitions and releases list each have a nil Scope and "scope":null in JSON.
    - `go test ./internal/ado -run 'Scope' -count=1 -v` runs at least the 6 scope tests from Tasks 1-3 (TestNewResultScopeIsNull, TestGetWorkItemsScope, TestGetDashboardCountsScope, TestGetPullRequestsScope, TestGetPendingApprovalsScope, TestUnscopedReadsReportNullScope) and passes.
    - The SUMMARY records RED evidence for TestGetPendingApprovalsScope.
    - The full module gate exits 0. The TUI packages compile unchanged, because they do not read Scope.
  </acceptance_criteria>
  <done>All 8 read envelopes carry scope. The four scoped reads state exactly the filter their requests apply, and the four latest-per-definition reads say null.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| internal/ado read to Azure DevOps | The request carries a WIQL state/iteration clause or a status filter that the server applies before answering. |
| ado.Result envelope to CLI --json stdout and the calling agent | The agent reads items, failures, complete and now scope, and decides whether an absent item is truly absent. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-06-09 | Spoofing (result integrity) | undisclosed server-side filters on workitems list, dashboard counts, prs list, releases approvals (audit W4) | medium | mitigate | Each scoped read sets a Scope naming its filter on every return path. TestGetWorkItemsScope, TestGetDashboardCountsScope, TestGetPullRequestsScope and TestGetPendingApprovalsScope assert the exact JSON object. |
| T-06-10 | Tampering (disclosure drift) | scope value vs the request actually sent | medium | mitigate | Status values derive from the SDK constants the requests use (git.PullRequestStatusValues.Active, release.ApprovalStatusValues.Pending), and the tests compare against the captured request arguments. The excluded states are tied to the captured WIQL NOT IN text, in order, so editing either side alone fails a test. |
| T-06-11 | Tampering (WIQL injection surface) | building NOT IN from the scope slice | medium | mitigate | The WIQL literal stays a literal. TestWIQLInterpolationStructural continues to forbid any non-literal operand other than iterationClause, and the tie runs in tests only. |
| T-06-12 | Repudiation (completeness signal) | scope altering complete | medium | mitigate | Scope is assigned after newResult and never read by it. The "a failed project keeps the scope" subtest and the complete:true assertions in every scope test pin that the two facts are independent. |
| T-06-13 | Information Disclosure | scope content | low | accept | Values are code and SDK enum constants only. No server-supplied text, credential or URL enters scope. |
| T-06-SC | Tampering | dependency installs | low | accept | Not applicable: no package install, and go.mod is untouched. |
</threat_model>

<verification>
- Module gate (proven command): `cd /Users/e1001547-mbp-it/repos/mine/GoApps/adoBizInt && go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"`. It fails on a non-zero exit, a "FAIL" package line, or gofmt listing a file.
- 06-VALIDATION.md W4 contract rows are green: `go test ./internal/ado -run 'TestJSONFieldNames|TestNewResultScopeIsNull' -count=1` and `go test ./internal/ado -run 'Scope' -count=1`.
- Live confirmation, a human step for /gsd-verify-work that does not block this plan: against the org, all eight reads keep their current `complete` values and now show `scope`. For example, `adoBizInt workitems list --json | jq .scope` prints the excluded-states object.
</verification>

<success_criteria>
- The data half of ROADMAP SC3 holds: the fixed server-side scope of workitems list, dashboard counts, prs list (D-01) and releases approvals (D-15) is in the JSON envelope, and the other four reads carry null (D-02, D-16).
- No query, flag or completeness value changes (D-03).
</success_criteria>

## Artifacts this phase produces

| Artifact | Kind | Purpose |
|----------|------|---------|
| `internal/ado/result.go` | modified | `type Scope` and `Result.Scope` (one-way JSON contract, D-01/D-02) |
| `internal/ado/workitems.go` | modified | Scope on both `GetWorkItems` returns; 30-day comment |
| `internal/ado/client.go` | modified | 90-day comments corrected to 30 days (comments only) |
| `internal/ado/dashboard.go` | modified | Scope with `pr_status` on both returns; 30-day comment |
| `internal/ado/pullrequests.go` | modified | `{"pr_status":"active"}` |
| `internal/ado/releases.go` | modified | `{"approval_status":"pending"}` (D-15) |
| `internal/ado/json_test.go` | modified | Key sets include scope; `TestNewResultScopeIsNull` |
| `internal/ado/wiql_test.go` | modified | `TestGetWorkItemsScope`, `TestGetDashboardCountsScope` |
| `internal/ado/pullrequests_test.go` | modified | `TestGetPullRequestsScope` |
| `internal/ado/releases_test.go` | modified | `TestGetPendingApprovalsScope`, `TestUnscopedReadsReportNullScope` |
| `.planning/phases/06-core-value-cleanup/06-03-SUMMARY.md` | new | Execution record, including the final scope key names as published |

<output>
Create `.planning/phases/06-core-value-cleanup/06-03-SUMMARY.md` when done. Record the published scope key names and per-read values (one-way contract), the omitempty-inside / no-omitempty-on-Result split, the tie tests, the comment corrections, and the RED-then-GREEN evidence.
</output>
