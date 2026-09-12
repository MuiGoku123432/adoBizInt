package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strconv"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/logging"
	"sentinovo.ai/bizInt/internal/ui/fetchctx"
)

// Work item field names as ADO spells them. They are the `field` values of the
// diff, so they are part of the published envelope and of every persisted audit
// line — a caller matching on "System.State" must keep matching.
const (
	fieldState            = "System.State"
	fieldOriginalEstimate = "Microsoft.VSTS.Scheduling.OriginalEstimate"
	fieldRemainingWork    = "Microsoft.VSTS.Scheduling.RemainingWork"
	fieldCompletedWork    = "Microsoft.VSTS.Scheduling.CompletedWork"
)

// fieldChange is one line of the current -> proposed diff D-01 requires. The
// `from` value is a claim about the moment of the pre-read, not a guarantee
// about the moment of the write: D-03 acknowledges the read-then-write race
// explicitly, and narrowing it is all a pre-read can do.
type fieldChange struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// writeTarget identifies what was (or would be) mutated. Repo is omitempty
// because most write verbs have no repository; a caller reads target.id and
// target.project unconditionally.
type writeTarget struct {
	Project string `json:"project"`
	Repo    string `json:"repo,omitempty"`
	ID      int    `json:"id"`
}

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

// runWrite is the guarded-write body, the sibling of runRead. The ORDER of its
// steps is the contract, not an implementation detail — in particular the audit
// sink is opened before anything is attempted (D-13), and the no-op refusal
// precedes the --confirm branch because D-03 refuses on both paths.
func runWrite(ctx context.Context, name string, c writeClient, args []string) int {
	fs, gf, _, wf := newFlagSet(name)
	if err := fs.Parse(args); err != nil {
		// flag.ErrHelp means -h/--help reached the FlagSet: usage has already
		// been written and the request succeeded rather than failed.
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUnknown
	}

	log := logging.Logger()

	// Arity, all three failing to exit 3: nothing was attempted, which is
	// exactly what exit 3 means.
	//
	// D-25: one target per invocation. Batching would force a partial-failure
	// exit rule this contract has no clean answer for and would multiply the
	// blast radius of a single mistyped --confirm, in the phase whose subject is
	// safety.
	if len(fs.Args()) != 1 {
		fmt.Fprintf(stderrWriter, "%s requires exactly one work item id as a positional argument\n", name)
		return exitUnknown
	}
	id, err := strconv.Atoi(fs.Args()[0])
	if err != nil {
		fmt.Fprintf(stderrWriter, "%s: %q is not a work item id\n", name, fs.Args()[0])
		return exitUnknown
	}

	// D-24: exactly one --project, and deliberately NOT emptyIfNil. That helper
	// exists so an omitted --project falls back to the read method's configured
	// project list; silently picking a project the caller did not name is
	// unacceptable for a mutation.
	if len(gf.projects) != 1 {
		fmt.Fprintf(stderrWriter, "%s requires exactly one --project value\n", name)
		return exitUnknown
	}
	project := gf.projects[0]

	// D-26: --state is required and is never inferred from
	// config.StateTransitions or GetNextState. A config-driven default means the
	// same command produces different mutations on two machines, and it would
	// also make the current -> proposed diff meaningless, since "proposed" would
	// not have come from the caller.
	if wf.state == "" {
		fmt.Fprintf(stderrWriter, "%s requires --state\n", name)
		return exitUnknown
	}

	target := writeTarget{Project: project, ID: id}

	// P-11: the pre-read and the write share one budget from a single parent,
	// matching runRead. The documented limit is that a slow pre-read can consume
	// the budget and push the write into outcome-unknown for a reason unrelated
	// to the write itself.
	budget := readBudget
	if gf.timeout != 0 {
		budget = gf.timeout
	}
	writeCtx, cancel := fetchctx.NewWithParent(ctx, budget)
	defer cancel()

	// D-13. This is the load-bearing line of the whole function and its POSITION
	// is the decision: the sink is proven writable before any request leaves, so
	// there is no window in which ADO changed and nothing recorded it. Moving it
	// below the mutate call reverses D-13 while appearing to honour it.
	sink, err := openAuditSink()
	if err != nil {
		log.Error("Audit sink unavailable", "command", name, "error", err)
		fmt.Fprintf(stderrWriter, "Refusing to write: audit sink %s is not writable: %v\n", auditPath, err)
		return exitUnknown
	}
	defer sink.Close()

	// D-02: a failed pre-read is fatal and emits NOTHING to stdout. A dry run
	// that says "would set State=Closed" without having established the current
	// state is a confident statement made without looking.
	detail, err := c.GetWorkItemDetail(writeCtx, id)
	if err != nil {
		log.Error("Write pre-read failed", "command", name, "id", id, "error", err)
		fmt.Fprintf(stderrWriter, "Error reading work item %d before writing: %v\n", id, err)
		return exitUnknown
	}
	if detail == nil {
		fmt.Fprintf(stderrWriter, "Work item %d was not found\n", id)
		return exitUnknown
	}

	changes := diffWorkItem(detail, wf)

	// D-03/D-08: a change already applied is refused with no mutation attempted,
	// on BOTH paths — this branch precedes the --confirm check deliberately.
	// Exit 4 rather than 0, because collapsing "I changed it" and "it was already
	// so" is the exact conflation this project exists to prevent.
	if len(changes) == 0 {
		return finishWrite(name, target, changes, false, outcomeNoOp, exitNoOp, gf.asJSON)
	}

	// D-07: a completed dry run exits 0. It did precisely what it was asked to
	// do. Reserving a non-zero code for "you did not pass --confirm" would abort
	// every preview under `set -e`; the executed/outcome fields carry the
	// distinction, and the caller already knows whether it passed the flag.
	if !wf.confirm {
		return finishWrite(name, target, changes, false, outcomeDryRun, exitOK, gf.asJSON)
	}

	err = c.UpdateWorkItemState(writeCtx, id, wf.state)

	// The parent's Err() is consulted, not the failure kind: classifyFailure maps
	// context.Canceled to FailureOther on purpose, so the kind alone cannot tell
	// you a signal fired. Unlike runRead this does NOT write a crash record and
	// does NOT return exit 3 (D-22) — a write already in flight must never
	// inherit a code that asserts nothing ran.
	outcome, code := writeExitCodeFor(err, ctx.Err())

	// D-14/D-16: executed and unknown are audited; dry runs and refused no-ops
	// are not, because the file answers "what changed in ADO" and neither
	// changed anything. Unknown is the deliberate exception — the case where a
	// mutation may or may not have landed is precisely the case most worth a
	// trail.
	if outcome == outcomeExecuted || outcome == outcomeUnknown {
		record := newAuditRecord(writeRecord{
			Command: name, Target: target, Changes: changes, Outcome: outcome,
		})
		if auditErr := appendAuditRecord(sink, record); auditErr != nil {
			log.Error("Failed to append audit record", "command", name, "error", auditErr)
			fmt.Fprintf(stderrWriter, "Failed to append audit record: %v\n", auditErr)
		}
	}

	if err != nil {
		log.Error("Write failed", "command", name, "id", id, "outcome", outcome, "error", err)
		fmt.Fprintf(stderrWriter, "Error running %s on %d: %v\n", name, id, err)
	} else {
		log.Info("Write complete", "command", name, "id", id, "outcome", outcome)
	}

	return finishWrite(name, target, changes, outcome == outcomeExecuted, outcome, code, gf.asJSON)
}

