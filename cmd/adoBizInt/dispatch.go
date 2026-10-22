package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/fetchctx"
	writeguard "sentinovo.ai/bizInt/internal/guard"
	"sentinovo.ai/bizInt/internal/logging"
)

// Process exit codes (D-06). These are a public contract from this plan onward:
// agents and CI jobs branch on them, so the integers are a one-way door.
const (
	exitOK      = 0 // the read completed and nothing failed
	exitPartial = 1 // the read ran but did not reach every configured project
	exitAuth    = 2 // at least one failure was an authentication rejection
	exitUnknown = 3 // the read never ran: usage error, config error, client error

	exitNoOp           = 4 // the pre-read showed the change was already applied: nothing was written
	exitOutcomeUnknown = 5 // the write was issued and its outcome could not be established
)

// stdoutWriter and stderrWriter are the dispatch layer's output sinks. The
// production defaults are the real streams; tests swap in buffers so stdout
// purity can be asserted without spawning a process.
var (
	stdoutWriter io.Writer = os.Stdout
	stderrWriter io.Writer = os.Stderr
)

// readBudget is the fan-out deadline every CLI read runs under when --timeout
// is not given. run() assigns it from the loaded configuration AFTER validation
// returns, which is the whole point of D-P6: the validator rejects a fetch
// budget at or below the poll interval, a rule that is correct for a refreshing
// TUI and meaningless for a one-shot read. Applying the override before
// validation would fail `--timeout 10s` with a message about a concept the CLI
// does not have.
//
// The zero value means "unset", and fetchctx turns an unset budget into its own
// default rather than into an unbounded read.
var readBudget time.Duration

// readClient is the consumer-side seam the dispatch table calls through
// (D-P1). It is declared here rather than in internal/ado because the consumer
// owns the interface in Go, and because the Phase 1 fake seam uses unexported
// Client fields and therefore cannot be constructed from this package.
// *ado.Client satisfies this structurally with no change to internal/ado.
//
// It declares exactly the eight aggregate reads of D-02 — the ones returning
// ado.Result[T]. GetRecentIterations, GetBranchesForPipeline, GetWorkItemDetail
// and GetChildWorkItems are deliberately excluded: they return bare types with
// no failure list and no completeness flag, so they cannot honour the envelope
// contract the exit-code mapper depends on.
type readClient interface {
	GetWorkItems(ctx context.Context, projects []string) (ado.Result[ado.WorkItem], error)
	GetPullRequests(ctx context.Context, projects []string) (ado.Result[ado.PullRequest], error)
	GetPipelineRuns(ctx context.Context, projects []string) (ado.Result[ado.PipelineRun], error)
	GetPipelineDefinitions(ctx context.Context, projects []string) (ado.Result[ado.PipelineDefinition], error)
	GetReleases(ctx context.Context, projects []string) (ado.Result[ado.Release], error)
	GetPendingApprovals(ctx context.Context, projects []string) (ado.Result[ado.ReleaseApproval], error)
	GetBranches(ctx context.Context, projects, repositories []string) (ado.Result[ado.Branch], error)
	GetDashboardCounts(ctx context.Context, projects []string) (ado.Result[ado.DashboardCounts], error)
	GetTeams(ctx context.Context, projects []string) (ado.Result[ado.Team], error)

	// ResolveTeams is the scrum-verb precondition (TEAM-01), not an envelope
	// read: it validates configured team names against ADO and returns IDs.
	ResolveTeams(ctx context.Context, projects []string, selected map[string][]string) ([]ado.Team, error)

	// The scrum reads take the teams selectTeams resolved, not projects.
	GetTeamMembers(ctx context.Context, teams []ado.Team) (ado.Result[ado.TeamMember], error)
	GetCurrentSprints(ctx context.Context, teams []ado.Team) (ado.Result[ado.Sprint], error)
	GetProcessFields(ctx context.Context, teams []ado.Team) (ado.Result[ado.ProcessFields], error)

	// CurrentUserEmail is the cached identity --assigned-to-me filters on. It
	// is a read of an already-resolved field, not a network call: Phase 1 D-12
	// makes identity resolution non-fatal, so it can legitimately return "".
	CurrentUserEmail() string
}

