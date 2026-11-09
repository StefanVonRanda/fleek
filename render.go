package main

import (
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
)

type Cell struct {
	Text  string
	Fg    RGB
	Bg    RGB
	Image bool
	Cont  bool
}

type Canvas struct {
	W, H  int
	Cells []Cell
}

func NewCanvas(width, height int) *Canvas {
	canvas := &Canvas{W: width, H: height, Cells: make([]Cell, width*height)}
	for index := range canvas.Cells {
		canvas.Cells[index] = Cell{Text: " ", Fg: colText, Bg: colBG}
	}
	return canvas
}

func (c *Canvas) index(x, y int) int {
	return y*c.W + x
}

func (c *Canvas) inBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < c.W && y < c.H
}

func (c *Canvas) put(x, y int, text string, fg, bg RGB) {
	if !c.inBounds(x, y) {
		return
	}
	c.Cells[c.index(x, y)] = Cell{Text: text, Fg: fg, Bg: bg}
}

func (c *Canvas) putImage(x, y int, text string, fg, bg RGB) {
	if !c.inBounds(x, y) {
		return
	}
	c.Cells[c.index(x, y)] = Cell{Text: text, Fg: fg, Bg: bg, Image: true}
}

func (c *Canvas) fillRect(x, y, width, height int, bg RGB) {
	for row := y; row < y+height; row++ {
		for column := x; column < x+width; column++ {
			if c.inBounds(column, row) {
				c.Cells[c.index(column, row)] = Cell{Text: " ", Fg: colText, Bg: bg}
			}
		}
	}
}

func (c *Canvas) text(x, y, maxWidth int, value string, fg, bg RGB) int {
	if maxWidth <= 0 {
		return x
	}
	start := x
	limit := x + maxWidth
	for _, character := range value {
		width := runewidth.RuneWidth(character)
		if width == 0 {
			if x > start {
				c.Cells[c.index(x-1, y)].Text += string(character)
			}
			continue
		}
		if x+width > limit {
			break
		}
		if !c.inBounds(x, y) {
			x += width
			continue
		}
		c.Cells[c.index(x, y)] = Cell{Text: string(character), Fg: fg, Bg: bg}
		if width == 2 && c.inBounds(x+1, y) {
			c.Cells[c.index(x+1, y)] = Cell{Text: "", Fg: fg, Bg: bg, Cont: true}
		}
		x += width
	}
	return x
}

func truncateText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(value) <= width {
		return value
	}
	var result strings.Builder
	used := 0
	for _, character := range value {
		characterWidth := runewidth.RuneWidth(character)
		if used+characterWidth > width-1 {
			break
		}
		result.WriteRune(character)
		used += characterWidth
	}
	result.WriteRune('…')
	return result.String()
}

func (c *Canvas) centerText(x, y, width int, value string, fg, bg RGB) {
	value = truncateText(value, width)
	textWidth := runewidth.StringWidth(value)
	start := x + (width-textWidth)/2
	if start < x {
		start = x
	}
	c.text(start, y, width-(start-x), value, fg, bg)
}

func (c *Canvas) box(x, y, width, height int, border, fill RGB) {
	if width < 2 || height < 2 {
		return
	}
	c.fillRect(x+1, y+1, width-2, height-2, fill)
	c.put(x, y, "╭", border, fill)
	c.put(x+width-1, y, "╮", border, fill)
	c.put(x, y+height-1, "╰", border, fill)
	c.put(x+width-1, y+height-1, "╯", border, fill)
	for column := x + 1; column < x+width-1; column++ {
		c.put(column, y, "─", border, fill)
		c.put(column, y+height-1, "─", border, fill)
	}
	for row := y + 1; row < y+height-1; row++ {
		c.put(x, row, "│", border, fill)
		c.put(x+width-1, row, "│", border, fill)
	}
}

func (c *Canvas) dimOutside(x, y, width, height int) {
	for row := 0; row < c.H; row++ {
		for column := 0; column < c.W; column++ {
			if column >= x && column < x+width && row >= y && row < y+height {
				continue
			}
			cell := &c.Cells[c.index(column, row)]
			if cell.Image {
				*cell = Cell{Text: " ", Fg: scale(cell.Bg, 0.7), Bg: scale(cell.Bg, 0.42)}
				continue
			}
			cell.Fg = scale(cell.Fg, 0.38)
			cell.Bg = scale(cell.Bg, 0.38)
		}
	}
}

func (c *Canvas) String() string {
	var builder strings.Builder
	builder.Grow(c.W * c.H * 4)
	currentFg := RGB{255, 255, 255}
	currentBg := RGB{255, 255, 255}
	styleSet := false
	for row := 0; row < c.H; row++ {
		for column := 0; column < c.W; column++ {
			cell := c.Cells[c.index(column, row)]
			if cell.Cont {
				continue
			}
			if !styleSet || cell.Fg != currentFg || cell.Bg != currentBg {
				builder.WriteString("\x1b[0m")
				builder.WriteString("\x1b[38;2;")
				builder.WriteString(strconv.Itoa(int(cell.Fg.R)))
				builder.WriteByte(';')
				builder.WriteString(strconv.Itoa(int(cell.Fg.G)))
				builder.WriteByte(';')
				builder.WriteString(strconv.Itoa(int(cell.Fg.B)))
				builder.WriteString("m\x1b[48;2;")
				builder.WriteString(strconv.Itoa(int(cell.Bg.R)))
				builder.WriteByte(';')
				builder.WriteString(strconv.Itoa(int(cell.Bg.G)))
				builder.WriteByte(';')
				builder.WriteString(strconv.Itoa(int(cell.Bg.B)))
				builder.WriteByte('m')
				currentFg, currentBg, styleSet = cell.Fg, cell.Bg, true
			}
			if cell.Text == "" {
				builder.WriteByte(' ')
			} else {
				builder.WriteString(cell.Text)
			}
		}
		builder.WriteString("\x1b[0m")
		if row < c.H-1 {
			builder.WriteByte('\n')
		}
		styleSet = false
	}
	return builder.String()
}
