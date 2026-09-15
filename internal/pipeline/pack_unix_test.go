//go:build !windows

package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func createTestFIFO(path string) error {
	return syscall.Mkfifo(path, 0600)
}

func TestOpenDeliverableHandle_Unix(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-open-deliv-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// 1. Missing file
	_, err = openDeliverableHandle(filepath.Join(tmpDir, "missing.pdf"))
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}

	// 2. Regular file
	regPath := filepath.Join(tmpDir, "regular.pdf")
	if wErr := os.WriteFile(regPath, []byte("CONTENT"), 0600); wErr != nil {
		t.Fatal(wErr)
	}
	f, err := openDeliverableHandle(regPath)
	if err != nil {
		t.Fatalf("unexpected error for regular file: %v", err)
	}
	_ = f.Close()

	// 3. Symlink
	symPath := filepath.Join(tmpDir, "sym.pdf")
	if symErr := os.Symlink(regPath, symPath); symErr != nil {
		t.Fatal(symErr)
	}
	_, err = openDeliverableHandle(symPath)
	if err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
		t.Errorf("expected symlink error, got: %v", err)
	}
}
