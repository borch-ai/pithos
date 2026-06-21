# plan: Task 5.41: Character Seed Generation and Review Command
 
**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-21
**Unit Test Coverage:** 91.2%

This task implements a dedicated `pithos character` subcommand to generate/regenerate the main character reference seed portrait, enabling users to preview and review character consistency prior to generating stanzas and full-book illustrations. It adopts a dual-model configuration structure so that character seeds are generated cleanly using still-image models, while pages can be generated using video models.

## Goal Description
Currently, visual character seed portrait generation and GCS cloud upload only happen during `pithos brew`. This couples character generation to the main illustration pipeline, making it impossible for the user to review the character's design without incurring the risk of generating the rest of the book's illustrations on a poor or inconsistent character design.
Additionally, utilizing video models (like Veo) directly for still character references results in compression artifacts being baked into the seed and subsequently propagated/amplified across all generated pages.
To solve this:
1. Adopt a **dual-model configuration** in Pithos: `character_backend` (for still image reference seeds) and `illustration_backend` (for book pages).
2. Implement a standalone `pithos character` CLI command that queries the capabilities of the configured `character_backend`, validates that the backend returns `output_type: "image"`, generates the character seed portrait (`character_seed.png`), uploads it to cloud storage, and saves the `character_reference_url` back into the manifest.
3. Refactor and export the core character seed portrait generation and cloud upload logic from `internal/pipeline/brew.go` to a shared helper so both `brew` and the new `character` command run the exact same generation routine using the `character_backend`.
4. Update the page illustration pipeline (`pithos brew`) to query the capabilities of `illustration_backend` and route page generation requests to it, passing the high-quality static GCS `character_reference_url` for consistency.

## User Review Required

> [!NOTE]
> **Cross-Repository Dependency**: This plan relies on **Powerword Task 4.15** to expose `output_type` from the imagegen MCP capabilities endpoint.

## Proposed Changes

### Configuration Layer

#### [MODIFY] [config.go](file://../../internal/config/config.go)
- Add dual-model configuration parameters to config structure:
  ```go
  type MCPConfig struct {
      // ...
      CharacterBackend   string `mapstructure:"character_backend"`
      IllustrationBackend string `mapstructure:"illustration_backend"`
  }
  ```
- Bind defaults in `LoadConfig`:
  - `mcp.character_backend` defaults to `"imagen"` (still image)
  - `mcp.illustration_backend` defaults to `"imagen"` (still image)
- Bind environment variables: `PITHOS_MCP_CHARACTER_BACKEND` and `PITHOS_MCP_ILLUSTRATION_BACKEND`.

### Command Layer

#### [NEW] [character.go](file://../../cmd/pithos/character.go)
- Create a new Cobra command `characterCmd` for `pithos character`.
- Supports the standard `--output` flag (default `"book"`).
- Short: "Generates or regenerates the visual character seed portrait using the configured image model"
- Resolve the output directory and trigger `pipeline.GenerateCharacterSeed`.

### Pipeline Layer

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Refactor `bootstrapCharacterReference` into an exported function:
  ```go
  func BootstrapCharacterReference(ctx context.Context, m *manifest.Manifest, outputDir string, mcpClient *mcp.PluginClient, cloudMCPTransport mcpsdk.Transport, styleID string, backend string) error
  ```
  - Pass the explicit `character_backend` config value to it.
  - Update `generateIllustrations` inside `brew.go` to invoke this exported helper.
- Update `generateIllustrations` to query the capabilities of the configured `illustration_backend` and run the generation loop targeting it.

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
  2. If `manifest.BookProperties.CharacterProfile` is empty, return an error.
  3. Start the `pw-mcp-imagegen` MCP client.
  4. Perform capabilities handshake for `character_backend` and verify `output_type == "image"`. If `output_type == "video"`, fail fast with a descriptive error.
  5. Register style guide and generate character seed using `BootstrapCharacterReference`.
  6. Save the manifest updates and run `GenerateWebPreview` to refresh the web preview.

---

## Verification Plan

### Automated Tests
- Add unit tests in `internal/pipeline/character_test.go` to verify:
  - Generating character seed portrait using an image backend creates `images/character_seed.png` on disk.
  - Trying to generate a character seed portrait with a video backend fails fast with a capabilities validation error.
- Run the full test suite and verify statement coverage remains $\ge 91\%$:
  ```bash
  make check-coverage
  ```

### Manual Verification
1. Run `./bin/pithos initiate --theme "A pessimistic turtle" --output books/turtle_book --no-brainstorm`.
2. Configure `character_backend = "veo"` in config, run `./bin/pithos character --output books/turtle_book`, and verify it fails fast.
3. Configure `character_backend = "imagen"` in config, run `./bin/pithos character --output books/turtle_book`, and verify it succeeds and generates `books/turtle_book/images/character_seed.png`.
4. Open the web previewer folder and inspect the character design.
