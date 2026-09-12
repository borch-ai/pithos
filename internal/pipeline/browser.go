package pipeline

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/borch-ai/pithos/internal/logger"
)

var (
	goos               = runtime.GOOS
	execCommandContext = exec.CommandContext
	lookPathFunc       = exec.LookPath
)

// defaultOpenBrowser executes the system command to open the browser using CommandContext.
//
// G204: urlStr is the generated local HTML preview path, which is constructed by the pipeline.
//
//nolint:gosec
func defaultOpenBrowser(ctx context.Context, urlStr string) error {
	var cmd *exec.Cmd
	switch goos {
	case "windows":
		cmd = execCommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", urlStr)
	case "darwin":
		cmd = execCommandContext(ctx, "open", urlStr)
	case "linux":
		cmd = execCommandContext(ctx, "xdg-open", urlStr)
	default:
		cmd = execCommandContext(ctx, "xdg-open", urlStr)
	}
	err := cmd.Start()
	if err == nil {
		// Reap process resources in the background to prevent zombie process leaks
		go func() {
			_ = cmd.Wait()
		}()
	}
	return err
}

// openBrowserFunc is a package-level variable that can be stubbed during unit tests.
var openBrowserFunc = func(ctx context.Context, urlStr string) error {
	if isTestEnv() {
		return nil
	}
	return defaultOpenBrowser(ctx, urlStr)
}

func isTestEnv() bool {
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			return true
		}
	}
	return strings.HasSuffix(os.Args[0], ".test") || strings.HasSuffix(os.Args[0], ".test.exe")
}

// HeadlessMode indicates whether browser launches should be globally suppressed.
var HeadlessMode bool

func isTruthy(val string) bool {
	v := strings.ToLower(strings.TrimSpace(val))
	return v == "1" || v == "true" || v == "yes" || v == "on" || v == "t" || v == "y"
}

func isHeadlessBrowser() bool {
	if HeadlessMode {
		return true
	}
	return isTruthy(os.Getenv("PITHOS_HEADLESS"))
}

func openBrowser(ctx context.Context, urlStr string) error {
	if isHeadlessBrowser() {
		logger.Debug("Suppressing browser open in headless mode", "url", urlStr)
		return nil
	}
	return openBrowserFunc(ctx, urlStr)
}

func triggerBrowserOpen(ctx context.Context, urlStr string) {
	if isHeadlessBrowser() {
		logger.Debug("Suppressing browser open in headless mode", "url", urlStr)
		return
	}
	if err := openBrowser(ctx, urlStr); err != nil {
		logger.Warn("Failed to automatically open browser preview", "error", err)
	}
}

// TriggerBrowserOpen formats a file path as a file:// URL and opens the browser.
func TriggerBrowserOpen(ctx context.Context, path string) {
	triggerBrowserOpen(ctx, formatFileURL(path))
}

// formatFileURL converts a local filepath into a valid file:// absolute URL.
func formatFileURL(path string) string {
	var query string
	if idx := strings.Index(path, "?"); idx != -1 {
		query = path[idx+1:]
		path = path[:idx]
	}
	if absPath, err := filepath.Abs(path); err == nil {
		path = absPath
	}
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	u := &url.URL{
		Scheme:   "file",
		Path:     p,
		RawQuery: query,
	}
	return u.String()
}
