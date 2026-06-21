package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func setupMockImageGenServerWithOutputType(t *testing.T, ctx context.Context, serverTransport mcpsdk.Transport, generatedImagePath string, backend string, supportsCref, supportsSref bool, outputType string) (*mcpsdk.ServerSession, func()) {
	t.Helper()
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "mock-imagegen-server",
		Version: "1.0.0",
	}, nil)

	server.AddTool(&mcpsdk.Tool{
		Name: "imagegen_get_capabilities",
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		capsJSON := fmt.Sprintf(`{"backend":%q,"supports_cref":%t,"supports_sref":%t,"output_type":%q}`, backend, supportsCref, supportsSref, outputType)
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: capsJSON},
			},
		}, nil
	})

	server.AddTool(&mcpsdk.Tool{
		Name: "imagegen_register_style",
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: "Successfully registered style"},
			},
		}, nil
	})

	server.AddTool(&mcpsdk.Tool{
		Name: "imagegen_generate",
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: "Successfully generated image and saved to: " + generatedImagePath},
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

func TestGenerateCharacterSeed_Success(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()
	config.Cfg = &config.Config{
		MCP: config.MCPConfig{
			CharacterBackend: "imagen",
			ImageGenPath:     "pw-mcp-imagegen",
			CloudPath:        "pw-mcp-cloud",
		},
	}

	tmpDir := t.TempDir()

	// Write dummy source image
	dummySourceImage := filepath.Join(tmpDir, "source.png")
	if writeErr := os.WriteFile(dummySourceImage, []byte("fake-image-bytes"), 0600); writeErr != nil {
		t.Fatalf("failed to write source image: %v", writeErr)
	}

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Pessimistic Turtle",
		TargetPageCount: 1,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}

	m.BookProperties.CharacterProfile = "A cute little pessimistic turtle"
	if err = m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServerWithOutputType(t, ctx, serverTransport, dummySourceImage, "mock", true, true, "image")
	defer cleanupMCP()

	cloudClientTransport, cloudCleanup := setupMockCloudTransport(t, ctx, "http://example.com/character_reference_portrait.png")
	defer cloudCleanup()

	optsChar := CharacterOptions{
		OutputDir:         tmpDir,
		MCPTransport:      clientTransport,
		CloudMCPTransport: cloudClientTransport,
	}

	err = GenerateCharacterSeed(ctx, optsChar)
	if err != nil {
		t.Fatalf("GenerateCharacterSeed failed: %v", err)
	}

	// Verify manifest updates
	reloaded, err := manifest.LoadManifest(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}

	if reloaded.BookProperties.CharacterReferenceURL != "http://example.com/character_reference_portrait.png" {
		t.Errorf("expected CharacterReferenceURL 'http://example.com/character_reference_portrait.png', got %q", reloaded.BookProperties.CharacterReferenceURL)
	}

	if _, statErr := os.Stat(filepath.Join(tmpDir, "images", "character_seed.png")); os.IsNotExist(statErr) {
		t.Error("expected character_seed.png to be created, but it is missing")
	}
}

func TestGenerateCharacterSeed_VideoFail(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()
	config.Cfg = &config.Config{
		MCP: config.MCPConfig{
			CharacterBackend: "veo",
			ImageGenPath:     "pw-mcp-imagegen",
			CloudPath:        "pw-mcp-cloud",
		},
	}

	tmpDir := t.TempDir()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Pessimistic Turtle",
		TargetPageCount: 1,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}

	m.BookProperties.CharacterProfile = "A cute little pessimistic turtle"
	if err = m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServerWithOutputType(t, ctx, serverTransport, "", "mock", true, true, "video")
	defer cleanupMCP()

	optsChar := CharacterOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
	}

	err = GenerateCharacterSeed(ctx, optsChar)
	if err == nil {
		t.Fatal("expected GenerateCharacterSeed to fail for video output type, got nil")
	}

	if !strings.Contains(err.Error(), "character seed portrait must be still image") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGenerateCharacterSeed_Errors(t *testing.T) {
	ctx := context.Background()

	// 1. Empty OutputDir
	err := GenerateCharacterSeed(ctx, CharacterOptions{OutputDir: ""})
	if err == nil {
		t.Error("expected error for empty output directory, got nil")
	}

	// 2. Missing manifest file
	err = GenerateCharacterSeed(ctx, CharacterOptions{OutputDir: "/nonexistent-path-for-manifest"})
	if err == nil {
		t.Error("expected error for non-existent manifest file, got nil")
	}

	// 3. Character profile is empty
	tmpDir := t.TempDir()
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Turtle Theme",
		TargetPageCount: 1,
	}
	_, err = Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	err = GenerateCharacterSeed(ctx, CharacterOptions{OutputDir: tmpDir})
	if err == nil {
		t.Error("expected error when character profile is empty, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "character profile is empty in manifest") {
		t.Errorf("unexpected error message: %v", err)
	}
}

type errorTransport struct{}

func (t *errorTransport) Connect(ctx context.Context) (mcpsdk.Connection, error) {
	return nil, fmt.Errorf("mock connection error")
}

