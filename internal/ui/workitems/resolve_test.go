package workitems

// Compound resolve (GUARD-03, GUARD-05, D-09): the effort modal stages, the
// confirm modal lists every field change once, and y runs the state write then
// the effort write, each through the guard.
//
// TestUpdateWorkItemStateWithEffortCallSitesForwardPointers is retired. It
// counted the two direct UpdateWorkItemStateWithEffort call sites in
// workitems.go and checked that each forwarded the EffortSubmitMsg pointers,
// but GUARD-02 forbids those very call sites, so it could no longer be
// satisfied. Its BL-01 guarantee (a blank field stays nil) is now asserted end
// to end by TestTaskResolveIsTwoGuardedWrites, from the modal's keys to the fake
// client's arguments: any layer that turns nil into a pointer to zero fails it.

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/microsoft/azure-devops-go-api/azuredevops"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/guard/guardtest"
	"sentinovo.ai/bizInt/internal/ui/confirm"
)

// writeCall is one mutation the fake client received, in order.
type writeCall struct {
	method        string
	project       string
	id            int
	state         string
	oe, rem, comp *float64
}

// newResolveModel loads items and points writes at a guardtest.Client whose
// pre-read reports details (by ID). Both write funcs append to one call log and
// update details, so a later pre-read sees the earlier write.
func newResolveModel(t *testing.T, transitions map[string]map[string]string, items []ado.WorkItem, details map[int]*ado.WorkItemDetail) (Model, *guardtest.Client, *[]writeCall) {
	t.Helper()
	m := New(&ado.Client{}, "https://dev.azure.com/org", nil, transitions, nil)
	m.assignedToMeFilter = false
	m, _ = m.Update(WorkItemsMsg{Result: ado.Result[ado.WorkItem]{Items: items, Complete: true}})

	var calls []writeCall
	w := &guardtest.Client{
		T: t,
		GetWorkItemDetailFunc: func(_ context.Context, id int) (*ado.WorkItemDetail, error) {
			d := *details[id]
			return &d, nil
		},
		UpdateWorkItemStateFunc: func(_ context.Context, project string, id int, state string) error {
			calls = append(calls, writeCall{method: "UpdateWorkItemState", project: project, id: id, state: state})
			details[id].State = state
			return nil
		},
		UpdateWorkItemStateWithEffortFunc: func(_ context.Context, project string, id int, state string, oe, rem, comp *float64) error {
			calls = append(calls, writeCall{"UpdateWorkItemStateWithEffort", project, id, state, oe, rem, comp})
			d := details[id]
			d.State = state
			if oe != nil {
				d.OriginalEstimate = oe
			}
			if rem != nil {
				d.RemainingWork = rem
			}
			if comp != nil {
				d.CompletedWork = comp
			}
			return nil
		},
	}
	m.writer = w
	return m, w, &calls
}

// newTaskResolveModel loads Task #5 (original estimate 8, remaining 3, no
// completed work) and Bug #6, both Active in project "P".
func newTaskResolveModel(t *testing.T) (Model, *guardtest.Client, *[]writeCall) {
	t.Helper()
	transitions := map[string]map[string]string{"Task": {"Active": "Closed"}, "Bug": {"Active": "Resolved"}}
	items := []ado.WorkItem{
		{ID: 5, Title: "task", Project: "P", Type: "Task", State: "Active"},
		{ID: 6, Title: "bug", Project: "P", Type: "Bug", State: "Active"},
	}
	details := map[int]*ado.WorkItemDetail{
		5: {WorkItem: ado.WorkItem{ID: 5, Project: "P", Type: "Task", State: "Active"}, OriginalEstimate: ptr(8), RemainingWork: ptr(3)},
		6: {WorkItem: ado.WorkItem{ID: 6, Project: "P", Type: "Bug", State: "Active"}},
	}
	return newResolveModel(t, transitions, items, details)
}

