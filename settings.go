package main

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-runewidth"
)

type settingKind int

const (
	settingHeader settingKind = iota
	settingColor
	settingToggle
	settingChoice
	settingNumber
)

type settingRow struct {
	label    string
	kind     settingKind
	options  []string
	get      func() string
	set      func(string) bool
	reload   bool
	protocol bool
}

type settingsState struct {
	cursor   int
	scroll   int
	dirty    bool
	snapshot Config
}

func settingRows() []settingRow {
	rows := []settingRow{{label: "THEME", kind: settingHeader}}
	addColor := func(label string, value *string) {
		rows = append(rows, settingRow{
			label: label,
			kind:  settingColor,
			get:   func() string { return *value },
			set: func(input string) bool {
				color, ok := parseHex(input)
				if !ok {
					return false
				}
				*value = fmt.Sprintf("#%02x%02x%02x", color.R, color.G, color.B)
				return true
			},
		})
	}
	addColor("Background", &appConfig.Theme.Background)
	addColor("Sidebar", &appConfig.Theme.Sidebar)
	addColor("Top bar", &appConfig.Theme.Topbar)
	addColor("Status bar", &appConfig.Theme.Status)
	addColor("Panel", &appConfig.Theme.Panel)
	addColor("Accent", &appConfig.Theme.Accent)
	addColor("Selected", &appConfig.Theme.Selected)
	addColor("Text", &appConfig.Theme.Text)
	addColor("Muted", &appConfig.Theme.Muted)
	addColor("Faint", &appConfig.Theme.Faint)
	addColor("Dimmer", &appConfig.Theme.Dimmer)
	addColor("Hover", &appConfig.Theme.Hover)
	addColor("Sidebar active", &appConfig.Theme.SidebarActive)
	addColor("Sidebar text", &appConfig.Theme.SidebarText)
	addColor("Search background", &appConfig.Theme.SearchBackground)

	rows = append(rows, settingRow{label: "BEHAVIOR", kind: settingHeader})
	addToggle := func(label string, value *bool) {
		rows = append(rows, settingRow{
			label:  label,
			kind:   settingToggle,
			reload: true,
			get: func() string {
				if *value {
					return "on"
				}
				return "off"
			},
			set: func(string) bool {
				*value = !*value
				return true
			},
		})
	}
	addToggle("Show hidden", &appConfig.Behavior.ShowHidden)
	addChoice := func(label string, value *string, options []string, reload bool) {
		rows = append(rows, settingRow{
			label:   label,
			kind:    settingChoice,
			options: options,
			reload:  reload,
			get:     func() string { return *value },
			set: func(input string) bool {
				*value = input
				return true
			},
		})
	}
	addChoice("Sort", &appConfig.Behavior.Sort, []string{"name", "size", "modified"}, true)
	addToggle("Folders first", &appConfig.Behavior.DirectoriesFirst)
	addToggle("Reverse sort", &appConfig.Behavior.SortReverse)

	rows = append(rows, settingRow{label: "LAYOUT", kind: settingHeader})
	addNumber := func(label string, value *int, minimum, maximum int) {
		rows = append(rows, settingRow{
			label: label,
			kind:  settingNumber,
			get:   func() string { return strconv.Itoa(*value) },
			set: func(input string) bool {
				number, err := strconv.Atoi(input)
				if err != nil {
					return false
				}
				if number < minimum {
					number = minimum
				}
				if number > maximum {
					number = maximum
				}
				*value = number
				return true
			},
		})
	}
	addNumber("Sidebar width", &appConfig.Layout.SidebarWidth, 14, 32)
	addNumber("Card width", &appConfig.Layout.CardWidth, 25, 40)
	addNumber("Gap", &appConfig.Layout.Gap, 1, 4)

	rows = append(rows, settingRow{label: "IMAGES", kind: settingHeader})
	rows = append(rows, settingRow{
		label:    "Protocol",
		kind:     settingChoice,
		options:  []string{"auto", "tgp", "halfblock"},
		reload:   true,
		protocol: true,
		get:      func() string { return appConfig.Images.Protocol },
		set: func(input string) bool {
			appConfig.Images.Protocol = input
			return true
		},
	})
	return rows
}

