package layout

import (
	"time"

	"github.com/charmbracelet/lipgloss"

	"zoneaware/internal/config"
)

const (
	minNameWidth  = 8
	baseFrameRows = 8
	cellWidth     = 3
)

func ZellijPaneSize(cfg config.Config, now time.Time, hours int) (width int, height int) {
	nameWidth := minNameWidth
	for _, member := range cfg.Team {
		nameWidth = max(nameWidth, lipgloss.Width(member.Name))
	}

	labelWidth := 0
	seenZones := make(map[string]bool)
	for _, member := range cfg.Team {
		labelWidth = max(labelWidth, lipgloss.Width(zoneLabel(member.Timezone, now)))
		if !seenZones[member.Timezone] {
			seenZones[member.Timezone] = true
		}
	}

	width = nameWidth + 1 + labelWidth + 1 + hours*cellWidth
	height = baseFrameRows + len(cfg.Team) + len(seenZones)
	return width, height
}

func zoneLabel(timezone string, now time.Time) string {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return timezone
	}

	label := now.In(location).Format("MST")
	if label == "" {
		return timezone
	}
	return label
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
