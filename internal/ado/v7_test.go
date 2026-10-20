package ado

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
)

// Every other test in this package drives the narrow interfaces through
// SDK-typed fakes, so it builds WrappedError from the same SDK module result.go
// imports and cannot notice a half-migration (PLAT-02, PITFALLS Pitfall 13).
// The helpers below put a real v7 ClientImpl on top of a fake base
// RoundTripper instead, so the error under test is the one the real SDK
// unwraps from a real HTTP response. Test files may import net/http:
// TestNoRawHTTPInADO parses non-test files only.

// rtFunc adapts a function to http.RoundTripper, so a test can answer the SDK's
// requests without a server.
//
// note: internal/transport's test file holds a twin of this type. Two copies in
// two packages is below the rule of three, and Go cannot share _test.go helpers
// across packages.
type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// jsonResp builds the response a fake base RoundTripper returns. ContentLength
// is set to len(body) because v7 UnwrapError branches on it: zero takes the
// pointer path (client.go:373-379), anything else decodes the body. header is
// optional key/value pairs, for headers such as Retry-After.
func jsonResp(r *http.Request, status int, body string, header ...string) *http.Response {
	h := http.Header{"Content-Type": []string{"application/json"}}
	for i := 0; i+1 < len(header); i += 2 {
		h.Add(header[i], header[i+1])
	}
	return &http.Response{
		Status:        http.StatusText(status),
		StatusCode:    status,
		Header:        h,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       r,
	}
}

// v7Location is one ApiResourceLocation entry of the OPTIONS {base}/_apis
// answer. The SDK resolves every call's route and api-version from this list
// before it sends the data request.
type v7Location struct {
	id, area, resource, route string
	resourceVersion           int
}

// v7Locations lists every location a real-SDK test in this package calls. The
// routes use the server's own {area}/{resource} tokens, which v7
// transformRouteTemplate fills from the entry's area and resourceName; the
// tests assert the resulting request paths. Later plans append the locations
// their tests call.
var v7Locations = []v7Location{
	{"e81700f7-3be2-46de-8624-2eb35882fcaa", "Location", "ResourceAreas", "_apis/{resource}/{areaId}", 1},
	{"225f7195-f9c7-4d14-ab28-a83f7ff77e1f", "git", "repositories", "{project}/_apis/{area}/{resource}/{repositoryId}", 1},
	{"72c7ddf8-2cdc-4f60-90cd-ab71c14a399b", "wit", "workItems", "{project}/_apis/{area}/{resource}/{id}", 3},
	{"00d9565f-ed9c-4a06-9a50-00e7896ccab4", "Location", "ConnectionData", "_apis/{resource}", 1},
}

// locationsFixture renders v7Locations as the {"count":N,"value":[...]} body
// the SDK expects from OPTIONS {base}/_apis. maxVersion 7.1 and a
// resourceVersion at least the requested one make v7 version negotiation send
// each call's 7.1-preview.N unchanged.
func locationsFixture() string {
	type entry struct {
		ID              string `json:"id"`
		Area            string `json:"area"`
		ResourceName    string `json:"resourceName"`
		RouteTemplate   string `json:"routeTemplate"`
		MinVersion      string `json:"minVersion"`
		MaxVersion      string `json:"maxVersion"`
		ReleasedVersion string `json:"releasedVersion"`
		ResourceVersion int    `json:"resourceVersion"`
	}
	value := make([]entry, 0, len(v7Locations))
	for _, l := range v7Locations {
		value = append(value, entry{l.id, l.area, l.resource, l.route, "1.0", "7.1", "7.0", l.resourceVersion})
	}
	body, err := json.Marshal(struct {
		Count int     `json:"count"`
		Value []entry `json:"value"`
	}{len(value), value})
	if err != nil {
		panic(err)
	}
	return string(body)
}

// withLocations answers every OPTIONS request with locationsFixture and hands
// everything else to next. OPTIONS is never passed on, so a test that counts
// requests counts data requests only, whether or not the SDK's location cache
// already held this base URL.
func withLocations(next rtFunc) rtFunc {
	return func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodOptions {
			return jsonResp(r, http.StatusOK, locationsFixture()), nil
		}
		return next(r)
	}
}

// uniqueOrgURL returns an organization URL no other test uses. The SDK's
// apiResourceLocationCache is a process global keyed by base URL (Pitfall 6),
// so two tests sharing an org would share each other's location lists.
func uniqueOrgURL(t *testing.T) string {
	t.Helper()
	var org strings.Builder
	for _, r := range strings.ToLower(t.Name()) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			org.WriteRune(r)
		}
	}
	return "https://dev.azure.com/" + org.String()
}

