package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/powerword/pkg/telemetry"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type mockLLM struct {
	stanzas []string
	prompts []string
	usage   telemetry.TokenUsage
	err     error
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount        int `json:"promptTokenCount"`
		CandidatesTokenCount    int `json:"candidatesTokenCount"`
		CachedContentTokenCount int `json:"cachedContentTokenCount"`
	} `json:"usageMetadata"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (m *mockLLM) GenerateStanzas(ctx context.Context, theme string, count int) ([]string, []string, telemetry.TokenUsage, error) {
	if m.err != nil {
		return nil, nil, telemetry.TokenUsage{}, m.err
	}
	prompts := m.prompts
	if len(prompts) == 0 && len(m.stanzas) > 0 {
		prompts = make([]string, len(m.stanzas))
		for i := range prompts {
			prompts[i] = fmt.Sprintf("Illustration prompt for stanza %d", i+1)
		}
	}
	return m.stanzas, prompts, m.usage, nil
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

		if strings.Contains(args.Prompt, "SLEEP") {
			time.Sleep(100 * time.Millisecond)
		}

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
	mockLLMClient := &mockLLM{
		stanzas: mockStanzas,
		usage: telemetry.TokenUsage{
			InputTokens:  1000,
			OutputTokens: 2000,
			CachedTokens: 500,
		},
	}

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

	// Verify telemetry updates
	if m.Telemetry.ImageGenerations != 2 {
		t.Errorf("expected 2 image generations, got %d", m.Telemetry.ImageGenerations)
	}
	mu := m.Telemetry.ModelUsages["unknown"]
	if mu == nil {
		t.Error("expected unknown model usages telemetry to exist")
	} else if mu.InputTokens != 1000 || mu.OutputTokens != 2000 || mu.CachedTokens != 500 {
		t.Errorf("unexpected token usage: %+v", mu)
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

	// 5. Stanzas count mismatch
	mockLLMMismatch := &mockLLM{stanzas: []string{"Only 1 stanza"}}
	err = Brew(context.Background(), BrewOptions{OutputDir: tmpDir, LLM: mockLLMMismatch})
	if err == nil {
		t.Error("expected error when LLM returns incorrect number of stanzas, got nil")
	} else if !strings.Contains(err.Error(), "LLM generated 1 stanzas, but target page count is 3") {
		t.Errorf("unexpected error message: %v", err)
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
	m, err := Initiate(InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Resume Theme",
		TargetPageCount: 3,
	})
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
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySourceImage)
	defer cleanupMCP()

	// Run Brew - should fail at page 2, but page 1 should remain complete, page 2 status should remain pending/saved, and page 3 untouched
	err = Brew(ctx, BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
	})
	if err == nil {
		t.Error("expected Brew to return error on imagegen failure for page 2, got nil")
	}

	// Load manifest and check checkpoints
	m2, err := manifest.LoadManifest(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}

	expectedStatuses := []manifest.PageStatus{manifest.StatusCompleted, manifest.StatusPending, manifest.StatusCompleted}
	for idx, expected := range expectedStatuses {
		if m2.Progress.Pages[idx].Status != expected {
			t.Errorf("expected page %d to be %q, got %q", idx+1, expected, m2.Progress.Pages[idx].Status)
		}
	}

	// Fix page 2 and rerun Brew
	m2.Progress.Pages[1].Text = "Success Stanza 2"
	if saveErr := m2.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	// Recreate transports and context for next run
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	clientTransport2, serverTransport2 := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP2 := setupMockImageGenServer(t, ctx2, serverTransport2, dummySourceImage)
	defer cleanupMCP2()

	err = Brew(ctx2, BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport2,
	})
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
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

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
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

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

//nolint:funlen // Gemini client setup and payload parsing test is inherently long
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
						{Text: `{"stanzas": ["Stanza 1"], "illustration_prompts": ["Prompt 1"]}`},
					},
				},
			},
		},
	}
	geminiMockResp.UsageMetadata = &struct {
		PromptTokenCount        int `json:"promptTokenCount"`
		CandidatesTokenCount    int `json:"candidatesTokenCount"`
		CachedContentTokenCount int `json:"cachedContentTokenCount"`
	}{
		PromptTokenCount:        100,
		CandidatesTokenCount:    200,
		CachedContentTokenCount: 50,
	}

	mockHttpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Host, "generativelanguage") {
				w := httptest.NewRecorder()
				w.Header().Set("Content-Type", "application/json")
				// Note: The official google/generative-ai-go/genai SDK's chat.SendMessage method
				// internally calls the streaming endpoint (:streamGenerateContent), which expects
				// the response to be wrapped in a JSON array of response chunks. Therefore, we
				// must encode it as a slice here to match the SDK's transport expectations.
				_ = json.NewEncoder(w).Encode([]geminiResponse{geminiMockResp})
				return w.Result(), nil
			}
			return nil, fmt.Errorf("unexpected request to: %s", req.URL)
		}),
	}

	ctx := context.Background()
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	dummySource := filepath.Join(tmpDir, "source.png")
	_ = os.WriteFile(dummySource, []byte("dummy"), 0600)
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySource)
	defer cleanupMCP()

	optsBrew := BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
		HTTPClient:   mockHttpClient,
	}

	err = Brew(ctx, optsBrew)
	if err != nil {
		t.Fatalf("expected Gemini-based Brew to succeed, got %v", err)
	}

	m, err := manifest.LoadManifest(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	mu := m.Telemetry.ModelUsages["gemini-2.5-flash"]
	if mu == nil {
		t.Fatal("expected gemini-2.5-flash model usages telemetry to exist")
	}
	if mu.InputTokens != 100 {
		t.Errorf("expected InputTokens 100, got %d", mu.InputTokens)
	}
	if mu.OutputTokens != 200 {
		t.Errorf("expected OutputTokens 200, got %d", mu.OutputTokens)
	}
	expectedCached := 50
	if mu.CachedTokens != expectedCached {
		t.Errorf("expected CachedTokens %d, got %d", expectedCached, mu.CachedTokens)
	}
}

//nolint:funlen // OpenAI client setup and payload parsing test is inherently long
func TestLLMProviderSelection_OpenAI(t *testing.T) {
	origBaseURL := os.Getenv("OPENAI_BASE_URL")
	os.Setenv("OPENAI_BASE_URL", "")
	defer os.Setenv("OPENAI_BASE_URL", origBaseURL)

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
					Content: `{"stanzas": ["Stanza OpenAI 1"], "illustration_prompts": ["Prompt OpenAI 1"]}`,
				},
			},
		},
	}
	openAIMockResp.Usage = &struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	}{
		PromptTokens:     150,
		CompletionTokens: 250,
		TotalTokens:      400,
	}

	mockHttpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Host, "api.openai.com") {
				w := httptest.NewRecorder()
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(openAIMockResp)
				return w.Result(), nil
			}
			return nil, fmt.Errorf("unexpected request to: %s", req.URL)
		}),
	}

	ctx := context.Background()
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	dummySource := filepath.Join(tmpDir, "source.png")
	_ = os.WriteFile(dummySource, []byte("dummy"), 0600)
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySource)
	defer cleanupMCP()

	optsBrew := BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
		HTTPClient:   mockHttpClient,
	}

	err = Brew(ctx, optsBrew)
	if err != nil {
		t.Fatalf("expected OpenAI-based Brew to succeed, got %v", err)
	}

	m, err := manifest.LoadManifest(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	mu := m.Telemetry.ModelUsages["gpt-4o"]
	if mu == nil {
		t.Fatal("expected gpt-4o model usages telemetry to exist")
	}
	if mu.InputTokens != 150 {
		t.Errorf("expected InputTokens 150, got %d", mu.InputTokens)
	}
	if mu.OutputTokens != 250 {
		t.Errorf("expected OutputTokens 250, got %d", mu.OutputTokens)
	}
	// The powerword OpenAI client does not populate cached tokens, so we explicitly expect 0.
	expectedCached := 0
	if mu.CachedTokens != expectedCached {
		t.Errorf("expected CachedTokens %d, got %d", expectedCached, mu.CachedTokens)
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
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

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
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

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
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	ctx := context.Background()
	optsBrew := BrewOptions{
		OutputDir: tmpDir,
	}
	err = Brew(ctx, optsBrew)
	if err != nil {
		t.Fatalf("expected Brew to short-circuit and succeed, got %v", err)
	}
}

func TestBrew_TelemetryCustomPricing(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	tmpDir, err := os.MkdirTemp("", "pithos-telemetry-pricing-*")
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
		Theme:           "Telemetry Custom Pricing",
		TargetPageCount: 2,
	}
	_, err = Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}

	// Configure custom pricing
	config.Cfg = &config.Config{
		MCP: config.MCPConfig{
			ImageGenPath: "pw-mcp-imagegen",
		},
		Pricing: map[string]telemetry.ModelPricing{
			"unknown":  {Input: 1.00, Output: 2.00, Cached: 0.50},
			"imagegen": {Input: 100000.00}, // $0.10 per image
		},
	}

	// Mock LLM & MCP Transport
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySourceImage)
	defer cleanupMCP()

	mockStanzas := []string{"Stanza 1 text", "Stanza 2 text"}
	mockLLMClient := &mockLLM{
		stanzas: mockStanzas,
		usage: telemetry.TokenUsage{
			InputTokens:  1000,
			OutputTokens: 2000,
			CachedTokens: 500,
		},
	}

	optsBrew := BrewOptions{
		OutputDir:    tmpDir,
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

	if m.Telemetry.ImageGenerations != 2 {
		t.Errorf("expected 2 image generations, got %d", m.Telemetry.ImageGenerations)
	}

	mu := m.Telemetry.ModelUsages["unknown"]
	if mu == nil {
		t.Error("expected unknown model usages telemetry to exist")
	} else if mu.InputTokens != 1000 || mu.OutputTokens != 2000 || mu.CachedTokens != 500 {
		t.Errorf("unexpected token usage: %+v", mu)
	}

	// Expected total cost calculation:
	// LLM cost for unknown model:
	// input billed = (1000 - 500) = 500 tokens * $1.00 / 1M = $0.00050
	// output = 2000 tokens * $2.00 / 1M = $0.00400
	// cached = 500 tokens * $0.50 / 1M = $0.00025
	// total LLM = $0.00475
	// Imagegen cost: 2 images * $0.10 = $0.20
	// Total cost expected = $0.20475
	expectedCost := 0.20475
	if math.Abs(m.Telemetry.TotalCostUSD-expectedCost) > 1e-6 {
		t.Errorf("expected TotalCostUSD %f, got %f", expectedCost, m.Telemetry.TotalCostUSD)
	}
}

func TestBrew_Concurrency(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-brew-concurrency-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	dummySourceImage := filepath.Join(tmpDir, "source.png")
	if writeErr := os.WriteFile(dummySourceImage, []byte("image"), 0600); writeErr != nil {
		t.Fatalf("failed to write source image: %v", writeErr)
	}

	m, err := Initiate(InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Concurrency Theme",
		TargetPageCount: 4,
	})
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusPending, Text: "SLEEP: Stanza 1"},
		{PageIndex: 2, Status: manifest.StatusPending, Text: "SLEEP: Stanza 2"},
		{PageIndex: 3, Status: manifest.StatusPending, Text: "SLEEP: Stanza 3"},
		{PageIndex: 4, Status: manifest.StatusPending, Text: "SLEEP: Stanza 4"},
	}
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySourceImage)
	defer cleanupMCP()

	start := time.Now()
	err = Brew(ctx, BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
		Concurrency:  4,
	})
	if err != nil {
		t.Fatalf("Brew failed: %v", err)
	}
	elapsed := time.Since(start)

	// Since we sleep for 100ms for each SLEEP prompt, if they run in parallel, it should take less than 300ms.
	// If they ran sequentially, it would take at least 400ms.
	if elapsed >= 300*time.Millisecond {
		t.Errorf("expected parallel execution to take less than 300ms, took %v", elapsed)
	}

	// Verify all pages completed
	m2, err := manifest.LoadManifest(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}
	for _, p := range m2.Progress.Pages {
		if p.Status != manifest.StatusCompleted {
			t.Errorf("expected page %d to be completed, got %q", p.PageIndex, p.Status)
		}
	}
}

func TestBrew_ReviewFlow_Export(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-brew-review-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// 1. Initialize Pithos Workspace
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Review Flow Theme",
		TargetPageCount: 3,
	}
	_, err = Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}

	mockStanzas := []string{
		"Stanza 1 original",
		"Stanza 2 original",
		"Stanza 3 original",
	}
	mockLLMClient := &mockLLM{
		stanzas: mockStanzas,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	dummySourceImage := filepath.Join(tmpDir, "source.png")
	if writeErr := os.WriteFile(dummySourceImage, []byte("fake"), 0600); writeErr != nil {
		t.Fatalf("failed to write dummy source image: %v", writeErr)
	}
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySourceImage)
	defer cleanupMCP()

	// 2. Run Brew with Review = true (first run, should export and exit with error)
	optsBrewReview := BrewOptions{
		OutputDir:    tmpDir,
		Review:       true,
		MCPTransport: clientTransport,
		LLM:          mockLLMClient,
	}

	err = Brew(ctx, optsBrewReview)
	if err == nil {
		t.Fatal("expected Brew to return review pause error, got nil")
	}
	if !strings.Contains(err.Error(), "review mode active") {
		t.Errorf("unexpected error message: %v", err)
	}

	// Verify manuscript.md exists
	manuscriptPath := filepath.Join(tmpDir, "manuscript.md")
	if _, statErr := os.Stat(manuscriptPath); os.IsNotExist(statErr) {
		t.Fatal("expected manuscript.md to be exported, but it does not exist")
	}

	//nolint:gosec // manuscriptPath is constructed in temp test directory
	data, readErr := os.ReadFile(manuscriptPath)
	if readErr != nil {
		t.Fatalf("failed to read manuscript.md: %v", readErr)
	}
	content := string(data)
	if !strings.Contains(content, "# Page 1\n## Text\nStanza 1 original\n\n## Prompt\nIllustration prompt for stanza 1") ||
		!strings.Contains(content, "# Page 2\n## Text\nStanza 2 original\n\n## Prompt\nIllustration prompt for stanza 2") ||
		!strings.Contains(content, "# Page 3\n## Text\nStanza 3 original\n\n## Prompt\nIllustration prompt for stanza 3") {
		t.Errorf("manuscript.md has incorrect format: %s", content)
	}
}

func TestBrew_ReviewFlow_ImportAndSync(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-brew-resume-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Write source image to be copied by mock MCP
	dummySourceImage := filepath.Join(tmpDir, "source.png")
	if writeErr := os.WriteFile(dummySourceImage, []byte("fake-image-bytes"), 0600); writeErr != nil {
		t.Fatalf("failed to write source image: %v", writeErr)
	}

	// 1. Initialize Pithos Workspace
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Review Flow Theme",
		TargetPageCount: 3,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}

	// Pre-populate generated manuscript and images
	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusCompleted, ImagePath: "images/page_1.png", Text: "Stanza 1 original"},
		{PageIndex: 2, Status: manifest.StatusCompleted, ImagePath: "images/page_2.png", Text: "Stanza 2 original"},
		{PageIndex: 3, Status: manifest.StatusCompleted, ImagePath: "images/page_3.png", Text: "Stanza 3 original"},
	}
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	// Write manuscript.md with modified page 2 text
	manuscriptPath := filepath.Join(tmpDir, "manuscript.md")
	content := "<!-- PITHOS REVIEW -->\n# Page 1\nStanza 1 original\n\n# Page 2\nStanza 2 edited text\n\n# Page 3\nStanza 3 original\n"
	if writeErr := os.WriteFile(manuscriptPath, []byte(content), 0600); writeErr != nil {
		t.Fatalf("failed to edit manuscript.md: %v", writeErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySourceImage)
	defer cleanupMCP()

	// 2. Run Brew with Review = false (should import edits, reset page 2 status to pending, and regenerate images)
	optsBrewResume := BrewOptions{
		OutputDir:    tmpDir,
		Review:       false,
		MCPTransport: clientTransport,
	}

	err = Brew(ctx, optsBrewResume)
	if err != nil {
		t.Fatalf("expected Brew to succeed on resume/import run, got %v", err)
	}

	// 3. Verify final manifest state
	m2, err := manifest.LoadManifest(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}

	if m2.Progress.Pages[1].Text != "Stanza 2 edited text" {
		t.Errorf("expected page 2 text to be imported, got %q", m2.Progress.Pages[1].Text)
	}
	for _, p := range m2.Progress.Pages {
		if p.Status != manifest.StatusCompleted {
			t.Errorf("expected page %d status to be Completed, got %q", p.PageIndex, p.Status)
		}
		if p.ImagePath == "" {
			t.Errorf("expected page %d to have image path set", p.PageIndex)
		}
	}
}

func TestImportManuscriptFromMarkdown_Errors_Basic(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-import-errs-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	m := manifest.NewManifest(filepath.Join(tmpDir, "manifest.json"))
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Text: "Stanza 1"},
	}

	// 1. manuscript.md does not exist
	changed, err := importManuscriptFromMarkdown(tmpDir, m)
	if err != nil {
		t.Fatalf("expected no error for non-existent manuscript, got %v", err)
	}
	if changed {
		t.Error("expected changed to be false for non-existent file")
	}

	// 2. Empty manuscript file
	manuscriptPath := filepath.Join(tmpDir, "manuscript.md")
	if writeErr := os.WriteFile(manuscriptPath, []byte(""), 0600); writeErr != nil {
		t.Fatalf("failed to write empty manuscript: %v", writeErr)
	}
	_, err = importManuscriptFromMarkdown(tmpDir, m)
	if err == nil {
		t.Error("expected error for empty manuscript file, got nil")
	}

	// 3. Invalid page header format
	invalidHeaders := "<!-- review -->\n# Page A\nStanza A"
	if writeErr := os.WriteFile(manuscriptPath, []byte(invalidHeaders), 0600); writeErr != nil {
		t.Fatalf("failed to write invalid headers: %v", writeErr)
	}
	_, err = importManuscriptFromMarkdown(tmpDir, m)
	if err == nil {
		t.Error("expected error for invalid page index format, got nil")
	}

	// 3b. Header with trailing extra text (e.g. # Page 1 (draft))
	trailingHeaders := "<!-- review -->\n# Page 1 (draft)\nStanza 1"
	if writeErr := os.WriteFile(manuscriptPath, []byte(trailingHeaders), 0600); writeErr != nil {
		t.Fatalf("failed to write trailing headers: %v", writeErr)
	}
	_, err = importManuscriptFromMarkdown(tmpDir, m)
	if err == nil {
		t.Error("expected error for header with trailing text, got nil")
	}
}

func TestImportManuscriptFromMarkdown_Errors_Validation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-import-errs-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	m := manifest.NewManifest(filepath.Join(tmpDir, "manifest.json"))
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Text: "Stanza 1"},
	}
	manuscriptPath := filepath.Join(tmpDir, "manuscript.md")

	// 4. Page index mismatch
	mismatchPage := "# Page 5\nStanza 5"
	if writeErr := os.WriteFile(manuscriptPath, []byte(mismatchPage), 0600); writeErr != nil {
		t.Fatalf("failed to write mismatch page: %v", writeErr)
	}
	_, err = importManuscriptFromMarkdown(tmpDir, m)
	if err == nil {
		t.Error("expected error for page index mismatch, got nil")
	}

	// 5. Non-positive page index
	nonPositivePage := "<!-- review -->\n# Page 0\nStanza 0"
	if writeErr := os.WriteFile(manuscriptPath, []byte(nonPositivePage), 0600); writeErr != nil {
		t.Fatalf("failed to write non-positive page: %v", writeErr)
	}
	_, err = importManuscriptFromMarkdown(tmpDir, m)
	if err == nil {
		t.Error("expected error for non-positive page index, got nil")
	}

	// 6. Duplicate page index
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Text: "Stanza 1"},
		{PageIndex: 2, Text: "Stanza 2"},
	}
	duplicatePage := "<!-- review -->\n# Page 1\nStanza 1\n\n# Page 1\nStanza 1 copy"
	if writeErr := os.WriteFile(manuscriptPath, []byte(duplicatePage), 0600); writeErr != nil {
		t.Fatalf("failed to write duplicate page manuscript: %v", writeErr)
	}
	_, err = importManuscriptFromMarkdown(tmpDir, m)
	if err == nil {
		t.Error("expected error for duplicate page index, got nil")
	}

	// 7. Missing manifest page in manuscript (mismatch in stanza count)
	missingPage := "<!-- review -->\n# Page 1\nStanza 1"
	if writeErr := os.WriteFile(manuscriptPath, []byte(missingPage), 0600); writeErr != nil {
		t.Fatalf("failed to write missing page manuscript: %v", writeErr)
	}
	_, err = importManuscriptFromMarkdown(tmpDir, m)
	if err == nil {
		t.Error("expected error for missing page stanza, got nil")
	}

	// 8. Partial new-format subheaders (## Text present but ## Prompt missing)
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Text: "Stanza 1"},
	}
	partialHeaders := "<!-- review -->\n# Page 1\n## Text\nSome text but no prompt header"
	if writeErr := os.WriteFile(manuscriptPath, []byte(partialHeaders), 0600); writeErr != nil {
		t.Fatalf("failed to write partial headers manuscript: %v", writeErr)
	}
	_, err = importManuscriptFromMarkdown(tmpDir, m)
	if err == nil {
		t.Error("expected error for page with ## Text but missing ## Prompt, got nil")
	}
}

func TestImportManuscriptFromMarkdown_DoubleSubheaders(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-import-double-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	m := manifest.NewManifest(filepath.Join(tmpDir, "manifest.json"))
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Text: "Stanza 1 original", IllustrationPrompt: "Prompt 1 original", Status: manifest.StatusCompleted, ImagePath: "images/page_1.png"},
		{PageIndex: 2, Text: "Stanza 2 original", IllustrationPrompt: "Prompt 2 original", Status: manifest.StatusCompleted, ImagePath: "images/page_2.png"},
	}

	manuscriptPath := filepath.Join(tmpDir, "manuscript.md")

	// Test 1: Write manuscript with modified text in double subheader format
	content := `<!-- review -->
