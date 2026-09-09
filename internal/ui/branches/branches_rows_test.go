package branches

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/evertras/bubble-table/table"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/utils"
)

// branchesFixture is the fixed slice every buildRows() case below runs against.
// One item per searchable field, with casing that differs from the queries so
// the case-insensitive rule is exercised in both directions.
func branchesFixture() []ado.Branch {
	return []ado.Branch{
		{Name: "feature/alpha", RepositoryName: "api", LastCommitBy: "Ann", AheadCount: 1, BehindCount: 2},
		{Name: "main", RepositoryName: "WEB", LastCommitBy: "Bob", IsDefault: true},
		{Name: "hotfix/gamma", RepositoryName: "docs", LastCommitBy: "Cid", AheadCount: 3},
	}
}

// wantRow mirrors the row construction in buildRows() for a fixture item at its
// original index. The index matters: buildRows() carries the position in
// m.items, not the position in the filtered output, and a search that drops an
// earlier item must not renumber the survivors.
//
// The comparison is on RowData rather than table.Row because table.NewRow
// stamps each row with a process-global incrementing id that is not part of the
// rendered output.
func wantRow(item ado.Branch, index int) table.RowData {
	lastCommit := "-"
	if !item.LastCommitDate.IsZero() {
		lastCommit = item.LastCommitDate.Format("2006-01-02 15:04")
	}
	nameDisplay := utils.Truncate(item.Name, 28)
	actions := "[o]pen [p]r"
	if item.IsDefault {
		nameDisplay = utils.Truncate(item.Name, 18) + " (default)"
		actions = "[o]pen"
	}
	return table.RowData{
		columnKeyRepo:        utils.Truncate(item.RepositoryName, 20),
		columnKeyBranch:      nameDisplay,
		columnKeyLastCommit:  lastCommit,
		columnKeyAuthor:      utils.Truncate(item.LastCommitBy, 12),
		columnKeyAheadBehind: fmt.Sprintf("%d / %d", item.AheadCount, item.BehindCount),
		columnKeyActions:     actions,
		columnKeyIndex:       index,
	}
}

// rowData projects the ordered rows onto their data, preserving order.
func rowData(rows []table.Row) []table.RowData {
	out := make([]table.RowData, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Data)
	}
	return out
}

// TestBuildRowsSearchEquivalence pins the ordered row output of buildRows()
// across the promotion of the free-text search comparison into
// ado.MatchesSearch. It asserts the full ordered slice, not a length or a set.
func TestBuildRowsSearchEquivalence(t *testing.T) {
	items := branchesFixture()

	tests := []struct {
		name   string
		search string
		want   []table.RowData
	}{
		{
			name: "no search keeps everything in order",
			want: []table.RowData{wantRow(items[0], 0), wantRow(items[1], 1), wantRow(items[2], 2)},
		},
		{
			name:   "a whitespace-only search keeps everything",
			search: "   ",
			want:   []table.RowData{wantRow(items[0], 0), wantRow(items[1], 1), wantRow(items[2], 2)},
		},
		{
			name:   "search matches the branch name",
			search: "alpha",
			want:   []table.RowData{wantRow(items[0], 0)},
		},
		{
			name:   "search matches the repository name case-insensitively",
			search: "web",
			want:   []table.RowData{wantRow(items[1], 1)},
		},
		{
			name:   "search matches the last committer",
			search: "cid",
			want:   []table.RowData{wantRow(items[2], 2)},
		},
		{
			name:   "an upper-case query matches a lower-case field",
			search: "HOTFIX",
			want:   []table.RowData{wantRow(items[2], 2)},
		},
		{
			name:   "a non-matching search keeps nothing",
			search: "zzz",
			want:   []table.RowData{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := textinput.New()
			ti.SetValue(tt.search)
			m := Model{items: branchesFixture(), searchInput: ti}
			got := rowData(m.buildRows())
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildRows() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBuildRowsPreservesOriginalIndex pins the one mechanical difference this
// view has from the other four: it ranges with `for i, item :=` and carries i
// onto every row. A filtered-out earlier item must not shift the survivors'
// indices, or the row-to-item mapping silently opens the wrong branch.
func TestBuildRowsPreservesOriginalIndex(t *testing.T) {
	ti := textinput.New()
	ti.SetValue("gamma") // drops items 0 and 1
	m := Model{items: branchesFixture(), searchInput: ti}

	rows := m.buildRows()
	if len(rows) != 1 {
		t.Fatalf("buildRows() returned %d rows, want 1", len(rows))
	}
	if got := rows[0].Data[columnKeyIndex]; got != 2 {
		t.Errorf("row index = %v, want 2 (the position in m.items, not in the filtered output)", got)
	}
}