// tf400813 is the body ADO returns for a rejected PAT on a data call.
const tf400813 = `{"$id":"1","message":"TF400813: The user is not authorized to access this resource.","typeKey":"UnauthorizedRequestException","errorCode":0,"eventId":3000}`

// v7AuthCases are the two shapes v7 UnwrapError gives a 401: a JSON body
// decodes to a bare WrappedError value, an empty body returns a pointer.
var v7AuthCases = []struct {
	name        string
	body        string
	wantPointer bool
}{
	{"json body (value form)", tf400813, false},
	{"empty body (pointer form)", "", true},
}

// assertV7AuthFailure checks that err classifies as auth and that it carries
// the WrappedError shape the case names, so a test cannot pass on one shape
// only.
func assertV7AuthFailure(t *testing.T, err error, wantPointer bool) {
	t.Helper()
	if err == nil {
		t.Fatal("real v7 call returned nil error, want the 401")
	}
	var value azuredevops.WrappedError
	var pointer *azuredevops.WrappedError
	isValue, isPointer := errors.As(err, &value), errors.As(err, &pointer)
	if isPointer != wantPointer || isValue == wantPointer {
		t.Fatalf("error shape: value=%v pointer=%v, want pointer=%v (%T)", isValue, isPointer, wantPointer, err)
	}
	if got := classifyFailure(err); got != FailureAuth {
		t.Fatalf("classifyFailure(real v7 401) = %q, want %q", got, FailureAuth)
	}
}

// v7HTTPOptions injects base as the transport of the SDK client under test.
func v7HTTPOptions(base http.RoundTripper) azuredevops.ClientOptionFunc {
	return azuredevops.WithHTTPClient(&http.Client{Transport: base})
}

// TestV7ReadAuthFailureClassifiesAuth proves a 401 from a real v7 git client on
// a GET read classifies as auth in both WrappedError shapes (PLAT-02, SC2).
func TestV7ReadAuthFailureClassifiesAuth(t *testing.T) {
	for _, tc := range v7AuthCases {
		t.Run(tc.name, func(t *testing.T) {
			var seen []string
			base := withLocations(func(r *http.Request) (*http.Response, error) {
				seen = append(seen, r.Method+" "+r.URL.Path)
				return jsonResp(r, http.StatusUnauthorized, tc.body), nil
			})
			conn := azuredevops.NewPatConnection(uniqueOrgURL(t), "fixture-pat")
			c := &git.ClientImpl{Client: *azuredevops.NewClientWithOptions(conn, conn.BaseUrl, v7HTTPOptions(base))}

			project := "p"
			_, err := c.GetRepositories(context.Background(), git.GetRepositoriesArgs{Project: &project})

			want := "GET " + strings.TrimPrefix(conn.BaseUrl, "https://dev.azure.com") + "/p/_apis/git/repositories"
			if len(seen) != 1 || seen[0] != want {
				t.Fatalf("data requests = %q, want [%q]", seen, want)
			}
			assertV7AuthFailure(t, err, tc.wantPointer)
		})
	}
}

// TestV7WriteAuthFailureClassifiesAuth proves a 401 from a real v7
// workitemtracking client on a PATCH write classifies as auth in both
// WrappedError shapes (PLAT-02, SC2).
func TestV7WriteAuthFailureClassifiesAuth(t *testing.T) {
	for _, tc := range v7AuthCases {
		t.Run(tc.name, func(t *testing.T) {
			var seen []string
			base := withLocations(func(r *http.Request) (*http.Response, error) {
				seen = append(seen, r.Method+" "+r.URL.Path)
				return jsonResp(r, http.StatusUnauthorized, tc.body), nil
			})
			conn := azuredevops.NewPatConnection(uniqueOrgURL(t), "fixture-pat")
			c := &workitemtracking.ClientImpl{Client: *azuredevops.NewClientWithOptions(conn, conn.BaseUrl, v7HTTPOptions(base))}

			project, id := "p", 42
			op, path := webapi.OperationValues.Add, "/fields/System.State"
			doc := []webapi.JsonPatchOperation{{Op: &op, Path: &path, Value: "Closed"}}
			_, err := c.UpdateWorkItem(context.Background(), workitemtracking.UpdateWorkItemArgs{Document: &doc, Id: &id, Project: &project})

			want := "PATCH " + strings.TrimPrefix(conn.BaseUrl, "https://dev.azure.com") + "/p/_apis/wit/workItems/42"
			if len(seen) != 1 || seen[0] != want {
				t.Fatalf("data requests = %q, want [%q]", seen, want)
			}
			assertV7AuthFailure(t, err, tc.wantPointer)
		})
	}
}
