package releases

import (
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
