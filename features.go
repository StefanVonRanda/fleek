package main

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	tea "charm.land/bubbletea/v2"
)

func trashLocations() (string, string) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(userHome(), ".local", "share")
	}
	root := filepath.Join(dataHome, "Trash")
	return filepath.Join(root, "files"), filepath.Join(root, "info")
}

func inTrash(path string) bool {
	files, _ := trashLocations()
	files, _ = filepath.Abs(files)
	path, _ = filepath.Abs(path)
	return insidePath(files, path)
}

func restoreEntries(paths []string) (string, error) {
	files, infoDir := trashLocations()
	files, _ = filepath.Abs(files)
	var first string
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil || !insidePath(files, absolute) {
			continue
		}
		name := filepath.Base(absolute)
		infoPath := filepath.Join(infoDir, name+".trashinfo")
		data, err := os.ReadFile(infoPath)
		if err != nil {
			return first, fmt.Errorf("missing trash info for %s", name)
		}
		original := ""
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "Path=") {
				value, err := url.PathUnescape(strings.TrimPrefix(line, "Path="))
				if err == nil {
					original = value
				}
				break
			}
		}
		if original == "" {
			return first, fmt.Errorf("invalid trash info for %s", name)
		}
		if err := os.MkdirAll(filepath.Dir(original), 0o755); err != nil {
			return first, err
		}
		target := original
		if _, err := os.Lstat(target); err == nil {
			target = uniquePath(filepath.Dir(original), filepath.Base(original))
		}
		if err := os.Rename(absolute, target); err != nil {
			if err := copyEntry(absolute, target); err != nil {
				return first, err
			}
			if err := os.RemoveAll(absolute); err != nil {
				return first, err
			}
		}
		_ = os.Remove(infoPath)
		if first == "" {
			first = target
		}
	}
	if first == "" {
		return "", fmt.Errorf("nothing to restore")
	}
	return first, nil
}

func emptyTrash() error {
	files, infoDir := trashLocations()
	for _, root := range []string{files, infoDir} {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, entry := range entries {
			if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *model) restoreSelected() tea.Cmd {
	paths := m.selectedPaths()
	if m.busy || len(paths) == 0 {
		return nil
	}
	m.busy = true
	m.notice = "Restoring…"
	return func() tea.Msg {
		first, err := restoreEntries(paths)
		notice := "Restored"
		if len(paths) > 1 {
			notice = fmt.Sprintf("Restored %d items", len(paths))
		}
		return opMsg{notice: notice, selectPath: first, err: err}
	}
}

func (m *model) confirmEmptyTrash() {
	m.menu = nil
	m.dialog = &dialogState{
		kind:    "confirm",
		action:  "emptytrash",
		title:   "Empty Trash",
		message: "Permanently delete everything in the Trash?",
	}
}

func (m *model) emptyTrashCmd() tea.Cmd {
	if m.busy {
		return nil
	}
	m.busy = true
	m.notice = "Emptying Trash…"
	return func() tea.Msg {
		return opMsg{notice: "Trash emptied", err: emptyTrash()}
	}
}

func (m *model) startProperties(entry *Entry) {
	if entry == nil {
		return
	}
	info, err := os.Lstat(entry.Path)
	if err != nil {
		m.notice = err.Error()
		return
	}
	kind := "File"
	if info.IsDir() {
		kind = "Folder"
	}
	if info.Mode()&os.ModeSymlink != 0 {
		kind = "Symlink"
	}
	size := humanBytes(uint64(info.Size()))
	if info.IsDir() {
		size = "—"
	}
	mode := info.Mode().String()
	owner := ""
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		owner = formatOwner(stat.Uid, stat.Gid)
	}
	lines := []string{
		"Name:     " + entry.Name,
		"Type:     " + kind,
		"Size:     " + size,
		"Mode:     " + mode,
		"Owner:    " + owner,
		"Modified: " + info.ModTime().Format("2006-01-02 15:04:05"),
		"Path:     " + entry.Path,
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if target, err := os.Readlink(entry.Path); err == nil {
			lines = append(lines, "Target:   "+target)
		}
	}
	m.menu = nil
	m.dialog = &dialogState{kind: "info", title: "Properties", lines: lines}
}

func formatOwner(uid, gid uint32) string {
	userName := strconv.FormatUint(uint64(uid), 10)
	if account, err := user.LookupId(userName); err == nil {
		userName = account.Username
	}
	groupName := strconv.FormatUint(uint64(gid), 10)
	if group, err := user.LookupGroupId(groupName); err == nil {
		groupName = group.Name
	}
	return userName + ":" + groupName
}

func (m *model) copyPath() tea.Cmd {
	entry := m.selectedEntry()
	if entry == nil {
		return nil
	}
	m.notice = "Copied path"
	encoded := base64.StdEncoding.EncodeToString([]byte(entry.Path))
	return tea.Raw("\x1b]52;c;" + encoded + "\a")
}

func (m *model) openTerminal() tea.Cmd {
	candidates := []string{os.Getenv("TERMINAL"), "x-terminal-emulator", "gnome-terminal", "konsole", "kitty", "wezterm", "alacritty", "foot", "xterm"}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		path, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		command := exec.Command(path)
		command.Dir = m.cwd
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := command.Start(); err != nil {
			m.notice = err.Error()
			return nil
		}
		m.notice = "Opened terminal"
		return nil
	}
	m.notice = "No terminal found"
	return nil
}

