package archive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func WriteDirectoryTarGzip(sourcePath string, writer io.Writer) (returnErr error) {
	cleanSourcePath := filepath.Clean(sourcePath)
	info, err := os.Stat(cleanSourcePath)
	if err != nil {
		return fmt.Errorf("stat archive source: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("archive source %q is not a directory", cleanSourcePath)
	}

	gzipWriter := gzip.NewWriter(writer)
	tarWriter := tar.NewWriter(gzipWriter)
	defer func() {
		if closeErr := tarWriter.Close(); returnErr == nil && closeErr != nil {
			returnErr = fmt.Errorf("close tar writer: %w", closeErr)
		}

		if closeErr := gzipWriter.Close(); returnErr == nil && closeErr != nil {
			returnErr = fmt.Errorf("close gzip writer: %w", closeErr)
		}
	}()

	err = filepath.WalkDir(cleanSourcePath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if path == cleanSourcePath {
			return nil
		}

		if entry.IsDir() {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat archive entry %q: %w", path, err)
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		relativePath, err := filepath.Rel(cleanSourcePath, path)
		if err != nil {
			return fmt.Errorf("make archive path relative for %q: %w", path, err)
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("create archive header for %q: %w", path, err)
		}
		header.Name = filepath.ToSlash(relativePath)

		if strings.HasPrefix(header.Name, "../") || header.Name == ".." {
			return fmt.Errorf("archive path escapes source directory: %q", header.Name)
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("write archive header for %q: %w", path, err)
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open archive entry %q: %w", path, err)
		}

		_, copyErr := io.Copy(tarWriter, file)
		closeErr := file.Close()
		if copyErr != nil {
			return fmt.Errorf("write archive entry %q: %w", path, copyErr)
		}

		if closeErr != nil {
			return fmt.Errorf("close archive entry %q: %w", path, closeErr)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
