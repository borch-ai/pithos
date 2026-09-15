//go:build windows

package pipeline

import (
	"fmt"
	"os"
)

func openDeliverableHandle(path string) (*os.File, error) {
	lfi, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to lstat deliverable %q: %w", path, err)
	}
	if lfi.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("security boundary violation: deliverable %q must not be a symlink", path)
	}
	if !lfi.Mode().IsRegular() {
		return nil, fmt.Errorf("deliverable %q is not a regular file", path)
	}
	//nolint:gosec // path is strictly validated within wsDir
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open deliverable file %q: %w", path, err)
	}
	return f, nil
}
