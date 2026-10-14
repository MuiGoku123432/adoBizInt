package guard

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"sentinovo.ai/bizInt/internal/ado"
)

// Client is the seam every guarded write calls through. *ado.Client satisfies
// it structurally. It is separate from the CLI's readClient: that interface's
// membership is pinned to the eight aggregate reads returning ado.Result[T], and
// a write returns a bare error with no failure list and no completeness flag, so
// the two contracts cannot share a mapper.
//
// The by-id pre-read belongs here, not in readClient, for the same reason:
// GetWorkItemDetail returns a bare type and is deliberately excluded there.
type Client interface {
	// GetWorkItemDetail is the write path's pre-read. D-01 makes a dry run
	// report current -> proposed rather than echoing intent, and D-03 uses the
	// same read to refuse a no-op on the confirmed path.
	GetWorkItemDetail(ctx context.Context, id int) (*ado.WorkItemDetail, error)

	UpdateWorkItemState(ctx context.Context, project string, itemID int, newState string) error

	// UpdateWorkItemStateWithEffort carries the three nullable effort values. A
	// nil pointer means the caller did not name the field, so no JSON-Patch
	// operation is emitted for it and ADO leaves it untouched (D-17/D-18). The
	// write path routes here only when at least one effort value was set, so an
	// effort-free write never hands this method three nils.
	UpdateWorkItemStateWithEffort(ctx context.Context, project string, itemID int, newState string, originalEstimate, remaining, completed *float64) error

	// The three by-id pre-reads for the non-work-item verbs. Unlike
	// GetWorkItemDetail they never return (nil, nil) for a missing target
	// (internal/ado's TestPreReadNeverReturnsNilNilStructural enforces that
	// structurally), so a caller here can never read a missing target as an
	// empty diff (D-02, threat T-04-13).
	GetPullRequestDetail(ctx context.Context, project, repositoryID string, prID int) (*ado.PullRequestDetail, error)
	GetReleaseApprovalDetail(ctx context.Context, project string, approvalID int) (*ado.ReleaseApprovalDetail, error)
	GetPipelineDefinitionDetail(ctx context.Context, project string, definitionID int) (*ado.PipelineDefinitionDetail, error)

	// The five remaining mutations. Each returns a bare error, which is why the
	// outcome is classified here rather than by the read path's exit mapping.
	ApprovePullRequest(ctx context.Context, project, repositoryID string, prID int) error
	CompletePullRequest(ctx context.Context, project, repositoryID string, prID int) error
	ApproveRelease(ctx context.Context, project string, approvalID int, comment string) error
	RejectRelease(ctx context.Context, project string, approvalID int, comment string) error

	// QueuePipeline returns the queued run; the write path discards it. The
	// report carries the change that was requested, not a second read of what
	// ADO then created, because those are different claims.
	QueuePipeline(ctx context.Context, project string, definitionID int, branch string) (*ado.PipelineRun, error)
}

// Work item field names as ADO spells them. They are the `field` values of the
// diff, so they are part of the published envelope and of every persisted audit
// line: a caller matching on "System.State" must keep matching.
const (
	FieldState            = "System.State"
	FieldOriginalEstimate = "Microsoft.VSTS.Scheduling.OriginalEstimate"
	FieldRemainingWork    = "Microsoft.VSTS.Scheduling.RemainingWork"
	FieldCompletedWork    = "Microsoft.VSTS.Scheduling.CompletedWork"
)

// Field names for the non-work-item verbs. Same contract as the block above:
// these strings are the `field` values of the envelope and of every persisted
// audit line, so they are a one-way door.
const (
	FieldReviewerVote   = "reviewer.vote"
	FieldPRStatus       = "status"
	FieldApprovalStatus = "approval.status"
	FieldQueuedBuild    = "queued_build"
)

// VoteApproved is Azure DevOps' own reviewer-vote value for "approved", the
// same literal ApprovePullRequest sends (pullrequests.go:183).
const VoteApproved = 10

// Effort holds the three nullable effort values. A nil pointer means "not
// named", which is different from zero (D-17, D-18, GUARD-05): never give these
// a float64 + omitempty shape anywhere between the caller and the patch.
type Effort struct {
	OriginalEstimate *float64
	Remaining        *float64
	Completed        *float64
}

