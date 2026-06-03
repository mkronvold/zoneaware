package config

import (
	"fmt"
	"slices"
	"time"
)

func (cfg *Config) ToggleMemberHour(name string, slotStart time.Time) error {
	for i := range cfg.Team {
		if cfg.Team[i].Name == name {
			return cfg.Team[i].ToggleHour(slotStart)
		}
	}

	return fmt.Errorf("team member %q not found", name)
}

func (member *TeamMember) ToggleHour(slotStart time.Time) error {
	location, err := time.LoadLocation(member.Timezone)
	if err != nil {
		return fmt.Errorf("load timezone: %w", err)
	}

	slots, err := HourlySlots(member.WorkingHours)
	if err != nil {
		return err
	}

	localHour := slotStart.In(location).Hour()
	slots[localHour] = !slots[localHour]
	member.WorkingHours = WorkingHoursFromHourlySlots(slots)

	return nil
}

func HourlySlots(ranges []WorkingHours) ([24]bool, error) {
	var slots [24]bool

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

		for hour := 0; hour < 24; hour++ {
			for sample := 0; sample < 4; sample++ {
				if containsMinute(start, end, hour*60+sample*15) {
					slots[hour] = true
					break
				}
			}
		}
	}

	return slots, nil
}

func WorkingHoursFromHourlySlots(slots [24]bool) []WorkingHours {
	if slices.Index(slots[:], true) == -1 {
		return nil
	}

	if allHoursEnabled(slots) {
		return []WorkingHours{{Start: "00:00", End: "24:00"}}
	}

	ranges := make([]WorkingHours, 0)
	for hour := 0; hour < 24; hour++ {
		if !slots[hour] || slots[(hour+23)%24] {
			continue
		}

		end := (hour + 1) % 24
		for end != hour && slots[end] {
			end = (end + 1) % 24
		}

		ranges = append(ranges, WorkingHours{
			Start: formatHour(hour),
			End:   formatHour(end),
		})
	}

	return ranges
}

func allHoursEnabled(slots [24]bool) bool {
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

func containsMinute(start, end, minuteOfDay int) bool {
	if start < end {
		return minuteOfDay >= start && minuteOfDay < end
	}

	return minuteOfDay >= start || minuteOfDay < end
}
