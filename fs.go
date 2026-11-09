package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Entry struct {
	Path    string
	Name    string
	Ext     string
	IsDir   bool
	IsLink  bool
	IsExec  bool
	Size    int64
	ModTime time.Time
	Kind    string
}

var extKinds = map[string]string{}

var (
	sortMode  = "name"
	dirsFirst = true
)

func init() {
	add := func(kind string, values string) {
		for _, ext := range strings.Fields(values) {
			extKinds[ext] = kind
		}
	}
	add("image", "jpg jpeg png gif webp bmp tif tiff avif heic heif jxl ico svg")
	add("video", "mp4 mkv webm mov avi m4v mpg mpeg wmv flv")
	add("audio", "mp3 flac wav ogg opus m4a aac wma aiff mid")
	add("pdf", "pdf")
	add("archive", "zip tar gz bz2 xz zst 7z rar iso dmg cab tgz tbz2")
	add("code", "py pyi js mjs cjs ts tsx jsx rs go c h cc cpp cxx hpp java kt kts sh bash zsh fish lua rb php swift zig nim sql html htm css scss sass less vue svelte astro nix ex exs hs ml mli clj cljs vim el scala dart r jl asm wat glsl hlsl wgsl vert frag cu")
	add("text", "md markdown txt rst org log csv tsv adoc tex")
	add("config", "json jsonc yaml yml toml ini cfg conf xml plist desktop service lock env properties editorconfig gitignore")
	add("doc", "doc docx odt rtf pages")
	add("sheet", "xls xlsx ods")
	add("slides", "ppt pptx odp key")
	add("font", "ttf otf woff woff2 eot")
	add("db", "db sqlite sqlite3 mdb realm")
	add("3d", "obj fbx gltf glb stl blend dae ply usdz")
	add("disk", "img vhd vhdx qcow2 raw")
	add("binary", "exe dll so dylib bin o a lib class pyc")
}

func classify(name string, isDir, isExec bool) string {
	if isDir {
		return "folder"
	}
	ext := ""
	if index := strings.LastIndexByte(name, '.'); index >= 0 {
		ext = strings.ToLower(name[index+1:])
	}
	if kind, ok := extKinds[ext]; ok {
		return kind
	}
	if isExec {
		return "binary"
	}
	return "file"
}

func scanDir(path string, hidden bool) ([]Entry, error) {
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(dirEntries))
	for _, item := range dirEntries {
		name := item.Name()
		if !hidden && strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(path, name)
		info, err := item.Info()
		if err != nil {
			continue
		}
		isLink := info.Mode()&os.ModeSymlink != 0
		resolved, statErr := os.Stat(full)
		if statErr != nil {
			continue
		}
		isDir := resolved.IsDir()
		if !isDir && !resolved.Mode().IsRegular() {
			continue
		}
		ext := ""
		if index := strings.LastIndexByte(name, '.'); index >= 0 {
			ext = strings.ToLower(name[index+1:])
		}
		entries = append(entries, Entry{
			Path:    full,
			Name:    name,
			Ext:     ext,
			IsDir:   isDir,
			IsLink:  isLink,
			IsExec:  resolved.Mode()&0o111 != 0 && !isDir,
			Size:    resolved.Size(),
			ModTime: resolved.ModTime(),
			Kind:    classify(name, isDir, resolved.Mode()&0o111 != 0 && !isDir),
		})
	}
	sortEntries(entries)
	return entries, nil
}

func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		if dirsFirst && left.IsDir != right.IsDir {
			return left.IsDir
		}
		switch sortMode {
		case "size":
			if left.Size != right.Size {
				return left.Size > right.Size
			}
		case "modified":
			if !left.ModTime.Equal(right.ModTime) {
				return left.ModTime.After(right.ModTime)
			}
		}
		return naturalLess(left.Name, right.Name)
	})
}

var naturalChunks = regexp.MustCompile(`\d+|[^\d]+`)

func naturalLess(a, b string) bool {
	ac := naturalChunks.FindAllString(a, -1)
	bc := naturalChunks.FindAllString(b, -1)
	for i := 0; i < len(ac) && i < len(bc); i++ {
		av, aErr := strconv.ParseUint(ac[i], 10, 64)
		bv, bErr := strconv.ParseUint(bc[i], 10, 64)
		if aErr == nil && bErr == nil {
			if av != bv {
				return av < bv
			}
			continue
		}
		al, bl := strings.ToLower(ac[i]), strings.ToLower(bc[i])
		if al != bl {
			return al < bl
		}
	}
	return len(ac) < len(bc)
}

func humanBytes(value uint64) string {
	switch {
	case value >= 1_000_000_000:
		return fmt.Sprintf("%.1f GB", float64(value)/1_000_000_000)
	case value >= 1_000_000:
		return fmt.Sprintf("%.0f MB", float64(value)/1_000_000)
	case value >= 1_000:
		return fmt.Sprintf("%.0f KB", float64(value)/1_000)
	default:
		return fmt.Sprintf("%d B", value)
	}
}

