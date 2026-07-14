package pipeline

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
)

//nolint:funlen // TestHandleReload_Basic verifies multiple hot-reload paths in sequence
func TestHandleReload_Basic(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-watcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Scaffolding a mock configuration
	cfgFile := filepath.Join(tmpDir, ".pithos.toml")
	typstPath := strings.ReplaceAll(os.Args[0], "\\", "\\\\")
	cfgContent := fmt.Sprintf("workspaces_root = %q\nconcurrency = 1\n[mcp]\ntypst_path = %q\n", tmpDir, typstPath)
	//nolint:gosec // Test scaffolding using secure temp directory
	if writeErr := os.WriteFile(cfgFile, []byte(cfgContent), 0600); writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}
	if _, loadErr := config.LoadConfig(cfgFile); loadErr != nil {
		t.Fatalf("failed to load config: %v", loadErr)
	}

	// Create a mock manifest
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m := manifest.NewManifest(manifestPath)
	m.BookProperties.Theme = "Initial Theme"
	m.BookProperties.Style = "initial-style"
	m.BookProperties.CharacterProfile = "initial-character"
	m.BookProperties.TrimSize = "6x9"
	m.BookProperties.Format = "paperback"
	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{
			PageIndex:          1,
			Status:             manifest.StatusCompleted,
			Text:               "Initial Stanza 1",
			ImagePath:          "images/page_1.png",
			IllustrationPrompt: "initial prompt 1",
		},
	}
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	ctx := context.Background()

	// 1. Run reload without manuscript.md
	if reloadErr := handleReload(ctx, tmpDir, cfgFile, true); reloadErr != nil {
		t.Fatalf("handleReload failed: %v", reloadErr)
	}

	// Verify web preview files were created
	previewJSPath := filepath.Join(tmpDir, "web_preview", "data.js")
	if _, statErr := os.Stat(previewJSPath); statErr != nil {
		t.Fatalf("expected web preview data.js to exist: %v", statErr)
	}

	// 2. Create manuscript.md and modify stanza and style
	manuscriptPath := filepath.Join(tmpDir, "manuscript.md")
	manuscriptContent := `<!-- Style: new-style -->
<!-- CharacterProfile: new-character -->

# Page 1
<!-- Layout: full-page -->
## Text
Modified Stanza 1
## Prompt
new prompt 1
`
	if writeErr := os.WriteFile(manuscriptPath, []byte(manuscriptContent), 0600); writeErr != nil {
		t.Fatalf("failed to write manuscript.md: %v", writeErr)
	}

	// Run reload again
	if reloadErr2 := handleReload(ctx, tmpDir, cfgFile, true); reloadErr2 != nil {
		t.Fatalf("handleReload failed with manuscript: %v", reloadErr2)
	}

	// Load manifest again to verify changes
	mUpdated, loadManifestErr := manifest.LoadManifest(manifestPath)
	if loadManifestErr != nil {
		t.Fatalf("failed to load updated manifest: %v", loadManifestErr)
	}

	if mUpdated.BookProperties.Style != "new-style" {
		t.Errorf("expected style 'new-style', got %q", mUpdated.BookProperties.Style)
	}
	if mUpdated.BookProperties.CharacterProfile != "new-character" {
		t.Errorf("expected characterProfile 'new-character', got %q", mUpdated.BookProperties.CharacterProfile)
	}
	if len(mUpdated.Progress.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(mUpdated.Progress.Pages))
	}
	if mUpdated.Progress.Pages[0].Text != "Modified Stanza 1" {
		t.Errorf("expected text 'Modified Stanza 1', got %q", mUpdated.Progress.Pages[0].Text)
	}

	// 3. Test Typst PDF compilation via handleReload by setting TypstPath to a valid executable (e.g. os.Args[0])
	oldTypstPath := config.Cfg.MCP.TypstPath
	config.Cfg.MCP.TypstPath = os.Args[0] // guarantees LookPath passes
	t.Cleanup(func() {
		config.Cfg.MCP.TypstPath = oldTypstPath
	})
	if err := handleReload(ctx, tmpDir, cfgFile, true); err != nil {
		t.Fatalf("handleReload failed during typst test: %v", err)
	}
	pdfPath := filepath.Join(tmpDir, "interior.pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		t.Fatalf("expected interior.pdf to be generated in dry-run mode, got error: %v", err)
	}
}

