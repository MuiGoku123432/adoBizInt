package main

import (
	"testing"

	"sentinovo.ai/bizInt/internal/ado"
)

// TestExitCodeFor pins the process exit contract (D-06) and its precedence rule
// (D-07). The mixed auth+timeout row is the load-bearing one: it is what proves
// a co-occurring transient failure cannot mask a credential rejection.
func TestExitCodeFor(t *testing.T) {
	auth := ado.Failure{Project: "alpha", Operation: "get pull requests", Kind: ado.FailureAuth}
	timeout := ado.Failure{Project: "beta", Operation: "get pull requests", Kind: ado.FailureTimeout}
	rateLimit := ado.Failure{Project: "gamma", Operation: "get pull requests", Kind: ado.FailureRateLimit}

	tests := []struct {
		name     string
		failures []ado.Failure
		complete bool
		want     int
	}{
		{"clean read", nil, true, exitOK},
		{"empty failure slice is still clean", []ado.Failure{}, true, exitOK},
		{"timeouts only, incomplete", []ado.Failure{timeout, timeout}, false, exitPartial},
		{"rate limits only, incomplete", []ado.Failure{rateLimit}, false, exitPartial},
		{"single auth failure", []ado.Failure{auth}, false, exitAuth},
		{"auth among three timeouts wins", []ado.Failure{timeout, timeout, auth}, false, exitAuth},
		{"auth first among timeouts wins", []ado.Failure{auth, timeout, timeout}, false, exitAuth},
		{"complete with an auth failure still maps by the auth scan", []ado.Failure{auth}, true, exitAuth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCodeFor(tt.failures, tt.complete); got != tt.want {
				t.Errorf("exitCodeFor(%d failures, complete=%v) = %d, want %d",
					len(tt.failures), tt.complete, got, tt.want)
			}
		})
	}
}
