package prs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/ui/link"
	"sentinovo.ai/bizInt/internal/ui/styles"
	"sentinovo.ai/bizInt/internal/utils"
)

const (
	columnKeyID      = "id"
	columnKeyTitle   = "title"
	columnKeyProject = "project"
	columnKeyRepo    = "repo"
	columnKeySource  = "source"
	columnKeyTarget  = "target"
	columnKeyAuthor  = "author"
	columnKeyStatus  = "status"
)

// PRsMsg is sent when pull requests are fetched. Result carries the items plus
// whatever failed and whether the read is the whole picture; Err is reserved for
// a genuine transport failure that produced no envelope at all.
type PRsMsg struct {
	Result ado.Result[ado.PullRequest]
	Err    error
}

// TickMsg is used for auto-refresh polling
type TickMsg struct{}

// PRActionMsg is sent when a PR action (approve/complete) completes
type PRActionMsg struct {
	PRID   int
	Action string // "approve" or "complete"
	Err    error
}

type Model struct {
	client       *ado.Client
	orgURL       string
	projects     []string
	repositories []string
	pollInterval time.Duration

	table       table.Model
	searchInput textinput.Model
	items       []ado.PullRequest

	loading bool
	err     error
	// authBlocked suppresses the table entirely: a rejected credential is not
	// partial data. partialWarning is persistent because the status line is
	// already used for action feedback and is too easy to miss across a
	// 30-second refresh. stale marks rows that survived a failed refresh.
	authBlocked    bool
	authBanner     string
	partialWarning string
	stale          bool
	complete       bool

	width         int
	height        int
	filterFocused int // 0 = table, 1 = search
	statusMsg     string
	statusErr     bool
	actionPending bool

	// confirmComplete is set while awaiting confirmation of a merge
	confirmComplete bool

	autoRefresh bool
}

func New(client *ado.Client, orgURL string, projects []string, repositories []string, pollInterval time.Duration) Model {
	// Create search input
	search := textinput.New()
	search.Placeholder = "Search PRs..."
	search.Width = 30

	// Create table with columns
	columns := []table.Column{
		table.NewColumn(columnKeyID, "ID", 6).WithStyle(lipgloss.NewStyle().Align(lipgloss.Right)),
		table.NewColumn(columnKeyTitle, "Title", 34),
		table.NewColumn(columnKeyProject, "Project", 12),
		table.NewColumn(columnKeyRepo, "Repository", 15),
		table.NewColumn(columnKeySource, "Source", 15),
		table.NewColumn(columnKeyTarget, "Target", 12),
		table.NewColumn(columnKeyAuthor, "Author", 15),
		table.NewColumn(columnKeyStatus, "Approval", 10),
	}

	t := table.New(columns).
		WithBaseStyle(lipgloss.NewStyle().Align(lipgloss.Left)).
		WithTargetWidth(80).
		BorderRounded().
		Focused(true).
		WithPageSize(15)

	return Model{
		client:        client,
		orgURL:        orgURL,
		projects:      projects,
		repositories:  repositories,
		pollInterval:  pollInterval,
		table:         t,
		searchInput:   search,
		loading:       true,
		filterFocused: 0,
		autoRefresh:   true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetchPRs(), m.tickCmd())
}

func (m Model) tickCmd() tea.Cmd {
	if m.pollInterval <= 0 {
		m.pollInterval = 30 * time.Second
	}
	return tea.Tick(m.pollInterval, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

func (m Model) fetchPRs() tea.Cmd {
	projects := m.projects
	client := m.client
	budget := client.FetchTimeout()
	return func() tea.Msg {
		ctx := context.Background()
		if budget > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, budget)
			defer cancel()
		}
		result, err := client.GetPullRequests(ctx, projects)
		return PRsMsg{Result: result, Err: err}
	}
}

