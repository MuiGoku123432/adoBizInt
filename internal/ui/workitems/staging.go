package workitems

// Staged edits (GUARD-03). A state change on a work item is staged, not sent.
// It is confirmed once, through the shared confirm flow, when the user moves to
// another row or opens another item (D-01, D-02), and only y writes it.

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evertras/bubble-table/table"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/guard"
	"sentinovo.ai/bizInt/internal/ui/confirm"
)

// stagedEdit is a state change waiting for confirmation. item is the row as it
// was when staged, so the write carries the row's own project; guard.SetState
// refuses a project that does not own the item (CR-01). effort is set when the
// change came from the effort modal (a resolve).
type stagedEdit struct {
	item   ado.WorkItem
	state  string
	effort *effortEdit
}

// effortEdit is the effort half of a resolve: the values the effort modal
// returned, for taskID (the staged item for a Task, the development task for a
// User Story). A nil value is a field the user left blank. It stays
// nil all the way to the client, so that field is left unchanged (GUARD-05,
// BL-01).
type effortEdit struct {
	taskID int
	values guard.Effort
}

// batch is the guarded write for s: the staged item's state first, then, for a
// User Story, closing its development task, then the effort (D-09). Each op is
// its own guarded write with its own audit record. The effort op re-sends the
// state, but its fresh pre-read already shows the new state, so its diff and
// record carry only the effort fields (GUARD-03). Execute stops at the first op
// that does not land and reports the rest as not attempted; nothing is rolled
// back, because a compensating write would be an unguarded guess (D-09).
func (s stagedEdit) batch(w guard.Client) confirm.Batch {
	b := confirm.Batch{
		ID:     s.item.ID,
		Ops:    []guard.Op{guard.SetState(w, s.item.Project, s.item.ID, s.state, guard.Effort{})},
		Labels: []string{strings.ToLower(s.state)},
	}
	if s.effort == nil {
		return b
	}
	taskState := s.state
	if s.effort.taskID != s.item.ID {
		// The development task is written under the story's project, because
		// GetChildWorkItems does not report a child's project. guard.SetState
		// refuses the write when the task lives in another project (CR-01).
		taskState = "Closed"
		b.Ops = append(b.Ops, guard.SetState(w, s.item.Project, s.effort.taskID, taskState, guard.Effort{}))
		b.Labels = append(b.Labels, fmt.Sprintf("task #%d closed", s.effort.taskID))
	}
	b.Ops = append(b.Ops, guard.SetState(w, s.item.Project, s.effort.taskID, taskState, s.effort.values))
	b.Labels = append(b.Labels, "effort update")
	return b
}

// highlightedID is the work item ID on t's highlighted row, or 0 when there is
// none.
func highlightedID(t table.Model) int {
	idStr, _ := t.HighlightedRow().Data[columnKeyID].(string)
	id, _ := strconv.Atoi(idStr)
	return id
}

// HasStaged reports whether an edit is staged and not yet confirmed.
func (m Model) HasStaged() bool { return m.staged != nil }

// stage replaces any staged edit with item → state, plus effort when the
// change came from the effort modal (nil otherwise). Nothing is sent.
func (m *Model) stage(item ado.WorkItem, state string, effort *effortEdit) {
	m.staged = &stagedEdit{item: item, state: state, effort: effort}
	m.statusMsg = fmt.Sprintf("#%d staged: %s — move to another row to confirm", item.ID, state)
	m.statusErr = false
	m.table = m.table.WithRows(m.buildRows())
}

// startConfirm previews the staged edit; the confirm modal opens when the
// pre-read finds a change.
func (m *Model) startConfirm() tea.Cmd {
	m.statusMsg = fmt.Sprintf("Checking #%d...", m.staged.item.ID)
	m.statusErr = false
	var cmd tea.Cmd
	m.flow, cmd = m.flow.Start(m.staged.batch(m.writer), m.client.FetchTimeout())
	return cmd
}

// applyFlow applies a message the flow handled. When the flow settles, the
// staged edit is dropped unless the user pressed esc (Kept), and the held
// navigation is replayed on Proceed or forgotten on Stay; a confirm started by
// ConfirmLeave yields ProceedMsg on Proceed. Every settle that is
// not Proceed (failed, unknown, failed preview) keeps the user on the item, and
// none of them leave the edit staged, so a failing preview cannot trap them.
func (m Model) applyFlow(res confirm.Result, flowCmd tea.Cmd) (Model, tea.Cmd) {
	settled := res.Proceed || res.Stay
	// A settle always replaces the status, so an unknown outcome (empty Status,
	// text on the sticky line) does not leave "Writing #<id>..." under it.
	if res.Status != "" || settled {
		m.statusMsg = res.Status
		m.statusErr = res.IsErr
	}
	cmds := []tea.Cmd{flowCmd}
	if settled && m.staged != nil {
		if res.Kept {
			m.statusMsg = fmt.Sprintf("#%d still staged — move on to confirm", m.staged.item.ID)
		} else {
			wrote := len(res.Batch.Reports) > 0 && res.Batch.Reports[0].Outcome == guard.OutcomeExecuted
			if wrote && ado.IsDoneState(m.staged.state) && m.priorities != nil {
				m.priorities.Remove(m.staged.item.ID)
				m.priorities.Save()
			}
			m.staged = nil
		}
		m.table = m.table.WithRows(m.buildRows())
	}
	if res.Refresh {
		cmds = append(cmds, m.fetchWorkItems())
	}
	key := m.pendingKey
	if settled {
		m.pendingKey = nil
	}
	if res.Proceed && key != nil {
		var cmd tea.Cmd
		m, cmd = m.Update(*key)
		cmds = append(cmds, cmd)
	}
	if res.Proceed && m.leaving {
		cmds = append(cmds, func() tea.Msg { return ProceedMsg{} })
	}
	if settled {
		m.leaving = false
	}
	return m, tea.Batch(cmds...)
}

// ProceedMsg tells app.go that a confirm started by ConfirmLeave settled in a
// way that lets the user leave the view.
type ProceedMsg struct{}

// WriteBusy reports whether a staged edit is previewing, awaiting confirm or
// being written.
func (m Model) WriteBusy() bool { return m.flow.Busy() }

// ConfirmLeave is what app.go calls when the user switches tabs or quits with
// an edit staged (D-02). It confirms the staged edit and yields ProceedMsg once
// leaving is safe: at once when nothing is staged, otherwise when the confirm
// settles with Proceed (y wrote, n discarded, or a no-op). esc, a failed write
// and a failed preview yield nothing. The navigation itself is app.go's to
// replay when ProceedMsg arrives, so a write is never cut off by a quit. While
// the flow is busy it does nothing; app.go holds keys then.
func (m Model) ConfirmLeave() (Model, tea.Cmd) {
	if m.staged == nil {
		return m, func() tea.Msg { return ProceedMsg{} }
	}
	if m.flow.Busy() {
		return m, nil
	}
	m.leaving = true
	cmd := m.startConfirm()
	return m, cmd
}
