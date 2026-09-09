package workitems

import (
	"reflect"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/evertras/bubble-table/table"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/config"
	"sentinovo.ai/bizInt/internal/utils"
)

// rowsFixture is the fixed slice every buildRows() case below runs against.
// One row per filter axis: a kept Bug, a Task, a Done item and an item
// assigned to somebody else.
func rowsFixture() []ado.WorkItem {
	return []ado.WorkItem{
		{ID: 1, Title: "Alpha", Summary: "s1", StoryPoints: 3, Type: "Bug", State: "Active", AssignedTo: "Me", AssignedToEmail: "me@x.com"},
		{ID: 2, Title: "Beta", Summary: "s2", StoryPoints: 1, Type: "Task", State: "Active", AssignedTo: "Me", AssignedToEmail: "ME@X.COM"},
		{ID: 3, Title: "Gamma", Summary: "s3", StoryPoints: 5, Type: "Bug", State: "Done", AssignedTo: "Me", AssignedToEmail: "me@x.com"},
		{ID: 4, Title: "Delta", Summary: "s4", StoryPoints: 2, Type: "Bug", State: "Active", AssignedTo: "Other", AssignedToEmail: "other@x.com"},
	}
}

// wantRow mirrors the row construction in buildRows() for a fixture item. Only
// the priority text varies per case; every other column is a function of the
// item, so the expectation stays a literal comparison of ordered rows.
//
// The comparison is on RowData rather than table.Row because table.NewRow
// stamps each row with a process-global incrementing id, which is not part of
// the rendered output and would make any literal expectation unequal.
func wantRow(item ado.WorkItem, priorityText string) table.RowData {
	return table.RowData{
		columnKeyPriority: priorityText,
		columnKeyID:       itoa(item.ID),
		columnKeyType:     getTypeStyle(item.Type).Render(item.Type),
		columnKeyStatus:   item.State,
		columnKeyTitle:    utils.Truncate(item.Title, 23),
		columnKeySummary:  utils.Truncate(stripHTML(item.Summary), 20),
		columnKeyAssignee: utils.Truncate(item.AssignedTo, 13),
		columnKeyPoints:   itoa(item.StoryPoints),
		columnKeyActions:  "",
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

func rowsModel(m Model) Model {
	m.items = rowsFixture()
	m.searchInput = textinput.New()
	m.currentUserEmail = "me@x.com"
	return m
}

// TestBuildRowsFilterEquivalence pins the ordered row output of buildRows()
// across the promotion of the filter predicates into internal/ado. It asserts
// the full ordered []table.Row, not a set or a count, because the priority sort
// is order-bearing.
func TestBuildRowsFilterEquivalence(t *testing.T) {
	items := rowsFixture()

	tests := []struct {
		name  string
		model Model
		want  []table.RowData
	}{
		{
			name:  "no filters keeps every item in fixture order",
			model: Model{},
			want: []table.RowData{
				wantRow(items[0], ""),
				wantRow(items[1], ""),
				wantRow(items[2], ""),
				wantRow(items[3], ""),
			},
		},
		{
			name:  "assigned-to-me drops the other user, case-insensitively",
			model: Model{assignedToMeFilter: true},
			want: []table.RowData{
				wantRow(items[0], ""),
				wantRow(items[1], ""),
				wantRow(items[2], ""),
			},
		},
		{
			name:  "hide-done drops the Done item",
			model: Model{hideDoneFilter: true},
			want: []table.RowData{
				wantRow(items[0], ""),
				wantRow(items[1], ""),
				wantRow(items[3], ""),
			},
		},
		{
			name:  "hide-tasks drops the Task",
			model: Model{hideTasksFilter: true},
			want: []table.RowData{
				wantRow(items[0], ""),
				wantRow(items[2], ""),
				wantRow(items[3], ""),
			},
		},
		{
			name:  "all three filters on",
			model: Model{assignedToMeFilter: true, hideDoneFilter: true, hideTasksFilter: true},
			want: []table.RowData{
				wantRow(items[0], ""),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rowData(rowsModel(tt.model).buildRows())
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildRows() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBuildRowsEmptyIdentityKeepsEveryone pins the empty-identity no-op that
// used to be the m.currentUserEmail != "" guard. Identity resolution is
// non-fatal, so an empty cache must not empty the view.
func TestBuildRowsEmptyIdentityKeepsEveryone(t *testing.T) {
	m := rowsModel(Model{assignedToMeFilter: true})
	m.currentUserEmail = ""

	if got, want := len(m.buildRows()), 4; got != want {
		t.Errorf("buildRows() returned %d rows with an empty identity, want %d", got, want)
	}
}

// TestBuildRowsPrioritySortOrder covers the order-bearing block at the
// assignedToMeFilter && priorities != nil gate that sits between the filter
// loop and the row loop. A set-only assertion would not catch a regression
// here, so the full ordered slice is compared.
func TestBuildRowsPrioritySortOrder(t *testing.T) {
	items := rowsFixture()
	m := rowsModel(Model{
		assignedToMeFilter: true,
		priorities:         &config.Priorities{Order: []int{3, 1}},
	})

	want := []table.RowData{
		wantRow(items[2], "1"), // Gamma, first in the priority list
		wantRow(items[0], "2"), // Alpha, second
		wantRow(items[1], ""),  // Beta, unprioritized, original order
	}

	got := rowData(m.buildRows())
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildRows() = %v, want prioritized rows first: %v", got, want)
	}
}

// TestBuildRowsPriorityColumnGate pins the SECOND assignedToMeFilter read,
// the one inside the row-construction loop that decides whether the Priority
// column renders a 1-indexed position or an empty string. It is 36 lines past
// the sort and would survive an order-only test if the extraction accidentally
// scoped the flag to the filter loop.
func TestBuildRowsPriorityColumnGate(t *testing.T) {
	priorities := &config.Priorities{Order: []int{3}}

	on := rowsModel(Model{assignedToMeFilter: true, priorities: priorities}).buildRows()
	if got := on[0].Data[columnKeyPriority]; got != "1" {
		t.Errorf("priority column with assignedToMeFilter on = %q, want %q", got, "1")
	}

	off := rowsModel(Model{assignedToMeFilter: false, priorities: priorities}).buildRows()
	// Same item (ID 3) is third in fixture order when no filter is applied.
	if got := off[2].Data[columnKeyID]; got != "3" {
		t.Fatalf("fixture order changed: row 2 has id %q, want %q", got, "3")
	}
	if got := off[2].Data[columnKeyPriority]; got != "" {
		t.Errorf("priority column with assignedToMeFilter off = %q, want empty", got)
	}
}

// searchModel is rowsModel with the search box populated. It sets searchInput
// after rowsModel because rowsModel installs a fresh, empty textinput.
func searchModel(m Model, query string) Model {
	m = rowsModel(m)
	ti := textinput.New()
	ti.SetValue(query)
	m.searchInput = ti
	return m
}

// TestBuildRowsSearchEquivalence pins the ordered row output of buildRows()
// across the promotion of the free-text search comparison into
// ado.MatchesSearch. The field set is unchanged: the formatted ID and the
// title, and nothing else.
func TestBuildRowsSearchEquivalence(t *testing.T) {
	items := rowsFixture()

	tests := []struct {
		name   string
		search string
		want   []table.RowData
	}{
		{
			name: "no search keeps every item in fixture order",
			want: []table.RowData{
				wantRow(items[0], ""), wantRow(items[1], ""),
				wantRow(items[2], ""), wantRow(items[3], ""),
			},
		},
		{
			name:   "a whitespace-only search keeps everything",
			search: "   ",
			want: []table.RowData{
				wantRow(items[0], ""), wantRow(items[1], ""),
				wantRow(items[2], ""), wantRow(items[3], ""),
			},
		},
		{
			name:   "search matches the title case-insensitively",
			search: "gamma",
			want:   []table.RowData{wantRow(items[2], "")},
		},
		{
			name:   "an upper-case query matches a lower-case title",
			search: "DELT",
			want:   []table.RowData{wantRow(items[3], "")},
		},
		{
			name:   "search matches the formatted numeric ID",
			search: "2",
			want:   []table.RowData{wantRow(items[1], "")},
		},
		{
			name:   "the assignee is not a searchable field",
			search: "other",
			want:   []table.RowData{},
		},
		{
			name:   "a non-matching search keeps nothing",
			search: "zzz",
			want:   []table.RowData{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rowData(searchModel(Model{}, tt.search).buildRows())
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildRows() = %v, want %v", got, tt.want)
			}
		})
	}
}
