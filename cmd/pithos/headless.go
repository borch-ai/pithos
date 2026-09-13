package main

import (
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

var isInputTTY = func(r io.Reader) bool {
	if f, ok := r.(*os.File); ok {
		return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
	}
	return false
}

var isStdinTTY = func() bool {
	return isInputTTY(os.Stdin)
}

// isTruthy returns true if val matches typical boolean true values.
func isTruthy(val string) bool {
	v := strings.ToLower(strings.TrimSpace(val))
	return v == "1" || v == "true" || v == "yes" || v == "on" || v == "t" || v == "y"
}

// IsHeadless determines whether Pithos should execute in non-interactive headless mode.
// Precedence order:
// 1. Explicit CLI flags: --headless / --non-interactive (true) or --interactive (false).
// 2. Environment variables: PITHOS_INTERACTIVE (false) or PITHOS_HEADLESS (true).
// 3. CI environment: CI (true).
// 4. Stdin terminal check: non-TTY stdin (true), TTY stdin (false).
func IsHeadless() bool {
	if rootHeadless || rootNonInteractive {
		return true
	}
	if rootInteractive {
		return false
	}
	if isTruthy(os.Getenv("PITHOS_INTERACTIVE")) {
		return false
	}
	if isTruthy(os.Getenv("PITHOS_HEADLESS")) {
		return true
	}
	if isTruthy(os.Getenv("CI")) {
		return true
	}
	if !isStdinTTY() {
		return true
	}
	return false
}
