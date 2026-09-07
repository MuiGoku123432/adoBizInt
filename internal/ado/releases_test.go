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

// TestGetReleasesSwallowsProjectFailure now pins the inverted TRUST-01
// behavior. The name is retained until the phase-wide rename sweep.
func TestGetReleasesSwallowsProjectFailure(t *testing.T) {
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

// TestGetPendingApprovalsSwallowsProjectFailure now pins the inverted TRUST-01
// behavior. The name is retained until the phase-wide rename sweep.
func TestGetPendingApprovalsSwallowsProjectFailure(t *testing.T) {
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
