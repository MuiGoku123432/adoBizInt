package prs

import (
	"reflect"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/evertras/bubble-table/table"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/utils"
)

// prsFixture is the fixed slice every buildRows() case below runs against.
// Two projects and three repositories, with casing that differs from the
// configured values so the case-insensitive membership rule is exercised.
func prsFixture() []ado.PullRequest {
	return []ado.PullRequest{
		{ID: 1, Title: "Alpha", Project: "Alpha", RepositoryName: "api", SourceBranch: "f/a", TargetBranch: "main", CreatedBy: "Ann", ApprovalStatus: "Approved"},
		{ID: 2, Title: "Beta", Project: "ALPHA", RepositoryName: "WEB", SourceBranch: "f/b", TargetBranch: "main", CreatedBy: "Bob", ApprovalStatus: "Waiting"},
		{ID: 3, Title: "Gamma", Project: "Beta", RepositoryName: "docs", SourceBranch: "f/c", TargetBranch: "main", CreatedBy: "Cid", ApprovalStatus: "Rejected"},
	}
}

// wantRow mirrors the row construction in buildRows() for a fixture item. The
// comparison is on RowData rather than table.Row because table.NewRow stamps
// each row with a process-global incrementing id that is not part of the
// rendered output.
func wantRow(item ado.PullRequest) table.RowData {
	titleDisplay := utils.Truncate(item.Title, 32)
	if item.IsDraft {
		titleDisplay = "[Draft] " + utils.Truncate(item.Title, 24)
	}
	return table.RowData{
		columnKeyID:      itoa(item.ID),
		columnKeyTitle:   titleDisplay,
		columnKeyProject: utils.Truncate(item.Project, 10),
		columnKeyRepo:    utils.Truncate(item.RepositoryName, 13),
		columnKeySource:  utils.Truncate(item.SourceBranch, 13),
		columnKeyTarget:  utils.Truncate(item.TargetBranch, 10),
		columnKeyAuthor:  utils.Truncate(item.CreatedBy, 13),
		columnKeyStatus:  getApprovalStyle(item.ApprovalStatus).Render(item.ApprovalStatus),
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// rowData projects the ordered rows onto their data, preserving order.
func rowData(rows []table.Row) []table.RowData {
	out := make([]table.RowData, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Data)
	}
	return out
}

// TestBuildRowsFilterEquivalence pins the ordered row output of buildRows()
// across the promotion of the project and repository predicates into
// internal/ado, including the empty-slice-means-match-all rule that keeps an
// unconfigured view from rendering empty.
func TestBuildRowsFilterEquivalence(t *testing.T) {
	items := prsFixture()

	tests := []struct {
		name         string
		projects     []string
		repositories []string
		want         []table.RowData
	}{
		{
			name: "no projects and no repositories configured keeps everything",
			want: []table.RowData{wantRow(items[0]), wantRow(items[1]), wantRow(items[2])},
		},
		{
			name:     "empty slices keep everything",
			projects: []string{}, repositories: []string{},
			want: []table.RowData{wantRow(items[0]), wantRow(items[1]), wantRow(items[2])},
		},
		{
			name:     "project filter is case-insensitive exact match",
			projects: []string{"alpha"},
			want:     []table.RowData{wantRow(items[0]), wantRow(items[1])},
		},
		{
			name:         "repository filter is case-insensitive exact match",
			repositories: []string{"web"},
			want:         []table.RowData{wantRow(items[1])},
		},
		{
			name:     "both filters applied together",
			projects: []string{"alpha"}, repositories: []string{"api"},
			want: []table.RowData{wantRow(items[0])},
		},
		{
			name:     "a non-matching project keeps nothing",
			projects: []string{"gamma"},
			want:     []table.RowData{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				items:        prsFixture(),
				projects:     tt.projects,
				repositories: tt.repositories,
				searchInput:  textinput.New(),
			}
			got := rowData(m.buildRows())
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildRows() = %v, want %v", got, tt.want)
			}
		})
	}
}
