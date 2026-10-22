package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"sentinovo.ai/bizInt/internal/ado"
)

// workItemsFixture is the fixture every filter-flag case reads. One item per
// filter axis, spread over two projects so --project has something to narrow.
func workItemsFixture() []ado.WorkItem {
	return []ado.WorkItem{
		{ID: 1, Title: "Alpha bug", Type: "Bug", State: "Active", Project: "one", AssignedToEmail: "me@x.com"},
		{ID: 2, Title: "Beta task", Type: "Task", State: "Active", Project: "one", AssignedToEmail: "other@x.com"},
		{ID: 3, Title: "Gamma bug", Type: "Bug", State: "Done", Project: "two", AssignedToEmail: "me@x.com"},
		{ID: 4, Title: "Delta bug", Type: "Bug", State: "Active", Project: "two", AssignedToEmail: "other@x.com"},
	}
}

// workItemsClient returns a fake whose GetWorkItems honours its projects
// argument the way a real read does: an empty slice means every configured
// project, a non-empty one narrows. It records the argument it was called with
// so a test can assert what the closure actually passed, not merely what came
// back.
func workItemsClient(t *testing.T, identity string, gotProjects *[]string) *fakeReadClient {
	t.Helper()
	return &fakeReadClient{
		t:                t,
		currentUserEmail: identity,
		GetWorkItemsFunc: func(_ context.Context, projects []string) (ado.Result[ado.WorkItem], error) {
			if gotProjects != nil {
				*gotProjects = projects
			}
			items := []ado.WorkItem{}
			for _, item := range workItemsFixture() {
				if len(projects) == 0 {
					items = append(items, item)
					continue
				}
				for _, p := range projects {
					if item.Project == p {
						items = append(items, item)
						break
					}
				}
			}
			return ado.Result[ado.WorkItem]{Items: items, Failures: []ado.Failure{}, Complete: true}, nil
		},
	}
}

