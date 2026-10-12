# Phase 6: Core-value cleanup - Research

**Researched:** 2026-10-12
**Domain:** Go CLI hardening: stdlib `flag` help detection, config validation, JSON envelope contract, panic recovery on the write path
**Confidence:** HIGH (every claim below is from source read this session, or from a scratch program run against the local Go toolchain; one ADO server-paging claim is ASSUMED and only affects an Open Question)

## Summary

All four warnings are small, local changes to code that already has the right seams. No new dependency, no new package, no new flag. The work splits cleanly into four independent units (W2, W3, W4, W1) that touch mostly disjoint files, so they can run as parallel plans. The only file touched by two of them is `cmd/adoBizInt/main.go`: W3 edits the config hint and W1 edits the `guard` comment. Those hunks do not overlap.

The biggest planning risks are not in the fixes themselves. They are in four details that are easy to miss:

1. **W2.** Re-parsing argv with the real `newFlagSet` before config is loaded fails, because the `--project` allow-list is empty at that point and rejects every value. `workitems list --project P --help` would exit 3 with "no projects are configured" instead of printing help. The help probe has to neutralise `Set`. It must not print usage from the neutralised FlagSet, because `PrintDefaults` then corrupts the output (verified). Write verbs also need `hoistLeadingPositionals` in the probe, or `set-state 4821 --help` stops printing help.
2. **W1.** `runWrite` closes the audit sink with `defer sink.Close()`. During a panic that defer runs **before** `guard`'s recover, so `handlePanic` can never append to it. The recover has to live inside `runWrite`, registered after `defer sink.Close()` (defers run LIFO) and immediately before `op.mutate`. The position of that `defer` statement is the structural "request issued" signal, so no flag variable is needed.
3. **W4.** Phase 3 D-11 says `--json` emits "the raw Phase 2 `Result[T]` envelope and nothing else". Adding `scope` at the CLI emit layer would need a wrapper type, which D-11 rejected. Put it on `ado.Result[T]` instead. Two code facts must shape the wording: "recent iterations" means **30** days in code, while the comments say 90; and when a project has no dated iteration in that window, **no** iteration filter applies.
4. **W3.** Two existing tests build a `config.Config` with no `Projects` and expect `Validate()` to pass: `internal/config/config_test.go:97-104` (the `base()` fixture) and `cmd/adoBizInt/main_test.go:735-743`. Both break the moment the rule lands.

**Primary recommendation:** Run four plans in one wave: W2 (`usage.go`), W3 (`config.go`), W4 (`result.go` plus three reads plus the CLI human line and help), and W1 (`write.go` plus `crash.go`). Write each test RED-first and pin it structurally (go/ast) where the property is positional.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

### W4 — Scope disclosure
- **D-01:** Every read envelope gains a **`scope` object**, structured data an
  agent can read without parsing prose. Example for `workitems list` /
  `dashboard counts`: `{"excluded_states": ["Done","Closed","Removed"], "iterations": "recent"}`;
  for `prs list`: `{"status": "active"}`. Exact key names are the planner's
  call within this shape, but they become part of the published JSON contract.
  — **Reversibility:** one-way — a published field in the `--json` envelope that agents branch on (Phase 2 D-01, Phase 3 D-06 contract); renaming or removing it later breaks callers.
- **D-02:** `scope` is present on **all 8 read envelopes**, `null` when a read
  has no built-in server-side filter. One shape for agents to check.
  — **Reversibility:** one-way — same published-contract reason as D-01.
- **D-03:** **Disclose only — no widening.** No `--include-done`,
  `--all-iterations` or `--status all` in this phase; changing the query is new
  capability and overlaps v2 DATA-01, which rewrites the same WIQL.
- **D-04:** Human (non-JSON) output prints **one scope line on stderr** for
  scoped reads (e.g. `scope: excludes Done/Closed/Removed; recent iterations only`).
  Stdout stays rows only (Phase 3 OUT-03 split).
- **D-05:** **Reword `--hide-done` help** so it is accurate: Done, Closed and
  Removed are always excluded by the server; the flag additionally drops
  Resolved and other done-like states client-side. The flag's behaviour is unchanged.
- **D-06:** Each scoped verb's `--help` text states its fixed scope.

### W2 — Help-token parsing
- **D-07:** `-h`, `--help`, `-help` mean help **only in flag position**, never
  as a flag's value. The bare word `help` means help only as the noun or verb
  (`adoBizInt help`, `adoBizInt workitems help`). `workitems list --search help`
  runs the read with search term "help".
- **D-08:** The same fix covers the **write side** (BL-02 WR-05,
  `--comment -h`), since it is the same `handleUsage` path. Remove WR-05 from
  `.planning/BACKLOG.md` BL-02 when this lands.
- **D-09:** `--help` must still work with no PAT and no config (Phase 3
  CLI-05); help detection stays ahead of `config.Load`.

### W3 — Empty projects
- **D-10:** `config.Validate` **rejects an empty `projects` list** with a
  `ConfigError{Field: "projects"}`. This applies to the CLI (exit 3, nothing
  ran) and the TUI (startup error) alike — a TUI with no projects shows only
  empty views, which is the same lie.
- **D-11:** **Blank or whitespace-only entries are rejected** too (e.g.
  `projects: [""]`), as a config error naming the `projects` field.

### W1 — Panic during a write
- **D-12:** Once a write's request has been issued, a **panic exits 5**
  (outcome unknown), mirroring Phase 4 D-22 for interrupts. A panic before the
  request is issued stays exit 3 (nothing ran).
- **D-13:** On a post-request panic, the recover path makes a **best-effort
  audit append with outcome `unknown`** if the audit sink is still usable. If
  that append itself fails, the crash record still goes to stderr and the
  exit is still 5. SAFE-02: every write that may have landed leaves a trace.
- **D-14:** Correct the stale comments in `cmd/adoBizInt/main.go` and
  `cmd/adoBizInt/crash.go` that call 3 "the outcome-unknown exit code".

### Claude's Discretion
- Exact key names inside `scope` and the stderr wording, within D-01/D-04.
- How the "request issued" moment is signalled to the panic handler for D-12 (e.g. a flag set in `runWrite` before `op.mutate`), provided it is structural and tested.
- Whether `scope` lives on the shared `Result` envelope or is added at the CLI emit layer, provided all 8 reads carry it (D-02) and the TUI is not required to render it.

### Deferred Ideas (OUT OF SCOPE)
- Widening flags (`--include-done`, `--all-iterations`, `--status all`) — v2 DATA-01 territory.
- Remaining audit tech debt (W5 dashboard string-literal coupling, W6 operation-name sanitisation, orphaned exports, prs.go duplicated summariser) — stays in the audit report / BL-02.
</user_constraints>

<phase_requirements>
## Phase Requirements

