package ado

import (
	"context"
	"errors"
	"go/ast"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/release"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"

	"sentinovo.ai/bizInt/internal/config"
	"sentinovo.ai/bizInt/internal/transport"
)

// writeMethods are the five narrow-interface methods that change state in
// Azure DevOps. No retry decorator may declare one (D-03, T-08-26). Every
// interface method is in exactly one of writeMethods and readMethods.
var writeMethods = map[string]bool{
	"UpdateWorkItem":            true,
	"CreatePullRequestReviewer": true,
	"UpdatePullRequest":         true,
	"QueueBuild":                true,
	"UpdateReleaseApproval":     true,
}

// readMethods are the narrow-interface methods that only read and must each
// have a retrying override. A method goes in exactly one of the two lists, and
// a new method is classified before anyone decides whether it may retry.
var readMethods = map[string]bool{
	"GetTeams":                 true,
	"GetWorkItems":             true,
	"GetWorkItem":              true,
	"QueryByWiql":              true,
	"GetClassificationNode":    true,
	"GetAttachmentContent":     true,
	"GetBranches":              true,
	"GetPullRequest":           true,
	"GetPullRequestsByProject": true,
	"GetRefs":                  true,
	"GetRepositories":          true,
	"GetBuilds":                true,
	"GetDefinition":            true,
	"GetDefinitions":           true,
	"GetApprovals":             true,
	"GetReleaseDefinitions":    true,
	"GetReleases":              true,
}

// TestRetryRead pins the D-03 rules over a fail-fast fake behind retryingGit,
// with no transport: only a raw 429 is re-issued, at most twice, and an
// exhausted read keeps its 429 reachable so it still classifies rate_limit
// (D-08). A D-01 fail-fast, any other status and an ended context are returned
// on the first attempt.
func TestRetryRead(t *testing.T) {
	throttled := func() error { return wrappedErrorValue(http.StatusTooManyRequests, "throttled") }
	tests := []struct {
		name        string
		errs        func(t *testing.T) []error // one per attempt; nil is success
		cancelled   bool
		wantCalls   int
		wantRetries int64
		wantErr     string // substring of the returned error; "" means success
		wantSame    bool   // the error returned is the last injected one, unchanged
	}{
		{"429 then success", func(*testing.T) []error { return []error{throttled(), nil} }, false, 2, 1, "", false},
		{"two 429s then success", func(*testing.T) []error { return []error{throttled(), throttled(), nil} }, false, 3, 2, "", false},
		{"three 429s", func(*testing.T) []error { return []error{throttled(), throttled(), throttled()} }, false, 3, 2, "rate limited after 2 retries", false},
		{"D-01 fail-fast", func(t *testing.T) []error { return []error{failFastErr(t)} }, false, 1, 0, "ADO asked for", true},
		{"status 500", func(*testing.T) []error { return []error{wrappedErrorValue(http.StatusInternalServerError, "boom")} }, false, 1, 0, "boom", true},
		{"429 under a cancelled context", func(*testing.T) []error { return []error{throttled()} }, true, 1, 0, "throttled", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.errs(t)
			calls := 0
			retries := new(atomic.Int64)
			var api gitAPI = retryingGit{gitAPI: &fakeGitAPI{t: t,
				GetRepositoriesFunc: func(context.Context, git.GetRepositoriesArgs) (*[]git.GitRepository, error) {
					calls++
					if calls > len(errs) {
						t.Fatalf("attempt %d, but the row scripts only %d", calls, len(errs))
					}
					if err := errs[calls-1]; err != nil {
						return nil, err
					}
					return &[]git.GitRepository{}, nil
				},
			}, retries: retries}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancelled {
				cancel()
			}
			_, err := api.GetRepositories(ctx, git.GetRepositoriesArgs{})

			if calls != tt.wantCalls {
				t.Errorf("GetRepositories attempts = %d, want %d", calls, tt.wantCalls)
			}
			if got := retries.Load(); got != tt.wantRetries {
				t.Errorf("retries counted = %d, want %d", got, tt.wantRetries)
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("GetRepositories() error = %v, want success", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("GetRepositories() error = %v, want it to contain %q", err, tt.wantErr)
			}
			last := errs[len(errs)-1]
			if !errors.Is(err, last) {
				t.Errorf("returned error does not wrap the last attempt's error %v", last)
			}
			if tt.wantSame && err.Error() != last.Error() {
				t.Errorf("returned error = %q, want the attempt's error unchanged (%q)", err, last)
			}
			if tt.name == "three 429s" {
				if got := classifyFailure(err); got != FailureRateLimit {
					t.Errorf("classifyFailure(exhausted) = %q, want %q (D-08)", got, FailureRateLimit)
				}
			}
		})
	}
}