// submitEffort fills the open effort modal (blank fields are skipped with tab)
// and applies the EffortSubmitMsg it yields.
func submitEffort(t *testing.T, m Model, origEst, remaining, completed string) (Model, tea.Cmd) {
	t.Helper()
	if !m.showEffort {
		t.Fatalf("effort modal is not open")
	}
	for i, value := range []string{origEst, remaining, completed} {
		if i > 0 {
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
		}
		if value != "" {
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)})
		}
	}
	m, cmd := press(m, "enter")
	return feed[EffortSubmitMsg](t, m, cmd)
}

// auditChanges renders a record's changes as "<field> <from>→<to>".
func auditChanges(rec map[string]any) []string {
	var out []string
	changes, _ := rec["changes"].([]any)
	for _, c := range changes {
		change, _ := c.(map[string]any)
		out = append(out, fmt.Sprintf("%v %v→%v", change["field"], change["from"], change["to"]))
	}
	return out
}

func TestTaskResolveIsTwoGuardedWrites(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, _, calls := newTaskResolveModel(t)

	m, _ = press(m, "n")
	if !m.showEffort {
		t.Fatalf("n on Active Task #5 did not open the effort modal")
	}
	if len(*calls) != 0 {
		t.Fatalf("n wrote %v; it must only open the effort modal", *calls)
	}

	m, cmd := submitEffort(t, m, "", "0", "2")
	if cmd != nil || len(*calls) != 0 {
		t.Fatalf("effort submit: cmd = %v, calls = %v; staging must send nothing", cmd != nil, *calls)
	}
	if !m.HasStaged() {
		t.Fatalf("HasStaged() = false after the effort submit, want the resolve staged")
	}

	m, cmd = press(m, "down")
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)
	view := m.View()
	for _, want := range []string{
		"System.State  Active → Closed",
		"Microsoft.VSTS.Scheduling.RemainingWork  3 → 0",
		"Microsoft.VSTS.Scheduling.CompletedWork  (none) → 2",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("preview lacks %q", want)
		}
	}
	if strings.Contains(view, "OriginalEstimate") {
		t.Errorf("preview lists OriginalEstimate, which the user left blank")
	}

	m, cmd = press(m, "y")
	m, cmd = feed[confirm.WriteMsg](t, m, cmd)
	m, _ = feed[confirm.WrittenMsg](t, m, cmd)

	if len(*calls) != 2 {
		t.Fatalf("calls = %v, want UpdateWorkItemState then UpdateWorkItemStateWithEffort", *calls)
	}
	first, second := (*calls)[0], (*calls)[1]
	if first.method != "UpdateWorkItemState" || first.project != "P" || first.id != 5 || first.state != "Closed" {
		t.Errorf("first call = %+v, want UpdateWorkItemState(P, 5, Closed)", first)
	}
	if second.method != "UpdateWorkItemStateWithEffort" || second.project != "P" || second.id != 5 || second.state != "Closed" {
		t.Errorf("second call = %+v, want UpdateWorkItemStateWithEffort(P, 5, Closed, ...)", second)
	}
	wantPtr(t, "originalEstimate", second.oe, nil)
	wantPtr(t, "remaining", second.rem, ptr(0))
	wantPtr(t, "completed", second.comp, ptr(2))

	records := guardtest.AuditRecords(t)
	if len(records) != 2 {
		t.Fatalf("audit records = %d, want 2", len(records))
	}
	wantChanges := [][]string{
		{"System.State Active→Closed"},
		{"Microsoft.VSTS.Scheduling.RemainingWork 3→0", "Microsoft.VSTS.Scheduling.CompletedWork →2"},
	}
	for i, rec := range records {
		if got := auditChanges(rec); !slices.Equal(got, wantChanges[i]) {
			t.Errorf("audit record %d changes = %v, want %v", i, got, wantChanges[i])
		}
		if rec["actor"] != "tui" || rec["user_approved"] != false {
			t.Errorf("audit record %d = %v, want actor tui, user_approved false", i, rec)
		}
	}

	if !strings.Contains(m.statusMsg, "#5 closed; effort update") {
		t.Errorf("status = %q, want it to contain %q", m.statusMsg, "#5 closed; effort update")
	}
	if id := highlightedID(m.table); id != 6 {
		t.Errorf("highlight = #%d after y, want #6", id)
	}
}

