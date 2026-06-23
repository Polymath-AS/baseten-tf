package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestWriteDirectoryTarGzip(t *testing.T) {
	sourcePath := t.TempDir()
	writeFile(t, filepath.Join(sourcePath, "config.yaml"), []byte("model_name: demo\n"))
	writeFile(t, filepath.Join(sourcePath, "model", "model.py"), []byte("class Model:\n    pass\n"))

	var buffer bytes.Buffer
	if err := WriteDirectoryTarGzip(sourcePath, &buffer); err != nil {
		t.Fatalf("WriteDirectoryTarGzip: %v", err)
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(buffer.Bytes()))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer func() {
		if err := gzipReader.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()

	tarReader := tar.NewReader(gzipReader)
	entries := make([]string, 0, 2)
	contents := make(map[string]string)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			t.Fatalf("Next: %v", err)
		}

		body, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}

		entries = append(entries, header.Name)
		contents[header.Name] = string(body)
	}

	sort.Strings(entries)
	wantEntries := []string{"config.yaml", "model/model.py"}
	if !reflect.DeepEqual(entries, wantEntries) {
		t.Fatalf("entries = %#v, want %#v", entries, wantEntries)
	}

	if contents["config.yaml"] != "model_name: demo\n" {
		t.Fatalf("config contents = %q, want model config", contents["config.yaml"])
	}
}

func TestWriteDirectoryTarGzipRejectsFiles(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "model.py")
	writeFile(t, sourcePath, []byte("class Model:\n    pass\n"))

	var buffer bytes.Buffer
	err := WriteDirectoryTarGzip(sourcePath, &buffer)
	if err == nil {
		t.Fatal("WriteDirectoryTarGzip accepted a file source")
	}
}

func writeFile(t *testing.T, path string, contents []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
