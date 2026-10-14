// Package confirm is the TUI side of a guarded write: preview, confirm and
// execute through internal/guard. It is shared by the four write-capable views
// (PRs, releases, pipelines, work items), so the rule of three is met.
package confirm

// PreviewedMsg carries a batch after its dry-run pre-read.
type PreviewedMsg struct{}

// WrittenMsg carries a batch after execution.
type WrittenMsg struct{}

// WriteMsg is sent when the user presses y in the confirm modal.
type WriteMsg struct{}