// TestRetryWritePassesThroughOnce proves the five writes reach the wrapped
// client exactly once through their decorator, even on a 429, and are never
// counted as retries (D-03, T-08-26). Each call goes through the interface
// type, as production code calls it.
func TestRetryWritePassesThroughOnce(t *testing.T) {
	tests := []struct {
		name string
		call func(t *testing.T, retries *atomic.Int64, fail func() error) error
	}{
		{"UpdateWorkItem", func(t *testing.T, retries *atomic.Int64, fail func() error) error {
			var api workItemAPI = retryingWorkItems{workItemAPI: &fakeWorkItemAPI{t: t,
				UpdateWorkItemFunc: func(context.Context, workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
					return nil, fail()
				},
			}, retries: retries}
			_, err := api.UpdateWorkItem(context.Background(), workitemtracking.UpdateWorkItemArgs{})
			return err
		}},
		{"CreatePullRequestReviewer", func(t *testing.T, retries *atomic.Int64, fail func() error) error {
			var api gitAPI = retryingGit{gitAPI: &fakeGitAPI{t: t,
				CreatePullRequestReviewerFunc: func(context.Context, git.CreatePullRequestReviewerArgs) (*git.IdentityRefWithVote, error) {
					return nil, fail()
				},
			}, retries: retries}
			_, err := api.CreatePullRequestReviewer(context.Background(), git.CreatePullRequestReviewerArgs{})
			return err
		}},
		{"UpdatePullRequest", func(t *testing.T, retries *atomic.Int64, fail func() error) error {
			var api gitAPI = retryingGit{gitAPI: &fakeGitAPI{t: t,
				UpdatePullRequestFunc: func(context.Context, git.UpdatePullRequestArgs) (*git.GitPullRequest, error) {
					return nil, fail()
				},
			}, retries: retries}
			_, err := api.UpdatePullRequest(context.Background(), git.UpdatePullRequestArgs{})
			return err
		}},
		{"QueueBuild", func(t *testing.T, retries *atomic.Int64, fail func() error) error {
			var api buildAPI = retryingBuild{buildAPI: &fakeBuildAPI{t: t,
				QueueBuildFunc: func(context.Context, build.QueueBuildArgs) (*build.Build, error) {
					return nil, fail()
				},
			}, retries: retries}
			_, err := api.QueueBuild(context.Background(), build.QueueBuildArgs{})
			return err
		}},
		{"UpdateReleaseApproval", func(t *testing.T, retries *atomic.Int64, fail func() error) error {
			var api releaseAPI = retryingRelease{releaseAPI: &fakeReleaseAPI{t: t,
				UpdateReleaseApprovalFunc: func(context.Context, release.UpdateReleaseApprovalArgs) (*release.ReleaseApproval, error) {
					return nil, fail()
				},
			}, retries: retries}
			_, err := api.UpdateReleaseApproval(context.Background(), release.UpdateReleaseApprovalArgs{})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			want := wrappedErrorValue(http.StatusTooManyRequests, "throttled")
			retries := new(atomic.Int64)

			err := tt.call(t, retries, func() error { calls++; return want })

			if calls != 1 {
				t.Errorf("%s reached the wrapped client %d times, want exactly 1: a write must never be re-sent", tt.name, calls)
			}
			if got := retries.Load(); got != 0 {
				t.Errorf("retries counted = %d, want 0", got)
			}
			if !errors.Is(err, want) || err.Error() != want.Error() {
				t.Errorf("%s error = %v, want the 429 unchanged", tt.name, err)
			}
		})
	}
}

