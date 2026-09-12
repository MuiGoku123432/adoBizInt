# Phase 04: Guarded writes - Pattern Map

**Mapped:** 2026-09-12
**Files analyzed:** 14 new/modified targets
**Analogs found:** 13 / 14
**Method note:** read-only pass using `read`/`grep`/`glob` only. Every excerpt below was read verbatim this session unless marked UNVERIFIED.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `cmd/adoBizInt/write.go` (new) — `runWrite`, write envelope, `writeClient` | dispatch/controller | request-response (pre-read → gate → mutate) | `cmd/adoBizInt/dispatch.go:159-258` (`runRead`) | exact (role + flow) |
| `cmd/adoBizInt/dispatch.go` (mod) — 7 `commands` entries, 2 new exit consts | dispatch/controller | request-response | itself, `dispatch.go:21-26`, `:100-137` | self |
| `cmd/adoBizInt/flags.go` (mod) — `writeSupportByCommand`, `writeFlags`, `--confirm`, effort flags | config/flag registration | parse-time | `flags.go:41-63` + `:103-141` (`filterSupportByCommand` + `newFlagSet`) | exact |
| `cmd/adoBizInt/exit.go` (mod or new `writeExitCodeFor`) | mapper/utility | transform | `exit.go:19-29` (`exitCodeFor`) | role-match (signature differs: bare `error`, not `[]Failure`) |
| `cmd/adoBizInt/audit.go` (new) — JSONL sink, pre-flight open, append | service / file-I/O | file-I/O append | `crash.go:44-68` (`writeCrashRecord`) + `logging.go:12-34` (`Init` open-for-append) | role-match (two partial analogs, no single one) |
| `cmd/adoBizInt/render.go` (mod) — human-mode diff table | component/renderer | transform | `render.go:26-56` + `writeRows`/`row` | exact |
| `cmd/adoBizInt/usage.go` (mod) — 7 verbs into three-level help | config/docs | n/a | `usage.go` (UNVERIFIED — not read) | UNVERIFIED |
| `internal/ado/workitems.go` (mod) — nullable effort signature | service/SDK adapter | CRUD (JSON-Patch) | `workitems.go:284-331` (itself, the blind-`Add` builder) | self |
| `internal/ado/workitems.go` (mod) — 3 effort fields on `GetWorkItemDetail` | service/SDK adapter | CRUD read | `workitems.go:370-372` (`StoryPoints` block) | exact |
| `internal/ado/writes_byid.go` or similar (new) — PR / approval / definition by-id pre-reads | service/SDK adapter | request-response read | `workitems.go:334-351` (`GetWorkItemDetail`) | exact |
| `internal/ado/interfaces.go` (mod) — possible `GetApproval` entry | model/interface | n/a | `interfaces.go:42-47` (`releaseAPI`) | exact |
| `internal/ado/fakes_test.go` (mod) — fake methods for new entries | test fake | n/a | `fakes_test.go:44-60` | exact |
| `cmd/adoBizInt/write_test.go`, `audit_test.go` (new) | test | n/a | `filter_test.go:311-349`, `usage_test.go:13-25`, `cmd/.../fakes_test.go:14-40` | exact |
| `internal/ui/workitems/workitems.go:266,:333,:347,:354` (mod) — pointer call sites | TUI call site | event-driven | UNVERIFIED (not read this session) | none |

---

## Pattern Assignments

### `cmd/adoBizInt/write.go` — `runWrite` (dispatch/controller, request-response)

**Analog:** `cmd/adoBizInt/dispatch.go:159-258` (`runRead`)

**Parse + exit-on-parse-error pattern** (`dispatch.go:168-177`) — **copy verbatim**:

```go
	fs, gf, ff := newFlagSet(name)
	if err := fs.Parse(args); err != nil {
		// flag.ErrHelp means -h/--help reached the FlagSet: usage has already
		// been written and the request succeeded rather than failed.
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUnknown
	}
```

