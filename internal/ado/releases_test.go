package ado

import (
	"context"
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
