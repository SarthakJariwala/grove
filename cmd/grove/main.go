package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SarthakJariwala/grove/internal/configfile"
	"github.com/SarthakJariwala/grove/internal/tmux"
	"github.com/SarthakJariwala/grove/internal/ui"
)

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.toml"
	}

	return filepath.Join(home, ".config", "grove", "config.toml")
}

func run() error {
	configPath := flag.String("config", defaultConfigPath(), "path to config.toml")
	flag.Parse()

	if err := configfile.EnsureTemplate(*configPath); err != nil {
		return fmt.Errorf("could not initialize config template: %w", err)
	}

	cfg, err := configfile.Load(*configPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	client := tmux.NewClient()
	model := ui.NewModel(cfg, *configPath, client)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("program error: %w", err)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
