package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/powerword/pkg/telemetry"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type mockLLM struct {
	stanzas     []string
	prompts     []string
	style       string
	charProfile string
	usage       telemetry.TokenUsage
	err         error
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

func (m *mockLLM) GenerateVisualGuides(ctx context.Context, theme string) (string, string, telemetry.TokenUsage, error) {
	if m.err != nil {
		return "", "", telemetry.TokenUsage{}, m.err
	}
	style := m.style
	if style == "" {
		style = "mock-style"
	}
	charProfile := m.charProfile
	if charProfile == "" {
		charProfile = "mock-char-profile"
	}
	return style, charProfile, m.usage, nil
}

func (m *mockLLM) GenerateStanzas(ctx context.Context, theme string, count int, style string, characterProfile string) ([]string, []string, telemetry.TokenUsage, error) {
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

func (m *mockLLM) Ping(ctx context.Context) error {
	return m.err
}

var (
	mockActiveCount    int32
	mockMaxActiveCount int32
)

func setupMockImageGenServer(t *testing.T, ctx context.Context, serverTransport mcpsdk.Transport, generatedImagePath string) (*mcpsdk.ServerSession, func()) {
	return setupMockImageGenServerWithCapabilities(t, ctx, serverTransport, generatedImagePath, "mock", true, true)
}

func setupMockImageGenServerWithCapabilities(t *testing.T, ctx context.Context, serverTransport mcpsdk.Transport, generatedImagePath string, backend string, supportsCref, supportsSref bool) (*mcpsdk.ServerSession, func()) {
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
		capsJSON := fmt.Sprintf(`{"backend":%q,"supports_cref":%t,"supports_sref":%t,"output_type":"image"}`, backend, supportsCref, supportsSref)
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
		// Verify if we should fail or return path
		var args struct {
			Prompt string `json:"prompt"`
		}
		_ = json.Unmarshal(req.Params.Arguments, &args)

		if strings.Contains(args.Prompt, "SLEEP") {
			atomic.AddInt32(&mockActiveCount, 1)
			defer atomic.AddInt32(&mockActiveCount, -1)
			for {
				currMax := atomic.LoadInt32(&mockMaxActiveCount)
				currActive := atomic.LoadInt32(&mockActiveCount)
				if currActive <= currMax {
					break
				}
				if atomic.CompareAndSwapInt32(&mockMaxActiveCount, currMax, currActive) {
					break
				}
			}
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

func setupMockCloudTransport(t *testing.T, ctx context.Context, returnedURL string) (mcpsdk.Transport, func()) {
	t.Helper()
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
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
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: returnedURL},
			},
		}, nil
	})

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}

	return clientTransport, func() {
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
		TrimSize:        "6x9",
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
	if m.BookProperties.TrimSize != "6x9" {
		t.Errorf("expected TrimSize '6x9', got %q", m.BookProperties.TrimSize)
	}
	if m.Kiln.Version != 1 {
		t.Errorf("expected Kiln.Version 1, got %d", m.Kiln.Version)
	}
	if len(m.Kiln.Milestones) != 1 || m.Kiln.Milestones[0] != "initiate_complete" {
		t.Errorf("expected Kiln.Milestones [initiate_complete], got %v", m.Kiln.Milestones)
	}

	// Test default TrimSize
	optsDefault := InitiateOptions{
		OutputDir:       t.TempDir(),
		Theme:           "Default Trim",
		TargetPageCount: 10,
	}
	mDefault, err := Initiate(optsDefault)
	if err != nil {
		t.Fatalf("Default Initiate failed: %v", err)
	}
	if mDefault.BookProperties.TrimSize != "8.5x8.5" {
		t.Errorf("expected default TrimSize '8.5x8.5', got %q", mDefault.BookProperties.TrimSize)
	}
}

func TestInitiate_Brainstorm_Enabled(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	// Configure valid config so getLLMClient might fallback if needed, but we pass mock LLM client.
	config.Cfg = &config.Config{
		API: config.APIConfig{
			GeminiKey: "fake-key",
		},
	}

	tmpDir := t.TempDir()
	mockClient := &mockLLM{
		style:       "cosmic space style",
		charProfile: "a cute little astronaut dog",
	}
	opts := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Space Dog Adventure",
		TargetPageCount: 15,
		NoBrainstorm:    false,
		LLM:             mockClient,
	}
	m, err := Initiate(opts)
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}
	if m.BookProperties.Style != "cosmic space style" {
		t.Errorf("expected style 'cosmic space style', got %q", m.BookProperties.Style)
	}
	if m.BookProperties.CharacterProfile != "a cute little astronaut dog" {
		t.Errorf("expected character profile 'a cute little astronaut dog', got %q", m.BookProperties.CharacterProfile)
	}
}

func TestInitiate_Brainstorm_Disabled(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	// Configure valid config so getLLMClient might fallback if needed, but we pass mock LLM client.
	config.Cfg = &config.Config{
		API: config.APIConfig{
			GeminiKey: "fake-key",
		},
	}

	tmpDir := t.TempDir()
	mockClient := &mockLLM{
		style:       "cosmic space style",
		charProfile: "a cute little astronaut dog",
	}
	opts := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Space Dog Adventure",
		TargetPageCount: 15,
		NoBrainstorm:    true,
		LLM:             mockClient,
	}
	m, err := Initiate(opts)
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}
	if m.BookProperties.Style != "" {
		t.Errorf("expected style to be empty, got %q", m.BookProperties.Style)
	}
	if m.BookProperties.CharacterProfile != "" {
		t.Errorf("expected character profile to be empty, got %q", m.BookProperties.CharacterProfile)
	}
}

func TestInitiate_Brainstorm_NoTheme(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	// Configure valid config so getLLMClient might fallback if needed, but we pass mock LLM client.
	config.Cfg = &config.Config{
		API: config.APIConfig{
			GeminiKey: "fake-key",
		},
	}

	tmpDir := t.TempDir()
	mockClient := &mockLLM{
		style:       "cosmic space style",
		charProfile: "a cute little astronaut dog",
	}
	opts := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "",
		TargetPageCount: 15,
		NoBrainstorm:    false,
		LLM:             mockClient,
	}
	m, err := Initiate(opts)
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}
	if m.BookProperties.Style != "" {
		t.Errorf("expected style to be empty, got %q", m.BookProperties.Style)
	}
	if m.BookProperties.CharacterProfile != "" {
		t.Errorf("expected character profile to be empty, got %q", m.BookProperties.CharacterProfile)
	}
}

