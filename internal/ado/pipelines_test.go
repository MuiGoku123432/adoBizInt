package ado

import (
	"context"
	"reflect"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
)

func TestQueuePipelineIssuesExactQueueCall(t *testing.T) {
	var got build.QueueBuildArgs
	queuedBuildID := 99
	client := &Client{
		buildClient: &fakeBuildAPI{
			t: t,
			QueueBuildFunc: func(_ context.Context, args build.QueueBuildArgs) (*build.Build, error) {
				got = args
				return &build.Build{Id: &queuedBuildID}, nil
			},
		},
	}

	if _, err := client.QueuePipeline(context.Background(), "project-one", 17, "main"); err != nil {
		t.Fatalf("QueuePipeline() error = %v, want nil", err)
	}

	wantProject := "project-one"
	wantDefinitionID := 17
	wantSourceBranch := "refs/heads/main"
	want := build.QueueBuildArgs{
		Build: &build.Build{
			Definition: &build.DefinitionReference{
				Id: &wantDefinitionID,
			},
			SourceBranch: &wantSourceBranch,
		},
		Project: &wantProject,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("QueueBuild args = %+v, want %+v", got, want)
	}
}
