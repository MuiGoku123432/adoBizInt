package workitems

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/ado"
)

// TestCloseDetailCancelsDownload pins WR-03. With request_timeout: 0 the SDK
// connection has no deadline, so a stalled download only ends when its context
// is cancelled. Closing the modal must cancel it; otherwise the download never
// returns and the operator has no way to stop it short of a restart.
func TestCloseDetailCancelsDownload(t *testing.T) {
	m := NewDetailModel(nil, 1)
	m.item = &ado.WorkItemDetail{Attachments: []ado.Attachment{{Name: "Spec.txt"}}}

	// The returned command is not run: it would call the nil client.
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if cmd == nil || !m.downloading || m.cancelDownload == nil {
		t.Fatalf("after d: cmd nil = %v, downloading = %v, cancelDownload nil = %v; want a cancellable download in flight",
			cmd == nil, m.downloading, m.cancelDownload == nil)
	}
	cancel := m.cancelDownload
	cancelled := false
	m.cancelDownload = func() { cancelled = true; cancel() }

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !cancelled {
		t.Error("closing the modal did not cancel the in-flight download")
	}
	if cmd == nil {
		t.Fatal("esc returned no command, want CloseDetailMsg")
	}
	if _, ok := cmd().(CloseDetailMsg); !ok {
		t.Errorf("esc command = %T, want CloseDetailMsg", cmd())
	}
}