// decodeWorkItemIDs runs a command under --json and returns the emitted item
// IDs in order. Going through the real dispatch path rather than calling a
// filter directly is what makes these tests about the CLI's behaviour.
func decodeWorkItemIDs(t *testing.T, stdout interface{ Bytes() []byte }) []int {
	t.Helper()
	var envelope struct {
		Items []struct {
			ID int `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v (raw: %q)", err, stdout.Bytes())
	}
	ids := make([]int, 0, len(envelope.Items))
	for _, item := range envelope.Items {
		ids = append(ids, item.ID)
	}
	return ids
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestDefaultsApplyNoFilter is the load-bearing test for D-18 and threat
// T-03-11. The work items TUI defaults assigned-to-me and hide-done to on; a
// CLI that inherited those defaults would hand an agent a filtered subset
// carrying complete:true. A read with zero filter flags must return every item.
func TestDefaultsApplyNoFilter(t *testing.T) {
	stdout, stderr := captureOutput(t)

	code := dispatch(context.Background(), workItemsClient(t, "me@x.com", nil),
		[]string{"workitems", "list", "--json"})
	if code != exitOK {
		t.Fatalf("dispatch = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
	}

	if got := decodeWorkItemIDs(t, stdout); !equalInts(got, []int{1, 2, 3, 4}) {
		t.Errorf("no-filter read returned IDs %v, want every fixture item [1 2 3 4]", got)
	}
	if stderr.Len() != 0 {
		t.Errorf("no-filter read wrote to stderr: %q", stderr.String())
	}
}

// TestFilterFlagsPostFilterItems covers the four flags that have no read-level
// parameter and are therefore applied to the returned Items slice.
func TestFilterFlagsPostFilterItems(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []int
	}{
		{"hide-done drops the done item", []string{"--hide-done"}, []int{1, 2, 4}},
		{"hide-tasks drops the Task", []string{"--hide-tasks"}, []int{1, 3, 4}},
		{"assigned-to-me keeps only the cached identity", []string{"--assigned-to-me"}, []int{1, 3}},
		{"search matches the title", []string{"--search", "gamma"}, []int{3}},
		{"search matches the formatted ID", []string{"--search", "4"}, []int{4}},
		{"search is case-insensitive", []string{"--search", "DELTA"}, []int{4}},
		{"an empty search disables the filter", []string{"--search", ""}, []int{1, 2, 3, 4}},
		{"flags compose", []string{"--assigned-to-me", "--hide-done"}, []int{1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t)
			args := append([]string{"workitems", "list", "--json"}, tt.args...)

			code := dispatch(context.Background(), workItemsClient(t, "me@x.com", nil), args)
			if code != exitOK {
				t.Fatalf("dispatch(%v) = %d, want %d (stderr: %q)", args, code, exitOK, stderr.String())
			}
			if got := decodeWorkItemIDs(t, stdout); !equalInts(got, tt.want) {
				t.Errorf("dispatch(%v) returned IDs %v, want %v", args, got, tt.want)
			}
		})
	}
}

// TestFilterFlagsProjectNarrowsTheRead pins that --project is passed as the
// read method's projects argument rather than post-filtered, and that omitting
// it passes an empty slice — never the configured project list, which the read
// method already falls back to on its own.
func TestFilterFlagsProjectNarrowsTheRead(t *testing.T) {
	withConfiguredProjects(t, "one")
	t.Run("a supplied project reaches the read method", func(t *testing.T) {
		stdout, stderr := captureOutput(t)
		var got []string

		code := dispatch(context.Background(), workItemsClient(t, "", &got),
			[]string{"workitems", "list", "--json", "--project", "one"})
		if code != exitOK {
			t.Fatalf("dispatch = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
		}
		if len(got) != 1 || got[0] != "one" {
			t.Errorf("GetWorkItems called with projects %v, want [one]", got)
		}
		if ids := decodeWorkItemIDs(t, stdout); !equalInts(ids, []int{1, 2}) {
			t.Errorf("--project one returned IDs %v, want only project one's items [1 2]", ids)
		}
	})

	t.Run("an omitted project passes an empty slice and reads every configured project", func(t *testing.T) {
		stdout, stderr := captureOutput(t)
		var got []string

		code := dispatch(context.Background(), workItemsClient(t, "", &got),
			[]string{"workitems", "list", "--json"})
		if code != exitOK {
			t.Fatalf("dispatch = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
		}
		if len(got) != 0 {
			t.Errorf("GetWorkItems called with projects %v, want an empty slice", got)
		}
		if ids := decodeWorkItemIDs(t, stdout); !equalInts(ids, []int{1, 2, 3, 4}) {
			t.Errorf("omitted --project returned IDs %v, want items from both projects", ids)
		}
	})
}

// TestUnknownProjectRejected pins D-09: a --project value that is not in the
// configured list exits 3 before any read or write is attempted.
//
// The rejection is fail-closed across the whole invocation, so a bogus value
// fails the command wherever it sits beside valid ones, and a nil configured
// list rejects everything. Matching is exact and case-sensitive (RESEARCH
// assumption A2): `p` against a configured `P` must exit 3, never be folded
// onto the near-miss project.
//
// Rejected rows set no fake closure, so any client call fails the row by name.
// Every row points the audit sink at a temp path, and a rejected row asserts
// the file was never created: the check precedes auditOpen (D-13).
func TestUnknownProjectRejected(t *testing.T) {
	cases := []struct {
		name       string
		configured []string
		args       []string
		wantExit   int
		wantStderr string   // rejected rows: the quoted offending value
		wantRead   []string // accepted rows: the projects the read received
	}{
		{name: "unknown project", configured: []string{"P"},
			args: []string{"workitems", "list", "--project", "bogus"}, wantExit: exitUnknown, wantStderr: `"bogus"`},
		{name: "bogus after a valid value in one flag", configured: []string{"P"},
			args: []string{"workitems", "list", "--project", "P,bogus"}, wantExit: exitUnknown, wantStderr: `"bogus"`},
		{name: "bogus before a valid repeated flag", configured: []string{"P"},
			args: []string{"workitems", "list", "--project", "bogus", "--project", "P"}, wantExit: exitUnknown, wantStderr: `"bogus"`},
		{name: "bogus before a valid value in one flag", configured: []string{"P"},
			args: []string{"workitems", "list", "--project", "bogus,P"}, wantExit: exitUnknown, wantStderr: `"bogus"`},
		{name: "case mismatch is not folded", configured: []string{"P"},
			args: []string{"workitems", "list", "--project", "p"}, wantExit: exitUnknown, wantStderr: `"p"`},
		{name: "nothing configured rejects everything", configured: nil,
			args: []string{"workitems", "list", "--project", "P"}, wantExit: exitUnknown, wantStderr: `"P"`},
		{name: "write verb set-state", configured: []string{"P"},
			args: []string{"workitems", "set-state", "4821", "--project", "bogus", "--state", "Closed", "--json"}, wantExit: exitUnknown, wantStderr: `"bogus"`},
		{name: "write verb prs approve", configured: []string{"P"},
			args: []string{"prs", "approve", "4821", "--project", "bogus", "--repo", "R"}, wantExit: exitUnknown, wantStderr: `"bogus"`},

		{name: "configured project reaches the read", configured: []string{"P"},
			args: []string{"workitems", "list", "--project", "P", "--json"}, wantExit: exitOK, wantRead: []string{"P"}},
		{name: "empty value contributes nothing", configured: []string{"P"},
			args: []string{"workitems", "list", "--project", "", "--json"}, wantExit: exitOK, wantRead: []string{}},
		{name: "separator-only value contributes nothing", configured: []string{"P"},
			args: []string{"workitems", "list", "--project", " , ", "--json"}, wantExit: exitOK, wantRead: []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withConfiguredProjects(t, tc.configured...)
			auditFile := tempAuditSink(t)
			stdout, stderr := captureOutput(t)

			if tc.wantExit == exitOK {
				var got []string
				code := dispatch(context.Background(), workItemsClient(t, "", &got), tc.args)
				if code != exitOK {
					t.Fatalf("exit = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
				}
				// emptyIfNil never passes nil, so nil here means the read never ran.
				if got == nil || !slices.Equal(got, tc.wantRead) {
					t.Errorf("GetWorkItems called with projects %q, want %q", got, tc.wantRead)
				}
				return
			}

			c := &fakeReadClient{t: t, fakeWriteClient: &fakeWriteClient{t: t}}
			if code := dispatch(context.Background(), c, tc.args); code != tc.wantExit {
				t.Fatalf("exit = %d, want %d (stderr: %q)", code, tc.wantExit, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("rejection wrote %q to stdout; want nothing", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stderr %q does not name the rejected value %s", stderr.String(), tc.wantStderr)
			}
			if _, err := os.Stat(auditFile); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("audit file stat err = %v, want not-exist: the rejection must precede auditOpen", err)
			}
		})
	}
}

// branchesClient returns a fake whose GetBranches honours its repositories
// argument the way the real one does (branches.go:92) and records it. The
// fixture deliberately includes a repository outside any plausible config list,
// so a closure that leaked cfg.Repositories into the call would be visible.
func branchesClient(t *testing.T, gotRepos *[]string) *fakeReadClient {
	t.Helper()
	all := []ado.Branch{
		{Name: "main", RepositoryName: "r1"},
		{Name: "feature/x", RepositoryName: "r2"},
		{Name: "hotfix/y", RepositoryName: "outside-the-config"},
	}
	return &fakeReadClient{
		t: t,
		GetBranchesFunc: func(_ context.Context, _ []string, repositories []string) (ado.Result[ado.Branch], error) {
			if gotRepos != nil {
				*gotRepos = repositories
			}
			items := []ado.Branch{}
			for _, b := range all {
				if ado.MatchesRepository(b.RepositoryName, repositories) {
					items = append(items, b)
				}
			}
			return ado.Result[ado.Branch]{Items: items, Failures: []ado.Failure{}, Complete: true}, nil
		},
	}
}

func decodeBranchNames(t *testing.T, stdout interface{ Bytes() []byte }) []string {
	t.Helper()
	var envelope struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v (raw: %q)", err, stdout.Bytes())
	}
	names := make([]string, 0, len(envelope.Items))
	for _, item := range envelope.Items {
		names = append(names, item.Name)
	}
	return names
}

// TestFilterFlagsRepoNarrowsTheBranchesRead pins the branches special case:
// --repo is GetBranches' repositories argument, not a post-filter, and an
// omitted flag passes an empty slice rather than cfg.Repositories. A closure
// that passed the configured list would silently hand a caller who asked for no
// repository filter a config-filtered subset.
func TestFilterFlagsRepoNarrowsTheBranchesRead(t *testing.T) {
	t.Run("no --repo passes an empty slice and returns branches outside any config list", func(t *testing.T) {
		stdout, stderr := captureOutput(t)
		var got []string

		code := dispatch(context.Background(), branchesClient(t, &got),
			[]string{"branches", "list", "--json"})
		if code != exitOK {
			t.Fatalf("dispatch = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
		}
		if len(got) != 0 {
			t.Errorf("GetBranches called with repositories %v, want an empty slice", got)
		}
		names := decodeBranchNames(t, stdout)
		if len(names) != 3 {
			t.Fatalf("omitted --repo returned %v, want all three branches", names)
		}
		if names[2] != "hotfix/y" {
			t.Errorf("the branch in a repository outside the config list was dropped: %v", names)
		}
	})

	t.Run("--repo reaches GetBranches and is not additionally post-filtered", func(t *testing.T) {
		stdout, stderr := captureOutput(t)
		var got []string

		code := dispatch(context.Background(), branchesClient(t, &got),
			[]string{"branches", "list", "--json", "--repo", "r1"})
		if code != exitOK {
			t.Fatalf("dispatch = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
		}
		if len(got) != 1 || got[0] != "r1" {
			t.Errorf("GetBranches called with repositories %v, want [r1]", got)
		}
		if names := decodeBranchNames(t, stdout); len(names) != 1 || names[0] != "main" {
			t.Errorf("--repo r1 returned %v, want [main]", names)
		}
	})

	// The post-filter for branches applies search only. If it also applied
	// --repo, a repository the read already accepted would be filtered a second
	// time — harmless here but a latent double-filter. This asserts the
	// post-filter passes through everything the read returned.
	t.Run("the branches post-filter does not re-apply the repository rule", func(t *testing.T) {
		ff := &filterFlags{repos: stringList{"nothing-matches"}}
		in := []ado.Branch{{Name: "main", RepositoryName: "r1"}}
		if got := filterBranches(in, ff, nil); len(got) != 1 {
			t.Errorf("filterBranches applied the repository rule; returned %d items, want 1", len(got))
		}
	})
}

// TestFilterFlagsAssignedToMeWarnsOnEmptyIdentity covers threat T-03-12. Phase 1
// D-12 leaves identity resolution non-fatal, so an empty cache is legitimate
// and ado.MatchesIdentity is a deliberate no-op on it. Returning the unfiltered
// list in silence would leave the caller believing a filter applied.
func TestFilterFlagsAssignedToMeWarnsOnEmptyIdentity(t *testing.T) {
	stdout, stderr := captureOutput(t)

	code := dispatch(context.Background(), workItemsClient(t, "", nil),
		[]string{"workitems", "list", "--json", "--assigned-to-me"})
	if code != exitOK {
		t.Fatalf("dispatch = %d, want %d (stderr: %q)", code, exitOK, stderr.String())
	}
	if got := decodeWorkItemIDs(t, stdout); !equalInts(got, []int{1, 2, 3, 4}) {
		t.Errorf("--assigned-to-me with an empty identity returned IDs %v, want every item", got)
	}
	if stderr.Len() == 0 {
		t.Fatal("--assigned-to-me with an empty identity wrote nothing to stderr; the no-op must be announced")
	}
	if !strings.Contains(stderr.String(), "--assigned-to-me") {
		t.Errorf("stderr warning does not name the flag: %q", stderr.String())
	}
}

// TestFilterFlagsRegisteredOnSupportedVerbsOnly pins that each filter flag
// appears in the --help output of the verbs whose payload supports it, and
// nowhere else. A flag that parses but cannot mean anything is worse than one
// that does not exist: an agent reading help would believe it filters.
func TestFilterFlagsRegisteredOnSupportedVerbsOnly(t *testing.T) {
	wantFlags := map[string][]string{
		"workitems list":        {"search", "assigned-to-me", "hide-done", "hide-tasks"},
		"prs list":              {"search", "repo"},
		"branches list":         {"search", "repo"},
		"releases list":         {"search"},
		"pipelines runs":        {"search"},
		"pipelines definitions": {},
		"releases approvals":    {},
		"dashboard counts":      {},
		"teams list":            {},
		"teams members":         {},
		"teams process":         {},
		"sprint current":        {},
		"workitems set-state":   {},
		// The two prs write verbs register --repo, but from writeSupport rather
		// than filterSupport: on a write it names the repository holding the
		// pull request with an arity of exactly one, it does not narrow a read.
		// This test can only see the FlagSet, so the expectation is recorded
		// here; TestWriteRepoArityIsExactlyOne pins the difference in meaning.
		"prs approve":      {"repo"},
		"prs complete":     {"repo"},
		"releases approve": {},
		"releases reject":  {},
		"pipelines queue":  {},
	}
	allFilterFlags := []string{"search", "assigned-to-me", "hide-done", "hide-tasks", "repo"}

	for _, name := range commandKeys() {
		t.Run(name, func(t *testing.T) {
			want, ok := wantFlags[name]
			if !ok {
				t.Fatalf("command %q is not covered by this test's expectation table", name)
			}
			registered := map[string]bool{}
			fs, _, _, _ := newFlagSet(name)
			for _, flagName := range allFilterFlags {
				if fs.Lookup(flagName) != nil {
					registered[flagName] = true
				}
			}
			for _, flagName := range want {
				if !registered[flagName] {
					t.Errorf("%q does not register --%s", name, flagName)
				}
			}
			for _, flagName := range allFilterFlags {
				if registered[flagName] && !contains(want, flagName) {
					t.Errorf("%q registers --%s, which its payload does not support", name, flagName)
				}
			}
		})
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
