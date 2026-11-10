package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-runewidth"
)

type focusPane int

const (
	focusGrid focusPane = iota
	focusSidebar
)

type menuItem struct {
	label     string
	shortcut  string
	action    string
	enabled   bool
	separator bool
}

type menuHit struct {
	y    int
	item menuItem
}

type contextMenu struct {
	x, y   int
	target *Entry
	items  []menuItem
	cursor int
	hits   []menuHit
}

type dialogState struct {
	kind    string
	action  string
	title   string
	message string
	value   string
	target  string
	paths   []string
	lines   []string
	replace bool
}

type previewState struct {
	entry      Entry
	png        []byte
	half       []HalfCell
	cols       int
	rows       int
	id         uint32
	uploaded   bool
	loading    bool
	err        string
	token      int
	text       []string
	textOffset int
	textMode   bool
}

type thumbMsg struct {
	gen   int
	path  string
	thumb *Thumb
	err   error
}

type repaintMsg struct{}

type startupMsg struct{}

type sideHit struct {
	y    int
	path string
}

type cardHit struct {
	x, y, width, height int
	index               int
}

type crumbHit struct {
	x1, x2 int
	path   string
}

type topHit struct {
	x1, x2 int
	action string
}

type model struct {
	width, height int
	cwd           string
	initial       string

	all             []Entry
	entries         []Entry
	filter          string
	filterLocal     bool
	search          bool
	searchResults   bool
	searching       bool
	searchTruncated bool
	searchToken     int
	hidden          bool

	selected int
	anchor   int
	scroll   int
	focus    focusPane

	selection map[string]bool

	sideCursor int
	help       bool
	helpOffset int

	history      []string
	historyIndex int

	gen     int
	thumbs  map[string]*Thumb
	pending map[string]bool
	tried   map[string]bool
	svc     *ThumbService

	protocol Protocol
	kitty    *Kitty

	places  []Place
	devices []Place

	sideHits  []sideHit
	cardHits  []cardHit
	crumbHits []crumbHit
	topHits   []topHit

	lastClick      time.Time
	lastClickIndex int
	notice         string
	started        bool
	busy           bool

	clipboard fileClipboard
	menu      *contextMenu
	dialog    *dialogState
	preview   *previewState
	settings  *settingsState

	previewSeq int
	cursorPosX int
	cursorPosY int

	fsType string
	free   uint64
}

func newModel(initial string) *model {
	return &model{
		initial:        initial,
		hidden:         appConfig.Behavior.ShowHidden,
		thumbs:         map[string]*Thumb{},
		pending:        map[string]bool{},
		tried:          map[string]bool{},
		selection:      map[string]bool{},
		lastClickIndex: -1,
	}
}

func (m *model) Init() tea.Cmd {
	m.protocol = detectProtocol()
	m.kitty = newKitty(os.Getenv("TMUX") != "")
	m.svc = newThumbService()
	m.places = buildPlaces()
	m.devices = buildDevices()

	path := m.initial
	selectPath := ""
	if path == "" {
		path = "."
	}
	absolute, err := filepath.Abs(path)
	if err == nil {
		path = absolute
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		selectPath = path
		path = filepath.Dir(path)
	}
	m.loadDir(path, false, selectPath)
	if m.cwd == "" {
		fallback := userHome()
		if info, err := os.Stat(fallback); err != nil || !info.IsDir() {
			fallback, _ = os.Getwd()
		}
		m.loadDir(fallback, false, "")
	}
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg { return startupMsg{} })
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		sidebarW = responsiveSidebarWidth(m.width, preferredSidebarW)
		if sidebarW == 0 && m.focus == focusSidebar {
			m.focus = focusGrid
		}
		m.ensureVisible()
		if m.started {
			return m, m.thumbCmds()
		}
		return m, nil
	case startupMsg:
		m.started = true
		return m, m.thumbCmds()
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.MouseClickMsg:
		return m.handleClick(msg.Mouse())
	case tea.MouseWheelMsg:
		return m.handleWheel(msg.Mouse())
	case thumbMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		delete(m.pending, msg.path)
		if msg.err != nil || msg.thumb == nil {
			m.tried[msg.path] = true
			return m, nil
		}
		m.thumbs[msg.path] = msg.thumb
		m.tried[msg.path] = true
		if m.protocol == protoTGP {
			raw := m.kitty.Upload(msg.thumb)
			if raw != "" {
				return m, tea.Sequence(tea.Raw(raw), func() tea.Msg { return repaintMsg{} })
			}
		}
	case previewMsg:
		return m.handlePreviewMsg(msg)
	case opMsg:
		return m.handleOpMsg(msg)
	case searchMsg:
		return m.handleSearchMsg(msg)
	case repaintMsg:
		return m, nil
	}
	return m, nil
}

