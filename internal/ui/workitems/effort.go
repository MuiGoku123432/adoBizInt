package workitems

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/ui/styles"
)

// EffortSubmitMsg is sent when user submits the effort form
type EffortSubmitMsg struct {
	ItemID           int
	TargetTaskID     int // Child task ID for User Stories (0 if none found)
	NewState         string
	OriginalEstimate float64
	Remaining        float64
	Completed        float64
	IsUserStory      bool // True if this is a User Story completion flow
	SkipTaskUpdate   bool // True if no dev task found and user chose to proceed anyway
}

// CloseEffortMsg signals the effort modal should close without saving
type CloseEffortMsg struct{}

// ChildTasksLoadedMsg is sent when child tasks are fetched for a User Story
type ChildTasksLoadedMsg struct {
	ParentID    int
	ParentTitle string
	NewState    string
	Children    []ado.WorkItem
	Err         error
}

// EffortModal represents the effort input popup for Tasks and User Stories
type EffortModal struct {
	itemID           int
	itemTitle        string
	newState         string
	originalEstimate textinput.Model
	remaining        textinput.Model
	completed        textinput.Model
	focusIndex       int // 0, 1, 2 for the 3 fields
	width            int
	height           int
	err              string
	// User Story support fields
	isUserStory     bool
	targetTaskID    int    // The dev task ID to update (0 if no dev task found)
	targetTaskTitle string // The dev task title for display
	warningMsg      string // Warning shown when no dev task found
}

// NewEffortModal creates a new effort input modal
func NewEffortModal(itemID int, itemTitle, newState string) EffortModal {
	// Original Estimate input
	origEst := textinput.New()
	origEst.Placeholder = "0"
	origEst.Width = 10
	origEst.CharLimit = 10
	origEst.Focus()

	// Remaining input
	remaining := textinput.New()
	remaining.Placeholder = "0"
	remaining.Width = 10
	remaining.CharLimit = 10

	// Completed input
	completed := textinput.New()
	completed.Placeholder = "0"
	completed.Width = 10
	completed.CharLimit = 10

	return EffortModal{
		itemID:           itemID,
		itemTitle:        itemTitle,
		newState:         newState,
		originalEstimate: origEst,
		remaining:        remaining,
		completed:        completed,
		focusIndex:       0,
	}
}

// NewEffortModalForUserStory creates an effort modal for User Story completion
// targetTaskID is 0 if no development task was found
func NewEffortModalForUserStory(userStoryID int, userStoryTitle, newState string, targetTaskID int, targetTaskTitle string) EffortModal {
	m := NewEffortModal(userStoryID, userStoryTitle, newState)
	m.isUserStory = true
	m.targetTaskID = targetTaskID
	m.targetTaskTitle = targetTaskTitle
	if targetTaskID == 0 {
		m.warningMsg = "No development task found. Hours will not be recorded."
	}
	return m
}

// Init initializes the effort modal
func (m EffortModal) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages for the effort modal
func (m EffortModal) Update(msg tea.Msg) (EffortModal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return CloseEffortMsg{} }

		case "tab", "down":
			m.focusIndex = (m.focusIndex + 1) % 3
			m.updateFocus()
			return m, nil

		case "shift+tab", "up":
			m.focusIndex = (m.focusIndex + 2) % 3
			m.updateFocus()
			return m, nil

		case "enter":
			// Validate and submit
			origEst, err1 := m.parseFloat(m.originalEstimate.Value())
			remaining, err2 := m.parseFloat(m.remaining.Value())
			completed, err3 := m.parseFloat(m.completed.Value())

			if err1 != nil || err2 != nil || err3 != nil {
				m.err = "Please enter valid numbers for all fields"
				return m, nil
			}

			// Validate non-zero completed hours (required for both Task and User Story)
			if completed <= 0 {
				m.err = "Completed hours must be greater than zero"
				return m, nil
			}

			return m, func() tea.Msg {
				return EffortSubmitMsg{
					ItemID:           m.itemID,
					TargetTaskID:     m.targetTaskID,
					NewState:         m.newState,
					OriginalEstimate: origEst,
					Remaining:        remaining,
					Completed:        completed,
					IsUserStory:      m.isUserStory,
					SkipTaskUpdate:   m.targetTaskID == 0,
				}
			}
		}
	}

	// Update the focused input
	var cmd tea.Cmd
	switch m.focusIndex {
	case 0:
		m.originalEstimate, cmd = m.originalEstimate.Update(msg)
	case 1:
		m.remaining, cmd = m.remaining.Update(msg)
	case 2:
		m.completed, cmd = m.completed.Update(msg)
	}

	return m, cmd
}

