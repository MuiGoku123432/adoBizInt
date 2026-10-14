package prs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/guard/guardtest"
	"sentinovo.ai/bizInt/internal/ui/confirm"
)

// loadWriteModel builds a Model whose writes go to w and loads one PR (#7 in
// project P, repository r1). The &ado.Client{} lets fetchPRs build its refresh
// cmd; tests never run that cmd. Callers set HOME before any write runs.
func loadWriteModel(t *testing.T, w *guardtest.Client) Model {
	t.Helper()
	m := New(&ado.Client{}, "https://dev.azure.com/org", []string{"P"}, nil, 0)
	m.writer = w
	m, _ = m.Update(PRsMsg{Result: ado.Result[ado.PullRequest]{
		Items:    []ado.PullRequest{{ID: 7, Title: "a pull request", Project: "P", RepositoryID: "r1"}},
		Complete: true,
	}})
	return m
}

func key(k string) tea.KeyMsg {
	if k == "esc" {
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

// feed runs cmd, requires it to yield a T, and updates m with the result.
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

// auditLines reads $HOME/.config/adoBizInt/audit.jsonl; a missing file is zero lines.
func auditLines(t *testing.T) []map[string]any {
	t.Helper()
	f, err := os.Open(filepath.Join(os.Getenv("HOME"), ".config", "adoBizInt", "audit.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer f.Close()
	var lines []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var rec map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			t.Fatalf("audit line %q: %v", scanner.Text(), err)
		}
		lines = append(lines, rec)
	}
	return lines
}

func activePR(vote int, status string) func(context.Context, string, string, int) (*ado.PullRequestDetail, error) {
	return func(_ context.Context, _, _ string, id int) (*ado.PullRequestDetail, error) {
		return &ado.PullRequestDetail{ID: id, Status: status, MyVote: vote}, nil
	}
}

func TestApproveRunsThroughGuard(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var calls []string
	w := &guardtest.Client{
		T:                        t,
		GetPullRequestDetailFunc: activePR(0, "active"),
		ApprovePullRequestFunc: func(_ context.Context, project, repo string, id int) error {
			calls = append(calls, fmt.Sprintf("%s/%s/%d", project, repo, id))
			return nil
		},
	}
	m := loadWriteModel(t, w)

	m, cmd := m.Update(key("a"))
	if !strings.Contains(m.statusMsg, "Checking PR #7") {
		t.Fatalf("status after a = %q, want it to contain %q", m.statusMsg, "Checking PR #7")
	}
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)

	view := m.View()
	for _, want := range []string{"Confirm #7", "reviewer.vote", "0 → 10", "[y] write"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() after preview lacks %q", want)
		}
	}

	m, cmd = m.Update(key("y"))
	m, cmd = feed[confirm.WriteMsg](t, m, cmd)
	m, cmd = feed[confirm.WrittenMsg](t, m, cmd)

	if len(calls) != 1 || calls[0] != "P/r1/7" {
		t.Errorf("ApprovePullRequest calls = %v, want exactly [P/r1/7]", calls)
	}
	if !strings.Contains(m.statusMsg, "#7 approved") {
		t.Errorf("status = %q, want it to contain %q", m.statusMsg, "#7 approved")
	}
	if cmd == nil {
		t.Error("no refresh cmd returned after the write")
	}
	lines := auditLines(t)
	if len(lines) != 1 {
		t.Fatalf("audit lines = %d, want 1", len(lines))
	}
	if lines[0]["command"] != "prs approve" || lines[0]["outcome"] != "executed" {
		t.Errorf("audit record = %v, want command %q outcome %q", lines[0], "prs approve", "executed")
	}
}

// pressAndPreview presses k and runs the preview cmd into m.
func pressAndPreview(t *testing.T, m Model, k string) Model {
	t.Helper()
	m, cmd := m.Update(key(k))
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)
	return m
}

func TestCompleteRunsThroughGuardAfterConfirm(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var calls []string
	w := &guardtest.Client{
		T:                        t,
		GetPullRequestDetailFunc: activePR(0, "active"),
		CompletePullRequestFunc: func(_ context.Context, project, repo string, id int) error {
			calls = append(calls, fmt.Sprintf("%s/%s/%d", project, repo, id))
			return nil
		},
	}
	m := loadWriteModel(t, w)

	m, cmd := m.Update(key("c"))
	if strings.Contains(m.statusMsg, "Merge PR") {
		t.Errorf("status after c = %q, want no status-line merge prompt", m.statusMsg)
	}
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)
	view := m.View()
	for _, want := range []string{"status", "active → completed", "[y] write"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() after preview lacks %q", want)
		}
	}

	m, cmd = m.Update(key("y"))
	m, cmd = feed[confirm.WriteMsg](t, m, cmd)
	m, _ = feed[confirm.WrittenMsg](t, m, cmd)

	if len(calls) != 1 || calls[0] != "P/r1/7" {
		t.Errorf("CompletePullRequest calls = %v, want exactly [P/r1/7]", calls)
	}
	if !strings.Contains(m.statusMsg, "#7 completed") {
		t.Errorf("status = %q, want it to contain %q", m.statusMsg, "#7 completed")
	}
	lines := auditLines(t)
	if len(lines) != 1 || lines[0]["command"] != "prs complete" || lines[0]["outcome"] != "executed" {
		t.Errorf("audit lines = %v, want one prs complete executed record", lines)
	}
}

func TestDiscardSendsNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// ApprovePullRequestFunc is nil: a call fails the test by name.
	w := &guardtest.Client{T: t, GetPullRequestDetailFunc: activePR(0, "active")}
	m := pressAndPreview(t, loadWriteModel(t, w), "a")

	m, cmd := m.Update(key("n"))
	m, _ = feed[confirm.DiscardMsg](t, m, cmd)

	if !strings.Contains(m.statusMsg, "Discarded") {
		t.Errorf("status = %q, want it to contain %q", m.statusMsg, "Discarded")
	}
	if n := len(auditLines(t)); n != 0 {
		t.Errorf("audit lines = %d, want 0", n)
	}
	if m.WriteBusy() {
		t.Error("WriteBusy() = true after discard, want false")
	}
}

func TestAlreadyApprovedIsNoChange(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	w := &guardtest.Client{T: t, GetPullRequestDetailFunc: activePR(10, "active")}
	m := pressAndPreview(t, loadWriteModel(t, w), "a")

	if want := "No change: #7 already approved"; m.statusMsg != want {
		t.Errorf("status = %q, want %q", m.statusMsg, want)
	}
	if strings.Contains(m.View(), "[y] write") {
		t.Error("View() shows the confirm modal for a no-op")
	}
}

func TestAbandonedPRCompleteIsNoChange(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// CompletePullRequestFunc is nil: a call fails the test by name.
	w := &guardtest.Client{T: t, GetPullRequestDetailFunc: activePR(0, "abandoned")}
	m := pressAndPreview(t, loadWriteModel(t, w), "c")

	if want := "No change: #7 already abandoned"; m.statusMsg != want {
		t.Errorf("status = %q, want %q", m.statusMsg, want)
	}
	if strings.Contains(m.View(), "[y] write") {
		t.Error("View() shows the confirm modal for a no-op")
	}
	if n := len(auditLines(t)); n != 0 {
		t.Errorf("audit lines = %d, want 0", n)
	}
}

func TestUnwritableSinkBlocksWrite(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".config"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	// GetPullRequestDetailFunc is nil: the sink is opened before the pre-read
	// (D-13), so a pre-read here fails the test by name.
	w := &guardtest.Client{T: t}
	m := pressAndPreview(t, loadWriteModel(t, w), "a")

	if !strings.HasPrefix(m.statusMsg, "write blocked: audit log not writable:") {
		t.Errorf("status = %q, want the write-blocked line", m.statusMsg)
	}
	if !m.statusErr {
		t.Error("statusErr = false, want true")
	}

	// Writes are not disabled for the session: the next press previews again.
	m, cmd := m.Update(key("a"))
	if cmd == nil || !strings.Contains(m.statusMsg, "Checking PR #7") {
		t.Fatalf("second a: status %q, cmd nil = %v; want a new preview", m.statusMsg, cmd == nil)
	}
	feed[confirm.PreviewedMsg](t, m, cmd)
}

func TestUnknownOutcomeIsSticky(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	w := &guardtest.Client{
		T:                        t,
		GetPullRequestDetailFunc: activePR(0, "active"),
		ApprovePullRequestFunc: func(context.Context, string, string, int) error {
			return context.DeadlineExceeded
		},
	}
	m := pressAndPreview(t, loadWriteModel(t, w), "a")
	m, cmd := m.Update(key("y"))
	m, cmd = feed[confirm.WriteMsg](t, m, cmd)
	m, cmd = feed[confirm.WrittenMsg](t, m, cmd)

	const want = "#7: outcome unknown — verify in ADO"
	if got := m.flow.Sticky(); got != want {
		t.Fatalf("Sticky() = %q, want %q", got, want)
	}
	if !strings.Contains(m.View(), want) {
		t.Error("View() lacks the sticky unknown line")
	}
	if cmd == nil {
		t.Error("no refresh cmd returned after an unknown outcome")
	}

	m, _ = m.Update(PRsMsg{Result: ado.Result[ado.PullRequest]{
		Items:    []ado.PullRequest{{ID: 7, Title: "a pull request", Project: "P", RepositoryID: "r1"}},
		Complete: true,
	}})
	if got := m.flow.Sticky(); got != want {
		t.Errorf("Sticky() after refresh = %q, want it kept", got)
	}

	m, _ = m.Update(key("j"))
	if got := m.flow.Sticky(); got != "" {
		t.Errorf("Sticky() after a keypress = %q, want cleared", got)
	}
}

func TestTickDoesNotRefreshWhileConfirmOpen(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	w := &guardtest.Client{T: t, GetPullRequestDetailFunc: activePR(0, "active")}
	m := pressAndPreview(t, loadWriteModel(t, w), "a")

	m, cmd := m.Update(TickMsg{})
	if m.loading {
		t.Error("loading = true after a tick with the confirm modal open, want no refresh")
	}
	if cmd == nil {
		t.Error("tick cmd = nil, want the tick re-armed")
	}
}

func TestTabHeldWhileWriteBusy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	w := &guardtest.Client{
		T:                        t,
		GetPullRequestDetailFunc: activePR(0, "active"),
		ApprovePullRequestFunc:   func(context.Context, string, string, int) error { return nil },
	}
	m := loadWriteModel(t, w)
	held := func(stage string) {
		t.Helper()
		if !m.IsTextInputFocused() || !m.WriteBusy() {
			t.Errorf("%s: IsTextInputFocused %v, WriteBusy %v; want both true", stage, m.IsTextInputFocused(), m.WriteBusy())
		}
	}

	m, cmd := m.Update(key("a"))
	held("preview in flight")
	m, _ = feed[confirm.PreviewedMsg](t, m, cmd)
	held("modal open")
	m, cmd = m.Update(key("y"))
	m, cmd = feed[confirm.WriteMsg](t, m, cmd)
	held("write in flight")
	m, _ = feed[confirm.WrittenMsg](t, m, cmd)
	if m.IsTextInputFocused() || m.WriteBusy() {
		t.Errorf("settled: IsTextInputFocused %v, WriteBusy %v; want both false", m.IsTextInputFocused(), m.WriteBusy())
	}
}
