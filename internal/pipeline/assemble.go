package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/borch-ai/pithos/internal/logger"
	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/pithos/internal/mcp"
	"github.com/borch-ai/pithos/internal/registry"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// AssembleOptions holds configuration parameters for the assemble command.
type AssembleOptions struct {
	InputDir          string
	Format            string // e.g. "paperback", "hardcover"
	Bleed             bool
	TrimSize          string // e.g. "6x9"
	PaperType         string // e.g. "white"
	Silent            bool
	MCPTransport      mcpsdk.Transport // For testing (fallback)
	KDPMathTransport  mcpsdk.Transport // For testing
	TypstTransport    mcpsdk.Transport // For testing
	PDFCheckTransport mcpsdk.Transport // For testing
	DryRun            bool
}

type geometryResult struct {
	SpineWidthInches     float64                `json:"spine_width_inches"`
	CoverWidthInches     float64                `json:"cover_width_inches"`
	CoverHeightInches    float64                `json:"cover_height_inches"`
	CoverWidthPoints     float64                `json:"cover_width_points"`
	CoverHeightPoints    float64                `json:"cover_height_points"`
	HingeWidthInches     float64                `json:"hinge_width_inches,omitempty"`
	WrapWidthInches      float64                `json:"wrap_width_inches,omitempty"`
	OverhangHeightInches float64                `json:"overhang_height_inches,omitempty"`
	SpineTextEligible    bool                   `json:"spine_text_eligible"`
	Guides               []manifest.LayoutGuide `json:"guides,omitempty"`
}

// Assemble validates the page count, calls pw-mcp-kdp-math to calculate dimensions,
// and saves the results to the manifest.
//
//nolint:gocognit,funlen // Assemble function coordinates page count validation, fetchGeometry, and Typst compile
func Assemble(ctx context.Context, opts AssembleOptions) (*manifest.Manifest, error) {
	if opts.InputDir == "" {
		return nil, errors.New("input directory is required")
	}
	opts.InputDir = resolveBookPath(opts.InputDir)

	manifestPath := filepath.Join(opts.InputDir, "manifest.json")
	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Register book workspace path globally
	if regErr := registry.Add(opts.InputDir); regErr != nil {
		logger.Warn("Failed to register book workspace path in global registry during assemble", "error", regErr)
	}

	// 1. Count total pages in Progress.Pages or fallback to target page count
	pageCount := len(m.Progress.Pages)
	if pageCount == 0 {
		pageCount = m.BookProperties.TargetPageCount
	}
	if pageCount <= 0 {
		return nil, errors.New("cannot assemble book: page count is zero or not defined")
	}

	// Default/fallback format logic
	format := opts.Format
	if format == "" {
		format = m.BookProperties.Format
	}
	if format == "" {
		format = "paperback"
	}

	// 2. Validate hardcover constraint: hardcover must have at least 75 pages
	if format == "hardcover" && pageCount < 75 {
		return nil, fmt.Errorf("hardcover validation failed: page count %d is less than the KDP hardcover minimum limit of 75 pages", pageCount)
	}

	if opts.TrimSize == "" {
		opts.TrimSize = m.BookProperties.TrimSize
	}
	if opts.TrimSize == "" {
		opts.TrimSize = "6x9"
	}

	geom, err := fetchGeometry(ctx, opts, pageCount, format)
	if err != nil {
		return nil, err
	}

	// Determine bleed and margin sizes
	bleedVal := 0.0
	if opts.Bleed {
		bleedVal = 0.125
	}
	marginVal := 0.75 // standard KDP safety margin

	// 4. Update manifest.json with calculated dimensions
	m.BookProperties.Format = format
	m.BookProperties.TrimSize = opts.TrimSize
	m.KDPLayout = manifest.KDPLayout{
		SpineWidth:           geom.SpineWidthInches,
		MarginSize:           marginVal,
		Bleed:                bleedVal,
		CoverWidthInches:     geom.CoverWidthInches,
		CoverHeightInches:    geom.CoverHeightInches,
		CoverWidthPoints:     geom.CoverWidthPoints,
		CoverHeightPoints:    geom.CoverHeightPoints,
		HingeWidthInches:     geom.HingeWidthInches,
		WrapWidthInches:      geom.WrapWidthInches,
		OverhangHeightInches: geom.OverhangHeightInches,
		SpineTextEligible:    geom.SpineTextEligible,
		Guides:               geom.Guides,
	}

	if saveErr := m.Save(); saveErr != nil {
		return nil, fmt.Errorf("failed to save manifest after geometry assembly: %w", saveErr)
	}

	var typstClient *mcp.PluginClient
	defer func() {
		if typstClient != nil {
			_ = typstClient.Stop()
		}
	}()

	// 5. Compile the interior PDF
	pdfPath, err := compileInteriorPDF(ctx, opts, m, &typstClient)
	if err != nil {
		return nil, err
	}

	if err := m.RegisterAsset("interior_pdf", pdfPath); err != nil {
		return nil, fmt.Errorf("failed to register interior PDF asset: %w", err)
	}

	// 6. Compile the cover PDF wrap (if cover image is available)
	coverPDFPath, cErr := compileCoverPDF(ctx, opts, m, pageCount, format, typstClient)
	if cErr != nil {
		return nil, cErr
	}
	if coverPDFPath != "" {
		if err := m.RegisterAsset("cover_pdf", coverPDFPath); err != nil {
			return nil, fmt.Errorf("failed to register cover PDF asset: %w", err)
		}
	}

	// Run PDF preflight checks (Interior and optionally Cover PDF)
	if err := runPDFPreflightCheck(ctx, opts, m, pdfPath); err != nil {
		return nil, fmt.Errorf("pdf preflight check failed: %w", err)
	}

	if err := m.UpdatePDFPaths(pdfPath, coverPDFPath); err != nil {
		return nil, fmt.Errorf("failed to update PDF paths in manifest: %w", err)
	}

	if err := m.AddMilestone("assemble_complete"); err != nil {
		return nil, fmt.Errorf("failed to record assemble_complete milestone: %w", err)
	}

	// Regenerate web preview with updated KDP layout calculations
	if previewErr := GenerateWebPreview(opts.InputDir, m); previewErr != nil {
		logger.Warn("Failed to regenerate web preview", "error", previewErr)
	} else if !opts.Silent {
		previewPath := filepath.Join(opts.InputDir, "web_preview", "preview.html")
		triggerBrowserOpen(ctx, formatFileURL(previewPath))
	}

	if err := Checkpoint(ctx, opts.InputDir, "Compiled print layouts and PDFs"); err != nil {
		return nil, err
	}

	return m, nil
}

