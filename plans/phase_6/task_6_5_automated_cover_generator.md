# plan: Task 6.5: Automated Cover Art & Title Layout Generator

**Status:** Complete
**Go Version:** 1.26+

Automates parodic book cover generation by creating a unified workflow that queries the LLM for cover art ideas, generates a high-resolution cover illustration matching the style guide, compiles KDP geometry specifications (margins, bleed, spine width), and produces a print-ready PDF cover wrap.

## User Review Required

> [!NOTE]
> All Task 6.5 implementation requirements have been implemented and verified. Unit test coverage meets the strict 91% requirement (achieved 91.2%), and linter checks pass cleanly with 0 issues.

## Proposed Changes

### LLM Layer

#### [MODIFY] [llm.go](file://../../internal/pipeline/llm.go)
- Added `CoverDesign` struct:
  ```go
  type CoverDesign struct {
      Title          string `json:"title"`
      Subtitle       string `json:"subtitle"`
      Author         string `json:"author"`
      BackCoverBlurb string `json:"back_cover_blurb"`
      CoverPrompt    string `json:"cover_prompt"`
  }
  ```
- Extended `LLMClient` interface with `GenerateCoverDesign`:
  ```go
  GenerateCoverDesign(ctx context.Context, theme, style, characterProfile string) (*CoverDesign, telemetry.TokenUsage, error)
  ```
- Implemented `GenerateCoverDesign` in `PowerwordClientAdapter` parsing structured JSON.

### Manifest Schema

#### [MODIFY] [manifest.go](file://../../internal/manifest/manifest.go)
- Added `Title`, `Subtitle`, `Author`, `BackCoverBlurb`, and `CoverPrompt` to `manifest.BookProperties`.
- Bumped `CurrentSchemaVersion` to 3 and registered sequential migration `migrateV2ToV3`.

### CLI & Workspace Initiation

#### [MODIFY] [initiate.go](file://../../cmd/pithos/initiate.go)
#### [MODIFY] [initiate.go](file://../../internal/pipeline/initiate.go)
- Added optional `--title` and `--author` CLI flags to allow user overrides at initiation time.
- Bound flags to `InitiateOptions` and saved into `manifest.BookProperties`.

### Pipeline Brew Engine

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- In `generateManuscript`: automatically generates missing `Title`, `Subtitle`, `Author`, `BackCoverBlurb`, and `CoverPrompt` via LLM or simulated fallback.
- In `generateIllustrations`: added `needCover` evaluation. Generates `images/cover.png` using the active MCP ImageGen client and registered style profile.
- Reuses active MCP client sessions, preventing redundant socket reconnections or EOF errors on in-memory test transports.
- Fully supports `--dry-run` simulation mode with dummy PNG generation and telemetry tracking.

### Print Layout & PDF Assembly Engine

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
- Added `compileCoverPDF` calling `pw-mcp-typst` tool `"compile_cover"`.
- Passes KDP spine width, bleed, trim size, paper type, front cover image path, title, subtitle, author, and back cover blurb.
- Records compiled cover wrap PDF in `AssetRegistry["cover_pdf"]` and updates `Kiln.CoverPDFPath`.
- Reuses active typst client session across interior and cover compilation.

### Web Previewer

#### [MODIFY] [preview.go](file://../../internal/pipeline/preview.go)
- Added Cover Wrap tab and spread view in `web_preview/preview.html`.
- Dynamically visualizes back cover (with back cover blurb), spine (with vertical title), and front cover art (with title/author overlay).
- Draws visual KDP wrap bleed guidelines and safety margins using geometry values stored in `manifest.json`.

---

## Verification Plan

### Automated Tests
- Unit test coverage verification:
  ```bash
  make check-coverage
  ```
  Result: Achieved 91.2% statement coverage (exceeds 91.0% requirement).
- Linter verification:
  ```bash
  make lint
  ```
  Result: 0 issues.
- Direct test execution across packages:
  ```bash
  go test -v ./...
  ```

### Manual Verification
- End-to-end dry-run CLI test:
  1. `go run ./cmd/pithos initiate --output test_book_cover --title "The Sisyphus Sprint" --author "Dr. Cynic" --theme "Sprint Burnout" --pages 4 --dry-run`
  2. `go run ./cmd/pithos brew --output test_book_cover --dry-run` -> verified `images/cover.png` created.
  3. `go run ./cmd/pithos assemble --input test_book_cover --dry-run` -> verified `cover.pdf` created.
  4. `go run ./cmd/pithos status test_book_cover` -> verified all milestones and telemetry updated cleanly.