func (m *model) View() tea.View {
	if m.width <= 0 || m.height <= 0 {
		view := tea.NewView("")
		view.AltScreen = true
		return view
	}
	canvas := m.renderCanvas()
	view := tea.NewView(canvas.String())
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	view.WindowTitle = "fleek — " + filepath.Base(m.cwd)
	if m.preview == nil && !m.help {
		if m.dialog != nil && m.dialog.kind == "prompt" && m.cursorPosX > 0 {
			view.Cursor = tea.NewCursor(m.cursorPosX, m.cursorPosY)
		} else if m.search || m.filterLocal {
			view.Cursor = tea.NewCursor(m.searchCursorX(), topBarY)
		}
	}
	return view
}

func (m *model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.preview != nil {
		return m, m.handlePreviewKey(msg)
	}
	if m.help {
		switch key {
		case "esc", "q", "?":
			m.help = false
		case "up", "k":
			if m.helpOffset > 0 {
				m.helpOffset--
			}
		case "down", "j":
			m.helpOffset++
		case "pgup":
			m.helpOffset -= 10
			if m.helpOffset < 0 {
				m.helpOffset = 0
			}
		case "pgdown":
			m.helpOffset += 10
		}
		return m, nil
	}
	if m.dialog != nil {
		return m, m.handleDialogKey(msg)
	}
	if m.menu != nil {
		return m, m.handleMenuKey(msg)
	}
	if m.settings != nil {
		return m, m.handleSettingsKey(msg)
	}
	if key == "ctrl+," {
		m.openSettings()
		return m, nil
	}
	if m.search {
		return m.handleSearchKey(msg)
	}
	if m.filterLocal {
		return m.handleFilterKey(msg)
	}

	switch key {
	case "q":
		return m, tea.Quit
	case "tab", "shift+tab":
		m.toggleFocus()
		return m, nil
	case "esc":
		if len(m.selection) > 0 {
			m.clearSelection()
			return m, nil
		}
		if m.focus == focusSidebar {
			m.focus = focusGrid
		}
		return m, nil
	case "ctrl+a":
		m.selectAll()
		return m, m.thumbCmds()
	case "t":
		m.toggleSelected(m.selected)
		return m, m.thumbCmds()
	case "shift+up":
		m.extendSelection(-m.gridCols())
		return m, m.thumbCmds()
	case "shift+down":
		m.extendSelection(m.gridCols())
		return m, m.thumbCmds()
	case "shift+left":
		m.extendSelection(-1)
		return m, m.thumbCmds()
	case "shift+right":
		m.extendSelection(1)
		return m, m.thumbCmds()
	case "shift+home":
		m.extendSelection(-m.selected)
		return m, m.thumbCmds()
	case "shift+end":
		if len(m.entries) > 0 {
			m.extendSelection(len(m.entries) - 1 - m.selected)
		}
		return m, m.thumbCmds()
	case "alt+left":
		return m, m.historyBack()
	case "alt+right":
		return m, m.historyForward()
	case "backspace":
		return m, m.actionParent()
	case "~":
		return m, m.loadDir(userHome(), true, "")
	case ".":
		m.hidden = !m.hidden
		return m, m.reload()
	case "r":
		return m, m.reload()
	case "o":
		sortReverse = !sortReverse
		appConfig.Behavior.SortReverse = sortReverse
		if sortReverse {
			m.notice = "Reverse sort on"
		} else {
			m.notice = "Reverse sort off"
		}
		return m, m.reload()
	case "/", "ctrl+f":
		return m, m.beginSearch()
	case "f":
		return m, m.beginLocalFilter()
	case "?":
		m.help = true
		m.helpOffset = 0
		return m, nil
	case "s":
		m.openSettings()
		return m, nil
	}

	if m.focus == focusSidebar {
		return m.handleSidebarKey(key)
	}
	switch key {
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
	case "home":
		m.clearSelection()
		m.setSelected(0)
	case "end":
		if len(m.entries) > 0 {
			m.clearSelection()
			m.setSelected(len(m.entries) - 1)
		}
	case "g":
		m.clearSelection()
		m.setSelected(0)
	case "G":
		if len(m.entries) > 0 {
			m.clearSelection()
			m.setSelected(len(m.entries) - 1)
		}
	case "enter":
		return m, m.openSelected()
	case " ", "space":
		return m, m.openSelectedPreview()
	case "c":
		return m, m.setClipboard(false)
	case "x":
		return m, m.setClipboard(true)
	case "v":
		return m, m.pasteClipboard()
	case "f2":
		if len(m.selection) > 1 {
			m.notice = "Rename is single-item"
			return m, nil
		}
		m.startRename(m.selectedEntry())
		return m, nil
	case "n":
		m.startNewFolder()
		return m, nil
	case "m":
		m.openMenuForSelected()
		return m, nil
	case "delete":
		m.confirmTrash(m.selectedEntry())
		return m, nil
	default:
		return m, nil
	}
	m.ensureVisible()
	return m, m.thumbCmds()
}

