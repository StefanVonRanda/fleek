package main

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type fileClipboard struct {
	paths []string
	cut   bool
}

type opMsg struct {
	notice     string
	selectPath string
	err        error
	cut        bool
}

func copyEntry(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(source)
		if err != nil {
			return err
		}
		return os.Symlink(link, target)
	}
	if info.IsDir() {
		if err := os.Mkdir(target, info.Mode().Perm()); err != nil {
			return err
		}
		children, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, child := range children {
			if err := copyEntry(filepath.Join(source, child.Name()), filepath.Join(target, child.Name())); err != nil {
				return err
			}
		}
		return os.Chtimes(target, info.ModTime(), info.ModTime())
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chtimes(target, info.ModTime(), info.ModTime())
}

func uniquePath(dir, name string) string {
	candidate := filepath.Join(dir, name)
	if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
		return candidate
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for index := 1; index < 10000; index++ {
		suffix := " (copy)"
		if index > 1 {
			suffix = fmt.Sprintf(" (copy %d)", index)
		}
		candidate = filepath.Join(dir, base+suffix+ext)
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return filepath.Join(dir, name)
}

func insidePath(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	return child == parent || strings.HasPrefix(child, parent+string(filepath.Separator))
}

func pasteEntries(paths []string, dest string, cut bool) (string, error) {
	var first string
	for _, source := range paths {
		if insidePath(source, dest) {
			return first, fmt.Errorf("can't paste %s inside itself", filepath.Base(source))
		}
		target := uniquePath(dest, filepath.Base(source))
		if cut {
			if err := os.Rename(source, target); err != nil {
				if err := copyEntry(source, target); err != nil {
					return first, err
				}
				if err := os.RemoveAll(source); err != nil {
					return first, err
				}
			}
		} else if err := copyEntry(source, target); err != nil {
			return first, err
		}
		if first == "" {
			first = target
		}
	}
	return first, nil
}

func trashEntry(path string) error {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(userHome(), ".local", "share")
	}
	filesDir := filepath.Join(dataHome, "Trash", "files")
	infoDir := filepath.Join(dataHome, "Trash", "info")
	if err := os.MkdirAll(filesDir, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(infoDir, 0o700); err != nil {
		return err
	}
	name := filepath.Base(path)
	target := uniquePath(filesDir, name)
	if err := os.Rename(path, target); err != nil {
		if err := copyEntry(path, target); err != nil {
			return err
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	info := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n", url.PathEscape(absolute), time.Now().Format("2006-01-02T15:04:05"))
	return os.WriteFile(filepath.Join(infoDir, filepath.Base(target)+".trashinfo"), []byte(info), 0o600)
}

func renameEntry(path, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("invalid name")
	}
	target := filepath.Join(filepath.Dir(path), name)
	if target == path {
		return path, nil
	}
	if _, err := os.Lstat(target); err == nil {
		return "", fmt.Errorf("something named %s already exists", name)
	}
	if err := os.Rename(path, target); err != nil {
		return "", err
	}
	return target, nil
}

func makeFolder(dir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("invalid name")
	}
	target := filepath.Join(dir, name)
	if err := os.Mkdir(target, 0o755); err != nil {
		return "", err
	}
	return target, nil
}
