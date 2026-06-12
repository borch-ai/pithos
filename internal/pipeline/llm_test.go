package pipeline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/borch-ai/powerword/pkg/telemetry"
)

func TestGeminiClient_GenerateStanzas_Success(t *testing.T) {
	mockResponse := geminiResponse{
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
						{Text: `{"stanzas": ["Stanza 1", "Stanza 2", "Stanza 3"], "illustration_prompts": ["Prompt 1", "Prompt 2", "Prompt 3"]}`},
					},
				},
			},
		},
	}
	mockResponse.UsageMetadata = &struct {
		PromptTokenCount        int `json:"promptTokenCount"`
		CandidatesTokenCount    int `json:"candidatesTokenCount"`
		CachedContentTokenCount int `json:"cachedContentTokenCount"`
	}{
		PromptTokenCount:        500,
		CandidatesTokenCount:    300,
		CachedContentTokenCount: 100,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := &GeminiClient{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
		Client:  server.Client(),
	}

	var usage telemetry.TokenUsage
	stanzas, prompts, usage, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(stanzas) != 3 {
		t.Errorf("expected 3 stanzas, got %d", len(stanzas))
	}
	if stanzas[0] != "Stanza 1" || stanzas[2] != "Stanza 3" {
		t.Errorf("unexpected stanzas content: %v", stanzas)
	}
	if len(prompts) != 3 {
		t.Errorf("expected 3 prompts, got %d", len(prompts))
	}
	if prompts[0] != "Prompt 1" || prompts[2] != "Prompt 3" {
		t.Errorf("unexpected prompts content: %v", prompts)
	}
	if usage.InputTokens != 500 || usage.OutputTokens != 300 || usage.CachedTokens != 100 {
		t.Errorf("unexpected usage parsing: %+v", usage)
	}
}

func TestGeminiClient_GenerateStanzas_MissingAPIKey(t *testing.T) {
	clientEmpty := &GeminiClient{}
	_, _, _, err := clientEmpty.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for empty API key, got nil")
	}
}

func TestGeminiClient_GenerateStanzas_StatusError(t *testing.T) {
	serverErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request format"))
	}))
	defer serverErr.Close()

	clientErr := &GeminiClient{
		APIKey:  "test-key",
		BaseURL: serverErr.URL,
		Client:  serverErr.Client(),
	}
	_, _, _, err := clientErr.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for bad status code, got nil")
	}
}

func TestGeminiClient_GenerateStanzas_EmptyCandidates(t *testing.T) {
	serverEmptyCandidates := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(geminiResponse{Candidates: nil})
	}))
	defer serverEmptyCandidates.Close()

	clientEmptyCand := &GeminiClient{
		APIKey:  "test-key",
		BaseURL: serverEmptyCandidates.URL,
		Client:  serverEmptyCandidates.Client(),
	}
	_, _, _, err := clientEmptyCand.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for empty candidates list, got nil")
	}
}

func TestGeminiClient_GenerateStanzas_InvalidJSONText(t *testing.T) {
	mockResponseInvalidJSON := geminiResponse{
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
						{Text: `invalid json`},
					},
				},
			},
		},
	}
	serverInvalidJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponseInvalidJSON)
	}))
	defer serverInvalidJSON.Close()

	clientInvalidJSON := &GeminiClient{
		APIKey:  "test-key",
		BaseURL: serverInvalidJSON.URL,
		Client:  serverInvalidJSON.Client(),
	}
	_, _, _, err := clientInvalidJSON.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for invalid json text content, got nil")
	}
}

func TestGeminiClient_GenerateStanzas_StanzasCountMismatch(t *testing.T) {
	mockResponseCountMismatch := geminiResponse{
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
						{Text: `{"stanzas": ["Stanza 1", "Stanza 2"], "illustration_prompts": ["Prompt 1", "Prompt 2"]}`},
					},
				},
			},
		},
	}
	serverCountMismatch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponseCountMismatch)
	}))
	defer serverCountMismatch.Close()

	clientCountMismatch := &GeminiClient{
		APIKey:  "test-key",
		BaseURL: serverCountMismatch.URL,
		Client:  serverCountMismatch.Client(),
	}
	_, _, _, err := clientCountMismatch.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for stanzas count mismatch, got nil")
	}
}

