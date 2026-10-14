package workitems

import (
	"context"
	"os"
	"path/filepath"
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

// run runs cmd and returns the messages it yields, with tea.BatchMsg
// flattened, since applyFlow batches the refresh with replayed cmds.
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

// stageAndPreview stages #1 with n, moves down, and applies the preview.
func stageAndPreview(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = press(m, "n")
	m, cmd := press(m, "down")
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)
	return m
}

func wantNoAudit(t *testing.T) {
	t.Helper()
	if records := guardtest.AuditRecords(t); len(records) != 0 {
		t.Errorf("audit records = %v, want none", records)
	}
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

func TestDiscardDropsStagedAndMoves(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, w, _ := newStagingModel(t)
	w.UpdateWorkItemStateFunc = nil // a write fails the test

	m = stageAndPreview(t, m)
	m, cmd := press(m, "n")
	m, _ = feed[confirm.DiscardMsg](t, m, cmd)

	wantNoAudit(t)
	if id := highlightedID(m.table); id != 2 {
		t.Errorf("highlight = #%d after discard, want #2", id)
	}
	if m.HasStaged() {
		t.Errorf("HasStaged() = true after discard")
	}
}

func TestStayKeepsStagedAndPosition(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, w, _ := newStagingModel(t)
	w.UpdateWorkItemStateFunc = nil

	m = stageAndPreview(t, m)
	m, cmd := press(m, "esc")
	m, _ = feed[confirm.StayMsg](t, m, cmd)

	if id := highlightedID(m.table); id != 1 {
		t.Errorf("highlight = #%d after esc, want #1", id)
	}
	if !m.HasStaged() {
		t.Errorf("HasStaged() = false after esc, want the edit kept")
	}
	if !strings.Contains(m.statusMsg, "still staged") {
		t.Errorf("status = %q, want it to contain %q", m.statusMsg, "still staged")
	}
	wantNoAudit(t)

	m, cmd = press(m, "down")
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)
	if !strings.Contains(m.View(), "Confirm #1") {
		t.Errorf("second down did not open the confirm again")
	}
}

func TestNoOpSkipsModalAndMoves(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, w, _ := newStagingModel(t)
	w.UpdateWorkItemStateFunc = nil
	w.GetWorkItemDetailFunc = func(_ context.Context, id int) (*ado.WorkItemDetail, error) {
		return &ado.WorkItemDetail{WorkItem: ado.WorkItem{ID: id, Project: "P", Type: "Bug", State: "Resolved"}}, nil
	}

	m = stageAndPreview(t, m)

	if m.statusMsg != "No change: #1 already resolved" {
		t.Errorf("status = %q, want %q", m.statusMsg, "No change: #1 already resolved")
	}
	if m.flow.Busy() || strings.Contains(m.View(), "Confirm #1") {
		t.Errorf("a no-op pre-read left the confirm open")
	}
	if id := highlightedID(m.table); id != 2 {
		t.Errorf("highlight = #%d after no-op, want #2", id)
	}
	if m.HasStaged() {
		t.Errorf("HasStaged() = true after no-op")
	}
	wantNoAudit(t)
}

func TestUnknownOutcomeStaysSticky(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, w, _ := newStagingModel(t)
	w.UpdateWorkItemStateFunc = func(context.Context, string, int, string) error {
		return context.DeadlineExceeded
	}

	m = stageAndPreview(t, m)
	m, cmd := press(m, "y")
	m, cmd = feed[confirm.WriteMsg](t, m, cmd)
	m, cmd = feed[confirm.WrittenMsg](t, m, cmd)

	if sticky := m.flow.Sticky(); !strings.Contains(sticky, "outcome unknown — verify in ADO") {
		t.Errorf("Sticky() = %q, want it to contain %q", sticky, "outcome unknown — verify in ADO")
	}
	if id := highlightedID(m.table); id != 1 {
		t.Errorf("highlight = #%d after unknown outcome, want #1", id)
	}
	if m.HasStaged() {
		t.Errorf("HasStaged() = true after unknown outcome")
	}
	refreshed := false
	for _, msg := range run(cmd) {
		if _, ok := msg.(WorkItemsMsg); ok {
			refreshed = true
		}
	}
	if !refreshed {
		t.Errorf("unknown outcome did not re-fetch the list")
	}
	records := guardtest.AuditRecords(t)
	if len(records) != 1 || records[0]["outcome"] != "unknown" {
		t.Errorf("audit records = %v, want one with outcome unknown", records)
	}
}

func TestUnwritableSinkDropsStagedAndStays(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".config"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	m, w, _ := newStagingModel(t)
	w.GetWorkItemDetailFunc = nil // the sink check comes before any pre-read
	w.UpdateWorkItemStateFunc = nil

	m = stageAndPreview(t, m)

	if !strings.HasPrefix(m.statusMsg, "write blocked: audit log not writable:") {
		t.Errorf("status = %q, want the D-10 sink message", m.statusMsg)
	}
	if m.HasStaged() {
		t.Errorf("HasStaged() = true after a failed preview; the user would be trapped")
	}
	if id := highlightedID(m.table); id != 1 {
		t.Errorf("highlight = #%d after a failed preview, want #1", id)
	}

	m, cmd := press(m, "down")
	for _, msg := range run(cmd) {
		if _, ok := msg.(confirm.PreviewedMsg); ok {
			t.Errorf("the next down started another preview")
		}
	}
	if id := highlightedID(m.table); id != 2 {
		t.Errorf("highlight = #%d after the next down, want #2", id)
	}
}

