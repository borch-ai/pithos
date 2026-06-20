package pipeline

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/mcp"
	"github.com/borch-ai/powerword/pkg/llm"
)

type mockMcpClient struct {
	binaryPath string
	startErr   error
	capText    string
	callErr    error
}

func (m *mockMcpClient) ResolveBinaryPath() string {
	return m.binaryPath
}

func (m *mockMcpClient) Start(ctx context.Context) error {
	return m.startErr
}

func (m *mockMcpClient) Stop() error {
	return nil
}

func (m *mockMcpClient) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	if m.callErr != nil {
		return "", m.callErr
	}
	if toolName == "imagegen_get_capabilities" {
		if m.capText != "" {
			return m.capText, nil
		}
		return `{"backend":"google","supports_cref":false,"supports_sref":false}`, nil
	}
	return "", nil
}

type mockPowerwordLLM struct {
	err error
}

func (m *mockPowerwordLLM) Generate(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition, opts ...llm.GenerateOption) (*llm.Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &llm.Message{Content: "pong"}, nil
}

func (m *mockPowerwordLLM) Stream(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition) (<-chan llm.StreamChunk, error) {
	return nil, nil
}

func (m *mockPowerwordLLM) ListModels(ctx context.Context) ([]string, error) {
	return nil, nil
}

func TestDoctor_NilConfig(t *testing.T) {
	origCfg := config.Cfg
	config.Cfg = nil
	defer func() { config.Cfg = origCfg }()

	results, hasFailure := RunDiagnostics(context.Background())
	if !hasFailure {
		t.Error("expected diagnostics failure when config is nil, got false")
	}

	foundConfigLoad := false
	for _, item := range results {
		if item.Name == "Configuration Load" {
			foundConfigLoad = true
			if item.Status != StatusFail {
				t.Errorf("expected 'Configuration Load' status to be FAIL, got %s", item.Status)
			}
		}
	}
	if !foundConfigLoad {
		t.Error("expected 'Configuration Load' check to be present in results")
	}
}

func TestDoctor_SuccessFlow(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	config.Cfg = &config.Config{
		API: config.APIConfig{
			GeminiKey: "dummy-gemini-key",
			OpenAIKey: "dummy-openai-key",
		},
		MCP: config.MCPConfig{
			ImageGenPath: os.Args[0], // executable that is guaranteed to exist
			KDPMathPath:  os.Args[0],
			SEOPath:      os.Args[0],
			ViralPath:    os.Args[0],
			TypstPath:    os.Args[0],
			CloudPath:    os.Args[0],
			PDFCheckPath: os.Args[0],
		},
	}

	// Mock LLM Client
	origNewLLM := newLLMClientFunc
	newLLMClientFunc = func(modelName string, geminiKey, openaiKey string, httpClient *http.Client) (llm.LLMClient, error) {
		return &mockPowerwordLLM{}, nil
	}
	defer func() { newLLMClientFunc = origNewLLM }()

	// Mock MCP Clients
	origNewMCP := newPluginClientFunc
	newPluginClientFunc = func(pType mcp.PluginType) mcpClientInterface {
		return &mockMcpClient{
			binaryPath: os.Args[0],
			startErr:   nil,
			capText:    `{"backend":"mock","supports_cref":true,"supports_sref":true}`,
		}
	}
	defer func() { newPluginClientFunc = origNewMCP }()

	results, hasFailure := RunDiagnostics(context.Background())
	if hasFailure {
		t.Errorf("expected diagnostics to pass, but got failure: %+v", results)
	}

	for _, item := range results {
		if item.Status != StatusOk && item.Status != StatusSkip {
			t.Errorf("expected status OK or SKIP for check %s, got %s: %s", item.Name, item.Status, item.Message)
		}
	}
}