func (m *model) startOpenWith(entry *Entry) {
	if entry == nil {
		return
	}
	m.menu = nil
	m.dialog = &dialogState{
		kind:   "prompt",
		action: "open_with",
		title:  "Open with",
		target: entry.Path,
	}
}

func launchWith(command, target string) error {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return fmt.Errorf("empty command")
	}
	args := append(fields[1:], target)
	process := exec.Command(fields[0], args...)
	process.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return process.Start()
}

func (m *model) startNewFile() {
	m.menu = nil
	m.dialog = &dialogState{kind: "prompt", action: "mkfile", title: "New File", target: m.cwd}
}

func makeFile(dir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("invalid name")
	}
	target := filepath.Join(dir, name)
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return target, nil
}

func (m *model) duplicateSelected() tea.Cmd {
	entry := m.selectedEntry()
	if entry == nil || m.busy {
		return nil
	}
	target := uniquePath(filepath.Dir(entry.Path), entry.Name)
	m.busy = true
	m.notice = "Duplicating…"
	return func() tea.Msg {
		err := copyEntry(entry.Path, target)
		return opMsg{notice: "Duplicated " + entry.Name, selectPath: target, err: err}
	}
}

func (m *model) startNewSymlink(entry *Entry) {
	if entry == nil {
		return
	}
	m.menu = nil
	m.dialog = &dialogState{
		kind:    "prompt",
		action:  "symlink",
		title:   "New Symlink",
		target:  entry.Path,
		value:   entry.Name + "-link",
		replace: true,
	}
}

func makeSymlink(dir, name, target string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("invalid name")
	}
	link := filepath.Join(dir, name)
	if err := os.Symlink(target, link); err != nil {
		return "", err
	}
	return link, nil
}

func (m *model) startChmod() {
	paths := m.selectedPaths()
	if len(paths) == 0 {
		return
	}
	m.menu = nil
	m.dialog = &dialogState{
		kind:    "prompt",
		action:  "chmod",
		title:   "Permissions",
		value:   "644",
		paths:   paths,
		replace: true,
	}
}

func (m *model) startChown() {
	paths := m.selectedPaths()
	if len(paths) == 0 {
		return
	}
	m.menu = nil
	m.dialog = &dialogState{
		kind:    "prompt",
		action:  "chown",
		title:   "Owner",
		value:   "user:group",
		paths:   paths,
		replace: true,
	}
}

func applyChmod(paths []string, value string) error {
	number, err := strconv.ParseUint(strings.TrimPrefix(value, "0o"), 8, 32)
	if err != nil {
		return fmt.Errorf("use an octal mode like 644")
	}
	for _, path := range paths {
		if err := os.Chmod(path, os.FileMode(number)); err != nil {
			return err
		}
	}
	return nil
}

