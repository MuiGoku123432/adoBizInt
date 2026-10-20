package ado

import (
	"strings"
	"testing"
	"time"

	"sentinovo.ai/bizInt/internal/config"
	"sentinovo.ai/bizInt/internal/transport"
)

// resourceAreasRequest runs NewClient with budget over the fake base from
// v7_test.go and returns the ResourceAreas GET the base saw, with the wall
// times just before NewClient was called and just after it returned.
//
// NewClient is unit-testable over a fake base RoundTripper, so these tests
// drive it directly. The budget moved from connection.Timeout to the injected
// client: WithHTTPClient ignores connection.Timeout, and http.Client installs
// its Timeout as a request-context deadline before calling RoundTrip. The
// startup context NewClient uses has no deadline of its own, so any deadline
// the base sees on that request is the budget.
func resourceAreasRequest(t *testing.T, budget time.Duration) (req v7Request, start, end time.Time) {
	t.Helper()
	orgURL := uniqueOrgURL(t)
	var seen []v7Request
	base := newClientBase(t, usedAreas(orgURL), &seen)
	cfg := &config.Config{OrgURL: orgURL, PAT: "token", RequestTimeout: budget}

	start = time.Now()
	if _, err := NewClient(cfg, transport.New(cfg.Org(), cfg.RequestTimeout, base)); err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	end = time.Now()

	for _, r := range seen {
		if strings.HasSuffix(strings.ToLower(r.path), "/_apis/resourceareas") {
			return r, start, end
		}
	}
	t.Fatalf("base saw no ResourceAreas request (all requests: %+v)", seen)
	return v7Request{}, start, end
}

// TestNewClientSetsTimeout asserts the per-call budget reaches the wire. The
// deadline must fall between start+budget and end+budget: http.Client derives
// it from the moment the request is sent, which lies inside the NewClient call.
func TestNewClientSetsTimeout(t *testing.T) {
	const budget = 45 * time.Second
	req, start, end := resourceAreasRequest(t, budget)

	if !req.hasDeadline {
		t.Fatalf("ResourceAreas request carried no deadline, want the %v per-call budget", budget)
	}
	if req.deadline.Before(start.Add(budget)) || req.deadline.After(end.Add(budget)) {
		t.Errorf("ResourceAreas deadline = %v after the call started, want %v", req.deadline.Sub(start), budget)
	}
}

// TestNewClientZeroTimeoutInstallsNoDeadline keeps a zero budget meaning no
// per-call bound at all, as it did when the budget lived on the connection.
func TestNewClientZeroTimeoutInstallsNoDeadline(t *testing.T) {
	req, start, _ := resourceAreasRequest(t, 0)

	if req.hasDeadline {
		t.Errorf("ResourceAreas request carried a deadline %v after the call started, want none for a zero budget", req.deadline.Sub(start))
	}
}
