package workitems

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/logging"
	"sentinovo.ai/bizInt/internal/ui/styles"
)

// DetailLoadedMsg is sent when work item details are fetched
type DetailLoadedMsg struct {
	Item *ado.WorkItemDetail
	Err  error
}

// CloseDetailMsg signals the detail modal should close
type CloseDetailMsg struct{}

// DetailModel represents the work item detail popup
type DetailModel struct {
	client   *ado.Client
	item     *ado.WorkItemDetail
	itemID   int
	loading  bool
	err      error
	viewport viewport.Model
	width    int
	height   int
	ready    bool
}

// NewDetailModel creates a new detail modal for a work item
func NewDetailModel(client *ado.Client, itemID int) DetailModel {
	return DetailModel{
		client:  client,
		itemID:  itemID,
		loading: true,
	}
}

// Init initializes the detail modal and fetches work item details
func (m DetailModel) Init() tea.Cmd {
	log := logging.Logger()
	log.Debug("DetailModel.Init called", "itemID", m.itemID)
	return m.fetchDetail()
}

func (m DetailModel) fetchDetail() tea.Cmd {
	id := m.itemID
	client := m.client
	log := logging.Logger()
	log.Debug("fetchDetail called", "itemID", id)
	return func() tea.Msg {
		log.Debug("fetchDetail goroutine starting", "itemID", id)
		detail, err := client.GetWorkItemDetail(context.Background(), id)
		if err != nil {
			log.Debug("fetchDetail error", "itemID", id, "error", err)
		} else {
			log.Debug("fetchDetail success", "itemID", id, "hasDetail", detail != nil)
		}
		return DetailLoadedMsg{Item: detail, Err: err}
	}
}

// Update handles messages for the detail modal
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewport()

	case DetailLoadedMsg:
		log := logging.Logger()
		log.Debug("DetailLoadedMsg received", "hasItem", msg.Item != nil, "hasErr", msg.Err != nil)
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.item = msg.Item
			m.updateViewport()
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return m, func() tea.Msg { return CloseDetailMsg{} }
		case "up", "k":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		case "down", "j":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		case "pgup":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		case "pgdown":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *DetailModel) updateViewport() {
	if m.item == nil {
		return
	}

	// Modal dimensions
	modalWidth := min(m.width-4, 100)
	modalHeight := min(m.height-6, 40)
	contentWidth := modalWidth - 4 // Account for borders and padding

	// Build content
	content := m.buildContent(contentWidth)

	// Initialize or update viewport
	if !m.ready {
		m.viewport = viewport.New(contentWidth, modalHeight-6) // Leave room for header and footer
		m.viewport.SetContent(content)
		m.ready = true
	} else {
		m.viewport.Width = contentWidth
		m.viewport.Height = modalHeight - 6
		m.viewport.SetContent(content)
	}
}

func (m DetailModel) buildContent(width int) string {
	if m.item == nil {
		return ""
	}

	var b strings.Builder
	item := m.item

	// Title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	b.WriteString(titleStyle.Render(item.Title))
	b.WriteString("\n\n")

	// Description
	if item.Description != "" {
		b.WriteString(styles.SubtitleStyle.Render("Description:"))
		b.WriteString("\n")
		// Strip HTML and wrap text
		desc := stripHTML(item.Description)
		b.WriteString(wrapText(desc, width))
		b.WriteString("\n\n")
	}

	// Acceptance Criteria (for User Stories)
	if item.AcceptanceCriteria != "" {
		b.WriteString(styles.SubtitleStyle.Render("Acceptance Criteria:"))
		b.WriteString("\n")
		ac := stripHTML(item.AcceptanceCriteria)
		b.WriteString(wrapText(ac, width))
		b.WriteString("\n\n")
	}

	// Repro Steps (for Bugs)
	if item.ReproSteps != "" {
		b.WriteString(styles.SubtitleStyle.Render("Repro Steps:"))
		b.WriteString("\n")
		repro := stripHTML(item.ReproSteps)
		b.WriteString(wrapText(repro, width))
		b.WriteString("\n\n")
	}

	// Iteration/Area paths
	if item.IterationPath != "" {
		b.WriteString(fmt.Sprintf("Iteration: %s\n", item.IterationPath))
	}
	if item.AreaPath != "" {
		b.WriteString(fmt.Sprintf("Area: %s\n", item.AreaPath))
	}

	// Dates
	if !item.CreatedDate.IsZero() {
		b.WriteString(fmt.Sprintf("\nCreated: %s by %s\n", item.CreatedDate.Format("2006-01-02 15:04"), item.CreatedBy))
	}
	if !item.ModifiedDate.IsZero() {
		b.WriteString(fmt.Sprintf("Modified: %s by %s\n", item.ModifiedDate.Format("2006-01-02 15:04"), item.ModifiedBy))
	}

	// Attachments
	if len(item.Attachments) > 0 {
		b.WriteString("\n")
		b.WriteString(styles.SubtitleStyle.Render(fmt.Sprintf("Attachments (%d):", len(item.Attachments))))
		b.WriteString("\n")
		for _, att := range item.Attachments {
			sizeStr := formatSize(att.Size)
			b.WriteString(fmt.Sprintf("  - %s (%s)\n", att.Name, sizeStr))
		}
	}

	return b.String()
}