// diffWorkItem compares the pre-read state to what was requested.
//
// The entry ORDER is fixed — OriginalEstimate, RemainingWork, CompletedWork,
// then System.State — and matches the JSON-Patch operation order that
// UpdateWorkItemStateWithEffort's exact-patch test pins, so the diff a caller
// reads and the document ADO receives describe the same sequence. Only
// System.State can differ in 04-01; the effort branches are here so the order is
// a structural property of this function rather than a comment 04-03 must
// remember to honour.
func diffWorkItem(detail *ado.WorkItemDetail, wf *writeFlags) []fieldChange {
	// Non-nil at construction so it marshals as [] rather than null.
	changes := []fieldChange{}

	if wf.originalEstimate != nil {
		changes = appendFloatChange(changes, fieldOriginalEstimate, detail.OriginalEstimate, *wf.originalEstimate)
	}
	if wf.remaining != nil {
		changes = appendFloatChange(changes, fieldRemainingWork, detail.RemainingWork, *wf.remaining)
	}
	if wf.completed != nil {
		changes = appendFloatChange(changes, fieldCompletedWork, detail.CompletedWork, *wf.completed)
	}
	if wf.state != "" && detail.State != wf.state {
		changes = append(changes, fieldChange{Field: fieldState, From: detail.State, To: wf.state})
	}

	return changes
}

// appendFloatChange records an effort change only when the value actually
// differs, so an explicit --remaining equal to the current value refuses as a
// no-op exactly as a redundant --state does. An unset field on the work item
// renders as an empty from-value rather than "0": ADO not having the field and
// ADO holding zero are different facts.
func appendFloatChange(changes []fieldChange, field string, current *float64, proposed float64) []fieldChange {
	if current != nil && *current == proposed {
		return changes
	}
	from := ""
	if current != nil {
		from = strconv.FormatFloat(*current, 'f', -1, 64)
	}
	return append(changes, fieldChange{
		Field: field,
		From:  from,
		To:    strconv.FormatFloat(proposed, 'f', -1, 64),
	})
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
