package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"sentinovo.ai/bizInt/internal/ado"
	"sentinovo.ai/bizInt/internal/config"
	"sentinovo.ai/bizInt/internal/logging"
	"sentinovo.ai/bizInt/internal/ui"
)

func main() {
	// Initialize logging first
	if err := logging.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not initialize logging: %v\n", err)
	}
	log := logging.Logger()
	log.Info("Starting adoBizInt")

	cfg, err := config.Load()
	if err != nil {
		log.Error("Failed to load config", "error", err)
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Log config (redact PAT)
	log.Info("Config loaded",
		"org_url", cfg.OrgURL,
		"projects", cfg.Projects,
		"projects_count", len(cfg.Projects),
	)

	if err := cfg.Validate(); err != nil {
		log.Error("Config validation failed", "error", err)
		fmt.Fprintf(os.Stderr, "Invalid config: %v\n", err)
		fmt.Fprintln(os.Stderr, "\nPlease set ADO_PAT and ADO_ORG_URL environment variables")
		fmt.Fprintln(os.Stderr, "or create a config file at ~/.config/adoBizInt/config.yaml")
		os.Exit(1)
	}

	client, err := ado.NewClient(cfg)
	if err != nil {
		log.Error("Failed to create ADO client", "error", err)
		fmt.Fprintf(os.Stderr, "Error connecting to Azure DevOps: %v\n", err)
		os.Exit(1)
	}
	log.Info("ADO client created successfully")

	model := ui.NewModel(client, cfg.OrgURL, cfg.Projects, cfg.StateTransitions)
	p := tea.NewProgram(model, tea.WithAltScreen())

	log.Info("Starting TUI")
	if _, err := p.Run(); err != nil {
		log.Error("TUI error", "error", err)
		fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
		os.Exit(1)
	}
	log.Info("adoBizInt exited")
}
