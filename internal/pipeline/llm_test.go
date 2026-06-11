package pipeline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
						{Text: `{"stanzas": ["Stanza 1", "Stanza 2", "Stanza 3"]}`},
					},
				},
			},
		},
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

	stanzas, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(stanzas) != 3 {
		t.Errorf("expected 3 stanzas, got %d", len(stanzas))
	}
	if stanzas[0] != "Stanza 1" || stanzas[2] != "Stanza 3" {
		t.Errorf("unexpected stanzas content: %v", stanzas)
	}
}

func TestGeminiClient_GenerateStanzas_MissingAPIKey(t *testing.T) {
	clientEmpty := &GeminiClient{}
	_, err := clientEmpty.GenerateStanzas(context.Background(), "theme", 3)
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
	_, err := clientErr.GenerateStanzas(context.Background(), "theme", 3)
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
	_, err := clientEmptyCand.GenerateStanzas(context.Background(), "theme", 3)
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
	_, err := clientInvalidJSON.GenerateStanzas(context.Background(), "theme", 3)
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
						{Text: `{"stanzas": ["Stanza 1", "Stanza 2"]}`},
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
	_, err := clientCountMismatch.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for stanzas count mismatch, got nil")
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
					Content: `{"stanzas": ["Stanza A", "Stanza B"]}`,
				},
			},
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

	stanzas, err := client.GenerateStanzas(context.Background(), "theme", 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(stanzas) != 2 {
		t.Errorf("expected 2 stanzas, got %d", len(stanzas))
	}
	if stanzas[0] != "Stanza A" || stanzas[1] != "Stanza B" {
		t.Errorf("unexpected stanzas content: %v", stanzas)
	}
}

func TestOpenAIClient_GenerateStanzas_Errors(t *testing.T) {
	// 1. Missing API Key
	clientEmpty := &OpenAIClient{}
	_, err := clientEmpty.GenerateStanzas(context.Background(), "theme", 3)
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
	_, err = clientErr.GenerateStanzas(context.Background(), "theme", 3)
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
	_, err = clientEmptyChoices.GenerateStanzas(context.Background(), "theme", 3)
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
	_, err = clientInvalidJSON.GenerateStanzas(context.Background(), "theme", 3)
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
					Content: `{"stanzas": ["Stanza 1"]}`,
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
	_, err = clientCountMismatch.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for stanzas count mismatch, got nil")
	}
}

func TestGeminiClient_GenerateStanzas_RequestError(t *testing.T) {
	client := &GeminiClient{
		APIKey:  "key",
		BaseURL: "%%invalid%%",
	}
	_, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for invalid URL character, got nil")
	}
}

func TestGeminiClient_GenerateStanzas_DoError(t *testing.T) {
	client := &GeminiClient{
		APIKey:  "key",
		BaseURL: "http://localhost:54321", // unused port
	}
	_, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for connection refused, got nil")
	}
}

func TestGeminiClient_GenerateStanzas_ReadError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100") // promise bytes
		// close connection immediately or write chunked error (we can just hijack or use close)
	}))
	defer server.Close()

	client := &GeminiClient{
		APIKey:  "key",
		BaseURL: server.URL,
		Client:  server.Client(),
	}
	_, err := client.GenerateStanzas(context.Background(), "theme", 3)
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
	_, err := client.GenerateStanzas(context.Background(), "theme", 3)
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
	_, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for empty parts content, got nil")
	}
}

func TestOpenAIClient_GenerateStanzas_RequestError(t *testing.T) {
	client := &OpenAIClient{
		APIKey:  "key",
		BaseURL: "%%invalid%%",
	}
	_, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for invalid URL character, got nil")
	}
}

func TestOpenAIClient_GenerateStanzas_DoError(t *testing.T) {
	client := &OpenAIClient{
		APIKey:  "key",
		BaseURL: "http://localhost:54321", // unused port
	}
	_, err := client.GenerateStanzas(context.Background(), "theme", 3)
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
	_, err := client.GenerateStanzas(context.Background(), "theme", 3)
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
	_, err := client.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for outer JSON parsing, got nil")
	}
}
