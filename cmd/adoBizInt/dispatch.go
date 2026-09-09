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

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/logging"
)

// Process exit codes (D-06). These are a public contract from this plan onward:
// agents and CI jobs branch on them, so the integers are a one-way door.
const (
	exitOK      = 0 // the read completed and nothing failed
	exitPartial = 1 // the read ran but did not reach every configured project
	exitAuth    = 2 // at least one failure was an authentication rejection
	exitUnknown = 3 // the read never ran: usage error, config error, client error
)

// stdoutWriter and stderrWriter are the dispatch layer's output sinks. The
// production defaults are the real streams; tests swap in buffers so stdout
// purity can be asserted without spawning a process.
var (
	stdoutWriter io.Writer = os.Stdout
	stderrWriter io.Writer = os.Stderr
)

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
}

// commandFunc runs one `noun verb` command over the remaining arguments and
// returns the process exit code. Each dispatch entry is its own closure, so no
// uniform read signature is required — GetBranches' extra repositories
// parameter is absorbed by its own entry rather than by an adapter (D-P2).
type commandFunc func(ctx context.Context, client readClient, args []string) int

// commands is keyed on the two-token `noun verb` string (D-01, D-02). The key
// space is a fixed set of literals: an unmatched key returns exitUnknown and
// never reaches a client call (threat T-03-02).
//
// Each verb maps 1:1 to exactly one read method. There is deliberately no flag
// that swaps a command's payload type — a boolean that changes the output
// schema is hostile to an agent parsing it (D-02).
var commands = map[string]commandFunc{
	"workitems list": func(ctx context.Context, c readClient, args []string) int {
		return runRead(ctx, "workitems list", args, c.GetWorkItems, renderWorkItems)
	},
	"prs list": func(ctx context.Context, c readClient, args []string) int {
		return runRead(ctx, "prs list", args, c.GetPullRequests, renderPullRequests)
	},
	"pipelines runs": func(ctx context.Context, c readClient, args []string) int {
		return runRead(ctx, "pipelines runs", args, c.GetPipelineRuns, renderPipelineRuns)
	},
	"pipelines definitions": func(ctx context.Context, c readClient, args []string) int {
		return runRead(ctx, "pipelines definitions", args, c.GetPipelineDefinitions, renderPipelineDefinitions)
	},
	"releases list": func(ctx context.Context, c readClient, args []string) int {
		return runRead(ctx, "releases list", args, c.GetReleases, renderReleases)
	},
	"releases approvals": func(ctx context.Context, c readClient, args []string) int {
		return runRead(ctx, "releases approvals", args, c.GetPendingApprovals, renderReleaseApprovals)
	},
	"branches list": func(ctx context.Context, c readClient, args []string) int {
		// The extra repositories parameter is absorbed here rather than by a
		// uniform method type (D-P2). Passing nil is the unfiltered set: a read
		// verb applies no filter the caller did not request (D-18).
		fetch := func(ctx context.Context, projects []string) (ado.Result[ado.Branch], error) {
			return c.GetBranches(ctx, projects, nil)
		}
		return runRead(ctx, "branches list", args, fetch, renderBranches)
	},
	"dashboard counts": func(ctx context.Context, c readClient, args []string) int {
		return runRead(ctx, "dashboard counts", args, c.GetDashboardCounts, renderDashboardCounts)
	},
}

// dispatch routes os.Args[1:] to a command. Help, unknown nouns, unknown verbs
// and bare nouns are already resolved by handleUsage before config is loaded,
// so a key reaching here is expected to resolve; the fallback stays because a
// typo must never fall through to the TUI (D-09, D-22).
func dispatch(ctx context.Context, client readClient, args []string) int {
	if len(args) >= 2 {
		if cmd, ok := commands[args[0]+" "+args[1]]; ok {
			return cmd(ctx, client, args[2:])
		}
	}
	usageRoot()
	return exitUnknown
}

// runRead is the single body every read verb shares: parse, fetch, emit, map to
// an exit code. It is generic over the payload type, so the eight commands
// differ only in their fetch function and their renderer.
func runRead[T any](
	ctx context.Context,
	name string,
	args []string,
	fetch func(context.Context, []string) (ado.Result[T], error),
	render func(io.Writer, []T),
) int {
	fs, gf := newFlagSet(name)
	if err := fs.Parse(args); err != nil {
		// flag.ErrHelp means -h/--help reached the FlagSet: usage has already
		// been written and the request succeeded rather than failed.
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUnknown
	}

	log := logging.Logger()

	// --project is parsed and carried but not yet applied: plan 03-04 Task 2
	// owns passing it as the projects argument and settling what an unset flag
	// means. Until then no filter the caller did not request is applied, so the
	// read returns the unfiltered set even where the TUI defaults to filtering
	// (D-18).
	_ = gf.projects
	result, err := fetch(ctx, nil)
	if err != nil {
		log.Error("Read failed", "command", name, "error", err)
		fmt.Fprintf(stderrWriter, "Error running %s: %v\n", name, err)
		return exitUnknown
	}
	log.Info("Read complete", "command", name, "items", len(result.Items), "failures", len(result.Failures))

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
		render(stdoutWriter, result.Items)
		renderFailures(stderrWriter, result.Failures)
	}

	return exitCodeFor(result.Failures, result.Complete)
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
