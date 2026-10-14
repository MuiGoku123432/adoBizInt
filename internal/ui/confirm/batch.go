package confirm

import (
	"context"
	"fmt"
	"slices"
	"time"

	"sentinovo.ai/bizInt/internal/guard"
)

// Batch is one guarded write as the user sees it: the row it is about and the
// ops it runs, in order.
//
// Labels has one past-tense word per op for status text ("approved",
// "completed", "closed", "effort update"). NoOpLabel is optional: when it is set
// and returns a non-empty string, the all-no-op status uses it in place of
// Labels[0]. Reports holds the latest run's reports, one per op that ran.
type Batch struct {
	ID        int
	Ops       []guard.Op
	Labels    []string
	NoOpLabel func() string
	Reports   []guard.Report
}

// Preview dry-runs each op (no Confirm) and stops after the first report whose
// sink or pre-read failed. The guard bounds the context with budget through
// fetchctx (GUARD-02), so the root context is safe here.
//
// This file holds the TUI's only guard.Run calls, and each one names the TUI
// as the actor (GUARD-04).
func Preview(b Batch, budget time.Duration) Batch {
	reports := make([]guard.Report, 0, len(b.Ops))
	for _, op := range b.Ops {
		report := guard.Run(context.Background(), op, guard.Options{Budget: budget, Actor: guard.ActorTUI})
		reports = append(reports, report)
		if report.SinkErr != nil || report.PreReadErr != nil {
			break
		}
	}
	b.Reports = reports
	return b
}

// Execute runs each op that has a preview report, confirming it only when the
// fresh pre-read matches what the modal showed, and stops after the first
// outcome that is neither executed nor no_op. The returned Reports are the
// execution reports.
func Execute(b Batch, budget time.Duration) Batch {
	previews := b.Reports
	reports := make([]guard.Report, 0, len(previews))
	for i, op := range b.Ops {
		if i >= len(previews) {
			break
		}
		report := guard.Run(context.Background(), op, guard.Options{
			Budget:  budget,
			Actor:   guard.ActorTUI,
			Confirm: approves(previews[i].Changes),
		})
		reports = append(reports, report)
		if report.Outcome != guard.OutcomeExecuted && report.Outcome != guard.OutcomeNoOp {
			break
		}
	}
	b.Reports = reports
	return b
}

// approves is what makes y mean "write what I saw": it accepts the fresh diff
// only when every change in it is one the modal showed (T-07-15).
func approves(shown []guard.FieldChange) func([]guard.FieldChange) bool {
	return func(fresh []guard.FieldChange) bool {
		for _, change := range fresh {
			if !slices.Contains(shown, change) {
				return false
			}
		}
		return true
	}
}

// Lines formats the reports' changes as "<Field>  <From> → <To>", one per
// distinct change. An empty From is "(none)", and a change on another target
// than the batch's row is prefixed "#<id> ".
func (b Batch) Lines() []string {
	type seenKey struct {
		id     int
		change guard.FieldChange
	}
	seen := map[seenKey]bool{}
	var lines []string
	for i, report := range b.Reports {
		id := b.Ops[i].Target.ID
		for _, change := range report.Changes {
			k := seenKey{id, change}
			if seen[k] {
				continue
			}
			seen[k] = true
			from := change.From
			if from == "" {
				from = "(none)"
			}
			line := fmt.Sprintf("%s  %s → %s", change.Field, from, change.To)
			if id != b.ID {
				line = fmt.Sprintf("#%d %s", id, line)
			}
			lines = append(lines, line)
		}
	}
	return lines
}
