package main

import (
	"context"
	"testing"

	"sentinovo.ai/bizInt/internal/ado"
)

// This file expands the tracer's guarantees across all six write verbs. Every
// assertion here is stated once and driven from a table, because the point of
// 04-03 is that the six verbs are the SAME contract rather than six variations
// of it: a guard proven per-verb by hand is a guard that can be forgotten on
// verb seven.

// writeVerbFixture describes one write verb well enough to drive the whole
// guard set against it.
//
// newFake is the load-bearing field. It sets ONLY the pre-read closure unless
// allowMutate is true, so on every dry-run and no-op row the verb's mutating
// closure stays nil and any mutation fails the subtest by name at the call
// site. That nil field IS the "no mutating call" assertion; a counter asserted
// == 0 would only record a call that had already happened.
type writeVerbFixture struct {
	key string
	// args is a complete dispatch invocation in --json mode, with every
	// required flag present and --confirm absent.
	args []string
	// hasNoOpCondition is false only for `pipelines queue`: a queue is not an
	// idempotent state change, so no pre-read state makes it redundant.
	hasNoOpCondition bool
	// wantChange is the single diff row a pending target produces.
	wantChange fieldChange
	newFake    func(t *testing.T, applied, allowMutate bool, mutateErr error) *fakeReadClient
	// parseErrors are invocations that must exit 3 with no client call at all.
	parseErrors map[string][]string
}

func wrapFake(t *testing.T, w *fakeWriteClient) *fakeReadClient {
	t.Helper()
	return &fakeReadClient{t: t, fakeWriteClient: w}
}

func writeVerbFixtures() []writeVerbFixture {
	return []writeVerbFixture{
		{
			key:              "workitems set-state",
			args:             []string{"workitems", "set-state", "4821", "--project", "P", "--state", "Closed", "--json"},
			hasNoOpCondition: true,
			wantChange:       fieldChange{Field: fieldState, From: "Active", To: "Closed"},
			newFake: func(t *testing.T, applied, allowMutate bool, mutateErr error) *fakeReadClient {
				w := &fakeWriteClient{t: t,
					GetWorkItemDetailFunc: func(_ context.Context, id int) (*ado.WorkItemDetail, error) {
						d := &ado.WorkItemDetail{}
						d.ID = id
						d.State = "Active"
						if applied {
							d.State = "Closed"
						}
						return d, nil
					}}
				if allowMutate {
					w.UpdateWorkItemStateFunc = func(context.Context, int, string) error { return mutateErr }
				}
				return wrapFake(t, w)
			},
			parseErrors: map[string][]string{
				"missing state": {"workitems", "set-state", "4821", "--project", "P"},
			},
		},
		{
			key:              "prs approve",
			args:             []string{"prs", "approve", "4821", "--project", "P", "--repo", "R", "--json"},
			hasNoOpCondition: true,
			wantChange:       fieldChange{Field: fieldReviewerVote, From: "0", To: "10"},
			newFake: func(t *testing.T, applied, allowMutate bool, mutateErr error) *fakeReadClient {
				w := &fakeWriteClient{t: t,
					GetPullRequestDetailFunc: func(_ context.Context, _, _ string, prID int) (*ado.PullRequestDetail, error) {
						d := &ado.PullRequestDetail{ID: prID, Status: "active"}
						if applied {
							d.MyVote = voteApproved
						}
						return d, nil
					}}
				if allowMutate {
					w.ApprovePullRequestFunc = func(context.Context, string, string, int) error { return mutateErr }
				}
				return wrapFake(t, w)
			},
			parseErrors: map[string][]string{
				"zero repos": {"prs", "approve", "4821", "--project", "P"},
				"two repos":  {"prs", "approve", "4821", "--project", "P", "--repo", "R", "--repo", "S"},
			},
		},
		{
			key:              "prs complete",
			args:             []string{"prs", "complete", "4821", "--project", "P", "--repo", "R", "--json"},
			hasNoOpCondition: true,
			wantChange:       fieldChange{Field: fieldPRStatus, From: "active", To: "completed"},
			newFake: func(t *testing.T, applied, allowMutate bool, mutateErr error) *fakeReadClient {
				w := &fakeWriteClient{t: t,
					GetPullRequestDetailFunc: func(_ context.Context, _, _ string, prID int) (*ado.PullRequestDetail, error) {
						d := &ado.PullRequestDetail{ID: prID, Status: "active"}
						if applied {
							d.Status = "completed"
						}
						return d, nil
					}}
				if allowMutate {
					w.CompletePullRequestFunc = func(context.Context, string, string, int) error { return mutateErr }
				}
				return wrapFake(t, w)
			},
			parseErrors: map[string][]string{
				"zero repos": {"prs", "complete", "4821", "--project", "P"},
				"two repos":  {"prs", "complete", "4821", "--project", "P", "--repo", "R", "--repo", "S"},
			},
		},
		{
			key:              "releases approve",
			args:             []string{"releases", "approve", "77", "--project", "P", "--json"},
			hasNoOpCondition: true,
			wantChange:       fieldChange{Field: fieldApprovalStatus, From: "pending", To: "approved"},
			newFake: func(t *testing.T, applied, allowMutate bool, mutateErr error) *fakeReadClient {
				w := &fakeWriteClient{t: t,
					GetReleaseApprovalDetailFunc: func(_ context.Context, _ string, id int) (*ado.ReleaseApprovalDetail, error) {
						d := &ado.ReleaseApprovalDetail{ID: id, Status: "pending"}
						if applied {
							d.Status = "approved"
						}
						return d, nil
					}}
				if allowMutate {
					w.ApproveReleaseFunc = func(context.Context, string, int, string) error { return mutateErr }
				}
				return wrapFake(t, w)
			},
		},
		{
			key:              "releases reject",
			args:             []string{"releases", "reject", "77", "--project", "P", "--json"},
			hasNoOpCondition: true,
			wantChange:       fieldChange{Field: fieldApprovalStatus, From: "pending", To: "rejected"},
			newFake: func(t *testing.T, applied, allowMutate bool, mutateErr error) *fakeReadClient {
				w := &fakeWriteClient{t: t,
					GetReleaseApprovalDetailFunc: func(_ context.Context, _ string, id int) (*ado.ReleaseApprovalDetail, error) {
						d := &ado.ReleaseApprovalDetail{ID: id, Status: "pending"}
						if applied {
							d.Status = "rejected"
						}
						return d, nil
					}}
				if allowMutate {
					w.RejectReleaseFunc = func(context.Context, string, int, string) error { return mutateErr }
				}
				return wrapFake(t, w)
			},
		},
		{
			key: "pipelines queue",
			// --branch is required, never defaulted from the definition (P-04).
			args:             []string{"pipelines", "queue", "12", "--project", "P", "--branch", "main", "--json"},
			hasNoOpCondition: false,
			wantChange:       fieldChange{Field: fieldQueuedBuild, From: "", To: "main"},
			newFake: func(t *testing.T, _, allowMutate bool, mutateErr error) *fakeReadClient {
				w := &fakeWriteClient{t: t,
					GetPipelineDefinitionDetailFunc: func(_ context.Context, _ string, id int) (*ado.PipelineDefinitionDetail, error) {
						return &ado.PipelineDefinitionDetail{ID: id, Name: "nightly", DefaultBranch: "trunk"}, nil
					}}
				if allowMutate {
					w.QueuePipelineFunc = func(context.Context, string, int, string) (*ado.PipelineRun, error) {
						return &ado.PipelineRun{ID: 900}, mutateErr
					}
				}
				return wrapFake(t, w)
			},
			parseErrors: map[string][]string{
				"missing branch": {"pipelines", "queue", "12", "--project", "P"},
			},
		},
	}
}

