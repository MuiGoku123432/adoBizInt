package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/guard"
	"sentinovo.ai/bizInt/internal/ui/confirm"
	"sentinovo.ai/bizInt/internal/ui/prs"
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

// leaveAndPreview stages #1, presses the leave key k, and applies the crafted
// preview, so the confirm modal is open.
func leaveAndPreview(t *testing.T, k string) Model {
	t.Helper()
	m := stagedApp(t)
	m, _ = press(m, k)
	m, _ = update(m, confirm.PreviewedMsg{Batch: batch(guard.OutcomeDryRun)})
	if !strings.Contains(m.View(), "Confirm #1") {
		t.Fatalf("setup: View() after %s and the preview lacks %q", k, "Confirm #1")
	}
	return m
}

// answer presses k on the open modal, applies the one message it yields, and
// returns the flattened messages of the cmd that follows. Not for y, whose
// follow-up cmd is the Execute the tests never run.
func answer(t *testing.T, m Model, k string) (Model, []tea.Msg) {
	t.Helper()
	m, cmd := press(m, k)
	msgs := run(cmd)
	if len(msgs) != 1 {
		t.Fatalf("%s on the modal yielded %v, want one message", k, msgs)
	}
	m, cmd = update(m, msgs[0])
	return m, run(cmd)
}

func TestTabSwitchWithStagedEditDiscardProceeds(t *testing.T) {
	m := leaveAndPreview(t, "3")
	if m.view != WorkItemsView {
		t.Fatalf("view = %v with the confirm open, want WorkItemsView", m.view)
	}

	m, msgs := answer(t, m, "n")
	if !yields[workitems.ProceedMsg](msgs) {
		t.Fatalf("n on the modal yielded %v, want a workitems.ProceedMsg", msgs)
	}
	m, _ = update(m, workitems.ProceedMsg{}) // prs Init is not run
	if m.view != PRsView {
		t.Errorf("view = %v after ProceedMsg, want PRsView", m.view)
	}
	if m.workitems.HasStaged() {
		t.Errorf("HasStaged() = true after n discarded the edit")
	}
}

func TestEscKeepsViewAndStagedEdit(t *testing.T) {
	m := leaveAndPreview(t, "3")

	m, msgs := answer(t, m, "esc")
	if yields[workitems.ProceedMsg](msgs) {
		t.Errorf("esc on the modal yielded a ProceedMsg")
	}
	if m.view != WorkItemsView {
		t.Errorf("view = %v after esc, want WorkItemsView", m.view)
	}
	if !m.workitems.HasStaged() {
		t.Errorf("HasStaged() = false after esc, want the edit kept")
	}
}

func TestUnknownOutcomeCancelsQuit(t *testing.T) {
	m := leaveAndPreview(t, "q")

	m, cmd := press(m, "y")
	msgs := run(cmd)
	if len(msgs) != 1 {
		t.Fatalf("y yielded %v, want one confirm.WriteMsg", msgs)
	}
	m, _ = update(m, msgs[0]) // the Execute cmd it returns is not run

	m, cmd = update(m, confirm.WrittenMsg{Batch: batch(guard.OutcomeUnknown)})
	msgs = run(cmd)
	if yields[workitems.ProceedMsg](msgs) || yields[tea.QuitMsg](msgs) {
		t.Errorf("an unknown outcome yielded %v, want neither ProceedMsg nor QuitMsg", msgs)
	}
	if want := "outcome unknown — verify in ADO"; !strings.Contains(m.View(), want) {
		t.Errorf("View() after an unknown outcome lacks %q", want)
	}
	if m.view != WorkItemsView {
		t.Errorf("view = %v after an unknown outcome, want WorkItemsView", m.view)
	}
}

func TestQuitHeldWhileWriteBusy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := NewModel(&ado.Client{}, "https://dev.azure.com/org", nil, nil, nil, 0, nil)
	m, _ = press(m, "3") // the prs Init cmd is not run
	m, _ = update(m, prs.PRsMsg{Result: ado.Result[ado.PullRequest]{
		Items:    []ado.PullRequest{{ID: 7, Title: "a pull request", Project: "P", RepositoryID: "r1"}},
		Complete: true,
	}})
	m, _ = press(m, "a") // the preview cmd is not run
	if !m.prs.WriteBusy() {
		t.Fatalf("setup: a did not start the PR approve flow")
	}

	for _, k := range []string{"q", "ctrl+c"} {
		var cmd tea.Cmd
		m, cmd = press(m, k)
		if yields[tea.QuitMsg](run(cmd)) {
			t.Errorf("%s quit while the PR write was busy", k)
		}
	}
}

func TestPlainQuitAndTabUnchanged(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := NewModel(&ado.Client{}, "https://dev.azure.com/org", nil, nil, nil, 0, nil)

	_, cmd := press(m, "q")
	if !yields[tea.QuitMsg](run(cmd)) {
		t.Errorf("q on the dashboard with nothing staged did not quit")
	}

	m, _ = press(m, "2") // the workitems Init cmd is not run
	if m.view != WorkItemsView {
		t.Errorf("view = %v after 2, want WorkItemsView", m.view)
	}
}
