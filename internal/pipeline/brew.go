package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/logger"
	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/pithos/internal/mcp"
	"github.com/borch-ai/pithos/internal/registry"
	"github.com/borch-ai/pithos/internal/ui"
	"github.com/borch-ai/powerword/pkg/telemetry"
	"github.com/charmbracelet/huh"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ErrReviewPause is returned when the pipeline pauses for local manuscript review.
var ErrReviewPause = errors.New("review mode active")

// BrewOptions holds configuration parameters for the brew command.
type BrewOptions struct {
	OutputDir         string
	Theme             string
	Style             string
	Concurrency       int
	Review            bool
	Pages             []int
	Select            bool
	Silent            bool
	MCPTransport      mcpsdk.Transport // For testing
	CloudMCPTransport mcpsdk.Transport // For testing
	LLM               LLMClient        // For testing
	HTTPClient        *http.Client     // For testing
	In                io.Reader        // For testing
	Out               io.Writer        // For testing
	DryRun            bool
	Budget            float64
}

// Brew executes the manuscript generation and page-by-page illustration generation.
//
//nolint:gocognit,funlen // Brew function integrates manifest loading, overrides, manuscript generation, review loop, and illustration generation
func Brew(ctx context.Context, opts BrewOptions) error {
	if opts.OutputDir == "" {
		return errors.New("output directory is required")
	}
	opts.OutputDir = resolveBookPath(opts.OutputDir)

	if len(opts.Pages) > 0 && opts.Select {
		return errors.New("cannot specify both --pages and --select; please use one or the other")
	}

	manifestPath := filepath.Join(opts.OutputDir, "manifest.json")
	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest from %s: %w", manifestPath, err)
	}

	// Register book workspace path globally
	if regErr := registry.Add(opts.OutputDir); regErr != nil {
		logger.Warn("Failed to register book workspace path in global registry during brew", "error", regErr)
	}

	if opts.Select {
		selectedPages, err := handleSelectPages(m, opts)
		if err != nil {
			return err
		}
		if len(selectedPages) == 0 {
			return nil
		}
		opts.Pages = selectedPages
	}

	// Validate and apply target pages redo/reset
	if len(opts.Pages) > 0 {
		if err := resetManifestPages(m, opts.Pages); err != nil {
			return err
		}
		if err := m.Save(); err != nil {
			return fmt.Errorf("failed to save manifest after resetting target pages: %w", err)
		}
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

	// Import edits from manuscript.md if it exists and manuscript is generated
	manuscriptPath := filepath.Join(opts.OutputDir, "manuscript.md")
	var hasManuscript bool
	if _, statErr := os.Stat(manuscriptPath); statErr == nil {
		hasManuscript = true
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("failed to check manuscript.md status: %w", statErr)
	}
	//nolint:nestif // Import block check nested conditions are clean but exceed nestif threshold
	if m.Progress.ManuscriptGenerated && hasManuscript {
		changed, importErr := importManuscriptFromMarkdown(opts.OutputDir, m, opts.Pages)
		if importErr != nil {
			return importErr
		}
		if changed {
			logger.Info("Successfully imported edits from manuscript.md")
			if err := Checkpoint(ctx, opts.OutputDir, "Imported manuscript edits from review"); err != nil {
				return err
			}
		}
	}

	// Verify budget constraints
	if err := checkBudget(m, &opts); err != nil {
		return err
	}

	// 1. Generate manuscript text if not yet generated
	if err := generateManuscript(ctx, m, opts); err != nil {
		return err
	}

	if err := handleReviewCheckpoint(ctx, opts, m, manuscriptPath); err != nil {
		return err
	}

	// 2. Generate illustrations via MCP ImageGen server
	if err := generateIllustrations(ctx, m, opts); err != nil {
		return err
	}

	if err := m.AddMilestone("brew_complete"); err != nil {
		return fmt.Errorf("failed to record brew_complete milestone: %w", err)
	}

	if err := Checkpoint(ctx, opts.OutputDir, "Completed illustration generation"); err != nil {
		return err
	}

	if previewErr := GenerateWebPreview(opts.OutputDir, m); previewErr != nil {
		logger.Warn("Failed to generate web preview", "error", previewErr)
	} else {
		previewPath := filepath.Join(opts.OutputDir, "web_preview", "preview.html")
		urlStr := formatFileURL(previewPath)
		logger.Info("Web preview generated", "url", urlStr)
		if !opts.Silent {
			triggerBrowserOpen(ctx, urlStr)
		}
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

	if !isTTY() {
		fmt.Println("----------------------------------------")
		fmt.Print(tracker.FormatSummary(pricing))
		if m.Telemetry.ImageGenerations > 0 {
			fmt.Printf("- Image Generations: %d\n", m.Telemetry.ImageGenerations)
		}
		fmt.Printf("- Pipeline Total Cost: $%.5f\n", m.Telemetry.TotalCostUSD)
		fmt.Println("----------------------------------------")
		return nil
	}

	titleStyle := ui.HeaderStyle.Padding(0, 1)
	boxStyle := ui.BoxStyle

	var sb strings.Builder
	summaryText := tracker.FormatSummary(pricing)
	if summaryText != "" {
		sb.WriteString(strings.TrimSpace(summaryText))
		sb.WriteString("\n")
	}
	if m.Telemetry.ImageGenerations > 0 {
		fmt.Fprintf(&sb, "- Image Generations: %d\n", m.Telemetry.ImageGenerations)
	}

	costStyle := ui.HighlightStyle
	fmt.Fprintf(&sb, "- Pipeline Total Cost: %s", costStyle.Render(fmt.Sprintf("$%.5f", m.Telemetry.TotalCostUSD)))

	titleStr := titleStyle.Render("TELEMETRY SUMMARY")
	card := boxStyle.Render(titleStr + "\n\n" + sb.String())
	fmt.Println(card)

	return nil
}

func handleReviewCheckpoint(ctx context.Context, opts BrewOptions, m *manifest.Manifest, manuscriptPath string) error {
	if !opts.Review {
		return nil
	}
	_, statErr := os.Stat(manuscriptPath)
	if statErr != nil && !os.IsNotExist(statErr) {
		return fmt.Errorf("failed to check manuscript.md status for review: %w", statErr)
	}
	if os.IsNotExist(statErr) {
		if exportErr := exportManuscriptToMarkdown(opts.OutputDir, m.BookProperties.Style, m.BookProperties.CharacterProfile, m.Progress.Pages); exportErr != nil {
			return exportErr
		}
	}

	if previewErr := GenerateWebPreview(opts.OutputDir, m); previewErr != nil {
		logger.Warn("Failed to generate web preview", "error", previewErr)
	} else {
		previewPath := filepath.Join(opts.OutputDir, "web_preview", "preview.html")
		urlStr := formatFileURL(previewPath)
		logger.Info("Web preview generated", "url", urlStr)
		if !opts.Silent {
			triggerBrowserOpen(ctx, urlStr)
		}
	}

	return fmt.Errorf("%w: manuscript is available at %s. Edit the file, then run brew without --review to generate illustrations", ErrReviewPause, manuscriptPath)
}

func getLLMClient(llmOverride LLMClient, httpClient *http.Client) (LLMClient, error) {
	switch {
	case llmOverride != nil:
		return llmOverride, nil
	case config.Cfg != nil && config.Cfg.API.GeminiKey != "":
		pwClient, err := newLLMClientFunc("gemini-2.5-flash", config.Cfg.API.GeminiKey, "", httpClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create gemini client: %w", err)
		}
		return &PowerwordClientAdapter{client: pwClient, modelName: "gemini-2.5-flash"}, nil
	case config.Cfg != nil && config.Cfg.API.OpenAIKey != "":
		pwClient, err := newLLMClientFunc("gpt-4o", "", config.Cfg.API.OpenAIKey, httpClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create openai client: %w", err)
		}
		return &PowerwordClientAdapter{client: pwClient, modelName: "gpt-4o"}, nil
	default:
		return nil, errors.New("neither Gemini nor OpenAI API key is configured")
	}
}

func setupLLMClient(opts BrewOptions) (LLMClient, error) {
	return getLLMClient(opts.LLM, opts.HTTPClient)
}

func generateAndRecordVisualGuides(ctx context.Context, m *manifest.Manifest, theme string, llmClient LLMClient) error {
	styleVal, charProfileVal, guideUsage, err := llmClient.GenerateVisualGuides(ctx, theme)
	if err != nil {
		return fmt.Errorf("failed to generate visual style/character guides: %w", err)
	}

	var modelName string
	switch client := llmClient.(type) {
	case *PowerwordClientAdapter:
		modelName = client.modelName
	default:
		modelName = "unknown"
	}
	var pricing map[string]telemetry.ModelPricing
	if config.Cfg != nil {
		pricing = config.Cfg.Pricing
	}
	m.RecordLLMUsage(modelName, guideUsage, pricing)

	if m.BookProperties.Style == "" {
		m.BookProperties.Style = styleVal
	}
	if m.BookProperties.CharacterProfile == "" {
		m.BookProperties.CharacterProfile = charProfileVal
	}
	return nil
}

//nolint:funlen,gocognit,nestif // Manuscript generation pipeline step handles LLM client setup, text generation, and recording telemetry
func generateManuscript(ctx context.Context, m *manifest.Manifest, opts BrewOptions) error {
	if m.Progress.ManuscriptGenerated {
		return nil
	}

	if len(opts.Pages) > 0 {
		return errors.New("cannot perform selective page redo before manuscript is generated")
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

	var stanzas []string
	var prompts []string
	var tokenUsage telemetry.TokenUsage
	var modelName string

	if opts.DryRun {
		if m.BookProperties.Style == "" {
			m.BookProperties.Style = "Simulated style description"
		}
		if m.BookProperties.CharacterProfile == "" {
			m.BookProperties.CharacterProfile = "Simulated character profile"
		}
		stanzas = make([]string, pageCount)
		prompts = make([]string, pageCount)
		for i := 0; i < pageCount; i++ {
			stanzas[i] = fmt.Sprintf("This is page %d simulated stanza.", i+1)
			prompts[i] = fmt.Sprintf("Illustration prompt for page %d", i+1)
		}
		tokenUsage = telemetry.TokenUsage{
			InputTokens:  100,
			OutputTokens: 200,
		}
		modelName = "simulated-model"
	} else {
		llmClient, err := setupLLMClient(opts)
		if err != nil {
			return err
		}

		if m.BookProperties.Style == "" || m.BookProperties.CharacterProfile == "" {
			if err = generateAndRecordVisualGuides(ctx, m, theme, llmClient); err != nil {
				return err
			}
		}

		var genErr error
		stanzas, prompts, tokenUsage, genErr = llmClient.GenerateStanzas(ctx, theme, pageCount, m.BookProperties.Style, m.BookProperties.CharacterProfile)
		if genErr != nil {
			return fmt.Errorf("manuscript text generation failed: %w", genErr)
		}

		switch client := llmClient.(type) {
		case *PowerwordClientAdapter:
			modelName = client.modelName
		default:
			modelName = "unknown"
		}
	}

	if len(stanzas) != pageCount {
		return fmt.Errorf("LLM generated %d stanzas, but target page count is %d", len(stanzas), pageCount)
	}

	m.Progress.Pages = make([]manifest.PageState, len(stanzas))
	for i, text := range stanzas {
		promptVal := ""
		if i < len(prompts) {
			promptVal = prompts[i]
		}
		m.Progress.Pages[i] = manifest.PageState{
			PageIndex:          i + 1,
			Status:             manifest.StatusPending,
			Text:               text,
			IllustrationPrompt: promptVal,
			Layout:             "full-bleed",
		}
	}
	m.Progress.ManuscriptGenerated = true

	var pricing map[string]telemetry.ModelPricing
	if config.Cfg != nil {
		pricing = config.Cfg.Pricing
	}
	m.RecordLLMUsage(modelName, tokenUsage, pricing)

	if err := m.Save(); err != nil {
		return fmt.Errorf("failed to save manifest after manuscript generation: %w", err)
	}
	if err := Checkpoint(ctx, opts.OutputDir, "Generated stanzas and prompts"); err != nil {
		return err
	}
	return nil
}

//nolint:gocognit,funlen,nestif // Illustration loop handles MCP client lifecycle, style registration, and page checkpoint updates
func generateIllustrations(ctx context.Context, m *manifest.Manifest, opts BrewOptions) error {
	var allowedPages map[int]bool
	if len(opts.Pages) > 0 {
		allowedPages = make(map[int]bool, len(opts.Pages))
		for _, pIdx := range opts.Pages {
			allowedPages[pIdx] = true
		}
	}

	var pendingPages []*manifest.PageState
	for i := range m.Progress.Pages {
		page := &m.Progress.Pages[i]
		if allowedPages != nil && !allowedPages[page.PageIndex] {
			continue
		}
		if page.Status != manifest.StatusCompleted || page.ImagePath == "" {
			pendingPages = append(pendingPages, page)
		}
	}

	if len(pendingPages) == 0 {
		return nil
	}

	if opts.DryRun {
		if m.BookProperties.CharacterProfile != "" && m.BookProperties.CharacterReferenceURL == "" {
			charSeedDest := filepath.Join(opts.OutputDir, "images", "character_seed.png")
			if err := writeDummyPNG(charSeedDest); err != nil {
				return fmt.Errorf("failed to write simulated character seed PNG: %w", err)
			}
			m.BookProperties.CharacterReferenceURL = "http://storage.googleapis.com/simulated-bucket/character_seed.png"
			if err := m.Save(); err != nil {
				return fmt.Errorf("failed to save manifest after setting character reference URL: %w", err)
			}
		}

		for _, page := range pendingPages {
			actualCost := m.GetTotalCost()
			budget := 5.00
			if opts.Budget > 0 {
				budget = opts.Budget
			} else if config.Cfg != nil {
				if cfgMax := config.Cfg.GetMaxCostUSD(); cfgMax > 0 {
					budget = cfgMax
				}
			}
			if actualCost > budget {
				return fmt.Errorf("budget exceeded during execution: actual cost $%.4f exceeds budget limit $%.4f", actualCost, budget)
			}

			if err := m.UpdatePageStatus(page.PageIndex, manifest.StatusGeneratingImages, "", ""); err != nil {
				return fmt.Errorf("failed to update page %d status to generating: %w", page.PageIndex, err)
			}
			imgName := fmt.Sprintf("page_%d.png", page.PageIndex)
			destPath := filepath.Join(opts.OutputDir, "images", imgName)
			if err := writeDummyPNG(destPath); err != nil {
				return fmt.Errorf("failed to write simulated page PNG: %w", err)
			}

			var pricing map[string]telemetry.ModelPricing
			if config.Cfg != nil {
				pricing = config.Cfg.Pricing
			}
			m.RecordImageGeneration(pricing)

			relPath := filepath.Join("images", imgName)
			if err := m.UpdatePageStatus(page.PageIndex, manifest.StatusCompleted, relPath, "simulated-model"); err != nil {
				return fmt.Errorf("failed to update page %d status to completed: %w", page.PageIndex, err)
			}
		}
		return nil
	}

	charBackend := "imagen"
	illusBackend := "imagen"
	if config.Cfg != nil {
		if config.Cfg.MCP.CharacterBackend != "" {
			charBackend = config.Cfg.MCP.CharacterBackend
		}
		if config.Cfg.MCP.IllustrationBackend != "" {
			illusBackend = config.Cfg.MCP.IllustrationBackend
		}
	}

	var mcpClient *mcp.PluginClient
	var styleID string

	if charBackend == illusBackend {
		// Start a single imagegen client for both character reference and page illustrations
		if illusBackend != "" {
			_ = os.Setenv("POWERWORD_IMAGEGEN_BACKEND", illusBackend)
		}
		mcpClient = mcp.NewPluginClient(mcp.PluginImageGen)
		if opts.MCPTransport != nil {
			mcpClient.SetTransport(opts.MCPTransport)
		}

		if err := mcpClient.Start(ctx); err != nil {
			return fmt.Errorf("failed to start MCP imagegen client: %w", err)
		}
		defer func() { _ = mcpClient.Stop() }()

		if err := checkBackendCapabilities(ctx, mcpClient, m.BookProperties.CharacterProfile); err != nil {
			return err
		}

		if m.BookProperties.Style != "" {
			var err error
			styleID, err = registerStyleProfile(ctx, mcpClient, m.BookProperties.Style)
			if err != nil {
				return err
			}
		}

		// Bootstrap character reference using the same client
		if err := bootstrapCharacterReferenceWithClient(ctx, m, opts.OutputDir, mcpClient, opts.CloudMCPTransport, charBackend); err != nil {
			return err
		}
	} else {
		// Bootstrap Character Seed Portrait (starts and stops its own client with charBackend)
		if err := BootstrapCharacterReference(ctx, m, opts.OutputDir, opts.MCPTransport, opts.CloudMCPTransport, charBackend, opts.DryRun); err != nil {
			return err
		}

		// Start the illustration backend MCP Client
		if illusBackend != "" {
			_ = os.Setenv("POWERWORD_IMAGEGEN_BACKEND", illusBackend)
		}
		mcpClient = mcp.NewPluginClient(mcp.PluginImageGen)
		if opts.MCPTransport != nil {
			mcpClient.SetTransport(opts.MCPTransport)
		}

		if err := mcpClient.Start(ctx); err != nil {
			return fmt.Errorf("failed to start MCP imagegen client: %w", err)
		}
		defer func() { _ = mcpClient.Stop() }()

		if err := checkBackendCapabilities(ctx, mcpClient, m.BookProperties.CharacterProfile); err != nil {
			return err
		}

		if m.BookProperties.Style != "" {
			var err error
			styleID, err = registerStyleProfile(ctx, mcpClient, m.BookProperties.Style)
			if err != nil {
				return err
			}
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

	brewCtx, cancel := context.WithCancel(ctx)
	defer cancel()

Loop:
	for _, page := range pendingPages {
		// Compute per-image cost for look-ahead budget check
		var pricing map[string]telemetry.ModelPricing
		if config.Cfg != nil {
			pricing = config.Cfg.Pricing
		}
		imageCost := 0.04
		if pricing != nil {
			if p, ok := pricing["imagegen"]; ok {
				imageCost = p.Input / 1_000_000.0
			}
		}

		// Check budget limit dynamically before dispatching each worker.
		// Use a look-ahead (actualCost + imageCost) so concurrent goroutines
		// can't collectively overspend by more than one image's worth.
		actualCost := m.GetTotalCost()
		budget := 5.00
		if opts.Budget > 0 {
			budget = opts.Budget
		} else if config.Cfg != nil {
			if cfgMax := config.Cfg.GetMaxCostUSD(); cfgMax > 0 {
				budget = cfgMax
			}
		}
		if actualCost+imageCost > budget {
			errsMu.Lock()
			workerErrors = append(workerErrors, fmt.Errorf("budget exceeded during execution: actual cost $%.4f exceeds budget limit $%.4f", actualCost, budget))
			errsMu.Unlock()
			cancel()
			break Loop
		}

		select {
		case sem <- struct{}{}:
		case <-brewCtx.Done():
			errsMu.Lock()
			workerErrors = append(workerErrors, brewCtx.Err())
			errsMu.Unlock()
			break Loop
		}

		wg.Add(1)
		go func(p *manifest.PageState) {
			defer wg.Done()
			defer func() { <-sem }()

			// Check live budget limit before doing work
			actualCost := m.GetTotalCost()
			budget := 5.00
			if opts.Budget > 0 {
				budget = opts.Budget
			} else if config.Cfg != nil {
				if cfgMax := config.Cfg.GetMaxCostUSD(); cfgMax > 0 {
					budget = cfgMax
				}
			}
			if actualCost > budget {
				errsMu.Lock()
				workerErrors = append(workerErrors, fmt.Errorf("budget exceeded: actual cost $%.4f exceeds budget limit $%.4f", actualCost, budget))
				errsMu.Unlock()
				// Revert to pending
				_ = m.UpdatePageStatus(p.PageIndex, manifest.StatusPending, "", "")
				cancel()
				return
			}

			// 1. Update status to generating
			if err := m.UpdatePageStatus(p.PageIndex, manifest.StatusGeneratingImages, "", ""); err != nil {
				errsMu.Lock()
				workerErrors = append(workerErrors, fmt.Errorf("failed to update page %d status to generating: %w", p.PageIndex, err))
				errsMu.Unlock()
				return
			}

			prompt := p.IllustrationPrompt
			if prompt == "" {
				prompt = p.Text
			}
			if m.BookProperties.CharacterProfile != "" {
				prompt = m.BookProperties.CharacterProfile + ", " + prompt
			}
			var charWeight *int
			if p.CharacterWeight != nil {
				charWeight = p.CharacterWeight
			} else {
				w := m.BookProperties.CharacterWeight
				if w == 0 {
					w = 100
				}
				charWeight = &w
			}
			imageSize := getBestImageSize(m.BookProperties.TrimSize)
			imgPath, modelName, err := generateSingleImage(brewCtx, mcpClient, p.PageIndex, prompt, styleID, opts.OutputDir, imageSize, m.BookProperties.CharacterReferenceURL, charWeight)
			if err != nil {
				logger.Error("Error generating image", "page", p.PageIndex, "error", err)
				// Revert to pending
				if revertErr := m.UpdatePageStatus(p.PageIndex, manifest.StatusPending, "", ""); revertErr != nil {
					logger.Error("Error reverting page status", "page", p.PageIndex, "error", revertErr)
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

			if err := m.UpdatePageStatus(p.PageIndex, manifest.StatusCompleted, imgPath, modelName); err != nil {
				errsMu.Lock()
				workerErrors = append(workerErrors, fmt.Errorf("failed to update page %d status to completed: %w", p.PageIndex, err))
				errsMu.Unlock()
				return
			}
			logger.Info("Generated illustration", "page", p.PageIndex, "model", modelName)
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

func parseTOMLKeyNotEmpty(tomlContent string, key string) bool {
	lines := strings.Split(tomlContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, key) {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		if k != key {
			continue
		}
		v := strings.TrimSpace(parts[1])
		v = strings.Trim(v, `"'`)
		if v != "" && v != "YOUR_GEMINI_API_KEY" && v != "YOUR_OPENAI_API_KEY" {
			return true
		}
	}
	return false
}

func checkEnvCredentials(backend string) bool {
	switch backend {
	case "google":
		return os.Getenv("POWERWORD_GEMINI_API_KEY") != "" ||
			os.Getenv("GEMINI_API_KEY") != "" ||
			os.Getenv("GOOGLE_API_KEY") != "" ||
			os.Getenv("POWERWORD_API_KEYS_GEMINI") != "" ||
			os.Getenv("POWERWORD_PLUGINS_IMAGEGEN_GOOGLE_API_KEY") != ""
	case "openai":
		return os.Getenv("POWERWORD_OPENAI_API_KEY") != "" ||
			os.Getenv("OPENAI_API_KEY") != "" ||
			os.Getenv("POWERWORD_API_KEYS_OPENAI") != "" ||
			os.Getenv("POWERWORD_PLUGINS_IMAGEGEN_OPENAI_API_KEY") != ""
	default:
		return false
	}
}

func checkConfigCredentials(backend string) bool {
	if config.Cfg == nil {
		return false
	}
	switch backend {
	case "google":
		return config.Cfg.API.GeminiKey != ""
	case "openai":
		return config.Cfg.API.OpenAIKey != ""
	default:
		return false
	}
}

func checkConfigFileCredential(backend string, path string) bool {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return false
	}
	//nolint:gosec // local config file read
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	switch backend {
	case "google":
		return parseTOMLKeyNotEmpty(content, "gemini") ||
			parseTOMLKeyNotEmpty(content, "google") ||
			parseTOMLKeyNotEmpty(content, "google_api_key")
	case "openai":
		return parseTOMLKeyNotEmpty(content, "openai") ||
			parseTOMLKeyNotEmpty(content, "openai_api_key")
	default:
		return false
	}
}

func searchParentDirsForCredential(backend string, startDir string) bool {
	curr := startDir
	for {
		for _, name := range []string{"powerword.toml", ".powerword.toml"} {
			path := filepath.Join(curr, name)
			if checkConfigFileCredential(backend, path) {
				return true
			}
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return false
}

func checkFileCredentials(backend string, outputDir string) bool {
	dirsToSearch := []string{outputDir}
	if cwd, err := os.Getwd(); err == nil {
		dirsToSearch = append(dirsToSearch, cwd)
	}

	for _, startDir := range dirsToSearch {
		if startDir == "" {
			continue
		}
		if searchParentDirsForCredential(backend, startDir) {
			return true
		}
	}
	return false
}

func hasPowerwordCredential(backend string, outputDir string) bool {
	return checkEnvCredentials(backend) ||
		checkConfigCredentials(backend) ||
		checkFileCredentials(backend, outputDir)
}

func handleImagegenFallback(ctx context.Context, mcpClient *mcp.PluginClient, primaryErr error, generateArgs map[string]interface{}, outputDir string) (string, string, error) {
	errMsg := strings.ToLower(primaryErr.Error())
	isAuthOrEligibleError := strings.Contains(errMsg, "401") ||
		strings.Contains(errMsg, "403") ||
		strings.Contains(errMsg, "unauthorized") ||
		strings.Contains(errMsg, "forbidden") ||
		strings.Contains(errMsg, "permission denied") ||
		strings.Contains(errMsg, "permissiondenied") ||
		strings.Contains(errMsg, "api key") ||
		strings.Contains(errMsg, "model does not exist") ||
		strings.Contains(errMsg, "not configured") ||
		strings.Contains(errMsg, "model not found")

	if !isAuthOrEligibleError {
		return "", "", primaryErr
	}

	capJSON, capErr := mcpClient.CallTool(ctx, "imagegen_get_capabilities", map[string]interface{}{})
	if capErr != nil {
		return "", "", primaryErr
	}

	var caps imagegenCapabilities
	if err := json.Unmarshal([]byte(capJSON), &caps); err != nil || caps.Backend == "" {
		return "", "", primaryErr
	}

	activeBackend := strings.ToLower(caps.Backend)
	var fallbackBackend string

	switch activeBackend {
	case "openai", "midjourney":
		fallbackBackend = "google"
	case "google", "imagen", "veo":
		fallbackBackend = "openai"
	}

	if fallbackBackend == "" || !hasPowerwordCredential(fallbackBackend, outputDir) {
		return "", "", primaryErr
	}

	logger.Warn("Primary imagegen backend failed with credential/model error. Attempting fallback...",
		"primary", activeBackend,
		"fallback", fallbackBackend,
		"error", primaryErr)

	fallbackClient := mcp.NewPluginClient(mcp.PluginImageGen)
	if t := mcpClient.GetTransport(); t != nil {
		fallbackClient.SetTransport(t)
	}

	fallbackClient.SetEnv([]string{
		fmt.Sprintf("POWERWORD_IMAGEGEN_BACKEND=%s", fallbackBackend),
	})

	if errStart := fallbackClient.Start(ctx); errStart != nil {
		logger.Error("Failed to start fallback client", "error", errStart)
		return "", "", primaryErr
	}
	defer func() { _ = fallbackClient.Stop() }()

	resTextFallback, errFallback := fallbackClient.CallTool(ctx, "imagegen_generate", generateArgs)
	if errFallback != nil {
		return "", "", errFallback
	}

	var jsonRes struct {
		ImagePath string `json:"image_path"`
		Model     string `json:"model"`
	}
	if errJson := json.Unmarshal([]byte(resTextFallback), &jsonRes); errJson == nil && jsonRes.ImagePath != "" {
		if jsonRes.Model == "" {
			jsonRes.Model = fallbackBackend
		}
		return jsonRes.ImagePath, jsonRes.Model, nil
	}

	prefix := "Successfully generated image and saved to: "
	if strings.HasPrefix(resTextFallback, prefix) {
		return strings.TrimPrefix(resTextFallback, prefix), fallbackBackend, nil
	}
	return "", "", fmt.Errorf("unexpected imagegen response format: %q", resTextFallback)
}

func generateImageRaw(ctx context.Context, mcpClient *mcp.PluginClient, prompt string, styleID string, imageSize string, crefURL string, charWeight *int, outputDir string) (string, string, error) {
	if imageSize == "" {
		imageSize = "1024x1024"
	}
	generateArgs := map[string]interface{}{
		"prompt": prompt,
		"size":   imageSize,
	}
	if styleID != "" {
		generateArgs["style_id"] = styleID
	}
	if crefURL != "" {
		generateArgs["cref_url"] = crefURL
		if charWeight != nil {
			generateArgs["character_weight"] = *charWeight
		}
	}

	resText, err := mcpClient.CallTool(ctx, "imagegen_generate", generateArgs)
	if err != nil {
		return handleImagegenFallback(ctx, mcpClient, err, generateArgs, outputDir)
	}

	var jsonRes struct {
		ImagePath string `json:"image_path"`
		Model     string `json:"model"`
	}
	if err := json.Unmarshal([]byte(resText), &jsonRes); err == nil && jsonRes.ImagePath != "" {
		if jsonRes.Model == "" {
			jsonRes.Model = "unknown"
		}
		return jsonRes.ImagePath, jsonRes.Model, nil
	}

	prefix := "Successfully generated image and saved to: "
	if !strings.HasPrefix(resText, prefix) {
		return "", "", fmt.Errorf("unexpected imagegen response format: %q", resText)
	}

	return strings.TrimPrefix(resText, prefix), "unknown", nil
}

func generateSingleImage(ctx context.Context, mcpClient *mcp.PluginClient, pageIndex int, pageText string, styleID string, outputDir string, imageSize string, crefURL string, charWeight *int) (string, string, error) {
	srcPath, modelName, err := generateImageRaw(ctx, mcpClient, pageText, styleID, imageSize, crefURL, charWeight, outputDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate image for page %d: %w", pageIndex, err)
	}

	ext := filepath.Ext(srcPath)
	if ext == "" {
		ext = ".png"
	}

	destFileName := fmt.Sprintf("page_%d%s", pageIndex, ext)
	destPath := filepath.Join(outputDir, "images", destFileName)

	if err := copyFile(srcPath, destPath); err != nil {
		return "", "", fmt.Errorf("failed to copy generated image for page %d: %w", pageIndex, err)
	}

	return filepath.Join("images", destFileName), modelName, nil
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

func sanitizeCommentText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\n", " ")
	return strings.ReplaceAll(text, "-->", "--")
}

func exportManuscriptToMarkdown(outputDir string, style, charProfile string, pages []manifest.PageState) error {
	var sb strings.Builder
	sb.WriteString("<!-- PITHOS MANUSCRIPT REVIEW -->\n")
	fmt.Fprintf(&sb, "<!-- Style: %s -->\n", sanitizeCommentText(style))
	fmt.Fprintf(&sb, "<!-- CharacterProfile: %s -->\n", sanitizeCommentText(charProfile))
	sb.WriteString("<!-- Edit the stanzas, prompts, and global style/character guides above. -->\n")
	sb.WriteString("<!-- Each page block begins with a '# Page N' header, followed by two subsections: -->\n")
	sb.WriteString("<!-- '## Text' — the stanza/prose to edit, and '## Prompt' — the illustration prompt to edit. -->\n")
	sb.WriteString("<!-- Do not change any '# Page N' or '## Text'/'## Prompt' headers themselves. -->\n")
	sb.WriteString("<!-- When done, save this file and run 'pithos brew' again to import and continue. -->\n\n")

	for _, p := range pages {
		fmt.Fprintf(&sb, "# Page %d\n", p.PageIndex)
		layoutVal := p.Layout
		if layoutVal == "" {
			layoutVal = "full-bleed"
		}
		fmt.Fprintf(&sb, "<!-- Layout: %s -->\n", sanitizeCommentText(layoutVal))
		sb.WriteString("## Text\n")
		sb.WriteString(strings.TrimSpace(p.Text))
		sb.WriteString("\n\n## Prompt\n")
		sb.WriteString(strings.TrimSpace(p.IllustrationPrompt))
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

type parsedPage struct {
	index         int
	text          string
	prompt        string
	layout        string
	hasSubheaders bool
}

func extractLayoutComment(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "<!--") && strings.HasSuffix(trimmed, "-->") {
		commentContent := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "<!--"), "-->"))
		if strings.HasPrefix(strings.ToLower(commentContent), "layout:") {
			return strings.TrimSpace(commentContent[len("layout:"):]), true
		}
	}
	return "", false
}

func parsePageBlock(index int, lines []string) (parsedPage, bool, error) {
	var layoutVal string
	var filteredLines []string
	textCount := 0
	promptCount := 0

	for _, line := range lines {
		if val, ok := extractLayoutComment(line); ok {
			layoutVal = val
			continue
		}
		filteredLines = append(filteredLines, line)
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "## Text":
			textCount++
		case "## Prompt":
			promptCount++
		}
	}

	hasSubheaders := textCount > 0 || promptCount > 0

	if !hasSubheaders {
		// Legacy format: everything under "# Page N" is the text
		return parsedPage{
			index:         index,
			text:          strings.TrimSpace(strings.Join(filteredLines, "\n")),
			layout:        layoutVal,
			hasSubheaders: false,
		}, false, nil
	}

	// New format detected: validate that both headers are present exactly once to
	// prevent silent data loss if the user accidentally deletes or duplicates a header.
	if textCount != 1 || promptCount != 1 {
		return parsedPage{}, false, fmt.Errorf(
			"page %d: new-format page block must contain exactly one '## Text' and one '## Prompt' header, got %d and %d",
			index, textCount, promptCount,
		)
	}

	// New format: parse sections
	var textLines []string
	var promptLines []string
	currentSection := 0 // 0 = none, 1 = text, 2 = prompt

	for _, line := range filteredLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "## Text" {
			currentSection = 1
			continue
		}
		if trimmed == "## Prompt" {
			currentSection = 2
			continue
		}

		switch currentSection {
		case 1:
			textLines = append(textLines, line)
		case 2:
			promptLines = append(promptLines, line)
		}
	}

	return parsedPage{
		index:         index,
		text:          strings.TrimSpace(strings.Join(textLines, "\n")),
		prompt:        strings.TrimSpace(strings.Join(promptLines, "\n")),
		layout:        layoutVal,
		hasSubheaders: true,
	}, true, nil
}

//nolint:gocognit // parsing manuscript lines requires multi-pass state machine for blocks and subheaders
func parseManuscriptLines(lines []string) (pages []parsedPage, style string, styleOk bool, charProfile string, charProfileOk bool, hasSubheaders bool, err error) {
	var parsedPages []parsedPage
	var currentIdx int
	var currentLines []string
	hasSubheadersAny := false
	var parsedStyle string
	var parsedStyleOk bool
	var parsedCharProfile string
	var parsedCharProfileOk bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if len(parsedPages) == 0 && currentIdx == 0 && strings.HasPrefix(trimmed, "<!--") && strings.HasSuffix(trimmed, "-->") {
			commentContent := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "<!--"), "-->"))
			if strings.HasPrefix(commentContent, "Style:") {
				parsedStyle = strings.TrimSpace(strings.TrimPrefix(commentContent, "Style:"))
				parsedStyleOk = true
				continue
			}
			if strings.HasPrefix(commentContent, "CharacterProfile:") {
				parsedCharProfile = strings.TrimSpace(strings.TrimPrefix(commentContent, "CharacterProfile:"))
				parsedCharProfileOk = true
				continue
			}
		}

		if !strings.HasPrefix(trimmed, "# Page ") {
			if currentIdx > 0 {
				currentLines = append(currentLines, line)
			}
			continue
		}

		if currentIdx > 0 {
			pPage, hasSubs, err := parsePageBlock(currentIdx, currentLines)
			if err != nil {
				return nil, "", false, "", false, false, err
			}
			if hasSubs {
				hasSubheadersAny = true
			}
			parsedPages = append(parsedPages, pPage)
			currentLines = nil
		}

		header := strings.TrimPrefix(trimmed, "# Page ")
		idx, err := strconv.Atoi(header)
		if err != nil {
			return nil, "", false, "", false, false, fmt.Errorf("failed to parse page header %q: %w", line, err)
		}
		if idx <= 0 {
			return nil, "", false, "", false, false, fmt.Errorf("invalid page index %d in header %q: must be positive", idx, line)
		}
		currentIdx = idx
	}

	if currentIdx > 0 {
		pPage, hasSubs, err := parsePageBlock(currentIdx, currentLines)
		if err != nil {
			return nil, "", false, "", false, false, err
		}
		if hasSubs {
			hasSubheadersAny = true
		}
		parsedPages = append(parsedPages, pPage)
	}

	return parsedPages, parsedStyle, parsedStyleOk, parsedCharProfile, parsedCharProfileOk, hasSubheadersAny, nil
}

//nolint:gocognit // import verification requires matching multiple states (index, text modifications)
func validateManuscriptEdits(parsedPages []parsedPage, m *manifest.Manifest, pagesFilter []int) error {
	seen := make(map[int]bool)
	for _, pp := range parsedPages {
		if seen[pp.index] {
			return fmt.Errorf("duplicate page index %d in manuscript.md", pp.index)
		}
		seen[pp.index] = true
	}

	if len(seen) != len(m.Progress.Pages) {
		return fmt.Errorf("manuscript.md contains %d stanzas, but manifest expects %d stanzas", len(seen), len(m.Progress.Pages))
	}

	// First pass: validate edits against the pagesFilter to fail fast before mutating
	for _, pp := range parsedPages {
		found := false
		for _, page := range m.Progress.Pages {
			if page.PageIndex == pp.index {
				found = true
				textChanged := page.Text != pp.text
				promptChanged := pp.hasSubheaders && page.IllustrationPrompt != pp.prompt
				layoutChanged := pp.layout != "" && page.Layout != pp.layout
				if (textChanged || promptChanged || layoutChanged) && !isPageAllowed(pp.index, pagesFilter) {
					return fmt.Errorf("manuscript.md contains edits for page %d which is not included in the selective page override list: %v", pp.index, pagesFilter)
				}
			}
		}
		if !found {
			return fmt.Errorf("parsed page index %d does not exist in manifest", pp.index)
		}
	}
	return nil
}

func updateSinglePage(page *manifest.PageState, pp parsedPage) bool {
	textChanged := page.Text != pp.text
	promptChanged := pp.hasSubheaders && page.IllustrationPrompt != pp.prompt
	layoutChanged := pp.layout != "" && page.Layout != pp.layout

	if !textChanged && !promptChanged && !layoutChanged {
		return false
	}

	if textChanged || promptChanged {
		page.Text = pp.text
		if pp.hasSubheaders {
			page.IllustrationPrompt = pp.prompt
		}
		page.Status = manifest.StatusPending
	}
	if layoutChanged {
		page.Layout = pp.layout
	}
	return true
}

func updateManifestPages(m *manifest.Manifest, parsedPages []parsedPage) bool {
	changed := false
	for _, pp := range parsedPages {
		for i := range m.Progress.Pages {
			page := &m.Progress.Pages[i]
			if page.PageIndex == pp.index {
				if updateSinglePage(page, pp) {
					changed = true
				}
				break
			}
		}
	}
	return changed
}

//nolint:gocognit // import verification requires matching multiple states (index, text modifications)
func importManuscriptFromMarkdown(outputDir string, m *manifest.Manifest, pagesFilter []int) (bool, error) {
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

	parsedPages, parsedStyle, styleOk, parsedCharProfile, charProfileOk, _, err := parseManuscriptLines(lines)
	if err != nil {
		return false, err
	}

	if len(parsedPages) == 0 {
		return false, fmt.Errorf("no stanzas parsed from manuscript.md")
	}

	if err := validateManuscriptEdits(parsedPages, m, pagesFilter); err != nil {
		return false, err
	}

	changed := false
	styleChanged := false
	if styleOk && parsedStyle != m.BookProperties.Style {
		m.BookProperties.Style = parsedStyle
		styleChanged = true
		changed = true
	}
	charProfileChanged := false
	if charProfileOk && parsedCharProfile != m.BookProperties.CharacterProfile {
		m.BookProperties.CharacterProfile = parsedCharProfile
		m.BookProperties.CharacterReferenceURL = ""
		charProfileChanged = true
		changed = true
	}

	if updateManifestPages(m, parsedPages) {
		changed = true
	}

	if styleChanged || charProfileChanged {
		for i := range m.Progress.Pages {
			m.Progress.Pages[i].Status = manifest.StatusPending
			m.Progress.Pages[i].ImagePath = ""
		}
	}

	if changed {
		if err := m.Save(); err != nil {
			return false, fmt.Errorf("failed to save manifest after importing edits: %w", err)
		}
	}

	return changed, nil
}

func resetManifestPages(m *manifest.Manifest, pages []int) error {
	if !m.Progress.ManuscriptGenerated {
		return errors.New("cannot perform selective page redo before manuscript is generated")
	}
	for _, pageNum := range pages {
		found := false
		for idx, page := range m.Progress.Pages {
			if page.PageIndex == pageNum {
				m.Progress.Pages[idx].Status = manifest.StatusPending
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("page index %d is out of bounds, book has %d pages", pageNum, len(m.Progress.Pages))
		}
	}
	return nil
}

func isPageAllowed(pageIndex int, pagesFilter []int) bool {
	if len(pagesFilter) == 0 {
		return true
	}
	for _, p := range pagesFilter {
		if p == pageIndex {
			return true
		}
	}
	return false
}

func getBestImageSize(trimSize string) string {
	parts := strings.Split(strings.ToLower(trimSize), "x")
	if len(parts) != 2 {
		return "1024x1024"
	}
	var w, h float64
	if _, err := fmt.Sscanf(parts[0], "%f", &w); err != nil {
		return "1024x1024"
	}
	if _, err := fmt.Sscanf(parts[1], "%f", &h); err != nil {
		return "1024x1024"
	}
	if w <= 0 || h <= 0 {
		return "1024x1024"
	}
	switch {
	case w == h:
		return "1024x1024"
	case w < h:
		return "1024x1792"
	default:
		return "1792x1024"
	}
}

// BootstrapCharacterReference generates character seed portrait using the specified character backend.
func BootstrapCharacterReference(ctx context.Context, m *manifest.Manifest, outputDir string, mcpTransport mcpsdk.Transport, cloudMCPTransport mcpsdk.Transport, backend string, dryRun bool) error {
	if m.BookProperties.CharacterReferenceURL != "" || m.BookProperties.CharacterProfile == "" {
		return nil
	}

	if dryRun {
		charSeedDest := filepath.Join(outputDir, "images", "character_seed.png")
		if err := writeDummyPNG(charSeedDest); err != nil {
			return fmt.Errorf("failed to write simulated character seed PNG: %w", err)
		}
		m.BookProperties.CharacterReferenceURL = "http://storage.googleapis.com/simulated-bucket/character_seed.png"
		return m.Save()
	}

	if backend != "" {
		_ = os.Setenv("POWERWORD_IMAGEGEN_BACKEND", backend)
	}

	mcpClient := mcp.NewPluginClient(mcp.PluginImageGen)
	if mcpTransport != nil {
		mcpClient.SetTransport(mcpTransport)
	}

	if err := mcpClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start MCP imagegen client for character seed: %w", err)
	}
	defer func() { _ = mcpClient.Stop() }()

	return bootstrapCharacterReferenceWithClient(ctx, m, outputDir, mcpClient, cloudMCPTransport, backend)
}

//nolint:gocognit,funlen // character reference bootstrapping involves capability validation, style registration, and GCS upload
func bootstrapCharacterReferenceWithClient(ctx context.Context, m *manifest.Manifest, outputDir string, mcpClient *mcp.PluginClient, cloudMCPTransport mcpsdk.Transport, backend string) error {
	if m.BookProperties.CharacterReferenceURL != "" || m.BookProperties.CharacterProfile == "" {
		return nil
	}

	if err := checkBackendCapabilities(ctx, mcpClient, m.BookProperties.CharacterProfile); err != nil {
		return err
	}

	// Verify that character backend output type is exactly "image"
	capJSON, err := mcpClient.CallTool(ctx, "imagegen_get_capabilities", nil)
	if err != nil {
		return fmt.Errorf("failed to query capabilities for character backend: %w", err)
	}
	var caps imagegenCapabilities
	if unmarshalErr := json.Unmarshal([]byte(capJSON), &caps); unmarshalErr != nil {
		return fmt.Errorf("failed to parse capabilities: %w", unmarshalErr)
	}
	outType := strings.ToLower(strings.TrimSpace(caps.OutputType))
	if outType != "image" {
		return fmt.Errorf("character backend %q has output type %q; character seed portrait must be still image (output_type must be \"image\")", backend, caps.OutputType)
	}

	actualStyleID := ""
	if m.BookProperties.Style != "" {
		var styleErr error
		actualStyleID, styleErr = registerStyleProfile(ctx, mcpClient, m.BookProperties.Style)
		if styleErr != nil {
			return styleErr
		}
	}

	seedPrompt := "Detailed visual seed character portrait: " + m.BookProperties.CharacterProfile
	imageSize := getBestImageSize(m.BookProperties.TrimSize)

	logger.Info("Generating character seed portrait...")
	srcPath, modelName, err := generateImageRaw(ctx, mcpClient, seedPrompt, actualStyleID, imageSize, "", nil, outputDir)
	if err != nil {
		return fmt.Errorf("failed to generate character seed portrait: %w", err)
	}
	logger.Info("Generated character seed", "model", modelName)

	var pricing map[string]telemetry.ModelPricing
	if config.Cfg != nil {
		pricing = config.Cfg.Pricing
	}
	m.RecordImageGeneration(pricing)

	ext := filepath.Ext(srcPath)
	if ext == "" {
		ext = ".png"
	}

	destPath := filepath.Join(outputDir, "images", "character_seed"+ext)
	if copyErr := copyFile(srcPath, destPath); copyErr != nil {
		return fmt.Errorf("failed to copy character seed image: %w", copyErr)
	}

	lowerExt := strings.ToLower(ext)
	if lowerExt == ".mp4" || lowerExt == ".webm" {
		ffmpegPath, lookErr := lookPathFunc("ffmpeg")
		if lookErr != nil {
			return fmt.Errorf("ffmpeg not found in PATH: please install ffmpeg to extract static frames from video seeds for character portraits: %w", lookErr)
		}
		pngPath := filepath.Join(outputDir, "images", "character_seed.png")
		logger.Info("Extracting static frame from video seed", "path", pngPath)
		// #nosec G204
		cmd := execCommandContext(ctx, ffmpegPath, "-y", "-i", destPath, "-vframes", "1", "-f", "image2", pngPath)
		if out, runErr := cmd.CombinedOutput(); runErr != nil {
			return fmt.Errorf("failed to extract static frame from character seed video using ffmpeg: %w (output: %s)", runErr, string(out))
		}
		destPath = pngPath
	}

	// Initialize pw-mcp-cloud client
	cloudClient := mcp.NewPluginClient(mcp.PluginCloud)
	if cloudMCPTransport != nil {
		cloudClient.SetTransport(cloudMCPTransport)
	}

	if startErr := cloudClient.Start(ctx); startErr != nil {
		return fmt.Errorf("failed to start MCP cloud client: %w", startErr)
	}
	defer func() { _ = cloudClient.Stop() }()

	absPath, err := filepath.Abs(destPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path of character seed image: %w", err)
	}

	logger.Info("Uploading character seed portrait to cloud storage", "path", absPath)
	uploadArgs := map[string]interface{}{
		"local_path": absPath,
	}
	publicURL, err := cloudClient.CallTool(ctx, "cloud_upload_file", uploadArgs)
	if err != nil {
		return fmt.Errorf("failed to upload character seed portrait: %w", err)
	}

	m.BookProperties.CharacterReferenceURL = strings.TrimSpace(publicURL)
	if err := m.Save(); err != nil {
		return fmt.Errorf("failed to save manifest after setting character reference URL: %w", err)
	}

	logger.Info("Character seed portrait uploaded successfully", "url", m.BookProperties.CharacterReferenceURL)
	return nil
}

type imagegenCapabilities struct {
	Backend      string `json:"backend"`
	SupportsCref bool   `json:"supports_cref"`
	SupportsSref bool   `json:"supports_sref"`
	OutputType   string `json:"output_type"`
}

func checkBackendCapabilities(ctx context.Context, mcpClient *mcp.PluginClient, characterProfile string) error {
	capJSON, err := mcpClient.CallTool(ctx, "imagegen_get_capabilities", nil)
	if err != nil {
		return fmt.Errorf("failed to query imagegen backend capabilities: %w", err)
	}

	var caps imagegenCapabilities
	if err := json.Unmarshal([]byte(capJSON), &caps); err != nil {
		return fmt.Errorf("failed to parse imagegen backend capabilities JSON: %w", err)
	}

	if config.Cfg != nil && config.Cfg.MCP.ImageGenForceCref {
		caps.SupportsCref = true
	}
	if config.Cfg != nil && config.Cfg.MCP.ImageGenForceSref {
		caps.SupportsSref = true
	}

	if characterProfile != "" && !caps.SupportsCref {
		return fmt.Errorf("active imagegen backend [%s] does not support character references, but a character profile is defined; switch backend to midjourney or clean manifest character properties", caps.Backend)
	}

	return nil
}

var isTTY = func() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if maxLen < 3 {
		maxLen = 3
	}
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen-3]) + "..."
	}
	return s
}

