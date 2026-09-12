package ado

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/release"
)

// prePullRequest builds a GitPullRequest with one reviewer holding the given
// vote, so the happy-path test can assert the acting identity's vote survives.
func prePullRequest(id int, title, status string, reviewerID string, vote int) *git.GitPullRequest {
	prStatus := git.PullRequestStatus(status)
	mergeStatus := git.PullRequestAsyncStatus("succeeded")
	return &git.GitPullRequest{
		PullRequestId: &id,
		Title:         &title,
		Status:        &prStatus,
		MergeStatus:   &mergeStatus,
		Reviewers: &[]git.IdentityRefWithVote{
			{Id: &reviewerID, Vote: &vote},
		},
	}
}

func TestGetPullRequestDetail(t *testing.T) {
	c := &Client{
		currentUserID: "user-1",
		gitClient: &fakeGitAPI{
			t: t,
			GetPullRequestFunc: func(_ context.Context, args git.GetPullRequestArgs) (*git.GitPullRequest, error) {
				if args.PullRequestId == nil || *args.PullRequestId != 4821 {
					t.Fatalf("GetPullRequest args.PullRequestId = %v, want 4821", args.PullRequestId)
				}
				if args.Project == nil || *args.Project != "P" {
					t.Fatalf("GetPullRequest args.Project = %v, want P", args.Project)
				}
				if args.RepositoryId == nil || *args.RepositoryId != "R" {
					t.Fatalf("GetPullRequest args.RepositoryId = %v, want R", args.RepositoryId)
				}
				return prePullRequest(4821, "Fix the thing", "active", "user-1", 10), nil
			},
		},
	}

	got, err := c.GetPullRequestDetail(context.Background(), "P", "R", 4821)
	if err != nil {
		t.Fatalf("GetPullRequestDetail: %v", err)
	}
	if got.ID != 4821 || got.Title != "Fix the thing" || got.Status != "active" {
		t.Errorf("detail = %#v, want id 4821, title 'Fix the thing', status active", got)
	}
	if got.MergeStatus != "succeeded" {
		t.Errorf("mergeStatus = %q, want succeeded", got.MergeStatus)
	}
	if got.MyVote != 10 {
		t.Errorf("myVote = %d, want 10 (the acting identity's own vote)", got.MyVote)
	}
}

// TestGetPullRequestDetailIgnoresOtherReviewersVotes proves the vote is matched
// on the acting identity rather than on whichever reviewer happens to be first.
func TestGetPullRequestDetailIgnoresOtherReviewersVotes(t *testing.T) {
	c := &Client{
		currentUserID: "user-1",
		gitClient: &fakeGitAPI{
			t: t,
			GetPullRequestFunc: func(context.Context, git.GetPullRequestArgs) (*git.GitPullRequest, error) {
				return prePullRequest(1, "t", "active", "somebody-else", 10), nil
			},
		},
	}

	got, err := c.GetPullRequestDetail(context.Background(), "P", "R", 1)
	if err != nil {
		t.Fatalf("GetPullRequestDetail: %v", err)
	}
	if got.MyVote != 0 {
		t.Errorf("myVote = %d, want 0: another reviewer's approval is not ours", got.MyVote)
	}
}

func TestGetReleaseApprovalDetail(t *testing.T) {
	id := 77
	status := release.ApprovalStatusValues.Pending
	name := "Release-9"

	c := &Client{
		releaseClient: &fakeReleaseAPI{
			t: t,
			GetApprovalsFunc: func(_ context.Context, args release.GetApprovalsArgs) (*release.GetApprovalsResponseValue, error) {
				if args.StatusFilter == nil {
					t.Fatal("GetApprovals was called with the endpoint's pending-only default; an already-approved approval would be invisible")
				}
				return &release.GetApprovalsResponseValue{
					Value: []release.ReleaseApproval{
						{Id: &id, Status: &status, Release: &release.ReleaseShallowReference{Name: &name}},
					},
				}, nil
			},
		},
	}

	got, err := c.GetReleaseApprovalDetail(context.Background(), "P", 77)
	if err != nil {
		t.Fatalf("GetReleaseApprovalDetail: %v", err)
	}
	if got.ID != 77 || got.Status != "pending" || got.Release != "Release-9" {
		t.Errorf("detail = %#v, want id 77, status pending, release Release-9", got)
	}
}

