# plan: Task 6.2: Multi-Model Illustration Variations & Selection

**Status:** Open
**Go Version:** 1.26.4

Implement multi-model image generation and selection to enable creators to generate multiple visual candidates per page (using different image generation models like DALL-E 3, Flux, or Stable Diffusion) and select the best one during the review process. This updates the orchestrator to launch parallel model calls, register all variations in the manifest, expose them in the review loops, and copy the selected variation to the final layout assets path.

## User Review Required

> [!NOTE]
> **Storage & Telemetry Cost Impacts**:
> Generating multiple image variations per page will multiply local storage consumption (saving each model's output) and image generation API costs. The multi-model generation behaves as an opt-in feature configured in `.pithos.toml`. If only a single model is configured, Pithos will skip candidate variation generation and operate in standard mode.

## Proposed Changes

### Manifest Layer

#### [MODIFY] [manifest.go](file://../../internal/manifest/manifest.go)
- Update `PageState` struct to store alternative candidate variations:
  ```go
  type PageState struct {
      PageIndex          int        `json:"page_index"`
      Status             PageStatus `json:"status"`
      ImagePath          string     `json:"image_path,omitempty"` // The currently active/selected image
      Text               string     `json:"text,omitempty"`
      IllustrationPrompt string     `json:"illustration_prompt,omitempty"`
      
      // Multi-model variations tracking
      Variations         map[string]string `json:"variations,omitempty"` // Map of model_name -> relative_image_path
      SelectedModel      string            `json:"selected_model,omitempty"`
  }
  ```

### Configuration Layer

#### [MODIFY] [config.go](file://../../internal/config/config.go)
- Add image generation model configurations:
  ```toml
  [imagegen]
  models = ["dall-e-3", "flux-schnell"]
  default_model = "dall-e-3"
  ```

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- In `generateIllustrations`:
  - Read configured models list from `config.Cfg.ImageGen.Models`.
  - For each pending page, spawn concurrent worker tasks for **each** configured model.
  - Call the MCP client `imagegen_generate` tool passing the specific `model` argument.
  - Save output files as `images/page_<index>_<model>.png` (e.g. `images/page_1_dall-e-3.png` and `images/page_1_flux-schnell.png`).
  - Store paths in the page's `Variations` map in the manifest.
  - Set the initial `ImagePath` to the default model's output file, copying it to `images/page_<index>.png`.
- In `exportManuscriptToMarkdown`:
  - Append variation details as comments under each page section in `manuscript.md`:
    ```markdown
    # Page 1
    ...
    <!-- Variations: dall-e-3=images/page_1_dall-e-3.png | flux-schnell=images/page_1_flux-schnell.png -->
    <!-- SelectedModel: dall-e-3 -->
    ```
- In `importManuscriptFromMarkdown`:
  - Parse `<!-- SelectedModel: <name> -->` from each page block.
  - If the selected model has changed:
    - Update `SelectedModel` in the manifest.
    - Copy the selected model's candidate file (e.g., `images/page_1_flux-schnell.png`) to the final active layout path `images/page_1.png`.
    - Update `ImagePath` to `images/page_1.png` and save.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * Worker loop successfully spawns multiple image generation calls per page when multiple models are configured.
  * Image variations map is correctly updated in the manifest.
  * Changing the selected model via manuscript import updates the active `ImagePath` and copies the corresponding variation file.