// applyResult folds an aggregate read envelope into render state. An auth-kind
// failure blocks the table (D-14); any other failure keeps the rows and raises a
// persistent warning (D-13).
func (m *Model) applyResult(result ado.Result[ado.PullRequest]) {
	m.items = result.Items
	m.table = m.table.WithRows(m.buildRows())
	m.complete = result.Complete
	m.authBlocked = false
	m.authBanner = ""
	m.partialWarning = ""

	if len(result.Failures) == 0 {
		return
	}

	for _, failure := range result.Failures {
		if failure.Kind == ado.FailureAuth {
			message := ""
			if failure.Err != nil {
				message = failure.Err.Error()
			}
			m.authBlocked = true
			m.authBanner = styles.RenderAuthBanner(string(failure.Kind), failure.Project, message)
			return
		}
	}

	parts := make([]string, 0, len(result.Failures))
	for _, failure := range result.Failures {
		parts = append(parts, fmt.Sprintf("%s (%s)", failure.Project, failure.Kind))
	}
	m.partialWarning = "could not read: " + strings.Join(parts, ", ")
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table = m.table.WithTargetWidth(msg.Width - 4)

	case TickMsg:
		// Auto-refresh if enabled. Skipped while a merge confirmation is pending,
		// so the rows cannot shift between the prompt and the confirming keypress.
		if m.autoRefresh && !m.loading && !m.actionPending && !m.confirmComplete {
			m.loading = true
			cmds = append(cmds, m.fetchPRs(), m.tickCmd())
		} else {
			cmds = append(cmds, m.tickCmd())
		}

	case PRsMsg:
		m.loading = false
		if msg.Err != nil {
			// The previously loaded rows stay on screen; they are simply no
			// longer known to be current.
			m.err = msg.Err
			m.stale = len(m.items) > 0
		} else {
			m.err = nil
			m.stale = false
			m.applyResult(msg.Result)
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

	case link.ActionMsg:
		if msg.Err != nil {
			m.statusMsg = "Error: " + msg.Err.Error()
			m.statusErr = true
		} else if msg.Action == "copy" {
			m.statusMsg = "Copied link to clipboard"
			m.statusErr = false
		} else if msg.Action == "open" {
			m.statusMsg = "Opened in browser"
			m.statusErr = false
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

		// A pending merge confirmation swallows the next keypress, so it must be
		// checked before the regular bindings (notably "y", which copies a link).
		if m.confirmComplete {
			m.confirmComplete = false
			if msg.String() == "y" {
				cmd := m.completeSelectedPR()
				return m, cmd
			}
			m.statusMsg = "Merge cancelled"
			m.statusErr = false
			return m, nil
		}

		switch msg.String() {
		case "/":
			m.filterFocused = 1
			m.searchInput.Focus()
			return m, nil
		case "a":
			// Approve selected PR
			if m.filterFocused == 0 && !m.actionPending {
				cmd := m.approveSelectedPR()
				return m, cmd
			}
		case "c":
			// Completing a PR merges it, so ask for confirmation first
			if m.filterFocused == 0 && !m.actionPending {
				pr := m.getSelectedPR()
				if pr == nil {
					m.statusMsg = "No PR selected"
					m.statusErr = true
					return m, nil
				}
				m.confirmComplete = true
				m.statusMsg = fmt.Sprintf("Merge PR #%d %s? [y] confirm  [any other key] cancel",
					pr.ID, utils.Truncate(pr.Title, 40))
				m.statusErr = false
				return m, nil
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
		case "r":
			// Refresh PR list
			m.loading = true
			m.statusMsg = ""
			return m, m.fetchPRs()
		case "y":
			// Copy link to clipboard when table is focused
			if m.filterFocused == 0 {
				return m, m.copySelectedPRLink()
			}
		case "o":
			// Open in browser when table is focused
			if m.filterFocused == 0 {
				return m, m.openSelectedPRInBrowser()
			}
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

	// Title with auto-refresh indicator
	title := "Pull Requests"
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

	// A rejected credential replaces the table rather than annotating it: with no
	// data read, an empty table would assert something the tool does not know.
	if m.authBlocked {
		b.WriteString(m.authBanner)
		b.WriteString("\n")
	} else if m.partialWarning != "" {
		b.WriteString(styles.RenderPartialWarning(m.partialWarning))
		b.WriteString("\n")
	}
	if m.stale {
		b.WriteString(styles.RenderStaleMarker())
		b.WriteString("\n")
	}

	// Show loading or table. Once rows exist a refresh keeps them on screen and
	// only annotates the count line, rather than blanking the table.
	if m.authBlocked {
		// Nothing further: the banner above is the whole answer.
	} else if m.loading && len(m.items) == 0 {
		b.WriteString(styles.SubtitleStyle.Render("Loading pull requests..."))
	} else if len(m.items) == 0 {
		// Only a completed read may assert an absence of work.
		if m.complete {
			b.WriteString(styles.SubtitleStyle.Render("No open pull requests found"))
		}
	} else {
		// Show filtered count
		filteredCount := len(m.buildRows())
		totalCount := len(m.items)
		countInfo := styles.SubtitleStyle.Render(fmt.Sprintf("Showing %d of %d PRs", filteredCount, totalCount))
		b.WriteString(countInfo)
		if m.loading {
			b.WriteString(styles.SubtitleStyle.Render(" (refreshing...)"))
		}
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
	helpText := styles.HelpStyle.Render("[/] search  [a] approve  [c] merge  [r] refresh  [t] auto  [y] copy link  [o] open")
	b.WriteString(helpText)

	return b.String()
}

func (m Model) buildRows() []table.Row {
	var rows []table.Row
	searchQuery := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))

	// Create a set of valid projects for quick lookup
	validProjects := make(map[string]bool)
	for _, p := range m.projects {
		validProjects[strings.ToLower(p)] = true
	}

	// Create a set of valid repositories for quick lookup
	validRepos := make(map[string]bool)
	for _, r := range m.repositories {
		validRepos[strings.ToLower(r)] = true
	}

	for _, item := range m.items {
		// Filter by configured projects
		if len(validProjects) > 0 && !validProjects[strings.ToLower(item.Project)] {
			continue
		}

		// Filter by configured repositories (if any are defined)
		if len(validRepos) > 0 && !validRepos[strings.ToLower(item.RepositoryName)] {
			continue
		}

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

		// Style draft PRs differently. The "[Draft] " prefix is 8 chars, so the
		// title is trimmed to keep both variants inside the 34-wide column.
		titleDisplay := utils.Truncate(item.Title, 32)
		if item.IsDraft {
			titleDisplay = "[Draft] " + utils.Truncate(item.Title, 24)
		}

		// Format approval status with color
		statusDisplay := getApprovalStyle(item.ApprovalStatus).Render(item.ApprovalStatus)

		rows = append(rows, table.NewRow(table.RowData{
			columnKeyID:      fmt.Sprintf("%d", item.ID),
			columnKeyTitle:   titleDisplay,
			columnKeyProject: utils.Truncate(item.Project, 10),
			columnKeyRepo:    utils.Truncate(item.RepositoryName, 13),
			columnKeySource:  utils.Truncate(item.SourceBranch, 13),
			columnKeyTarget:  utils.Truncate(item.TargetBranch, 10),
			columnKeyAuthor:  utils.Truncate(item.CreatedBy, 13),
			columnKeyStatus:  statusDisplay,
		}))
	}
	return rows
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

// IsTextInputFocused returns true when the search input is focused
func (m Model) IsTextInputFocused() bool {
	return m.filterFocused == 1
}

// copySelectedPRLink copies the URL of the selected PR to clipboard
func (m *Model) copySelectedPRLink() tea.Cmd {
	pr := m.getSelectedPR()
	if pr == nil {
		return link.Fail("copy", fmt.Errorf("no PR selected"))
	}
	return link.Copy(utils.BuildPRURL(m.orgURL, pr.Project, pr.RepositoryName, pr.ID))
}

// openSelectedPRInBrowser opens the selected PR in the default browser
func (m *Model) openSelectedPRInBrowser() tea.Cmd {
	pr := m.getSelectedPR()
	if pr == nil {
		return link.Fail("open", fmt.Errorf("no PR selected"))
	}
	return link.Open("open", utils.BuildPRURL(m.orgURL, pr.Project, pr.RepositoryName, pr.ID))
}