func compileInteriorPDF(ctx context.Context, opts AssembleOptions, m *manifest.Manifest, sharedClients ...**mcp.PluginClient) (string, error) {
	manuscriptPath := filepath.Join(opts.InputDir, "manuscript.md")
	if _, err := os.Stat(manuscriptPath); err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to check manuscript.md status: %w", err)
		}
		if len(m.Progress.Pages) == 0 {
			return "", errors.New("cannot assemble book: manuscript.md is missing and no pages are generated in the manifest")
		}
		if exportErr := exportManuscriptToMarkdown(opts.InputDir, m.BookProperties.Style, m.BookProperties.CharacterProfile, m.Progress.Pages); exportErr != nil {
			return "", fmt.Errorf("failed to export manuscript.md: %w", exportErr)
		}
	}
	imagesDir := filepath.Join(opts.InputDir, "images")
	outputPath := filepath.Join(opts.InputDir, "interior.pdf")

	if opts.DryRun {
		if err := os.WriteFile(outputPath, []byte("SIMULATED PDF CONTENT"), 0600); err != nil {
			return "", fmt.Errorf("failed to write simulated PDF: %w", err)
		}
		return outputPath, nil
	}

	trimSize := opts.TrimSize
	if trimSize == "" {
		trimSize = "6x9"
	}

	pageSize := formatTrimSizeForTypst(trimSize)
	bleedVal := fmt.Sprintf("%.3fin", m.KDPLayout.Bleed)
	marginVal := fmt.Sprintf("%.3fin", m.KDPLayout.MarginSize)

	// Call pw-mcp-typst MCP client
	mcpClient, cleanup, clientErr := acquireInteriorTypstClient(ctx, opts, sharedClients)
	if clientErr != nil {
		return "", clientErr
	}
	defer cleanup()

	args := map[string]interface{}{
		"manuscript_path": manuscriptPath,
		"images_dir":      imagesDir,
		"output_path":     outputPath,
		"page_size":       pageSize,
		"bleed":           bleedVal,
		"margin_inside":   marginVal,
		"margin_outside":  marginVal,
	}

	resText, err := mcpClient.CallTool(ctx, "compile_interior", args)
	if err != nil {
		return "", fmt.Errorf("interior compilation failed: %w", err)
	}

	var result struct {
		OutputPDF string `json:"output_pdf"`
		PageCount int    `json:"page_count"`
	}
	if err := json.Unmarshal([]byte(resText), &result); err == nil {
		if result.OutputPDF != "" {
			return result.OutputPDF, nil
		}
		return outputPath, nil
	}

	// Fallback to checking if the raw response text is a plain path pointing to a PDF file
	trimmedRes := strings.TrimSpace(resText)
	if strings.HasSuffix(strings.ToLower(trimmedRes), ".pdf") {
		return trimmedRes, nil
	}

	return "", fmt.Errorf("unexpected non-JSON response from compile_interior tool (raw: %q)", resText)
}

