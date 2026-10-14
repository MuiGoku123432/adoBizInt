package workitems

import (
	"strconv"

	"github.com/evertras/bubble-table/table"
)

// highlightedID is the work item ID on t's highlighted row, or 0 when there is
// none.
func highlightedID(t table.Model) int {
	idStr, _ := t.HighlightedRow().Data[columnKeyID].(string)
	id, _ := strconv.Atoi(idStr)
	return id
}

// HasStaged reports whether an edit is staged and not yet confirmed.
func (m Model) HasStaged() bool { return false }
