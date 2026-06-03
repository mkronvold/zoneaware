package config

import (
	"fmt"
	"slices"
	"time"
)

const quarterMinutes = 15

func (cfg *Config) ToggleMemberHour(name string, slotStart time.Time) error {
	for i := range cfg.Team {
		if cfg.Team[i].Name == name {
			return cfg.Team[i].ToggleHour(slotStart)
		}
	}

	return fmt.Errorf("team member %q not found", name)
}

func (member *TeamMember) ToggleHour(slotStart time.Time) error {
	return member.toggleInterval(slotStart, time.Hour)
}

func (member *TeamMember) ToggleHalfHour(slotStart time.Time, halfIndex int) error {
	if halfIndex < 0 || halfIndex > 1 {
		return fmt.Errorf("halfIndex must be 0 or 1, got %d", halfIndex)
	}

	return member.toggleInterval(slotStart.Add(time.Duration(halfIndex)*30*time.Minute), 30*time.Minute)
}

func (member *TeamMember) toggleInterval(intervalStart time.Time, duration time.Duration) error {
	location, err := time.LoadLocation(member.Timezone)
	if err != nil {
		return fmt.Errorf("load timezone: %w", err)
	}

	slots, err := QuarterHourSlots(member.WorkingHours)
	if err != nil {
		return err
	}

	quarterIndexes := intervalQuarterIndexes(intervalStart, duration, location)
	allEnabled := true
	for _, index := range quarterIndexes {
		if !slots[index] {
			allEnabled = false
			break
		}
	}

	for _, index := range quarterIndexes {
		slots[index] = !allEnabled
	}

	member.WorkingHours = WorkingHoursFromQuarterSlots(slots)

	return nil
}

func HourlySlots(ranges []WorkingHours) ([24]bool, error) {
	quarterSlots, err := QuarterHourSlots(ranges)
	if err != nil {
		return [24]bool{}, err
	}

	var slots [24]bool
	for hour := 0; hour < 24; hour++ {
		for quarter := 0; quarter < 4; quarter++ {
			if quarterSlots[hour*4+quarter] {
				slots[hour] = true
				break
			}
		}
	}

	return slots, nil
}

func QuarterHourSlots(ranges []WorkingHours) ([96]bool, error) {
	var slots [96]bool

	for _, workRange := range ranges {
		start, err := ParseClock(workRange.Start)
		if err != nil {
			return slots, err
		}

		end, err := ParseClock(workRange.End)
		if err != nil {
			return slots, err
		}

		if start == 24*60 {
			start = 0
		}
		if end == 24*60 {
			end = 0
		}

		for quarter := 0; quarter < len(slots); quarter++ {
			minuteOfDay := quarter * quarterMinutes
			if containsMinute(start, end, minuteOfDay) {
				slots[quarter] = true
			}
		}
	}

	return slots, nil
}

func WorkingHoursFromHourlySlots(slots [24]bool) []WorkingHours {
	var quarterSlots [96]bool
	for hour, enabled := range slots {
		if !enabled {
			continue
		}
		for quarter := 0; quarter < 4; quarter++ {
			quarterSlots[hour*4+quarter] = true
		}
	}

	return WorkingHoursFromQuarterSlots(quarterSlots)
}

func WorkingHoursFromQuarterSlots(slots [96]bool) []WorkingHours {
	if slices.Index(slots[:], true) == -1 {
		return nil
	}

	if allSlotsEnabled(slots[:]) {
		return []WorkingHours{{Start: "00:00", End: "24:00"}}
	}

	ranges := make([]WorkingHours, 0)
	for quarter := 0; quarter < len(slots); quarter++ {
		if !slots[quarter] || slots[(quarter+len(slots)-1)%len(slots)] {
			continue
		}

		end := (quarter + 1) % len(slots)
		for end != quarter && slots[end] {
			end = (end + 1) % len(slots)
		}

		ranges = append(ranges, WorkingHours{
			Start: formatMinute(quarter * quarterMinutes),
			End:   formatMinute(end * quarterMinutes),
		})
	}

	return ranges
}

func allSlotsEnabled(slots []bool) bool {
	for _, enabled := range slots {
		if !enabled {
			return false
		}
	}

	return true
}

func formatHour(hour int) string {
	return fmt.Sprintf("%02d:00", hour)
}

func formatMinute(minute int) string {
	hour := (minute / 60) % 24
	minute = minute % 60
	return fmt.Sprintf("%02d:%02d", hour, minute)
}

func containsMinute(start, end, minuteOfDay int) bool {
	if start < end {
		return minuteOfDay >= start && minuteOfDay < end
	}

	return minuteOfDay >= start || minuteOfDay < end
}

func intervalQuarterIndexes(start time.Time, duration time.Duration, location *time.Location) []int {
	steps := int(duration / (15 * time.Minute))
	if steps < 1 {
		steps = 1
	}

	indexes := make([]int, 0, steps)
	seen := make(map[int]bool)
	for sample := 0; sample < steps; sample++ {
		local := start.Add(time.Duration(sample) * 15 * time.Minute).In(location)
		index := (local.Hour()*60 + local.Minute()) / quarterMinutes
		if seen[index] {
			continue
		}
		seen[index] = true
		indexes = append(indexes, index)
	}

	return indexes
}
