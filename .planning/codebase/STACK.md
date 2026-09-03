# Technology Stack

**Analysis Date:** 2026-09-03

## Languages

**Primary:**
- Go 1.25.4 (declared) - 100% of the codebase. 31 `.go` files, 7,601 lines total across `cmd/` and `internal/`. Declared at `go.mod:3`.

**Secondary:**
- YAML - configuration format only, not a build language. Parsed at `internal/config/config.go:119` and `internal/config/priorities.go:39`.

There is no second implementation language. No shell scripts, no embedded templates, no SQL, no JS/CSS.

## Runtime

**Environment:**
- Go standard runtime, compiled to a single static binary. No VM, no interpreter, no container runtime required.
- `go.mod` declares `go 1.25.4` (`go.mod:3`). The toolchain installed on this machine is `go1.27.1 darwin/arm64`. The declared version is the floor, not a pin -- there is no `toolchain` directive in `go.mod`, so any Go >= 1.25.4 will build it.
- The binary is a terminal application. It requires an interactive TTY: `main.go:60` starts Bubbletea with `tea.WithAltScreen()`, which takes over the terminal alternate screen buffer. It will not work correctly when piped or run headless.

**Package Manager:**
- Go modules. Module path `sentinovo.ai/bizInt` (`go.mod:1`).
- Lockfile: present. `go.sum` (7,922 bytes) pins all module hashes.
- No vendor directory. Dependencies resolve from the module cache/proxy at build time.

## Frameworks

**Core:**

| Framework | Version | Purpose | Evidence |
|-----------|---------|---------|----------|
| `github.com/charmbracelet/bubbletea` | v1.3.10 | The Elm-architecture TUI runtime. Owns the entire application event loop. Every UI package implements `Init/Update/View`. | `go.mod:7`, `cmd/adoBizInt/main.go:60` |
| `github.com/charmbracelet/bubbles` | v0.21.0 | Prebuilt Bubbletea components. Four subpackages used: `help`, `key`, `textinput`, `viewport`. | `go.mod:6` |
| `github.com/charmbracelet/lipgloss` | v1.1.0 | Terminal styling and layout primitives (borders, padding, colors, joins). Centralized in `internal/ui/styles/styles.go`. | `go.mod:8` |
| `github.com/evertras/bubble-table` | v0.19.2 | Scrollable/sortable data table widget. This is the primary rendering surface for all five list views. | `go.mod:9` |
| `github.com/rmhubbert/bubbletea-overlay` | v0.5.3 | Composites a foreground view over a background view to produce modal dialogs. Used in exactly one place. | `go.mod:11`, `internal/ui/modal/modal.go:7,22` |

`bubble-table` is imported by five view packages: `internal/ui/releases/releases.go`, `internal/ui/prs/prs.go`, `internal/ui/workitems/workitems.go`, `internal/ui/pipelines/pipelines.go`, `internal/ui/branches/branches.go`.

**Testing:**
- Go standard library `testing` only. No assertion library, no mocking framework, no test-container tooling.
- Two test files exist out of 31 Go files: `internal/ui/releases/releases_test.go` and `internal/utils/text_test.go`. The entire `internal/ado/` service layer is untested.

**Build/Dev:**
- The Go toolchain, unassisted. There is no Makefile, no Taskfile, no `.github/` directory, no Dockerfile, no `golangci-lint` config, and no `air`/`reflex` hot-reload setup.

## Key Dependencies

**Critical:**

| Package | Version | Why it matters |
|---------|---------|----------------|
| `github.com/microsoft/azure-devops-go-api/azuredevops` | v1.0.0-b5 | The only external-system client in the codebase. Every network call except one goes through it. Note the version is a **beta 5 pre-release**, not a stable v1. | `go.mod:10` |
| `gopkg.in/yaml.v3` | v3.0.1 | Parses both config files. `internal/config/config.go` also implements a custom `UnmarshalYAML` on `StringSlice` (`internal/config/config.go:15`) so `projects` accepts either a YAML list or a comma-separated scalar. | `go.mod:12` |