All seven requirements are already satisfied. This phase closes the warnings recorded against them.

| ID | Description (REQUIREMENTS.md) | Research Support |
|----|-------------|------------------|
| CLI-05 | Every command responds to `--help` with its flags and arguments | W2: the stdlib-driven help probe in `handleUsage` keeps help ahead of `config.Load` (D-09) and fixes the value-position false positive. Pitfalls 1-3. |
| CLI-06 | Filtering an agent would want ... is available as flags | W2 (`--search help` is a real filter value). W4: the `--hide-done` help is reworded (D-05). |
| OUT-04 | Exit codes distinguish success, partial result, authentication failure, and outcome-unknown | W1 (post-request panic exits 5), W2 (a non-read no longer exits 0), W3 (misconfiguration exits 3, not 0). |
| OUT-05 | A panic emits structured JSON rather than dying silently | W1: the crash record is still emitted, now with exit 5 on the write path after mutate. |
| TRUST-03 | A partial result is explicitly marked incomplete | W3: no `complete: true` over zero projects. W4: `scope` tells "outside the scope" apart from "does not exist". |
| SAFE-02 | Every executed write appends an audit record | W1: best-effort `unknown` record on a post-request panic (D-13). |
| SAFE-04 | A write that times out reports outcome-unknown | W1 extends the same classification to panics (D-12). |
</phase_requirements>

## Project Constraints (from CLAUDE.md and org rules)

No `./CLAUDE.md` exists in the repo. These directives come from the user's global CLAUDE.md and the Uniti org instructions:

- **Simplest code.** Single-use logic stays inline, or in a private function in the same file. Never extract it to a shared util.
- **Rule of three.** Extract only on a third occurrence. Two occurrences get a `// note:` comment instead (the repo already does this, e.g. `wiql_test.go:28`).
- **No speculative generality.** No new interfaces, managers, factories or config options.
- **No new dependencies or architectural layers** without justification. This phase needs none.
- **Reuse before creating.** `hoistLeadingPositionals`, `newAuditRecord`, `appendAuditRecord`, `writeCrashRecord`, `handlePanic`, `failingAuditSink`, `tempAuditSink`, `preReadFake` and `setStateArgs` already exist and must be reused.
- **Target sizes.** Under 300 lines per module. `write.go` is already 612 lines, so keep the W1 addition to roughly 20 lines inside `runWrite`.
- **OWASP.** No secrets in output. The crash record and audit record must keep carrying no stack, argv or URL (existing T-03-14 / T-04-01 / T-04-02).
- **Repo convention (STATE.md pending todo F-03-3).** Prefer go/ast structural assertions over grep-negative criteria. Comments are not in the AST.
- **Self-review.** No new symbol may duplicate an existing one.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Help-token detection (W2) | CLI pre-config (`cmd/adoBizInt/usage.go` `handleUsage`) | stdlib `flag` | Must run before `config.Load` (D-09). Position vs value is stdlib's job. |
| Empty/blank projects (W3) | Config (`internal/config/config.go` `Validate`) | CLI startup (`main.go` `run`) | `run()` calls `Validate` before both the CLI branch and the TUI branch, so one rule covers both. |
| Scope data (W4) | Data access (`internal/ado`: `Result[T]` plus the three reads that own the queries) | — | The read that builds the WIQL or search criteria is the one place that knows the scope. D-11 forbids a CLI wrapper. |
| Scope presentation (W4) | CLI (`render.go` stderr line, `usage.go` help, `flags.go` `--hide-done` text) | — | Human rendering and help are CLI concerns. The TUI is not required to render it. |
| Panic classification on writes (W1) | CLI write path (`write.go` `runWrite`) | `crash.go` `handlePanic` | Only `runWrite` knows the mutate boundary and still holds the open audit sink. |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `flag` | Go 1.25.4 (go.mod / CI), 1.27.1 local | Flag position vs value disambiguation, `flag.ErrHelp` | Already the CLI parser (Phase 3 D-19/D-20). It handles `-h`, `-help`, `--h`, `--help` and `--help=x` natively when no `h`/`help` flag is defined (verified below). |
| Go stdlib `encoding/json` | same | `scope` field marshalling | Existing envelope discipline: marshal to bytes, then one Write. |
| `github.com/microsoft/azure-devops-go-api/azuredevops` | v1.0.0-b5 | `git.PullRequestStatusValues.Active` = `"active"`, `release.ApprovalStatusValues.Pending` = `"pending"` | Lets the scope value be derived from the same constant the request uses, so the two cannot drift. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `go/ast`, `go/parser`, `go/token` | stdlib | Structural tests (defer position in `runWrite`; `Validate` before the TUI/dispatch branch) | Repo precedent: `internal/ado/nilguard_test.go:139` `parsePackageFiles`, `wiql_test.go:195`. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| stdlib probe parse | Hand-scanning argv with `fs.Lookup` arity | Re-implements `flag.parseOne`. CONTEXT already notes stdlib "distinguishes flag position from flag value". Rejected. |
| `scope` on `Result[T]` | CLI wrapper `struct{ ado.Result[T]; Scope }` in `emitJSON` | Contradicts Phase 3 D-11 ("raw `Result[T]` envelope and nothing else"; wrapper "rejected"). Puts scope knowledge in `cmd`, far from the WIQL. Rejected. |
| `defer`/`recover` inside `runWrite` | Package var `writeIssued` read by `handlePanic` | The sink is already closed by `runWrite`'s `defer sink.Close()` when `guard` recovers, and the target and diff would need global state. Rejected. |

**Installation:** none. No new module. `go.mod` is unchanged.

## Package Legitimacy Audit

This phase installs no external packages. Every API used is the Go standard library or the SDK module the repo already pins at `v1.0.0-b5` (`go list -m` this session).

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| (none) | — | — | — | — | — | — |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
argv ──► run() ──► initLogging
           │
           ├─► handleUsage(argv)                       [W2: before config]
           │     ├─ args[0] ∈ helpTokens ─► usageRoot, exit 0
           │     ├─ args[1] ∈ helpTokens ─► usageNoun, exit 0
           │     └─ valid "noun verb":
           │          probe := newFlagSet(name), Values wrapped (Set = no-op), silent
           │          probe.Parse(hoistLeadingPositionals(args[2:]))
           │             ├─ ErrHelp ─► usageVerb(name, fresh newFlagSet), exit 0
           │             └─ anything else ─► not handled (fall through)
           │
           ├─► config.Load ─► cfg.Validate()                [W3: projects non-empty, no blank entry]
           │        └─ error ─► stderr "Invalid config: projects: ..." + hint, exit 3   (CLI and TUI)
           │
           ├─► len(argv)>1 ─► dispatch ─► runRead / runWrite
           │                    │
           │   runRead: fetch ─► ado.Result{items, failures, complete, scope}   [W4]
           │      ├─ --json ─► emitJSON(result)  (scope: {...} | null)
           │      └─ human  ─► rows→stdout; renderScope→stderr; renderFailures→stderr
           │
           │   runWrite: parse → resolve → auditOpen; defer sink.Close()
           │      → plan (pre-read)  [panic here ─► guard ─► exit 3, no audit]
           │      → no-op / dry-run return
           │      → defer recover-if-panicked {append unknown if not yet audited;
           │                                    handlePanic(r, exitOutcomeUnknown)}   [W1]
           │      → op.mutate → classify → audit append → finishWrite
           │
           └─► TUI (ui.NewModel) — reached only after Validate passed
