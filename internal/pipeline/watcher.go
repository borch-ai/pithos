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
	if opts.BookDir == "" {
		return fmt.Errorf("book directory is not specified")
	}

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

	// Set up a reload channel and worker goroutine to serialize handleReload executions
	reloadChan := make(chan struct{}, 1)
	var workerWg sync.WaitGroup
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer func() {
		mu.Lock()
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		mu.Unlock()

		cancelWorker()
		workerWg.Wait()
	}()

	workerWg.Add(1)
	go func() {
		defer workerWg.Done()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-reloadChan:
				select {
				case <-workerCtx.Done():
					return
				default:
				}
				if err := handleReload(workerCtx, absBookDir, opts.ConfigFile, opts.DryRun); err != nil {
					logger.Error("Hot-reload failed", "error", err)
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping workspace watcher: context cancelled")
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return fmt.Errorf("watcher events channel closed unexpectedly")
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
					select {
					case reloadChan <- struct{}{}:
					default:
						// Already queued, no need to block or queue another
					}
				})
				mu.Unlock()
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return fmt.Errorf("watcher errors channel closed unexpectedly")
			}
			return fmt.Errorf("watcher filesystem error: %w", err)
		}
	}
}

// handleReload loads configuration and manifest updates, updates previews, and recompiles PDFs.
//
//nolint:gocognit // handleReload handles sequential steps of configuration reloading, manuscript importing, preview generation and PDF compilation.
func handleReload(ctx context.Context, bookDir string, configFile string, dryRun bool) error {
	logger.Info("Change detected. Starting hot-reload...")

	// 1. Reload configuration
	// If a book-local configuration exists, reload from it; otherwise, fall back to the custom configuration file.
	bookLocalConfig := filepath.Join(bookDir, ".pithos.toml")
	reloadFile := bookLocalConfig
	if _, err := os.Stat(bookLocalConfig); err != nil {
		if os.IsNotExist(err) {
			reloadFile = configFile
		} else {
			return fmt.Errorf("failed to check local configuration status: %w", err)
		}
	}
	if reloadFile != "" {
		if _, err := config.LoadConfig(reloadFile); err != nil {
			logger.Warn("Failed to reload configuration, using previous configuration", "error", err)
		}
	}

	// 2. Load latest manifest
	manifestPath := filepath.Join(bookDir, "manifest.json")
	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to reload manifest: %w", err)
	}

	// 3. Import manuscript edits if manuscript.md exists
	if err := ctx.Err(); err != nil {
		return err
	}
	manuscriptPath := filepath.Join(bookDir, "manuscript.md")
	if _, err := os.Stat(manuscriptPath); err == nil {
		changed, importErr := importManuscriptFromMarkdown(bookDir, m, nil)
		if importErr != nil {
			return fmt.Errorf("failed to import edits from manuscript.md: %w", importErr)
		}
		if changed {
			logger.Info("Imported manuscript edits from manuscript.md")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check manuscript.md status: %w", err)
	}

	// 4. Regenerate web preview database
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := GenerateWebPreview(bookDir, m); err != nil {
		return fmt.Errorf("failed to generate web preview: %w", err)
	}

	// 5. Re-compile Typst PDF layout if available
	if err := ctx.Err(); err != nil {
		return err
	}
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
