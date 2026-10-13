package branches

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/fetchctx"
	"sentinovo.ai/bizInt/internal/ui/link"
	"sentinovo.ai/bizInt/internal/ui/styles"
	"sentinovo.ai/bizInt/internal/utils"
)

const (
	columnKeyRepo        = "repo"
	columnKeyBranch      = "branch"
	columnKeyLastCommit  = "lastCommit"
	columnKeyAuthor      = "author"
	columnKeyAheadBehind = "aheadBehind"
	columnKeyActions     = "actions"

	// columnKeyIndex is carried on each row (not rendered) to map back to m.items
	columnKeyIndex = "index"
)

// BranchesMsg is sent when branches are fetched. Result carries the items plus
// whatever failed and whether the read is the whole picture; Err is reserved for
// a transport failure that produced no envelope at all.
type BranchesMsg struct {
	Result ado.Result[ado.Branch]
	Err    error
}

type Model struct {
	client        *ado.Client
	orgURL        string
	projects      []string
	repositories  []string
	table         table.Model
	searchInput   textinput.Model
	items         []ado.Branch
	loading       bool
	err           error
	width         int
	height        int
	filterFocused int // 0 = table, 1 = search
	statusMsg     string
	statusErr     bool

	// authBlocked suppresses the table entirely: a rejected credential is not
	// partial data. partialWarning is persistent because the status line is
	// already used for action feedback. Branches is not a polling view, so
	// D-15's stale marker does not apply here — D-15 names the three polling
	// views specifically, and a marker on a manually refreshed table would
	// assert a staleness the view has no timer to measure.
	authBlocked    bool
	authBanner     string
	partialWarning string
	complete       bool
}

