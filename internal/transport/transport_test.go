package transport

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// rtFunc adapts a function to http.RoundTripper, so a test can stand in for the
// base transport without a server.
//
// note: internal/ado/v7_test.go holds a twin of this type. Two copies in two
// packages is below the rule of three, and Go cannot share _test.go helpers
// across packages.
type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// mustNotSend is a base RoundTripper that fails the test if any request reaches
// it. http.Client calls RoundTrip on the caller's goroutine, so Fatalf is safe.
func mustNotSend(t *testing.T) rtFunc {
	return func(r *http.Request) (*http.Response, error) {
		t.Fatalf("base RoundTripper called for %s %s; the request should have been refused first", r.Method, r.URL)
		return nil, nil
	}
}

// emptyResp is a response with an empty body.
func emptyResp(r *http.Request, status int, header ...string) *http.Response {
	h := http.Header{}
	for i := 0; i+1 < len(header); i += 2 {
		h.Add(header[i], header[i+1])
	}
	return &http.Response{
		Status:     http.StatusText(status),
		StatusCode: status,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    r,
	}
}

// basicAuth builds the header value the SDK sends for a PAT, without importing
// the SDK: Basic base64(":" + pat).
func basicAuth(pat string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+pat))
}

// TestAllowed pins the D-09 org binding row by row: https only, no userinfo or
// port, dev.azure.com and *.dev.azure.com only under /{org} on the escaped
// path, with non-canonical paths refused, {org}.visualstudio.com and
// {org}.*.visualstudio.com on any path, host and path case-insensitive, and
// every look-alike, traversal and cross-org form refused. A non-ASCII host,
// including an IDN homograph that folds onto an allowed host, is refused
// (CR-01); non-ASCII names after the org stay allowed.
// A failing row is a defect in Allowed, never a row to weaken.
func TestAllowed(t *testing.T) {
	tests := []struct {
		name string
		org  string
		url  string
		want bool
	}{
		{name: "dev.azure.com org root", org: "acme", url: "https://dev.azure.com/acme", want: true},
		{name: "dev.azure.com org root with slash", org: "acme", url: "https://dev.azure.com/acme/", want: true},
		{name: "dev.azure.com org api path", org: "acme", url: "https://dev.azure.com/acme/_apis/git", want: true},
		{name: "upper-case host and org", org: "acme", url: "https://DEV.AZURE.COM/ACME/x", want: true},
		{name: "vsrm subdomain under org", org: "acme", url: "https://vsrm.dev.azure.com/acme/_apis", want: true},
		{name: "vssps subdomain org root", org: "acme", url: "https://vssps.dev.azure.com/acme", want: true},
		{name: "legacy org host any path", org: "acme", url: "https://acme.visualstudio.com/any/path", want: true},
		{name: "legacy org service host", org: "acme", url: "https://acme.vsrm.visualstudio.com/x", want: true},

		{name: "http scheme", org: "acme", url: "http://dev.azure.com/acme", want: false},
		{name: "other org on dev.azure.com", org: "acme", url: "https://dev.azure.com/other/x", want: false},
		{name: "org name prefix of another org", org: "acme", url: "https://dev.azure.com/acmeextra/x", want: false},
		{name: "path traversal to another org", org: "acme", url: "https://dev.azure.com/acme/../other/x", want: false},
		{name: "dev.azure.com root", org: "acme", url: "https://dev.azure.com/", want: false},
		{name: "explicit port", org: "acme", url: "https://dev.azure.com:443/acme", want: false},
		{name: "userinfo", org: "acme", url: "https://user@dev.azure.com/acme", want: false},
		{name: "look-alike suffix host", org: "acme", url: "https://dev.azure.com.evil.com/acme", want: false},
		{name: "look-alike label without dot boundary", org: "acme", url: "https://evildev.azure.com/acme", want: false},
		{name: "trailing-dot host", org: "acme", url: "https://dev.azure.com./acme", want: false},
		{name: "other org legacy host", org: "acme", url: "https://other.visualstudio.com", want: false},
		{name: "legacy org host with foreign suffix", org: "acme", url: "https://acme.visualstudio.com.evil.com", want: false},
		{name: "legacy look-alike without dot boundary", org: "acme", url: "https://evilacme.visualstudio.com", want: false},
		{name: "bare visualstudio.com", org: "acme", url: "https://visualstudio.com", want: false},
		{name: "foreign host with org path", org: "acme", url: "https://example.com/acme", want: false},

		// 08-02: an empty org fails closed, or "/"+org would admit the root.
		{name: "empty org refuses dev.azure.com root", org: "", url: "https://dev.azure.com/", want: false},

		// 08-10 CR-01: Unicode case folding maps U+0130 to i and U+212A to k,
		// while net/http dials the IDNA form of the original host, so each of
		// these would match an allowed host after folding yet dial a different,
		// registrable domain. A non-ASCII host is refused before any folding.
		{name: "homograph host percent-encoded dotted capital I", org: "acme", url: "https://acme.v%C4%B0sualstudio.com/x", want: false},
		{name: "homograph service host dotted capital I", org: "acme", url: "https://acme.evil.v\u0130sualstudio.com/x", want: false},
		{name: "homograph org label Kelvin sign", org: "kcme", url: "https://\u212Acme.visualstudio.com/x", want: false},
		// Cyrillic a does not fold to ASCII, so this was already refused; it
		// stays pinned because no decoded-path refusal exists.
		{name: "Cyrillic look-alike org segment", org: "acme", url: "https://dev.azure.com/\u0430cme/x", want: false},
		// Non-ASCII names after the org are allowed (user decision 2026-10-21,
		// "Allow non-ASCII names"): the SDK sends them percent-encoded.
		{name: "non-ASCII project after the org", org: "acme", url: "https://dev.azure.com/acme/Caf%C3%A9/_apis/wit/workitems/42", want: true},
		{name: "non-ASCII project on the legacy org host", org: "acme", url: "https://acme.visualstudio.com/Caf%C3%A9/_apis", want: true},

		// 08-10 WR-01: the org binding is decided on the escaped path the
		// request line carries. Encoded separators, encoded dots, encoded
		// letters, dot segments and empty segments are refused, never resolved.
		{name: "encoded traversal from another org", org: "acme", url: "https://dev.azure.com/other%2F..%2Facme/_apis", want: false},
		{name: "dot-segment traversal from another org", org: "acme", url: "https://dev.azure.com/other/../acme/_apis", want: false},
		{name: "encoded dot segment after the org", org: "acme", url: "https://dev.azure.com/acme/%2e%2e/other/_apis", want: false},
		{name: "dot dot with encoded slash after the org", org: "acme", url: "https://dev.azure.com/acme/..%2Fother/_apis", want: false},
		{name: "encoded backslash segment", org: "acme", url: "https://vsrm.dev.azure.com/acme/%5C/x", want: false},
		{name: "empty segment after the org", org: "acme", url: "https://dev.azure.com/acme//_apis", want: false},
		{name: "dot segment after the org", org: "acme", url: "https://dev.azure.com/acme/./_apis", want: false},
		{name: "encoded letter in the org segment", org: "acme", url: "https://dev.azure.com/%61cme/_apis", want: false},
		// CR-01 path variant: a Kelvin-sign org segment folds to kcme after
		// decoding, but goes on the wire as /%E2%84%AAcme, which is not /kcme.
		{name: "Kelvin sign org segment", org: "kcme", url: "https://dev.azure.com/\u212Acme/x", want: false},
		{name: "percent-encoded Kelvin sign org segment", org: "kcme", url: "https://dev.azure.com/%E2%84%AAcme/x", want: false},

		{name: "encoded space in a project segment", org: "acme", url: "https://dev.azure.com/acme/My%20Project/_apis/wit/workitems/42", want: true},
		{name: "api path with trailing slash", org: "acme", url: "https://dev.azure.com/acme/_apis/", want: true},
		{name: "upper-case org on vsrm", org: "acme", url: "https://vsrm.dev.azure.com/ACME/_apis/release/releases", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("url.Parse(%q) error = %v", tt.url, err)
			}
			if got := New(tt.org, 0, nil).Allowed(u); got != tt.want {
				t.Errorf("New(%q).Allowed(%q) = %v, want %v", tt.org, tt.url, got, tt.want)
			}
		})
	}

	// An opaque URL cannot come from parsing an https string, so it is built
	// directly: the request line would carry Opaque, not Path.
	t.Run("opaque URL", func(t *testing.T) {
		u := &url.URL{Scheme: "https", Host: "dev.azure.com", Path: "/acme/_apis", Opaque: "/other/_apis"}
		if New("acme", 0, nil).Allowed(u) {
			t.Errorf("Allowed(%#v) = true, want false: the request line would carry %q", u, u.RequestURI())
		}
	})
}