func TestInitiate_Brainstorm_NoAPIKeys(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	config.Cfg = &config.Config{
		API: config.APIConfig{
			GeminiKey: "",
			OpenAIKey: "",
		},
	}

	tmpDir := t.TempDir()
	opts := InitiateOptions{
		OutputDir:           tmpDir,
		Theme:               "Space Theme",
		TargetPageCount:     15,
		NoBrainstorm:        false,
		StrictBrainstorming: true,
		LLM:                 nil, // Force standard client setup to fail
	}
	_, err := Initiate(opts)
	if err == nil {
		t.Fatal("expected Initiate to fail due to missing API keys, got nil")
	}
	if !strings.Contains(err.Error(), "--no-brainstorm") {
		t.Errorf("expected error message to suggest --no-brainstorm, got: %v", err)
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

//nolint:funlen,gocognit // End-to-end mocked brew verification involves extensive mock setup and output checking
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
	mInit, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}
	mInit.BookProperties.CharacterProfile = "mock-character-profile"
	if err = mInit.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	// Mock LLM & MCP Transport
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySourceImage)
	defer cleanupMCP()

	cloudClientTransport, cloudCleanup := setupMockCloudTransport(t, ctx, "http://example.com/uploaded_character.png")
	defer cloudCleanup()

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
		OutputDir:         tmpDir,
		Theme:             "Overridden Theme",
		Style:             "new-style --sref http://example.com/new.png",
		MCPTransport:      clientTransport,
		CloudMCPTransport: cloudClientTransport,
		LLM:               mockLLMClient,
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
	if m.BookProperties.CharacterReferenceURL != "http://example.com/uploaded_character.png" {
		t.Errorf("expected character reference URL 'http://example.com/uploaded_character.png', got %q", m.BookProperties.CharacterReferenceURL)
	}
	if _, statErr := os.Stat(filepath.Join(tmpDir, "images", "character_seed.png")); os.IsNotExist(statErr) {
		t.Error("expected character_seed.png to be created, but it is missing")
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

	// Verify Kiln milestones
	if len(m.Kiln.Milestones) != 2 || m.Kiln.Milestones[0] != "initiate_complete" || m.Kiln.Milestones[1] != "brew_complete" {
		t.Errorf("expected milestones [initiate_complete, brew_complete], got %v", m.Kiln.Milestones)
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
	mInit, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}
	mInit.BookProperties.Style = "mock-style"
	mInit.BookProperties.CharacterProfile = "mock-character-profile"
	mInit.BookProperties.CharacterReferenceURL = "http://example.com/character.png"
	if err = mInit.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
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
				if encErr := json.NewEncoder(w).Encode([]geminiResponse{geminiMockResp}); encErr != nil {
					return nil, encErr
				}
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
	t.Setenv("OPENAI_BASE_URL", "")

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
	mInit, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}
	mInit.BookProperties.Style = "mock-style"
	mInit.BookProperties.CharacterProfile = "mock-character-profile"
	mInit.BookProperties.CharacterReferenceURL = "http://example.com/character.png"
	if err = mInit.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
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
				if encErr := json.NewEncoder(w).Encode(openAIMockResp); encErr != nil {
					return nil, encErr
				}
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
	mInit, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}
	mInit.BookProperties.Style = "mock-style"
	mInit.BookProperties.CharacterProfile = "mock-character-profile"
	mInit.BookProperties.CharacterReferenceURL = "http://example.com/character.png"
	if err = mInit.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
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

	oldIsTTY := isTTY
	isTTY = func() bool { return true }
	defer func() { isTTY = oldIsTTY }()

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

	atomic.StoreInt32(&mockMaxActiveCount, 0)
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

	// Verify concurrent execution occurred by checking maximum active concurrent requests
	maxActive := atomic.LoadInt32(&mockMaxActiveCount)
	if maxActive <= 1 {
		t.Errorf("expected concurrent execution (max active requests > 1), got max active: %d", maxActive)
	}

	// Since we sleep for 100ms for each SLEEP prompt, if they run in parallel, it should take less than 300ms under normal conditions.
	// However, under throttled CI/CD test runners, scheduling overhead can exceed 300ms. We use a relaxed threshold of 1500ms to avoid flakes.
	if elapsed >= 1500*time.Millisecond {
		t.Errorf("expected parallel execution to take less than 1500ms, took %v", elapsed)
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
	if !strings.Contains(content, "# Page 1") || !strings.Contains(content, "## Text\nStanza 1 original\n\n## Prompt\nIllustration prompt for stanza 1") ||
		!strings.Contains(content, "# Page 2") || !strings.Contains(content, "## Text\nStanza 2 original\n\n## Prompt\nIllustration prompt for stanza 2") ||
		!strings.Contains(content, "# Page 3") || !strings.Contains(content, "## Text\nStanza 3 original\n\n## Prompt\nIllustration prompt for stanza 3") {
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
	changed, err := importManuscriptFromMarkdown(tmpDir, m, nil)
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
	_, err = importManuscriptFromMarkdown(tmpDir, m, nil)
	if err == nil {
		t.Error("expected error for empty manuscript file, got nil")
	}

	// 3. Invalid page header format
	invalidHeaders := "<!-- review -->\n# Page A\nStanza A"
	if writeErr := os.WriteFile(manuscriptPath, []byte(invalidHeaders), 0600); writeErr != nil {
		t.Fatalf("failed to write invalid headers: %v", writeErr)
	}
	_, err = importManuscriptFromMarkdown(tmpDir, m, nil)
	if err == nil {
		t.Error("expected error for invalid page index format, got nil")
	}

	// 3b. Header with trailing extra text (e.g. # Page 1 (draft))
	trailingHeaders := "<!-- review -->\n# Page 1 (draft)\nStanza 1"
	if writeErr := os.WriteFile(manuscriptPath, []byte(trailingHeaders), 0600); writeErr != nil {
		t.Fatalf("failed to write trailing headers: %v", writeErr)
	}
	_, err = importManuscriptFromMarkdown(tmpDir, m, nil)
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
	_, err = importManuscriptFromMarkdown(tmpDir, m, nil)
	if err == nil {
		t.Error("expected error for page index mismatch, got nil")
	}

	// 5. Non-positive page index
	nonPositivePage := "<!-- review -->\n# Page 0\nStanza 0"
	if writeErr := os.WriteFile(manuscriptPath, []byte(nonPositivePage), 0600); writeErr != nil {
		t.Fatalf("failed to write non-positive page: %v", writeErr)
	}
	_, err = importManuscriptFromMarkdown(tmpDir, m, nil)
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
	_, err = importManuscriptFromMarkdown(tmpDir, m, nil)
	if err == nil {
		t.Error("expected error for duplicate page index, got nil")
	}

	// 7. Missing manifest page in manuscript (mismatch in stanza count)
	missingPage := "<!-- review -->\n# Page 1\nStanza 1"
	if writeErr := os.WriteFile(manuscriptPath, []byte(missingPage), 0600); writeErr != nil {
		t.Fatalf("failed to write missing page manuscript: %v", writeErr)
	}
	_, err = importManuscriptFromMarkdown(tmpDir, m, nil)
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
	_, err = importManuscriptFromMarkdown(tmpDir, m, nil)
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
		{PageIndex: 1, Text: "Stanza 1 original", IllustrationPrompt: "Prompt 1 original", Status: manifest.StatusCompleted, ImagePath: "images/page_1.png", Layout: "full-bleed"},
		{PageIndex: 2, Text: "Stanza 2 original", IllustrationPrompt: "Prompt 2 original", Status: manifest.StatusCompleted, ImagePath: "images/page_2.png", Layout: "full-bleed"},
	}

	manuscriptPath := filepath.Join(tmpDir, "manuscript.md")

	// Test 1: Write manuscript with modified text in double subheader format
	content := `<!-- review -->
# Page 1
<!-- Layout: facing-pages -->
## Text
Stanza 1 edited text

## Prompt
Prompt 1 original

# Page 2
<!-- Layout: facing-pages-flipped -->
## Text
Stanza 2 original

## Prompt
Prompt 2 edited prompt
`
	if writeErr := os.WriteFile(manuscriptPath, []byte(content), 0600); writeErr != nil {
		t.Fatalf("failed to write manuscript.md: %v", writeErr)
	}

	changed, err := importManuscriptFromMarkdown(tmpDir, m, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !changed {
		t.Error("expected changed to be true")
	}

	if m.Progress.Pages[0].Text != "Stanza 1 edited text" {
		t.Errorf("expected page 1 text to be edited, got %q", m.Progress.Pages[0].Text)
	}
	if m.Progress.Pages[0].Layout != "facing-pages" {
		t.Errorf("expected page 1 layout to be facing-pages, got %q", m.Progress.Pages[0].Layout)
	}
	if m.Progress.Pages[0].Status != manifest.StatusPending || m.Progress.Pages[0].ImagePath != "images/page_1.png" {
		t.Errorf("expected page 1 status to be reset, got status %q path %q", m.Progress.Pages[0].Status, m.Progress.Pages[0].ImagePath)
	}

	if m.Progress.Pages[1].IllustrationPrompt != "Prompt 2 edited prompt" {
		t.Errorf("expected page 2 prompt to be edited, got %q", m.Progress.Pages[1].IllustrationPrompt)
	}
	if m.Progress.Pages[1].Layout != "facing-pages-flipped" {
		t.Errorf("expected page 2 layout to be facing-pages-flipped, got %q", m.Progress.Pages[1].Layout)
	}
	if m.Progress.Pages[1].Status != manifest.StatusPending || m.Progress.Pages[1].ImagePath != "images/page_2.png" {
		t.Errorf("expected page 2 status to be reset, got status %q path %q", m.Progress.Pages[1].Status, m.Progress.Pages[1].ImagePath)
	}
}

func TestImportManuscriptFromMarkdown_LegacyFallback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-import-legacy-*")
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

	legacyContent := `<!-- review -->
# Page 1
Stanza 1 legacy edited

# Page 2
Stanza 2 original
`
	if writeErr := os.WriteFile(manuscriptPath, []byte(legacyContent), 0600); writeErr != nil {
		t.Fatalf("failed to write manuscript.md: %v", writeErr)
	}

	changed, err := importManuscriptFromMarkdown(tmpDir, m, nil)
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
	if m.Progress.Pages[0].Status != manifest.StatusPending || m.Progress.Pages[0].ImagePath != "images/page_1.png" {
		t.Errorf("expected page 1 status to be reset, got status %q path %q", m.Progress.Pages[0].Status, m.Progress.Pages[0].ImagePath)
	}

	if m.Progress.Pages[1].Text != "Stanza 2 original" {
		t.Errorf("expected page 2 text to remain same, got %q", m.Progress.Pages[1].Text)
	}
	if m.Progress.Pages[1].IllustrationPrompt != "Prompt 2 original" {
		t.Errorf("expected page 2 illustration prompt to be preserved, got %q", m.Progress.Pages[1].IllustrationPrompt)
	}
	if m.Progress.Pages[1].Status != manifest.StatusCompleted {
		t.Errorf("expected page 2 status to remain completed, got status %q", m.Progress.Pages[1].Status)
	}
}

