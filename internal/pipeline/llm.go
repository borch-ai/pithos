package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/borch-ai/powerword/pkg/telemetry"
)

// LLMClient defines the interface for generating stanzas using an LLM.
type LLMClient interface {
	GenerateStanzas(ctx context.Context, theme string, count int) ([]string, []string, telemetry.TokenUsage, error)
}

// GeminiClient interacts with Google's Gemini API via REST.
type GeminiClient struct {
	APIKey  string
	BaseURL string
	Client  *http.Client
}

// OpenAIClient interacts with OpenAI's Chat API via REST.
type OpenAIClient struct {
	APIKey  string
	BaseURL string
	Client  *http.Client
}

// stanzasResponse is the uniform JSON format we expect from the LLM.
type stanzasResponse struct {
	Stanzas             []string `json:"stanzas"`
	IllustrationPrompts []string `json:"illustration_prompts"`
}

type geminiRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig *geminiConfig   `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiConfig struct {
	ResponseMimeType string `json:"responseMimeType,omitempty"`
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

type openAIRequest struct {
	Model          string          `json:"model"`
	Messages       []openAIMessage `json:"messages"`
	ResponseFormat *openAIFormat   `json:"response_format,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIFormat struct {
	Type string `json:"type"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
}

// GenerateStanzas queries Gemini to generate parodic stanzas based on a theme.
//
//nolint:funlen // LLM manuscript prompt construction and JSON parsing is inherently long
func (g *GeminiClient) GenerateStanzas(ctx context.Context, theme string, count int) ([]string, []string, telemetry.TokenUsage, error) {
	if g.APIKey == "" {
		return nil, nil, telemetry.TokenUsage{}, errors.New("gemini api key is required")
	}

	httpClient := g.Client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	baseURL := g.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}

	url := fmt.Sprintf("%s/v1beta/models/gemini-2.5-flash:generateContent?key=%s", baseURL, g.APIKey)

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

	reqPayload := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: &geminiConfig{
			ResponseMimeType: "application/json",
		},
	}

	reqBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("failed to marshal gemini request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("failed to create gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("gemini api request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("gemini api returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("failed to read gemini response body: %w", err)
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("failed to parse gemini response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 ||
		len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, nil, telemetry.TokenUsage{}, errors.New("empty response content received from gemini")
	}

	rawJSONText := geminiResp.Candidates[0].Content.Parts[0].Text

	var finalResp stanzasResponse
	if err := json.Unmarshal([]byte(rawJSONText), &finalResp); err != nil {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("failed to parse stanzas json from gemini: %w (raw content: %s)", err, truncateString(rawJSONText, 200))
	}

	if len(finalResp.Stanzas) != count {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("gemini generated %d stanzas, expected exactly %d", len(finalResp.Stanzas), count)
	}

	if len(finalResp.IllustrationPrompts) != count {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("gemini generated %d illustration prompts, expected exactly %d", len(finalResp.IllustrationPrompts), count)
	}

	var usage telemetry.TokenUsage
	if geminiResp.UsageMetadata != nil {
		usage.InputTokens = geminiResp.UsageMetadata.PromptTokenCount
		usage.OutputTokens = geminiResp.UsageMetadata.CandidatesTokenCount
		usage.CachedTokens = geminiResp.UsageMetadata.CachedContentTokenCount
	}

	return finalResp.Stanzas, finalResp.IllustrationPrompts, usage, nil
}

// GenerateStanzas queries OpenAI to generate parodic stanzas based on a theme.
//
//nolint:funlen // LLM manuscript prompt construction and JSON parsing is inherently long
func (o *OpenAIClient) GenerateStanzas(ctx context.Context, theme string, count int) ([]string, []string, telemetry.TokenUsage, error) {
	if o.APIKey == "" {
		return nil, nil, telemetry.TokenUsage{}, errors.New("openai api key is required")
	}

	httpClient := o.Client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	baseURL := o.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}

	url := fmt.Sprintf("%s/v1/chat/completions", baseURL)

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

	reqPayload := openAIRequest{
		Model: "gpt-4o",
		Messages: []openAIMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		ResponseFormat: &openAIFormat{
			Type: "json_object",
		},
	}

	reqBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("failed to marshal openai request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("failed to create openai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("openai api request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("openai api returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("failed to read openai response body: %w", err)
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(bodyBytes, &openAIResp); err != nil {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("failed to parse openai response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, nil, telemetry.TokenUsage{}, errors.New("empty response choices received from openai")
	}

	rawJSONText := openAIResp.Choices[0].Message.Content

	var finalResp stanzasResponse
	if err := json.Unmarshal([]byte(rawJSONText), &finalResp); err != nil {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("failed to parse stanzas json from openai: %w (raw content: %s)", err, truncateString(rawJSONText, 200))
	}

	if len(finalResp.Stanzas) != count {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("openai generated %d stanzas, expected exactly %d", len(finalResp.Stanzas), count)
	}

	if len(finalResp.IllustrationPrompts) != count {
		return nil, nil, telemetry.TokenUsage{}, fmt.Errorf("openai generated %d illustration prompts, expected exactly %d", len(finalResp.IllustrationPrompts), count)
	}

	var usage telemetry.TokenUsage
	if openAIResp.Usage != nil {
		usage.InputTokens = openAIResp.Usage.PromptTokens
		usage.OutputTokens = openAIResp.Usage.CompletionTokens
		if openAIResp.Usage.PromptTokensDetails != nil {
			usage.CachedTokens = openAIResp.Usage.PromptTokensDetails.CachedTokens
		}
	}

	return finalResp.Stanzas, finalResp.IllustrationPrompts, usage, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
