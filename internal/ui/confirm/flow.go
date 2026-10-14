// Package confirm is the TUI side of a guarded write: preview, confirm and
// execute through internal/guard. It is shared by the four write-capable views
// (PRs, releases, pipelines, work items), so the rule of three is met.
package confirm

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/guard"
	"sentinovo.ai/bizInt/internal/ui/modal"
	"sentinovo.ai/bizInt/internal/utils"
)

// PreviewedMsg carries a batch after its dry-run pre-read.
type PreviewedMsg struct{ Batch Batch }

// WrittenMsg carries a batch after execution.
type WrittenMsg struct{ Batch Batch }

// Result tells the embedding view what the flow did with a message.
//
// Handled means the view must not process the message itself. Status, when
// non-empty, replaces the view's status line, styled as an error when IsErr.
// Refresh asks the view to re-fetch. Proceed and Stay are set only when the
// flow settles; Kept is set only on esc and means the user wants pending edits
// kept. Batch is the written batch.
type Result struct {
	Handled bool
	Status  string
	IsErr   bool
	Refresh bool
	Proceed bool
	Stay    bool
	Kept    bool
	Batch   Batch
}

// Flow is the preview, confirm and execute state machine every write-capable
// view embeds, so the D-07, D-08 and D-10 handling exists once.
type Flow struct {
	modal   *Model
	pending Batch
	budget  time.Duration
	busy    bool
	sticky  string
}

// Start previews b. It does nothing while a write is already in flight.
func (f Flow) Start(b Batch, budget time.Duration) (Flow, tea.Cmd) {
	if f.busy {
		return f, nil
	}
	f.busy = true
	f.budget = budget
	return f, func() tea.Msg { return PreviewedMsg{Batch: Preview(b, budget)} }
}

// Busy is true from Start until the flow settles: preview in flight, modal
// open, or execution in flight.
func (f Flow) Busy() bool { return f.busy }

// Sticky is the unknown-outcome line (D-08). It survives refreshes and is
// cleared by the next keypress.
func (f Flow) Sticky() string { return f.sticky }

// View overlays the modal on background when it is open.
func (f Flow) View(background string) string {
	if f.modal == nil {
		return background
	}
	return modal.Center(f.modal.View(), background)
}

// Update runs the flow's half of a message. Every message the flow handles
// returns Handled; everything else is left to the view.
func (f Flow) Update(msg tea.Msg) (Flow, tea.Cmd, Result) {
	switch msg := msg.(type) {
	case PreviewedMsg:
		if !f.busy || f.modal != nil {
			return f, nil, Result{Handled: true}
		}
		return f.previewed(msg.Batch)

	case tea.KeyMsg:
		if f.modal != nil {
			m, cmd := f.modal.Update(msg)
			f.modal = &m
			return f, cmd, Result{Handled: true}
		}
		if f.busy {
			// A request is in flight: rows must not move under it.
			return f, nil, Result{Handled: true}
		}
		f.sticky = ""
		return f, nil, Result{}

	case WriteMsg:
		if f.modal == nil {
			return f, nil, Result{Handled: true}
		}
		f.modal = nil
		pending, budget := f.pending, f.budget
		return f, func() tea.Msg { return WrittenMsg{Batch: Execute(pending, budget)} },
			Result{Handled: true, Status: fmt.Sprintf("Writing #%d...", pending.ID)}

	case DiscardMsg:
		if f.modal == nil {
			return f, nil, Result{Handled: true}
		}
		id := f.pending.ID
		f, res := f.settle(Result{Status: fmt.Sprintf("Discarded: nothing written for #%d", id), Proceed: true})
		return f, nil, res

	case StayMsg:
		if f.modal == nil {
			return f, nil, Result{Handled: true}
		}
		id := f.pending.ID
		f, res := f.settle(Result{Status: fmt.Sprintf("Not written: #%d", id), Stay: true, Kept: true})
		return f, nil, res

	case WrittenMsg:
		if !f.busy || f.modal != nil {
			return f, nil, Result{Handled: true}
		}
		res, sticky := written(msg.Batch)
		if sticky != "" {
			f.sticky = sticky
		}
		f, res = f.settle(res)
		return f, nil, res
	}
	return f, nil, Result{}
}