func setupSelectiveRedoTest(t *testing.T, theme string) (string, *manifest.Manifest, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "pithos-brew-selective-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	dummySourceImage := filepath.Join(tmpDir, "source.png")
	if writeErr := os.WriteFile(dummySourceImage, []byte("fake-image-bytes"), 0600); writeErr != nil {
		t.Fatalf("failed to write source image: %v", writeErr)
	}
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           theme,
		TargetPageCount: 3,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}
	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusCompleted, ImagePath: "images/page_1.png", Text: "Stanza 1"},
		{PageIndex: 2, Status: manifest.StatusCompleted, ImagePath: "images/page_2.png", Text: "Stanza 2"},
		{PageIndex: 3, Status: manifest.StatusCompleted, ImagePath: "images/page_3.png", Text: "Stanza 3"},
	}
	if err := m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}
	return tmpDir, m, dummySourceImage
}

func TestBrew_SelectivePageRedo(t *testing.T) {
	tmpDir, _, dummySourceImage := setupSelectiveRedoTest(t, "Selective Redo Theme")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySourceImage)
	defer cleanupMCP()

	optsBrew := BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
		Pages:        []int{2},
	}

	imagesDir := filepath.Join(tmpDir, "images")
	if mkdirErr := os.MkdirAll(imagesDir, 0750); mkdirErr != nil {
		t.Fatalf("failed to create images dir: %v", mkdirErr)
	}

	page1ImgPath := filepath.Join(imagesDir, "page_1.png")
	page2ImgPath := filepath.Join(imagesDir, "page_2.png")
	page3ImgPath := filepath.Join(imagesDir, "page_3.png")

	if writeErr1 := os.WriteFile(page1ImgPath, []byte("original-image-1"), 0600); writeErr1 != nil {
		t.Fatalf("failed to write page 1 image: %v", writeErr1)
	}
	if writeErr2 := os.WriteFile(page2ImgPath, []byte("old-image-data"), 0600); writeErr2 != nil {
		t.Fatalf("failed to write page 2 image: %v", writeErr2)
	}
	if writeErr3 := os.WriteFile(page3ImgPath, []byte("original-image-3"), 0600); writeErr3 != nil {
		t.Fatalf("failed to write page 3 image: %v", writeErr3)
	}

	if brewErr := Brew(ctx, optsBrew); brewErr != nil {
		t.Fatalf("Brew failed: %v", brewErr)
	}

	m2, loadErr := manifest.LoadManifest(filepath.Join(tmpDir, "manifest.json"))
	if loadErr != nil {
		t.Fatalf("failed to load manifest: %v", loadErr)
	}

	if m2.Progress.Pages[1].Status != manifest.StatusCompleted {
		t.Errorf("expected Page 2 to be Completed, got %q", m2.Progress.Pages[1].Status)
	}
	if m2.Progress.Pages[0].Status != manifest.StatusCompleted || m2.Progress.Pages[2].Status != manifest.StatusCompleted {
		t.Errorf("expected Page 1 and 3 to remain Completed")
	}

	// #nosec G304
	content, readErr := os.ReadFile(page2ImgPath)
	if readErr != nil {
		t.Fatalf("failed to read page 2 image: %v", readErr)
	}
	if string(content) != "fake-image-bytes" {
		t.Errorf("expected page 2 image content to be overwritten, got %q", string(content))
	}

	// Verify non-target pages remain unchanged
	// #nosec G304
	content1, readErr1 := os.ReadFile(page1ImgPath)
	if readErr1 != nil {
		t.Fatalf("failed to read page 1 image: %v", readErr1)
	}
	if string(content1) != "original-image-1" {
		t.Errorf("expected page 1 image content to remain unchanged, got %q", string(content1))
	}

	// #nosec G304
	content3, readErr3 := os.ReadFile(page3ImgPath)
	if readErr3 != nil {
		t.Fatalf("failed to read page 3 image: %v", readErr3)
	}
	if string(content3) != "original-image-3" {
		t.Errorf("expected page 3 image content to remain unchanged, got %q", string(content3))
	}
}