// closeRecorder is a request body that records Close.
type closeRecorder struct {
	io.Reader
	closed bool
}

func (c *closeRecorder) Close() error {
	c.closed = true
	return nil
}

// TestRefuseNeverReachesBase pins D-10b on a first hop: a request to a host
// outside the organization is refused before the base transport sees it, the
// body is closed (http.Client marks the body closed after a send error and does
// not close it itself, so only RoundTrip can have closed it), and the error is
// the typed not-sent refusal naming the host (D-02).
func TestRefuseNeverReachesBase(t *testing.T) {
	tr := New("acme", 0, mustNotSend(t))
	body := &closeRecorder{Reader: strings.NewReader(`{"op":"add"}`)}
	req, err := http.NewRequest(http.MethodPost, "https://evil.example/x", body)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", basicAuth("refuse-fixture-pat"))

	resp, err := tr.HTTPClient().Do(req)

	if resp != nil {
		t.Errorf("Do() returned a response, want nil")
	}
	if err == nil {
		t.Fatal("Do() error = nil, want the refusal")
	}
	if !errors.Is(err, ErrNotSent) {
		t.Errorf("errors.Is(%v, ErrNotSent) = false, want true on a first hop", err)
	}
	var ns *notSentError
	if !errors.As(err, &ns) {
		t.Fatalf("Do() error = %v (%T), want it to wrap *notSentError", err, err)
	}
	if !strings.Contains(ns.Error(), "evil.example") {
		t.Errorf("refusal %q does not name the refused host evil.example", ns.Error())
	}
	if !body.closed {
		t.Errorf("request body was not closed on refusal (RoundTripper contract)")
	}
}

// TestRefuseHomographNeverReachesBase pins CR-01 on a first hop: an IDN
// homograph of an allowed host, which Go's case folding would map onto
// acme.visualstudio.com while net/http dials acme.xn--visualstudio-n4g.com, is
// refused before the base transport sees it, with the typed not-sent refusal and
// the body closed.
func TestRefuseHomographNeverReachesBase(t *testing.T) {
	tr := New("acme", 0, mustNotSend(t))
	body := &closeRecorder{Reader: strings.NewReader(`{"op":"add"}`)}
	req, err := http.NewRequest(http.MethodPost, "https://acme.v%C4%B0sualstudio.com/x", body)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", basicAuth("homograph-fixture-pat"))

	resp, err := tr.HTTPClient().Do(req)

	if resp != nil {
		t.Errorf("Do() returned a response, want nil")
	}
	if err == nil {
		t.Fatal("Do() error = nil, want the refusal")
	}
	if !errors.Is(err, ErrNotSent) {
		t.Errorf("errors.Is(%v, ErrNotSent) = false, want true on a first hop", err)
	}
	var ns *notSentError
	if !errors.As(err, &ns) {
		t.Fatalf("Do() error = %v (%T), want it to wrap *notSentError", err, err)
	}
	if !body.closed {
		t.Errorf("request body was not closed on refusal (RoundTripper contract)")
	}
}

