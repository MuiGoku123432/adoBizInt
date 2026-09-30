package ado

import (
	"context"
	"errors"
	"go/ast"
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

// TestGetWorkItemDetailAttachmentID pins D-05 and D-06: only an AttachedFile
// relation with a name and a non-nil GUID in its last URL path segment becomes
// an Attachment, and the ones that fail are dropped without a failure.
func TestGetWorkItemDetailAttachmentID(t *testing.T) {
	validID := uuid.MustParse("098a279a-60b9-40a8-868b-b7fd00c0a439")
	base := "https://dev.azure.com/fabrikam/_apis/wit/attachments/"
	attached := "AttachedFile"
	hyperlink := "Hyperlink"
	str := func(s string) *string { return &s }
	attrs := func(name string, size float64) *map[string]interface{} {
		return &map[string]interface{}{"name": name, "resourceSize": size}
	}

	relations := []workitemtracking.WorkItemRelation{
		{Rel: &attached, Url: str(base + validID.String() + "?fileName=Spec.txt"), Attributes: attrs("Spec.txt", 7)},
		{Rel: &attached, Url: str(base + "not-a-guid"), Attributes: attrs("NotGuid.txt", 1)},
		{Rel: &attached, Url: nil, Attributes: attrs("NoURL.txt", 1)},
		{Rel: &attached, Url: str(base + uuid.New().String()), Attributes: attrs("", 1)},
		{Rel: &attached, Url: str(base + uuid.Nil.String()), Attributes: attrs("Zero.txt", 1)},
		{Rel: &hyperlink, Url: str(base + uuid.New().String()), Attributes: attrs("Link.txt", 1)},
	}

	client := &Client{
		workitemClient: &fakeWorkItemAPI{
			t: t,
			GetWorkItemFunc: func(context.Context, workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
				return &workitemtracking.WorkItem{
					Fields:    &map[string]interface{}{"System.Title": "Mixed relations"},
					Relations: &relations,
				}, nil
			},
		},
	}

	detail, err := client.GetWorkItemDetail(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetWorkItemDetail() error = %v, want nil", err)
	}

	want := []Attachment{{
		ID:   validID,
		Name: "Spec.txt",
		URL:  base + validID.String() + "?fileName=Spec.txt",
		Size: 7,
	}}
	if !reflect.DeepEqual(detail.Attachments, want) {
		t.Errorf("Attachments = %+v, want %+v", detail.Attachments, want)
	}
}

// TestNoRawHTTPInADO keeps the PAT inside the SDK connection. It fails if any
// non-test file in this package imports net/http, reads AuthorizationString, or
// declares a struct field of type *azuredevops.Connection.
//
// This is an AST walk rather than a grep for a reason that is specific to this
// repository: STATE.md records three Phase 3 incidents where a "grep returns
// nothing" criterion was satisfied and then quietly defeated by a prose comment
// explaining the forbidden thing. Comments and string literals are not part of
// the AST, so neither a doc line nor a message string can satisfy or defeat this
// assertion. It is ROADMAP Phase 5 SC1 ("the PAT cannot reach an arbitrary
// host") expressed structurally, and the struct-field rule keeps RESEARCH OQ4's
// deleted Client.connection from coming back.
func TestNoRawHTTPInADO(t *testing.T) {
	fset, files := parsePackageFiles(t)
	for _, file := range files {
		for _, imp := range file.Imports {
			if imp.Path.Value == `"net/http"` {
				t.Errorf("%s imports net/http; attachment and API traffic must go through the SDK", fset.Position(imp.Pos()))
			}
		}

		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if n.Sel.Name == "AuthorizationString" {
					t.Errorf("%s reads AuthorizationString; the PAT header belongs to the SDK", fset.Position(n.Pos()))
				}
			case *ast.StructType:
				// Only struct fields: newConnection legitimately returns the
				// connection, and its result list is also an *ast.Field.
				for _, f := range n.Fields.List {
					if isConnectionPointer(f.Type) {
						t.Errorf("%s declares a *azuredevops.Connection struct field; no struct may hold the credential-bearing connection", fset.Position(f.Pos()))
					}
				}
			}
			return true
		})
	}

	if files["attachments.go"] == nil {
		t.Errorf("parsed files do not include attachments.go; the structural assertion covered nothing")
	}
}

// isConnectionPointer reports whether expr is the type *azuredevops.Connection.
func isConnectionPointer(expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "azuredevops" && sel.Sel.Name == "Connection"
}
