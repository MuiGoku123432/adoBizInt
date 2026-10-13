# Phase 5: Hardening - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-28
**Phase:** 5-hardening
**Areas discussed:** Attachment surface shape, Unknown-project failure mode, Nil-guard blast radius, CI gate scope and strictness, Log hardening breadth

---

## Attachment surface shape

### How should DownloadAttachment stop the PAT from reaching an arbitrary host?

| Option | Description | Selected |
|--------|-------------|----------|
| SDK identifier, delete the raw HTTP path | Signature takes a uuid.UUID, calls SDK GetAttachmentContent through c.connection, deletes the hand-rolled request and AuthorizationString header | ✓ |
| Keep URL, add a host allow-list check | Parse the URL and reject non-configured hosts before attaching the header | |
| Identifier primary, URL fallback | Identifier as the primary path with an allow-listed URL fallback for unparseable GUIDs | |

**Notes:** The SDK path inherits the Phase 2 `connection.Timeout`, which also
resolves the TRUST-04 exclusion Phase 2 deferred to this phase.

### Does Phase 5 add a CLI attachment download verb, or fix the internal path only?

| Option | Description | Selected |
|--------|-------------|----------|
| TUI-only fix, defer the CLI verb | Fix internal/ado and the single TUI call site; no new verb | ✓ |
| Add the CLI verb now with --output-dir | Ship workitems download-attachment with caller-supplied output directory containment | |

**Notes:** Phase 3 defined the read CLI surface and deliberately excluded
attachments. A file-writing verb also needs an output contract the Phase 3
envelope does not cover.

### Where does path containment live?

| Option | Description | Selected |
|--------|-------------|----------|
| Enforce in internal/ado, service-layer | Signature becomes (ctx, id, destDir, name); the method sanitizes and containment-checks before any network call | ✓ |
| Enforce at the call site | Keep destPath and sanitize in internal/ui/workitems/detail.go | |

**Notes:** Same reasoning as Phase 4's shared `runWrite` guard body — a guard in
one shared place cannot be forgotten by a future caller.

### Roadmap plan 05-01 names a size cap. How should it work?

| Option | Description | Selected |
|--------|-------------|----------|
| Hard cap on bytes copied, config-tunable | io.LimitReader at a configured cap, removing the partial file on overrun | |
| Pre-check the server-reported Size | Reject before download if Attachment.Size exceeds the cap | |
| No cap -- defer with the CLI verb | Human-initiated one-at-a-time TUI download is not a disk-fill path | ✓ |

**Notes:** Overrides the "size cap" wording in the ROADMAP's 05-01 plan sketch.

### The GUID has to come from somewhere. What happens to Attachment.URL?

| Option | Description | Selected |
|--------|-------------|----------|
| Add ID, keep URL as informational | ID drives the fetch; URL stays for a human pasting into a browser | ✓ |
| Replace URL with ID | Smallest struct, removes the field the vulnerability was built on | |
| No struct change, parse on demand | Parse the GUID inside DownloadAttachment | |

**Notes:** There is no `workitems show` verb, so `Attachment` is not published
output yet and the shape change is cheap now.

### What happens when the GUID cannot be parsed out of a relation URL?

| Option | Description | Selected |
|--------|-------------|----------|
| Skip the attachment entirely, like the Name guard | Not appended to detail.Attachments, matching the existing Name=="" guard | ✓ |
| Include it, fail at download time | Visible in the list, errors when selected | |
| Record a Failure -- dropped items must be visible | Machine-visible incompleteness on the Result envelope | |

**Notes:** Matches the Phase 2 D-07 precedent of suppressing rather than naming
a non-call anomaly.

### How does a containment rejection surface in the TUI?

| Option | Description | Selected |
|--------|-------------|----------|
| Plain error through the existing AttachmentDownloadedMsg path | Indistinguishable from a network failure | ✓ |
| Distinct message that names the rejected filename | Tells the operator their ADO served a hostile name | |

