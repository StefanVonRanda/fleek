package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mattn/go-runewidth"
)

const (
	topH    = 2
	statusH = 1
	padX    = 2
	cardH   = 11
	imgW    = 23
	imgH    = 7
)

var (
	sidebarW          = 20
	preferredSidebarW = 20
	minCardW          = 25
	gapX              = 2
	gapY              = 2
)

func responsiveSidebarWidth(width, preferred int) int {
	if width < 36 {
		return 0
	}
	if width < 90 {
		return 3
	}
	if preferred < 14 {
		return 14
	}
	if preferred > 32 {
		return 32
	}
	return preferred
}

type crumb struct {
	label string
	path  string
}

func (m *model) renderCanvas() *Canvas {
	m.cardHits = nil
	m.sideHits = nil
	m.crumbHits = nil
	m.topHits = nil
	canvas := NewCanvas(m.width, m.height)
	if m.preview != nil {
		m.drawPreview(canvas)
		return canvas
	}
	m.drawTopbar(canvas)
	m.drawSidebar(canvas)
	m.drawGallery(canvas)
	m.drawStatus(canvas)
	if m.help {
		m.drawHelp(canvas)
		return canvas
	}
	if m.dialog != nil {
		m.drawDialog(canvas)
		return canvas
	}
	if m.menu != nil {
		m.drawMenu(canvas)
	}
	return canvas
}

func (m *model) drawTopbar(canvas *Canvas) {
	canvas.fillRect(0, 0, m.width, topH, colTop)

	back := colDimmer
	if m.historyIndex > 0 {
		back = colMuted
	}
	forward := colDimmer
	if m.historyIndex < len(m.history)-1 {
		forward = colMuted
	}
	compactSearch := m.search && m.width < 50
	if !compactSearch {
		canvas.text(2, 0, 1, iconBack, back, colTop)
		canvas.text(4, 0, 1, iconForward, forward, colTop)
		m.topHits = append(m.topHits,
			topHit{2, 2, "back"},
			topHit{4, 4, "forward"},
		)
	}

	if compactSearch {
		canvas.fillRect(2, 0, m.width-3, 1, colSearchBG)
		canvas.text(2, 0, 1, iconSearch, colMuted, colSearchBG)
		canvas.text(4, 0, m.width-6, m.filter, colText, colSearchBG)
	} else if m.search {
		canvas.fillRect(7, 0, m.width-22, 1, colSearchBG)
		canvas.text(7, 0, 1, iconSearch, colMuted, colSearchBG)
		canvas.text(9, 0, 8, "Search: ", colFaint, colSearchBG)
		canvas.text(17, 0, m.width-33, m.filter, colText, colSearchBG)
	} else {
		x := 7
		limit := m.width - 15
		crumbs := m.crumbs()
		for index, item := range crumbs {
			label := truncateText(item.label, limit-x)
			width := runewidth.StringWidth(label)
			if width == 0 {
				break
			}
			color := colMuted
			if index == len(crumbs)-1 {
				color = colText
			}
			canvas.text(x, 0, width, label, color, colTop)
			m.crumbHits = append(m.crumbHits, crumbHit{x, x + width - 1, item.path})
			x += width
			if index < len(crumbs)-1 && !(index == 0 && item.path == "/") {
				if x >= limit {
					break
				}
				canvas.text(x, 0, 1, "/", colDimmer, colTop)
				x++
			}
			if x >= limit {
				break
			}
		}
	}

	searchColor := colMuted
	if m.search {
		searchColor = colAccent
	}
	hiddenColor := colMuted
	hiddenIcon := iconEye
	if m.hidden {
		hiddenColor = colAccent
		hiddenIcon = iconEyeOff
	}
	iconPositions := []struct {
		x      int
		icon   string
		color  RGB
		action string
	}{
		{m.width - 13, iconSearch, searchColor, "search"},
		{m.width - 10, hiddenIcon, hiddenColor, "hidden"},
		{m.width - 7, iconRefresh, colMuted, "refresh"},
		{m.width - 4, iconHelp, colMuted, "help"},
	}
	if !compactSearch {
		for _, item := range iconPositions {
			if item.x < 8 {
				continue
			}
			canvas.text(item.x, 0, 1, item.icon, item.color, colTop)
			m.topHits = append(m.topHits, topHit{item.x, item.x, item.action})
		}
	}
}

