package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/pithos/internal/mcp"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// AssembleOptions holds configuration parameters for the assemble command.
type AssembleOptions struct {
	InputDir     string
	Format       string // e.g. "paperback", "hardcover"
	Bleed        bool
	TrimSize     string           // e.g. "6x9"
	PaperType    string           // e.g. "white"
	MCPTransport mcpsdk.Transport // For testing
}

// Assemble validates the page count, calls pw-mcp-kdp-math to calculate dimensions,
// and saves the results to the manifest.
func Assemble(ctx context.Context, opts AssembleOptions) error {
	if opts.InputDir == "" {
		return errors.New("input directory is required")
	}

	manifestPath := filepath.Join(opts.InputDir, "manifest.json")
	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// 1. Count total pages in Progress.Pages or fallback to target page count
	pageCount := len(m.Progress.Pages)
	if pageCount == 0 {
		pageCount = m.BookProperties.TargetPageCount
	}
	if pageCount <= 0 {
		return errors.New("cannot assemble book: page count is zero or not defined")
	}

	// 2. Validate hardcover constraint: hardcover must have at least 75 pages
	if opts.Format == "hardcover" && pageCount < 75 {
		return fmt.Errorf("hardcover validation failed: page count %d is less than the KDP hardcover minimum limit of 75 pages", pageCount)
	}

	// Default trim size and paper type
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
	if opts.MCPTransport != nil {
		mcpClient.SetTransport(opts.MCPTransport)
	}

	if startErr := mcpClient.Start(ctx); startErr != nil {
		return fmt.Errorf("failed to start MCP kdp-math client: %w", startErr)
	}
	defer func() { _ = mcpClient.Stop() }()

	// Call kdp_calculate_geometry tool
	args := map[string]interface{}{
		"page_count":   pageCount,
		"binding_type": opts.Format,
		"paper_type":   paperType,
		"trim_size":    trimSize,
	}

	resText, err := mcpClient.CallTool(ctx, "kdp_calculate_geometry", args)
	if err != nil {
		return fmt.Errorf("geometry calculation failed: %w", err)
	}

	// Parse GeometryResult JSON response from the tool
	var geom struct {
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

	if err := json.Unmarshal([]byte(resText), &geom); err != nil {
		return fmt.Errorf("failed to parse geometry result JSON: %w (raw response: %q)", err, resText)
	}

	// Determine bleed and margin sizes
	bleedVal := 0.0
	if opts.Bleed {
		bleedVal = 0.125
	}
	marginVal := 0.75 // standard KDP safety margin

	// 4. Update manifest.json with calculated dimensions
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

	if err := m.Save(); err != nil {
		return fmt.Errorf("failed to save manifest after geometry assembly: %w", err)
	}

	fmt.Printf("pithos assemble: Successfully generated layout manifest parameters for %s format.\n", opts.Format)
	fmt.Printf("Calculated Dimensions (inches): Cover Width: %.3f, Cover Height: %.3f, Spine Width: %.3f\n",
		geom.CoverWidthInches, geom.CoverHeightInches, geom.SpineWidthInches)

	return nil
}
