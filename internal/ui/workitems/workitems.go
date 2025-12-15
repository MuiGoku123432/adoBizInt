package workitems

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
	"github.com/rmhubbert/bubbletea-overlay"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/ui/styles"
)

const (
	columnKeyID      = "id"
	columnKeyType     = "type"
	columnKeyStatus   = "status"
	columnKeyTitle    = "title"
	columnKeySummary  = "summary"
	columnKeyAssignee = "assignee"
	columnKeyPoints   = "points"
	columnKeyActions  = "actions"
)

// States considered "done" or "resolved" for filtering
var doneStates = map[string]bool{
	"Done":     true,
	"Closed":   true,
	"Resolved": true,
	"Removed":  true,
}

// WorkItemsMsg is sent when work items are fetched
type WorkItemsMsg struct {
	Items []ado.WorkItem
	Err   error
}

// viewWrapper wraps a pre-rendered view string for overlay
type viewWrapper struct {
	content string
}

func (w viewWrapper) Init() tea.Cmd                         { return nil }
func (w viewWrapper) Update(tea.Msg) (tea.Model, tea.Cmd)   { return w, nil }
func (w viewWrapper) View() string                          { return w.content }

// StateUpdateMsg is sent when a work item state is updated
type StateUpdateMsg struct {
	ItemID   int
	NewState string
	Err      error
}

type Model struct {
	client             *ado.Client
	projects           []string
	table              table.Model
	searchInput        textinput.Model
	items              []ado.WorkItem
	loading            bool
	err                error
	width              int
	height             int
	assignedToMeFilter bool
	hideDoneFilter     bool
	currentUser        string
	currentUserEmail   string
	filterFocused      int // 0 = table, 1 = checkbox 1, 2 = checkbox 2, 3 = search
	stateTransitions   map[string]map[string]string
	statusMsg          string
	statusErr          bool
	// Detail modal
	detailModal *DetailModel
	showDetail  bool
	// Effort modal for Task state transitions
	effortModal *EffortModal
	showEffort  bool
}

