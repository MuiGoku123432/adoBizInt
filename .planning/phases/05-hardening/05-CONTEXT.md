# Phase 5: Hardening - Context

**Gathered:** 2026-09-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Close the security findings that the CLI surface makes reachable, and add CI so
they stay closed. Six requirements: HARD-01 (attachment fetch cannot send the
PAT to an arbitrary host), HARD-02 (attachment writes cannot escape the intended
directory), HARD-03 (the two MED4 nil-dereference sites are guarded), HARD-04
(WIQL project interpolation is escaped and validated against the configured
project list), HARD-05 (CI runs build, vet, race tests and gofmt on every push),
HARD-06 (log file 0600 / directory 0700, with a regression test asserting the PAT
never appears in log output).

This phase closes findings. It does not add capability. No new CLI verb ships
here.

</domain>

<decisions>
## Implementation Decisions

### Attachment fetch and write (HARD-01, HARD-02)

- **D-01:** `DownloadAttachment` takes an attachment identifier (`uuid.UUID`),
  not a URL. It calls the SDK's `GetAttachmentContent` through `c.connection`,
  and the hand-rolled `http.NewRequestWithContext` + `AuthorizationString`
  header + `http.DefaultClient` path at `internal/ado/workitems.go:612-643` is
  deleted outright. The PAT can then only reach the configured organization
  host structurally — there is no URL parameter left to abuse. This also
  inherits the Phase 2 `connection.Timeout`, which resolves the TRUST-04
  exclusion Phase 2 deferred to this phase. Adds one entry
  (`GetAttachmentContent`) to the Phase 1 `workitemAPI` interface; the entry is
  called, so the Phase 4 SC4 no-uncalled-interface-entry criterion holds.
  — **Reversibility:** costly — undoing means reinstating a raw HTTP path and
  re-deriving the header, which is the vulnerability itself.

- **D-02:** Phase 5 fixes the internal path and its single existing caller
  (`internal/ui/workitems/detail.go:191`) only. No CLI attachment download verb
  ships in this phase. Phase 3 defined the read CLI surface and deliberately
  excluded attachments; adding a verb here is a new capability, and HARD-01 and
  HARD-02 are both satisfied without it. `--output-dir` containment gets
  designed when the verb is actually planned.

- **D-03:** Path containment is enforced in `internal/ado`, not at the call
  site. The signature becomes `(ctx, id, destDir, name)`; the method itself does
  `filepath.Base`, rejects empty / `.` / `..` / absolute results, and performs a
  post-join prefix check against the resolved destination directory — all before
  any network call. Every future caller, including the deferred CLI verb,
  inherits the guard and cannot forget it. This is the same reasoning that put
  the Phase 4 write guards in a shared `runWrite` body rather than in six copies.
  — **Reversibility:** costly — the `destPath` → `destDir` + `name` signature
  change touches the call site and the interface.

- **D-04:** No download size cap in this phase. Attachments come from the
  operator's own ADO organization, the TUI download is human-initiated and
  one-at-a-time, so disk-fill is not a realistic path until the deferred CLI
  verb enables bulk iteration. The cap is deferred with the verb. This overrides
  the "size cap" wording in the ROADMAP's 05-01 plan sketch.

- **D-05:** `Attachment` gains an `ID` field (uuid string); `URL` is retained as
  informational only. Nothing in the code path consumes `URL` after D-01 — it
  stays for a human pasting it into a browser. `WorkItemDetail` is JSON-tagged
  but there is no `workitems show` verb in the dispatch table, so `Attachment`
  is not yet published output and this shape change is cheap now and expensive
  after such a verb ships.

- **D-06:** When the GUID cannot be parsed out of a relation URL, the attachment
  is not appended to `detail.Attachments` — the same rule as the existing
  `attachment.Name != ""` guard at `internal/ado/workitems.go:492`. It cannot be
  selected in the TUI, so it cannot fail at download time. No `Failure` is
  recorded; this matches the Phase 2 D-07 precedent of suppressing rather than
  naming a non-call anomaly.

- **D-07:** A containment rejection surfaces through the existing
  `AttachmentDownloadedMsg{Err: ...}` path as a plain error, indistinguishable
  from a network failure. A server-crafted traversal name is an attack, not a
  user mistake, and the operator's action is the same either way.

