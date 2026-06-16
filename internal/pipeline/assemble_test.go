package pipeline

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/borch-ai/pithos/internal/manifest"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func setupMockKDPMathServer(t *testing.T, ctx context.Context, serverTransport mcpsdk.Transport) (*mcpsdk.ServerSession, func()) {
	t.Helper()
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "mock-kdp-math-server",
		Version: "1.0.0",
	}, nil)

	server.AddTool(&mcpsdk.Tool{
		Name: "kdp_calculate_geometry",
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		var args struct {
			PageCount   int    `json:"page_count"`
			BindingType string `json:"binding_type"`
			PaperType   string `json:"paper_type"`
			TrimSize    string `json:"trim_size"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, err
		}

		if args.PageCount == 999 { // Special error trigger page count
			return &mcpsdk.CallToolResult{
				IsError: true,
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: "failed to calculate geometry: mock error"},
				},
			}, nil
		}

		if args.PageCount == 888 { // Special format error trigger
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: "not-json-content"},
				},
			}, nil
		}

		// Success mock geometry response
		resMap := map[string]interface{}{
			"binding_type":        args.BindingType,
			"paper_type":          args.PaperType,
			"page_count":          args.PageCount,
			"trim_width_inches":   6.0,
			"trim_height_inches":  9.0,
			"spine_width_inches":  0.15,
			"spine_width_points":  0.15 * 72.0,
			"cover_width_inches":  12.55,
			"cover_width_points":  12.55 * 72.0,
			"cover_height_inches": 9.25,
			"cover_height_points": 9.25 * 72.0,
			"spine_text_eligible": args.PageCount >= 79,
			"guides": []map[string]interface{}{
				{"label": "Spine", "x_inches": 6.125, "y_inches": 0.125, "width_inches": 0.15, "height_inches": 9.0},
			},
		}

		resBytes, _ := json.Marshal(resMap)
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: string(resBytes)},
			},
		}, nil
	})

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}

	return serverSession, func() {
		_ = serverSession.Close()
	}
}

func setupMockTypstServer(t *testing.T, ctx context.Context, serverTransport mcpsdk.Transport) (*mcpsdk.ServerSession, func()) {
	t.Helper()
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "mock-typst-server",
		Version: "1.0.0",
	}, nil)

	server.AddTool(&mcpsdk.Tool{
		Name: "compile_interior",
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		var args struct {
			ManuscriptPath string `json:"manuscript_path"`
			ImagesDir      string `json:"images_dir"`
			OutputPath     string `json:"output_path"`
			PageSize       string `json:"page_size"`
			Bleed          string `json:"bleed"`
			MarginInside   string `json:"margin_inside"`
			MarginOutside  string `json:"margin_outside"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, err
		}

		resMap := map[string]interface{}{
			"output_pdf": args.OutputPath,
			"page_count": 80,
		}

		resBytes, _ := json.Marshal(resMap)
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: string(resBytes)},
			},
		}, nil
	})

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}

	return serverSession, func() {
		_ = serverSession.Close()
	}
}

func TestAssemble_Success(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-assemble-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Initiate manifest
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Parody Theme",
		TargetPageCount: 80,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}

	// Populate mock pages in manifest with content to verify export structure
	m.Progress.Pages = make([]manifest.PageState, 80)
	for i := 0; i < 80; i++ {
		m.Progress.Pages[i] = manifest.PageState{
			PageIndex:          i + 1,
			Status:             manifest.StatusCompleted,
			Text:               "Stanza content text",
			IllustrationPrompt: "Illustration prompt details",
		}
	}
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientKDP, serverKDP := mcpsdk.NewInMemoryTransports()
	_, cleanupKDP := setupMockKDPMathServer(t, ctx, serverKDP)
	defer cleanupKDP()

	clientTypst, serverTypst := mcpsdk.NewInMemoryTransports()
	_, cleanupTypst := setupMockTypstServer(t, ctx, serverTypst)
	defer cleanupTypst()

	optsAssemble := AssembleOptions{
		InputDir:         tmpDir,
		Format:           "paperback",
		Bleed:            true,
		KDPMathTransport: clientKDP,
		TypstTransport:   clientTypst,
	}

	_, err = Assemble(ctx, optsAssemble)
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	// Verify manuscript.md was exported and has correct structure/content
	manuscriptPath := filepath.Join(tmpDir, "manuscript.md")
	if _, statErr := os.Stat(manuscriptPath); os.IsNotExist(statErr) {
		t.Error("expected manuscript.md to be automatically exported during Assemble, but it was not found")
	} else {
		//nolint:gosec // manuscriptPath is constructed in temp test directory
		contentBytes, readErr := os.ReadFile(manuscriptPath)
		if readErr != nil {
			t.Fatalf("failed to read manuscript.md: %v", readErr)
		}
		content := string(contentBytes)
		if !strings.Contains(content, "# Page 1") || !strings.Contains(content, "Stanza content text") || !strings.Contains(content, "Illustration prompt details") {
			t.Errorf("manuscript.md did not contain expected content: %s", content)
		}
	}

	// Reload manifest and check KDP layout details
	m2, err := manifest.LoadManifest(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	if math.Abs(m2.KDPLayout.SpineWidth-0.15) > 1e-9 {
		t.Errorf("expected SpineWidth 0.15, got %f", m2.KDPLayout.SpineWidth)
	}
	if !m2.KDPLayout.SpineTextEligible {
		t.Error("expected SpineTextEligible to be true")
	}
	if len(m2.KDPLayout.Guides) != 1 || m2.KDPLayout.Guides[0].Label != "Spine" {
		t.Errorf("expected 1 Guide labelled 'Spine', got: %v", m2.KDPLayout.Guides)
	}
	if m2.AssetRegistry["interior_pdf"] == "" {
		t.Error("expected interior_pdf asset to be registered, got empty")
	}
	if len(m2.Kiln.Milestones) != 2 || m2.Kiln.Milestones[0] != "initiate_complete" || m2.Kiln.Milestones[1] != "assemble_complete" {
		t.Errorf("expected milestones [initiate_complete, assemble_complete], got %v", m2.Kiln.Milestones)
	}
	if m2.Kiln.InteriorPDFPath == "" {
		t.Error("expected InteriorPDFPath to be set in Kiln sync data")
	}
	if m2.Kiln.InteriorPDFPath != m2.AssetRegistry["interior_pdf"] {
		t.Errorf("expected Kiln InteriorPDFPath to match AssetRegistry, got %q vs %q", m2.Kiln.InteriorPDFPath, m2.AssetRegistry["interior_pdf"])
	}
}

