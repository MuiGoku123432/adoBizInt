package ui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/keymap"
	"sentinovo.ai/bizInt/internal/ui/dashboard"
	"sentinovo.ai/bizInt/internal/ui/styles"
	"sentinovo.ai/bizInt/internal/ui/workitems"
)

type View int

const (
	DashboardView View = iota
	WorkItemsView
	PRsView
	PipelinesView
)

type Model struct {
	client           *ado.Client
	projects         []string
	stateTransitions map[string]map[string]string
	keys             keymap.KeyMap
	help             help.Model
	dashboard        dashboard.Model
	workitems        workitems.Model
	view             View
	width            int
	height           int
	err              error
}

func NewModel(client *ado.Client, orgURL string, projects []string, stateTransitions map[string]map[string]string) Model {
	return Model{
		client:           client,
		projects:         projects,
		stateTransitions: stateTransitions,
		keys:             keymap.DefaultKeyMap(),
		help:             help.New(),
		dashboard:        dashboard.New(client, orgURL, projects),
		workitems:        workitems.New(client, projects, stateTransitions),
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
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
	}

	// Update current view
	switch m.view {
	case DashboardView:
		var cmd tea.Cmd
		m.dashboard, cmd = m.dashboard.Update(msg)
		cmds = append(cmds, cmd)
	case WorkItemsView:
		var cmd tea.Cmd
		m.workitems, cmd = m.workitems.Update(msg)
		cmds = append(cmds, cmd)
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
	default:
		content = "View not implemented"
	}

	helpView := m.help.View(m.keys)
	return content + "\n" + helpView
}
