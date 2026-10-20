package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	writeguard "sentinovo.ai/bizInt/internal/guard"
	"sentinovo.ai/bizInt/internal/transport"
)

// TestRateLimitedWriteExitsOne is the write half of SC3. A write mutation that
// ADO answered 429, that the host rule refused, or that the pacer refused under
// D-01 because ADO's wait did not fit the deadline, runs exactly once (the
// guard never retries a write), is failed with no audit record, and maps to the
// write-failure code 1, never 4 (exitNoOp, which means nothing needed
// writing). A rate-limited pre-read is a different path, exit 3, and is not
// what this pins (RESEARCH Pitfall 16). This lives in its own file so none of
// the Phase-7-locked CLI test files is edited.
func TestRateLimitedWriteExitsOne(t *testing.T) {
	tests := []struct {
		name string
		err  func(t *testing.T) error
	}{
		{"429", func(*testing.T) error { return wrappedErrorValue(429, "too many requests") }},
		{"refused host", func(t *testing.T) error {
			resp, err := transport.New("acme", 0, nil).HTTPClient().Get("https://evil.example/x")
			if resp != nil {
				resp.Body.Close()
			}
			return err
		}},
		{"fail-fast", failFastWriteErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutateErr := tt.err(t)
			if mutateErr == nil {
				t.Fatal("fixture returned nil, want the not-sent or 429 error")
			}
			path := filepath.Join(t.TempDir(), "audit.jsonl")
			calls := 0
			op := writeguard.Op{
				Name:   "workitems set-state",
				Target: writeguard.Target{Project: "P", ID: 4821},
				Plan: func(context.Context) ([]writeguard.FieldChange, error) {
					return []writeguard.FieldChange{{Field: "State", From: "Active", To: "Closed"}}, nil
				},
				Mutate: func(context.Context) error { calls++; return mutateErr },
			}

			report := writeguard.Run(context.Background(), op, writeguard.Options{
				Confirm: func([]writeguard.FieldChange) bool { return true },
				Open:    func() (*os.File, error) { return writeguard.OpenAuditSink(path) },
				Actor:   writeguard.ActorAgent,
			})

			if calls != 1 {
				t.Errorf("Mutate ran %d times, want exactly 1 (a write is never retried)", calls)
			}
			if report.Outcome != outcomeFailed {
				t.Errorf("Outcome = %q, want %q", report.Outcome, outcomeFailed)
			}
			if got := writeExitCodeFor(report.Outcome, report.MutateErr); got != exitPartial {
				t.Errorf("writeExitCodeFor(%q, %v) = %d, want exitPartial (%d), never exitNoOp (%d)", report.Outcome, report.MutateErr, got, exitPartial, exitNoOp)
			}
			if data, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(data))) != 0 {
				t.Errorf("audit file holds %q, want no record for a failed write", data)
			}
		})
	}
}

// failFastWriteErr returns the D-01 fail-fast error: a transport whose first
// GET was answered 200 with Retry-After: 60 refuses a second request that has
// only a 1s deadline, at once and without sending it.
func failFastWriteErr(t *testing.T) error {
	t.Helper()
	calls := 0
	base := rtFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls > 1 {
			t.Errorf("base received a second request %s %s; the pacer should have refused it", r.Method, r.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Retry-After": []string{"60"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})
	c := transport.New("acme", 0, base).HTTPClient()
	resp, err := c.Get("https://dev.azure.com/acme/x")
	if err != nil {
		t.Fatalf("priming GET error = %v", err)
	}
	resp.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, "https://dev.azure.com/acme/x", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	resp, err = c.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	return err
}