func TestBrew_SelectivePageRedo_Errors(t *testing.T) {
	tmpDir, m, _ := setupSelectiveRedoTest(t, "Selective Redo Theme")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	optsBrewInvalidPage := BrewOptions{
		OutputDir: tmpDir,
		Pages:     []int{4},
	}
	if err := Brew(ctx, optsBrewInvalidPage); err == nil {
		t.Error("expected error for out of bounds page index, got nil")
	} else if !strings.Contains(err.Error(), "page index 4 is out of bounds") {
		t.Errorf("unexpected error: %v", err)
	}

	m.Progress.ManuscriptGenerated = false
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}
	optsBrewNoManuscript := BrewOptions{
		OutputDir: tmpDir,
		Pages:     []int{2},
	}
	if err := Brew(ctx, optsBrewNoManuscript); err == nil {
		t.Error("expected error when manuscript is not generated, got nil")
	} else if !strings.Contains(err.Error(), "cannot perform selective page redo before manuscript is generated") {
		t.Errorf("unexpected error: %v", err)
	}

	m.Progress.ManuscriptGenerated = true
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	manuscriptContent := `<!-- review -->
# Page 1
<!-- prompt: New Prompt for Page 1 -->
Stanza 1 Edited!

# Page 2
<!-- prompt: Stanza 2 prompt -->
Stanza 2

# Page 3
<!-- prompt: Stanza 3 prompt -->
Stanza 3
`
	if writeErr := os.WriteFile(filepath.Join(tmpDir, "manuscript.md"), []byte(manuscriptContent), 0600); writeErr != nil {
		t.Fatalf("failed to write manuscript.md: %v", writeErr)
	}

	optsBrewOutsideEdits := BrewOptions{
		OutputDir: tmpDir,
		Pages:     []int{2},
	}
	if err := Brew(ctx, optsBrewOutsideEdits); err == nil {
		t.Error("expected error when manuscript.md has edits outside selective page list, got nil")
	} else if !strings.Contains(err.Error(), "not included in the selective page override list") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBrew_InteractiveSelect(t *testing.T) {
	// 1. Conflicting options validation
	ctx := context.Background()
	optsBoth := BrewOptions{
		OutputDir: "/dummy",
		Pages:     []int{1},
		Select:    true,
	}
	err := Brew(ctx, optsBoth)
	if err == nil || !strings.Contains(err.Error(), "cannot specify both --pages and --select") {
		t.Errorf("expected error for both pages and select, got %v", err)
	}

	// 2. Non-TTY error
	oldIsTTY := isTTY
	isTTY = func() bool { return false }
	defer func() { isTTY = oldIsTTY }()

	optsSelectNonTTY := BrewOptions{
		OutputDir: "/dummy",
		Select:    true,
	}
	tmpDir := t.TempDir()
	m := manifest.NewManifest(filepath.Join(tmpDir, "manifest.json"))
	m.BookProperties.Theme = "Test Theme"
	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusCompleted, Text: "Page 1 Text"},
		{PageIndex: 2, Status: manifest.StatusCompleted, Text: "Page 2 Text"},
	}
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	optsSelectNonTTY.OutputDir = tmpDir
	err = Brew(ctx, optsSelectNonTTY)
	if err == nil || !strings.Contains(err.Error(), "interactive selection requires a TTY terminal") {
		t.Errorf("expected TTY error, got %v", err)
	}

	// 3. Successful select
	isTTY = func() bool { return true }
	inR, inW := io.Pipe()
	go func() {
		// Toggle page 2 (choice 2), then confirm (choice 0)
		_, _ = inW.Write([]byte("2\n"))
		_, _ = inW.Write([]byte("0\n"))
		_ = inW.Close()
	}()

	var buf strings.Builder
	optsSelectSuccess := BrewOptions{
		OutputDir: tmpDir,
		Select:    true,
		Review:    true,
		Silent:    true,
		In:        inR,
		Out:       &buf,
	}

	err = Brew(ctx, optsSelectSuccess)
	if err == nil || !errors.Is(err, ErrReviewPause) {
		t.Errorf("expected ErrReviewPause, got %v", err)
	}

	// Verify that page 2 status was indeed reset to pending in the manifest on disk!
	mLoaded, err := manifest.LoadManifest(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	if mLoaded.Progress.Pages[0].Status != manifest.StatusCompleted {
		t.Errorf("expected page 1 to remain completed, got %s", mLoaded.Progress.Pages[0].Status)
	}
	if mLoaded.Progress.Pages[1].Status != manifest.StatusPending {
		t.Errorf("expected page 2 to be reset to pending, got %s", mLoaded.Progress.Pages[1].Status)
	}
}

