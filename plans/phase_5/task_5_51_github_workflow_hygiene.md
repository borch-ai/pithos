# plan: Task 5.51: Adopt Aeolian's GitHub Workflow Hygiene in Pithos

**Status:** Completed (Issue #62)
**Date Completed:** 2026-07-13
**Unit Test Coverage:** 92.2%
**Go Version:** 1.26.5

## User Review Required

> [!NOTE]
> **GitHub Repository Rulesets**: The branch naming ruleset configuration file `branch_naming_rules.json` will be added to `.github/rulesets/`. To enforce this ruleset, a repository administrator must manually import or configure it under the repository settings on GitHub.

> [!IMPORTANT]
> **CodeQL Workflow Sibling Check**: Pithos requires `powerword` as a sibling checkout to compile correctly due to the `replace` directive in its `go.mod`. We will pin the CodeQL workflow using the shared `codeql-with-sibling.yml` workflow instead of Aeolian's standalone `codeql.yml`.

## Open Questions

None.

## Proposed Changes

### GitHub Workflows and Configurations

***

#### [MODIFY] [dependabot.yml](file://../../.github/dependabot.yml)
Update dependabot configuration to run daily (instead of weekly) and inject commit message prefixes and scopes to match Aeolian's hygiene.

#### [MODIFY] [pull_request_template.md](file://../../.github/pull_request_template.md)
Update the pull request template to integrate Aeolian's overview and issue auto-linking notes while retaining Pithos's mandatory 91% coverage and `make lint` verification checklist.

#### [NEW] [branch_naming_rules.json](file://../../.github/rulesets/branch_naming_rules.json)
Create branch naming enforcement rule allowing prefixes `feature/`, `docs/`, `bugfix/`, `chore/`, `refactor/`, and `test/`.

#### [MODIFY] [ci.yml](file://../../.github/workflows/ci.yml)
Pin the reusable `go-ci-with-sibling.yml` workflow version to `@6fd0a9a05fa00d21159402147a3445af04a191c5` (v1.0.0) and explicitly set the `coverage-threshold` to `91`.

#### [MODIFY] [codeql.yml](file://../../.github/workflows/codeql.yml)
Pin the reusable `codeql-with-sibling.yml` workflow version to `@6fd0a9a05fa00d21159402147a3445af04a191c5` (v1.0.0).

#### [MODIFY] [link-task-issue.yml](file://../../.github/workflows/link-task-issue.yml)
Refactor to execute the local `link_task_issue.js` script on `pull_request_target` events to parse modified plan files for active issue IDs securely from the base branch context.

#### [NEW] [link_task_issue.js](file://../../.github/workflows/link_task_issue.js)
Copy the node script that extracts issue IDs from modified plans at the PR's HEAD SHA (fetched via `git fetch` and read using `git show`) and appends them to the PR description via the GitHub CLI. PR description text is deliberately not echoed to CI logs to avoid leaking sensitive credentials.

#### [NEW] [markdown-lint.yml](file://../../.github/workflows/markdown-lint.yml)
Introduce a markdown lint workflow triggered on `.md` changes and configuration updates, using the pinned shared workflow version.

#### [NEW] [pr-description.yml](file://../../.github/workflows/pr-description.yml)
Introduce a PR description validation workflow using the pinned shared workflow version.

#### [MODIFY] [pr-title.yml](file://../../.github/workflows/pr-title.yml)
Pin the reusable `pr-title.yml` workflow version to `@6fd0a9a05fa00d21159402147a3445af04a191c5` (v1.0.0).

#### [MODIFY] [release.yml](file://../../.github/workflows/release.yml)
Align with Aeolian's release workflow by explicitly checking out and installing the powerword sibling first, and running linting via `make lint`.

#### [NEW] [.markdownlint-cli2.jsonc](file://../../.markdownlint-cli2.jsonc)
Create markdown linter configuration to ignore all plan files except for the active one.

#### [NEW] [task_5_51_github_workflow_hygiene.md](file://task_5_51_github_workflow_hygiene.md)
Save this implementation plan under the repository's `plans/` directory to document and track the task completion.

## Verification Plan

### Automated Tests
- Run `make check-coverage` to verify local unit tests pass with coverage $\ge$ 91%.
- Run `make lint` to verify Go codebase formatting and quality checks pass.
- Validate workflow YAML syntax.
- Validate `link_task_issue.js` syntax.
