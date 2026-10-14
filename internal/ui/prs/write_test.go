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
