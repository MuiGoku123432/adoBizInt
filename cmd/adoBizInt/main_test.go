package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sentinovo.ai/bizInt/internal/ado"
)

// TestExitCodeFor pins the process exit contract (D-06) and its precedence rule
// (D-07). The mixed auth+timeout row is the load-bearing one: it is what proves
// a co-occurring transient failure cannot mask a credential rejection.
func TestExitCodeFor(t *testing.T) {
	auth := ado.Failure{Project: "alpha", Operation: "get pull requests", Kind: ado.FailureAuth}
	timeout := ado.Failure{Project: "beta", Operation: "get pull requests", Kind: ado.FailureTimeout}
	rateLimit := ado.Failure{Project: "gamma", Operation: "get pull requests", Kind: ado.FailureRateLimit}

	tests := []struct {
		name     string
		failures []ado.Failure
		complete bool
		want     int
	}{
		{"clean read", nil, true, exitOK},
		{"empty failure slice is still clean", []ado.Failure{}, true, exitOK},
		{"timeouts only, incomplete", []ado.Failure{timeout, timeout}, false, exitPartial},
		{"rate limits only, incomplete", []ado.Failure{rateLimit}, false, exitPartial},
		{"single auth failure", []ado.Failure{auth}, false, exitAuth},
		{"auth among three timeouts wins", []ado.Failure{timeout, timeout, auth}, false, exitAuth},
		{"auth first among timeouts wins", []ado.Failure{auth, timeout, timeout}, false, exitAuth},
		{"complete with an auth failure still maps by the auth scan", []ado.Failure{auth}, true, exitAuth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCodeFor(tt.failures, tt.complete); got != tt.want {
				t.Errorf("exitCodeFor(%d failures, complete=%v) = %d, want %d",
					len(tt.failures), tt.complete, got, tt.want)
			}
		})
	}
}

// captureOutput swaps the dispatch layer's writers for buffers and restores the
// production defaults when the test ends.
func captureOutput(t *testing.T) (stdout, stderr *bytes.Buffer) {
	t.Helper()

	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}

	prevOut, prevErr := stdoutWriter, stderrWriter
	stdoutWriter, stderrWriter = stdout, stderr
	t.Cleanup(func() {
		stdoutWriter, stderrWriter = prevOut, prevErr
	})

	return stdout, stderr
}

// emptyReadClient returns a fake whose single read succeeds with nothing to
// report — the shape a real read produces for a configured project with no open
// pull requests.
func emptyReadClient(t *testing.T) *fakeReadClient {
	t.Helper()

	return &fakeReadClient{
		t: t,
		GetPullRequestsFunc: func(_ context.Context, _ []string) (ado.Result[ado.PullRequest], error) {
			return ado.Result[ado.PullRequest]{
				Items:    []ado.PullRequest{},
				Failures: []ado.Failure{},
				Complete: true,
			}, nil
		},
	}
}

