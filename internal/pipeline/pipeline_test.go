package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type mockLLM struct {
	stanzas []string
	err     error
}

func (m *mockLLM) GenerateStanzas(ctx context.Context, theme string, count int) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.stanzas, nil
}

func setupMockImageGenServer(t *testing.T, ctx context.Context, serverTransport mcpsdk.Transport, generatedImagePath string) (*mcpsdk.ServerSession, func()) {
	t.Helper()
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "mock-imagegen-server",
		Version: "1.0.0",
	}, nil)

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
		// Verify if we should fail or return path
		var args struct {
			Prompt string `json:"prompt"`
		}
		_ = json.Unmarshal(req.Params.Arguments, &args)

		if strings.Contains(args.Prompt, "FAIL_GENERATION") {
			return &mcpsdk.CallToolResult{
				IsError: true,
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: "failed to generate image: mock error"},
				},
			}, nil
		}

		if strings.Contains(args.Prompt, "FAIL_FORMAT") {
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: "bad response format"},
				},
			}, nil
		}

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

func TestInitiate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-initiate-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	opts := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Parody Theme",
		Style:           "water color",
		Format:          "paperback",
		TargetPageCount: 10,
	}

	m, err := Initiate(opts)
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}

	// Verify folders
	if _, statErr := os.Stat(filepath.Join(tmpDir, "images")); os.IsNotExist(statErr) {
		t.Error("images subdirectory does not exist")
	}
	if _, statErr := os.Stat(filepath.Join(tmpDir, "release")); os.IsNotExist(statErr) {
		t.Error("release subdirectory does not exist")
	}
	if _, statErr := os.Stat(filepath.Join(tmpDir, "manifest.json")); os.IsNotExist(statErr) {
		t.Error("manifest.json does not exist")
	}

	// Verify manifest values
	if m.BookProperties.Theme != "Parody Theme" {
		t.Errorf("expected Theme 'Parody Theme', got %q", m.BookProperties.Theme)
	}
	if m.BookProperties.TargetPageCount != 10 {
		t.Errorf("expected TargetPageCount 10, got %d", m.BookProperties.TargetPageCount)
	}
}

func TestInitiate_Errors(t *testing.T) {
	// Empty OutputDir
	_, err := Initiate(InitiateOptions{})
	if err == nil {
		t.Error("expected error for empty output directory, got nil")
	}

	// Error creating OutputDir
	tmpDir, err := os.MkdirTemp("", "pithos-initiate-err-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a file at OutputDir to force MkdirAll to fail
	dummyFile := filepath.Join(tmpDir, "dummy")
	if wErr := os.WriteFile(dummyFile, []byte(""), 0600); wErr != nil {
		t.Fatalf("failed to write dummy file: %v", wErr)
	}

	_, err = Initiate(InitiateOptions{OutputDir: filepath.Join(dummyFile, "book")})
	if err == nil {
		t.Error("expected error when directory creation fails, got nil")
	}

	// Default page count test
	m, err := Initiate(InitiateOptions{OutputDir: filepath.Join(tmpDir, "default-pages"), TargetPageCount: 0})
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}
	if m.BookProperties.TargetPageCount != 15 {
		t.Errorf("expected default page count to be 15, got %d", m.BookProperties.TargetPageCount)
	}
}

