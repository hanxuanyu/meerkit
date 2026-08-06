package plugin

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

const maxArchiveFiles = 256
const maxArchiveBytes int64 = 256 << 20

func extractArchive(archivePath, destination string) error {
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return extractZIP(archivePath, destination)
	}
	if strings.HasSuffix(strings.ToLower(archivePath), ".tar.gz") {
		return extractTarGZ(archivePath, destination)
	}
	return fmt.Errorf("plugin package must be a .zip or .tar.gz archive")
}

func extractZIP(archivePath, destination string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	if len(reader.File) > maxArchiveFiles {
		return fmt.Errorf("archive contains too many files")
	}
	seen := map[string]struct{}{}
	var total int64
	for _, file := range reader.File {
		name, err := safeArchivePath(file.Name, seen)
		if err != nil {
			return err
		}
		if file.Mode()&os.ModeSymlink != 0 || !file.Mode().IsRegular() && !file.FileInfo().IsDir() {
			return fmt.Errorf("archive entry %s is not a regular file", name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filepath.Join(destination, filepath.FromSlash(name)), 0o750); err != nil {
				return err
			}
			continue
		}
		total += int64(file.UncompressedSize64)
		if total > maxArchiveBytes {
			return fmt.Errorf("archive uncompressed size exceeds limit")
		}
		if file.CompressedSize64 > 0 && file.UncompressedSize64/file.CompressedSize64 > 200 {
			return fmt.Errorf("archive entry %s has an unsafe compression ratio", name)
		}
		input, err := file.Open()
		if err != nil {
			return err
		}
		err = writeExtractedFile(filepath.Join(destination, filepath.FromSlash(name)), input, int64(file.UncompressedSize64))
		input.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGZ(archivePath, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	seen := map[string]struct{}{}
	var count int
	var total int64
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		count++
		if count > maxArchiveFiles {
			return fmt.Errorf("archive contains too many files")
		}
		name, err := safeArchivePath(header.Name, seen)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(filepath.Join(destination, filepath.FromSlash(name)), 0o750); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			total += header.Size
			if total > maxArchiveBytes {
				return fmt.Errorf("archive uncompressed size exceeds limit")
			}
			if err := writeExtractedFile(filepath.Join(destination, filepath.FromSlash(name)), reader, header.Size); err != nil {
				return err
			}
		default:
			return fmt.Errorf("archive entry %s is not a regular file", name)
		}
	}
	return nil
}

func safeArchivePath(name string, seen map[string]struct{}) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	clean := filepath.ToSlash(filepath.Clean(name))
	if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || filepath.IsAbs(name) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	if _, exists := seen[clean]; exists {
		return "", fmt.Errorf("duplicate archive path %q", clean)
	}
	seen[clean] = struct{}{}
	return clean, nil
}
func writeExtractedFile(path string, input io.Reader, expected int64) error {
	if expected < 0 || expected > maxArchiveBytes {
		return fmt.Errorf("archive entry exceeds size limit")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.CopyN(output, input, expected)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written != expected {
		return fmt.Errorf("archive entry size mismatch")
	}
	return nil
}
