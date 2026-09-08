package dashboard

import (
	"errors"
	"strings"
	"testing"

	"sentinovo.ai/bizInt/internal/ado"
)

// newTestModel builds a Model with no client. The client is only dereferenced
// inside the fetch command, which these tests never run.
func newTestModel() Model {
	return New(nil, "https://dev.azure.com/org", []string{"project-one"})
}

// countsResult wraps an aggregate in the one-element Items slice 02-03
// established as the dashboard's envelope shape.
func countsResult(counts ado.DashboardCounts, failures []ado.Failure, complete bool) ado.Result[ado.DashboardCounts] {
	return ado.Result[ado.DashboardCounts]{
		Items:    []ado.DashboardCounts{counts},
		Failures: failures,
		Complete: complete,
	}
}

// Regression: an auth failure produced a zero total that rendered as a plain
// "0" — visually identical to a genuinely empty organisation.
func TestUpdateAuthFailureBlocksTheCounts(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(CountsMsg{Result: countsResult(ado.DashboardCounts{}, []ado.Failure{{
		Project:   "project-one",
		Operation: "get dashboard work item count",
		Kind:      ado.FailureAuth,
		Err:       errors.New("TF400813: The user is not authorized"),
	}}, false)})

	if !m.authBlocked {
		t.Errorf("Update(auth failure) authBlocked = %v, want true", m.authBlocked)
	}

	view := m.View()
	if !strings.Contains(view, string(ado.FailureAuth)) {
		t.Errorf("View() did not name the failure kind:\n%s", view)
	}
	if strings.Contains(view, "Work Items") {
		t.Errorf("View() rendered the count cards under a rejected token:\n%s", view)
	}
}

// The dashboard's warning must say the totals are understated, not merely that
// something failed: naming what broke leaves the effect on the number to
// inference, which is exactly what D-03 exists to prevent (threat T-02-07).
func TestUpdatePartialFailureRendersCountsAndSaysUnderstated(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(CountsMsg{Result: countsResult(
		ado.DashboardCounts{WorkItems: 12, PRs: 3, Pipelines: 5},
		[]ado.Failure{{
			Project:   "project-two",
			Operation: "get dashboard pull request count",
			Kind:      ado.FailureTimeout,
			Err:       errors.New("deadline exceeded"),
		}}, false)})

	if m.authBlocked {
		t.Errorf("Update(timeout failure) authBlocked = %v, want false", m.authBlocked)
	}
	if m.workItemCount != 12 || m.prCount != 3 || m.pipelineCount != 5 {
		t.Errorf("Update(timeout failure) counts = %d/%d/%d, want 12/3/5 — partial counts are still shown",
			m.workItemCount, m.prCount, m.pipelineCount)
	}
	if !strings.Contains(strings.ToLower(m.partialWarning), "understated") {
		t.Errorf("partialWarning = %q, want it to state the totals are understated", m.partialWarning)
	}
	if !strings.Contains(m.partialWarning, "project-two") {
		t.Errorf("partialWarning = %q, want it to name the failed project", m.partialWarning)
	}
	if !strings.Contains(m.partialWarning, "get dashboard pull request count") {
		t.Errorf("partialWarning = %q, want it to name the failed sub-query", m.partialWarning)
	}

	view := m.View()
	if !strings.Contains(strings.ToLower(view), "understated") {
		t.Errorf("View() did not surface the understatement:\n%s", view)
	}
	if !strings.Contains(view, "Work Items") {
		t.Errorf("View() suppressed the counts on a non-auth partial:\n%s", view)
	}
}

// A failed iteration lookup drops the WIQL iteration filter, so the work-item
// count is overstated rather than missing (02-03). Wording it the same as a
// project that did not answer would train the user to ignore the line.
func TestUpdateDegradedIterationFilterIsWordedDistinctly(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(CountsMsg{Result: countsResult(
		ado.DashboardCounts{WorkItems: 40},
		[]ado.Failure{{
			Project:   "project-one",
			Operation: "get dashboard iterations",
			Kind:      ado.FailureOther,
			Err:       errors.New("classification node not found"),
		}}, false)})

	if !strings.Contains(strings.ToLower(m.partialWarning), "unfiltered") {
		t.Errorf("partialWarning = %q, want the iteration degradation described as an unfiltered query", m.partialWarning)
	}
}