// TestRetryNeverWrapsWrites keeps the decorators exhaustive for reads and
// empty of writes, over the AST so a comment can neither satisfy nor defeat it
// (STATE.md F-03-3). Every method of every interface in interfaces.go must be
// classified in exactly one of readMethods and writeMethods; a method in
// neither list fails as unclassified, so a write added to an interface can
// never be mistaken for a read that needs a retry (WR-04, T-08-26). For every
// retrying* type, the methods declared on it anywhere in the package must be
// exactly the reads of the interface it embeds: a missing read override fails,
// and so does any write override (D-03).
func TestRetryNeverWrapsWrites(t *testing.T) {
	_, files := parsePackageFiles(t)
	if files["retry.go"] == nil || files["interfaces.go"] == nil {
		t.Fatalf("parsed files do not include retry.go and interfaces.go; the structural assertion would cover nothing")
	}

	// Interface name -> method names, from interfaces.go.
	ifaces := map[string]map[string]bool{}
	ast.Inspect(files["interfaces.go"], func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		it, ok := spec.Type.(*ast.InterfaceType)
		if !ok {
			return true
		}
		methods := map[string]bool{}
		for _, m := range it.Methods.List {
			for _, name := range m.Names {
				methods[name.Name] = true
			}
		}
		ifaces[spec.Name.Name] = methods
		return true
	})

	// Two-sided classification: every interface method is exactly one of read
	// or write. The unclassified message must never point at an override.
	var ifaceNames []string
	for name := range ifaces {
		ifaceNames = append(ifaceNames, name)
	}
	sort.Strings(ifaceNames)
	for _, iface := range ifaceNames {
		var methods []string
		for m := range ifaces[iface] {
			methods = append(methods, m)
		}
		sort.Strings(methods)
		for _, m := range methods {
			switch {
			case readMethods[m] && writeMethods[m]:
				t.Errorf("%s.%s is in both readMethods and writeMethods; it must be exactly one (D-03)", iface, m)
			case !readMethods[m] && !writeMethods[m]:
				t.Errorf("%s.%s is unclassified: add it to readMethods or writeMethods in retry_test.go before deciding whether it may retry; a write must never be retried (D-03)", iface, m)
			}
		}
	}

	for _, list := range []struct {
		kind    string
		methods map[string]bool
	}{{"write", writeMethods}, {"read", readMethods}} {
		for m := range list.methods {
			found := false
			for _, methods := range ifaces {
				found = found || methods[m]
			}
			if !found {
				t.Errorf("%s %s is in no interface in interfaces.go; the %s list is stale and the check is partly vacuous", list.kind, m, list.kind)
			}
		}
	}

	// Decorator name -> embedded interface name, from retry.go.
	decorators := map[string]string{}
	ast.Inspect(files["retry.go"], func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok || !strings.HasPrefix(spec.Name.Name, "retrying") {
			return true
		}
		st, ok := spec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, f := range st.Fields.List {
			if id, ok := f.Type.(*ast.Ident); ok && len(f.Names) == 0 && ifaces[id.Name] != nil {
				decorators[spec.Name.Name] = id.Name
			}
		}
		if decorators[spec.Name.Name] == "" {
			t.Errorf("%s embeds none of the interfaces in interfaces.go", spec.Name.Name)
		}
		return true
	})

	// Decorator name -> methods declared on it, value or pointer receiver, in
	// any file of the package.
	declared := map[string]map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			recv := fn.Recv.List[0].Type
			if star, ok := recv.(*ast.StarExpr); ok {
				recv = star.X
			}
			id, ok := recv.(*ast.Ident)
			if !ok || !strings.HasPrefix(id.Name, "retrying") {
				continue
			}
			if declared[id.Name] == nil {
				declared[id.Name] = map[string]bool{}
			}
			declared[id.Name][fn.Name.Name] = true
		}
	}

	overrides := 0
	for decorator, iface := range decorators {
		got := declared[decorator]
		overrides += len(got)
		for m := range got {
			switch {
			case writeMethods[m]:
				t.Errorf("%s overrides the write %s; a write must never be retried (D-03)", decorator, m)
			case !ifaces[iface][m]:
				t.Errorf("%s declares %s, which %s does not have", decorator, m, iface)
			}
		}
		var missing []string
		for m := range ifaces[iface] {
			if readMethods[m] && !writeMethods[m] && !got[m] {
				missing = append(missing, m)
			}
		}
		sort.Strings(missing)
		for _, m := range missing {
			t.Errorf("%s does not override the read %s.%s, so it is never retried", decorator, iface, m)
		}
	}
	for name := range declared {
		if decorators[name] == "" {
			t.Errorf("methods are declared on %s, which is not a decorator over an interface in interfaces.go", name)
		}
	}

	if len(decorators) < 4 || overrides < 16 {
		t.Errorf("found %d decorator types and %d overrides, want at least 4 and 16; the assertion covered too little", len(decorators), overrides)
	}
}

// TestStartupReadsRetry proves the two startup reads retry under the same rule
// as every other read: resource-area discovery and the identity
// GetConnectionData each get one 429, are re-issued once through the pacer,
// and NewClient succeeds with the identity cached and both retries counted.
func TestStartupReadsRetry(t *testing.T) {
	orgURL := uniqueOrgURL(t)
	synctest.Test(t, func(t *testing.T) {
		var seen []v7Request
		discovery := newClientBase(t, usedAreas(orgURL), &seen)
		areasCalls, identityCalls := 0, 0
		base := rtFunc(func(r *http.Request) (*http.Response, error) {
			p := strings.ToLower(r.URL.Path)
			if r.Method == http.MethodGet && strings.HasSuffix(p, "/_apis/resourceareas") {
				areasCalls++
				if areasCalls == 1 {
					return jsonResp(r, http.StatusTooManyRequests, tf400733, "Retry-After", "1"), nil
				}
			}
			if r.Method == http.MethodGet && strings.HasSuffix(p, "/_apis/connectiondata") {
				identityCalls++
				if identityCalls == 1 {
					return jsonResp(r, http.StatusTooManyRequests, tf400733, "Retry-After", "1"), nil
				}
			}
			return discovery(r)
		})
		cfg := &config.Config{OrgURL: orgURL, PAT: "fixture-pat"}

		c, err := NewClient(cfg, transport.New(cfg.Org(), 0, base))
		if err != nil {
			t.Fatalf("NewClient() error = %v, want the startup reads to recover by retry", err)
		}
		if areasCalls != 2 || identityCalls != 2 {
			t.Errorf("resourceAreas requests = %d, connectionData requests = %d, want 2 each", areasCalls, identityCalls)
		}
		if c.currentUserID != fixtureUserID {
			t.Errorf("currentUserID = %q, want %q", c.currentUserID, fixtureUserID)
		}
		if got := c.FetchRetries(); got != 2 {
			t.Errorf("FetchRetries() = %d, want 2", got)
		}
	})
}
