# plan: Task 7.6: Speculative: Publish Pithos as Homebrew Package with Dependencies

**Status:** Speculative

This plan outlines packaging and publishing Pithos as a Homebrew package (`brew install pithos`), configuring the formula to automatically list and install all required Powerword MCP plugins as dependencies, allowing non-Go developers to run Pithos out-of-the-box.

## User Review Required

> [!IMPORTANT]
> - **Platform Support:** Pre-compiled binaries will be generated for macOS (Intel & Apple Silicon), Linux, and Windows (Windows is supported via direct binary download; Homebrew formula is installable on macOS/Linux only).
> - **Dependency Ordering:** The Homebrew tap formula for Pithos will declare dependencies on the Powerword plugin formulas to ensure they are fetched and linked first.

## Proposed Changes

### Release Workflow

#### [NEW] [release.yml](file://../../.github/workflows/release.yml)
- Create a post-merge release workflow in Pithos.
- Use `go-semantic-release/action` to version the code, tag it, and create GitHub Releases.
- Cross-compile Pithos binaries for macOS (amd64, arm64), Linux (amd64, arm64), and Windows (amd64).
- Upload the binaries as assets to GitHub Releases.

### Homebrew Tap Integration

#### [NEW] [pithos.rb](file://../../Formula/pithos.rb)
- Define the Homebrew formula in a new `Formula/pithos.rb` (or inside a shared `borch-ai/homebrew-tap` repo).
- Point the URL to the published GitHub release tarball.
- Define dependency declarations:
  ```ruby
  depends_on "pw-mcp-kdp-math"
  depends_on "pw-mcp-imagegen"
  depends_on "pw-mcp-seo"
  depends_on "pw-mcp-video"
  ```

---

## Verification Plan

### Manual Verification
- Test installing Pithos in a clean environment using:
  ```bash
  brew tap borch-ai/tap
  brew install pithos
  ```
- Confirm that both Pithos and all plugin dependencies are correctly installed and runnable from the terminal.
