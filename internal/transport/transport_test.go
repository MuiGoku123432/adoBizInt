package transport

import (
	"bytes"
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
	"testing"
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
// port, dev.azure.com and *.dev.azure.com only under /{org} after path.Clean,
// {org}.visualstudio.com and {org}.*.visualstudio.com on any path, host and path
// case-insensitive, and every look-alike, traversal and cross-org form refused.
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

// TestRefuseRedirectHopIsNotNotSent pins D-10b on redirect hops and Pitfall 4.
// The first hop reached ADO, so a refused hop must not read as never sent: a
// write that got this far is unknown, not failed. The second row is the case
// the per-hop check exists for: Go forwards Authorization on a redirect to the
// same host, so a cross-org redirect on dev.azure.com would carry the PAT.
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
