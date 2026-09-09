package releases

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/ui/styles"
	"sentinovo.ai/bizInt/internal/utils"
)

// releasesFixture is the fixed slice every buildRows() case below runs against.
// The two searchable fields are release name and definition name, and the
// casing differs from the queries so the case-insensitive rule is exercised in
// both directions.
func releasesFixture() []ado.Release {
	return []ado.Release{
		{ID: 1, Name: "Release-Alpha", DefinitionName: "web-deploy", Status: "active"},
		{ID: 2, Name: "Release-Beta", DefinitionName: "API-DEPLOY", Status: "draft", PendingApprovals: 2},
		{ID: 3, Name: "", DefinitionName: "docs-deploy", Status: "noReleases"},
	}
}

// wantRow mirrors the row construction in buildRows() for a fixture item.
//
// The comparison is on RowData rather than table.Row because table.NewRow
// stamps each row with a process-global incrementing id that is not part of the
// rendered output.
func wantRow(rel ado.Release) table.RowData {
	releaseDisplay := utils.Truncate(rel.Name, 16)
	if rel.Name == "" {
		releaseDisplay = styles.MutedStyle.Render("-")
	}
	createdDisplay := ""
	if !rel.CreatedOn.IsZero() {
		createdDisplay = rel.CreatedOn.Format("01/02 15:04")
	} else {
		createdDisplay = styles.MutedStyle.Render("-")
	}
	approvalsDisplay := "-"
	if rel.PendingApprovals > 0 {
		approvalsDisplay = lipgloss.NewStyle().Foreground(styles.Warning).Bold(true).Render(fmt.Sprintf("%d", rel.PendingApprovals))
	}
	return table.RowData{
		columnKeyRelease:      releaseDisplay,
		columnKeyDefinition:   utils.Truncate(rel.DefinitionName, 18),
		columnKeyStatus:       getReleaseStatusStyle(rel.Status).Render(rel.Status),
		columnKeyCreated:      createdDisplay,
		columnKeyEnvironments: formatEnvironments(rel.Environments),
		columnKeyApprovals:    approvalsDisplay,
		columnKeyReleaseID:    rel.ID,
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
	items := releasesFixture()

	tests := []struct {
		name   string
		search string
		want   []table.RowData
	}{
		{
			name: "no search keeps everything in order",
			want: []table.RowData{wantRow(items[0]), wantRow(items[1]), wantRow(items[2])},
		},
		{
			name:   "a whitespace-only search keeps everything",
			search: "  \t ",
			want:   []table.RowData{wantRow(items[0]), wantRow(items[1]), wantRow(items[2])},
		},
		{
			name:   "search matches the release name",
			search: "alpha",
			want:   []table.RowData{wantRow(items[0])},
		},
		{
			name:   "search matches the definition name case-insensitively",
			search: "api-deploy",
			want:   []table.RowData{wantRow(items[1])},
		},
		{
			name:   "an upper-case query matches a lower-case definition",
			search: "DOCS",
			want:   []table.RowData{wantRow(items[2])},
		},
		{
			name:   "a shared substring keeps every match in order",
			search: "release-",
			want:   []table.RowData{wantRow(items[0]), wantRow(items[1])},
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
			m := Model{releases: releasesFixture(), searchInput: ti}
			got := rowData(m.buildRows())
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildRows() = %v, want %v", got, tt.want)
			}
		})
	}
}
