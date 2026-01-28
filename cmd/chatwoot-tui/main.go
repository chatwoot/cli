package main

import (
	"fmt"
	"os"

	"github.com/chatwoot/chatwoot-cli/internal/config"
	"github.com/chatwoot/chatwoot-cli/internal/sdk"
	"github.com/chatwoot/chatwoot-cli/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Try to load existing config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// If no config or invalid, run setup
	if cfg == nil || !cfg.IsValid() {
		cfg, err = runSetup()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
			os.Exit(1)
		}
		if cfg == nil {
			// User quit setup
			os.Exit(0)
		}
	}

	// Create client and run main TUI
	client := sdk.NewClient(cfg.BaseURL, cfg.APIKey, cfg.AccountID)
	model := tui.NewModel(client)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func runSetup() (*config.Config, error) {
	setupModel := tui.NewSetupModel()
	p := tea.NewProgram(setupModel, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	setup, ok := finalModel.(*tui.SetupModel)
	if !ok || !setup.Done() {
		return nil, nil
	}

	return setup.Config(), nil
}
