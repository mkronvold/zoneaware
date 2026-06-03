package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"zoneaware/internal/config"
	"zoneaware/internal/schedule"
)

func TestVisibleZoneLabelsCollapseDuplicatesAndHiddenZones(t *testing.T) {
	model := NewModel(config.Config{}, "", func() time.Time {
		return time.Date(2024, 6, 3, 12, 0, 0, 0, time.UTC)
	})
	model.renderNow = time.Date(2024, 6, 3, 12, 0, 0, 0, time.UTC)
	model.hiddenZones["America/Denver"] = true

	members := []visibleMember{
		{timeline: schedule.MemberTimeline{Name: "Alice", Timezone: "America/Los_Angeles"}},
		{timeline: schedule.MemberTimeline{Name: "Bob", Timezone: "America/Los_Angeles"}},
		{timeline: schedule.MemberTimeline{Name: "Chen", Timezone: "America/Denver"}},
	}

	zones := model.visibleZoneLabels(members)
	if len(zones) != 1 {
		t.Fatalf("len(zones) = %d, want 1", len(zones))
	}

	if zones[0].label != "PDT" {
		t.Fatalf("zones[0].label = %q, want PDT", zones[0].label)
	}
}

func TestViewRendersTimezoneHeaderRowsAndMemberSuffixes(t *testing.T) {
	cfg := config.Config{
		ReferenceTimezone: "UTC",
		WindowHours:       24,
		Team: []config.TeamMember{
			{
				Name:     "Alice",
				Timezone: "America/Los_Angeles",
				WorkingHours: []config.WorkingHours{
					{Start: "09:00", End: "17:00"},
				},
			},
			{
				Name:     "Bob",
				Timezone: "America/Denver",
				WorkingHours: []config.WorkingHours{
					{Start: "09:00", End: "17:00"},
				},
			},
		},
	}

	model := NewModel(cfg, "", func() time.Time {
		return time.Date(2024, 6, 3, 12, 0, 0, 0, time.UTC)
	})
	model.width = 110
	model.height = 18

	view := model.View()

	if strings.Count(view, "PDT") < 2 {
		t.Fatalf("expected PDT to appear in both header and member row:\n%s", view)
	}

	if strings.Count(view, "MDT") < 2 {
		t.Fatalf("expected MDT to appear in both header and member row:\n%s", view)
	}

	if strings.Index(view, "PDT") > strings.Index(view, "Alice") {
		t.Fatalf("expected timezone header rows before member rows:\n%s", view)
	}
}

func TestCtrlLReturnsRefreshCommand(t *testing.T) {
	model := NewModel(config.Config{}, "", time.Now)

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	if cmd == nil {
		t.Fatal("expected ctrl+l to return a refresh command")
	}
}

func TestVisibleZoneLabelsSortByCurrentLocalTime(t *testing.T) {
	model := NewModel(config.Config{}, "", func() time.Time {
		return time.Date(2024, 6, 3, 12, 0, 0, 0, time.UTC)
	})
	model.renderNow = time.Date(2024, 6, 3, 12, 0, 0, 0, time.UTC)

	members := []visibleMember{
		{timeline: schedule.MemberTimeline{Name: "Tokyo", Timezone: "Asia/Tokyo"}},
		{timeline: schedule.MemberTimeline{Name: "Denver", Timezone: "America/Denver"}},
		{timeline: schedule.MemberTimeline{Name: "Los Angeles", Timezone: "America/Los_Angeles"}},
	}

	zones := model.visibleZoneLabels(members)
	if len(zones) != 3 {
		t.Fatalf("len(zones) = %d, want 3", len(zones))
	}

	if zones[0].label != "PDT" || zones[1].label != "MDT" {
		t.Fatalf("unexpected sort order: %#v", zones)
	}
	if zones[2].label != "JST" {
		t.Fatalf("unexpected third zone: %#v", zones)
	}
}

func TestDisplayedTimezonesUsesLiveFilter(t *testing.T) {
	model := NewModel(config.Config{ReferenceTimezone: "UTC"}, "", time.Now)
	model.timezoneOptions = []string{
		"America/Chicago",
		"America/Denver",
		"Asia/Tokyo",
	}
	model.pickerFilter = "den"

	options := model.displayedTimezones()
	if len(options) != 1 || options[0] != "America/Denver" {
		t.Fatalf("displayedTimezones() = %#v, want America/Denver", options)
	}
}

