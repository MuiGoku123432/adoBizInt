// Package transport builds the one HTTP client the Azure DevOps SDK uses, and
// binds the PAT that client carries to the configured organization (D-09, D-10,
// D-11). It is the only production package that constructs that client:
// internal/ado never imports net/http and obtains the client from here.
//
// Every request the SDK sends passes through Transport.RoundTrip, including
// each redirect hop, so a host outside the organization is refused before the
// Authorization header leaves the process. ado.NewClient applies the same rule,
// Allowed, to every discovered resource-area location at startup.
//
// It also paces: a response that tells the client to slow down holds every
// later request until the instructed time (D-04). It paces but never retries,
// and never reads or replays a request body (Pitfall 8): reads retry in the
// service layer, writes never.
//
// The package imports the standard library and internal/logging only. It must
// never import internal/ado, internal/guard or internal/config: ado imports
// transport, so any of those would cycle. It never writes stdout or stderr
// (OUT-03), and it never logs the URL path or query, request headers or the
// body (Pitfall 15).
package transport

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"sentinovo.ai/bizInt/internal/logging"
)

// ErrNotSent marks a request that provably never left the process, so a write
// refused here can be reported failed rather than unknown (D-02). Only a
// first-hop refusal satisfies it: on a redirect hop the first request already
// reached ADO (Pitfall 4).
var ErrNotSent = errors.New("request not sent")

// Transport is the http.RoundTripper every SDK sub-client sends through. The
// zero value is not usable; build one with New.
type Transport struct {
	org            string
	requestTimeout time.Duration
	base           http.RoundTripper

	// mu guards notBefore, the earliest time the next request may be sent.
	// There is one per process, not one per resource: ADO budgets per
	// identity, so an instruction from any response holds every request.
	mu        sync.Mutex
	notBefore time.Time
}

// New returns a Transport bound to org that hands allowed requests to base.
// org is the lowercase {org} parsed from org_url (config.Config.Org); an empty
// org refuses every request. requestTimeout bounds a single HTTP call, and zero
// keeps its existing meaning of no per-call bound.
func New(org string, requestTimeout time.Duration, base http.RoundTripper) *Transport {
	return &Transport{org: strings.ToLower(org), requestTimeout: requestTimeout, base: base}
}

// HTTPClient returns the client to inject with azuredevops.WithHTTPClient. The
// per-call budget lives here because WithHTTPClient ignores
// connection.Timeout; http.Client installs Timeout as a request-context
// deadline before RoundTrip, and a zero Timeout installs none.
func (t *Transport) HTTPClient() *http.Client {
	return &http.Client{Transport: t, Timeout: t.requestTimeout}
}

// Allowed reports whether u may receive the PAT (D-09). It is the one host
// rule: RoundTrip applies it to every request and redirect hop (D-10b), and
// ado.NewClient applies it to every discovered location at startup (D-10a).
//
// Only https with no userinfo and no explicit port is considered. A
// dev.azure.com host, or any *.dev.azure.com host such as vsrm or vssps, is
// allowed only when the cleaned path is /{org} or starts with /{org}/; that
// holds even when org_url itself is on dev.azure.com, so another organization
// on the same host is refused. {org}.visualstudio.com and
// {org}.*.visualstudio.com are allowed for any path. Host and path compare
// case-insensitively (Pitfall 7). Everything else is refused.
//
// An empty org refuses everything: path.Clean yields "/" for an empty path, and
// "/"+org would also be "/", which would admit the root of dev.azure.com.
func (t *Transport) Allowed(u *url.URL) bool {
	if t.org == "" {
		return false
	}
	if u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "dev.azure.com" || strings.HasSuffix(host, ".dev.azure.com") {
		p := strings.ToLower(path.Clean("/" + u.Path))
		return p == "/"+t.org || strings.HasPrefix(p, "/"+t.org+"/")
	}
	if host == t.org+".visualstudio.com" {
		return true
	}
	return strings.HasPrefix(host, t.org+".") && strings.HasSuffix(host, ".visualstudio.com")
}

