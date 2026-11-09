package main

import (
	"image"
	"image/color"
	"image/draw"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

var (
	iconFontOnce sync.Once
	iconFontPath string
)

func findIconFont() string {
	if value := os.Getenv("FLEEK_FONT"); value != "" {
		if validIconFont(value) {
			iconFontPath = value
			return value
		}
	}
	output, err := exec.Command("fc-list", "-f", "%{file}\n", ":charset=f07b").Output()
	if err == nil {
		var fallback string
		for _, line := range strings.Split(string(output), "\n") {
			path := strings.TrimSpace(line)
			if path == "" || !validIconFont(path) {
				continue
			}
			if fallback == "" {
				fallback = path
			}
			lower := strings.ToLower(path)
			if strings.Contains(lower, "nerd") || strings.Contains(lower, "jetbrains") || strings.Contains(lower, "fira") {
				iconFontPath = path
				return path
			}
		}
		if fallback != "" {
			iconFontPath = fallback
			return fallback
		}
	}
	for _, path := range iconFontCandidates() {
		if validIconFont(path) {
			iconFontPath = path
			return path
		}
	}
	return ""
}

func iconFontCandidates() []string {
	home := userHome()
	var paths []string
	for _, root := range []string{
		filepath.Join(home, ".local", "share", "fonts"),
		filepath.Join(home, ".fonts"),
		"/usr/share/fonts",
		"/usr/local/share/fonts",
	} {
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			lower := strings.ToLower(path)
			if (strings.HasSuffix(lower, ".ttf") || strings.HasSuffix(lower, ".otf")) && strings.Contains(lower, "nerd") {
				paths = append(paths, path)
			}
			return nil
		})
		if len(paths) > 8 {
			break
		}
	}
	return paths
}

func validIconFont(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	parsed, err := opentype.Parse(data)
	if err != nil {
		return false
	}
	var buffer sfnt.Buffer
	index, err := parsed.GlyphIndex(&buffer, 0xF07B)
	return err == nil && index != 0
}

func iconFont() string {
	iconFontOnce.Do(func() {
		if iconFontPath == "" {
			findIconFont()
		}
	})
	return iconFontPath
}

func fontFolderTile() (image.Image, error) {
	path := iconFont()
	if path == "" {
		return nil, os.ErrNotExist
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	parsed, err := opentype.Parse(data)
	if err != nil {
		return nil, err
	}
	var buffer sfnt.Buffer
	index, err := parsed.GlyphIndex(&buffer, 0xF07B)
	if err != nil || index == 0 {
		return nil, os.ErrNotExist
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    210,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	defer face.Close()

	img := image.NewRGBA(image.Rect(0, 0, thumbWidth, thumbHeight))
	background := color.RGBA{32, 31, 44, 255}
	draw.Draw(img, img.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)

	glyph := string(rune(0xF07B))
	measure := font.Drawer{Face: face}
	bounds, _ := measure.BoundString(glyph)
	glyphWidth := int(bounds.Max.X-bounds.Min.X) >> 6
	glyphHeight := int(bounds.Max.Y-bounds.Min.Y) >> 6
	x := fixed.I((thumbWidth-glyphWidth)/2) - bounds.Min.X
	y := fixed.I((thumbHeight-glyphHeight)/2) - bounds.Min.Y

	shadowDrawer := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{12, 15, 25, 255}),
		Face: face,
		Dot:  fixed.Point26_6{X: x + fixed.I(5), Y: y + fixed.I(7)},
	}
	shadowDrawer.DrawString(glyph)
	drawer := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{122, 162, 247, 255}),
		Face: face,
		Dot:  fixed.Point26_6{X: x, Y: y},
	}
	drawer.DrawString(glyph)
	return img, nil
}
