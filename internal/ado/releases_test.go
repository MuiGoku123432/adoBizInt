package ado

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/release"
)

func TestApproveReleaseIssuesExactApproval(t *testing.T) {
	var got release.UpdateReleaseApprovalArgs
	client := &Client{
		releaseClient: &fakeReleaseAPI{
			t: t,
			UpdateReleaseApprovalFunc: func(_ context.Context, args release.UpdateReleaseApprovalArgs) (*release.ReleaseApproval, error) {
				got = args
				return &release.ReleaseApproval{}, nil
			},
		},
	}

	if err := client.ApproveRelease(context.Background(), "project-one", 42, "ship it"); err != nil {
		t.Fatalf("ApproveRelease() error = %v, want nil", err)
	}

	wantProject := "project-one"
	wantApprovalID := 42
	wantStatus := release.ApprovalStatusValues.Approved
	wantComment := "ship it"
	want := release.UpdateReleaseApprovalArgs{
		Project:    &wantProject,
		ApprovalId: &wantApprovalID,
		Approval: &release.ReleaseApproval{
			Status:   &wantStatus,
			Comments: &wantComment,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("UpdateReleaseApproval args = %+v, want %+v", got, want)
	}
}

func TestRejectReleaseIssuesExactApproval(t *testing.T) {
	var got release.UpdateReleaseApprovalArgs
	client := &Client{
		releaseClient: &fakeReleaseAPI{
			t: t,
			UpdateReleaseApprovalFunc: func(_ context.Context, args release.UpdateReleaseApprovalArgs) (*release.ReleaseApproval, error) {
				got = args
				return &release.ReleaseApproval{}, nil
			},
		},
	}

	if err := client.RejectRelease(context.Background(), "project-one", 42, "not ready"); err != nil {
		t.Fatalf("RejectRelease() error = %v, want nil", err)
	}

	wantProject := "project-one"
	wantApprovalID := 42
	wantStatus := release.ApprovalStatusValues.Rejected
	wantComment := "not ready"
	want := release.UpdateReleaseApprovalArgs{
		Project:    &wantProject,
		ApprovalId: &wantApprovalID,
		Approval: &release.ReleaseApproval{
			Status:   &wantStatus,
			Comments: &wantComment,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("UpdateReleaseApproval args = %+v, want %+v", got, want)
	}
}

// Regression guard: ApproveRelease and RejectRelease are byte-identical apart
// from the status constant. A copy-paste swap is exactly what a test here must catch.
func TestApproveAndRejectReleaseSendDifferentStatuses(t *testing.T) {
	var statuses []release.ApprovalStatus
	client := &Client{
		releaseClient: &fakeReleaseAPI{
			t: t,
			UpdateReleaseApprovalFunc: func(_ context.Context, args release.UpdateReleaseApprovalArgs) (*release.ReleaseApproval, error) {
				statuses = append(statuses, *args.Approval.Status)
				return &release.ReleaseApproval{}, nil
			},
		},
	}

	if err := client.ApproveRelease(context.Background(), "project-one", 42, "approve"); err != nil {
		t.Fatalf("ApproveRelease() error = %v, want nil", err)
	}
	if err := client.RejectRelease(context.Background(), "project-one", 42, "reject"); err != nil {
		t.Fatalf("RejectRelease() error = %v, want nil", err)
	}
	if len(statuses) != 2 || statuses[0] == statuses[1] {
		t.Fatalf("copy-paste swap guard: statuses = %v, want two different values", statuses)
	}
}

// Regression: a project whose release definition listing failed used to be
// dropped silently, so a caller could not tell a failed read from a project
// with no releases. The failure is now named and the result is incomplete.
func TestGetReleasesReportsProjectFailure(t *testing.T) {
	const (
		failedProject    = "project-release-definitions-fail"
		survivingProject = "project-release-definitions-succeed"
		definitionName   = "release-definition-one"
	)

	client := &Client{
		projects:           []string{failedProject, survivingProject},
		releaseDefinitions: []string{definitionName},
		releaseClient: &fakeReleaseAPI{
			t: t,
			GetReleaseDefinitionsFunc: func(_ context.Context, args release.GetReleaseDefinitionsArgs) (*release.GetReleaseDefinitionsResponseValue, error) {
				definitionID := 31
				definitions := &release.GetReleaseDefinitionsResponseValue{Value: []release.ReleaseDefinition{{
					Id:   &definitionID,
					Name: strPtr(definitionName),
				}}}
				if args.Project != nil && *args.Project == failedProject {
					return definitions, errors.New("release definitions failed")
				}
				return definitions, nil
			},
			GetReleasesFunc: func(_ context.Context, _ release.GetReleasesArgs) (*release.GetReleasesResponseValue, error) {
				return &release.GetReleasesResponseValue{}, nil
			},
		},
	}

	result, err := client.GetReleases(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetReleases() error = %v, want nil", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("GetReleases() returned %d releases, want 1 surviving release: %+v", len(result.Items), result.Items)
	}
	if result.Items[0].Project != survivingProject || result.Items[0].DefinitionName != definitionName {
		t.Errorf("GetReleases()[0] = %+v, want project %q and definition %q", result.Items[0], survivingProject, definitionName)
	}
	if len(result.Failures) != 1 || result.Failures[0].Project != failedProject || result.Failures[0].Operation == "" {
		t.Fatalf("GetReleases() Failures = %+v, want exactly one named entry for %q", result.Failures, failedProject)
	}
	if result.Complete {
		t.Errorf("GetReleases() Complete = %v, want false", result.Complete)
	}
}

// Regression: a project whose approval lookup failed used to be skipped
// silently, so a pending approval that could not be read looked exactly like no
// approval pending. The failure is now named and the result is incomplete.
func TestGetPendingApprovalsReportsProjectFailure(t *testing.T) {
	const (
		failedProject    = "project-approvals-fail"
		survivingProject = "project-approvals-succeed"
	)

	client := &Client{
		projects:           []string{failedProject, survivingProject},
		releaseDefinitions: []string{"release-definition-one"},
		releaseClient: &fakeReleaseAPI{
			t: t,
			GetApprovalsFunc: func(_ context.Context, args release.GetApprovalsArgs) (*release.GetApprovalsResponseValue, error) {
				approvalID := 47
				approvals := &release.GetApprovalsResponseValue{Value: []release.ReleaseApproval{{Id: &approvalID}}}
				if args.Project != nil && *args.Project == failedProject {
					return approvals, errors.New("approvals failed")
				}
				return approvals, nil
			},
		},
	}

	result, err := client.GetPendingApprovals(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPendingApprovals() error = %v, want nil", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("GetPendingApprovals() returned %d approvals, want 1 surviving approval: %+v", len(result.Items), result.Items)
	}
	if result.Items[0].Project != survivingProject || result.Items[0].ID != 47 {
		t.Errorf("GetPendingApprovals()[0] = %+v, want project %q and approval ID 47", result.Items[0], survivingProject)
	}
	if len(result.Failures) != 1 || result.Failures[0].Project != failedProject || result.Failures[0].Operation == "" {
		t.Fatalf("GetPendingApprovals() Failures = %+v, want exactly one named entry for %q", result.Failures, failedProject)
	}
	if result.Complete {
		t.Errorf("GetPendingApprovals() Complete = %v, want false", result.Complete)
	}
}

// releaseTestClient builds a client whose release-definition and approval calls
// behave per the supplied hook, sharing one fixture across the happy-path
// row-count and auth cases below.
func releaseTestClient(t *testing.T, projects []string, callErr func(project string) error) *Client {
	t.Helper()
	return &Client{
		projects:           projects,
		releaseDefinitions: []string{"release-definition-one"},
		releaseClient: &fakeReleaseAPI{
			t: t,
			GetReleaseDefinitionsFunc: func(_ context.Context, args release.GetReleaseDefinitionsArgs) (*release.GetReleaseDefinitionsResponseValue, error) {
				if args.Project != nil {
					if err := callErr(*args.Project); err != nil {
						return nil, err
					}
				}
				matchedID := 31
				unmatchedID := 32
				return &release.GetReleaseDefinitionsResponseValue{Value: []release.ReleaseDefinition{
					{Id: &matchedID, Name: strPtr("release-definition-one")},
					// A non-matching name and a nil name: both are data filters,
					// not failures.
					{Id: &unmatchedID, Name: strPtr("unrelated-definition")},
					{Id: &unmatchedID},
				}}, nil
			},
			GetReleasesFunc: func(_ context.Context, _ release.GetReleasesArgs) (*release.GetReleasesResponseValue, error) {
				return &release.GetReleasesResponseValue{}, nil
			},
			GetApprovalsFunc: func(_ context.Context, args release.GetApprovalsArgs) (*release.GetApprovalsResponseValue, error) {
				if args.Project != nil {
					if err := callErr(*args.Project); err != nil {
						return nil, err
					}
				}
				approvalID := 47
				return &release.GetApprovalsResponseValue{Value: []release.ReleaseApproval{{Id: &approvalID}}}, nil
			},
		},
	}
}

// TestGetReleasesHappyPathRowCountUnchanged detects a data filter accidentally
// converted into a failure record: a converted filter changes the row count on a
// fully successful fetch.
func TestGetReleasesHappyPathRowCountUnchanged(t *testing.T) {
	client := releaseTestClient(t, []string{"project-one", "project-two"}, func(string) error { return nil })

	result, err := client.GetReleases(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetReleases() error = %v, want nil", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("GetReleases() returned %d releases, want 2 — one matching definition per project", len(result.Items))
	}
	if !result.Complete || len(result.Failures) != 0 {
		t.Errorf("GetReleases() Complete/Failures = %v/%+v, want true/none", result.Complete, result.Failures)
	}
}

func TestGetPendingApprovalsHappyPathRowCountUnchanged(t *testing.T) {
	client := releaseTestClient(t, []string{"project-one", "project-two"}, func(string) error { return nil })

	result, err := client.GetPendingApprovals(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPendingApprovals() error = %v, want nil", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("GetPendingApprovals() returned %d approvals, want 2 — one per project", len(result.Items))
	}
	if !result.Complete || len(result.Failures) != 0 {
		t.Errorf("GetPendingApprovals() Complete/Failures = %v/%+v, want true/none", result.Complete, result.Failures)
	}
}

func TestGetReleasesClassifiesRejectedToken(t *testing.T) {
	client := releaseTestClient(t, []string{"project-unauthorized"}, func(string) error {
		return wrappedErrorValue(401, "TF400813: The user is not authorized")
	})

	result, err := client.GetReleases(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetReleases() error = %v, want nil", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("GetReleases() Items = %+v, want empty", result.Items)
	}
	if len(result.Failures) != 1 || result.Failures[0].Kind != FailureAuth {
		t.Fatalf("GetReleases() Failures = %+v, want one auth entry", result.Failures)
	}
	if result.Complete {
		t.Errorf("GetReleases() Complete = %v, want false", result.Complete)
	}
}

func TestGetPendingApprovalsClassifiesRejectedToken(t *testing.T) {
	client := releaseTestClient(t, []string{"project-unauthorized"}, func(string) error {
		return wrappedErrorPointer(403, "TF400813: The user is not authorized")
	})

	result, err := client.GetPendingApprovals(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPendingApprovals() error = %v, want nil", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("GetPendingApprovals() Items = %+v, want empty", result.Items)
	}
	if len(result.Failures) != 1 || result.Failures[0].Kind != FailureAuth {
		t.Fatalf("GetPendingApprovals() Failures = %+v, want one auth entry", result.Failures)
	}
	if result.Complete {
		t.Errorf("GetPendingApprovals() Complete = %v, want false", result.Complete)
	}
}

// TestFetchTimeoutMarksUnreachedReleases is D-12's gate for both release reads.
func TestFetchTimeoutMarksUnreachedReleases(t *testing.T) {
	projects := []string{"project-one", "project-two", "project-three"}

	client := &Client{
		projects:           projects,
		releaseDefinitions: []string{"release-definition-one"},
		releaseClient: &fakeReleaseAPI{
			t: t,
			GetReleaseDefinitionsFunc: func(_ context.Context, _ release.GetReleaseDefinitionsArgs) (*release.GetReleaseDefinitionsResponseValue, error) {
				t.Errorf("GetReleaseDefinitions called after the deadline elapsed; want no doomed calls issued")
				return nil, nil
			},
			GetApprovalsFunc: func(_ context.Context, _ release.GetApprovalsArgs) (*release.GetApprovalsResponseValue, error) {
				t.Errorf("GetApprovals called after the deadline elapsed; want no doomed calls issued")
				return nil, nil
			},
		},
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	releases, err := client.GetReleases(ctx, nil)
	if err != nil {
		t.Fatalf("GetReleases() error = %v, want nil", err)
	}
	assertAllUnreached(t, "GetReleases", releases.Failures, releases.Complete, projects)

	approvals, err := client.GetPendingApprovals(ctx, nil)
	if err != nil {
		t.Fatalf("GetPendingApprovals() error = %v, want nil", err)
	}
	assertAllUnreached(t, "GetPendingApprovals", approvals.Failures, approvals.Complete, projects)
}

// Regression (gap 1): a failed per-definition release lookup used to append the
// definition as "noReleases" and record nothing, so a rejected token or a
// timeout read as an empty state under complete:true. The definition is now
// absent from Items and named in Failures.
func TestGetReleasesReportsDefinitionFailure(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantKind FailureKind
	}{
		// The pointer SDK shape here and the value shape in the GetPipelineRuns
		// counterpart cover both forms azuredevops.UnwrapError returns.
		{name: "auth", err: wrappedErrorPointer(403, "TF400813: The user is not authorized"), wantKind: FailureAuth},
		{name: "timeout", err: context.DeadlineExceeded, wantKind: FailureTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				projects:           []string{"project-one"},
				releaseDefinitions: []string{"release-ok", "release-broken"},
				releaseClient: &fakeReleaseAPI{
					t: t,
					GetReleaseDefinitionsFunc: func(_ context.Context, _ release.GetReleaseDefinitionsArgs) (*release.GetReleaseDefinitionsResponseValue, error) {
						okID, brokenID := 31, 32
						return &release.GetReleaseDefinitionsResponseValue{Value: []release.ReleaseDefinition{
							{Id: &okID, Name: strPtr("release-ok")},
							{Id: &brokenID, Name: strPtr("release-broken")},
						}}, nil
					},
					GetReleasesFunc: func(_ context.Context, args release.GetReleasesArgs) (*release.GetReleasesResponseValue, error) {
						if *args.DefinitionId == 32 {
							return nil, tt.err
						}
						releaseID := 901
						return &release.GetReleasesResponseValue{Value: []release.Release{{Id: &releaseID, Name: strPtr("Release-1")}}}, nil
					},
				},
			}

			result, err := client.GetReleases(context.Background(), nil)
			if err != nil {
				t.Fatalf("GetReleases() error = %v, want nil", err)
			}
			if len(result.Items) != 1 || result.Items[0].DefinitionName != "release-ok" {
				t.Errorf("GetReleases() Items = %+v, want only release-ok — a failed lookup must not render as a noReleases row", result.Items)
			}
			assertDefinitionFailure(t, "GetReleases", result.Failures, result.Complete, "project-one", tt.wantKind, "release-broken")
		})
	}
}