func TestHandleReload_InvalidManuscript(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-watcher-invalid-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Scaffolding config
	cfgFile := filepath.Join(tmpDir, ".pithos.toml")
	if writeErr := os.WriteFile(cfgFile, []byte(""), 0600); writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}
	if _, loadErr := config.LoadConfig(cfgFile); loadErr != nil {
		t.Fatalf("failed to load config: %v", loadErr)
	}

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m := manifest.NewManifest(manifestPath)
	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Text: "Initial"},
	}
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	// Create invalid manuscript.md (empty/invalid pages)
	manuscriptPath := filepath.Join(tmpDir, "manuscript.md")
	if writeErr := os.WriteFile(manuscriptPath, []byte("Invalid contents without Page header"), 0600); writeErr != nil {
		t.Fatalf("failed to write manuscript: %v", writeErr)
	}

	ctx := context.Background()
	// Should return error from importManuscriptFromMarkdown
	err = handleReload(ctx, tmpDir, cfgFile, true)
	if err == nil {
		t.Fatal("expected handleReload to fail with invalid manuscript, got nil")
	}
	if !strings.Contains(err.Error(), "failed to import edits") {
		t.Errorf("expected error to contain 'failed to import edits', got: %v", err)
	}
}

func TestWatchWorkspace_ContextCancelled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-watcher-loop-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Setup config
	cfgFile := filepath.Join(tmpDir, ".pithos.toml")
	if err := os.WriteFile(cfgFile, []byte(""), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	if _, err := config.LoadConfig(cfgFile); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m := manifest.NewManifest(manifestPath)
	if err := m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := WatchOptions{
		BookDir:    tmpDir,
		ConfigFile: cfgFile,
		DryRun:     true,
	}

	// We start the watcher in a goroutine and cancel it after a short delay
	errChan := make(chan error, 1)
	go func() {
		errChan <- WatchWorkspace(ctx, opts)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("expected nil error on clean cancellation shutdown, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for WatchWorkspace to exit")
	}
}

func TestWatchWorkspace_TriggersReload(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-watcher-reload-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Scaffolding a mock configuration
	cfgFile := filepath.Join(tmpDir, ".pithos.toml")
	if writeErr := os.WriteFile(cfgFile, []byte("workspaces_root = \""+tmpDir+"\"\nconcurrency = 1\n"), 0600); writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}
	if _, loadErr := config.LoadConfig(cfgFile); loadErr != nil {
		t.Fatalf("failed to load config: %v", loadErr)
	}

	// Create a mock manifest
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m := manifest.NewManifest(manifestPath)
	m.BookProperties.Theme = "Watch Theme"
	m.BookProperties.Style = "watch-style"
	m.BookProperties.Format = "paperback"
	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Text: "Initial"},
	}
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := WatchOptions{
		BookDir:    tmpDir,
		ConfigFile: cfgFile,
		DryRun:     true,
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- WatchWorkspace(ctx, opts)
	}()

	// Wait for watcher to register folder
	time.Sleep(250 * time.Millisecond)

	// Write dummy.txt (non-watched file to hit non-watched skip branch)
	dummyPath := filepath.Join(tmpDir, "dummy.txt")
	if writeErr := os.WriteFile(dummyPath, []byte("ignored"), 0600); writeErr != nil {
		t.Fatalf("failed to write dummy: %v", writeErr)
	}

	manuscriptPath := filepath.Join(tmpDir, "manuscript.md")
	manuscriptContent := `<!-- Style: reload-style -->
# Page 1
## Text
Reloaded text
## Prompt
Reloaded prompt
`
	if writeErr := os.WriteFile(manuscriptPath, []byte(manuscriptContent), 0600); writeErr != nil {
		t.Fatalf("failed to write manuscript: %v", writeErr)
	}

	// Write .pithos.toml again to hit watched config change branch
	if writeErr := os.WriteFile(cfgFile, []byte("workspaces_root = \""+tmpDir+"\"\nconcurrency = 1\n"), 0600); writeErr != nil {
		t.Fatalf("failed to write config again: %v", writeErr)
	}

	// Poll manifest file until reload completes or timeout is hit
	var mUpdated *manifest.Manifest
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var loadManifestErr error
		mUpdated, loadManifestErr = manifest.LoadManifest(manifestPath)
		if loadManifestErr == nil && mUpdated.BookProperties.Style == "reload-style" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()

	// Wait for watcher exit
	select {
	case watchErr := <-errChan:
		if watchErr != nil {
			t.Errorf("expected nil error on clean cancellation shutdown, got: %v", watchErr)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for WatchWorkspace to exit")
	}

	// Verify reload occurred (manifest updated style)
	if mUpdated == nil || mUpdated.BookProperties.Style != "reload-style" {
		t.Errorf("expected style 'reload-style', got %v (reload did not trigger/propagate)", mUpdated)
	}
}

