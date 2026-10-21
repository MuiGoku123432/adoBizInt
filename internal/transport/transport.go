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
// later request until the instructed time (D-04). A hold that does not fit the
// hold budget, the time left before the request's deadline less minSendBudget
// and never more than maxHold, is refused at once as rate limited (D-01). It
// paces but never retries, and never reads or replays a request body (Pitfall
// 8): reads retry in the service layer, writes never.
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
	"math"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"sentinovo.ai/bizInt/internal/logging"
)

// ErrNotSent marks a request that provably never left the process, so a write
// refused here can be reported failed rather than unknown (D-02). Only a
// first-hop refusal satisfies it: on a redirect hop the first request already
// reached ADO (Pitfall 4).
var ErrNotSent = errors.New("request not sent")

// ErrRateLimited marks the D-01 fail-fast only: ADO asked for a wait that does
// not fit the hold budget, so the request was refused instead of held. The hold
// budget is the time left before the request's deadline less minSendBudget,
// and never more than maxHold, with or without a deadline. It lets
// classifyFailure label the refusal rate_limit structurally, without reading
// error text. A request held and then cancelled, or refused by the host rule,
// does not match it.
var ErrRateLimited = errors.New("rate limited")

// minSendBudget is the least time a request must still have before its
// deadline when its hold ends. A hold that would leave less is refused as rate
// limited, because sending into a near-expired deadline turns a throttle into a
// timeout read or an unknown write (D-01, D-02). At the 30s default
// request_timeout this honours waits up to 28s, and a per-call budget at or
// below 2s fails every instructed wait fast. Its size is the user's choice
// (2026-10-21), under the 08-CONTEXT discretion for a cap on a single honoured
// Retry-After.
const minSendBudget = 2 * time.Second

// maxHold is the longest single hold the pacer takes. ADO documents a 5-minute
// consumption window and delays of up to 30s (Pitfall 9), so no honest
// instruction needs a longer hold. A longer wait is refused at once as rate
// limited, never held and never sent early, so no header can park the process,
// including on the deadline-less startup context when request_timeout is 0.
const maxHold = 5 * time.Minute

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

	// waits and waitedNanos count the holds the pacer actually took and their
	// total length, for the CLI's end-of-run summary (D-05). Atomic, because
	// concurrent TUI fetches pace through one transport.
	waits       atomic.Int64
	waitedNanos atomic.Int64
}

// New returns a Transport bound to org that hands allowed requests to base.
// org is the lowercase {org} parsed from org_url (config.Config.Org); an empty
// org refuses every request. requestTimeout bounds a single HTTP call, and zero
// keeps its existing meaning of no per-call bound. Either way a single pacer
// hold is bounded by maxHold.
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
// Only https with no userinfo, no explicit port and no opaque part is
// considered. A host that holds any non-ASCII byte is refused before anything
// is folded (CR-01). A dev.azure.com host, or any *.dev.azure.com host such as
// vsrm or vssps, is allowed only when the escaped path the request line carries
// is /{org} or starts with /{org}/, with the org in its own escaped form; that
// holds even when org_url itself is on dev.azure.com, so another organization
// on the same host is refused. That path is checked as sent and refused rather
// than normalised (WR-01): an encoded separator (%2F, %5C) hides a segment
// boundary, an encoded dot (%2E) hides a dot segment, and a path that
// path.Clean would change other than by one trailing slash holds a dot or empty
// segment. {org}.visualstudio.com and {org}.*.visualstudio.com are allowed for
// any path. Host and path compare case-insensitively (Pitfall 7). Everything
// else is refused.
//
// A non-ASCII name after the org is allowed: project, repository and team
// names may be non-ASCII, and their escaped form, such as Caf%C3%A9, is
// canonical ASCII. A non-ASCII org segment is not: \u212Acme goes on the wire
// as /%E2%84%AAcme, which never equals /kcme.
//
// An empty org refuses everything, through the first check: without it the
// root path "/" would equal "/" plus the empty org and admit the root of
// dev.azure.com.
func (t *Transport) Allowed(u *url.URL) bool {
	if t.org == "" {
		return false
	}
	if u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Opaque != "" {
		return false
	}
	host, ok := asciiLower(u.Hostname())
	if !ok {
		return false
	}
	if host == "dev.azure.com" || strings.HasSuffix(host, ".dev.azure.com") {
		raw := u.EscapedPath()
		if !strings.HasPrefix(raw, "/") {
			raw = "/" + raw
		}
		p, ok := asciiLower(raw)
		if !ok || strings.Contains(p, "%2f") || strings.Contains(p, "%5c") || strings.Contains(p, "%2e") {
			return false
		}
		if c := path.Clean(raw); c != raw && c+"/" != raw {
			return false
		}
		org := "/" + strings.ToLower(url.PathEscape(t.org))
		return p == org || strings.HasPrefix(p, org+"/")
	}
	if host == t.org+".visualstudio.com" {
		return true
	}
	return strings.HasPrefix(host, t.org+".") && strings.HasSuffix(host, ".visualstudio.com")
}

