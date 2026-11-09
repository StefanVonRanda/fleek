package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"
)

const (
	thumbWidth  = 480
	thumbHeight = 320
	halfWidth   = 23
	cacheFormat = 4
)

type HalfCell struct {
	Top    RGB
	Bottom RGB
}

type Thumb struct {
	Path     string
	ModTime  int64
	Size     int64
	Img      image.Image
	PNG      []byte
	Half     []HalfCell
	ID       uint32
	Uploaded bool
}

type ThumbService struct {
	dir string
	sem chan struct{}
}

func newThumbService() *ThumbService {
	dir := filepath.Join(os.Getenv("XDG_CACHE_HOME"), "fleek", "thumbs")
	if os.Getenv("XDG_CACHE_HOME") == "" {
		dir = filepath.Join(userHome(), ".cache", "fleek", "thumbs")
	}
	if override := os.Getenv("FLEEK_CACHE_DIR"); override != "" {
		dir = filepath.Join(override, "thumbs")
	}
	_ = os.MkdirAll(dir, 0o755)
	return &ThumbService{dir: dir, sem: make(chan struct{}, 6)}
}

func (s *ThumbService) cachePath(entry Entry) string {
	key := fmt.Sprintf("%s\x00%d\x00%d\x00%s\x00%d\x00%d", entry.Path, entry.ModTime.UnixNano(), entry.Size, entry.Kind, cacheFormat, thumbWidth)
	sum := sha1.Sum([]byte(key))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".png")
}

func (s *ThumbService) Get(entry Entry) (*Thumb, error) {
	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	cache := s.cachePath(entry)
	if content, err := os.ReadFile(cache); err == nil {
		if decoded, err := png.Decode(bytes.NewReader(content)); err == nil {
			return newThumb(entry, decoded, content), nil
		}
	}
	img, err := generateThumb(entry)
	if err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return nil, err
	}
	content := encoded.Bytes()
	writeCache(cache, content)
	return newThumb(entry, img, content), nil
}

func newThumb(entry Entry, img image.Image, content []byte) *Thumb {
	if img.Bounds().Dx() != thumbWidth || img.Bounds().Dy() != thumbHeight {
		img = imaging.Fill(img, thumbWidth, thumbHeight, imaging.Center, imaging.Lanczos)
	}
	return &Thumb{
		Path:    entry.Path,
		ModTime: entry.ModTime.UnixNano(),
		Size:    entry.Size,
		Img:     img,
		PNG:     content,
		Half:    buildHalfcellsSize(img, halfWidth, imgH),
	}
}

func writeCache(path string, content []byte) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".thumb-*")
	if err != nil {
		return
	}
	name := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		os.Remove(name)
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return
	}
	_ = os.Rename(name, path)
}

func buildHalfcellsSize(img image.Image, columns, rows int) []HalfCell {
	if columns < 1 || rows < 1 {
		return nil
	}
	small := imaging.Resize(img, columns, rows*2, imaging.Lanczos)
	cells := make([]HalfCell, 0, columns*rows)
	for y := 0; y < rows*2; y += 2 {
		for x := 0; x < columns; x++ {
			cells = append(cells, HalfCell{Top: imageColor(small.At(x, y)), Bottom: imageColor(small.At(x, y+1))})
		}
	}
	return cells
}

func imageColor(value color.Color) RGB {
	r, g, b, _ := value.RGBA()
	return RGB{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
}

func generateThumb(entry Entry) (image.Image, error) {
	switch entry.Kind {
	case "folder":
		return fontFolderTile()
	case "image":
		if entry.Ext == "svg" {
			return nil, fmt.Errorf("svg preview unsupported")
		}
		img, err := imaging.Open(entry.Path, imaging.AutoOrientation(true))
		if err != nil {
			return nil, err
		}
		return imaging.Fill(img, thumbWidth, thumbHeight, imaging.Center, imaging.Lanczos), nil
	case "video":
		return videoThumb(entry.Path)
	case "pdf":
		return pdfThumb(entry.Path)
	default:
		return nil, fmt.Errorf("no thumbnail")
	}
}

func videoFrame(path string) (image.Image, error) {
	for _, seek := range []string{"1", "0"} {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		command := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-ss", seek, "-i", path,
			"-frames:v", "1", "-vf", "scale=1280:-2", "-f", "image2pipe", "-vcodec", "png", "-")
		output, err := command.Output()
		cancel()
		if err != nil || len(output) == 0 {
			continue
		}
		img, err := imaging.Decode(bytes.NewReader(output))
		if err != nil {
			continue
		}
		return img, nil
	}
	return nil, fmt.Errorf("video frame failed")
}

func videoThumb(path string) (image.Image, error) {
	frame, err := videoFrame(path)
	if err != nil {
		return nil, err
	}
	img := imaging.Fill(frame, thumbWidth, thumbHeight, imaging.Center, imaging.Lanczos)
	return filmstrip(img), nil
}

func pdfImage(path string, scale int) (image.Image, error) {
	dir, err := os.MkdirTemp("", "fleek-pdf-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	prefix := filepath.Join(dir, "page")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "pdftoppm", "-f", "1", "-l", "1", "-singlefile", "-scale-to", fmt.Sprintf("%d", scale), "-png", path, prefix)
	if err := command.Run(); err != nil {
		return nil, err
	}
	return imaging.Open(prefix + ".png")
}

func pdfThumb(path string) (image.Image, error) {
	img, err := pdfImage(path, 540)
	if err != nil {
		return nil, err
	}
	return imaging.Fill(img, thumbWidth, thumbHeight, imaging.Center, imaging.Lanczos), nil
}

func filmstrip(img image.Image) image.Image {
	target, ok := img.(draw.Image)
	if !ok {
		converted := image.NewRGBA(img.Bounds())
		draw.Draw(converted, converted.Bounds(), img, img.Bounds().Min, draw.Src)
		target = converted
	}
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	bar := int(float64(width) * 0.052)
	holeWidth := bar * 46 / 100
	holeHeight := holeWidth * 155 / 100
	if holeWidth < 4 {
		holeWidth = 4
	}
	dark := image.NewUniform(color.RGBA{14, 13, 19, 255})
	light := image.NewUniform(color.RGBA{231, 234, 244, 255})
	for _, left := range []int{0, width - bar} {
		draw.Draw(target, image.Rect(left, 0, left+bar, height), dark, image.Point{}, draw.Src)
	}
	for index := 0; index < 5; index++ {
		center := int(float64(height) / 5 * (float64(index) + 0.5))
		for _, left := range []int{0, width - bar} {
			x := left + (bar-holeWidth)/2
			y := center - holeHeight/2
			draw.Draw(target, image.Rect(x, y, x+holeWidth, y+holeHeight), light, image.Point{}, draw.Src)
		}
	}
	return target
}
