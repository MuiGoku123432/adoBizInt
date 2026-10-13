---
phase: 05-hardening
plan: 03
type: execute
wave: 1
depends_on: []
files_modified:
  - .github/workflows/ci.yml
autonomous: true
requirements: [HARD-05]

estimate:
  tokens: 20000
  raw_tokens: 20000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "`.github/workflows/ci.yml` runs on every `push` and every `pull_request`, with no branch filter (HARD-05)."
    - "The workflow runs exactly four gates, in order: `go build ./...`, `go vet ./...`, `go test -race ./...`, and a `gofmt -l .` step that exits 1 and lists the offending files when its output is non-empty. It adds no `govulncheck` and no `golangci-lint` (D-15)."
    - "`actions/setup-go` is given `go-version: '1.25.4'` as a quoted string that exactly matches `go.mod` (D-16)."
    - "The workflow references no secret, no ADO credential and no `ADO_*` environment variable, and has no skip guard. A test that escapes the fake seam fails in CI on the missing PAT, and that failure is the signal (D-17)."
    - "No current test reads a real `ADO_PAT`. The only reference, at `cmd/adoBizInt/usage_test.go:50-51`, blanks `ADO_PAT` and `ADO_ORG_URL` so that `--help` is proven to need no credential. The full suite passes with every `ADO_*` variable unset and an empty `$HOME` (D-17: confirmed at planning, re-run by Task 2)."
    - "The workflow token is read-only (`permissions: contents: read`), and both actions are pinned to 40-hex commit SHAs with the release tag in a trailing comment (org least-privilege and supply-chain rule)."
    - "When the gofmt step's own script is run in a directory holding an unformatted `.go` file, it exits non-zero and names that file. Run at the repo root, it exits 0 (edge-probe HARD-05/empty: empty `gofmt -l` output is the pass signal, and any output fails)."
    - statement: "A push of the phase branch to GitHub triggers the ci workflow and the run goes green; a deliberately unformatted file on a throwaway branch turns it red. PHASE-BLOCKING for the green run: Phase 5 is not complete without one green GitHub Actions run on the phase branch (05-VALIDATION.md). Checked by the Task 2 human-check."
      verification: backstop
  prohibitions:
    - statement: "The CI workflow must never be given ADO credentials or a secret, and must never convert a missing credential into a skipped test (D-17)."
      status: resolved
      verification: test
  artifacts:
    - path: ".github/workflows/ci.yml"
      provides: "GitHub Actions gate: build, vet, race tests, gofmt"
      contains: "go test -race ./..."
  key_links:
    - from: ".github/workflows/ci.yml"
      to: "go.mod"
      via: "setup-go go-version pinned to the go directive's exact value"
      pattern: "go-version: '1\\.25\\.4'"
---

<objective>
Close CONCERNS.md MED6. Add a GitHub Actions workflow that runs build, vet, race tests
and a gofmt check on every push and pull request, and fails on any regression.

Purpose: every other plan in this phase closes a finding with a test. This workflow is
what keeps those tests closing it.
Output: `.github/workflows/ci.yml`. There is no other file and no linter config (D-15).
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/05-hardening/05-CONTEXT.md
@.planning/phases/05-hardening/05-RESEARCH.md
</context>

## Artifacts this phase produces

Phase 05 creates or changes these symbols. None of them exists yet. **This plan owns the 05-03 row.**

| Plan | Symbol / file | Kind |
|---|---|---|
| 05-01 | `internal/ado/attachments.go`; `DownloadAttachment(ctx, id uuid.UUID, destDir, name string) error`; `attachmentPath(name, destDir string) (string, error)`; `Attachment.ID`; `workItemAPI.GetAttachmentContent`; `fakeWorkItemAPI.GetAttachmentContentFunc` | new / replaced |
| 05-01 | removed: `Client.connection`, `AttachmentDownloadedMsg.Path`, URL-based `DownloadAttachment` | deletions |
| 05-01 | `TestDownloadAttachmentFetchesByID`, `TestAttachmentPath`, `TestDownloadAttachmentRefusesBeforeFetch`, `TestDownloadAttachmentRemovesPartialFile`, `TestGetWorkItemDetailAttachmentID`, `TestNoRawHTTPInADO` | new tests |
| 05-02 | `internal/ado/nilguard_test.go` with three tests | new tests |
| **05-03** | **`.github/workflows/ci.yml` — workflow `ci`, job `check`, steps build / vet / race test / `gofmt`** | **new file** |
| 05-04 | `var configuredProjects []string` (`cmd/adoBizInt/flags.go`); `--project` via `fs.Func`; `withConfiguredProjects`; `TestUnknownProjectRejected` | new |
| 05-05 | `func iterationClause(paths []string) string` (`internal/ado/client.go`); `internal/ado/wiql_test.go` with four tests | new |
| 05-06 | `TestInitSetsOwnerOnlyPermissions`; `TestPATNeverLogged` | new tests |