func TestGeminiClient_GenerateStanzas_PromptsCountMismatch(t *testing.T) {
	mockResponseCountMismatch := geminiResponse{
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
						{Text: `{"stanzas": ["Stanza 1", "Stanza 2", "Stanza 3"], "illustration_prompts": ["Prompt 1", "Prompt 2"]}`},
					},
				},
			},
		},
	}
	serverCountMismatch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponseCountMismatch)
	}))
	defer serverCountMismatch.Close()

	clientCountMismatch := &GeminiClient{
		APIKey:  "test-key",
		BaseURL: serverCountMismatch.URL,
		Client:  serverCountMismatch.Client(),
	}
	_, _, _, err := clientCountMismatch.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for prompts count mismatch, got nil")
	}
}

func TestOpenAIClient_GenerateStanzas_Success(t *testing.T) {
	mockResponse := openAIResponse{
		Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			{
				Message: struct {
					Content string `json:"content"`
				}{
					Content: `{"stanzas": ["Stanza A", "Stanza B"], "illustration_prompts": ["Prompt A", "Prompt B"]}`,
				},
			},
		},
	}
	mockResponse.Usage = &struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	}{
		PromptTokens:     1000,
		CompletionTokens: 500,
		PromptTokensDetails: &struct {
			CachedTokens int `json:"cached_tokens"`
		}{
			CachedTokens: 200,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := &OpenAIClient{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
		Client:  server.Client(),
	}

	var usage telemetry.TokenUsage
	stanzas, prompts, usage, err := client.GenerateStanzas(context.Background(), "theme", 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(stanzas) != 2 {
		t.Errorf("expected 2 stanzas, got %d", len(stanzas))
	}
	if stanzas[0] != "Stanza A" || stanzas[1] != "Stanza B" {
		t.Errorf("unexpected stanzas content: %v", stanzas)
	}
	if len(prompts) != 2 {
		t.Errorf("expected 2 prompts, got %d", len(prompts))
	}
	if prompts[0] != "Prompt A" || prompts[1] != "Prompt B" {
		t.Errorf("unexpected prompts content: %v", prompts)
	}
	if usage.InputTokens != 1000 || usage.OutputTokens != 500 || usage.CachedTokens != 200 {
		t.Errorf("unexpected usage parsing: %+v", usage)
	}
}

//nolint:funlen // Test suite testing multiple error pathways is inherently long
func TestOpenAIClient_GenerateStanzas_Errors(t *testing.T) {
	// 1. Missing API Key
	clientEmpty := &OpenAIClient{}
	_, _, _, err := clientEmpty.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for empty API key, got nil")
	}

	// 2. HTTP Error Status
	serverErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	defer serverErr.Close()

	clientErr := &OpenAIClient{
		APIKey:  "test-key",
		BaseURL: serverErr.URL,
		Client:  serverErr.Client(),
	}
	_, _, _, err = clientErr.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for bad status code, got nil")
	}

	// 3. Empty Response Choices
	serverEmptyChoices := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openAIResponse{Choices: nil})
	}))
	defer serverEmptyChoices.Close()

	clientEmptyChoices := &OpenAIClient{
		APIKey:  "test-key",
		BaseURL: serverEmptyChoices.URL,
		Client:  serverEmptyChoices.Client(),
	}
	_, _, _, err = clientEmptyChoices.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for empty choices list, got nil")
	}

	// 4. Invalid JSON content in message
	mockResponseInvalidJSON := openAIResponse{
		Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			{
				Message: struct {
					Content string `json:"content"`
				}{
					Content: `invalid`,
				},
			},
		},
	}
	serverInvalidJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponseInvalidJSON)
	}))
	defer serverInvalidJSON.Close()

	clientInvalidJSON := &OpenAIClient{
		APIKey:  "test-key",
		BaseURL: serverInvalidJSON.URL,
		Client:  serverInvalidJSON.Client(),
	}
	_, _, _, err = clientInvalidJSON.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for invalid json text content, got nil")
	}

	// 5. Stanzas count mismatch
	mockResponseCountMismatch := openAIResponse{
		Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			{
				Message: struct {
					Content string `json:"content"`
				}{
					Content: `{"stanzas": ["Stanza 1"], "illustration_prompts": ["Prompt 1"]}`,
				},
			},
		},
	}
	serverCountMismatch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponseCountMismatch)
	}))
	defer serverCountMismatch.Close()

	clientCountMismatch := &OpenAIClient{
		APIKey:  "test-key",
		BaseURL: serverCountMismatch.URL,
		Client:  serverCountMismatch.Client(),
	}
	_, _, _, err = clientCountMismatch.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for stanzas count mismatch, got nil")
	}

	// 6. Prompts count mismatch
	mockResponsePromptMismatch := openAIResponse{
		Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			{
				Message: struct {
					Content string `json:"content"`
				}{
					Content: `{"stanzas": ["Stanza 1", "Stanza 2", "Stanza 3"], "illustration_prompts": ["Prompt 1", "Prompt 2"]}`,
				},
			},
		},
	}
	serverPromptMismatch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponsePromptMismatch)
	}))
	defer serverPromptMismatch.Close()

	clientPromptMismatch := &OpenAIClient{
		APIKey:  "test-key",
		BaseURL: serverPromptMismatch.URL,
		Client:  serverPromptMismatch.Client(),
	}
	_, _, _, err = clientPromptMismatch.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for prompts count mismatch, got nil")
	}
}

