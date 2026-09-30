package ado

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
)

// TestDownloadAttachmentFetchesByID drives the TUI's real path at the ado layer:
// GetWorkItemDetail parses the attachment GUID out of the relation URL, and
// DownloadAttachment fetches by that GUID through the SDK seam (HARD-01, D-01).
func TestDownloadAttachmentFetchesByID(t *testing.T) {
	want := uuid.MustParse("098a279a-60b9-40a8-868b-b7fd00c0a439")
	rel := "AttachedFile"
	relURL := "https://dev.azure.com/fabrikam/_apis/wit/attachments/" + want.String() + "?fileName=Spec.txt"

	var got workitemtracking.GetAttachmentContentArgs
	calls := 0
	client := &Client{
		workitemClient: &fakeWorkItemAPI{
			t: t,
			GetWorkItemFunc: func(context.Context, workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
				return &workitemtracking.WorkItem{
					Fields: &map[string]interface{}{"System.Title": "Has attachment"},
					Relations: &[]workitemtracking.WorkItemRelation{{
						Rel:        &rel,
						Url:        &relURL,
						Attributes: &map[string]interface{}{"name": "Spec.txt", "resourceSize": float64(7)},
					}},
				}, nil
			},
			GetAttachmentContentFunc: func(_ context.Context, args workitemtracking.GetAttachmentContentArgs) (io.ReadCloser, error) {
				calls++
				got = args
				return io.NopCloser(strings.NewReader("payload")), nil
			},
		},
	}

	detail, err := client.GetWorkItemDetail(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetWorkItemDetail() error = %v, want nil", err)
	}
	if len(detail.Attachments) != 1 {
		t.Fatalf("GetWorkItemDetail() attachments = %d, want 1", len(detail.Attachments))
	}
	att := detail.Attachments[0]
	if att.ID != want || att.Name != "Spec.txt" {
		t.Fatalf("attachment = %+v, want ID %s and Name Spec.txt", att, want)
	}

	destDir := t.TempDir()
	if err := client.DownloadAttachment(context.Background(), att.ID, destDir, att.Name); err != nil {
		t.Fatalf("DownloadAttachment() error = %v, want nil", err)
	}

	if calls != 1 {
		t.Errorf("GetAttachmentContent calls = %d, want 1", calls)
	}
	if !reflect.DeepEqual(got, workitemtracking.GetAttachmentContentArgs{Id: &want}) {
		t.Errorf("GetAttachmentContent args = %+v, want only Id %s", got, want)
	}
	body, err := os.ReadFile(filepath.Join(destDir, "Spec.txt"))
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(body) != "payload" {
		t.Errorf("downloaded bytes = %q, want %q", body, "payload")
	}
}