**Deviation:** after `fs.Parse`, add (a) `fs.Visit` presence capture for the effort flags (D-18), (b) `len(fs.Args()) == 1` positional arity check (D-25), (c) `len(gf.projects) == 1` arity check (D-24). All three failures return `exitUnknown` — nothing attempted, which is exactly what exit 3 means.

**Context/deadline pattern** (`dispatch.go:203-208`) — **copy verbatim**:

```go
	budget := readBudget
	if gf.timeout != 0 {
		budget = gf.timeout
	}
	readCtx, cancel := fetchctx.NewWithParent(ctx, budget)
	defer cancel()
```

**Deviation:** one open question (RESEARCH Q11) — the pre-read and the write now share this single budget. Plan must state whether they split it.

**Interrupt check — the single most important copy** (`dispatch.go:212-225`):

```go
	// The interrupt check keys on the parent's Err(), not on the failure kind.
	// The failure classifier maps a cancellation to the generic kind on purpose,
	// so a user-initiated abort is never reported as a server timeout — which
	// means the kind alone cannot tell you a signal fired, only the parent can.
	if ctx.Err() != nil {
		log.Warn("Read interrupted", "command", name, "error", ctx.Err())
		writeCrashRecord(crashKindInterrupt,
			"read interrupted before any output was written: "+ctx.Err().Error())
		return exitUnknown
	}
```

**Deviation (D-22, supersedes Phase 3 D-25 for writes):** keep the `ctx.Err() != nil` check — it is the *only* way to see a signal, since `classifyFailure` maps `context.Canceled` to `FailureOther` on purpose. But **do not** call `writeCrashRecord` and **do not** return `exitUnknown` (3). Route to the outcome-unknown code B, emit the write envelope with `outcome:"unknown"`, and append the audit record (D-16). A mid-write interrupt must never assert "nothing ran".

**JSON vs human split** (`dispatch.go:242-255`):

```go
	if gf.asJSON {
		if err := emitJSON(result); err != nil { ... return exitUnknown }
	} else {
		render(stdoutWriter, result.Items)
		renderFailures(stderrWriter, result.Failures)
	}
```

**Deviation:** `emitJSON` takes `any` (`dispatch.go:274`) so the write envelope passes through it unchanged — **required** by D-10/Phase 3 D-P4 (marshal to a slice, single `Write`). Human mode renders the diff table instead of rows.

**MUST NOT copy — `emptyIfNil`** (`dispatch.go:263-268`):

```go
func emptyIfNil(v stringList) []string {
	if v == nil {
		return []string{}
	}
	return v
}
```

Its whole purpose is to let an omitted `--project` fall back to the read method's configured list (`dispatch.go:181-190`). **D-24 forbids that for writes** — an omitted `--project` on a mutation must be a parse error, never a silently-chosen project. This is RESEARCH Risk 7.

---

### `cmd/adoBizInt/write.go` — the write envelope type (model, transform)

**Analog:** `cmd/adoBizInt/crash.go:28-42` (`crashRecord` / `crashEnvelope`)

```go
type crashRecord struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	LogPath string `json:"log_path"`
}

// crashEnvelope is the top-level document. The record is nested under a single
// `error` key rather than being expressed as a result envelope with a synthetic
// failure entry (D-26). A crash must not be representable as a normal read
// outcome...
type crashEnvelope struct {
	Error crashRecord `json:"error"`
}
```

**Deviation:** identical construction, different key. D-10 nests under `"write"`:
`{"write":{"command":..,"target":{..},"changes":[{"field":..,"from":..,"to":..}],"executed":false,"outcome":"dry_run"}}`.
Same reasoning verbatim — an agent branches on which top-level key is present, so a write can never be misread as a one-row result set. Carry the `crashEnvelope` doc-comment's argument across.

