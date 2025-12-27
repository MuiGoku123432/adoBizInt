package pipelines

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/ui/styles"
)

// CloseTriggerMsg signals the trigger modal should close
type CloseTriggerMsg struct{}

// TriggerSubmitMsg is sent when user submits the trigger form
type TriggerSubmitMsg struct {
	DefinitionID int
	PipelineName string
	Branch       string
	Project      string
}

// BranchesMsg is sent when branches are fetched
type BranchesMsg struct {
	Branches []string
	Err      error
}

// TriggerModal represents the pipeline trigger popup
type TriggerModal struct {
	client      *ado.Client
	definitions []ado.PipelineDefinition
	branches    []string

	// Selection state
	selectedDef    int // Index into definitions
	selectedBranch int // Index into branches
	mode           int // 0 = selecting pipeline, 1 = selecting branch

	// State
	loading bool
	err     error
	width   int
	height  int
}

func NewTriggerModal(client *ado.Client, definitions []ado.PipelineDefinition) TriggerModal {
	return TriggerModal{
		client:      client,
		definitions: definitions,
		mode:        0,
	}
}

func (m TriggerModal) Init() tea.Cmd {
	return nil
}

func (m TriggerModal) Update(msg tea.Msg) (TriggerModal, tea.Cmd) {
	// Early validation - ensure definitions exist
	if len(m.definitions) == 0 {
		m.err = fmt.Errorf("no pipeline definitions available")
		return m, nil
	}

	// Ensure selectedDef is in bounds
	if m.selectedDef >= len(m.definitions) {
		m.selectedDef = len(m.definitions) - 1
	}
	if m.selectedDef < 0 {
		m.selectedDef = 0
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case BranchesMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.branches = msg.Branches
			m.selectedBranch = 0
			// Find default branch index (with bounds check)
			if m.selectedDef < len(m.definitions) {
				def := m.definitions[m.selectedDef]
				for i, b := range m.branches {
					if b == def.DefaultBranch || b == "main" || b == "master" {
						m.selectedBranch = i
						break
					}
				}
			}
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.mode == 1 {
				// Go back to pipeline selection
				m.mode = 0
				m.branches = nil
				m.err = nil
				return m, nil
			}
			return m, func() tea.Msg { return CloseTriggerMsg{} }

		case "up", "k":
			if m.mode == 0 && m.selectedDef > 0 {
				m.selectedDef--
			} else if m.mode == 1 && m.selectedBranch > 0 {
				m.selectedBranch--
			}

		case "down", "j":
			if m.mode == 0 && m.selectedDef < len(m.definitions)-1 {
				m.selectedDef++
			} else if m.mode == 1 && m.selectedBranch < len(m.branches)-1 {
				m.selectedBranch++
			}

		case "enter":
			if m.mode == 0 {
				// Move to branch selection, fetch branches
				m.mode = 1
				m.loading = true
				m.err = nil
				return m, m.fetchBranches()
			} else if m.mode == 1 && len(m.branches) > 0 {
				// Submit trigger
				def := m.definitions[m.selectedDef]
				branch := m.branches[m.selectedBranch]
				return m, func() tea.Msg {
					return TriggerSubmitMsg{
						DefinitionID: def.ID,
						PipelineName: def.Name,
						Branch:       branch,
						Project:      def.Project,
					}
				}
			}
		}
	}

	return m, nil
}

func (m TriggerModal) fetchBranches() tea.Cmd {
	// Bounds check before accessing definitions
	if m.selectedDef < 0 || m.selectedDef >= len(m.definitions) {
		return func() tea.Msg {
			return BranchesMsg{Err: fmt.Errorf("invalid pipeline selection")}
		}
	}

	def := m.definitions[m.selectedDef]
	client := m.client
	project := def.Project
	repoID := def.RepositoryID

	// Validate repository ID exists
	if repoID == "" {
		return func() tea.Msg {
			return BranchesMsg{Err: fmt.Errorf("pipeline '%s' has no associated repository", def.Name)}
		}
	}

	return func() tea.Msg {
		branches, err := client.GetBranchesForPipeline(
			context.Background(),
			project,
			repoID,
		)
		return BranchesMsg{Branches: branches, Err: err}
	}
}

func (m TriggerModal) View() string {
	if m.width == 0 || m.height == 0 {
		return styles.SubtitleStyle.Render("Loading...")
	}

	modalWidth := 50
	modalHeight := 18

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Width(modalWidth - 6).
		Align(lipgloss.Center)

	if m.mode == 0 {
		b.WriteString(titleStyle.Render("Select Pipeline"))
	} else {
		b.WriteString(titleStyle.Render("Select Branch"))
	}
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(styles.SubtitleStyle.Render("Loading branches..."))
	} else if m.err != nil {
		b.WriteString(styles.ErrorStyle.Render("Error: " + m.err.Error()))
		b.WriteString("\n\n")
		b.WriteString(styles.HelpStyle.Render("[Esc] back"))
	} else if m.mode == 0 {
		// Pipeline selection
		maxDisplay := 10
		start := 0
		if m.selectedDef >= maxDisplay {
			start = m.selectedDef - maxDisplay + 1
		}
		end := start + maxDisplay
		if end > len(m.definitions) {
			end = len(m.definitions)
		}

		for i := start; i < end; i++ {
			def := m.definitions[i]
			cursor := "  "
			if i == m.selectedDef {
				cursor = "> "
			}
			line := fmt.Sprintf("%s%s", cursor, def.Name)
			if def.Project != "" && len(m.definitions) > 1 {
				line += fmt.Sprintf(" (%s)", def.Project)
			}
			if i == m.selectedDef {
				line = styles.SelectedStyle.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}

		if len(m.definitions) > maxDisplay {
			b.WriteString(fmt.Sprintf("\n... showing %d-%d of %d", start+1, end, len(m.definitions)))
		}
	} else {
		// Branch selection
		def := m.definitions[m.selectedDef]
		b.WriteString(styles.SubtitleStyle.Render(fmt.Sprintf("Pipeline: %s", def.Name)))
		b.WriteString("\n\n")

		if len(m.branches) == 0 {
			b.WriteString(styles.MutedStyle.Render("No branches found"))
		} else {
			maxDisplay := 10
			start := 0
			if m.selectedBranch >= maxDisplay {
				start = m.selectedBranch - maxDisplay + 1
			}
			end := start + maxDisplay
			if end > len(m.branches) {
				end = len(m.branches)
			}

			for i := start; i < end; i++ {
				branch := m.branches[i]
				cursor := "  "
				if i == m.selectedBranch {
					cursor = "> "
				}
				line := fmt.Sprintf("%s%s", cursor, branch)
				if i == m.selectedBranch {
					line = styles.SelectedStyle.Render(line)
				}
				b.WriteString(line)
				b.WriteString("\n")
			}

			if len(m.branches) > maxDisplay {
				b.WriteString(fmt.Sprintf("\n... showing %d-%d of %d", start+1, end, len(m.branches)))
			}
		}
	}

	b.WriteString("\n\n")

	// Help
	if m.mode == 0 {
		b.WriteString(styles.HelpStyle.Render("[j/k] select  [Enter] next  [Esc] cancel"))
	} else {
		b.WriteString(styles.HelpStyle.Render("[j/k] select  [Enter] trigger  [Esc] back"))
	}

	return borderStyle.Render(b.String())
}
