# plan: Task 5.6: Unified LLM Integration

**Status:** Open (Issue #[TBD])

Refactor the Pithos LLM client layer to consume the refactored `github.com/borch-ai/powerword/pkg/llm` package, eliminating the local raw HTTP REST implementations and unifying provider communication logic.

## User Review Required

None. This is a clean internal refactoring to unify the LLM layer using the shared Powerword LLM package.

## Proposed Changes

### Pipeline Abstraction

#### [MODIFY] [llm.go](../../internal/pipeline/llm.go)
- Import `github.com/borch-ai/powerword/pkg/llm`.
- Define an adapter structure `PowerwordClientAdapter` that implements Pithos's local `LLMClient` interface:
  ```go
  type PowerwordClientAdapter struct {
      client    llm.LLMClient
      modelName string
  }
  ```
- Implement `GenerateStanzas` on the adapter to:
  * Build the user message with the parody generation prompt.
  * Call `client.Generate(ctx, messages, nil, llm.WithResponseMIMEType("application/json"))`.
  * Parse the returned JSON response text into stanzas.
  * Extract and return `telemetry.TokenUsage` from the response `msg.Usage`.
- Modify client factories to return the adapter wrapping either Gemini or OpenAI clients.

### Pipeline Runner

#### [MODIFY] [brew.go](../../internal/pipeline/brew.go)
- Remove manual type assertions of LLM client concrete types and use the model name returned by the client adapter or configuration directly.

---

## Verification Plan

### Automated Tests
- Run `go test ./internal/pipeline/...`
- Update pipeline tests to verify that the adapter correctly passes formatting requirements and captures token metrics from the underlying Powerword LLM client.