// previewTaskResolve stages Task #5's resolve with remaining 0 and completed 2
// and opens the confirm modal.
func previewTaskResolve(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = press(m, "n")
	m, _ = submitEffort(t, m, "", "0", "2")
	m, cmd := press(m, "down")
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)
	return m
}

func TestResolveEffortFailureReportsBoth(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, w, calls := newTaskResolveModel(t)
	w.UpdateWorkItemStateWithEffortFunc = func(_ context.Context, project string, id int, state string, oe, rem, comp *float64) error {
		*calls = append(*calls, writeCall{"UpdateWorkItemStateWithEffort", project, id, state, oe, rem, comp})
		status, message := 409, "conflict"
		return azuredevops.WrappedError{Message: &message, StatusCode: &status}
	}

	m = previewTaskResolve(t, m)
	m, cmd := press(m, "y")
	m, cmd = feed[confirm.WriteMsg](t, m, cmd)
	m, cmd = feed[confirm.WrittenMsg](t, m, cmd)

	var methods []string
	for _, c := range *calls {
		methods = append(methods, c.method)
	}
	if !slices.Equal(methods, []string{"UpdateWorkItemState", "UpdateWorkItemStateWithEffort"}) {
		t.Errorf("calls = %v, want one state write then one effort write and no compensation", methods)
	}
	for _, want := range []string{"#5 closed", "effort update FAILED"} {
		if !strings.Contains(m.statusMsg, want) {
			t.Errorf("status = %q, want it to contain %q", m.statusMsg, want)
		}
	}
	if !m.statusErr {
		t.Errorf("statusErr = false after a failed effort write")
	}
	refreshed := false
	for _, msg := range run(cmd) {
		if _, ok := msg.(WorkItemsMsg); ok {
			refreshed = true
		}
	}
	if !refreshed {
		t.Errorf("no refresh after a partially applied resolve")
	}
	if id := highlightedID(m.table); id != 5 {
		t.Errorf("highlight = #%d after a failed effort write, want #5", id)
	}
	if records := guardtest.AuditRecords(t); len(records) != 1 {
		t.Errorf("audit records = %d, want 1 (the executed state write)", len(records))
	}
}

// newStoryResolveModel loads User Story #10 and Bug #12, both Active in "P";
// the story's development task #11 lives in devTaskProject.
func newStoryResolveModel(t *testing.T, devTaskProject string) (Model, *guardtest.Client, *[]writeCall) {
	t.Helper()
	transitions := map[string]map[string]string{"User Story": {"Active": "Resolved"}}
	items := []ado.WorkItem{
		{ID: 10, Title: "story", Project: "P", Type: "User Story", State: "Active"},
		{ID: 12, Title: "bug", Project: "P", Type: "Bug", State: "Active"},
	}
	details := map[int]*ado.WorkItemDetail{
		10: {WorkItem: ado.WorkItem{ID: 10, Project: "P", Type: "User Story", State: "Active"}},
		11: {WorkItem: ado.WorkItem{ID: 11, Project: devTaskProject, Type: "Task", State: "Active"}},
		12: {WorkItem: ado.WorkItem{ID: 12, Project: "P", Type: "Bug", State: "Active"}},
	}
	return newResolveModel(t, transitions, items, details)
}

// stageStoryResolve presses n on story #10, answers the children fetch with
// children (the fetch cmd itself is not run), enters completed hours 4 and
// submits.
func stageStoryResolve(t *testing.T, m Model, children []ado.WorkItem) Model {
	t.Helper()
	m, _ = press(m, "n")
	m, _ = m.Update(ChildTasksLoadedMsg{ParentID: 10, ParentTitle: "story", NewState: "Resolved", Children: children})
	m, cmd := submitEffort(t, m, "", "", "4")
	if cmd != nil || !m.HasStaged() {
		t.Fatalf("effort submit: cmd = %v, HasStaged = %v; want the resolve staged and nothing sent", cmd != nil, m.HasStaged())
	}
	return m
}

