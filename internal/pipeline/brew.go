package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/pithos/internal/mcp"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// BrewOptions holds configuration parameters for the brew command.
type BrewOptions struct {
	OutputDir    string
	Theme        string
	Style        string
	MCPTransport mcpsdk.Transport // For testing
	LLM          LLMClient        // For testing
}

// Brew executes the manuscript generation and page-by-page illustration generation.
func Brew(ctx context.Context, opts BrewOptions) error {
	if opts.OutputDir == "" {
		return errors.New("output directory is required")
	}

	manifestPath := filepath.Join(opts.OutputDir, "manifest.json")
	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest from %s: %w", manifestPath, err)
	}

	// Apply overrides if specified
	hasOverrides := false
	if opts.Theme != "" {
		m.BookProperties.Theme = opts.Theme
		hasOverrides = true
	}
	if opts.Style != "" {
		m.BookProperties.Style = opts.Style
		hasOverrides = true
	}
	if hasOverrides {
		if err := m.Save(); err != nil {
			return fmt.Errorf("failed to save manifest overrides: %w", err)
		}
	}

	// 1. Generate manuscript text if not yet generated
	if err := generateManuscript(ctx, m, opts); err != nil {
		return err
	}

	// 2. Generate illustrations via MCP ImageGen server
	return generateIllustrations(ctx, m, opts)
}

func generateManuscript(ctx context.Context, m *manifest.Manifest, opts BrewOptions) error {
	if m.Progress.ManuscriptGenerated {
		return nil
	}

	theme := m.BookProperties.Theme
	if theme == "" {
		return errors.New("theme is required for manuscript generation")
	}

	pageCount := m.BookProperties.TargetPageCount
	if pageCount <= 0 {
		pageCount = 15
		m.BookProperties.TargetPageCount = 15
	}

	var llmClient LLMClient
	switch {
	case opts.LLM != nil:
		llmClient = opts.LLM
	case config.Cfg != nil && config.Cfg.API.GeminiKey != "":
		llmClient = &GeminiClient{APIKey: config.Cfg.API.GeminiKey}
	case config.Cfg != nil && config.Cfg.API.OpenAIKey != "":
		llmClient = &OpenAIClient{APIKey: config.Cfg.API.OpenAIKey}
	default:
		return errors.New("neither Gemini nor OpenAI API key is configured")
	}

	stanzas, err := llmClient.GenerateStanzas(ctx, theme, pageCount)
	if err != nil {
		return fmt.Errorf("manuscript text generation failed: %w", err)
	}

	m.Progress.Pages = make([]manifest.PageState, len(stanzas))
	for i, text := range stanzas {
		m.Progress.Pages[i] = manifest.PageState{
			PageIndex: i + 1,
			Status:    manifest.StatusPending,
			Text:      text,
		}
	}
	m.Progress.ManuscriptGenerated = true

	if err := m.Save(); err != nil {
		return fmt.Errorf("failed to save manifest after manuscript generation: %w", err)
	}
	return nil
}

func needsIllustrationGen(m *manifest.Manifest) bool {
	for _, page := range m.Progress.Pages {
		if page.Status != manifest.StatusCompleted || page.ImagePath == "" {
			return true
		}
	}
	return false
}

func generateIllustrations(ctx context.Context, m *manifest.Manifest, opts BrewOptions) error {
	if !needsIllustrationGen(m) {
		return nil
	}

	mcpClient := mcp.NewPluginClient(mcp.PluginImageGen)
	if opts.MCPTransport != nil {
		mcpClient.SetTransport(opts.MCPTransport)
	}

	if err := mcpClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start MCP imagegen client: %w", err)
	}
	defer func() { _ = mcpClient.Stop() }()

	styleID := ""
	if m.BookProperties.Style != "" {
		var err error
		styleID, err = registerStyleProfile(ctx, mcpClient, m.BookProperties.Style)
		if err != nil {
			return err
		}
	}

	for i := range m.Progress.Pages {
		page := &m.Progress.Pages[i]
		if page.Status == manifest.StatusCompleted && page.ImagePath != "" {
			continue
		}

		page.Status = manifest.StatusGeneratingImages
		if err := m.Save(); err != nil {
			return fmt.Errorf("failed to update page status to generating: %w", err)
		}
		if err := generateSingleImage(ctx, mcpClient, page, styleID, opts.OutputDir); err != nil {
			page.Status = manifest.StatusPending
			if saveErr := m.Save(); saveErr != nil {
				return fmt.Errorf("failed to save manifest status to pending: %w (original error: %v)", saveErr, err)
			}
			return err
		}
		if err := m.Save(); err != nil {
			return fmt.Errorf("failed to save manifest after page %d image generation: %w", page.PageIndex, err)
		}
	}
	return nil
}

func registerStyleProfile(ctx context.Context, mcpClient *mcp.PluginClient, styleStr string) (string, error) {
	styleID := "pithos-style"
	promptSeed := styleStr
	var srefURL string

	if idx := strings.Index(styleStr, " --sref "); idx != -1 {
		promptSeed = strings.TrimSpace(styleStr[:idx])
		srefURL = strings.TrimSpace(styleStr[idx+len(" --sref "):])
	}

	registerArgs := map[string]interface{}{
		"style_id":    styleID,
		"prompt_seed": promptSeed,
	}
	if srefURL != "" {
		registerArgs["sref_url"] = srefURL
	}

	_, err := mcpClient.CallTool(ctx, "imagegen_register_style", registerArgs)
	if err != nil {
		return "", fmt.Errorf("failed to register style profile %q: %w", styleID, err)
	}
	return styleID, nil
}

func generateSingleImage(ctx context.Context, mcpClient *mcp.PluginClient, page *manifest.PageState, styleID string, outputDir string) error {
	generateArgs := map[string]interface{}{
		"prompt": page.Text,
		"size":   "1024x1024",
	}
	if styleID != "" {
		generateArgs["style_id"] = styleID
	}

	resText, err := mcpClient.CallTool(ctx, "imagegen_generate", generateArgs)
	if err != nil {
		return fmt.Errorf("failed to generate image for page %d: %w", page.PageIndex, err)
	}

	prefix := "Successfully generated image and saved to: "
	if !strings.HasPrefix(resText, prefix) {
		return fmt.Errorf("unexpected imagegen response format: %q", resText)
	}

	srcPath := strings.TrimPrefix(resText, prefix)
	ext := filepath.Ext(srcPath)
	if ext == "" {
		ext = ".png"
	}

	destFileName := fmt.Sprintf("page_%d%s", page.PageIndex, ext)
	destPath := filepath.Join(outputDir, "images", destFileName)

	if err := copyFile(srcPath, destPath); err != nil {
		return fmt.Errorf("failed to copy generated image for page %d: %w", page.PageIndex, err)
	}

	page.ImagePath = filepath.Join("images", destFileName)
	page.Status = manifest.StatusCompleted
	return nil
}

func copyFile(src, dst string) error {
	//nolint:gosec // src is validated return value from local MCP server tool call
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	dir := filepath.Dir(dst)
	if mkdirErr := os.MkdirAll(dir, 0750); mkdirErr != nil {
		return fmt.Errorf("failed to create destination directory: %w", mkdirErr)
	}

	//nolint:gosec // dst path is safely constructed inside configured output directory
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