func (m *model) openSettings() {
	m.menu = nil
	m.dialog = nil
	m.help = false
	state := &settingsState{snapshot: appConfig}
	rows := settingRows()
	for index, row := range rows {
		if row.kind != settingHeader {
			state.cursor = index
			break
		}
	}
	m.settings = state
	m.notice = ""
}

func (m *model) moveSettingsCursor(delta int) {
	state := m.settings
	if state == nil {
		return
	}
	rows := settingRows()
	if len(rows) == 0 {
		return
	}
	for step := 0; step < len(rows); step++ {
		state.cursor = (state.cursor + delta + len(rows)) % len(rows)
		if rows[state.cursor].kind != settingHeader {
			return
		}
	}
}

func (m *model) changeSetting(row settingRow, delta int) tea.Cmd {
	switch row.kind {
	case settingToggle:
		row.set("")
	case settingChoice:
		current := row.get()
		index := 0
		for optionIndex, option := range row.options {
			if option == current {
				index = optionIndex
				break
			}
		}
		index = (index + delta + len(row.options)) % len(row.options)
		row.set(row.options[index])
	case settingNumber:
		number, _ := strconv.Atoi(row.get())
		row.set(strconv.Itoa(number + delta))
	default:
		return nil
	}
	return m.settingChanged(row)
}

func (m *model) settingChanged(row settingRow) tea.Cmd {
	if m.settings != nil {
		m.settings.dirty = true
	}
	applyConfig(appConfig)
	m.hidden = appConfig.Behavior.ShowHidden
	sidebarW = responsiveSidebarWidth(m.width, preferredSidebarW)
	if sidebarW == 0 && m.focus == focusSidebar {
		m.focus = focusGrid
	}
	if row.kind == settingColor {
		m.colorsChanged()
		return nil
	}
	if row.protocol {
		next := detectProtocol()
		if next != m.protocol {
			m.protocol = next
			m.gen++
			m.thumbs = map[string]*Thumb{}
			m.pending = map[string]bool{}
			m.tried = map[string]bool{}
		}
	}
	if row.reload {
		return m.reload()
	}
	m.ensureVisible()
	return m.thumbCmds()
}

func (m *model) colorsChanged() {
	m.places = buildPlaces()
	m.devices = buildDevices()
}

func (m *model) applySettingColor(label, value string) bool {
	for _, row := range settingRows() {
		if row.kind != settingColor || row.label != label {
			continue
		}
		if !row.set(value) {
			return false
		}
		m.settingChanged(row)
		return true
	}
	return false
}

func (m *model) cancelSettings() tea.Cmd {
	state := m.settings
	if state == nil {
		return nil
	}
	m.settings = nil
	if !state.dirty {
		return nil
	}
	appConfig = state.snapshot
	applyConfig(appConfig)
	m.hidden = appConfig.Behavior.ShowHidden
	m.colorsChanged()
	next := detectProtocol()
	if next != m.protocol {
		m.protocol = next
		m.gen++
		m.thumbs = map[string]*Thumb{}
		m.pending = map[string]bool{}
		m.tried = map[string]bool{}
	}
	sidebarW = responsiveSidebarWidth(m.width, preferredSidebarW)
	if sidebarW == 0 && m.focus == focusSidebar {
		m.focus = focusGrid
	}
	return m.reload()
}

func (m *model) saveSettings() tea.Cmd {
	state := m.settings
	if state == nil {
		return nil
	}
	if !state.dirty {
		m.settings = nil
		return nil
	}
	if err := saveConfig(); err != nil {
		m.notice = "Could not save settings: " + err.Error()
		return nil
	}
	m.settings = nil
	m.notice = "Settings saved"
	return nil
}

