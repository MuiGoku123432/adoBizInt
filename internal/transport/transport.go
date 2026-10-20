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
// The package imports the standard library and internal/logging only. It must
// never import internal/ado, internal/guard or internal/config: ado imports
// transport, so any of those would cycle. It paces but never retries (pacing
// lands in 08-05; reads retry in the service layer, writes never). It never
// writes stdout or stderr (OUT-03), and it never logs the URL path or query,
// request headers or the body (Pitfall 15).
package transport

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
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
// the base transport sees it, and otherwise passes the request on unchanged.
// A redirect hop is any request carrying the response that caused it.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	hop := req.Response != nil
	if !t.Allowed(req.URL) {
		// The RoundTripper contract requires closing the body on every error path.
		if req.Body != nil {
			req.Body.Close()
		}
		logging.Logger().Warn("Refused to send credentials to a host outside the organization",
			"host", req.URL.Hostname(), "redirect_hop", hop)
		return nil, &notSentError{host: req.URL.Hostname(), org: t.org, hop: hop}
	}
	return t.base.RoundTrip(req)
}

// notSentError is the refusal RoundTrip returns. It deliberately has no Timeout
// method: url.Error forwards Timeout to the wrapped error, and classifyFailure
// would then read a refusal as a timeout (Pitfall 5).
type notSentError struct {
	host string
	org  string
	hop  bool
}

func (e *notSentError) Error() string {
	return fmt.Sprintf("refused to send credentials to host %q: not bound to organization %q", e.host, e.org)
}

// Is matches ErrNotSent on the first hop only. A refused redirect hop followed
// a request that already reached ADO, so it is not "never sent" (Pitfall 4).
func (e *notSentError) Is(target error) bool {
	return target == ErrNotSent && !e.hop
}
