//go:build !windows

package pipeline

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func openDeliverableHandle(path string) (*os.File, error) {
	// Open with O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK to prevent symlink traversal and avoid blocking on FIFOs
	//nolint:gosec // path is strictly validated within wsDir
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, syscall.ELOOP) {
			return nil, fmt.Errorf("security boundary violation: deliverable %q must not be a symlink: %w", path, err)
		}
		return nil, fmt.Errorf("failed to open deliverable file %q: %w", path, err)
	}
	return f, nil
}