**Notes:** A server-crafted traversal name is an attack, not a user mistake; the
operator's action is the same either way.

### How is containment verified?

| Option | Description | Selected |
|--------|-------------|----------|
| Pure sanitizer function, table-tested, plus one fake-seam test | Hostile-name table test plus an integration test proving refusal before any SDK call | ✓ |
| Inline, test only through the fake seam | Honors rule-of-three literally, at the cost of a network-shaped fake for pure string logic | |

**Notes:** Acknowledged as a two-use extraction, short of CONVENTIONS.md's rule
of three. Justified because the guard is the feature.

---

## Unknown-project failure mode

### An agent passes --project with a value not in the configured Projects list. What happens?

| Option | Description | Selected |
|--------|-------------|----------|
| Reject upfront, exit 3 | Validate after config.Load, before any read; fail closed on the whole invocation | ✓ |
| Record a Failure per unknown project, exit 1 | Partial results with the unknown project recorded as a failure | |
| Exit 3 only if no project is valid | Hybrid: proceed with the valid subset unless all are unknown | |

**Notes:** An agent retrying a `1` expecting transience would loop forever
against a permanently bogus name — the trap Phase 3 D-07 cites for auth.

### With the allow-list in place, do we still escape the two WIQL interpolation sites?

| Option | Description | Selected |
|--------|-------------|----------|
| Escape at both sites regardless of the allow-list | Escaping is the structural guarantee in internal/ado; validation is the cmd/ policy layer | ✓ |
| Allow-list is sufficient, skip escaping | Only operator-configured strings can reach WIQL | |
| Escape plus re-validate inside internal/ado | Library self-defends against its own caller | |

**Notes:** The TUI never runs the `cmd/` allow-list, so `internal/ado` cannot
assume validation happened.

### Does dropping the interpolated-query log line land here?

| Option | Description | Selected |
|--------|-------------|----------|
| Log parameters, drop the interpolated query | Structured slog attributes for project, iteration filter, result count | ✓ |
| Keep the full query at Debug | Useful when debugging a WIQL that returns nothing | |
| Keep it, gate behind configurable Debug level | Opt-in only | |

**Notes:** Closes H5 remediation item 3 and part of MED2, landing with the
escaping change rather than with the logging work.

### How do we make the escaping stay fixed?

| Option | Description | Selected |
|--------|-------------|----------|
| AST assertion over WIQL construction | go/ast walk asserting no bare identifier is concatenated into a FROM WorkItems literal | ✓ |
| Behavioural tests on the two known sites | Adversarial table tests plus call-level assertions | |
| Both -- AST plus adversarial table tests | Structural guarantee plus behavioural proof | |

**Notes:** Follows the Phase 4 precedent set after three Phase 3 grep-defeat
incidents.

---

## Nil-guard blast radius

### How wide does HARD-03 go?

| Option | Description | Selected |
|--------|-------------|----------|
| Fix the two sites plus a pattern-scoped AST test | Guards at pipelines.go:171 and releases.go:125, plus an AST test pinning the collect-then-deref shape | ✓ |
| Fix the two sites, behavioural tests only | Nil-Id fixtures through the Phase 1 fake, no structural guard | |
| Repo-wide AST sweep of all deref sites | Flow-insensitive checker over all 423 deref expressions in internal/ado | |

**Notes:** Flagged during discussion that `go vet`'s `nilness` analyzer does not
catch this class — SDK pointer fields are not provably nil, which is why
`go vet ./...` passes clean today.

### When the guard fires on a nil Id, what does it do?

| Option | Description | Selected |
|--------|-------------|----------|
| Silent continue, matching the existing Name guard | Malformed server data, not a failed call | ✓ |
| Record a Failure -- a dropped definition is invisible otherwise | Machine-visible incompleteness | |
| continue plus a Warn log naming the definition | Operator gets a thread to pull, contract unchanged | |

---

## CI gate scope and strictness

### What runs in CI?