func TestAssemble_HardcoverValidation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-assemble-val-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Parody Theme",
		TargetPageCount: 15, // less than 75 pages
	}
	_, err = Initiate(optsInit)
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}

	ctx := context.Background()
	optsAssemble := AssembleOptions{
		InputDir: tmpDir,
		Format:   "hardcover",
	}

	_, err = Assemble(ctx, optsAssemble)
	if err == nil {
		t.Error("expected hardcover assembly to fail validation for page count < 75, got nil")
	} else if !strings.Contains(err.Error(), "hardcover validation failed: page count 15 is less than the KDP hardcover minimum limit of 75 pages") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAssemble_MissingInputDir(t *testing.T) {
	_, err := Assemble(context.Background(), AssembleOptions{})
	if err == nil {
		t.Error("expected error when InputDir is missing, got nil")
	}
}

func TestAssemble_MissingManifest(t *testing.T) {
	_, err := Assemble(context.Background(), AssembleOptions{InputDir: "/nonexistent-dir"})
	if err == nil {
		t.Error("expected error when manifest is missing, got nil")
	}
}

func TestAssemble_ZeroPageCount(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-assemble-zero-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		TargetPageCount: 0,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}
	m.BookProperties.TargetPageCount = 0
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	optsAssemble := AssembleOptions{
		InputDir: tmpDir,
	}
	_, err = Assemble(context.Background(), optsAssemble)
	if err == nil {
		t.Error("expected error when page count is zero, got nil")
	}
}

func TestAssemble_MCPError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-assemble-mcp-err-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		TargetPageCount: 999, // Special value for mock server error
	}
	_, err = Initiate(optsInit)
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockKDPMathServer(t, ctx, serverTransport)
	defer cleanupMCP()

	optsAssemble := AssembleOptions{
		InputDir:     tmpDir,
		MCPTransport: clientTransport,
	}

	_, err = Assemble(ctx, optsAssemble)
	if err == nil {
		t.Error("expected error when MCP tool returns failure, got nil")
	} else if !strings.Contains(err.Error(), "geometry calculation failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAssemble_UnmarshalJSONError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-assemble-json-err-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		TargetPageCount: 888, // Special value for mock json error
	}
	_, err = Initiate(optsInit)
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockKDPMathServer(t, ctx, serverTransport)
	defer cleanupMCP()

	optsAssemble := AssembleOptions{
		InputDir:     tmpDir,
		MCPTransport: clientTransport,
	}

	_, err = Assemble(ctx, optsAssemble)
	if err == nil {
		t.Error("expected error when MCP tool returns invalid json, got nil")
	} else if !strings.Contains(err.Error(), "failed to parse geometry result JSON") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAssemble_FormatDefaulting(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Case A: opts.Format is empty, but manifest.BookProperties.Format is "hardcover"
	tmpDir, err := os.MkdirTemp("", "pithos-assemble-def-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		TargetPageCount: 80,
		Format:          "hardcover",
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}
	m.Progress.Pages = make([]manifest.PageState, 80)
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	clientKDP, serverKDP := mcpsdk.NewInMemoryTransports()
	_, cleanupKDP := setupMockKDPMathServer(t, ctx, serverKDP)
	defer cleanupKDP()

	clientTypst, serverTypst := mcpsdk.NewInMemoryTransports()
	_, cleanupTypst := setupMockTypstServer(t, ctx, serverTypst)
	defer cleanupTypst()

	optsAssemble := AssembleOptions{
		InputDir:         tmpDir,
		Format:           "", // empty to trigger defaulting to manifest
		KDPMathTransport: clientKDP,
		TypstTransport:   clientTypst,
	}

	mRes, err := Assemble(ctx, optsAssemble)
	if err != nil {
		t.Fatalf("Assemble with manifest format failed: %v", err)
	}
	if mRes.BookProperties.Format != "hardcover" {
		t.Errorf("expected format to remain 'hardcover', got %q", mRes.BookProperties.Format)
	}

	// Case B: both opts.Format and manifest.BookProperties.Format are empty
	tmpDir2, err := os.MkdirTemp("", "pithos-assemble-def2-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir2) }()

	optsInit2 := InitiateOptions{
		OutputDir:       tmpDir2,
		TargetPageCount: 15,
		Format:          "", // empty
	}
	m2, err := Initiate(optsInit2)
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}
	m2.Progress.Pages = make([]manifest.PageState, 15)
	if saveErr := m2.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	clientKDP2, serverKDP2 := mcpsdk.NewInMemoryTransports()
	_, cleanupKDP2 := setupMockKDPMathServer(t, ctx, serverKDP2)
	defer cleanupKDP2()

	clientTypst2, serverTypst2 := mcpsdk.NewInMemoryTransports()
	_, cleanupTypst2 := setupMockTypstServer(t, ctx, serverTypst2)
	defer cleanupTypst2()

	optsAssemble2 := AssembleOptions{
		InputDir:         tmpDir2,
		Format:           "", // empty
		KDPMathTransport: clientKDP2,
		TypstTransport:   clientTypst2,
	}

	mRes2, err := Assemble(ctx, optsAssemble2)
	if err != nil {
		t.Fatalf("Assemble with default paperback failed: %v", err)
	}
	if mRes2.BookProperties.Format != "paperback" {
		t.Errorf("expected format to default to 'paperback', got %q", mRes2.BookProperties.Format)
	}
}

