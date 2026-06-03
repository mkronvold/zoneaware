package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"zoneaware/internal/config"
	"zoneaware/internal/schedule"
)

const (
	minWidth           = 48
	cellWidth          = 3
	minNameWidth       = 8
	baseFrameRows      = 8
	maxTimezoneOptions = 6
)

type hotspotKind int

const (
	hotspotMember hotspotKind = iota
	hotspotTimezone
	hotspotMemberTimezone
	hotspotCell
	hotspotShowAll
	hotspotAdd
	hotspotReference
	hotspotReferenceOption
)

type hotspot struct {
	kind      hotspotKind
	value     string
	index     int
	slotStart time.Time
	x1        int
	x2        int
	y         int
}

type tickMsg time.Time

type styles struct {
	name             lipgloss.Style
	available        lipgloss.Style
	offHours         lipgloss.Style
	currentAvailable lipgloss.Style
	currentOffHours  lipgloss.Style
	currentHeader    lipgloss.Style
	hidden           lipgloss.Style
	showAll          lipgloss.Style
	info             lipgloss.Style
	muted            lipgloss.Style
	title            lipgloss.Style
	timezone         lipgloss.Style
	error            lipgloss.Style
	pickerInput      lipgloss.Style
	pickerOption     lipgloss.Style
	pickerSelected   lipgloss.Style
}

type Model struct {
	cfg             config.Config
	configPath      string
	now             func() time.Time
	renderNow       time.Time
	width           int
	height          int
	timeOffset      int
	hiddenMembers   map[string]bool
	hiddenZones     map[string]bool
	hotspots        []hotspot
	styles          styles
	lastError       string
	timezoneOptions []string
	pickerOpen      bool
	pickerFilter    string
	pickerSelection int
	pickerTarget    int
	editingIndex    int
	editingValue    string
}

func NewModel(cfg config.Config, configPath string, now func() time.Time) *Model {
	return &Model{
		cfg:           cfg,
		configPath:    configPath,
		now:           now,
		width:         100,
		height:        20,
		hiddenMembers: make(map[string]bool),
		hiddenZones:   make(map[string]bool),
		pickerTarget:  -1,
		editingIndex:  -1,
		styles: styles{
			name:             lipgloss.NewStyle().Foreground(lipgloss.Color("110")).Bold(true),
			available:        lipgloss.NewStyle().Foreground(lipgloss.Color("108")),
			offHours:         lipgloss.NewStyle().Foreground(lipgloss.Color("239")),
			currentAvailable: lipgloss.NewStyle().Foreground(lipgloss.Color("217")).Bold(true),
			currentOffHours:  lipgloss.NewStyle().Foreground(lipgloss.Color("217")).Bold(true),
			currentHeader:    lipgloss.NewStyle().Foreground(lipgloss.Color("217")).Bold(true),
			hidden:           lipgloss.NewStyle().Foreground(lipgloss.Color("167")),
			showAll:          lipgloss.NewStyle().Foreground(lipgloss.Color("222")).Bold(true),
			info:             lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
			muted:            lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
			title:            lipgloss.NewStyle().Foreground(lipgloss.Color("222")).Bold(true),
			timezone:         lipgloss.NewStyle().Foreground(lipgloss.Color("110")),
			error:            lipgloss.NewStyle().Foreground(lipgloss.Color("167")).Bold(true),
			pickerInput:      lipgloss.NewStyle().Foreground(lipgloss.Color("222")).Bold(true),
			pickerOption:     lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
			pickerSelected:   lipgloss.NewStyle().Foreground(lipgloss.Color("222")).Bold(true),
		},
	}
}

