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
