package releases

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/ui/styles"
)

// CloseApprovalMsg signals the approval modal should close
type CloseApprovalMsg struct{}

// ApprovalSubmitMsg is sent when user submits the approval form
type ApprovalSubmitMsg struct {
	ApprovalID int
	Project    string
	Action     string // "approve" or "reject"
	Comment    string
}

// ApprovalModal represents the approval popup
type ApprovalModal struct {
	approval     ado.ReleaseApproval
	commentInput textinput.Model
	action       string // "approve" or "reject" - toggled by user
	width        int
	height       int
}

func NewApprovalModal(approval ado.ReleaseApproval) ApprovalModal {
	comment := textinput.New()
	comment.Placeholder = "Optional comment..."
	comment.Width = 40
	comment.Focus()

	return ApprovalModal{
		approval:     approval,
		commentInput: comment,
		action:       "approve", // Default to approve
	}
}

func (m ApprovalModal) Init() tea.Cmd {
	return nil
}

func (m ApprovalModal) Update(msg tea.Msg) (ApprovalModal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return CloseApprovalMsg{} }
		case "tab":
			// Toggle between approve and reject
			if m.action == "approve" {
				m.action = "reject"
			} else {
				m.action = "approve"
			}
		case "enter":
			return m, func() tea.Msg {
				return ApprovalSubmitMsg{
					ApprovalID: m.approval.ID,
					Project:    m.approval.Project,
					Action:     m.action,
					Comment:    m.commentInput.Value(),
				}
			}
		default:
			var cmd tea.Cmd
			m.commentInput, cmd = m.commentInput.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m ApprovalModal) View() string {
	if m.width == 0 || m.height == 0 {
		return styles.SubtitleStyle.Render("Loading...")
	}

	modalWidth := 55
	modalHeight := 16

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

	if m.action == "approve" {
		b.WriteString(titleStyle.Render("Approve Deployment"))
	} else {
		b.WriteString(titleStyle.Render("Reject Deployment"))
	}
	b.WriteString("\n\n")

	// Release info
	b.WriteString(styles.SubtitleStyle.Render(fmt.Sprintf("Release: %s", m.approval.ReleaseName)))
	b.WriteString("\n")
	b.WriteString(styles.SubtitleStyle.Render(fmt.Sprintf("Environment: %s", m.approval.EnvironmentName)))
	b.WriteString("\n")
	b.WriteString(styles.SubtitleStyle.Render(fmt.Sprintf("Type: %s", m.approval.ApprovalType)))
	b.WriteString("\n\n")

	// Action toggle
	approveStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	rejectStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	if m.action == "approve" {
		approveStyle = lipgloss.NewStyle().Foreground(styles.Success).Bold(true)
	} else {
		rejectStyle = lipgloss.NewStyle().Foreground(styles.Error).Bold(true)
	}

	b.WriteString("Action: ")
	b.WriteString(approveStyle.Render("[Approve]"))
	b.WriteString(" / ")
	b.WriteString(rejectStyle.Render("[Reject]"))
	b.WriteString("\n\n")

	// Comment input
	b.WriteString("Comment: ")
	b.WriteString(m.commentInput.View())
	b.WriteString("\n\n")

	// Help
	b.WriteString(styles.HelpStyle.Render("[Tab] toggle action  [Enter] submit  [Esc] cancel"))

	return borderStyle.Render(b.String())
}