func TestBrew_ReviewFlow_GuidesExport(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-review-guides-export-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	m := manifest.NewManifest(filepath.Join(tmpDir, "manifest.json"))
	m.BookProperties = manifest.BookProperties{
		Theme:            "Test Theme",
		Style:            "claymation style",
		CharacterProfile: "A chubby cat",
		TargetPageCount:  2,
	}
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusCompleted, ImagePath: "images/page_1.png", Text: "Stanza 1 text", IllustrationPrompt: "Prompt 1", Layout: "facing-pages"},
		{PageIndex: 2, Status: manifest.StatusCompleted, ImagePath: "images/page_2.png", Text: "Stanza 2 text", IllustrationPrompt: "Prompt 2", Layout: "full-bleed"},
	}
	if err = m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	err = exportManuscriptToMarkdown(tmpDir, m.BookProperties.Style, m.BookProperties.CharacterProfile, m.Progress.Pages)
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	manuscriptPath := filepath.Join(tmpDir, "manuscript.md")
	//nolint:gosec
	data, err := os.ReadFile(manuscriptPath)
	if err != nil {
		t.Fatalf("failed to read manuscript.md: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "<!-- Style: claymation style -->") {
		t.Errorf("exported file missing Style comment: %s", content)
	}
	if !strings.Contains(content, "<!-- CharacterProfile: A chubby cat -->") {
		t.Errorf("exported file missing CharacterProfile comment: %s", content)
	}
	if !strings.Contains(content, "<!-- Layout: facing-pages -->") {
		t.Errorf("exported file missing Layout comment: %s", content)
	}
	if !strings.Contains(content, "<!-- Layout: full-bleed -->") {
		t.Errorf("exported file missing Layout comment: %s", content)
	}

	// Verify sanitization of comment terminators (-->) in Style and CharacterProfile
	unsanitizedStyle := "claymation --> style"
	unsanitizedProfile := "A chubby cat --> who wears glasses"
	err = exportManuscriptToMarkdown(tmpDir, unsanitizedStyle, unsanitizedProfile, m.Progress.Pages)
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	//nolint:gosec
	dataSanitized, err := os.ReadFile(manuscriptPath)
	if err != nil {
		t.Fatalf("failed to read manuscript.md: %v", err)
	}
	contentSanitized := string(dataSanitized)
	if !strings.Contains(contentSanitized, "<!-- Style: claymation -- style -->") {
		t.Errorf("Style comment was not sanitized correctly: %s", contentSanitized)
	}
	if !strings.Contains(contentSanitized, "<!-- CharacterProfile: A chubby cat -- who wears glasses -->") {
		t.Errorf("CharacterProfile comment was not sanitized correctly: %s", contentSanitized)
	}
}

func TestBrew_ReviewFlow_GuidesStyleReset(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-review-guides-style-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	m := manifest.NewManifest(filepath.Join(tmpDir, "manifest.json"))
	m.BookProperties = manifest.BookProperties{
		Theme:            "Test Theme",
		Style:            "claymation style",
		CharacterProfile: "A chubby cat",
		TargetPageCount:  2,
	}
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusCompleted, ImagePath: "images/page_1.png", Text: "Stanza 1 text", IllustrationPrompt: "Prompt 1"},
		{PageIndex: 2, Status: manifest.StatusCompleted, ImagePath: "images/page_2.png", Text: "Stanza 2 text", IllustrationPrompt: "Prompt 2"},
	}
	if err = m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	err = exportManuscriptToMarkdown(tmpDir, m.BookProperties.Style, m.BookProperties.CharacterProfile, m.Progress.Pages)
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	manuscriptPath := filepath.Join(tmpDir, "manuscript.md")
	//nolint:gosec
	data, err := os.ReadFile(manuscriptPath)
	if err != nil {
		t.Fatalf("failed to read manuscript.md: %v", err)
	}

	contentModified := strings.Replace(string(data), "<!-- Style: claymation style -->", "<!-- Style: sketch style -->", 1)
	//nolint:gosec
	if err = os.WriteFile(manuscriptPath, []byte(contentModified), 0600); err != nil {
		t.Fatalf("failed to write modified manuscript: %v", err)
	}

	changed, err := importManuscriptFromMarkdown(tmpDir, m, nil)
	if err != nil {
		t.Fatalf("failed to import modified manuscript: %v", err)
	}
	if !changed {
		t.Error("expected changed to be true after modifying style comment")
	}

	if m.BookProperties.Style != "sketch style" {
		t.Errorf("expected style in manifest to be updated to 'sketch style', got %q", m.BookProperties.Style)
	}
	for _, p := range m.Progress.Pages {
		if p.Status != manifest.StatusPending {
			t.Errorf("expected page %d status to be reset to pending, got %s", p.PageIndex, p.Status)
		}
		if p.ImagePath != "" {
			t.Errorf("expected page %d ImagePath to be cleared, got %q", p.PageIndex, p.ImagePath)
		}
	}
}

func TestBrew_ReviewFlow_GuidesCharacterProfileReset(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-review-guides-char-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	m := manifest.NewManifest(filepath.Join(tmpDir, "manifest.json"))
	m.BookProperties = manifest.BookProperties{
		Theme:            "Test Theme",
		Style:            "claymation style",
		CharacterProfile: "A chubby cat",
		TargetPageCount:  2,
	}
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusCompleted, ImagePath: "images/page_1.png", Text: "Stanza 1 text", IllustrationPrompt: "Prompt 1"},
		{PageIndex: 2, Status: manifest.StatusCompleted, ImagePath: "images/page_2.png", Text: "Stanza 2 text", IllustrationPrompt: "Prompt 2"},
	}
	if err = m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	err = exportManuscriptToMarkdown(tmpDir, m.BookProperties.Style, m.BookProperties.CharacterProfile, m.Progress.Pages)
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	manuscriptPath := filepath.Join(tmpDir, "manuscript.md")
	//nolint:gosec
	data, err := os.ReadFile(manuscriptPath)
	if err != nil {
		t.Fatalf("failed to read manuscript.md: %v", err)
	}

	contentModified := strings.Replace(string(data), "<!-- CharacterProfile: A chubby cat -->", "<!-- CharacterProfile: A skinny dog -->", 1)
	//nolint:gosec
	if err = os.WriteFile(manuscriptPath, []byte(contentModified), 0600); err != nil {
		t.Fatalf("failed to write modified manuscript: %v", err)
	}

	changed, err := importManuscriptFromMarkdown(tmpDir, m, nil)
	if err != nil {
		t.Fatalf("failed to import modified manuscript: %v", err)
	}
	if !changed {
		t.Error("expected changed to be true after modifying character profile comment")
	}

	if m.BookProperties.CharacterProfile != "A skinny dog" {
		t.Errorf("expected character profile in manifest to be updated to 'A skinny dog', got %q", m.BookProperties.CharacterProfile)
	}
	for _, p := range m.Progress.Pages {
		if p.Status != manifest.StatusPending {
			t.Errorf("expected page %d status to be reset to pending, got %s", p.PageIndex, p.Status)
		}
		if p.ImagePath != "" {
			t.Errorf("expected page %d ImagePath to be cleared, got %q", p.PageIndex, p.ImagePath)
		}
	}
}

func TestGetBestImageSize(t *testing.T) {
	tests := []struct {
		trimSize string
		expected string
	}{
		{"8.5x8.5", "1024x1024"},
		{"6x9", "1024x1792"},
		{"11x8.5", "1792x1024"},
		{"invalid", "1024x1024"},
		{"", "1024x1024"},
		{"0x0", "1024x1024"},
		{"-6x9", "1024x1024"},
		{"6x-9", "1024x1024"},
		{"abcxdef", "1024x1024"},
		{"6xinvalid", "1024x1024"},
	}

	for _, tc := range tests {
		actual := getBestImageSize(tc.trimSize)
		if actual != tc.expected {
			t.Errorf("getBestImageSize(%q) = %q; want %q", tc.trimSize, actual, tc.expected)
		}
	}
}