func (m *model) crumbs() []crumb {
	home := userHome()
	if m.cwd == home {
		return []crumb{{"~", home}}
	}
	if strings.HasPrefix(m.cwd, home+string(filepath.Separator)) {
		relative, err := filepath.Rel(home, m.cwd)
		if err == nil {
			result := []crumb{{"~", home}}
			current := home
			for _, part := range strings.Split(relative, string(filepath.Separator)) {
				if part == "" {
					continue
				}
				current = filepath.Join(current, part)
				result = append(result, crumb{part, current})
			}
			return result
		}
	}
	result := []crumb{{"/", "/"}}
	current := "/"
	for _, part := range strings.Split(strings.Trim(m.cwd, "/"), "/") {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		result = append(result, crumb{part, current})
	}
	return result
}

func (m *model) drawSidebar(canvas *Canvas) {
	body := m.height - topH - statusH
	if body <= 0 {
		return
	}
	if sidebarW <= 0 {
		return
	}
	canvas.fillRect(0, topH, sidebarW, body, colSidebar)
	active, hasActive := m.activePlaceIndex()

	if sidebarW <= 3 {
		all := append(append([]Place{}, m.places...), m.devices...)
		y := topH + 1
		for index, place := range all {
			if y >= topH+body-1 {
				break
			}
			background := colSidebar
			if hasActive && index == active {
				background = colSidebarActive
			}
			if m.focus == focusSidebar && index == m.sideCursor {
				background = colSidebarActive
			}
			canvas.fillRect(0, y, sidebarW, 1, background)
			canvas.text(1, y, 1, place.Icon, place.Color, background)
			m.sideHits = append(m.sideHits, sideHit{y, place.Path})
			y++
		}
		return
	}

	y := topH + 1
	drawSection := func(label string) {
		if y >= topH+body {
			return
		}
		canvas.text(1, y, sidebarW-2, label, colFaint, colSidebar)
		y++
	}
	drawPlace := func(index int, place Place) {
		if y >= topH+body {
			return
		}
		background := colSidebar
		foreground := colSidebarText
		if hasActive && index == active {
			background = colSidebarActive
			foreground = colText
		}
		if m.focus == focusSidebar && index == m.sideCursor {
			background = colSidebarActive
			foreground = colText
		}
		canvas.fillRect(0, y, sidebarW, 1, background)
		canvas.text(1, y, 1, place.Icon, place.Color, background)
		canvas.text(4, y, sidebarW-5, truncateText(place.Label, sidebarW-5), foreground, background)
		m.sideHits = append(m.sideHits, sideHit{y, place.Path})
		y++
	}

	drawSection("PLACES")
	for index, place := range m.places {
		drawPlace(index, place)
	}
	if len(m.devices) > 0 {
		if y < topH+body {
			y++
		}
		drawSection("DEVICES")
		for index, place := range m.devices {
			drawPlace(len(m.places)+index, place)
		}
	}
}

func (m *model) drawGallery(canvas *Canvas) {
	body := m.height - topH - statusH
	if body <= 0 {
		return
	}
	if len(m.entries) == 0 {
		m.drawEmpty(canvas)
		return
	}
	columns := m.gridCols()
	rows := m.visibleRows()
	m.clampScroll()
	start := m.scroll * columns
	end := start + rows*columns
	if end > len(m.entries) {
		end = len(m.entries)
	}
	_, cardWidth, startX := m.cardMetrics()
	startY := topH + 1
	for index := start; index < end; index++ {
		row := index / columns
		column := index % columns
		x := startX + column*(cardWidth+gapX)
		y := startY + (row-m.scroll)*(cardH+gapY)
		if y >= topH+body || x+cardWidth > m.width {
			continue
		}
		m.cardHits = append(m.cardHits, cardHit{x, y, cardWidth, cardH, index})
		m.drawCard(canvas, x, y, cardWidth, index)
	}
	m.drawScrollbar(canvas)
}

