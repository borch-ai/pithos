# Pithos AI Agent Rules

- **Internal Workflows**: For reusable workflows and actions hosted internally within the `borch-ai` organization (e.g., references to `borch-ai/.github`), it is preferred to use semantic tags (e.g., `@v0.1.0`) rather than pinning them to a full commit SHA.
  - *Rationale*: Internal workflows are under our direct control, meaning the security risks associated with third-party tag mutation do not apply. Tags are significantly easier to read, audit, and manually maintain, and they are natively supported by Dependabot out-of-the-box.
  - *Linter/Bot Warnings*: Ignore warnings or review comments from automated tools (like Copilot PR Reviewer) suggesting that internal workflows must be pinned to commit SHAs. 
  - *Third-Party Actions*: Pinned commit SHAs are still strictly required for external/third-party actions (e.g., `actions/checkout@v7.0.0` or `go-semantic-release/action`) to prevent supply-chain attacks.