//nolint:gocognit,funlen
func TestBrew_CharacterInvariantInjection(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Write a dummy source image to be copied
	dummySourceImage := filepath.Join(tmpDir, "source.png")
	if err := os.WriteFile(dummySourceImage, []byte("fake-image-bytes"), 0600); err != nil {
		t.Fatalf("failed to write source image: %v", err)
	}

	// 2. Initiate book
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Invariant Test Theme",
		TargetPageCount: 2,
	}
	mInit, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	// Set character profile and default weight
	mInit.BookProperties.CharacterProfile = "mock-char-profile"
	mInit.BookProperties.CharacterWeight = 50

	// Set page 1 specific weight override to 75
	mInit.Progress.ManuscriptGenerated = true
	mInit.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusPending, Text: "Stanza 1 text", IllustrationPrompt: "Prompt 1"},
		{PageIndex: 2, Status: manifest.StatusPending, Text: "Stanza 2 text", IllustrationPrompt: "Prompt 2"},
	}
	wOverride := 75
	mInit.Progress.Pages[0].CharacterWeight = &wOverride

	if err = mInit.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 3. Setup mock ImageGen Server
	imageGenClientTransport, imageGenServerTransport := mcpsdk.NewInMemoryTransports()
	var requestsMu sync.Mutex
	var imageGenRequests []generateRequest

	imageGenServer, err := createMockImageGenServer(ctx, dummySourceImage, &imageGenRequests, &requestsMu)
	if err != nil {
		t.Fatalf("failed to create mock imagegen server: %v", err)
	}

	imageGenSession, err := imageGenServer.Connect(ctx, imageGenServerTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = imageGenSession.Close() }()

	// 4. Setup mock Cloud Server
	cloudClientTransport, cloudServerTransport := mcpsdk.NewInMemoryTransports()
	var uploadPaths []string
	cloudServer, err := createMockCloudServer(&uploadPaths, &requestsMu)
	if err != nil {
		t.Fatalf("failed to create mock cloud server: %v", err)
	}

	cloudSession, err := cloudServer.Connect(ctx, cloudServerTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cloudSession.Close() }()

	// 5. Run Brew
	opts := BrewOptions{
		OutputDir:         tmpDir,
		MCPTransport:      imageGenClientTransport,
		CloudMCPTransport: cloudClientTransport,
	}

	err = Brew(ctx, opts)
	if err != nil {
		t.Fatalf("Brew failed: %v", err)
	}

	// 6. Assertions
	m, err := manifest.LoadManifest(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	if m.BookProperties.CharacterReferenceURL != "http://example.com/character_seed.png" {
		t.Errorf("expected reference URL to be 'http://example.com/character_seed.png', got %q", m.BookProperties.CharacterReferenceURL)
	}

	// Verify character seed image path exists
	seedPath := filepath.Join(tmpDir, "images", "character_seed.png")
	if _, statErr := os.Stat(seedPath); os.IsNotExist(statErr) {
		t.Error("expected character seed image file to be created on disk")
	}

	// Verify cloud upload was called with absolute path
	requestsMu.Lock()
	defer requestsMu.Unlock()

	if len(uploadPaths) != 1 {
		t.Fatalf("expected 1 upload request, got %d", len(uploadPaths))
	}
	if !filepath.IsAbs(uploadPaths[0]) {
		t.Errorf("expected upload path to be absolute, got %q", uploadPaths[0])
	}
	if !strings.HasSuffix(uploadPaths[0], "character_seed.png") {
		t.Errorf("expected uploaded file name to be character_seed.png, got %q", uploadPaths[0])
	}

	// Verify imagegen requests had prepended prompts and correct weights
	// There should be 3 requests: 1 for character seed generation, 2 for pages
	if len(imageGenRequests) != 3 {
		t.Fatalf("expected 3 image generation requests (1 seed + 2 pages), got %d", len(imageGenRequests))
	}

	// First request is seed portrait: prompt should be "Detailed visual seed character portrait: mock-char-profile"
	// Cref should be empty, weight should be nil
	seedReq := imageGenRequests[0]
	if seedReq.Prompt != "Detailed visual seed character portrait: mock-char-profile" {
		t.Errorf("unexpected seed prompt: %q", seedReq.Prompt)
	}
	if seedReq.CrefURL != "" {
		t.Errorf("expected empty cref for seed generation, got %q", seedReq.CrefURL)
	}
	if seedReq.CharacterWeight != nil {
		t.Errorf("expected nil weight for seed generation, got %d", *seedReq.CharacterWeight)
	}

	// Second request is page 1: prompt prepended with character profile, cref set to uploaded URL, weight set to override (75)
	page1Req := imageGenRequests[1]
	if page1Req.Prompt != "mock-char-profile, Prompt 1" {
		t.Errorf("unexpected page 1 prompt: %q", page1Req.Prompt)
	}
	if page1Req.CrefURL != "http://example.com/character_seed.png" {
		t.Errorf("unexpected page 1 cref URL: %q", page1Req.CrefURL)
	}
	if page1Req.CharacterWeight == nil || *page1Req.CharacterWeight != 75 {
		t.Errorf("expected page 1 weight 75, got %v", page1Req.CharacterWeight)
	}

	// Third request is page 2: prompt prepended, cref set, weight set to global default (50)
	page2Req := imageGenRequests[2]
	if page2Req.Prompt != "mock-char-profile, Prompt 2" {
		t.Errorf("unexpected page 2 prompt: %q", page2Req.Prompt)
	}
	if page2Req.CrefURL != "http://example.com/character_seed.png" {
		t.Errorf("unexpected page 2 cref URL: %q", page2Req.CrefURL)
	}
	if page2Req.CharacterWeight == nil || *page2Req.CharacterWeight != 50 {
		t.Errorf("expected page 2 weight 50, got %v", page2Req.CharacterWeight)
	}
}

func TestGetUniqueOutputDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-test-unique-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	baseDir := filepath.Join(tmpDir, "book")

	// Case 1: Directory does not exist. Should return baseDir unchanged.
	got1 := GetUniqueOutputDir(baseDir)
	if got1 != baseDir {
		t.Errorf("expected %q, got %q", baseDir, got1)
	}

	// Case 2: Base directory exists. Should return baseDir-1.
	if err := os.Mkdir(baseDir, 0750); err != nil {
		t.Fatalf("failed to create baseDir: %v", err)
	}
	got2 := GetUniqueOutputDir(baseDir)
	want2 := baseDir + "-1"
	if got2 != want2 {
		t.Errorf("expected %q, got %q", want2, got2)
	}

	// Case 3: Base directory and baseDir-1 exist. Should return baseDir-2.
	if err := os.Mkdir(want2, 0750); err != nil {
		t.Fatalf("failed to create want2: %v", err)
	}
	got3 := GetUniqueOutputDir(baseDir)
	want3 := baseDir + "-2"
	if got3 != want3 {
		t.Errorf("expected %q, got %q", want3, got3)
	}
}