func TestDoctor_FailuresAndSkips(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	config.Cfg = &config.Config{
		API: config.APIConfig{
			GeminiKey: "",
			OpenAIKey: "",
		},
		MCP: config.MCPConfig{
			ImageGenPath: "", // trigger missing path/LookPath failure
			KDPMathPath:  "/nonexistent/kdp-math",
			SEOPath:      os.Args[0],
			ViralPath:    os.Args[0],
			TypstPath:    os.Args[0],
			CloudPath:    os.Args[0],
		},
	}

	// Mock LLM Client creation with error for ping checks (though they will be skipped because keys are empty)
	origNewLLM := newLLMClientFunc
	newLLMClientFunc = func(modelName string, geminiKey, openaiKey string, httpClient *http.Client) (llm.LLMClient, error) {
		return nil, errors.New("failed LLM initialization")
	}
	defer func() { newLLMClientFunc = origNewLLM }()

	// Mock MCP Client with handshake failures
	origNewMCP := newPluginClientFunc
	newPluginClientFunc = func(pType mcp.PluginType) mcpClientInterface {
		if pType == mcp.PluginSEO {
			return &mockMcpClient{
				binaryPath: os.Args[0],
				startErr:   errors.New("handshake timeout"),
			}
		}
		return &mockMcpClient{
			binaryPath: "/nonexistent/path",
			startErr:   nil,
		}
	}
	defer func() { newPluginClientFunc = origNewMCP }()

	results, hasFailure := RunDiagnostics(context.Background())
	if !hasFailure {
		t.Error("expected diagnostics to fail, got success")
	}

	// Verify specific failures
	foundLLMIntegrationFail := false
	foundMcpHandshakeFail := false
	foundMcpLookpathFail := false

	for _, item := range results {
		if item.Name == "LLM Integration" && item.Status == StatusFail {
			foundLLMIntegrationFail = true
		}
		if item.Name == "SEO Metadata Plugin (pw-mcp-seo)" && item.Status == StatusWarning {
			foundMcpHandshakeFail = true
		}
		if item.Name == "KDP Mathematics Plugin (pw-mcp-kdp-math)" && item.Status == StatusFail {
			foundMcpLookpathFail = true
		}
	}

	if !foundLLMIntegrationFail {
		t.Error("expected LLM Integration failure due to empty API keys")
	}
	if !foundMcpHandshakeFail {
		t.Error("expected SEO Metadata Plugin handshake failure")
	}
	if !foundMcpLookpathFail {
		t.Error("expected KDP Math Plugin lookpath lookup failure")
	}
}

func TestDoctor_LLMPingFailure(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	config.Cfg = &config.Config{
		API: config.APIConfig{
			GeminiKey: "dummy-gemini-key",
		},
	}

	// Mock LLM Client ping failure
	origNewLLM := newLLMClientFunc
	newLLMClientFunc = func(modelName string, geminiKey, openaiKey string, httpClient *http.Client) (llm.LLMClient, error) {
		return &mockPowerwordLLM{err: errors.New("API rate limit exceeded")}, nil
	}
	defer func() { newLLMClientFunc = origNewLLM }()

	// Stub MCP
	origNewMCP := newPluginClientFunc
	newPluginClientFunc = func(pType mcp.PluginType) mcpClientInterface {
		return &mockMcpClient{
			binaryPath: os.Args[0],
		}
	}
	defer func() { newPluginClientFunc = origNewMCP }()

	results, hasFailure := RunDiagnostics(context.Background())
	if !hasFailure {
		t.Error("expected diagnostics to fail due to LLM ping failure, got success")
	}

	foundPingFail := false
	for _, item := range results {
		if item.Name == "Gemini API Handshake" {
			foundPingFail = true
			if item.Status != StatusFail {
				t.Errorf("expected Gemini API Handshake check status to be FAIL, got %s", item.Status)
			}
		}
	}
	if !foundPingFail {
		t.Error("expected Gemini API Handshake check to be present in results")
	}
}