func New(client *ado.Client, orgURL string, projects []string, repositories []string) Model {
	// Create search input
	search := textinput.New()
	search.Placeholder = "Search branches..."
	search.Width = 30

	// Create table with columns
	columns := []table.Column{
		table.NewColumn(columnKeyRepo, "Repository", 22),
		table.NewColumn(columnKeyBranch, "Branch", 30),
		table.NewColumn(columnKeyLastCommit, "Last Commit", 16),
		table.NewColumn(columnKeyAuthor, "Author", 14),
		table.NewColumn(columnKeyAheadBehind, "Ahead/Behind", 12),
		table.NewColumn(columnKeyActions, "Actions", 14),
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
		table:         t,
		searchInput:   search,
		loading:       true,
		filterFocused: 0,
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchBranches()
}

func (m Model) fetchBranches() tea.Cmd {
	client := m.client
	projects := m.projects
	repositories := m.repositories
	budget := client.FetchTimeout()
	return func() tea.Msg {
		ctx, cancel := fetchctx.New(budget)
		defer cancel()
		result, err := client.GetBranches(ctx, projects, repositories)
		return BranchesMsg{Result: result, Err: err}
	}
}

// applyResult folds the read envelope into render state. An auth-kind failure
// blocks the table (D-14); any other failure keeps the rows and raises a
// persistent warning (D-13). The warning carries the operation label because
// 02-02 encodes the repository name there.
func (m *Model) applyResult(result ado.Result[ado.Branch]) {
	m.items = result.Items
	m.table = m.table.WithRows(m.buildRows())
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

	m.partialWarning = "could not read: " + styles.SummarizeFailures(result.Failures)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table = m.table.WithTargetWidth(msg.Width - 4)

	case BranchesMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.err = nil
			m.applyResult(msg.Result)
		}

	case link.ActionMsg:
		if msg.Err != nil {
			m.statusMsg = "Error: " + msg.Err.Error()
			m.statusErr = true
		} else {
			switch msg.Action {
			case "copy":
				m.statusMsg = "Copied link to clipboard"
			case "open":
				m.statusMsg = "Opened branch in browser"
			case "pr":
				m.statusMsg = "Opened new pull request in browser"
			}
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
		case "r":
			// Refresh branch list
			m.loading = true
			m.statusMsg = ""
			return m, m.fetchBranches()
		case "y":
			// Copy branch link to clipboard when table is focused
			if m.filterFocused == 0 {
				return m, m.copySelectedBranchLink()
			}
		case "o":
			// Open branch in browser when table is focused
			if m.filterFocused == 0 {
				return m, m.openSelectedBranchInBrowser()
			}
		case "p":
			// Open a new pull request for the branch when table is focused
			if m.filterFocused == 0 {
				return m, m.openPullRequestForSelectedBranch()
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

func (m Model) getSelectedBranch() *ado.Branch {
	highlightedRow := m.table.HighlightedRow()
	if highlightedRow.Data == nil {
		return nil
	}

	idx, ok := highlightedRow.Data[columnKeyIndex].(int)
	if !ok || idx < 0 || idx >= len(m.items) {
		return nil
	}
	return &m.items[idx]
}

func (m Model) View() string {
	var b strings.Builder

	// Title
	title := styles.TitleStyle.Render("Branches")
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

	// A rejected credential replaces the table rather than annotating it: with no
	// data read, an empty table would assert something the tool does not know.
	if m.authBlocked {
		b.WriteString(m.authBanner)
		b.WriteString("\n")
	} else if m.partialWarning != "" {
		b.WriteString(styles.RenderPartialWarning(m.partialWarning))
		b.WriteString("\n")
	}

	// Show loading or table
	if m.authBlocked {
		// Nothing further: the banner above is the whole answer.
	} else if m.loading {
		b.WriteString(styles.SubtitleStyle.Render("Loading branches..."))
	} else if len(m.items) == 0 {
		// Only a completed read may assert an absence of work.
		if m.complete {
			b.WriteString(styles.SubtitleStyle.Render("No branches found"))
		}
	} else {
		// Show filtered count
		filteredCount := len(m.buildRows())
		totalCount := len(m.items)
		countInfo := styles.SubtitleStyle.Render(fmt.Sprintf("Showing %d of %d branches (newest first)", filteredCount, totalCount))
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
	helpText := styles.HelpStyle.Render("[/] search  [r] refresh  [o] open branch  [p] new pull request  [y] copy link")
	b.WriteString(helpText)

	return b.String()
}

func (m Model) buildRows() []table.Row {
	var rows []table.Row
	searchQuery := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))

	// The loop index is consumed downstream by the row-selection bookkeeping, so
	// this stays `for i, item :=` where the other four views use `for _, x :=`.
	for i, item := range m.items {
		// Apply search filter. The comparison lives in ado.MatchesSearch
		// (CLI-06). The field set is unchanged: name, repository, last committer.
		if !ado.MatchesSearch(searchQuery, item.Name, item.RepositoryName, item.LastCommitBy) {
			continue
		}

		lastCommit := "-"
		if !item.LastCommitDate.IsZero() {
			lastCommit = item.LastCommitDate.Format("2006-01-02 15:04")
		}

		// The default branch is the PR target, so it gets no PR action
		nameDisplay := utils.Truncate(item.Name, 28)
		actions := "[o]pen [p]r"
		if item.IsDefault {
			nameDisplay = utils.Truncate(item.Name, 18) + " (default)"
			actions = "[o]pen"
		}

		rows = append(rows, table.NewRow(table.RowData{
			columnKeyRepo:        utils.Truncate(item.RepositoryName, 20),
			columnKeyBranch:      nameDisplay,
			columnKeyLastCommit:  lastCommit,
			columnKeyAuthor:      utils.Truncate(item.LastCommitBy, 12),
			columnKeyAheadBehind: fmt.Sprintf("%d / %d", item.AheadCount, item.BehindCount),
			columnKeyActions:     actions,
			columnKeyIndex:       i,
		}))
	}
	return rows
}

// IsTextInputFocused returns true when the search input is focused
func (m Model) IsTextInputFocused() bool {
	return m.filterFocused == 1
}

// copySelectedBranchLink copies the URL of the selected branch to clipboard
func (m *Model) copySelectedBranchLink() tea.Cmd {
	branch := m.getSelectedBranch()
	if branch == nil {
		return link.Fail("copy", fmt.Errorf("no branch selected"))
	}
	return link.Copy(utils.BuildBranchURL(m.orgURL, branch.Project, branch.RepositoryName, branch.Name))
}

// openSelectedBranchInBrowser opens the selected branch in Azure DevOps
func (m *Model) openSelectedBranchInBrowser() tea.Cmd {
	branch := m.getSelectedBranch()
	if branch == nil {
		return link.Fail("open", fmt.Errorf("no branch selected"))
	}
	return link.Open("open", utils.BuildBranchURL(m.orgURL, branch.Project, branch.RepositoryName, branch.Name))
}

// openPullRequestForSelectedBranch opens the Azure DevOps new pull request page with
// the selected branch as the source and the repository default branch as the target
func (m *Model) openPullRequestForSelectedBranch() tea.Cmd {
	branch := m.getSelectedBranch()
	if branch == nil {
		return link.Fail("pr", fmt.Errorf("no branch selected"))
	}
	if branch.IsDefault {
		return link.Fail("pr", fmt.Errorf("%s is the repository default branch", branch.Name))
	}

	target := branch.DefaultBranch
	if target == "" {
		target = "main"
	}
	return link.Open("pr", utils.BuildPRCreateURL(m.orgURL, branch.Project, branch.RepositoryName, branch.Name, target))
}