type errorReader struct{}

func (errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

func TestConfirmOverwrite(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"yes", "y\n", true},
		{"yes-long", "yes\n", true},
		{"yes-caps", "Y\n", true},
		{"yes-mixed", "Yes\n", true},
		{"yes-caps-long", "YES\n", true},
		{"no", "n\n", false},
		{"no-long", "no\n", false},
		{"empty", "\n", false},
		{"invalid", "maybe\n", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf strings.Builder
			r := strings.NewReader(tc.input)
			got, err := ConfirmOverwrite(r, &buf, "/some/path")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("for input %q, expected %t, got %t", tc.input, tc.expected, got)
			}
			out := buf.String()
			if !strings.Contains(out, "overwrite") {
				t.Errorf("expected output prompt to contain 'overwrite', got %q", out)
			}
		})
	}

	t.Run("read-error", func(t *testing.T) {
		var buf strings.Builder
		got, err := ConfirmOverwrite(errorReader{}, &buf, "/some/path")
		if err == nil {
			t.Error("expected error on read error, got nil")
		}
		if got {
			t.Error("expected false on read error, got true")
		}
	})
}

type generateRequest struct {
	Prompt          string `json:"prompt"`
	CrefURL         string `json:"cref_url"`
	CharacterWeight *int   `json:"character_weight"`
}

func createMockImageGenServer(ctx context.Context, dummySourceImage string, requests *[]generateRequest, requestsMu *sync.Mutex) (*mcpsdk.Server, error) {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "mock-imagegen-server",
		Version: "1.0.0",
	}, nil)

	server.AddTool(&mcpsdk.Tool{
		Name:        "imagegen_get_capabilities",
		InputSchema: map[string]any{"type": "object"},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: `{"backend":"mock","supports_cref":true,"supports_sref":true,"output_type":"image"}`},
			},
		}, nil
	})

	server.AddTool(&mcpsdk.Tool{
		Name:        "imagegen_register_style",
		InputSchema: map[string]any{"type": "object"},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "Successfully registered style"}},
		}, nil
	})

	server.AddTool(&mcpsdk.Tool{
		Name:        "imagegen_generate",
		InputSchema: map[string]any{"type": "object"},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		var args generateRequest
		if unmarshalErr := json.Unmarshal(req.Params.Arguments, &args); unmarshalErr != nil {
			return nil, unmarshalErr
		}
		requestsMu.Lock()
		*requests = append(*requests, args)
		requestsMu.Unlock()

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "Successfully generated image and saved to: " + dummySourceImage}},
		}, nil
	})
	return server, nil
}

func createMockCloudServer(requests *[]string, requestsMu *sync.Mutex) (*mcpsdk.Server, error) {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "mock-cloud-server",
		Version: "1.0.0",
	}, nil)

	server.AddTool(&mcpsdk.Tool{
		Name:        "cloud_upload_file",
		InputSchema: map[string]any{"type": "object"},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		var args struct {
			LocalPath string `json:"local_path"`
		}
		_ = json.Unmarshal(req.Params.Arguments, &args)
		requestsMu.Lock()
		*requests = append(*requests, args.LocalPath)
		requestsMu.Unlock()

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "http://example.com/character_seed.png"}},
		}, nil
	})
	return server, nil
}

func TestBrew_CrefUnsupportedError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-brew-cref-err-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Setup pipeline initiate
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Theme",
		TargetPageCount: 1,
	}
	mInit, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}
	mInit.BookProperties.CharacterProfile = "mock-character-profile"
	mInit.Progress.ManuscriptGenerated = true
	mInit.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusPending, Text: "Stanza 1"},
	}
	if err = mInit.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, cleanupMCP := setupMockImageGenServerWithCapabilities(t, ctx, serverTransport, "/dummy/source.png", "openai", false, false)
	defer cleanupMCP()

	optsBrew := BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
	}

	err = Brew(ctx, optsBrew)
	if err == nil {
		t.Fatal("expected Brew to fail when active backend does not support cref but a character profile is defined")
	}

	expectedErr := "active imagegen backend [openai] does not support character references, but a character profile is defined; switch backend to midjourney or clean manifest character properties"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("expected error containing %q, got %q", expectedErr, err.Error())
	}
}

func TestBrew_CapabilitiesQueryError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-brew-cap-query-err-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Theme",
		TargetPageCount: 1,
	}
	mInit, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}
	mInit.BookProperties.CharacterProfile = "mock-character-profile"
	mInit.Progress.ManuscriptGenerated = true
	mInit.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusPending, Text: "Stanza 1"},
	}
	if err = mInit.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()

	// Create server that returns an error when callTool imagegen_get_capabilities is queried
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "mock-imagegen-server",
		Version: "1.0.0",
	}, nil)

	server.AddTool(&mcpsdk.Tool{
		Name:        "imagegen_get_capabilities",
		InputSchema: map[string]any{"type": "object"},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{
			IsError: true,
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: "failed to fetch capabilities"},
			},
		}, nil
	})

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = serverSession.Close() }()

	optsBrew := BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
	}

	err = Brew(ctx, optsBrew)
	if err == nil {
		t.Fatal("expected Brew to fail when capabilities query fails")
	}
	if !strings.Contains(err.Error(), "failed to query imagegen backend capabilities") {
		t.Errorf("unexpected error: %v", err)
	}
}

type videoSeedInjectSetup struct {
	Ctx              context.Context
	Cancel           context.CancelFunc
	Opts             BrewOptions
	ImageGenRequests *[]generateRequest
	UploadPaths      *[]string
	RequestsMu       *sync.Mutex
}