func TestGetPipelineDefinitionDetail(t *testing.T) {
	id := 12
	name := "nightly"
	defaultBranch := "refs/heads/main"

	c := &Client{
		buildClient: &fakeBuildAPI{
			t: t,
			GetDefinitionFunc: func(_ context.Context, args build.GetDefinitionArgs) (*build.BuildDefinition, error) {
				if args.DefinitionId == nil || *args.DefinitionId != 12 {
					t.Fatalf("GetDefinition args.DefinitionId = %v, want 12", args.DefinitionId)
				}
				return &build.BuildDefinition{
					Id:         &id,
					Name:       &name,
					Repository: &build.BuildRepository{DefaultBranch: &defaultBranch},
				}, nil
			},
		},
	}

	got, err := c.GetPipelineDefinitionDetail(context.Background(), "P", 12)
	if err != nil {
		t.Fatalf("GetPipelineDefinitionDetail: %v", err)
	}
	if got.ID != 12 || got.Name != "nightly" || got.DefaultBranch != "main" {
		t.Errorf("detail = %#v, want id 12, name nightly, defaultBranch main", got)
	}
}

// TestPreReadNotFoundReturnsError is the behavioural half of the (nil, nil)
// prohibition. GetWorkItemDetail returns (nil, nil) for a missing item
// (workitems.go:384-386); a write pre-read must not, because D-02 makes a failed
// pre-read fatal precisely so a missing target can never present as an empty
// diff and a silently refused no-op (threat T-04-13).
//
// The SDK-error rows go through wrappedErrorValue/wrappedErrorPointer rather
// than errors.New because classifyFailure reads the status structurally from
// both WrappedError shapes.
func TestPreReadNotFoundReturnsError(t *testing.T) {
	cases := []struct {
		name string
		call func(*testing.T) (any, error)
	}{
		{
			name: "pull request: SDK returns nil with no error",
			call: func(t *testing.T) (any, error) {
				c := &Client{gitClient: &fakeGitAPI{t: t,
					GetPullRequestFunc: func(context.Context, git.GetPullRequestArgs) (*git.GitPullRequest, error) {
						return nil, nil
					}}}
				d, err := c.GetPullRequestDetail(context.Background(), "P", "R", 1)
				return d, err
			},
		},
		{
			name: "pull request: SDK returns 404",
			call: func(t *testing.T) (any, error) {
				c := &Client{gitClient: &fakeGitAPI{t: t,
					GetPullRequestFunc: func(context.Context, git.GetPullRequestArgs) (*git.GitPullRequest, error) {
						return nil, wrappedErrorValue(404, "not found")
					}}}
				d, err := c.GetPullRequestDetail(context.Background(), "P", "R", 1)
				return d, err
			},
		},
		{
			name: "release approval: id absent from the list",
			call: func(t *testing.T) (any, error) {
				c := &Client{releaseClient: &fakeReleaseAPI{t: t,
					GetApprovalsFunc: func(context.Context, release.GetApprovalsArgs) (*release.GetApprovalsResponseValue, error) {
						return &release.GetApprovalsResponseValue{Value: []release.ReleaseApproval{}}, nil
					}}}
				d, err := c.GetReleaseApprovalDetail(context.Background(), "P", 77)
				return d, err
			},
		},
		{
			name: "release approval: SDK returns nil response",
			call: func(t *testing.T) (any, error) {
				c := &Client{releaseClient: &fakeReleaseAPI{t: t,
					GetApprovalsFunc: func(context.Context, release.GetApprovalsArgs) (*release.GetApprovalsResponseValue, error) {
						return nil, nil
					}}}
				d, err := c.GetReleaseApprovalDetail(context.Background(), "P", 77)
				return d, err
			},
		},
		{
			name: "release approval: SDK returns 403",
			call: func(t *testing.T) (any, error) {
				c := &Client{releaseClient: &fakeReleaseAPI{t: t,
					GetApprovalsFunc: func(context.Context, release.GetApprovalsArgs) (*release.GetApprovalsResponseValue, error) {
						return nil, wrappedErrorPointer(403, "forbidden")
					}}}
				d, err := c.GetReleaseApprovalDetail(context.Background(), "P", 77)
				return d, err
			},
		},
		{
			name: "pipeline definition: SDK returns nil with no error",
			call: func(t *testing.T) (any, error) {
				c := &Client{buildClient: &fakeBuildAPI{t: t,
					GetDefinitionFunc: func(context.Context, build.GetDefinitionArgs) (*build.BuildDefinition, error) {
						return nil, nil
					}}}
				d, err := c.GetPipelineDefinitionDetail(context.Background(), "P", 12)
				return d, err
			},
		},
		{
			name: "pipeline definition: SDK returns 404",
			call: func(t *testing.T) (any, error) {
				c := &Client{buildClient: &fakeBuildAPI{t: t,
					GetDefinitionFunc: func(context.Context, build.GetDefinitionArgs) (*build.BuildDefinition, error) {
						return nil, wrappedErrorValue(404, "not found")
					}}}
				d, err := c.GetPipelineDefinitionDetail(context.Background(), "P", 12)
				return d, err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			detail, err := tc.call(t)
			if err == nil {
				t.Fatalf("missing target returned a nil error; a caller would read it as an empty diff")
			}
			if !isNilDetail(detail) {
				t.Errorf("detail = %#v, want nil alongside the error", detail)
			}
		})
	}
}

