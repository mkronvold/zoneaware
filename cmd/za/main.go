package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"zoneaware/internal/config"
	"zoneaware/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "zoneaware: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", config.DefaultPath(), "Path to zoneaware config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	program := tea.NewProgram(
		ui.NewModel(cfg, *configPath, time.Now),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := program.Run(); err != nil {
		return err
	}

	return nil
}
