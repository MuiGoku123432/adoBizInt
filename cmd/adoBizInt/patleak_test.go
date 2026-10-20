package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"log/slog"
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
)

// TestPATNeverLogged is the HARD-06 regression test (D-19). It drives a read, an
// executed write and a recovered panic through the cmd fakes with slog captured
// into a buffer, and asserts that no form of the credential reaches the log.
//
// It is a tripwire for a future careless slog attribute, not a detector of a
// present leak. After 05-01 no value reachable from dispatch holds the PAT, and
// Go renders stack-frame arguments as hex words rather than dereferenced
// strings, so the panic stack is not expected to contain it today (RESEARCH
// Pitfall 10).
//
// ADO_PAT and HOME are set so that any future config.Load reachable from
// dispatch would carry the fixture rather than the developer's real token and
// config: a test must never see a real secret.
//
// Because nothing reachable from dispatch holds the PAT, this test cannot catch
// a leak from the code that does. That half lives with the secret:
// internal/config's TestLoadedConfigNeverRendersPAT loads a Config from
// ADO_PAT and pins that slog and fmt render it without the credential (WR-05).
//
// Do not call initLogging or logging.Init here. A successful Init points
// logging.Logger() at a file instead of slog.Default(), and the capture would
// silently go empty (Pitfall 8). The three marker checks at the end are what
// catch that.
func TestPATNeverLogged(t *testing.T) {
	t.Setenv("ADO_PAT", patFixture)
	t.Setenv("HOME", t.TempDir())

	var buf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	// slog.SetDefault also redirects the stdlib log package, so restoring it
	// matters for every later test in the binary.
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	// handlePanic writes the stack only when logging init succeeded.
	prevReady := loggingReady
	loggingReady = true
	t.Cleanup(func() { loggingReady = prevReady })

	withReadBudget(t, 2*time.Minute)
	withConfiguredProjects(t, "P")
	tempAuditSink(t)
	_, stderr := captureOutput(t)
	ctx := context.Background()

	if code := dispatch(ctx, allReadsFake(t), []string{"prs", "list", "--json"}); code != 0 {
		t.Fatalf("read exited %d, want 0 (stderr: %q)", code, stderr.String())
	}

	// An executed write, not a dry run: a dry run logs nothing, which would make
	// the write path vacuous here.
	writer := preReadFake(t, "Active")
	writer.UpdateWorkItemStateFunc = func(context.Context, string, int, string) error { return nil }
	if code := dispatch(ctx, writer, setStateArgs("--confirm")); code != 0 {
		t.Fatalf("write exited %d, want 0 (stderr: %q)", code, stderr.String())
	}

	if code := guard(func() int {
		return dispatch(ctx, panickingClient(t), []string{"prs", "list", "--json"})
	}); code != exitUnknown {
		t.Fatalf("panic exited %d, want %d", code, exitUnknown)
	}

	logged := buf.Bytes()
	// The connection holds the PAT as a Basic header, so search the raw value,
	// the header, and the base64 part alone.
	for _, needle := range []struct{ form, value string }{
		{"raw", patFixture},
		{"basic header", azuredevops.CreateBasicAuthHeaderValue("", patFixture)},
		{"base64", base64.StdEncoding.EncodeToString([]byte(":" + patFixture))},
	} {
		if bytes.Contains(logged, []byte(needle.value)) {
			t.Errorf("log output contains the %s form of the PAT", needle.form)
		}
	}

	// Each path must have logged into the capture, or the absence checks above
	// pass for the wrong reason.
	for _, marker := range []string{"Read complete", "Write complete", "panic recovered"} {
		if !bytes.Contains(logged, []byte(marker)) {
			t.Errorf("capture is vacuous: no %q line was logged", marker)
		}
	}
}
