package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/pithos/internal/mcp"
	"github.com/borch-ai/powerword/pkg/telemetry"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// BrewOptions holds configuration parameters for the brew command.
type BrewOptions struct {
	OutputDir    string
	Theme        string
	Style        string
	Concurrency  int
	Review       bool
	MCPTransport mcpsdk.Transport // For testing
	LLM          LLMClient        // For testing
	HTTPClient   *http.Client     // For testing
}

// Brew executes the manuscript generation and page-by-page illustration generation.
//
//nolint:gocognit // Brew function integrates manifest loading, overrides, manuscript generation, review loop, and illustration generation
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

	// Import edits from manuscript.md if it exists
	manuscriptPath := filepath.Join(opts.OutputDir, "manuscript.md")
	if _, statErr := os.Stat(manuscriptPath); statErr == nil {
		changed, importErr := importManuscriptFromMarkdown(opts.OutputDir, m)
		if importErr != nil {
			return importErr
		}
		if changed {
			fmt.Println("Successfully imported edits from manuscript.md.")
		}
	}

	// 1. Generate manuscript text if not yet generated
	if err := generateManuscript(ctx, m, opts); err != nil {
		return err
	}

	if opts.Review {
		if _, statErr := os.Stat(manuscriptPath); os.IsNotExist(statErr) {
			if exportErr := exportManuscriptToMarkdown(opts.OutputDir, m.Progress.Pages); exportErr != nil {
				return exportErr
			}
		}
		return fmt.Errorf("review mode active: manuscript is available at %s. Edit the file, then run brew without --review to generate illustrations", manuscriptPath)
	}

	// 2. Generate illustrations via MCP ImageGen server
	if err := generateIllustrations(ctx, m, opts); err != nil {
		return err
	}

	// Log telemetry summary
	tracker := telemetry.NewUsageTracker()
	for model, usage := range m.Telemetry.ModelUsages {
		if usage != nil {
			tracker.RecordUsage(model, telemetry.TokenUsage{
				InputTokens:  usage.InputTokens,
				OutputTokens: usage.OutputTokens,
				CachedTokens: usage.CachedTokens,
			})
		}
	}

	var pricing map[string]telemetry.ModelPricing
	if config.Cfg != nil {
		pricing = config.Cfg.Pricing
	}

	fmt.Println("----------------------------------------")
	fmt.Print(tracker.FormatSummary(pricing))
	if m.Telemetry.ImageGenerations > 0 {
		fmt.Printf("- Image Generations: %d\n", m.Telemetry.ImageGenerations)
	}
	fmt.Printf("- Pipeline Total Cost: $%.5f\n", m.Telemetry.TotalCostUSD)
	fmt.Println("----------------------------------------")

	return nil
}

//nolint:funlen // Manuscript generation pipeline step handles LLM client setup, text generation, and recording telemetry
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
		llmClient = &GeminiClient{APIKey: config.Cfg.API.GeminiKey, Client: opts.HTTPClient}
	case config.Cfg != nil && config.Cfg.API.OpenAIKey != "":
		llmClient = &OpenAIClient{APIKey: config.Cfg.API.OpenAIKey, Client: opts.HTTPClient}
	default:
		return errors.New("neither Gemini nor OpenAI API key is configured")
	}

	stanzas, tokenUsage, err := llmClient.GenerateStanzas(ctx, theme, pageCount)
	if err != nil {
		return fmt.Errorf("manuscript text generation failed: %w", err)
	}

	if len(stanzas) != pageCount {
		return fmt.Errorf("LLM generated %d stanzas, but target page count is %d", len(stanzas), pageCount)
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

	// Record token usage
	var modelName string
	switch llmClient.(type) {
	case *GeminiClient:
		modelName = "gemini-1.5-flash"
	case *OpenAIClient:
		modelName = "gpt-4o"
	default:
		modelName = "unknown"
	}

	var pricing map[string]telemetry.ModelPricing
	if config.Cfg != nil {
		pricing = config.Cfg.Pricing
	}
	m.RecordLLMUsage(modelName, tokenUsage, pricing)

	if err := m.Save(); err != nil {
		return fmt.Errorf("failed to save manifest after manuscript generation: %w", err)
	}
	return nil
}