func setupVideoSeedInjectTest(t *testing.T, tmpDir, sourceExt, theme string) videoSeedInjectSetup {
	dummySourceImage := filepath.Join(tmpDir, "source"+sourceExt)
	if err := os.WriteFile(dummySourceImage, []byte("fake-video-bytes"), 0600); err != nil {
		t.Fatalf("failed to write source image: %v", err)
	}

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           theme,
		TargetPageCount: 2,
	}
	mInit, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	mInit.BookProperties.CharacterProfile = "mock-char-profile"
	mInit.Progress.ManuscriptGenerated = true
	mInit.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusPending, Text: "Stanza 1 text", IllustrationPrompt: "Prompt 1"},
		{PageIndex: 2, Status: manifest.StatusPending, Text: "Stanza 2 text", IllustrationPrompt: "Prompt 2"},
	}
	if err = mInit.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	// Setup mock ImageGen Server
	imageGenClientTransport, imageGenServerTransport := mcpsdk.NewInMemoryTransports()
	var requestsMu sync.Mutex
	var imageGenRequests []generateRequest
	imageGenServer, err := createMockImageGenServer(ctx, dummySourceImage, &imageGenRequests, &requestsMu)
	if err != nil {
		cancel()
		t.Fatalf("failed to create mock imagegen server: %v", err)
	}
	imageGenSession, err := imageGenServer.Connect(ctx, imageGenServerTransport, nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = imageGenSession.Close() })

	// Setup mock Cloud Server
	cloudClientTransport, cloudServerTransport := mcpsdk.NewInMemoryTransports()
	var uploadPaths []string
	cloudServer, err := createMockCloudServer(&uploadPaths, &requestsMu)
	if err != nil {
		cancel()
		t.Fatalf("failed to create mock cloud server: %v", err)
	}
	cloudSession, err := cloudServer.Connect(ctx, cloudServerTransport, nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cloudSession.Close() })

	opts := BrewOptions{
		OutputDir:         tmpDir,
		MCPTransport:      imageGenClientTransport,
		CloudMCPTransport: cloudClientTransport,
	}

	return videoSeedInjectSetup{
		Ctx:              ctx,
		Cancel:           cancel,
		Opts:             opts,
		ImageGenRequests: &imageGenRequests,
		UploadPaths:      &uploadPaths,
		RequestsMu:       &requestsMu,
	}
}

func TestBrew_CharacterInvariantInjection_VideoSeed_FFmpegNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	setup := setupVideoSeedInjectTest(t, tmpDir, ".mp4", "Invariant Test Theme")
	defer setup.Cancel()

	// Stub lookPathFunc to return error
	origLookPath := lookPathFunc
	lookPathFunc = func(file string) (string, error) {
		return "", exec.ErrNotFound
	}
	defer func() { lookPathFunc = origLookPath }()

	err := Brew(setup.Ctx, setup.Opts)
	if err == nil {
		t.Fatal("expected Brew to fail due to ffmpeg not found, got nil")
	}
	if !strings.Contains(err.Error(), "ffmpeg not found in PATH") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBrew_CharacterInvariantInjection_VideoSeed_ExtractionFailed(t *testing.T) {
	tmpDir := t.TempDir()
	setup := setupVideoSeedInjectTest(t, tmpDir, ".mp4", "Invariant Test Theme")
	defer setup.Cancel()

	// Stub lookPathFunc to succeed, but execCommandContext to execute a failing command ("false")
	origLookPath := lookPathFunc
	origExec := execCommandContext
	lookPathFunc = func(file string) (string, error) {
		return "ffmpeg", nil
	}
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}
	defer func() {
		lookPathFunc = origLookPath
		execCommandContext = origExec
	}()

	err := Brew(setup.Ctx, setup.Opts)
	if err == nil {
		t.Fatal("expected Brew to fail due to ffmpeg failure, got nil")
	}
	if !strings.Contains(err.Error(), "failed to extract static frame from character seed video") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBrew_CharacterInvariantInjection_VideoSeed_Success(t *testing.T) {
	tmpDir := t.TempDir()
	setup := setupVideoSeedInjectTest(t, tmpDir, ".mp4", "Invariant Test Theme")
	defer setup.Cancel()

	// Stub lookPathFunc to succeed, and execCommandContext to write the PNG file and run "true"
	origLookPath := lookPathFunc
	origExec := execCommandContext
	lookPathFunc = func(file string) (string, error) {
		return "ffmpeg", nil
	}
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		pngPath := args[len(args)-1]
		_ = os.WriteFile(pngPath, []byte("fake-extracted-png"), 0600)
		return exec.CommandContext(ctx, "true")
	}
	defer func() {
		lookPathFunc = origLookPath
		execCommandContext = origExec
	}()

	err := Brew(setup.Ctx, setup.Opts)
	if err != nil {
		t.Fatalf("expected Brew to succeed, got error: %v", err)
	}

	setup.RequestsMu.Lock()
	defer setup.RequestsMu.Unlock()
	if len(*setup.UploadPaths) == 0 {
		t.Fatal("expected character_seed to be uploaded, but uploadPaths is empty")
	}
	if !strings.Contains((*setup.UploadPaths)[0], "character_seed.png") {
		t.Errorf("expected uploaded path to contain character_seed.png, got %q", (*setup.UploadPaths)[0])
	}
}

func TestBrew_CharacterInvariantInjection_WebmVideoSeed_Success(t *testing.T) {
	tmpDir := t.TempDir()
	setup := setupVideoSeedInjectTest(t, tmpDir, ".WEBM", "Invariant Test Theme Webm")
	defer setup.Cancel()

	// Stub lookPathFunc to succeed, and execCommandContext to write the PNG file and run "true"
	origLookPath := lookPathFunc
	origExec := execCommandContext
	lookPathFunc = func(file string) (string, error) {
		return "ffmpeg", nil
	}
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		pngPath := args[len(args)-1]
		_ = os.WriteFile(pngPath, []byte("fake-extracted-png"), 0600)
		return exec.CommandContext(ctx, "true")
	}
	defer func() {
		lookPathFunc = origLookPath
		execCommandContext = origExec
	}()

	err := Brew(setup.Ctx, setup.Opts)
	if err != nil {
		t.Fatalf("expected Brew to succeed, got error: %v", err)
	}

	setup.RequestsMu.Lock()
	defer setup.RequestsMu.Unlock()
	if len(*setup.UploadPaths) == 0 {
		t.Fatal("expected character_seed to be uploaded, but uploadPaths is empty")
	}
	if !strings.Contains((*setup.UploadPaths)[0], "character_seed.png") {
		t.Errorf("expected uploaded path to contain character_seed.png, got %q", (*setup.UploadPaths)[0])
	}
}

func TestBrew_CapabilitiesInvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-brew-cap-json-err-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Theme",
		TargetPageCount: 1,
	}
	mInit, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}
	mInit.BookProperties.CharacterProfile = "mock-character-profile"
	mInit.Progress.ManuscriptGenerated = true
	mInit.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusPending, Text: "Stanza 1"},
	}
	if err = mInit.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()

	// Create server that returns invalid JSON for capabilities
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "mock-imagegen-server",
		Version: "1.0.0",
	}, nil)

	server.AddTool(&mcpsdk.Tool{
		Name:        "imagegen_get_capabilities",
		InputSchema: map[string]any{"type": "object"},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: "invalid-json"},
			},
		}, nil
	})

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = serverSession.Close() }()

	optsBrew := BrewOptions{
		OutputDir:    tmpDir,
		MCPTransport: clientTransport,
	}

	err = Brew(ctx, optsBrew)
	if err == nil {
		t.Fatal("expected Brew to fail when capabilities JSON is invalid")
	}
	if !strings.Contains(err.Error(), "failed to parse imagegen backend capabilities JSON") {
		t.Errorf("unexpected error: %v", err)
	}
}