func (m *model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		return m, m.closeSearch()
	case "enter":
		entry := m.selectedEntry()
		if entry == nil {
			return m, nil
		}
		if entry.IsDir {
			return m, m.loadDir(entry.Path, true, "")
		}
		if err := launch(entry.Path); err != nil {
			m.notice = err.Error()
		}
		return m, nil
	case "backspace":
		if m.filter != "" {
			_, size := utf8.DecodeLastRuneInString(m.filter)
			m.filter = m.filter[:len(m.filter)-size]
		}
		return m, m.queueSearch()
	case "ctrl+u":
		m.filter = ""
		return m, m.queueSearch()
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
	case "home":
		m.setSelected(0)
	case "end":
		if len(m.entries) > 0 {
			m.setSelected(len(m.entries) - 1)
		}
	default:
		if text := msg.Key().Text; text != "" && len(m.filter) < 120 {
			m.filter += text
			return m, m.queueSearch()
		}
	}
	m.ensureVisible()
	return m, m.thumbCmds()
}

func (m *model) handleSidebarKey(key string) (tea.Model, tea.Cmd) {
	rows := len(m.places) + len(m.devices)
	switch key {
	case "up", "k":
		if m.sideCursor > 0 {
			m.sideCursor--
		}
	case "down", "j":
		if m.sideCursor < rows-1 {
			m.sideCursor++
		}
	case "enter":
		place, ok := m.sidePlace(m.sideCursor)
		if ok {
			return m, m.loadDir(place.Path, true, "")
		}
	case "esc", "left", "h":
		m.focus = focusGrid
	}
	return m, nil
}

