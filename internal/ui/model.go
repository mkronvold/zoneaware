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
	minWidth      = 48
	cellWidth     = 3
	minNameWidth  = 8
	baseFrameRows = 8
)

type hotspotKind int

const (
	hotspotMember hotspotKind = iota
	hotspotTimezone
	hotspotMemberTimezone
	hotspotHoursHeader
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
	half      int
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
	rowEditing       lipgloss.Style
	availableHalf    lipgloss.Style
	offHoursHalf     lipgloss.Style
	currentHalf      lipgloss.Style
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
	hiddenHours     bool
	hotspots        []hotspot
	styles          styles
	lastError       string
	timezoneOptions []string
	pickerOpen      bool
	pickerFilter    string
	pickerSelection int
	pickerOffset    int
	pickerTarget    int
	editingIndex    int
	editingValue    string
	startupAligned  bool
}

func NewModel(cfg config.Config, configPath string, now func() time.Time) *Model {
	return &Model{
		cfg:            cfg,
		configPath:     configPath,
		now:            now,
		width:          100,
		height:         20,
		hiddenMembers:  make(map[string]bool),
		hiddenZones:    make(map[string]bool),
		pickerOffset:   0,
		pickerTarget:   -1,
		editingIndex:   -1,
		startupAligned: false,
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
			rowEditing:       lipgloss.NewStyle().Background(lipgloss.Color("236")),
			availableHalf:    lipgloss.NewStyle().Foreground(lipgloss.Color("108")),
			offHoursHalf:     lipgloss.NewStyle().Foreground(lipgloss.Color("239")),
			currentHalf:      lipgloss.NewStyle().Foreground(lipgloss.Color("217")).Bold(true),
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
			m.hiddenHours = false
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
			if m.pickerOpen {
				m.movePicker(-1)
			} else {
				m.scrollTime(-1)
			}
		case tea.MouseWheelDown:
			if m.pickerOpen {
				m.movePicker(1)
			} else {
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
	labelWidth := max(zoneLabelWidth(visibleZones), teamMemberZoneLabelWidth(visibleConfigMembers, m.renderNow))
	hoursWidth := m.hoursColumnWidth(visibleConfigMembers)
	visibleHours := m.hourCount(nameWidth, rightColumnWidth(labelWidth, hoursWidth))
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

	if !m.startupAligned {
		m.timeOffset = m.defaultStartupOffset(fullWindow.Start, fullWindow.Hours, visibleHours, referenceLocation)
		m.startupAligned = true
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
	labelWidth = max(zoneLabelWidth(visibleZones), visibleMemberZoneLabelWidth(visibleRows, m.renderNow))
	requiredHeight := m.requiredHeight(len(visibleRows), len(visibleZones), hoursWidth > 0)
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
	lines = append(lines, m.renderSeparator())
	currentY++

	for _, zone := range visibleZones {
		lines = append(lines, m.renderLine(m.renderZoneHeaderRow(nameWidth, labelWidth, hoursWidth, window, zone, currentY, referenceLocation)))
		currentY++
	}
	if hoursWidth > 0 {
		lines = append(lines, m.renderLine(m.renderHoursHeaderRow(nameWidth, labelWidth, hoursWidth, window, currentY)))
		currentY++
	}

	lines = append(lines, m.renderLine(m.renderTimelineDivider(nameWidth, window, referenceLocation)))
	currentY++

	for idx := 0; idx < len(visibleRows); idx++ {
		lineY := currentY + idx
		lines = append(lines, m.renderLine(m.renderMemberRow(nameWidth, labelWidth, hoursWidth, visibleRows[idx], lineY, referenceLocation)))
	}
	currentY += len(visibleRows)

	lines = append(lines, m.renderLine(m.styles.muted.Render("Click a cell to toggle that hour · click a name to edit it · click a header timezone or Hours to hide it · click a member timezone or Local Reference to change it · wheel scrolls time horizontally · Ctrl+L refresh · r resets · q quits")))
	currentY++
	lines = append(lines, m.renderSeparator())
	currentY++
	lines = append(lines, m.renderLine(m.renderHiddenLine()))
	currentY++
	lines = append(lines, m.renderLine(m.renderFooterActions(currentY)))

	if m.pickerOpen {
		lines = m.overlayReferencePicker(lines, 1)
	}

	return strings.Join(lines, "\n")
}

func (m *Model) updatePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.pickerOpen = false
		m.pickerFilter = ""
		m.pickerSelection = 0
		m.pickerOffset = 0
		m.pickerTarget = -1
		return m, nil
	case "ctrl+l":
		m.lastError = ""
		return m, tea.ClearScreen
	case "enter":
		options := m.filteredTimezones()
		if len(options) == 0 {
			return m, nil
		}
		if m.pickerSelection >= len(options) {
			m.pickerSelection = len(options) - 1
		}
		if err := m.selectPickedTimezone(options[m.pickerSelection]); err != nil {
			m.lastError = err.Error()
		}
		return m, tea.ClearScreen
	case "up":
		m.movePicker(-1)
		return m, nil
	case "down":
		m.movePicker(1)
		return m, nil
	case "backspace", "ctrl+h":
		if m.pickerFilter != "" {
			runes := []rune(m.pickerFilter)
			m.pickerFilter = string(runes[:len(runes)-1])
			m.pickerSelection = 0
			m.pickerOffset = 0
		}
		return m, nil
	}

	if len(msg.Runes) > 0 {
		m.pickerFilter += string(msg.Runes)
		m.pickerSelection = 0
		m.pickerOffset = 0
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
	lines := make([]string, 0, m.pickerVisibleRows())
	filter := m.pickerFilter
	if filter == "" {
		filter = "type to filter timezones"
	}
	lines = append(lines, m.renderLine(m.styles.pickerInput.Render("Timezone filter: "+filter)))

	filtered := m.filteredTimezones()
	if len(filtered) == 0 {
		lines = append(lines, m.renderLine(m.styles.muted.Render("No matching timezones")))
		return lines
	}

	if m.pickerSelection >= len(filtered) {
		m.pickerSelection = len(filtered) - 1
	}
	options := m.displayedTimezones()

	for idx, zone := range options {
		style := m.styles.pickerOption
		absoluteIndex := m.pickerOffset + idx
		if absoluteIndex == m.pickerSelection {
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

func (m *Model) overlayReferencePicker(lines []string, startY int) []string {
	overlay := m.renderReferencePicker(startY)
	if len(lines) < m.height {
		for len(lines) < m.height {
			lines = append(lines, m.renderLine(""))
		}
	}
	for index, line := range overlay {
		target := startY + index
		if target >= len(lines) {
			lines = append(lines, line)
			continue
		}
		lines[target] = line
	}
	return lines
}

func (m *Model) renderZoneHeaderRow(nameWidth, labelWidth, hoursWidth int, window schedule.Window, zone zoneInfo, lineY int, referenceLocation *time.Location) string {
	location, err := time.LoadLocation(zone.id)
	if err != nil {
		location = window.Start.Location()
	}

	left := strings.Repeat(" ", nameWidth+1)
	cells := m.renderZoneHeaderCells(window.Start, window.Hours, location)
	label := m.renderRightColumns(
		m.styles.timezone.Render(padLeft(zone.label, labelWidth)),
		strings.Repeat(" ", hoursWidth),
	)
	rightWidth := rightColumnWidth(labelWidth, hoursWidth)
	m.hotspots = append(m.hotspots, hotspot{
		kind:  hotspotTimezone,
		value: zone.id,
		x1:    m.contentWidth() - rightWidth,
		x2:    m.contentWidth() - rightWidth + labelWidth,
		y:     lineY,
	})

	return m.composeTimelineRow(left, cells, label)
}

func (m *Model) renderHoursHeaderRow(nameWidth, labelWidth, hoursWidth int, window schedule.Window, lineY int) string {
	left := strings.Repeat(" ", nameWidth+1)
	cells := strings.Repeat(" ", window.Hours*cellWidth)
	label := m.renderRightColumns(
		strings.Repeat(" ", labelWidth),
		m.styles.timezone.Render(padLeft("Hours", hoursWidth)),
	)
	m.hotspots = append(m.hotspots, hotspot{
		kind: hotspotHoursHeader,
		x1:   m.contentWidth() - hoursWidth,
		x2:   m.contentWidth(),
		y:    lineY,
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

func (m *Model) renderMemberRow(nameWidth, labelWidth, hoursWidth int, row visibleMember, lineY int, referenceLocation *time.Location) string {
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
		x2:    lipgloss.Width(builder.String()),
		y:     lineY,
	})

	for idx, cell := range row.timeline.Cells {
		cellX1 := lipgloss.Width(builder.String())
		separator := m.cellSeparator(row.timeline.Cells[0].Start, idx, len(row.timeline.Cells), referenceLocation)
		leftText := m.renderHalfCell(cell.Start, 0, cell.Halves[0])
		rightText := m.renderHalfCell(cell.Start, 1, cell.Halves[1])
		cellText := leftText + rightText + separator

		m.hotspots = append(m.hotspots, hotspot{
			kind:      hotspotCell,
			value:     row.timeline.Name,
			index:     row.index,
			half:      0,
			slotStart: cell.Start,
			x1:        cellX1,
			x2:        cellX1 + lipgloss.Width(leftText),
			y:         lineY,
		})
		m.hotspots = append(m.hotspots, hotspot{
			kind:      hotspotCell,
			value:     row.timeline.Name,
			index:     row.index,
			half:      1,
			slotStart: cell.Start,
			x1:        cellX1 + lipgloss.Width(leftText),
			x2:        cellX1 + lipgloss.Width(leftText) + lipgloss.Width(rightText),
			y:         lineY,
		})
		builder.WriteString(cellText)
	}

	rightLabel := ""
	rightWidth := rightColumnWidth(labelWidth, hoursWidth)
	if labelWidth > 0 {
		timezoneText := m.styles.timezone.Render(padLeft(zoneLabel(row.timeline.Timezone, m.renderNow), labelWidth))
		hoursText := ""
		if hoursWidth > 0 {
			hoursText = m.styles.info.Render(padLeft(memberScheduleHoursLabel(m.cfg.Team[row.index]), hoursWidth))
		}
		rightLabel = m.renderRightColumns(timezoneText, hoursText)
		m.hotspots = append(m.hotspots, hotspot{
			kind:  hotspotMemberTimezone,
			value: row.timeline.Timezone,
			index: row.index,
			x1:    m.contentWidth() - rightWidth,
			x2:    m.contentWidth() - rightWidth + labelWidth,
			y:     lineY,
		})
	} else if hoursWidth > 0 {
		rightLabel = m.styles.info.Render(padLeft(memberScheduleHoursLabel(m.cfg.Team[row.index]), hoursWidth))
	}

	rowText := m.composeTimelineRow("", builder.String(), rightLabel)
	if row.index == m.editingIndex {
		return m.styles.rowEditing.Render(padANSI(rowText, m.contentWidth()))
	}

	return rowText
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
	if m.hiddenHours {
		items = append(items, "Hours")
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
	for i := len(m.hotspots) - 1; i >= 0; i-- {
		spot := m.hotspots[i]
		if y != spot.y || x < spot.x1 || x >= spot.x2 {
			continue
		}

		switch spot.kind {
		case hotspotMember:
			if m.editingIndex == spot.index {
				if err := m.commitEditMember(); err != nil {
					m.lastError = err.Error()
				}
				return
			}
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
		case hotspotHoursHeader:
			m.hiddenHours = !m.hiddenHours
			m.clampTimeOffset(config.EffectiveWindowHours(m.cfg), m.currentVisibleHourCount())
		case hotspotCell:
			if m.editingIndex >= 0 {
				return
			}
			if err := m.toggleMemberHalfHour(spot.index, spot.slotStart, spot.half); err != nil {
				m.lastError = err.Error()
			} else {
				m.lastError = ""
			}
		case hotspotShowAll:
			m.hiddenMembers = make(map[string]bool)
			m.hiddenZones = make(map[string]bool)
			m.hiddenHours = false
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
	m.pickerOffset = 0
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
	m.pickerOffset = 0
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
	m.pickerOffset = 0
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
	maxOptions := m.pickerVisibleOptions()
	if len(options) <= maxOptions {
		return options
	}

	maxOffset := len(options) - maxOptions
	if m.pickerOffset < 0 {
		m.pickerOffset = 0
	}
	if m.pickerOffset > maxOffset {
		m.pickerOffset = maxOffset
	}

	end := min(len(options), m.pickerOffset+maxOptions)
	return options[m.pickerOffset:end]
}

func (m *Model) pickerVisibleRows() int {
	if !m.pickerOpen {
		return 0
	}

	optionCount := len(m.filteredTimezones())
	if optionCount == 0 {
		return 2
	}
	maxOptions := m.pickerVisibleOptions()
	if optionCount > maxOptions {
		optionCount = maxOptions
	}
	return 1 + optionCount
}

func (m *Model) pickerVisibleOptions() int {
	return max(1, m.height-2)
}

func (m *Model) movePicker(delta int) {
	options := m.filteredTimezones()
	if len(options) == 0 {
		m.pickerSelection = 0
		m.pickerOffset = 0
		return
	}

	m.pickerSelection += delta
	if m.pickerSelection < 0 {
		m.pickerSelection = 0
	}
	if m.pickerSelection >= len(options) {
		m.pickerSelection = len(options) - 1
	}

	if m.pickerSelection < m.pickerOffset {
		m.pickerOffset = m.pickerSelection
	}
	maxOptions := m.pickerVisibleOptions()
	if m.pickerSelection >= m.pickerOffset+maxOptions {
		m.pickerOffset = m.pickerSelection - maxOptions + 1
	}
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

func (m *Model) defaultStartupOffset(start time.Time, totalHours, visibleHours int, referenceLocation *time.Location) int {
	current := start.In(referenceLocation)
	target := time.Date(current.Year(), current.Month(), current.Day(), 7, 0, 0, 0, referenceLocation)
	if target.Before(current) {
		target = target.Add(24 * time.Hour)
	}

	offset := int(target.Sub(current).Hours())
	maxOffset := max(0, totalHours-visibleHours)
	if offset < 0 {
		return 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
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

func (m *Model) hourCount(nameWidth, rightWidth int) int {
	usable := m.contentWidth() - nameWidth - 1
	if rightWidth > 0 {
		usable -= rightWidth + 1
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

func (m *Model) requiredHeight(memberCount, zoneCount int, showHoursHeader bool) int {
	height := baseFrameRows + zoneCount + memberCount
	if showHoursHeader {
		height++
	}
	return height
}

func (m *Model) currentVisibleHourCount() int {
	visibleMembers := m.visibleConfigMembers()
	labelWidth := max(zoneLabelWidth(m.visibleConfigZones(visibleMembers)), teamMemberZoneLabelWidth(visibleMembers, m.renderNow))
	hoursWidth := m.hoursColumnWidth(visibleMembers)
	return m.hourCount(m.nameColumnWidth(), rightColumnWidth(labelWidth, hoursWidth))
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

func teamMemberZoneLabelWidth(members []config.TeamMember, now time.Time) int {
	width := 0
	for _, member := range members {
		width = max(width, lipgloss.Width(zoneLabel(member.Timezone, now)))
	}
	return width
}

func visibleMemberZoneLabelWidth(members []visibleMember, now time.Time) int {
	width := 0
	for _, member := range members {
		width = max(width, lipgloss.Width(zoneLabel(member.timeline.Timezone, now)))
	}
	return width
}

func (m *Model) hoursColumnWidth(members []config.TeamMember) int {
	if m.hiddenHours {
		return 0
	}
	return memberScheduleHoursWidth(members)
}

func memberScheduleHoursWidth(members []config.TeamMember) int {
	width := 0
	for _, member := range members {
		width = max(width, lipgloss.Width(memberScheduleHoursLabel(member)))
	}
	return width
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

	return formatQuarterHours(quarters)
}

func formatQuarterHours(quarters int) string {
	wholeHours := quarters / 4
	switch quarters % 4 {
	case 0:
		return fmt.Sprintf("%dh", wholeHours)
	case 1:
		return fmt.Sprintf("%d.25h", wholeHours)
	case 2:
		return fmt.Sprintf("%d.5h", wholeHours)
	default:
		return fmt.Sprintf("%d.75h", wholeHours)
	}
}

func rightColumnWidth(labelWidth, hoursWidth int) int {
	width := 0
	if labelWidth > 0 {
		width += labelWidth
	}
	if hoursWidth > 0 {
		if width > 0 {
			width++
		}
		width += hoursWidth
	}
	return width
}

func (m *Model) renderRightColumns(left, right string) string {
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	return left + " " + right
}

func (m *Model) renderZoneHeaderCells(start time.Time, hours int, location *time.Location) string {
	var builder strings.Builder
	padding := m.zoneHeaderPadding(start, location)
	for idx := 0; idx < hours; idx++ {
		slotStart := start.Add(time.Duration(idx) * time.Hour)
		hour := slotStart.In(location)
		prefix := ""
		if idx == 0 {
			prefix = padding
		}
		separator := " "
		if idx == hours-1 && padding != "" {
			separator = ""
		}
		style := m.styles.info
		if m.isCurrentHour(slotStart) {
			style = m.styles.currentHeader
		}
		builder.WriteString(style.Render(fmt.Sprintf("%s%02d%s", prefix, hour.Hour(), separator)))
	}

	return builder.String()
}

func (m *Model) zoneHeaderPadding(start time.Time, location *time.Location) string {
	if start.In(location).Minute() == 30 {
		return " "
	}
	return ""
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

func (m *Model) isCurrentHalf(slotStart time.Time, halfIndex int) bool {
	halfStart := slotStart.Add(time.Duration(halfIndex) * 30 * time.Minute)
	return !m.renderNow.Before(halfStart) && m.renderNow.Before(halfStart.Add(30*time.Minute))
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

func (m *Model) toggleMemberHalfHour(memberIndex int, slotStart time.Time, halfIndex int) error {
	if memberIndex < 0 || memberIndex >= len(m.cfg.Team) {
		return fmt.Errorf("team member index %d is out of range", memberIndex)
	}

	original := cloneConfig(m.cfg)
	if err := m.cfg.Team[memberIndex].ToggleHalfHour(slotStart, halfIndex); err != nil {
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

func (m *Model) renderHalfCell(slotStart time.Time, halfIndex int, available bool) string {
	char := "░"
	style := m.styles.offHoursHalf
	if available {
		char = "█"
		style = m.styles.availableHalf
	}
	if m.isCurrentHalf(slotStart, halfIndex) {
		style = m.styles.currentHalf
	}
	return style.Render(char)
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
