package pipelines

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
	columnKeyPipeline    = "pipeline"
	columnKeyBuildNumber = "buildNumber"
	columnKeyStatus      = "status"
	columnKeyResult      = "result"
	columnKeyBranch      = "branch"
	columnKeyRequestedBy = "requestedBy"
	columnKeyStartTime   = "startTime"
	columnKeyDuration    = "duration"
)

// PipelinesMsg is sent when pipeline runs are fetched
type PipelinesMsg struct {
	Runs []ado.PipelineRun
	Err  error
}

// DefinitionsMsg is sent when pipeline definitions are fetched
type DefinitionsMsg struct {
	Definitions []ado.PipelineDefinition
	Err         error
}

// TriggerResultMsg is sent when a pipeline is triggered
type TriggerResultMsg struct {
	PipelineName string
	BuildNumber  string
	Err          error
}

// TickMsg is used for auto-refresh polling
type TickMsg struct{}

type Model struct {
	client       *ado.Client
	projects     []string
	pollInterval time.Duration

	// Data
	runs        []ado.PipelineRun
	definitions []ado.PipelineDefinition

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

	// Trigger modal
	triggerModal *TriggerModal
	showTrigger  bool

	// Auto-refresh
	autoRefresh bool
}

func New(client *ado.Client, projects []string, pollInterval time.Duration) Model {
	// Create search input
	search := textinput.New()
	search.Placeholder = "Search pipelines..."
	search.Width = 30

	// Create table with columns
	columns := []table.Column{
		table.NewColumn(columnKeyPipeline, "Pipeline", 20),
		table.NewColumn(columnKeyBuildNumber, "#", 10),
		table.NewColumn(columnKeyStatus, "Status", 12),
		table.NewColumn(columnKeyResult, "Result", 12),
		table.NewColumn(columnKeyBranch, "Branch", 18),
		table.NewColumn(columnKeyRequestedBy, "Triggered By", 15),
		table.NewColumn(columnKeyStartTime, "Started", 16),
		table.NewColumn(columnKeyDuration, "Duration", 10),
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
		m.fetchPipelineRuns(),
		m.fetchDefinitions(),
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

func (m Model) fetchPipelineRuns() tea.Cmd {
	projects := m.projects
	client := m.client
	return func() tea.Msg {
		runs, err := client.GetPipelineRuns(context.Background(), projects)
		return PipelinesMsg{Runs: runs, Err: err}
	}
}

func (m Model) fetchDefinitions() tea.Cmd {
	projects := m.projects
	client := m.client
	return func() tea.Msg {
		defs, err := client.GetPipelineDefinitions(context.Background(), projects)
		return DefinitionsMsg{Definitions: defs, Err: err}
	}
}

func (m Model) queuePipeline(definitionID int, branch, project, pipelineName string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		run, err := client.QueuePipeline(context.Background(), project, definitionID, branch)
		var buildNumber string
		if run != nil {
			buildNumber = run.BuildNumber
		}
		return TriggerResultMsg{
			PipelineName: pipelineName,
			BuildNumber:  buildNumber,
			Err:          err,
		}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle trigger modal close
	if _, ok := msg.(CloseTriggerMsg); ok {
		m.showTrigger = false
		m.triggerModal = nil
		return m, nil
	}

	// Handle trigger submit
	if triggerMsg, ok := msg.(TriggerSubmitMsg); ok {
		m.showTrigger = false
		m.triggerModal = nil
		m.statusMsg = fmt.Sprintf("Queuing %s on %s...", triggerMsg.PipelineName, triggerMsg.Branch)
		m.statusErr = false
		return m, m.queuePipeline(triggerMsg.DefinitionID, triggerMsg.Branch, triggerMsg.Project, triggerMsg.PipelineName)
	}

	// Delegate to trigger modal when open
	if m.showTrigger && m.triggerModal != nil {
		if wsm, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = wsm.Width
			m.height = wsm.Height
		}
		m.triggerModal.width = m.width
		m.triggerModal.height = m.height

		var cmd tea.Cmd
		newModal, cmd := m.triggerModal.Update(msg)
		m.triggerModal = &newModal
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
			cmds = append(cmds, m.fetchPipelineRuns(), m.tickCmd())
		} else {
			cmds = append(cmds, m.tickCmd())
		}

	case PipelinesMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.runs = msg.Runs
			m.table = m.table.WithRows(m.buildRows())
		}

	case DefinitionsMsg:
		if msg.Err != nil {
			// Log but don't show error - definitions are only needed for triggering
		} else {
			m.definitions = msg.Definitions
		}

	case TriggerResultMsg:
		if msg.Err != nil {
			m.statusMsg = "Error: " + msg.Err.Error()
			m.statusErr = true
		} else {
			m.statusMsg = fmt.Sprintf("Queued %s: Build #%s", msg.PipelineName, msg.BuildNumber)
			m.statusErr = false
			// Refresh to show new build
			m.loading = true
			return m, m.fetchPipelineRuns()
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
			return m, m.fetchPipelineRuns()
		case "t":
			// Trigger new pipeline run
			if len(m.definitions) > 0 {
				modal := NewTriggerModal(m.client, m.definitions)
				m.triggerModal = &modal
				m.showTrigger = true
				return m, nil
			} else {
				m.statusMsg = "No pipeline definitions available"
				m.statusErr = true
			}
		case "a":
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

func (m Model) View() string {
	// If trigger modal is open, use overlay
	if m.showTrigger && m.triggerModal != nil {
		modalView := m.triggerModal.View()
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
	title := "Pipelines"
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
	if m.loading && len(m.runs) == 0 {
		b.WriteString(styles.SubtitleStyle.Render("Loading pipeline runs..."))
	} else if len(m.runs) == 0 {
		b.WriteString(styles.SubtitleStyle.Render("No pipeline runs found. Configure pipelines in config.yaml"))
	} else {
		filteredCount := len(m.buildRows())
		totalCount := len(m.runs)
		countInfo := styles.SubtitleStyle.Render(fmt.Sprintf("Showing %d of %d runs", filteredCount, totalCount))
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
	helpText := styles.HelpStyle.Render("[/] search  [r] refresh  [t] trigger  [a] toggle auto  [1-4] views")
	b.WriteString(helpText)

	return b.String()
}

func (m Model) buildRows() []table.Row {
	var rows []table.Row
	searchQuery := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))

	for _, run := range m.runs {
		// Apply search filter
		if searchQuery != "" {
			pipelineLower := strings.ToLower(run.PipelineName)
			branchLower := strings.ToLower(run.SourceBranch)
			requestedByLower := strings.ToLower(run.RequestedBy)
			buildNumLower := strings.ToLower(run.BuildNumber)
			if !strings.Contains(pipelineLower, searchQuery) &&
				!strings.Contains(branchLower, searchQuery) &&
				!strings.Contains(requestedByLower, searchQuery) &&
				!strings.Contains(buildNumLower, searchQuery) {
				continue
			}
		}

		// Format status with color
		statusDisplay := getStatusStyle(run.Status).Render(run.Status)

		// Format result with color
		resultDisplay := run.Result
		if run.Result != "" {
			resultDisplay = getResultStyle(run.Result).Render(run.Result)
		} else {
			resultDisplay = styles.MutedStyle.Render("-")
		}

		// Format start time
		startTimeDisplay := ""
		if !run.StartTime.IsZero() {
			startTimeDisplay = run.StartTime.Format("01/02 15:04")
		}

		// Format duration
		durationDisplay := formatDuration(run.Duration)

		rows = append(rows, table.NewRow(table.RowData{
			columnKeyPipeline:    truncate(run.PipelineName, 18),
			columnKeyBuildNumber: run.BuildNumber,
			columnKeyStatus:      statusDisplay,
			columnKeyResult:      resultDisplay,
			columnKeyBranch:      truncate(run.SourceBranch, 16),
			columnKeyRequestedBy: truncate(run.RequestedBy, 13),
			columnKeyStartTime:   startTimeDisplay,
			columnKeyDuration:    durationDisplay,
		}))
	}
	return rows
}

func getStatusStyle(status string) lipgloss.Style {
	switch status {
	case "inProgress":
		return lipgloss.NewStyle().Foreground(styles.Warning)
	case "completed":
		return lipgloss.NewStyle().Foreground(styles.Muted)
	case "cancelling":
		return lipgloss.NewStyle().Foreground(styles.Warning)
	case "notStarted":
		return lipgloss.NewStyle().Foreground(styles.Muted)
	default:
		return lipgloss.NewStyle().Foreground(styles.Muted)
	}
}

func getResultStyle(result string) lipgloss.Style {
	switch result {
	case "succeeded":
		return lipgloss.NewStyle().Foreground(styles.Success)
	case "failed":
		return lipgloss.NewStyle().Foreground(styles.Error)
	case "partiallySucceeded":
		return lipgloss.NewStyle().Foreground(styles.Warning)
	case "canceled":
		return lipgloss.NewStyle().Foreground(styles.Muted)
	default:
		return lipgloss.NewStyle().Foreground(styles.Muted)
	}
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
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
