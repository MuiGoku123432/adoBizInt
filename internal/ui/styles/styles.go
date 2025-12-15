package styles

import "github.com/charmbracelet/lipgloss"

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
