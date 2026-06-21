package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// CharacterOptions holds options for generating a character seed.
type CharacterOptions struct {
	OutputDir         string
	MCPTransport      mcpsdk.Transport // For tests
	CloudMCPTransport mcpsdk.Transport // For tests
}

// GenerateCharacterSeed implements the character seed generation logic.
func GenerateCharacterSeed(ctx context.Context, opts CharacterOptions) error {
	if opts.OutputDir == "" {
		return errors.New("output directory is required")
	}
	opts.OutputDir = resolveBookPath(opts.OutputDir)

	manifestPath := filepath.Join(opts.OutputDir, "manifest.json")
	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest from %s: %w", manifestPath, err)
	}

	if m.BookProperties.CharacterProfile == "" {
		return errors.New("character profile is empty in manifest; cannot generate character seed portrait")
	}

	// Reset character reference URL so that regeneration actually triggers
	m.BookProperties.CharacterReferenceURL = ""

	charBackend := "imagen"
	if config.Cfg != nil && config.Cfg.MCP.CharacterBackend != "" {
		charBackend = config.Cfg.MCP.CharacterBackend
	}

	if err := BootstrapCharacterReference(ctx, m, opts.OutputDir, opts.MCPTransport, opts.CloudMCPTransport, charBackend); err != nil {
		return err
	}

	if err := Checkpoint(ctx, opts.OutputDir, "Generated character seed portrait"); err != nil {
		return err
	}

	if previewErr := GenerateWebPreview(opts.OutputDir, m); previewErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to generate web preview: %v\n", previewErr)
	}

	return nil
}