func (m *Model) Init() tea.Cmd {
	return tickCmd()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampTimeOffset(config.EffectiveWindowHours(m.cfg), m.currentVisibleHourCount())
		return m, nil
	case tickMsg:
		return m, tickCmd()
	case tea.KeyMsg:
		if m.pickerOpen {
			return m.updatePickerKey(msg)
		}
		if m.editingIndex >= 0 {
			return m.updateNameEditor(msg)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "ctrl+l":
			m.lastError = ""
			return m, tea.ClearScreen
		case "r":
			m.hiddenMembers = make(map[string]bool)
			m.hiddenZones = make(map[string]bool)
			m.timeOffset = 0
			m.lastError = ""
			m.cancelEditMember()
		case "left", "up":
			m.scrollTime(-1)
		case "right", "down":
			m.scrollTime(1)
		case "pgup":
			m.scrollTime(-m.currentVisibleHourCount())
		case "pgdown":
			m.scrollTime(m.currentVisibleHourCount())
		}
	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			if !m.pickerOpen {
				m.scrollTime(-1)
			}
		case tea.MouseWheelDown:
			if !m.pickerOpen {
				m.scrollTime(1)
			}
		case tea.MouseLeft:
			if msg.Action == tea.MouseActionPress {
				m.handleClick(msg.X, msg.Y)
			}
		}
	}

	return m, nil
}

func (m *Model) View() string {
	m.renderNow = m.now()

	if m.width < minWidth {
		return m.renderMessage("Resize the terminal to at least 48 columns to render the timeline.", "q to quit")
	}

	visibleConfigMembers := m.visibleConfigMembers()
	visibleZones := m.visibleConfigZones(visibleConfigMembers)
	nameWidth := m.nameColumnWidth()
	labelWidth := zoneLabelWidth(visibleZones)
	visibleHours := m.hourCount(nameWidth, labelWidth)
	if visibleHours < 1 {
		return m.renderMessage("Terminal is too narrow to render any timeline columns.", "q to quit")
	}

	fullWindow, err := schedule.Build(m.cfg, m.renderNow, config.EffectiveWindowHours(m.cfg))
	if err != nil {
		return m.renderMessage(fmt.Sprintf("Failed to build timeline: %v", err), "q to quit")
	}

	referenceLocation, err := time.LoadLocation(m.cfg.ReferenceTimezone)
	if err != nil {
		return m.renderMessage(fmt.Sprintf("Failed to load reference timezone: %v", err), "q to quit")
	}

	m.clampTimeOffset(fullWindow.Hours, visibleHours)
	window := sliceWindow(fullWindow, m.timeOffset, visibleHours)

	visibleRows := make([]visibleMember, 0, len(window.Members))
	for index, member := range window.Members {
		if !m.hiddenMembers[member.Name] {
			visibleRows = append(visibleRows, visibleMember{index: index, timeline: member})
		}
	}

	visibleZones = m.visibleZoneLabels(visibleRows)
	labelWidth = zoneLabelWidth(visibleZones)
	pickerRows := m.referencePickerRows()
	requiredHeight := m.requiredHeight(len(visibleRows), len(visibleZones), pickerRows)
	if m.height < requiredHeight {
		return m.renderMessage(
			fmt.Sprintf("Need at least %d rows to fit %d visible team members; current height is %d.", requiredHeight, len(visibleRows), m.height),
			"Resize the terminal taller to show everyone at once.",
		)
	}

	lines := make([]string, 0, m.height)
	m.hotspots = nil

	lines = append(lines, m.renderLine(m.styles.title.Render("ZoneAware (za)")))
	lines = append(lines, m.renderLine(m.renderStatusLine(1, window, fullWindow.Hours, visibleRows)))

	currentY := 2
	if m.pickerOpen {
		pickerLines := m.renderReferencePicker(currentY)
		lines = append(lines, pickerLines...)
		currentY += len(pickerLines)
	}

	lines = append(lines, m.renderSeparator())
	currentY++

	for _, zone := range visibleZones {
		lines = append(lines, m.renderLine(m.renderZoneHeaderRow(nameWidth, labelWidth, window, zone, currentY, referenceLocation)))
		currentY++
	}

	lines = append(lines, m.renderLine(m.renderTimelineDivider(nameWidth, window, referenceLocation)))
	currentY++

	for idx := 0; idx < len(visibleRows); idx++ {
		lineY := currentY + idx
		lines = append(lines, m.renderLine(m.renderMemberRow(nameWidth, labelWidth, visibleRows[idx], lineY, referenceLocation)))
	}
	currentY += len(visibleRows)

	lines = append(lines, m.renderLine(m.styles.muted.Render("Click a cell to toggle that hour · click a name to edit it · click a header timezone to hide it · click a member timezone or Local Reference to change it · wheel scrolls time horizontally · Ctrl+L refresh · r resets · q quits")))
	currentY++
	lines = append(lines, m.renderSeparator())
	currentY++
	lines = append(lines, m.renderLine(m.renderHiddenLine()))
	currentY++
	lines = append(lines, m.renderLine(m.renderFooterActions(currentY)))

	return strings.Join(lines, "\n")
}

