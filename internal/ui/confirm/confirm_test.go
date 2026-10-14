package confirm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sentinovo.ai/bizInt/internal/guard"
)

func TestModalOnlyYWrites(t *testing.T) {
	m := New("Confirm #1", []string{"status  active → completed"})
	cases := []struct {
		name string
		key  tea.KeyMsg
		want tea.Msg
	}{
		{"y", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}, WriteMsg{}},
		{"n", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}, DiscardMsg{}},
		{"esc", tea.KeyMsg{Type: tea.KeyEsc}, StayMsg{}},
		{"enter", tea.KeyMsg{Type: tea.KeyEnter}, nil},
		{"space", tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}, nil},
		{"Y", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Y")}, nil},
	}
	for _, tc := range cases {
		_, cmd := m.Update(tc.key)
		if tc.want == nil {
			if cmd != nil {
				t.Errorf("%s: cmd yields %T, want no cmd", tc.name, cmd())
			}
			continue
		}
		if cmd == nil {
			t.Errorf("%s: cmd = nil, want %T", tc.name, tc.want)
			continue
		}
		if got := cmd(); got != tc.want {
			t.Errorf("%s: cmd yields %T, want %T", tc.name, got, tc.want)
		}
	}
}

func TestModalDropsControlRunes(t *testing.T) {
	view := New("Confirm #1", []string{"System.Title  a → \x1b]52;c;ZXZpbA==\x07b"}).View()
	if strings.ContainsAny(view, "\x1b\x07") {
		t.Errorf("View() carries an ESC or BEL byte: %q", view)
	}
}

func TestModalFitsSixtyRuneLineOnOneRow(t *testing.T) {
	long := "#11 Microsoft.VSTS.Scheduling.CompletedWork  (none) → 4"
	sixty := strings.Repeat("x", 60)
	view := New("Confirm #10", []string{long, sixty}).View()

	for _, line := range []string{long, sixty} {
		rows := 0
		for _, row := range strings.Split(view, "\n") {
			if strings.Contains(row, line) {
				rows++
			}
		}
		if rows != 1 {
			t.Errorf("%d rows contain the whole %d-rune line %q, want 1:\n%s", rows, len([]rune(line)), line, view)
		}
	}
	if w := lipgloss.Width(view); w != 66 {
		t.Errorf("lipgloss.Width(View()) = %d, want 66 (Width(64) plus the border)", w)
	}
}

func TestBatchLinesDedupeAndPrefix(t *testing.T) {
	state := guard.FieldChange{Field: guard.FieldState, From: "Active", To: "Closed"}
	remaining := guard.FieldChange{Field: guard.FieldRemainingWork, From: "3", To: "0"}
	completed := guard.FieldChange{Field: guard.FieldCompletedWork, From: "", To: "4"}
	b := Batch{
		ID: 10,
		Ops: []guard.Op{
			{Target: guard.Target{ID: 10}},
			{Target: guard.Target{ID: 10}},
			{Target: guard.Target{ID: 11}},
		},
		Reports: []guard.Report{
			{Changes: []guard.FieldChange{state}},
			{Changes: []guard.FieldChange{state, remaining}},
			{Changes: []guard.FieldChange{completed}},
		},
	}
	want := []string{
		"System.State  Active → Closed",
		"Microsoft.VSTS.Scheduling.RemainingWork  3 → 0",
		"#11 Microsoft.VSTS.Scheduling.CompletedWork  (none) → 4",
	}
	if got := b.Lines(); !reflect.DeepEqual(got, want) {
		t.Errorf("Lines() = %q, want %q", got, want)
	}
}

func TestExecuteRefusesChangedDiff(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	remaining := "3"
	op := guard.Op{
		Name:   "workitems set-state",
		Target: guard.Target{Project: "P", ID: 5},
		Plan: func(context.Context) ([]guard.FieldChange, error) {
			return []guard.FieldChange{{Field: guard.FieldRemainingWork, From: remaining, To: "0"}}, nil
		},
		Mutate: func(context.Context) error {
			t.Fatal("Mutate called for a diff the user never saw")
			return nil
		},
	}
	b := Preview(Batch{ID: 5, Ops: []guard.Op{op}, Labels: []string{"resolved"}}, time.Second)

	remaining = "5" // changed in ADO between the preview and y
	got := Execute(b, time.Second)

	if len(got.Reports) != 1 || got.Reports[0].Outcome != guard.OutcomeDryRun {
		t.Fatalf("Execute reports = %+v, want one dry_run", got.Reports)
	}
	data, err := os.ReadFile(filepath.Join(home, ".config", "adoBizInt", "audit.jsonl"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), "\n"); n != 0 {
		t.Errorf("audit lines = %d, want 0", n)
	}
}

func TestFlowWrittenTextReportsEachStep(t *testing.T) {
	boom := errors.New("boom")
	two := []guard.Op{{}, {}}
	labels := []string{"closed", "effort update"}

	written := func(t *testing.T, b Batch) (Flow, Result) {
		t.Helper()
		f, _, res := Flow{busy: true}.Update(WrittenMsg{Batch: b})
		return f, res
	}

	t.Run("single op unknown", func(t *testing.T) {
		f, res := written(t, Batch{ID: 7, Ops: []guard.Op{{}}, Labels: []string{"approved"},
			Reports: []guard.Report{{Outcome: guard.OutcomeUnknown}}})
		if got, want := f.Sticky(), "#7: outcome unknown — verify in ADO"; got != want {
			t.Errorf("Sticky() = %q, want %q", got, want)
		}
		if res.Status != "" || !res.IsErr {
			t.Errorf("Status %q IsErr %v, want empty status and IsErr", res.Status, res.IsErr)
		}
	})

	cases := []struct {
		name    string
		reports []guard.Report
		want    string
		refresh bool
	}{
		{"executed then failed",
			[]guard.Report{{Outcome: guard.OutcomeExecuted}, {Outcome: guard.OutcomeFailed, MutateErr: boom}},
			"#1 closed; effort update FAILED: boom", true},
		{"failed then no report",
			[]guard.Report{{Outcome: guard.OutcomeFailed, MutateErr: boom}},
			"#1: write FAILED: boom; effort update not attempted", true},
		{"dry_run first",
			[]guard.Report{{Outcome: guard.OutcomeDryRun}},
			"#1: not written — changed since preview; press the action key again to re-check; effort update not attempted", false},
		{"pre-read error first",
			[]guard.Report{{PreReadErr: boom}},
			"#1: could not read before writing: boom; effort update not attempted", false},
		{"executed then pre-read error",
			[]guard.Report{{Outcome: guard.OutcomeExecuted}, {PreReadErr: boom}},
			"#1 closed; effort update not written: could not read before writing: boom", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, res := written(t, Batch{ID: 1, Ops: two, Labels: labels, Reports: tc.reports})
			if res.Status != tc.want {
				t.Errorf("Status = %q, want %q", res.Status, tc.want)
			}
			if !res.IsErr || !res.Stay || res.Proceed {
				t.Errorf("IsErr %v Stay %v Proceed %v, want IsErr and Stay", res.IsErr, res.Stay, res.Proceed)
			}
			if res.Refresh != tc.refresh {
				t.Errorf("Refresh = %v, want %v", res.Refresh, tc.refresh)
			}
			if f.Busy() || f.Sticky() != "" {
				t.Errorf("Busy %v Sticky %q, want settled with no sticky", f.Busy(), f.Sticky())
			}
		})
	}
}