func TestBrew_EndToEnd_Mocked(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-brew-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Write dummy image to be copied
	dummySourceImage := filepath.Join(tmpDir, "source.png")
	if writeErr := os.WriteFile(dummySourceImage, []byte("fake-image-bytes"), 0600); writeErr != nil {
		t.Fatalf("failed to write source image: %v", writeErr)
	}

	// Setup pipeline initiate
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Original Theme",
		Style:           "original-style --sref http://example.com/original.png",
		TargetPageCount: 2,
	}
	_, err = Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}

	// Mock LLM & MCP Transport
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySourceImage)
	defer cleanupMCP()

	mockStanzas := []string{"Stanza 1 text", "Stanza 2 text"}
	mockLLMClient := &mockLLM{stanzas: mockStanzas}

	// Run Brew
	optsBrew := BrewOptions{
		OutputDir:    tmpDir,
		Theme:        "Overridden Theme",
		Style:        "new-style --sref http://example.com/new.png",
		MCPTransport: clientTransport,
		LLM:          mockLLMClient,
	}

	err = Brew(ctx, optsBrew)
	if err != nil {
		t.Fatalf("Brew failed: %v", err)
	}

	// Verify manifest updates
	m, err := manifest.LoadManifest(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	if m.BookProperties.Theme != "Overridden Theme" {
		t.Errorf("expected theme override 'Overridden Theme', got %q", m.BookProperties.Theme)
	}
	if m.BookProperties.Style != "new-style --sref http://example.com/new.png" {
		t.Errorf("expected style override, got %q", m.BookProperties.Style)
	}
	if !m.Progress.ManuscriptGenerated {
		t.Error("expected ManuscriptGenerated to be true")
	}

	if len(m.Progress.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(m.Progress.Pages))
	}

	page1 := m.Progress.Pages[0]
	if page1.Status != manifest.StatusCompleted {
		t.Errorf("expected page 1 status Completed, got %q", page1.Status)
	}
	if page1.ImagePath != "images/page_1.png" {
		t.Errorf("expected page 1 ImagePath 'images/page_1.png', got %q", page1.ImagePath)
	}

	// Verify that files were copied
	if _, statErr := os.Stat(filepath.Join(tmpDir, "images", "page_1.png")); os.IsNotExist(statErr) {
		t.Error("page_1.png image file does not exist")
	}
}

func TestBrew_Errors(t *testing.T) {
	// 1. Missing OutputDir
	err := Brew(context.Background(), BrewOptions{})
	if err == nil {
		t.Error("expected error for empty OutputDir, got nil")
	}

	// 2. Non-existent manifest
	err = Brew(context.Background(), BrewOptions{OutputDir: "/nonexistent-dir"})
	if err == nil {
		t.Error("expected error loading non-existent manifest, got nil")
	}

	tmpDir, err := os.MkdirTemp("", "pithos-brew-errs-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// 3. Manuscript generation LLM error
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Theme",
		TargetPageCount: 3,
	}
	_, err = Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	mockLLMError := &mockLLM{err: errors.New("LLM outage")}
	err = Brew(context.Background(), BrewOptions{OutputDir: tmpDir, LLM: mockLLMError})
	if err == nil {
		t.Error("expected error when LLM fails, got nil")
	}

	// 4. Missing API keys when no mock is set
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()
	config.Cfg = &config.Config{
		API: config.APIConfig{
			GeminiKey: "",
			OpenAIKey: "",
		},
	}
	err = Brew(context.Background(), BrewOptions{OutputDir: tmpDir})
	if err == nil {
		t.Error("expected error when no API keys are provided and no mock is set, got nil")
	}
}

func TestBrew_ResumabilityAndCheckpoints(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-brew-resume-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Write source image
	dummySourceImage := filepath.Join(tmpDir, "source.png")
	if writeErr := os.WriteFile(dummySourceImage, []byte("image"), 0600); writeErr != nil {
		t.Fatalf("failed to write source image: %v", writeErr)
	}

	// Setup pipeline initiate
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Resume Theme",
		TargetPageCount: 3,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	// Pre-populate manuscript to skip LLM step
	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusCompleted, ImagePath: "images/page_1.png", Text: "Stanza 1"},
		{PageIndex: 2, Status: manifest.StatusPending, Text: "FAIL_GENERATION: Stanza 2"},
		{PageIndex: 3, Status: manifest.StatusPending, Text: "Stanza 3"},
	}
	_ = m.Save()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySourceImage)
	defer cleanupMCP()

	// Run Brew - should fail at page 2, but page 1 should remain complete, page 2 status should remain pending/saved, and page 3 untouched
	optsBrew := BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
	}

	err = Brew(ctx, optsBrew)
	if err == nil {
		t.Error("expected Brew to return error on imagegen failure for page 2, got nil")
	}

	// Load manifest and check checkpoints
	m2, err := manifest.LoadManifest(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}

	if m2.Progress.Pages[0].Status != manifest.StatusCompleted {
		t.Errorf("expected page 1 to remain Completed, got %q", m2.Progress.Pages[0].Status)
	}
	if m2.Progress.Pages[1].Status != manifest.StatusPending {
		t.Errorf("expected page 2 to remain Pending after failure, got %q", m2.Progress.Pages[1].Status)
	}
	if m2.Progress.Pages[2].Status != manifest.StatusPending {
		t.Errorf("expected page 3 to remain Pending (not reached), got %q", m2.Progress.Pages[2].Status)
	}

	// Fix page 2 and rerun Brew
	m2.Progress.Pages[1].Text = "Success Stanza 2"
	_ = m2.Save()

	// Recreate transports and context for next run
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	clientTransport2, serverTransport2 := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP2 := setupMockImageGenServer(t, ctx2, serverTransport2, dummySourceImage)
	defer cleanupMCP2()

	optsBrew2 := BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport2,
	}

	err = Brew(ctx2, optsBrew2)
	if err != nil {
		t.Fatalf("expected Brew to succeed on rerun, got %v", err)
	}

	// Verify all pages are now Completed
	m3, err := manifest.LoadManifest(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}

	for _, p := range m3.Progress.Pages {
		if p.Status != manifest.StatusCompleted {
			t.Errorf("expected page %d to be Completed, got %q", p.PageIndex, p.Status)
		}
	}
}