func (m *model) handleSettingsKey(msg tea.KeyPressMsg) tea.Cmd {
	state := m.settings
	if state == nil {
		return nil
	}
	rows := settingRows()
	if len(rows) == 0 {
		return nil
	}
	if state.cursor < 0 || state.cursor >= len(rows) || rows[state.cursor].kind == settingHeader {
		state.cursor = 0
		m.moveSettingsCursor(1)
	}
	switch msg.String() {
	case "esc", "q":
		return m.cancelSettings()
	case "ctrl+s", "ctrl+shift+s", "s":
		return m.saveSettings()
	case "up", "k":
		m.moveSettingsCursor(-1)
	case "down", "j":
		m.moveSettingsCursor(1)
	case "left", "h":
		return m.changeSetting(rows[state.cursor], -1)
	case "right", "l":
		return m.changeSetting(rows[state.cursor], 1)
	case "enter", " ", "space":
		row := rows[state.cursor]
		if row.kind == settingColor {
			m.dialog = &dialogState{
				kind:    "prompt",
				action:  "setting_color",
				title:   row.label + " color",
				value:   row.get(),
				target:  row.label,
				replace: true,
			}
			return nil
		}
		return m.changeSetting(row, 1)
	}
	return nil
}

func (m *model) drawSettings(canvas *Canvas) {
	state := m.settings
	if state == nil {
		return
	}
	rows := settingRows()
	width := 64
	if width > m.width-2 {
		width = m.width - 2
	}
	if width < 16 {
		width = 16
	}
	height := len(rows) + 6
	if height > m.height-2 {
		height = m.height - 2
	}
	if height < 10 {
		height = 10
	}
	x := (m.width - width) / 2
	y := (m.height - height) / 2
	canvas.dimOutside(x, y, width, height)
	canvas.fillRect(x, y, width, height, colPanel)
	canvas.box(x, y, width, height, colAccent, colPanel)
	title := "fleek settings"
	if state.dirty {
		title += "  ·  unsaved"
	}
	canvas.text(x+2, y+1, width-4, title, colText, colPanel)

	contentStart := y + 3
	footerY := y + height - 2
	available := footerY - contentStart - 1
	if available < 1 {
		available = 1
	}
	if state.cursor < state.scroll {
		state.scroll = state.cursor
	}
	if state.cursor >= state.scroll+available {
		state.scroll = state.cursor - available + 1
	}
	maxOffset := len(rows) - available
	if maxOffset < 0 {
		maxOffset = 0
	}
	if state.scroll > maxOffset {
		state.scroll = maxOffset
	}
	if state.scroll < 0 {
		state.scroll = 0
	}

	for index := state.scroll; index < len(rows) && index < state.scroll+available; index++ {
		row := rows[index]
		rowY := contentStart + index - state.scroll
		if row.kind == settingHeader {
			canvas.text(x+2, rowY, width-4, row.label, colFaint, colPanel)
			continue
		}
		background := colPanel
		if index == state.cursor {
			background = colSidebarActive
		}
		canvas.fillRect(x+1, rowY, width-2, 1, background)
		if index == state.cursor {
			canvas.text(x+1, rowY, 1, "›", colAccent, background)
		}
		value := row.get()
		valueColor := colMuted
		if index == state.cursor {
			valueColor = colAccent
		}
		valueWidth := runewidth.StringWidth(value)
		valueX := x + width - 2 - valueWidth
		labelWidth := valueX - (x + 2) - 1
		if row.kind == settingColor {
			labelWidth -= 3
		}
		if labelWidth < 1 {
			labelWidth = 1
		}
		canvas.text(x+2, rowY, labelWidth, row.label, colText, background)
		if valueX > x+2 {
			canvas.text(valueX, rowY, valueWidth, value, valueColor, background)
		}
		if row.kind == settingColor {
			swatchX := valueX - 3
			if swatchX > x+2 {
				canvas.fillRect(swatchX, rowY, 2, 1, hexRGB(value))
			}
		}
	}

	canvas.text(x+2, footerY-1, width-4, strings.Repeat("─", width-4), colDimmer, colPanel)
	hint := "↑↓ move · ←→ change · enter edit · ctrl+s save · esc cancel"
	switch {
	case width < 32:
		hint = "ctrl+s · esc"
	case width < 38:
		hint = "↑↓ move · ctrl+s save · esc"
	case width < 63:
		hint = "↑↓ move · ctrl+s save · esc cancel"
	}
	canvas.centerText(x+2, footerY, width-4, hint, colFaint, colPanel)
}