// isNilDetail reports whether a typed pointer held in an interface is nil. A
// plain `detail != nil` would be true for a typed nil pointer, which is exactly
// the confusion this test exists to rule out.
func isNilDetail(v any) bool {
	switch d := v.(type) {
	case *PullRequestDetail:
		return d == nil
	case *ReleaseApprovalDetail:
		return d == nil
	case *PipelineDefinitionDetail:
		return d == nil
	default:
		return v == nil
	}
}

// preReadFuncNames are the three write pre-reads whose bodies the structural
// assertion below walks.
var preReadFuncNames = map[string]bool{
	"GetPullRequestDetail":        true,
	"GetReleaseApprovalDetail":    true,
	"GetPipelineDefinitionDetail": true,
}

// TestPreReadNeverReturnsNilNilStructural parses preread.go and fails on any
// `return nil, nil` inside the three pre-read bodies.
//
// This is an AST walk rather than a grep for a reason that is specific to this
// repository: STATE.md records three Phase 3 incidents where a "grep returns
// nothing" criterion was satisfied and then quietly defeated by a prose comment
// explaining the forbidden thing. Comments and string literals are not part of
// the AST, so neither a doc line nor a message string can satisfy or defeat this
// assertion — only a real return statement can.
func TestPreReadNeverReturnsNilNilStructural(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "preread.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse preread.go: %v", err)
	}

	seen := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !preReadFuncNames[fn.Name.Name] {
			continue
		}
		seen[fn.Name.Name] = true

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 2 {
				return true
			}
			first, firstOK := ret.Results[0].(*ast.Ident)
			second, secondOK := ret.Results[1].(*ast.Ident)
			if firstOK && secondOK && first.Name == "nil" && second.Name == "nil" {
				t.Errorf("%s returns (nil, nil) at %s; a missing target must be an error, never an empty result",
					fn.Name.Name, fset.Position(ret.Pos()))
			}
			return true
		})
	}

	for name := range preReadFuncNames {
		if !seen[name] {
			t.Errorf("preread.go declares no %s; the structural assertion covered nothing", name)
		}
	}
}