func TestDoctor_DiagnoseMCPPlugins_NilConfig(t *testing.T) {
	origCfg := config.Cfg
	config.Cfg = nil
	defer func() { config.Cfg = origCfg }()

	mcpItems := DiagnoseMCPPlugins(context.Background())
	if len(mcpItems) != 0 {
		t.Errorf("expected 0 items when config is nil, got %d", len(mcpItems))
	}

	llmItems := DiagnoseLLMConnection(context.Background())
	if len(llmItems) != 0 {
		t.Errorf("expected 0 items when config is nil, got %d", len(llmItems))
	}
}

func TestDoctor_ImageGenCapabilitiesRequiredFail(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	config.Cfg = &config.Config{
		API: config.APIConfig{
			GeminiKey: "dummy-gemini-key",
		},
		MCP: config.MCPConfig{
			ImageGenPath: os.Args[0],
			KDPMathPath:  os.Args[0],
			SEOPath:      os.Args[0],
			ViralPath:    os.Args[0],
			TypstPath:    os.Args[0],
			CloudPath:    os.Args[0],
			PDFCheckPath: os.Args[0],
		},
	}

	// Use isolated temp working directory to prevent clobbering developer's books directory
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	tempWd := t.TempDir()
	if err := os.Chdir(tempWd); err != nil {
		t.Fatalf("failed to change directory to temp dir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	// Create temporary books directory with a manifest that requires character profiles
	if err := os.MkdirAll("books/test-book", 0750); err != nil {
		t.Fatalf("failed to create books dir: %v", err)
	}

	dummyManifest := `{"book_properties": {"character_profile": "A parodic frog"}}`
	if err := os.WriteFile("books/test-book/manifest.json", []byte(dummyManifest), 0600); err != nil {
		t.Fatalf("failed to write dummy manifest: %v", err)
	}

	// Mock LLM Client
	origNewLLM := newLLMClientFunc
	newLLMClientFunc = func(modelName string, geminiKey, openaiKey string, httpClient *http.Client) (llm.LLMClient, error) {
		return &mockPowerwordLLM{}, nil
	}
	defer func() { newLLMClientFunc = origNewLLM }()

	// Mock MCP Clients
	origNewMCP := newPluginClientFunc
	newPluginClientFunc = func(pType mcp.PluginType) mcpClientInterface {
		return &mockMcpClient{
			binaryPath: os.Args[0],
			startErr:   nil,
			capText:    `{"backend":"google","supports_cref":false,"supports_sref":false}`,
		}
	}
	defer func() { newPluginClientFunc = origNewMCP }()

	results, hasFailure := RunDiagnostics(context.Background())
	if !hasFailure {
		t.Fatalf("expected diagnostics to fail due to unsupported cref check, but it passed: %+v", results)
	}

	foundFailure := false
	for _, item := range results {
		if item.Name == "Image Generation Plugin (pw-mcp-imagegen)" {
			if item.Status != StatusFail {
				t.Errorf("expected StatusFail, got %s", item.Status)
			}
			expectedMsg := "Connected successfully. Active backend: [google] (cref: UNSUPPORTED, sref: UNSUPPORTED) - ERROR: active backend does not support cref, but local books request character profiles"
			if item.Message != expectedMsg {
				t.Errorf("expected message:\n%q\ngot:\n%q", expectedMsg, item.Message)
			}
			foundFailure = true
		}
	}

	if !foundFailure {
		t.Error("expected to find failure for image generation plugin")
	}
}

func TestDoctor_ImageGenCapabilitiesError(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	config.Cfg = &config.Config{
		API: config.APIConfig{
			GeminiKey: "dummy-gemini-key",
		},
		MCP: config.MCPConfig{
			ImageGenPath: os.Args[0],
			KDPMathPath:  os.Args[0],
			SEOPath:      os.Args[0],
			ViralPath:    os.Args[0],
			TypstPath:    os.Args[0],
			CloudPath:    os.Args[0],
			PDFCheckPath: os.Args[0],
		},
	}

	origNewLLM := newLLMClientFunc
	newLLMClientFunc = func(modelName string, geminiKey, openaiKey string, httpClient *http.Client) (llm.LLMClient, error) {
		return &mockPowerwordLLM{}, nil
	}
	defer func() { newLLMClientFunc = origNewLLM }()

	origNewMCP := newPluginClientFunc
	newPluginClientFunc = func(pType mcp.PluginType) mcpClientInterface {
		return &mockMcpClient{
			binaryPath: os.Args[0],
			startErr:   nil,
			callErr:    errors.New("capabilities lookup failed"),
		}
	}
	defer func() { newPluginClientFunc = origNewMCP }()

	results, hasFailure := RunDiagnostics(context.Background())
	if hasFailure {
		t.Fatalf("expected diagnostics to not fail, got failure: %+v", results)
	}

	foundWarning := false
	for _, item := range results {
		if item.Name == "Image Generation Plugin (pw-mcp-imagegen)" {
			if item.Status != StatusWarning {
				t.Errorf("expected StatusWarning, got %s", item.Status)
			}
			if !strings.Contains(item.Message, "but failed to get capabilities: capabilities lookup failed") {
				t.Errorf("unexpected message: %q", item.Message)
			}
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Error("expected to find warning for image generation plugin")
	}
}

func TestDoctor_ImageGenCapabilitiesInvalidJSON(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	config.Cfg = &config.Config{
		API: config.APIConfig{
			GeminiKey: "dummy-gemini-key",
		},
		MCP: config.MCPConfig{
			ImageGenPath: os.Args[0],
			KDPMathPath:  os.Args[0],
			SEOPath:      os.Args[0],
			ViralPath:    os.Args[0],
			TypstPath:    os.Args[0],
			CloudPath:    os.Args[0],
			PDFCheckPath: os.Args[0],
		},
	}

	origNewLLM := newLLMClientFunc
	newLLMClientFunc = func(modelName string, geminiKey, openaiKey string, httpClient *http.Client) (llm.LLMClient, error) {
		return &mockPowerwordLLM{}, nil
	}
	defer func() { newLLMClientFunc = origNewLLM }()

	origNewMCP := newPluginClientFunc
	newPluginClientFunc = func(pType mcp.PluginType) mcpClientInterface {
		return &mockMcpClient{
			binaryPath: os.Args[0],
			startErr:   nil,
			capText:    "invalid-json",
		}
	}
	defer func() { newPluginClientFunc = origNewMCP }()

	results, hasFailure := RunDiagnostics(context.Background())
	if hasFailure {
		t.Fatalf("expected diagnostics to not fail, got failure: %+v", results)
	}

	foundWarning := false
	for _, item := range results {
		if item.Name == "Image Generation Plugin (pw-mcp-imagegen)" {
			if item.Status != StatusWarning {
				t.Errorf("expected StatusWarning, got %s", item.Status)
			}
			if !strings.Contains(item.Message, "but capability response is not valid JSON") {
				t.Errorf("unexpected message: %q", item.Message)
			}
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Error("expected to find warning for image generation plugin")
	}
}

func TestDoctor_PDFCheckHandshakeWarning(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	config.Cfg = &config.Config{
		API: config.APIConfig{
			GeminiKey: "dummy-gemini-key",
		},
		MCP: config.MCPConfig{
			ImageGenPath: os.Args[0],
			KDPMathPath:  os.Args[0],
			SEOPath:      os.Args[0],
			ViralPath:    os.Args[0],
			TypstPath:    os.Args[0],
			CloudPath:    os.Args[0],
			PDFCheckPath: os.Args[0],
		},
	}

	origNewLLM := newLLMClientFunc
	newLLMClientFunc = func(modelName string, geminiKey, openaiKey string, httpClient *http.Client) (llm.LLMClient, error) {
		return &mockPowerwordLLM{}, nil
	}
	defer func() { newLLMClientFunc = origNewLLM }()

	origNewMCP := newPluginClientFunc
	newPluginClientFunc = func(pType mcp.PluginType) mcpClientInterface {
		if pType == mcp.PluginPDFCheck {
			return &mockMcpClient{
				binaryPath: os.Args[0],
				startErr:   errors.New("handshake failed for pdfcheck"),
			}
		}
		return &mockMcpClient{
			binaryPath: os.Args[0],
			startErr:   nil,
		}
	}
	defer func() { newPluginClientFunc = origNewMCP }()

	results, hasFailure := RunDiagnostics(context.Background())
	if hasFailure {
		t.Fatalf("expected diagnostics to not fail since pdfcheck is non-critical, got failure: %+v", results)
	}

	foundWarning := false
	for _, item := range results {
		if item.Name == "PDF Preflight Validation Plugin (pw-mcp-pdfcheck)" {
			if item.Status != StatusWarning {
				t.Errorf("expected StatusWarning, got %s", item.Status)
			}
			if !strings.Contains(item.Message, "handshake failed for pdfcheck") {
				t.Errorf("unexpected message: %q", item.Message)
			}
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Error("expected to find warning for PDF check plugin")
	}
}

func TestDoctor_ImageGenCapabilitiesOverrides(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	config.Cfg = &config.Config{
		API: config.APIConfig{
			GeminiKey: "dummy-gemini-key",
		},
		MCP: config.MCPConfig{
			ImageGenPath:      os.Args[0],
			KDPMathPath:       os.Args[0],
			SEOPath:           os.Args[0],
			ViralPath:         os.Args[0],
			TypstPath:         os.Args[0],
			CloudPath:         os.Args[0],
			PDFCheckPath:      os.Args[0],
			ImageGenForceCref: true,
			ImageGenForceSref: true,
		},
	}

	// Use isolated temp working directory to prevent clobbering developer's books directory
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	tempWd := t.TempDir()
	if err := os.Chdir(tempWd); err != nil {
		t.Fatalf("failed to change directory to temp dir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	// Create temporary books directory with a manifest that requires character profiles
	if err := os.MkdirAll("books/test-book", 0750); err != nil {
		t.Fatalf("failed to create books dir: %v", err)
	}

	dummyManifest := `{"book_properties": {"character_profile": "A parodic frog"}}`
	if err := os.WriteFile("books/test-book/manifest.json", []byte(dummyManifest), 0600); err != nil {
		t.Fatalf("failed to write dummy manifest: %v", err)
	}

	// Mock LLM Client
	origNewLLM := newLLMClientFunc
	newLLMClientFunc = func(modelName string, geminiKey, openaiKey string, httpClient *http.Client) (llm.LLMClient, error) {
		return &mockPowerwordLLM{}, nil
	}
	defer func() { newLLMClientFunc = origNewLLM }()

	// Mock MCP Clients (backend reports cref/sref unsupported, but config overrides them)
	origNewMCP := newPluginClientFunc
	newPluginClientFunc = func(pType mcp.PluginType) mcpClientInterface {
		return &mockMcpClient{
			binaryPath: os.Args[0],
			startErr:   nil,
			capText:    `{"backend":"google","supports_cref":false,"supports_sref":false}`,
		}
	}
	defer func() { newPluginClientFunc = origNewMCP }()

	results, hasFailure := RunDiagnostics(context.Background())
	if hasFailure {
		t.Fatalf("expected diagnostics to pass with overrides enabled, but it failed: %+v", results)
	}

	foundPlugin := false
	for _, item := range results {
		if item.Name == "Image Generation Plugin (pw-mcp-imagegen)" {
			if item.Status != StatusOk {
				t.Errorf("expected StatusOk, got %s", item.Status)
			}
			expectedMsg := "Connected successfully. Active backend: [google] (cref: SUPPORTED [overridden], sref: SUPPORTED [overridden])"
			if item.Message != expectedMsg {
				t.Errorf("expected message:\n%q\ngot:\n%q", expectedMsg, item.Message)
			}
			foundPlugin = true
		}
	}

	if !foundPlugin {
		t.Error("expected to find status for image generation plugin")
	}
}
