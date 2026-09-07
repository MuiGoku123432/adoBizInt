package styles

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"sentinovo.ai/bizInt/internal/utils"
)

// maxFailureMessage bounds the server-controlled portion of a rendered failure.
// WrappedError.Error() returns the message the server chose and for a rejected
// PAT can be an HTML sign-in fragment; an unbounded string here would let the
// server take over the terminal (threat T-02-02).
const maxFailureMessage = 120

var (
	// Colors
	Primary   = lipgloss.Color("#0078D4") // Azure blue
	Secondary = lipgloss.Color("#106EBE")
	Success   = lipgloss.Color("#107C10")
	Warning   = lipgloss.Color("#FFB900")
	Error     = lipgloss.Color("#D83B01")
	Muted     = lipgloss.Color("#605E5C")
	White     = lipgloss.Color("#FFFFFF")

	// Base styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(Muted).
			MarginBottom(1)

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(1, 2)

	// Status styles
	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success)

	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error)

	// Help style
	HelpStyle = lipgloss.NewStyle().
			Foreground(Muted).
			MarginTop(1)

	// Muted text style
	MutedStyle = lipgloss.NewStyle().
			Foreground(Muted)

	// Selected item style
	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(White).
			Background(Primary).
			Padding(0, 1)

	// Work item type colors
	TypeUserStory = lipgloss.Color("#0078D4") // Blue
	TypeTask      = lipgloss.Color("#107C10") // Green
	TypeBug       = lipgloss.Color("#D83B01") // Red/Orange
	TypeFeature   = lipgloss.Color("#8764B8") // Purple
	TypeEpic      = lipgloss.Color("#FF8C00") // Orange

	// Work item type styles
	UserStoryStyle = lipgloss.NewStyle().Foreground(TypeUserStory)
	TaskStyle      = lipgloss.NewStyle().Foreground(TypeTask)
	BugStyle       = lipgloss.NewStyle().Foreground(TypeBug)
	FeatureStyle   = lipgloss.NewStyle().Foreground(TypeFeature)
	EpicStyle      = lipgloss.NewStyle().Foreground(TypeEpic)
)

// The three renderers below are shared rather than inlined per view: all six
// views need identical failure treatment in this phase, which is past the rule
// of three, so extracting here is convention compliance rather than speculative
// generality. In each case the classified kind is the prominent element and the
// server-controlled message is secondary and truncated (threat T-02-02).

// RenderAuthBanner renders a blocking banner for a rejected credential. It
// replaces the data table rather than sitting above it: an auth failure is not
// partial data, it is no data.
func RenderAuthBanner(kind, project, message string) string {
	headline := fmt.Sprintf("%s failure — %s could not be read", kind, project)
	body := utils.Truncate(message, maxFailureMessage)
	return ErrorStyle.Render(headline) + "\n" + MutedStyle.Render(body)
}

// RenderPartialWarning renders the persistent warning line shown above rows that
// are known to be incomplete.
func RenderPartialWarning(summary string) string {
	return WarningStyle.Render("Partial results — " + utils.Truncate(summary, maxFailureMessage))
}

// RenderStaleMarker marks rows that survived a failed refresh and are therefore
// no longer known to be current.
func RenderStaleMarker() string {
	return MutedStyle.Render("(stale — last successful refresh shown)")
}