func TestBrew_MCPFormatError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-brew-format-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Theme",
		TargetPageCount: 1,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusPending, Text: "FAIL_FORMAT: Stanza 1"},
	}
	_ = m.Save()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, "")
	defer cleanupMCP()

	optsBrew := BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
	}

	err = Brew(ctx, optsBrew)
	if err == nil {
		t.Error("expected error for bad MCP output format, got nil")
	}
}

func TestBrew_CopyFileError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-brew-copy-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Theme",
		TargetPageCount: 1,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusPending, Text: "Stanza 1"},
	}
	_ = m.Save()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	// Pass non-existent path to trigger copyFile failure
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, "/nonexistent/source.png")
	defer cleanupMCP()

	optsBrew := BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
	}

	err = Brew(ctx, optsBrew)
	if err == nil {
		t.Error("expected error when copying file fails, got nil")
	}
}

func TestLLMProviderSelection_Gemini(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	tmpDir, err := os.MkdirTemp("", "pithos-llm-select-gem-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Theme",
		TargetPageCount: 1,
	}
	_, err = Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	config.Cfg = &config.Config{
		API: config.APIConfig{
			GeminiKey: "gemini-key",
			OpenAIKey: "",
		},
		MCP: config.MCPConfig{
			ImageGenPath: "pw-mcp-imagegen",
		},
	}

	geminiMockResp := geminiResponse{
		Candidates: []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		}{
			{
				Content: struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				}{
					Parts: []struct {
						Text string `json:"text"`
					}{
						{Text: `{"stanzas": ["Stanza 1"]}`},
					},
				},
			},
		},
	}

	origTransport := http.DefaultTransport
	defer func() { http.DefaultTransport = origTransport }()

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Host, "generativelanguage") {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(geminiMockResp)
			return w.Result(), nil
		}
		return nil, fmt.Errorf("unexpected request to: %s", req.URL)
	})

	ctx := context.Background()
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	dummySource := filepath.Join(tmpDir, "source.png")
	_ = os.WriteFile(dummySource, []byte("dummy"), 0600)
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySource)
	defer cleanupMCP()

	optsBrew := BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
	}

	err = Brew(ctx, optsBrew)
	if err != nil {
		t.Fatalf("expected Gemini-based Brew to succeed, got %v", err)
	}
}