func TestDisplayedTimezonesScrollsWindow(t *testing.T) {
	model := NewModel(config.Config{ReferenceTimezone: "UTC"}, "", time.Now)
	model.height = 8
	model.timezoneOptions = []string{
		"Africa/Abidjan",
		"Africa/Accra",
		"Africa/Addis_Ababa",
		"Africa/Algiers",
		"Africa/Asmara",
		"Africa/Bamako",
		"Africa/Bangui",
	}
	model.pickerOpen = true

	for i := 0; i < 6; i++ {
		model.movePicker(1)
	}

	options := model.displayedTimezones()
	if len(options) != 6 {
		t.Fatalf("len(displayedTimezones()) = %d, want 6", len(options))
	}
	if options[0] != "Africa/Accra" || options[5] != "Africa/Bangui" {
		t.Fatalf("displayedTimezones() = %#v, want scrolled window", options)
	}
}

func TestSliceWindowShiftsVisibleRange(t *testing.T) {
	window := schedule.Window{
		Start: time.Date(2024, 6, 3, 12, 0, 0, 0, time.UTC),
		Hours: 4,
		Members: []schedule.MemberTimeline{
			{
				Name: "Alice",
				Cells: []schedule.Cell{
					{Start: time.Date(2024, 6, 3, 12, 0, 0, 0, time.UTC)},
					{Start: time.Date(2024, 6, 3, 13, 0, 0, 0, time.UTC)},
					{Start: time.Date(2024, 6, 3, 14, 0, 0, 0, time.UTC)},
					{Start: time.Date(2024, 6, 3, 15, 0, 0, 0, time.UTC)},
				},
			},
		},
	}

	sliced := sliceWindow(window, 1, 2)
	if sliced.Hours != 2 {
		t.Fatalf("sliced.Hours = %d, want 2", sliced.Hours)
	}
	if sliced.Start.Hour() != 13 {
		t.Fatalf("sliced.Start = %v, want hour 13", sliced.Start)
	}
	if len(sliced.Members[0].Cells) != 2 || sliced.Members[0].Cells[0].Start.Hour() != 13 {
		t.Fatalf("unexpected sliced cells: %#v", sliced.Members[0].Cells)
	}
}

func TestViewWarnsWhenHeightCannotFitVisibleMembers(t *testing.T) {
	cfg := config.Config{
		ReferenceTimezone: "UTC",
		WindowHours:       48,
		Team: []config.TeamMember{
			{Name: "Alice", Timezone: "UTC", WorkingHours: []config.WorkingHours{{Start: "09:00", End: "17:00"}}},
			{Name: "Bob", Timezone: "UTC", WorkingHours: []config.WorkingHours{{Start: "09:00", End: "17:00"}}},
			{Name: "Chen", Timezone: "UTC", WorkingHours: []config.WorkingHours{{Start: "09:00", End: "17:00"}}},
		},
	}

	model := NewModel(cfg, "", func() time.Time {
		return time.Date(2024, 6, 3, 12, 0, 0, 0, time.UTC)
	})
	model.width = 110
	model.height = 5

	view := model.View()
	if !strings.Contains(view, "Need at least") {
		t.Fatalf("expected height warning, got:\n%s", view)
	}
}

func TestCommitEditMemberRenamesMember(t *testing.T) {
	model := NewModel(config.Config{
		ReferenceTimezone: "UTC",
		Team: []config.TeamMember{
			{Name: "Alice", Timezone: "UTC"},
		},
	}, "", time.Now)

	model.beginEditMember(0)
	model.editingValue = "Alicia"
	if err := model.commitEditMember(); err != nil {
		t.Fatalf("commitEditMember() error = %v", err)
	}

	if model.cfg.Team[0].Name != "Alicia" {
		t.Fatalf("member name = %q, want Alicia", model.cfg.Team[0].Name)
	}
}

func TestAddMemberCreatesPlaceholderAndStartsEditing(t *testing.T) {
	model := NewModel(config.Config{
		ReferenceTimezone: "UTC",
		Team: []config.TeamMember{
			{Name: "Alice", Timezone: "UTC"},
		},
	}, "", time.Now)

	if err := model.addMember(); err != nil {
		t.Fatalf("addMember() error = %v", err)
	}

	if len(model.cfg.Team) != 2 {
		t.Fatalf("len(cfg.Team) = %d, want 2", len(model.cfg.Team))
	}
	if model.cfg.Team[1].Name != "New Member" {
		t.Fatalf("new member name = %q, want New Member", model.cfg.Team[1].Name)
	}
	if model.editingIndex != 1 {
		t.Fatalf("editingIndex = %d, want 1", model.editingIndex)
	}
}