- **D-08:** Containment is verified by extracting a pure function
  `(name, destDir) -> (path, error)` and table-testing the hostile set directly:
  `../`, `..\`, absolute paths, empty, `.`, `..`, embedded NUL, all-separator
  names. Plus one integration test through the Phase 1 fake proving
  `DownloadAttachment` refuses before any `GetAttachmentContent` call is made.
  This is a two-use extraction (download now, deferred CLI verb later), short of
  the CONVENTIONS.md rule of three — justified explicitly because the guard is
  the feature.

### WIQL and project validation (HARD-04)

- **D-09:** An unknown `--project` value is rejected upfront in `cmd/` right
  after `config.Load`, before any read runs, and exits `3`. Nothing ran, so
  exit 3 is the accurate fact and matches the Phase 3 D-08 rationale. Fail
  closed: the whole invocation is rejected even if two of three supplied
  projects were valid. Rejected the alternative of recording a per-project
  `Failure` and exiting `1` — an agent that retries a `1` expecting transience
  would loop forever against a permanently bogus project name, which is the
  exact trap D-07 cites for auth.
  — **Reversibility:** one-way — the exit-code vocabulary is a published
  contract an agent branches on (Phase 3 D-06/D-07/D-08); moving this mapping
  later breaks callers already written against it.

- **D-10:** Both WIQL interpolation sites are escaped with
  `strings.ReplaceAll(project, "'", "''")` — `internal/ado/workitems.go:149` and
  `internal/ado/dashboard.go:142` — matching the escaping already applied to
  iteration paths a few lines above each. The allow-list does not remove this:
  it is a `cmd/` policy check the TUI never runs, and `internal/ado` is a library
  that must not assume its caller validated anything. Escaping is the structural
  guarantee; validation is the policy layer; they live in different packages on
  purpose.

- **D-11:** The interpolated query is no longer logged. `internal/ado/workitems.go`
  and `internal/ado/dashboard.go` log the project name, iteration filter and
  result count as structured `slog` attributes instead of the assembled query
  string. This closes the H5 remediation item 3 and part of MED2, and it lands
  with the escaping change rather than with the logging work.

- **D-12:** Escaping is kept fixed by a `go/ast` test that walks `internal/ado`,
  finds every string literal containing `FROM WorkItems`, and asserts that no
  concatenation operand in that expression is a bare identifier — every
  interpolated value must be a call result. A third WIQL site added later fails
  the test, and a prose comment cannot defeat it. This follows the Phase 4
  precedent established after three Phase 3 incidents where a grep-negative
  criterion was satisfied and then defeated by an explanatory comment.

### Nil guards (HARD-03)

- **D-13:** HARD-03 covers the two MED4 sites only — `internal/ado/pipelines.go:171`
  and `internal/ado/releases.go:125` — each gaining `|| d.Id == nil` at the
  collection guard. Plus a `go/ast` test pinning the specific shape: any loop
  that collects SDK definitions into a slice and later dereferences `.Id` must
  guard both `.Name` and `.Id` at the collection point. Rejected a repo-wide
  sweep of all 423 deref expressions in `internal/ado`: a flow-insensitive
  checker cannot see the enclosing guard, so most hits would be false positives,
  and CONCERNS.md itself calls these two sites outliers rather than a systemic
  pattern. `go vet`'s `nilness` analyzer does not catch this class — SDK pointer
  fields are not provably nil, which is why `go vet ./...` passes clean today.
  The `handlePanic` recover in `cmd/adoBizInt/crash.go` remains the backstop for
  everything else.

- **D-14:** When the guard fires on a nil `Id` it does a silent `continue`,
  matching the existing `Name == nil` guard immediately above it. A definition
  the server returned without an `Id` is malformed server data, not a failed
  call, and Phase 2 D-07 already set the precedent of suppressing rather than
  naming a non-call anomaly.

### CI (HARD-05)

- **D-15:** CI is exactly the ROADMAP SC5 set and nothing more: GitHub Actions
  on push and PR running `go build ./...`, `go vet ./...`, `go test -race ./...`,
  and a `gofmt -l` check that fails on any output. `govulncheck` and
  `golangci-lint` are deferred — `golangci-lint` on a 7,600-line brownfield repo
  with this comment-heavy house style produces a triage backlog before the gate
  can be made blocking, and that backlog is not what this phase is for.

- **D-16:** The workflow pins `go-version: 1.25.4`, matching `go.mod` exactly.
  CI failures then reflect the code rather than toolchain drift the developer
  did not choose, and the one-line bump on a Go upgrade becomes an explicit
  commit.

- **D-17:** CI gets no ADO credentials, and that absence is itself the
  assertion. Every test runs against the Phase 1 fake seam; a test that reaches
  real ADO fails on a missing PAT, which is the correct signal that a test
  escaped the seam. Rejected adding explicit skip guards — that converts the
  escape signal into silence, which is the failure mode this project exists to
  avoid. Planning must confirm no current test reads `ADO_PAT`.

### Log hardening (HARD-06)

- **D-18:** Exactly SC6 lands here: `0600` file, `0700` directory, and the
  PAT-absence regression test. Configurable log level, rotation and size cap are
  CONCERNS.md MED2 items the ROADMAP deliberately did not promote to a success
  criterion — deferred. (The interpolated-WIQL log line is already handled by
  D-11.)

- **D-19:** The regression test points the logger at an in-memory buffer, sets a
  distinctive sentinel PAT, drives a representative read, a write, and a forced
  panic through the Phase 1 fake, and asserts the sentinel appears nowhere in
  the captured bytes. The panic path is included because it is where HARD-06
  originated (Phase 3 threat T-03-15): `handlePanic` at
  `cmd/adoBizInt/crash.go:96` deliberately writes the full `debug.Stack()` to
  the log file. Recorded honestly for planning: Go renders stack frame arguments
  as hex words rather than dereferenced strings, so the stack is unlikely to leak
  the PAT literally — the realistic regression path this test defends is a future
  `slog` attribute, not the stack.

- **D-20:** `logging.Init` explicitly `chmod`s both the log file to `0600` and
  the directory to `0700` on every call, not just at creation. `os.OpenFile`'s
  mode and `MkdirAll`'s mode apply only at creation, so a mode-only change would
  fix nothing for any existing install. The directory also holds the
  PAT-bearing config file, which makes tightening it the higher-value half.

- **D-21:** A failed `chmod` returns the error from `Init`. `cmd/adoBizInt/main.go:17-19`
  already treats an `Init` failure as a non-fatal warning and falls back to
  `slog.Default()` on stderr, so the existing policy applies unchanged and a
  `--json` consumer discards the diagnostics. No new failure mode is introduced.

### the agent's Discretion

None — every question in this discussion was answered with an explicit choice.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements and scope
- `.planning/ROADMAP.md` § "Phase 5: Hardening" — the six success criteria and
  the five-plan sketch. Note D-04 overrides the "size cap" wording in 05-01.
- `.planning/REQUIREMENTS.md` §§ HARD-01 … HARD-06 (lines 57-62) — the
  requirement text, including the T-03-15 provenance note on HARD-06.
- `.planning/PROJECT.md` § Constraints — no new dependencies without stated
  justification; PAT never to a caller-supplied host, never to stdout or the log.

### Findings being closed
- `.planning/codebase/CONCERNS.md` § C3 — PAT on a caller-supplied URL
  (HARD-01).
- `.planning/codebase/CONCERNS.md` § H6 — path traversal in attachment download
  (HARD-02).
- `.planning/codebase/CONCERNS.md` § H5 — WIQL injection via unescaped project
  name (HARD-04), including remediation item 3 on query logging.
- `.planning/codebase/CONCERNS.md` § MED4 — nil pointer dereference sites
  (HARD-03).
- `.planning/codebase/CONCERNS.md` § MED2 — log permissions and content
  (HARD-06); note only the permissions and PAT-test portions are in scope per
  D-18.
- `.planning/codebase/CONCERNS.md` § MED6 — no CI, no linter config (HARD-05).

### Locked contracts this phase must not break
- `cmd/adoBizInt/exit.go` — the Phase 3 exit-code contract (`exitCodeFor`,
  `writeExitCodeFor`) and its D-06/D-07/D-08 rationale, which D-09 extends
  rather than amends.
- `cmd/adoBizInt/crash.go` — the D-26 crash record shape and the deliberate
  stack-to-log-only policy that HARD-06 exists to protect.
- `.planning/phases/04-guarded-writes/04-CONTEXT.md` — the write envelope,
  audit record, and the go/ast-over-grep acceptance-criteria precedent that
  D-12 and D-13 follow.
- `.planning/phases/02-honest-reads/02-CONTEXT.md` — the `Result` / `Failure`
  envelope and `FailureKind` vocabulary, and the D-07 precedent for suppressing
  non-call anomalies that D-06 and D-14 rely on.
- `.planning/codebase/CONVENTIONS.md` — rule of three, no speculative
  generality, files under 300 lines. D-08 is an explicit, justified exception.

### Code touched
- `internal/ado/workitems.go` — `Attachment` (`:67`), attachment extraction
  (`:475-497`), WIQL construction (`:149`), `DownloadAttachment` (`:612-643`).
- `internal/ado/dashboard.go:142` — the second WIQL site.
- `internal/ado/pipelines.go:171` and `internal/ado/releases.go:125` — the MED4
  sites.
- `internal/ui/workitems/detail.go:186-193` — the sole `DownloadAttachment`
  caller, including the discarded `os.UserHomeDir()` error.
- `internal/logging/logging.go:19-31` — directory mode, file mode, level.
- `cmd/adoBizInt/flags.go:177` and `cmd/adoBizInt/dispatch.go:257-266` — where
  `--project` is registered and passed through unvalidated.
- `internal/config/config.go:100-102, :137-140` — where `cfg.Projects` is
  populated, the allow-list source for D-09.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **Phase 1 fake seam** (`internal/ado`, unexported client fields): in-package
  fakes drive every test in this phase — the containment integration test
  (D-08), the nil-guard tests (D-13), and the PAT-absence test (D-19). It is
  in-package and cannot be built from `cmd/`, which is why D-08's integration
  half lives in `internal/ado`.
- **SDK `GetAttachmentContent`** (`workitemtracking/client.go:826`): takes
  `Id *uuid.UUID`, optional `Project` and `FileName`, returns `io.ReadCloser`,
  and routes through `c.connection`. This is what makes D-01 a deletion rather
  than a rewrite.
- **Existing iteration-path escaping** in both WIQL functions: the exact
  `strings.ReplaceAll(x, "'", "''")` call D-10 needs already sits a few lines
  above each unescaped project interpolation.
- **`cmd/adoBizInt/crash.go`**: the panic-to-structured-JSON recover is already
  shipped, so MED4's "install a top-level recover" remediation half is done and
  D-13 only owes the two guards.

### Established Patterns
- **go/ast structural assertions over grep-negative criteria** (Phase 4, after
  three Phase 3 grep-defeat incidents). D-12 and D-13 both take this form. Any
  acceptance criterion in this phase phrased as "grep returns nothing" should be
  rewritten as a structural test.
- **Guards live in the shared body, not in each caller** (Phase 4 `runWrite`).
  D-03 applies the same reasoning to path containment.
- **Suppress non-call anomalies rather than inventing a `FailureKind`**
  (Phase 2 D-07, the empty-repository 404 case). D-06 and D-14 both rely on it.
- **Auth short-circuits before completeness in exit mapping** (Phase 3 D-07).
  D-09 adds a pre-read rejection that never reaches this mapping at all.

### Integration Points
- `workitemAPI` interface gains `GetAttachmentContent` (D-01) — the one
  interface addition in this phase, and it is called, satisfying Phase 4 SC4.
- `cmd/` gains a project allow-list check between `config.Load` and dispatch
  (D-09) — a new pre-read validation stage that must return `exitUnknown`'s
  sibling `3`, not a read outcome.
- `.github/workflows/` is created from nothing (D-15) — there is no CI, no
  linter config, and no build automation in the repo today.

</code_context>

<specifics>
## Specific Ideas

- The `--project` flag already ships (`cmd/adoBizInt/flags.go:177`), so H5 is no
  longer the latent, operator-controlled issue CONCERNS.md describes. Agent-supplied
  strings reach both unescaped WIQL sites today. Planning should treat HARD-04 as
  live rather than preventative.
- There is no `workitems show` verb in the dispatch table. `WorkItemDetail` and
  `Attachment` are JSON-tagged in anticipation but are not published output yet,
  which is what makes D-05's struct change cheap. If a `show` verb is ever added,
  the `Attachment` shape becomes a one-way door.
- Phase 2 explicitly deferred `DownloadAttachment`'s unbounded `http.DefaultClient`
  (TRUST-04) to "Phase 5 with the host allow-list." D-01 resolves that deferral by
  a different route than anticipated — through the SDK connection rather than an
  allow-list — and it should be marked resolved, not dropped.
- HARD-06's stated threat (T-03-15, a panic stack rendering the PAT into a
  world-readable file) is weaker than it reads: Go prints stack frame arguments as
  hex words. The permissions fix and the test are still worth having; the test's
  real value is catching a future careless `slog` attribute. Planning should not
  write an acceptance criterion that assumes the stack currently leaks.

</specifics>

<deferred>
## Deferred Ideas

- **CLI attachment download verb** (`workitems download-attachment`) with
  `--output-dir` containment and a config-tunable byte cap. Deferred out of
  Phase 5 as a new capability (D-02, D-04). Note it needs an output contract for
  a verb that writes a file rather than emitting JSON — a shape the Phase 3
  envelope does not cover.
- **`govulncheck ./...` in CI.** CONCERNS.md MED5 asks for it specifically
  before scoping an SDK migration off the 2020 beta. Deferred from D-15; pairs
  naturally with the v2 SDK-upgrade item.
- **`golangci-lint` with a `.golangci.yml`.** Deferred from D-15 — needs a
  triage pass over the existing 7,600 lines before the gate can be blocking.
- **Configurable log level defaulting to Info** (CONCERNS.md MED2). Deferred
  from D-18.
- **Log rotation or a size cap** (CONCERNS.md MED2). Deferred from D-18; would
  need a new dependency or hand-rolled logic, against the PROJECT.md constraint.
- **Attachment download byte cap.** Deferred with the CLI verb (D-04).
- **Routing TUI writes through `runWrite`** (BACKLOG.md BL-01 part 2) — already
  carried in STATE.md as needing its own discuss-phase. Not touched here.

</deferred>

---

*Phase: 5-Hardening*
*Context gathered: 2026-09-28*
