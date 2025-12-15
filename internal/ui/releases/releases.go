package releases

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
	"github.com/rmhubbert/bubbletea-overlay"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/ui/styles"
)

// Table column keys
const (
	columnKeyRelease      = "release"
	columnKeyDefinition   = "definition"
	columnKeyStatus       = "status"
	columnKeyCreated      = "created"
	columnKeyEnvironments = "environments"
	columnKeyApprovals    = "approvals"
)

// ReleasesMsg is sent when releases are fetched
type ReleasesMsg struct {
	Releases []ado.Release
	Err      error
}

// ApprovalsMsg is sent when approvals are fetched
type ApprovalsMsg struct {
	Approvals []ado.ReleaseApproval
	Err       error
}

// ApprovalResultMsg is sent when an approval action completes
type ApprovalResultMsg struct {
	ApprovalID int
	Action     string // "approve" or "reject"
	Err        error
}

// TickMsg is used for auto-refresh polling
type TickMsg struct{}

type Model struct {
	client       *ado.Client
	projects     []string
	pollInterval time.Duration

	// Data
	releases  []ado.Release
	approvals []ado.ReleaseApproval

	// UI Components
	table       table.Model
	searchInput textinput.Model

	// State
	loading       bool
	err           error
	width         int
	height        int
	filterFocused int // 0 = table, 1 = search
	statusMsg     string
	statusErr     bool

	// Approval modal
	approvalModal *ApprovalModal
	showApproval  bool

	// Auto-refresh
	autoRefresh bool
}

func New(client *ado.Client, projects []string, pollInterval time.Duration) Model {
	// Create search input
	search := textinput.New()
	search.Placeholder = "Search releases..."
	search.Width = 30

	// Create table with columns
	columns := []table.Column{
		table.NewColumn(columnKeyRelease, "Release", 18),
		table.NewColumn(columnKeyDefinition, "Definition", 20),
		table.NewColumn(columnKeyStatus, "Status", 12),
		table.NewColumn(columnKeyCreated, "Created", 16),
		table.NewColumn(columnKeyEnvironments, "Environments", 30),
		table.NewColumn(columnKeyApprovals, "Pending", 8),
	}

	t := table.New(columns).
		WithBaseStyle(lipgloss.NewStyle().Align(lipgloss.Left)).
		WithTargetWidth(80).
		BorderRounded().
		Focused(true).
		WithPageSize(15)

	return Model{
		client:       client,
		projects:     projects,
		pollInterval: pollInterval,
		table:        t,
		searchInput:  search,
		loading:      true,
		autoRefresh:  true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.fetchReleases(),
		m.fetchApprovals(),
		m.tickCmd(),
	)
}

