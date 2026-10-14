package pipelines

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/guard/guardtest"
	"sentinovo.ai/bizInt/internal/ui/confirm"
)

// note: prs/write_test.go and releases/write_test.go have the same helper over
// their own Model types.
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

// submitAndPreview builds a Model whose writes go to w, submits the trigger
// modal for definition #9 on main, and runs the pre-read into the diff modal.
// The &ado.Client{} lets the refresh cmd be built; tests never run it.
func submitAndPreview(t *testing.T, w *guardtest.Client) Model {
	t.Helper()
	m := New(&ado.Client{}, "https://dev.azure.com/org", []string{"P"}, 0)
	m.writer = w

	m, cmd := m.Update(TriggerSubmitMsg{DefinitionID: 9, PipelineName: "build", Branch: "main", Project: "P"})
	if !strings.Contains(m.statusMsg, "Checking build on main") {
		t.Fatalf("status after submit = %q, want it to contain %q", m.statusMsg, "Checking build on main")
	}
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)

	view := m.View()
	for _, want := range []string{"Confirm #9", "queued_build", "(none) → main", "[y] write"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() after preview lacks %q", want)
		}
	}
	return m
}

func definition(_ context.Context, _ string, id int) (*ado.PipelineDefinitionDetail, error) {
	return &ado.PipelineDefinitionDetail{ID: id}, nil
}

func TestQueueSubmitPreviewsThenConfirms(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var calls []string
	w := &guardtest.Client{
		T:                               t,
		GetPipelineDefinitionDetailFunc: definition,
		QueuePipelineFunc: func(_ context.Context, project string, id int, branch string) (*ado.PipelineRun, error) {
			calls = append(calls, fmt.Sprintf("%s/%d/%s", project, id, branch))
			return &ado.PipelineRun{}, nil
		},
	}
	m := submitAndPreview(t, w)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m, cmd = feed[confirm.WriteMsg](t, m, cmd)
	m, cmd = feed[confirm.WrittenMsg](t, m, cmd)

	if len(calls) != 1 || calls[0] != "P/9/main" {
		t.Errorf("QueuePipeline calls = %v, want exactly [P/9/main]", calls)
	}
	if !strings.Contains(m.statusMsg, "#9 queued on main") {
		t.Errorf("status = %q, want it to contain %q", m.statusMsg, "#9 queued on main")
	}
	if cmd == nil {
		t.Error("no refresh cmd returned after the write")
	}
	records := guardtest.AuditRecords(t)
	if len(records) != 1 {
		t.Fatalf("audit records = %d, want 1", len(records))
	}
	rec := records[0]
	if rec["command"] != "pipelines queue" || rec["outcome"] != "executed" || rec["actor"] != "tui" || rec["user_approved"] != false {
		t.Errorf("audit record = %v, want command %q, outcome executed, actor tui, user_approved false", rec, "pipelines queue")
	}
}

func TestQueueEscSendsNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// QueuePipelineFunc is nil: a call fails the test by name.
	w := &guardtest.Client{T: t, GetPipelineDefinitionDetailFunc: definition}
	m := submitAndPreview(t, w)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m, _ = feed[confirm.StayMsg](t, m, cmd)

	if !strings.Contains(m.statusMsg, "Not written: #9") {
		t.Errorf("status = %q, want it to contain %q", m.statusMsg, "Not written: #9")
	}
	if n := len(guardtest.AuditRecords(t)); n != 0 {
		t.Errorf("audit records = %d, want 0", n)
	}
	if m.WriteBusy() {
		t.Error("WriteBusy() = true after esc, want false")
	}
}
