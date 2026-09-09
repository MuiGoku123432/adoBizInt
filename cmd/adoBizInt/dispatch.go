package main

import (
	"context"
	"encoding/json"
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
// Plan 03-02 widens this to all eight reads.
type readClient interface {
	GetPullRequests(ctx context.Context, projects []string) (ado.Result[ado.PullRequest], error)
}

// commandFunc runs one `noun verb` command over the remaining arguments and
// returns the process exit code. Each dispatch entry is its own closure, so no
// uniform read signature is required — GetBranches' extra repositories
// parameter is absorbed by its own entry rather than by an adapter (D-P2).
type commandFunc func(ctx context.Context, client readClient, args []string) int

// commands is keyed on the two-token `noun verb` string (D-01, D-02). The key
// space is a fixed set of literals: an unmatched key returns exitUnknown and
// never reaches a client call (threat T-03-02).
var commands = map[string]commandFunc{
	"prs list": cmdPRsList,
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

// printUsage lists the available commands on stderr. Usage text is a
// diagnostic, so it never touches stdout (OUT-03).
func printUsage() {
	keys := make([]string, 0, len(commands))
	for k := range commands {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Fprintf(stderrWriter, "Usage: adoBizInt [command] [flags]\n")
	fmt.Fprintf(stderrWriter, "\nRun with no arguments to start the interactive TUI.\n")
	fmt.Fprintf(stderrWriter, "\nAvailable commands:\n")
	for _, k := range keys {
		fmt.Fprintf(stderrWriter, "  %s\n", k)
	}
}

// cmdPRsList runs the `prs list` read.
func cmdPRsList(ctx context.Context, client readClient, args []string) int {
	fs := flag.NewFlagSet("prs list", flag.ContinueOnError)
	fs.SetOutput(stderrWriter)
	asJSON := fs.Bool("json", false, "emit the result envelope as one JSON document on stdout")
	if err := fs.Parse(args); err != nil {
		return exitUnknown
	}

	log := logging.Logger()

	// No filter flags are passed to the read: a read verb with no filter flags
	// returns the unfiltered set, even where the TUI defaults to filtering (D-18).
	result, err := client.GetPullRequests(ctx, nil)
	if err != nil {
		log.Error("Failed to read pull requests", "error", err)
		fmt.Fprintf(stderrWriter, "Error reading pull requests: %v\n", err)
		return exitUnknown
	}
	log.Info("Read pull requests", "items", len(result.Items), "failures", len(result.Failures))

	if *asJSON {
		if err := emitJSON(result); err != nil {
			log.Error("Failed to marshal result", "error", err)
			fmt.Fprintf(stderrWriter, "Error encoding result: %v\n", err)
			return exitUnknown
		}
	} else {
		// Minimal human-readable fallback. Plan 03-03 replaces this with the
		// real renderer; it exists here so the verb is usable without --json.
		emitPlain(result)
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

// emitPlain writes one line per pull request in a single Write, preserving the
// same single-write discipline as emitJSON.
func emitPlain(result ado.Result[ado.PullRequest]) {
	var b strings.Builder
	for _, pr := range result.Items {
		fmt.Fprintf(&b, "%d\t%s\t%s\n", pr.ID, pr.RepositoryName, pr.Title)
	}
	if b.Len() > 0 {
		_, _ = io.WriteString(stdoutWriter, b.String())
	}
}
