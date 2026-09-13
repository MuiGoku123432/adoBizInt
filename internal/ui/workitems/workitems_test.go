package workitems

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"sentinovo.ai/bizInt/internal/ado"
)

// newTestModel builds a Model with no client. The client is only dereferenced
// inside the fetch and update commands, which these tests never run; New itself
// tolerates a nil client so the reducer can be driven in isolation.
func newTestModel() Model {
	return New(nil, "https://dev.azure.com/org", []string{"project-one"}, nil, nil)
}

// Regression: an auth failure used to arrive as an empty item slice, so a
// rejected token rendered as "No work items found".
func TestUpdateAuthFailureBlocksTheTable(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(WorkItemsMsg{Result: ado.Result[ado.WorkItem]{
		Failures: []ado.Failure{{
			Project:   "project-one",
			Operation: "get work items",
			Kind:      ado.FailureAuth,
			Err:       errors.New("TF400813: The user is not authorized"),
		}},
	}})

	if !m.authBlocked {
		t.Errorf("Update(auth failure) authBlocked = %v, want true", m.authBlocked)
	}

	view := m.View()
	if strings.Contains(view, "No work items found") {
		t.Errorf("View() presented a rejected token as an absence of work:\n%s", view)
	}
	if !strings.Contains(view, string(ado.FailureAuth)) {
		t.Errorf("View() did not name the failure kind:\n%s", view)
	}
}

func TestUpdateNonAuthFailureKeepsRowsAndWarns(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(WorkItemsMsg{Result: ado.Result[ado.WorkItem]{
		Items: []ado.WorkItem{{ID: 1, Title: "a task", Project: "project-one"}},
		Failures: []ado.Failure{{
			Project:   "project-two",
			Operation: "get work items",
			Kind:      ado.FailureTimeout,
			Err:       errors.New("deadline exceeded"),
		}},
	}})

	if m.authBlocked {
		t.Errorf("Update(timeout failure) authBlocked = %v, want false", m.authBlocked)
	}
	if len(m.items) != 1 {
		t.Errorf("Update(timeout failure) items length = %d, want 1", len(m.items))
	}
	if !strings.Contains(m.partialWarning, "project-two") {
		t.Errorf("partialWarning = %q, want it to name the failed project", m.partialWarning)
	}
	if !strings.Contains(m.partialWarning, string(ado.FailureTimeout)) {
		t.Errorf("partialWarning = %q, want it to name the failure kind", m.partialWarning)
	}
	if !strings.Contains(m.View(), "project-two") {
		t.Errorf("View() did not surface the partial warning:\n%s", m.View())
	}
}

// Regression: the zero-row branch asserted an absence of work regardless of
// whether the read had actually completed.
func TestViewDistinguishesEmptyFromIncomplete(t *testing.T) {
	t.Run("complete and empty says no work items found", func(t *testing.T) {
		m := newTestModel()
		m, _ = m.Update(WorkItemsMsg{Result: ado.Result[ado.WorkItem]{Complete: true}})

		if !strings.Contains(m.View(), "No work items found") {
			t.Errorf("View() omitted the no-work message for a complete empty read:\n%s", m.View())
		}
	})

	t.Run("incomplete and empty does not say no work items found", func(t *testing.T) {
		m := newTestModel()
		m, _ = m.Update(WorkItemsMsg{Result: ado.Result[ado.WorkItem]{
			Failures: []ado.Failure{{
				Project:   "project-one",
				Operation: "get work items",
				Kind:      ado.FailureRateLimit,
				Err:       errors.New("too many requests"),
			}},
		}})

		if strings.Contains(m.View(), "No work items found") {
			t.Errorf("View() presented an incomplete read as an absence of work:\n%s", m.View())
		}
	})
}

func TestUpdateCompleteResultClearsFailureState(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(WorkItemsMsg{Result: ado.Result[ado.WorkItem]{
		Failures: []ado.Failure{{Project: "project-two", Kind: ado.FailureAuth, Err: errors.New("boom")}},
	}})
	m, _ = m.Update(WorkItemsMsg{Result: ado.Result[ado.WorkItem]{
		Items:    []ado.WorkItem{{ID: 1, Title: "a task", Project: "project-one"}},
		Complete: true,
	}})

	if m.authBlocked {
		t.Errorf("Update(complete result) authBlocked = %v, want false", m.authBlocked)
	}
	if m.partialWarning != "" {
		t.Errorf("Update(complete result) partialWarning = %q, want cleared", m.partialWarning)
	}
}

// BL-01 regression guard: both UpdateWorkItemStateWithEffort call sites must
// forward the EffortSubmitMsg pointers directly. Taking the address of a local
// copy (&originalEstimate) turns a nil "leave unchanged" into a non-nil pointer
// to a zero value, which is exactly how blank effort fields used to be
// overwritten with 0. Asserted structurally over the AST — comments are not AST
// nodes, so prose cannot defeat this guard the way it defeats a text search.
func TestUpdateWorkItemStateWithEffortCallSitesForwardPointers(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "workitems.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parser.ParseFile(workitems.go) error = %v", err)
	}

	wantFields := []string{"OriginalEstimate", "Remaining", "Completed"}
	var calls []*ast.CallExpr

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "UpdateWorkItemStateWithEffort" {
			return true
		}
		calls = append(calls, call)
		return true
	})

	if len(calls) != 2 {
		t.Fatalf("found %d UpdateWorkItemStateWithEffort call sites, want 2", len(calls))
	}

	for i, call := range calls {
		if len(call.Args) != 6 {
			t.Errorf("call site %d has %d args, want 6", i, len(call.Args))
			continue
		}
		for j, field := range wantFields {
			arg := call.Args[3+j]

			if unary, ok := arg.(*ast.UnaryExpr); ok && unary.Op == token.AND {
				t.Errorf("call site %d arg %d is an address-of expression, which reintroduces BL-01", i, 4+j)
				continue
			}

			sel, ok := arg.(*ast.SelectorExpr)
			if !ok {
				t.Errorf("call site %d arg %d is %T, want *ast.SelectorExpr on effortMsg", i, 4+j, arg)
				continue
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok || ident.Name != "effortMsg" {
				t.Errorf("call site %d arg %d selects on %v, want effortMsg", i, 4+j, sel.X)
				continue
			}
			if sel.Sel.Name != field {
				t.Errorf("call site %d arg %d = effortMsg.%s, want effortMsg.%s", i, 4+j, sel.Sel.Name, field)
			}
		}
	}
}