// TestRefuseRedirectHopIsNotNotSent pins D-10b on redirect hops and Pitfall 4.
// The first hop reached ADO, so a refused hop must not read as never sent: a
// write that got this far is unknown, not failed. The second row is the case
// the per-hop check exists for: Go forwards Authorization on a redirect to the
// same host, so a cross-org redirect on dev.azure.com would carry the PAT. The
// third row is CR-01: an IDN homograph host must be refused by the transport
// itself, not left to net/http's cross-domain header stripping.
func TestRefuseRedirectHopIsNotNotSent(t *testing.T) {
	tests := []struct {
		name     string
		first    string
		location string
		wantHost string
	}{
		{
			name:     "redirect to a foreign host",
			first:    "https://dev.azure.com/acme/_apis/wit/workitems",
			location: "https://evil.example/y",
			wantHost: "evil.example",
		},
		{
			name:     "redirect to another org on the same host",
			first:    "https://dev.azure.com/acme/_apis/wit/workitems",
			location: "https://dev.azure.com/other/_apis/wit/workitems",
			wantHost: "dev.azure.com",
		},
		{
			name:     "redirect to an IDN homograph of the legacy org host",
			first:    "https://dev.azure.com/acme/_apis/wit/workitems",
			location: "https://acme.v%C4%B0sualstudio.com/y",
			wantHost: "acme.v\u0130sualstudio.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			base := rtFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.String() != tt.first {
					t.Errorf("base received %s, want only the first hop %s", r.URL, tt.first)
				}
				return emptyResp(r, http.StatusTemporaryRedirect, "Location", tt.location), nil
			})
			tr := New("acme", 0, base)
			// A *strings.Reader body gives the request GetBody, so the client
			// follows the 307 with the body, as it would for a write.
			req, err := http.NewRequest(http.MethodPost, tt.first, strings.NewReader(`{"op":"add"}`))
			if err != nil {
				t.Fatalf("NewRequest() error = %v", err)
			}
			req.Header.Set("Authorization", basicAuth("redirect-fixture-pat"))

			_, err = tr.HTTPClient().Do(req)

			if calls != 1 {
				t.Errorf("base called %d times, want exactly 1 (the first hop)", calls)
			}
			if err == nil {
				t.Fatal("Do() error = nil, want the redirect hop refused")
			}
			if errors.Is(err, ErrNotSent) {
				t.Errorf("errors.Is(%v, ErrNotSent) = true, want false: the first hop already reached ADO", err)
			}
			var ns *notSentError
			if !errors.As(err, &ns) {
				t.Fatalf("Do() error = %v (%T), want it to wrap *notSentError", err, err)
			}
			if !ns.hop {
				t.Errorf("refusal hop = false, want true")
			}
			if !strings.Contains(ns.Error(), tt.wantHost) {
				t.Errorf("refusal %q does not name the refused host %s", ns.Error(), tt.wantHost)
			}
		})
	}
}

// TestRefusalNeverLogsPAT pins T-08-10: the refusal log line and the refusal
// error carry the host and org only, never the Authorization header. It drives
// one refused and one allowed request carrying the PAT and searches the log and
// every error for the raw, Basic-header and base64 forms.
//
// Do not call logging.Init here: a successful Init points logging.Logger() at a
// file instead of slog.Default(), and the capture would silently go empty
// (cmd/adoBizInt/patleak_test.go). The host=evil.example check at the end is
// what catches that.
func TestRefusalNeverLogsPAT(t *testing.T) {
	const patFixture = "transport-pat-fixture-do-not-disclose-9c41d7"

	var buf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	// slog.SetDefault also redirects the stdlib log package, so restoring it
	// matters for every later test in the binary.
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	base := rtFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() != "dev.azure.com" {
			t.Errorf("base received %s, want only the allowed request", r.URL)
		}
		return emptyResp(r, http.StatusOK), nil
	})
	client := New("acme", 0, base).HTTPClient()

	var errs []string
	for _, target := range []string{"https://evil.example/x", "https://dev.azure.com/acme/_apis/x"} {
		req, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			t.Fatalf("NewRequest(%q) error = %v", target, err)
		}
		req.Header.Set("Authorization", basicAuth(patFixture))
		resp, err := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		if err != nil {
			errs = append(errs, err.Error())
			var ns *notSentError
			if errors.As(err, &ns) {
				errs = append(errs, ns.Error())
			}
		}
	}
	if len(errs) == 0 {
		t.Fatal("no request was refused, so the error checks below are vacuous")
	}

	logged := buf.Bytes()
	for _, needle := range []struct{ form, value string }{
		{"raw", patFixture},
		{"basic header", basicAuth(patFixture)},
		{"base64", base64.StdEncoding.EncodeToString([]byte(":" + patFixture))},
	} {
		if bytes.Contains(logged, []byte(needle.value)) {
			t.Errorf("transport log output contains the %s form of the PAT", needle.form)
		}
		for _, e := range errs {
			if strings.Contains(e, needle.value) {
				t.Errorf("error %q contains the %s form of the PAT", e, needle.form)
			}
		}
	}

	// The refusal must have logged into the capture, or the absence checks
	// above pass for the wrong reason.
	if !bytes.Contains(logged, []byte("host=evil.example")) {
		t.Errorf("capture saw nothing: no refusal line with host=evil.example was logged (got %q)", logged)
	}
}