func TestAssemble_TypstError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-assemble-typst-err-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Parody Theme",
		TargetPageCount: 80,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}
	m.Progress.Pages = make([]manifest.PageState, 80)
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientKDP, serverKDP := mcpsdk.NewInMemoryTransports()
	_, cleanupKDP := setupMockKDPMathServer(t, ctx, serverKDP)
	defer cleanupKDP()

	clientTypst, serverTypst := mcpsdk.NewInMemoryTransports()
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "mock-typst-server-error",
		Version: "1.0.0",
	}, nil)
	server.AddTool(&mcpsdk.Tool{
		Name: "compile_interior",
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{
			IsError: true,
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: "failed to compile interior: typst mock error"},
			},
		}, nil
	})
	serverSession, err := server.Connect(ctx, serverTypst, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = serverSession.Close() }()

	optsAssemble := AssembleOptions{
		InputDir:         tmpDir,
		KDPMathTransport: clientKDP,
		TypstTransport:   clientTypst,
	}

	_, err = Assemble(ctx, optsAssemble)
	if err == nil {
		t.Error("expected error when Typst compilation fails, got nil")
	} else if !strings.Contains(err.Error(), "interior compilation failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAssemble_ManuscriptStatError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-assemble-stat-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Initiate manifest
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Parody Theme",
		TargetPageCount: 3,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}

	m.Progress.Pages = make([]manifest.PageState, 3)
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	manuscriptPath := filepath.Join(tmpDir, "manuscript.md")
	// Create a symlink loop (manuscript.md points to itself) to force os.Stat to fail
	if symlinkErr := os.Symlink("manuscript.md", manuscriptPath); symlinkErr != nil {
		t.Skipf("skipping test: symlink creation is not supported on this platform/environment: %v", symlinkErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientKDP, serverKDP := mcpsdk.NewInMemoryTransports()
	_, cleanupKDP := setupMockKDPMathServer(t, ctx, serverKDP)
	defer cleanupKDP()

	optsAssemble := AssembleOptions{
		InputDir:         tmpDir,
		KDPMathTransport: clientKDP,
	}

	_, err = Assemble(ctx, optsAssemble)
	if err == nil {
		t.Error("expected error when os.Stat fails on manuscript.md with symlink loop, got nil")
	} else if !strings.Contains(err.Error(), "failed to check manuscript.md status") {
		t.Errorf("expected error to contain 'failed to check manuscript.md status', got: %v", err)
	}
}

func TestAssemble_MissingManuscriptNoPagesError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-assemble-missing-manuscript-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Initiate manifest
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Parody Theme",
		TargetPageCount: 3,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}

	// Make sure pages are empty
	m.Progress.Pages = nil
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientKDP, serverKDP := mcpsdk.NewInMemoryTransports()
	_, cleanupKDP := setupMockKDPMathServer(t, ctx, serverKDP)
	defer cleanupKDP()

	optsAssemble := AssembleOptions{
		InputDir:         tmpDir,
		KDPMathTransport: clientKDP,
	}

	_, err = Assemble(ctx, optsAssemble)
	if err == nil {
		t.Error("expected error when manuscript.md is missing and manifest has no pages, got nil")
	} else if !strings.Contains(err.Error(), "manuscript.md is missing and no pages are generated in the manifest") {
		t.Errorf("unexpected error: %v", err)
	}
}
