package workitems

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/guard/guardtest"
	"sentinovo.ai/bizInt/internal/ui/confirm"
)

type stateWrite struct {
	project string
	id      int
	state   string
}

// newStagingModel loads Bug #1 and Bug #2, both Active in project "P", and
// points writes at a guardtest.Client backed by an in-memory state map. The
// pre-read reports the map; a write records its arguments and updates the map.
// Callers set HOME first and may replace either func on the returned client.
//
// New gets no projects so the refresh cmd, when a test runs it, reads nothing
// and returns at once: &ado.Client{} has no transport behind it.
func newStagingModel(t *testing.T) (Model, *guardtest.Client, *[]stateWrite) {
	t.Helper()
	transitions := map[string]map[string]string{"Bug": {"Active": "Resolved"}}
	m := New(&ado.Client{}, "https://dev.azure.com/org", nil, transitions, nil)
	m.assignedToMeFilter = false
	m, _ = m.Update(WorkItemsMsg{Result: ado.Result[ado.WorkItem]{
		Items: []ado.WorkItem{
			{ID: 1, Title: "first", Project: "P", Type: "Bug", State: "Active"},
			{ID: 2, Title: "second", Project: "P", Type: "Bug", State: "Active"},
		},
		Complete: true,
	}})

	states := map[int]string{1: "Active", 2: "Active"}
	var writes []stateWrite
	w := &guardtest.Client{
		T: t,
		GetWorkItemDetailFunc: func(_ context.Context, id int) (*ado.WorkItemDetail, error) {
			return &ado.WorkItemDetail{WorkItem: ado.WorkItem{ID: id, Project: "P", Type: "Bug", State: states[id]}}, nil
		},
		UpdateWorkItemStateFunc: func(_ context.Context, project string, id int, state string) error {
			writes = append(writes, stateWrite{project, id, state})
			states[id] = state
			return nil
		},
	}
	m.writer = w
	return m, w, &writes
}

func press(m Model, key string) (Model, tea.Cmd) {
	switch key {
	case "down":
		return m.Update(tea.KeyMsg{Type: tea.KeyDown})
	case "enter":
		return m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	case "esc":
		return m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	}
	return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

// feed runs cmd, requires it to yield a T, and updates m with the result.
// note: prs, releases and pipelines keep the same helper over their own Model
// types; it is typed on each package's Model, so it is not shared.
func feed[T tea.Msg](t *testing.T, m Model, cmd tea.Cmd) (Model, tea.Cmd) {
	t.Helper()
	var want T
	if cmd == nil {
		t.Fatalf("cmd = nil, want one yielding %T", want)
	}
	msg := cmd()
	if _, ok := msg.(T); !ok {
		t.Fatalf("cmd yielded %T, want %T", msg, want)
	}
	return m.Update(msg)
}

func actionsCell(t *testing.T, m Model, id string) string {
	t.Helper()
	for _, row := range m.buildRows() {
		if row.Data[columnKeyID] == id {
			cell, _ := row.Data[columnKeyActions].(string)
			return cell
		}
	}
	t.Fatalf("no row for #%s", id)
	return ""
}

func TestStagedStateConfirmsOnRowMove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, _, writes := newStagingModel(t)

	m, cmd := press(m, "n")
	if cmd != nil {
		t.Fatalf("n returned a cmd; staging must send nothing")
	}
	if len(*writes) != 0 || !m.HasStaged() {
		t.Fatalf("after n: writes = %v, HasStaged = %v; want no write and a staged edit", *writes, m.HasStaged())
	}
	if cell := actionsCell(t, m, "1"); !strings.Contains(cell, "staged → Resolved") {
		t.Errorf("Actions cell for #1 = %q, want it to contain %q", cell, "staged → Resolved")
	}

	m, cmd = press(m, "down")
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)
	view := m.View()
	for _, want := range []string{"Confirm #1", "System.State  Active → Resolved"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() after preview lacks %q", want)
		}
	}
	if id := highlightedID(m.table); id != 1 {
		t.Fatalf("highlight moved to #%d before the confirm was answered", id)
	}

	m, cmd = press(m, "y")
	m, cmd = feed[confirm.WriteMsg](t, m, cmd)
	m, _ = feed[confirm.WrittenMsg](t, m, cmd)

	if len(*writes) != 1 || (*writes)[0] != (stateWrite{"P", 1, "Resolved"}) {
		t.Errorf("writes = %v, want exactly [{P 1 Resolved}]", *writes)
	}
	if id := highlightedID(m.table); id != 2 {
		t.Errorf("highlight = #%d after y, want #2", id)
	}
	if m.HasStaged() {
		t.Errorf("HasStaged() = true after the write")
	}

	records := guardtest.AuditRecords(t)
	if len(records) != 1 {
		t.Fatalf("audit records = %d, want 1", len(records))
	}
	rec := records[0]
	if rec["command"] != "workitems set-state" || rec["actor"] != "tui" || rec["user_approved"] != false {
		t.Errorf("audit record = %v, want command workitems set-state, actor tui, user_approved false", rec)
	}
	changes, _ := rec["changes"].([]any)
	if len(changes) != 1 {
		t.Fatalf("audit changes = %v, want one", rec["changes"])
	}
	change, _ := changes[0].(map[string]any)
	if change["field"] != "System.State" || change["from"] != "Active" || change["to"] != "Resolved" {
		t.Errorf("audit change = %v, want System.State Active → Resolved", change)
	}
}