// TestTransportNeverPrints pins OUT-03 for the transport: it never writes
// stdout or stderr and never exits, because the CLI's stdout is one parsed
// document and the TUI owns the terminal. It is an AST walk, so neither a
// comment nor a string literal can satisfy or defeat it.
func TestTransportNeverPrints(t *testing.T) {
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob package files: %v", err)
	}
	parsedTransport := false
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if name == "transport.go" {
			parsedTransport = true
		}
		for _, finding := range forbiddenOutput(fset, file) {
			t.Errorf("%s", finding)
		}
	}
	if !parsedTransport {
		t.Fatalf("transport.go was not parsed, so the check is vacuous (files: %v)", names)
	}

	t.Run("tripwire", func(t *testing.T) {
		src := `package snippet

import (
	"fmt"
	stdlog "log"
	"os"
)

func bad() {
	os.Exit(1)
	_ = os.Stdout
	_ = os.Stderr
	fmt.Print("x")
	fmt.Printf("x")
	fmt.Println("x")
	stdlog.Fatal("x")
	stdlog.Fatalf("x")
	stdlog.Panicln("x")
	print("x")
	println("x")
}
`
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "snippet.go", src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse snippet: %v", err)
		}
		if got := forbiddenOutput(fset, file); len(got) != 11 {
			t.Errorf("checker found %d forms, want one per forbidden form (11): %v", len(got), got)
		}
	})
}

// forbiddenOutput reports every os.Exit, os.Stdout, os.Stderr, fmt.Print*,
// log.Fatal* and log.Panic* selector, and every print/println builtin call, in
// file. Package selectors are resolved through the file's imports, so an alias
// cannot hide one.
//
// note: copied from internal/guard/structural_test.go, with os.Stderr added
// because the transport must not write stderr either.
func forbiddenOutput(fset *token.FileSet, file *ast.File) []string {
	pkgOf := map[string]string{}
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		local := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			local = imp.Name.Name
		}
		pkgOf[local] = path
	}

	var findings []string
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			x, ok := n.X.(*ast.Ident)
			if !ok {
				return true
			}
			sel := n.Sel.Name
			forbidden := false
			switch pkgOf[x.Name] {
			case "os":
				forbidden = sel == "Exit" || sel == "Stdout" || sel == "Stderr"
			case "fmt":
				forbidden = sel == "Print" || sel == "Printf" || sel == "Println"
			case "log":
				forbidden = strings.HasPrefix(sel, "Fatal") || strings.HasPrefix(sel, "Panic")
			}
			if forbidden {
				findings = append(findings, fset.Position(n.Pos()).String()+": "+x.Name+"."+sel)
			}
		case *ast.CallExpr:
			if id, ok := n.Fun.(*ast.Ident); ok && (id.Name == "print" || id.Name == "println") {
				findings = append(findings, fset.Position(n.Pos()).String()+": "+id.Name)
			}
		}
		return true
	})
	return findings
}

// TestHTTPClientTimeout pins where the per-call budget lives: on the client the
// SDK is given, because WithHTTPClient ignores connection.Timeout. A zero budget
// keeps its meaning of no per-call bound.
func TestHTTPClientTimeout(t *testing.T) {
	base := rtFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("unused") })

	tr := New("acme", 45*time.Second, base)
	c := tr.HTTPClient()
	if c.Timeout != 45*time.Second {
		t.Errorf("HTTPClient().Timeout = %v, want 45s", c.Timeout)
	}
	if c.Transport != http.RoundTripper(tr) {
		t.Errorf("HTTPClient().Transport = %v (%T), want the transport itself", c.Transport, c.Transport)
	}

	if got := New("acme", 0, base).HTTPClient().Timeout; got != 0 {
		t.Errorf("HTTPClient().Timeout with a zero budget = %v, want 0", got)
	}
}

// pacedURL is an org-bound URL the pacing tests send to.
const pacedURL = "https://dev.azure.com/acme/_apis/x"

// send issues one GET to target through c and closes the response. It returns
// the error instead of failing the test, so goroutines may call it.
func send(c *http.Client, target string) error {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// gapAfter sends two requests through a fresh transport whose first response
// carries header (key/value pairs) on an HTTP 200, and returns how long after
// the first the second reached the base. It must run inside a synctest bubble,
// where the fake clock makes the gap exact and costs no wall time.
func gapAfter(t *testing.T, header ...string) time.Duration {
	t.Helper()
	var sent []time.Time
	base := rtFunc(func(r *http.Request) (*http.Response, error) {
		sent = append(sent, time.Now())
		if len(sent) == 1 {
			return emptyResp(r, http.StatusOK, header...), nil
		}
		return emptyResp(r, http.StatusOK), nil
	})
	client := New("acme", 0, base).HTTPClient()
	for i := 0; i < 2; i++ {
		if err := send(client, pacedURL); err != nil {
			t.Fatalf("request %d error = %v", i+1, err)
		}
	}
	if len(sent) != 2 {
		t.Fatalf("base saw %d requests, want 2", len(sent))
	}
	return sent[1].Sub(sent[0])
}

// TestPaceRetryAfterHTTPDate pins the HTTP-date form of Retry-After (D-04): a
// date ten seconds ahead holds the next request exactly ten seconds.
func TestPaceRetryAfterHTTPDate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		date := time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)
		if gap := gapAfter(t, "Retry-After", date); gap != 10*time.Second {
			t.Errorf("Retry-After %q held the next request %v, want exactly 10s", date, gap)
		}
	})
}

// TestPaceRateLimitDelay pins X-RateLimit-Delay on a 200 at the precision ADO
// gives it: decimal seconds, so 0.125 is exactly 125ms (D-04, Pitfall 10).
func TestPaceRateLimitDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		if gap := gapAfter(t, "X-RateLimit-Delay", "0.125"); gap != 125*time.Millisecond {
			t.Errorf("X-RateLimit-Delay 0.125 held the next request %v, want exactly 125ms", gap)
		}
	})
}

// TestPaceMaxNotSum pins that two instructions on one response combine as
// their maximum, never their sum (Pitfall 10).
func TestPaceMaxNotSum(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		if gap := gapAfter(t, "Retry-After", "3", "X-RateLimit-Delay", "5.5"); gap != 5500*time.Millisecond {
			t.Errorf("Retry-After 3 with X-RateLimit-Delay 5.5 held the next request %v, want exactly 5.5s", gap)
		}
	})
}