```

### Recommended Project Structure (files touched)
```
cmd/adoBizInt/
├── usage.go        # W2: replace args[2:] scan with a stdlib probe; W4: per-verb scope help line
├── usage_test.go   # W2 tests (value vs flag position, pre-config --project, hoisted target)
├── flags.go        # W4/D-05: --hide-done usage string
├── dispatch.go     # W4: one renderScope call in runRead's human branch
├── render.go       # W4: renderScope(w, *ado.Scope)
├── write.go        # W1: named return + deferred recover before op.mutate
├── crash.go        # W1: handlePanic takes the exit code; D-14 comment
├── main.go         # W1/D-14 guard comment; W3 config hint mentions ADO_PROJECTS
├── crash_test.go / write_test.go / main_test.go   # W1 tests, W4 human-line test, W3 fixture fix
internal/config/
├── config.go       # W3: two checks in Validate
└── config_test.go  # W3: base() gains Projects; new rows
internal/ado/
├── result.go       # W4: Scope type + Result.Scope field
├── workitems.go / dashboard.go / pullrequests.go (/ releases.go — see OQ1)  # W4: set Scope
└── json_test.go / wiql_test.go / *_test.go     # W4 contract + tie tests
```

### Pattern 1: W2 stdlib help probe (no hand-rolled argv scan)

**What:** Before config loads, parse the verb's arguments with the verb's real FlagSet. Wrap every flag's `Value` so `Set` accepts anything without side effects (the `--project` allow-list would otherwise reject every value). Run it silent. `flag.ErrHelp` means a help token was in flag position.

**Current code being replaced** `[VERIFIED: cmd/adoBizInt/usage.go:178-186]`:
```go
	// A valid subcommand: verb-level help is handled here rather than by the
	// FlagSet, so it too runs before config is loaded.
	for _, arg := range args[2:] {
		if helpTokens[arg] {
			fs, _, _, _ := newFlagSet(noun + " " + verb)
			usageVerb(noun+" "+verb, fs)
			return exitOK, true
		}
	}
```
with `helpTokens` `[VERIFIED: cmd/adoBizInt/usage.go:11-16]`:
```go
var helpTokens = map[string]bool{
	"--help": true,
	"-help":  true,
	"-h":     true,
	"help":   true,
}
```
`helpTokens` stays for the `args[0]` and `args[1]` checks (`usage.go:148`, `:168`). That keeps `adoBizInt help` and `adoBizInt workitems help` working per D-07.

**Recommended shape** (a private helper in usage.go, or inline in `handleUsage`; it is single-use):
```go
// Source: stdlib flag semantics verified by scratch run (see Verification Evidence)
probe, _, _, _ := newFlagSet(name)
probe.SetOutput(io.Discard)
probe.Usage = func() {}
// Before config.Load the --project allow-list is empty and rejects every value,
// so the probe applies no value at all: it only asks stdlib where the flags are.
probe.VisitAll(func(f *flag.Flag) { f.Value = helpProbeValue{f.Value} })
if errors.Is(probe.Parse(hoistLeadingPositionals(args[2:])), flag.ErrHelp) {
	fs, _, _, _ := newFlagSet(name) // fresh: PrintDefaults on wrapped Values is corrupt
	usageVerb(name, fs)
	return exitOK, true
}
return exitOK, false

// helpProbeValue keeps a flag's arity (bool flags take no value) but applies nothing.
type helpProbeValue struct{ flag.Value }
func (helpProbeValue) Set(string) error { return nil }
func (v helpProbeValue) IsBoolFlag() bool {
	b, ok := v.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}
```
- `IsBoolFlag` must be forwarded. Without it, `--json --help` would treat `--help` as `--json`'s value. The wrapper above returns true for `-json` (verified).
- `hoistLeadingPositionals` (`write.go:597`) is required so `workitems set-state 4821 --help` still prints help. stdlib stops parsing at the first non-flag (`4821`). Applying it to read verbs too keeps `prs list foo --help` printing help, as it does today.
- Non-help parse errors (`--bogus`, `--search` with no value) return "not handled". `dispatch` then reports them after config loads, which is unchanged behaviour.

### Pattern 2: W3 two checks in `Validate`

Current `[VERIFIED: internal/config/config.go:171-184]`:
```go
func (c *Config) Validate() error {
	if c.PAT == "" {
		return &ConfigError{Field: "pat", Message: "Personal Access Token is required"}
	}
	if c.OrgURL == "" {
		return &ConfigError{Field: "org_url", Message: "Organization URL is required"}
	}
	// ...
	if c.FetchTimeout <= c.PollInterval {
		return &ConfigError{Field: "fetch_timeout", Message: "must be greater than poll_interval"}
	}
	return nil
}
```
Add, after `org_url`:
```go
if len(c.Projects) == 0 {
	return &ConfigError{Field: "projects", Message: "at least one project is required"}
}
for _, p := range c.Projects {
	if strings.TrimSpace(p) == "" {
		return &ConfigError{Field: "projects", Message: "project names must not be blank"}
	}
}
```
`strings` is already imported (`config.go:8`).

**Where blanks can come from** `[VERIFIED: config.go:17-37, 121-124, 156-164]`:
- The YAML **sequence** path (`projects: [""]` or `["  "]`) decodes verbatim with no trim. This is the D-11 case.
- The backward-compatible `project: "  "` path becomes `Projects = ["  "]` (`config.go:122-123`).
- The YAML scalar path and the `ADO_PROJECTS` path already trim and drop empty parts. `ADO_PROJECTS=" , "` therefore leaves the list empty, which the length check catches.

**TUI coverage is free.** `run()` calls `cfg.Validate()` at `main.go:91` before both `dispatch` (`:135`) and `ui.NewModel` (`:138`) `[VERIFIED: cmd/adoBizInt/main.go:91-97,126-138]`. The validation failure returns `exitUnknown` (3) on both paths.

Also update the hint at `main.go:94-95` ("Please set ADO_PAT and ADO_ORG_URL environment variables") so it mentions `ADO_PROJECTS` or `projects:`. As it stands, a projects error tells the user to set two variables that are already set.

### Pattern 3: W4 `scope` on `Result[T]`

Current `[VERIFIED: internal/ado/result.go:98-102]`:
```go
type Result[T any] struct {
	Items    []T       `json:"items"`
	Failures []Failure `json:"failures"`
	Complete bool      `json:"complete"`
}
```
Recommended:
```go
// Scope discloses the fixed filter Azure DevOps applies server-side for this
// read, so a caller can tell "outside the scope" from "does not exist". nil
// (JSON null) means the read applies no built-in server-side filter (D-02).
// It never affects Complete. Keys are a published one-way contract (D-01).
type Scope struct {
	ExcludedStates []string `json:"excluded_states,omitempty"`
	Iterations     string   `json:"iterations,omitempty"`
	PRStatus       string   `json:"pr_status,omitempty"`
	ApprovalStatus string   `json:"approval_status,omitempty"` // only if OQ1 is accepted
}