## Planner decisions recorded in this plan

- **Action versions** (verified at planning time with `gh api repos/actions/{checkout,setup-go}/git/ref/tags/...`; both tags point directly at commits):
  - `actions/checkout` v7.0.1 is `3d3c42e5aac5ba805825da76410c181273ba90b1`.
  - `actions/setup-go` v7.0.0 is `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e`. Its `cache` input defaults to on, keyed on `go.sum`.
  - Re-check both if this plan executes after 2026-10-30, the RESEARCH validity date.
- **No `.gitignore` change.** `.gsd/` is untracked locally and does not exist in a CI checkout. `gofmt -l .` is clean today.
- **Verification uses Ruby's stdlib YAML parser** (present on macOS), not a grep. Comments are not part of the parse tree, so a commented-out `permissions:` line can neither satisfy nor defeat the check. This is the same reasoning as the phase's go/ast tests. No Go test is added for the workflow file, because it would be single-use scaffolding.
- **The GitHub run check is a PHASE-BLOCKING `<human-check>` on Task 2**, not on the Task 1 tracer. The verifier harvests `<human-check>` only from `auto` tasks. On the tracer, the check would halt an interactive run mid-flight and be skipped in auto mode, so the 05-VALIDATION.md phase gate would never reach UAT.
- **The first run on Linux is the first Linux run of this suite** (RESEARCH A4). It passes on darwin under Go 1.25.4 and 1.27.1, and `GOOS=linux go vet` is clean, so a red first run is possible and would be real signal.

## Edge-probe and prohibition-probe disposition (spec-less fallback)

| Probe item | Disposition |
|---|---|
| HARD-05 / empty | resolved (explicit): empty `gofmt -l` output passes and any output fails (truth 7, Task 2). |
| HARD-05 / ordering | resolved (explicit): the four steps run in a fixed order, and GitHub stops the job at the first failing step, so a build failure is never masked by a later gate (truth 2). |
| HARD-05 / adjacency (push and pull_request both fire for the same commit) | resolved (explicit): accepted as intended. Both events trigger, which can mean two runs for one commit on a PR branch. D-15 asks for "push and PR" and names no de-duplication. |
| Prohibition recall | One kept (D-17 credentials). Action SHA pinning and a read-only token are canon supply-chain items, recorded as truths and covered by `/gsd-secure-phase`. |

<tasks>

