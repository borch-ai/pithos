package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/borch-ai/pithos/internal/logger"
	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/pithos/internal/registry"
	"github.com/charmbracelet/huh"
)

// InitiateOptions contains configuration fields for initializing a book workspace.
type InitiateOptions struct {
	OutputDir           string
	Title               string
	Author              string
	Theme               string
	Style               string
	Format              string
	TargetPageCount     int
	TrimSize            string
	NoBrainstorm        bool
	StrictBrainstorming bool
	LLM                 LLMClient
	HTTPClient          *http.Client
	Context             context.Context
	DryRun              bool
}

// Initiate scaffolds a new book project directory structure and writes the initial manifest.json.
func Initiate(opts InitiateOptions) (*manifest.Manifest, error) {
	if opts.OutputDir == "" {
		return nil, errors.New("output directory is required")
	}
	opts.OutputDir = resolveBookPath(opts.OutputDir)
	logger.Info("Initiating book workspace", "dir", opts.OutputDir, "theme", opts.Theme)

	// Default target page count to 15 if not specified or invalid
	if opts.TargetPageCount <= 0 {
		opts.TargetPageCount = 15
	}

	trimSize := opts.TrimSize
	if trimSize == "" {
		trimSize = "8.5x8.5"
	}

	// 1. Create target output directory
	if err := os.MkdirAll(opts.OutputDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create output directory %s: %w", opts.OutputDir, err)
	}

	// 2. Create assets/images directory
	imagesDir := filepath.Join(opts.OutputDir, "images")
	if err := os.MkdirAll(imagesDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create images directory %s: %w", imagesDir, err)
	}

	// 3. Create final output/release directory
	releaseDir := filepath.Join(opts.OutputDir, "release")
	if err := os.MkdirAll(releaseDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create release directory %s: %w", releaseDir, err)
	}

	// 4. Initialize and write initial manifest.json
	manifestPath := filepath.Join(opts.OutputDir, "manifest.json")
	m := manifest.NewManifest(manifestPath)
	m.BookProperties = manifest.BookProperties{
		Title:           opts.Title,
		Author:          opts.Author,
		Theme:           opts.Theme,
		Style:           opts.Style,
		Format:          opts.Format,
		TargetPageCount: opts.TargetPageCount,
		TrimSize:        trimSize,
		CharacterWeight: 100,
	}
	m.Kiln.Version = 1
	m.Kiln.Milestones = []string{"initiate_complete"}

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := brainstormVisualGuides(ctx, m, opts); err != nil {
		return nil, err
	}

	if err := m.Save(); err != nil {
		return nil, fmt.Errorf("failed to save initial manifest.json: %w", err)
	}

	// 5. Create initial Git checkpoint
	if err := Checkpoint(ctx, opts.OutputDir, "Initial workspace setup"); err != nil {
		return nil, fmt.Errorf("failed to create initial git checkpoint: %w", err)
	}

	// 6. Register book workspace path globally
	if err := registry.Add(opts.OutputDir); err != nil {
		logger.Warn("Failed to register book workspace path in global registry", "error", err)
	}

	logger.Info("Initial workspace setup complete and git checkpoint created", "dir", opts.OutputDir)
	return m, nil
}

// GetUniqueOutputDir resolves a unique directory path by appending a suffix (e.g. -1, -2) if the base directory exists.
func GetUniqueOutputDir(baseDir string) string {
	if _, err := os.Stat(baseDir); err != nil {
		// If it's a non-NotExist error, let the caller handle it (e.g. during MkdirAll)
		return baseDir
	}
	counter := 1
	for {
		candidate := fmt.Sprintf("%s-%d", baseDir, counter)
		if _, err := os.Stat(candidate); err != nil {
			// If candidate does not exist, or we hit a non-NotExist error, return it
			return candidate
		}
		counter++
	}
}

// ConfirmOverwrite prompts the user via r/w to confirm overwriting an existing directory.
func ConfirmOverwrite(r io.Reader, w io.Writer, path string) (bool, error) {
	if r == nil {
		r = os.Stdin
	}
	if w == nil {
		w = os.Stdout
	}

	var confirm bool
	f := huh.NewConfirm().
		Title(fmt.Sprintf("Directory %s already exists. overwrite?", path)).
		Description("Are you sure you want to reuse this directory and overwrite its manifest.json?").
		Value(&confirm)

	form := huh.NewForm(huh.NewGroup(f)).WithInput(r).WithOutput(w)
	form.WithAccessible(r != os.Stdin || !isTTY())

	if err := form.Run(); err != nil {
		return false, err
	}
	return confirm, nil
}

func brainstormVisualGuides(ctx context.Context, m *manifest.Manifest, opts InitiateOptions) error {
	if opts.NoBrainstorm || opts.Theme == "" {
		return nil
	}
	logger.Info("Brainstorming book style and character profile...", "theme", opts.Theme)

	if opts.DryRun {
		logger.Debug("Simulating brainstorming in dry-run mode")
		if m.BookProperties.Style == "" {
			m.BookProperties.Style = "Simulated style description"
		}
		if m.BookProperties.CharacterProfile == "" {
			m.BookProperties.CharacterProfile = "Simulated character profile"
		}
		return nil
	}

	llmClient, err := getLLMClient(opts.LLM, opts.HTTPClient)
	if err != nil {
		if !isTestEnv() || opts.LLM != nil || opts.StrictBrainstorming {
			return fmt.Errorf("failed to initialize LLM client for brainstorming: %w. If you wish to skip brainstorming, run with --no-brainstorm", err)
		}
		return nil
	}

	if err := generateAndRecordVisualGuides(ctx, m, opts.Theme, llmClient); err != nil {
		return err
	}
	logger.Info("Brainstorming complete")
	logger.Debug("Brainstorming results", "style", m.BookProperties.Style, "character", m.BookProperties.CharacterProfile)
	return nil
}