// withConfirm appends --confirm to a fixture's invocation.
func withConfirm(args []string, extra ...string) []string {
	out := append(append([]string{}, args...), "--confirm")
	return append(out, extra...)
}

// TestDryRunIssuesNoMutatingCallAllVerbs is SAFE-01, D-01, D-03, D-08, D-09 and
// the arity rules stated once for all six verbs. Every row that must not mutate
// runs against a fake whose mutating closure is nil.
func TestDryRunIssuesNoMutatingCallAllVerbs(t *testing.T) {
	fixtures := writeVerbFixtures()

	// Drift guard: a verb registered in the dispatch table but absent here is a
	// verb whose guards nothing proves.
	covered := map[string]bool{}
	for _, f := range fixtures {
		covered[f.key] = true
	}
	for key := range writeSupportByCommand {
		if !covered[key] {
			t.Errorf("write verb %q has no fixture in this table; its guards are unproven", key)
		}
	}
	if len(fixtures) != len(writeSupportByCommand) {
		t.Errorf("table covers %d verbs, writeSupportByCommand registers %d", len(fixtures), len(writeSupportByCommand))
	}

	for _, f := range fixtures {
		t.Run(f.key, func(t *testing.T) {

			t.Run("dry run reports a real diff and mutates nothing", func(t *testing.T) {
				sink := tempAuditSink(t)
				stdout, stderr := captureOutput(t)

				code := dispatch(context.Background(), f.newFake(t, false, false, nil), f.args)
				if code != exitOK {
					t.Fatalf("dry run = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
				}
				got := decodeWriteEnvelope(t, stdout.Bytes())
				if got.Executed {
					t.Error("dry run reported executed:true")
				}
				if got.Outcome != outcomeDryRun {
					t.Errorf("outcome = %q, want %q", got.Outcome, outcomeDryRun)
				}
				if got.Command != f.key {
					t.Errorf("command = %q, want %q", got.Command, f.key)
				}
				if len(got.Changes) != 1 || got.Changes[0] != f.wantChange {
					t.Errorf("changes = %#v, want exactly %#v", got.Changes, f.wantChange)
				}
				// D-14: a dry run changed nothing, so it records nothing.
				assertAuditLineCount(t, sink, 0)
			})

			if f.hasNoOpCondition {
				for _, confirm := range []bool{false, true} {
					name := "no-op refused on the dry-run path"
					args := f.args
					if confirm {
						name = "no-op refused on the --confirm path"
						args = withConfirm(f.args)
					}
					t.Run(name, func(t *testing.T) {
						sink := tempAuditSink(t)
						stdout, stderr := captureOutput(t)

						// allowMutate stays false even with --confirm: the
						// refusal precedes the confirm branch (D-03).
						code := dispatch(context.Background(), f.newFake(t, true, false, nil), args)
						if code != exitNoOp {
							t.Fatalf("refused no-op = %d, want %d (stderr: %q)", code, exitNoOp, stderr.String())
						}
						got := decodeWriteEnvelope(t, stdout.Bytes())
						if got.Outcome != outcomeNoOp {
							t.Errorf("outcome = %q, want %q", got.Outcome, outcomeNoOp)
						}
						if got.Executed {
							t.Error("refused no-op reported executed:true")
						}
						if len(got.Changes) != 0 {
							t.Errorf("changes = %#v, want empty", got.Changes)
						}
						assertAuditLineCount(t, sink, 0)
					})
				}
			}

			t.Run("confirmed write executes and audits once", func(t *testing.T) {
				sink := tempAuditSink(t)
				stdout, stderr := captureOutput(t)

				code := dispatch(context.Background(), f.newFake(t, false, true, nil), withConfirm(f.args))
				if code != exitOK {
					t.Fatalf("confirmed write = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
				}
				got := decodeWriteEnvelope(t, stdout.Bytes())
				if !got.Executed || got.Outcome != outcomeExecuted {
					t.Errorf("executed = %v, outcome = %q; want true / %q", got.Executed, got.Outcome, outcomeExecuted)
				}
				assertAuditLineCount(t, sink, 1)
			})

			t.Run("deadline exceeded is outcome-unknown, not nothing-ran", func(t *testing.T) {
				sink := tempAuditSink(t)
				stdout, _ := captureOutput(t)

				code := dispatch(context.Background(),
					f.newFake(t, false, true, context.DeadlineExceeded), withConfirm(f.args))
				if code != exitOutcomeUnknown {
					t.Fatalf("timed-out write = %d, want %d", code, exitOutcomeUnknown)
				}
				got := decodeWriteEnvelope(t, stdout.Bytes())
				if got.Outcome != outcomeUnknown {
					t.Errorf("outcome = %q, want %q", got.Outcome, outcomeUnknown)
				}
				if got.Executed {
					t.Error("an unresolved write reported executed:true")
				}
				// D-16: the case where a mutation may or may not have landed is
				// precisely the case most worth a trail.
				assertAuditLineCount(t, sink, 1)
			})

			t.Run("parse errors exit 3 with no client call", func(t *testing.T) {
				cases := map[string][]string{}
				noun, verb := splitKey(f.key)
				cases["missing positional"] = stripPositional(f.args)
				cases["two positionals"] = append([]string{noun, verb, "1", "2"}, f.args[3:]...)
				cases["non-numeric target"] = append([]string{noun, verb, "abc"}, f.args[3:]...)
				cases["zero projects"] = stripFlag(f.args, "--project")
				cases["two projects"] = append(append([]string{}, f.args...), "--project", "Q")
				for name, args := range f.parseErrors {
					cases[name] = args
				}

				for name, args := range cases {
					t.Run(name, func(t *testing.T) {
						tempAuditSink(t)
						stdout, stderr := captureOutput(t)

						// Every closure nil: any client call fails by name.
						c := &fakeReadClient{t: t, fakeWriteClient: &fakeWriteClient{t: t}}
						if code := dispatch(context.Background(), c, args); code != exitUnknown {
							t.Fatalf("exit = %d, want %d (stderr: %q)", code, exitUnknown, stderr.String())
						}
						if stdout.Len() != 0 {
							t.Errorf("parse error wrote %q to stdout; want nothing", stdout.String())
						}
					})
				}
			})

			t.Run("failed pre-read is fatal and emits no report", func(t *testing.T) {
				tempAuditSink(t)
				stdout, stderr := captureOutput(t)

				c := failingPreReadFake(t, f.key)
				if code := dispatch(context.Background(), c, f.args); code == exitOK {
					t.Fatal("failed pre-read exited 0, want non-zero")
				}
				if stdout.Len() != 0 {
					t.Errorf("failed pre-read wrote %q to stdout; want nothing", stdout.String())
				}
				if stderr.Len() == 0 {
					t.Error("failed pre-read wrote nothing to stderr; the failure must be announced")
				}
			})
		})
	}
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
