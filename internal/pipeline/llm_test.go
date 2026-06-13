package pipeline

import (
	"context"
	"errors"
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

	stanzas, prompts, usage, err := adapter.GenerateStanzas(context.Background(), "theme", 3)
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

	_, _, _, err := adapter.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestPowerwordClientAdapter_GenerateStanzas_EmptyResponse(t *testing.T) {
	mockClient := &mockPWClient{response: nil}
	adapter := &PowerwordClientAdapter{client: mockClient, modelName: "test-model"}

	_, _, _, err := adapter.GenerateStanzas(context.Background(), "theme", 3)
	if err == nil {
		t.Error("expected error for nil response message, got nil")
	}

	mockClientEmpty := &mockPWClient{response: &llm.Message{Content: ""}}
	adapterEmpty := &PowerwordClientAdapter{client: mockClientEmpty, modelName: "test-model"}
	_, _, _, err = adapterEmpty.GenerateStanzas(context.Background(), "theme", 3)
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

	_, _, _, err := adapter.GenerateStanzas(context.Background(), "theme", 3)
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

	_, _, _, err := adapter.GenerateStanzas(context.Background(), "theme", 3)
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

	_, _, _, err := adapter.GenerateStanzas(context.Background(), "theme", 3)
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
