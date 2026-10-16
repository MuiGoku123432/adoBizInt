package confirm

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"sentinovo.ai/bizInt/internal/guard"
)

// inflight counts the Execute calls in flight, so that process exit can wait
// for each one's audit append (CR-01, Phase 4 D-13).
var inflight sync.WaitGroup

// writeParent is the parent of every TUI guard.Run. main replaces it with its
// signal context, so an interrupt mid-write is classified unknown and audited
// (Phase 4 D-22, D-08), exactly as on the CLI. It is set once, before the
// program starts.
var writeParent = context.Background()

// SetParent installs ctx as the parent of every TUI guarded write. Call it once
// before the TUI starts (main) or in a test before any write runs: it is not
// synchronised with writes in flight.
func SetParent(ctx context.Context) {
	writeParent = ctx
}

// Wait blocks until every Execute in flight has returned, which means each
// record is appended or its failure reported. It cannot hang: every guard.Run
// is bounded by its budget through fetchctx, and a cancelled parent ends an
// in-flight Mutate at once.
func Wait() {
	inflight.Wait()
}

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
// sink or pre-read failed. The guard bounds writeParent with budget through
// fetchctx (GUARD-02), and a cancelled parent aborts a slow pre-read.
//
// Preview is not counted in inflight: with no Confirm it is a dry run, which
// sends no mutation and writes no audit record, so exiting during it loses
// nothing.
//
// This file holds the TUI's only guard.Run calls, and each one names the TUI
// as the actor (GUARD-04).
func Preview(b Batch, budget time.Duration) Batch {
	reports := make([]guard.Report, 0, len(b.Ops))
	for _, op := range b.Ops {
		report := guard.Run(writeParent, op, guard.Options{Budget: budget, Actor: guard.ActorTUI})
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
//
// Execute counts itself in inflight for its whole body, so Wait covers every
// op it runs. The Add lives here, never in Flow: bubbletea can drop a cmd once
// its event loop has stopped, and an Add with no matching Execute would make
// Wait hang. The Done is deferred so a panic out of a Plan still releases the
// wait.
func Execute(b Batch, budget time.Duration) Batch {
	inflight.Add(1)
	defer inflight.Done()

	previews := b.Reports
	reports := make([]guard.Report, 0, len(previews))
	for i, op := range b.Ops {
		if i >= len(previews) {
			break
		}
		report := guard.Run(writeParent, op, guard.Options{
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
