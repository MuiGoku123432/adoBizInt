package ado

import (
	"context"
	"encoding/json"
	"errors"
	"net"

	"github.com/microsoft/azure-devops-go-api/azuredevops"
)

// This file declares the one result shape every aggregate read in this package
// returns (D-01). Eight reads need an identical "items plus what failed plus
// whether this is the whole picture" contract, which is well past the rule of
// three, so a single generic envelope is the reuse the standard asks for rather
// than the speculative generality it forbids. It takes the same CONVENTIONS
// rule 5 carve-out that justified the SEAM-01 interfaces in interfaces.go: the
// abstraction exists because the current task requires it, not in anticipation
// of a future one.

// FailureKind classifies why a per-project read failed. The set is closed (D-07);
// a caller may switch on it exhaustively.
type FailureKind string

const (
	// FailureAuth means the credential was rejected — the read cannot succeed
	// until the operator acts.
	FailureAuth FailureKind = "auth"
	// FailureTimeout means a deadline elapsed before the server answered.
	FailureTimeout FailureKind = "timeout"
	// FailureRateLimit means the server asked the caller to slow down.
	FailureRateLimit FailureKind = "rate_limit"
	// FailureOther covers everything else, including transport errors and any
	// server status this classifier does not recognise.
	FailureOther FailureKind = "other"
)

// Failure records one per-project read failure so a caller can tell a broken call
// from an empty result.
//
// Security constraint (threat T-02-01): this struct must never gain a field
// holding a request URL, a request header, an authorization value, or the PAT.
// Failures are logged and rendered into a terminal; anything stored here is
// assumed to reach both. Project, operation and kind are the whole vocabulary,
// with the underlying error carried for the caller to inspect via errors.Is.
//
// From Phase 3 onward these four field names are a machine-readable public
// contract. Adding a fifth field REQUIRES updating the exact-key-set assertion
// in json_test.go in the same change; that test exists to make a silent
// addition impossible.
//
// Err is tagged json:"-" and re-emitted as a plain string by MarshalJSON below.
// Letting encoding/json marshal the interface directly would serialise the
// dynamic value's exported fields — an azuredevops.WrappedError can carry
// server-controlled response detail — which is precisely the structural leak
// T-02-01 forbids. The message text itself is emitted untruncated: the
// terminal-rendering truncation from 02-01 is a display concern, and a
// machine-readable failure record is more useful at full fidelity. Keeping that
// text out of parseable output when it should be is Phase 3's OUT-03
// stdout/stderr split, not this struct's job.
type Failure struct {
	Project   string      `json:"project"`   // the configured project whose read failed
	Operation string      `json:"operation"` // short label for the call being made, e.g. "get pull requests"
	Kind      FailureKind `json:"kind"`      // "auth", "timeout", "rate_limit", "other"
	Err       error       `json:"-"`         // the underlying error, preserved for errors.Is / errors.As
}

// MarshalJSON emits the four-key failure record, flattening Err to its message
// string (or null) so no error implementation's internal fields can reach the
// output. See the T-02-01 note on Failure.
func (f Failure) MarshalJSON() ([]byte, error) {
	var msg *string
	if f.Err != nil {
		s := f.Err.Error()
		msg = &s
	}
	return json.Marshal(struct {
		Project   string      `json:"project"`
		Operation string      `json:"operation"`
		Kind      FailureKind `json:"kind"`
		Err       *string     `json:"err"`
	}{
		Project:   f.Project,
		Operation: f.Operation,
		Kind:      f.Kind,
		Err:       msg,
	})
}

// Result is the envelope every aggregate read returns. Items holds whatever was
// successfully fetched, Failures names every project that did not answer, and
// Complete reports whether the items are the whole picture (TRUST-03).
//
// The json tags use the same snake_case convention as the payload types (D-04),
// so Phase 3 serialises the whole read response under one contract rather than
// wrapping snake_case payloads in Go-cased keys. This holds uniformly for all
// eight reads, including the dashboard's single-element Items slice.
type Result[T any] struct {
	Items    []T       `json:"items"`
	Failures []Failure `json:"failures"`
	Complete bool      `json:"complete"`
}

// newResult derives Complete from the failure slice so no call site has to
// remember to set it. Complete is true only when nothing failed.
//
// Both slices are pre-allocated when the caller passes nil (D-P3) so they
// marshal as [] rather than null. This is not cosmetic: failures is nil on
// every successful read, so "failures":null was the happy-path output, and an
// agent evaluating `.failures | length` errored on the ordinary success case.
func newResult[T any](items []T, failures []Failure) Result[T] {
	if items == nil {
		items = []T{}
	}
	if failures == nil {
		failures = []Failure{}
	}
	return Result[T]{
		Items:    items,
		Failures: failures,
		Complete: len(failures) == 0,
	}
}

// appendFailure records a classified failure against a project and operation.
// A nil error is not a failure and is ignored, so callers may pass an error
// unconditionally. This is the one call-site shape the expansion reads copy.
func appendFailure(failures []Failure, project, operation string, err error) []Failure {
	if err == nil {
		return failures
	}
	return append(failures, Failure{
		Project:   project,
		Operation: operation,
		Kind:      classifyFailure(err),
		Err:       err,
	})
}

// classifyFailure maps an error to its FailureKind.
//
// It classifies structurally, never by matching the error text (D-06):
// WrappedError.Error() returns the server-controlled, localisable message and
// can be an HTML fragment, so string matching would be both fragile and a
// spoofing surface.
func classifyFailure(err error) FailureKind {
	if err == nil {
		return ""
	}

	// Cancellation is checked before the timeout branch and deliberately falls
	// through to FailureOther. Phase 3's CLI-04 interrupt handling depends on a
	// user-initiated cancel not being reported as a server timeout.
	if errors.Is(err, context.Canceled) {
		return FailureOther
	}

	// Belt and braces: the deadline may come from context.WithTimeout (the
	// fan-out budget) or from http.Client.Timeout (the per-call budget via
	// connection.Timeout). Modern toolchains surface both as
	// context.DeadlineExceeded, but the net.Error check does not depend on that.
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return FailureTimeout
	}

	if status, ok := statusCodeOf(err); ok {
		switch status {
		case 401, 403:
			return FailureAuth
		case 429:
			return FailureRateLimit
		}
	}

	return FailureOther
}

// statusCodeOf extracts the HTTP status from an SDK error.
//
// azuredevops.Client.UnwrapError returns a *WrappedError on two paths
// (client.go:366, :403) and a bare WrappedError value on two others
// (client.go:389, :411). WrappedError.Error() has a value receiver, so both
// forms satisfy error and errors.As will not convert between them — both
// targets must be tried.
func statusCodeOf(err error) (int, bool) {
	var value azuredevops.WrappedError
	if errors.As(err, &value) && value.StatusCode != nil {
		return *value.StatusCode, true
	}

	var pointer *azuredevops.WrappedError
	if errors.As(err, &pointer) && pointer != nil && pointer.StatusCode != nil {
		return *pointer.StatusCode, true
	}

	return 0, false
}
