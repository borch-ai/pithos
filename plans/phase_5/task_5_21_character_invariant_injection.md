# plan: Task 5.21: Character Invariant Injection

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** June 18, 2026
**Unit Test Coverage:** 91.30%

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
To ensure high-fidelity and consistent character generation across the pipeline, perform the following verification process:

1. **Initiate a New Book Workspace**:
   - Run the initiate command with a specific theme:
     ```bash
     ./bin/pithos initiate --theme "A grumpy little frog who lost his pond" --output books/frog_book
     ```
2. **Define the Character Profile**:
   - Open `books/frog_book/manifest.json` and set a descriptive, high-quality character profile under `book_properties`:
     ```json
     "character_profile": "A small, chubby, grumpy-looking green tree frog wearing a tiny yellow raincoat and a red scarf, cartoon/storybook illustration style"
     ```
   - Ensure `character_weight` is set (e.g., to the default `100` or a customized value like `80` to allow scene flexibility).
3. **Execute the Brew Pipeline**:
   - Run the brew pipeline using active MCP servers:
     ```bash
     ./bin/pithos brew --output books/frog_book
     ```
4. **Verify Bootstrapping & Character Seed Creation**:
   - Verify that the pipeline first generates a dedicated character seed portrait.
   - Inspect the created image at `books/frog_book/images/character_seed.png` to confirm it accurately captures the character description (the grumpy green frog in a yellow raincoat).
5. **Verify Cloud Upload**:
   - Check the console logs for the upload confirmation through the `pw-mcp-cloud` server.
   - Open `books/frog_book/manifest.json` and verify that `character_reference_url` has been updated with a valid public URL pointing to the uploaded seed image.
6. **Verify Character Consistency in Story Pages**:
   - Inspect the generated page images in `books/frog_book/images/`.
   - Confirm that the character's core visual features (grumpy expression, green color, yellow raincoat, red scarf) remain consistent and recognizable across different environments described in the stanzas.
   - Open the local preview page `books/frog_book/web_preview/preview.html` in a web browser to check the final layout and page-to-page visual flow.
7. **Test Character Weight Overrides**:
   - Select a specific page (e.g., page 3) in `books/frog_book/manifest.json` where the character might need to be less prominent or rendered differently.
   - Add a page-level override in that page's JSON entry:
     ```json
     "character_weight": 30
     ```
   - Re-run the brew command for that specific page:
     ```bash
     ./bin/pithos brew --output books/frog_book --pages 3
     ```
   - Verify that the image generation request for page 3 used a weight of `30` (visible via debug logs or by examining the prompt/params sent to `imagegen`) and check if the character is less strictly bound to the reference image.

### CLI Initiate Directory Safety & Overwrite Prompts
- Added directory unique-ification logic in `internal/pipeline/initiate.go` via `GetUniqueOutputDir`. If `pithos initiate` is called without `--output`, it generates a unique non-existent directory suffix (e.g. `books/book-1`, `books/book-2`) if the default `books/book` exists.
- Added overwrite protection via `ConfirmOverwrite`. If `--output` is explicitly passed and the target directory exists, it prompts the user to confirm. If rejected, it aborts initiation.
- Verified both helpers with unit tests in `internal/pipeline/pipeline_test.go` (`TestGetUniqueOutputDir` and `TestConfirmOverwrite`), achieving 100% statement coverage of the new code paths.


