package main

import (
	"context"
	"testing"

	"sentinovo.ai/bizInt/internal/ado"
)

// This file holds the per-verb SEMANTIC facts that the phase-wide guard matrix
// in write_test.go cannot state from a table: which SDK method a verb must call,
// which spellings are deliberately absent, and which verb has no no-op
// condition. The five structural guards themselves live in TestWriteGuardMatrix,
// which ranges over writeSupportByCommand so no verb can escape them; the
// earlier TestDryRunIssuesNoMutatingCallAllVerbs was fully subsumed by it and
// removed rather than left as a second definition of the same guarantee.

// wrapFake wraps a write fake in the combined read/write client the dispatch
// table is handed.
func wrapFake(t *testing.T, w *fakeWriteClient) *fakeReadClient {
	t.Helper()
	return &fakeReadClient{t: t, fakeWriteClient: w}
}

// withConfirm appends --confirm to a fixture's invocation.
func withConfirm(args []string, extra ...string) []string {
	out := append(append([]string{}, args...), "--confirm")
	return append(out, extra...)
}

// failingPreReadFake answers whichever pre-read the named verb uses with an
// error, leaving every mutating closure nil.
func failingPreReadFake(t *testing.T, key string) *fakeReadClient {
	t.Helper()
	boom := wrappedErrorValue(500, "boom")
	w := &fakeWriteClient{t: t,
		GetWorkItemDetailFunc: func(context.Context, int) (*ado.WorkItemDetail, error) {
			return nil, boom
		},
		GetPullRequestDetailFunc: func(context.Context, string, string, int) (*ado.PullRequestDetail, error) {
			return nil, boom
		},
		GetReleaseApprovalDetailFunc: func(context.Context, string, int) (*ado.ReleaseApprovalDetail, error) {
			return nil, boom
		},
		GetPipelineDefinitionDetailFunc: func(context.Context, string, int) (*ado.PipelineDefinitionDetail, error) {
			return nil, boom
		},
	}
	return wrapFake(t, w)
}

func splitKey(key string) (string, string) {
	for i := range key {
		if key[i] == ' ' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}

// stripPositional drops the target id, which sits at index 2 of every fixture
// invocation.
func stripPositional(args []string) []string {
	out := append([]string{}, args[:2]...)
	return append(out, args[3:]...)
}

// stripFlag removes a flag and its value from an invocation.
func stripFlag(args []string, flag string) []string {
	out := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == flag {
			i++
			continue
		}
		out = append(out, args[i])
	}
	return out
}

// TestWriteRepoArityIsExactlyOne pins the difference in meaning between the read
// --repo and the write --repo. On a read it narrows a fan-out and a second value
// is legitimate; on a mutation there is exactly one target, so a second value is
// a parse error with nothing attempted (D-24, D-25).
func TestWriteRepoArityIsExactlyOne(t *testing.T) {
	tempAuditSink(t)
	_, _ = captureOutput(t)

	c := &fakeReadClient{t: t, fakeWriteClient: &fakeWriteClient{t: t}}
	args := []string{"prs", "approve", "4821", "--project", "P", "--repo", "R", "--repo", "S"}
	if code := dispatch(context.Background(), c, args); code != exitUnknown {
		t.Errorf("two --repo values on a write = %d, want %d", code, exitUnknown)
	}
}

// TestNoPrsMergeAlias pins D-23's reasoning applied to verb names: one spelling
// per concept. `prs merge` is what a user might search for, but accepting both
// spellings means agent-generated scripts diverge and --help documents two names
// for one mutation.
func TestNoPrsMergeAlias(t *testing.T) {
	if _, ok := commands["prs merge"]; ok {
		t.Error("commands registers `prs merge`; `prs complete` is the one ratified spelling")
	}
}

