package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/borch-ai/pithos/internal/manifest"
)

// InitiateOptions contains configuration fields for initializing a book workspace.
type InitiateOptions struct {
	OutputDir       string
	Theme           string
	Style           string
	Format          string
	TargetPageCount int
	TrimSize        string
}

// Initiate scaffolds a new book project directory structure and writes the initial manifest.json.
func Initiate(opts InitiateOptions) (*manifest.Manifest, error) {
	if opts.OutputDir == "" {
		return nil, errors.New("output directory is required")
	}
	opts.OutputDir = resolveBookPath(opts.OutputDir)

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
		Theme:           opts.Theme,
		Style:           opts.Style,
		Format:          opts.Format,
		TargetPageCount: opts.TargetPageCount,
		TrimSize:        trimSize,
	}
	m.Kiln.Version = 1
	m.Kiln.Milestones = []string{"initiate_complete"}

	if err := m.Save(); err != nil {
		return nil, fmt.Errorf("failed to save initial manifest.json: %w", err)
	}

	// 5. Create initial Git checkpoint
	if err := Checkpoint(context.Background(), opts.OutputDir, "Initial workspace setup"); err != nil {
		return nil, fmt.Errorf("failed to create initial git checkpoint: %w", err)
	}

	return m, nil
}