**Also inherit crash.go's credential rule:** `crashRecord` "carries no stack... a stack on stdout would be a credential-disclosure path (D-24)". D-15 extends this to the audit file: no argv, no actor, no URL, no headers. Never embed an `error` value — flatten to a message string (the `Failure.MarshalJSON` pattern, `result.go:71-88`).

---

### `cmd/adoBizInt/flags.go` — `--confirm` + effort flags (config, parse-time)

**Analog:** `flags.go:41-63` and `flags.go:103-141`

**Capability-map pattern** (`flags.go:41-63`) — **mirror exactly**:

```go
// filterSupport records which filter flags a verb registers. It is keyed on the
// same `noun verb` string as the dispatch table so the two cannot disagree
// about which verbs exist...
type filterSupport struct {
	search       bool
	assignedToMe bool
	hideDone     bool
	hideTasks    bool
	repo         bool
}

var filterSupportByCommand = map[string]filterSupport{
	"workitems list": {search: true, assignedToMe: true, hideDone: true, hideTasks: true},
	"prs list":       {search: true, repo: true},
	...
}
```

**Deviation:** add a sibling `writeSupport` struct + `writeSupportByCommand map[string]writeSupport`, keyed on the same `noun verb` string, with fields for `confirm`, `state`, `effort`, `comment`, `branch`, `repo`. Do **not** widen `filterSupport`.

**Conditional registration inside the single FlagSet site** (`flags.go:115-136`):

```go
	ff := &filterFlags{}
	sup := filterSupportByCommand[name]
	if sup.search {
		fs.StringVar(&ff.search, "search", "",
			"keep only items matching this free-text query; empty disables the filter")
	}
	if sup.repo {
		fs.Var(&ff.repos, "repo",
			"restrict the read to a repository; repeatable or comma-separated")
	}
```

**Deviation:** `newFlagSet` gains a third return value (`*writeFlags`) or returns a struct — either way it stays the **single** `flag.NewFlagSet` site (`flags.go:104`), which is what Phase 3 D-20 exists to protect. The rationale for gating is already written at `flags.go:19-22` and settles the open discretion item: "a flag that parses but cannot mean anything is worse than a flag that does not exist" → **`--confirm` is not registered on read verbs.**

**Effort flag types — the D-17/Risk-2/Risk-3 deviation:** register `--original-estimate`, `--remaining`, `--completed` as plain `float64` locals, then convert to `*float64` **only for names `fs.Visit` reported**:

```go
seen := map[string]bool{}
fs.Visit(func(f *flag.Flag) { seen[f.Name] = true })
if seen["remaining"] { eff.Remaining = &remainingVal }  // nil otherwise
```

Never `float64` + `omitempty` anywhere between the flag and the patch builder — that silently drops `--remaining 0`, the inverse bug.

**`stringList` precedent if `fs.Visit` proves insufficient** (`flags.go:76-87`):

```go
type stringList []string

func (p *stringList) String() string { return strings.Join(*p, ",") }

func (p *stringList) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*p = append(*p, part)
		}
	}
	return nil
}
```

---

### `cmd/adoBizInt/exit.go` — write exit mapper (utility, transform)

**Analog:** `exit.go:19-29`

```go
func exitCodeFor(failures []ado.Failure, complete bool) int {
	for _, f := range failures {
		if f.Kind == ado.FailureAuth {
			return exitAuth
		}
	}
	if !complete {
		return exitPartial
	}
	return exitOK
}
```

**Deviation:** a **new function**, not an overload. Writes return bare `error` — there is no `[]ado.Failure` and no `complete` flag, so this signature cannot be reused. Copy two properties verbatim: (1) auth is checked first and short-circuits, because auth is the one non-transient kind (`exit.go:6-13`); (2) it "never compares failure counts and never matches on error text" (`exit.go:15-18`) — the write mapper must key on `ado.FailureKind` + the parent `ctx.Err()`, never on message strings.

**Exit-code const block** (`dispatch.go:19-26`) — extend in place:

