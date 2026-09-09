package main

import (
	"context"
	"testing"

	"sentinovo.ai/bizInt/internal/ado"
)

// fakeReadClient satisfies readClient with per-call closure fields, mirroring
// the internal/ado fake convention: an unset closure fails the test by
// interface and method name rather than returning a misleading zero value, so
// an unexpected read is a loud failure at the call site.
type fakeReadClient struct {
	t *testing.T

	GetPullRequestsFunc func(ctx context.Context, projects []string) (ado.Result[ado.PullRequest], error)
}

func (f *fakeReadClient) GetPullRequests(ctx context.Context, projects []string) (ado.Result[ado.PullRequest], error) {
	if f.GetPullRequestsFunc == nil {
		f.t.Fatalf("readClient.GetPullRequests called with no GetPullRequestsFunc set")
	}
	return f.GetPullRequestsFunc(ctx, projects)
}
