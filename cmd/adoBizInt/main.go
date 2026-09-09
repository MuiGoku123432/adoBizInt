package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/config"
	"sentinovo.ai/bizInt/internal/logging"
	"sentinovo.ai/bizInt/internal/ui"
)

// main holds the only process-exit call site in this package. Every failure
// path is a return out of run(), because exiting the process directly skips
// deferred functions and the panic handler this entry point will carry is a
// defer inside run().
func main() {
	os.Exit(guard(run))
}

// guard runs the whole program with a deferred recover, so a panic anywhere
// inside it becomes a structured crash record and the outcome-unknown exit code
// rather than the runtime's own dump and status (D-23, OUT-05).
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
			code = handlePanic(r)
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
// failures return exitUnknown rather than exitPartial (D-08): the read never
// happened, so its outcome is unknown rather than incomplete.
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
		fmt.Fprintf(stderrWriter, "\nPlease set ADO_PAT and ADO_ORG_URL environment variables\n")
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

	client, err := ado.NewClient(cfg)
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

		return dispatch(signalCtx, client, os.Args[1:])
	}

	model := ui.NewModel(client, cfg.OrgURL, cfg.Projects, cfg.Repositories, cfg.StateTransitions, cfg.PollInterval, priorities)
	p := tea.NewProgram(model, tea.WithAltScreen())

	log.Info("Starting TUI")
	if _, err := p.Run(); err != nil {
		log.Error("TUI error", "error", err)
		fmt.Fprintf(stderrWriter, "Error running app: %v\n", err)
		// Unchanged from the pre-CLI behavior: CLI-01 forbids altering the TUI path.
		return 1
	}
	log.Info("adoBizInt exited")
	return exitOK
}
