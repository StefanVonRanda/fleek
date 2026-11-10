package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"strings"

	"github.com/disintegration/imaging"
	"golang.org/x/sys/unix"
)

type Preview struct {
	Entry Entry
	Img   image.Image
	PNG   []byte
	Half  []HalfCell
	Cols  int
	Rows  int
}

type previewMsg struct {
	token   int
	preview *Preview
	err     error
}

func previewable(entry Entry) bool {
	switch entry.Kind {
	case "image":
		return entry.Ext != "svg"
	case "video", "pdf":
		return true
	case "text", "code", "config":
		return true
	}
	return false
}

func textPreviewable(entry Entry) bool {
	switch entry.Kind {
	case "text", "code", "config":
		return true
	}
	return false
}

func readTextPreview(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, 2<<20))
	if err != nil {
		return nil, err
	}
	text := strings.ToValidUTF8(string(content), "�")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 20000 {
		lines = lines[:20000]
	}
	return lines, nil
}

func cellPixels() (int, int) {
	size, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err == nil && size.Col > 0 && size.Row > 0 && size.Xpixel > 0 && size.Ypixel > 0 {
		width := int(size.Xpixel) / int(size.Col)
		height := int(size.Ypixel) / int(size.Row)
		if width > 0 && height > 0 {
			return width, height
		}
	}
	return 10, 20
}

func makePreview(entry Entry, availCols, availRows, cellW, cellH int) (*Preview, error) {
	var source image.Image
	var err error
	switch entry.Kind {
	case "image":
		source, err = imaging.Open(entry.Path, imaging.AutoOrientation(true))
	case "video":
		source, err = videoFrame(entry.Path)
	case "pdf":
		source, err = pdfImage(entry.Path, 1600)
	default:
		return nil, fmt.Errorf("no preview")
	}
	if err != nil {
		return nil, err
	}
	if availCols < 4 {
		availCols = 4
	}
	if availRows < 3 {
		availRows = 3
	}
	bounds := source.Bounds()
	aspect := float64(bounds.Dx()) / float64(bounds.Dy())
	if aspect <= 0 {
		return nil, fmt.Errorf("invalid image")
	}
	rows := availRows
	cols := int(math.Round(float64(rows) * aspect * float64(cellH) / float64(cellW)))
	if cols > availCols {
		cols = availCols
		rows = int(math.Round(float64(cols) * float64(cellW) / (aspect * float64(cellH))))
	}
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	if cols >= len(diacritics) {
		cols = len(diacritics) - 1
	}
	if rows >= len(diacritics) {
		rows = len(diacritics) - 1
	}
	pixelWidth := cols * cellW
	pixelHeight := rows * cellH
	if pixelWidth > 2400 {
		pixelWidth = 2400
	}
	if pixelHeight > 1400 {
		pixelHeight = 1400
	}
	fitted := imaging.Fill(source, pixelWidth, pixelHeight, imaging.Center, imaging.Lanczos)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, fitted); err != nil {
		return nil, err
	}
	return &Preview{
		Entry: entry,
		Img:   fitted,
		PNG:   encoded.Bytes(),
		Half:  buildHalfcellsSize(fitted, cols, rows),
		Cols:  cols,
		Rows:  rows,
	}, nil
}
