package ado

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

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

// Regression: a project whose build listing failed used to be dropped silently,
// so a caller could not tell a failed read from a project with no runs. The
// failure is now named in the envelope and the result is marked incomplete.
func TestGetPipelineRunsReportsProjectFailure(t *testing.T) {
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

	result, err := client.GetPipelineRuns(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPipelineRuns() error = %v, want nil", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("GetPipelineRuns() returned %d runs, want 1 surviving run: %+v", len(result.Items), result.Items)
	}
	if result.Items[0].Project != survivingProject || result.Items[0].PipelineName != pipelineName {
		t.Errorf("GetPipelineRuns()[0] = %+v, want project %q and pipeline %q", result.Items[0], survivingProject, pipelineName)
	}
	if len(result.Failures) != 1 || result.Failures[0].Project != failedProject || result.Failures[0].Operation == "" {
		t.Fatalf("GetPipelineRuns() Failures = %+v, want exactly one named entry for %q", result.Failures, failedProject)
	}
	if result.Complete {
		t.Errorf("GetPipelineRuns() Complete = %v, want false", result.Complete)
	}
}

// Regression: a project whose definition listing failed used to be skipped
// silently, so a caller could not tell a failed read from a project with no
// pipelines configured. The failure now reaches the caller by name.
func TestGetPipelineDefinitionsReportsProjectFailure(t *testing.T) {
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

	result, err := client.GetPipelineDefinitions(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPipelineDefinitions() error = %v, want nil", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("GetPipelineDefinitions() returned %d definitions, want 1 surviving definition: %+v", len(result.Items), result.Items)
	}
	if result.Items[0].Project != survivingProject || result.Items[0].Name != pipelineName {
		t.Errorf("GetPipelineDefinitions()[0] = %+v, want project %q and pipeline %q", result.Items[0], survivingProject, pipelineName)
	}
	if len(result.Failures) != 1 || result.Failures[0].Project != failedProject || result.Failures[0].Operation == "" {
		t.Fatalf("GetPipelineDefinitions() Failures = %+v, want exactly one named entry for %q", result.Failures, failedProject)
	}
	if result.Complete {
		t.Errorf("GetPipelineDefinitions() Complete = %v, want false", result.Complete)
	}
}

// definitionsClient builds a two-project client whose definitions call behaves
// per the supplied hook, so the happy-path row-count and auth cases below share
// one fixture shape.
func pipelineTestClient(t *testing.T, projects []string, defsErr func(project string) error) *Client {
	t.Helper()
	return &Client{
		projects:  projects,
		pipelines: []string{"pipeline-one"},
		buildClient: &fakeBuildAPI{
			t: t,
			GetDefinitionsFunc: func(_ context.Context, args build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
				if args.Project != nil {
					if err := defsErr(*args.Project); err != nil {
						return nil, err
					}
				}
				definitionID := 17
				unmatchedID := 18
				return &build.GetDefinitionsResponseValue{Value: []build.BuildDefinitionReference{
					{Id: &definitionID, Name: strPtr("pipeline-one")},
					// A definition that does not match the configured pattern, and
					// one with a nil name: both are data filters, not failures.
					{Id: &unmatchedID, Name: strPtr("unrelated-pipeline")},
					{Id: &unmatchedID},
				}}, nil
			},
			GetBuildsFunc: func(_ context.Context, _ build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
				return &build.GetBuildsResponseValue{}, nil
			},
			GetDefinitionFunc: func(_ context.Context, _ build.GetDefinitionArgs) (*build.BuildDefinition, error) {
				return nil, nil
			},
		},
	}
}

// TestGetPipelineRunsHappyPathRowCountUnchanged is the detector for a data
// filter accidentally converted into a failure record: a converted filter
// changes the row count on a fully successful fetch.
func TestGetPipelineRunsHappyPathRowCountUnchanged(t *testing.T) {
	client := pipelineTestClient(t, []string{"project-one", "project-two"}, func(string) error { return nil })

	result, err := client.GetPipelineRuns(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPipelineRuns() error = %v, want nil", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("GetPipelineRuns() returned %d runs, want 2 — one matching definition per project", len(result.Items))
	}
	if !result.Complete || len(result.Failures) != 0 {
		t.Errorf("GetPipelineRuns() Complete/Failures = %v/%+v, want true/none", result.Complete, result.Failures)
	}
}

