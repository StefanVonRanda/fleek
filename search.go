package main

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	m.filter = ""
	m.searchToken++
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
	m.entries = m.all
	m.selected = 0
	m.scroll = 0
	m.ensureVisible()
	return m.thumbCmds()
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
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].IsDir != results[j].IsDir {
			return results[i].IsDir
		}
		return naturalLess(results[i].Path, results[j].Path)
	})
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
