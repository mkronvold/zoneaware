package config

import (
	"testing"
	"time"
)

func TestParseAppliesDefaults(t *testing.T) {
	cfg, err := Parse([]byte(`
team:
  - name: Alice
    timezone: America/Los_Angeles
    working_hours:
      - start: "09:00"
        end: "17:00"
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.ReferenceTimezone == "" {
		t.Fatal("ReferenceTimezone = empty, want shell default timezone")
	}
	if _, err := time.LoadLocation(cfg.ReferenceTimezone); err != nil {
		t.Fatalf("ReferenceTimezone %q should be loadable: %v", cfg.ReferenceTimezone, err)
	}

	if cfg.WindowHours != DefaultWindowHours {
		t.Fatalf("WindowHours = %d, want %d", cfg.WindowHours, DefaultWindowHours)
	}
}

func TestParseRejectsDuplicateNames(t *testing.T) {
	_, err := Parse([]byte(`
reference_timezone: UTC
team:
  - name: Alice
    timezone: UTC
    working_hours:
      - start: "09:00"
        end: "17:00"
  - name: alice
    timezone: UTC
    working_hours:
      - start: "10:00"
        end: "18:00"
`))
	if err == nil {
		t.Fatal("Parse() error = nil, want duplicate-name error")
	}
}

func TestToggleMemberHourAddsAndRemovesSlots(t *testing.T) {
	cfg := Config{
		Team: []TeamMember{
			{
				Name:     "Alice",
				Timezone: "America/Los_Angeles",
				WorkingHours: []WorkingHours{
					{Start: "09:00", End: "10:00"},
				},
			},
		},
	}

	slot := time.Date(2024, 6, 3, 16, 0, 0, 0, time.UTC)
	if err := cfg.ToggleMemberHour("Alice", slot); err != nil {
		t.Fatalf("ToggleMemberHour() remove error = %v", err)
	}
	if len(cfg.Team[0].WorkingHours) != 0 {
		t.Fatalf("WorkingHours after remove = %#v, want none", cfg.Team[0].WorkingHours)
	}

	if err := cfg.ToggleMemberHour("Alice", slot); err != nil {
		t.Fatalf("ToggleMemberHour() add error = %v", err)
	}
	if len(cfg.Team[0].WorkingHours) != 1 {
		t.Fatalf("WorkingHours after add len = %d, want 1", len(cfg.Team[0].WorkingHours))
	}
	if cfg.Team[0].WorkingHours[0] != (WorkingHours{Start: "09:00", End: "10:00"}) {
		t.Fatalf("WorkingHours after add = %#v", cfg.Team[0].WorkingHours)
	}
}

func TestToggleMemberHourUsesQuarterPrecisionForHalfHourOffsets(t *testing.T) {
	member := TeamMember{
		Name:     "Mahabelesh",
		Timezone: "Asia/Kolkata",
	}

	slot := time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC)
	if err := member.ToggleHour(slot); err != nil {
		t.Fatalf("ToggleHour() error = %v", err)
	}

	if len(member.WorkingHours) != 1 {
		t.Fatalf("len(WorkingHours) = %d, want 1", len(member.WorkingHours))
	}

	want := WorkingHours{Start: "05:30", End: "06:30"}
	if member.WorkingHours[0] != want {
		t.Fatalf("WorkingHours[0] = %#v, want %#v", member.WorkingHours[0], want)
	}
}
