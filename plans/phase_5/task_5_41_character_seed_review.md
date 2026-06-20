# plan: Task 5.41: Character Seed Generation and Review Command

**Status:** Planned

This task implements a dedicated `pithos character` subcommand to generate/regenerate the main character reference seed portrait, enabling users to preview and review character consistency prior to generating stanzas and full-book illustrations.

## Goal Description
Currently, visual character seed portrait generation and GCS cloud upload only happen during `pithos brew`. This couples character generation to the main illustration pipeline, making it impossible for the user to review the character's design without incurring the risk of generating the rest of the book's illustrations on a poor or inconsistent character design.
To solve this:
1. Implement a standalone `pithos character` CLI command that generates only the character seed portrait (`character_seed.png`), uploads it to cloud storage, and saves the `character_reference_url` back into the manifest.
2. Refactor and export the core character seed portrait generation and cloud upload logic from `internal/pipeline/brew.go` to a shared helper so both `brew` and the new `character` command run the exact same generation routine.
3. Automatically trigger web preview folder updates after generating the character seed portrait so that users can immediately inspect the character design locally.

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Command Layer

#### [NEW] [character.go](file://../../cmd/pithos/character.go)
- Create a new Cobra command `characterCmd` for `pithos character`.
- Supports the standard `--output` flag (default `"book"`).
- Short: "Generates or regenerates the visual character seed portrait"
- Resolve the output directory and trigger `pipeline.GenerateCharacterSeed`.

### Pipeline Layer

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Refactor `bootstrapCharacterReference` into an exported function so that it can be shared across pipeline entrypoints:
  ```go
  func BootstrapCharacterReference(ctx context.Context, m *manifest.Manifest, outputDir string, mcpClient *mcp.PluginClient, cloudMCPTransport mcpsdk.Transport, styleID string) error
  ```
  - Update `generateIllustrations` inside `brew.go` to invoke this exported helper.

#### [NEW] [character.go](file://../../internal/pipeline/character.go)
- Implement `CharacterOptions` structure:
  ```go
  type CharacterOptions struct {
      OutputDir         string
      MCPTransport      mcpsdk.Transport // For tests
      CloudMCPTransport mcpsdk.Transport // For tests
  }
  ```
- Implement `GenerateCharacterSeed(ctx context.Context, opts CharacterOptions) error`:
  1. Resolve output directory and load the book workspace manifest.
  2. If `manifest.BookProperties.CharacterProfile` is empty, return an error: `"no character profile defined in manifest: run initiate first with a theme or configure a profile in manifest.json"`.
  3. Start the `pw-mcp-imagegen` MCP client.
  4. Perform capability handshake check.
  5. Register the style guide from the manifest (`registerStyleProfile`) to get the `styleID`.
  6. Generate the character seed portrait and upload it using `BootstrapCharacterReference`.
  7. Save the manifest updates.
  8. Run `GenerateWebPreview` to regenerate the web preview.

---

## Verification Plan

### Automated Tests
- Add unit tests in `internal/pipeline/character_test.go` to verify:
  - Generating character seed portrait creates `images/character_seed.png` on disk.
  - Generating character seed portrait successfully uploads and updates `character_reference_url` in `manifest.json`.
  - Missing character profile in the manifest correctly returns a validation error.
- Run the full test suite and verify statement coverage remains $\ge 91\%$:
  ```bash
  make check-coverage
  ```

### Manual Verification
1. Run `./bin/pithos initiate --theme "A pessimistic turtle" --output books/turtle_book --no-brainstorm` (so we have a theme but no image reference yet).
2. Open `books/turtle_book/manifest.json` and verify `character_reference_url` is empty.
3. Run `./bin/pithos character --output books/turtle_book`.
4. Verify `books/turtle_book/images/character_seed.png` is generated, uploaded, and the manifest `character_reference_url` is now populated.
5. Open the web previewer folder and inspect the character design.