// SetState is the work item verb. The caller supplies the proposed state; it is
// never inferred from config (D-26).
func SetState(c Client, project string, id int, state string, effort Effort) Op {
	return Op{
		Name:   "workitems set-state",
		Target: Target{Project: project, ID: id},
		Plan: func(ctx context.Context) ([]FieldChange, error) {
			detail, err := c.GetWorkItemDetail(ctx, id)
			if err != nil {
				return nil, err
			}
			// GetWorkItemDetail predates preread.go and still returns (nil, nil)
			// for a missing item (workitems.go:384-386), so the nil is checked
			// here rather than trusted. The three pre-reads in preread.go cannot
			// do this to us: TestPreReadNeverReturnsNilNilStructural forbids it.
			if detail == nil {
				return nil, fmt.Errorf("work item %d was not found", id)
			}
			// CR-01. Work item ids are unique across the organization, so the
			// id alone does not scope this write to the project: without this
			// check the mutation lands on whichever project owns the item and
			// the audit record names the one the caller typed. A mismatch is
			// refused here, before the mutation and before the audit append,
			// like every other "nothing ran" refusal. The comparison is exact for
			// the same reason the allow-list's is. An item whose project the
			// server did not report is refused too.
			if detail.Project != project {
				return nil, fmt.Errorf("work item %d belongs to project %q, not %q", id, detail.Project, project)
			}
			return diffWorkItem(detail, state, effort), nil
		},
		Mutate: func(ctx context.Context) error {
			// Route through the effort method only when the caller named an
			// effort field. Sending three nils through it would be harmless but
			// dishonest: an effort-free write is not an effort write with
			// nothing to say.
			if effort.OriginalEstimate != nil || effort.Remaining != nil || effort.Completed != nil {
				return c.UpdateWorkItemStateWithEffort(ctx, project, id, state,
					effort.OriginalEstimate, effort.Remaining, effort.Completed)
			}
			return c.UpdateWorkItemState(ctx, project, id, state)
		},
	}
}

// ApprovePR casts the acting identity's approving vote.
func ApprovePR(c Client, project, repo string, id int) Op {
	return Op{
		Name:   "prs approve",
		Target: Target{Project: project, Repo: repo, ID: id},
		Plan: func(ctx context.Context) ([]FieldChange, error) {
			detail, err := c.GetPullRequestDetail(ctx, project, repo, id)
			if err != nil {
				return nil, err
			}
			changes := []FieldChange{}
			// Already approved by this identity: D-03 refuses rather than
			// re-casting a vote that is already recorded.
			if detail.MyVote != VoteApproved {
				changes = append(changes, FieldChange{
					Field: FieldReviewerVote,
					From:  strconv.Itoa(detail.MyVote),
					To:    strconv.Itoa(VoteApproved),
				})
			}
			return changes, nil
		},
		Mutate: func(ctx context.Context) error {
			return c.ApprovePullRequest(ctx, project, repo, id)
		},
	}
}

// CompletePR completes (merges) a pull request. The verb is `complete`, not
// `merge`: the one spelling matches the ado method it drives, and D-23's
// reasoning against accepting two spellings for one input applies to verb names
// as much as to flags.
func CompletePR(c Client, project, repo string, id int) Op {
	return Op{
		Name:   "prs complete",
		Target: Target{Project: project, Repo: repo, ID: id},
		Plan: func(ctx context.Context) ([]FieldChange, error) {
			detail, err := c.GetPullRequestDetail(ctx, project, repo, id)
			if err != nil {
				return nil, err
			}
			changes := []FieldChange{}
			// Completed and abandoned are both terminal: neither can be
			// completed again, so both refuse as a no-op rather than being sent
			// to ADO to be rejected.
			switch strings.ToLower(detail.Status) {
			case "completed", "abandoned":
			default:
				changes = append(changes, FieldChange{
					Field: FieldPRStatus, From: detail.Status, To: "completed",
				})
			}
			return changes, nil
		},
		Mutate: func(ctx context.Context) error {
			return c.CompletePullRequest(ctx, project, repo, id)
		},
	}
}

// ApproveRelease approves a pending release approval.
func ApproveRelease(c Client, project string, id int, comment string) Op {
	return releaseApproval("releases approve", "approved", c.ApproveRelease, c, project, id, comment)
}

