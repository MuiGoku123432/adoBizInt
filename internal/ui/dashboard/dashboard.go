package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/ui/fetchctx"
	"sentinovo.ai/bizInt/internal/ui/styles"
)

// CountsMsg is sent when dashboard counts are fetched. Result carries the
// aggregate as a one-element Items slice plus whatever failed and whether the
// totals are the whole picture; Err is reserved for a transport failure that
// produced no envelope at all.
type CountsMsg struct {
	Result ado.Result[ado.DashboardCounts]
	Err    error
}

// FilterKeyMap defines keybindings for filter mode
type FilterKeyMap struct {
	Toggle    key.Binding
	SelectAll key.Binding
	Apply     key.Binding
	Cancel    key.Binding
	Up        key.Binding
	Down      key.Binding
}

func defaultFilterKeyMap() FilterKeyMap {
	return FilterKeyMap{
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		SelectAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "select all"),
		),
		Apply: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "apply"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("up/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("down/j", "move down"),
		),
	}
}

type Model struct {
	client           *ado.Client
	orgURL           string
	projects         []string // All configured projects
	selectedProjects []string // Currently selected for filter (empty = all)
	tempSelected     []string // Temp selection while in filter mode
	filterActive     bool
	filterCursor     int
	filterKeys       FilterKeyMap
	workItemCount    int
	prCount          int
	pipelineCount    int
	loading          bool
	err              error
	width            int
	height           int

	// The dashboard is the view where honesty is hardest to render: three
	// integers have no visible seam, so a partial sum looks exactly like a
	// correct one. authBlocked suppresses the cards entirely for a rejected
	// credential; partialWarning states in words that the totals are
	// understated rather than leaving the user to infer it (D-03, D-14).
	authBlocked    bool
	authBanner     string
	partialWarning string
	complete       bool
}

func New(client *ado.Client, orgURL string, projects []string) Model {
	return Model{
		client:           client,
		orgURL:           orgURL,
		projects:         projects,
		selectedProjects: []string{}, // Empty means all
		filterKeys:       defaultFilterKeyMap(),
		loading:          true,
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchCounts()
}

func (m Model) fetchCounts() tea.Cmd {
	selected := m.selectedProjects
	client := m.client
	budget := client.FetchTimeout()
	return func() tea.Msg {
		ctx, cancel := fetchctx.New(budget)
		defer cancel()
		result, err := client.GetDashboardCounts(ctx, selected)
		return CountsMsg{Result: result, Err: err}
	}
}

// opDashboardIterations is 02-03's label for the iteration lookup. A failure
// here is not a project that went silent: the WIQL query below it loses its
// iteration filter and runs unfiltered, so the work-item total comes back
// larger than it should. Wording it identically to a dead project would train
// the user to ignore a line that means something different.
const opDashboardIterations = "get dashboard iterations"

// applyResult folds the counts envelope into render state. An auth-kind failure
// blocks the cards (D-14); any other failure renders the counts alongside a
// warning that states they are understated (D-03, D-13).
func (m *Model) applyResult(result ado.Result[ado.DashboardCounts]) {
	if len(result.Items) > 0 {
		counts := result.Items[0]
		m.workItemCount = counts.WorkItems
		m.prCount = counts.PRs
		m.pipelineCount = counts.Pipelines
	}
	m.complete = result.Complete
	m.authBlocked = false
	m.authBanner = ""
	m.partialWarning = ""

	if len(result.Failures) == 0 {
		return
	}

	if failure := styles.FirstAuthFailure(result.Failures); failure != nil {
		m.authBlocked = true
		m.authBanner = styles.AuthBannerFor(*failure)
		return
	}

	m.partialWarning = summarizeCountFailures(result.Failures)
}

// summarizeCountFailures states the effect on the number first and the cause
// second. Degraded iteration lookups are separated out because they skew the
// total upward, not downward, and a user cannot act on the two the same way.
func summarizeCountFailures(failures []ado.Failure) string {
	var degraded, unanswered []ado.Failure
	for _, failure := range failures {
		if failure.Operation == opDashboardIterations {
			degraded = append(degraded, failure)
		} else {
			unanswered = append(unanswered, failure)
		}
	}

	parts := []string{"totals understated"}
	if len(unanswered) > 0 {
		parts = append(parts, "did not answer: "+styles.SummarizeFailures(unanswered))
	}
	if len(degraded) > 0 {
		parts = append(parts, "counted unfiltered by iteration: "+styles.SummarizeFailures(degraded))
	}
	return strings.Join(parts, "; ")
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.filterActive {
			return m.updateFilter(msg)
		}
		// Handle 'f' to open filter
		if msg.String() == "f" && len(m.projects) > 0 {
			m.filterActive = true
			m.filterCursor = 0
			// Initialize temp selection from current selection
			if len(m.selectedProjects) == 0 {
				// All selected - copy all projects
				m.tempSelected = make([]string, len(m.projects))
				copy(m.tempSelected, m.projects)
			} else {
				m.tempSelected = make([]string, len(m.selectedProjects))
				copy(m.tempSelected, m.selectedProjects)
			}
			return m, nil
		}

	case CountsMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.err = nil
			m.applyResult(msg.Result)
		}
	}
	return m, nil
}

