package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/config"
	"sentinovo.ai/bizInt/internal/keymap"
	"sentinovo.ai/bizInt/internal/ui/branches"
	"sentinovo.ai/bizInt/internal/ui/dashboard"
	"sentinovo.ai/bizInt/internal/ui/pipelines"
	"sentinovo.ai/bizInt/internal/ui/prs"
	"sentinovo.ai/bizInt/internal/ui/releases"
	"sentinovo.ai/bizInt/internal/ui/styles"
	"sentinovo.ai/bizInt/internal/ui/workitems"
)

type View int

const (
	DashboardView View = iota
	WorkItemsView
	PRsView
	PipelinesView
	ReleasesView
	BranchesView
)

type Model struct {
	client           *ado.Client
	orgURL           string
	projects         []string
	stateTransitions map[string]map[string]string
	keys             keymap.KeyMap
	help             help.Model
	dashboard        dashboard.Model
	workitems        workitems.Model
	prs              prs.Model
	pipelines        pipelines.Model
	releases         releases.Model
	branches         branches.Model
	view             View
	width            int
	height           int
	err              error
	// pendingLeave is the quit or tab key held while Work Items confirms a
	// staged edit (D-02). Every intercepted leave overwrites it, and only
	// workitems.ProceedMsg consumes it. Work Items emits that only for a
	// confirm ConfirmLeave started, so a stale key can never fire on its own.
	pendingLeave *tea.KeyMsg
}

func NewModel(client *ado.Client, orgURL string, projects []string, repositories []string, stateTransitions map[string]map[string]string, pollInterval time.Duration, priorities *config.Priorities) Model {
	return Model{
		client:           client,
		orgURL:           orgURL,
		projects:         projects,
		stateTransitions: stateTransitions,
		keys:             keymap.DefaultKeyMap(),
		help:             help.New(),
		dashboard:        dashboard.New(client, orgURL, projects),
		workitems:        workitems.New(client, orgURL, projects, stateTransitions, priorities),
		prs:              prs.New(client, orgURL, projects, repositories, pollInterval),
		pipelines:        pipelines.New(client, orgURL, projects, pollInterval),
		releases:         releases.New(client, orgURL, projects, pollInterval),
		branches:         branches.New(client, orgURL, projects, repositories),
		view:             DashboardView,
	}
}

