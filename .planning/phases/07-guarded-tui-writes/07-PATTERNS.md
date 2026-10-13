# Phase 7: Guarded TUI Writes - Pattern Map

**Mapped:** 2026-10-13
**Files analyzed:** 14
**Analogs found:** 13 / 14

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/guard/guard.go` (new: op, Run, fieldChange, target, record, outcomes) | service | request-response (pre-read -> confirm -> mutate -> audit) | `cmd/adoBizInt/write.go` (`writeOp` L103-108, `runWrite` L137-309) | exact (extraction) |
| `internal/guard/audit.go` (new) | utility | file-I/O append | `cmd/adoBizInt/audit.go` L47-119 | exact (extraction) |
| `internal/guard/ops.go` (new: per-verb op constructors, diffWorkItem, appendFloatChange) | service | request-response | `cmd/adoBizInt/write.go` L311-575 | exact (extraction) |
| `internal/guard/*_test.go` | test | - | `cmd/adoBizInt/write_test.go`, `writeverbs_test.go`, `audit_test.go`, `fakes_test.go` L147+ | exact |
| `cmd/adoBizInt/write.go`, `audit.go`, `exit.go` (modified: thin adapters over guard) | controller | request-response | self | n/a |
| `internal/fetchctx/fetchctx.go` (moved from `internal/ui/fetchctx`) | utility | - | `internal/ui/fetchctx/fetchctx.go` (git mv) | exact |
| `internal/ui/confirm/confirm.go` (new shared diff-confirm modal) | component | event-driven | `internal/ui/workitems/effort.go` (Update L116+, View L216+), `internal/ui/pipelines/trigger_modal.go` View L183-215 | role-match |
| `internal/ui/workitems/workitems.go` (L266, 336, 350, 360) | component | event-driven | self; modal wiring L85-86, L238-243, L283-284 | n/a |
| `internal/ui/prs/prs.go` (L343-386; remove `confirmComplete` L82/197/268/299) | component | event-driven | self | n/a |
| `internal/ui/releases/releases.go` (L206, 214) + `approval_modal.go` | component | event-driven | `prs.go` approve cmd | n/a |
| `internal/ui/pipelines/pipelines.go` (L205) + `trigger_modal.go` | component | event-driven | `prs.go` approve cmd | n/a |
| `internal/ui/app.go` (quit/tab-switch triggers confirm, D-02) | component | event-driven | self | n/a |
| `internal/ui/nodirectwrites_test.go` or `internal/guard/structural_test.go` (GUARD-02) | test | static AST scan | `internal/ado/nilguard_test.go` L103-195, `cmd/adoBizInt/main_test.go` L945-1013 | role-match |
| TUI audit `actor`/`user_approved` field | model | - | `cmd/adoBizInt/audit.go` `auditRecord` L47-64 | partial (one-way door: new keys only) |

## Pattern Assignments

### `internal/guard/guard.go` — extract from `cmd/adoBizInt/write.go`

Current op shape (L103-108) is CLI-flag-coupled; guard must drop `*globalFlags`/`*writeFlags`:
```go
type writeOp struct {
	name    string
	resolve func(gf *globalFlags, wf *writeFlags, id int) (writeTarget, error)
	plan    func(ctx context.Context, t writeTarget, wf *writeFlags) ([]fieldChange, error)
	mutate  func(ctx context.Context, t writeTarget, wf *writeFlags) error
}
```
Data types to move verbatim (L19-85): field-name consts (`fieldState`, `fieldRemainingWork`, `fieldReviewerVote`, `fieldPRStatus`, `fieldApprovalStatus`, `fieldQueuedBuild`), `voteApproved = 10`, `fieldChange{Field,From,To}` JSON tags, `writeTarget{Project, Repo omitempty, ID}`. JSON tag strings are a one-way door (audit lines).

Core sequence to preserve in `Run` (L240-300): open sink BEFORE mutate (D-13); defer-recover appends `unknown` only if `!audited`; after mutate `outcome, code := writeExitCodeFor(err, ctx.Err())`; audit only `executed`/`unknown`; if append fails, downgrade to `unknown`:
```go
if outcome == outcomeExecuted || outcome == outcomeUnknown {
	record := newAuditRecord(writeRecord{Command: name, Target: target, Changes: changes, Outcome: outcome})
	audited = true
	if auditErr := appendAuditRecord(sink, record); auditErr != nil {
		...
		outcome = outcomeUnknown
		code = exitOutcomeUnknown
	}
}
```
GUARD-06 constraints: guard must not use `fmt.Fprintf(stderrWriter, ...)`, `finishWrite`, `handlePanic`, or exit codes — those (L245-305, L578-625) stay in `cmd/adoBizInt` and consume a returned Report. Classification in `cmd/adoBizInt/exit.go` L35-41 (outcome consts) moves to guard; `writeExitCodeFor` L62 (code mapping) splits: outcome classification -> guard, int exit code -> cmd. Logging uses `logging.Logger()` (`internal/logging`), fine in guard.

Client interface: `cmd/adoBizInt/dispatch.go` L86+ `writeClient` (GetWorkItemDetail, UpdateWorkItemState, UpdateWorkItemStateWithEffort, GetPullRequestDetail, GetReleaseApprovalDetail, GetPipelineDefinitionDetail, ApprovePullRequest, ...). Move to guard as the consumer-side interface; `*ado.Client` satisfies it, TUI passes `m.client`.

### `internal/guard/ops.go` — from write.go L311-575
Constructors `setStateOp` (L311), `prsApproveOp` (L370), `prsCompleteOp` (L402), `releaseApprovalOp(name, wantStatus, mutate, c)` (L434), `pipelinesQueueOp` (L467), helpers `diffWorkItem` (L535), `appendFloatChange` (L560). Keep flag-resolution (`oneProject`, `oneRepo`, `resolveProjectRepo` L116-135, L512) in cmd. Effort inputs must stay `*float64` (nil = absent, GUARD-05).

### `internal/guard/audit.go` — from `cmd/adoBizInt/audit.go`
Opener to change (L76-81) per D-11:
```go
func openAuditSink() (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(auditPath), 0755); err != nil {   // -> 0700
		return nil, err
	}
	return os.OpenFile(auditPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644) // -> 0600
}
var auditOpen = openAuditSink   // test seam; keep (CR-01 tests in audit_test.go use it)
```
Append (L109-119) moves verbatim: single `json.Marshal` + `'\n'` + one `f.Write` + `f.Sync()`. `auditRecord` (L47) + `newAuditRecord` (L57): append `Actor string json:"actor"` and `UserApproved bool json:"user_approved"` (no omitempty — must be present on every record). `defaultAuditPath` L27 moves too.

### `internal/fetchctx/fetchctx.go`
`git mv internal/ui/fetchctx internal/fetchctx`; content unchanged (`New`, `NewWithParent`, `defaultBudget = 60s`). Update imports in: `cmd/adoBizInt/dispatch.go:16`, `write.go:13`, `internal/ui/{releases,prs,dashboard,pipelines,workitems,branches}/*.go`. TUI writes use `ctx, cancel := fetchctx.New(budget)` with `defer cancel()` INSIDE the `tea.Cmd` closure (doc comment L29-31) instead of `context.Background()`.

### `internal/ui/confirm/confirm.go` (new shared modal)
Model/msg shape from `internal/ui/workitems/effort.go`: value-receiver `Update(msg) (Model, tea.Cmd)` returning close/submit messages:
```go
case tea.KeyMsg:
	switch msg.String() {
	case "esc":
		return m, func() tea.Msg { return CloseEffortMsg{} }
```
Messages as plain structs (`EffortSubmitMsg` L23, `CloseEffortMsg` L35) -> e.g. `confirm.WriteMsg`, `DiscardMsg`, `StayMsg`.
View styling from `trigger_modal.go` L183-205:
```go
borderStyle := lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("62")).
	Padding(1, 2).
	Width(modalWidth)
titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Width(modalWidth - 6).Align(lipgloss.Center)
```
Overlay via `internal/ui/modal.Center(fg, bg)` (modal.go L21). Used by 4 views -> rule of three satisfied for a shared package.

### View call sites (workitems/prs/releases/pipelines)
Current shape to replace (`prs.go` L355-363):
```go
client := m.client; prID := pr.ID; project := pr.Project; repoID := pr.RepositoryID
return func() tea.Msg {
	err := client.ApprovePullRequest(context.Background(), project, repoID, prID)
	return PRActionMsg{PRID: prID, Action: "approve", Err: err}
}
```
New flow: cmd 1 = guard pre-read (plan) -> msg with changes (or no-op -> status `No change: #<id> already <state>`, D-07) -> open confirm modal -> on `y` cmd 2 = guard execute -> result msg carrying outcome; `unknown` -> sticky `statusErr` + re-fetch (D-08). Keep `actionPending`/`statusMsg`/`statusErr` conventions (prs.go L351-353).
Auto-refresh suppression (prs.go L194-197): replace `!m.confirmComplete` with `!m.showConfirm` (or equivalent) in every view's `TickMsg` branch.
Modal state wiring (workitems.go L85-86, L238-243, L283-284): `confirmModal *confirm.Model; showConfirm bool`, set width/height from model, nil on close.

## Shared Patterns

### Test fakes
**Source:** `cmd/adoBizInt/fakes_test.go` L147-230 (`fakeWriteClient` with per-method funcs/recorded calls). For guard tests, copy into `internal/guard` test file (test fakes are package-scoped; CLI copy stays so `write_test.go` passes unchanged per GUARD-06/Pitfall 14). TUI view tests can reuse the same shape.

### Structural test (GUARD-02)
**Source:** `internal/ado/nilguard_test.go` L103-195 (`parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)` + `ast.Inspect(fn.Body, ...)`) and `cmd/adoBizInt/main_test.go` L945-1013. Walk `internal/ui/**/*.go` (non-test), flag any `*ast.SelectorExpr` whose `Sel.Name` is in {UpdateWorkItemState, UpdateWorkItemStateWithEffort, ApprovePullRequest, CompletePullRequest, ApproveRelease/Reject*, QueueBuild/RunPipeline}. Assert the list is non-empty-on-purpose by including a tripwire marker check like `patleak_test.go`.

### Leak regression
`cmd/adoBizInt/patleak_test.go` drives an executed write through fakes with slog captured; extend or mirror for the guard path if it moves logging.

## No Analog Found

| File | Role | Reason |
|---|---|---|
| Staged-edit / move-on confirm in `workitems.go` + `app.go` (D-02/D-03) | component state | No existing "intercept navigation pending confirmation" pattern; nearest is prs.go `confirmComplete` key gate (L268-299), which only blocks keys in one view. Planner must design focus-change interception in `app.go` (tab switch/quit) and row movement. |

## Metadata
**Analog search scope:** cmd/adoBizInt, internal/ui/**, internal/ado tests
**Files scanned:** ~20 (all tracked; `git ls-files` confirmed, no .gsd mirrors)
**Pattern extraction date:** 2026-10-13
