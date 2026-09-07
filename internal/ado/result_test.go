package ado

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
)

// fakeNetTimeout satisfies net.Error with Timeout() true without wrapping
// context.DeadlineExceeded, proving the net.Error half of the belt-and-braces
// timeout detection is load-bearing on its own.
type fakeNetTimeout struct{}

func (fakeNetTimeout) Error() string   { return "i/o timeout" }
func (fakeNetTimeout) Timeout() bool   { return true }
func (fakeNetTimeout) Temporary() bool { return false }

var _ net.Error = fakeNetTimeout{}

func TestClassifyFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want FailureKind
	}{
		{
			name: "nil error is not a failure",
			err:  nil,
			want: "",
		},
		{
			// azuredevops client.go:389 and :411 return a bare value.
			name: "WrappedError value carrying 401 is auth",
			err:  wrappedErrorValue(401, "unauthorized"),
			want: FailureAuth,
		},
		{
			// azuredevops client.go:366 and :403 return a pointer.
			name: "WrappedError pointer carrying 401 is auth",
			err:  wrappedErrorPointer(401, "unauthorized"),
			want: FailureAuth,
		},
		{
			name: "status 403 is auth",
			err:  wrappedErrorValue(403, "forbidden"),
			want: FailureAuth,
		},
		{
			name: "status 429 is rate_limit",
			err:  wrappedErrorPointer(429, "too many requests"),
			want: FailureRateLimit,
		},
		{
			name: "status 500 is other",
			err:  wrappedErrorValue(500, "server error"),
			want: FailureOther,
		},
		{
			name: "status 404 is other",
			err:  wrappedErrorPointer(404, "not found"),
			want: FailureOther,
		},
		{
			name: "deadline exceeded is timeout",
			err:  fmt.Errorf("get pull requests: %w", context.DeadlineExceeded),
			want: FailureTimeout,
		},
		{
			name: "net.Error reporting Timeout is timeout",
			err:  fmt.Errorf("dial: %w", fakeNetTimeout{}),
			want: FailureTimeout,
		},
		{
			// Phase 3 CLI-04 interrupt handling depends on cancellation
			// staying out of the timeout bucket.
			name: "context canceled is other, not timeout",
			err:  fmt.Errorf("get pull requests: %w", context.Canceled),
			want: FailureOther,
		},
		{
			name: "WrappedError with nil StatusCode is other, not auth",
			err:  wrappedErrorNoStatus("something went wrong"),
			want: FailureOther,
		},
		{
			name: "wrapped pointer WrappedError still reaches auth through the wrapper",
			err:  fmt.Errorf("get pull requests for project: %w", wrappedErrorPointer(401, "unauthorized")),
			want: FailureAuth,
		},
		{
			name: "wrapped value WrappedError still reaches auth through the wrapper",
			err:  fmt.Errorf("get pull requests for project: %w", wrappedErrorValue(403, "forbidden")),
			want: FailureAuth,
		},
		{
			name: "plain error is other",
			err:  errors.New("boom"),
			want: FailureOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyFailure(tt.err); got != tt.want {
				t.Errorf("classifyFailure() kind = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyFailureIsSingleValued(t *testing.T) {
	// No status code may map to two kinds: classification must be deterministic
	// across both SDK error shapes for the same code.
	for _, code := range []int{401, 403, 429, 400, 404, 500, 503} {
		value := classifyFailure(wrappedErrorValue(code, "x"))
		pointer := classifyFailure(wrappedErrorPointer(code, "x"))
		if value != pointer {
			t.Errorf("classifyFailure(status %d) value form = %v, pointer form = %v, want identical", code, value, pointer)
		}
	}
}

func TestResultCompleteness(t *testing.T) {
	t.Run("zero failures reports complete", func(t *testing.T) {
		got := newResult([]int{1, 2}, nil)
		if !got.Complete {
			t.Errorf("newResult() Complete = %v, want true", got.Complete)
		}
		if len(got.Items) != 2 {
			t.Errorf("newResult() Items length = %d, want 2", len(got.Items))
		}
		if len(got.Failures) != 0 {
			t.Errorf("newResult() Failures length = %d, want 0", len(got.Failures))
		}
	})

	t.Run("one failure reports incomplete", func(t *testing.T) {
		failures := appendFailure(nil, "project-a", "get pull requests", errors.New("boom"))
		got := newResult([]int{1}, failures)
		if got.Complete {
			t.Errorf("newResult() Complete = %v, want false", got.Complete)
		}
		if len(got.Failures) != 1 {
			t.Fatalf("newResult() Failures length = %d, want 1", len(got.Failures))
		}
	})

	t.Run("empty items with no failures is still complete", func(t *testing.T) {
		got := newResult([]int(nil), nil)
		if !got.Complete {
			t.Errorf("newResult() Complete = %v, want true for an empty but successful read", got.Complete)
		}
	})
}

func TestAppendFailureRecordsProjectOperationAndKind(t *testing.T) {
	err := wrappedErrorValue(401, "unauthorized")
	failures := appendFailure(nil, "project-a", "get pull requests", err)
	if len(failures) != 1 {
		t.Fatalf("appendFailure() length = %d, want 1", len(failures))
	}
	got := failures[0]
	if got.Project != "project-a" {
		t.Errorf("appendFailure() Project = %v, want project-a", got.Project)
	}
	if got.Operation != "get pull requests" {
		t.Errorf("appendFailure() Operation = %v, want %q", got.Operation, "get pull requests")
	}
	if got.Kind != FailureAuth {
		t.Errorf("appendFailure() Kind = %v, want %v", got.Kind, FailureAuth)
	}
	if !errors.Is(got.Err, err) {
		t.Errorf("appendFailure() Err = %v, want the underlying error preserved", got.Err)
	}
}

func TestAppendFailurePreservesInsertionOrder(t *testing.T) {
	var failures []Failure
	failures = appendFailure(failures, "project-a", "op", wrappedErrorValue(500, "a"))
	failures = appendFailure(failures, "project-b", "op", wrappedErrorValue(401, "b"))
	failures = appendFailure(failures, "project-c", "op", wrappedErrorValue(500, "c"))

	wantProjects := []string{"project-a", "project-b", "project-c"}
	for i, want := range wantProjects {
		if failures[i].Project != want {
			t.Errorf("appendFailure() failures[%d].Project = %v, want %v", i, failures[i].Project, want)
		}
	}
	if failures[0].Kind != FailureOther || failures[2].Kind != FailureOther {
		t.Errorf("appendFailure() equal-kind failures lost insertion order: %+v", failures)
	}
}

func TestAppendFailureIgnoresNilError(t *testing.T) {
	if got := appendFailure(nil, "project-a", "op", nil); got != nil {
		t.Errorf("appendFailure(nil error) = %+v, want nil — a nil error is not a failure", got)
	}
}