func TestBootstrapCharacterReference_CloudStartError(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()
	config.Cfg = &config.Config{
		MCP: config.MCPConfig{
			CharacterBackend: "imagen",
			ImageGenPath:     "pw-mcp-imagegen",
			CloudPath:        "pw-mcp-cloud",
		},
	}

	tmpDir := t.TempDir()

	// Write dummy source image
	dummySourceImage := filepath.Join(tmpDir, "source.png")
	if writeErr := os.WriteFile(dummySourceImage, []byte("fake-image-bytes"), 0600); writeErr != nil {
		t.Fatalf("failed to write source image: %v", writeErr)
	}

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Pessimistic Turtle",
		TargetPageCount: 1,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}

	m.BookProperties.CharacterProfile = "A cute little pessimistic turtle"
	if err = m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServerWithOutputType(t, ctx, serverTransport, dummySourceImage, "mock", true, true, "image")
	defer cleanupMCP()

	optsChar := CharacterOptions{
		OutputDir:         tmpDir,
		MCPTransport:      clientTransport,
		CloudMCPTransport: &errorTransport{},
	}

	err = GenerateCharacterSeed(ctx, optsChar)
	if err == nil {
		t.Fatal("expected GenerateCharacterSeed to fail on cloud client start error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to start MCP cloud client") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestBootstrapCharacterReference_CloudUploadError(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()
	config.Cfg = &config.Config{
		MCP: config.MCPConfig{
			CharacterBackend: "imagen",
			ImageGenPath:     "pw-mcp-imagegen",
			CloudPath:        "pw-mcp-cloud",
		},
	}

	tmpDir := t.TempDir()

	// Write dummy source image
	dummySourceImage := filepath.Join(tmpDir, "source.png")
	if writeErr := os.WriteFile(dummySourceImage, []byte("fake-image-bytes"), 0600); writeErr != nil {
		t.Fatalf("failed to write source image: %v", writeErr)
	}

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Pessimistic Turtle",
		TargetPageCount: 1,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}

	m.BookProperties.CharacterProfile = "A cute little pessimistic turtle"
	if err = m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServerWithOutputType(t, ctx, serverTransport, dummySourceImage, "mock", true, true, "image")
	defer cleanupMCP()

	// Create a cloud mock that returns error on upload
	cloudClientTransport, cloudServerTransport := mcpsdk.NewInMemoryTransports()
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "mock-cloud-server",
		Version: "1.0.0",
	}, nil)

	server.AddTool(&mcpsdk.Tool{
		Name: "cloud_upload_file",
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{
			IsError: true,
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: "failed to upload: bucket is locked"},
			},
		}, nil
	})

	serverSession, err := server.Connect(ctx, cloudServerTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = serverSession.Close() }()

	optsChar := CharacterOptions{
		OutputDir:         tmpDir,
		MCPTransport:      clientTransport,
		CloudMCPTransport: cloudClientTransport,
	}

	err = GenerateCharacterSeed(ctx, optsChar)
	if err == nil {
		t.Fatal("expected GenerateCharacterSeed to fail on cloud upload error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to upload character seed portrait") {
		t.Errorf("unexpected error message: %v", err)
	}
}

type reusableTransport struct {
	t                  *testing.T
	ctx                context.Context
	generatedImagePath string
	backend            string
	supportsCref       bool
	supportsSref       bool
	outputType         string
}

func (rt *reusableTransport) Connect(ctx context.Context) (mcpsdk.Connection, error) {
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	// setupMockImageGenServerWithOutputType starts a server and registers it to close.
	// To avoid leaking, we can keep the session but it gets closed when the client stops anyway.
	_, _ = setupMockImageGenServerWithOutputType(rt.t, rt.ctx, serverTransport, rt.generatedImagePath, rt.backend, rt.supportsCref, rt.supportsSref, rt.outputType)
	return clientTransport.Connect(ctx)
}

func TestBrew_DualBackend_Success(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()
	config.Cfg = &config.Config{
		MCP: config.MCPConfig{
			CharacterBackend:    "imagen",
			IllustrationBackend: "openai",
			ImageGenPath:        "pw-mcp-imagegen",
			CloudPath:           "pw-mcp-cloud",
		},
	}

	tmpDir := t.TempDir()

	// Write dummy source image
	dummySourceImage := filepath.Join(tmpDir, "source.png")
	if writeErr := os.WriteFile(dummySourceImage, []byte("fake-image-bytes"), 0600); writeErr != nil {
		t.Fatalf("failed to write source image: %v", writeErr)
	}

	// Initialize book project
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Turtle Theme",
		TargetPageCount: 1,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	m.BookProperties.CharacterProfile = "A cynical turtle"
	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusPending, Text: "Stanza 1"},
	}
	if err = m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rt := &reusableTransport{
		t:                  t,
		ctx:                ctx,
		generatedImagePath: dummySourceImage,
		backend:            "mock",
		supportsCref:       true,
		supportsSref:       true,
		outputType:         "image",
	}

	cloudClientTransport, cloudCleanup := setupMockCloudTransport(t, ctx, "http://example.com/character_reference.png")
	defer cloudCleanup()

	optsBrew := BrewOptions{
		OutputDir:         tmpDir,
		MCPTransport:      rt,
		CloudMCPTransport: cloudClientTransport,
	}

	err = Brew(ctx, optsBrew)
	if err != nil {
		t.Fatalf("Brew failed: %v", err)
	}

	reloaded, err := manifest.LoadManifest(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}

	if reloaded.BookProperties.CharacterReferenceURL != "http://example.com/character_reference.png" {
		t.Errorf("expected CharacterReferenceURL, got %q", reloaded.BookProperties.CharacterReferenceURL)
	}

	if reloaded.Progress.Pages[0].Status != manifest.StatusCompleted {
		t.Errorf("expected page 1 status Completed, got %q", reloaded.Progress.Pages[0].Status)
	}
}
