package main

import "strings"

const (
	iconHome     = "\uf015"
	iconDownload = "\uf019"
	iconDocument = "\uf15c"
	iconMusic    = "\uf001"
	iconPicture  = "\uf03e"
	iconVideo    = "\uf03d"
	iconCode     = "\uf121"
	iconTrash    = "\uf1f8"
	iconDisk     = "\uf0a0"
	iconBack     = "\uf060"
	iconForward  = "\uf061"
	iconSearch   = "\uf002"
	iconEye      = "\uf06e"
	iconEyeOff   = "\uf070"
	iconRefresh  = "\uf021"
	iconHelp     = "\uf059"
	iconSettings = "\uf013"
	iconLink     = "\uf0c1"
	iconFolder   = "\uf07b"
	iconBookmark = "\uf02e"
)

type kindStyle struct {
	icon  string
	label string
	color RGB
}

var kindStyles = map[string]kindStyle{
	"folder":  {iconFolder, "DIR", colAccent},
	"image":   {"\uf1c5", "IMG", colCyan},
	"video":   {"\uf1c8", "VID", colPurple},
	"audio":   {"\uf1c7", "AUD", colAmber},
	"pdf":     {"\uf1c1", "PDF", colRed},
	"archive": {"\uf1c6", "ZIP", colAmber},
	"code":    {iconCode, "SRC", colGreen},
	"text":    {iconDocument, "TXT", colMuted},
	"config":  {"\uf013", "CFG", colAmber},
	"doc":     {"\uf1c2", "DOC", colAccent},
	"sheet":   {"\uf1c3", "XLS", colGreen},
	"slides":  {"\uf1c4", "PPT", colOrange},
	"font":    {"\uf031", "TTF", colPurple},
	"db":      {"\uf1c0", "DB", colCyan},
	"3d":      {"\uf1b2", "3D", colOrange},
	"disk":    {iconDisk, "ISO", colAmber},
	"binary":  {"\uf120", "BIN", colRed},
	"file":    {"\uf15b", "FILE", colMuted},
}

func styleFor(entry Entry) kindStyle {
	style, ok := kindStyles[entry.Kind]
	if !ok {
		style = kindStyles["file"]
	}
	if !entry.IsDir && entry.Ext != "" {
		switch entry.Kind {
		case "code", "text", "config", "archive", "doc", "sheet", "slides", "font", "db", "3d", "disk", "binary":
			label := strings.ToUpper(entry.Ext)
			if len(label) > 4 {
				label = label[:4]
			}
			style.label = label
		}
	}
	return style
}

func tileColor(style kindStyle) RGB {
	return blend(hexRGB("#1e1e2a"), style.color, 0.13)
}

func placeIcon(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "home":
		return iconHome
	case "download":
		return iconDownload
	case "document", "documents":
		return iconDocument
	case "music":
		return iconMusic
	case "picture", "pictures":
		return iconPicture
	case "video", "videos":
		return iconVideo
	case "code", "projects":
		return iconCode
	case "bookmark", "bookmarks":
		return iconBookmark
	case "trash":
		return iconTrash
	case "disk":
		return iconDisk
	default:
		return iconFolder
	}
}