func (m Model) tickCmd() tea.Cmd {
	if m.pollInterval <= 0 {
		m.pollInterval = 30 * time.Second
	}
	return tea.Tick(m.pollInterval, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

func (m Model) fetchReleases() tea.Cmd {
	projects := m.projects
	client := m.client
	return func() tea.Msg {
		releases, err := client.GetReleases(context.Background(), projects)
		return ReleasesMsg{Releases: releases, Err: err}
	}
}

func (m Model) fetchApprovals() tea.Cmd {
	projects := m.projects
	client := m.client
	return func() tea.Msg {
		approvals, err := client.GetPendingApprovals(context.Background(), projects)
		return ApprovalsMsg{Approvals: approvals, Err: err}
	}
}

func (m Model) approveRelease(approvalID int, project, comment string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := client.ApproveRelease(context.Background(), project, approvalID, comment)
		return ApprovalResultMsg{ApprovalID: approvalID, Action: "approve", Err: err}
	}
}

func (m Model) rejectRelease(approvalID int, project, comment string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := client.RejectRelease(context.Background(), project, approvalID, comment)
		return ApprovalResultMsg{ApprovalID: approvalID, Action: "reject", Err: err}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle approval modal close
	if _, ok := msg.(CloseApprovalMsg); ok {
		m.showApproval = false
		m.approvalModal = nil
		return m, nil
	}

	// Handle approval submit
	if submitMsg, ok := msg.(ApprovalSubmitMsg); ok {
		m.showApproval = false
		m.approvalModal = nil
		m.statusMsg = fmt.Sprintf("Processing %s...", submitMsg.Action)
		m.statusErr = false
		if submitMsg.Action == "approve" {
			return m, m.approveRelease(submitMsg.ApprovalID, submitMsg.Project, submitMsg.Comment)
		} else {
			return m, m.rejectRelease(submitMsg.ApprovalID, submitMsg.Project, submitMsg.Comment)
		}
	}

	// Delegate to approval modal when open
	if m.showApproval && m.approvalModal != nil {
		if wsm, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = wsm.Width
			m.height = wsm.Height
		}
		m.approvalModal.width = m.width
		m.approvalModal.height = m.height

		var cmd tea.Cmd
		newModal, cmd := m.approvalModal.Update(msg)
		m.approvalModal = &newModal
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table = m.table.WithTargetWidth(msg.Width - 4)

	case TickMsg:
		// Auto-refresh if enabled
		if m.autoRefresh && !m.loading {
			m.loading = true
			cmds = append(cmds, m.fetchReleases(), m.fetchApprovals(), m.tickCmd())
		} else {
			cmds = append(cmds, m.tickCmd())
		}

	case ReleasesMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.releases = msg.Releases
			m.table = m.table.WithRows(m.buildRows())
		}

	case ApprovalsMsg:
		if msg.Err != nil {
			// Log but don't show error - approvals are secondary
		} else {
			m.approvals = msg.Approvals
			// Update pending approval counts in releases
			m.updatePendingCounts()
			m.table = m.table.WithRows(m.buildRows())
		}

	case ApprovalResultMsg:
		if msg.Err != nil {
			m.statusMsg = "Error: " + msg.Err.Error()
			m.statusErr = true
		} else {
			m.statusMsg = fmt.Sprintf("Successfully %sd approval #%d", msg.Action, msg.ApprovalID)
			m.statusErr = false
			// Refresh to show updated status
			m.loading = true
			return m, tea.Batch(m.fetchReleases(), m.fetchApprovals())
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
		case "r":
			// Manual refresh
			m.loading = true
			m.statusMsg = "Refreshing..."
			m.statusErr = false
			return m, tea.Batch(m.fetchReleases(), m.fetchApprovals())
		case "a":
			// Approve selected release (open modal if has pending approvals)
			if len(m.approvals) > 0 {
				// For now, just open modal with first pending approval
				// A more sophisticated version would use the selected table row
				modal := NewApprovalModal(m.approvals[0])
				m.approvalModal = &modal
				m.showApproval = true
				return m, nil
			} else {
				m.statusMsg = "No pending approvals"
				m.statusErr = false
			}
		case "t":
			// Toggle auto-refresh
			m.autoRefresh = !m.autoRefresh
			if m.autoRefresh {
				m.statusMsg = "Auto-refresh enabled"
			} else {
				m.statusMsg = "Auto-refresh disabled"
			}
			m.statusErr = false
			return m, nil
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

func (m *Model) updatePendingCounts() {
	// Count pending approvals per release
	pendingCounts := make(map[int]int)
	for _, a := range m.approvals {
		pendingCounts[a.ReleaseID]++
	}

	// Update releases with pending counts
	for i := range m.releases {
		m.releases[i].PendingApprovals = pendingCounts[m.releases[i].ID]
	}
}

func (m Model) View() string {
	// If approval modal is open, use overlay
	if m.showApproval && m.approvalModal != nil {
		modalView := m.approvalModal.View()
		tableView := m.renderTableView()

		fg := viewWrapper{content: modalView}
		bg := viewWrapper{content: tableView}
		o := overlay.New(fg, bg, overlay.Center, overlay.Center, 0, 0)
		return o.View()
	}

	return m.renderTableView()
}

func (m Model) renderTableView() string {
	var b strings.Builder

	// Title with auto-refresh indicator
	title := "Releases"
	if m.autoRefresh {
		title += " (auto)"
	}
	b.WriteString(styles.TitleStyle.Render(title))
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
	if m.loading && len(m.releases) == 0 {
		b.WriteString(styles.SubtitleStyle.Render("Loading releases..."))
	} else if len(m.releases) == 0 {
		b.WriteString(styles.SubtitleStyle.Render("No releases found. Configure pipelines in config.yaml"))
	} else {
		filteredCount := len(m.buildRows())
		totalCount := len(m.releases)
		pendingCount := len(m.approvals)
		countInfo := styles.SubtitleStyle.Render(fmt.Sprintf("Showing %d of %d releases, %d pending approvals", filteredCount, totalCount, pendingCount))
		b.WriteString(countInfo)
		if m.loading {
			b.WriteString(styles.SubtitleStyle.Render(" (refreshing...)"))
		}
		b.WriteString("\n")
		b.WriteString(m.table.View())
	}

	b.WriteString("\n")

	// Status message
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
	helpText := styles.HelpStyle.Render("[/] search  [r] refresh  [a] approve  [t] toggle auto  [1-5] views")
	b.WriteString(helpText)

	return b.String()
}

func (m Model) buildRows() []table.Row {
	var rows []table.Row
	searchQuery := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))

	for _, rel := range m.releases {
		// Apply search filter
		if searchQuery != "" {
			releaseLower := strings.ToLower(rel.Name)
			defLower := strings.ToLower(rel.DefinitionName)
			if !strings.Contains(releaseLower, searchQuery) &&
				!strings.Contains(defLower, searchQuery) {
				continue
			}
		}

		// Format status with color
		statusDisplay := getReleaseStatusStyle(rel.Status).Render(rel.Status)

		// Format created time
		createdDisplay := ""
		if !rel.CreatedOn.IsZero() {
			createdDisplay = rel.CreatedOn.Format("01/02 15:04")
		}

		// Format environments with status indicators
		envsDisplay := formatEnvironments(rel.Environments)

		// Format pending approvals
		approvalsDisplay := "-"
		if rel.PendingApprovals > 0 {
			approvalsDisplay = lipgloss.NewStyle().Foreground(styles.Warning).Bold(true).Render(fmt.Sprintf("%d", rel.PendingApprovals))
		}

		rows = append(rows, table.NewRow(table.RowData{
			columnKeyRelease:      truncate(rel.Name, 16),
			columnKeyDefinition:   truncate(rel.DefinitionName, 18),
			columnKeyStatus:       statusDisplay,
			columnKeyCreated:      createdDisplay,
			columnKeyEnvironments: envsDisplay,
			columnKeyApprovals:    approvalsDisplay,
		}))
	}
	return rows
}