func (m *Model) updatePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.pickerOpen = false
		m.pickerFilter = ""
		m.pickerSelection = 0
		m.pickerTarget = -1
		return m, nil
	case "ctrl+l":
		m.lastError = ""
		return m, tea.ClearScreen
	case "enter":
		options := m.displayedTimezones()
		if len(options) == 0 {
			return m, nil
		}
		if err := m.selectPickedTimezone(options[m.pickerSelection]); err != nil {
			m.lastError = err.Error()
		}
		return m, tea.ClearScreen
	case "up":
		if m.pickerSelection > 0 {
			m.pickerSelection--
		}
		return m, nil
	case "down":
		if m.pickerSelection < len(m.displayedTimezones())-1 {
			m.pickerSelection++
		}
		return m, nil
	case "backspace", "ctrl+h":
		if m.pickerFilter != "" {
			runes := []rune(m.pickerFilter)
			m.pickerFilter = string(runes[:len(runes)-1])
			m.pickerSelection = 0
		}
		return m, nil
	}

	if len(msg.Runes) > 0 {
		m.pickerFilter += string(msg.Runes)
		m.pickerSelection = 0
	}

	return m, nil
}

func (m *Model) updateNameEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.cancelEditMember()
		return m, nil
	case "enter":
		if err := m.commitEditMember(); err != nil {
			m.lastError = err.Error()
		}
		return m, nil
	case "ctrl+l":
		m.lastError = ""
		return m, tea.ClearScreen
	case "backspace", "ctrl+h":
		if m.editingValue != "" {
			runes := []rune(m.editingValue)
			m.editingValue = string(runes[:len(runes)-1])
		}
		return m, nil
	}

	if len(msg.Runes) > 0 {
		m.editingValue += string(msg.Runes)
	}

	return m, nil
}

func (m *Model) renderStatusLine(lineY int, window schedule.Window, totalHours int, visibleRows []visibleMember) string {
	if m.lastError != "" {
		return m.styles.error.Render(m.lastError)
	}

	prefix := "Local Reference: "
	reference := m.referenceDisplay()
	suffix := fmt.Sprintf(" · Window: %dh view of %dh · Team: %d/%d", window.Hours, totalHours, len(visibleRows), len(window.Members))

	m.hotspots = append(m.hotspots, hotspot{
		kind:  hotspotReference,
		x1:    lipgloss.Width(prefix),
		x2:    lipgloss.Width(prefix) + lipgloss.Width(reference),
		y:     lineY,
		value: m.cfg.ReferenceTimezone,
	})

	return m.styles.info.Render(prefix) + m.styles.timezone.Render(reference) + m.styles.info.Render(suffix)
}

func (m *Model) renderReferencePicker(startY int) []string {
	lines := make([]string, 0, m.referencePickerRows())
	filter := m.pickerFilter
	if filter == "" {
		filter = "type to filter timezones"
	}
	lines = append(lines, m.renderLine(m.styles.pickerInput.Render("Timezone filter: "+filter)))

	options := m.displayedTimezones()
	if len(options) == 0 {
		lines = append(lines, m.renderLine(m.styles.muted.Render("No matching timezones")))
		return lines
	}

	if m.pickerSelection >= len(options) {
		m.pickerSelection = len(options) - 1
	}

	for idx, zone := range options {
		style := m.styles.pickerOption
		if idx == m.pickerSelection {
			style = m.styles.pickerSelected
		}
		label := zone
		lines = append(lines, m.renderLine(style.Render(label)))
		m.hotspots = append(m.hotspots, hotspot{
			kind:  hotspotReferenceOption,
			value: zone,
			x1:    0,
			x2:    lipgloss.Width(label),
			y:     startY + 1 + idx,
		})
	}

	return lines
}