// TestPaceSameInstructionDoesNotStack pins idempotency: two responses that
// arrive at the same instant, each with Retry-After 5, hold the next request
// 5s, not 10s. The fake base holds both requests behind a barrier until both
// have arrived, so neither waited behind the other's instruction.
func TestPaceSameInstructionDoesNotStack(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		arrived := make(chan struct{})
		release := make(chan struct{})
		var mu sync.Mutex
		var sent []time.Time
		base := rtFunc(func(r *http.Request) (*http.Response, error) {
			mu.Lock()
			sent = append(sent, time.Now())
			first := len(sent) <= 2
			mu.Unlock()
			if first {
				arrived <- struct{}{}
				<-release
				return emptyResp(r, http.StatusOK, "Retry-After", "5"), nil
			}
			return emptyResp(r, http.StatusOK), nil
		})
		client := New("acme", 0, base).HTTPClient()

		errs := make(chan error, 2)
		for i := 0; i < 2; i++ {
			go func() { errs <- send(client, pacedURL) }()
		}
		<-arrived
		<-arrived
		answered := time.Now()
		close(release)
		for i := 0; i < 2; i++ {
			if err := <-errs; err != nil {
				t.Fatalf("concurrent request error = %v", err)
			}
		}
		if err := send(client, pacedURL); err != nil {
			t.Fatalf("third request error = %v", err)
		}

		if len(sent) != 3 {
			t.Fatalf("base saw %d requests, want 3", len(sent))
		}
		if gap := sent[2].Sub(answered); gap != 5*time.Second {
			t.Errorf("third request sent %v after two Retry-After 5 answers, want exactly 5s (the same instruction must not stack)", gap)
		}
	})
}

// TestPaceRemainingIgnored pins D-04's no proactive slowdown: the
// informational headers alone never hold a request, even with nothing
// remaining.
func TestPaceRemainingIgnored(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gap := gapAfter(t, "X-RateLimit-Remaining", "0", "X-RateLimit-Limit", "200", "X-RateLimit-Resource", "Core")
		if gap != 0 {
			t.Errorf("X-RateLimit-Remaining 0 held the next request %v, want no hold", gap)
		}
	})
}

// TestPaceIgnoresInvalid pins that a negative, unparseable, non-finite,
// overflowing or past instruction holds nothing.
func TestPaceIgnoresInvalid(t *testing.T) {
	tests := []struct {
		name   string
		header func() []string // built inside the bubble, so a date uses the fake clock
	}{
		{"negative Retry-After", func() []string { return []string{"Retry-After", "-5"} }},
		{"garbage Retry-After", func() []string { return []string{"Retry-After", "garbage"} }},
		{"past HTTP-date", func() []string {
			return []string{"Retry-After", time.Now().Add(-10 * time.Second).UTC().Format(http.TimeFormat)}
		}},
		{"negative X-RateLimit-Delay", func() []string { return []string{"X-RateLimit-Delay", "-1"} }},
		{"NaN X-RateLimit-Delay", func() []string { return []string{"X-RateLimit-Delay", "NaN"} }},
		{"infinite X-RateLimit-Delay", func() []string { return []string{"X-RateLimit-Delay", "+Inf"} }},
		{"overflowing Retry-After", func() []string { return []string{"Retry-After", "99999999999999"} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				header := tt.header()
				if gap := gapAfter(t, header...); gap != 0 {
					t.Errorf("%s %q held the next request %v, want no hold", header[0], header[1], gap)
				}
			})
		})
	}
}

// TestPaceConcurrent pins the shared pacer under concurrency (T-08-25,
// T-08-35): after Retry-After 4, eight requests sent at once all wait for it,
// each reaches the base exactly once, and -race sees no data race. Each of the
// eight answers carries Retry-After 1, so notBefore is written while other
// requests are still reading it.
func TestPaceConcurrent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		var notBefore time.Time
		calls := map[string]int{}
		var early []string
		base := rtFunc(func(r *http.Request) (*http.Response, error) {
			mu.Lock()
			defer mu.Unlock()
			n := r.URL.Query().Get("n")
			if n == "" {
				notBefore = time.Now().Add(4 * time.Second)
				return emptyResp(r, http.StatusOK, "Retry-After", "4"), nil
			}
			calls[n]++
			if time.Now().Before(notBefore) {
				early = append(early, n)
			}
			return emptyResp(r, http.StatusOK, "Retry-After", "1"), nil
		})
		client := New("acme", 0, base).HTTPClient()
		if err := send(client, pacedURL); err != nil {
			t.Fatalf("priming request error = %v", err)
		}

		const senders = 8
		errs := make(chan error, senders)
		for i := 0; i < senders; i++ {
			go func() { errs <- send(client, pacedURL+"?n="+strconv.Itoa(i)) }()
		}
		for i := 0; i < senders; i++ {
			if err := <-errs; err != nil {
				t.Errorf("concurrent request error = %v", err)
			}
		}

		if len(early) != 0 {
			t.Errorf("requests %v reached the base before the Retry-After 4 had passed", early)
		}
		if len(calls) != senders {
			t.Errorf("base saw %d distinct requests, want %d", len(calls), senders)
		}
		for n, c := range calls {
			if c != 1 {
				t.Errorf("request n=%s reached the base %d times, want exactly once", n, c)
			}
		}
	})
}