func promptSelectPages(m *manifest.Manifest, opts BrewOptions) ([]int, error) {
	if !isTTY() {
		return nil, errors.New("interactive selection requires a TTY terminal")
	}
	var selectedPages []int
	var options []huh.Option[int]
	for _, page := range m.Progress.Pages {
		label := fmt.Sprintf("Page %d (%s): %s", page.PageIndex, page.Status, truncate(page.Text, 50))
		options = append(options, huh.NewOption(label, page.PageIndex).Selected(page.Status != manifest.StatusCompleted))
	}
	if len(options) == 0 {
		return nil, errors.New("no pages found in manifest to select")
	}

	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[int]().
				Title("Select Pages to Brew/Regenerate").
				Description("Use Space to toggle, Enter to confirm").
				Options(options...).
				Value(&selectedPages),
		),
	).WithInput(in).WithOutput(out)
	form.WithAccessible(opts.In != nil)

	if err := form.Run(); err != nil {
		return nil, err
	}

	return selectedPages, nil
}

// handleSelectPages prompts the user to select pages, handles empty selections gracefully,
// and returns the chosen page indices.
func handleSelectPages(m *manifest.Manifest, opts BrewOptions) ([]int, error) {
	selectedPages, err := promptSelectPages(m, opts)
	if err != nil {
		return nil, err
	}
	if len(selectedPages) == 0 {
		out := opts.Out
		if out == nil {
			out = os.Stdout
		}
		_, _ = fmt.Fprintln(out, "No pages selected. Exiting.")
	}
	return selectedPages, nil
}

