package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"

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

// Field names for the non-work-item verbs. Same contract as the block above:
// these strings are the `field` values of the envelope and of every persisted
// audit line, so they are a one-way door.
const (
	fieldReviewerVote   = "reviewer.vote"
	fieldPRStatus       = "status"
	fieldApprovalStatus = "approval.status"
	fieldQueuedBuild    = "queued_build"
)

// voteApproved is Azure DevOps' own reviewer-vote value for "approved", the
// same literal ApprovePullRequest sends (pullrequests.go:183).
const voteApproved = 10

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

// writeOp is the per-verb half of a guarded write. It exists so the six verbs
// share ONE body rather than six copies of the ten-step order, mirroring how the
// read side gives each verb its own closure differing only in fetch func and
// renderer (dispatch.go:136-175). A guard that lives in a shared body cannot be
// forgotten on a verb; a guard copied six times can.
//
// Each field is the whole of one verb's difference:
//   - resolve enforces the per-verb arity rules and builds the target. Returning
//     an error means nothing was attempted, which is exit 3.
//   - plan runs the by-id pre-read and computes the current -> proposed diff. A
//     non-nil error is fatal and emits nothing to stdout (D-02).
//   - mutate issues the single mutation.
type writeOp struct {
	name    string
	resolve func(gf *globalFlags, wf *writeFlags, id int) (writeTarget, error)
	plan    func(ctx context.Context, t writeTarget, wf *writeFlags) ([]fieldChange, error)
	mutate  func(ctx context.Context, t writeTarget, wf *writeFlags) error
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

// runWrite is the guarded-write body, the sibling of runRead. The ORDER of its
// steps is the contract, not an implementation detail — in particular the audit
// sink is opened before anything is attempted (D-13), and the no-op refusal
// precedes the --confirm branch because D-03 refuses on both paths.
func runWrite(ctx context.Context, op writeOp, args []string) int {
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

	log := logging.Logger()

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
	// whichever of --repo and --branch this verb's payload actually needs.
	target, err := op.resolve(gf, wf, id)
	if err != nil {
		fmt.Fprintf(stderrWriter, "%v\n", err)
		return exitUnknown
	}

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
	sink, err := auditOpen()
	if err != nil {
		log.Error("Audit sink unavailable", "command", name, "error", err)
		fmt.Fprintf(stderrWriter, "Refusing to write: audit sink %s is not writable: %v\n", auditPath, err)
		return exitUnknown
	}
	defer sink.Close()

	// D-02: a failed pre-read is fatal and emits NOTHING to stdout. A dry run
	// that says "would set State=Closed" without having established the current
	// state is a confident statement made without looking.
	changes, err := op.plan(writeCtx, target, wf)
	if err != nil {
		log.Error("Write pre-read failed", "command", name, "id", id, "error", err)
		fmt.Fprintf(stderrWriter, "Error reading %s target %d before writing: %v\n", name, id, err)
		return exitUnknown
	}

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

	err = op.mutate(writeCtx, target, wf)

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
			// The operator needs to know WHAT to go check, not merely that a
			// write to a file failed — the mutation may already be live in ADO.
			fmt.Fprintf(stderrWriter, "Write may have landed but was NOT recorded in %s: %v\n", auditPath, auditErr)

			// CR-01. The mutation landed in ADO and nothing recorded it, which
			// genuinely IS an unknown outcome: SAFE-02 ("every executed write
			// appends an audit record") is falsified for this run, so reporting
			// exit 0 / "executed" would be the exact conflation this project
			// exists to prevent. This reuses the already-ratified D-10/D-23
			// vocabulary rather than inventing a seventh exit code. The
			// reassignment is idempotent: a run already classified unknown is
			// unchanged. finishWrite's `executed` argument at the return below
			// falls to false on its own, because it is computed as
			// `outcome == outcomeExecuted`.
			outcome = outcomeUnknown
			code = exitOutcomeUnknown
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

// setStateOp is the work item verb (04-01's tracer), expressed as a writeOp so
// it shares the same body as the five verbs added alongside it.
func setStateOp(c writeClient) writeOp {
	const name = "workitems set-state"
	return writeOp{
		name: name,
		resolve: func(gf *globalFlags, wf *writeFlags, id int) (writeTarget, error) {
			project, err := oneProject(name, gf)
			if err != nil {
				return writeTarget{}, err
			}
			// D-26: --state is required and is never inferred from
			// config.StateTransitions or GetNextState. A config-driven default
			// means the same command produces different mutations on two
			// machines, and it would also make the current -> proposed diff
			// meaningless, since "proposed" would not have come from the caller.
			if wf.state == "" {
				return writeTarget{}, fmt.Errorf("%s requires --state", name)
			}
			return writeTarget{Project: project, ID: id}, nil
		},
		plan: func(ctx context.Context, t writeTarget, wf *writeFlags) ([]fieldChange, error) {
			detail, err := c.GetWorkItemDetail(ctx, t.ID)
			if err != nil {
				return nil, err
			}
			// GetWorkItemDetail predates preread.go and still returns (nil, nil)
			// for a missing item (workitems.go:384-386), so the nil is checked
			// here rather than trusted. The three pre-reads in preread.go cannot
			// do this to us — TestPreReadNeverReturnsNilNilStructural forbids it.
			if detail == nil {
				return nil, fmt.Errorf("work item %d was not found", t.ID)
			}
			return diffWorkItem(detail, wf), nil
		},
		mutate: func(ctx context.Context, t writeTarget, wf *writeFlags) error {
			// Route through the effort method only when the caller named an
			// effort field. Sending three nils through it would be harmless but
			// dishonest: an effort-free write is not an effort write with
			// nothing to say.
			if wf.hasEffort() {
				return c.UpdateWorkItemStateWithEffort(ctx, t.ID, wf.state,
					wf.originalEstimate, wf.remaining, wf.completed)
			}
			return c.UpdateWorkItemState(ctx, t.ID, wf.state)
		},
	}
}

// prsApproveOp casts the acting identity's approving vote.
func prsApproveOp(c writeClient) writeOp {
	const name = "prs approve"
	return writeOp{
		name:    name,
		resolve: resolveProjectRepo(name),
		plan: func(ctx context.Context, t writeTarget, _ *writeFlags) ([]fieldChange, error) {
			detail, err := c.GetPullRequestDetail(ctx, t.Project, t.Repo, t.ID)
			if err != nil {
				return nil, err
			}
			changes := []fieldChange{}
			// Already approved by this identity: D-03 refuses rather than
			// re-casting a vote that is already recorded.
			if detail.MyVote != voteApproved {
				changes = append(changes, fieldChange{
					Field: fieldReviewerVote,
					From:  strconv.Itoa(detail.MyVote),
					To:    strconv.Itoa(voteApproved),
				})
			}
			return changes, nil
		},
		mutate: func(ctx context.Context, t writeTarget, _ *writeFlags) error {
			return c.ApprovePullRequest(ctx, t.Project, t.Repo, t.ID)
		},
	}
}

// prsCompleteOp completes (merges) a pull request. The verb is `complete`, not
// `merge`: the one spelling matches the ado method it drives, and D-23's
// reasoning against accepting two spellings for one input applies to verb names
// as much as to flags.
func prsCompleteOp(c writeClient) writeOp {
	const name = "prs complete"
	return writeOp{
		name:    name,
		resolve: resolveProjectRepo(name),
		plan: func(ctx context.Context, t writeTarget, _ *writeFlags) ([]fieldChange, error) {
			detail, err := c.GetPullRequestDetail(ctx, t.Project, t.Repo, t.ID)
			if err != nil {
				return nil, err
			}
			changes := []fieldChange{}
			// Completed and abandoned are both terminal: neither can be
			// completed again, so both refuse as a no-op rather than being sent
			// to ADO to be rejected.
			switch strings.ToLower(detail.Status) {
			case "completed", "abandoned":
			default:
				changes = append(changes, fieldChange{
					Field: fieldPRStatus, From: detail.Status, To: "completed",
				})
			}
			return changes, nil
		},
		mutate: func(ctx context.Context, t writeTarget, _ *writeFlags) error {
			return c.CompletePullRequest(ctx, t.Project, t.Repo, t.ID)
		},
	}
}

// releaseApprovalOp builds the approve and reject verbs, which differ only in
// the status they propose and the method they call. Writing them as one
// constructor is what makes their guard behaviour identical by construction.
func releaseApprovalOp(name, wantStatus string, mutate func(context.Context, string, int, string) error, c writeClient) writeOp {
	return writeOp{
		name: name,
		resolve: func(gf *globalFlags, _ *writeFlags, id int) (writeTarget, error) {
			project, err := oneProject(name, gf)
			if err != nil {
				return writeTarget{}, err
			}
			return writeTarget{Project: project, ID: id}, nil
		},
		plan: func(ctx context.Context, t writeTarget, _ *writeFlags) ([]fieldChange, error) {
			detail, err := c.GetReleaseApprovalDetail(ctx, t.Project, t.ID)
			if err != nil {
				return nil, err
			}
			changes := []fieldChange{}
			if !strings.EqualFold(detail.Status, wantStatus) {
				changes = append(changes, fieldChange{
					Field: fieldApprovalStatus, From: detail.Status, To: wantStatus,
				})
			}
			return changes, nil
		},
		mutate: func(ctx context.Context, t writeTarget, wf *writeFlags) error {
			// --comment defaults to empty, which is what the ado method already
			// receives when the TUI omits it (P-03). The comment is carried into
			// ADO, not into the audit record.
			return mutate(ctx, t.Project, t.ID, wf.comment)
		},
	}
}

// pipelinesQueueOp triggers a pipeline run.
func pipelinesQueueOp(c writeClient) writeOp {
	const name = "pipelines queue"
	return writeOp{
		name: name,
		resolve: func(gf *globalFlags, wf *writeFlags, id int) (writeTarget, error) {
			project, err := oneProject(name, gf)
			if err != nil {
				return writeTarget{}, err
			}
			// P-04: --branch is required, never defaulted from the definition's
			// own default branch. The branch IS the substance of the mutation,
			// and inferring it would be exactly the config-driven behaviour D-26
			// rejects — the same invocation would queue a different build on two
			// machines.
			if wf.branch == "" {
				return writeTarget{}, fmt.Errorf("%s requires --branch", name)
			}
			return writeTarget{Project: project, ID: id}, nil
		},
		plan: func(ctx context.Context, t writeTarget, wf *writeFlags) ([]fieldChange, error) {
			// The pre-read still runs, and its failure is still fatal (D-02):
			// queuing a build against a definition that does not exist must fail
			// loudly rather than be attempted.
			if _, err := c.GetPipelineDefinitionDetail(ctx, t.Project, t.ID); err != nil {
				return nil, err
			}
			// A queue is not an idempotent state change: there is no state the
			// definition could already be in that makes "queue a run" redundant,
			// so this verb has no "already applied" condition and emits one
			// change unconditionally. That is deliberate — it keeps `changes`
			// non-empty, so the no-op branch is never reached for this verb and
			// a queue is never refused as a no-op it cannot be.
			return []fieldChange{{
				Field: fieldQueuedBuild, From: "", To: wf.branch,
			}}, nil
		},
		mutate: func(ctx context.Context, t writeTarget, wf *writeFlags) error {
			_, err := c.QueuePipeline(ctx, t.Project, t.ID, wf.branch)
			return err
		},
	}
}

// resolveProjectRepo is the shared arity rule for the two verbs addressing a
// pull request: exactly one --project and exactly one --repo.
func resolveProjectRepo(name string) func(*globalFlags, *writeFlags, int) (writeTarget, error) {
	return func(gf *globalFlags, wf *writeFlags, id int) (writeTarget, error) {
		project, err := oneProject(name, gf)
		if err != nil {
			return writeTarget{}, err
		}
		repo, err := oneRepo(name, wf)
		if err != nil {
			return writeTarget{}, err
		}
		return writeTarget{Project: project, Repo: repo, ID: id}, nil
	}
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
