package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func archiveFormat(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return "targz"
	case strings.HasSuffix(lower, ".tar"):
		return "tar"
	}
	return ""
}

func isArchive(name string) bool {
	return archiveFormat(name) != ""
}

func archiveBase(name string) string {
	lower := strings.ToLower(name)
	for _, suffix := range []string{".tar.gz", ".tgz", ".zip", ".tar"} {
		if strings.HasSuffix(lower, suffix) {
			return name[:len(name)-len(suffix)]
		}
	}
	return name
}

func compressPaths(paths []string, dest string) error {
	format := archiveFormat(dest)
	if format == "" {
		dest += ".zip"
		format = "zip"
	}
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	var closeWriter func() error
	switch format {
	case "zip":
		writer := zip.NewWriter(file)
		closeWriter = writer.Close
		err = writeZip(writer, paths, dest)
		if closeErr := closeWriter(); err == nil {
			err = closeErr
		}
	case "tar", "targz":
		var gz *gzip.Writer
		var writer *tar.Writer
		if format == "targz" {
			gz = gzip.NewWriter(file)
			writer = tar.NewWriter(gz)
		} else {
			writer = tar.NewWriter(file)
		}
		err = writeTar(writer, paths, dest)
		if closeErr := writer.Close(); err == nil {
			err = closeErr
		}
		if gz != nil {
			if closeErr := gz.Close(); err == nil {
				err = closeErr
			}
		}
	}
	if err != nil {
		_ = os.Remove(dest)
	}
	return err
}

func writeZip(writer *zip.Writer, paths []string, dest string) error {
	absoluteDest, _ := filepath.Abs(dest)
	for _, root := range paths {
		root = filepath.Clean(root)
		base := filepath.Base(root)
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			absolute, _ := filepath.Abs(path)
			if absolute == absoluteDest {
				return nil
			}
			relative, err := filepath.Rel(filepath.Dir(root), path)
			if err != nil {
				return err
			}
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(relative)
			if info.IsDir() {
				header.Name += "/"
			}
			entry, err := writer.CreateHeader(header)
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				target, err := os.Readlink(path)
				if err != nil {
					return err
				}
				_, err = io.WriteString(entry, target)
				return err
			}
			if info.IsDir() {
				return nil
			}
			input, err := os.Open(path)
			if err != nil {
				return err
			}
			defer input.Close()
			_, err = io.Copy(entry, input)
			return err
		})
		if err != nil {
			return fmt.Errorf("%s: %w", base, err)
		}
	}
	return nil
}

func writeTar(writer *tar.Writer, paths []string, dest string) error {
	absoluteDest, _ := filepath.Abs(dest)
	for _, root := range paths {
		root = filepath.Clean(root)
		base := filepath.Base(root)
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			absolute, _ := filepath.Abs(path)
			if absolute == absoluteDest {
				return nil
			}
			relative, err := filepath.Rel(filepath.Dir(root), path)
			if err != nil {
				return err
			}
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(relative)
			if info.Mode()&os.ModeSymlink != 0 {
				target, err := os.Readlink(path)
				if err != nil {
					return err
				}
				header.Linkname = target
			}
			if err := writer.WriteHeader(header); err != nil {
				return err
			}
			if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return nil
			}
			input, err := os.Open(path)
			if err != nil {
				return err
			}
			defer input.Close()
			_, err = io.Copy(writer, input)
			return err
		})
		if err != nil {
			return fmt.Errorf("%s: %w", base, err)
		}
	}
	return nil
}

func extractArchive(path, dest string) error {
	format := archiveFormat(path)
	if format == "" {
		return fmt.Errorf("unsupported archive")
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	switch format {
	case "zip":
		return extractZip(path, dest)
	case "tar", "targz":
		return extractTar(path, dest, format == "targz")
	}
	return fmt.Errorf("unsupported archive")
}

func safeArchiveTarget(root, name string) (string, bool) {
	name = filepath.Clean(filepath.FromSlash(name))
	if name == "." || filepath.IsAbs(name) || strings.HasPrefix(name, ".."+string(filepath.Separator)) || name == ".." {
		return "", false
	}
	target := filepath.Join(root, name)
	if !insidePath(root, target) {
		return "", false
	}
	return target, true
}

func extractZip(path, dest string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		target, ok := safeArchiveTarget(dest, file.Name)
		if !ok {
			continue
		}
		info := file.FileInfo()
		if info.IsDir() {
			mode := info.Mode().Perm()
			if mode == 0 {
				mode = 0o755
			}
			if err := os.MkdirAll(target, mode); err != nil {
				return err
			}
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		input, err := file.Open()
		if err != nil {
			return err
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		_ = os.Chtimes(target, info.ModTime(), info.ModTime())
	}
	return nil
}

func extractTar(path, dest string, compressed bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	var reader io.Reader = file
	if compressed {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer gz.Close()
		reader = gz
	}
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target, ok := safeArchiveTarget(dest, header.Name)
		if !ok {
			continue
		}
		switch header.Typeflag {
		case tar.TypeDir:
			mode := os.FileMode(header.Mode) & 0o777
			if mode == 0 {
				mode = 0o755
			}
			if err := os.MkdirAll(target, mode); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(header.Mode) & 0o777
			if mode == 0 {
				mode = 0o644
			}
			output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(output, tarReader)
			closeErr := output.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			_ = os.Chtimes(target, header.ModTime, header.ModTime)
		}
	}
	return nil
}
