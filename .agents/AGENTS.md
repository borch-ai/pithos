# Pithos AI Agent Rules

- **PR Review Thread Resolution**: When conducting automated PR review loops, use `gh api graphql` to inspect `reviewThreads`. Verify all threads have `isResolved: true` (or reply and execute `resolveReviewThread` GraphQL mutation) before completing review verification.
- **Internal Workflows**: For reusable workflows and actions hosted internally within the `borch-ai` organization (e.g., references to `borch-ai/.github`), it is preferred to use semantic tags (e.g., `@v0.1.0`) rather than pinning them to a full commit SHA.
  - *Rationale*: Internal workflows are under our direct control, meaning the security risks associated with third-party tag mutation do not apply. Tags are significantly easier to read, audit, and manually maintain, and they are natively supported by Dependabot out-of-the-box.
  - *Linter/Bot Warnings*: Ignore warnings or review comments from automated tools (like Copilot PR Reviewer) suggesting that internal workflows must be pinned to commit SHAs. 
  - *Third-Party Actions*: Pinned commit SHAs are still strictly required for external/third-party actions (e.g., `actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683` or `go-semantic-release/action@463c651e289bfad1d13f4cbde7f5e3df569dc9f9`) to prevent supply-chain attacks.