// previewed settles on a blocked sink (D-10), a failed pre-read or an all
// no-op preview (D-07), and otherwise opens the modal.
func (f Flow) previewed(b Batch) (Flow, tea.Cmd, Result) {
	for _, report := range b.Reports {
		if report.SinkErr != nil {
			f, res := f.settle(Result{Status: "write blocked: audit log not writable: " + guard.DefaultAuditPath(), IsErr: true, Stay: true})
			return f, nil, res
		}
		if report.PreReadErr != nil {
			f, res := f.settle(Result{Status: fmt.Sprintf("#%d: could not read before writing: %s", b.ID, clean(report.PreReadErr.Error())), IsErr: true, Stay: true})
			return f, nil, res
		}
	}

	allNoOp := len(b.Reports) > 0
	for _, report := range b.Reports {
		if report.Outcome != guard.OutcomeNoOp {
			allNoOp = false
		}
	}
	if allNoOp {
		label := b.Labels[0]
		if b.NoOpLabel != nil && b.NoOpLabel() != "" {
			label = b.NoOpLabel()
		}
		f, res := f.settle(Result{Status: fmt.Sprintf("No change: #%d already %s", b.ID, label), Proceed: true})
		return f, nil, res
	}

	m := New(fmt.Sprintf("Confirm #%d", b.ID), b.Lines())
	f.modal = &m
	f.pending = b
	return f, nil, Result{Handled: true}
}

func (f Flow) settle(res Result) (Flow, Result) {
	f.busy = false
	f.modal = nil
	f.pending = Batch{}
	res.Handled = true
	return f, res
}

// written classifies an executed batch. An unknown outcome returns the text as
// the sticky line and leaves Status empty (D-08); any other non-executed,
// non-no-op part makes the text an error status.
func written(b Batch) (Result, string) {
	res := Result{Batch: b, Proceed: true}
	unknown := false
	for i := range b.Ops {
		if i >= len(b.Reports) {
			res.Proceed = false
			continue
		}
		switch b.Reports[i].Outcome {
		case guard.OutcomeExecuted:
			res.Refresh = true
		case guard.OutcomeNoOp:
		case guard.OutcomeFailed:
			res.Refresh = true
			res.Proceed = false
		case guard.OutcomeUnknown:
			res.Refresh = true
			res.Proceed = false
			unknown = true
		default:
			res.Proceed = false
		}
	}
	res.Stay = !res.Proceed
	text := writtenText(b)
	if unknown {
		res.IsErr = true
		return res, text
	}
	res.Status = text
	res.IsErr = !res.Proceed
	return res, ""
}

// writtenText builds one part per op under the 07-UI-SPEC first-part rule (C3):
// labels are past-tense success words, so a first op that did not execute
// attaches to the ID with ": " and no label. Later parts keep their label,
// which yields the D-09 literal "#5 closed; effort update FAILED: <error>".
func writtenText(b Batch) string {
	parts := make([]string, 0, len(b.Ops))
	labelLess := false
	for i := range b.Ops {
		label := b.Labels[i]
		if i >= len(b.Reports) {
			parts = append(parts, label+" not attempted")
			continue
		}
		report := b.Reports[i]
		var part, bare string
		switch {
		case report.SinkErr != nil:
			if i == 0 {
				// D-10: nothing was sent, and the literal carries no ID.
				return "write blocked: audit log not writable: " + guard.DefaultAuditPath()
			}
			part = label + " not written: audit log not writable: " + guard.DefaultAuditPath()
		case report.PreReadErr != nil:
			bare = "could not read before writing: " + clean(report.PreReadErr.Error())
			part = label + " not written: " + bare
		case report.Outcome == guard.OutcomeExecuted:
			part = label
		case report.Outcome == guard.OutcomeNoOp:
			part = label + " (no change)"
		case report.Outcome == guard.OutcomeDryRun:
			bare = "not written — changed since preview; press the action key again to re-check"
			// No re-check hint on a later op: an earlier op already wrote, so
			// pressing the key again would act on the new state rather than
			// re-check this change.
			part = label + " not written: changed since preview"
		case report.Outcome == guard.OutcomeFailed:
			bare = "write FAILED: " + clean(report.MutateErr.Error())
			part = label + " FAILED: " + clean(report.MutateErr.Error())
		case report.Outcome == guard.OutcomeUnknown:
			bare = "outcome unknown — verify in ADO"
			part = label + ": " + bare
		}
		if i == 0 && bare != "" {
			part = bare
			labelLess = true
		}
		if report.Outcome == guard.OutcomeUnknown && report.AuditErr != nil {
			part += " (not recorded in audit log)"
		}
		parts = append(parts, part)
	}
	sep := " "
	if labelLess {
		sep = ": "
	}
	return fmt.Sprintf("#%d%s%s", b.ID, sep, strings.Join(parts, "; "))
}

// clean drops control runes from error text and truncates it to 120 runes, so a
// server-controlled message cannot carry escape sequences into the status line.
func clean(s string) string {
	return utils.Truncate(dropControl(s), 120)
}
