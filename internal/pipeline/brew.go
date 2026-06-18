package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/pithos/internal/mcp"
	"github.com/borch-ai/powerword/pkg/telemetry"
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
	Silent            bool
	MCPTransport      mcpsdk.Transport // For testing
	CloudMCPTransport mcpsdk.Transport // For testing
	LLM               LLMClient        // For testing
	HTTPClient        *http.Client     // For testing
}

// Brew executes the manuscript generation and page-by-page illustration generation.
//
//nolint:gocognit,funlen // Brew function integrates manifest loading, overrides, manuscript generation, review loop, and illustration generation
func Brew(ctx context.Context, opts BrewOptions) error {
	if opts.OutputDir == "" {
		return errors.New("output directory is required")
	}
	opts.OutputDir = resolveBookPath(opts.OutputDir)

	manifestPath := filepath.Join(opts.OutputDir, "manifest.json")
	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest from %s: %w", manifestPath, err)
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
			fmt.Println("Successfully imported edits from manuscript.md.")
			if err := Checkpoint(ctx, opts.OutputDir, "Imported manuscript edits from review"); err != nil {
				return err
			}
		}
	}

	// 1. Generate manuscript text if not yet generated
	if err := generateManuscript(ctx, m, opts); err != nil {
		return err
	}

	if err := handleReviewCheckpoint(opts, m, manuscriptPath); err != nil {
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
		fmt.Fprintf(os.Stderr, "Warning: failed to generate web preview: %v\n", previewErr)
	} else {
		previewPath := filepath.Join(opts.OutputDir, "web_preview", "preview.html")
		if absPath, err := filepath.Abs(previewPath); err == nil {
			previewPath = absPath
		}
		u := &url.URL{
			Scheme: "file",
			Path:   filepath.ToSlash(previewPath),
		}
		fmt.Printf("\nWeb preview generated: open %s in your browser to flip through the book!\n\n", u.String())
		if !opts.Silent {
			triggerBrowserOpen(u.String())
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

	fmt.Println("----------------------------------------")
	fmt.Print(tracker.FormatSummary(pricing))
	if m.Telemetry.ImageGenerations > 0 {
		fmt.Printf("- Image Generations: %d\n", m.Telemetry.ImageGenerations)
	}
	fmt.Printf("- Pipeline Total Cost: $%.5f\n", m.Telemetry.TotalCostUSD)
	fmt.Println("----------------------------------------")

	return nil
}

func handleReviewCheckpoint(opts BrewOptions, m *manifest.Manifest, manuscriptPath string) error {
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
		fmt.Fprintf(os.Stderr, "Warning: failed to generate web preview: %v\n", previewErr)
	} else {
		previewPath := filepath.Join(opts.OutputDir, "web_preview", "preview.html")
		if absPath, err := filepath.Abs(previewPath); err == nil {
			previewPath = absPath
		}
		u := &url.URL{
			Scheme: "file",
			Path:   filepath.ToSlash(previewPath),
		}
		fmt.Printf("\nWeb preview generated: open %s in your browser to flip through the book!\n\n", u.String())
		if !opts.Silent {
			triggerBrowserOpen(u.String())
		}
	}

	return fmt.Errorf("%w: manuscript is available at %s. Edit the file, then run brew without --review to generate illustrations", ErrReviewPause, manuscriptPath)
}

func setupLLMClient(opts BrewOptions) (LLMClient, error) {
	switch {
	case opts.LLM != nil:
		return opts.LLM, nil
	case config.Cfg != nil && config.Cfg.API.GeminiKey != "":
		pwClient, err := newLLMClientFunc("gemini-2.5-flash", config.Cfg.API.GeminiKey, "", opts.HTTPClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create gemini client: %w", err)
		}
		return &PowerwordClientAdapter{client: pwClient, modelName: "gemini-2.5-flash"}, nil
	case config.Cfg != nil && config.Cfg.API.OpenAIKey != "":
		pwClient, err := newLLMClientFunc("gpt-4o", "", config.Cfg.API.OpenAIKey, opts.HTTPClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create openai client: %w", err)
		}
		return &PowerwordClientAdapter{client: pwClient, modelName: "gpt-4o"}, nil
	default:
		return nil, errors.New("neither Gemini nor OpenAI API key is configured")
	}
}

