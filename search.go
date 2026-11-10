package main

import (
	"io/fs"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

type searchMsg struct {
	token     int
	query     string
	results   []Entry
	truncated bool
	err       error
}

func (m *model) beginSearch() tea.Cmd {
	m.search = true
	m.searchResults = false
	m.searching = false
	m.searchTruncated = false
	m.filterLocal = false
	m.filter = ""
	m.searchToken++
	m.clearSelection()
	m.entries = m.all
	m.selected = 0
	m.scroll = 0
	m.focus = focusGrid
	m.ensureVisible()
	return m.thumbCmds()
}

func (m *model) closeSearch() tea.Cmd {
	m.search = false
	m.searchResults = false
	m.searching = false
	m.searchTruncated = false
	m.filter = ""
	m.searchToken++
	m.clearSelection()
	m.entries = m.all
	m.selected = 0
	m.scroll = 0
	m.ensureVisible()
	return m.thumbCmds()
}

func (m *model) beginLocalFilter() tea.Cmd {
	m.filterLocal = true
	m.search = false
	m.searchResults = false
	m.searching = false
	m.filter = ""
	m.clearSelection()
	m.entries = m.all
	m.selected = 0
	m.scroll = 0
	m.focus = focusGrid
	m.ensureVisible()
	return m.thumbCmds()
}

func (m *model) closeLocalFilter() tea.Cmd {
	m.filterLocal = false
	m.filter = ""
	m.clearSelection()
	m.entries = m.all
	m.selected = 0
	m.scroll = 0
	m.ensureVisible()
	return m.thumbCmds()
}

func (m *model) applyLocalFilter() tea.Cmd {
	if m.filter == "" {
		m.entries = m.all
	} else {
		needle := strings.ToLower(m.filter)
		filtered := make([]Entry, 0, len(m.all))
		for _, entry := range m.all {
			if strings.Contains(strings.ToLower(entry.Name), needle) {
				filtered = append(filtered, entry)
			}
		}
		m.entries = filtered
	}
	m.clearSelection()
	m.selected = 0
	m.scroll = 0
	m.ensureVisible()
	return m.thumbCmds()
}

func (m *model) handleFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		return m, m.closeLocalFilter()
	case "enter":
		entry := m.selectedEntry()
		if entry == nil {
			return m, nil
		}
		target := *entry
		m.closeLocalFilter()
		if target.IsDir {
			return m, m.loadDir(target.Path, true, "")
		}
		if err := launch(target.Path); err != nil {
			m.notice = err.Error()
		}
		return m, nil
	case "backspace":
		if m.filter != "" {
			_, size := utf8.DecodeLastRuneInString(m.filter)
			m.filter = m.filter[:len(m.filter)-size]
		}
		return m, m.applyLocalFilter()
	case "ctrl+u":
		m.filter = ""
		return m, m.applyLocalFilter()
	case "up":
		m.moveSelection(-m.gridCols())
	case "down":
		m.moveSelection(m.gridCols())
	case "left":
		m.moveSelection(-1)
	case "right":
		m.moveSelection(1)
	case "pgup", "pageup":
		m.moveSelection(-m.visibleRows() * m.gridCols())
	case "pgdown", "pagedown":
		m.moveSelection(m.visibleRows() * m.gridCols())
	default:
		if text := msg.Key().Text; text != "" && len(m.filter) < 120 {
			m.filter += text
			return m, m.applyLocalFilter()
		}
	}
	m.ensureVisible()
	return m, m.thumbCmds()
}

func (m *model) queueSearch() tea.Cmd {
	m.searchToken++
	if m.filter == "" {
		m.searchResults = false
		m.searching = false
		m.searchTruncated = false
		m.entries = m.all
		m.selected = 0
		m.scroll = 0
		m.ensureVisible()
		return m.thumbCmds()
	}
	m.searching = true
	m.searchResults = true
	m.searchTruncated = false
	token := m.searchToken
	query := m.filter
	root := m.cwd
	hidden := m.hidden
	return func() tea.Msg {
		time.Sleep(150 * time.Millisecond)
		results, truncated, err := walkSearch(root, query, hidden, 500)
		return searchMsg{token: token, query: query, results: results, truncated: truncated, err: err}
	}
}

func (m *model) handleSearchMsg(msg searchMsg) (tea.Model, tea.Cmd) {
	if !m.search || msg.token != m.searchToken || msg.query != m.filter {
		return m, nil
	}
	m.searching = false
	if msg.err != nil {
		m.notice = "Search failed"
		return m, nil
	}
	m.searchResults = true
	m.searchTruncated = msg.truncated
	m.clearSelection()
	m.entries = msg.results
	m.selected = 0
	m.scroll = 0
	m.ensureVisible()
	return m, m.thumbCmds()
}

func walkSearch(root, query string, hidden bool, limit int) ([]Entry, bool, error) {
	needle := strings.ToLower(query)
	var results []Entry
	truncated := false
	err := filepath.WalkDir(root, func(path string, item fs.DirEntry, err error) error {
		if err != nil {
			if item != nil && item.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}
		name := item.Name()
		if !hidden && strings.HasPrefix(name, ".") {
			if item.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if item.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if item.IsDir() {
			// Do not descend into build/dependency directories unless requested.
			lower := strings.ToLower(name)
			if !hidden && (lower == "node_modules" || lower == ".git" || lower == "vendor" || lower == "target") {
				return filepath.SkipDir
			}
		}
		if !strings.Contains(strings.ToLower(name), needle) {
			return nil
		}
		info, err := item.Info()
		if err != nil {
			return nil
		}
		isDir := info.IsDir()
		if !isDir && !info.Mode().IsRegular() {
			return nil
		}
		ext := ""
		if index := strings.LastIndexByte(name, '.'); index >= 0 {
			ext = strings.ToLower(name[index+1:])
		}
		results = append(results, Entry{
			Path:    path,
			Name:    name,
			Ext:     ext,
			IsDir:   isDir,
			IsExec:  info.Mode()&0o111 != 0 && !isDir,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Kind:    classify(name, isDir, info.Mode()&0o111 != 0 && !isDir),
		})
		if len(results) >= limit {
			truncated = true
			return filepath.SkipAll
		}
		return nil
	})
	sortEntries(results)
	return results, truncated, err
}

func (m *model) entryLabel(entry Entry) string {
	if m.searchResults && m.filter != "" {
		relative, err := filepath.Rel(m.cwd, entry.Path)
		if err == nil {
			return relative
		}
	}
	return entry.Name
}
