package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/keymap"
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
	view             View
	width            int
	height           int
	err              error
}

func NewModel(client *ado.Client, orgURL string, projects []string, stateTransitions map[string]map[string]string, pollInterval time.Duration) Model {
	return Model{
		client:           client,
		orgURL:           orgURL,
		projects:         projects,
		stateTransitions: stateTransitions,
		keys:             keymap.DefaultKeyMap(),
		help:             help.New(),
		dashboard:        dashboard.New(client, orgURL, projects),
		workitems:        workitems.New(client, orgURL, projects, stateTransitions),
		prs:              prs.New(client, orgURL, projects),
		pipelines:        pipelines.New(client, orgURL, projects, pollInterval),
		releases:         releases.New(client, orgURL, projects, pollInterval),
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
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
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
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		// Propagate WindowSizeMsg to ALL sub-models
		var dashCmd, workitemsCmd, prsCmd, pipelinesCmd, releasesCmd tea.Cmd
		m.dashboard, dashCmd = m.dashboard.Update(msg)
		m.workitems, workitemsCmd = m.workitems.Update(msg)
		m.prs, prsCmd = m.prs.Update(msg)
		m.pipelines, pipelinesCmd = m.pipelines.Update(msg)
		m.releases, releasesCmd = m.releases.Update(msg)
		cmds = append(cmds, dashCmd, workitemsCmd, prsCmd, pipelinesCmd, releasesCmd)
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
	default:
		content = "View not implemented"
	}

	helpView := m.help.View(m.keys)
	return content + "\n" + helpView
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
	default:
		return false
	}
}
