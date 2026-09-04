package ado

import (
	"context"
	"errors"
	"reflect"
	"testing"

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

// TestGetReleasesSwallowsProjectFailure pins pre-TRUST-01 behavior.
// Phase 2 inverts this characterization so the project failure reaches the caller.
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

	releases, err := client.GetReleases(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetReleases() error = %v, want nil", err)
	}
	if len(releases) != 1 {
		t.Fatalf("GetReleases() returned %d releases, want 1 surviving release: %+v", len(releases), releases)
	}
	if releases[0].Project != survivingProject || releases[0].DefinitionName != definitionName {
		t.Errorf("GetReleases()[0] = %+v, want project %q and definition %q", releases[0], survivingProject, definitionName)
	}
}

// TestGetPendingApprovalsSwallowsProjectFailure pins pre-TRUST-01 behavior.
// Phase 2 inverts this characterization so the project failure reaches the caller.
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

	approvals, err := client.GetPendingApprovals(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPendingApprovals() error = %v, want nil", err)
	}
	if len(approvals) != 1 {
		t.Fatalf("GetPendingApprovals() returned %d approvals, want 1 surviving approval: %+v", len(approvals), approvals)
	}
	if approvals[0].Project != survivingProject || approvals[0].ID != 47 {
		t.Errorf("GetPendingApprovals()[0] = %+v, want project %q and approval ID 47", approvals[0], survivingProject)
	}
}
