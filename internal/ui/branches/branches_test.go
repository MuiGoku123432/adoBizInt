package branches

import (
	"errors"
	"strings"
	"testing"

	"sentinovo.ai/bizInt/internal/ado"
)

// newTestModel builds a Model with no client. The client is only dereferenced
// inside the fetch command, which these tests never run.
func newTestModel() Model {
	return New(nil, "https://dev.azure.com/org", []string{"project-one"}, nil)
}

// Regression: an auth failure used to arrive as an empty item slice, so a
// rejected token rendered as "No branches found".
func TestUpdateAuthFailureBlocksTheTable(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(BranchesMsg{Result: ado.Result[ado.Branch]{
		Failures: []ado.Failure{{
			Project:   "project-one",
			Operation: "get repositories",
			Kind:      ado.FailureAuth,
			Err:       errors.New("TF400813: The user is not authorized"),
		}},
	}})

	if !m.authBlocked {
		t.Errorf("Update(auth failure) authBlocked = %v, want true", m.authBlocked)
	}

	view := m.View()
	if strings.Contains(view, "No branches found") {
		t.Errorf("View() presented a rejected token as an absence of work:\n%s", view)
	}
	if !strings.Contains(view, string(ado.FailureAuth)) {
		t.Errorf("View() did not name the failure kind:\n%s", view)
	}
}

func TestUpdateNonAuthFailureKeepsRowsAndWarns(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(BranchesMsg{Result: ado.Result[ado.Branch]{
		Items: []ado.Branch{{Name: "main", RepositoryName: "repo-a", IsDefault: true}},
		Failures: []ado.Failure{{
			Project:   "project-two",
			Operation: "get branch stats",
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
}

// 02-02 encodes the repository name in the branch-stats operation label. Without
// it, a user with many repositories cannot tell which one is broken.
func TestPerRepositoryFailureNamesTheRepositoryInTheWarning(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(BranchesMsg{Result: ado.Result[ado.Branch]{
		Items: []ado.Branch{{Name: "main", RepositoryName: "repo-a"}},
		Failures: []ado.Failure{{
			Project:   "project-one",
			Operation: "get branch stats for repo-b",
			Kind:      ado.FailureOther,
			Err:       errors.New("server error"),
		}},
	}})

	if !strings.Contains(m.partialWarning, "repo-b") {
		t.Errorf("partialWarning = %q, want it to name the failing repository", m.partialWarning)
	}
	if !strings.Contains(m.View(), "repo-b") {
		t.Errorf("View() did not surface the failing repository:\n%s", m.View())
	}
}

// Regression: the zero-row branch asserted an absence of work regardless of
// whether the read had actually completed.
func TestViewDistinguishesEmptyFromIncomplete(t *testing.T) {
	t.Run("complete and empty says no branches found", func(t *testing.T) {
		m := newTestModel()
		m, _ = m.Update(BranchesMsg{Result: ado.Result[ado.Branch]{Complete: true}})

		if !strings.Contains(m.View(), "No branches found") {
			t.Errorf("View() omitted the no-work message for a complete empty read:\n%s", m.View())
		}
	})

	t.Run("incomplete and empty does not say no branches found", func(t *testing.T) {
		m := newTestModel()
		m, _ = m.Update(BranchesMsg{Result: ado.Result[ado.Branch]{
			Failures: []ado.Failure{{
				Project:   "project-one",
				Operation: "get repositories",
				Kind:      ado.FailureRateLimit,
				Err:       errors.New("too many requests"),
			}},
		}})

		if strings.Contains(m.View(), "No branches found") {
			t.Errorf("View() presented an incomplete read as an absence of work:\n%s", m.View())
		}
	})
}

func TestUpdateCompleteResultClearsFailureState(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(BranchesMsg{Result: ado.Result[ado.Branch]{
		Failures: []ado.Failure{{Project: "project-two", Kind: ado.FailureAuth, Err: errors.New("boom")}},
	}})
	m, _ = m.Update(BranchesMsg{Result: ado.Result[ado.Branch]{
		Items:    []ado.Branch{{Name: "main", RepositoryName: "repo-a"}},
		Complete: true,
	}})

	if m.authBlocked {
		t.Errorf("Update(complete result) authBlocked = %v, want false", m.authBlocked)
	}
	if m.partialWarning != "" {
		t.Errorf("Update(complete result) partialWarning = %q, want cleared", m.partialWarning)
	}
}
