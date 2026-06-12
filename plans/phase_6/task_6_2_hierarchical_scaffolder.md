# plan: Task 6.2: Hierarchical Outline-Driven Book Scaffolder

**Status:** Open
**Go Version:** 1.26.4

Implement a multi-tier hierarchical outline generator to support serious long-form content (like self-help, technical writing, and sci-fi/fantasy series) that exceeds single-prompt context limits. This workflow separates book creation into four logical stages: Series Outline, Volume Outline, Chapter Outlines, and Section Drafts, loaded dynamically from the active Author Profile (Task 6.1).

## User Review Required

> [!NOTE]
> **Incremental Construction Flow**:
> The orchestrator will require outlines to be saved to the manifest and approved or reviewed before text drafts are started. This prevents narrative drift and excessive token spend on incorrect directions.

## Proposed Changes

### Manifest Layer

#### [MODIFY] [manifest.go](file://../../internal/manifest/manifest.go)
- Add new properties to structure hierarchies:
  ```go
  type ChapterState struct {
      ChapterIndex int            `json:"chapter_index"`
      Title        string         `json:"title"`
      Summary      string         `json:"summary"`
      Sections     []SectionState `json:"sections"`
      Status       PageStatus     `json:"status"`
  }

  type SectionState struct {
      SectionIndex int        `json:"section_index"`
      Title        string     `json:"title"`
      TargetWords  int        `json:"target_words"`
      Text         string     `json:"text,omitempty"`
      Status       PageStatus `json:"status"`
  }
  
  type BookProperties struct {
      Genre            string         `json:"genre"` // Loaded from AuthorProfile
      Theme            string         `json:"theme"`
      Style            string         `json:"style,omitempty"`
      CharacterProfile string         `json:"character_profile,omitempty"`
      TargetPageCount  int            `json:"target_page_count"`
      
      // Hierarchical tracking
      Chapters         []ChapterState `json:"chapters,omitempty"`
  }
  ```

### CLI Command Layer

#### [MODIFY] [initiate.go](file://../../cmd/pithos/initiate.go)
- In addition to `--author`, read the target genre from the loaded Author Profile configuration.
- If a serious genre is selected, write a hierarchical skeleton in the manifest.

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Implement `generateOutline(ctx context.Context, m *manifest.Manifest) error`:
  - Query LLM to generate a structured book outline (JSON containing chapters, titles, section breakdowns, and target word counts).
  - Run **Semantic Overlap Analysis**:
    - Query the LLM or run a semantic similarity checker to identify adjacent chapters with high thematic overlap (e.g. Chapter 2's Middleman introduction and Chapter 3's Proxy-Shine).
    - If overlaps are detected, add warning comments and merge recommendations inside `outline.md` (e.g., `<!-- Suggestion: Merge Chapters 2 and 3 into a single unified chapter to prevent redundancy. -->`).
    - If accepted during review, consolidate the chapters in `manifest.json`.
  - Save outline structures to `m.BookProperties.Chapters`.
  - Export outline structures to `outline.md` for user review.
- Implement `generateChapters(ctx context.Context, m *manifest.Manifest) error`:
  - For each chapter and section, pass the outline constraints, previous section text (for narrative flow), and terminology guides to the LLM to generate the content incrementally.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * Recursive outline parser successfully constructs `Chapters` and `Sections` arrays from mock LLM responses.
  * Hierarchical manifest is correctly updated and saved.

### Manual Verification
1. Initiate a serious technical book:
   ```bash
   ./bin/pithos initiate --output exec-life --theme "Life as an engineering executive" --author sinope
   ```
2. Run brew to generate the outline:
   ```bash
   ./bin/pithos brew --output exec-life --review
   ```
3. Inspect `books/exec-life/outline.md` to confirm the generated chapter structure matches the executive theme.
