package ado

import (
	"context"
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
type Failure struct {
	Project   string      // the configured project whose read failed
	Operation string      // short label for the call being made, e.g. "get pull requests"
	Kind      FailureKind // "auth", "timeout", "rate_limit", "other"
	Err       error       // the underlying error, preserved for errors.Is / errors.As
}

// Result is the envelope every aggregate read returns. Items holds whatever was
// successfully fetched, Failures names every project that did not answer, and
// Complete reports whether the items are the whole picture (TRUST-03).
type Result[T any] struct {
	Items    []T
	Failures []Failure
	Complete bool
}

// newResult derives Complete from the failure slice so no call site has to
// remember to set it. Complete is true only when nothing failed.
func newResult[T any](items []T, failures []Failure) Result[T] {
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
