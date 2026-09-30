package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/config"
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

// wantWriteCommands is the write half of the same published surface, kept apart
// from wantCommands because TestDispatchResolvesEveryVerb drives the read verbs
// with a bare `--json` invocation that a guarded write correctly rejects: a
// write requires a positional target, exactly one --project and --state.
var wantWriteCommands = []string{
	"pipelines queue",
	"prs approve",
	"prs complete",
	"releases approve",
	"releases reject",
	"workitems set-state",
}

// allCommands is the full published `noun verb` surface in commandKeys order.
func allCommands() []string {
	all := append(append([]string{}, wantCommands...), wantWriteCommands...)
	sort.Strings(all)
	return all
}

// TestDispatchTableMatchesD02 pins every registered `noun verb` key.
func TestDispatchTableMatchesD02(t *testing.T) {
	got := commandKeys()
	want := allCommands()
	if len(got) != len(want) {
		t.Fatalf("commands has %d entries (%v), want %d", len(got), got, len(want))
	}
	for i, want := range want {
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

// --- Task 3: human-mode renderer -------------------------------------------

// twoFailures is the failure list the human-mode tests assert against. Both
// entries carry distinct project, operation and kind values, so a renderer that
// prints only one of the three fields is caught.
func twoFailures() []ado.Failure {
	return []ado.Failure{
		{Project: "alpha", Operation: "get pull requests", Kind: ado.FailureAuth},
		{Project: "beta", Operation: "get work items", Kind: ado.FailureTimeout},
	}
}

// renderCase drives one payload type through dispatch in human mode.
type renderCase struct {
	command string
	client  func(t *testing.T, failures []ado.Failure) *fakeReadClient
	// wantRows are substrings that must each appear on exactly one stdout line.
	wantRows []string
}

func renderCases() []renderCase {
	return []renderCase{
		{
			command:  "workitems list",
			wantRows: []string{"Bug", "Fix the thing"},
			client: func(t *testing.T, f []ado.Failure) *fakeReadClient {
				c := allReadsFake(t)
				c.GetWorkItemsFunc = func(context.Context, []string) (ado.Result[ado.WorkItem], error) {
					return ado.Result[ado.WorkItem]{
						Items: []ado.WorkItem{
							{ID: 1, Title: "Fix the thing", Type: "Bug", State: "Active", Project: "alpha"},
							{ID: 2, Title: "Ship the other thing", Type: "Task", State: "New", Project: "beta"},
						},
						Failures: f, Complete: len(f) == 0,
					}, nil
				}
				return c
			},
		},
		{
			command:  "prs list",
			wantRows: []string{"Add a feature", "repo-a"},
			client: func(t *testing.T, f []ado.Failure) *fakeReadClient {
				c := allReadsFake(t)
				c.GetPullRequestsFunc = func(context.Context, []string) (ado.Result[ado.PullRequest], error) {
					return ado.Result[ado.PullRequest]{
						Items: []ado.PullRequest{
							{ID: 10, Title: "Add a feature", RepositoryName: "repo-a", Project: "alpha"},
							{ID: 11, Title: "Remove a feature", RepositoryName: "repo-b", Project: "beta"},
						},
						Failures: f, Complete: len(f) == 0,
					}, nil
				}
				return c
			},
		},
		{
			command:  "pipelines runs",
			wantRows: []string{"20240101.1", "succeeded"},
			client: func(t *testing.T, f []ado.Failure) *fakeReadClient {
				c := allReadsFake(t)
				c.GetPipelineRunsFunc = func(context.Context, []string) (ado.Result[ado.PipelineRun], error) {
					return ado.Result[ado.PipelineRun]{
						Items: []ado.PipelineRun{
							{ID: 1, BuildNumber: "20240101.1", PipelineName: "build", Status: "completed", Result: "succeeded", Project: "alpha"},
							{ID: 2, BuildNumber: "20240101.2", PipelineName: "build", Status: "inProgress", Result: "none", Project: "alpha"},
						},
						Failures: f, Complete: len(f) == 0,
					}, nil
				}
				return c
			},
		},
		{
			command:  "pipelines definitions",
			wantRows: []string{"nightly", "enabled"},
			client: func(t *testing.T, f []ado.Failure) *fakeReadClient {
				c := allReadsFake(t)
				c.GetPipelineDefinitionsFunc = func(context.Context, []string) (ado.Result[ado.PipelineDefinition], error) {
					return ado.Result[ado.PipelineDefinition]{
						Items: []ado.PipelineDefinition{
							{ID: 1, Name: "nightly", QueueStatus: "enabled", Project: "alpha"},
							{ID: 2, Name: "release", QueueStatus: "paused", Project: "beta"},
						},
						Failures: f, Complete: len(f) == 0,
					}, nil
				}
				return c
			},
		},
		{
			command:  "releases list",
			wantRows: []string{"Release-1", "active"},
			client: func(t *testing.T, f []ado.Failure) *fakeReadClient {
				c := allReadsFake(t)
				c.GetReleasesFunc = func(context.Context, []string) (ado.Result[ado.Release], error) {
					return ado.Result[ado.Release]{
						Items: []ado.Release{
							{ID: 1, Name: "Release-1", DefinitionName: "prod", Status: "active", Project: "alpha"},
							{ID: 2, Name: "Release-2", DefinitionName: "prod", Status: "draft", Project: "alpha"},
						},
						Failures: f, Complete: len(f) == 0,
					}, nil
				}
				return c
			},
		},
		{
			command:  "releases approvals",
			wantRows: []string{"preDeploy", "staging"},
			client: func(t *testing.T, f []ado.Failure) *fakeReadClient {
				c := allReadsFake(t)
				c.GetPendingApprovalsFunc = func(context.Context, []string) (ado.Result[ado.ReleaseApproval], error) {
					return ado.Result[ado.ReleaseApproval]{
						Items: []ado.ReleaseApproval{
							{ID: 1, ReleaseName: "Release-1", EnvironmentName: "staging", ApprovalType: "preDeploy", Status: "pending", Project: "alpha"},
							{ID: 2, ReleaseName: "Release-2", EnvironmentName: "prod", ApprovalType: "postDeploy", Status: "pending", Project: "alpha"},
						},
						Failures: f, Complete: len(f) == 0,
					}, nil
				}
				return c
			},
		},
		{
			command:  "branches list",
			wantRows: []string{"feature/x", "repo-a"},
			client: func(t *testing.T, f []ado.Failure) *fakeReadClient {
				c := allReadsFake(t)
				c.GetBranchesFunc = func(context.Context, []string, []string) (ado.Result[ado.Branch], error) {
					return ado.Result[ado.Branch]{
						Items: []ado.Branch{
							{Name: "feature/x", RepositoryName: "repo-a", Project: "alpha", AheadCount: 2, BehindCount: 1},
							{Name: "main", RepositoryName: "repo-a", Project: "alpha", IsDefault: true},
						},
						Failures: f, Complete: len(f) == 0,
					}, nil
				}
				return c
			},
		},
		{
			command:  "dashboard counts",
			wantRows: []string{"7"},
			client: func(t *testing.T, f []ado.Failure) *fakeReadClient {
				c := allReadsFake(t)
				c.GetDashboardCountsFunc = func(context.Context, []string) (ado.Result[ado.DashboardCounts], error) {
					return ado.Result[ado.DashboardCounts]{
						Items:    []ado.DashboardCounts{{WorkItems: 7, PRs: 3, Pipelines: 5}},
						Failures: f, Complete: len(f) == 0,
					}, nil
				}
				return c
			},
		},
	}
}

// TestRenderRowsToStdout pins the first human-mode behaviour: rows land on
// stdout, and a clean read writes nothing at all to stderr.
func TestRenderRowsToStdout(t *testing.T) {
	for _, tc := range renderCases() {
		t.Run(tc.command, func(t *testing.T) {
			stdout, stderr := captureOutput(t)

			code := dispatch(context.Background(), tc.client(t, nil), strings.Split(tc.command, " "))
			if code != exitOK {
				t.Fatalf("dispatch(%q) = %d, want %d (stderr: %q)", tc.command, code, exitOK, stderr.String())
			}

			lines := nonEmptyLines(stdout.String())
			wantLines := 1
			if tc.command != "dashboard counts" {
				wantLines = 2
			}
			if len(lines) != wantLines {
				t.Errorf("stdout had %d row lines (%q), want %d", len(lines), stdout.String(), wantLines)
			}
			for _, want := range tc.wantRows {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("stdout %q does not contain %q", stdout.String(), want)
				}
			}
			if stderr.Len() != 0 {
				t.Errorf("clean read wrote %q to stderr, want nothing", stderr.String())
			}
		})
	}
}

// TestRenderFailuresToStderrOnly pins D-13 and threat T-03-07: failure detail
// is a diagnostic and must never appear on the parseable channel.
func TestRenderFailuresToStderrOnly(t *testing.T) {
	for _, tc := range renderCases() {
		t.Run(tc.command, func(t *testing.T) {
			stdout, stderr := captureOutput(t)

			code := dispatch(context.Background(), tc.client(t, twoFailures()), strings.Split(tc.command, " "))
			if code != exitAuth {
				t.Fatalf("dispatch(%q) = %d, want %d (an auth failure is present)", tc.command, code, exitAuth)
			}

			for _, want := range []string{"alpha", "get pull requests", string(ado.FailureAuth), "beta", "get work items", string(ado.FailureTimeout)} {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr %q does not name %q", stderr.String(), want)
				}
			}
			for _, forbidden := range []string{"get pull requests", "get work items", string(ado.FailureAuth)} {
				if strings.Contains(stdout.String(), forbidden) {
					t.Errorf("stdout %q leaked failure text %q", stdout.String(), forbidden)
				}
			}
		})
	}
}