// The headline states the direction of the error, so it must be derived from
// the failure mix rather than hardcoded. Asserting the wrong direction is worse
// than asserting none: it launders a wrong number as a verified one, which is
// the C1 failure mode this phase exists to remove.
func TestSummarizeCountFailuresHeadlineMatchesSkewDirection(t *testing.T) {
	degraded := ado.Failure{
		Project:   "project-one",
		Operation: opDashboardIterations,
		Kind:      ado.FailureOther,
		Err:       errors.New("classification node not found"),
	}
	unanswered := ado.Failure{
		Project:   "project-two",
		Operation: "get dashboard pull request count",
		Kind:      ado.FailureTimeout,
		Err:       errors.New("deadline exceeded"),
	}

	// Regression pin: a dropped iteration filter runs the WIQL query
	// unfiltered, returning MORE items. Calling that "understated" points the
	// user's correction in exactly the wrong direction.
	t.Run("degraded iteration lookups alone are overstated", func(t *testing.T) {
		got := strings.ToLower(summarizeCountFailures([]ado.Failure{degraded}))

		if strings.Contains(got, "understated") {
			t.Errorf("summarizeCountFailures(degraded-only) = %q, must not claim the totals are understated: a dropped iteration filter inflates the count", got)
		}
		if !strings.Contains(got, "overstated") {
			t.Errorf("summarizeCountFailures(degraded-only) = %q, want it to state the totals are overstated", got)
		}
		if !strings.Contains(got, "unfiltered") {
			t.Errorf("summarizeCountFailures(degraded-only) = %q, want the unfiltered cause retained", got)
		}
	})

	t.Run("projects that did not answer are understated", func(t *testing.T) {
		got := strings.ToLower(summarizeCountFailures([]ado.Failure{unanswered}))

		if !strings.Contains(got, "understated") {
			t.Errorf("summarizeCountFailures(unanswered-only) = %q, want it to state the totals are understated", got)
		}
		if strings.Contains(got, "overstated") {
			t.Errorf("summarizeCountFailures(unanswered-only) = %q, must not claim the totals are overstated", got)
		}
		if !strings.Contains(got, "did not answer") {
			t.Errorf("summarizeCountFailures(unanswered-only) = %q, want the unanswered cause retained", got)
		}
	})

	// Both directions are in play and they do not cancel to a knowable net.
	// Picking either one asserts a fact the tool does not have.
	t.Run("a mixed failure set asserts no direction", func(t *testing.T) {
		got := strings.ToLower(summarizeCountFailures([]ado.Failure{unanswered, degraded}))

		if strings.Contains(got, "understated") {
			t.Errorf("summarizeCountFailures(mixed) = %q, must not assert understated: degraded lookups skew the other way", got)
		}
		if strings.Contains(got, "overstated") {
			t.Errorf("summarizeCountFailures(mixed) = %q, must not assert overstated: unanswered projects skew the other way", got)
		}
		if !strings.Contains(got, "did not answer") || !strings.Contains(got, "unfiltered") {
			t.Errorf("summarizeCountFailures(mixed) = %q, want both causes retained", got)
		}
	})
}

// Regression: a zero count under an incomplete read rendered with exactly the
// authority of a correct one.
func TestZeroCountsDistinguishEmptyFromIncomplete(t *testing.T) {
	t.Run("complete zero renders the counts with no warning", func(t *testing.T) {
		m := newTestModel()
		m, _ = m.Update(CountsMsg{Result: countsResult(ado.DashboardCounts{}, nil, true)})

		if m.partialWarning != "" {
			t.Errorf("partialWarning = %q, want empty on a complete result", m.partialWarning)
		}
		if !strings.Contains(m.View(), "Work Items") {
			t.Errorf("View() suppressed the counts on a complete result:\n%s", m.View())
		}
	})

	t.Run("incomplete zero is not presented as authoritative", func(t *testing.T) {
		m := newTestModel()
		m, _ = m.Update(CountsMsg{Result: countsResult(ado.DashboardCounts{}, []ado.Failure{{
			Project:   "project-one",
			Operation: "get dashboard work item count",
			Kind:      ado.FailureRateLimit,
			Err:       errors.New("too many requests"),
		}}, false)})

		if m.complete {
			t.Errorf("complete = %v, want false", m.complete)
		}
		if m.partialWarning == "" {
			t.Fatal("partialWarning was empty: an incomplete zero rendered as an authoritative zero")
		}
		if !strings.Contains(strings.ToLower(m.View()), "understated") {
			t.Errorf("View() presented an incomplete zero without qualification:\n%s", m.View())
		}
	})
}

func TestUpdateCompleteResultClearsFailureState(t *testing.T) {
	m := newTestModel()

	m, _ = m.Update(CountsMsg{Result: countsResult(ado.DashboardCounts{}, []ado.Failure{{
		Project: "project-two", Kind: ado.FailureAuth, Err: errors.New("boom"),
	}}, false)})
	m, _ = m.Update(CountsMsg{Result: countsResult(ado.DashboardCounts{WorkItems: 1}, nil, true)})

	if m.authBlocked {
		t.Errorf("Update(complete result) authBlocked = %v, want false", m.authBlocked)
	}
	if m.partialWarning != "" {
		t.Errorf("Update(complete result) partialWarning = %q, want cleared", m.partialWarning)
	}
}
