# plan: Task 5.50: Automated CI/CD & GoReleaser Pipeline

**Status:** Proposed

## User Review Required
> [!NOTE]
> Review the target platforms for the GoReleaser builds. I propose targeting macOS (darwin) `arm64`/`amd64`, Linux `amd64`/`arm64`, and Windows `amd64`. Also, ensure you are comfortable with GitHub Actions automatically enforcing the 91% coverage rule on Pull Requests.

## Proposed Changes

### 1. Continuous Integration (CI) Workflow
Create `.github/workflows/ci.yml` to run automatically on every Pull Request to `main`.
- **Linting**: Run `golangci-lint` to enforce style and security guidelines.
- **Testing & Coverage**: Execute `make check-coverage` to run the test suite and verify that overall test coverage remains strictly at or above the required **91% threshold**.
- **Result**: The PR will be blocked from merging if the tests fail, the linter catches an issue, or coverage drops below 91%.

### 2. GoReleaser Configuration
Create `.goreleaser.yaml` in the project root to define the cross-compilation matrix and release packaging.
- **Builds**:
  - `os`: darwin (macOS), linux, windows
  - `arch`: amd64, arm64
- **Binary Output**: Compile the `pithos` binary and inject versioning information via `-ldflags` using git tags/commits.
- **Archives**: Package binaries as `tar.gz` for macOS/Linux and `.zip` for Windows. Include `README.md` and `LICENSE`.
- **Checksums**: Auto-generate a `checksums.txt` file (SHA256) for all artifacts.

### 3. Continuous Deployment (CD) Workflow
Create `.github/workflows/release.yml` to trigger automatically when a new version tag (e.g., `v1.0.0`) is pushed to GitHub.
- **Environment**: Needs `GITHUB_TOKEN` with write permissions to create a release.
- **Execution**: 
  - Checks out code and sets up Go.
  - Runs the test suite one final time.
  - Uses `goreleaser/goreleaser-action` to build binaries and publish a new GitHub Release with the artifacts and changelog attached.

## Verification Plan
1. Create a dummy PR and ensure `.github/workflows/ci.yml` successfully runs `make check-coverage`.
2. Push a dummy tag (e.g., `v0.0.1-alpha`) locally and run `goreleaser release --snapshot --clean` to verify the cross-compilation paths and `.goreleaser.yaml` correctness without interacting with the live GitHub API.