// TestQueueHasNoNoOpCondition states explicitly why `pipelines queue` is the one
// verb without an already-applied refusal: queuing a run is not an idempotent
// state change, so there is no pre-read state that makes it redundant. Its diff
// is therefore never empty and exit 4 is unreachable for it.
func TestQueueHasNoNoOpCondition(t *testing.T) {
	tempAuditSink(t)
	stdout, stderr := captureOutput(t)

	c := wrapFake(t, &fakeWriteClient{t: t,
		GetPipelineDefinitionDetailFunc: func(_ context.Context, _ string, id int) (*ado.PipelineDefinitionDetail, error) {
			// The definition's own default branch differs from the requested
			// one, and it must not be consulted as a default either way (P-04).
			return &ado.PipelineDefinitionDetail{ID: id, DefaultBranch: "main"}, nil
		}})

	args := []string{"pipelines", "queue", "12", "--project", "P", "--branch", "main", "--json"}
	if code := dispatch(context.Background(), c, args); code != exitOK {
		t.Fatalf("queue dry run = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
	}
	got := decodeWriteEnvelope(t, stdout.Bytes())
	if len(got.Changes) != 1 {
		t.Fatalf("changes = %#v, want exactly one; an empty diff would refuse a queue as a no-op it cannot be", got.Changes)
	}
}

// TestReleaseCommentReachesTheMutation pins P-03: --comment is carried into
// Azure DevOps rather than dropped, and defaults to empty when omitted.
func TestReleaseCommentReachesTheMutation(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"supplied", []string{"releases", "approve", "77", "--project", "P", "--comment", "ship it", "--confirm", "--json"}, "ship it"},
		{"omitted", []string{"releases", "approve", "77", "--project", "P", "--confirm", "--json"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tempAuditSink(t)
			_, stderr := captureOutput(t)

			var got string
			c := wrapFake(t, &fakeWriteClient{t: t,
				GetReleaseApprovalDetailFunc: func(_ context.Context, _ string, id int) (*ado.ReleaseApprovalDetail, error) {
					return &ado.ReleaseApprovalDetail{ID: id, Status: "pending"}, nil
				},
				ApproveReleaseFunc: func(_ context.Context, _ string, _ int, comment string) error {
					got = comment
					return nil
				}})

			if code := dispatch(context.Background(), c, tc.args); code != exitOK {
				t.Fatalf("write = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
			}
			if got != tc.want {
				t.Errorf("comment reaching ADO = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRejectUsesTheRejectMethod proves approve and reject are not the same call
// with a different label: they share one constructor, so a transposition would
// otherwise be invisible. ApproveReleaseFunc stays nil, so reaching it fails by
// name.
func TestRejectUsesTheRejectMethod(t *testing.T) {
	tempAuditSink(t)
	_, stderr := captureOutput(t)

	called := false
	c := wrapFake(t, &fakeWriteClient{t: t,
		GetReleaseApprovalDetailFunc: func(_ context.Context, _ string, id int) (*ado.ReleaseApprovalDetail, error) {
			return &ado.ReleaseApprovalDetail{ID: id, Status: "pending"}, nil
		},
		RejectReleaseFunc: func(context.Context, string, int, string) error {
			called = true
			return nil
		}})

	args := []string{"releases", "reject", "77", "--project", "P", "--confirm", "--json"}
	if code := dispatch(context.Background(), c, args); code != exitOK {
		t.Fatalf("reject = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
	}
	if !called {
		t.Error("RejectRelease was not called for `releases reject`")
	}
}

// TestAbandonedPullRequestRefusesCompletion pins the second terminal status:
// abandoned cannot be completed, so it refuses as a no-op rather than being sent
// to ADO to be rejected.
func TestAbandonedPullRequestRefusesCompletion(t *testing.T) {
	tempAuditSink(t)
	_, stderr := captureOutput(t)

	c := wrapFake(t, &fakeWriteClient{t: t,
		GetPullRequestDetailFunc: func(_ context.Context, _, _ string, prID int) (*ado.PullRequestDetail, error) {
			return &ado.PullRequestDetail{ID: prID, Status: "abandoned"}, nil
		}})

	args := []string{"prs", "complete", "4821", "--project", "P", "--repo", "R", "--confirm", "--json"}
	if code := dispatch(context.Background(), c, args); code != exitNoOp {
		t.Fatalf("completing an abandoned PR = %d, want %d (stderr: %q)", code, exitNoOp, stderr.String())
	}
}
