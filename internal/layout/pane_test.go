package layout

import (
	"testing"
	"time"

	"zoneaware/internal/config"
)

func TestZellijPaneSize(t *testing.T) {
	cfg := config.Config{
		Team: []config.TeamMember{
			{Name: "Alice", Timezone: "America/Los_Angeles"},
			{Name: "Mahabelesh", Timezone: "Asia/Kolkata"},
			{Name: "Bob", Timezone: "America/Denver"},
		},
	}

	width, height := ZellijPaneSize(cfg, time.Date(2024, 6, 3, 12, 0, 0, 0, time.UTC), 24)

	if width != 89 {
		t.Fatalf("width = %d, want 89", width)
	}
	if height != 16 {
		t.Fatalf("height = %d, want 16", height)
	}
}