func compileCoverPDF(ctx context.Context, opts AssembleOptions, m *manifest.Manifest, pageCount int, format string, typstClients ...*mcp.PluginClient) (string, error) {
	coverImgRel := m.Progress.CoverImagePath
	if coverImgRel == "" {
		coverImgRel = "images/cover.png"
	}
	coverImgPath := filepath.Join(opts.InputDir, coverImgRel)
	if _, err := os.Stat(coverImgPath); err != nil {
		logger.Warn("Cover image not found, skipping cover PDF compilation", "path", coverImgPath)
		return "", nil
	}

	outputPath := filepath.Join(opts.InputDir, "cover.pdf")

	if opts.DryRun {
		if err := os.WriteFile(outputPath, []byte("SIMULATED COVER PDF CONTENT"), 0600); err != nil {
			return "", fmt.Errorf("failed to write simulated cover PDF: %w", err)
		}
		return outputPath, nil
	}

	title := m.BookProperties.Title
	if title == "" {
		title = m.BookProperties.Theme
	}
	subtitle := m.BookProperties.Subtitle
	author := m.BookProperties.Author
	if author == "" {
		author = "Anonymous"
	}
	trimSize := opts.TrimSize
	if trimSize == "" {
		trimSize = m.BookProperties.TrimSize
	}
	if trimSize == "" {
		trimSize = "6x9"
	}
	paperType := opts.PaperType
	if paperType == "" {
		paperType = "white"
	}

	mcpClient, cleanup, clientErr := getTypstClient(ctx, opts, typstClients, "failed to start MCP typst client for cover")
	if clientErr != nil {
		return "", clientErr
	}
	defer cleanup()

	args := map[string]interface{}{
		"front_image_path": coverImgPath,
		"title":            title,
		"subtitle":         subtitle,
		"author":           author,
		"back_cover_blurb": m.BookProperties.BackCoverBlurb,
		"page_count":       pageCount,
		"output_path":      outputPath,
		"trim_size":        trimSize,
		"paper_type":       paperType,
		"binding_type":     format,
		"bleed":            fmt.Sprintf("%.3fin", m.KDPLayout.Bleed),
		"spine_width":      fmt.Sprintf("%.3fin", m.KDPLayout.SpineWidth),
	}

	resText, err := mcpClient.CallTool(ctx, "compile_cover", args)
	if err != nil {
		return "", fmt.Errorf("cover compilation failed: %w", err)
	}

	var result struct {
		OutputPDF string `json:"output_pdf"`
	}
	if err := json.Unmarshal([]byte(resText), &result); err == nil && result.OutputPDF != "" {
		return result.OutputPDF, nil
	}

	trimmedRes := strings.TrimSpace(resText)
	if strings.HasSuffix(strings.ToLower(trimmedRes), ".pdf") {
		return trimmedRes, nil
	}

	return outputPath, nil
}

