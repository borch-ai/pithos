package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/pithos/internal/mcp"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// AssembleOptions holds configuration parameters for the assemble command.
type AssembleOptions struct {
	InputDir         string
	Format           string // e.g. "paperback", "hardcover"
	Bleed            bool
	TrimSize         string           // e.g. "6x9"
	PaperType        string           // e.g. "white"
	MCPTransport     mcpsdk.Transport // For testing (fallback)
	KDPMathTransport mcpsdk.Transport // For testing
	TypstTransport   mcpsdk.Transport // For testing
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

	// 5. Compile the interior PDF
	pdfPath, err := compileInteriorPDF(ctx, opts, m)
	if err != nil {
		return nil, err
	}

	if err := m.RegisterAsset("interior_pdf", pdfPath); err != nil {
		return nil, fmt.Errorf("failed to register interior PDF asset: %w", err)
	}

	if err := Checkpoint(ctx, opts.InputDir, "Compiled print layouts and PDFs"); err != nil {
		return nil, err
	}

	return m, nil
}

func compileInteriorPDF(ctx context.Context, opts AssembleOptions, m *manifest.Manifest) (string, error) {
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

	trimSize := opts.TrimSize
	if trimSize == "" {
		trimSize = "6x9"
	}

	pageSize := formatTrimSizeForTypst(trimSize)
	bleedVal := fmt.Sprintf("%.3fin", m.KDPLayout.Bleed)
	marginVal := fmt.Sprintf("%.3fin", m.KDPLayout.MarginSize)

	// Call pw-mcp-typst MCP client
	mcpClient := mcp.NewPluginClient(mcp.PluginTypst)
	if opts.TypstTransport != nil {
		mcpClient.SetTransport(opts.TypstTransport)
	} else if opts.MCPTransport != nil {
		mcpClient.SetTransport(opts.MCPTransport)
	}

	if startErr := mcpClient.Start(ctx); startErr != nil {
		return "", fmt.Errorf("failed to start MCP typst client: %w", startErr)
	}
	defer func() { _ = mcpClient.Stop() }()

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

func formatTrimSizeForTypst(trimSize string) string {
	parts := strings.Split(strings.ToLower(trimSize), "x")
	if len(parts) == 2 {
		return fmt.Sprintf("%sin,%sin", strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}
	return "6in,9in" // fallback default
}

func fetchGeometry(ctx context.Context, opts AssembleOptions, pageCount int, format string) (geometryResult, error) {
	var geom geometryResult

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