// RejectRelease rejects a pending release approval.
func RejectRelease(c Client, project string, id int, comment string) Op {
	return releaseApproval("releases reject", "rejected", c.RejectRelease, c, project, id, comment)
}

// releaseApproval builds the approve and reject verbs, which differ only in the
// status they propose and the method they call. Writing them as one constructor
// is what makes their guard behaviour identical by construction.
func releaseApproval(name, wantStatus string, mutate func(context.Context, string, int, string) error, c Client, project string, id int, comment string) Op {
	return Op{
		Name:   name,
		Target: Target{Project: project, ID: id},
		Plan: func(ctx context.Context) ([]FieldChange, error) {
			detail, err := c.GetReleaseApprovalDetail(ctx, project, id)
			if err != nil {
				return nil, err
			}
			changes := []FieldChange{}
			if !strings.EqualFold(detail.Status, wantStatus) {
				changes = append(changes, FieldChange{
					Field: FieldApprovalStatus, From: detail.Status, To: wantStatus,
				})
			}
			return changes, nil
		},
		Mutate: func(ctx context.Context) error {
			// The comment defaults to empty, which is what the ado method already
			// receives when the TUI omits it (P-03). The comment is carried into
			// ADO, not into the audit record.
			return mutate(ctx, project, id, comment)
		},
	}
}

// QueuePipeline triggers a pipeline run on the given branch. The branch is
// required by every caller and never defaulted (P-04).
func QueuePipeline(c Client, project string, id int, branch string) Op {
	return Op{
		Name:   "pipelines queue",
		Target: Target{Project: project, ID: id},
		Plan: func(ctx context.Context) ([]FieldChange, error) {
			// The pre-read still runs, and its failure is still fatal (D-02):
			// queuing a build against a definition that does not exist must fail
			// loudly rather than be attempted.
			if _, err := c.GetPipelineDefinitionDetail(ctx, project, id); err != nil {
				return nil, err
			}
			// A queue is not an idempotent state change: there is no state the
			// definition could already be in that makes "queue a run" redundant,
			// so this verb has no "already applied" condition and emits one
			// change unconditionally. That is deliberate: it keeps the changes
			// non-empty, so the no-op branch is never reached for this verb and a
			// queue is never refused as a no-op it cannot be.
			return []FieldChange{{
				Field: FieldQueuedBuild, From: "", To: branch,
			}}, nil
		},
		Mutate: func(ctx context.Context) error {
			_, err := c.QueuePipeline(ctx, project, id, branch)
			return err
		},
	}
}

// diffWorkItem compares the pre-read state to what was requested.
//
// The entry ORDER is fixed (OriginalEstimate, RemainingWork, CompletedWork,
// then System.State) and matches the JSON-Patch operation order that
// UpdateWorkItemStateWithEffort's exact-patch test pins, so the diff a caller
// reads and the document ADO receives describe the same sequence.
func diffWorkItem(detail *ado.WorkItemDetail, state string, effort Effort) []FieldChange {
	// Non-nil at construction so it marshals as [] rather than null.
	changes := []FieldChange{}

	if effort.OriginalEstimate != nil {
		changes = appendFloatChange(changes, FieldOriginalEstimate, detail.OriginalEstimate, *effort.OriginalEstimate)
	}
	if effort.Remaining != nil {
		changes = appendFloatChange(changes, FieldRemainingWork, detail.RemainingWork, *effort.Remaining)
	}
	if effort.Completed != nil {
		changes = appendFloatChange(changes, FieldCompletedWork, detail.CompletedWork, *effort.Completed)
	}
	if state != "" && detail.State != state {
		changes = append(changes, FieldChange{Field: FieldState, From: detail.State, To: state})
	}

	return changes
}

// appendFloatChange records an effort change only when the value actually
// differs, so an explicit remaining value equal to the current one refuses as a
// no-op exactly as a redundant state does. An unset field on the work item
// renders as an empty from-value rather than "0": ADO not having the field and
// ADO holding zero are different facts.
func appendFloatChange(changes []FieldChange, field string, current *float64, proposed float64) []FieldChange {
	if current != nil && *current == proposed {
		return changes
	}
	from := ""
	if current != nil {
		from = strconv.FormatFloat(*current, 'f', -1, 64)
	}
	return append(changes, FieldChange{
		Field: field,
		From:  from,
		To:    strconv.FormatFloat(proposed, 'f', -1, 64),
	})
}
