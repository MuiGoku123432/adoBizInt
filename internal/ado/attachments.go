package ado

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
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

// maxSameNameCopies bounds the " (n)" suffixes DownloadAttachment tries before
// giving up on a name that is already taken.
const maxSameNameCopies = 100

// DownloadAttachment fetches the attachment with the given GUID into destDir
// and returns the path it wrote.
//
// The SDK's GetAttachmentContent owns the host, the auth header and the
// connection timeout, so no argument here can send the PAT elsewhere (D-01).
// The name is checked by attachmentPath before any request is made (D-03).
// There is no byte cap; it is deferred with the CLI verb (D-04). A refused name
// is a plain error, the same as a network failure (D-07).
//
// The body goes to a temporary file in destDir, which is linked into place only
// once it is complete, so nothing under the attachment's name is ever partial
// and a failed download never touches a file already there. An existing file
// is never overwritten either (WR-01): work items routinely carry several
// attachments with one name (every pasted screenshot is image.png), so a taken
// name gets " (1)", " (2)" ... before its extension. os.Link is used rather
// than os.Rename because Link refuses an existing target and Rename replaces it.
func (c *Client) DownloadAttachment(ctx context.Context, id uuid.UUID, destDir, name string) (string, error) {
	path, err := attachmentPath(name, destDir)
	if err != nil {
		return "", err
	}

	body, err := c.workitemClient.GetAttachmentContent(ctx, workitemtracking.GetAttachmentContentArgs{Id: &id})
	if err != nil {
		return "", err
	}
	defer body.Close()

	// Owner-only on every call, not just at creation (WR-02): attachments can
	// carry customer data, and installs from before Phase 5 have this directory
	// at 0755. MkdirAll's mode applies only when it creates the directory, the
	// same reason logging.Init chmods its own (D-20). CreateTemp makes the file
	// 0600, and the hard link below keeps that mode.
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(destDir, 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(destDir, ".download-*")
	if err != nil {
		return "", err
	}
	// Removed on every path: before a successful Link it is a partial
	// download, and after one the content lives on under its final name.
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, body); err != nil {
		tmp.Close()
		return "", err
	}
	// Close is checked rather than deferred: it reports a failed flush.
	if err := tmp.Close(); err != nil {
		return "", err
	}

	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	for n := 0; n < maxSameNameCopies; n++ {
		candidate := path
		if n > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", stem, n, ext)
		}
		err := os.Link(tmp.Name(), candidate)
		if err == nil {
			return candidate, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("attachment name %q is taken %d times over in %s", name, maxSameNameCopies, filepath.Dir(path))
}
