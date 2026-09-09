package pipelines

import (
	"reflect"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/evertras/bubble-table/table"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/ui/styles"
	"sentinovo.ai/bizInt/internal/utils"
)

// runsFixture is the fixed slice every buildRows() case below runs against.
// One item per searchable field — pipeline, branch, requester, build number —
// with casing that differs from the queries so the case-insensitive rule is
// exercised in both directions.
func runsFixture() []ado.PipelineRun {
	return []ado.PipelineRun{
		{PipelineName: "web-ci", BuildNumber: "20240101.1", SourceBranch: "main", RequestedBy: "Ann", Status: "completed", Result: "succeeded"},
		{PipelineName: "api-ci", BuildNumber: "20240102.7", SourceBranch: "FEATURE/beta", RequestedBy: "Bob", Status: "inProgress"},
		{PipelineName: "docs-ci", BuildNumber: "", SourceBranch: "", RequestedBy: "", Status: "notStarted"},
	}
}

// wantRow mirrors the row construction in buildRows() for a fixture item.
//
// The comparison is on RowData rather than table.Row because table.NewRow
// stamps each row with a process-global incrementing id that is not part of the
// rendered output.
func wantRow(run ado.PipelineRun) table.RowData {
	resultDisplay := run.Result
	if run.Result != "" {
		resultDisplay = getResultStyle(run.Result).Render(run.Result)
	} else {
		resultDisplay = styles.MutedStyle.Render("-")
	}
	buildNumDisplay := run.BuildNumber
	if buildNumDisplay == "" {
		buildNumDisplay = styles.MutedStyle.Render("-")
	}
	branchDisplay := utils.Truncate(run.SourceBranch, 16)
	if run.SourceBranch == "" {
		branchDisplay = styles.MutedStyle.Render("-")
	}
	requestedByDisplay := utils.Truncate(run.RequestedBy, 13)
	if run.RequestedBy == "" {
		requestedByDisplay = styles.MutedStyle.Render("-")
	}
	startTimeDisplay := ""
	if !run.StartTime.IsZero() {
		startTimeDisplay = run.StartTime.Format("01/02 15:04")
	}
	return table.RowData{
		columnKeyPipeline:    utils.Truncate(run.PipelineName, 18),
		columnKeyBuildNumber: buildNumDisplay,
		columnKeyStatus:      getStatusStyle(run.Status).Render(run.Status),
		columnKeyResult:      resultDisplay,
		columnKeyBranch:      branchDisplay,
		columnKeyRequestedBy: requestedByDisplay,
		columnKeyStartTime:   startTimeDisplay,
		columnKeyDuration:    formatDuration(run.Duration),
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
// ado.MatchesSearch. It asserts the full ordered slice, not a length or a set,
// and covers all four searchable fields — the build number in particular, which
// no other view searches.
func TestBuildRowsSearchEquivalence(t *testing.T) {
	items := runsFixture()

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
			search: "   ",
			want:   []table.RowData{wantRow(items[0]), wantRow(items[1]), wantRow(items[2])},
		},
		{
			name:   "search matches the pipeline name",
			search: "docs-ci",
			want:   []table.RowData{wantRow(items[2])},
		},
		{
			name:   "search matches the source branch case-insensitively",
			search: "feature/beta",
			want:   []table.RowData{wantRow(items[1])},
		},
		{
			name:   "search matches the requester",
			search: "ann",
			want:   []table.RowData{wantRow(items[0])},
		},
		{
			name:   "search matches the build number",
			search: "20240102",
			want:   []table.RowData{wantRow(items[1])},
		},
		{
			name:   "an upper-case query matches a lower-case field",
			search: "WEB-CI",
			want:   []table.RowData{wantRow(items[0])},
		},
		{
			name:   "a shared substring keeps every match in order",
			search: "-ci",
			want:   []table.RowData{wantRow(items[0]), wantRow(items[1]), wantRow(items[2])},
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
			m := Model{runs: runsFixture(), searchInput: ti}
			got := rowData(m.buildRows())
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildRows() = %v, want %v", got, tt.want)
			}
		})
	}
}
