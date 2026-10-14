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
// refuses a project that does not own the item (CR-01).
type stagedEdit struct {
	item  ado.WorkItem
	state string
}

// batch is the guarded write for s: one set-state op on the staged item.
func (s stagedEdit) batch(w guard.Client) confirm.Batch {
	return confirm.Batch{
		ID:     s.item.ID,
		Ops:    []guard.Op{guard.SetState(w, s.item.Project, s.item.ID, s.state, guard.Effort{})},
		Labels: []string{strings.ToLower(s.state)},
	}
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

// stage replaces any staged edit with item → state. Nothing is sent.
func (m *Model) stage(item ado.WorkItem, state string) {
	m.staged = &stagedEdit{item: item, state: state}
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
// navigation is replayed on Proceed or forgotten on Stay. Every settle that is
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
	return m, tea.Batch(cmds...)
}
