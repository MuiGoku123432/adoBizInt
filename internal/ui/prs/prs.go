package prs

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/ui/styles"
)

const (
	columnKeyID      = "id"
	columnKeyTitle   = "title"
	columnKeyRepo    = "repo"
	columnKeySource  = "source"
	columnKeyTarget  = "target"
	columnKeyAuthor  = "author"
	columnKeyStatus  = "status"
	columnKeyActions = "actions"
)

// PRsMsg is sent when pull requests are fetched
type PRsMsg struct {
	Items []ado.PullRequest
	Err   error
}

// PRActionMsg is sent when a PR action (approve/complete) completes
type PRActionMsg struct {
	PRID   int
	Action string // "approve" or "complete"
	Err    error
}

type Model struct {
	client        *ado.Client
	projects      []string
	table         table.Model
	searchInput   textinput.Model
	items         []ado.PullRequest
	loading       bool
	err           error
	width         int
	height        int
	filterFocused int // 0 = table, 1 = search
	statusMsg     string
	statusErr     bool
	actionPending bool
}

func New(client *ado.Client, projects []string) Model {
	// Create search input
	search := textinput.New()
	search.Placeholder = "Search PRs..."
	search.Width = 30

	// Create table with columns
	columns := []table.Column{
		table.NewColumn(columnKeyID, "ID", 6).WithStyle(lipgloss.NewStyle().Align(lipgloss.Right)),
		table.NewColumn(columnKeyTitle, "Title", 30),
		table.NewColumn(columnKeyRepo, "Repository", 15),
		table.NewColumn(columnKeySource, "Source", 15),
		table.NewColumn(columnKeyTarget, "Target", 12),
		table.NewColumn(columnKeyAuthor, "Author", 15),
		table.NewColumn(columnKeyStatus, "Approval", 10),
		table.NewColumn(columnKeyActions, "Actions", 20),
	}

	t := table.New(columns).
		WithBaseStyle(lipgloss.NewStyle().Align(lipgloss.Left)).
		WithTargetWidth(80).
		BorderRounded().
		Focused(true).
		WithPageSize(15)

	return Model{
		client:        client,
		projects:      projects,
		table:         t,
		searchInput:   search,
		loading:       true,
		filterFocused: 0,
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchPRs()
}

func (m Model) fetchPRs() tea.Cmd {
	projects := m.projects
	client := m.client
	return func() tea.Msg {
		items, err := client.GetPullRequests(context.Background(), projects)
		return PRsMsg{Items: items, Err: err}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table = m.table.WithTargetWidth(msg.Width - 4)

	case PRsMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.items = msg.Items
			m.table = m.table.WithRows(m.buildRows())
		}

	case PRActionMsg:
		m.actionPending = false
		if msg.Err != nil {
			m.statusMsg = fmt.Sprintf("Error: %s", msg.Err.Error())
			m.statusErr = true
		} else {
			m.statusMsg = fmt.Sprintf("PR #%d %sd successfully", msg.PRID, msg.Action)
			m.statusErr = false
			// Refresh PR list
			return m, m.fetchPRs()
		}

	case tea.KeyMsg:
		// Handle search input when focused
		if m.filterFocused == 1 {
			switch msg.String() {
			case "esc":
				m.searchInput.SetValue("")
				m.searchInput.Blur()
				m.filterFocused = 0
				m.table = m.table.WithRows(m.buildRows())
				return m, nil
			case "enter", "tab":
				m.searchInput.Blur()
				m.filterFocused = 0
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
		case "/":
			m.filterFocused = 1
			m.searchInput.Focus()
			return m, nil
		case "a":
			// Approve selected PR
			if m.filterFocused == 0 && !m.actionPending {
				return m, m.approveSelectedPR()
			}
		case "c":
			// Complete/merge selected PR
			if m.filterFocused == 0 && !m.actionPending {
				return m, m.completeSelectedPR()
			}
		case "r":
			// Refresh PR list
			m.loading = true
			m.statusMsg = ""
			return m, m.fetchPRs()
		}

		// Pass key events to table if focused
		if m.filterFocused == 0 {
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) approveSelectedPR() tea.Cmd {
	pr := m.getSelectedPR()
	if pr == nil {
		return func() tea.Msg {
			return PRActionMsg{Err: fmt.Errorf("no PR selected")}
		}
	}

	m.actionPending = true
	m.statusMsg = fmt.Sprintf("Approving PR #%d...", pr.ID)
	m.statusErr = false

	client := m.client
	prID := pr.ID
	project := pr.Project
	repoID := pr.RepositoryID

	return func() tea.Msg {
		err := client.ApprovePullRequest(context.Background(), project, repoID, prID)
		return PRActionMsg{PRID: prID, Action: "approve", Err: err}
	}
}

func (m *Model) completeSelectedPR() tea.Cmd {
	pr := m.getSelectedPR()
	if pr == nil {
		return func() tea.Msg {
			return PRActionMsg{Err: fmt.Errorf("no PR selected")}
		}
	}

	m.actionPending = true
	m.statusMsg = fmt.Sprintf("Completing PR #%d...", pr.ID)
	m.statusErr = false

	client := m.client
	prID := pr.ID
	project := pr.Project
	repoID := pr.RepositoryID

	return func() tea.Msg {
		err := client.CompletePullRequest(context.Background(), project, repoID, prID)
		return PRActionMsg{PRID: prID, Action: "complete", Err: err}
	}
}

func (m Model) getSelectedPR() *ado.PullRequest {
	highlightedRow := m.table.HighlightedRow()
	if highlightedRow.Data == nil {
		return nil
	}

	idStr, ok := highlightedRow.Data[columnKeyID].(string)
	if !ok {
		return nil
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return nil
	}

	for i := range m.items {
		if m.items[i].ID == id {
			return &m.items[i]
		}
	}
	return nil
}

func (m Model) View() string {
	var b strings.Builder

	// Title
	title := styles.TitleStyle.Render("Pull Requests")
	b.WriteString(title)
	b.WriteString("\n")

	// Search bar
	searchLabel := fmt.Sprintf("Search: %s", m.searchInput.View())
	if m.filterFocused == 1 {
		searchLabel = styles.SelectedStyle.Render(searchLabel)
	}
	b.WriteString(searchLabel)
	b.WriteString("\n\n")

	// Show error if any
	if m.err != nil {
		b.WriteString(styles.ErrorStyle.Render("Error: " + m.err.Error()))
		b.WriteString("\n\n")
	}

	// Show loading or table
	if m.loading {
		b.WriteString(styles.SubtitleStyle.Render("Loading pull requests..."))
	} else if len(m.items) == 0 {
		b.WriteString(styles.SubtitleStyle.Render("No open pull requests found"))
	} else {
		// Show filtered count
		filteredCount := len(m.buildRows())
		totalCount := len(m.items)
		countInfo := styles.SubtitleStyle.Render(fmt.Sprintf("Showing %d of %d PRs", filteredCount, totalCount))
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
			b.WriteString(styles.SuccessStyle.Render(m.statusMsg))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Help bar
	helpText := styles.HelpStyle.Render("[/] search  [a] approve  [c] complete  [r] refresh  [1] Dashboard  [2] Work Items  [3] PRs")
	b.WriteString(helpText)

	return b.String()
}

func (m Model) buildRows() []table.Row {
	var rows []table.Row
	searchQuery := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))

	for _, item := range m.items {
		// Apply search filter
		if searchQuery != "" {
			idStr := fmt.Sprintf("%d", item.ID)
			titleLower := strings.ToLower(item.Title)
			repoLower := strings.ToLower(item.RepositoryName)
			authorLower := strings.ToLower(item.CreatedBy)
			if !strings.Contains(idStr, searchQuery) &&
				!strings.Contains(titleLower, searchQuery) &&
				!strings.Contains(repoLower, searchQuery) &&
				!strings.Contains(authorLower, searchQuery) {
				continue
			}
		}

		// Style draft PRs differently
		titleDisplay := truncate(item.Title, 28)
		if item.IsDraft {
			titleDisplay = "[Draft] " + truncate(item.Title, 20)
		}

		// Format approval status with color
		statusDisplay := getApprovalStyle(item.ApprovalStatus).Render(item.ApprovalStatus)

		rows = append(rows, table.NewRow(table.RowData{
			columnKeyID:      fmt.Sprintf("%d", item.ID),
			columnKeyTitle:   titleDisplay,
			columnKeyRepo:    truncate(item.RepositoryName, 13),
			columnKeySource:  truncate(item.SourceBranch, 13),
			columnKeyTarget:  truncate(item.TargetBranch, 10),
			columnKeyAuthor:  truncate(item.CreatedBy, 13),
			columnKeyStatus:  statusDisplay,
			columnKeyActions: "[a]pprove [c]omplete",
		}))
	}
	return rows
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func getApprovalStyle(status string) lipgloss.Style {
	switch status {
	case "Approved":
		return lipgloss.NewStyle().Foreground(styles.Success)
	case "Rejected":
		return lipgloss.NewStyle().Foreground(styles.Error)
	case "Waiting":
		return lipgloss.NewStyle().Foreground(styles.Warning)
	default:
		return lipgloss.NewStyle().Foreground(styles.Muted)
	}
}
