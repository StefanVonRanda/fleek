package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-runewidth"
)

func (m *model) setClipboard(cut bool) tea.Cmd {
	entry := m.selectedEntry()
	if entry == nil {
		m.notice = "Nothing selected"
		return nil
	}
	m.clipboard = fileClipboard{paths: []string{entry.Path}, cut: cut}
	if cut {
		m.notice = "Cut " + entry.Name
	} else {
		m.notice = "Copied " + entry.Name
	}
	return nil
}

func (m *model) pasteClipboard() tea.Cmd {
	if m.busy || len(m.clipboard.paths) == 0 {
		if len(m.clipboard.paths) == 0 {
			m.notice = "Clipboard is empty"
		}
		return nil
	}
	m.busy = true
	m.notice = "Pasting…"
	clip := m.clipboard
	destination := m.cwd
	return func() tea.Msg {
		first, err := pasteEntries(clip.paths, destination, clip.cut)
		return opMsg{notice: "Pasted " + filepath.Base(first), selectPath: first, err: err, cut: clip.cut}
	}
}

func (m *model) confirmTrash(entry *Entry) {
	if entry == nil {
		return
	}
	m.menu = nil
	m.dialog = &dialogState{
		kind:    "confirm",
		action:  "trash",
		title:   "Move to Trash",
		message: entry.Name,
		target:  entry.Path,
	}
}

func (m *model) trashPath(path string) tea.Cmd {
	if m.busy {
		return nil
	}
	m.busy = true
	m.notice = "Moving to Trash…"
	return func() tea.Msg {
		err := trashEntry(path)
		return opMsg{notice: "Moved to Trash", err: err}
	}
}

func (m *model) startRename(entry *Entry) {
	if entry == nil {
		return
	}
	m.menu = nil
	m.dialog = &dialogState{
		kind:    "prompt",
		action:  "rename",
		title:   "Rename",
		value:   entry.Name,
		target:  entry.Path,
		replace: true,
	}
}

func (m *model) startNewFolder() {
	m.menu = nil
	m.dialog = &dialogState{
		kind:   "prompt",
		action: "mkdir",
		title:  "New Folder",
		target: m.cwd,
	}
}

func (m *model) openPreview(entry Entry) tea.Cmd {
	if !previewable(entry) {
		return nil
	}
	m.menu = nil
	m.dialog = nil
	m.help = false
	m.previewSeq++
	token := m.previewSeq
	var deleteRaw string
	if m.preview != nil && m.preview.id != 0 {
		deleteRaw = m.kitty.DeleteImage(m.preview.id)
	}
	cellW, cellH := cellPixels()
	availCols := m.width - 8
	availRows := m.height - 6
	if availCols < 10 {
		availCols = 10
	}
	if availRows < 5 {
		availRows = 5
	}
	m.preview = &previewState{entry: entry, loading: true, token: token}
	command := m.previewCmd(entry, token, availCols, availRows, cellW, cellH)
	if deleteRaw != "" {
		return tea.Batch(tea.Raw(deleteRaw), command)
	}
	return command
}

func (m *model) previewCmd(entry Entry, token, availCols, availRows, cellW, cellH int) tea.Cmd {
	return func() tea.Msg {
		preview, err := makePreview(entry, availCols, availRows, cellW, cellH)
		return previewMsg{token: token, preview: preview, err: err}
	}
}

func (m *model) handlePreviewMsg(msg previewMsg) (tea.Model, tea.Cmd) {
	if m.preview == nil || m.preview.token != msg.token {
		return m, nil
	}
	m.preview.loading = false
	if msg.err != nil || msg.preview == nil {
		m.preview.err = "Could not preview this file"
		m.notice = "Preview failed"
		return m, nil
	}
	m.preview.png = msg.preview.PNG
	m.preview.half = msg.preview.Half
	m.preview.cols = msg.preview.Cols
	m.preview.rows = msg.preview.Rows
	if m.protocol == protoTGP {
		id, raw := m.kitty.UploadImage(msg.preview.PNG, msg.preview.Cols, msg.preview.Rows)
		m.preview.id = id
		m.preview.uploaded = raw != ""
		if raw != "" {
			return m, tea.Sequence(tea.Raw(raw), func() tea.Msg { return repaintMsg{} })
		}
	}
	return m, nil
}

func (m *model) closePreview() tea.Cmd {
	if m.preview == nil {
		return nil
	}
	id := m.preview.id
	m.preview = nil
	if id != 0 {
		return tea.Sequence(tea.Raw(m.kitty.DeleteImage(id)), func() tea.Msg { return repaintMsg{} })
	}
	return nil
}