// writeDummyPNG creates a minimal valid PNG at destPath.
func writeDummyPNG(destPath string) error {
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory structure for dummy PNG: %w", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 200, G: 200, B: 200, A: 255})
	//nolint:gosec // destPath is validated temporary/workspace destination
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return png.Encode(f, img)
}

// checkBudget estimates cost of run and prompts/aborts if it exceeds configured/requested budget limit.
// estimateCost estimates the expected cost for images and LLM stanzas for the current run.
// countPendingImages counts the number of pages that still need illustration generations.
func countPendingImages(m *manifest.Manifest, opts *BrewOptions) int {
	var allowedPages map[int]bool
	if len(opts.Pages) > 0 {
		allowedPages = make(map[int]bool, len(opts.Pages))
		for _, pIdx := range opts.Pages {
			allowedPages[pIdx] = true
		}
	}
	expectedImageCount := 0
	for _, page := range m.Progress.Pages {
		if allowedPages != nil && !allowedPages[page.PageIndex] {
			continue
		}
		if page.Status != manifest.StatusCompleted || page.ImagePath == "" {
			expectedImageCount++
		}
	}
	return expectedImageCount
}

// estimateCost estimates the expected cost for images and LLM stanzas for the current run.
func estimateCost(m *manifest.Manifest, opts *BrewOptions) (float64, float64) {
	var pricing map[string]telemetry.ModelPricing
	if config.Cfg != nil {
		pricing = config.Cfg.Pricing
	}

	imageCost := 0.04
	if pricing != nil {
		if p, ok := pricing["imagegen"]; ok {
			imageCost = p.Input / 1_000_000.0
		}
	}

	expectedImageCount := 0
	if !m.Progress.ManuscriptGenerated {
		expectedImageCount = m.BookProperties.TargetPageCount
		if expectedImageCount <= 0 {
			expectedImageCount = 15
		}
	} else {
		expectedImageCount = countPendingImages(m, opts)
	}

	if m.BookProperties.CharacterProfile != "" && m.BookProperties.CharacterReferenceURL == "" {
		expectedImageCount++
	}

	expectedImageCost := float64(expectedImageCount) * imageCost

	expectedLlmCost := 0.0
	if !m.Progress.ManuscriptGenerated {
		expectedLlmCost = 0.01
	}

	return expectedImageCost, expectedLlmCost
}