// TestGetReleasesMarksUnreachedDefinitions is D-12 inside the per-definition
// loop: once the fetch deadline expires, the definitions not yet reached are
// recorded as timeouts and no further GetReleases call is issued.
func TestGetReleasesMarksUnreachedDefinitions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	calls := 0
	client := &Client{
		projects:           []string{"project-one"},
		releaseDefinitions: []string{"release-ok", "release-broken"},
		releaseClient: &fakeReleaseAPI{
			t: t,
			// note: same two-definition fixture as TestGetReleasesReportsDefinitionFailure; two uses stay inline.
			GetReleaseDefinitionsFunc: func(_ context.Context, _ release.GetReleaseDefinitionsArgs) (*release.GetReleaseDefinitionsResponseValue, error) {
				okID, brokenID := 31, 32
				return &release.GetReleaseDefinitionsResponseValue{Value: []release.ReleaseDefinition{
					{Id: &okID, Name: strPtr("release-ok")},
					{Id: &brokenID, Name: strPtr("release-broken")},
				}}, nil
			},
			GetReleasesFunc: func(ctx context.Context, _ release.GetReleasesArgs) (*release.GetReleasesResponseValue, error) {
				calls++
				if calls > 1 {
					t.Errorf("GetReleases call %d issued after the fetch deadline expired; want no doomed calls", calls)
					return nil, ctx.Err()
				}
				select {
				case <-ctx.Done():
				case <-time.After(5 * time.Second):
					t.Error("fetch deadline did not expire within 5s")
				}
				releaseID := 901
				return &release.GetReleasesResponseValue{Value: []release.Release{{Id: &releaseID, Name: strPtr("Release-1")}}}, nil
			},
		},
	}

	result, err := client.GetReleases(ctx, nil)
	if err != nil {
		t.Fatalf("GetReleases() error = %v, want nil", err)
	}
	if calls != 1 {
		t.Errorf("GetReleases calls = %d, want 1", calls)
	}
	if len(result.Items) != 1 || result.Items[0].DefinitionName != "release-ok" {
		t.Errorf("GetReleases() Items = %+v, want only release-ok", result.Items)
	}
	assertDefinitionFailure(t, "GetReleases", result.Failures, result.Complete, "project-one", FailureTimeout, "release-broken")
}