<task type="tracer">
  <name>Task 1: A workflow that runs the four gates on push and pull request</name>
  <files>.github/workflows/ci.yml</files>
  <read_first>
    - go.mod (the `go 1.25.4` directive, which is the D-16 source of truth)
    - .gitignore (confirm no change is needed)
    - .planning/phases/05-hardening/05-RESEARCH.md § "CI workflow" code example and § "Supporting (CI)"
    - .planning/phases/05-hardening/05-CONTEXT.md D-15, D-16, D-17
  </read_first>
  <action>
    Create `.github/workflows/ci.yml`. The `.github` directory does not exist yet.

    Top-level keys:
    - `name: ci`
    - `on:` with bare `push:` and `pull_request:` keys and no branch filters
    - `permissions:` with only `contents: read`

    One job, `check`, with `runs-on: ubuntu-latest`, `timeout-minutes: 15`, and these steps in order:
    1. `uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1` with the trailing comment `# v7.0.1`.
    2. `uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` with the trailing comment `# v7.0.0`, and `with: go-version: '1.25.4'`. Single-quote the version, because unquoted YAML can read it as a number (D-16).
    3. `run: go build ./...`
    4. `run: go vet ./...`
    5. `run: go test -race ./...`
    6. A step named exactly `gofmt` whose multi-line `run` assigns `$(gofmt -l .)` to a variable. If the variable is non-empty, it prints a heading line and the variable, then exits 1. It does not use a bare `test -z`, which fails without naming the files.

    Add no `env:` block, no `secrets` reference, no matrix, no `govulncheck`, no `golangci-lint` and no cache override (D-15, D-17). Use at most two comment lines at the top, stating that the gate set is exactly ROADMAP SC5 and citing D-15 to D-17.
  </action>
  <verify>
    <automated>ruby -ryaml -e 'w = YAML.load_file(".github/workflows/ci.yml"); t = w["on"] || w[true]; abort "triggers" unless t.is_a?(Hash) && t.key?("push") && t.key?("pull_request"); abort "permissions" unless w["permissions"] == {"contents" => "read"}; s = w["jobs"]["check"]["steps"]; u = s.map { |x| x["uses"] }.compact; abort "pins" unless u.size == 2 && u.all? { |x| x =~ /\A[\w.\/-]+@[0-9a-f]{40}\z/ }; g = s.find { |x| x["uses"].to_s.start_with?("actions/setup-go@") }; abort "go-version" unless g["with"]["go-version"] == "1.25.4"; r = s.map { |x| x["run"] }.compact.join("\n"); ["go build ./...", "go vet ./...", "go test -race ./...", "gofmt -l ."].each { |c| abort "missing: #{c}" unless r.include?(c) }; abort "credential reference" if w.to_s =~ /secrets|ADO_|token/i; puts "ci.yml structure ok"' && go build ./... && go vet ./... && go test -race ./... && test -z "$(gofmt -l .)"</automated>
    <fails_when>non-zero exit, or the output lacks the line `ci.yml structure ok`, or Ruby prints one of `triggers`, `permissions`, `pins`, `go-version`, `missing:` or `credential reference`</fails_when>
  </verify>
  <acceptance_criteria>
    - `.github/workflows/ci.yml` exists and the Ruby structural check prints `ci.yml structure ok`.
    - The parsed workflow has exactly one job (`check`) and exactly six steps, in the order listed in `<action>`.
    - `go build ./...`, `go vet ./...` and `go test -race ./...` exit 0 locally, and `gofmt -l .` prints nothing.
  </acceptance_criteria>
  <done>The workflow exists, is structurally correct, and its four gates pass locally. Committed.</done>
</task>