func applyChown(paths []string, value string) error {
	parts := strings.SplitN(value, ":", 2)
	uid, err := resolveUser(parts[0])
	if err != nil {
		return err
	}
	gid := -1
	if len(parts) == 2 && parts[1] != "" {
		gid, err = resolveGroup(parts[1])
		if err != nil {
			return err
		}
	}
	for _, path := range paths {
		if err := os.Chown(path, uid, gid); err != nil {
			return err
		}
	}
	return nil
}

func resolveUser(value string) (int, error) {
	if number, err := strconv.Atoi(value); err == nil {
		return number, nil
	}
	account, err := user.Lookup(value)
	if err != nil {
		return -1, fmt.Errorf("unknown user %s", value)
	}
	return strconv.Atoi(account.Uid)
}

func resolveGroup(value string) (int, error) {
	if number, err := strconv.Atoi(value); err == nil {
		return number, nil
	}
	group, err := user.LookupGroup(value)
	if err != nil {
		return -1, fmt.Errorf("unknown group %s", value)
	}
	return strconv.Atoi(group.Gid)
}

func (m *model) startBatchRename() {
	paths := m.selectedPaths()
	if len(paths) < 2 {
		m.notice = "Select at least two items"
		return
	}
	m.menu = nil
	m.dialog = &dialogState{
		kind:    "prompt",
		action:  "batchrename",
		title:   "Batch rename",
		value:   "{name}{ext}",
		paths:   paths,
		replace: true,
	}
}

func batchRename(paths []string, pattern string) (string, error) {
	var first string
	for index, path := range paths {
		name := filepath.Base(path)
		ext := filepath.Ext(name)
		stem := strings.TrimSuffix(name, ext)
		replacer := strings.NewReplacer("{name}", stem, "{ext}", ext, "{n}", strconv.Itoa(index+1))
		newName := strings.TrimSpace(replacer.Replace(pattern))
		if newName == "" || newName == "." || newName == ".." || strings.ContainsAny(newName, "/\\") {
			return first, fmt.Errorf("invalid name %q", newName)
		}
		target := filepath.Join(filepath.Dir(path), newName)
		if target == path {
			continue
		}
		if _, err := os.Lstat(target); err == nil {
			return first, fmt.Errorf("%s already exists", newName)
		}
		if err := os.Rename(path, target); err != nil {
			return first, err
		}
		if first == "" {
			first = target
		}
	}
	return first, nil
}

func (m *model) startCompress() {
	paths := m.selectedPaths()
	if len(paths) == 0 {
		return
	}
	name := "archive.zip"
	if len(paths) == 1 {
		base := filepath.Base(paths[0])
		base = strings.TrimSuffix(base, filepath.Ext(base))
		if base == "" {
			base = "archive"
		}
		name = base + ".zip"
	}
	m.menu = nil
	m.dialog = &dialogState{
		kind:    "prompt",
		action:  "compress",
		title:   "Compress",
		value:   name,
		target:  m.cwd,
		paths:   paths,
		replace: true,
	}
}

func (m *model) compressCmd(paths []string, dest string) tea.Cmd {
	if m.busy || len(paths) == 0 {
		return nil
	}
	if archiveFormat(dest) == "" {
		dest += ".zip"
	}
	m.busy = true
	m.notice = "Compressing…"
	return func() tea.Msg {
		err := compressPaths(paths, dest)
		return opMsg{notice: "Created " + filepath.Base(dest), selectPath: dest, err: err}
	}
}

func (m *model) extractCmd(path string) tea.Cmd {
	if m.busy {
		return nil
	}
	dest := uniquePath(m.cwd, archiveBase(filepath.Base(path)))
	m.busy = true
	m.notice = "Extracting…"
	return func() tea.Msg {
		err := extractArchive(path, dest)
		return opMsg{notice: "Extracted to " + filepath.Base(dest), selectPath: dest, err: err}
	}
}