func generateAndRecordVisualGuides(ctx context.Context, m *manifest.Manifest, theme string, llmClient LLMClient, opts BrewOptions) error {
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

//nolint:funlen // Manuscript generation pipeline step handles LLM client setup, text generation, and recording telemetry
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

	llmClient, err := setupLLMClient(opts)
	if err != nil {
		return err
	}

	if m.BookProperties.Style == "" || m.BookProperties.CharacterProfile == "" {
		if err = generateAndRecordVisualGuides(ctx, m, theme, llmClient, opts); err != nil {
			return err
		}
	}

	stanzas, prompts, tokenUsage, err := llmClient.GenerateStanzas(ctx, theme, pageCount, m.BookProperties.Style, m.BookProperties.CharacterProfile)
	if err != nil {
		return fmt.Errorf("manuscript text generation failed: %w", err)
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

	// Record token usage
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
	m.RecordLLMUsage(modelName, tokenUsage, pricing)

	if err := m.Save(); err != nil {
		return fmt.Errorf("failed to save manifest after manuscript generation: %w", err)
	}
	if err := Checkpoint(ctx, opts.OutputDir, "Generated stanzas and prompts"); err != nil {
		return err
	}
	return nil
}

//nolint:gocognit,funlen // Illustration loop handles MCP client lifecycle, style registration, and page checkpoint updates
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

	// Character Seed Portrait Bootstrapping
	if err := bootstrapCharacterReference(ctx, m, opts, mcpClient, styleID); err != nil {
		return err
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
			imgPath, err := generateSingleImage(ctx, mcpClient, p.PageIndex, prompt, styleID, opts.OutputDir, imageSize, m.BookProperties.CharacterReferenceURL, charWeight)
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

func generateImageRaw(ctx context.Context, mcpClient *mcp.PluginClient, prompt string, styleID string, imageSize string, crefURL string, charWeight *int) (string, error) {
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
		return "", err
	}

	prefix := "Successfully generated image and saved to: "
	if !strings.HasPrefix(resText, prefix) {
		return "", fmt.Errorf("unexpected imagegen response format: %q", resText)
	}

	return strings.TrimPrefix(resText, prefix), nil
}

func generateSingleImage(ctx context.Context, mcpClient *mcp.PluginClient, pageIndex int, pageText string, styleID string, outputDir string, imageSize string, crefURL string, charWeight *int) (string, error) {
	srcPath, err := generateImageRaw(ctx, mcpClient, pageText, styleID, imageSize, crefURL, charWeight)
	if err != nil {
		return "", fmt.Errorf("failed to generate image for page %d: %w", pageIndex, err)
	}

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

func bootstrapCharacterReference(ctx context.Context, m *manifest.Manifest, opts BrewOptions, mcpClient *mcp.PluginClient, styleID string) error {
	if m.BookProperties.CharacterReferenceURL != "" || m.BookProperties.CharacterProfile == "" {
		return nil
	}

	seedPrompt := "Detailed visual seed character portrait: " + m.BookProperties.CharacterProfile
	imageSize := getBestImageSize(m.BookProperties.TrimSize)

	fmt.Println("Generating character seed portrait...")
	srcPath, err := generateImageRaw(ctx, mcpClient, seedPrompt, styleID, imageSize, "", nil)
	if err != nil {
		return fmt.Errorf("failed to generate character seed portrait: %w", err)
	}

	ext := filepath.Ext(srcPath)
	if ext == "" {
		ext = ".png"
	}

	destPath := filepath.Join(opts.OutputDir, "images", "character_seed"+ext)
	if copyErr := copyFile(srcPath, destPath); copyErr != nil {
		return fmt.Errorf("failed to copy character seed image: %w", copyErr)
	}

	// Initialize pw-mcp-cloud client
	cloudClient := mcp.NewPluginClient(mcp.PluginCloud)
	if opts.CloudMCPTransport != nil {
		cloudClient.SetTransport(opts.CloudMCPTransport)
	}

	if startErr := cloudClient.Start(ctx); startErr != nil {
		return fmt.Errorf("failed to start MCP cloud client: %w", startErr)
	}
	defer func() { _ = cloudClient.Stop() }()

	absPath, err := filepath.Abs(destPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path of character seed image: %w", err)
	}

	fmt.Printf("Uploading character seed portrait to cloud storage from %s...\n", absPath)
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

	fmt.Printf("Character seed portrait uploaded successfully: %s\n", m.BookProperties.CharacterReferenceURL)
	return nil
}
