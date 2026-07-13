package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/logger"
	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/pithos/internal/mcp"
	"github.com/borch-ai/pithos/internal/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
)

// WatchOptions defines configurations for watching a book workspace.
type WatchOptions struct {
	BookDir    string
	ConfigFile string
	DryRun     bool
}

// WatchWorkspace starts a filesystem watcher for changes to manuscript.md and .pithos.toml.
// It blocks until context is cancelled or a fatal watcher error occurs.
//
//nolint:gocognit // WatchWorkspace coordinates multiple asynchronous channels (cancellation, fsnotify events, errors)
func WatchWorkspace(ctx context.Context, opts WatchOptions) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to initialize fsnotify watcher: %w", err)
	}
	defer func() { _ = watcher.Close() }()

	absBookDir, err := filepath.Abs(opts.BookDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute book directory: %w", err)
	}

	if err := watcher.Add(absBookDir); err != nil {
		return fmt.Errorf("failed to add watch on book directory %s: %w", absBookDir, err)
	}

	logger.Info("Starting live workspace watcher...", "directory", absBookDir)
	logger.Info("Watching for changes to manuscript.md and .pithos.toml. Press Ctrl+C to stop.")

	var (
		mu            sync.Mutex
		debounceTimer *time.Timer
	)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping workspace watcher: context cancelled")
			return ctx.Err()

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			baseName := filepath.Base(event.Name)
			if baseName != "manuscript.md" && baseName != ".pithos.toml" {
				continue
			}

			// Watch for Write, Create, or Rename (editor temp files)
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
				mu.Lock()
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(150*time.Millisecond, func() {
					if err := handleReload(ctx, absBookDir, opts.ConfigFile, opts.DryRun); err != nil {
						logger.Error("Hot-reload failed", "error", err)
					}
				})
				mu.Unlock()
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			logger.Error("Watcher filesystem error", "error", err)
		}
	}
}

// handleReload loads configuration and manifest updates, updates previews, and recompiles PDFs.
func handleReload(ctx context.Context, bookDir string, configFile string, dryRun bool) error {
	logger.Info("Change detected. Starting hot-reload...")

	// 1. Reload configuration
	if _, err := config.LoadConfig(configFile); err != nil {
		logger.Warn("Failed to reload configuration, using previous configuration", "error", err)
	}

	// 2. Load latest manifest
	manifestPath := filepath.Join(bookDir, "manifest.json")
	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to reload manifest: %w", err)
	}

	// 3. Import manuscript edits if manuscript.md exists
	manuscriptPath := filepath.Join(bookDir, "manuscript.md")
	if _, err := os.Stat(manuscriptPath); err == nil {
		changed, importErr := importManuscriptFromMarkdown(bookDir, m, nil)
		if importErr != nil {
			return fmt.Errorf("failed to import edits from manuscript.md: %w", importErr)
		}
		if changed {
			logger.Info("Imported manuscript edits from manuscript.md")
		}
	}

	// 4. Regenerate web preview database
	if err := GenerateWebPreview(bookDir, m); err != nil {
		return fmt.Errorf("failed to generate web preview: %w", err)
	}

	// 5. Re-compile Typst PDF layout if available
	mcpClient := mcp.NewPluginClient(mcp.PluginTypst)
	binaryPath := mcpClient.ResolveBinaryPath()
	if _, err := exec.LookPath(binaryPath); err == nil {
		logger.Info("Typst plugin found. Re-compiling interior PDF...")
		opts := AssembleOptions{
			InputDir: bookDir,
			Format:   m.BookProperties.Format,
			TrimSize: m.BookProperties.TrimSize,
			Bleed:    m.KDPLayout.Bleed > 0,
			Silent:   true,
			DryRun:   dryRun,
		}
		pdfPath, compileErr := compileInteriorPDF(ctx, opts, m)
		if compileErr != nil {
			return fmt.Errorf("interior PDF compilation failed: %w", compileErr)
		}
		logger.Info("Interior PDF compiled successfully", "path", pdfPath)
	} else {
		logger.Info("Typst plugin is not available or not configured in path. Skipping PDF compilation.", "path", binaryPath)
	}

	// 6. Print styled success alert
	printSuccessAlert(filepath.Base(bookDir))

	return nil
}

func printSuccessAlert(bookName string) {
	alertStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ui.ColorGreen)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorPurple)).
		Padding(1, 2).
		Margin(1, 0)

	msg := fmt.Sprintf("🏺 PITHOS HOT-RELOAD SUCCESSFUL!\nBook: %s\nWeb preview and assets updated.", bookName)
	fmt.Println(alertStyle.Render(msg))
}
