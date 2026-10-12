package ado

import (
	"context"
	"errors"
	"reflect"
	"strings"
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

// assertDefinitionFailure is the per-definition counterpart of
// assertAllUnreached: exactly one failure, against the project, of the given
// kind, whose operation names the definition that failed, and an incomplete
// result. The per-definition tests of all three N+1 reads share it.
func assertDefinitionFailure(t *testing.T, name string, failures []Failure, complete bool, project string, kind FailureKind, definition string) {
	t.Helper()
	if len(failures) != 1 {
		t.Fatalf("%s() Failures = %+v, want exactly one entry naming %q", name, failures, definition)
	}
	failure := failures[0]
	if failure.Project != project || failure.Kind != kind || !strings.Contains(failure.Operation, definition) {
		t.Errorf("%s() Failures[0] = {Project:%q Kind:%q Operation:%q}, want project %q, kind %q and an operation naming %q",
			name, failure.Project, failure.Kind, failure.Operation, project, kind, definition)
	}
	if complete {
		t.Errorf("%s() Complete = %v, want false", name, complete)
	}
}

// twoPipelineDefinitions answers GetDefinitions with one healthy and one broken
// matching definition, in that order, for the per-definition failure tests.
func twoPipelineDefinitions(_ context.Context, _ build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
	okID, brokenID := 17, 18
	return &build.GetDefinitionsResponseValue{Value: []build.BuildDefinitionReference{
		{Id: &okID, Name: strPtr("pipeline-ok")},
		{Id: &brokenID, Name: strPtr("pipeline-broken")},
	}}, nil
}

// Regression (gap 1): a failed per-definition build lookup used to append the
// definition as "neverRun" and record nothing, so a rejected token or a timeout
// read as an empty state under complete:true. The definition is now absent from
// Items and named in Failures.
func TestGetPipelineRunsReportsDefinitionFailure(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantKind FailureKind
	}{
		{name: "auth", err: wrappedErrorValue(401, "TF400813: The user is not authorized"), wantKind: FailureAuth},
		{name: "timeout", err: context.DeadlineExceeded, wantKind: FailureTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				projects:  []string{"project-one"},
				pipelines: []string{"pipeline-ok", "pipeline-broken"},
				buildClient: &fakeBuildAPI{
					t:                  t,
					GetDefinitionsFunc: twoPipelineDefinitions,
					GetBuildsFunc: func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
						if (*args.Definitions)[0] == 18 {
							return nil, tt.err
						}
						buildID := 501
						status := build.BuildStatusValues.Completed
						return &build.GetBuildsResponseValue{Value: []build.Build{{Id: &buildID, Status: &status}}}, nil
					},
				},
			}

			result, err := client.GetPipelineRuns(context.Background(), nil)
			if err != nil {
				t.Fatalf("GetPipelineRuns() error = %v, want nil", err)
			}
			if len(result.Items) != 1 || result.Items[0].PipelineName != "pipeline-ok" {
				t.Errorf("GetPipelineRuns() Items = %+v, want only pipeline-ok — a failed lookup must not render as a neverRun row", result.Items)
			}
			assertDefinitionFailure(t, "GetPipelineRuns", result.Failures, result.Complete, "project-one", tt.wantKind, "pipeline-broken")
		})
	}
}

// TestGetPipelineRunsMarksUnreachedDefinitions is D-12 inside the per-definition
// loop: once the fetch deadline expires, the definitions not yet reached are
// recorded as timeouts and no further GetBuilds call is issued.
func TestGetPipelineRunsMarksUnreachedDefinitions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	calls := 0
	client := &Client{
		projects:  []string{"project-one"},
		pipelines: []string{"pipeline-ok", "pipeline-broken"},
		buildClient: &fakeBuildAPI{
			t:                  t,
			GetDefinitionsFunc: twoPipelineDefinitions,
			GetBuildsFunc: func(ctx context.Context, _ build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
				calls++
				if calls > 1 {
					t.Errorf("GetBuilds call %d issued after the fetch deadline expired; want no doomed calls", calls)
					return nil, ctx.Err()
				}
				select {
				case <-ctx.Done():
				case <-time.After(5 * time.Second):
					t.Error("fetch deadline did not expire within 5s")
				}
				// note: same one-build answer as TestGetPipelineRunsReportsDefinitionFailure; two uses stay inline.
				buildID := 501
				status := build.BuildStatusValues.Completed
				return &build.GetBuildsResponseValue{Value: []build.Build{{Id: &buildID, Status: &status}}}, nil
			},
		},
	}

	result, err := client.GetPipelineRuns(ctx, nil)
	if err != nil {
		t.Fatalf("GetPipelineRuns() error = %v, want nil", err)
	}
	if calls != 1 {
		t.Errorf("GetBuilds calls = %d, want 1", calls)
	}
	if len(result.Items) != 1 || result.Items[0].PipelineName != "pipeline-ok" {
		t.Errorf("GetPipelineRuns() Items = %+v, want only pipeline-ok", result.Items)
	}
	assertDefinitionFailure(t, "GetPipelineRuns", result.Failures, result.Complete, "project-one", FailureTimeout, "pipeline-broken")
}

