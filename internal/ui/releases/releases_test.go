package releases

import (
	"errors"
	"strings"
	"testing"

	"sentinovo.ai/bizInt/internal/ado"
)

// newTestModel builds a Model backed by the given releases. The client is never
// dereferenced when building rows or resolving the selection, so nil is fine.
func newTestModel(releases []ado.Release) Model {
	m := New(nil, "https://dev.azure.com/org", nil, 0)
	m.releases = releases
	m.table = m.table.WithRows(m.buildRows())
	return m
}

// Regression: the selection used to be resolved by matching the truncated
// display name, so releases sharing a prefix all resolved to the first one.
func TestGetSelectedReleaseDistinguishesSharedNamePrefix(t *testing.T) {
	m := newTestModel([]ado.Release{
		{ID: 101, Name: "Deploy to Production 2024-01-15"},
		{ID: 102, Name: "Deploy to Production 2024-02-20"},
	})

	for row, wantID := range []int{101, 102} {
		m.table = m.table.WithHighlightedRow(row)

		got := m.getSelectedRelease()
		if got == nil {
			t.Fatalf("row %d: got nil, want release %d", row, wantID)
		}
		if got.ID != wantID {
			t.Errorf("row %d: got release %d, want %d", row, got.ID, wantID)
		}
	}
}

// Regression: releases with no name render a styled "-" placeholder, which never
// matched the truncated empty name, making those rows impossible to select.
func TestGetSelectedReleaseHandlesUnnamedRelease(t *testing.T) {
	m := newTestModel([]ado.Release{{ID: 7, Status: "noReleases"}})
	m.table = m.table.WithHighlightedRow(0)

	got := m.getSelectedRelease()
	if got == nil {
		t.Fatal("got nil, want release 7")
	}
	if got.ID != 7 {
		t.Errorf("got release %d, want 7", got.ID)
	}
}

func releasesAuthFailure(project string) ado.Failure {
	return ado.Failure{
		Project:   project,
		Operation: "get release definitions",
		Kind:      ado.FailureAuth,
		Err:       errors.New("TF400813: The user is not authorized"),
	}
}

// Regression: an auth failure used to arrive as an empty item slice, so a
// rejected token rendered as "No releases found. Configure pipelines in
// config.yaml" — blaming the user's configuration for a credential problem.
func TestUpdateAuthFailureBlocksTheTable(t *testing.T) {
	m := New(nil, "https://dev.azure.com/org", []string{"project-one"}, 0)

	m, _ = m.Update(ReleasesMsg{Result: ado.Result[ado.Release]{
		Failures: []ado.Failure{releasesAuthFailure("project-one")},
	}})

	if !m.authBlocked {
		t.Errorf("Update(auth failure) authBlocked = %v, want true", m.authBlocked)
	}

	view := m.View()
	if strings.Contains(view, "Configure pipelines in config.yaml") {
		t.Errorf("View() blamed the user's configuration for a rejected token:\n%s", view)
	}
	if !strings.Contains(view, string(ado.FailureAuth)) {
		t.Errorf("View() did not name the failure kind:\n%s", view)
	}
}

func TestUpdateNonAuthFailureKeepsRowsAndWarns(t *testing.T) {
	m := New(nil, "https://dev.azure.com/org", []string{"project-one"}, 0)

	m, _ = m.Update(ReleasesMsg{Result: ado.Result[ado.Release]{
		Items: []ado.Release{{ID: 1, Name: "Deploy"}},
		Failures: []ado.Failure{{
			Project:   "project-two",
			Operation: "get release definitions",
			Kind:      ado.FailureTimeout,
			Err:       errors.New("deadline exceeded"),
		}},
	}})

	if m.authBlocked {
		t.Errorf("Update(timeout failure) authBlocked = %v, want false", m.authBlocked)
	}
	if len(m.releases) != 1 {
		t.Errorf("Update(timeout failure) releases length = %d, want 1", len(m.releases))
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

func TestUpdateCompleteResultClearsFailureState(t *testing.T) {
	m := New(nil, "https://dev.azure.com/org", []string{"project-one"}, 0)

	m, _ = m.Update(ReleasesMsg{Result: ado.Result[ado.Release]{
		Failures: []ado.Failure{releasesAuthFailure("project-two")},
	}})
	m, _ = m.Update(ReleasesMsg{Result: ado.Result[ado.Release]{
		Items:    []ado.Release{{ID: 1, Name: "Deploy"}},
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
	m := New(nil, "https://dev.azure.com/org", []string{"project-one"}, 0)

	m, _ = m.Update(ReleasesMsg{Result: ado.Result[ado.Release]{
		Items:    []ado.Release{{ID: 1, Name: "Deploy"}},
		Complete: true,
	}})
	m, _ = m.Update(ReleasesMsg{Err: errors.New("connection refused")})

	if len(m.releases) != 1 {
		t.Errorf("Update(transport error) releases length = %d, want the last good rows retained", len(m.releases))
	}
	if !m.stale {
		t.Errorf("Update(transport error) stale = %v, want true", m.stale)
	}
}

// Regression: the zero-row render branch asserted "no releases, configure your
// pipelines" regardless of whether the read had actually completed.
func TestZeroReleasesDistinguishesEmptyFromIncomplete(t *testing.T) {
	t.Run("complete and empty says no releases found", func(t *testing.T) {
		m := New(nil, "https://dev.azure.com/org", []string{"project-one"}, 0)
		m, _ = m.Update(ReleasesMsg{Result: ado.Result[ado.Release]{Complete: true}})

		if !strings.Contains(m.View(), "No releases found") {
			t.Errorf("View() omitted the no-work message for a complete empty read:\n%s", m.View())
		}
	})

	t.Run("incomplete and empty blames nothing", func(t *testing.T) {
		m := New(nil, "https://dev.azure.com/org", []string{"project-one"}, 0)
		m, _ = m.Update(ReleasesMsg{Result: ado.Result[ado.Release]{
			Failures: []ado.Failure{{
				Project:   "project-one",
				Operation: "get release definitions",
				Kind:      ado.FailureRateLimit,
				Err:       errors.New("too many requests"),
			}},
		}})

		if strings.Contains(m.View(), "No releases found") {
			t.Errorf("View() presented an incomplete read as an absence of work:\n%s", m.View())
		}
	})
}

// Only the primary message drives the blocking banner.
func TestSecondaryApprovalsAuthFailureDoesNotBlock(t *testing.T) {
	m := New(nil, "https://dev.azure.com/org", []string{"project-one"}, 0)

	m, _ = m.Update(ReleasesMsg{Result: ado.Result[ado.Release]{
		Items:    []ado.Release{{ID: 1, Name: "Deploy"}},
		Complete: true,
	}})
	m, _ = m.Update(ApprovalsMsg{Result: ado.Result[ado.ReleaseApproval]{
		Failures: []ado.Failure{{
			Project:   "project-one",
			Operation: "get pending approvals",
			Kind:      ado.FailureAuth,
			Err:       errors.New("unauthorized"),
		}},
	}})

	if m.authBlocked {
		t.Errorf("Update(secondary auth failure) authBlocked = %v, want false", m.authBlocked)
	}
	if m.statusMsg == "" {
		t.Errorf("statusMsg = %q, want the existing status-line warning", m.statusMsg)
	}
}
