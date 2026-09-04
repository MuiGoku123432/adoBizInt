package ado

import (
	"context"
	"errors"
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

// TestGetPipelineRunsSwallowsProjectFailure pins pre-TRUST-01 behavior.
// Phase 2 inverts this characterization so the project failure reaches the caller.
func TestGetPipelineRunsSwallowsProjectFailure(t *testing.T) {
	const (
		failedProject    = "project-definitions-fail"
		survivingProject = "project-definitions-succeed"
		pipelineName     = "pipeline-one"
	)

	client := &Client{
		projects:  []string{failedProject, survivingProject},
		pipelines: []string{pipelineName},
		buildClient: &fakeBuildAPI{
			t: t,
			GetDefinitionsFunc: func(_ context.Context, args build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
				definitionID := 17
				definitions := &build.GetDefinitionsResponseValue{Value: []build.BuildDefinitionReference{{
					Id:   &definitionID,
					Name: strPtr(pipelineName),
				}}}
				if args.Project != nil && *args.Project == failedProject {
					return definitions, errors.New("definitions failed")
				}
				return definitions, nil
			},
			GetBuildsFunc: func(_ context.Context, _ build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
				return &build.GetBuildsResponseValue{}, nil
			},
		},
	}

	runs, err := client.GetPipelineRuns(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPipelineRuns() error = %v, want nil", err)
	}
	if len(runs) != 1 {
		t.Fatalf("GetPipelineRuns() returned %d runs, want 1 surviving run: %+v", len(runs), runs)
	}
	if runs[0].Project != survivingProject || runs[0].PipelineName != pipelineName {
		t.Errorf("GetPipelineRuns()[0] = %+v, want project %q and pipeline %q", runs[0], survivingProject, pipelineName)
	}
}

// TestGetPipelineDefinitionsSwallowsProjectFailure pins pre-TRUST-01 behavior.
// Phase 2 inverts this characterization so the project failure reaches the caller.
func TestGetPipelineDefinitionsSwallowsProjectFailure(t *testing.T) {
	const (
		failedProject    = "project-definitions-fail"
		survivingProject = "project-definitions-succeed"
		pipelineName     = "pipeline-one"
	)

	client := &Client{
		projects:  []string{failedProject, survivingProject},
		pipelines: []string{pipelineName},
		buildClient: &fakeBuildAPI{
			t: t,
			GetDefinitionsFunc: func(_ context.Context, args build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
				definitionID := 23
				definitions := &build.GetDefinitionsResponseValue{Value: []build.BuildDefinitionReference{{
					Id:   &definitionID,
					Name: strPtr(pipelineName),
				}}}
				if args.Project != nil && *args.Project == failedProject {
					return definitions, errors.New("definitions failed")
				}
				return definitions, nil
			},
			GetDefinitionFunc: func(_ context.Context, _ build.GetDefinitionArgs) (*build.BuildDefinition, error) {
				return nil, nil
			},
		},
	}

	definitions, err := client.GetPipelineDefinitions(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPipelineDefinitions() error = %v, want nil", err)
	}
	if len(definitions) != 1 {
		t.Fatalf("GetPipelineDefinitions() returned %d definitions, want 1 surviving definition: %+v", len(definitions), definitions)
	}
	if definitions[0].Project != survivingProject || definitions[0].Name != pipelineName {
		t.Errorf("GetPipelineDefinitions()[0] = %+v, want project %q and pipeline %q", definitions[0], survivingProject, pipelineName)
	}
}