func (m *Model) renderZoneHeaderRow(nameWidth, labelWidth int, window schedule.Window, zone zoneInfo, lineY int, referenceLocation *time.Location) string {
	location, err := time.LoadLocation(zone.id)
	if err != nil {
		location = window.Start.Location()
	}

	left := strings.Repeat(" ", nameWidth+1)
	cells := m.renderHourCells(window.Start, window.Hours, location, referenceLocation)
	label := m.styles.timezone.Render(padLeft(zone.label, labelWidth))
	m.hotspots = append(m.hotspots, hotspot{
		kind:  hotspotTimezone,
		value: zone.id,
		x1:    m.contentWidth() - labelWidth,
		x2:    m.contentWidth(),
		y:     lineY,
	})

	return m.composeTimelineRow(left, cells, label)
}

func (m *Model) renderTimelineDivider(nameWidth int, window schedule.Window, referenceLocation *time.Location) string {
	var builder strings.Builder
	builder.WriteString(strings.Repeat(" ", nameWidth+1))
	for idx := 0; idx < window.Hours; idx++ {
		separator := m.cellSeparator(window.Start, idx, window.Hours, referenceLocation)
		builder.WriteString(m.styles.muted.Render("──" + separator))
	}
	return builder.String()
}

func (m *Model) renderMemberRow(nameWidth, labelWidth int, row visibleMember, lineY int, referenceLocation *time.Location) string {
	var builder strings.Builder
	nameText := row.timeline.Name
	if row.index == m.editingIndex {
		nameText = m.editingDisplay(nameWidth)
		builder.WriteString(m.styles.pickerInput.Render(padRight(nameText, nameWidth)))
	} else {
		builder.WriteString(m.styles.name.Render(padRight(nameText, nameWidth)))
	}
	builder.WriteString(" ")

	m.hotspots = append(m.hotspots, hotspot{
		kind:  hotspotMember,
		value: row.timeline.Name,
		index: row.index,
		x1:    0,
		x2:    nameWidth,
		y:     lineY,
	})

	cellsStartX := nameWidth + 1
	for idx, cell := range row.timeline.Cells {
		m.hotspots = append(m.hotspots, hotspot{
			kind:      hotspotCell,
			value:     row.timeline.Name,
			index:     row.index,
			slotStart: cell.Start,
			x1:        cellsStartX + idx*cellWidth,
			x2:        cellsStartX + (idx+1)*cellWidth,
			y:         lineY,
		})

		separator := m.cellSeparator(row.timeline.Cells[0].Start, idx, len(row.timeline.Cells), referenceLocation)
		if m.isCurrentHour(cell.Start) {
			if cell.Available {
				builder.WriteString(m.styles.currentAvailable.Render("██" + separator))
			} else {
				builder.WriteString(m.styles.currentOffHours.Render("░░" + separator))
			}
			continue
		}

		if cell.Available {
			builder.WriteString(m.styles.available.Render("██" + separator))
		} else {
			builder.WriteString(m.styles.offHours.Render("░░" + separator))
		}
	}

	rightLabel := ""
	if !m.hiddenZones[row.timeline.Timezone] && labelWidth > 0 {
		rightLabel = m.styles.timezone.Render(padLeft(zoneLabel(row.timeline.Timezone, m.renderNow), labelWidth))
		m.hotspots = append(m.hotspots, hotspot{
			kind:  hotspotMemberTimezone,
			value: row.timeline.Timezone,
			index: row.index,
			x1:    m.contentWidth() - labelWidth,
			x2:    m.contentWidth(),
			y:     lineY,
		})
	}

	return m.composeTimelineRow("", builder.String(), rightLabel)
}

func (m *Model) renderHiddenLine() string {
	items := make([]string, 0)
	for _, member := range m.cfg.Team {
		if m.hiddenMembers[member.Name] {
			items = append(items, member.Name)
		}
	}

	seenZones := make(map[string]bool)
	for _, member := range m.cfg.Team {
		if seenZones[member.Timezone] || !m.hiddenZones[member.Timezone] {
			continue
		}
		seenZones[member.Timezone] = true
		items = append(items, zoneLabel(member.Timezone, m.renderNow))
	}

	if len(items) == 0 {
		return m.styles.info.Render("Hidden: ") + m.styles.muted.Render("none")
	}

	return m.styles.info.Render("Hidden: ") + m.styles.hidden.Render(strings.Join(items, ", "))
}