// RoundTrip refuses a request whose URL the host rule does not allow, before
// the base transport sees it, then holds an allowed request until the pacer's
// notBefore has passed, and otherwise passes the request on unchanged. Every
// response's headers are observed before it is returned, whatever its status.
// A redirect hop is any request carrying the response that caused it.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	hop := req.Response != nil
	if !t.Allowed(req.URL) {
		closeBody(req)
		logging.Logger().Warn("Refused to send credentials to a host outside the organization",
			"host", req.URL.Hostname(), "redirect_hop", hop)
		return nil, &notSentError{reason: reasonHost, host: req.URL.Hostname(), org: t.org, hop: hop}
	}
	if err := t.hold(req, hop); err != nil {
		return nil, err
	}
	resp, err := t.base.RoundTrip(req)
	if resp != nil {
		t.observe(resp.Header)
	}
	return resp, err
}

// hold waits until notBefore has passed. It loops because another response
// may move notBefore later while this request sleeps. If the request's context
// ends first, the request is not sent and the error wraps the context error.
func (t *Transport) hold(req *http.Request, hop bool) error {
	for {
		wait := t.untilNotBefore()
		if wait <= 0 {
			return nil
		}
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-req.Context().Done():
			timer.Stop()
			closeBody(req)
			return &notSentError{reason: reasonHeld, host: req.URL.Hostname(), org: t.org, hop: hop, cause: req.Context().Err()}
		}
	}
}

// untilNotBefore returns how long the next request must still wait.
func (t *Transport) untilNotBefore() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return time.Until(t.notBefore)
}

// observe reads the server's pacing instruction from a response's headers and
// moves notBefore to the instructed time if that is later than the current
// one. Instructions are a maximum, never added: two responses asking for the
// same wait hold the next request once (D-04).
func (t *Transport) observe(h http.Header) {
	secs, err := strconv.Atoi(h.Get("Retry-After"))
	if err != nil || secs < 0 {
		return
	}
	until := time.Now().Add(time.Duration(secs) * time.Second)
	t.mu.Lock()
	defer t.mu.Unlock()
	if until.After(t.notBefore) {
		t.notBefore = until
	}
}

// closeBody closes the request body, which the RoundTripper contract requires
// on every error path.
func closeBody(req *http.Request) {
	if req.Body != nil {
		req.Body.Close()
	}
}

// notSentReason says why a request never left the process.
type notSentReason string

const (
	// reasonHost: the host rule refused the URL (D-10b).
	reasonHost notSentReason = "host"
	// reasonHeld: the request's context ended while the pacer held it.
	reasonHeld notSentReason = "held"
)

// notSentError is the error RoundTrip returns for a request it did not send.
// It deliberately has no Timeout method: url.Error forwards Timeout to the
// wrapped error, and classifyFailure would then read a refusal as a timeout
// (Pitfall 5). A held request instead unwraps to its context error, so a read
// keeps its cancel or timeout classification.
type notSentError struct {
	reason notSentReason
	host   string
	org    string
	hop    bool
	cause  error
}

func (e *notSentError) Error() string {
	if e.reason == reasonHeld {
		return fmt.Sprintf("request to host %q not sent: its context ended while held by ADO rate-limit pacing: %v", e.host, e.cause)
	}
	return fmt.Sprintf("refused to send credentials to host %q: not bound to organization %q", e.host, e.org)
}

// Unwrap returns the context error of a held request, and nil otherwise.
func (e *notSentError) Unwrap() error {
	return e.cause
}

// Is matches ErrNotSent on the first hop only. A refused redirect hop followed
// a request that already reached ADO, so it is not "never sent" (Pitfall 4).
func (e *notSentError) Is(target error) bool {
	return target == ErrNotSent && !e.hop
}