// TestAttachmentPath table-tests the containment guard directly on the D-08
// hostile set. Every row asserts the invariant: an error, or a direct child of
// destDir.
func TestAttachmentPath(t *testing.T) {
	destDir := t.TempDir()

	cases := map[string]struct {
		name    string
		destDir string // empty means use t.TempDir()
		want    string // expected base name under destDir; ignored when osDep or wantErr
		wantErr bool
		// On Linux and macOS a backslash is a literal file-name byte; on Windows
		// filepath.Base strips it. Either answer satisfies HARD-02 (RESEARCH OQ3),
		// so these rows assert containment only.
		osDep bool
	}{
		"plain name":          {name: "Spec.txt", want: "Spec.txt"},
		"nested traversal":    {name: "../../.ssh/authorized_keys", want: "authorized_keys"},
		"absolute name":       {name: "/etc/passwd", want: "passwd"},
		"parent with slash":   {name: "../", wantErr: true},
		"parent":              {name: "..", wantErr: true},
		"dot":                 {name: ".", wantErr: true},
		"empty":               {name: "", wantErr: true},
		"embedded NUL":        {name: "a\x00b", wantErr: true},
		"all separators":      {name: "///", wantErr: true},
		"relative destDir":    {name: "Spec.txt", destDir: "attachments", wantErr: true},
		"backslash parent":    {name: `..\`, osDep: true},
		"backslash traversal": {name: `..\..\evil`, osDep: true},
		"all backslashes":     {name: `\\\`, osDep: true},
	}

	for label, tc := range cases {
		t.Run(label, func(t *testing.T) {
			dir := destDir
			if tc.destDir != "" {
				dir = tc.destDir
			}

			got, err := attachmentPath(tc.name, dir)

			if err == nil && filepath.Dir(got) != filepath.Clean(dir) {
				t.Fatalf("attachmentPath(%q) = %q, not a direct child of %q", tc.name, got, dir)
			}
			if tc.osDep {
				return
			}
			if tc.wantErr {
				if err == nil {
					t.Fatalf("attachmentPath(%q) = %q, want an error", tc.name, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("attachmentPath(%q) error = %v, want nil", tc.name, err)
			}
			if want := filepath.Join(dir, tc.want); got != want {
				t.Errorf("attachmentPath(%q) = %q, want %q", tc.name, got, want)
			}
		})
	}
}

// TestDownloadAttachmentRefusesBeforeFetch proves the guard runs before the
// request: GetAttachmentContentFunc is left nil, so the fake's own t.Fatalf is
// the "no network call" assertion (D-08 integration half).
func TestDownloadAttachmentRefusesBeforeFetch(t *testing.T) {
	cases := map[string]string{
		"parent with slash": "../",
		"parent":            "..",
		"dot":               ".",
		"empty":             "",
		"embedded NUL":      "a\x00b",
		"all separators":    "///",
	}

	for label, name := range cases {
		t.Run(label, func(t *testing.T) {
			destDir := t.TempDir()
			client := &Client{workitemClient: &fakeWorkItemAPI{t: t}}

			if err := client.DownloadAttachment(context.Background(), uuid.New(), destDir, name); err == nil {
				t.Fatalf("DownloadAttachment(%q) error = nil, want a refusal", name)
			}
			entries, err := os.ReadDir(destDir)
			if err != nil {
				t.Fatalf("read destDir: %v", err)
			}
			if len(entries) != 0 {
				t.Errorf("destDir holds %d entries after a refusal, want 0", len(entries))
			}
		})
	}
}

// failingBody serves a fixed prefix, then fails, the way a connection timeout
// interrupts a body mid-read. It records whether Close was called.
type failingBody struct {
	prefix *strings.Reader
	err    error
	closed bool
}

func (b *failingBody) Read(p []byte) (int, error) {
	if b.prefix.Len() > 0 {
		return b.prefix.Read(p)
	}
	return 0, b.err
}

func (b *failingBody) Close() error {
	b.closed = true
	return nil
}

// TestDownloadAttachmentRemovesPartialFile pins that a mid-body failure leaves
// no truncated file behind that could pass for a completed download.
func TestDownloadAttachmentRemovesPartialFile(t *testing.T) {
	errMidBody := errors.New("injected mid-body failure")
	body := &failingBody{prefix: strings.NewReader("part"), err: errMidBody}
	client := &Client{
		workitemClient: &fakeWorkItemAPI{
			t: t,
			GetAttachmentContentFunc: func(context.Context, workitemtracking.GetAttachmentContentArgs) (io.ReadCloser, error) {
				return body, nil
			},
		},
	}
	destDir := t.TempDir()

	err := client.DownloadAttachment(context.Background(), uuid.New(), destDir, "Spec.txt")

	if !errors.Is(err, errMidBody) {
		t.Fatalf("DownloadAttachment() error = %v, want %v", err, errMidBody)
	}
	if _, err := os.Stat(filepath.Join(destDir, "Spec.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("partial file stat error = %v, want not-exist", err)
	}
	if !body.closed {
		t.Error("response body was not closed")
	}
}