// client is what the dispatch table is handed. It embeds the read seam and the
// guard's write seam (writeguard.Client) so one value serves read and write verbs
// without either interface widening to accommodate the other. *ado.Client
// satisfies it structurally.
type client interface {
	readClient
	writeguard.Client
}

// commandFunc runs one `noun verb` command over the remaining arguments and
// returns the process exit code. Each dispatch entry is its own closure, so no
// uniform read signature is required — GetBranches' extra repositories
// parameter is absorbed by its own entry rather than by an adapter (D-P2).
type commandFunc func(ctx context.Context, c client, args []string) int

// commands is keyed on the two-token `noun verb` string (D-01, D-02). The key
// space is a fixed set of literals: an unmatched key returns exitUnknown and
// never reaches a client call (threat T-03-02).
//
// Each verb maps 1:1 to exactly one read method. There is deliberately no flag
// that swaps a command's payload type — a boolean that changes the output
// schema is hostile to an agent parsing it (D-02).
// ignoreRepos adapts a read method that takes only projects to the uniform
// fetch signature runRead uses. Seven of the eight reads have no repositories
// parameter; only GetBranches does, and it is the one verb where --repo narrows
// the read rather than post-filtering it.
func ignoreRepos[T any](
	f func(context.Context, []string) (ado.Result[T], error),
) func(context.Context, []string, []string) (ado.Result[T], error) {
	return func(ctx context.Context, projects, _ []string) (ado.Result[T], error) {
		return f(ctx, projects)
	}
}

var commands = map[string]commandFunc{
	"workitems list": func(ctx context.Context, c client, args []string) int {
		return runRead(ctx, "workitems list", args, c.CurrentUserEmail(),
			ignoreRepos(c.GetWorkItems), filterWorkItems, renderWorkItems)
	},
	"prs list": func(ctx context.Context, c client, args []string) int {
		return runRead(ctx, "prs list", args, "",
			ignoreRepos(c.GetPullRequests), filterPullRequests, renderPullRequests)
	},
	"pipelines runs": func(ctx context.Context, c client, args []string) int {
		return runRead(ctx, "pipelines runs", args, "",
			ignoreRepos(c.GetPipelineRuns), filterPipelineRuns, renderPipelineRuns)
	},
	"pipelines definitions": func(ctx context.Context, c client, args []string) int {
		return runRead[ado.PipelineDefinition](ctx, "pipelines definitions", args, "",
			ignoreRepos(c.GetPipelineDefinitions), nil, renderPipelineDefinitions)
	},
	"releases list": func(ctx context.Context, c client, args []string) int {
		return runRead(ctx, "releases list", args, "",
			ignoreRepos(c.GetReleases), filterReleases, renderReleases)
	},
	"releases approvals": func(ctx context.Context, c client, args []string) int {
		return runRead[ado.ReleaseApproval](ctx, "releases approvals", args, "",
			ignoreRepos(c.GetPendingApprovals), nil, renderReleaseApprovals)
	},
	"branches list": func(ctx context.Context, c client, args []string) int {
		// GetBranches is the only read with a repositories parameter, so --repo
		// narrows the read here rather than post-filtering the result
		// (branches.go:92 does the filtering inside). filterBranches therefore
		// applies search only, never repository, or the rule would run twice.
		return runRead(ctx, "branches list", args, "",
			c.GetBranches, filterBranches, renderBranches)
	},
	"dashboard counts": func(ctx context.Context, c client, args []string) int {
		return runRead[ado.DashboardCounts](ctx, "dashboard counts", args, "",
			ignoreRepos(c.GetDashboardCounts), nil, renderDashboardCounts)
	},
	"teams list": func(ctx context.Context, c client, args []string) int {
		return runRead[ado.Team](ctx, "teams list", args, "",
			ignoreRepos(c.GetTeams), nil, renderTeams)
	},
	"teams members": func(ctx context.Context, c client, args []string) int {
		return runScrum(ctx, "teams members", args, c, c.GetTeamMembers, renderTeamMembers)
	},
	"teams process": func(ctx context.Context, c client, args []string) int {
		return runScrum(ctx, "teams process", args, c, c.GetProcessFields, renderProcessFields)
	},
	"sprint current": func(ctx context.Context, c client, args []string) int {
		return runScrum(ctx, "sprint current", args, c, c.GetCurrentSprints, renderSprints)
	},
	"workitems set-state": func(ctx context.Context, c client, args []string) int {
		return runWrite(ctx, setStateOp(c), args)
	},
	"prs approve": func(ctx context.Context, c client, args []string) int {
		return runWrite(ctx, prsApproveOp(c), args)
	},
	"prs complete": func(ctx context.Context, c client, args []string) int {
		return runWrite(ctx, prsCompleteOp(c), args)
	},
	"releases approve": func(ctx context.Context, c client, args []string) int {
		return runWrite(ctx, releasesApproveOp(c), args)
	},
	"releases reject": func(ctx context.Context, c client, args []string) int {
		return runWrite(ctx, releasesRejectOp(c), args)
	},
	"pipelines queue": func(ctx context.Context, c client, args []string) int {
		return runWrite(ctx, pipelinesQueueOp(c), args)
	},
}