func (m *model) handleClick(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	if mouse.Button == tea.MouseRight {
		if m.preview != nil {
			return m, m.closePreview()
		}
		if m.dialog != nil {
			m.dialog = nil
			return m, nil
		}
		if m.menu != nil {
			m.menu = nil
			return m, nil
		}
		if m.help {
			m.help = false
			return m, nil
		}
		if m.settings != nil {
			return m, m.cancelSettings()
		}
		if mouse.Y >= topH && mouse.Y < m.height-statusH && mouse.X >= sidebarW {
			m.openMenuAt(mouse.X, mouse.Y)
		}
		return m, nil
	}
	if mouse.Button != tea.MouseLeft {
		return m, nil
	}
	if m.preview != nil {
		return m, m.closePreview()
	}
	if m.dialog != nil {
		return m, nil
	}
	if m.menu != nil {
		return m.handleMenuClick(mouse)
	}
	if m.help {
		m.help = false
		return m, nil
	}
	if m.settings != nil {
		return m, nil
	}
	if mouse.Y >= topBarY && mouse.Y < topH {
		for _, hit := range m.topHits {
			if mouse.X >= hit.x1 && mouse.X <= hit.x2 {
				switch hit.action {
				case "back":
					return m, m.historyBack()
				case "forward":
					return m, m.historyForward()
				case "search":
					return m, m.beginSearch()
				case "hidden":
					m.hidden = !m.hidden
					return m, m.reload()
				case "refresh":
					return m, m.reload()
				case "help":
					m.help = true
					m.helpOffset = 0
					return m, nil
				case "settings":
					m.openSettings()
					return m, nil
				}
			}
		}
		for _, hit := range m.crumbHits {
			if mouse.X >= hit.x1 && mouse.X <= hit.x2 {
				return m, m.loadDir(hit.path, true, "")
			}
		}
		return m, nil
	}
	if mouse.X < sidebarW {
		for _, hit := range m.sideHits {
			if mouse.Y == hit.y {
				return m, m.loadDir(hit.path, true, "")
			}
		}
		return m, nil
	}
	for _, hit := range m.cardHits {
		if mouse.X >= hit.x && mouse.X < hit.x+hit.width && mouse.Y >= hit.y && mouse.Y < hit.y+hit.height {
			now := time.Now()
			double := now.Sub(m.lastClick) < 450*time.Millisecond && m.lastClickIndex == hit.index
			extend := mouse.Mod&tea.ModShift != 0
			toggle := mouse.Mod&(tea.ModCtrl|tea.ModMeta) != 0
			m.lastClick = now
			m.lastClickIndex = hit.index
			m.focus = focusGrid
			switch {
			case extend:
				m.selectRangeTo(hit.index)
			case toggle:
				m.toggleSelected(hit.index)
			default:
				m.clearSelection()
				m.setSelected(hit.index)
				m.anchor = hit.index
			}
			m.ensureVisible()
			if double && !extend && !toggle {
				entry := m.entries[hit.index]
				if entry.IsDir {
					return m, m.loadDir(entry.Path, true, "")
				}
				if previewable(entry) {
					return m, m.openPreview(entry)
				}
				return m, m.openSelected()
			}
			return m, m.thumbCmds()
		}
	}
	return m, nil
}

func (m *model) handleWheel(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	if m.settings != nil {
		switch mouse.Button {
		case tea.MouseWheelUp:
			m.moveSettingsCursor(-1)
		case tea.MouseWheelDown:
			m.moveSettingsCursor(1)
		}
		return m, nil
	}
	if m.help {
		switch mouse.Button {
		case tea.MouseWheelUp:
			if m.helpOffset > 0 {
				m.helpOffset -= 2
			}
		case tea.MouseWheelDown:
			m.helpOffset += 2
		}
		return m, nil
	}
	delta := 0
	switch mouse.Button {
	case tea.MouseWheelUp:
		delta = -2
	case tea.MouseWheelDown:
		delta = 2
	}
	if delta == 0 {
		return m, nil
	}
	m.scroll += delta
	m.clampScroll()
	return m, m.thumbCmds()
}

func (m *model) toggleFocus() {
	if sidebarW == 0 {
		m.focus = focusGrid
		return
	}
	if m.focus == focusGrid {
		m.focus = focusSidebar
		if index, ok := m.activePlaceIndex(); ok {
			m.sideCursor = index
		}
	} else {
		m.focus = focusGrid
	}
}

func (m *model) loadDir(path string, addHistory bool, selectPath string) tea.Cmd {
	target, err := filepath.Abs(path)
	if err != nil {
		m.notice = err.Error()
		return nil
	}
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		m.notice = fmt.Sprintf("Can't open %s", filepath.Base(target))
		return nil
	}
	entries, err := scanDir(target, m.hidden)
	if err != nil {
		m.notice = fmt.Sprintf("Can't read %s", filepath.Base(target))
		return nil
	}
	m.cwd = target
	m.all = entries
	m.entries = entries
	m.clearSelection()
	m.filter = ""
	m.filterLocal = false
	m.search = false
	m.searchResults = false
	m.searching = false
	m.searchTruncated = false
	m.searchToken++
	m.selected = 0
	m.scroll = 0
	if selectPath != "" {
		for index, entry := range m.entries {
			if entry.Path == selectPath {
				m.selected = index
				break
			}
		}
	}
	if addHistory {
		if len(m.history) == 0 {
			m.history = []string{target}
			m.historyIndex = 0
		} else if m.history[m.historyIndex] != target {
			m.history = append(m.history[:m.historyIndex+1], target)
			m.historyIndex = len(m.history) - 1
		}
	} else if len(m.history) == 0 {
		m.history = []string{target}
		m.historyIndex = 0
	}
	m.focus = focusGrid
	m.gen++
	m.pending = map[string]bool{}
	m.tried = map[string]bool{}
	m.fsType, m.free = filesystemInfo(target)
	m.ensureVisible()
	m.notice = ""
	return m.thumbCmds()
}

