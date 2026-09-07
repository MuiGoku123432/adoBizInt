package ado

import (
	"testing"
	"time"

	"sentinovo.ai/bizInt/internal/config"
)

// TestNewClientSetsTimeout asserts the connection carries the per-call budget.
//
// It exercises the connection helper rather than NewClient itself: NewClient
// performs six network round trips against Azure DevOps and cannot run as a unit
// test. The helper is the whole of the behaviour under test, because
// azuredevops.NewClient reads connection.Timeout once at construction and caches
// the resulting http.Client on the Connection — assigning Timeout after any
// sub-client is built compiles, vets, passes, and bounds nothing.
func TestNewClientSetsTimeout(t *testing.T) {
	cfg := &config.Config{
		OrgURL:         "https://dev.azure.com/example",
		PAT:            "token",
		RequestTimeout: 45 * time.Second,
	}

	connection := newConnection(cfg)

	if connection == nil {
		t.Fatalf("newConnection() = nil, want a connection")
	}
	if connection.Timeout == nil {
		t.Fatalf("newConnection() Timeout = nil, want the per-call budget set before any sub-client is constructed")
	}
	if *connection.Timeout != cfg.RequestTimeout {
		t.Errorf("newConnection() Timeout = %v, want %v", *connection.Timeout, cfg.RequestTimeout)
	}
}

func TestNewConnectionLeavesTimeoutUnsetForZeroBudget(t *testing.T) {
	// A zero duration on http.Client means no bound at all, which is worse than
	// the SDK default path. Leave Timeout nil rather than installing a zero.
	cfg := &config.Config{
		OrgURL: "https://dev.azure.com/example",
		PAT:    "token",
	}

	connection := newConnection(cfg)

	if connection.Timeout != nil {
		t.Errorf("newConnection() Timeout = %v, want nil for a zero configured budget", *connection.Timeout)
	}
}
