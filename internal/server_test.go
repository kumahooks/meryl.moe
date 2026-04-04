package internal

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestFileOnlyFS_ServesFile(t *testing.T) {
	testDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(testDirectory, "lain.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	filesystem := fileOnlyFS{fileSystem: http.Dir(testDirectory)}

	file, err := filesystem.Open("lain.txt")
	if err != nil {
		t.Fatalf("Open file: %v", err)
	}

	file.Close()
}

func TestFileOnlyFS_RejectsDirectory(t *testing.T) {
	testDirectory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(testDirectory, "subdir"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	filesystem := fileOnlyFS{fileSystem: http.Dir(testDirectory)}

	_, err := filesystem.Open("subdir")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected ErrNotExist for directory, got: %v", err)
	}
}
