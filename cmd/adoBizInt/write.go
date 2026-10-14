package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strconv"

	// Aliased because package main already declares func guard, the
	// top-level panic boundary in main.go.
	writeguard "sentinovo.ai/bizInt/internal/guard"
)

// fieldChange and writeTarget are the guard's types under their CLI names, so
// the write envelope and every test literal keep compiling unchanged (GUARD-06).
type (
	fieldChange = writeguard.FieldChange
	writeTarget = writeguard.Target
)

// writeRecord is the write report. Executed and Outcome are the only fields
// that differ between a dry run and a performed write (D-11): a caller parses
// one document shape and reads a field, rather than branching on which of two
// documents it received.
//
// Changes is always present and always an array. A refused no-op carries `[]`,
// never null and never an absent key, so `.write.changes | length` evaluates
// without a presence check (P-09) — the same lesson Result[T] learned when
// "failures":null was the happy-path output.
type writeRecord struct {
	Command  string        `json:"command"`
	Target   writeTarget   `json:"target"`
	Changes  []fieldChange `json:"changes"`
	Executed bool          `json:"executed"`
	Outcome  string        `json:"outcome"`
}

// writeEnvelope is the top-level document for a write. The record is nested
// under a single `write` key rather than being expressed as an ado.Result[T]
// with one item (D-10). An agent distinguishes a write report from a result set
// by which top-level key is present, so a write can never be misread as a
// one-row read: `.items | length` would report a write as a row, and `complete`
// and `failures` mean something different for a mutation. This is the same
// reasoning that produced crashEnvelope's distinct `error` key.
//
// It inherits crashRecord's credential rule: no stack, no argv, no URL, no
// headers, no raw error values (threat T-04-02). The envelope carries only the
// command string, target identifiers, field diffs and the outcome.
type writeEnvelope struct {
	Write writeRecord `json:"write"`
}

// writeOp is the CLI half of a guarded write: the verb name and the per-verb
// flag rules that build the guard's Op. The guarded-write sequence itself lives
// once, in internal/guard Run, which the TUI calls too (GUARD-01). A guard that
// lives in one shared body cannot be forgotten on a surface; a guard copied per
// surface can.
//
// build enforces the per-verb arity rules and returns the Op. Returning an error
// means nothing was attempted, which is exit 3.
type writeOp struct {
	name  string
	build func(gf *globalFlags, wf *writeFlags, id int) (writeguard.Op, error)
}

// oneProject enforces D-24's arity of exactly one on --project.
//
// It is deliberately NOT emptyIfNil. That helper exists so an omitted --project
// falls back to the read method's configured project list; silently landing a
// mutation on a project the caller did not name is unacceptable (threat T-04-12),
// so every write verb comes through here instead.
func oneProject(name string, gf *globalFlags) (string, error) {
	if len(gf.projects) != 1 {
		return "", fmt.Errorf("%s requires exactly one --project value", name)
	}
	return gf.projects[0], nil
}

// oneRepo enforces the same arity on --repo for the verbs that address a
// repository. Transposing project and repo on an approve has no type system to
// catch it, so both are named flags with an arity of one rather than positionals.
func oneRepo(name string, wf *writeFlags) (string, error) {
	if len(wf.repo) != 1 {
		return "", fmt.Errorf("%s requires exactly one --repo value", name)
	}
	return wf.repo[0], nil
}