func New(client *ado.Client, projects []string, stateTransitions map[string]map[string]string) Model {
	// Create search input
	search := textinput.New()
	search.Placeholder = "Search work items..."
	search.Width = 30

	// Create table with columns
	columns := []table.Column{
		table.NewColumn(columnKeyID, "ID", 6).WithStyle(lipgloss.NewStyle().Align(lipgloss.Right)),
		table.NewColumn(columnKeyType, "Type", 12),
		table.NewColumn(columnKeyStatus, "Status", 12),
		table.NewColumn(columnKeyTitle, "Title", 25),
		table.NewColumn(columnKeySummary, "Summary", 22),
		table.NewColumn(columnKeyAssignee, "Assignee", 15),
		table.NewColumn(columnKeyPoints, "Points", 6).WithStyle(lipgloss.NewStyle().Align(lipgloss.Right)),
		table.NewColumn(columnKeyActions, "Actions", 10),
	}

	t := table.New(columns).
		WithBaseStyle(lipgloss.NewStyle().Align(lipgloss.Left)).
		WithTargetWidth(80).
		BorderRounded().
		Focused(true).
		WithPageSize(15)

	return Model{
		client:             client,
		projects:           projects,
		table:              t,
		searchInput:        search,
		loading:            true,
		assignedToMeFilter: true,
		hideDoneFilter:     true,
		currentUser:        client.CurrentUser(),
		currentUserEmail:   client.CurrentUserEmail(),
		filterFocused:      0,
		stateTransitions:   stateTransitions,
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchWorkItems()
}

func (m Model) fetchWorkItems() tea.Cmd {
	projects := m.projects
	return func() tea.Msg {
		items, err := m.client.GetWorkItems(context.Background(), projects)
		return WorkItemsMsg{Items: items, Err: err}
	}
}

func (m *Model) updateSelectedItemState() tea.Cmd {
	// Get the highlighted row from the table
	highlightedRow := m.table.HighlightedRow()
	if highlightedRow.Data == nil {
		return func() tea.Msg {
			return StateUpdateMsg{Err: fmt.Errorf("no item selected")}
		}
	}

	// Get the ID from the row data
	idStr, ok := highlightedRow.Data[columnKeyID].(string)
	if !ok {
		return func() tea.Msg {
			return StateUpdateMsg{Err: fmt.Errorf("invalid item ID")}
		}
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return func() tea.Msg {
			return StateUpdateMsg{Err: fmt.Errorf("invalid item ID: %s", idStr)}
		}
	}

	// Find the work item in our items list
	var item *ado.WorkItem
	for i := range m.items {
		if m.items[i].ID == id {
			item = &m.items[i]
			break
		}
	}
	if item == nil {
		return func() tea.Msg {
			return StateUpdateMsg{Err: fmt.Errorf("work item #%d not found", id)}
		}
	}

	// Get the next state from config
	nextState, hasNext := ado.GetNextState(m.stateTransitions, item.Type, item.State)
	if !hasNext {
		return func() tea.Msg {
			return StateUpdateMsg{Err: fmt.Errorf("no next state defined for %s in %s", item.Type, item.State)}
		}
	}

	// For Tasks going from Active to Closed, show the effort modal
	if item.Type == "Task" && item.State == "Active" && nextState == "Closed" {
		effortModal := NewEffortModal(id, item.Title, nextState)
		effortModal.width = m.width
		effortModal.height = m.height
		m.effortModal = &effortModal
		m.showEffort = true
		return m.effortModal.Init()
	}

	// Create async command to update the state
	client := m.client
	return func() tea.Msg {
		err := client.UpdateWorkItemState(context.Background(), id, nextState)
		return StateUpdateMsg{ItemID: id, NewState: nextState, Err: err}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle detail modal close message
	if _, ok := msg.(CloseDetailMsg); ok {
		m.showDetail = false
		m.detailModal = nil
		return m, nil
	}

	// Handle effort modal close message
	if _, ok := msg.(CloseEffortMsg); ok {
		m.showEffort = false
		m.effortModal = nil
		return m, nil
	}

	// Handle effort modal submit
	if effortMsg, ok := msg.(EffortSubmitMsg); ok {
		m.showEffort = false
		m.effortModal = nil
		// Call API to update state with effort
		client := m.client
		return m, func() tea.Msg {
			err := client.UpdateWorkItemStateWithEffort(
				context.Background(),
				effortMsg.ItemID,
				effortMsg.NewState,
				effortMsg.OriginalEstimate,
				effortMsg.Remaining,
				effortMsg.Completed,
			)
			return StateUpdateMsg{ItemID: effortMsg.ItemID, NewState: effortMsg.NewState, Err: err}
		}
	}

	// Delegate to effort modal when open
	if m.showEffort && m.effortModal != nil {
		if wsm, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = wsm.Width
			m.height = wsm.Height
		}
		m.effortModal.width = m.width
		m.effortModal.height = m.height

		var cmd tea.Cmd
		newModal, cmd := m.effortModal.Update(msg)
		m.effortModal = &newModal
		return m, cmd
	}

	// Delegate to detail modal when open
	if m.showDetail && m.detailModal != nil {
		// Update dimensions from WindowSizeMsg if present
		if wsm, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = wsm.Width
			m.height = wsm.Height
		}
		// Always propagate current dimensions to modal before Update
		m.detailModal.width = m.width
		m.detailModal.height = m.height

		var cmd tea.Cmd
		newModal, cmd := m.detailModal.Update(msg)
		m.detailModal = &newModal
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update table width
		m.table = m.table.WithTargetWidth(msg.Width - 4)

	case WorkItemsMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.items = msg.Items
			m.table = m.table.WithRows(m.buildRows())
		}

	case StateUpdateMsg:
		if msg.Err != nil {
			m.statusMsg = "Error: " + msg.Err.Error()
			m.statusErr = true
		} else {
			m.statusMsg = fmt.Sprintf("Updated #%d to %s", msg.ItemID, msg.NewState)
			m.statusErr = false
			// Refresh work items to get updated state
			return m, m.fetchWorkItems()
		}

	case tea.KeyMsg:
		// Handle search input when focused
		if m.filterFocused == 3 {
			switch msg.String() {
			case "esc":
				m.searchInput.SetValue("")
				m.searchInput.Blur()
				m.filterFocused = 0
				m.table = m.table.WithRows(m.buildRows())
				return m, nil
			case "enter":
				m.searchInput.Blur()
				m.filterFocused = 0
				return m, nil
			case "tab":
				m.searchInput.Blur()
				m.filterFocused = 0
				return m, nil
			case "shift+tab":
				m.searchInput.Blur()
				m.filterFocused = 2
				return m, nil
			default:
				var cmd tea.Cmd
				prevValue := m.searchInput.Value()
				m.searchInput, cmd = m.searchInput.Update(msg)
				if m.searchInput.Value() != prevValue {
					m.table = m.table.WithRows(m.buildRows())
				}
				return m, cmd
			}
		}

		switch msg.String() {
		case "tab":
			m.filterFocused = (m.filterFocused + 1) % 4
			if m.filterFocused == 3 {
				m.searchInput.Focus()
			}
			return m, nil
		case "shift+tab":
			m.filterFocused = (m.filterFocused + 3) % 4
			if m.filterFocused == 3 {
				m.searchInput.Focus()
			}
			return m, nil
		case "/":
			m.filterFocused = 3
			m.searchInput.Focus()
			return m, nil
		case "esc":
			if m.searchInput.Value() != "" {
				m.searchInput.SetValue("")
				m.table = m.table.WithRows(m.buildRows())
				return m, nil
			}
		case " ":
			if m.filterFocused == 1 {
				m.assignedToMeFilter = !m.assignedToMeFilter
				m.table = m.table.WithRows(m.buildRows())
				return m, nil
			} else if m.filterFocused == 2 {
				m.hideDoneFilter = !m.hideDoneFilter
				m.table = m.table.WithRows(m.buildRows())
				return m, nil
			}
		case "enter":
			if m.filterFocused == 1 {
				m.assignedToMeFilter = !m.assignedToMeFilter
				m.table = m.table.WithRows(m.buildRows())
				return m, nil
			} else if m.filterFocused == 2 {
				m.hideDoneFilter = !m.hideDoneFilter
				m.table = m.table.WithRows(m.buildRows())
				return m, nil
			} else if m.filterFocused == 0 {
				// Open detail modal for selected work item
				return m, m.openDetailModal()
			}
		case "n":
			// Move to next state when table is focused
			if m.filterFocused == 0 {
				return m, m.updateSelectedItemState()
			}
		}
		// Only pass key events to table if table is focused
		if m.filterFocused == 0 {
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) buildRows() []table.Row {
	var rows []table.Row
	searchQuery := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))

	for _, item := range m.items {
		// Apply "Assigned to Me" filter (compare emails, case-insensitive)
		if m.assignedToMeFilter && m.currentUserEmail != "" && !strings.EqualFold(item.AssignedToEmail, m.currentUserEmail) {
			continue
		}
		// Apply "Hide Done/Resolved" filter
		if m.hideDoneFilter && doneStates[item.State] {
			continue
		}
		// Apply search filter (ID or Title)
		if searchQuery != "" {
			idStr := fmt.Sprintf("%d", item.ID)
			titleLower := strings.ToLower(item.Title)
			if !strings.Contains(idStr, searchQuery) && !strings.Contains(titleLower, searchQuery) {
				continue
			}
		}
		// Determine if "Next" action is available
		actionText := ""
		if _, hasNext := ado.GetNextState(m.stateTransitions, item.Type, item.State); hasNext {
			actionText = "[n] Next"
		}

		// Style the type with color
		styledType := getTypeStyle(item.Type).Render(item.Type)

		rows = append(rows, table.NewRow(table.RowData{
			columnKeyID:       fmt.Sprintf("%d", item.ID),
			columnKeyType:     styledType,
			columnKeyStatus:   item.State,
			columnKeyTitle:    truncate(item.Title, 23),
			columnKeySummary:  truncate(stripHTML(item.Summary), 20),
			columnKeyAssignee: truncate(item.AssignedTo, 13),
			columnKeyPoints:   fmt.Sprintf("%d", item.StoryPoints),
			columnKeyActions:  actionText,
		}))
	}
	return rows
}