<task type="auto">
  <name>Task 2: Prove the gates fail in the right direction and that the suite needs no credential</name>
  <files>.github/workflows/ci.yml</files>
  <read_first>
    - .github/workflows/ci.yml (as written in Task 1)
    - cmd/adoBizInt/usage_test.go:44-60 (`TestHelpNeedsNoCredentials`, which blanks `ADO_PAT` and is the only ADO_* reference in any test)
    - internal/config/config.go:128-146 (`loadFromEnv`, the only reader of `ADO_PAT`)
  </read_first>
  <action>
    This task changes no file unless it finds a defect. It runs the two failure-direction proofs below and records both results in the plan summary.

    First, extract the `gofmt` step's `run` script from the parsed workflow and run it with `bash -e`, first in a temporary directory holding one deliberately unformatted `.go` file, then at the repo root. The first run must exit non-zero and name the file. The second must exit 0. This tests the actual step text, not a copy of it.

    Second, run the full race suite with `ADO_PAT`, `ADO_ORG_URL`, `ADO_PROJECTS` and `ADO_PROJECT` unset and `HOME` pointed at an empty temporary directory. Pass `GOMODCACHE` and `GOCACHE` through from `go env`, so the empty `HOME` does not force a module download. This is the local stand-in for CI's credential-free runner (D-17). Record in the summary that, at planning time, no test read a real `ADO_PAT`; the only reference blanks it.

    If either proof fails, fix `ci.yml` (proof 1) or report the credential-reading test by name without adding a skip guard (proof 2; D-17 forbids converting the signal into silence).
  </action>
  <verify>
    <automated>S=$(ruby -ryaml -e 'w = YAML.load_file(".github/workflows/ci.yml"); puts w["jobs"]["check"]["steps"].find { |x| x["name"] == "gofmt" }["run"]') && D=$(mktemp -d) && printf 'package p\nfunc  F() {}\n' > "$D/bad.go" && OUT=$(cd "$D" && bash -e -c "$S" 2>&1); RC=$?; rm -rf "$D"; [ "$RC" -ne 0 ] && printf '%s' "$OUT" | grep -q 'bad.go' && bash -e -c "$S" && echo "gofmt step: fails on bad.go, passes on repo"</automated>
    <fails_when>non-zero exit, or the output lacks the line `gofmt step: fails on bad.go, passes on repo`</fails_when>
    <automated>H=$(mktemp -d) && GOMODCACHE="$(go env GOMODCACHE)" GOCACHE="$(go env GOCACHE)" env -u ADO_PAT -u ADO_ORG_URL -u ADO_PROJECTS -u ADO_PROJECT HOME="$H" go test -race -count=1 ./...; RC=$?; rm -rf "$H"; exit $RC</automated>
    <fails_when>non-zero exit, or any `--- FAIL` or `FAIL	sentinovo.ai/bizInt` line in the output</fails_when>
    <human-check>
      <test>PHASE-BLOCKING (05-VALIDATION.md phase gate). Run this after every Phase 5 plan, including both wave-2 plans, has landed. Push the phase branch to `origin` yourself; the executor does not push. Open the repository's Actions tab on GitHub and find the `ci` run for the pushed HEAD commit. Optional and not blocking: on a throwaway branch, commit a `.go` file with deliberately broken formatting, push it, watch the run, then delete the branch.</test>
      <expected>The `ci` run for the phase branch's HEAD commit is green, with the four gate steps shown and passing. If the optional throwaway run was done, it is red at the `gofmt` step, and its log names the unformatted file.</expected>
      <why_human>PHASE-BLOCKING. 05-VALIDATION.md requires one green GitHub Actions run on the phase branch before `/gsd-verify-work`. Phase 5 is not complete without it, even when every local gate is green. Only a real push proves the workflow triggers on GitHub, and pushing is the operator's call. The check sits on this `auto` task, not on the Task 1 tracer, because the end-of-phase harvest collects human-check blocks only from `auto` tasks.</why_human>
    </human-check>
  </verify>
  <acceptance_criteria>
    - The gofmt-step proof prints `gofmt step: fails on bad.go, passes on repo`.
    - The credential-free, empty-HOME race suite exits 0.
    - The plan summary records both results and the D-17 confirmation.
    - `git status --short` shows no stray `bad.go` or temporary directory inside the repository.
  </acceptance_criteria>
  <done>The workflow's gates are shown to go red on a regression and green on the current tree, and the suite is shown to need no credential.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| GitHub Actions runner → repository | Third-party action code executes with the workflow token |
| Pull request author → workflow | A PR, possibly from a fork, triggers the workflow on its own code |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-05-10 | Tampering | `actions/checkout` and `actions/setup-go` tags re-pointed upstream | high | mitigate | Both are pinned to full 40-hex commit SHAs, asserted by the Ruby structural check (`pins`). The tag lives only in a comment. |
| T-05-11 | Elevation of Privilege | Workflow `GITHUB_TOKEN` scope | medium | mitigate | `permissions: contents: read` at the top level, asserted structurally (`permissions`). |
| T-05-12 | Information Disclosure | ADO credentials exposed to CI or to fork PRs | high | mitigate | No `secrets`, no `ADO_*` and no `env` in the workflow (D-17), asserted structurally (`credential reference`). The suite is proven credential-free by Task 2. |
| T-05-13 | Tampering | Fork PR runs untrusted test code on the runner | low | accept | The `pull_request` event runs fork code with a read-only token and no secrets, and no secret exists to take. |
| T-05-SC | Tampering | npm/pip/cargo installs | high | mitigate | Not applicable. No package install occurs. The two actions are official `actions/*` repositories, pinned by SHA. |
</threat_model>

<verification>
- The Ruby structural check prints `ci.yml structure ok`.
- The gofmt-step failure-direction proof and the credential-free suite both pass.
- End of phase: the Task 2 human-check (a green `ci` run on the pushed phase branch) is harvested into UAT. It is PHASE-BLOCKING, because it is the 05-VALIDATION.md phase gate "one green GitHub Actions run on the phase branch". Run it after the wave-2 plans have landed. Phase 5 is not complete until it passes.
</verification>

<success_criteria>
- ROADMAP Phase 5 SC5 holds: CI runs build, vet, race tests and gofmt on every push and fails on regression.
- The 05-VALIDATION.md HARD-05 row is green locally. Its GitHub half is queued as the phase-blocking Task 2 human-check, and it must pass before Phase 5 is marked complete.
</success_criteria>

<output>
Create `.planning/phases/05-hardening/05-03-SUMMARY.md` when done
</output>