func (m *model) previewBrowse(delta int) tea.Cmd {
	if m.preview == nil {
		return nil
	}
	var entries []Entry
	for _, entry := range m.entries {
		if previewable(entry) {
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 {
		return nil
	}
	current := -1
	for index, entry := range entries {
		if entry.Path == m.preview.entry.Path {
			current = index
			break
		}
	}
	if current < 0 {
		current = 0
	}
	next := current + delta
	if next < 0 {
		next = len(entries) - 1
	}
	if next >= len(entries) {
		next = 0
	}
	return m.openPreview(entries[next])
}

func (m *model) handlePreviewKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "q", " ", "space":
		return m.closePreview()
	case "left", "h":
		return m.previewBrowse(-1)
	case "right", "l":
		return m.previewBrowse(1)
	case "enter":
		if m.preview != nil {
			_ = launch(m.preview.entry.Path)
		}
	}
	return nil
}

func (m *model) openMenuForSelected() {
	for _, hit := range m.cardHits {
		if hit.index == m.selected {
			m.openMenuAt(hit.x+2, hit.y+2)
			return
		}
	}
	m.openMenuAt(sidebarW+4, topH+3)
}

func (m *model) openMenuAt(x, y int) {
	index := -1
	for _, hit := range m.cardHits {
		if x >= hit.x && x < hit.x+hit.width && y >= hit.y && y < hit.y+hit.height {
			index = hit.index
			break
		}
	}
	var target *Entry
	if index >= 0 {
		m.setSelected(index)
		m.ensureVisible()
		entry := m.entries[index]
		target = &entry
	}
	items := m.menuItemsFor(target)
	cursor := 0
	for cursor < len(items) && (items[cursor].separator || !items[cursor].enabled) {
		cursor++
	}
	if cursor >= len(items) {
		cursor = 0
	}
	m.menu = &contextMenu{x: x, y: y, target: target, items: items, cursor: cursor}
}

func (m *model) menuItemsFor(target *Entry) []menuItem {
	var items []menuItem
	if target != nil {
		items = append(items, menuItem{label: "Open", shortcut: "enter", action: "open", enabled: true})
		if previewable(*target) {
			items = append(items, menuItem{label: "Preview", shortcut: "space", action: "preview", enabled: true})
		}
		items = append(items,
			menuItem{label: "Copy", shortcut: "c", action: "copy", enabled: true},
			menuItem{label: "Cut", shortcut: "x", action: "cut", enabled: true},
			menuItem{label: "Rename", shortcut: "F2", action: "rename", enabled: true},
			menuItem{label: "Move to Trash", shortcut: "Del", action: "trash", enabled: true},
			menuItem{separator: true},
		)
	}
	items = append(items,
		menuItem{label: "New folder", shortcut: "n", action: "mkdir", enabled: true},
		menuItem{label: "Paste", shortcut: "v", action: "paste", enabled: len(m.clipboard.paths) > 0 && !m.busy},
		menuItem{label: "Refresh", shortcut: "r", action: "refresh", enabled: true},
	)
	return items
}

func (m *model) handleMenuKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.menu == nil {
		return nil
	}
	key := msg.String()
	switch key {
	case "esc", "q":
		m.menu = nil
		return nil
	case "up", "k":
		m.moveMenuCursor(-1)
		return nil
	case "down", "j":
		m.moveMenuCursor(1)
		return nil
	case "enter":
		return m.runMenuAction(m.menu.items[m.menu.cursor].action)
	case "c":
		return m.runMenuAction("copy")
	case "x":
		return m.runMenuAction("cut")
	case "v":
		return m.runMenuAction("paste")
	case "n":
		return m.runMenuAction("mkdir")
	case "r":
		return m.runMenuAction("refresh")
	}
	return nil
}

func (m *model) moveMenuCursor(delta int) {
	if m.menu == nil {
		return
	}
	for index := 0; index < len(m.menu.items); index++ {
		m.menu.cursor = (m.menu.cursor + delta + len(m.menu.items)) % len(m.menu.items)
		item := m.menu.items[m.menu.cursor]
		if !item.separator && item.enabled {
			return
		}
	}
}

func (m *model) handleMenuClick(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	if m.menu == nil {
		return m, nil
	}
	width := m.menuWidth()
	height := len(m.menu.items) + 2
	x := m.menu.x
	if x+width > m.width-1 {
		x = m.width - width - 1
	}
	if x < 1 {
		x = 1
	}
	y := m.menu.y
	if y+height > m.height-1 {
		y = m.height - height - 1
	}
	if y < 1 {
		y = 1
	}
	for _, hit := range m.menu.hits {
		if mouse.Y == hit.y && mouse.X >= x && mouse.X < x+width && hit.item.enabled && !hit.item.separator {
			return m, m.runMenuAction(hit.item.action)
		}
	}
	m.menu = nil
	return m, nil
}

