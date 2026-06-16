package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/borch-ai/powerword/pkg/llm"
	"github.com/borch-ai/powerword/pkg/telemetry"
)

type mockPWClient struct {
	response *llm.Message
	err      error
}

func (m *mockPWClient) Generate(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition, opts ...llm.GenerateOption) (*llm.Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockPWClient) Stream(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}

func (m *mockPWClient) ListModels(ctx context.Context) ([]string, error) {
	return nil, nil
}

func TestPowerwordClientAdapter_GenerateStanzas_Success(t *testing.T) {
	mockMsg := &llm.Message{
		Role:    llm.RoleAssistant,
		Content: `{"stanzas": ["Stanza 1", "Stanza 2", "Stanza 3"], "illustration_prompts": ["Prompt 1", "Prompt 2", "Prompt 3"]}`,
		Usage: &telemetry.TokenUsage{
			InputTokens:  500,
			OutputTokens: 300,
			CachedTokens: 100,
		},
	}

	mockClient := &mockPWClient{response: mockMsg}
	adapter := &PowerwordClientAdapter{client: mockClient, modelName: "test-model"}

	stanzas, prompts, usage, err := adapter.GenerateStanzas(context.Background(), "theme", 3, "mock-style", "mock-profile")
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
		t.Errorf("unexpected usage: %+v", usage)
	}
}

func TestPowerwordClientAdapter_GenerateStanzas_GenerateError(t *testing.T) {
	mockClient := &mockPWClient{err: errors.New("API error")}
	adapter := &PowerwordClientAdapter{client: mockClient, modelName: "test-model"}

	_, _, _, err := adapter.GenerateStanzas(context.Background(), "theme", 3, "mock-style", "mock-profile")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestPowerwordClientAdapter_GenerateStanzas_EmptyResponse(t *testing.T) {
	mockClient := &mockPWClient{response: nil}
	adapter := &PowerwordClientAdapter{client: mockClient, modelName: "test-model"}

	_, _, _, err := adapter.GenerateStanzas(context.Background(), "theme", 3, "mock-style", "mock-profile")
	if err == nil {
		t.Error("expected error for nil response message, got nil")
	}

	mockClientEmpty := &mockPWClient{response: &llm.Message{Content: ""}}
	adapterEmpty := &PowerwordClientAdapter{client: mockClientEmpty, modelName: "test-model"}
	_, _, _, err = adapterEmpty.GenerateStanzas(context.Background(), "theme", 3, "mock-style", "mock-profile")
	if err == nil {
		t.Error("expected error for empty response content, got nil")
	}
}

func TestPowerwordClientAdapter_GenerateStanzas_InvalidJSON(t *testing.T) {
	mockMsg := &llm.Message{
		Role:    llm.RoleAssistant,
		Content: `invalid json`,
	}
	mockClient := &mockPWClient{response: mockMsg}
	adapter := &PowerwordClientAdapter{client: mockClient, modelName: "test-model"}

	_, _, _, err := adapter.GenerateStanzas(context.Background(), "theme", 3, "mock-style", "mock-profile")
	if err == nil {
		t.Error("expected error for invalid JSON content, got nil")
	}
}

func TestPowerwordClientAdapter_GenerateStanzas_StanzasMismatch(t *testing.T) {
	mockMsg := &llm.Message{
		Role:    llm.RoleAssistant,
		Content: `{"stanzas": ["Stanza 1"], "illustration_prompts": ["Prompt 1", "Prompt 2", "Prompt 3"]}`,
	}
	mockClient := &mockPWClient{response: mockMsg}
	adapter := &PowerwordClientAdapter{client: mockClient, modelName: "test-model"}

	_, _, _, err := adapter.GenerateStanzas(context.Background(), "theme", 3, "mock-style", "mock-profile")
	if err == nil {
		t.Error("expected error for stanzas count mismatch, got nil")
	}
}

func TestPowerwordClientAdapter_GenerateStanzas_PromptsMismatch(t *testing.T) {
	mockMsg := &llm.Message{
		Role:    llm.RoleAssistant,
		Content: `{"stanzas": ["Stanza 1", "Stanza 2", "Stanza 3"], "illustration_prompts": ["Prompt 1"]}`,
	}
	mockClient := &mockPWClient{response: mockMsg}
	adapter := &PowerwordClientAdapter{client: mockClient, modelName: "test-model"}

	_, _, _, err := adapter.GenerateStanzas(context.Background(), "theme", 3, "mock-style", "mock-profile")
	if err == nil {
		t.Error("expected error for prompts count mismatch, got nil")
	}
}

func TestNewPowerwordLLMClient_Errors(t *testing.T) {
	// Missing Gemini key
	_, err := newPowerwordLLMClient("gemini-2.5-flash", "", "", nil)
	if err == nil {
		t.Error("expected error for missing gemini key, got nil")
	}

	// Missing OpenAI key
	_, err = newPowerwordLLMClient("gpt-4o", "", "", nil)
	if err == nil {
		t.Error("expected error for missing openai key, got nil")
	}
}

func TestNewPowerwordLLMClient_BaseURLNormalization(t *testing.T) {
	tests := []struct {
		envVal string
	}{
		{"https://api.openai.com"},
		{"https://api.openai.com/"},
		{"https://api.openai.com/v1"},
		{"https://api.openai.com/v1/"},
	}

	for _, tc := range tests {
		t.Run(tc.envVal, func(t *testing.T) {
			t.Setenv("OPENAI_BASE_URL", tc.envVal)
			_, err := newPowerwordLLMClient("gpt-4o", "", "test-key", nil)
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}
		})
	}
}