// TestDispatchEmptyReadEmitsEmptyArrays pins the wire shape of an empty read.
//
// Regression: an agent evaluating `.failures | length` errored on the ordinary
// successful read, because a nil slice marshals to null rather than [].
func TestDispatchEmptyReadEmitsEmptyArrays(t *testing.T) {
	stdout, stderr := captureOutput(t)

	code := dispatch(context.Background(), emptyReadClient(t), []string{"prs", "list", "--json"})
	if code != exitOK {
		t.Fatalf("dispatch(prs list --json) = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
	}

	raw := stdout.Bytes()
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Fatalf("stdout = %q, want a newline-terminated document", raw)
	}
	if bytes.Count(raw, []byte("\n")) != 1 {
		t.Errorf("stdout = %q, want exactly one newline (one document, no extra output)", raw)
	}

	var envelope struct {
		Items    json.RawMessage `json:"items"`
		Failures json.RawMessage `json:"failures"`
		Complete bool            `json:"complete"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", raw, err)
	}
	if string(envelope.Items) != "[]" {
		t.Errorf("items = %s, want []", envelope.Items)
	}
	if string(envelope.Failures) != "[]" {
		t.Errorf("failures = %s, want []", envelope.Failures)
	}
	if !envelope.Complete {
		t.Errorf("complete = %v, want true", envelope.Complete)
	}
}

// TestDispatchIsDeterministic pins stable ordering across runs, so an agent can
// diff successive --json outputs and trust that a difference means the data
// changed.
func TestDispatchIsDeterministic(t *testing.T) {
	client := &fakeReadClient{
		t: t,
		GetPullRequestsFunc: func(_ context.Context, _ []string) (ado.Result[ado.PullRequest], error) {
			return ado.Result[ado.PullRequest]{
				Items: []ado.PullRequest{
					{ID: 3, Title: "third", Project: "alpha", RepositoryName: "repo-a"},
					{ID: 1, Title: "first", Project: "alpha", RepositoryName: "repo-a"},
					{ID: 2, Title: "second", Project: "beta", RepositoryName: "repo-b"},
				},
				Failures: []ado.Failure{},
				Complete: true,
			}, nil
		},
	}

	runOnce := func() []byte {
		stdout, stderr := captureOutput(t)
		if code := dispatch(context.Background(), client, []string{"prs", "list", "--json"}); code != exitOK {
			t.Fatalf("dispatch(prs list --json) = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
		}
		return append([]byte(nil), stdout.Bytes()...)
	}

	first, second := runOnce(), runOnce()
	if !bytes.Equal(first, second) {
		t.Errorf("stdout differed across identical invocations:\nfirst:  %q\nsecond: %q", first, second)
	}
}

// TestDispatchUsageErrors covers the paths that never reach a client call. Each
// must diagnose on stderr, leave stdout untouched, and return exitUnknown — a
// typo must not start a full-screen program in a non-interactive context.
func TestDispatchUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"noun with no verb", []string{"prs"}},
		{"unrecognised first token", []string{"nonsense"}},
		{"unrecognised verb", []string{"prs", "nonsense"}},
		{"no arguments at all", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t)

			// The fake has no closure set: reaching a read fails the subtest.
			code := dispatch(context.Background(), &fakeReadClient{t: t}, tt.args)

			if code != exitUnknown {
				t.Errorf("dispatch(%q) = %d, want %d", tt.args, code, exitUnknown)
			}
			if stdout.Len() != 0 {
				t.Errorf("dispatch(%q) wrote %q to stdout, want nothing", tt.args, stdout.String())
			}
			if stderr.Len() == 0 {
				t.Errorf("dispatch(%q) wrote nothing to stderr, want a diagnostic", tt.args)
			}
		})
	}
}

// TestLoggingInitFailureKeepsStdoutClean closes RESEARCH assumption A1: when
// logging.Init fails, logging.Logger() falls back to slog.Default(), and this
// plan's stdout-purity guarantee depends on that default writing to stderr
// rather than stdout. The subtest forces the init failure, then asserts that
// the only bytes reaching the real os.Stdout are the JSON document itself.
func TestLoggingInitFailureKeepsStdoutClean(t *testing.T) {
	// os.UserHomeDir reads $HOME; pointing it at a regular file makes the log
	// directory creation inside logging.Init fail.
	notADir := filepath.Join(t.TempDir(), "home-is-a-file")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Setenv("HOME", notADir)

	_, stderr := captureOutput(t)

	// Capture the real os.Stdout too: slog.Default() writes to the process
	// stream, not to the injected writer, so only a process-level capture can
	// prove it never lands on stdout.
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	prevStdout := os.Stdout
	os.Stdout = writeEnd
	stdoutWriter = writeEnd

	initLogging()
	code := dispatch(context.Background(), emptyReadClient(t), []string{"prs", "list", "--json"})

	os.Stdout = prevStdout
	if err := writeEnd.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	captured, err := io.ReadAll(readEnd)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}

	if code != exitOK {
		t.Fatalf("dispatch(prs list --json) = %d, want %d", code, exitOK)
	}
	if stderr.Len() == 0 {
		t.Errorf("stderr = %q, want the logging-init warning", stderr.String())
	}
	if bytes.Count(captured, []byte("\n")) != 1 {
		t.Errorf("stdout = %q, want exactly the one JSON document", captured)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(captured, &envelope); err != nil {
		t.Errorf("stdout %q is not a single JSON document: %v", captured, err)
	}
}

// wantCommands is the D-02 command list written out longhand rather than
// derived from the table under test. Deriving it would make the assertion
// tautological: the point is that the registered keys match the published
// surface, which is a one-way door once agents invoke these paths.
var wantCommands = []string{
	"branches list",
	"dashboard counts",
	"pipelines definitions",
	"pipelines runs",
	"prs list",
	"releases approvals",
	"releases list",
	"workitems list",
}

// TestDispatchTableMatchesD02 pins the eight registered `noun verb` keys.
func TestDispatchTableMatchesD02(t *testing.T) {
	got := commandKeys()
	if len(got) != len(wantCommands) {
		t.Fatalf("commands has %d entries (%v), want %d", len(got), got, len(wantCommands))
	}
	for i, want := range wantCommands {
		if got[i] != want {
			t.Errorf("commandKeys()[%d] = %q, want %q", i, got[i], want)
		}
	}
}

// TestDispatchResolvesEveryVerb runs each registered command against a fake
// whose every read succeeds, proving each key resolves to a distinct method
// that is actually reached. An unknown key must not reach any read at all.
func TestDispatchResolvesEveryVerb(t *testing.T) {
	for _, key := range wantCommands {
		t.Run(key, func(t *testing.T) {
			_, stderr := captureOutput(t)

			args := append(strings.Split(key, " "), "--json")
			if code := dispatch(context.Background(), allReadsFake(t), args); code != exitOK {
				t.Fatalf("dispatch(%q) = %d, want %d (stderr: %q)", args, code, exitOK, stderr.String())
			}
		})
	}

	t.Run("unknown key reaches no read", func(t *testing.T) {
		stdout, _ := captureOutput(t)
		// The fake has no closures set: any read call fails the subtest.
		if code := dispatch(context.Background(), &fakeReadClient{t: t}, []string{"widgets", "list"}); code != exitUnknown {
			t.Errorf("dispatch(widgets list) = %d, want %d", code, exitUnknown)
		}
		if stdout.Len() != 0 {
			t.Errorf("stdout = %q, want nothing", stdout.String())
		}
	})
}

// TestDispatchEachVerbCallsItsOwnMethod proves the 1:1 mapping of D-02: a fake
// with exactly one closure set must succeed for its own verb, and the seven
// other verbs must not reach it.
func TestDispatchEachVerbCallsItsOwnMethod(t *testing.T) {
	for _, key := range wantCommands {
		t.Run(key, func(t *testing.T) {
			called := 0
			client := allReadsFake(t)
			wrapOne(t, client, key, &called)

			_, stderr := captureOutput(t)
			args := append(strings.Split(key, " "), "--json")
			if code := dispatch(context.Background(), client, args); code != exitOK {
				t.Fatalf("dispatch(%q) = %d, want %d (stderr: %q)", args, code, exitOK, stderr.String())
			}
			if called != 1 {
				t.Errorf("dispatch(%q) called its mapped method %d times, want exactly 1", key, called)
			}
		})
	}
}

// wrapOne increments counter each time the method mapped to key is called,
// leaving the other seven closures untouched.
func wrapOne(t *testing.T, f *fakeReadClient, key string, counter *int) {
	t.Helper()
	switch key {
	case "workitems list":
		inner := f.GetWorkItemsFunc
		f.GetWorkItemsFunc = func(ctx context.Context, p []string) (ado.Result[ado.WorkItem], error) {
			*counter++
			return inner(ctx, p)
		}
	case "prs list":
		inner := f.GetPullRequestsFunc
		f.GetPullRequestsFunc = func(ctx context.Context, p []string) (ado.Result[ado.PullRequest], error) {
			*counter++
			return inner(ctx, p)
		}
	case "pipelines runs":
		inner := f.GetPipelineRunsFunc
		f.GetPipelineRunsFunc = func(ctx context.Context, p []string) (ado.Result[ado.PipelineRun], error) {
			*counter++
			return inner(ctx, p)
		}
	case "pipelines definitions":
		inner := f.GetPipelineDefinitionsFunc
		f.GetPipelineDefinitionsFunc = func(ctx context.Context, p []string) (ado.Result[ado.PipelineDefinition], error) {
			*counter++
			return inner(ctx, p)
		}
	case "releases list":
		inner := f.GetReleasesFunc
		f.GetReleasesFunc = func(ctx context.Context, p []string) (ado.Result[ado.Release], error) {
			*counter++
			return inner(ctx, p)
		}
	case "releases approvals":
		inner := f.GetPendingApprovalsFunc
		f.GetPendingApprovalsFunc = func(ctx context.Context, p []string) (ado.Result[ado.ReleaseApproval], error) {
			*counter++
			return inner(ctx, p)
		}
	case "branches list":
		inner := f.GetBranchesFunc
		f.GetBranchesFunc = func(ctx context.Context, p, r []string) (ado.Result[ado.Branch], error) {
			*counter++
			return inner(ctx, p, r)
		}
	case "dashboard counts":
		inner := f.GetDashboardCountsFunc
		f.GetDashboardCountsFunc = func(ctx context.Context, p []string) (ado.Result[ado.DashboardCounts], error) {
			*counter++
			return inner(ctx, p)
		}
	default:
		t.Fatalf("wrapOne has no case for %q", key)
	}
}
