package pipeline

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

//nolint:gosec // G204: urlStr is the generated local HTML preview path
var openBrowserFunc = func(urlStr string) error {
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

func openBrowser(urlStr string) error {
	return openBrowserFunc(urlStr)
}

func triggerBrowserOpen(urlStr string) {
	if err := openBrowser(urlStr); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to automatically open browser preview: %v\n", err)
	}
}
