package pipeline

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/mcp"
)

// DiagnosticStatus represents status of a check.
type DiagnosticStatus string

const (
	StatusOk      DiagnosticStatus = "OK"
	StatusFail    DiagnosticStatus = "FAIL"
	StatusWarning DiagnosticStatus = "WARNING"
	StatusSkip    DiagnosticStatus = "SKIPPED"
)

// DiagnosticItem holds results of one check.
type DiagnosticItem struct {
	Name    string
	Status  DiagnosticStatus
	Message string
}

type mcpClientInterface interface {
	ResolveBinaryPath() string
	Start(ctx context.Context) error
	Stop() error
}

var newPluginClientFunc = func(pType mcp.PluginType) mcpClientInterface {
	return mcp.NewPluginClient(pType)
}

// DiagnoseConfig checks the configuration settings.
func DiagnoseConfig() []DiagnosticItem {
	var items []DiagnosticItem
	if config.Cfg == nil {
		items = append(items, DiagnosticItem{
			Name:    "Configuration Load",
			Status:  StatusFail,
			Message: "No configuration has been loaded",
		})
		return items
	}
	items = append(items, DiagnosticItem{
		Name:    "Configuration Load",
		Status:  StatusOk,
		Message: "Configuration loaded successfully",
	})
	return items
}

// DiagnoseCredentials checks if LLM credentials are set.
func DiagnoseCredentials() []DiagnosticItem {
	var items []DiagnosticItem
	if config.Cfg == nil {
		return items
	}

	// Gemini
	if config.Cfg.API.GeminiKey == "" {
		items = append(items, DiagnosticItem{
			Name:    "Gemini API Key Setup",
			Status:  StatusSkip,
			Message: "Gemini key is empty (Gemini operations skipped)",
		})
	} else {
		items = append(items, DiagnosticItem{
			Name:    "Gemini API Key Setup",
			Status:  StatusOk,
			Message: "Gemini API key is configured",
		})
	}

	// OpenAI
	if config.Cfg.API.OpenAIKey == "" {
		items = append(items, DiagnosticItem{
			Name:    "OpenAI API Key Setup",
			Status:  StatusSkip,
			Message: "OpenAI key is empty (OpenAI operations skipped)",
		})
	} else {
		items = append(items, DiagnosticItem{
			Name:    "OpenAI API Key Setup",
			Status:  StatusOk,
			Message: "OpenAI API key is configured",
		})
	}

	if config.Cfg.API.GeminiKey == "" && config.Cfg.API.OpenAIKey == "" {
		items = append(items, DiagnosticItem{
			Name:    "LLM Integration",
			Status:  StatusFail,
			Message: "Neither Gemini nor OpenAI API key is configured. LLM pipelines will fail.",
		})
	}

	return items
}

// DiagnoseMCPPlugins checks if MCP plugin binaries are executable and if they handshake successfully.
func DiagnoseMCPPlugins(ctx context.Context) []DiagnosticItem {
	var items []DiagnosticItem
	if config.Cfg == nil {
		return items
	}

	plugins := []struct {
		pType      mcp.PluginType
		name       string
		isCritical bool
	}{
		{mcp.PluginImageGen, "Image Generation Plugin (pw-mcp-imagegen)", true},
		{mcp.PluginKDPMath, "KDP Mathematics Plugin (pw-mcp-kdp-math)", true},
		{mcp.PluginSEO, "SEO Metadata Plugin (pw-mcp-seo)", false},
		{mcp.PluginViral, "Viral Promotional/Video Plugin (pw-mcp-viral)", false},
		{mcp.PluginTypst, "Typst Compiler Plugin (pw-mcp-typst)", true},
		{mcp.PluginCloud, "Cloud Storage Plugin (pw-mcp-cloud)", false},
	}

	for _, p := range plugins {
		client := newPluginClientFunc(p.pType)
		binaryPath := client.ResolveBinaryPath()

		if binaryPath == "" {
			status := StatusFail
			if !p.isCritical {
				status = StatusWarning
			}
			items = append(items, DiagnosticItem{
				Name:    p.name,
				Status:  status,
				Message: "No binary path configured or resolved",
			})
			continue
		}

		// Check executable presence
		_, lookErr := exec.LookPath(binaryPath)
		if lookErr != nil {
			status := StatusFail
			if !p.isCritical {
				status = StatusWarning
			}
			items = append(items, DiagnosticItem{
				Name:    p.name,
				Status:  status,
				Message: fmt.Sprintf("Executable not found at %q", binaryPath),
			})
			continue
		}

		// Try connection handshake
		handshakeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := client.Start(handshakeCtx)
		cancel()

		if err != nil {
			status := StatusFail
			if !p.isCritical {
				status = StatusWarning
			}
			items = append(items, DiagnosticItem{
				Name:    p.name,
				Status:  status,
				Message: fmt.Sprintf("Handshake failed for binary %q: %v", binaryPath, err),
			})
			_ = client.Stop() // Best effort cleanup to avoid subprocess leak
		} else {
			items = append(items, DiagnosticItem{
				Name:    p.name,
				Status:  StatusOk,
				Message: fmt.Sprintf("Connected successfully to %s", binaryPath),
			})
			_ = client.Stop()
		}
	}

	return items
}

func pingModel(ctx context.Context, name string, modelName string, geminiKey string, openaiKey string) DiagnosticItem {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pwClient, err := newLLMClientFunc(modelName, geminiKey, openaiKey, nil)
	if err != nil {
		return DiagnosticItem{
			Name:    name,
			Status:  StatusFail,
			Message: fmt.Sprintf("Failed to initialize client: %v", err),
		}
	}

	adapter := &PowerwordClientAdapter{client: pwClient, modelName: modelName}
	if pingErr := adapter.Ping(pingCtx); pingErr != nil {
		return DiagnosticItem{
			Name:    name,
			Status:  StatusFail,
			Message: fmt.Sprintf("Ping request failed: %v", pingErr),
		}
	}

	return DiagnosticItem{
		Name:    name,
		Status:  StatusOk,
		Message: "Ping succeeded",
	}
}

// DiagnoseLLMConnection checks connection status by sending a ping request to the configured LLMs.
func DiagnoseLLMConnection(ctx context.Context) []DiagnosticItem {
	var items []DiagnosticItem
	if config.Cfg == nil {
		return items
	}

	// Gemini Ping
	if config.Cfg.API.GeminiKey != "" {
		items = append(items, pingModel(ctx, "Gemini API Handshake", "gemini-2.5-flash", config.Cfg.API.GeminiKey, ""))
	}

	// OpenAI Ping
	if config.Cfg.API.OpenAIKey != "" {
		items = append(items, pingModel(ctx, "OpenAI API Handshake", "gpt-4o", "", config.Cfg.API.OpenAIKey))
	}

	return items
}

// RunDiagnostics executes the full checklist of preflight tests.
func RunDiagnostics(ctx context.Context) ([]DiagnosticItem, bool) {
	var results []DiagnosticItem
	hasFailure := false

	configResults := DiagnoseConfig()
	results = append(results, configResults...)
	for _, item := range configResults {
		if item.Status == StatusFail {
			return results, true
		}
	}

	results = append(results, DiagnoseCredentials()...)
	results = append(results, DiagnoseMCPPlugins(ctx)...)
	results = append(results, DiagnoseLLMConnection(ctx)...)

	for _, item := range results {
		if item.Status == StatusFail {
			hasFailure = true
		}
	}

	return results, hasFailure
}