// checkBudget estimates cost of run and prompts/aborts if it exceeds configured/requested budget limit.
func checkBudget(m *manifest.Manifest, opts *BrewOptions) error {
	expectedImageCost, expectedLlmCost := estimateCost(m, opts)

	currentCost := m.GetTotalCost()
	totalEstimatedCost := currentCost + expectedImageCost + expectedLlmCost

	budget := 5.00
	if opts.Budget > 0 {
		budget = opts.Budget
	} else if config.Cfg != nil {
		if cfgMax := config.Cfg.GetMaxCostUSD(); cfgMax > 0 {
			budget = cfgMax
		}
	}

	if totalEstimatedCost <= budget {
		return nil
	}

	out := opts.Out
	if out == nil {
		out = os.Stdout
	}

	// Print warnings
	warnTitle := ui.FailStyle.Render("⚠️  BUDGET WARNING: The estimated cost of this run exceeds your budget limit.")
	_, _ = fmt.Fprintln(out, warnTitle)
	_, _ = fmt.Fprintf(out, "  Current Cost:              $%.4f\n", currentCost)
	_, _ = fmt.Fprintf(out, "  Estimated Additional Cost: $%.4f\n", expectedImageCost+expectedLlmCost)
	_, _ = fmt.Fprintf(out, "  Total Estimated Cost:      $%.4f\n", totalEstimatedCost)
	_, _ = fmt.Fprintf(out, "  Budget Limit:              $%.4f\n", budget)
	_, _ = fmt.Fprintf(out, "  Excess Cost:               $%.4f\n\n", totalEstimatedCost-budget)

	if !isTTY() || opts.Silent {
		return fmt.Errorf("budget exceeded: estimated cost $%.4f exceeds budget limit $%.4f", totalEstimatedCost, budget)
	}

	in := opts.In
	if in == nil {
		in = os.Stdin
	}

	var proceed bool
	confirm := huh.NewConfirm().
		Title("Do you want to proceed anyway?").
		Value(&proceed).
		WithTheme(huh.ThemeCharm())

	form := huh.NewForm(huh.NewGroup(confirm)).WithInput(in).WithOutput(out)
	form.WithAccessible(opts.In != nil)
	if err := form.Run(); err != nil {
		return fmt.Errorf("prompt error: %w", err)
	}

	if !proceed {
		return fmt.Errorf("run aborted: estimated cost $%.4f exceeds budget limit $%.4f", totalEstimatedCost, budget)
	}

	_, _ = fmt.Fprintln(out, ui.SuccessStyle.Render("Proceeding with custom budget override for this run."))
	opts.Budget = totalEstimatedCost
	return nil
}
