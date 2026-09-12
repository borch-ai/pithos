package main

import (
	"os"
	"strings"
)

// isTruthy returns true if val matches typical boolean true values.
func isTruthy(val string) bool {
	v := strings.ToLower(strings.TrimSpace(val))
	return v == "1" || v == "true" || v == "yes" || v == "on" || v == "t" || v == "y"
}

// IsHeadless determines whether Pithos should execute in non-interactive headless mode.
// It returns true if:
// 1. The --headless or --non-interactive persistent CLI flag is set.
// 2. The PITHOS_HEADLESS environment variable is set to a truthy value ("1", "true", "yes", "on", etc.).
// 3. The CI environment variable is set to a truthy value.
func IsHeadless() bool {
	if rootHeadless || rootNonInteractive {
		return true
	}
	if isTruthy(os.Getenv("PITHOS_HEADLESS")) {
		return true
	}
	if isTruthy(os.Getenv("CI")) {
		return true
	}
	return false
}