// dispatch routes os.Args[1:] to a command. Help, unknown nouns, unknown verbs
// and bare nouns are already resolved by handleUsage before config is loaded,
// so a key reaching here is expected to resolve; the fallback stays because a
// typo must never fall through to the TUI (D-09, D-22).
func dispatch(ctx context.Context, c client, args []string) int {
	if len(args) >= 2 {
		if cmd, ok := commands[args[0]+" "+args[1]]; ok {
			return cmd(ctx, c, args[2:])
		}
	}
	usageRoot()
	return exitUnknown
}

// runRead is the single body every read verb shares: parse, fetch, filter,
// emit, map to an exit code. It is generic over the payload type, so the eight
// commands differ only in their fetch function, their post-filter and their
// renderer.
//
// postFilter may be nil for a payload with no filterable fields.
func runRead[T any](
	ctx context.Context,
	name string,
	args []string,
	identity string,
	fetch func(context.Context, []string, []string) (ado.Result[T], error),
	postFilter func([]T, *filterFlags, io.Writer) []T,
	render func(io.Writer, []T),
) int {
	gf, ff, code, ok := parseRead(name, args)
	if !ok {
		return code
	}
	ff.identity = identity

	// --project narrows the read: its values become the projects argument. An
	// omitted flag passes an empty slice rather than the configured project
	// list, because every read method already falls back to its own configured
	// list on an empty slice (branches.go:56-59). Restating that default here
	// would put it in two places and let the two drift.
	//
	// --repo is passed as GetBranches' repositories argument for the one verb
	// that has one; an omitted flag is likewise an empty slice, never
	// the configured repository list. Handing a caller who asked for no filter a
	// config-filtered subset is the over-filtering D-18 forbids.
	projects := emptyIfNil(gf.projects)
	repositories := emptyIfNil(ff.repos)

	// --timeout overrides the configured fetch budget for this invocation only
	// (CLI-04, D-P5). It maps to the fan-out budget rather than the per-request
	// one, because the per-request value is baked into the HTTP client at
	// construction, before any flag value could reach it.
	//
	// The context is derived from ctx, the caller's cancellable parent, so a
	// signal aborts the read in flight. The budget goes through fetchctx so the
	// zero-budget fallback stays defined in exactly one place: an explicit
	// --timeout 0s means "use the default budget", never "no deadline" (D-P7).
	budget := readBudget
	if gf.timeout != 0 {
		budget = gf.timeout
	}
	readCtx, cancel := fetchctx.NewWithParent(ctx, budget)
	defer cancel()

	result, err := fetch(readCtx, projects, repositories)

	return finishRead(ctx, name, gf, ff, result, err, postFilter, render)
}

