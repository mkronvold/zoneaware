package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"zoneaware/internal/config"
	"zoneaware/internal/layout"
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
	printZellijPaneSize := flag.Bool("print-zellij-pane-size", false, "Print the width and height needed for a 24-hour Zellij floating pane")
	paneHours := flag.Int("pane-hours", 24, "Hour width to use when printing the Zellij pane size")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	if *printZellijPaneSize {
		width, height := layout.ZellijPaneSize(cfg, time.Now(), *paneHours)
		fmt.Printf("%d %d\n", width, height)
		return nil
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