var devTask = []ado.WorkItem{{ID: 11, Type: "Task", Title: "Development", State: "Active"}}

func TestUserStoryResolveClosesDevTaskThenRecordsEffort(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, _, calls := newStoryResolveModel(t, "P")

	m = stageStoryResolve(t, m, devTask)
	m, cmd := press(m, "down")
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)
	view := m.View()
	for _, want := range []string{
		"System.State  Active → Resolved",
		"#11 System.State  Active → Closed",
		"#11 Microsoft.VSTS.Scheduling.CompletedWork  (none) → 4",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("preview lacks %q", want)
		}
	}

	m, cmd = press(m, "y")
	m, cmd = feed[confirm.WriteMsg](t, m, cmd)
	m, _ = feed[confirm.WrittenMsg](t, m, cmd)

	if len(*calls) != 3 {
		t.Fatalf("calls = %+v, want story state, task close, task effort", *calls)
	}
	wantCalls := []writeCall{
		{method: "UpdateWorkItemState", project: "P", id: 10, state: "Resolved"},
		{method: "UpdateWorkItemState", project: "P", id: 11, state: "Closed"},
		{method: "UpdateWorkItemStateWithEffort", project: "P", id: 11, state: "Closed"},
	}
	for i, want := range wantCalls {
		got := (*calls)[i]
		if got.method != want.method || got.project != want.project || got.id != want.id || got.state != want.state {
			t.Errorf("call %d = %+v, want %s(%s, %d, %s)", i, got, want.method, want.project, want.id, want.state)
		}
	}
	effort := (*calls)[2]
	wantPtr(t, "originalEstimate", effort.oe, nil)
	wantPtr(t, "remaining", effort.rem, nil)
	wantPtr(t, "completed", effort.comp, ptr(4))

	records := guardtest.AuditRecords(t)
	if len(records) != 3 {
		t.Fatalf("audit records = %d, want 3", len(records))
	}
	for i, rec := range records {
		if rec["actor"] != "tui" {
			t.Errorf("audit record %d actor = %v, want tui", i, rec["actor"])
		}
	}

	t.Run("dev task in another project", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		m, w, calls := newStoryResolveModel(t, "Q")
		w.UpdateWorkItemStateFunc = nil // any write fails the test
		w.UpdateWorkItemStateWithEffortFunc = nil

		m = stageStoryResolve(t, m, devTask)
		m, cmd := press(m, "down")
		m, _ = feed[confirm.PreviewedMsg](t, m, cmd)

		for _, want := range []string{"could not read before writing", "belongs to project"} {
			if !strings.Contains(m.statusMsg, want) {
				t.Errorf("status = %q, want it to contain %q", m.statusMsg, want)
			}
		}
		if len(*calls) != 0 || m.HasStaged() {
			t.Errorf("calls = %v, HasStaged = %v; want nothing sent and nothing staged", *calls, m.HasStaged())
		}
		wantNoAudit(t)
	})
}

func TestUserStoryResolveWithoutDevTaskWritesStateOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m, w, calls := newStoryResolveModel(t, "P")
	w.UpdateWorkItemStateWithEffortFunc = nil // an effort write fails the test

	m = stageStoryResolve(t, m, nil)
	m, cmd := press(m, "down")
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)
	if view := m.View(); strings.Contains(view, "#11") || strings.Contains(view, "CompletedWork") {
		t.Errorf("preview names the development task or effort with no dev task found")
	}
	m, cmd = press(m, "y")
	m, cmd = feed[confirm.WriteMsg](t, m, cmd)
	m, _ = feed[confirm.WrittenMsg](t, m, cmd)

	if len(*calls) != 1 || (*calls)[0] != (writeCall{method: "UpdateWorkItemState", project: "P", id: 10, state: "Resolved"}) {
		t.Errorf("calls = %+v, want only UpdateWorkItemState(P, 10, Resolved)", *calls)
	}
	if m.statusMsg != "#10 resolved" {
		t.Errorf("status = %q, want %q (a one-op batch)", m.statusMsg, "#10 resolved")
	}
}
