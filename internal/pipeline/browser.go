package pipeline

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	goos               = runtime.GOOS
	execCommandContext = exec.CommandContext
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

func openBrowser(ctx context.Context, urlStr string) error {
	return openBrowserFunc(ctx, urlStr)
}

func triggerBrowserOpen(ctx context.Context, urlStr string) {
	if err := openBrowser(ctx, urlStr); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to automatically open browser preview: %v\n", err)
	}
}

// formatFileURL converts a local filepath into a valid file:// absolute URL.
func formatFileURL(path string) string {
	if absPath, err := filepath.Abs(path); err == nil {
		path = absPath
	}
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	u := &url.URL{
		Scheme: "file",
		Path:   p,
	}
	return u.String()
}
