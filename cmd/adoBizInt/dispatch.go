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
	"strings"

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
		return runRead(ctx, "workitems list", args, c.GetWorkItems, plainRows[ado.WorkItem])
	},
	"prs list": func(ctx context.Context, c readClient, args []string) int {
		return runRead(ctx, "prs list", args, c.GetPullRequests, plainRows[ado.PullRequest])
	},
	"pipelines runs": func(ctx context.Context, c readClient, args []string) int {
		return runRead(ctx, "pipelines runs", args, c.GetPipelineRuns, plainRows[ado.PipelineRun])
	},
	"pipelines definitions": func(ctx context.Context, c readClient, args []string) int {
		return runRead(ctx, "pipelines definitions", args, c.GetPipelineDefinitions, plainRows[ado.PipelineDefinition])
	},
	"releases list": func(ctx context.Context, c readClient, args []string) int {
		return runRead(ctx, "releases list", args, c.GetReleases, plainRows[ado.Release])
	},
	"releases approvals": func(ctx context.Context, c readClient, args []string) int {
		return runRead(ctx, "releases approvals", args, c.GetPendingApprovals, plainRows[ado.ReleaseApproval])
	},
	"branches list": func(ctx context.Context, c readClient, args []string) int {
		// The extra repositories parameter is absorbed here rather than by a
		// uniform method type (D-P2). Passing nil is the unfiltered set: a read
		// verb applies no filter the caller did not request (D-18).
		fetch := func(ctx context.Context, projects []string) (ado.Result[ado.Branch], error) {
			return c.GetBranches(ctx, projects, nil)
		}
		return runRead(ctx, "branches list", args, fetch, plainRows[ado.Branch])
	},
	"dashboard counts": func(ctx context.Context, c readClient, args []string) int {
		return runRead(ctx, "dashboard counts", args, c.GetDashboardCounts, plainRows[ado.DashboardCounts])
	},
}

// dispatch routes os.Args[1:] to a command. An unrecognised first token, or a
// recognised noun with no verb, prints the command list and returns
// exitUnknown — it deliberately does not fall through to the TUI (D-09, D-22),
// because a typo must not start a full-screen program in a non-interactive
// context.
func dispatch(ctx context.Context, client readClient, args []string) int {
	if len(args) >= 2 {
		if cmd, ok := commands[args[0]+" "+args[1]]; ok {
			return cmd(ctx, client, args[2:])
		}
	}
	printUsage()
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
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderrWriter)
	asJSON := fs.Bool("json", false, "emit the result envelope as one JSON document on stdout")
	if err := fs.Parse(args); err != nil {
		// flag.ErrHelp means -h/--help reached the FlagSet: usage has already
		// been written and the request succeeded rather than failed.
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUnknown
	}

	log := logging.Logger()

	// No filter flags are passed to the read: a read verb with no filter flags
	// returns the unfiltered set, even where the TUI defaults to filtering (D-18).
	result, err := fetch(ctx, nil)
	if err != nil {
		log.Error("Read failed", "command", name, "error", err)
		fmt.Fprintf(stderrWriter, "Error running %s: %v\n", name, err)
		return exitUnknown
	}
	log.Info("Read complete", "command", name, "items", len(result.Items), "failures", len(result.Failures))

	if *asJSON {
		if err := emitJSON(result); err != nil {
			log.Error("Failed to marshal result", "error", err)
			fmt.Fprintf(stderrWriter, "Error encoding result: %v\n", err)
			return exitUnknown
		}
	} else {
		render(stdoutWriter, result.Items)
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

// plainRows is the provisional human-mode renderer carried forward from 03-01's
// prs-list fallback, generalised so the seven newly registered verbs are not
// broken without --json between this task and the real renderer. Task 3 of this
// plan replaces every use of it with a per-type renderer in render.go.
func plainRows[T any](w io.Writer, items []T) {
	var b strings.Builder
	for _, item := range items {
		fmt.Fprintf(&b, "%+v\n", item)
	}
	if b.Len() > 0 {
		_, _ = io.WriteString(w, b.String())
	}
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

// printUsage lists the available commands on stderr. Usage text is a
// diagnostic, so it never touches stdout (OUT-03).
func printUsage() {
	fmt.Fprintf(stderrWriter, "Usage: adoBizInt [command] [flags]\n")
	fmt.Fprintf(stderrWriter, "\nRun with no arguments to start the interactive TUI.\n")
	fmt.Fprintf(stderrWriter, "\nAvailable commands:\n")
	for _, k := range commandKeys() {
		fmt.Fprintf(stderrWriter, "  %s\n", k)
	}
}