```go
// Process exit codes (D-06). These are a public contract from this plan onward:
// agents and CI jobs branch on them, so the integers are a one-way door.
const (
	exitOK      = 0 // the read completed and nothing failed
	exitPartial = 1 // the read ran but did not reach every configured project
	exitAuth    = 2 // at least one failure was an authentication rejection
	exitUnknown = 3 // the read never ran: usage error, config error, client error
)
```

**Deviation:** add exactly two constants in this same block with the same one-line-comment discipline (D-08 no-op-refused, D-09 outcome-unknown). RESEARCH recommends 4 and 5 ascending. Add `TestExitCodesArePairwiseDistinct` (Risk 8).

---

### `cmd/adoBizInt/audit.go` — JSONL sink (service, file-I/O)

**No single analog.** Two partial ones, both required.

**Serialization + single-Write discipline** — from `crash.go:55-68`:

```go
func writeCrashRecord(kind, message string) {
	payload, err := json.Marshal(crashEnvelope{...})
	if err != nil {
		return
	}
	payload = append(payload, '\n')
	_, _ = stdoutWriter.Write(payload)
}
```

**Deviation:** same marshal-then-one-`Write` shape (a partial line in a JSONL file is an unparseable record forever), but the target is the audit file, not `stdoutWriter`, and a marshal/write failure here must **not** be swallowed — D-13 makes the sink fatal.

**Open-for-append + directory** — from `internal/logging/logging.go:12-34`:

```go
	logDir := filepath.Join(home, ".config", "adoBizInt")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}
	logPath := filepath.Join(logDir, "adoBizInt.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
```

**Deviation:** same directory (`~/.config/adoBizInt`), filename `audit.jsonl` (D-12), so Phase 5 HARD-06's `0600`/`0700` covers both. **Do not derive the path from `logging.LogPath()`** — it returns the full *log file* path (`logging.go:46-49`), not the directory. RESEARCH Q5 is hereby resolved: `LogPath()` is `filepath.Join(home, ".config", "adoBizInt", "adoBizInt.log")`. Either add a `logging.ConfigDir()` or join independently.

**The D-13 ordering deviation — the whole decision:** the `os.OpenFile` call must appear **above** the pre-read and far above the mutate call in the function body. Ordering is the acceptance criterion, not the append itself.

**Third-sink rule:** `stdoutWriter` / `stderrWriter` are the two existing package-level sinks (`dispatch.go:31-34`). The audit file is a **third**, distinct from both. For testability, declare the audit path as a package-level var a test can point at `t.TempDir()`, mirroring how the writers are swapped.

---

### `internal/ado/workitems.go` — nullable effort (service, CRUD/JSON-Patch)

**Analog:** itself, `workitems.go:284-331` — the code being fixed.

Current signature (`workitems.go:284`):

```go
func (c *Client) UpdateWorkItemStateWithEffort(ctx context.Context, itemID int, newState string, originalEstimate, remaining, completed float64) error {
```

Current blind-overwrite builder (`workitems.go:287-316`):

```go
	addOp := webapi.OperationValues.Add
	statePath := "/fields/System.State"
	originalEstimatePath := "/fields/Microsoft.VSTS.Scheduling.OriginalEstimate"
	remainingPath := "/fields/Microsoft.VSTS.Scheduling.RemainingWork"
	completedPath := "/fields/Microsoft.VSTS.Scheduling.CompletedWork"

	// Update effort fields first, then state change
	// Using Add operation which works for both new and existing values
	document := []webapi.JsonPatchOperation{
		{Op: &addOp, Path: &originalEstimatePath, Value: originalEstimate},
		{Op: &addOp, Path: &remainingPath,        Value: remaining},
		{Op: &addOp, Path: &completedPath,        Value: completed},
		{Op: &addOp, Path: &statePath,            Value: newState},
	}
```