type Result[T any] struct {
	Items    []T       `json:"items"`
	Failures []Failure `json:"failures"`
	Complete bool      `json:"complete"`
	Scope    *Scope    `json:"scope"` // NO omitempty: null must be emitted (D-02)
}
```
- **No `omitempty` on `Scope`.** D-02 needs the key present as `null`. A nil pointer marshals to `null`.
- `newResult` does not change. The scoped reads set `r.Scope` after `newResult` on **every** return path. `GetWorkItems` has an early return at `workitems.go:102-105` (`return newResult([]WorkItem{}, nil), nil`), and `GetDashboardCounts` has one at `dashboard.go:58-61`.
- **Key name `pr_status`, not `status`.** `dashboard counts` carries both the work-item filter and the Active-only PR count (`dashboard.go:155-160` `Status: &git.PullRequestStatusValues.Active`). A bare `status` on the dashboard would be ambiguous. One key for one meaning across both verbs is simpler for agents. (D-01 lets the planner pick key names.)
- Derive the status values from the SDK constant the request already uses, e.g. `string(git.PullRequestStatusValues.Active)`. The SDK defines `Active: "active"` `[VERIFIED: $GOMODCACHE/.../azuredevops/git/models.go:2591-2602]`. The scope string and the request then cannot drift.
- The excluded-state list **must stay a literal inside the WIQL**. `TestWIQLInterpolationStructural` (`wiql_test.go:195-207`) forbids non-literal operands in a WIQL `+` chain beyond `iterationClause`. Tie the scope to the query with a **test** instead: assert the captured query contains `NOT IN ('` + `strings.Join(scope.ExcludedStates, "', '")` + `')`. The capture fakes already exist in `TestGetWorkItemsUsesProjectMacro` (`wiql_test.go:27`) and `TestDashboardUsesProjectMacro` (`:105`).

**Scope per read** (evidence read this session):

| Verb | Server-side filter in code | Recommended `scope` |
|------|---------------------------|---------------------|
| `workitems list` | `workitems.go:163` `NOT IN ('Done', 'Closed', 'Removed')` + `iterationClause(iterations)` | `{"excluded_states":["Done","Closed","Removed"],"iterations":"recent"}` |
| `dashboard counts` | `dashboard.go:136` same WIQL; `:158` PR `Status: &git.PullRequestStatusValues.Active` | `{"excluded_states":["Done","Closed","Removed"],"iterations":"recent","pr_status":"active"}` |
| `prs list` | `pullrequests.go:66` `Status: &git.PullRequestStatusValues.Active` | `{"pr_status":"active"}` |
| `releases approvals` | `releases.go:257` `statusFilter := release.ApprovalStatusValues.Pending` | **See OQ1.** By D-02's rule: `{"approval_status":"pending"}` |
| `branches list`, `pipelines runs`, `pipelines definitions`, `releases list` | No status or state filter (latest-per-definition `Top: 1` and config name patterns; see OQ2) | `null` |

**"recent" means 30 days, and it is conditional** `[VERIFIED: internal/ado/client.go:190, 233-239; client.go:289-292]`:
```go
	cutoffDate := time.Now().AddDate(0, 0, -30)
