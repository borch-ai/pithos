package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/borch-ai/powerword/pkg/llm"
	"github.com/borch-ai/powerword/pkg/telemetry"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/api/option"
)

// LLMClient defines the interface for generating stanzas using an LLM.
type LLMClient interface {
	GenerateStanzas(ctx context.Context, theme string, count int) ([]string, []string, telemetry.TokenUsage, error)
}

// PowerwordClientAdapter wraps a powerword LLMClient.
type PowerwordClientAdapter struct {
	client    llm.LLMClient
	modelName string
}

// stanzasResponse is the uniform JSON format we expect from the LLM.
type stanzasResponse struct {
	Stanzas             []string `json:"stanzas"`
	IllustrationPrompts []string `json:"illustration_prompts"`
}

// GenerateStanzas queries the powerword LLMClient to generate parodic stanzas based on a theme.
func (a *PowerwordClientAdapter) GenerateStanzas(ctx context.Context, theme string, count int) ([]string, []string, telemetry.TokenUsage, error) {
	prompt := fmt.Sprintf(
		"Write a parodic children's book poem in strict rhythmic meter about the theme: %q. "+
			"The poem must have exactly %d stanzas. "+
			"Additionally, you must define a consistent visual character style and description for the characters "+
			"in the book (e.g., specific clothing, hair color, and features) to avoid character drift. "+
			"For each stanza, generate a detailed visual description (illustration prompt) that describes the scene's "+
			"action, setting, and integrates the consistent character details. "+
			"Return the output in JSON format with two keys: "+
			"1. 'stanzas': an array of %d strings, where each element is one stanza representing one page of the book. "+
			"2. 'illustration_prompts': an array of %d strings, where each element is the detailed illustration prompt for the corresponding stanza.",
		theme, count, count, count,
	)

	messages := []llm.Message{
		{
			Role:    llm.RoleUser,
			Content: prompt,
		},
	}

	msg, err := a.client.Generate(ctx, messages, nil, llm.WithResponseSchema(stanzasResponse{}))
	if err != nil {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("llm generate error: %w", err)
	}

	if msg == nil || msg.Content == "" {
		return nil, nil, telemetry.TokenUsage{}, errors.New("empty response from llm client")
	}

	var finalResp stanzasResponse
	if err := json.Unmarshal([]byte(msg.Content), &finalResp); err != nil {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("failed to parse stanzas json from llm response: %w (raw content: %s)", err, truncateString(msg.Content, 200))
	}

	if len(finalResp.Stanzas) != count {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("llm generated %d stanzas, expected exactly %d", len(finalResp.Stanzas), count)
	}

	if len(finalResp.IllustrationPrompts) != count {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("llm generated %d illustration prompts, expected exactly %d", len(finalResp.IllustrationPrompts), count)
	}

	var usage telemetry.TokenUsage
	if msg.Usage != nil {
		usage = *msg.Usage
	}

	return finalResp.Stanzas, finalResp.IllustrationPrompts, usage, nil
}

// newPowerwordLLMClient creates a powerword LLMClient configured for either Gemini or OpenAI/Anthropic
// with optional HTTP client override (primarily for test mocking).
func newPowerwordLLMClient(modelName string, geminiKey, openaiKey string, httpClient *http.Client) (llm.LLMClient, error) {
	if strings.Contains(strings.ToLower(modelName), "gemini") {
		if geminiKey == "" {
			return nil, errors.New("gemini API key is required")
		}
		var opts []option.ClientOption
		opts = append(opts, option.WithAPIKey(geminiKey))
		if httpClient != nil {
			opts = append(opts, option.WithHTTPClient(httpClient))
		}
		return llm.NewGeminiClientWithOpts(modelName, opts...)
	}

	// Fallback/default to OpenAI
	if openaiKey == "" {
		return nil, errors.New("openai API key is required")
	}
	cfg := openai.DefaultConfig(openaiKey)
	if httpClient != nil {
		cfg.HTTPClient = httpClient
	}
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL != "" {
		if !strings.HasSuffix(baseURL, "/v1") {
			baseURL = strings.TrimRight(baseURL, "/") + "/v1"
		}
		cfg.BaseURL = baseURL
	}
	return llm.NewOpenAIClientWithConfig(cfg, modelName), nil
}

func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