func TestEnterOnAnotherItemConfirmsFirst(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, w, _ := newStagingModel(t)
	w.UpdateWorkItemStateFunc = nil

	m, _ = press(m, "n")
	m, _ = press(m, "/")
	m, _ = press(m, "2")
	m, _ = press(m, "enter") // leaves the search box
	if id := highlightedID(m.table); id != 2 {
		t.Fatalf("highlight = #%d after searching for 2, want #2", id)
	}

	// Checked before the cmd runs: the detail's fetch would reach the client.
	m, cmd := press(m, "enter")
	if m.showDetail {
		t.Fatalf("enter opened #2's detail before the staged #1 was confirmed")
	}
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)
	if !m.IsTextInputFocused() {
		t.Errorf("IsTextInputFocused() = false with the confirm open; tab keys would leave the view")
	}

	m, cmd = press(m, "n")
	m, _ = feed[confirm.DiscardMsg](t, m, cmd)
	if !m.showDetail || m.detailModal == nil || m.detailModal.itemID != 2 {
		t.Errorf("replayed enter did not open #2's detail")
	}
	wantNoAudit(t)
}

func proceedCount(msgs []tea.Msg) int {
	n := 0
	for _, msg := range msgs {
		if _, ok := msg.(ProceedMsg); ok {
			n++
		}
	}
	return n
}

// stageAndLeave stages #1, asks the view to confirm before leaving, and
// applies the preview.
func stageAndLeave(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = press(m, "n")
	m, cmd := m.ConfirmLeave()
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)
	return m
}

func TestConfirmLeaveProceedsAfterWrite(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, _, writes := newStagingModel(t)

	m = stageAndLeave(t, m)
	if !m.WriteBusy() {
		t.Errorf("WriteBusy() = false with the confirm open")
	}
	m, cmd := press(m, "y")
	m, cmd = feed[confirm.WriteMsg](t, m, cmd)
	m, cmd = feed[confirm.WrittenMsg](t, m, cmd)

	if n := proceedCount(run(cmd)); n != 1 {
		t.Errorf("ProceedMsg count after the write = %d, want 1", n)
	}
	if len(*writes) != 1 {
		t.Errorf("writes = %v, want one", *writes)
	}
	if m.HasStaged() {
		t.Errorf("HasStaged() = true after the write")
	}
}

func TestConfirmLeaveEscDoesNotProceed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, w, _ := newStagingModel(t)
	w.UpdateWorkItemStateFunc = nil

	m = stageAndLeave(t, m)
	m, cmd := press(m, "esc")
	m, cmd = feed[confirm.StayMsg](t, m, cmd)

	if n := proceedCount(run(cmd)); n != 0 {
		t.Errorf("ProceedMsg count after esc = %d, want 0", n)
	}
	if !m.HasStaged() {
		t.Errorf("HasStaged() = false after esc, want the edit kept")
	}
}

func TestConfirmLeaveDiscardProceeds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, w, _ := newStagingModel(t)
	w.UpdateWorkItemStateFunc = nil // a write fails the test

	m = stageAndLeave(t, m)
	m, cmd := press(m, "n")
	_, cmd = feed[confirm.DiscardMsg](t, m, cmd)

	if n := proceedCount(run(cmd)); n != 1 {
		t.Errorf("ProceedMsg count after discard = %d, want 1", n)
	}
	wantNoAudit(t)
}

func TestConfirmLeaveWithNothingStagedProceedsImmediately(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, _, _ := newStagingModel(t)
	m.writer = &guardtest.Client{T: t} // any call fails the test

	_, cmd := m.ConfirmLeave()
	msgs := run(cmd)
	if len(msgs) != 1 || proceedCount(msgs) != 1 {
		t.Errorf("ConfirmLeave with nothing staged yielded %v, want one ProceedMsg", msgs)
	}
}

// The confirm modal must be drawn on top of an open detail modal: the flow
// takes the keys first, so a hidden confirm would swallow them unseen.
func TestConfirmLeaveShowsOverDetail(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, w, _ := newStagingModel(t)
	w.UpdateWorkItemStateFunc = nil

	m, _ = press(m, "n")
	m, _ = press(m, "enter") // the staged item's own detail; its fetch is not run
	if !m.showDetail {
		t.Fatalf("enter on the staged item did not open its detail")
	}
	m, cmd := m.ConfirmLeave()
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)
	if !strings.Contains(m.View(), "Confirm #1") {
		t.Errorf("View() hides the confirm modal behind the detail modal")
	}
}