func (m *model) drawCard(canvas *Canvas, x, y, width, index int) {
	entry := m.entries[index]
	selected := index == m.selected
	border := colBG
	fill := colBG
	if selected {
		border = colAccent
		fill = colSelected
	}
	canvas.box(x, y, width, cardH, border, fill)
	contentWidth := width - 2
	imageX := x + 1 + (contentWidth-imgW)/2
	if imageX < x+1 {
		imageX = x + 1
	}
	canvas.fillRect(imageX, y+1, imgW, imgH, fill)

	thumb := m.thumbs[entry.Path]
	if thumb != nil && (thumb.ModTime != entry.ModTime.UnixNano() || thumb.Size != entry.Size) {
		thumb = nil
	}
	drawImage := false
	if thumb != nil {
		switch m.protocol {
		case protoTGP:
			if thumb.Uploaded {
				for row := 0; row < imgH; row++ {
					for column := 0; column < imgW; column++ {
						canvas.putImage(imageX+column, y+1+row, placeholder(thumb.ID, row, column), kittyColor(thumb.ID), fill)
					}
				}
				drawImage = true
			}
		default:
			if len(thumb.Half) >= imgW*imgH {
				for row := 0; row < imgH; row++ {
					for column := 0; column < imgW; column++ {
						cell := thumb.Half[row*imgW+column]
						canvas.putImage(imageX+column, y+1+row, "▀", cell.Top, cell.Bottom)
					}
				}
				drawImage = true
			}
		}
	}
	if !drawImage {
		if entry.Kind == "folder" {
			drawFolderGlyph(canvas, imageX, y+1, fill)
		} else {
			style := styleFor(entry)
			background := tileColor(style)
			canvas.fillRect(imageX, y+1, imgW, imgH, background)
			canvas.centerText(imageX, y+2, imgW, style.icon, style.color, background)
			canvas.centerText(imageX, y+4, imgW, style.label, colMuted, background)
		}
	}

	canvas.fillRect(x+1, y+cardH-2, width-2, 1, fill)
	name := m.entryLabel(entry)
	if entry.IsLink {
		name += " " + iconLink
	}
	labelColor := colMuted
	if selected {
		labelColor = colText
	}
	canvas.centerText(x+1, y+cardH-2, width-2, name, labelColor, fill)
}

func (m *model) drawScrollbar(canvas *Canvas) {
	total := m.totalRows()
	visible := m.visibleRows()
	if total <= visible {
		return
	}
	trackTop := topH + 1
	trackHeight := m.height - topH - statusH - 2
	if trackHeight <= 0 {
		return
	}
	canvas.fillRect(m.width-1, trackTop, 1, trackHeight, colBG)
	thumbHeight := trackHeight * visible / total
	if thumbHeight < 1 {
		thumbHeight = 1
	}
	maxScroll := total - visible
	position := 0
	if maxScroll > 0 {
		position = m.scroll * (trackHeight - thumbHeight) / maxScroll
	}
	for row := 0; row < thumbHeight; row++ {
		canvas.fillRect(m.width-1, trackTop+position+row, 1, 1, colHover)
	}
}

func (m *model) drawEmpty(canvas *Canvas) {
	width := m.width - sidebarW
	if width <= 0 {
		return
	}
	centerY := topH + (m.height-topH-statusH)/2
	message := "Empty folder"
	if m.searching {
		message = "Searching…"
	} else if m.searchResults {
		message = fmt.Sprintf("No matches for “%s”", m.filter)
	}
	canvas.centerText(sidebarW, centerY-1, width, message, colMuted, colBG)
	if !m.searchResults && !m.searching && !m.hidden {
		canvas.centerText(sidebarW, centerY+1, width, "Press . to show hidden files", colFaint, colBG)
	}
}