```
```go
func iterationClause(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
```
- An iteration counts as recent when its start or finish date is after the 30-day cutoff. Undated iterations are never included.
- If a project has no qualifying iteration, `iterationClause` returns `""` and **no iteration filter applies** to that project.
- The comments at `client.go:167,209,231`, `workitems.go:147` and `dashboard.go:120` say "90 days". They are wrong. Help text and the stderr line must say 30 days, not 90. Correcting those comments in the same plan is a cheap honesty fix (not required by CONTEXT).

**Human stderr line** (one `Write`, following `render.go`'s single-write discipline). Add to `render.go` and call it from the human branch of `runRead` (`dispatch.go:323-330`), next to `renderFailures(stderrWriter, ...)`:
```go
// renderScope writes the one D-04 scope line to stderr; nothing when unscoped.
func renderScope(w io.Writer, s *ado.Scope) {
	if s == nil {
		return
	}
	var parts []string
	if len(s.ExcludedStates) > 0 {
		parts = append(parts, "excludes "+strings.Join(s.ExcludedStates, "/"))
	}
	if s.Iterations != "" {
		parts = append(parts, s.Iterations+" iterations only")
	}
	if s.PRStatus != "" {
		parts = append(parts, s.PRStatus+" pull requests only")
	}
	_, _ = io.WriteString(w, "scope: "+strings.Join(parts, "; ")+"\n")
}
```
It must print even when there are zero rows. That is exactly the case W4 exists for. It must not print under `--json`, where the envelope carries the scope.

**Help (D-06).** Add a `verbScopes` map in usage.go keyed like `verbSummaries` (`usage.go:46-62`) and print it in `usageVerb` (`usage.go:119-131`) as a `Scope:` paragraph, e.g. `workitems list`: "Azure DevOps returns only items not in Done, Closed or Removed, in iterations dated within the last 30 days (a project with no such iteration is not limited by iteration). --hide-done additionally drops Resolved." A help-text test pins the content: each scoped verb's help contains each of `Done`, `Closed`, `Removed`, `30 days`, `active`, as appropriate.

**`--hide-done` (D-05).** Current text `[VERIFIED: cmd/adoBizInt/flags.go:227-230]`:
```go
	if sup.hideDone {
		fs.BoolVar(&ff.hideDone, "hide-done", false,
			"drop items in a done or resolved state (off by default)")
	}
```
`doneStates` is `Done`, `Closed`, `Resolved`, `Removed` `[VERIFIED: internal/ado/filters.go:9-14]`. The server already removes three of those, so the flag's only effective extra is `Resolved`. Suggested text: `"also drop Resolved items; Done, Closed and Removed are always excluded by Azure DevOps (off by default)"`.

### Pattern 4: W1 deferred recover at the mutate boundary

Current ordering `[VERIFIED: cmd/adoBizInt/write.go:196-202, 230, 237, 244-267]`:
```go
	sink, err := auditOpen()
	if err != nil {
		...
		return exitUnknown
	}
	defer sink.Close()
	...
	err = op.mutate(writeCtx, target, wf)
	...
	outcome, code := writeExitCodeFor(err, ctx.Err())
	...
	if outcome == outcomeExecuted || outcome == outcomeUnknown {
		record := newAuditRecord(writeRecord{
			Command: name, Target: target, Changes: changes, Outcome: outcome,
		})
		if auditErr := appendAuditRecord(sink, record); auditErr != nil {
```
Current panic path `[VERIFIED: cmd/adoBizInt/main.go:36-43; cmd/adoBizInt/crash.go:87-99]`:
```go
func guard(body func() int) (code int) {
	defer func() {
		if r := recover(); r != nil {
			code = handlePanic(r)
		}
	}()
	return body()
}
```
```go
func handlePanic(r any) int {
	message := fmt.Sprint(r)
	if loggingReady { logging.Logger().Error("panic recovered", "message", message, "stack", string(debug.Stack())) }
	writeCrashRecord(crashKindPanic, message)
	return exitUnknown
}
```
Recommended change (illustrative):
```go
func runWrite(ctx context.Context, op writeOp, args []string) (code int) {
	...
	defer sink.Close()
	...
	// D-12: the request is about to leave. Registered here, AFTER defer
	// sink.Close() (defers run LIFO, so this runs first and the sink is still
	// open) and BEFORE op.mutate, so its position is the "request issued" signal:
	// a panic in resolve/plan never reaches it and still exits 3 via guard.
	audited := false
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		if !audited { // D-13 best effort; never a second record for one write
			rec := newAuditRecord(writeRecord{Command: name, Target: target, Changes: changes, Outcome: outcomeUnknown})
			if err := appendAuditRecord(sink, rec); err != nil {
				fmt.Fprintf(stderrWriter, "Write may have landed but was NOT recorded in %s: %v\n", auditPath, err)
			}
		}
		code = handlePanic(r, exitOutcomeUnknown)
	}()

	err = op.mutate(writeCtx, target, wf)
	...
		audited = true   // set where the normal append is attempted
		if auditErr := appendAuditRecord(sink, record); auditErr != nil { ...
```
- Change `handlePanic(r any) int` to `handlePanic(r any, code int) int`. `guard` passes `exitUnknown` and `runWrite` passes `exitOutcomeUnknown`. Its doc comment then stops claiming 3 is "the outcome-unknown exit code" (D-14). Fix `main.go:26-28` the same way, and fix `main.go:62-64` ("its outcome is unknown rather than incomplete") to say "nothing ran".
- The crash record shape is unchanged: `{"error":{"kind":"panic","message":...,"log_path":...}}` (`crash.go:15-18,28-42`). Consider prefixing the message (free text, not a key), e.g. `"panic after the write was issued; outcome unknown: " + fmt.Sprint(r)`, as the interrupt record does at `dispatch.go:297-298`. That is an implementation choice inside `handlePanic`'s caller.
- `recover()` returns nil for `runtime.Goexit`. The fakes' `t.Fatalf` on a nil closure (`fakes_test.go:166-172`) therefore still behaves as before.

### Anti-Patterns to Avoid
- **Parsing with the real FlagSet before config.** The `--project` Func callback (`flags.go:198-215`) rejects every value while `configuredProjects` is nil, so `--project P --help` fails before stdlib reaches `--help`.
- **Printing usage from the probe FlagSet.** `PrintDefaults` on wrapped Values prints `value` instead of `duration`/`string` and emits `panic calling String method on zero ...` lines (verified).
- **`omitempty` on `Result.Scope`.** This silently breaks D-02's "null when unscoped".
- **Building the WIQL `NOT IN` list from the scope slice.** It trips `TestWIQLInterpolationStructural` and reintroduces an interpolation surface. Keep the literal and tie it with a test.
- **Recovering in `guard` and re-opening the audit file there.** This needs global state for target and changes, and duplicates `runWrite`'s knowledge.
- **A `writeIssued` package variable.** It needs per-test reset, and it races the closed sink.
- **Appending a second audit record** when a panic follows the normal append (e.g. in `finishWrite`). This is guarded by `audited`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| "Is this token a flag or a flag's value?" | An argv scanner that tracks which flags take values | stdlib `flag.Parse` → `flag.ErrHelp` on a wrapped-Value FlagSet | stdlib already encodes `--`, `--x=v`, bool-vs-value arity, `-h`/`--h`/`-help`/`--help=x` |
| Leading positional before flags (`set-state 4821 --help`) | New reordering logic | `hoistLeadingPositionals` (`write.go:597-612`) | Already the write path's pre-parse step |
| Audit append on panic | A new writer or encoder | `newAuditRecord` + `appendAuditRecord` (`audit.go`) | Single Write + `Sync()`, the D-15 five-key shape |
| Crash record | A second record type | `writeCrashRecord` via `handlePanic` | Published one-way shape (Phase 3 D-26) |
| Audit-sink failure in tests | Permission-based fixtures | `failingAuditSink(t)` (`audit_test.go:228-243`) | Works under root (WR-09) |
| Source parsing for structural tests | Regex over source | `go/parser` + `go/ast` (`cmd` has no helper yet; parse `write.go`/`main.go` directly with `parser.ParseFile`) | Comments cannot satisfy or defeat it (F-03-3) |

**Key insight:** every piece W1–W4 needs already exists in this package. The phase is wiring plus contract tests. Any new helper beyond `helpProbeValue`, `Scope`, `renderScope` and `verbScopes` should be challenged in plan review.

## Common Pitfalls

### Pitfall 1: `--project P --help` stops printing help (W2)
**What goes wrong:** The probe uses an unwrapped `newFlagSet`. The allow-list closure sees `configuredProjects == nil` (config is not loaded yet) and returns "not a configured project ... (no projects are configured)". Parse stops there.
**How to avoid:** Wrap `Set` (Pattern 1). Test with `withConfiguredProjects(t)` given **no names**, which models the pre-config state.
**Warning signs:** A test for `workitems list --project P --help` returns `handled=false`.

### Pitfall 2: Target-first write help stops working (W2)
**What goes wrong:** `workitems set-state 4821 --help` → stdlib stops at `4821` → no ErrHelp → falls through to `config.Load`. CLI-05 regresses for the documented spelling.
**How to avoid:** Run `hoistLeadingPositionals(args[2:])` before the probe parse.

### Pitfall 3: Corrupted verb help (W2)
**What goes wrong:** Usage is printed through the wrapped FlagSet.
**How to avoid:** Keep the probe silent (`SetOutput(io.Discard)`, `Usage = func(){}`) and print from a fresh `newFlagSet(name)`. `TestHelpIsDeterministic` (`usage_test.go:91`) still holds.

### Pitfall 4: Stray read positionals (W2-adjacent, pre-existing)
**What goes wrong:** `runRead` never checks `fs.Args()` (`dispatch.go:243-252`). `workitems list help --json` therefore parses `help` as a positional, stops, and silently drops `--json`, giving human output. After W2, the bare `help` there is no longer help (D-07).
**How to avoid:** See OQ3. This phase does not have to fix it, but the planner should decide explicitly.

### Pitfall 5: Existing tests that build a config with no projects (W3)
**What goes wrong:** `config_test.go:97-104` `base()` has no `Projects`, so the "valid config" row fails. `main_test.go:735-743` (`TestTimeoutOverridesConfiguredBudgetAfterValidation`) fails at its first `Validate()`.
**How to avoid:** Add `Projects: StringSlice{"P"}` (config_test) and `Projects: config.StringSlice{"P"}` (main_test) in the same commit as the rule. In RED-first order, the new rule-row tests go red first, then these fixtures are fixed with the implementation.

### Pitfall 6: Missed early-return paths for scope (W4)
**What goes wrong:** `GetWorkItems` (`workitems.go:102-105`) and `GetDashboardCounts` (`dashboard.go:58-61`) return through `newResult` before the loop. A scope set only at the final return leaves `null` on those paths.
**How to avoid:** Set it on every return. Test with an empty `projects` client.

### Pitfall 7: Overstating the scope (W4)
**What goes wrong:** The help says "recent" or "90 days" when the code means 30, or claims an iteration limit that does not apply when a project has no dated recent iteration (`client.go:233`, `client.go:289-292`).
**How to avoid:** Help text states 30 days and the no-iteration fallback. The JSON value stays the compact `"recent"`, and help is where it is defined.

### Pitfall 8: `scope` changing `complete` (W4)
**What goes wrong:** Someone sets `Complete=false` for scoped reads "for honesty". The live evidence in CONTEXT says all eight reads must keep their current `complete` values.
**How to avoid:** Add a test row asserting that a scoped read with no failures is `complete: true` and that its `scope` is non-null.

### Pitfall 9: Sink already closed when guard recovers (W1)
**What goes wrong:** The best-effort append is placed in `guard`/`handlePanic`. `runWrite`'s `defer sink.Close()` has already run during unwinding, so the append fails and D-13 is not met.
**How to avoid:** Use the recover inside `runWrite`, registered after `defer sink.Close()`. Pin this with a go/ast test: in `runWrite`'s body, the `DeferStmt` whose `FuncLit` calls `recover()` is the statement immediately before the statement containing `op.mutate`, and it comes after the `defer sink.Close()` statement.

### Pitfall 10: Double audit record (W1)
**What goes wrong:** A panic after the normal append (e.g. in `finishWrite`'s `emitJSON`) appends a second `unknown` line for a write already recorded as `executed`.
**How to avoid:** Use the `audited` local, set where the normal append is attempted. A test can drive it with a stdout writer that panics on its first `Write` only.

### Pitfall 11: D-13's "crash record still goes to stderr" (W1)
**What goes wrong:** The record is moved to stderr. Phase 3 D-23 puts the crash record on **stdout** (stderr-only was rejected), and D-26 makes the shape one-way.
**How to avoid:** Keep `writeCrashRecord` on stdout. The audit-failure diagnostic is the stderr line. See OQ4.

### Pitfall 12: Read-path panic tests must still expect 3 (W1)
`TestPanicExitsThree`, `TestPanicWithFailedLoggingWritesNoStackToStderr` and `TestPATNeverLogged` (`patleak_test.go:71-75`) drive a **read** panic and assert `exitUnknown`. They must stay green unchanged.

## Code Examples

### W2 test rows (usage_test.go)
```go
// Source: pattern of TestHelpNeedsNoCredentials (usage_test.go:51-87)
func TestHelpTokenAsFlagValueIsNotHelp(t *testing.T) {
	withConfiguredProjects(t) // nil list: models the pre-config state
	for _, args := range [][]string{
		{"workitems", "list", "--search", "help"},
		{"workitems", "list", "--search", "-h"},
		{"workitems", "list", "--search", "--help"},
		{"workitems", "list", "--search=-help"},
		{"releases", "approve", "77", "--project", "P", "--comment", "-h"}, // WR-05
		{"pipelines", "queue", "7", "--project", "P", "--branch", "-h"},
		{"workitems", "list", "help"}, // bare word, not noun/verb position (D-07)
	} {
		stdout, stderr := captureOutput(t)
		if _, handled := handleUsage(args); handled { t.Errorf(...) }
		if stdout.Len()+stderr.Len() != 0 { t.Errorf(...) } // probe is silent
	}
}

func TestHelpTokenInFlagPositionIsHelp(t *testing.T) {
	withConfiguredProjects(t)
	for _, args := range [][]string{
		{"workitems", "list", "--project", "P", "--help"}, // Pitfall 1
		{"workitems", "set-state", "4821", "--help"},      // Pitfall 2
		{"workitems", "list", "--search", "x", "-h"},
		{"prs", "list", "--json", "-help"},
	} { /* handled == true, code == exitOK, stderr contains "Usage: adoBizInt", stdout empty */ }
}
```
End to end for D-08: add a row to `TestReleaseCommentReachesTheMutation` (`writeverbs_test.go:136-168`) with `--comment -h`, assert `handleUsage(args)` is not handled, and assert the mutation receives `"-h"`.

### W1 test (crash_test.go or write_test.go)
```go
// Source: helpers preReadFake/setStateArgs/tempAuditSink (write_test.go:31-65),
// assertAuditLineCount/decodeAuditRecord (audit_test.go:18-56), decodeCrash (crash_test.go:44)
func TestPanicAfterWriteIssuedExitsOutcomeUnknown(t *testing.T) {
	withConfiguredProjects(t, "P")
	sink := tempAuditSink(t)
	stdout, _ := captureOutput(t)
	c := preReadFake(t, "Active")
	c.UpdateWorkItemStateFunc = func(context.Context, string, int, string) error {
		panic("deliberate panic mid-write")
	}
	code := guard(func() int { return dispatch(context.Background(), c, setStateArgs("--confirm")) })
	// want exitOutcomeUnknown; stdout top-level keys exactly [error], kind "panic";
	// one audit line, outcome "unknown", command "workitems set-state", target.id 4821
}
// Companions: panic in GetWorkItemDetailFunc → exitUnknown, 0 audit lines;
// failingAuditSink + panicking mutate → still exitOutcomeUnknown, stderr "NOT recorded".
```

### W4 contract test updates (json_test.go)
Current assertions `[VERIFIED: internal/ado/json_test.go:155-166]`:
```go
	t.Run("Result", func(t *testing.T) {
		assertKeySet(t, Result[WorkItem]{}, []string{"items", "failures", "complete"})
	})
	...
		assertKeySet(t, Result[Branch]{}, []string{"items", "failures", "complete"})
```
Every `{"items", "failures", "complete"}` set in `TestJSONFieldNames` becomes `{"items", "failures", "complete", "scope"}`. Also add one assertion that `newResult[Branch](nil, nil)` marshals `"scope":null` (alongside `TestNewResultEmitsEmptyArraysNotNull`, `json_test.go:423`), and one exact-JSON assertion per scoped read.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Help when any token in `args[2:]` is a help token | Help only when stdlib parses a help flag (flag position) | This phase | `--search help` runs the read |
| `Validate` ignores projects | Requires non-empty, non-blank projects | This phase | Misconfiguration exits 3 for CLI and TUI |
| Envelope `{items, failures, complete}` | `{items, failures, complete, scope}` | This phase | One-way contract addition; agents can ignore it |
| Every panic → exit 3 | A panic after mutate → exit 5 plus best-effort `unknown` audit | This phase | Aligns panics with Phase 4 D-22 interrupts |

**Deprecated/outdated:** the "90 days" comments in `client.go`, `workitems.go` and `dashboard.go` (code is 30 days); the "outcome-unknown exit code" wording for 3 (D-14).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `pr_status` (not D-01's example `status`) is acceptable as the key for the Active-only PR filter, and `dashboard counts` should carry it too | Pattern 3 | One-way contract; a rename later breaks agents. Confirm at plan review. |
| A2 | `"iterations": "recent"` (with 30 days defined in help) is precise enough for agents | Pattern 3 | An agent cannot learn the window from JSON alone. The alternative `"iterations_within_days": 30` is also one-way. |
| A3 | Azure DevOps `Get Pull Requests By Project` applies a server default page size when `$top` is unset. The docs list `$top` with no default (CITED below); community sources say about 100 | OQ2 | If true, `prs list` can be silently truncated, which is a completeness issue separate from scope. Not a Phase 6 deliverable (D-03). |
| A4 | Treating "mutate entered" as "request issued" is acceptable: a panic inside `op.mutate` before the HTTP call leaves is reported 5, not 3 | Pattern 4 | Over-reports unknown, the safe direction per Phase 4 D-22 reasoning |

## Open Questions

1. **Does `releases approvals` get a non-null scope?**
   - What we know: it applies a server-side `StatusFilter: Pending` (`releases.go:257`). D-02's rule ("null when a read has no built-in server-side filter") therefore makes it non-null. ROADMAP SC3 names only `workitems list` and `prs list`. CONTEXT names `dashboard counts` as well.
   - What's unclear: whether the user meant the rule or the list.
   - Recommendation: follow D-02 literally with `{"approval_status":"pending"}`. That is one line plus one test. Confirm at plan review, because it is one-way.
2. **Latest-per-definition and config name filters.**
   - What we know: `pipelines runs` and `releases list` request `Top: 1` per definition (`pipelines.go` GetBuilds args; `releases.go` GetReleases args). `pipelines runs`/`definitions` return empty `complete: true` when no `pipelines:` are configured (`pipelines.go:123-126, 288-291`). That is client-side, config-driven filtering, the same shape of lie as W3.
   - Recommendation: keep `null` for these four (D-02 says *server-side* filter, and "one row per definition" is the payload's definition). Record the empty-`pipelines:` case as a follow-up in BL-02, not in this phase. Also note A3 (PR paging) there.
3. **Should `runRead` reject stray positionals (Pitfall 4)?**
   - Recommendation: the minimal fix is 3 lines (`if len(fs.Args()) > 0 { ...; return exitUnknown }`) and closes "`--json` silently ignored". It is outside the four warnings, though, so add it only with user approval. Otherwise record it in BL-02.
4. **D-13 "crash record still goes to stderr".**
   - Recommendation: read this as "the crash record is still emitted (on stdout, per Phase 3 D-23/D-26) and the audit-failure diagnostic goes to stderr". Do not move the record.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build/test | ✓ | go1.27.1 darwin/arm64 (go.mod `go 1.25.4`, CI `go-version: '1.25.4'`) | — |
| gofmt / go vet | CI gates (`.github/workflows/ci.yml:21-28`) | ✓ | bundled | — |
| Azure DevOps org | optional live smoke only | not probed (read-only live checks are user-run) | — | Unit tests through fakes cover every criterion |

**Missing dependencies with no fallback:** none.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib), no third-party assertion library |
| Config file | none; CI at `.github/workflows/ci.yml` |
| Quick run command | `go test -count=1 ./cmd/adoBizInt/ ./internal/config/ ./internal/ado/` (baseline about 1s this session, all `ok`) |
| Full suite command | `go build ./... && go vet ./... && go test -race -count=1 ./... && test -z "$(gofmt -l .)"` |

### Phase Requirements → Test Map
Test names are proposals. ❌ means the test is created in this phase.

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CLI-05, CLI-06, OUT-04 (SC1/W2) | Help token as a flag value is the value; read/write runs | unit | `go test ./cmd/adoBizInt -run 'TestHelpTokenAsFlagValueIsNotHelp|TestReleaseCommentReachesTheMutation' -count=1` | ❌ new / ✅ extend |
| CLI-05 (SC1/W2, D-09) | Help in flag position still works with no config, including `--project P --help` and `set-state 4821 --help` | unit | `go test ./cmd/adoBizInt -run 'TestHelpTokenInFlagPositionIsHelp|TestHelpNeedsNoCredentials|TestHelpIsDeterministic' -count=1` | ❌ / ✅ |
| CLI-06 (D-08) | WR-05: `--comment -h` reaches the mutation | unit | `go test ./cmd/adoBizInt -run TestReleaseCommentReachesTheMutation -count=1` | ✅ extend |
| TRUST-03, OUT-04 (SC2/W3) | Empty, nil, blank and whitespace projects → `ConfigError{Field:"projects"}` | unit | `go test ./internal/config -run 'TestValidate|TestLoadRejectsBlankProjects' -count=1` | ✅ extend / ❌ |
| TRUST-03 (SC2/W3, TUI) | `Validate` precedes both `dispatch` and `ui.NewModel` in `run()` | structural (go/ast on main.go) | `go test ./cmd/adoBizInt -run TestValidateGuardsCLIAndTUI -count=1` | ❌ |
| OUT-04, CLI-06 (SC3/W4) | `scope` key on every `Result[T]`; null when unscoped | unit (contract) | `go test ./internal/ado -run 'TestJSONFieldNames|TestNewResultScopeIsNull' -count=1` | ✅ update / ❌ |
| TRUST-03 (SC3/W4) | Exact scope per read; scope ↔ WIQL / SearchCriteria tie; `complete` unchanged; early-return paths | unit | `go test ./internal/ado -run 'Scope' -count=1` | ❌ |
| CLI-06 (SC3/W4, D-04/05/06) | Human stderr scope line (also on zero rows), never under `--json`; verb help states scope; `--hide-done` text | unit | `go test ./cmd/adoBizInt -run 'Scope|TestHideDoneHelp' -count=1` | ❌ |
| OUT-04, OUT-05, SAFE-02, SAFE-04 (SC4/W1) | Panic in mutate → exit 5, crash record on stdout, one `unknown` audit line; failing sink → still 5; pre-mutate panic → 3, no audit; no double record | unit | `go test ./cmd/adoBizInt -run 'TestPanic(After|Before)Write' -count=1` | ❌ |
| SAFE-02 (SC4/W1) | Recover defer positioned after `defer sink.Close()` and immediately before `op.mutate` | structural (go/ast on write.go) | `go test ./cmd/adoBizInt -run TestWriteRecoverPrecedesMutate -count=1` | ❌ |
| OUT-05 (regression) | Read panic still exits 3 | unit | `go test ./cmd/adoBizInt -run 'TestPanicExitsThree|TestPATNeverLogged' -count=1` | ✅ |

**Manual-only (optional):** live `adoBizInt workitems list --search help --json` (expect `scope` and the items) and `ADO_PROJECTS= ` with no projects in config launching the TUI (expect "Invalid config: projects: ..." and exit 3). Both are covered by automated tests. The live runs only confirm.

### Sampling Rate
- **Per task commit:** the quick run command.
- **Per wave merge:** the full suite command.
- **Phase gate:** full suite green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] W2 test functions in `cmd/adoBizInt/usage_test.go` (new), plus the `-h` row in `writeverbs_test.go`
- [ ] W3 rows in `internal/config/config_test.go` (`base()` gains `Projects`), plus the `main_test.go:735` fixture fix
- [ ] W4 contract updates in `internal/ado/json_test.go`; scope tests next to `wiql_test.go` / `pullrequests_test.go`; CLI scope-line and help tests
- [ ] W1 tests in `cmd/adoBizInt/crash_test.go` or `write_test.go`; a structural test for the defer position (no AST helper exists in `cmd/`, so use `parser.ParseFile` on `write.go` directly, single-use and inline)
- Framework install: none.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | PAT handling unchanged |
| V3 Session Management | no | — |
| V4 Access Control | yes (indirect) | `--project` allow-list unchanged. The help probe applies no values, so it cannot bypass the list: the real parse in `dispatch` still enforces it |
| V5 Input Validation | yes | stdlib `flag` parsing; `config.Validate` rejects empty or blank projects |
| V6 Cryptography | no | — |
| V7 Error Handling and Logging | yes | Crash record stays three keys with no stack (`crash.go:28-32`); the stack goes to the log only when `loggingReady`; the audit record keeps its five keys and no argv (`audit.go:31-53`) |

### Known Threat Patterns for this change

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Help-token injection in a value makes a read look empty | Spoofing / Repudiation | stdlib position-aware parse (W2) |
| Misconfiguration reported as a complete empty answer | Spoofing | `Validate` fails closed (W3) |
| Silent server-side filter read as "does not exist" | Information integrity | `scope` disclosure (W4); constants only, no server text |
| A write that may have landed leaves no trace | Repudiation (SAFE-02) | Best-effort `unknown` audit plus exit 5 (W1) |
| Panic message disclosing secrets in the audit file | Information disclosure | The audit record carries no panic message, only the five keys. The message stays in the crash record, as today |

## Verification Evidence (run this session)

Scratch program (outside the repo) run with the local toolchain. The wrapped column is the Pattern 1 probe. Output is abridged: whitespace is collapsed and the repeated `err=flag: help requested` is trimmed on some rows.
```
["--search" "help"]            real: help=false err=<nil> | probe: help=false err=<nil>
["--search" "-h"]              real: help=false err=<nil> | probe: help=false err=<nil>
["--comment" "-h"]             real: help=false err=<nil> | probe: help=false err=<nil>
["--json" "--help"]            real: help=true  err=flag: help requested | probe: help=true
["--project" "P" "--help"]     real: help=false err=invalid value "P" for flag -project: not configured "P" | probe: help=true
["foo" "--help"]               real: help=false err=<nil> | probe: help=false err=<nil>
["--help=x"]                   real: help=true | probe: help=true
```
`PrintDefaults` on a FlagSet with wrapped Values printed `-timeout value` (type name lost) followed by `panic calling String method on zero main.probe for flag json: runtime error: invalid memory address or nil pointer dereference`. This confirms Pitfall 3.

Baseline: `go test -count=1 ./...` passed in every package (cmd/adoBizInt 0.26s, internal/ado 0.42s).

## Sources

### Primary (HIGH confidence)
- Repo source read this session: `cmd/adoBizInt/{usage,flags,main,crash,exit,dispatch,write,audit,render,filter}.go`; `internal/ado/{result,workitems,dashboard,pullrequests,pipelines,releases,client,filters}.go`; `internal/config/config.go`; tests `usage_test.go`, `crash_test.go`, `audit_test.go`, `write_test.go` (helpers), `writeverbs_test.go`, `patleak_test.go`, `main_test.go` (helpers, timeout test), `json_test.go`, `wiql_test.go`, `config_test.go`
- Phase contracts: `03-CONTEXT.md` D-11, D-23, D-25, D-26; `04-CONTEXT.md` D-22; `v1.0-MILESTONE-AUDIT.md` W1–W4; `BACKLOG.md` BL-02 WR-05
- SDK enum values in module cache: `azuredevops@v1.0.0-b5/git/models.go:2591-2602`, `release/models.go:190-205`
- Empirical stdlib `flag` behaviour: scratch run above

### Secondary (MEDIUM confidence)
- [Pull Requests - Get Pull Requests By Project (REST 7.1)](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests/get-pull-requests-by-project?view=azure-devops-rest-7.1): `searchCriteria.status` "Defaults to Active if unset"; `$top` documented with no stated default

### Tertiary (LOW confidence)
- Web search snippets claiming a default PR page size of about 100 (A3). Unverified, used only in OQ2.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH. Stdlib only, behaviour verified by running it.
- Architecture: HIGH. Every insertion point was read with line numbers. Placement is constrained by locked Phase 3 decisions.
- Pitfalls: HIGH. Three were reproduced empirically (allow-list, PrintDefaults, positional stop). The fixture breakages were found by reading the tests.

**Research date:** 2026-10-12
**Valid until:** 2026-11-11 (stable code; re-check if `write.go`, `usage.go` or `result.go` change before planning)