// runWrite is the CLI adapter over the guarded-write body, the sibling of
// runRead. It parses and checks arity, hands the Op to writeguard.Run, and maps
// the Report to stderr, one stdout document and an exit code. Run owns the order
// of the guarded steps (audit open, pre-read, no-op refusal, confirm gate,
// mutate under recover, audit append); none of it is repeated here.
func runWrite(ctx context.Context, op writeOp, args []string) (code int) {
	name := op.name
	fs, gf, _, wf := newFlagSet(name)
	if err := fs.Parse(hoistLeadingPositionals(args)); err != nil {
		// flag.ErrHelp means -h/--help reached the FlagSet: usage has already
		// been written and the request succeeded rather than failed.
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUnknown
	}

	// D-18: promote the effort flags the caller actually set into non-nil
	// pointers. This runs immediately after Parse and before the arity checks,
	// so nothing downstream can read a pointer that has not been resolved yet.
	wf.promoteEffort(fs)

	// Arity, all failing to exit 3: nothing was attempted, which is exactly
	// what exit 3 means.
	//
	// D-25: one target per invocation. Batching would force a partial-failure
	// exit rule this contract has no clean answer for and would multiply the
	// blast radius of a single mistyped --confirm, in the phase whose subject is
	// safety.
	if len(fs.Args()) != 1 {
		fmt.Fprintf(stderrWriter, "%s requires exactly one id as a positional argument\n", name)
		return exitUnknown
	}
	id, err := strconv.Atoi(fs.Args()[0])
	if err != nil {
		fmt.Fprintf(stderrWriter, "%s: %q is not a numeric id\n", name, fs.Args()[0])
		return exitUnknown
	}

	// Per-verb arity: exactly one --project for every verb (D-24), plus
	// whichever of --repo, --state and --branch this verb's payload needs.
	gop, err := op.build(gf, wf, id)
	if err != nil {
		fmt.Fprintf(stderrWriter, "%v\n", err)
		return exitUnknown
	}

	budget := readBudget
	if gf.timeout != 0 {
		budget = gf.timeout
	}

	report := writeguard.Run(ctx, gop, writeguard.Options{
		Budget:  budget,
		Confirm: func([]fieldChange) bool { return wf.confirm },
		Open:    auditOpen,
	})

	switch {
	case report.SinkErr != nil:
		fmt.Fprintf(stderrWriter, "Refusing to write: audit sink %s is not writable: %v\n", auditPath, report.SinkErr)
		return exitUnknown
	case report.PreReadErr != nil:
		// D-02: a failed pre-read is fatal and emits NOTHING to stdout.
		fmt.Fprintf(stderrWriter, "Error reading %s target %d before writing: %v\n", name, id, report.PreReadErr)
		return exitUnknown
	case report.Outcome == outcomeNoOp:
		// D-03/D-08: exit 4 rather than 0, because collapsing "I changed it"
		// and "it was already so" is the exact conflation this project exists to
		// prevent.
		return finishWrite(name, gop.Target, report.Changes, false, outcomeNoOp, exitNoOp, gf.asJSON)
	case report.Outcome == outcomeDryRun:
		// D-07: a completed dry run exits 0. It did precisely what it was asked
		// to do; the executed/outcome fields carry the distinction, and the
		// caller already knows whether it passed --confirm.
		return finishWrite(name, gop.Target, report.Changes, false, outcomeDryRun, exitOK, gf.asJSON)
	}

	// From here the request was issued and may be live in ADO.
	if report.AuditErr != nil {
		// The operator needs to know WHAT to go check, not merely that a write
		// to a file failed: the mutation may already be live in ADO.
		fmt.Fprintf(stderrWriter, "Write may have landed but was NOT recorded in %s: %v\n", auditPath, report.AuditErr)
	}

	// D-12: a panic after the request was issued exits 5, as an interrupt does
	// (Phase 4 D-22), never 3. The guard recovered a panic in the mutation and
	// recorded it; a panic while emitting the report below is caught by the
	// deferred recover, after the guard's audit append, so it writes no second
	// record.
	// note: the message is built twice here; two uses is below the rule of three.
	if report.Panic != nil {
		return handlePanic(fmt.Sprintf("panic after %s was issued; outcome unknown: %v", name, report.Panic), exitOutcomeUnknown)
	}
	defer func() {
		if r := recover(); r != nil {
			code = handlePanic(fmt.Sprintf("panic after %s was issued; outcome unknown: %v", name, r), exitOutcomeUnknown)
		}
	}()

	if report.MutateErr != nil {
		fmt.Fprintf(stderrWriter, "Error running %s on %d: %v\n", name, id, report.MutateErr)
	}

	// CR-01: finishWrite's `executed` argument is computed from the outcome, so
	// a write the guard downgraded to unknown can never report executed:true.
	return finishWrite(name, gop.Target, report.Changes, report.Outcome == outcomeExecuted, report.Outcome,
		writeExitCodeFor(report.Outcome, report.MutateErr), gf.asJSON)
}

// setStateOp is the work item verb.
func setStateOp(c writeguard.Client) writeOp {
	const name = "workitems set-state"
	return writeOp{
		name: name,
		build: func(gf *globalFlags, wf *writeFlags, id int) (writeguard.Op, error) {
			project, err := oneProject(name, gf)
			if err != nil {
				return writeguard.Op{}, err
			}
			// D-26: --state is required and is never inferred from
			// config.StateTransitions or GetNextState. A config-driven default
			// means the same command produces different mutations on two
			// machines, and it would also make the current -> proposed diff
			// meaningless, since "proposed" would not have come from the caller.
			if wf.state == "" {
				return writeguard.Op{}, fmt.Errorf("%s requires --state", name)
			}
			effort := writeguard.Effort{OriginalEstimate: wf.originalEstimate, Remaining: wf.remaining, Completed: wf.completed}
			return writeguard.SetState(c, project, id, wf.state, effort), nil
		},
	}
}

// prsApproveOp casts the acting identity's approving vote.
func prsApproveOp(c writeguard.Client) writeOp {
	const name = "prs approve"
	return writeOp{
		name: name,
		build: func(gf *globalFlags, wf *writeFlags, id int) (writeguard.Op, error) {
			project, repo, err := resolveProjectRepo(name, gf, wf)
			if err != nil {
				return writeguard.Op{}, err
			}
			return writeguard.ApprovePR(c, project, repo, id), nil
		},
	}
}

// prsCompleteOp completes (merges) a pull request.
func prsCompleteOp(c writeguard.Client) writeOp {
	const name = "prs complete"
	return writeOp{
		name: name,
		build: func(gf *globalFlags, wf *writeFlags, id int) (writeguard.Op, error) {
			project, repo, err := resolveProjectRepo(name, gf, wf)
			if err != nil {
				return writeguard.Op{}, err
			}
			return writeguard.CompletePR(c, project, repo, id), nil
		},
	}
}

