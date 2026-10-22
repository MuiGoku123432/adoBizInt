package ado

import (
	"context"
	"encoding/json"
	"errors"
	"net"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"

	"sentinovo.ai/bizInt/internal/transport"
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
// From Phase 3 onward these field names (five since Phase 9, TEAM-08) are a machine-readable public
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
	Team      string      `json:"team"`      // the team whose read failed; "" for project-scoped reads (TEAM-08)
	Operation string      `json:"operation"` // short label for the call being made, e.g. "get pull requests"
	Kind      FailureKind `json:"kind"`      // "auth", "timeout", "rate_limit", "other"
	Err       error       `json:"-"`         // the underlying error, preserved for errors.Is / errors.As
}

// MarshalJSON emits the five-key failure record, flattening Err to its message
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
		Team      string      `json:"team"`
		Operation string      `json:"operation"`
		Kind      FailureKind `json:"kind"`
		Err       *string     `json:"err"`
	}{
		Project:   f.Project,
		Team:      f.Team,
		Operation: f.Operation,
		Kind:      f.Kind,
		Err:       msg,
	})
}

// Scope discloses the fixed filter Azure DevOps applies server-side for a read,
// so a caller can tell "outside the scope" from "does not exist" (audit W4,
// D-01).
//
// A key is present only when that filter applies. This deliberately departs
// from TestJSONFieldNames' stable-key-set rule, because a scope key's presence
// IS the fact (D-01's examples are per-verb objects). "recent" means iterations
// starting or finishing after a cutoff 30 days back (GetRecentIterations); a
// project with no such dated iteration is not limited by iteration.
//
// Values are code or SDK constants, never server text, and scope never affects
// Complete. The key names are a published one-way contract (D-01). pr_status is
// used rather than status because dashboard counts carries both a work-item
// filter and a PR filter, and a bare status would be ambiguous there.
type Scope struct {
	ExcludedStates []string `json:"excluded_states,omitempty"`
	Iterations     string   `json:"iterations,omitempty"`
	PRStatus       string   `json:"pr_status,omitempty"`
	ApprovalStatus string   `json:"approval_status,omitempty"`
}

// Result is the envelope every aggregate read returns. Items holds whatever was
// successfully fetched, Failures names every project that did not answer, and
// Complete reports whether the items are the whole picture (TRUST-03).
//
// The json tags use the same snake_case convention as the payload types (D-04),
// so Phase 3 serialises the whole read response under one contract rather than
// wrapping snake_case payloads in Go-cased keys. This holds uniformly for all
// eight reads, including the dashboard's single-element Items slice. Scope is
// never omitted: it is an object when the read applies a fixed server-side
// filter and null when it does not (D-02).
type Result[T any] struct {
	Items    []T       `json:"items"`
	Failures []Failure `json:"failures"`
	Findings []Finding `json:"findings"`
	Complete bool      `json:"complete"`
	Scope    *Scope    `json:"scope"`
}

// FindingKind classifies a finding. The set is closed; a caller may switch on
// it exhaustively. Phase 9 defines only cadence_mismatch (TEAM-05).
type FindingKind string

// FindingCadenceMismatch means configured teams resolved current sprints with
// different dates, so a cross-team rollup would compare unlike periods.
const FindingCadenceMismatch FindingKind = "cadence_mismatch"

// Finding reports something true about the data that a caller should act on,
// as opposed to a Failure, which reports data that could not be read. Findings
// never affect Complete. Detail is code-generated text, never server text, for
// the same reason Failure never carries response detail (T-02-01). Teams holds
// "project/team" names.
type Finding struct {
	Kind    FindingKind `json:"kind"`
	Project string      `json:"project"`
	Teams   []string    `json:"teams"`
	Detail  string      `json:"detail"`
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
		Findings: []Finding{},
		Complete: len(failures) == 0,
	}
}

// appendFailure records a classified failure against a project, team and
// operation. Project-scoped reads pass team "" (TEAM-08). A nil error is not a
// failure and is ignored, so callers may pass an error unconditionally. This is
// the one call-site shape the expansion reads copy.
func appendFailure(failures []Failure, project, team, operation string, err error) []Failure {
	if err == nil {
		return failures
	}
	return append(failures, Failure{
		Project:   project,
		Team:      team,
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

	// D-01, D-08: the transport refused to wait into the request's deadline
	// because ADO asked for a longer wait than the budget had left. That is a
	// throttle, not a timeout: reporting it as timeout hides the real cause and
	// pushes agents to retry harder (PITFALLS Pitfall 9). The fail-fast wraps no
	// context error and has no Timeout method, so this branch is the only one
	// that can match it.
	if errors.Is(err, transport.ErrRateLimited) {
		return FailureRateLimit
	}

	// Cancellation is checked before the timeout branch and deliberately falls
	// through to FailureOther. Phase 3's CLI-04 interrupt handling depends on a
	// user-initiated cancel not being reported as a server timeout.
	if errors.Is(err, context.Canceled) {
		return FailureOther
	}

	// Belt and braces: the deadline may come from context.WithTimeout (the
	// fan-out budget) or from http.Client.Timeout on the client
	// internal/transport builds (the per-call budget). Modern toolchains
	// surface both as context.DeadlineExceeded, but the net.Error check does
	// not depend on that.
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

// wrappedErrorOf extracts the SDK's WrappedError from err.
//
// azuredevops.Client.UnwrapError returns a *WrappedError on two paths
// (v7 client.go:375, :412) and a bare WrappedError value on two others
// (v7 client.go:398, :419). WrappedError.Error() has a value receiver, so both
// forms satisfy error and errors.As will not convert between them — both
// targets must be tried.
func wrappedErrorOf(err error) (azuredevops.WrappedError, bool) {
	var value azuredevops.WrappedError
	if errors.As(err, &value) {
		return value, true
	}

	var pointer *azuredevops.WrappedError
	if errors.As(err, &pointer) && pointer != nil {
		return *pointer, true
	}

	return azuredevops.WrappedError{}, false
}

// statusCodeOf extracts the HTTP status from an SDK error.
func statusCodeOf(err error) (int, bool) {
	wrapped, ok := wrappedErrorOf(err)
	if !ok || wrapped.StatusCode == nil {
		return 0, false
	}
	return *wrapped.StatusCode, true
}

// StatusCodeOf is the exported wrapper over statusCodeOf. The CLI's write exit
// mapper needs the HTTP status to distinguish a 5xx (outcome genuinely
// unresolved on a non-idempotent request) from a 409 (the server answered and
// refused), and duplicating the extraction there would duplicate the
// value-vs-pointer WrappedError subtlety documented above — which is exactly the
// detail a second copy would get wrong.
func StatusCodeOf(err error) (int, bool) {
	return statusCodeOf(err)
}

// ClassifyFailure is the exported wrapper over classifyFailure, for the same
// reason as StatusCodeOf: the CLI's write exit mapper is required to key on
// FailureKind rather than on error text, and that is only possible if the
// structural classifier is reachable from outside this package. Re-implementing
// it in cmd would put the "cancellation must not look like a timeout" rule in
// two places and let the two drift.
func ClassifyFailure(err error) FailureKind {
	return classifyFailure(err)
}