func (m *Model) renderFooterActions(lineY int) string {
	showAll := "[ Show All ]"
	add := "[ Add ]"
	m.hotspots = append(m.hotspots, hotspot{
		kind: hotspotShowAll,
		x1:   0,
		x2:   lipgloss.Width(showAll),
		y:    lineY,
	})

	addStart := lipgloss.Width(showAll) + 2
	m.hotspots = append(m.hotspots, hotspot{
		kind: hotspotAdd,
		x1:   addStart,
		x2:   addStart + lipgloss.Width(add),
		y:    lineY,
	})

	return m.styles.showAll.Render(showAll) + "  " + m.styles.showAll.Render(add)
}

func (m *Model) handleClick(x, y int) {
	for _, spot := range m.hotspots {
		if y != spot.y || x < spot.x1 || x >= spot.x2 {
			continue
		}

		switch spot.kind {
		case hotspotMember:
			m.beginEditMember(spot.index)
		case hotspotTimezone:
			m.hiddenZones[spot.value] = !m.hiddenZones[spot.value]
			if !m.hiddenZones[spot.value] {
				delete(m.hiddenZones, spot.value)
			}
			m.clampTimeOffset(config.EffectiveWindowHours(m.cfg), m.currentVisibleHourCount())
		case hotspotMemberTimezone:
			if m.editingIndex >= 0 {
				return
			}
			if err := m.openTimezonePicker(spot.index); err != nil {
				m.lastError = err.Error()
			}
		case hotspotCell:
			if m.editingIndex >= 0 {
				return
			}
			if err := m.toggleMemberHour(spot.index, spot.slotStart); err != nil {
				m.lastError = err.Error()
			} else {
				m.lastError = ""
			}
		case hotspotShowAll:
			m.hiddenMembers = make(map[string]bool)
			m.hiddenZones = make(map[string]bool)
			m.timeOffset = 0
			m.lastError = ""
			m.cancelEditMember()
		case hotspotAdd:
			if m.editingIndex >= 0 {
				return
			}
			if err := m.addMember(); err != nil {
				m.lastError = err.Error()
			}
		case hotspotReference:
			if m.editingIndex >= 0 {
				return
			}
			if m.pickerOpen {
				m.pickerOpen = false
				m.pickerFilter = ""
				m.pickerSelection = 0
				m.pickerTarget = -1
				return
			}
			if err := m.openTimezonePicker(-1); err != nil {
				m.lastError = err.Error()
			}
		case hotspotReferenceOption:
			if err := m.selectPickedTimezone(spot.value); err != nil {
				m.lastError = err.Error()
			}
		}
		return
	}

	if m.pickerOpen {
		m.pickerOpen = false
		m.pickerFilter = ""
		m.pickerSelection = 0
		m.pickerTarget = -1
	}
}

func (m *Model) openTimezonePicker(targetMember int) error {
	if err := m.ensureTimezoneOptions(); err != nil {
		return err
	}

	m.pickerOpen = true
	m.pickerFilter = ""
	m.pickerSelection = 0
	m.pickerTarget = targetMember
	m.lastError = ""
	return nil
}

func (m *Model) ensureTimezoneOptions() error {
	if len(m.timezoneOptions) > 0 {
		return nil
	}

	timezones, err := config.AvailableTimezones()
	if err != nil {
		return err
	}

	if !containsString(timezones, m.cfg.ReferenceTimezone) {
		timezones = append(timezones, m.cfg.ReferenceTimezone)
		sort.Strings(timezones)
	}
	m.timezoneOptions = timezones
	return nil
}

func (m *Model) selectPickedTimezone(timezone string) error {
	if m.pickerTarget >= 0 {
		return m.selectMemberTimezone(m.pickerTarget, timezone)
	}
	return m.selectReferenceTimezone(timezone)
}

func (m *Model) selectReferenceTimezone(timezone string) error {
	original := cloneConfig(m.cfg)
	m.cfg.ReferenceTimezone = timezone
	if m.configPath != "" {
		if err := config.Save(m.configPath, m.cfg); err != nil {
			m.cfg = original
			return err
		}
	}

	m.pickerOpen = false
	m.pickerFilter = ""
	m.pickerSelection = 0
	m.pickerTarget = -1
	m.lastError = ""
	return nil
}

