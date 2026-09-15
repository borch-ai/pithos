//go:build windows

package pipeline

import (
	"errors"
)

func createTestFIFO(path string) error {
	return errors.New("fifos not supported on windows")
}