func (m Model) updateFilter(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.filterKeys.Cancel):
		m.filterActive = false
		m.tempSelected = nil
		return m, nil

	case key.Matches(msg, m.filterKeys.Apply):
		m.filterActive = false
		// If all are selected, use empty slice (means "all")
		if len(m.tempSelected) == len(m.projects) {
			m.selectedProjects = []string{}
		} else {
			m.selectedProjects = m.tempSelected
		}
		m.tempSelected = nil
		m.loading = true
		return m, m.fetchCounts()

	case key.Matches(msg, m.filterKeys.Up):
		if m.filterCursor > 0 {
			m.filterCursor--
		}
		return m, nil

	case key.Matches(msg, m.filterKeys.Down):
		if m.filterCursor < len(m.projects)-1 {
			m.filterCursor++
		}
		return m, nil

	case key.Matches(msg, m.filterKeys.Toggle):
		project := m.projects[m.filterCursor]
		if m.isProjectSelected(project) {
			// Remove from selection
			newSelected := []string{}
			for _, p := range m.tempSelected {
				if p != project {
					newSelected = append(newSelected, p)
				}
			}
			m.tempSelected = newSelected
		} else {
			// Add to selection
			m.tempSelected = append(m.tempSelected, project)
		}
		return m, nil

	case key.Matches(msg, m.filterKeys.SelectAll):
		// Toggle all
		if len(m.tempSelected) == len(m.projects) {
			m.tempSelected = []string{}
		} else {
			m.tempSelected = make([]string, len(m.projects))
			copy(m.tempSelected, m.projects)
		}
		return m, nil
	}

	return m, nil
}

func (m Model) isProjectSelected(project string) bool {
	for _, p := range m.tempSelected {
		if p == project {
			return true
		}
	}
	return false
}

func (m Model) View() string {
	if m.filterActive {
		return m.renderFilterView()
	}

	var b strings.Builder

	title := styles.TitleStyle.Render("Azure DevOps Dashboard")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Organization info
	orgInfo := fmt.Sprintf("Organization: %s", m.orgURL)
	b.WriteString(styles.SubtitleStyle.Render(orgInfo))
	b.WriteString("\n")

	// Projects info
	projectInfo := m.renderProjectsInfo()
	b.WriteString(styles.SubtitleStyle.Render(projectInfo))
	b.WriteString("\n\n")

	// Show error if any
	if m.err != nil {
		b.WriteString(styles.ErrorStyle.Render("Error: " + m.err.Error()))
		b.WriteString("\n\n")
	}

	// A rejected credential replaces the cards rather than annotating them: with
	// no data read, a zero total would assert something the tool does not know.
	if m.authBlocked {
		b.WriteString(m.authBanner)
		b.WriteString("\n\n")
		b.WriteString(styles.HelpStyle.Render("Press 'f' to filter projects, ? for help, q to quit"))
		return b.String()
	}
	if m.partialWarning != "" {
		b.WriteString(styles.RenderUnderstatedWarning(m.partialWarning))
		b.WriteString("\n\n")
	}

	// Summary cards
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Padding(1, 3).
		MarginRight(2)

	workItemCard := cardStyle.Render(m.renderCard("Work Items", m.workItemCount))
	prCard := cardStyle.Render(m.renderCard("Pull Requests", m.prCount))
	pipelineCard := cardStyle.Render(m.renderCard("Pipelines", m.pipelineCount))

	cards := lipgloss.JoinHorizontal(lipgloss.Top, workItemCard, prCard, pipelineCard)
	b.WriteString(cards)
	b.WriteString("\n\n")

	// Navigation hint
	hint := "Press 'f' to filter projects, ? for help, q to quit"
	b.WriteString(styles.HelpStyle.Render(hint))

	return b.String()
}

func (m Model) renderProjectsInfo() string {
	if len(m.projects) == 0 {
		return "Projects: None configured"
	}

	if len(m.selectedProjects) == 0 {
		return fmt.Sprintf("Projects: All (%d)", len(m.projects))
	}

	if len(m.selectedProjects) <= 3 {
		return fmt.Sprintf("Projects: %s", strings.Join(m.selectedProjects, ", "))
	}

	return fmt.Sprintf("Projects: %d of %d selected", len(m.selectedProjects), len(m.projects))
}

func (m Model) renderFilterView() string {
	var b strings.Builder

	title := styles.TitleStyle.Render("Select Projects")
	b.WriteString(title)
	b.WriteString("\n\n")

	hint := styles.HelpStyle.Render("space: toggle, a: all, enter: apply, esc: cancel")
	b.WriteString(hint)
	b.WriteString("\n\n")

	for i, project := range m.projects {
		cursor := "  "
		if i == m.filterCursor {
			cursor = "> "
		}

		checkbox := "[ ]"
		if m.isProjectSelected(project) {
			checkbox = "[x]"
		}

		line := fmt.Sprintf("%s%s %s", cursor, checkbox, project)
		if i == m.filterCursor {
			line = styles.SelectedStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderCard(title string, count int) string {
	var countStr string
	if m.loading {
		countStr = lipgloss.NewStyle().Foreground(styles.Muted).Render("Loading...")
	} else {
		countStr = fmt.Sprintf("%d", count)
	}

	return fmt.Sprintf("%s\n%s",
		lipgloss.NewStyle().Bold(true).Render(title),
		countStr,
	)
}

// IsTextInputFocused returns false as dashboard has no text inputs
func (m Model) IsTextInputFocused() bool {
	return false
}