// View renders the detail modal
func (m DetailModel) View() string {
	log := logging.Logger()
	log.Debug("DetailModel.View called", "width", m.width, "height", m.height, "loading", m.loading, "hasItem", m.item != nil)

	if m.width == 0 || m.height == 0 {
		log.Debug("DetailModel.View returning early - zero dimensions")
		return styles.SubtitleStyle.Render("Loading...")
	}

	// Modal dimensions
	modalWidth := min(m.width-4, 100)
	modalHeight := min(m.height-6, 40)

	// Modal border style
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1).
		Width(modalWidth).
		Height(modalHeight)

	var content string

	if m.loading {
		content = lipgloss.Place(modalWidth-2, modalHeight-2,
			lipgloss.Center, lipgloss.Center,
			styles.SubtitleStyle.Render("Loading..."))
	} else if m.err != nil {
		content = lipgloss.Place(modalWidth-2, modalHeight-2,
			lipgloss.Center, lipgloss.Center,
			styles.ErrorStyle.Render("Error: "+m.err.Error()))
	} else if m.item != nil {
		// Header
		header := m.renderHeader(modalWidth - 4)

		// Footer
		footer := m.renderFooter(modalWidth - 4)

		// Combine header + viewport + footer
		content = lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.viewport.View(),
			footer,
		)
	}

	// Render modal - overlay handles centering
	return borderStyle.Render(content)
}

func (m DetailModel) renderHeader(width int) string {
	if m.item == nil {
		return ""
	}

	item := m.item

	// Title bar with work item info
	titleText := fmt.Sprintf("Work Item #%d", item.ID)
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Width(width).
		Align(lipgloss.Center)

	// Type and State row
	typeStyle := getTypeStyle(item.Type)
	infoLine := fmt.Sprintf("%s  |  State: %s  |  Assignee: %s",
		typeStyle.Render(item.Type),
		item.State,
		truncate(item.AssignedTo, 20))
	if item.StoryPoints > 0 {
		infoLine += fmt.Sprintf("  |  Points: %d", item.StoryPoints)
	}

	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(strings.Repeat("─", width))

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(titleText),
		infoLine,
		separator,
		"",
	)
}

func (m DetailModel) renderFooter(width int) string {
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(strings.Repeat("─", width))

	scrollInfo := ""
	if m.ready {
		scrollInfo = fmt.Sprintf(" %d%%", int(m.viewport.ScrollPercent()*100))
	}

	helpText := styles.HelpStyle.Render(fmt.Sprintf("[Esc]close  [↑↓]scroll%s", scrollInfo))

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		separator,
		helpText,
	)
}

// wrapText wraps text to fit within the given width
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		// Simple word wrap
		words := strings.Fields(line)
		currentLine := ""
		for _, word := range words {
			if currentLine == "" {
				currentLine = word
			} else if len(currentLine)+1+len(word) <= width {
				currentLine += " " + word
			} else {
				result.WriteString(currentLine)
				result.WriteString("\n")
				currentLine = word
			}
		}
		if currentLine != "" {
			result.WriteString(currentLine)
		}
	}

	return result.String()
}

// formatSize formats bytes into human readable format
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
