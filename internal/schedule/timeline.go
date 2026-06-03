package schedule

import (
	"fmt"
	"time"

	"zoneaware/internal/config"
)

type Window struct {
	ReferenceTimezone string
	Start             time.Time
	Hours             int
	Members           []MemberTimeline
}

type MemberTimeline struct {
	Name     string
	Timezone string
	Cells    []Cell
}

type Cell struct {
	Start     time.Time
	Available bool
}

type workingRange struct {
	start int
	end   int
}

func Build(cfg config.Config, now time.Time, hours int) (Window, error) {
	windowHours := config.EffectiveWindowHours(cfg)
	if hours < 1 {
		hours = 1
	}
	if hours > windowHours {
		hours = windowHours
	}

	referenceLocation, err := time.LoadLocation(cfg.ReferenceTimezone)
	if err != nil {
		return Window{}, fmt.Errorf("load reference timezone: %w", err)
	}

	start := now.In(referenceLocation).Truncate(time.Hour)
	window := Window{
		ReferenceTimezone: cfg.ReferenceTimezone,
		Start:             start,
		Hours:             hours,
		Members:           make([]MemberTimeline, 0, len(cfg.Team)),
	}

	for _, member := range cfg.Team {
		location, err := time.LoadLocation(member.Timezone)
		if err != nil {
			return Window{}, fmt.Errorf("load timezone for %s: %w", member.Name, err)
		}

		ranges, err := parseWorkingRanges(member.WorkingHours)
		if err != nil {
			return Window{}, fmt.Errorf("parse working hours for %s: %w", member.Name, err)
		}

		timeline := MemberTimeline{
			Name:     member.Name,
			Timezone: member.Timezone,
			Cells:    make([]Cell, hours),
		}

		for i := 0; i < hours; i++ {
			slotStart := start.Add(time.Duration(i) * time.Hour)
			timeline.Cells[i] = Cell{
				Start:     slotStart,
				Available: slotIsAvailable(slotStart, location, ranges),
			}
		}

		window.Members = append(window.Members, timeline)
	}

	return window, nil
}

func parseWorkingRanges(input []config.WorkingHours) ([]workingRange, error) {
	ranges := make([]workingRange, 0, len(input))
	for _, entry := range input {
		start, err := config.ParseClock(entry.Start)
		if err != nil {
			return nil, err
		}

		end, err := config.ParseClock(entry.End)
		if err != nil {
			return nil, err
		}

		if start == 24*60 {
			start = 0
		}
		if end == 24*60 {
			end = 0
		}

		ranges = append(ranges, workingRange{start: start, end: end})
	}

	return ranges, nil
}

func slotIsAvailable(slotStart time.Time, location *time.Location, ranges []workingRange) bool {
	for sample := 0; sample < 4; sample++ {
		local := slotStart.Add(time.Duration(sample) * 15 * time.Minute).In(location)
		minuteOfDay := local.Hour()*60 + local.Minute()
		for _, workRange := range ranges {
			if containsMinute(workRange, minuteOfDay) {
				return true
			}
		}
	}

	return false
}

func containsMinute(workRange workingRange, minuteOfDay int) bool {
	if workRange.start < workRange.end {
		return minuteOfDay >= workRange.start && minuteOfDay < workRange.end
	}

	return minuteOfDay >= workRange.start || minuteOfDay < workRange.end
}