// TestHeldCancelIsNotSent pins D-02's held case: a request whose context ends
// while the pacer holds it never reaches the base, closes its body, and
// returns an error that is ErrNotSent and also the context error, so a write
// reads failed while a read keeps its cancel or timeout classification.
func TestHeldCancelIsNotSent(t *testing.T) {
	tests := []struct {
		name     string
		ctx      func() (context.Context, context.CancelFunc)
		wantErr  error
		wantHeld time.Duration
	}{
		{
			name: "already cancelled",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			wantErr:  context.Canceled,
			wantHeld: 0,
		},
		{
			// A deadline can no longer end during a hold: a wait that does not
			// fit the deadline is refused before it starts (D-01, the
			// TestFailFast tests). A cancel can, such as a signal.
			name: "cancelled while held",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				time.AfterFunc(10*time.Second, cancel)
				return ctx, cancel
			},
			wantErr:  context.Canceled,
			wantHeld: 10 * time.Second,
		},
		{
			// A context that already ended reports its own cause, even with
			// a wait pending that its deadline could never fit: the deadline
			// ran out before the pacer was asked, so this is a timeout, not a
			// throttle.
			name: "deadline already passed",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
			wantErr:  context.DeadlineExceeded,
			wantHeld: 0,
		},
		{
			// The CLI's shape after a signal: a fetchctx deadline that the
			// 60s wait does not fit, then a cancel. The cancel wins over the
			// D-01 fail-fast, so an interrupted read stays other, not
			// rate_limit.
			name: "cancelled under a short deadline",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				cancel()
				return ctx, cancel
			},
			wantErr:  context.Canceled,
			wantHeld: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				calls := 0
				base := rtFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					return emptyResp(r, http.StatusOK, "Retry-After", "60"), nil
				})
				client := New("acme", 0, base).HTTPClient()
				if err := send(client, pacedURL); err != nil {
					t.Fatalf("priming request error = %v", err)
				}

				ctx, cancel := tt.ctx()
				defer cancel()
				body := &closeRecorder{Reader: strings.NewReader(`{"op":"add"}`)}
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, pacedURL, body)
				if err != nil {
					t.Fatalf("NewRequest() error = %v", err)
				}
				start := time.Now()
				resp, err := client.Do(req)

				if resp != nil {
					t.Errorf("Do() returned a response, want nil")
				}
				if calls != 1 {
					t.Errorf("base called %d times, want 1 (the priming request only)", calls)
				}
				if !errors.Is(err, ErrNotSent) {
					t.Errorf("errors.Is(%v, ErrNotSent) = false, want true", err)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("errors.Is(%v, %v) = false, want the context error wrapped", err, tt.wantErr)
				}
				if errors.Is(err, ErrRateLimited) {
					t.Errorf("errors.Is(%v, ErrRateLimited) = true, want false: only the D-01 fail-fast is rate limited", err)
				}
				if held := time.Since(start); held != tt.wantHeld {
					t.Errorf("request held %v before release, want %v", held, tt.wantHeld)
				}
				if !body.closed {
					t.Errorf("request body was not closed (RoundTripper contract)")
				}
			})
		})
	}
}

// TestStats pins the D-05 counters: a wait the pacer actually takes is counted
// with its duration, and a request sent with no pending wait adds nothing.
func TestStats(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := 0
		base := rtFunc(func(r *http.Request) (*http.Response, error) {
			n++
			if n == 1 {
				return emptyResp(r, http.StatusOK, "Retry-After", "7"), nil
			}
			return emptyResp(r, http.StatusOK), nil
		})
		tr := New("acme", 0, base)
		client := tr.HTTPClient()

		if waits, waited := tr.Stats(); waits != 0 || waited != 0 {
			t.Fatalf("Stats() before any request = %d, %v, want 0, 0", waits, waited)
		}
		for i := 0; i < 2; i++ {
			if err := send(client, pacedURL); err != nil {
				t.Fatalf("request %d error = %v", i+1, err)
			}
		}
		if waits, waited := tr.Stats(); waits != 1 || waited != 7*time.Second {
			t.Errorf("Stats() after one paced wait = %d, %v, want 1, 7s", waits, waited)
		}
		if err := send(client, pacedURL); err != nil {
			t.Fatalf("request 3 error = %v", err)
		}
		if waits, waited := tr.Stats(); waits != 1 || waited != 7*time.Second {
			t.Errorf("Stats() after an unpaced request = %d, %v, want 1, 7s unchanged", waits, waited)
		}
	})
}

// captureLog points slog.Default, which logging.Logger() falls back to, at a
// buffer for the rest of the test. Do not call logging.Init in this package's
// tests: it would point logging.Logger() at a file and the capture would go
// silently empty (cmd/adoBizInt/patleak_test.go).
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	// slog.SetDefault also redirects the stdlib log package, so restoring it
	// matters for every later test in the binary.
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// logLines returns the non-empty lines in buf.
func logLines(buf *bytes.Buffer) []string {
	var lines []string
	for _, l := range strings.Split(buf.String(), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// TestPaceLogsRateLimitHeadersOnly pins the bounded header logging (Pitfall
// 15, T-08-22, T-08-24): a per-response cost header alone logs nothing, a
// response carrying an instruction logs one line with its rate-limit values,
// and no line carries the URL path or query or the Authorization header.
func TestPaceLogsRateLimitHeadersOnly(t *testing.T) {
	const patFixture = "pacing-log-pat-fixture-do-not-disclose-41c2"
	buf := captureLog(t)
	var answer []string
	base := rtFunc(func(r *http.Request) (*http.Response, error) {
		return emptyResp(r, http.StatusOK, answer...), nil
	})
	client := New("acme", 0, base).HTTPClient()
	do := func(target string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			t.Fatalf("NewRequest(%q) error = %v", target, err)
		}
		req.Header.Set("Authorization", basicAuth(patFixture))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Do(%q) error = %v", target, err)
		}
		resp.Body.Close()
	}

	answer = []string{"x-ratelimit-cost", "1.5"}
	do(pacedURL + "?secretquery=first")
	if lines := logLines(buf); len(lines) != 0 {
		t.Errorf("a response with only x-ratelimit-cost logged %q, want nothing", lines)
	}

	answer = []string{"Retry-After", "2", "X-RateLimit-Resource", "Core"}
	do(pacedURL + "?secretquery=second")
	lines := logLines(buf)
	if len(lines) != 1 {
		t.Fatalf("a response with Retry-After logged %d lines, want 1: %q", len(lines), lines)
	}
	for _, want := range []string{"status=200", "resource=Core", "retry_after=2"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("rate-limit line %q does not contain %s", lines[0], want)
		}
	}
	for _, needle := range []string{
		"secretquery", "_apis",
		patFixture, basicAuth(patFixture), base64.StdEncoding.EncodeToString([]byte(":" + patFixture)),
	} {
		if strings.Contains(buf.String(), needle) {
			t.Errorf("transport log contains %q; it must never log the URL path or query or the Authorization header", needle)
		}
	}
}

