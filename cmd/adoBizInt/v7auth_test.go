package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"

	"sentinovo.ai/bizInt/internal/ado"
)

// rtFunc adapts a function to http.RoundTripper, so a test can answer a real
// SDK client's requests without a server.
//
// note: internal/ado/v7_test.go holds the original. Go cannot share _test.go
// helpers across packages, so this package keeps a twin; 08-06's
// cmd/adoBizInt/ratelimit_test.go reuses this one.
type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// v7AuthLocations is the OPTIONS {base}/_apis answer for the two calls below:
// git repositories (GET) and work item update (PATCH). The id, area, route and
// version fields match the internal/ado/v7_test.go fixture.
const v7AuthLocations = `{"count":2,"value":[` +
	`{"id":"225f7195-f9c7-4d14-ab28-a83f7ff77e1f","area":"git","resourceName":"repositories","routeTemplate":"{project}/_apis/{area}/{resource}/{repositoryId}","minVersion":"1.0","maxVersion":"7.1","releasedVersion":"7.0","resourceVersion":1},` +
	`{"id":"72c7ddf8-2cdc-4f60-90cd-ab71c14a399b","area":"wit","resourceName":"workItems","routeTemplate":"{project}/_apis/{area}/{resource}/{id}","minVersion":"1.0","maxVersion":"7.1","releasedVersion":"7.0","resourceVersion":3}]}`

// v7AuthBody is the body ADO returns for a rejected PAT on a data call.
const v7AuthBody = `{"$id":"1","message":"TF400813: The user is not authorized to access this resource.","typeKey":"UnauthorizedRequestException","errorCode":0,"eventId":3000}`

// TestV7AuthFailureExitsAuth is the SC2 exit-code proof (08-CONTEXT SC2
// clarification, user, 2026-10-19). SC2 is proven on the read and write paths,
// where a 401 reaches exitCodeFor and writeExitCodeFor: a real v7 ClientImpl
// over a fake transport returns the 401, in the JSON-body (value) and
// empty-body (pointer) shapes, and both mappers must yield exitAuth. A fully
// rejected PAT that fails during startup resource-area discovery still exits 3,
// unchanged from b5 by user decision.
func TestV7AuthFailureExitsAuth(t *testing.T) {
	project, id := "p", 42
	op, path := webapi.OperationValues.Add, "/fields/System.State"
	doc := []webapi.JsonPatchOperation{{Op: &op, Path: &path, Value: "Closed"}}

	tests := []struct {
		name  string
		body  string
		route string // the data request after OPTIONS, relative to the org path
		call  func(azuredevops.Client) error
	}{
		{"read json body", v7AuthBody, "GET /p/_apis/git/repositories", func(c azuredevops.Client) error {
			_, err := (&git.ClientImpl{Client: c}).GetRepositories(context.Background(), git.GetRepositoriesArgs{Project: &project})
			return err
		}},
		{"read empty body", "", "GET /p/_apis/git/repositories", func(c azuredevops.Client) error {
			_, err := (&git.ClientImpl{Client: c}).GetRepositories(context.Background(), git.GetRepositoriesArgs{Project: &project})
			return err
		}},
		{"write json body", v7AuthBody, "PATCH /p/_apis/wit/workItems/42", func(c azuredevops.Client) error {
			_, err := (&workitemtracking.ClientImpl{Client: c}).UpdateWorkItem(context.Background(), workitemtracking.UpdateWorkItemArgs{Document: &doc, Id: &id, Project: &project})
			return err
		}},
		{"write empty body", "", "PATCH /p/_apis/wit/workItems/42", func(c azuredevops.Client) error {
			_, err := (&workitemtracking.ClientImpl{Client: c}).UpdateWorkItem(context.Background(), workitemtracking.UpdateWorkItemArgs{Document: &doc, Id: &id, Project: &project})
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A per-subtest org: the SDK's location cache is a process global
			// keyed by base URL (RESEARCH Pitfall 6).
			var org strings.Builder
			for _, r := range strings.ToLower(t.Name()) {
				if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
					org.WriteRune(r)
				}
			}

			var seen []string
			base := rtFunc(func(r *http.Request) (*http.Response, error) {
				seen = append(seen, r.Method+" "+r.URL.Path)
				status, body := http.StatusUnauthorized, tt.body
				if r.Method == http.MethodOptions {
					status, body = http.StatusOK, v7AuthLocations
				}
				return &http.Response{
					Status:        http.StatusText(status),
					StatusCode:    status,
					Header:        http.Header{"Content-Type": []string{"application/json"}},
					Body:          io.NopCloser(strings.NewReader(body)),
					ContentLength: int64(len(body)), // 0 takes v7 UnwrapError's pointer path
					Request:       r,
				}, nil
			})
			conn := azuredevops.NewPatConnection("https://dev.azure.com/"+org.String(), "fixture-pat")
			client := azuredevops.NewClientWithOptions(conn, conn.BaseUrl, azuredevops.WithHTTPClient(&http.Client{Transport: base}))

			err := tt.call(*client)
			if err == nil {
				t.Fatal("real v7 call returned nil error, want the 401")
			}

			orgPath := "/" + org.String()
			want := []string{"OPTIONS " + orgPath + "/_apis", strings.Replace(tt.route, " ", " "+orgPath, 1)}
			if len(seen) != 2 || seen[0] != want[0] || seen[1] != want[1] {
				t.Fatalf("requests = %q, want %q (the 401 must come from the data route, not discovery)", seen, want)
			}

			failures := []ado.Failure{{Project: "p", Operation: "smoke", Kind: ado.ClassifyFailure(err), Err: err}}
			if got := exitCodeFor(failures, false); got != exitAuth {
				t.Errorf("exitCodeFor(real v7 401) = %d, want exitAuth (%d)", got, exitAuth)
			}
			if got := writeExitCodeFor(outcomeFailed, err); got != exitAuth {
				t.Errorf("writeExitCodeFor(outcomeFailed, real v7 401) = %d, want exitAuth (%d)", got, exitAuth)
			}
		})
	}
}
