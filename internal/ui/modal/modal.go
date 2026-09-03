// Package modal composites a pre-rendered modal on top of a pre-rendered
// background view.
package modal

import (
	tea "github.com/charmbracelet/bubbletea"
	overlay "github.com/rmhubbert/bubbletea-overlay"
)

// view adapts a pre-rendered string to the tea.Model interface the overlay
// package expects.
type view struct {
	content string
}

func (v view) Init() tea.Cmd                       { return nil }
func (v view) Update(tea.Msg) (tea.Model, tea.Cmd) { return v, nil }
func (v view) View() string                        { return v.content }

// Center renders fg centered over bg.
func Center(fg, bg string) string {
	o := overlay.New(view{content: fg}, view{content: bg}, overlay.Center, overlay.Center, 0, 0)
	return o.View()
}