// Regression (gap-1 class): a failed per-definition detail lookup used to be
// dropped behind `err == nil && fullDef != nil`, so the definition came back
// with an empty repository_id/default_branch under complete:true and the
// trigger modal later failed with an unrelated branch-fetch error. The
// definition is now absent from Items and named in Failures.
func TestGetPipelineDefinitionsReportsDetailFailure(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantKind FailureKind
	}{
		{name: "auth", err: wrappedErrorValue(401, "TF400813: The user is not authorized"), wantKind: FailureAuth},
		{name: "timeout", err: context.DeadlineExceeded, wantKind: FailureTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				projects:  []string{"project-one"},
				pipelines: []string{"pipeline-ok", "pipeline-broken"},
				buildClient: &fakeBuildAPI{
					t:                  t,
					GetDefinitionsFunc: twoPipelineDefinitions,
					GetDefinitionFunc: func(_ context.Context, args build.GetDefinitionArgs) (*build.BuildDefinition, error) {
						if *args.DefinitionId == 18 {
							return nil, tt.err
						}
						return &build.BuildDefinition{Repository: &build.BuildRepository{
							Id:            strPtr("repo-guid-17"),
							DefaultBranch: strPtr("refs/heads/main"),
						}}, nil
					},
				},
			}

			result, err := client.GetPipelineDefinitions(context.Background(), nil)
			if err != nil {
				t.Fatalf("GetPipelineDefinitions() error = %v, want nil", err)
			}
			if len(result.Items) != 1 || result.Items[0].Name != "pipeline-ok" {
				t.Fatalf("GetPipelineDefinitions() Items = %+v, want only pipeline-ok — a failed detail lookup must not yield a row with a blank repository_id", result.Items)
			}
			if got := result.Items[0]; got.RepositoryID != "repo-guid-17" || got.DefaultBranch != "main" {
				t.Errorf("GetPipelineDefinitions()[0] RepositoryID/DefaultBranch = %q/%q, want %q/%q", got.RepositoryID, got.DefaultBranch, "repo-guid-17", "main")
			}
			assertDefinitionFailure(t, "GetPipelineDefinitions", result.Failures, result.Complete, "project-one", tt.wantKind, "pipeline-broken")
		})
	}
}

// TestGetPipelineDefinitionsMarksUnreachedDefinitions is D-12 inside the
// per-definition detail loop: once the fetch deadline expires, the definitions
// not yet reached are recorded as timeouts and no further GetDefinition call is
// issued.
func TestGetPipelineDefinitionsMarksUnreachedDefinitions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	calls := 0
	client := &Client{
		projects:  []string{"project-one"},
		pipelines: []string{"pipeline-ok", "pipeline-broken"},
		buildClient: &fakeBuildAPI{
			t:                  t,
			GetDefinitionsFunc: twoPipelineDefinitions,
			GetDefinitionFunc: func(ctx context.Context, _ build.GetDefinitionArgs) (*build.BuildDefinition, error) {
				calls++
				if calls > 1 {
					t.Errorf("GetDefinition call %d issued after the fetch deadline expired; want no doomed calls", calls)
					return nil, ctx.Err()
				}
				select {
				case <-ctx.Done():
				case <-time.After(5 * time.Second):
					t.Error("fetch deadline did not expire within 5s")
				}
				// note: same detail answer as TestGetPipelineDefinitionsReportsDetailFailure; two uses stay inline.
				return &build.BuildDefinition{Repository: &build.BuildRepository{
					Id:            strPtr("repo-guid-17"),
					DefaultBranch: strPtr("refs/heads/main"),
				}}, nil
			},
		},
	}

	result, err := client.GetPipelineDefinitions(ctx, nil)
	if err != nil {
		t.Fatalf("GetPipelineDefinitions() error = %v, want nil", err)
	}
	if calls != 1 {
		t.Errorf("GetDefinition calls = %d, want 1", calls)
	}
	if len(result.Items) != 1 || result.Items[0].Name != "pipeline-ok" {
		t.Errorf("GetPipelineDefinitions() Items = %+v, want only pipeline-ok", result.Items)
	}
	assertDefinitionFailure(t, "GetPipelineDefinitions", result.Failures, result.Complete, "project-one", FailureTimeout, "pipeline-broken")
}