func TestGetPipelineDefinitionsHappyPathRowCountUnchanged(t *testing.T) {
	client := pipelineTestClient(t, []string{"project-one", "project-two"}, func(string) error { return nil })

	result, err := client.GetPipelineDefinitions(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPipelineDefinitions() error = %v, want nil", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("GetPipelineDefinitions() returned %d definitions, want 2 — one matching definition per project", len(result.Items))
	}
	if !result.Complete || len(result.Failures) != 0 {
		t.Errorf("GetPipelineDefinitions() Complete/Failures = %v/%+v, want true/none", result.Complete, result.Failures)
	}
}

func TestGetPipelineRunsClassifiesRejectedToken(t *testing.T) {
	client := pipelineTestClient(t, []string{"project-unauthorized"}, func(string) error {
		return wrappedErrorValue(401, "TF400813: The user is not authorized")
	})

	result, err := client.GetPipelineRuns(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPipelineRuns() error = %v, want nil", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("GetPipelineRuns() Items = %+v, want empty", result.Items)
	}
	if len(result.Failures) != 1 || result.Failures[0].Kind != FailureAuth {
		t.Fatalf("GetPipelineRuns() Failures = %+v, want one auth entry", result.Failures)
	}
	if result.Complete {
		t.Errorf("GetPipelineRuns() Complete = %v, want false", result.Complete)
	}
}

func TestGetPipelineDefinitionsClassifiesRejectedToken(t *testing.T) {
	client := pipelineTestClient(t, []string{"project-unauthorized"}, func(string) error {
		return wrappedErrorPointer(403, "TF400813: The user is not authorized")
	})

	result, err := client.GetPipelineDefinitions(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPipelineDefinitions() error = %v, want nil", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("GetPipelineDefinitions() Items = %+v, want empty", result.Items)
	}
	if len(result.Failures) != 1 || result.Failures[0].Kind != FailureAuth {
		t.Fatalf("GetPipelineDefinitions() Failures = %+v, want one auth entry", result.Failures)
	}
	if result.Complete {
		t.Errorf("GetPipelineDefinitions() Complete = %v, want false", result.Complete)
	}
}

// TestFetchTimeoutMarksUnreachedPipelines is D-12's gate for both pipeline reads.
func TestFetchTimeoutMarksUnreachedPipelines(t *testing.T) {
	projects := []string{"project-one", "project-two", "project-three"}

	client := &Client{
		projects:  projects,
		pipelines: []string{"pipeline-one"},
		buildClient: &fakeBuildAPI{
			t: t,
			GetDefinitionsFunc: func(_ context.Context, _ build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
				t.Errorf("GetDefinitions called after the deadline elapsed; want no doomed calls issued")
				return nil, nil
			},
		},
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	runs, err := client.GetPipelineRuns(ctx, nil)
	if err != nil {
		t.Fatalf("GetPipelineRuns() error = %v, want nil", err)
	}
	assertAllUnreached(t, "GetPipelineRuns", runs.Failures, runs.Complete, projects)

	definitions, err := client.GetPipelineDefinitions(ctx, nil)
	if err != nil {
		t.Fatalf("GetPipelineDefinitions() error = %v, want nil", err)
	}
	assertAllUnreached(t, "GetPipelineDefinitions", definitions.Failures, definitions.Complete, projects)
}

// assertAllUnreached is the shared D-12 assertion: every configured project got
// a timeout-kind failure in iteration order and the result is incomplete.
func assertAllUnreached(t *testing.T, name string, failures []Failure, complete bool, projects []string) {
	t.Helper()
	if len(failures) != len(projects) {
		t.Fatalf("%s() Failures length = %d, want %d — one per unreached project", name, len(failures), len(projects))
	}
	for i, failure := range failures {
		if failure.Kind != FailureTimeout {
			t.Errorf("%s() Failures[%d].Kind = %v, want %v", name, i, failure.Kind, FailureTimeout)
		}
		if failure.Project != projects[i] {
			t.Errorf("%s() Failures[%d].Project = %v, want %v", name, i, failure.Project, projects[i])
		}
	}
	if complete {
		t.Errorf("%s() Complete = %v, want false", name, complete)
	}
}
