package main

import (
	"fmt"
	"path/filepath"
)

func (m *model) snapshotTab() tabState {
	selection := make(map[string]bool, len(m.selection))
	for path, selected := range m.selection {
		selection[path] = selected
	}
	history := append([]string(nil), m.history...)
	all := append([]Entry(nil), m.all...)
	entries := append([]Entry(nil), m.entries...)
	return tabState{
		cwd:             m.cwd,
		all:             all,
		entries:         entries,
		filter:          m.filter,
		filterLocal:     m.filterLocal,
		search:          m.search,
		searchResults:   m.searchResults,
		searchTruncated: m.searchTruncated,
		selected:        m.selected,
		anchor:          m.anchor,
		scroll:          m.scroll,
		selection:       selection,
		history:         history,
		historyIndex:    m.historyIndex,
		fsType:          m.fsType,
		free:            m.free,
	}
}

func (m *model) restoreTab(state tabState) {
	m.cwd = state.cwd
	m.all = append([]Entry(nil), state.all...)
	m.entries = append([]Entry(nil), state.entries...)
	m.filter = state.filter
	m.filterLocal = state.filterLocal
	m.search = state.search
	m.searchResults = state.searchResults
	m.searching = false
	m.searchTruncated = state.searchTruncated
	m.selected = state.selected
	m.anchor = state.anchor
	m.scroll = state.scroll
	m.selection = make(map[string]bool, len(state.selection))
	for path, selected := range state.selection {
		m.selection[path] = selected
	}
	m.history = append([]string(nil), state.history...)
	m.historyIndex = state.historyIndex
	m.fsType = state.fsType
	m.free = state.free
	m.searchToken++
	m.focus = focusGrid
	m.clampScroll()
	m.ensureVisible()
}

func (m *model) ensureTabs() {
	if len(m.tabs) == 0 {
		m.tabs = []tabState{m.snapshotTab()}
		m.tabIndex = 0
	}
}

func (m *model) saveCurrentTab() {
	m.ensureTabs()
	if m.tabIndex >= 0 && m.tabIndex < len(m.tabs) {
		m.tabs[m.tabIndex] = m.snapshotTab()
	}
}

func (m *model) switchTab(index int) {
	if len(m.tabs) == 0 {
		return
	}
	if index < 0 {
		index = len(m.tabs) - 1
	}
	if index >= len(m.tabs) {
		index = 0
	}
	if index == m.tabIndex {
		return
	}
	m.saveCurrentTab()
	m.tabIndex = index
	m.restoreTab(m.tabs[index])
	m.notice = ""
}

func (m *model) newTab() {
	m.ensureTabs()
	m.saveCurrentTab()
	state := tabState{
		cwd:       m.cwd,
		selection: map[string]bool{},
		history:   []string{m.cwd},
	}
	m.tabs = append(m.tabs, state)
	m.tabIndex = len(m.tabs) - 1
	m.restoreTab(state)
	m.loadDir(state.cwd, false, "")
	m.notice = fmt.Sprintf("Tab %d", m.tabIndex+1)
}

func (m *model) closeTab() {
	m.ensureTabs()
	if len(m.tabs) <= 1 {
		m.notice = "Can't close the last tab"
		return
	}
	m.tabs = append(m.tabs[:m.tabIndex], m.tabs[m.tabIndex+1:]...)
	if m.tabIndex >= len(m.tabs) {
		m.tabIndex = len(m.tabs) - 1
	}
	m.restoreTab(m.tabs[m.tabIndex])
	m.notice = fmt.Sprintf("Tab %d", m.tabIndex+1)
}

func (m *model) nextTab(delta int) {
	m.ensureTabs()
	if len(m.tabs) <= 1 {
		return
	}
	m.switchTab(m.tabIndex + delta)
}

func (m *model) tabTitle(index int) string {
	if index == m.tabIndex && m.cwd != "" {
		return tabPathTitle(m.cwd)
	}
	if index >= 0 && index < len(m.tabs) {
		return tabPathTitle(m.tabs[index].cwd)
	}
	return "~"
}

func tabPathTitle(path string) string {
	if path == userHome() {
		return "~"
	}
	base := filepath.Base(path)
	if base == "" || base == "." {
		return "/"
	}
	return base
}