func (m *Model) selectMemberTimezone(index int, timezone string) error {
	if index < 0 || index >= len(m.cfg.Team) {
		return fmt.Errorf("team member index %d is out of range", index)
	}

	original := cloneConfig(m.cfg)
	m.cfg.Team[index].Timezone = timezone
	if m.configPath != "" {
		if err := config.Save(m.configPath, m.cfg); err != nil {
			m.cfg = original
			return err
		}
	}

	m.pickerOpen = false
	m.pickerFilter = ""
	m.pickerSelection = 0
	m.pickerTarget = -1
	m.lastError = ""
	return nil
}

func (m *Model) beginEditMember(index int) {
	if index < 0 || index >= len(m.cfg.Team) {
		return
	}

	m.editingIndex = index
	m.editingValue = m.cfg.Team[index].Name
	m.lastError = ""
}

func (m *Model) cancelEditMember() {
	m.editingIndex = -1
	m.editingValue = ""
}

func (m *Model) commitEditMember() error {
	if m.editingIndex < 0 || m.editingIndex >= len(m.cfg.Team) {
		m.cancelEditMember()
		return nil
	}

	name := strings.TrimSpace(m.editingValue)
	if name == "" {
		return fmt.Errorf("member name cannot be empty")
	}

	original := cloneConfig(m.cfg)
	m.cfg.Team[m.editingIndex].Name = name
	if m.configPath != "" {
		if err := config.Save(m.configPath, m.cfg); err != nil {
			m.cfg = original
			return err
		}
	}

	m.cancelEditMember()
	m.lastError = ""
	return nil
}

func (m *Model) addMember() error {
	original := cloneConfig(m.cfg)
	name := m.nextNewMemberName()
	m.cfg.Team = append(m.cfg.Team, config.TeamMember{
		Name:         name,
		Timezone:     m.cfg.ReferenceTimezone,
		WorkingHours: nil,
	})
	if m.configPath != "" {
		if err := config.Save(m.configPath, m.cfg); err != nil {
			m.cfg = original
			return err
		}
	}

	m.beginEditMember(len(m.cfg.Team) - 1)
	return nil
}

func (m *Model) nextNewMemberName() string {
	base := "New Member"
	if !m.memberNameExists(base) {
		return base
	}

	for index := 2; ; index++ {
		candidate := fmt.Sprintf("%s %d", base, index)
		if !m.memberNameExists(candidate) {
			return candidate
		}
	}
}

func (m *Model) memberNameExists(name string) bool {
	for _, member := range m.cfg.Team {
		if member.Name == name {
			return true
		}
	}
	return false
}

func (m *Model) filteredTimezones() []string {
	options := append([]string(nil), m.timezoneOptions...)
	if !containsString(options, m.cfg.ReferenceTimezone) {
		options = append(options, m.cfg.ReferenceTimezone)
		sort.Strings(options)
	}

	if m.pickerFilter == "" {
		return options
	}

	filter := strings.ToLower(m.pickerFilter)
	filtered := make([]string, 0, len(options))
	for _, option := range options {
		if strings.Contains(strings.ToLower(option), filter) {
			filtered = append(filtered, option)
		}
	}
	return filtered
}

func (m *Model) displayedTimezones() []string {
	options := m.filteredTimezones()
	if len(options) > maxTimezoneOptions {
		return options[:maxTimezoneOptions]
	}
	return options
}

func (m *Model) referencePickerRows() int {
	if !m.pickerOpen {
		return 0
	}

	optionCount := len(m.filteredTimezones())
	if optionCount == 0 {
		return 2
	}
	if optionCount > maxTimezoneOptions {
		optionCount = maxTimezoneOptions
	}
	return 1 + optionCount
}

func (m *Model) visibleZoneLabels(members []visibleMember) []zoneInfo {
	seen := make(map[string]bool)
	zones := make([]zoneInfo, 0)
	for _, member := range members {
		if seen[member.timeline.Timezone] || m.hiddenZones[member.timeline.Timezone] {
			continue
		}
		seen[member.timeline.Timezone] = true
		zones = append(zones, zoneInfo{
			id:    member.timeline.Timezone,
			label: zoneLabel(member.timeline.Timezone, m.renderNow),
		})
	}

	m.sortZonesByCurrentTime(zones)
	return zones
}