func (m Model) Init() tea.Cmd {
	// Initialize dashboard on start
	return m.dashboard.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case workitems.ProceedMsg:
		if m.pendingLeave == nil {
			return m, nil
		}
		leave := *m.pendingLeave
		m.pendingLeave = nil
		return m.Update(leave)

	case tea.KeyMsg:
		quit := key.Matches(msg, m.keys.Quit)
		target, isTab := m.tabTarget(msg)
		tab := isTab && target != m.view && !m.isCurrentViewTextInputFocused()
		// Quit is held while a guarded write is busy: exiting mid-write could
		// cut its audit append (Phase 4 D-13), and an open confirm must be
		// answered with y, n or esc.
		if quit && m.currentViewWriteBusy() {
			return m, nil
		}
		// Leaving Work Items with a staged edit confirms it first (D-02). The
		// key is replayed when the confirm settles with ProceedMsg.
		if (quit || tab) && m.view == WorkItemsView && m.workitems.HasStaged() {
			leave := msg
			m.pendingLeave = &leave
			var cmd tea.Cmd
			m.workitems, cmd = m.workitems.ConfirmLeave()
			return m, cmd
		}

		switch {
		case quit:
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		}

		// Only handle tab navigation when no text input is focused
		if !m.isCurrentViewTextInputFocused() {
			switch {
			case key.Matches(msg, m.keys.Tab1):
				m.view = DashboardView
				return m, nil
			case key.Matches(msg, m.keys.Tab2):
				if m.view != WorkItemsView {
					m.view = WorkItemsView
					// Initialize work items if switching to it
					return m, m.workitems.Init()
				}
				return m, nil
			case key.Matches(msg, m.keys.Tab3):
				if m.view != PRsView {
					m.view = PRsView
					// Initialize PRs if switching to it
					return m, m.prs.Init()
				}
				return m, nil
			case key.Matches(msg, m.keys.Tab4):
				if m.view != PipelinesView {
					m.view = PipelinesView
					// Initialize pipelines if switching to it
					return m, m.pipelines.Init()
				}
				return m, nil
			case key.Matches(msg, m.keys.Tab5):
				if m.view != ReleasesView {
					m.view = ReleasesView
					// Initialize releases if switching to it
					return m, m.releases.Init()
				}
				return m, nil
			case key.Matches(msg, m.keys.Tab6):
				if m.view != BranchesView {
					m.view = BranchesView
					// Initialize branches if switching to it
					return m, m.branches.Init()
				}
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		// Propagate WindowSizeMsg to ALL sub-models
		var dashCmd, workitemsCmd, prsCmd, pipelinesCmd, releasesCmd, branchesCmd tea.Cmd
		m.dashboard, dashCmd = m.dashboard.Update(msg)
		m.workitems, workitemsCmd = m.workitems.Update(msg)
		m.prs, prsCmd = m.prs.Update(msg)
		m.pipelines, pipelinesCmd = m.pipelines.Update(msg)
		m.releases, releasesCmd = m.releases.Update(msg)
		m.branches, branchesCmd = m.branches.Update(msg)
		cmds = append(cmds, dashCmd, workitemsCmd, prsCmd, pipelinesCmd, releasesCmd, branchesCmd)
	}

	// Update current view (skip WindowSizeMsg since already handled above)
	if _, isWindowSize := msg.(tea.WindowSizeMsg); !isWindowSize {
		switch m.view {
		case DashboardView:
			var cmd tea.Cmd
			m.dashboard, cmd = m.dashboard.Update(msg)
			cmds = append(cmds, cmd)
		case WorkItemsView:
			var cmd tea.Cmd
			m.workitems, cmd = m.workitems.Update(msg)
			cmds = append(cmds, cmd)
		case PRsView:
			var cmd tea.Cmd
			m.prs, cmd = m.prs.Update(msg)
			cmds = append(cmds, cmd)
		case PipelinesView:
			var cmd tea.Cmd
			m.pipelines, cmd = m.pipelines.Update(msg)
			cmds = append(cmds, cmd)
		case ReleasesView:
			var cmd tea.Cmd
			m.releases, cmd = m.releases.Update(msg)
			cmds = append(cmds, cmd)
		case BranchesView:
			var cmd tea.Cmd
			m.branches, cmd = m.branches.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.err != nil {
		return styles.ErrorStyle.Render("Error: " + m.err.Error())
	}

	var content string
	switch m.view {
	case DashboardView:
		content = m.dashboard.View()
	case WorkItemsView:
		content = m.workitems.View()
	case PRsView:
		content = m.prs.View()
	case PipelinesView:
		content = m.pipelines.View()
	case ReleasesView:
		content = m.releases.View()
	case BranchesView:
		content = m.branches.View()
	default:
		content = "View not implemented"
	}

	helpView := m.help.View(m.keys)
	return content + "\n" + helpView
}

// tabTarget maps a tab key (1-6) to its view.
func (m Model) tabTarget(msg tea.KeyMsg) (View, bool) {
	switch {
	case key.Matches(msg, m.keys.Tab1):
		return DashboardView, true
	case key.Matches(msg, m.keys.Tab2):
		return WorkItemsView, true
	case key.Matches(msg, m.keys.Tab3):
		return PRsView, true
	case key.Matches(msg, m.keys.Tab4):
		return PipelinesView, true
	case key.Matches(msg, m.keys.Tab5):
		return ReleasesView, true
	case key.Matches(msg, m.keys.Tab6):
		return BranchesView, true
	}
	return 0, false
}

// currentViewWriteBusy reports whether the current view's guarded write is
// previewing, awaiting confirm or in flight. Dashboard and Branches never write.
func (m Model) currentViewWriteBusy() bool {
	switch m.view {
	case WorkItemsView:
		return m.workitems.WriteBusy()
	case PRsView:
		return m.prs.WriteBusy()
	case PipelinesView:
		return m.pipelines.WriteBusy()
	case ReleasesView:
		return m.releases.WriteBusy()
	default:
		return false
	}
}

// isCurrentViewTextInputFocused checks if the current view has a text input focused
func (m Model) isCurrentViewTextInputFocused() bool {
	switch m.view {
	case DashboardView:
		return m.dashboard.IsTextInputFocused()
	case WorkItemsView:
		return m.workitems.IsTextInputFocused()
	case PRsView:
		return m.prs.IsTextInputFocused()
	case PipelinesView:
		return m.pipelines.IsTextInputFocused()
	case ReleasesView:
		return m.releases.IsTextInputFocused()
	case BranchesView:
		return m.branches.IsTextInputFocused()
	default:
		return false
	}
}
