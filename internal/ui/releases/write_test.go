package releases

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/guard"
	"sentinovo.ai/bizInt/internal/guard/guardtest"
	"sentinovo.ai/bizInt/internal/ui/confirm"
)

// newWriteModel builds a Model whose writes go to w. The &ado.Client{} lets the
// refresh cmds be built; tests never run them. Callers set HOME first.
func newWriteModel(w *guardtest.Client) Model {
	m := New(&ado.Client{}, "https://dev.azure.com/org", []string{"P"}, 0)
	m.writer = w
	return m
}

// feed runs cmd, requires it to yield a T, and updates m with the result.
// note: prs/write_test.go has the same helper over its own Model type.
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

func approvalStatus(status string) func(context.Context, string, int) (*ado.ReleaseApprovalDetail, error) {
	return func(_ context.Context, _ string, id int) (*ado.ReleaseApprovalDetail, error) {
		return &ado.ReleaseApprovalDetail{ID: id, Status: status}, nil
	}
}

// submitAndConfirm submits the approval modal for #42, checks the pre-read
// status and the diff modal, then presses y and runs the write.
func submitAndConfirm(t *testing.T, m Model, action, wantTo string) (Model, tea.Cmd) {
	t.Helper()
	m, cmd := m.Update(ApprovalSubmitMsg{ApprovalID: 42, Project: "P", Action: action, Comment: "ship it"})
	if !strings.Contains(m.statusMsg, "Checking approval #42") {
		t.Fatalf("status after submit = %q, want it to contain %q", m.statusMsg, "Checking approval #42")
	}
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)

	view := m.View()
	for _, want := range []string{"Confirm #42", "approval.status", "pending → " + wantTo, "[y] write"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() after preview lacks %q", want)
		}
	}

	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m, cmd = feed[confirm.WriteMsg](t, m, cmd)
	return feed[confirm.WrittenMsg](t, m, cmd)
}

// checkRecord asserts one executed record for command, written by the TUI
// without --yes, and that the release comment never reached the audit file.
func checkRecord(t *testing.T, command string) {
	t.Helper()
	records := guardtest.AuditRecords(t)
	if len(records) != 1 {
		t.Fatalf("audit records = %d, want 1", len(records))
	}
	rec := records[0]
	if rec["command"] != command || rec["outcome"] != "executed" || rec["actor"] != "tui" || rec["user_approved"] != false {
		t.Errorf("audit record = %v, want command %q, outcome executed, actor tui, user_approved false", rec, command)
	}
	raw, err := os.ReadFile(guard.DefaultAuditPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "ship it") {
		t.Errorf("audit file carries the release comment: %s", raw)
	}
}

func TestApproveSubmitPreviewsThenConfirms(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var calls []string
	w := &guardtest.Client{
		T:                            t,
		GetReleaseApprovalDetailFunc: approvalStatus("pending"),
		ApproveReleaseFunc: func(_ context.Context, project string, id int, comment string) error {
			calls = append(calls, fmt.Sprintf("%s/%d/%s", project, id, comment))
			return nil
		},
	}

	m, cmd := submitAndConfirm(t, newWriteModel(w), "approve", "approved")

	if len(calls) != 1 || calls[0] != "P/42/ship it" {
		t.Errorf("ApproveRelease calls = %v, want exactly [P/42/ship it]", calls)
	}
	if !strings.Contains(m.statusMsg, "#42 approved") {
		t.Errorf("status = %q, want it to contain %q", m.statusMsg, "#42 approved")
	}
	if cmd == nil {
		t.Error("no refresh cmd returned after the write")
	}
	checkRecord(t, "releases approve")
}

func TestRejectUsesRejectAfterConfirm(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var calls []string
	// ApproveReleaseFunc is nil: a call fails the test by name.
	w := &guardtest.Client{
		T:                            t,
		GetReleaseApprovalDetailFunc: approvalStatus("pending"),
		RejectReleaseFunc: func(_ context.Context, project string, id int, comment string) error {
			calls = append(calls, fmt.Sprintf("%s/%d/%s", project, id, comment))
			return nil
		},
	}

	m, _ := submitAndConfirm(t, newWriteModel(w), "reject", "rejected")

	if len(calls) != 1 || calls[0] != "P/42/ship it" {
		t.Errorf("RejectRelease calls = %v, want exactly [P/42/ship it]", calls)
	}
	if !strings.Contains(m.statusMsg, "#42 rejected") {
		t.Errorf("status = %q, want it to contain %q", m.statusMsg, "#42 rejected")
	}
	checkRecord(t, "releases reject")
}

func TestAlreadyApprovedReleaseIsNoChange(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// ApproveReleaseFunc is nil: a call fails the test by name.
	w := &guardtest.Client{T: t, GetReleaseApprovalDetailFunc: approvalStatus("approved")}
	m := newWriteModel(w)

	m, cmd := m.Update(ApprovalSubmitMsg{ApprovalID: 42, Project: "P", Action: "approve", Comment: "ship it"})
	if !strings.Contains(m.statusMsg, "Checking approval #42") {
		t.Fatalf("status after submit = %q, want it to contain %q", m.statusMsg, "Checking approval #42")
	}
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)

	if want := "No change: #42 already approved"; m.statusMsg != want {
		t.Errorf("status = %q, want %q", m.statusMsg, want)
	}
	if strings.Contains(m.View(), "[y] write") {
		t.Error("View() shows the confirm modal for a no-op")
	}
	if m.WriteBusy() {
		t.Error("WriteBusy() = true after a no-op, want false")
	}
	if n := len(guardtest.AuditRecords(t)); n != 0 {
		t.Errorf("audit records = %d, want 0", n)
	}
}