**Deviations (all four are load-bearing):**
1. Signature → `*float64` for the three effort params (D-17).
2. The nil check gates **`append`**, never `Value`. `JsonPatchOperation.Value` is `interface{}` — a nil `*float64` there serializes as `"value":null`, which is a *clear*, not a no-op. This is Risk 2 and the single most likely way to "fix" D-17 and still break it.
3. **Preserve operation order**: three effort ops, then state. `TestUpdateWorkItemStateWithEffortIssuesExactPatch` (`workitems_test.go:341`) `reflect.DeepEqual`s the whole slice literal; reordering survivors fails it (Risk 9).
4. **Keep `Op: Add`, not `Replace`** — the comment at `:294` states the rationale ("works for both new and existing values"); `Replace` fails on a never-set field.
5. Loop-variable aliasing: if refactored to a loop over `[]struct{path string; val *float64}`, bind `p := entry.path` inside the loop before taking its address (go.mod version UNVERIFIED).

The error-and-log tail (`workitems.go:323-330`) is unchanged and copied as-is.

---

### `internal/ado/*` — new by-id pre-reads (service, request-response read)

**Analog:** `workitems.go:334-351` (`GetWorkItemDetail`) — the one by-id read that exists.

```go
// GetWorkItemDetail fetches full details for a single work item
func (c *Client) GetWorkItemDetail(ctx context.Context, id int) (*WorkItemDetail, error) {
	log := logging.Logger()

	// Use Relations expand to get attachments (can't use Fields with Expand)
	expand := workitemtracking.WorkItemExpandValues.Relations

	item, err := c.workitemClient.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Id:     &id,
		Expand: &expand,
	})
	if err != nil {
		log.Error("Failed to get work item detail", "id", id, "error", err)
		return nil, err
	}

	if item == nil || item.Fields == nil {
		return nil, nil
	}
```

Field-extraction pattern to mirror three times for the effort gap (`workitems.go:370-372`):

```go
	if points, ok := itemFields["Microsoft.VSTS.Scheduling.StoryPoints"].(float64); ok {
		detail.StoryPoints = int(points)
	}
```

