package workitems

import (
	"strings"
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

// TestDetailNeverRendersRawControlBytes pins WR-04's display half. Anyone who
// can attach a file in the organization chooses its name, so a name carrying
// ESC or C1 sequences must not reach the operator's terminal from the
// attachment list, and the success status names the file actually written,
// quoted.
func TestDetailNeverRendersRawControlBytes(t *testing.T) {
	m := NewDetailModel(nil, 1)
	m.item = &ado.WorkItemDetail{Attachments: []ado.Attachment{{Name: "a\x1b]52;c;ZXZpbA==\x07\u009bb.png"}}}

	// Only the attachment's own line is checked: the section heading is
	// styled, and lipgloss emits ANSI of its own when a terminal is attached.
	content := m.buildContent(80)
	want := "> a]52;c;ZXZpbA==b.png (0 B)"
	found := false
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "> ") {
			found = true
			if line != want {
				t.Errorf("attachment line = %q, want %q (control runes dropped)", line, want)
			}
		}
	}
	if !found {
		t.Fatalf("no attachment line in %q", content)
	}

	m, _ = m.Update(AttachmentDownloadedMsg{Name: "x", Path: "/home/op/attachments/Spec (1).txt"})
	if want := `Downloaded to "/home/op/attachments/Spec (1).txt"`; m.downloadStatus != want {
		t.Errorf("downloadStatus = %q, want %q", m.downloadStatus, want)
	}
}
