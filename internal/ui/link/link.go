// Package link provides the commands each view uses to copy an Azure DevOps
// URL to the clipboard or open it in a browser, plus the message they report
// back with.
package link

import (
	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/utils"
)

// ActionMsg is sent when a link action (copy/open/pr) completes.
type ActionMsg struct {
	Action string // "copy", "open" or "pr"
	URL    string
	Err    error
}

// Fail reports an action that could not be attempted.
func Fail(action string, err error) tea.Cmd {
	return func() tea.Msg {
		return ActionMsg{Action: action, Err: err}
	}
}

// Copy puts url on the system clipboard.
func Copy(url string) tea.Cmd {
	return func() tea.Msg {
		return ActionMsg{Action: "copy", URL: url, Err: utils.CopyToClipboard(url)}
	}
}

// Open launches url in the default browser. action distinguishes opening the
// resource itself ("open") from opening a derived page such as PR creation ("pr").
func Open(action, url string) tea.Cmd {
	return func() tea.Msg {
		return ActionMsg{Action: action, URL: url, Err: utils.OpenInBrowser(url)}
	}
}