func TestGeminiClient_GenerateStanzas_RequestError(t *testing.T) {
	client := &GeminiClient{
		APIKey:  "key",
		BaseURL: "%%invalid%%",
	}
	_, _, _, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for invalid URL character, got nil")
	}
}

func TestGeminiClient_GenerateStanzas_DoError(t *testing.T) {
	client := &GeminiClient{
		APIKey:  "key",
		BaseURL: "http://localhost:54321", // unused port
	}
	_, _, _, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for connection refused, got nil")
	}
}

func TestGeminiClient_GenerateStanzas_ReadError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100") // promise bytes
	}))
	defer server.Close()

	client := &GeminiClient{
		APIKey:  "key",
		BaseURL: server.URL,
		Client:  server.Client(),
	}
	_, _, _, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error due to unexpected EOF on read, got nil")
	}
}

func TestGeminiClient_GenerateStanzas_OuterJSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{invalid-json}"))
	}))
	defer server.Close()

	client := &GeminiClient{
		APIKey:  "key",
		BaseURL: server.URL,
		Client:  server.Client(),
	}
	_, _, _, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for outer JSON parsing, got nil")
	}
}

func TestGeminiClient_GenerateStanzas_EmptyParts(t *testing.T) {
	// Candidates exists, but Parts is empty
	mockResponse := geminiResponse{
		Candidates: []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		}{
			{}, // Parts is nil/empty
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := &GeminiClient{
		APIKey:  "key",
		BaseURL: server.URL,
		Client:  server.Client(),
	}
	_, _, _, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for empty parts content, got nil")
	}
}

func TestOpenAIClient_GenerateStanzas_RequestError(t *testing.T) {
	client := &OpenAIClient{
		APIKey:  "key",
		BaseURL: "%%invalid%%",
	}
	_, _, _, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for invalid URL character, got nil")
	}
}

func TestOpenAIClient_GenerateStanzas_DoError(t *testing.T) {
	client := &OpenAIClient{
		APIKey:  "key",
		BaseURL: "http://localhost:54321", // unused port
	}
	_, _, _, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for connection refused, got nil")
	}
}

func TestOpenAIClient_GenerateStanzas_ReadError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
	}))
	defer server.Close()

	client := &OpenAIClient{
		APIKey:  "key",
		BaseURL: server.URL,
		Client:  server.Client(),
	}
	_, _, _, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error due to unexpected EOF on read, got nil")
	}
}

func TestOpenAIClient_GenerateStanzas_OuterJSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{invalid-json}"))
	}))
	defer server.Close()

	client := &OpenAIClient{
		APIKey:  "key",
		BaseURL: server.URL,
		Client:  server.Client(),
	}
	_, _, _, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for outer JSON parsing, got nil")
	}
}
