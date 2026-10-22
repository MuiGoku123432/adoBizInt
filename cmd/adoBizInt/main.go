package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/config"
	"sentinovo.ai/bizInt/internal/logging"
	"sentinovo.ai/bizInt/internal/transport"
	"sentinovo.ai/bizInt/internal/ui"
	"sentinovo.ai/bizInt/internal/ui/confirm"
)

// main holds the only process-exit call site in this package. Every failure
// path is a return out of run(), because exiting the process directly skips
// deferred functions and the panic handler this entry point will carry is a
// defer inside run().
func main() {
	os.Exit(guard(run))
}

// guard runs the whole program with a deferred recover, so a panic anywhere
// inside it becomes a structured crash record and exit 3 (nothing ran, D-08)
// rather than the runtime's own dump and status (D-23, OUT-05). A panic inside a
// write after its request was issued never reaches guard: runWrite recovers it
// first and exits 5 (D-12).
//
// It wraps run rather than living inside it so a test can drive a real panic
// through the real handler; run itself loads configuration and constructs a
// live client, which no test can reach. The coverage is identical either way:
// every path in run is inside this call. It works at all only because 03-01
// moved every exit path to a return -- a surviving process exit inside run
// would skip this deferred function for that path.
func guard(body func() int) (code int) {
	defer func() {
		if r := recover(); r != nil {
			code = handlePanic(r, exitUnknown)
		}
	}()
	return body()
}

// initLogging brings up file logging. A failure is deliberately non-fatal and
// deliberately not skipped: the crash record references the log path, so a
// skipped init would leave that path dangling (D-24).
//
// The outcome is recorded because the panic handler needs it. When init failed,
// the package logger falls back to the default one, which writes to stderr --
// so writing a stack unconditionally would print credential-bearing frames to
// the terminal.
func initLogging() {
	if err := logging.Init(); err != nil {
		loggingReady = false
		fmt.Fprintf(stderrWriter, "Warning: Could not initialize logging: %v\n", err)
		return
	}
	loggingReady = true
}

// run performs the whole program and returns the process exit code. Startup
// failures return exitUnknown rather than exitPartial (D-08): nothing ran, so
// there is no partial result to report.
func run() int {
	initLogging()
	log := logging.Logger()
	log.Info("Starting adoBizInt")

	// Usage resolution runs before config is touched. cfg.Validate aborts when
	// PAT is empty (config.go:151), so `adoBizInt --help` failing for lack of a
	// credential would be hostile — and it is the first thing a new user runs.
	if code, handled := handleUsage(os.Args[1:]); handled {
		return code
	}

	cfg, err := config.Load()
	if err != nil {
		log.Error("Failed to load config", "error", err)
		fmt.Fprintf(stderrWriter, "Error loading config: %v\n", err)
		return exitUnknown
	}

	// Log config (redact PAT)
	log.Info("Config loaded",
		"org_url", cfg.OrgURL,
		"projects", cfg.Projects,
		"projects_count", len(cfg.Projects),
	)

	if err := cfg.Validate(); err != nil {
		log.Error("Config validation failed", "error", err)
		fmt.Fprintf(stderrWriter, "Invalid config: %v\n", err)
		fmt.Fprintf(stderrWriter, "\nPlease set ADO_PAT, ADO_ORG_URL and ADO_PROJECTS environment variables\n")
		fmt.Fprintf(stderrWriter, "or create a config file at ~/.config/adoBizInt/config.yaml\n")
		return exitUnknown
	}

	// The fan-out budget is captured only now, after validation has passed
	// (D-P6). A --timeout given on the command line replaces it per invocation,
	// downstream of the rule at config validation that rejects a fetch budget at
	// or below the poll interval — that rule protects a refreshing TUI and has
	// no meaning for a one-shot read, so a CLI flag must not be measured
	// against it.
	readBudget = cfg.FetchTimeout
	configuredProjects = []string(cfg.Projects) // D-09: the --project allow-list, trusted only after validation
	scrumConfig = cfg                           // read only by scrum verbs, which run ValidateScrum first (D-2)

	// D-09, D-11: every SDK request goes through this one transport, which binds
	// the PAT to the configured organization and refuses any other host.
	tr := transport.New(cfg.Org(), cfg.RequestTimeout, http.DefaultTransport)
	client, err := ado.NewClient(cfg, tr)
	if err != nil {
		log.Error("Failed to create ADO client", "error", err)
		fmt.Fprintf(stderrWriter, "Error connecting to Azure DevOps: %v\n", err)
		return exitUnknown
	}
	log.Info("ADO client created successfully")

	// Load priorities for work item ordering
	priorities, err := config.LoadPriorities()
	if err != nil {
		log.Warn("Failed to load priorities, using empty list", "error", err)
	}

	// The CLI branch lands before ui.NewModel: that constructor eagerly builds
	// six sub-models and six bubble-table instances, so branching after it would
	// pay full TUI construction for a non-interactive read. Zero arguments falls
	// through to the TUI path unchanged (CLI-01).
	if len(os.Args) > 1 {
		// The read runs under a signal-cancelled parent so an interrupt aborts
		// it in flight rather than after the fan-out completes. NotifyContext
		// owns the goroutine, the cleanup and the repeat-signal case; a
		// hand-rolled Notify plus a channel would reimplement all three and get
		// the cleanup wrong on the paths that return early (D-25).
		signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stopSignals()

		// D-05, D-06: one throttle line on stderr once the command has run,
		// when the transport paced it or a read was retried. It prints on a
		// failed run too: the simplest rule, and never misleading (RESEARCH
		// Open Question 9). The TUI path below never reaches it (D-07).
		code := dispatch(signalCtx, client, os.Args[1:])
		waits, waited := tr.Stats()
		renderThrottleSummary(stderrWriter, waits, waited, client.FetchRetries())
		return code
	}

	model := ui.NewModel(client, cfg.OrgURL, cfg.Projects, cfg.Repositories, cfg.StateTransitions, cfg.PollInterval, priorities)
	p := tea.NewProgram(model, tea.WithAltScreen())

	// CR-01. Bubbletea turns SIGTERM, and SIGINT when stdin is not a TTY, into a
	// quit that its event loop handles before Model.Update, so the 07-08 quit
	// hold never sees it, and Program.Run does not wait for command goroutines.
	// Every TUI guarded write therefore runs under this signal context, so the
	// guard classifies an interrupted write unknown and audits it (Phase 4
	// D-22), as the CLI does, and confirm.Wait holds the exit until that record
	// is appended. The deferred stop keeps both signals captured through the
	// wait, so a second SIGTERM cannot kill the process mid-append. The context
	// is deliberately not handed to the bubbletea program: its signal goroutine
	// makes a blocking send that a cancelled event loop never receives, which
	// would hang shutdown with the terminal still raw.
	tuiSignalCtx, stopTUISignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopTUISignals()
	confirm.SetParent(tuiSignalCtx)

	log.Info("Starting TUI")
	_, err = p.Run()
	// Before the error branch: Run returns an error after a panic it recovered,
	// which is exactly when another goroutine may be mid-write.
	confirm.Wait()
	if err != nil {
		log.Error("TUI error", "error", err)
		fmt.Fprintf(stderrWriter, "Error running app: %v\n", err)
		// Unchanged from the pre-CLI behavior: CLI-01 forbids altering the TUI path.
		return 1
	}
	log.Info("adoBizInt exited")
	return exitOK
}
