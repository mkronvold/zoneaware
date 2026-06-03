package schedule

import (
	"testing"
	"time"

	"zoneaware/internal/config"
)

func TestBuildTracksTimezoneOffsetAcrossDST(t *testing.T) {
	cfg := config.Config{
		ReferenceTimezone: "UTC",
		WindowHours:       24,
		Team: []config.TeamMember{
			{
				Name:     "Alice",
				Timezone: "America/New_York",
				WorkingHours: []config.WorkingHours{
					{Start: "09:00", End: "10:00"},
				},
			},
		},
	}

	beforeDST, err := Build(cfg, time.Date(2024, 3, 8, 14, 0, 0, 0, time.UTC), 1)
	if err != nil {
		t.Fatalf("Build() before DST error = %v", err)
	}

	afterDST, err := Build(cfg, time.Date(2024, 3, 11, 13, 0, 0, 0, time.UTC), 1)
	if err != nil {
		t.Fatalf("Build() after DST error = %v", err)
	}

	if !beforeDST.Members[0].Cells[0].Halves[0] || !beforeDST.Members[0].Cells[0].Halves[1] {
		t.Fatal("before DST slot should be available at 14:00 UTC")
	}

	if !afterDST.Members[0].Cells[0].Halves[0] || !afterDST.Members[0].Cells[0].Halves[1] {
		t.Fatal("after DST slot should be available at 13:00 UTC")
	}
}

func TestBuildHandlesOvernightRanges(t *testing.T) {
	cfg := config.Config{
		ReferenceTimezone: "UTC",
		WindowHours:       24,
		Team: []config.TeamMember{
			{
				Name:     "Devi",
				Timezone: "UTC",
				WorkingHours: []config.WorkingHours{
					{Start: "22:00", End: "02:00"},
				},
			},
		},
	}

	window, err := Build(cfg, time.Date(2024, 3, 8, 21, 0, 0, 0, time.UTC), 5)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	got := make([]bool, 0, len(window.Members[0].Cells))
	for _, cell := range window.Members[0].Cells {
		got = append(got, cell.Halves[0] || cell.Halves[1])
	}

	want := []bool{false, true, true, true, true}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cell %d = %t, want %t", i, got[i], want[i])
		}
	}
}