// TestRenderHasNoANSI pins the plain-text choice. Styled escapes in a
// redirected stdout are hostile to the agent this CLI exists for.
func TestRenderHasNoANSI(t *testing.T) {
	for _, tc := range renderCases() {
		t.Run(tc.command, func(t *testing.T) {
			stdout, stderr := captureOutput(t)

			dispatch(context.Background(), tc.client(t, twoFailures()), strings.Split(tc.command, " "))

			if bytes.Contains(stdout.Bytes(), []byte("\x1b[")) {
				t.Errorf("stdout %q contains an ANSI escape sequence", stdout.String())
			}
			if bytes.Contains(stderr.Bytes(), []byte("\x1b[")) {
				t.Errorf("stderr %q contains an ANSI escape sequence", stderr.String())
			}
		})
	}
}

// TestRenderEmptyWritesNothing pins the zero-item case: no rows and no
// placeholder. A caller counting lines must get zero, not one line saying
// "empty" that a naive parser would count as a result.
func TestRenderEmptyWritesNothing(t *testing.T) {
	for _, key := range wantCommands {
		t.Run(key, func(t *testing.T) {
			stdout, stderr := captureOutput(t)

			if code := dispatch(context.Background(), allReadsFake(t), strings.Split(key, " ")); code != exitOK {
				t.Fatalf("dispatch(%q) = %d, want %d", key, code, exitOK)
			}
			if stdout.Len() != 0 {
				t.Errorf("empty read wrote %q to stdout, want nothing", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("empty read wrote %q to stderr, want nothing", stderr.String())
			}
		})
	}
}

// TestRenderNotInvokedUnderJSON pins that --json bypasses the human renderer
// entirely and the failures ride inside the stdout envelope instead.
func TestRenderNotInvokedUnderJSON(t *testing.T) {
	stdout, stderr := captureOutput(t)

	client := renderCases()[1].client(t, twoFailures()) // prs list
	if code := dispatch(context.Background(), client, []string{"prs", "list", "--json"}); code != exitAuth {
		t.Fatalf("dispatch(prs list --json) = %d, want %d", code, exitAuth)
	}

	if bytes.Count(stdout.Bytes(), []byte("\n")) != 1 {
		t.Errorf("stdout = %q, want exactly one JSON document", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"failures"`) {
		t.Errorf("stdout = %q, want the failures inside the envelope", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("--json wrote %q to stderr, want nothing", stderr.String())
	}
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// withReadBudget sets the package-level fan-out budget for one test and
// restores it afterwards, standing in for the assignment run() makes after
// config validation.
func withReadBudget(t *testing.T, d time.Duration) {
	t.Helper()
	prev := readBudget
	readBudget = d
	t.Cleanup(func() { readBudget = prev })
}

// withConfiguredProjects sets the --project allow-list for one test and
// restores it afterwards, standing in for the assignment run() makes after
// config validation.
func withConfiguredProjects(t *testing.T, names ...string) {
	t.Helper()
	prev := configuredProjects
	configuredProjects = names
	t.Cleanup(func() { configuredProjects = prev })
}

// deadlineCapturingClient records the deadline of the context the read was
// invoked with, which is how these tests observe the applied budget without
// waiting for it to expire.
func deadlineCapturingClient(t *testing.T, got *time.Duration, ok *bool) *fakeReadClient {
	t.Helper()
	return &fakeReadClient{
		t: t,
		GetPullRequestsFunc: func(ctx context.Context, _ []string) (ado.Result[ado.PullRequest], error) {
			deadline, has := ctx.Deadline()
			*ok = has
			if has {
				*got = time.Until(deadline)
			}
			return emptyResult[ado.PullRequest](), nil
		},
	}
}

// TestTimeoutOverridesConfiguredBudgetAfterValidation is the ordering proof for
// D-P6. A 5s fetch budget written into the config would be rejected by
// validation, because it sits below the 30s default poll interval. The same
// value supplied as --timeout must be accepted, because the override lands
// after validation has already passed on the configured value.
func TestTimeoutOverridesConfiguredBudgetAfterValidation(t *testing.T) {
	cfg := &config.Config{
		PAT:          "fixture-pat",
		OrgURL:       "https://dev.azure.com/fixture",
		PollInterval: 30 * time.Second,
		FetchTimeout: 2 * time.Minute,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("configured budget must validate before the override: %v", err)
	}

	// The same duration written into the config would fail. Asserting that here
	// is what makes the ordering load-bearing rather than incidental.
	rejected := *cfg
	rejected.FetchTimeout = 5 * time.Second
	if err := rejected.Validate(); err == nil {
		t.Fatalf("a 5s fetch budget below the poll interval should fail validation; the ordering proof is vacuous otherwise")
	}

	withReadBudget(t, cfg.FetchTimeout)
	captureOutput(t)

	var applied time.Duration
	var hasDeadline bool
	client := deadlineCapturingClient(t, &applied, &hasDeadline)

	code := dispatch(context.Background(), client, []string{"prs", "list", "--json", "--timeout", "5s"})
	if code != exitOK {
		t.Fatalf("dispatch with --timeout 5s = %d, want %d (no validation error)", code, exitOK)
	}
	if !hasDeadline {
		t.Fatalf("the read ran with no deadline")
	}
	if applied > 10*time.Second {
		t.Errorf("applied budget is %v, want roughly 5s (the flag, not the configured 2m)", applied)
	}
}

// TestTimeoutOmittedUsesConfiguredBudget pins the default path: no flag means
// the configured fetch budget, not the fetchctx fallback.
func TestTimeoutOmittedUsesConfiguredBudget(t *testing.T) {
	withReadBudget(t, 17*time.Minute)
	captureOutput(t)

	var applied time.Duration
	var hasDeadline bool
	client := deadlineCapturingClient(t, &applied, &hasDeadline)

	if code := dispatch(context.Background(), client, []string{"prs", "list", "--json"}); code != exitOK {
		t.Fatalf("dispatch = %d, want %d", code, exitOK)
	}
	if !hasDeadline {
		t.Fatalf("the read ran with no deadline")
	}
	if applied < 16*time.Minute {
		t.Errorf("applied budget is %v, want roughly the configured 17m", applied)
	}
}

// TestTimeoutZeroFallsBackToDefaultBudget guards the reading of --timeout 0s as
// "unbounded". A caller who writes it gets the fetchctx default; every CLI read
// carries a deadline (threat T-03-17).
func TestTimeoutZeroFallsBackToDefaultBudget(t *testing.T) {
	withReadBudget(t, 0)
	captureOutput(t)

	var applied time.Duration
	var hasDeadline bool
	client := deadlineCapturingClient(t, &applied, &hasDeadline)

	if code := dispatch(context.Background(), client, []string{"prs", "list", "--json", "--timeout", "0s"}); code != exitOK {
		t.Fatalf("dispatch = %d, want %d", code, exitOK)
	}
	if !hasDeadline {
		t.Fatalf("--timeout 0s produced a context with no deadline; it must fall back to the default budget")
	}
	if applied <= 0 {
		t.Errorf("applied budget is %v, want the positive default budget", applied)
	}
}

// TestTimeoutEqualToPollIntervalIsAccepted is the boundary the config validator
// rejects. As a flag it is legitimate, because nothing is refreshing.
func TestTimeoutEqualToPollIntervalIsAccepted(t *testing.T) {
	withReadBudget(t, 2*time.Minute)
	stdout, _ := captureOutput(t)

	var applied time.Duration
	var hasDeadline bool
	client := deadlineCapturingClient(t, &applied, &hasDeadline)

	if code := dispatch(context.Background(), client, []string{"prs", "list", "--json", "--timeout", "30s"}); code != exitOK {
		t.Fatalf("dispatch with --timeout 30s = %d, want %d", code, exitOK)
	}
	if !hasDeadline || applied > time.Minute {
		t.Errorf("applied budget is %v (deadline=%v), want roughly 30s", applied, hasDeadline)
	}
	if stdout.Len() == 0 {
		t.Errorf("stdout is empty; want the result envelope")
	}
}

// TestTimeoutUnparseableExitsThreeWithCleanStdout pins the malformed-value path
// against the usage contract from 03-02: usage to stderr, nothing to stdout.
func TestTimeoutUnparseableExitsThreeWithCleanStdout(t *testing.T) {
	withReadBudget(t, 2*time.Minute)
	stdout, stderr := captureOutput(t)

	code := dispatch(context.Background(), allReadsFake(t), []string{"prs", "list", "--timeout", "banana"})
	if code != exitUnknown {
		t.Errorf("dispatch with an unparseable --timeout = %d, want %d", code, exitUnknown)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Errorf("stderr is empty; want the parse error and usage")
	}
}