// releasesApproveOp approves a release approval. --comment defaults to empty.
// note: releasesRejectOp is the same shape over writeguard.RejectRelease; two
// uses is below the rule of three, so it is not extracted.
func releasesApproveOp(c writeguard.Client) writeOp {
	const name = "releases approve"
	return writeOp{
		name: name,
		build: func(gf *globalFlags, wf *writeFlags, id int) (writeguard.Op, error) {
			project, err := oneProject(name, gf)
			if err != nil {
				return writeguard.Op{}, err
			}
			return writeguard.ApproveRelease(c, project, id, wf.comment), nil
		},
	}
}

// releasesRejectOp rejects a release approval. --comment defaults to empty.
func releasesRejectOp(c writeguard.Client) writeOp {
	const name = "releases reject"
	return writeOp{
		name: name,
		build: func(gf *globalFlags, wf *writeFlags, id int) (writeguard.Op, error) {
			project, err := oneProject(name, gf)
			if err != nil {
				return writeguard.Op{}, err
			}
			return writeguard.RejectRelease(c, project, id, wf.comment), nil
		},
	}
}

// pipelinesQueueOp triggers a pipeline run.
func pipelinesQueueOp(c writeguard.Client) writeOp {
	const name = "pipelines queue"
	return writeOp{
		name: name,
		build: func(gf *globalFlags, wf *writeFlags, id int) (writeguard.Op, error) {
			project, err := oneProject(name, gf)
			if err != nil {
				return writeguard.Op{}, err
			}
			// P-04: --branch is required, never defaulted from the definition's
			// own default branch. The branch IS the substance of the mutation,
			// and inferring it would be exactly the config-driven behaviour D-26
			// rejects: the same invocation would queue a different build on two
			// machines.
			if wf.branch == "" {
				return writeguard.Op{}, fmt.Errorf("%s requires --branch", name)
			}
			return writeguard.QueuePipeline(c, project, id, wf.branch), nil
		},
	}
}

// resolveProjectRepo is the shared arity rule for the two verbs addressing a
// pull request: exactly one --project and exactly one --repo.
func resolveProjectRepo(name string, gf *globalFlags, wf *writeFlags) (project, repo string, err error) {
	if project, err = oneProject(name, gf); err != nil {
		return "", "", err
	}
	if repo, err = oneRepo(name, wf); err != nil {
		return "", "", err
	}
	return project, repo, nil
}

// finishWrite emits exactly one document on stdout and returns the exit code.
// Both the dry-run and the executed paths come through here, which is what makes
// D-11's single envelope shape true by construction rather than by discipline.
func finishWrite(name string, target writeTarget, changes []fieldChange, executed bool, outcome string, code int, asJSON bool) int {
	record := writeRecord{
		Command:  name,
		Target:   target,
		Changes:  changes,
		Executed: executed,
		Outcome:  outcome,
	}

	if asJSON {
		// The same emitJSON the reads use: marshal to a slice, one Write. A
		// streaming encoder could leave a partial document on stdout ahead of a
		// crash record.
		if err := emitJSON(writeEnvelope{Write: record}); err != nil {
			fmt.Fprintf(stderrWriter, "Error encoding write report: %v\n", err)
			return exitUnknown
		}
		return code
	}

	renderWriteRecord(record)
	return code
}

// renderWriteRecord prints the human diff table. Rows go to stdout; every
// diagnostic in this file goes to stderr (D-13's read-path split, unchanged for
// writes).
func renderWriteRecord(r writeRecord) {
	for _, ch := range r.Changes {
		fmt.Fprintf(stdoutWriter, "%s: %s -> %s\n", ch.Field, ch.From, ch.To)
	}
	fmt.Fprintf(stdoutWriter, "%s %s #%d: %s\n", r.Command, r.Target.Project, r.Target.ID, r.Outcome)
}

// hoistLeadingPositionals moves the arguments that appear before the first
// flag to the end of the slice, so `set-state 4821 --project P` and
// `set-state --project P 4821` parse identically.
//
// This exists because stdlib flag stops parsing at the first non-flag argument,
// and D-23 puts the target in positional form specifically so a missing target
// is a parse error rather than an --id defaulting to zero that could plausibly
// be issued. Without this the natural spelling — target first, as every
// `git`-shaped CLI writes it — would silently leave --project and --state unset
// and every write would fail as a usage error.
//
// It moves rather than extracts, so the arity check still sees every stray
// positional in fs.Args() and a second target is still a parse error (D-25).
// Nothing is dropped and nothing is reinterpreted: a token is relocated only
// when it precedes the first flag, so a flag's own value can never be hoisted.
func hoistLeadingPositionals(args []string) []string {
	firstFlag := len(args)
	for i, a := range args {
		if len(a) > 1 && a[0] == '-' {
			firstFlag = i
			break
		}
	}
	if firstFlag == 0 || firstFlag == len(args) {
		return args
	}
	hoisted := make([]string, 0, len(args))
	hoisted = append(hoisted, args[firstFlag:]...)
	hoisted = append(hoisted, args[:firstFlag]...)
	return hoisted
}