func (m Model) View() string {
	// If effort modal is open, use overlay to composite modal on top of table
	if m.showEffort && m.effortModal != nil {
		modalView := m.effortModal.View()
		tableView := m.renderTableView()

		fg := viewWrapper{content: modalView}
		bg := viewWrapper{content: tableView}
		o := overlay.New(fg, bg, overlay.Center, overlay.Center, 0, 0)
		return o.View()
	}

	// If detail modal is open, use overlay to composite modal on top of table
	if m.showDetail && m.detailModal != nil {
		// Create overlay fresh with current state - avoids stale pointer issues
		modalView := m.detailModal.View()
		tableView := m.renderTableView()

		fg := viewWrapper{content: modalView}
		bg := viewWrapper{content: tableView}
		o := overlay.New(fg, bg, overlay.Center, overlay.Center, 0, 0)
		return o.View()
	}

	return m.renderTableView()
}

// renderTableView renders the work items table view
func (m Model) renderTableView() string {
	var b strings.Builder

	// Title
	title := styles.TitleStyle.Render("Work Items")
	b.WriteString(title)
	b.WriteString("\n")

	// Filter checkboxes row
	b.WriteString(m.renderFilters())
	b.WriteString("\n\n")

	// Show error if any
	if m.err != nil {
		b.WriteString(styles.ErrorStyle.Render("Error: " + m.err.Error()))
		b.WriteString("\n\n")
	}

	// Show loading or table
	if m.loading {
		b.WriteString(styles.SubtitleStyle.Render("Loading work items..."))
	} else if len(m.items) == 0 {
		b.WriteString(styles.SubtitleStyle.Render("No work items found"))
	} else {
		// Show filtered count
		filteredCount := len(m.buildRows())
		totalCount := len(m.items)
		countInfo := styles.SubtitleStyle.Render(fmt.Sprintf("Showing %d of %d items", filteredCount, totalCount))
		b.WriteString(countInfo)
		b.WriteString("\n")
		b.WriteString(m.table.View())
	}

	b.WriteString("\n")

	// Show status message if any
	if m.statusMsg != "" {
		if m.statusErr {
			b.WriteString(styles.ErrorStyle.Render(m.statusMsg))
		} else {
			b.WriteString(styles.SubtitleStyle.Render(m.statusMsg))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Help bar
	helpText := styles.HelpStyle.Render("[Tab] cycle  [Space] toggle  [/] search  [Enter] details  [n] next state  [1] Dashboard  [2] Work Items")
	b.WriteString(helpText)

	return b.String()
}

func (m Model) renderFilters() string {
	check := func(on bool) string {
		if on {
			return "[x]"
		}
		return "[ ]"
	}

	assignedLabel := fmt.Sprintf("%s Assigned to Me", check(m.assignedToMeFilter))
	doneLabel := fmt.Sprintf("%s Hide Done/Resolved", check(m.hideDoneFilter))

	if m.filterFocused == 1 {
		assignedLabel = styles.SelectedStyle.Render(assignedLabel)
	}
	if m.filterFocused == 2 {
		doneLabel = styles.SelectedStyle.Render(doneLabel)
	}

	// Build search box
	searchLabel := fmt.Sprintf("Search: %s", m.searchInput.View())
	if m.filterFocused == 3 {
		searchLabel = styles.SelectedStyle.Render(searchLabel)
	}

	return fmt.Sprintf("Filters: %s   %s   %s", assignedLabel, doneLabel, searchLabel)
}

// truncate truncates a string to maxLen and adds "..." if truncated
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// stripHTML removes HTML tags from a string (basic implementation)
func stripHTML(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(r)
		}
	}
	// Clean up whitespace
	text := result.String()
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\t", " ")
	// Collapse multiple spaces
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}
	return strings.TrimSpace(text)
}

// openDetailModal opens the detail modal for the currently selected work item
func (m *Model) openDetailModal() tea.Cmd {
	// Get the highlighted row from the table
	highlightedRow := m.table.HighlightedRow()
	if highlightedRow.Data == nil {
		return nil
	}

	// Get the ID from the row data
	idStr, ok := highlightedRow.Data[columnKeyID].(string)
	if !ok {
		return nil
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return nil
	}

	// Create detail modal
	detail := NewDetailModel(m.client, id)
	detail.width = m.width
	detail.height = m.height
	m.detailModal = &detail
	m.showDetail = true

	return m.detailModal.Init()
}

// getTypeStyle returns the appropriate style for a work item type
func getTypeStyle(itemType string) lipgloss.Style {
	switch itemType {
	case "User Story":
		return styles.UserStoryStyle
	case "Task":
		return styles.TaskStyle
	case "Bug":
		return styles.BugStyle
	case "Feature":
		return styles.FeatureStyle
	case "Epic":
		return styles.EpicStyle
	default:
		return lipgloss.NewStyle()
	}
}