func TestPowerwordClientAdapter_GenerateStanzas_NilClient(t *testing.T) {
	var adapter *PowerwordClientAdapter
	_, _, _, err := adapter.GenerateStanzas(context.Background(), "theme", 3, "mock-style", "mock-profile")
	if err == nil {
		t.Error("expected error for nil adapter, got nil")
	}

	adapter2 := &PowerwordClientAdapter{client: nil, modelName: "test-model"}
	_, _, _, err = adapter2.GenerateStanzas(context.Background(), "theme", 3, "mock-style", "mock-profile")
	if err == nil {
		t.Error("expected error for nil underlying client, got nil")
	}
}

func TestTruncateString(t *testing.T) {
	s1 := "short"
	if res := truncateString(s1, 10); res != "short" {
		t.Errorf("expected %q, got %q", "short", res)
	}

	s2 := "longer-than-max"
	if res := truncateString(s2, 5); res != "longe..." {
		t.Errorf("expected %q, got %q", "longe...", res)
	}
}

func TestPowerwordClientAdapter_GenerateStanzas_InvalidJSON_Truncation(t *testing.T) {
	// Construct a long invalid JSON string
	longInvalidContent := "invalid json" + string(make([]byte, 300))
	mockMsg := &llm.Message{
		Role:    llm.RoleAssistant,
		Content: longInvalidContent,
	}
	mockClient := &mockPWClient{response: mockMsg}
	adapter := &PowerwordClientAdapter{client: mockClient, modelName: "test-model"}

	_, _, _, err := adapter.GenerateStanzas(context.Background(), "theme", 3, "mock-style", "mock-profile")
	if err == nil {
		t.Error("expected error for long invalid JSON, got nil")
	} else if !strings.Contains(err.Error(), "...") {
		t.Errorf("expected error message to contain truncated text (with ellipsis), got: %v", err)
	}
}

func TestPowerwordClientAdapter_GenerateVisualGuides_Success(t *testing.T) {
	mockMsg := &llm.Message{
		Role:    llm.RoleAssistant,
		Content: `{"style_seed": "claymation style", "character_profile": "A chubby orange cat"}`,
		Usage: &telemetry.TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	mockClient := &mockPWClient{response: mockMsg}
	adapter := &PowerwordClientAdapter{client: mockClient, modelName: "test-model"}

	style, profile, usage, err := adapter.GenerateVisualGuides(context.Background(), "theme")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if style != "claymation style" {
		t.Errorf("expected style 'claymation style', got %q", style)
	}
	if profile != "A chubby orange cat" {
		t.Errorf("expected profile 'A chubby orange cat', got %q", profile)
	}
	if usage.InputTokens != 100 || usage.OutputTokens != 50 {
		t.Errorf("unexpected usage: %+v", usage)
	}
}

func TestPowerwordClientAdapter_GenerateVisualGuides_Error(t *testing.T) {
	mockClient := &mockPWClient{err: errors.New("API error")}
	adapter := &PowerwordClientAdapter{client: mockClient, modelName: "test-model"}

	_, _, _, err := adapter.GenerateVisualGuides(context.Background(), "theme")
	if err == nil {
		t.Error("expected error, got nil")
	}

	mockClientNilMsg := &mockPWClient{response: nil}
	adapterNil := &PowerwordClientAdapter{client: mockClientNilMsg, modelName: "test-model"}
	_, _, _, err = adapterNil.GenerateVisualGuides(context.Background(), "theme")
	if err == nil {
		t.Error("expected error for nil response, got nil")
	}
}

func TestCleanJSONText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"{}", "{}"},
		{" \n {\"key\": \"value\"} \n ", "{\"key\": \"value\"}"},
		{"```json\n{\"foo\": \"bar\"}\n```", "{\"foo\": \"bar\"}"},
		{"```\n{\"foo\": \"bar\"}\n```", "{\"foo\": \"bar\"}"},
		{"```json\n{\"foo\": \"bar\"}```", "{\"foo\": \"bar\"}"},
		{"```json\n\n```", ""},
		{"", ""},
		{"Here is the JSON:\n```json\n{\"foo\": \"bar\"}\n```", "{\"foo\": \"bar\"}"},
		{"```\n{\"foo\": \"bar\"}\n```\nSome trailing explanation.", "{\"foo\": \"bar\"}"},
		{"Some prefix\n```\n{\"foo\": \"bar\"}\n```\nSome suffix", "{\"foo\": \"bar\"}"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := cleanJSONText(tc.input)
			if got != tc.expected {
				t.Errorf("cleanJSONText(%q) = %q; expected %q", tc.input, got, tc.expected)
			}
		})
	}
}