func getReleaseStatusStyle(status string) lipgloss.Style {
	switch status {
	case "active":
		return lipgloss.NewStyle().Foreground(styles.Success)
	case "draft":
		return lipgloss.NewStyle().Foreground(styles.Warning)
	case "abandoned":
		return lipgloss.NewStyle().Foreground(styles.Muted)
	default:
		return lipgloss.NewStyle().Foreground(styles.Muted)
	}
}

func formatEnvironments(envs []ado.ReleaseEnvironment) string {
	if len(envs) == 0 {
		return styles.MutedStyle.Render("-")
	}

	var parts []string
	for _, env := range envs {
		// Truncate env name to first 3 chars
		name := truncate(env.Name, 3)
		indicator := getEnvStatusIndicator(env.Status)
		parts = append(parts, indicator+name)
	}

	// Limit to 5 environments to avoid overflow
	if len(parts) > 5 {
		parts = parts[:5]
		parts = append(parts, "...")
	}

	return strings.Join(parts, " ")
}

func getEnvStatusIndicator(status string) string {
	switch status {
	case "succeeded":
		return lipgloss.NewStyle().Foreground(styles.Success).Render("*")
	case "inProgress", "queued":
		return lipgloss.NewStyle().Foreground(styles.Warning).Render("*")
	case "failed", "rejected", "canceled":
		return lipgloss.NewStyle().Foreground(styles.Error).Render("*")
	case "notStarted":
		return lipgloss.NewStyle().Foreground(styles.Muted).Render("*")
	case "partiallySucceeded":
		return lipgloss.NewStyle().Foreground(styles.Warning).Render("*")
	default:
		return lipgloss.NewStyle().Foreground(styles.Muted).Render("*")
	}
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

// viewWrapper for overlay
type viewWrapper struct {
	content string
}

func (w viewWrapper) Init() tea.Cmd                       { return nil }
func (w viewWrapper) Update(tea.Msg) (tea.Model, tea.Cmd) { return w, nil }
func (w viewWrapper) View() string                        { return w.content }