| Option | Description | Selected |
|--------|-------------|----------|
| Exactly SC5 -- build, vet, race, gofmt | GitHub Actions on push and PR, nothing beyond the roadmap criterion | ✓ |
| SC5 plus govulncheck | Answers the MED5 question about the 2020 beta SDK | |
| SC5 plus govulncheck plus golangci-lint | Broadest net, at the cost of a triage backlog before the gate can block | |

### Which Go toolchain does CI run?

| Option | Description | Selected |
|--------|-------------|----------|
| Pin the exact Go version from go.mod | go-version: 1.25.4 | ✓ |
| Read the version from go.mod | go-version-file: go.mod | |
| Track stable | Early warning on new toolchains, at the cost of reproducibility | |

### Does CI get ADO credentials?

| Option | Description | Selected |
|--------|-------------|----------|
| No secrets -- a network-touching test failing is the point | Absence of a PAT becomes a hermeticity guard for free | ✓ |
| No secrets plus explicit skip guards | Cleaner output, converts the escape signal into silence | |
| Provide a PAT secret for live tests | Highest fidelity, puts a live PAT in CI | |

**Notes:** Planning must confirm no current test reads `ADO_PAT`.

---

## Log hardening breadth

### How much of CONCERNS.md MED2 lands in this phase?

| Option | Description | Selected |
|--------|-------------|----------|
| Exactly SC6 -- permissions plus the PAT test | 0600 file, 0700 directory, PAT-absence regression test | ✓ |
| SC6 plus configurable level defaulting to Info | Shrinks disk growth and disclosure surface under agent-paced logging | |
| SC6 plus level plus rotation or size cap | Closes MED2 fully, needs a dependency or hand-rolled rotation | |

### What shape does the PAT-absence regression test take?

| Option | Description | Selected |
|--------|-------------|----------|
| Sentinel PAT, buffer-captured, across read + write + panic | Covers the normal path and the path HARD-06 originated from | ✓ |
| Sentinel across read and write paths only | Skips the panic path on the grounds that Go stacks render arguments as hex | |
| Assert against the real log file including its mode | Tests the actual artifact, touches the developer's real config directory | |

**Notes:** Recorded honestly that Go renders stack frame arguments as hex words,
so the stack is unlikely to leak the PAT literally. The test's real value is
catching a future careless `slog` attribute. Planning should not write an
acceptance criterion assuming the stack currently leaks.

### The modes only apply at creation. What about paths that already exist?

| Option | Description | Selected |
|--------|-------------|----------|
| chmod both the file and the directory on every Init | A mode-only change fixes nothing for existing installs | ✓ |
| chmod the log file only | Leaves the PAT-bearing config directory at 0755 | |
| Creation modes only, no chmod of existing paths | Correct for fresh installs, silently ineffective otherwise | |

### What happens when the chmod itself fails?

| Option | Description | Selected |
|--------|-------------|----------|
| Return the error, let the existing non-fatal warning handle it | main.go:17-19 already falls back to slog.Default() on stderr | ✓ |
| Warn and proceed with the existing mode | Keeps logging on a read-only mount, with a file just declared unsafe | |
| Fatal -- refuse to run | Turns a permissions quirk into a total outage | |

---

## the agent's Discretion

None. Every question in this discussion was answered with an explicit choice.

---

## Deferred Ideas

- CLI attachment download verb (`workitems download-attachment`) with
  `--output-dir` containment and a config-tunable byte cap
- `govulncheck ./...` in CI (CONCERNS.md MED5)
- `golangci-lint` with a `.golangci.yml` (needs a triage pass first)
- Configurable log level defaulting to Info (CONCERNS.md MED2)
- Log rotation or size cap (CONCERNS.md MED2)
- Attachment download byte cap (deferred with the CLI verb)
- Routing TUI writes through `runWrite` (BACKLOG.md BL-01 part 2, already
  carried in STATE.md as needing its own discuss-phase)
