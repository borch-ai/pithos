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
	GenerateVisualGuides(ctx context.Context, theme string) (string, string, telemetry.TokenUsage, error)
	GenerateStanzas(ctx context.Context, theme string, count int, style string, characterProfile string) ([]string, []string, telemetry.TokenUsage, error)
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

// visualGuidesResponse is the JSON format for Level 1 & Level 2 visual guide generation.
type visualGuidesResponse struct {
	StyleSeed        string `json:"style_seed"`
	CharacterProfile string `json:"character_profile"`
}

// GenerateVisualGuides generates global visual style and character consistency profile.
func (a *PowerwordClientAdapter) GenerateVisualGuides(ctx context.Context, theme string) (string, string, telemetry.TokenUsage, error) {
	if a == nil || a.client == nil {
		return "", "", telemetry.TokenUsage{}, errors.New("underlying powerword client is nil")
	}
	prompt := fmt.Sprintf(
		"You are an expert children's book illustrator and art director. "+
			"Based on the theme %q, generate the visual style guide and character profile for a parodic children's book.\n"+
			"Return the output in JSON format with two keys:\n"+
			"1. 'style_seed': General art style medium, rendering style, lighting, and palette (e.g., claymation style, vibrant cinematic lighting, shallow depth of field).\n"+
			"2. 'character_profile': Persistent visual properties and description of the main character(s) (e.g., clothing colors, physical features, accessories) to ensure visual consistency.",
		theme,
	)

	messages := []llm.Message{
		{
			Role:    llm.RoleUser,
			Content: prompt,
		},
	}

	msg, err := a.client.Generate(ctx, messages, nil, llm.WithResponseSchema(visualGuidesResponse{}))
	if err != nil {
		return "", "", telemetry.TokenUsage{}, fmt.Errorf("llm generate visual guides error: %w", err)
	}

	if msg == nil || msg.Content == "" {
		return "", "", telemetry.TokenUsage{}, errors.New("empty response from llm client for visual guides")
	}

	cleanedContent := cleanJSONText(msg.Content)

	var finalResp visualGuidesResponse
	if err := json.Unmarshal([]byte(cleanedContent), &finalResp); err != nil {
		return "", "", telemetry.TokenUsage{}, fmt.Errorf("failed to parse visual guides json from llm response: %w (raw content: %s)", err, truncateString(msg.Content, 200))
	}

	var usage telemetry.TokenUsage
	if msg.Usage != nil {
		usage = *msg.Usage
	}

	return finalResp.StyleSeed, finalResp.CharacterProfile, usage, nil
}

// GenerateStanzas queries the powerword LLMClient to generate parodic stanzas based on a theme, incorporating the global style and character guides.
func (a *PowerwordClientAdapter) GenerateStanzas(ctx context.Context, theme string, count int, style string, characterProfile string) ([]string, []string, telemetry.TokenUsage, error) {
	if a == nil || a.client == nil {
		return nil, nil, telemetry.TokenUsage{}, errors.New("underlying powerword client is nil")
	}
	prompt := fmt.Sprintf(
		"Write a parodic children's book poem in strict rhythmic meter about the theme: %q. "+
			"The poem must have exactly %d stanzas. "+
			"The global art style for the book is: %q. "+
			"The main character(s) profile is: %q. "+
			"For each stanza, generate a detailed visual description (illustration prompt) that describes the scene's "+
			"action, setting, and integrates the consistent character details from the character profile above. "+
			"Return the output in JSON format with two keys: "+
			"1. 'stanzas': an array of %d strings, where each element is one stanza representing one page of the book. "+
			"2. 'illustration_prompts': an array of %d strings, where each element is the detailed illustration prompt for the corresponding stanza.",
		theme, count, style, characterProfile, count, count,
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

	cleanedContent := cleanJSONText(msg.Content)

	var finalResp stanzasResponse
	if err := json.Unmarshal([]byte(cleanedContent), &finalResp); err != nil {
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

// cleanJSONText trims whitespace and strips markdown code blocks (e.g. ```json ... ```) from output text.
func cleanJSONText(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		// Find the first newline to strip the opening fence (e.g., ```json)
		if idx := strings.Index(text, "\n"); idx != -1 {
			text = text[idx+1:]
		} else {
			text = strings.TrimPrefix(text, "```")
		}
		// Strip the trailing fence if it exists
		if strings.HasSuffix(text, "```") {
			text = strings.TrimSuffix(text, "```")
		}
		text = strings.TrimSpace(text)
	}
	return text
}

// newPowerwordLLMClient creates a powerword LLMClient configured for either Gemini or OpenAI
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
		baseURL = strings.TrimRight(baseURL, "/")
		if !strings.HasSuffix(baseURL, "/v1") {
			baseURL += "/v1"
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