// TestPaceLogsWaitTaken pins D-05's log half: every wait the pacer actually
// takes is one line in adoBizInt.log with its duration and the host, and a
// request sent with no pending wait logs no wait line.
func TestPaceLogsWaitTaken(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		buf := captureLog(t)
		n := 0
		base := rtFunc(func(r *http.Request) (*http.Response, error) {
			n++
			if n == 1 {
				return emptyResp(r, http.StatusOK, "Retry-After", "2"), nil
			}
			return emptyResp(r, http.StatusOK), nil
		})
		client := New("acme", 0, base).HTTPClient()
		waitLines := func() []string {
			var out []string
			for _, l := range logLines(buf) {
				if strings.Contains(l, " wait=") {
					out = append(out, l)
				}
			}
			return out
		}

		if err := send(client, "https://dev.azure.com/acme/x"); err != nil {
			t.Fatalf("request 1 error = %v", err)
		}
		if got := waitLines(); len(got) != 0 {
			t.Fatalf("a request with no pending wait logged wait lines %q", got)
		}
		if err := send(client, "https://dev.azure.com/acme/x"); err != nil {
			t.Fatalf("request 2 error = %v", err)
		}
		got := waitLines()
		if len(got) != 1 {
			t.Fatalf("the paced request logged %d wait lines, want 1: %q", len(got), got)
		}
		for _, want := range []string{"wait=2s", "host=dev.azure.com"} {
			if !strings.Contains(got[0], want) {
				t.Errorf("wait line %q does not contain %s", got[0], want)
			}
		}
		if err := send(client, "https://dev.azure.com/acme/x"); err != nil {
			t.Fatalf("request 3 error = %v", err)
		}
		if got := waitLines(); len(got) != 1 {
			t.Errorf("a request with no pending wait added wait lines: %q", got)
		}
	})
}

// assertFailFast checks that err is the D-01 fail-fast: never sent, rate
// limited, not a deadline and not a timeout, so classifyFailure can only read
// it as rate_limit.
func assertFailFast(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil, want the D-01 fail-fast")
	}
	if !errors.Is(err, ErrNotSent) {
		t.Errorf("errors.Is(%v, ErrNotSent) = false, want true", err)
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("errors.Is(%v, ErrRateLimited) = false, want true", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(%v, context.DeadlineExceeded) = true, want false: it would classify timeout", err)
	}
	var to interface{ Timeout() bool }
	if errors.As(err, &to) && to.Timeout() {
		t.Errorf("%v reports Timeout() = true, want false: it would classify timeout", err)
	}
}

// TestFailFastBoundary is the PLAT-03 boundary probe for D-01 with the send
// budget (WR-02): after Retry-After 5, a request is held only when the hold
// ends with at least minSendBudget (2s) of its deadline left, so the boundary
// is 7s. With less than 7s left it fails fast without reaching the base; with
// exactly 7s or more it waits exactly 5s and is sent, and reaches the base with
// at least 2s left. The 5s plus 1ns row is the old boundary: it was held and
// sent with 1ns left, which WR-02 flips to a refusal.
func TestFailFastBoundary(t *testing.T) {
	tests := []struct {
		name     string
		left     time.Duration
		wantSent bool
	}{
		{name: "old boundary 5s plus 1ns", left: 5*time.Second + time.Nanosecond, wantSent: false},
		{name: "7s minus 1ns", left: 7*time.Second - time.Nanosecond, wantSent: false},
		{name: "exactly 7s", left: 7 * time.Second, wantSent: true},
		{name: "7s plus 1ns", left: 7*time.Second + time.Nanosecond, wantSent: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var sent []time.Time
				// atBase is how long each request still had before its
				// context deadline when it reached the base.
				var atBase []time.Duration
				base := rtFunc(func(r *http.Request) (*http.Response, error) {
					sent = append(sent, time.Now())
					if deadline, ok := r.Context().Deadline(); ok {
						atBase = append(atBase, time.Until(deadline))
					}
					if len(sent) == 1 {
						return emptyResp(r, http.StatusOK, "Retry-After", "5"), nil
					}
					return emptyResp(r, http.StatusOK), nil
				})
				client := New("acme", 0, base).HTTPClient()
				if err := send(client, pacedURL); err != nil {
					t.Fatalf("priming request error = %v", err)
				}
				primed := time.Now()

				ctx, cancel := context.WithTimeout(context.Background(), tt.left)
				defer cancel()
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, pacedURL, nil)
				if err != nil {
					t.Fatalf("NewRequest() error = %v", err)
				}
				resp, err := client.Do(req)
				if resp != nil {
					resp.Body.Close()
				}

				if !tt.wantSent {
					assertFailFast(t, err)
					if len(sent) != 1 {
						t.Errorf("base saw %d requests, want 1 (the priming request only)", len(sent))
					}
					if held := time.Since(primed); held != 0 {
						t.Errorf("fail-fast took %v, want no hold at all", held)
					}
					return
				}
				if err != nil {
					t.Fatalf("Do() error = %v, want the request held 5s and sent", err)
				}
				if len(sent) != 2 {
					t.Fatalf("base saw %d requests, want 2", len(sent))
				}
				if gap := sent[1].Sub(primed); gap != 5*time.Second {
					t.Errorf("request sent %v after Retry-After 5, want exactly 5s", gap)
				}
				// The priming request has no deadline, so atBase holds only
				// the held request's.
				if len(atBase) != 1 {
					t.Fatalf("base saw %d requests with a deadline, want 1", len(atBase))
				}
				if atBase[0] < 2*time.Second {
					t.Errorf("request reached the base with %v left, want at least minSendBudget (2s)", atBase[0])
				}
				if want := tt.left - 5*time.Second; atBase[0] != want {
					t.Errorf("request reached the base with %v left, want exactly %v (%v less the 5s hold)", atBase[0], want, tt.left)
				}
			})
		})
	}
}