func unescapeMount(value string) string {
	return regexp.MustCompile(`\\([0-7]{3})`).ReplaceAllStringFunc(value, func(match string) string {
		value, _ := strconv.ParseUint(match[1:], 8, 8)
		return string(byte(value))
	})
}

func mountTable() [][2]string {
	content, err := os.ReadFile("/proc/self/mounts")
	if err != nil {
		return nil
	}
	var result [][2]string
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			result = append(result, [2]string{unescapeMount(fields[1]), fields[2]})
		}
	}
	return result
}

func filesystemInfo(path string) (string, uint64) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		real = path
	}
	fsType := "disk"
	best := -1
	for _, mount := range mountTable() {
		root := strings.TrimRight(mount[0], "/")
		if root == "" {
			root = "/"
		}
		if root == "/" || real == root || strings.HasPrefix(real, root+"/") {
			if len(root) > best {
				best = len(root)
				fsType = mount[1]
			}
		}
	}
	var stats syscall.Statfs_t
	if err := syscall.Statfs(real, &stats); err == nil {
		return fsType, stats.Bavail * uint64(stats.Bsize)
	}
	return fsType, 0
}

type Place struct {
	Label  string
	Path   string
	Icon   string
	Color  RGB
	Device bool
}

func userHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/"
	}
	return home
}

func buildPlaces() []Place {
	home := userHome()
	candidates := []Place{
		{"Home", home, iconHome, colAccent, false},
		{"Downloads", filepath.Join(home, "Downloads"), iconDownload, colAccent, false},
		{"Documents", filepath.Join(home, "Documents"), iconDocument, colAccent, false},
		{"Music", filepath.Join(home, "Music"), iconMusic, colAmber, false},
		{"Pictures", filepath.Join(home, "Pictures"), iconPicture, colCyan, false},
		{"Videos", filepath.Join(home, "Videos"), iconVideo, colPurple, false},
		{"Projects", filepath.Join(home, "work"), iconCode, colGreen, false},
		{"Trash", filepath.Join(home, ".local/share/Trash/files"), iconTrash, colRed, false},
	}
	places := make([]Place, 0, len(candidates)+len(appConfig.Places))
	seen := map[string]bool{}
	add := func(place Place) {
		real, err := filepath.EvalSymlinks(place.Path)
		if err != nil {
			real = place.Path
		}
		if seen[real] {
			return
		}
		seen[real] = true
		places = append(places, place)
	}
	for _, place := range candidates {
		if info, err := os.Stat(place.Path); err == nil && info.IsDir() {
			add(place)
		}
	}
	for _, custom := range appConfig.Places {
		path := expandPath(custom.Path)
		if path == "" {
			continue
		}
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			continue
		}
		label := strings.TrimSpace(custom.Label)
		if label == "" {
			label = filepath.Base(path)
		}
		color := colAccent
		if parsed, ok := parseHex(custom.Color); ok {
			color = parsed
		}
		add(Place{label, path, placeIcon(custom.Icon), color, false})
	}
	return places
}

func buildDevices() []Place {
	var result []Place
	seen := map[string]bool{}
	add := func(label, path string) {
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			real = path
		}
		if seen[real] {
			return
		}
		seen[real] = true
		result = append(result, Place{label, path, iconDisk, colAmber, true})
	}
	add("File system", "/")
	for _, parent := range []string{"/media", filepath.Join("/run/media", os.Getenv("USER")), "/mnt"} {
		children, err := os.ReadDir(parent)
		if err != nil {
			continue
		}
		for _, child := range children {
			if child.IsDir() {
				add(child.Name(), filepath.Join(parent, child.Name()))
			}
		}
	}
	extraFS := map[string]bool{
		"ext4": true, "ext3": true, "btrfs": true, "xfs": true, "f2fs": true,
		"vfat": true, "exfat": true, "ntfs": true, "ntfs3": true, "fuseblk": true,
		"nfs": true, "nfs4": true, "cifs": true, "fuse.sshfs": true,
	}
	skip := []string{"/proc", "/sys", "/dev", "/run", "/snap", "/var/lib", "/boot", "/etc", "/usr", "/home"}
	for _, mount := range mountTable() {
		mountPoint, kind := mount[0], mount[1]
		if mountPoint == "/" || !extraFS[kind] {
			continue
		}
		skipMount := false
		for _, prefix := range skip {
			if mountPoint == prefix || strings.HasPrefix(mountPoint, prefix+"/") {
				skipMount = true
				break
			}
		}
		if skipMount {
			continue
		}
		if info, err := os.Stat(mountPoint); err != nil || !info.IsDir() {
			continue
		}
		add(filepath.Base(mountPoint), mountPoint)
	}
	return result
}

func launch(path string) error {
	opener, err := exec.LookPath("xdg-open")
	if err != nil {
		opener, err = exec.LookPath("gio")
		if err != nil {
			return fmt.Errorf("no file opener found")
		}
	}
	args := []string{path}
	if filepath.Base(opener) == "gio" {
		args = []string{"open", path}
	}
	command := exec.Command(opener, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return err
	}
	return nil
}
