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
	"sentinovo.ai/bizInt/internal/fetchctx"
	"sentinovo.ai/bizInt/internal/guard"
	"sentinovo.ai/bizInt/internal/ui/confirm"
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

type Model struct {
	client *ado.Client
	// writer is the guarded-write seam; New sets it to client and tests
	// replace it with a fake.
	writer       guard.Client
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

	autoRefresh bool

	// flow previews, confirms and executes PR writes through internal/guard.
	flow confirm.Flow
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
		writer:        client,
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
		ctx, cancel := fetchctx.New(budget)
		defer cancel()
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
	// The write flow sees every message first: while it is busy it owns the
	// keyboard, and its own messages never reach the view's bindings.
	var flowCmd tea.Cmd
	var res confirm.Result
	m.flow, flowCmd, res = m.flow.Update(msg)
	if res.Handled {
		if res.Status != "" || res.Proceed || res.Stay {
			m.statusMsg = res.Status
			m.statusErr = res.IsErr
		}
		if res.Refresh {
			m.loading = true
			return m, tea.Batch(flowCmd, m.fetchPRs())
		}
		return m, flowCmd
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table = m.table.WithTargetWidth(msg.Width - 4)

	case TickMsg:
		// Auto-refresh if enabled. Skipped while a guarded write is previewing,
		// awaiting confirm or in flight, so the rows cannot shift under it; the
		// tick is still re-armed.
		if m.autoRefresh && !m.loading && !m.flow.Busy() {
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

		switch msg.String() {
		case "/":
			m.filterFocused = 1
			m.searchInput.Focus()
			return m, nil
		case "a", "c":
			// Approve, or complete (merge), through the guard: pre-read, the
			// diff modal, then y (D-01, D-05, GUARD-02).
			if m.filterFocused == 0 {
				pr := m.getSelectedPR()
				if pr == nil {
					m.statusMsg = "No PR selected"
					m.statusErr = true
					return m, nil
				}
				b := confirm.Batch{
					ID:     pr.ID,
					Ops:    []guard.Op{guard.ApprovePR(m.writer, pr.Project, pr.RepositoryID, pr.ID)},
					Labels: []string{"approved"},
				}
				if msg.String() == "c" {
					// The no-op status names the terminal status the pre-read
					// saw: "already abandoned" or "already completed".
					seen := new(string)
					b.Ops = []guard.Op{guard.CompletePR(prStatusClient{Client: m.writer, seen: seen}, pr.Project, pr.RepositoryID, pr.ID)}
					b.Labels = []string{"completed"}
					b.NoOpLabel = func() string { return *seen }
				}
				var cmd tea.Cmd
				m.flow, cmd = m.flow.Start(b, m.client.FetchTimeout())
				m.statusMsg = fmt.Sprintf("Checking PR #%d...", pr.ID)
				m.statusErr = false
				return m, cmd
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

	// D-08: an unknown outcome stays on its own line above the status line
	// until the next keypress.
	if sticky := m.flow.Sticky(); sticky != "" {
		b.WriteString(styles.ErrorStyle.Render(sticky))
		b.WriteString("\n")
	}

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

	return m.flow.View(b.String())
}

func (m Model) buildRows() []table.Row {
	var rows []table.Row
	searchQuery := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))

	for _, item := range m.items {
		// Filter by configured projects. An empty configured slice disables the
		// filter; that rule now lives in ado.MatchesProject (CLI-06, D-18).
		if !ado.MatchesProject(item.Project, m.projects) {
			continue
		}

		// Filter by configured repositories (if any are defined)
		if !ado.MatchesRepository(item.RepositoryName, m.repositories) {
			continue
		}

		// Apply search filter. The comparison lives in ado.MatchesSearch
		// (CLI-06); the ID is formatted here because the matcher takes strings
		// only. The field set is unchanged: ID, title, repository, author.
		if !ado.MatchesSearch(searchQuery,
			fmt.Sprintf("%d", item.ID),
			item.Title,
			item.RepositoryName,
			item.CreatedBy,
		) {
			continue
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

// IsTextInputFocused returns true when the search input is focused or a
// guarded write owns the keyboard (preview, confirm modal or write in flight),
// so app.go holds tab navigation.
func (m Model) IsTextInputFocused() bool {
	return m.filterFocused == 1 || m.flow.Busy()
}

// WriteBusy reports whether a guarded write is previewing, awaiting confirm or
// in flight.
func (m Model) WriteBusy() bool { return m.flow.Busy() }

// prStatusClient records the PR status the complete pre-read saw.
// guard.CompletePR refuses completed and abandoned PRs alike as an empty diff
// (CLI parity) and guard.Report carries no pre-read state, so this is how the
// no-op status can say "already abandoned". seen is written inside the preview
// cmd and read only after PreviewedMsg is delivered.
type prStatusClient struct {
	guard.Client
	seen *string
}

func (c prStatusClient) GetPullRequestDetail(ctx context.Context, project, repositoryID string, prID int) (*ado.PullRequestDetail, error) {
	detail, err := c.Client.GetPullRequestDetail(ctx, project, repositoryID, prID)
	if detail != nil {
		*c.seen = strings.ToLower(detail.Status)
	}
	return detail, err
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