// TestFailFastClientTimeoutIsDeadline pins the deadline source the CLI and TUI
// rely on when no caller deadline is set: http.Client.Timeout, which
// http.Client installs on the request context before calling a custom
// RoundTripper. A 45s instruction with a 10s per-call budget fails fast, and
// the message gives the hold budget: the 10s less the 2s send budget.
func TestFailFastClientTimeoutIsDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		base := rtFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			return emptyResp(r, http.StatusOK, "Retry-After", "45"), nil
		})
		client := New("acme", 10*time.Second, base).HTTPClient()
		if err := send(client, pacedURL); err != nil {
			t.Fatalf("priming request error = %v", err)
		}

		err := send(client, pacedURL)

		assertFailFast(t, err)
		if calls != 1 {
			t.Errorf("base called %d times, want 1 (the priming request only)", calls)
		}
		for _, want := range []string{"ADO asked for 45s", "8s left in budget"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("fail-fast error %q does not contain %q (D-08)", err, want)
			}
		}
	})
}

// TestFailFastNotCounted pins that a fail-fast is not a wait: Stats is
// unchanged and no wait-taken line is logged, but one Warn line records what
// ADO asked for, the hold budget and the host, never the URL. The budget is
// the 1s left less the 2s send budget, floored at zero.
func TestFailFastNotCounted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		buf := captureLog(t)
		base := rtFunc(func(r *http.Request) (*http.Response, error) {
			return emptyResp(r, http.StatusOK, "Retry-After", "60"), nil
		})
		tr := New("acme", 0, base)
		client := tr.HTTPClient()
		if err := send(client, pacedURL); err != nil {
			t.Fatalf("priming request error = %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pacedURL, nil)
		if err != nil {
			t.Fatalf("NewRequest() error = %v", err)
		}
		resp, err := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		assertFailFast(t, err)

		if waits, waited := tr.Stats(); waits != 0 || waited != 0 {
			t.Errorf("Stats() after a fail-fast = %d, %v, want 0, 0", waits, waited)
		}
		var warn []string
		for _, l := range logLines(buf) {
			if strings.Contains(l, " wait=") {
				t.Errorf("a fail-fast logged a wait-taken line %q", l)
			}
			if strings.Contains(l, "level=WARN") {
				warn = append(warn, l)
			}
		}
		if len(warn) != 1 {
			t.Fatalf("a fail-fast logged %d Warn lines, want 1: %q", len(warn), warn)
		}
		for _, want := range []string{"asked=1m0s", "budget=0s", "host=dev.azure.com"} {
			if !strings.Contains(warn[0], want) {
				t.Errorf("fail-fast line %q does not contain %s", warn[0], want)
			}
		}
		if strings.Contains(buf.String(), "_apis") {
			t.Errorf("transport log contains the URL path: %q", buf.String())
		}
	})
}

// TestFailFastWhenNotBeforeMovesPastDeadline pins that the wait is decided
// again after every sleep: a hold that fitted, overtaken by a later response
// that moves notBefore past the hold budget, fails fast before the deadline
// arrives. With a 10s per-call budget the first 5s hold fits its 8s hold
// budget (10s less the 2s send budget); when it ends, 5s are left, a 3s hold
// budget, against the 57s still to wait. The budget is http.Client.Timeout on
// purpose: if the hold ran on to the deadline, the client's own timer would
// replace the not-sent error with a plain timeout, losing ErrNotSent, and a
// never-sent write would read unknown.
func TestFailFastWhenNotBeforeMovesPastDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		var mu sync.Mutex
		var seen []string
		base := rtFunc(func(r *http.Request) (*http.Response, error) {
			n := r.URL.Query().Get("n")
			mu.Lock()
			seen = append(seen, n)
			mu.Unlock()
			switch n {
			case "slow":
				<-release
				return emptyResp(r, http.StatusOK, "Retry-After", "60"), nil
			case "prime":
				return emptyResp(r, http.StatusOK, "Retry-After", "5"), nil
			}
			return emptyResp(r, http.StatusOK), nil
		})
		tr := New("acme", 10*time.Second, base)
		client := tr.HTTPClient()

		// One request is already at ADO when the pacer starts holding.
		slow := make(chan error, 1)
		go func() { slow <- send(client, pacedURL+"?n=slow") }()
		synctest.Wait()
		if err := send(client, pacedURL+"?n=prime"); err != nil {
			t.Fatalf("priming request error = %v", err)
		}
		start := time.Now()
		held := make(chan error, 1)
		go func() { held <- send(client, pacedURL+"?n=held") }()
		synctest.Wait()

		// 2s into the 5s hold, the slow request answers Retry-After 60.
		time.Sleep(2 * time.Second)
		close(release)
		if err := <-slow; err != nil {
			t.Fatalf("slow request error = %v", err)
		}
		err := <-held
		elapsed := time.Since(start)

		assertFailFast(t, err)
		if want := "ADO asked for 57s, 3s left in budget"; err != nil && !strings.Contains(err.Error(), want) {
			t.Errorf("fail-fast error %q does not contain %q", err, want)
		}
		if elapsed != 5*time.Second {
			t.Errorf("held request failed after %v, want exactly 5s (when its first hold ended)", elapsed)
		}
		for _, n := range seen {
			if n == "held" {
				t.Errorf("the held request reached the base, want it refused")
			}
		}
		if waits, waited := tr.Stats(); waits != 1 || waited != 5*time.Second {
			t.Errorf("Stats() = %d, %v, want 1, 5s (the hold actually taken)", waits, waited)
		}
	})
}
