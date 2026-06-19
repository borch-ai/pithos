# plan: Task 5.29: Imagegen Backend Capability Validation

**Status:** Pending
**Go Version:** 1.26.4
**Unit Test Coverage:** 91.0% (Target)

This task integrates the capability handshake in Pithos's brew pipeline. Before bootstrapping the character reference image or generating stanzas illustrations, Pithos will query the `pw-mcp-imagegen` server to ensure the active backend supports the features required by the book properties (such as character references/seeding).

## User Review Required

> [!IMPORTANT]
> **Cost Prevention:** By failing fast during the initialization handshake, we avoid wasting time and API costs (typically $0.04/image) on generating and uploading the character seed portrait when the active backend cannot utilize it.

---

## Proposed Changes

### Brew Engine

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Define a capability check helper `checkBackendCapabilities` in `brew.go`:
  ```go
  type imagegenCapabilities struct {
      Backend      string `json:"backend"`
      SupportsCref bool   `json:"supports_cref"`
      SupportsSref bool   `json:"supports_sref"`
  }
  ```
- In `generateIllustrations` (right after starting the `mcpClient` in [brew.go](file://../../internal/pipeline/brew.go)), call the `imagegen_get_capabilities` tool.
- Unmarshal the capability response:
  * If a character profile is defined (`m.BookProperties.CharacterProfile != ""`) but the active backend does not support character references (`SupportsCref == false`), halt and return a descriptive error:
    `"active imagegen backend [%s] does not support character references, but a character profile is defined; switch backend to midjourney or clean manifest character properties"`
- Ensure this validation is performed *before* calling `bootstrapCharacterReference` to prevent generating and uploading the character seed portrait when it will be ignored.

#### [MODIFY] [doctor.go](file://../../internal/pipeline/doctor.go)
- In the `DiagnoseMCPPlugins` function, during the `pw-mcp-imagegen` diagnostics check:
  * If the connection handshake succeeds, call `imagegen_get_capabilities`.
  * Append capability status to the `DiagnosticItem` output message (e.g. `"Connected successfully. Active backend: [google] (cref: UNSUPPORTED, sref: UNSUPPORTED)"`).
  * If the configured backend does not support `cref` but a character profile check in local workspaces shows character seeding is requested, flag it as a `WARNING` in the doctor report.

---

## Verification Plan

### Automated Tests
- Mock the `imagegen_get_capabilities` tool in the unit tests in `brew_test.go` to return different backends/capabilities.
- Verify that Pithos errors out correctly if cref is required but unsupported.
- Assert overall statement coverage meets the strict 91% threshold (`make check-coverage`).

### Manual Verification
- Configure `pithos` to use the `google` backend (which ignores cref) in `.env` / `.pithos.toml`.
- Run the brew command:
  ```bash
  ./bin/pithos brew --output books/cyber_diogenes
  ```
- Verify the process halts immediately with the descriptive capability error before generating any images.
