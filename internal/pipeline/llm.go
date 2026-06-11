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
)

// LLMClient defines the interface for generating stanzas using an LLM.
type LLMClient interface {
	GenerateStanzas(ctx context.Context, theme string, count int) ([]string, error)
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
	Stanzas []string `json:"stanzas"`
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
}

// GenerateStanzas queries Gemini to generate parodic stanzas based on a theme.
func (g *GeminiClient) GenerateStanzas(ctx context.Context, theme string, count int) ([]string, error) {
	if g.APIKey == "" {
		return nil, errors.New("gemini api key is required")
	}

	httpClient := g.Client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	baseURL := g.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}

	url := fmt.Sprintf("%s/v1beta/models/gemini-1.5-flash:generateContent?key=%s", baseURL, g.APIKey)

	prompt := fmt.Sprintf(
		"Write a parodic children's book poem in strict rhythmic meter about the theme: %q. "+
			"The poem must have exactly %d stanzas. "+
			"Return the output in JSON format with a single key 'stanzas' containing an array of strings, "+
			"where each element is one stanza representing one page of the book.",
		theme, count,
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
		return nil, fmt.Errorf("failed to marshal gemini request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini api request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini api returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read gemini response body: %w", err)
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse gemini response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 ||
		len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, errors.New("empty response content received from gemini")
	}

	rawJSONText := geminiResp.Candidates[0].Content.Parts[0].Text

	var finalResp stanzasResponse
	if err := json.Unmarshal([]byte(rawJSONText), &finalResp); err != nil {
		return nil, fmt.Errorf("failed to parse stanzas json from gemini: %w (raw content: %s)", err, truncateString(rawJSONText, 200))
	}

	if len(finalResp.Stanzas) != count {
		return nil, fmt.Errorf("gemini generated %d stanzas, expected exactly %d", len(finalResp.Stanzas), count)
	}

	return finalResp.Stanzas, nil
}

// GenerateStanzas queries OpenAI to generate parodic stanzas based on a theme.
func (o *OpenAIClient) GenerateStanzas(ctx context.Context, theme string, count int) ([]string, error) {
	if o.APIKey == "" {
		return nil, errors.New("openai api key is required")
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
			"Return the output in JSON format with a single key 'stanzas' containing an array of strings, "+
			"where each element is one stanza representing one page of the book.",
		theme, count,
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
		return nil, fmt.Errorf("failed to marshal openai request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create openai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai api request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai api returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read openai response body: %w", err)
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(bodyBytes, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to parse openai response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, errors.New("empty response choices received from openai")
	}

	rawJSONText := openAIResp.Choices[0].Message.Content

	var finalResp stanzasResponse
	if err := json.Unmarshal([]byte(rawJSONText), &finalResp); err != nil {
		return nil, fmt.Errorf("failed to parse stanzas json from openai: %w (raw content: %s)", err, truncateString(rawJSONText, 200))
	}

	if len(finalResp.Stanzas) != count {
		return nil, fmt.Errorf("openai generated %d stanzas, expected exactly %d", len(finalResp.Stanzas), count)
	}

	return finalResp.Stanzas, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