//nolint:gocognit,funlen // Illustration loop handles MCP client lifecycle, style registration, and page checkpoint updates
func generateIllustrations(ctx context.Context, m *manifest.Manifest, opts BrewOptions) error {
	var pendingPages []*manifest.PageState
	for i := range m.Progress.Pages {
		page := &m.Progress.Pages[i]
		if page.Status != manifest.StatusCompleted || page.ImagePath == "" {
			pendingPages = append(pendingPages, page)
		}
	}

	if len(pendingPages) == 0 {
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

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		if config.Cfg != nil && config.Cfg.Concurrency > 0 {
			concurrency = config.Cfg.Concurrency
		} else {
			concurrency = 1
		}
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var errsMu sync.Mutex
	var workerErrors []error

Loop:
	for _, page := range pendingPages {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			errsMu.Lock()
			workerErrors = append(workerErrors, ctx.Err())
			errsMu.Unlock()
			break Loop
		}

		wg.Add(1)
		go func(p *manifest.PageState) {
			defer wg.Done()
			defer func() { <-sem }()

			// 1. Update status to generating
			if err := m.UpdatePageStatus(p.PageIndex, manifest.StatusGeneratingImages, ""); err != nil {
				errsMu.Lock()
				workerErrors = append(workerErrors, fmt.Errorf("failed to update page %d status to generating: %w", p.PageIndex, err))
				errsMu.Unlock()
				return
			}

			imgPath, err := generateSingleImage(ctx, mcpClient, p.PageIndex, p.Text, styleID, opts.OutputDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating image for page %d: %v\n", p.PageIndex, err)
				// Revert to pending
				if revertErr := m.UpdatePageStatus(p.PageIndex, manifest.StatusPending, ""); revertErr != nil {
					fmt.Fprintf(os.Stderr, "Error reverting page %d status: %v\n", p.PageIndex, revertErr)
				}
				errsMu.Lock()
				workerErrors = append(workerErrors, err)
				errsMu.Unlock()
				return
			}

			// 3. Record image generation and update status to completed
			var pricing map[string]telemetry.ModelPricing
			if config.Cfg != nil {
				pricing = config.Cfg.Pricing
			}
			m.RecordImageGeneration(pricing)

			if err := m.UpdatePageStatus(p.PageIndex, manifest.StatusCompleted, imgPath); err != nil {
				errsMu.Lock()
				workerErrors = append(workerErrors, fmt.Errorf("failed to update page %d status to completed: %w", p.PageIndex, err))
				errsMu.Unlock()
				return
			}
		}(page)
	}

	wg.Wait()

	if len(workerErrors) > 0 {
		return fmt.Errorf("failed to generate some illustrations: %w", workerErrors[0])
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

func generateSingleImage(ctx context.Context, mcpClient *mcp.PluginClient, pageIndex int, pageText string, styleID string, outputDir string) (string, error) {
	generateArgs := map[string]interface{}{
		"prompt": pageText,
		"size":   "1024x1024",
	}
	if styleID != "" {
		generateArgs["style_id"] = styleID
	}

	resText, err := mcpClient.CallTool(ctx, "imagegen_generate", generateArgs)
	if err != nil {
		return "", fmt.Errorf("failed to generate image for page %d: %w", pageIndex, err)
	}

	prefix := "Successfully generated image and saved to: "
	if !strings.HasPrefix(resText, prefix) {
		return "", fmt.Errorf("unexpected imagegen response format: %q", resText)
	}

	srcPath := strings.TrimPrefix(resText, prefix)
	ext := filepath.Ext(srcPath)
	if ext == "" {
		ext = ".png"
	}

	destFileName := fmt.Sprintf("page_%d%s", pageIndex, ext)
	destPath := filepath.Join(outputDir, "images", destFileName)

	if err := copyFile(srcPath, destPath); err != nil {
		return "", fmt.Errorf("failed to copy generated image for page %d: %w", pageIndex, err)
	}

	return filepath.Join("images", destFileName), nil
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

func exportManuscriptToMarkdown(outputDir string, pages []manifest.PageState) error {
	var sb strings.Builder
	sb.WriteString("<!-- PITHOS MANUSCRIPT REVIEW -->\n")
	sb.WriteString("<!-- Edit the stanzas below. Do not change the '# Page N' headers. -->\n")
	sb.WriteString("<!-- When done, save this file and run 'pithos brew' again to import and continue. -->\n\n")

	for _, p := range pages {
		fmt.Fprintf(&sb, "# Page %d\n", p.PageIndex)
		sb.WriteString(p.Text)
		sb.WriteString("\n\n")
	}

	manuscriptPath := filepath.Join(outputDir, "manuscript.md")
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Errorf("failed to create directory for manuscript export: %w", err)
	}

	if err := os.WriteFile(manuscriptPath, []byte(sb.String()), 0600); err != nil {
		return fmt.Errorf("failed to write manuscript.md: %w", err)
	}
	return nil
}

//nolint:gocognit // import parsing loop requires checking multiple state variables (index, lines, headers)
func importManuscriptFromMarkdown(outputDir string, m *manifest.Manifest) (bool, error) {
	manuscriptPath := filepath.Join(outputDir, "manuscript.md")
	//nolint:gosec // manuscriptPath is constructed in local CLI environment
	data, err := os.ReadFile(manuscriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read manuscript.md: %w", err)
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(content, "\n")

	type parsedPage struct {
		index int
		text  string
	}
	var parsedPages []parsedPage

	var currentIdx int
	var currentLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# Page ") {
			if currentIdx > 0 {
				parsedPages = append(parsedPages, parsedPage{
					index: currentIdx,
					text:  strings.TrimSpace(strings.Join(currentLines, "\n")),
				})
				currentLines = nil
			}

			header := strings.TrimPrefix(trimmed, "# Page ")
			var idx int
			if _, err := fmt.Sscanf(header, "%d", &idx); err != nil {
				return false, fmt.Errorf("failed to parse page header %q: %w", line, err)
			}
			currentIdx = idx
		} else if currentIdx > 0 {
			currentLines = append(currentLines, line)
		}
	}

	if currentIdx > 0 {
		parsedPages = append(parsedPages, parsedPage{
			index: currentIdx,
			text:  strings.TrimSpace(strings.Join(currentLines, "\n")),
		})
	}

	if len(parsedPages) == 0 {
		return false, fmt.Errorf("no stanzas parsed from manuscript.md")
	}

	changed := false
	for _, pp := range parsedPages {
		found := false
		for i, page := range m.Progress.Pages {
			if page.PageIndex == pp.index {
				found = true
				if page.Text != pp.text {
					m.Progress.Pages[i].Text = pp.text
					m.Progress.Pages[i].ImagePath = ""
					m.Progress.Pages[i].Status = manifest.StatusPending
					changed = true
				}
				break
			}
		}
		if !found {
			return false, fmt.Errorf("parsed page index %d does not exist in manifest", pp.index)
		}
	}

	if changed {
		if err := m.Save(); err != nil {
			return false, fmt.Errorf("failed to save manifest after importing edits: %w", err)
		}
	}

	return changed, nil
}