func TestSelectMemberTimezoneUpdatesConfig(t *testing.T) {
	model := NewModel(config.Config{
		ReferenceTimezone: "UTC",
		Team: []config.TeamMember{
			{Name: "Alice", Timezone: "UTC"},
		},
	}, "", time.Now)

	if err := model.selectMemberTimezone(0, "America/Denver"); err != nil {
		t.Fatalf("selectMemberTimezone() error = %v", err)
	}

	if model.cfg.Team[0].Timezone != "America/Denver" {
		t.Fatalf("member timezone = %q, want America/Denver", model.cfg.Team[0].Timezone)
	}
}

func TestMemberTimezoneLabelsRemainWhenHeaderTimezoneHidden(t *testing.T) {
	cfg := config.Config{
		ReferenceTimezone: "UTC",
		WindowHours:       24,
		Team: []config.TeamMember{
			{
				Name:         "Alice",
				Timezone:     "America/Denver",
				WorkingHours: []config.WorkingHours{{Start: "09:00", End: "17:00"}},
			},
		},
	}

	model := NewModel(cfg, "", func() time.Time {
		return time.Date(2024, 6, 3, 12, 0, 0, 0, time.UTC)
	})
	model.hiddenZones["America/Denver"] = true
	model.width = 110
	model.height = 14

	view := model.View()
	if !strings.Contains(view, "MDT") {
		t.Fatalf("expected member timezone label to remain visible:\n%s", view)
	}
}

func TestEditingRowIsSoftHighlighted(t *testing.T) {
	cfg := config.Config{
		ReferenceTimezone: "UTC",
		WindowHours:       24,
		Team: []config.TeamMember{
			{Name: "Alice", Timezone: "America/Denver", WorkingHours: []config.WorkingHours{{Start: "09:00", End: "17:00"}}},
		},
	}

	model := NewModel(cfg, "", func() time.Time {
		return time.Date(2024, 6, 3, 12, 0, 0, 0, time.UTC)
	})
	model.width = 110
	model.height = 14
	baseView := model.View()
	model.beginEditMember(0)

	view := model.View()
	if view == baseView {
		t.Fatalf("expected edit row rendering to differ from base view")
	}
}

func TestHandleClickTogglesOnlyTargetCell(t *testing.T) {
	model := NewModel(config.Config{
		ReferenceTimezone: "UTC",
		WindowHours:       48,
		Team: []config.TeamMember{
			{Name: "Alice", Timezone: "UTC"},
		},
	}, "", func() time.Time {
		return time.Date(2024, 6, 3, 12, 0, 0, 0, time.UTC)
	})
	model.width = 120
	model.height = 20

	_ = model.View()

	targetTime := time.Date(2024, 6, 3, 16, 0, 0, 0, time.UTC)
	var target hotspot
	found := false
	for _, spot := range model.hotspots {
		if spot.kind == hotspotCell && spot.index == 0 && spot.slotStart.Equal(targetTime) {
			target = spot
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("did not find hotspot for %v", targetTime)
	}

	model.handleClick(target.x1, target.y)

	slots, err := config.HourlySlots(model.cfg.Team[0].WorkingHours)
	if err != nil {
		t.Fatalf("HourlySlots() error = %v", err)
	}
	for hour, enabled := range slots {
		want := hour == 16
		if enabled != want {
			t.Fatalf("slot %d = %t, want %t", hour, enabled, want)
		}
	}
}

func TestHandleClickTogglesOnlySelectedCellForHalfHourOffset(t *testing.T) {
	model := NewModel(config.Config{
		ReferenceTimezone: "UTC",
		WindowHours:       48,
		Team: []config.TeamMember{
			{Name: "Mahabelesh", Timezone: "Asia/Kolkata"},
		},
	}, "", func() time.Time {
		return time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC)
	})
	model.width = 120
	model.height = 20

	_ = model.View()

	var target hotspot
	found := false
	for _, spot := range model.hotspots {
		if spot.kind == hotspotCell && spot.index == 0 && spot.slotStart.Equal(time.Date(2024, 6, 3, 1, 0, 0, 0, time.UTC)) {
			target = spot
			found = true
			break
		}
	}
	if !found {
		t.Fatal("did not find second-column hotspot")
	}

	model.handleClick(target.x1, target.y)

	window, err := schedule.Build(model.cfg, time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC), 4)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	got := []bool{
		window.Members[0].Cells[0].Available,
		window.Members[0].Cells[1].Available,
		window.Members[0].Cells[2].Available,
	}
	want := []bool{false, true, false}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cell %d = %t, want %t (got=%v)", i, got[i], want[i], got)
		}
	}
}