func TestHandleReload_ManifestLoadError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-watcher-manifest-err-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	ctx := context.Background()
	// No manifest exists in tmpDir
	err = handleReload(ctx, tmpDir, "", true)
	if err == nil {
		t.Fatal("expected handleReload to fail due to missing manifest, got nil")
	}
	if !strings.Contains(err.Error(), "failed to reload manifest") {
		t.Errorf("expected error to contain 'failed to reload manifest', got: %v", err)
	}
}

func TestHandleReload_GenerateWebPreviewError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-watcher-preview-err-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Save a valid manifest
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m := manifest.NewManifest(manifestPath)
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	// Create a file where the web_preview directory should be created
	blockingFile := filepath.Join(tmpDir, "web_preview")
	if writeErr := os.WriteFile(blockingFile, []byte(""), 0600); writeErr != nil {
		t.Fatalf("failed to write blocking file: %v", writeErr)
	}

	ctx := context.Background()
	err = handleReload(ctx, tmpDir, "", true)
	if err == nil {
		t.Fatal("expected handleReload to fail due to blocked web_preview folder, got nil")
	}
	if !strings.Contains(err.Error(), "failed to generate web preview") {
		t.Errorf("expected error to contain 'failed to generate web preview', got: %v", err)
	}
}

func TestWatchWorkspace_InvalidDirectory(t *testing.T) {
	ctx := context.Background()
	opts := WatchOptions{
		BookDir: "/nonexistent/directory/path/here",
	}
	err := WatchWorkspace(ctx, opts)
	if err == nil {
		t.Fatal("expected WatchWorkspace to fail with non-existent directory, got nil")
	}
}

func TestHandleReload_ConfigLoadError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-watcher-cfg-err-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Save a valid manifest
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m := manifest.NewManifest(manifestPath)
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Text: "Text"},
	}
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	ctx := context.Background()
	// Pass an invalid config file with bad TOML
	badCfgFile := filepath.Join(tmpDir, "invalid.toml")
	if writeErr := os.WriteFile(badCfgFile, []byte("invalid = = = TOML"), 0600); writeErr != nil {
		t.Fatalf("failed to write bad config: %v", writeErr)
	}

	// Should reload cleanly because ConfigLoadError is logged as warning but does not fail the reload
	err = handleReload(ctx, tmpDir, badCfgFile, true)
	if err != nil {
		t.Fatalf("expected handleReload to succeed despite config error, got: %v", err)
	}
}

func TestHandleReload_TypstCompileError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-watcher-typst-err-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m := manifest.NewManifest(manifestPath)
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Text: "Text"},
	}
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	oldTypstPath := config.Cfg.MCP.TypstPath
	config.Cfg.MCP.TypstPath = os.Args[0] // guarantees LookPath passes
	t.Cleanup(func() {
		config.Cfg.MCP.TypstPath = oldTypstPath
	})

	cfgFile := filepath.Join(tmpDir, ".pithos.toml")
	typstPath := strings.ReplaceAll(os.Args[0], "\\", "\\\\")
	cfgContent := fmt.Sprintf("[mcp]\ntypst_path = %q\n", typstPath)
	//nolint:gosec // Test scaffolding using secure temp directory
	if writeErr := os.WriteFile(cfgFile, []byte(cfgContent), 0600); writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}

	ctx := context.Background()
	// Run with dryRun = false so it attempts starting the binary as MCP server and fails
	err = handleReload(ctx, tmpDir, "", false)
	if err == nil {
		t.Fatal("expected handleReload to fail with typst compile error, got nil")
	}
	if !strings.Contains(err.Error(), "interior PDF compilation failed") {
		t.Errorf("expected error to contain 'interior PDF compilation failed', got: %v", err)
	}
}

// Redirect stdout/stderr helper to capture output during printSuccessAlert test
func captureStdout(t *testing.T, f func()) string {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	defer func() {
		os.Stdout = old
		_ = r.Close()
		_ = w.Close()
	}()

	f()

	_ = w.Close()

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestPrintSuccessAlert(t *testing.T) {
	output := captureStdout(t, func() {
		printSuccessAlert("TestBookName")
	})
	if !strings.Contains(output, "HOT-RELOAD SUCCESSFUL") {
		t.Errorf("expected output to contain 'HOT-RELOAD SUCCESSFUL', got %q", output)
	}
	if !strings.Contains(output, "TestBookName") {
		t.Errorf("expected output to contain 'TestBookName', got %q", output)
	}
}