func (m *model) menuWidth() int {
	width := 32
	if width > m.width-2 {
		width = m.width - 2
	}
	if width < 12 {
		width = 12
	}
	return width
}

func (m *model) runMenuAction(action string) tea.Cmd {
	menu := m.menu
	if menu == nil {
		return nil
	}
	target := menu.target
	m.menu = nil
	switch action {
	case "open":
		if target != nil {
			if target.IsDir {
				return m.loadDir(target.Path, true, "")
			}
			_ = launch(target.Path)
		}
	case "preview":
		if target != nil {
			return m.openPreview(*target)
		}
	case "copy":
		return m.setClipboard(false)
	case "cut":
		return m.setClipboard(true)
	case "rename":
		m.startRename(target)
	case "trash":
		m.confirmTrash(target)
	case "mkdir":
		m.startNewFolder()
	case "paste":
		return m.pasteClipboard()
	case "refresh":
		return m.reload()
	}
	return nil
}

func (m *model) handleDialogKey(msg tea.KeyPressMsg) tea.Cmd {
	dialog := m.dialog
	if dialog == nil {
		return nil
	}
	key := msg.String()
	if dialog.kind == "confirm" {
		switch key {
		case "esc", "n", "q":
			m.dialog = nil
		case "enter", "y":
			path := dialog.target
			m.dialog = nil
			return m.trashPath(path)
		}
		return nil
	}
	switch key {
	case "esc":
		m.dialog = nil
	case "enter":
		return m.applyDialog()
	case "ctrl+u":
		dialog.value = ""
		dialog.replace = false
	case "backspace":
		if dialog.replace {
			dialog.value = ""
			dialog.replace = false
		} else if dialog.value != "" {
			_, size := utf8.DecodeLastRuneInString(dialog.value)
			dialog.value = dialog.value[:len(dialog.value)-size]
		}
	default:
		if text := msg.Key().Text; text != "" {
			if dialog.replace {
				dialog.value = ""
				dialog.replace = false
			}
			if len(dialog.value) < 120 {
				dialog.value += text
			}
		}
	}
	return nil
}

func (m *model) applyDialog() tea.Cmd {
	dialog := m.dialog
	if dialog == nil {
		return nil
	}
	value := strings.TrimSpace(dialog.value)
	if value == "" {
		m.notice = "Name cannot be empty"
		return nil
	}
	switch dialog.action {
	case "rename":
		target, err := renameEntry(dialog.target, value)
		if err != nil {
			m.notice = err.Error()
			return nil
		}
		m.dialog = nil
		m.notice = "Renamed to " + filepath.Base(target)
		return m.reloadSelect(target)
	case "mkdir":
		target, err := makeFolder(dialog.target, value)
		if err != nil {
			m.notice = fmt.Sprintf("Could not create folder: %v", err)
			return nil
		}
		m.dialog = nil
		m.notice = "Created " + filepath.Base(target)
		return m.reloadSelect(target)
	}
	m.dialog = nil
	return nil
}

func (m *model) handleOpMsg(msg opMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	if msg.err != nil {
		m.notice = fmt.Sprintf("Operation failed: %v", msg.err)
		return m, nil
	}
	if msg.cut {
		m.clipboard = fileClipboard{}
	}
	if msg.notice != "" {
		m.notice = msg.notice
	}
	return m, m.reloadSelect(msg.selectPath)
}

func (m *model) reloadSelect(selectPath string) tea.Cmd {
	if m.cwd == "" {
		return nil
	}
	entries, err := scanDir(m.cwd, m.hidden)
	if err != nil {
		m.notice = "Could not refresh this folder"
		return nil
	}
	if selectPath == "" {
		selectPath = m.selectedPath()
	}
	m.search = false
	m.searchResults = false
	m.searching = false
	m.searchTruncated = false
	m.filter = ""
	m.searchToken++
	m.all = entries
	m.entries = entries
	m.selected = 0
	for index, entry := range entries {
		if entry.Path == selectPath {
			m.selected = index
			break
		}
	}
	m.scroll = 0
	m.gen++
	m.pending = map[string]bool{}
	m.tried = map[string]bool{}
	m.fsType, m.free = filesystemInfo(m.cwd)
	m.ensureVisible()
	return m.thumbCmds()
}

