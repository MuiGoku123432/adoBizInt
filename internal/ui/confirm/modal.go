package confirm

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sentinovo.ai/bizInt/internal/ui/styles"
	"sentinovo.ai/bizInt/internal/utils"
)

// WriteMsg is sent when the user presses y: write what the modal showed.
type WriteMsg struct{}

// DiscardMsg is sent when the user presses n: write nothing.
type DiscardMsg struct{}

// StayMsg is sent when the user presses esc: write nothing and stay.
type StayMsg struct{}

// Model is the D-01 diff modal: a title, one line per field change, and the
// [y] write  [n] discard  [esc] stay keys.
type Model struct {
	title string
	lines []string
}

// New builds the modal. Lines carry server-controlled values, so each has its
// control runes dropped and is truncated to 60 runes, the modal's content width
// (T-02-02/WR-04).
func New(title string, lines []string) Model {
	kept := make([]string, len(lines))
	for i, line := range lines {
		kept[i] = utils.Truncate(dropControl(line), 60)
	}
	return Model{title: title, lines: kept}
}

// Update maps y, n and esc to their messages. Every other key, including enter,
// space and Y, does nothing: only a deliberate y writes (T-07-14).
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "y":
		return m, func() tea.Msg { return WriteMsg{} }
	case "n":
		return m, func() tea.Msg { return DiscardMsg{} }
	case "esc":
		return m, func() tea.Msg { return StayMsg{} }
	}
	return m, nil
}

// View renders the modal in the trigger and effort modals' family. Width(64)
// with Padding(1, 2) leaves 60 content columns, equal to the 60-rune line cap,
// so no diff line wraps (07-UI-SPEC C1). There is no Height: the modal follows
// its content.
func (m Model) View() string {
	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(64)
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Width(60).
		Align(lipgloss.Center)

	var b strings.Builder
	b.WriteString(title.Render(m.title))
	b.WriteString("\n\n")
	b.WriteString(strings.Join(m.lines, "\n"))
	b.WriteString("\n")
	// HelpStyle's MarginTop(1) supplies the blank row above the key hint.
	b.WriteString(styles.HelpStyle.Render("[y] write  [n] discard  [esc] stay"))
	return frame.Render(b.String())
}

// dropControl removes control runes so server-controlled text cannot carry
// terminal escape sequences into the TUI (WR-04).
// note: internal/ui/workitems/detail.go has a twin of this inline strings.Map;
// two packages is below the rule of three, so it is not shared.
func dropControl(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}
