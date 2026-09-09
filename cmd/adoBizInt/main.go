package main

import (
	"context"
	"fmt"
	"os"

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
	os.Exit(run())
}

// initLogging brings up file logging. A failure is deliberately non-fatal and
// deliberately not skipped: the crash record references logging.LogPath(), so a
// skipped init would leave that path dangling (D-24).
func initLogging() {
	if err := logging.Init(); err != nil {
		fmt.Fprintf(stderrWriter, "Warning: Could not initialize logging: %v\n", err)
	}
}

// run performs the whole program and returns the process exit code. Startup
// failures return exitUnknown rather than exitPartial (D-08): the read never
// happened, so its outcome is unknown rather than incomplete.
func run() int {
	initLogging()
	log := logging.Logger()
	log.Info("Starting adoBizInt")

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
		return dispatch(context.Background(), client, os.Args[1:])
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
