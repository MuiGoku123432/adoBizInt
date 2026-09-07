package prs

import (
	"errors"
	"strings"
	"testing"

	"sentinovo.ai/bizInt/internal/ado"
)

// newTestModel builds a Model with no client. The client is only dereferenced
// inside fetch commands, which these tests never run.
func newTestModel() Model {
	return New(nil, "https://dev.azure.com/org", []string{"project-one"}, nil, 0)
}

func samplePR(id int, project string) ado.PullRequest {
	return ado.PullRequest{ID: id, Title: "a pull request", Project: project}
}

// Regression: an auth failure used to arrive as an empty item slice, so a
// rejected token was indistinguishable from a project with no open PRs.
func TestUpdateAuthFailureBlocksTheTable(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(PRsMsg{Result: ado.Result[ado.PullRequest]{
		Failures: []ado.Failure{{
			Project:   "project-one",
			Operation: "get pull requests",
			Kind:      ado.FailureAuth,
			Err:       errors.New("TF400813: The user is not authorized"),
		}},
	}})

	if !m.authBlocked {
		t.Errorf("Update(auth failure) authBlocked = %v, want true", m.authBlocked)
	}

	view := m.View()
	if strings.Contains(view, "No open pull requests found") {
		t.Errorf("View() presented a rejected token as an absence of work:\n%s", view)
	}
	if !strings.Contains(view, string(ado.FailureAuth)) {
		t.Errorf("View() did not name the failure kind:\n%s", view)
	}
}

func TestUpdateNonAuthFailureKeepsRowsAndWarns(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(PRsMsg{Result: ado.Result[ado.PullRequest]{
		Items: []ado.PullRequest{samplePR(1, "project-one")},
		Failures: []ado.Failure{{
			Project:   "project-two",
			Operation: "get pull requests",
			Kind:      ado.FailureTimeout,
			Err:       errors.New("deadline exceeded"),
		}},
	}})

	if m.authBlocked {
		t.Errorf("Update(timeout failure) authBlocked = %v, want false", m.authBlocked)
	}
	if len(m.items) != 1 {
		t.Errorf("Update(timeout failure) items length = %d, want 1 — surviving rows are retained", len(m.items))
	}
	if m.partialWarning == "" {
		t.Fatalf("Update(timeout failure) partialWarning = %q, want a persistent warning", m.partialWarning)
	}
	if !strings.Contains(m.partialWarning, "project-two") {
		t.Errorf("Update() partialWarning = %q, want it to name the failed project", m.partialWarning)
	}
	if !strings.Contains(m.partialWarning, string(ado.FailureTimeout)) {
		t.Errorf("Update() partialWarning = %q, want it to name the failure kind", m.partialWarning)
	}
	if !strings.Contains(m.View(), "project-two") {
		t.Errorf("View() did not surface the partial warning:\n%s", m.View())
	}
}

func TestUpdateCompleteResultClearsFailureState(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(PRsMsg{Result: ado.Result[ado.PullRequest]{
		Failures: []ado.Failure{{Project: "project-two", Kind: ado.FailureAuth, Err: errors.New("boom")}},
	}})
	m, _ = m.Update(PRsMsg{Result: ado.Result[ado.PullRequest]{
		Items:    []ado.PullRequest{samplePR(1, "project-one")},
		Complete: true,
	}})

	if m.authBlocked {
		t.Errorf("Update(complete result) authBlocked = %v, want false", m.authBlocked)
	}
	if m.partialWarning != "" {
		t.Errorf("Update(complete result) partialWarning = %q, want cleared", m.partialWarning)
	}
	if m.stale {
		t.Errorf("Update(complete result) stale = %v, want false", m.stale)
	}
}

func TestUpdateTransportErrorMarksPreviousRowsStale(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(PRsMsg{Result: ado.Result[ado.PullRequest]{
		Items:    []ado.PullRequest{samplePR(1, "project-one")},
		Complete: true,
	}})
	m, _ = m.Update(PRsMsg{Err: errors.New("connection refused")})

	if len(m.items) != 1 {
		t.Errorf("Update(transport error) items length = %d, want the last good rows retained", len(m.items))
	}
	if !m.stale {
		t.Errorf("Update(transport error) stale = %v, want true", m.stale)
	}
}

// Regression: the zero-item branch asserted an absence of work regardless of
// whether the read had actually completed.
func TestViewDistinguishesEmptyFromIncomplete(t *testing.T) {
	t.Run("complete and empty says no work found", func(t *testing.T) {
		m := newTestModel()
		m, _ = m.Update(PRsMsg{Result: ado.Result[ado.PullRequest]{Complete: true}})

		if !strings.Contains(m.View(), "No open pull requests found") {
			t.Errorf("View() omitted the no-work message for a complete empty read:\n%s", m.View())
		}
	})

	t.Run("incomplete and empty does not say no work found", func(t *testing.T) {
		m := newTestModel()
		m, _ = m.Update(PRsMsg{Result: ado.Result[ado.PullRequest]{
			Failures: []ado.Failure{{
				Project:   "project-one",
				Operation: "get pull requests",
				Kind:      ado.FailureRateLimit,
				Err:       errors.New("too many requests"),
			}},
		}})

		if strings.Contains(m.View(), "No open pull requests found") {
			t.Errorf("View() presented an incomplete read as an absence of work:\n%s", m.View())
		}
	})
}