func (m *model) drawMenu(canvas *Canvas) {
	if m.menu == nil {
		return
	}
	width := m.menuWidth()
	height := len(m.menu.items) + 2
	x := m.menu.x
	if x+width > m.width-1 {
		x = m.width - width - 1
	}
	if x < 1 {
		x = 1
	}
	y := m.menu.y
	if y+height > m.height-1 {
		y = m.height - height - 1
	}
	if y < 1 {
		y = 1
	}
	m.menu.x, m.menu.y = x, y
	m.menu.hits = m.menu.hits[:0]
	canvas.fillRect(x, y, width, height, colPanel)
	canvas.box(x, y, width, height, colHover, colPanel)
	for index, item := range m.menu.items {
		row := y + 1 + index
		if item.separator {
			canvas.text(x+2, row, width-4, strings.Repeat("─", width-4), colDimmer, colPanel)
			continue
		}
		background := colPanel
		foreground := colText
		if !item.enabled {
			foreground = colDimmer
		} else if index == m.menu.cursor {
			background = colSidebarActive
			foreground = colText
		}
		canvas.fillRect(x+1, row, width-2, 1, background)
		canvas.text(x+2, row, width-12, item.label, foreground, background)
		if item.shortcut != "" {
			canvas.text(x+width-2-runewidth.StringWidth(item.shortcut), row, runewidth.StringWidth(item.shortcut), item.shortcut, colFaint, background)
		}
		m.menu.hits = append(m.menu.hits, menuHit{y: row, item: item})
	}
}

func (m *model) drawDialog(canvas *Canvas) {
	if m.dialog == nil {
		return
	}
	width := 56
	if width > m.width-4 {
		width = m.width - 4
	}
	height := 7
	x := (m.width - width) / 2
	y := (m.height - height) / 2
	canvas.dimOutside(x, y, width, height)
	canvas.fillRect(x, y, width, height, colPanel)
	canvas.box(x, y, width, height, colAccent, colPanel)
	canvas.text(x+2, y+1, width-4, m.dialog.title, colText, colPanel)
	if m.dialog.kind == "confirm" {
		canvas.centerText(x+2, y+3, width-4, m.dialog.message, colMuted, colPanel)
		canvas.centerText(x+2, y+5, width-4, "enter / y   ·   esc / n", colFaint, colPanel)
		m.cursorPosX = -1
		return
	}
	canvas.fillRect(x+2, y+3, width-4, 1, colSearchBG)
	value := m.dialog.value
	shown := value
	if m.dialog.replace {
		shown = value
	}
	if shown == "" {
		canvas.text(x+3, y+3, width-6, "new name", colFaint, colSearchBG)
	} else {
		canvas.text(x+3, y+3, width-6, shown, colText, colSearchBG)
	}
	cursor := x + 3 + runewidth.StringWidth(shown)
	if cursor > x+width-3 {
		cursor = x + width - 3
	}
	m.cursorPosX = cursor
	m.cursorPosY = y + 3
	canvas.centerText(x+2, y+5, width-4, "enter apply   ·   esc cancel   ·   ctrl+u clear", colFaint, colPanel)
}

func (m *model) drawPreview(canvas *Canvas) {
	if m.preview == nil {
		return
	}
	canvas.fillRect(0, 0, m.width, m.height, hexRGB("#0d0c14"))
	canvas.text(2, 0, m.width-4, truncateText(m.preview.entry.Name, m.width-4), colMuted, hexRGB("#0d0c14"))
	footer := "← → browse   ·   esc close"
	canvas.centerText(0, m.height-1, m.width, footer, colFaint, hexRGB("#0d0c14"))
	if m.preview.loading {
		canvas.centerText(0, m.height/2, m.width, "Loading preview…", colMuted, hexRGB("#0d0c14"))
		return
	}
	if m.preview.err != "" {
		canvas.centerText(0, m.height/2, m.width, m.preview.err, colRed, hexRGB("#0d0c14"))
		return
	}
	x := (m.width - m.preview.cols) / 2
	y := 1 + (m.height-2-m.preview.rows)/2
	if x < 0 {
		x = 0
	}
	if y < 1 {
		y = 1
	}
	if m.protocol == protoTGP {
		if !m.preview.uploaded {
			canvas.centerText(0, m.height/2, m.width, "Loading preview…", colMuted, hexRGB("#0d0c14"))
			return
		}
		for row := 0; row < m.preview.rows; row++ {
			for column := 0; column < m.preview.cols; column++ {
				canvas.putImage(x+column, y+row, placeholder(m.preview.id, row, column), kittyColor(m.preview.id), hexRGB("#0d0c14"))
			}
		}
		return
	}
	for row := 0; row < m.preview.rows; row++ {
		for column := 0; column < m.preview.cols; column++ {
			index := row*m.preview.cols + column
			if index >= len(m.preview.half) {
				continue
			}
			cell := m.preview.half[index]
			canvas.putImage(x+column, y+row, "▀", cell.Top, cell.Bottom)
		}
	}
}

func (m *model) openSelectedPreview() tea.Cmd {
	entry := m.selectedEntry()
	if entry == nil || !previewable(*entry) {
		return nil
	}
	return m.openPreview(*entry)
}