**Infrastructure:**

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/atotto/clipboard` | v0.1.4 | System clipboard write, used by `utils.CopyToClipboard` (`internal/utils/links.go:9,45`). |
| `github.com/google/uuid` | v1.1.1 | Transitive via the ADO SDK. Repository IDs surface as `uuid.UUID` and are stringified at `internal/ado/branches.go:65`. |

**go.mod hygiene note:** `github.com/atotto/clipboard` is listed in the indirect block (`go.mod:16`) but is imported directly by `internal/utils/links.go:9`. It arrived transitively via Bubbletea and was never promoted. `go mod tidy` would move it into the direct `require` block. This is cosmetic and does not affect the build.

## Configuration

**Mechanism:**

Two-layer load in `config.Load()` (`internal/config/config.go:80`):

1. File: `~/.config/adoBizInt/config.yaml`, read at `internal/config/config.go:113-119`. A missing file is not an error -- `os.IsNotExist` is swallowed at `internal/config/config.go:87`.
2. Environment: `loadFromEnv()` at `internal/config/config.go:122`. Env vars overwrite anything the file set.

**Environment variables (all four are the complete set):**

| Variable | Maps to | Notes |
|----------|---------|-------|
| `ADO_PAT` | `Config.PAT` | `internal/config/config.go:123` |
| `ADO_ORG_URL` | `Config.OrgURL` | `internal/config/config.go:126` |
| `ADO_PROJECTS` | `Config.Projects` | Comma-separated. Clears any file-supplied list first (`internal/config/config.go:131`). |
| `ADO_PROJECT` | `Config.Project` | Backwards-compat single project. Folded into `Projects` at `internal/config/config.go:95-97` only if `Projects` is empty. |

**File-only settings (no env override exists):** `pipelines`, `release_definitions`, `repositories`, `poll_interval`, `state_transitions`, `user_email`. See the struct tags at `internal/config/config.go:38-49`.

**Defaults:**
- `PollInterval` defaults to 30 seconds (`internal/config/config.go:82`). Each view re-defaults to 30s independently if the value is <= 0 (`internal/ui/prs/prs.go:118`, `internal/ui/pipelines/pipelines.go:132`, `internal/ui/releases/releases.go:133`).
- `StateTransitions` defaults to `DefaultStateTransitions` (`internal/config/config.go:53-78`) covering User Story, Task, Bug, Feature, Epic.

**Validation:** `Config.Validate()` (`internal/config/config.go:144`) checks only two fields -- `PAT` and `OrgURL`. `Projects` may legitimately be empty; downstream code degrades to empty result sets rather than erroring.

**Secondary state file:** `~/.config/adoBizInt/priorities.yaml` holds a user-ordered list of work item IDs (`internal/config/priorities.go:30`). It is the only file the application *writes* (`internal/config/priorities.go:70`, mode `0644`, directory `0755`). A dirty-flag guard at `internal/config/priorities.go:49` skips the write when nothing changed.

## Logging

- Standard library `log/slog`, wrapped in `internal/logging/logging.go` (49 lines).
- File-only sink: `~/.config/adoBizInt/adoBizInt.log`, opened append-mode at `internal/logging/logging.go:24`.
- `slog.NewTextHandler` at `LevelDebug` (`internal/logging/logging.go:29-31`). Debug is unconditional -- there is no level knob in config or env.
- File output rather than stderr is a deliberate TUI constraint: stdout/stderr belong to the alternate screen buffer, so console logging would corrupt the display.
- `Logger()` falls back to `slog.Default()` if `Init()` failed (`internal/logging/logging.go:38-41`), which would write to stderr and garble the TUI. This is the failure mode, not the intent.
- No log rotation, size cap, or retention. The file grows without bound.

## Build and Run

There is no build tooling wrapper. Use the toolchain directly from the repo root:

```bash
go build ./cmd/adoBizInt        # produces ./adoBizInt (gitignored, .gitignore:2)
go run ./cmd/adoBizInt          # build and launch the TUI
go test ./...                   # runs the two existing test files
go build ./...                  # compile check, currently passes
go vet ./...                    # not currently wired into any gate
```

A stale 13 MB compiled `adoBizInt` binary sits at the repo root. It is gitignored (`.gitignore:2`) but present on disk.

## Platform Requirements

**Development:**
- Go >= 1.25.4.
- Network egress to the Azure DevOps organization URL over HTTPS.
- A valid Azure DevOps Personal Access Token.

**Runtime:**
- An interactive terminal with alternate-screen support and 256-color/truecolor for the lipgloss styling to render as designed.
- A writable `~/.config/adoBizInt/` directory for logs and priorities.
- Platform-specific browser-open support: `open` on darwin, `xdg-open` on linux, `cmd /c start` on windows (`internal/utils/links.go:51-59`). Any other GOOS returns an explicit unsupported-OS error at `internal/utils/links.go:59`.
- Cross-compiles cleanly in principle -- there is no cgo and no build-tagged platform code. Verified as darwin/arm64 only.

**Production:**
- Not applicable. This is a locally-run developer tool, not a deployed service. There is no deployment target, no service manifest, and no release pipeline in the repo.

## Notable Absences

These are deliberate scope boundaries, and each one narrows what a future phase can assume:

| Absent | Consequence |
|--------|-------------|
| **CLI framework** (cobra, urfave/cli, flag) | `cmd/adoBizInt/main.go` is 69 lines and parses zero arguments. `main()` goes straight from config load to `tea.NewProgram(...).Run()`. Adding any subcommand, `--version`, or `--config path` flag means introducing arg parsing from scratch. |
| **HTTP server** | Nothing listens. No `net/http.Server`, no router, no middleware. The only `net/http` import is outbound (`internal/ado/workitems.go:7`). |
| **Database** | No SQL driver, no ORM, no embedded KV store. All state is either in-memory for the session or in the two YAML files. |
| **Configuration library** (viper, koanf, envconfig) | Config loading is ~60 hand-written lines. Layering, defaults, and env binding are all manual. |
| **HTTP client tuning** | No custom `http.Client`, no transport config, no connection pool sizing, no timeouts. `internal/ado/workitems.go:528` uses `http.DefaultClient` bare. |
| **Retry / backoff / rate limiting** | Zero matches for retry, backoff, ratelimit, or 429 handling across `internal/`. See INTEGRATIONS.md. |
| **Concurrency primitives** | Zero matches for `go func`, `sync.`, or `errgroup` in `internal/`. All multi-project fetches are sequential `for` loops. |
| **CI/CD** | No `.github/`, no `.gitlab-ci.yml`, no Jenkinsfile. Nothing enforces build, vet, test, or lint. |
| **Linter config** | No `.golangci.yml`. Formatting and style are unenforced. |
| **Containerization** | No Dockerfile, no compose file. |
| **README or docs** | No markdown at the repo root. A 7 MB `systemDesign.pdf` sits at the root, unreferenced by any code. |
| **Dependency injection** | `ado.Client` is a concrete struct passed by pointer into every view constructor (`cmd/adoBizInt/main.go:59`). There is no interface seam, which is why `internal/ado/` has no tests. |

---

*Stack analysis: 2026-09-03*