# Page 1
## Text
Stanza 1 edited text

## Prompt
Prompt 1 original

# Page 2
## Text
Stanza 2 original

## Prompt
Prompt 2 edited prompt
`
	if writeErr := os.WriteFile(manuscriptPath, []byte(content), 0600); writeErr != nil {
		t.Fatalf("failed to write manuscript.md: %v", writeErr)
	}

	changed, err := importManuscriptFromMarkdown(tmpDir, m)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !changed {
		t.Error("expected changed to be true")
	}

	if m.Progress.Pages[0].Text != "Stanza 1 edited text" {
		t.Errorf("expected page 1 text to be edited, got %q", m.Progress.Pages[0].Text)
	}
	if m.Progress.Pages[0].Status != manifest.StatusPending || m.Progress.Pages[0].ImagePath != "" {
		t.Errorf("expected page 1 status to be reset, got status %q path %q", m.Progress.Pages[0].Status, m.Progress.Pages[0].ImagePath)
	}

	if m.Progress.Pages[1].IllustrationPrompt != "Prompt 2 edited prompt" {
		t.Errorf("expected page 2 prompt to be edited, got %q", m.Progress.Pages[1].IllustrationPrompt)
	}
	if m.Progress.Pages[1].Status != manifest.StatusPending || m.Progress.Pages[1].ImagePath != "" {
		t.Errorf("expected page 2 status to be reset, got status %q path %q", m.Progress.Pages[1].Status, m.Progress.Pages[1].ImagePath)
	}

	// Test 2: Fallback behavior for legacy manuscripts
	m.Progress.Pages[0].Status = manifest.StatusCompleted
	m.Progress.Pages[0].ImagePath = "images/page_1.png"
	m.Progress.Pages[1].Status = manifest.StatusCompleted
	m.Progress.Pages[1].ImagePath = "images/page_2.png"

	legacyContent := `<!-- review -->
