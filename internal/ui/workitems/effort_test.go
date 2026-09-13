package workitems

// BL-01: the effort modal used to overwrite blank effort fields with 0, because
// parseFloat("") returned (0, nil) and EffortSubmitMsg carried plain float64s.
// These tests pin blank-means-absent (nil pointer, no patch operation) and
// explicit-zero-means-zero (non-nil pointer to 0, a real write).
//
// This reverses 04-CONTEXT.md decision D-17 ("preserve TUI behaviour exactly")
// for the TUI layer only. internal/ado is unchanged and its D-17 regression
// guard, TestUpdateWorkItemStateWithEffortIssuesExactPatch, still passes.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func ptr(f float64) *float64 { return &f }

// wantPtr compares an actual *float64 against an expectation by nil-ness first,
// then by dereferenced value, so a nil pointer cannot panic the suite.
func wantPtr(t *testing.T, field string, got, want *float64) {
	t.Helper()
	switch {
	case want == nil && got != nil:
		t.Errorf("%s = ptr(%v), want nil", field, *got)
	case want != nil && got == nil:
		t.Errorf("%s = nil, want ptr(%v)", field, *want)
	case want != nil && got != nil && *got != *want:
		t.Errorf("%s = ptr(%v), want ptr(%v)", field, *got, *want)
	}
}

func TestEffortModalSubmitNullability(t *testing.T) {
	tests := []struct {
		name         string
		origEst      string
		remaining    string
		completed    string
		wantSubmit   bool
		wantOrigEst  *float64
		wantRemain   *float64
		wantComplete *float64
		wantErrPart  string
	}{
		{
			name:         "blank original estimate and remaining are absent",
			origEst:      "",
			remaining:    "",
			completed:    "4.5",
			wantSubmit:   true,
			wantOrigEst:  nil,
			wantRemain:   nil,
			wantComplete: ptr(4.5),
		},
		{
			name:         "whitespace-only original estimate is absent",
			origEst:      "   ",
			remaining:    "",
			completed:    "4.5",
			wantSubmit:   true,
			wantOrigEst:  nil,
			wantRemain:   nil,
			wantComplete: ptr(4.5),
		},
		{
			name:         "explicit zero original estimate is a real zero write",
			origEst:      "0",
			remaining:    "",
			completed:    "4.5",
			wantSubmit:   true,
			wantOrigEst:  ptr(0),
			wantRemain:   nil,
			wantComplete: ptr(4.5),
		},
		{
			name:         "all three supplied",
			origEst:      "8",
			remaining:    "3.5",
			completed:    "4.5",
			wantSubmit:   true,
			wantOrigEst:  ptr(8),
			wantRemain:   ptr(3.5),
			wantComplete: ptr(4.5),
		},
		{
			name:        "blank completed is rejected",
			origEst:     "8",
			remaining:   "3.5",
			completed:   "",
			wantSubmit:  false,
			wantErrPart: "greater than zero",
		},
		{
			name:        "invalid original estimate is rejected",
			origEst:     "abc",
			remaining:   "",
			completed:   "4.5",
			wantSubmit:  false,
			wantErrPart: "valid numbers",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := NewEffortModal(42, "Some task", "Closed")
			m.originalEstimate.SetValue(tc.origEst)
			m.remaining.SetValue(tc.remaining)
			m.completed.SetValue(tc.completed)

			m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

			if !tc.wantSubmit {
				if cmd != nil {
					if msg := cmd(); msg != nil {
						if _, ok := msg.(EffortSubmitMsg); ok {
							t.Fatalf("Update(enter) emitted EffortSubmitMsg, want rejection")
						}
					}
				}
				if tc.wantErrPart != "" && m.err == "" {
					t.Fatalf("Update(enter) err = %q, want an error naming %q", m.err, tc.wantErrPart)
				}
				if tc.wantErrPart != "" && !strings.Contains(m.err, tc.wantErrPart) {
					t.Errorf("Update(enter) err = %q, want it to name %q", m.err, tc.wantErrPart)
				}
				return
			}

			if cmd == nil {
				t.Fatalf("Update(enter) cmd = nil, want an EffortSubmitMsg command (err = %q)", m.err)
			}
			msg := cmd()
			submit, ok := msg.(EffortSubmitMsg)
			if !ok {
				t.Fatalf("Update(enter) msg = %T, want EffortSubmitMsg", msg)
			}

			wantPtr(t, "OriginalEstimate", submit.OriginalEstimate, tc.wantOrigEst)
			wantPtr(t, "Remaining", submit.Remaining, tc.wantRemain)
			wantPtr(t, "Completed", submit.Completed, tc.wantComplete)
		})
	}
}

func TestEffortModalParseEffort(t *testing.T) {
	m := NewEffortModal(1, "t", "Closed")

	tests := []struct {
		in      string
		want    *float64
		wantErr bool
	}{
		{in: "", want: nil},
		{in: "   ", want: nil},
		{in: "0", want: ptr(0)},
		{in: "8.5", want: ptr(8.5)},
		{in: "abc", want: nil, wantErr: true},
	}

	for _, tc := range tests {
		got, err := m.parseEffort(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseEffort(%q) err = nil, want an error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseEffort(%q) err = %v, want nil", tc.in, err)
			continue
		}
		wantPtr(t, "parseEffort("+tc.in+")", got, tc.want)
	}
}