func (m *Model) scrollTime(delta int) {
	m.timeOffset += delta
	m.clampTimeOffset(config.EffectiveWindowHours(m.cfg), m.currentVisibleHourCount())
}

func (m *Model) clampTimeOffset(totalHours, visibleHours int) {
	maxOffset := max(0, totalHours-visibleHours)
	if m.timeOffset < 0 {
		m.timeOffset = 0
	}
	if m.timeOffset > maxOffset {
		m.timeOffset = maxOffset
	}
}

func (m *Model) nameColumnWidth() int {
	maxName := minNameWidth
	for _, member := range m.cfg.Team {
		maxName = max(maxName, lipgloss.Width(member.Name))
	}

	maxAllowed := max(minNameWidth, m.contentWidth()-(cellWidth*4)-1)
	if maxName > maxAllowed {
		return maxAllowed
	}

	return maxName
}

func (m *Model) hourCount(nameWidth, labelWidth int) int {
	usable := m.contentWidth() - nameWidth - 1
	if labelWidth > 0 {
		usable -= labelWidth + 1
	}
	if usable <= 0 {
		return 0
	}

	hours := usable / cellWidth
	if hours > config.EffectiveWindowHours(m.cfg) {
		hours = config.EffectiveWindowHours(m.cfg)
	}

	return hours
}

func (m *Model) requiredHeight(memberCount, zoneCount, pickerRows int) int {
	return baseFrameRows + zoneCount + pickerRows + memberCount
}

func (m *Model) currentVisibleHourCount() int {
	return m.hourCount(m.nameColumnWidth(), zoneLabelWidth(m.visibleConfigZones(m.visibleConfigMembers())))
}

func (m *Model) renderMessage(message, footer string) string {
	return strings.Join([]string{
		m.renderLine(m.styles.title.Render("ZoneAware (za)")),
		m.renderLine(m.styles.error.Render(message)),
		m.renderSeparator(),
		m.renderLine(m.styles.muted.Render(footer)),
	}, "\n")
}

func (m *Model) renderSeparator() string {
	return strings.Repeat("─", max(1, m.contentWidth()))
}

func (m *Model) renderLine(content string) string {
	return padANSI(content, m.contentWidth())
}

func (m *Model) composeTimelineRow(left, cells, right string) string {
	content := left + cells
	if right == "" {
		return content
	}

	gap := m.contentWidth() - lipgloss.Width(content) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	return content + strings.Repeat(" ", gap) + right
}

type zoneInfo struct {
	id    string
	label string
}

type visibleMember struct {
	index    int
	timeline schedule.MemberTimeline
}

func (m *Model) visibleConfigMembers() []config.TeamMember {
	members := make([]config.TeamMember, 0, len(m.cfg.Team))
	for _, member := range m.cfg.Team {
		if !m.hiddenMembers[member.Name] {
			members = append(members, member)
		}
	}

	return members
}

func (m *Model) visibleConfigZones(members []config.TeamMember) []zoneInfo {
	seen := make(map[string]bool)
	zones := make([]zoneInfo, 0)
	for _, member := range members {
		if seen[member.Timezone] || m.hiddenZones[member.Timezone] {
			continue
		}
		seen[member.Timezone] = true
		zones = append(zones, zoneInfo{
			id:    member.Timezone,
			label: zoneLabel(member.Timezone, m.renderNow),
		})
	}

	m.sortZonesByCurrentTime(zones)
	return zones
}

