package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-runewidth"
)

func (m *model) setClipboard(cut bool) tea.Cmd {
	paths := m.selectedPaths()
	if len(paths) == 0 {
		m.notice = "Nothing selected"
		return nil
	}
	m.clipboard = fileClipboard{paths: paths, cut: cut}
	if len(paths) == 1 {
		entry := m.selectedEntry()
		name := filepath.Base(paths[0])
		if entry != nil {
			name = entry.Name
		}
		if cut {
			m.notice = "Cut " + name
		} else {
			m.notice = "Copied " + name
		}
		return nil
	}
	if cut {
		m.notice = fmt.Sprintf("Cut %d items", len(paths))
	} else {
		m.notice = fmt.Sprintf("Copied %d items", len(paths))
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
	paths := m.selectedPaths()
	if len(paths) == 0 {
		return
	}
	message := ""
	if len(paths) == 1 && entry != nil {
		message = entry.Name
	} else {
		message = fmt.Sprintf("%d items", len(paths))
	}
	action := "trash"
	title := "Move to Trash"
	if inTrash(m.cwd) {
		action = "delete"
		title = "Delete Permanently"
	}
	m.menu = nil
	m.dialog = &dialogState{
		kind:    "confirm",
		action:  action,
		title:   title,
		message: message,
		target:  paths[0],
		paths:   paths,
	}
}

func (m *model) trashPaths(paths []string) tea.Cmd {
	if m.busy || len(paths) == 0 {
		return nil
	}
	m.busy = true
	if len(paths) == 1 {
		m.notice = "Moving to Trash…"
	} else {
		m.notice = fmt.Sprintf("Moving %d items to Trash…", len(paths))
	}
	return func() tea.Msg {
		for _, path := range paths {
			if err := trashEntry(path); err != nil {
				return opMsg{notice: "Moved to Trash", err: err}
			}
		}
		notice := "Moved to Trash"
		if len(paths) > 1 {
			notice = fmt.Sprintf("Moved %d items to Trash", len(paths))
		}
		return opMsg{notice: notice}
	}
}

func (m *model) deletePaths(paths []string) tea.Cmd {
	if m.busy || len(paths) == 0 {
		return nil
	}
	m.busy = true
	m.notice = "Deleting…"
	return func() tea.Msg {
		for _, path := range paths {
			if err := os.RemoveAll(path); err != nil {
				return opMsg{notice: "Deleted", err: err}
			}
		}
		notice := "Deleted"
		if len(paths) > 1 {
			notice = fmt.Sprintf("Deleted %d items", len(paths))
		}
		return opMsg{notice: notice}
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
	if textPreviewable(entry) {
		lines, err := readTextPreview(entry.Path)
		if err != nil {
			m.notice = "Preview failed"
			return nil
		}
		m.preview = &previewState{entry: entry, text: lines, textMode: true}
		return nil
	}
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
	if m.preview != nil && m.preview.textMode {
		switch msg.String() {
		case "esc", "q", " ", "space":
			return m.closePreview()
		case "up", "k":
			m.preview.textOffset--
		case "down", "j":
			m.preview.textOffset++
		case "pgup":
			m.preview.textOffset -= m.height - 2
		case "pgdown":
			m.preview.textOffset += m.height - 2
		case "home":
			m.preview.textOffset = 0
		}
		return nil
	}
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
			y := hit.y + 2
			if hit.height < 3 {
				y = hit.y
			}
			m.openMenuAt(hit.x+2, y)
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
		entry := m.entries[index]
		if len(m.selection) == 0 || !m.selection[entry.Path] {
			m.clearSelection()
			m.setSelected(index)
			m.anchor = index
		}
		m.ensureVisible()
		target = &entry
	}
	items := m.menuItemsFor(target, false)
	cursor := 0
	for cursor < len(items) && (items[cursor].separator || !items[cursor].enabled) {
		cursor++
	}
	if cursor >= len(items) {
		cursor = 0
	}
	m.menu = &contextMenu{x: x, y: y, target: target, items: items, cursor: cursor}
}

func (m *model) menuItemsFor(target *Entry, advanced bool) []menuItem {
	var items []menuItem
	count := m.selectionCount()
	bulk := len(m.selection) > 1
	if !advanced {
		if target != nil {
			items = append(items, menuItem{label: "Open", shortcut: "enter", action: "open", enabled: !bulk})
			if previewable(*target) {
				items = append(items, menuItem{label: "Preview", shortcut: "space", action: "preview", enabled: !bulk})
			}
			if target.IsDir && !bulk {
				switch {
				case m.placeConfigIndex(target.Path) >= 0:
					items = append(items, menuItem{label: "Remove from sidebar", action: "unbookmark", enabled: true})
				case m.sidebarHasPath(target.Path):
					items = append(items, menuItem{label: "Already in sidebar", enabled: false})
				default:
					items = append(items, menuItem{label: "Add to sidebar", action: "bookmark", enabled: true})
				}
			}
			copyLabel := "Copy"
			cutLabel := "Cut"
			trashLabel := "Move to Trash"
			if inTrash(m.cwd) {
				trashLabel = "Delete permanently"
			}
			if bulk {
				copyLabel = fmt.Sprintf("Copy %d items", count)
				cutLabel = fmt.Sprintf("Cut %d items", count)
				trashLabel = fmt.Sprintf("Move %d to Trash", count)
				if inTrash(m.cwd) {
					trashLabel = fmt.Sprintf("Delete %d permanently", count)
				}
			}
			items = append(items,
				menuItem{label: copyLabel, shortcut: "c", action: "copy", enabled: true},
				menuItem{label: cutLabel, shortcut: "x", action: "cut", enabled: true},
				menuItem{label: "Rename", shortcut: "F2", action: "rename", enabled: !bulk},
				menuItem{label: trashLabel, shortcut: "Del", action: "trash", enabled: true},
				menuItem{separator: true},
			)
		}
		items = append(items,
			menuItem{label: "New folder", shortcut: "n", action: "mkdir", enabled: true},
			menuItem{label: "Paste", shortcut: "v", action: "paste", enabled: len(m.clipboard.paths) > 0 && !m.busy},
			menuItem{label: "Refresh", shortcut: "r", action: "refresh", enabled: true},
			menuItem{separator: true},
			menuItem{label: "More…", action: "more", enabled: true},
		)
		return items
	}

	if target != nil {
		if inTrash(m.cwd) {
			label := "Restore"
			if bulk {
				label = fmt.Sprintf("Restore %d items", count)
			}
			items = append(items, menuItem{label: label, action: "restore", enabled: true})
		}
		compressLabel := "Compress…"
		if bulk {
			compressLabel = fmt.Sprintf("Compress %d items…", count)
		}
		items = append(items, menuItem{label: compressLabel, action: "compress", enabled: true})
		if bulk {
			items = append(items, menuItem{label: "Batch rename…", action: "batchrename", enabled: true})
		}
		if !bulk && isArchive(target.Name) {
			items = append(items, menuItem{label: "Extract to folder", action: "extract", enabled: true})
		}
		items = append(items, menuItem{separator: true})
		if !bulk {
			items = append(items,
				menuItem{label: "Duplicate", action: "duplicate", enabled: true},
				menuItem{label: "Copy path", action: "copypath", enabled: true},
				menuItem{label: "Open with…", action: "openwith", enabled: true},
				menuItem{label: "New symlink", action: "symlink", enabled: true},
				menuItem{label: "Permissions…", action: "chmod", enabled: true},
				menuItem{label: "Owner…", action: "chown", enabled: true},
				menuItem{label: "Properties", action: "properties", enabled: true},
			)
		}
	}
	items = append(items,
		menuItem{label: "New file", action: "mkfile", enabled: true},
		menuItem{label: "Open terminal here", action: "terminal", enabled: true},
		menuItem{separator: true},
		menuItem{label: "Sort by name", action: "sort_name", enabled: true},
		menuItem{label: "Sort by size", action: "sort_size", enabled: true},
		menuItem{label: "Sort by modified", action: "sort_modified", enabled: true},
		menuItem{label: "Reverse sort", action: "sort_reverse", enabled: true},
	)
	if inTrash(m.cwd) {
		items = append(items, menuItem{label: "Empty Trash", action: "emptytrash", enabled: true})
	}
	items = append(items,
		menuItem{separator: true},
		menuItem{label: "← Back", action: "back", enabled: true},
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
	height := len(m.menu.items)
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
	if action == "more" || action == "back" {
		m.menu.items = m.menuItemsFor(target, action == "more")
		m.menu.cursor = 0
		for m.menu.cursor < len(m.menu.items) && (m.menu.items[m.menu.cursor].separator || !m.menu.items[m.menu.cursor].enabled) {
			m.menu.cursor++
		}
		if m.menu.cursor >= len(m.menu.items) {
			m.menu.cursor = 0
		}
		m.menu.hits = nil
		return nil
	}
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
	case "bookmark":
		return m.addBookmark(target)
	case "unbookmark":
		return m.removeBookmark(target)
	case "compress":
		m.startCompress()
	case "batchrename":
		m.startBatchRename()
	case "extract":
		if target != nil {
			return m.extractCmd(target.Path)
		}
	case "restore":
		return m.restoreSelected()
	case "emptytrash":
		m.confirmEmptyTrash()
	case "duplicate":
		return m.duplicateSelected()
	case "copypath":
		return m.copyPath()
	case "openwith":
		m.startOpenWith(target)
	case "symlink":
		m.startNewSymlink(target)
	case "chmod":
		m.startChmod()
	case "chown":
		m.startChown()
	case "properties":
		m.startProperties(target)
	case "terminal":
		return m.openTerminal()
	case "mkfile":
		m.startNewFile()
	case "sort_name":
		sortMode = "name"
		appConfig.Behavior.Sort = "name"
		return m.reload()
	case "sort_size":
		sortMode = "size"
		appConfig.Behavior.Sort = "size"
		return m.reload()
	case "sort_modified":
		sortMode = "modified"
		appConfig.Behavior.Sort = "modified"
		return m.reload()
	case "sort_reverse":
		sortReverse = !sortReverse
		appConfig.Behavior.SortReverse = sortReverse
		return m.reload()
	}
	return nil
}

func bookmarkPath(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return filepath.Clean(path)
}

func (m *model) placeConfigIndex(path string) int {
	target := bookmarkPath(path)
	for index, place := range appConfig.Places {
		if bookmarkPath(expandPath(place.Path)) == target {
			return index
		}
	}
	return -1
}

func (m *model) sidebarHasPath(path string) bool {
	target := bookmarkPath(path)
	for _, place := range m.places {
		if bookmarkPath(place.Path) == target {
			return true
		}
	}
	return false
}

func (m *model) addBookmark(entry *Entry) tea.Cmd {
	if entry == nil || !entry.IsDir {
		return nil
	}
	if m.sidebarHasPath(entry.Path) {
		m.notice = "Already in sidebar"
		return nil
	}
	appConfig.Places = append(appConfig.Places, PlaceConfig{
		Label: entry.Name,
		Path:  entry.Path,
		Icon:  "bookmark",
	})
	m.places = buildPlaces()
	if err := saveConfig(); err != nil {
		m.notice = "Could not save bookmark: " + err.Error()
		return nil
	}
	m.notice = "Added " + entry.Name + " to sidebar"
	return nil
}

func (m *model) removeBookmark(entry *Entry) tea.Cmd {
	if entry == nil {
		return nil
	}
	index := m.placeConfigIndex(entry.Path)
	if index < 0 {
		return nil
	}
	label := appConfig.Places[index].Label
	if strings.TrimSpace(label) == "" {
		label = filepath.Base(entry.Path)
	}
	appConfig.Places = append(appConfig.Places[:index], appConfig.Places[index+1:]...)
	m.places = buildPlaces()
	if total := len(m.places) + len(m.devices); m.sideCursor >= total && m.sideCursor > 0 {
		m.sideCursor = total - 1
	}
	if err := saveConfig(); err != nil {
		m.notice = "Could not save bookmark: " + err.Error()
		return nil
	}
	m.notice = "Removed " + label + " from sidebar"
	return nil
}

func (m *model) handleDialogKey(msg tea.KeyPressMsg) tea.Cmd {
	dialog := m.dialog
	if dialog == nil {
		return nil
	}
	key := msg.String()
	if dialog.kind == "info" {
		switch key {
		case "esc", "enter", "q":
			m.dialog = nil
		}
		return nil
	}
	if dialog.kind == "confirm" {
		switch key {
		case "esc", "n", "q":
			m.dialog = nil
		case "enter", "y":
			switch dialog.action {
			case "emptytrash":
				m.dialog = nil
				return m.emptyTrashCmd()
			case "delete":
				paths := dialog.paths
				if len(paths) == 0 && dialog.target != "" {
					paths = []string{dialog.target}
				}
				m.dialog = nil
				return m.deletePaths(paths)
			default:
				paths := dialog.paths
				if len(paths) == 0 && dialog.target != "" {
					paths = []string{dialog.target}
				}
				m.dialog = nil
				return m.trashPaths(paths)
			}
		}
		return nil
	}
	if key == "ctrl+s" || key == "ctrl+shift+s" {
		if m.settings != nil && dialog.action == "setting_color" {
			cmd := m.applyDialog()
			if m.dialog == nil {
				return m.saveSettings()
			}
			return cmd
		}
	}
	dialog.message = ""
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
		switch dialog.action {
		case "setting_color":
			dialog.message = "Enter a hex color"
			return nil
		case "open_with":
			dialog.message = "Enter a command"
			return nil
		}
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
	case "mkfile":
		target, err := makeFile(dialog.target, value)
		if err != nil {
			m.notice = fmt.Sprintf("Could not create file: %v", err)
			return nil
		}
		m.dialog = nil
		m.notice = "Created " + filepath.Base(target)
		return m.reloadSelect(target)
	case "symlink":
		target, err := makeSymlink(m.cwd, value, dialog.target)
		if err != nil {
			m.notice = fmt.Sprintf("Could not create symlink: %v", err)
			return nil
		}
		m.dialog = nil
		m.notice = "Created " + filepath.Base(target)
		return m.reloadSelect(target)
	case "batchrename":
		paths := dialog.paths
		first, err := batchRename(paths, value)
		if err != nil {
			m.notice = err.Error()
			return nil
		}
		m.dialog = nil
		m.notice = fmt.Sprintf("Renamed %d items", len(paths))
		return m.reloadSelect(first)
	case "chmod":
		paths := dialog.paths
		if err := applyChmod(paths, value); err != nil {
			m.notice = err.Error()
			return nil
		}
		m.dialog = nil
		if len(paths) == 1 {
			m.notice = "Updated permissions"
		} else {
			m.notice = fmt.Sprintf("Updated permissions on %d items", len(paths))
		}
		return m.reloadSelect(m.selectedPath())
	case "chown":
		paths := dialog.paths
		if err := applyChown(paths, value); err != nil {
			m.notice = err.Error()
			return nil
		}
		m.dialog = nil
		if len(paths) == 1 {
			m.notice = "Updated owner"
		} else {
			m.notice = fmt.Sprintf("Updated owner on %d items", len(paths))
		}
		return m.reloadSelect(m.selectedPath())
	case "compress":
		paths := dialog.paths
		if len(paths) == 0 {
			paths = m.selectedPaths()
		}
		m.dialog = nil
		return m.compressCmd(paths, filepath.Join(m.cwd, value))
	case "open_with":
		target := dialog.target
		m.dialog = nil
		if err := launchWith(value, target); err != nil {
			m.notice = err.Error()
			return nil
		}
		m.notice = "Opened with " + strings.Fields(value)[0]
		return nil
	case "setting_color":
		if !m.applySettingColor(dialog.target, value) {
			dialog.message = "Use #rgb or #rrggbb"
			return nil
		}
		m.dialog = nil
		return nil
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
	m.clearSelection()
	m.filterLocal = false
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
	height := len(m.menu.items)
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
	canvas.fillRect(x, y, width, height, colSidebar)
	for index, item := range m.menu.items {
		row := y + index
		if item.separator {
			canvas.text(x+2, row, width-4, strings.Repeat("─", width-4), colDimmer, colSidebar)
			continue
		}
		background := colSidebar
		foreground := colText
		if !item.enabled {
			foreground = colDimmer
		} else if index == m.menu.cursor {
			background = colSidebarActive
			foreground = colText
		}
		canvas.fillRect(x, row, width, 1, background)
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
	if m.dialog.kind == "info" {
		width := 64
		if width > m.width-2 {
			width = m.width - 2
		}
		if width < 24 {
			width = 24
		}
		height := len(m.dialog.lines) + 4
		if height > m.height-2 {
			height = m.height - 2
		}
		if height < 6 {
			height = 6
		}
		x := (m.width - width) / 2
		y := (m.height - height) / 2
		canvas.dimOutside(x, y, width, height)
		canvas.fillRect(x, y, width, height, colPanel)
		canvas.box(x, y, width, height, colAccent, colPanel)
		canvas.text(x+2, y+1, width-4, m.dialog.title, colText, colPanel)
		available := height - 4
		for index, line := range m.dialog.lines {
			if index >= available {
				break
			}
			canvas.text(x+2, y+2+index, width-4, line, colText, colPanel)
		}
		canvas.centerText(x+2, y+height-2, width-4, "esc close", colFaint, colPanel)
		m.cursorPosX = -1
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
	placeholder := "new name"
	switch m.dialog.action {
	case "setting_color":
		placeholder = "#rrggbb"
	case "open_with":
		placeholder = "command"
	case "mkfile":
		placeholder = "file name"
	case "compress":
		placeholder = "archive.zip"
	case "batchrename":
		placeholder = "{name}{ext}"
	case "chmod":
		placeholder = "644"
	case "chown":
		placeholder = "user:group"
	case "symlink":
		placeholder = "link name"
	}
	if shown == "" {
		canvas.text(x+3, y+3, width-6, placeholder, colFaint, colSearchBG)
	} else {
		canvas.text(x+3, y+3, width-6, shown, colText, colSearchBG)
	}
	cursor := x + 3 + runewidth.StringWidth(shown)
	if cursor > x+width-3 {
		cursor = x + width - 3
	}
	m.cursorPosX = cursor
	m.cursorPosY = y + 3
	hint := "enter apply   ·   esc cancel   ·   ctrl+u clear"
	hintColor := colFaint
	if m.dialog.message != "" {
		hint = m.dialog.message
		hintColor = colRed
	}
	canvas.centerText(x+2, y+5, width-4, hint, hintColor, colPanel)
}

func (m *model) drawPreview(canvas *Canvas) {
	if m.preview == nil {
		return
	}
	canvas.fillRect(0, 0, m.width, m.height, hexRGB("#0d0c14"))
	canvas.text(2, 0, m.width-4, truncateText(m.preview.entry.Name, m.width-4), colMuted, hexRGB("#0d0c14"))
	footer := "← → browse   ·   esc close"
	if m.preview.textMode {
		footer = "↑↓ scroll   ·   esc close"
	}
	canvas.centerText(0, m.height-1, m.width, footer, colFaint, hexRGB("#0d0c14"))
	if m.preview.textMode {
		background := hexRGB("#0d0c14")
		rows := m.height - 2
		maxOffset := len(m.preview.text) - rows
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.preview.textOffset > maxOffset {
			m.preview.textOffset = maxOffset
		}
		if m.preview.textOffset < 0 {
			m.preview.textOffset = 0
		}
		for index := 0; index < rows && m.preview.textOffset+index < len(m.preview.text); index++ {
			line := m.preview.text[m.preview.textOffset+index]
			canvas.text(2, 1+index, m.width-4, truncateText(line, m.width-4), colText, background)
		}
		return
	}
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