func TestLLMProviderSelection_OpenAI(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	tmpDir, err := os.MkdirTemp("", "pithos-llm-select-oa-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Theme",
		TargetPageCount: 1,
	}
	_, err = Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	config.Cfg = &config.Config{
		API: config.APIConfig{
			GeminiKey: "",
			OpenAIKey: "openai-key",
		},
		MCP: config.MCPConfig{
			ImageGenPath: "pw-mcp-imagegen",
		},
	}

	openAIMockResp := openAIResponse{
		Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			{
				Message: struct {
					Content string `json:"content"`
				}{
					Content: `{"stanzas": ["Stanza OpenAI 1"]}`,
				},
			},
		},
	}

	origTransport := http.DefaultTransport
	defer func() { http.DefaultTransport = origTransport }()

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Host, "api.openai.com") {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(openAIMockResp)
			return w.Result(), nil
		}
		return nil, fmt.Errorf("unexpected request to: %s", req.URL)
	})

	ctx := context.Background()
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	dummySource := filepath.Join(tmpDir, "source.png")
	_ = os.WriteFile(dummySource, []byte("dummy"), 0600)
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySource)
	defer cleanupMCP()

	optsBrew := BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
	}

	err = Brew(ctx, optsBrew)
	if err != nil {
		t.Fatalf("expected OpenAI-based Brew to succeed, got %v", err)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestInitiate_SubdirectoriesFailure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-sub-err-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// 1. Fail imagesDir creation by creating a file named "images"
	err = os.MkdirAll(filepath.Join(tmpDir, "case1"), 0750)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(tmpDir, "case1", "images"), []byte(""), 0600)
	opts1 := InitiateOptions{
		OutputDir:       filepath.Join(tmpDir, "case1"),
		Theme:           "Theme",
		TargetPageCount: 1,
	}
	_, err = Initiate(opts1)
	if err == nil {
		t.Error("expected error when images directory cannot be created, got nil")
	}

	// 2. Fail releaseDir creation by creating a file named "release"
	err = os.MkdirAll(filepath.Join(tmpDir, "case2"), 0750)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(tmpDir, "case2", "release"), []byte(""), 0600)
	opts2 := InitiateOptions{
		OutputDir:       filepath.Join(tmpDir, "case2"),
		Theme:           "Theme",
		TargetPageCount: 1,
	}
	_, err = Initiate(opts2)
	if err == nil {
		t.Error("expected error when release directory cannot be created, got nil")
	}
}

func TestBrew_CopyFileDestDirectoryFailure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-copy-dest-err-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Write dummy image to copy
	dummySource := filepath.Join(tmpDir, "source.png")
	_ = os.WriteFile(dummySource, []byte("fake"), 0600)

	// We make output/images/page_1.png a directory instead of a file
	destDir := filepath.Join(tmpDir, "images")
	err = os.MkdirAll(destDir, 0750)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(filepath.Join(destDir, "page_1.png"), 0750) // directory
	if err != nil {
		t.Fatal(err)
	}

	// Setup manifest
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Theme",
		TargetPageCount: 1,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}
	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusPending, Text: "Stanza 1"},
	}
	_ = m.Save()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySource)
	defer cleanupMCP()

	optsBrew := BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
	}

	err = Brew(ctx, optsBrew)
	if err == nil {
		t.Error("expected error because destination is a directory, got nil")
	}
}

func TestBrew_CopyFileMkdirAllFailure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-copy-mkdir-err-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Write dummy image
	dummySource := filepath.Join(tmpDir, "source.png")
	_ = os.WriteFile(dummySource, []byte("fake"), 0600)

	// Make "images" a file instead of a directory inside tmpDir
	_ = os.WriteFile(filepath.Join(tmpDir, "images"), []byte(""), 0600)

	// Create a manifest directly (since Initiate will fail due to images file)
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m := manifest.NewManifest(manifestPath)
	m.BookProperties.Theme = "Theme"
	m.BookProperties.TargetPageCount = 1
	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusPending, Text: "Stanza 1"},
	}
	_ = m.Save()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySource)
	defer cleanupMCP()

	optsBrew := BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
	}

	err = Brew(ctx, optsBrew)
	if err == nil {
		t.Error("expected error because images is a file and cannot create directory, got nil")
	}
}

func TestBrew_NoIllustrationsNeeded(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-no-ill-needed-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Theme",
		TargetPageCount: 1,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusCompleted, ImagePath: "images/page_1.png", Text: "Stanza 1"},
	}
	_ = m.Save()

	ctx := context.Background()
	optsBrew := BrewOptions{
		OutputDir: tmpDir,
	}
	err = Brew(ctx, optsBrew)
	if err != nil {
		t.Fatalf("expected Brew to short-circuit and succeed, got %v", err)
	}
}