// parseRead parses a read verb's flags. ok is false when the caller must
// return code without reading: help was shown (0) or the arguments were bad (3).
func parseRead(name string, args []string) (gf *globalFlags, ff *filterFlags, code int, ok bool) {
	fs, gf, ff, _ := newFlagSet(name)
	if err := fs.Parse(args); err != nil {
		// flag.ErrHelp means -h/--help reached the FlagSet: usage has already
		// been written and the request succeeded rather than failed.
		if errors.Is(err, flag.ErrHelp) {
			return nil, nil, exitOK, false
		}
		return nil, nil, exitUnknown, false
	}
	// D-17: stdlib stops parsing at the first token it cannot read as a flag,
	// so a stray positional would silently drop itself and every flag after
	// it. Nothing ran, so the exit is 3. The token is quoted with %q, so a
	// control character cannot reach the terminal raw; naming the first one is
	// enough to locate the mistake.
	if len(fs.Args()) > 0 {
		fmt.Fprintf(stderrWriter, "%s takes no positional arguments; got %q; run 'adoBizInt %s --help' for its flags\n",
			name, fs.Args()[0], name)
		return nil, nil, exitUnknown, false
	}
	return gf, ff, exitOK, true
}

// finishRead is the shared tail of every read verb, v1 and scrum alike:
// interrupt check, error, post-filter, output and exit-code mapping.
func finishRead[T any](
	ctx context.Context,
	name string,
	gf *globalFlags,
	ff *filterFlags,
	result ado.Result[T],
	err error,
	postFilter func([]T, *filterFlags, io.Writer) []T,
	render func(io.Writer, []T),
) int {
	log := logging.Logger()

	// The interrupt check keys on the parent's Err(), not on the failure kind.
	// The failure classifier maps a cancellation to the generic kind on purpose,
	// so a user-initiated abort is never reported as a server timeout — which
	// means the kind alone cannot tell you a signal fired, only the parent can.
	//
	// It runs before any stdout write, so an interrupt that lands between the
	// read returning and the output being written still yields exactly one
	// top-level document: the crash record instead of the envelope, never both.
	if ctx.Err() != nil {
		log.Warn("Read interrupted", "command", name, "error", ctx.Err())
		writeCrashRecord(crashKindInterrupt,
			"read interrupted before any output was written: "+ctx.Err().Error())
		return exitUnknown
	}

	if err != nil {
		log.Error("Read failed", "command", name, "error", err)
		fmt.Fprintf(stderrWriter, "Error running %s: %v\n", name, err)
		return exitUnknown
	}
	log.Info("Read complete", "command", name, "items", len(result.Items), "failures", len(result.Failures))

	// Filtering runs after the read, on the returned Items only. The failure
	// list and the Complete flag continue to describe the read itself, which is
	// what the exit-code mapping keys on: a search that matched nothing is a
	// complete read of zero interesting rows, not a partial one.
	if postFilter != nil {
		result.Items = postFilter(result.Items, ff, stderrWriter)
	}

	if gf.asJSON {
		if err := emitJSON(result); err != nil {
			log.Error("Failed to marshal result", "error", err)
			fmt.Fprintf(stderrWriter, "Error encoding result: %v\n", err)
			return exitUnknown
		}
	} else {
		// Human mode: rows to stdout, failures to stderr (D-13). Failures are
		// never suppressed here — a human running the default is exactly the
		// caller who must not mistake a broken read for an empty one. Under
		// --json the failures already ride inside the stdout envelope.
		//
		// The scope line is the human counterpart of the envelope's scope
		// (D-04): it goes to stderr ahead of any warnings, and is written even
		// for zero rows.
		render(stdoutWriter, result.Items)
		renderScope(stderrWriter, result.Scope)
		renderFindings(stderrWriter, result.Findings)
		renderFailures(stderrWriter, result.Failures)
	}

	return exitCodeFor(result.Failures, result.Complete)
}

// emptyIfNil normalises an unset repeatable flag to a non-nil empty slice, so
// a read method receives "no filter requested" as an empty set rather than nil
// and the two are never distinguishable at the call site.
func emptyIfNil(v stringList) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// emitJSON serialises the raw envelope — no wrapper, no invocation metadata
// (D-11, D-14) — into a byte slice and writes it with a single Write. A
// streaming encoder is deliberately not used: it can leave a partial document
// on stdout ahead of a crash record (D-P4, D-23).
func emitJSON(v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	_, err = stdoutWriter.Write(payload)
	return err
}

// commandKeys returns every registered `noun verb` key in sorted order so any
// listing built from the table is deterministic across runs.
func commandKeys() []string {
	keys := make([]string, 0, len(commands))
	for k := range commands {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