func (m *model) drawStatus(canvas *Canvas) {
	y := m.height - statusH
	canvas.fillRect(0, y, m.width, statusH, colStatus)
	shown := len(m.entries)
	total := len(m.all)
	label := fmt.Sprintf("%d items", shown)
	if shown == 1 {
		label = "1 item"
	}
	if m.searching {
		label = "Searching…"
	} else if m.searchResults {
		if m.searchTruncated {
			label = "500+ results"
		} else if shown == 1 {
			label = "1 result"
		} else {
			label = fmt.Sprintf("%d results", shown)
		}
	} else if shown != total {
		label = fmt.Sprintf("%d of %d items", shown, total)
	}
	canvas.text(2, y, m.width-4, label, colSidebarText, colStatus)

	disk := fmt.Sprintf("%s · %s free", m.fsType, humanBytes(m.free))
	diskX := m.width - 2 - runewidth.StringWidth(disk)
	if diskX > 20 {
		canvas.text(diskX, y, runewidth.StringWidth(disk), disk, colMuted, colStatus)
	}
	if m.notice != "" {
		canvas.centerText(0, y, m.width, m.notice, colRed, colStatus)
	}
}

func (m *model) helpRows() [][2]string {
	rows := [][2]string{
		{"← ↑ ↓ →  hjkl", "move around the grid"},
		{"enter", "open folder or file"},
		{"double click", "open"},
		{"backspace", "parent folder"},
		{"alt+← / alt+→", "back / forward"},
		{"~", "home"},
		{".", "show or hide dotfiles"},
		{"/ or ctrl+f", "search this folder tree"},
		{"c / x / v", "copy / cut / paste"},
		{"F2", "rename"},
		{"n", "new folder"},
		{"Del", "move to trash"},
		{"space", "preview image"},
		{"m / right click", "file actions"},
		{"tab", "jump to the sidebar"},
		{"r", "refresh"},
		{"?", "this help"},
		{"q / ctrl+c", "quit"},
	}
	rows = append(rows, [2]string{"", ""})
	rows = append(rows, [2]string{"images", protocolName(m.protocol)})
	return rows
}

func (m *model) drawHelp(canvas *Canvas) {
	panelWidth := 64
	if panelWidth > m.width-2 {
		panelWidth = m.width - 2
	}
	if panelWidth < 16 {
		panelWidth = 16
	}
	panelHeight := 27
	if panelHeight > m.height-2 {
		panelHeight = m.height - 2
	}
	if panelHeight < 8 {
		panelHeight = 8
	}
	x := (m.width - panelWidth) / 2
	y := (m.height - panelHeight) / 2
	canvas.dimOutside(x, y, panelWidth, panelHeight)
	canvas.fillRect(x, y, panelWidth, panelHeight, colPanel)
	canvas.box(x, y, panelWidth, panelHeight, colAccent, colPanel)
	canvas.text(x+2, y+1, panelWidth-4, "fleek  keys", colText, colPanel)

	keyWidth := 18
	if panelWidth < 62 {
		keyWidth = 14
	}
	actionX := x + 2 + keyWidth
	actionWidth := panelWidth - keyWidth - 4
	if actionWidth < 4 {
		actionWidth = 4
	}
	contentStart := y + 3
	footerY := y + panelHeight - 2
	available := footerY - contentStart
	if available < 1 {
		available = 1
	}
	rows := m.helpRows()
	maxOffset := len(rows) - available
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.helpOffset > maxOffset {
		m.helpOffset = maxOffset
	}
	if m.helpOffset < 0 {
		m.helpOffset = 0
	}
	for index := 0; index < available && m.helpOffset+index < len(rows); index++ {
		row := rows[m.helpOffset+index]
		if row[0] == "" {
			continue
		}
		rowY := contentStart + index
		canvas.text(x+2, rowY, keyWidth, truncateText(row[0], keyWidth), colAccent, colPanel)
		canvas.text(actionX, rowY, actionWidth, truncateText(row[1], actionWidth), colSidebarText, colPanel)
	}
	hint := "esc close"
	if maxOffset > 0 {
		hint = "↑↓ scroll   ·   esc close"
	}
	canvas.centerText(x+2, footerY, panelWidth-4, hint, colFaint, colPanel)
}

func drawFolderGlyph(canvas *Canvas, x, y int, fill RGB) {
	background := blend(fill, colAccent, 0.10)
	canvas.fillRect(x, y, imgW, imgH, background)
	canvas.centerText(x, y+3, imgW, iconFolder, colAccent, background)
}
