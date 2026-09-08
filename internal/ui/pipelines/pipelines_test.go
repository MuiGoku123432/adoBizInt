package pipelines

import (
	"errors"
	"strings"
	"testing"

	"sentinovo.ai/bizInt/internal/ado"
)

// newTestModel builds a Model with no client. The client is only dereferenced
// inside fetch commands, which these tests never run.
func newTestModel() Model {
	return New(nil, "https://dev.azure.com/org", []string{"project-one"}, 0)
}

func sampleRun(name, project string) ado.PipelineRun {
	return ado.PipelineRun{PipelineName: name, Project: project, BuildNumber: "1"}
}

func authFailure(project string) ado.Failure {
	return ado.Failure{
		Project:   project,
		Operation: "get pipeline definitions for runs",
		Kind:      ado.FailureAuth,
		Err:       errors.New("TF400813: The user is not authorized"),
	}
}

// Regression: an auth failure used to arrive as an empty item slice, so a
// rejected token was indistinguishable from a project with no pipeline runs —
// and the view then blamed the user's config.yaml for it.
func TestUpdateAuthFailureBlocksTheTable(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(PipelinesMsg{Result: ado.Result[ado.PipelineRun]{
		Failures: []ado.Failure{authFailure("project-one")},
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
	m := newTestModel()

	m, _ = m.Update(PipelinesMsg{Result: ado.Result[ado.PipelineRun]{
		Items: []ado.PipelineRun{sampleRun("build", "project-one")},
		Failures: []ado.Failure{{
			Project:   "project-two",
			Operation: "get pipeline definitions for runs",
			Kind:      ado.FailureTimeout,
			Err:       errors.New("deadline exceeded"),
		}},
	}})

	if m.authBlocked {
		t.Errorf("Update(timeout failure) authBlocked = %v, want false", m.authBlocked)
	}
	if len(m.runs) != 1 {
		t.Errorf("Update(timeout failure) runs length = %d, want 1 — surviving rows are retained", len(m.runs))
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
	m := newTestModel()

	m, _ = m.Update(PipelinesMsg{Result: ado.Result[ado.PipelineRun]{
		Failures: []ado.Failure{authFailure("project-two")},
	}})
	m, _ = m.Update(PipelinesMsg{Result: ado.Result[ado.PipelineRun]{
		Items:    []ado.PipelineRun{sampleRun("build", "project-one")},
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

	m, _ = m.Update(PipelinesMsg{Result: ado.Result[ado.PipelineRun]{
		Items:    []ado.PipelineRun{sampleRun("build", "project-one")},
		Complete: true,
	}})
	m, _ = m.Update(PipelinesMsg{Err: errors.New("connection refused")})

	if len(m.runs) != 1 {
		t.Errorf("Update(transport error) runs length = %d, want the last good rows retained", len(m.runs))
	}
	if !m.stale {
		t.Errorf("Update(transport error) stale = %v, want true", m.stale)
	}
	if !strings.Contains(m.View(), "stale") {
		t.Errorf("View() did not mark the retained rows stale:\n%s", m.View())
	}
}

// Regression: both the reducer's status-line message and the View() render
// branch asserted "no pipeline runs, check your config" regardless of whether
// the read had actually completed. Both sites are gated, and both are asserted
// here — gating only one leaves the misdirection the user actually reads.
func TestZeroRunsDistinguishesEmptyFromIncomplete(t *testing.T) {
	t.Run("complete and empty says no runs found", func(t *testing.T) {
		m := newTestModel()
		m, _ = m.Update(PipelinesMsg{Result: ado.Result[ado.PipelineRun]{Complete: true}})

		if m.statusMsg == "" {
			t.Errorf("statusMsg = %q, want the configured-pipelines hint on a complete empty read", m.statusMsg)
		}
		if !strings.Contains(m.View(), "No pipeline runs found") {
			t.Errorf("View() omitted the no-work message for a complete empty read:\n%s", m.View())
		}
	})

	t.Run("incomplete and empty blames nothing", func(t *testing.T) {
		m := newTestModel()
		m, _ = m.Update(PipelinesMsg{Result: ado.Result[ado.PipelineRun]{
			Failures: []ado.Failure{{
				Project:   "project-one",
				Operation: "get pipeline definitions for runs",
				Kind:      ado.FailureRateLimit,
				Err:       errors.New("too many requests"),
			}},
		}})

		// The reducer status-line site.
		if strings.Contains(m.statusMsg, "config.yaml") {
			t.Errorf("statusMsg = %q, want no configuration blame on an incomplete read", m.statusMsg)
		}
		// The View() render branch — the text the user actually reads.
		if strings.Contains(m.View(), "No pipeline runs found") {
			t.Errorf("View() presented an incomplete read as an absence of work:\n%s", m.View())
		}
	})
}

// Only the primary message drives the blocking banner. A secondary fetch failing
// auth must not blank a table the primary fetch populated correctly.
func TestSecondaryDefinitionsAuthFailureDoesNotBlock(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(PipelinesMsg{Result: ado.Result[ado.PipelineRun]{
		Items:    []ado.PipelineRun{sampleRun("build", "project-one")},
		Complete: true,
	}})
	m, _ = m.Update(DefinitionsMsg{Result: ado.Result[ado.PipelineDefinition]{
		Failures: []ado.Failure{authFailure("project-one")},
	}})

	if m.authBlocked {
		t.Errorf("Update(secondary auth failure) authBlocked = %v, want false", m.authBlocked)
	}
	if m.statusMsg == "" {
		t.Errorf("statusMsg = %q, want the existing status-line warning", m.statusMsg)
	}
}