func (m *model) reload() tea.Cmd {
	return m.reloadSelect("")
}

func (m *model) actionParent() tea.Cmd {
	parent := filepath.Dir(m.cwd)
	if parent == m.cwd || m.cwd == "" {
		return nil
	}
	return m.loadDir(parent, true, "")
}

func (m *model) historyBack() tea.Cmd {
	if m.historyIndex <= 0 {
		return nil
	}
	m.historyIndex--
	return m.loadDir(m.history[m.historyIndex], false, "")
}

func (m *model) historyForward() tea.Cmd {
	if m.historyIndex >= len(m.history)-1 {
		return nil
	}
	m.historyIndex++
	return m.loadDir(m.history[m.historyIndex], false, "")
}

func (m *model) openSelected() tea.Cmd {
	entry := m.selectedEntry()
	if entry == nil {
		return nil
	}
	if entry.IsDir {
		return m.loadDir(entry.Path, true, "")
	}
	if err := launch(entry.Path); err != nil {
		m.notice = err.Error()
	}
	return nil
}

func (m *model) selectedEntry() *Entry {
	if m.selected < 0 || m.selected >= len(m.entries) {
		return nil
	}
	return &m.entries[m.selected]
}

func (m *model) selectedPath() string {
	if entry := m.selectedEntry(); entry != nil {
		return entry.Path
	}
	return ""
}

func (m *model) selectionCount() int {
	if len(m.selection) > 0 {
		return len(m.selection)
	}
	if m.selectedEntry() != nil {
		return 1
	}
	return 0
}

func (m *model) selectedPaths() []string {
	if len(m.selection) == 0 {
		if entry := m.selectedEntry(); entry != nil {
			return []string{entry.Path}
		}
		return nil
	}
	paths := make([]string, 0, len(m.selection))
	for _, entry := range m.entries {
		if m.selection[entry.Path] {
			paths = append(paths, entry.Path)
		}
	}
	return paths
}

func (m *model) isSelected(index int) bool {
	if index < 0 || index >= len(m.entries) {
		return false
	}
	if len(m.selection) == 0 {
		return index == m.selected
	}
	return m.selection[m.entries[index].Path]
}

func (m *model) clearSelection() {
	m.selection = map[string]bool{}
}

func (m *model) toggleSelected(index int) {
	if index < 0 || index >= len(m.entries) {
		return
	}
	path := m.entries[index].Path
	if m.selection[path] {
		delete(m.selection, path)
	} else {
		m.selection[path] = true
	}
	m.selected = index
	m.anchor = index
}

