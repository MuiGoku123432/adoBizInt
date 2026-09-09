package main

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"sentinovo.ai/bizInt/internal/ado"
)

// cancelledParent returns a context that is already done, standing in for a
// SIGINT that arrived during the read. Driving cancellation through an injected
// context rather than a real signal keeps the test hermetic: a test that raises
// SIGINT at itself is at the mercy of the runner's own signal handling.
func cancelledParent() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// interruptedClient returns a fake whose read observes the cancellation and
// reports it the way a real fan-out read does.
func interruptedClient(t *testing.T) *fakeReadClient {
	t.Helper()
	return &fakeReadClient{
		t: t,
		GetPullRequestsFunc: func(ctx context.Context, _ []string) (ado.Result[ado.PullRequest], error) {
			return ado.Result[ado.PullRequest]{}, ctx.Err()
		},
	}
}

// decodeCrash unmarshals stdout into a generic map so the test can assert on
// the exact top-level key set rather than on a struct that would silently
// tolerate extra keys.
func decodeCrash(t *testing.T, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("stdout %q is not a single JSON document: %v", raw, err)
	}
	return top
}

// TestInterruptExitsThree pins D-25. Not 130: a fifth code outside the
// published four would force every caller to learn a second convention. Not 0:
// a script interrupting its own child could not then tell "interrupted, no
// data" from "clean read, nothing matched".
func TestInterruptExitsThree(t *testing.T) {
	withReadBudget(t, 2*time.Minute)
	captureOutput(t)

	code := dispatch(cancelledParent(), interruptedClient(t), []string{"prs", "list", "--json"})
	if code != exitUnknown {
		t.Errorf("interrupted dispatch = %d, want %d", code, exitUnknown)
	}
}

// TestInterruptEmitsDistinctErrorObject is the D-26 assertion. The record must
// not be representable as a normal read outcome: an agent distinguishes a crash
// from a result by which top-level key is present, so a synthetic empty
// envelope would be read as real data.
func TestInterruptEmitsDistinctErrorObject(t *testing.T) {
	withReadBudget(t, 2*time.Minute)
	stdout, _ := captureOutput(t)

	dispatch(cancelledParent(), interruptedClient(t), []string{"prs", "list", "--json"})

	top := decodeCrash(t, stdout.Bytes())
	if len(top) != 1 {
		t.Fatalf("top-level keys = %v, want exactly [error]", keysOf(top))
	}
	if _, ok := top["error"]; !ok {
		t.Fatalf("top-level keys = %v, want exactly [error]", keysOf(top))
	}
	for _, forbidden := range []string{"items", "failures", "complete"} {
		if _, ok := top[forbidden]; ok {
			t.Errorf("top-level key %q present; a crash must never look like a result envelope", forbidden)
		}
	}
}

// TestInterruptRecordShape pins the published fields: kind, a message, and the
// log path. No stack — that is Task 3's credential-disclosure constraint and it
// applies to every crash record, not only the panic one.
func TestInterruptRecordShape(t *testing.T) {
	withReadBudget(t, 2*time.Minute)
	stdout, _ := captureOutput(t)

	dispatch(cancelledParent(), interruptedClient(t), []string{"prs", "list", "--json"})

	var env struct {
		Error struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
			LogPath string `json:"log_path"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout %q does not decode: %v", stdout.String(), err)
	}
	if env.Error.Kind != "interrupt" {
		t.Errorf("kind = %q, want %q", env.Error.Kind, "interrupt")
	}
	if env.Error.Message == "" {
		t.Errorf("message is empty; the record must say what happened")
	}
	if env.Error.LogPath == "" {
		t.Errorf("log_path is empty; the record must name where the detail went")
	}

	inner := decodeCrash(t, stdout.Bytes())["error"]
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(inner, &fields); err != nil {
		t.Fatalf("error object does not decode: %v", err)
	}
	for _, forbidden := range []string{"stack", "stack_trace", "goroutines"} {
		if _, ok := fields[forbidden]; ok {
			t.Errorf("record carries %q; a stack prints function arguments and would leak the credential", forbidden)
		}
	}
}

// TestInterruptRecordNotEmittedOnCleanRead guards the other direction: an
// uninterrupted read must produce the ordinary envelope and no crash record.
func TestInterruptRecordNotEmittedOnCleanRead(t *testing.T) {
	withReadBudget(t, 2*time.Minute)
	stdout, _ := captureOutput(t)

	code := dispatch(context.Background(), emptyReadClient(t), []string{"prs", "list", "--json"})
	if code != exitOK {
		t.Fatalf("clean dispatch = %d, want %d", code, exitOK)
	}
	top := decodeCrash(t, stdout.Bytes())
	if _, ok := top["error"]; ok {
		t.Errorf("clean read emitted a crash record: %s", stdout.String())
	}
	if _, ok := top["items"]; !ok {
		t.Errorf("clean read did not emit the result envelope: %s", stdout.String())
	}
}

// keysOf renders a decoded map's key set for failure messages.
func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
