package layout

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"

	"zoneaware/internal/config"
)

const (
	minNameWidth    = 8
	baseFrameRows   = 8
	cellWidth       = 3
	zellijFrameRows = 2
	zellijFrameCols = 2
	hoursLabel      = "hrs"
)

func ZellijPaneSize(cfg config.Config, now time.Time, hours int) (width int, height int) {
	nameWidth := minNameWidth
	for _, member := range cfg.Team {
		nameWidth = max(nameWidth, lipgloss.Width(member.Name))
	}

	labelWidth := 0
	hoursWidth := len(hoursLabel)
	seenZones := make(map[string]bool)
	for _, member := range cfg.Team {
		labelWidth = max(labelWidth, lipgloss.Width(zoneLabel(member.Timezone, now)))
		hoursWidth = max(hoursWidth, lipgloss.Width(memberScheduleHoursLabel(member)))
		if !seenZones[member.Timezone] {
			seenZones[member.Timezone] = true
		}
	}

	rightWidth := labelWidth
	if hoursWidth > 0 {
		if rightWidth > 0 {
			rightWidth++
		}
		rightWidth += hoursWidth
	}

	width = nameWidth + 1 + hours*cellWidth + 1 + rightWidth + zellijFrameCols
	height = baseFrameRows + len(cfg.Team) + len(seenZones) + 1 + zellijFrameRows
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

func memberScheduleHoursLabel(member config.TeamMember) string {
	slots, err := config.QuarterHourSlots(member.WorkingHours)
	if err != nil {
		return "?h"
	}

	quarters := 0
	for _, enabled := range slots {
		if enabled {
			quarters++
		}
	}

	wholeHours := quarters / 4
	switch quarters % 4 {
	case 0:
		return formatHoursLabel(wholeHours, "")
	case 1:
		return formatHoursLabel(wholeHours, ".25")
	case 2:
		return formatHoursLabel(wholeHours, ".5")
	default:
		return formatHoursLabel(wholeHours, ".75")
	}
}

func formatHoursLabel(wholeHours int, suffix string) string {
	return fmt.Sprintf("%d%sh", wholeHours, suffix)
}