func (m *EffortModal) updateFocus() {
	m.originalEstimate.Blur()
	m.remaining.Blur()
	m.completed.Blur()

	switch m.focusIndex {
	case 0:
		m.originalEstimate.Focus()
	case 1:
		m.remaining.Focus()
	case 2:
		m.completed.Focus()
	}
}

func (m EffortModal) parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}

// View renders the effort modal
func (m EffortModal) View() string {
	if m.width == 0 || m.height == 0 {
		return styles.SubtitleStyle.Render("Loading...")
	}

	// Modal dimensions - adjust height for User Story with extra info
	modalWidth := 50
	modalHeight := 16
	if m.isUserStory {
		modalHeight = 18
	}

	// Modal border style
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	var b strings.Builder

	// Title - different for User Story vs Task
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Width(modalWidth - 6).
		Align(lipgloss.Center)

	if m.isUserStory {
		b.WriteString(titleStyle.Render("Enter Development Hours"))
	} else {
		b.WriteString(titleStyle.Render("Enter Effort (Hours)"))
	}
	b.WriteString("\n\n")

	// Item info - show both User Story and target Task for User Story flow
	if m.isUserStory {
		storyInfo := styles.SubtitleStyle.Render(fmt.Sprintf("Story #%d: %s", m.itemID, truncate(m.itemTitle, 30)))
		b.WriteString(storyInfo)
		b.WriteString("\n")
		if m.targetTaskID > 0 {
			taskInfo := styles.SubtitleStyle.Render(fmt.Sprintf("Dev Task #%d: %s", m.targetTaskID, truncate(m.targetTaskTitle, 26)))
			b.WriteString(taskInfo)
			b.WriteString("\n")
		}
	} else {
		taskInfo := styles.SubtitleStyle.Render(fmt.Sprintf("Task #%d: %s", m.itemID, truncate(m.itemTitle, 30)))
		b.WriteString(taskInfo)
		b.WriteString("\n")
	}
	b.WriteString(styles.SubtitleStyle.Render(fmt.Sprintf("Transitioning to: %s", m.newState)))
	b.WriteString("\n")

	// Warning message if no dev task found
	if m.warningMsg != "" {
		warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208")) // Orange warning color
		b.WriteString(warningStyle.Render(m.warningMsg))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Input fields
	labelStyle := lipgloss.NewStyle().Width(20)
	selectedLabelStyle := labelStyle.Copy().Bold(true).Foreground(styles.Primary)

	// Original Estimate
	label := labelStyle.Render("Original Estimate:")
	if m.focusIndex == 0 {
		label = selectedLabelStyle.Render("Original Estimate:")
	}
	b.WriteString(fmt.Sprintf("%s %s\n", label, m.originalEstimate.View()))

	// Remaining
	label = labelStyle.Render("Remaining:")
	if m.focusIndex == 1 {
		label = selectedLabelStyle.Render("Remaining:")
	}
	b.WriteString(fmt.Sprintf("%s %s\n", label, m.remaining.View()))

	// Completed
	label = labelStyle.Render("Completed:")
	if m.focusIndex == 2 {
		label = selectedLabelStyle.Render("Completed:")
	}
	b.WriteString(fmt.Sprintf("%s %s\n", label, m.completed.View()))

	// Error message
	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(styles.ErrorStyle.Render(m.err))
	}

	b.WriteString("\n")

	// Help
	helpText := styles.HelpStyle.Render("[Tab] next field  [Enter] submit  [Esc] cancel")
	b.WriteString(helpText)

	return borderStyle.Render(b.String())
}