// asciiLower lowercases s, or returns "" and false when s holds any non-ASCII
// byte. Go's Unicode case mapping folds U+0130 to i and U+212A to k, while
// net/http dials the IDNA form of the original host, so a folded match is not
// the host that gets dialed (CR-01): acme.v\u0130sualstudio.com folds to
// acme.visualstudio.com but dials acme.xn--visualstudio-n4g.com.
func asciiLower(s string) (string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return "", false
		}
	}
	return strings.ToLower(s), true
}

// RoundTrip refuses a request whose URL the host rule does not allow, before
// the base transport sees it, then holds an allowed request until the pacer's
// notBefore has passed, or refuses it as rate limited when that hold does not
// fit the hold budget, the time left before its deadline less minSendBudget
// and never more than maxHold (D-01), and otherwise passes the request on
// unchanged. Every response's headers are observed before it is returned,
// whatever its status. A redirect hop is any request carrying the response
// that caused it.
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
		logRateLimit(resp.StatusCode, resp.Header)
		t.observe(resp.Header)
	}
	return resp, err
}

// Stats returns how many holds the pacer actually took and their total length,
// for the CLI's end-of-run summary (D-05). A request sent with no pending wait
// adds nothing. The transport itself never writes stderr.
func (t *Transport) Stats() (waits int64, waited time.Duration) {
	return t.waits.Load(), time.Duration(t.waitedNanos.Load())
}

// hold waits until notBefore has passed. It loops because another response
// may move notBefore later while this request sleeps. Each hold that runs to
// its end is counted and logged with its length and the host (D-05).
//
// Before every sleep, including the re-check after one, it decides whether the
// wait fits: a context that has already ended is not sent and wraps its
// context error, and a wait longer than the hold budget is refused at once as
// rate limited (D-01). The hold budget is maxHold, lowered to the time left
// before the request's deadline less minSendBudget when the context has a
// deadline; a wait equal to it is taken. So a request that is held is sent
// with at least minSendBudget left, no single hold exceeds maxHold, and no hold
// outlives the deadline. That last matters beyond honesty: once http.Client's
// own Timeout fires, Client.do replaces whatever this returned with its timeout
// error, which would drop ErrNotSent and turn a never-sent write into unknown.
// The deadline seen here already includes the Client.Timeout, which
// http.Client installs on the request context before calling a custom
// RoundTripper. If the request's context ends during a hold, the request is
// not sent and the error wraps the context error.
func (t *Transport) hold(req *http.Request, hop bool) error {
	ctx := req.Context()
	host := req.URL.Hostname()
	for {
		wait := t.untilNotBefore()
		if wait <= 0 {
			return nil
		}
		if err := ctx.Err(); err != nil {
			closeBody(req)
			return &notSentError{reason: reasonHeld, host: host, org: t.org, hop: hop, cause: err}
		}
		budget := maxHold
		if deadline, ok := ctx.Deadline(); ok {
			budget = min(budget, time.Until(deadline)-minSendBudget)
		}
		if wait > budget {
			budget = max(budget, 0)
			closeBody(req)
			logging.Logger().Warn("Refused to hold a request for ADO rate-limit pacing",
				"asked", wait, "budget", budget, "host", host)
			return &notSentError{reason: reasonRateLimited, host: host, org: t.org, hop: hop, wait: wait, left: budget}
		}
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
			t.waits.Add(1)
			t.waitedNanos.Add(int64(wait))
			logging.Logger().Info("Held a request for ADO rate-limit pacing",
				"wait", wait, "host", host)
		case <-ctx.Done():
			timer.Stop()
			closeBody(req)
			return &notSentError{reason: reasonHeld, host: host, org: t.org, hop: hop, cause: ctx.Err()}
		}
	}
}

// untilNotBefore returns how long the next request must still wait.
func (t *Transport) untilNotBefore() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return time.Until(t.notBefore)
}

// observe reads the server's pacing instructions from a response's headers,
// on any status, and moves notBefore to the instructed time if that is later
// than the current one (D-04). Only Retry-After and X-RateLimit-Delay are
// instructions; X-RateLimit-Remaining, -Limit and -Resource never change
// timing. Instructions combine as a maximum, never a sum: Retry-After and
// X-RateLimit-Delay on one response wait the longer of the two (Pitfall 10),
// and two responses asking for the same wait hold the next request once. An
// unusable instruction is logged and ignored.
func (t *Transport) observe(h http.Header) {
	now := time.Now()
	retry, retryErr := retryAfter(h.Get("Retry-After"), now)
	delay, delayErr := rateLimitDelay(h.Get("X-RateLimit-Delay"))
	for _, err := range []error{retryErr, delayErr} {
		if err != nil {
			logging.Logger().Info("Ignored an unusable ADO pacing header", "reason", err)
		}
	}
	wait := max(retry, delay)
	if wait <= 0 {
		return
	}
	until := now.Add(wait)
	t.mu.Lock()
	defer t.mu.Unlock()
	if until.After(t.notBefore) {
		t.notBefore = until
	}
}