# Page 1
Stanza 1 legacy edited

# Page 2
Stanza 2 original
`
	if writeErr := os.WriteFile(manuscriptPath, []byte(legacyContent), 0600); writeErr != nil {
		t.Fatalf("failed to write manuscript.md: %v", writeErr)
	}

	changed, err = importManuscriptFromMarkdown(tmpDir, m)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !changed {
		t.Error("expected changed to be true")
	}

	if m.Progress.Pages[0].Text != "Stanza 1 legacy edited" {
		t.Errorf("expected page 1 text to be legacy edited, got %q", m.Progress.Pages[0].Text)
	}
	if m.Progress.Pages[0].IllustrationPrompt != "Prompt 1 original" {
		t.Errorf("expected page 1 illustration prompt to be preserved, got %q", m.Progress.Pages[0].IllustrationPrompt)
	}
	if m.Progress.Pages[0].Status != manifest.StatusPending || m.Progress.Pages[0].ImagePath != "" {
		t.Errorf("expected page 1 status to be reset, got status %q path %q", m.Progress.Pages[0].Status, m.Progress.Pages[0].ImagePath)
	}

	if m.Progress.Pages[1].Text != "Stanza 2 original" {
		t.Errorf("expected page 2 text to remain same, got %q", m.Progress.Pages[1].Text)
	}
	if m.Progress.Pages[1].IllustrationPrompt != "Prompt 2 edited prompt" {
		t.Errorf("expected page 2 illustration prompt to be preserved, got %q", m.Progress.Pages[1].IllustrationPrompt)
	}
	if m.Progress.Pages[1].Status != manifest.StatusCompleted {
		t.Errorf("expected page 2 status to remain completed, got status %q", m.Progress.Pages[1].Status)
	}
}