**Deviations:**
- **`GetWorkItemDetail` reads no effort field today** (confirmed: `:356-379` covers Title, Description, WorkItemType, State, StoryPoints, AcceptanceCriteria, ReproSteps, IterationPath…). A `current -> proposed` diff for `--remaining` is impossible until three `Microsoft.VSTS.Scheduling.*` blocks are added plus three `WorkItemDetail` fields. `Expand: Relations` already returns the full field map, so this is extraction only. **This belongs in plan 04-01, not 04-02** (Risk 4).
- `(nil, nil)` on a missing item (`:349-351`) is **wrong for a write pre-read** — D-02 makes a failed pre-read fatal. The new by-id reads should return a real error for "not found", not a nil/nil that a caller could read as an empty diff.
- Interface entries mostly already exist: `gitAPI.GetPullRequest` (`interfaces.go:28`), `buildAPI.GetDefinition` (`interfaces.go:37`), `releaseAPI.GetApprovals` (`interfaces.go:43`). **`GetApprovals` is a list call** — a by-id approval lookup likely needs a new `GetApproval` entry. Verify against the SDK; this is the only probable new interface entry (Risk 10 — D-01's cost is likely *over*-estimated).
- File placement is an explicit discretion item: `internal/ado` is ~2,015 lines against a 300-line standard and `workitems.go` alone is 591. State the placement in the plan.

---

### `internal/ado/interfaces.go` — new interface entry (model)

**Analog:** `interfaces.go:42-47`

```go
type releaseAPI interface {
	GetApprovals(context.Context, release.GetApprovalsArgs) (*release.GetApprovalsResponseValue, error)
	GetReleaseDefinitions(context.Context, release.GetReleaseDefinitionsArgs) (*release.GetReleaseDefinitionsResponseValue, error)
	GetReleases(context.Context, release.GetReleasesArgs) (*release.GetReleasesResponseValue, error)
	UpdateReleaseApproval(context.Context, release.UpdateReleaseApprovalArgs) (*release.ReleaseApproval, error)
}
```

**Deviation:** the header comment at `:12-16` is a hard constraint — "Each interface declares only methods this package calls; adding an uncalled method violates ROADMAP SC4." Add an entry **only** when the corresponding `ado` wrapper actually calls it in this phase.

---

### Test files (test, n/a)

**Analog A — the structural flag-registration guard**, `filter_test.go:311-349`. This is the exact shape for `TestConfirmRegisteredOnEveryWriteVerbAndNoReadVerb`:

```go
	for _, name := range commandKeys() {
		t.Run(name, func(t *testing.T) {
			want, ok := wantFlags[name]
			if !ok {
				t.Fatalf("command %q is not covered by this test's expectation table", name)
			}
			registered := map[string]bool{}
			fs, _, _ := newFlagSet(name)
			for _, flagName := range allFilterFlags {
				if fs.Lookup(flagName) != nil {
					registered[flagName] = true
				}
			}
			for _, flagName := range want { if !registered[flagName] { t.Errorf(...) } }
			for _, flagName := range allFilterFlags {
				if registered[flagName] && !contains(want, flagName) {
					t.Errorf("%q registers --%s, which its payload does not support", name, flagName)
				}
			}
		})
	}
```

**Deviation:** iterate `commandKeys()` (which will now include the seven write verbs), assert every write verb registers `confirm` and **no read verb does**. The `t.Fatalf` on an uncovered command is the drift guard — keep it. This is a structural assertion, not a grep-for-a-string, which is exactly what STATE.md's pending todo asks for.

**Analog B — the cmd-layer fake**, `cmd/adoBizInt/fakes_test.go:14-40`:

```go
type fakeReadClient struct {
	t *testing.T
	GetWorkItemsFunc func(ctx context.Context, projects []string) (ado.Result[ado.WorkItem], error)
	...
}

func (f *fakeReadClient) GetWorkItems(ctx context.Context, projects []string) (ado.Result[ado.WorkItem], error) {
	if f.GetWorkItemsFunc == nil {
		f.t.Fatalf("readClient.GetWorkItems called with no GetWorkItemsFunc set")
	}
	return f.GetWorkItemsFunc(ctx, projects)
}
```

**Deviation:** add a sibling `fakeWriteClient` with the seven mutating funcs plus the by-id read funcs. **The nil-func-field `t.Fatalf` is the SC1/SC2 "no mutation occurred" assertion for free** — a dry-run test sets only the pre-read func and leaves the mutating func nil, so any mutation is an automatic, precisely-named failure. Prefer this over a call counter: a counter asserts `== 0`; a nil field makes the call impossible to make silently.

**Analog C — the ado-layer fake + error injectors**, `fakes_test.go:14-42, 54-60`:

```go
var (
	_ workItemAPI = (*fakeWorkItemAPI)(nil)
	_ gitAPI      = (*fakeGitAPI)(nil)
	_ buildAPI    = (*fakeBuildAPI)(nil)
	_ releaseAPI  = (*fakeReleaseAPI)(nil)
)

func wrappedErrorValue(statusCode int, message string) error {
	return azuredevops.WrappedError{Message: &message, StatusCode: &statusCode}
}
func wrappedErrorPointer(statusCode int, message string) error {
	return &azuredevops.WrappedError{Message: &message, StatusCode: &statusCode}
}
func wrappedErrorNoStatus(message string) error {
	return azuredevops.WrappedError{Message: &message}
}

func (f *fakeWorkItemAPI) UpdateWorkItem(ctx context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
	f.t.Helper()
	if f.UpdateWorkItemFunc == nil {
		f.t.Fatalf("unexpected call to workItemAPI.UpdateWorkItem")
	}
	return f.UpdateWorkItemFunc(ctx, args)
}
```

**Deviation:** a write test injecting 409 / 403 / 500 must use these constructors, never `errors.New` — `classifyFailure` reads the status structurally via both the value and pointer shapes, and a bare error classifies as `FailureOther`.

**Analog D — the global-flag smoke loop**, `usage_test.go:13-25`, for the analogous "every write verb parses `--confirm`" pass.

---

## Shared Patterns

### Output sinks — three, kept distinct
**Source:** `dispatch.go:28-34`
**Apply to:** `write.go`, `audit.go`, `render.go`
```go
var (
	stdoutWriter io.Writer = os.Stdout
	stderrWriter io.Writer = os.Stderr
)
```
stdout carries exactly one parseable document; diagnostics go to stderr via `fmt.Fprintf(stderrWriter, ...)`; the audit file is a **third** sink. Tests swap the writers to buffers, so stdout purity (and "the failed pre-read emitted no report", D-02) is assertable without spawning a process.

### Single-Write JSON emission
**Source:** `dispatch.go:270-282`
**Apply to:** the write envelope, and (by analogy, to its own file) the audit record
```go
func emitJSON(v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	_, err = stdoutWriter.Write(payload)
	return err
}
```
Signature is `any`, so the write envelope goes through this exact function — no second emitter. A streaming encoder is deliberately rejected (it can leave a partial document ahead of a crash record).

### Structured logging at every boundary
**Source:** `dispatch.go:179`, `:221`, `:228`, `:232`; `workitems.go:256`, `:274`, `:278`
**Apply to:** every new `ado` method and the write path
```go
	log := logging.Logger()
	...
	log.Error("Failed to update work item state", "id", itemID, "state", newState, "error", err)
	...
	log.Info("Work item state updated", "id", itemID, "state", newState)
```
Key/value slog pairs, `Error` before returning an error, `Info` on success. The log is **not** the audit sink (D-12).

### Credential-disclosure rule on anything persisted or printed
**Source:** `crash.go:20-32` doc comment
**Apply to:** the write envelope, the audit record, all stderr messages
No stack traces, no argv, no URLs, no headers, no raw `error` values in persisted structures. Flatten a cause to a message string.

### Exit-code const discipline
**Source:** `dispatch.go:19-26`
**Apply to:** the two new codes — same block, same one-line comment per code, same "one-way door" framing.

---

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `cmd/adoBizInt/audit.go` | service | file-I/O append | No append-only JSONL sink exists. Composed from two partial analogs (`crash.go` serialization + `logging.go` file open). Planner should treat this as new construction. |
| `internal/ui/workitems/workitems.go:266,:333,:347,:354` | TUI call site | event-driven | UNVERIFIED — not read this session (tool budget). Planner must read before editing. `effort.go:18-27` shows `EffortSubmitMsg` carries plain `float64`, so the safe minimal change is wrapping each value in a pointer (always non-nil), preserving TUI behaviour exactly. Do **not** opportunistically make blank-input mean nil. |
| `cmd/adoBizInt/usage.go` | config/docs | n/a | UNVERIFIED — file exists (glob-confirmed) but not read. Phase 3 D-21's three-level help must gain seven verbs. |

---

## Metadata

**Analog search scope:** `cmd/adoBizInt/` (13 files, glob-confirmed), `internal/ado/`, `internal/logging/`
**Files read this session:** 04-CONTEXT.md, 04-RESEARCH.md, `dispatch.go`, `flags.go`, `exit.go`, `crash.go`, `render.go` (head), `cmd/.../fakes_test.go`, `filter_test.go` (tail), `usage_test.go` (head), `internal/ado/interfaces.go`, `internal/ado/fakes_test.go` (head), `internal/ado/workitems.go:250-379`, `internal/logging/logging.go`
**Grep passes:** write-method signatures across `internal/ado`; structural test names across `cmd/adoBizInt`
**Pattern extraction date:** 2026-09-12