func (m *model) selectRangeTo(index int) {
	if len(m.entries) == 0 {
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(m.entries) {
		index = len(m.entries) - 1
	}
	from, to := m.anchor, index
	if from < 0 || from >= len(m.entries) {
		from = m.selected
	}
	if from > to {
		from, to = to, from
	}
	for position := from; position <= to; position++ {
		m.selection[m.entries[position].Path] = true
	}
	m.selected = index
}

func (m *model) selectAll() {
	for _, entry := range m.entries {
		m.selection[entry.Path] = true
	}
	m.anchor = 0
}

func (m *model) extendSelection(delta int) {
	if len(m.entries) == 0 {
		return
	}
	if len(m.selection) == 0 {
		m.anchor = m.selected
		if entry := m.selectedEntry(); entry != nil {
			m.selection[entry.Path] = true
		}
	}
	m.selectRangeTo(m.selected + delta)
	m.ensureVisible()
}

func (m *model) setSelected(index int) {
	if len(m.entries) == 0 {
		m.selected = 0
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(m.entries) {
		index = len(m.entries) - 1
	}
	m.selected = index
	if len(m.selection) == 0 {
		m.anchor = index
	}
}

func (m *model) moveSelection(delta int) {
	m.setSelected(m.selected + delta)
	m.ensureVisible()
}

func (m *model) gridCols() int {
	if m.compact() {
		return 1
	}
	columns, _, _ := m.cardMetrics()
	return columns
}

func (m *model) cardMetrics() (int, int, int) {
	contentWidth := m.width - sidebarW - 2*padX
	if contentWidth < 2 {
		return 1, max(2, contentWidth), sidebarW + padX
	}
	columns := (contentWidth + gapX) / (minCardW + gapX)
	if columns < 1 {
		columns = 1
	}
	cardWidth := (contentWidth - (columns-1)*gapX) / columns
	if cardWidth < 2 {
		cardWidth = 2
	}
	used := columns*cardWidth + (columns-1)*gapX
	startX := sidebarW + padX + max(0, (contentWidth-used)/2)
	return columns, cardWidth, startX
}

func (m *model) visibleRows() int {
	body := m.height - topH - statusH
	if body <= 0 {
		return 1
	}
	if m.compact() {
		return body
	}
	rows := body / (cardH + gapY)
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m *model) totalRows() int {
	if len(m.entries) == 0 {
		return 0
	}
	return (len(m.entries) + m.gridCols() - 1) / m.gridCols()
}

func (m *model) clampScroll() {
	maxScroll := m.totalRows() - m.visibleRows()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}
}

func (m *model) ensureVisible() {
	if len(m.entries) == 0 {
		m.scroll = 0
		return
	}
	columns := m.gridCols()
	row := m.selected / columns
	rows := m.visibleRows()
	if row < m.scroll {
		m.scroll = row
	}
	if row >= m.scroll+rows {
		m.scroll = row - rows + 1
	}
	m.clampScroll()
}

func (m *model) thumbCmds() tea.Cmd {
	if !m.started || m.svc == nil || len(m.entries) == 0 {
		return nil
	}
	if m.compact() {
		return nil
	}
	start, end := m.visibleRange()
	var commands []tea.Cmd
	for index := start; index < end && index < len(m.entries); index++ {
		entry := m.entries[index]
		switch entry.Kind {
		case "folder", "image", "video", "pdf":
		default:
			continue
		}
		if thumb, ok := m.thumbs[entry.Path]; ok {
			if thumb.ModTime == entry.ModTime.UnixNano() && thumb.Size == entry.Size {
				continue
			}
			delete(m.thumbs, entry.Path)
			delete(m.tried, entry.Path)
		}
		if m.pending[entry.Path] || m.tried[entry.Path] {
			continue
		}
		m.pending[entry.Path] = true
		gen := m.gen
		service := m.svc
		commands = append(commands, func() tea.Msg {
			thumb, err := service.Get(entry)
			return thumbMsg{gen: gen, path: entry.Path, thumb: thumb, err: err}
		})
	}
	if len(commands) == 0 {
		return nil
	}
	return tea.Batch(commands...)
}

func (m *model) visibleRange() (int, int) {
	if m.width <= 0 || m.height <= 0 {
		end := len(m.entries)
		if end > 30 {
			end = 30
		}
		return 0, end
	}
	columns := m.gridCols()
	start := m.scroll * columns
	end := (m.scroll + m.visibleRows()) * columns
	if start < 0 {
		start = 0
	}
	if end > len(m.entries) {
		end = len(m.entries)
	}
	return start, end
}

func (m *model) searchCursorX() int {
	if m.width < 50 {
		x := 4 + runewidth.StringWidth(m.filter)
		if x > m.width-2 {
			x = m.width - 2
		}
		if x < 4 {
			return 4
		}
		return x
	}
	x := 17 + runewidth.StringWidth(m.filter)
	if x > m.width-16 {
		x = m.width - 16
	}
	if x < 17 {
		return 17
	}
	return x
}

func (m *model) sidePlace(index int) (Place, bool) {
	all := append(append([]Place{}, m.places...), m.devices...)
	if index < 0 || index >= len(all) {
		return Place{}, false
	}
	return all[index], true
}

func (m *model) activePlaceIndex() (int, bool) {
	best := -1
	bestLength := -1
	for index, place := range m.places {
		if m.cwd == place.Path || strings.HasPrefix(m.cwd, place.Path+string(filepath.Separator)) {
			if len(place.Path) > bestLength {
				best, bestLength = index, len(place.Path)
			}
		}
	}
	if best >= 0 {
		return best, true
	}
	for index, place := range m.devices {
		if place.Path == "/" || m.cwd == place.Path || strings.HasPrefix(m.cwd, place.Path+string(filepath.Separator)) {
			if len(place.Path) > bestLength {
				best, bestLength = len(m.places)+index, len(place.Path)
			}
		}
	}
	return best, best >= 0
}