// maxWaitSeconds is the longest wait, in whole seconds, a time.Duration holds.
// A longer instruction would wrap around, so it is ignored as unusable.
const maxWaitSeconds = math.MaxInt64 / int64(time.Second)

// retryAfter parses a Retry-After value: non-negative integer seconds, or an
// HTTP-date in any of the three formats http.ParseTime accepts, measured from
// now. An absent value is no wait. A present value that is negative,
// unparseable, too large or already past is no wait and an error.
func retryAfter(v string, now time.Time) (time.Duration, error) {
	if v == "" {
		return 0, nil
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 || int64(secs) > maxWaitSeconds {
			return 0, fmt.Errorf("Retry-After %d seconds is outside 0 to %d", secs, maxWaitSeconds)
		}
		return time.Duration(secs) * time.Second, nil
	}
	date, err := http.ParseTime(v)
	if err != nil {
		return 0, errors.New("Retry-After is neither seconds nor an HTTP-date")
	}
	wait := date.Sub(now)
	if wait < 0 {
		return 0, errors.New("Retry-After date is in the past")
	}
	return wait, nil
}

// rateLimitDelay parses X-RateLimit-Delay: decimal seconds, kept at nanosecond
// precision, so 0.125 is exactly 125ms. An absent value is no wait. A present
// value that is negative, unparseable, not finite or too large is no wait and
// an error.
func rateLimitDelay(v string) (time.Duration, error) {
	if v == "" {
		return 0, nil
	}
	secs, err := strconv.ParseFloat(v, 64)
	// !(secs >= 0) also rejects NaN, and the upper bound rejects +Inf.
	if err != nil || !(secs >= 0) || secs > float64(maxWaitSeconds) {
		return 0, errors.New("X-RateLimit-Delay is not a usable number of seconds")
	}
	return time.Duration(math.Round(secs * float64(time.Second))), nil
}

// logRateLimit logs one line of a response's rate-limit headers when it carries
// Retry-After, X-RateLimit-Delay or X-RateLimit-Remaining. ADO sends a cost
// header on ordinary responses, and logging that alone would grow the log on
// every request (Pitfall 15). Only the status and header values are logged,
// never the URL, request headers or body; slog quotes the server-controlled
// values (T-08-23).
func logRateLimit(status int, h http.Header) {
	retry, delay, remaining := h.Get("Retry-After"), h.Get("X-RateLimit-Delay"), h.Get("X-RateLimit-Remaining")
	if retry == "" && delay == "" && remaining == "" {
		return
	}
	logging.Logger().Info("ADO rate-limit headers",
		"status", status, "resource", h.Get("X-RateLimit-Resource"),
		"retry_after", retry, "delay", delay, "remaining", remaining, "limit", h.Get("X-RateLimit-Limit"))
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
	// reasonRateLimited: the pacer's wait did not fit the hold budget, the
	// time left before the request's deadline less minSendBudget and never
	// more than maxHold, so it refused the request instead of holding it
	// (D-01).
	reasonRateLimited notSentReason = "rate_limited"
)

// notSentError is the error RoundTrip returns for a request it did not send.
// It deliberately has no Timeout method: url.Error forwards Timeout to the
// wrapped error, and classifyFailure would then read a refusal as a timeout
// (Pitfall 5). A held request instead unwraps to its context error, so a read
// keeps its cancel or timeout classification. The D-01 fail-fast wraps
// nothing, in particular not context.DeadlineExceeded: it matches
// ErrRateLimited instead, so it classifies rate_limit.
type notSentError struct {
	reason notSentReason
	host   string
	org    string
	hop    bool
	cause  error
	// wait is what ADO asked for and left is the longest hold the budget
	// allowed (at most maxHold), floored at zero, for the D-01 fail-fast only.
	wait, left time.Duration
}

func (e *notSentError) Error() string {
	switch e.reason {
	case reasonRateLimited:
		// D-08: the cause, in the Failure.Message an agent reads. The asked
		// wait rounds up and the budget down: a wait is refused only when it
		// exceeds the budget, so the printed pair never reads as one that
		// fits. Ceiling by remainder, because adding a second to a saturated
		// wait would overflow.
		asked := e.wait / time.Second
		if e.wait%time.Second != 0 {
			asked++
		}
		return fmt.Sprintf("rate limited: ADO asked for %ds, %ds left in budget", int64(asked), int64(e.left/time.Second))
	case reasonHeld:
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
// It matches ErrRateLimited for the D-01 fail-fast on any hop.
func (e *notSentError) Is(target error) bool {
	return (target == ErrNotSent && !e.hop) || (target == ErrRateLimited && e.reason == reasonRateLimited)
}
