package ado

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
)

// attachmentPath resolves a server-supplied attachment name to a file path that
// is a direct child of destDir, or refuses it (HARD-02, D-03, D-08).
//
// It is a pure function on purpose, so the hostile-name table can test it
// directly. This is a two-use extraction, short of the rule of three: the
// download calls it now, and the deferred CLI attachment verb will call it
// later. The guard is the feature, so keep it in one place.
//
// The name is formatted with %q in every error so control characters from the
// server cannot reach the operator's terminal.
func attachmentPath(name, destDir string) (string, error) {
	if !filepath.IsAbs(destDir) {
		return "", fmt.Errorf("attachment destination %q is not an absolute directory", destDir)
	}
	if strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("attachment name %q contains a NUL byte", name)
	}
	base := filepath.Base(name) // Base("") is "."
	if base == "." || base == ".." || filepath.IsAbs(base) {
		return "", fmt.Errorf("attachment name %q is not a usable file name", name)
	}
	dir := filepath.Clean(destDir)
	path := filepath.Join(dir, base)
	if filepath.Dir(path) != dir {
		return "", fmt.Errorf("attachment name %q escapes %s", name, dir)
	}
	return path, nil
}

// DownloadAttachment fetches the attachment with the given GUID and writes it
// to destDir/name.
//
// The SDK's GetAttachmentContent owns the host, the auth header and the
// connection timeout, so no argument here can send the PAT elsewhere (D-01).
// The name is checked by attachmentPath before any request is made (D-03).
// There is no byte cap; it is deferred with the CLI verb (D-04). A refused name
// is a plain error, the same as a network failure (D-07).
//
// If the copy or the final close fails, the partial file is removed so a
// truncated download never looks complete.
func (c *Client) DownloadAttachment(ctx context.Context, id uuid.UUID, destDir, name string) error {
	path, err := attachmentPath(name, destDir)
	if err != nil {
		return err
	}

	body, err := c.workitemClient.GetAttachmentContent(ctx, workitemtracking.GetAttachmentContentArgs{Id: &id})
	if err != nil {
		return err
	}
	defer body.Close()

	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return err
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, body); err != nil {
		out.Close()
		os.Remove(path)
		return err
	}
	// Close is checked rather than deferred: it reports a failed flush.
	if err := out.Close(); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}
