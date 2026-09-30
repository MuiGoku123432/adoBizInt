package ado

import (
	"context"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
)

// TestGetPipelineRunsSkipsNilID pins the MED4 fix in GetPipelineRuns: a matching
// definition the server returned without an Id is skipped silently (D-14), and
// the well-formed definition beside it still produces its row.
func TestGetPipelineRunsSkipsNilID(t *testing.T) {
	definitionID := 17
	client := &Client{
		projects:  []string{"project-one"},
		pipelines: []string{"pipeline-one"},
		buildClient: &fakeBuildAPI{
			t: t,
			GetDefinitionsFunc: func(_ context.Context, _ build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
				return &build.GetDefinitionsResponseValue{Value: []build.BuildDefinitionReference{
					{Id: &definitionID, Name: strPtr("pipeline-one")},
					{Name: strPtr("pipeline-one")},
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

	result, err := client.GetPipelineRuns(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPipelineRuns() error = %v, want nil", err)
	}
	if len(result.Items) != 1 || result.Items[0].PipelineID != definitionID {
		t.Fatalf("GetPipelineRuns() Items = %+v, want exactly one row with PipelineID %d", result.Items, definitionID)
	}
	if !result.Complete || len(result.Failures) != 0 {
		t.Errorf("GetPipelineRuns() Complete/Failures = %v/%+v, want true/none — a nil Id is malformed data, not a failed call", result.Complete, result.Failures)
	}
}