func (m *Model) referenceDisplay() string {
	location, err := time.LoadLocation(m.cfg.ReferenceTimezone)
	if err != nil {
		return m.cfg.ReferenceTimezone
	}

	return fmt.Sprintf("%s (%s)", m.cfg.ReferenceTimezone, m.renderNow.In(location).Format("MST"))
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

func zoneLabelWidth(zones []zoneInfo) int {
	width := 0
	for _, zone := range zones {
		width = max(width, lipgloss.Width(zone.label))
	}

	return width
}

func (m *Model) renderHourCells(start time.Time, hours int, location, referenceLocation *time.Location) string {
	var builder strings.Builder
	for idx := 0; idx < hours; idx++ {
		slotStart := start.Add(time.Duration(idx) * time.Hour)
		hour := slotStart.In(location)
		separator := m.cellSeparator(start, idx, hours, referenceLocation)
		style := m.styles.info
		if m.isCurrentHour(slotStart) {
			style = m.styles.currentHeader
		}
		builder.WriteString(style.Render(fmt.Sprintf("%02d%s", hour.Hour(), separator)))
	}

	return builder.String()
}

func (m *Model) cellSeparator(start time.Time, index, hours int, referenceLocation *time.Location) string {
	if index >= hours-1 {
		return " "
	}

	current := start.Add(time.Duration(index) * time.Hour).In(referenceLocation)
	next := start.Add(time.Duration(index+1) * time.Hour).In(referenceLocation)
	if current.Day() != next.Day() || current.Month() != next.Month() || current.Year() != next.Year() {
		return "│"
	}

	return " "
}

func (m *Model) isCurrentHour(slotStart time.Time) bool {
	return !m.renderNow.Before(slotStart) && m.renderNow.Before(slotStart.Add(time.Hour))
}

func (m *Model) sortZonesByCurrentTime(zones []zoneInfo) {
	sort.Slice(zones, func(i, j int) bool {
		left := localClockMinutes(zones[i].id, m.renderNow)
		right := localClockMinutes(zones[j].id, m.renderNow)
		if left == right {
			return zones[i].label < zones[j].label
		}
		return left < right
	})
}

func localClockMinutes(timezone string, now time.Time) int {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return 0
	}

	local := now.In(location)
	return local.Hour()*60 + local.Minute()
}

func (m *Model) toggleMemberHour(memberIndex int, slotStart time.Time) error {
	if memberIndex < 0 || memberIndex >= len(m.cfg.Team) {
		return fmt.Errorf("team member index %d is out of range", memberIndex)
	}

	original := cloneConfig(m.cfg)
	if err := m.cfg.Team[memberIndex].ToggleHour(slotStart); err != nil {
		return err
	}

	if m.configPath == "" {
		return nil
	}

	if err := config.Save(m.configPath, m.cfg); err != nil {
		m.cfg = original
		return err
	}

	return nil
}

func (m *Model) editingDisplay(width int) string {
	display := m.editingValue
	if lipgloss.Width(display) < width {
		display += "|"
	}
	return display
}

func cloneConfig(cfg config.Config) config.Config {
	cloned := cfg
	cloned.Team = make([]config.TeamMember, len(cfg.Team))
	for i, member := range cfg.Team {
		cloned.Team[i] = member
		cloned.Team[i].WorkingHours = append([]config.WorkingHours(nil), member.WorkingHours...)
	}

	return cloned
}

func sliceWindow(window schedule.Window, offset, visibleHours int) schedule.Window {
	if offset < 0 {
		offset = 0
	}
	if visibleHours < 0 {
		visibleHours = 0
	}
	if offset > window.Hours {
		offset = window.Hours
	}

	end := min(window.Hours, offset+visibleHours)
	sliced := window
	sliced.Hours = end - offset
	sliced.Start = window.Start.Add(time.Duration(offset) * time.Hour)
	sliced.Members = make([]schedule.MemberTimeline, len(window.Members))
	for i, member := range window.Members {
		sliced.Members[i] = member
		sliced.Members[i].Cells = append([]schedule.Cell(nil), member.Cells[offset:end]...)
	}

	return sliced
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Minute, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *Model) contentWidth() int {
	return max(1, m.width)
}

func padRight(value string, width int) string {
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(value)
}

func padLeft(value string, width int) string {
	if lipgloss.Width(value) >= width {
		return lipgloss.NewStyle().MaxWidth(width).Render(value)
	}

	return strings.Repeat(" ", width-lipgloss.Width(value)) + value
}

func padANSI(value string, width int) string {
	displayWidth := lipgloss.Width(value)
	if displayWidth >= width {
		return lipgloss.NewStyle().MaxWidth(width).Render(value)
	}

	return value + strings.Repeat(" ", width-displayWidth)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