func formatTrimSizeForTypst(trimSize string) string {
	parts := strings.Split(strings.ToLower(trimSize), "x")
	if len(parts) == 2 {
		return fmt.Sprintf("%sin,%sin", strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}
	return "6in,9in" // fallback default
}

func parseTrimSize(trimSize string) (float64, float64, error) {
	parts := strings.Split(strings.ToLower(trimSize), "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid format: %q", trimSize)
	}
	w, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid width %q: %w", parts[0], err)
	}
	h, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid height %q: %w", parts[1], err)
	}
	return w, h, nil
}

func runPDFPreflightCheck(ctx context.Context, opts AssembleOptions, m *manifest.Manifest, pdfPath string) error {
	if opts.DryRun {
		return nil
	}

	expectedWidth, expectedHeight, err := parseTrimSize(m.BookProperties.TrimSize)
	if err != nil {
		return fmt.Errorf("failed to parse trim size %q: %w", m.BookProperties.TrimSize, err)
	}

	mcpClient := mcp.NewPluginClient(mcp.PluginPDFCheck)
	if opts.PDFCheckTransport != nil {
		mcpClient.SetTransport(opts.PDFCheckTransport)
	} else if opts.MCPTransport != nil {
		mcpClient.SetTransport(opts.MCPTransport)
	}

	if startErr := mcpClient.Start(ctx); startErr != nil {
		return fmt.Errorf("failed to start MCP pdfcheck client: %w", startErr)
	}
	defer func() { _ = mcpClient.Stop() }()

	if err := validateInteriorPDF(ctx, mcpClient, pdfPath, expectedWidth, expectedHeight, m); err != nil {
		return err
	}

	coverPDFPath := m.AssetRegistry["cover_pdf"]
	if coverPDFPath != "" {
		return validateCoverPDF(ctx, mcpClient, coverPDFPath, expectedWidth, expectedHeight, m, opts.PaperType)
	}

	return nil
}

type pdfCheckResult struct {
	Valid      bool     `json:"valid"`
	PageCount  int      `json:"page_count"`
	Dimensions string   `json:"dimensions"`
	Errors     []string `json:"errors"`
	Warnings   []string `json:"warnings"`
}

func validateInteriorPDF(ctx context.Context, mcpClient *mcp.PluginClient, pdfPath string, expectedWidth, expectedHeight float64, m *manifest.Manifest) error {
	args := map[string]interface{}{
		"pdf_path":               pdfPath,
		"expected_width_inches":  expectedWidth,
		"expected_height_inches": expectedHeight,
		"bleed_inches":           m.KDPLayout.Bleed,
		"min_gutter_inches":      m.KDPLayout.MarginSize,
	}

	resText, err := mcpClient.CallTool(ctx, "validate_pdf", args)
	if err != nil {
		return fmt.Errorf("validate_pdf tool invocation failed: %w", err)
	}

	var checkRes pdfCheckResult
	if err := json.Unmarshal([]byte(resText), &checkRes); err != nil {
		return fmt.Errorf("failed to parse validate_pdf response JSON: %w (raw response: %q)", err, resText)
	}

	if !checkRes.Valid || len(checkRes.Errors) > 0 {
		logger.Error("Interior PDF Preflight Validation failed", "path", pdfPath)
		for _, e := range checkRes.Errors {
			logger.Error("  - ERROR", "msg", e)
		}
		for _, w := range checkRes.Warnings {
			logger.Warn("  - WARNING", "msg", w)
		}
		return fmt.Errorf("interior PDF check reported errors: %v", checkRes.Errors)
	}
	return nil
}

