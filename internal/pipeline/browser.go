package pipeline

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//nolint:gosec // G204: urlStr is the generated local HTML preview path
var openBrowserFunc = func(urlStr string) error {
	if isTestEnv() {
		return nil
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr)
	case "darwin":
		cmd = exec.Command("open", urlStr)
	case "linux":
		cmd = exec.Command("xdg-open", urlStr)
	default:
		cmd = exec.Command("xdg-open", urlStr)
	}
	return cmd.Start()
}

func isTestEnv() bool {
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			return true
		}
	}
	return strings.HasSuffix(os.Args[0], ".test") || strings.HasSuffix(os.Args[0], ".test.exe")
}

func openBrowser(urlStr string) error {
	return openBrowserFunc(urlStr)
}

func triggerBrowserOpen(urlStr string) {
	if err := openBrowser(urlStr); err != nil {
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
