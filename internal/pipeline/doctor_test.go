package pipeline

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/mcp"
	"github.com/borch-ai/powerword/pkg/llm"
)

type mockMcpClient struct {
	binaryPath string
	startErr   error
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
