package ado

import (
	"context"
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
