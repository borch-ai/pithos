package pipeline

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
		CharacterWeight: 100,
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
	_, _ = fmt.Fprintf(w, "The output directory %s already exists. Are you sure you want to reuse this directory and overwrite its manifest.json? (y/N): ", path)
	reader := bufio.NewReader(r)
	response, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes", nil
}