func validateCoverPDF(ctx context.Context, mcpClient *mcp.PluginClient, coverPDFPath string, expectedWidth, expectedHeight float64, m *manifest.Manifest, paperType string) error {
	pageCount := len(m.Progress.Pages)
	if pageCount == 0 {
		pageCount = m.BookProperties.TargetPageCount
	}

	if paperType == "" {
		paperType = "white"
	}
	paperTypeMapped := strings.ToLower(paperType)
	if paperTypeMapped == "standard_color" || paperTypeMapped == "premium_color" {
		paperTypeMapped = "color"
	}

	coverArgs := map[string]interface{}{
		"pdf_path":               coverPDFPath,
		"expected_width_inches":  expectedWidth,
		"expected_height_inches": expectedHeight,
		"page_count":             pageCount,
		"paper_type":             paperTypeMapped,
		"bleed_inches":           m.KDPLayout.Bleed,
	}

	coverResText, err := mcpClient.CallTool(ctx, "validate_cover_pdf", coverArgs)
	if err != nil {
		return fmt.Errorf("validate_cover_pdf tool invocation failed: %w", err)
	}

	var coverCheckRes pdfCheckResult
	if err := json.Unmarshal([]byte(coverResText), &coverCheckRes); err != nil {
		return fmt.Errorf("failed to parse validate_cover_pdf response JSON: %w (raw response: %q)", err, coverResText)
	}

	if !coverCheckRes.Valid || len(coverCheckRes.Errors) > 0 {
		logger.Error("Cover PDF Preflight Validation failed", "path", coverPDFPath)
		for _, e := range coverCheckRes.Errors {
			logger.Error("  - ERROR", "msg", e)
		}
		for _, w := range coverCheckRes.Warnings {
			logger.Warn("  - WARNING", "msg", w)
		}
		return fmt.Errorf("cover PDF check reported errors: %v", coverCheckRes.Errors)
	}
	return nil
}

func fetchGeometry(ctx context.Context, opts AssembleOptions, pageCount int, format string) (geometryResult, error) {
	var geom geometryResult

	if opts.DryRun {
		geom = geometryResult{
			SpineWidthInches:  0.15,
			CoverWidthInches:  12.5,
			CoverHeightInches: 9.25,
			CoverWidthPoints:  900.0,
			CoverHeightPoints: 666.0,
			SpineTextEligible: false,
		}
		return geom, nil
	}

	trimSize := opts.TrimSize
	if trimSize == "" {
		trimSize = "6x9"
	}
	paperType := opts.PaperType
	if paperType == "" {
		paperType = "white"
	}

	// 3. Connect to pw-mcp-kdp-math MCP client
	mcpClient := mcp.NewPluginClient(mcp.PluginKDPMath)
	if opts.KDPMathTransport != nil {
		mcpClient.SetTransport(opts.KDPMathTransport)
	} else if opts.MCPTransport != nil {
		mcpClient.SetTransport(opts.MCPTransport)
	}

	if startErr := mcpClient.Start(ctx); startErr != nil {
		return geom, fmt.Errorf("failed to start MCP kdp-math client: %w", startErr)
	}
	defer func() { _ = mcpClient.Stop() }()

	// Call kdp_calculate_geometry tool
	args := map[string]interface{}{
		"page_count":   pageCount,
		"binding_type": format,
		"paper_type":   paperType,
		"trim_size":    trimSize,
	}

	resText, err := mcpClient.CallTool(ctx, "kdp_calculate_geometry", args)
	if err != nil {
		return geom, fmt.Errorf("geometry calculation failed: %w", err)
	}

	if err := json.Unmarshal([]byte(resText), &geom); err != nil {
		return geom, fmt.Errorf("failed to parse geometry result JSON: %w (raw response: %q)", err, resText)
	}

	return geom, nil
}

func getTypstClient(ctx context.Context, opts AssembleOptions, typstClients []*mcp.PluginClient, errPrefix string) (*mcp.PluginClient, func(), error) {
	if len(typstClients) > 0 && typstClients[0] != nil {
		return typstClients[0], func() {}, nil
	}
	client := mcp.NewPluginClient(mcp.PluginTypst)
	if opts.TypstTransport != nil {
		client.SetTransport(opts.TypstTransport)
	} else if opts.MCPTransport != nil {
		client.SetTransport(opts.MCPTransport)
	}

	if startErr := client.Start(ctx); startErr != nil {
		return nil, nil, fmt.Errorf("%s: %w", errPrefix, startErr)
	}
	return client, func() { _ = client.Stop() }, nil
}

func acquireInteriorTypstClient(ctx context.Context, opts AssembleOptions, sharedClients []**mcp.PluginClient) (*mcp.PluginClient, func(), error) {
	if len(sharedClients) == 0 || sharedClients[0] == nil {
		return getTypstClient(ctx, opts, nil, "failed to start MCP typst client")
	}
	if *sharedClients[0] != nil {
		return *sharedClients[0], func() {}, nil
	}
	client, _, err := getTypstClient(ctx, opts, nil, "failed to start MCP typst client")
	if err != nil {
		return nil, nil, err
	}
	*sharedClients[0] = client
	return client, func() {}, nil
}
