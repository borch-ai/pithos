# plan: Task 5.21: Character Invariant Injection

**Status:** Open
**Go Version:** 1.26.4
**Date Completed:** —
**Unit Test Coverage:** —

This task implements character visual consistency during the `brew` pipeline. It does this via a hybrid consistency model:
1. **Text Invariant Prepending:** Prepend the global `character_profile` (from manifest or review manuscript) to each page's illustration prompt in memory before generating the image.
2. **Native Image-Based Reference (`--cref`):** Pass `character_reference_url` and `character_weight` to the `pw-mcp-imagegen` server. If no URL exists, generate a character seed image, upload it via the `pw-mcp-cloud` server using the `cloud_upload_file` tool to obtain a public URL, and set that URL as the reference.

## User Review Required

> [!NOTE]
> **Orchestrator Responsibility:** Pithos is responsible for coordinating file uploads by invoking the `pw-mcp-cloud` server. The `pw-mcp-imagegen` server remains decoupled from cloud storage libraries and only receives public remote URLs.

---

## Proposed Changes

### Manifest & Schema

#### [MODIFY] [manifest.go](file://../../internal/manifest/manifest.go)
- [ ] Add `CharacterReferenceURL` (string) and `CharacterWeight` (int, default `100`) to the `BookProperties` struct. Mapped as `character_reference_url` and `character_weight`.
- [ ] Add `CharacterWeight` (optional pointer to int) to `PageState` to support per-page weight overrides.

### Configuration & MCP Client

#### [MODIFY] [config.go](file://../../internal/config/config.go)
- [ ] Add `CloudPath` (string) to `MCPConfig` struct mapped to `cloud_path`.
- [ ] Add validation/fallback resolution for `CloudPath` using `"pw-mcp-cloud"`.

#### [MODIFY] [client.go](file://../../internal/mcp/client.go)
- [ ] Add `PluginCloud PluginType = "pw-mcp-cloud"` to plugin constants.
- [ ] Update `ResolveBinaryPath` to return `config.Cfg.MCP.CloudPath` when resolving `PluginCloud`.

### Diagnostics Doctor

#### [MODIFY] [doctor.go](file://../../internal/pipeline/doctor.go)
- [ ] Add `PluginCloud` to the `plugins` slice in `DiagnoseMCPPlugins` (marking it as non-critical/optional) so its presence and connection handshake are verified by `pithos doctor`.

### Brew Engine

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- [ ] Implement a bootstrapping routine at the start of `generateIllustrations`:
  *   If `m.BookProperties.CharacterReferenceURL` is empty and `m.BookProperties.CharacterProfile` is not empty:
      1. Generate a single character seed portrait using the profile (e.g. prompt: `"Detailed visual seed character portrait: " + m.BookProperties.CharacterProfile`).
      2. Save it to `images/character_seed.png` inside the book output folder.
      3. Initialize the `pw-mcp-cloud` client.
      4. Call the `cloud_upload_file` tool, passing the absolute path to `images/character_seed.png`.
      5. Capture the returned public URL and record it in `m.BookProperties.CharacterReferenceURL`.
- [ ] During the illustration generation loop for each page:
  *   Construct the target prompt by prepending the textual `m.BookProperties.CharacterProfile` (if not empty) to the page's specific prompt.
  *   Retrieve the applicable `character_weight` (checking the page-specific override first, falling back to the book's global setting).
  *   Call `imagegen_generate`, passing `cref_url` (from the manifest) and `character_weight`.

---

## Verification Plan

### Automated Tests
- Run `go test ./internal/config/...`, `go test ./internal/mcp/...` and `go test ./internal/pipeline/...`
- Ensure overall coverage meets or exceeds 91% (`make check-coverage`).

### Manual Verification
1. Initiate a new book: `pithos initiate --theme "The Cynical Bear"`
2. Set a `character_profile` in `manifest.json`.
3. Run `pithos brew` with stubbed MCP servers. Verify `images/character_seed.png` is generated, uploaded via `pw-mcp-cloud`, and the resulting URL is passed to `pw-mcp-imagegen`.
