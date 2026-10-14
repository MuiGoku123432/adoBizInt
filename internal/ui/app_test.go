package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/guard"
	"sentinovo.ai/bizInt/internal/ui/confirm"
	"sentinovo.ai/bizInt/internal/ui/workitems"
)

// These tests drive the leave flow with crafted confirm messages in place of
// the guard's answers. The preview and execute cmds are never run: the zero
// ado.Client cannot serve them.

func update(m Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func press(m Model, k string) (Model, tea.Cmd) {
	switch k {
	case "esc":
		return update(m, tea.KeyMsg{Type: tea.KeyEsc})
	case "ctrl+c":
		return update(m, tea.KeyMsg{Type: tea.KeyCtrlC})
	}
	return update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
}

// run runs cmd and returns the messages it yields, with tea.BatchMsg
// flattened, since workitems batches its refresh with the ProceedMsg cmd.
// note: workitems/staging_test.go has the same helper; it is unexported there.
func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var msgs []tea.Msg
	for _, c := range batch {
		msgs = append(msgs, run(c)...)
	}
	return msgs
}

func yields[T tea.Msg](msgs []tea.Msg) bool {
	for _, msg := range msgs {
		if _, ok := msg.(T); ok {
			return true
		}
	}
	return false
}

// stagedApp opens Work Items with Bugs #1 and #2 Active in "P" and stages #1
// with n. NewModel gets no projects, so the work-item refresh a settled write
// batches reads nothing and returns at once when a test runs it.
func stagedApp(t *testing.T) Model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	transitions := map[string]map[string]string{"Bug": {"Active": "Resolved"}}
	m := NewModel(&ado.Client{}, "https://dev.azure.com/org", nil, nil, transitions, 0, nil)
	m, _ = press(m, "2")
	m, _ = update(m, workitems.WorkItemsMsg{Result: ado.Result[ado.WorkItem]{
		Items: []ado.WorkItem{
			{ID: 1, Title: "first", Project: "P", Type: "Bug", State: "Active"},
			{ID: 2, Title: "second", Project: "P", Type: "Bug", State: "Active"},
		},
		Complete: true,
	}})
	m, _ = press(m, "n")
	if !m.workitems.HasStaged() {
		t.Fatalf("setup: n did not stage #1")
	}
	return m
}

// batch is #1's staged state change with one report of the given outcome.
func batch(outcome string) confirm.Batch {
	return confirm.Batch{
		ID:     1,
		Ops:    []guard.Op{{Name: "workitems set-state", Target: guard.Target{Project: "P", ID: 1}}},
		Labels: []string{"resolved"},
		Reports: []guard.Report{{
			Outcome: outcome,
			Changes: []guard.FieldChange{{Field: "System.State", From: "Active", To: "Resolved"}},
		}},
	}
}

func TestQuitWithStagedEditConfirmsFirst(t *testing.T) {
	m := stagedApp(t)

	m, cmd := press(m, "q")
	if cmd == nil || !m.workitems.WriteBusy() {
		t.Fatalf("q with a staged edit: cmd nil = %v, WriteBusy = %v; want the preview started instead of a quit", cmd == nil, m.workitems.WriteBusy())
	}

	m, _ = update(m, confirm.PreviewedMsg{Batch: batch(guard.OutcomeDryRun)})
	if !strings.Contains(m.View(), "Confirm #1") {
		t.Fatalf("View() after the preview lacks %q", "Confirm #1")
	}

	m, cmd = press(m, "q")
	if yields[tea.QuitMsg](run(cmd)) {
		t.Fatalf("q with the confirm open quit the app")
	}

	m, cmd = press(m, "y")
	msgs := run(cmd)
	if len(msgs) != 1 {
		t.Fatalf("y yielded %v, want one confirm.WriteMsg", msgs)
	}
	if _, ok := msgs[0].(confirm.WriteMsg); !ok {
		t.Fatalf("y yielded %T, want confirm.WriteMsg", msgs[0])
	}
	m, _ = update(m, msgs[0]) // the Execute cmd it returns is not run

	m, cmd = update(m, confirm.WrittenMsg{Batch: batch(guard.OutcomeExecuted)})
	msgs = run(cmd)
	if !yields[workitems.ProceedMsg](msgs) {
		t.Fatalf("the settled write yielded %v, want a workitems.ProceedMsg", msgs)
	}

	_, cmd = update(m, workitems.ProceedMsg{})
	if !yields[tea.QuitMsg](run(cmd)) {
		t.Errorf("ProceedMsg did not replay the quit")
	}
}
