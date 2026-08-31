---
name: pr-review-loop
description: Standardized procedure for executing PR code review polling loops, querying GraphQL review threads, posting replies, and resolving comment threads via GitHub API.
---

# PR Review Loop & Thread Resolution Skill

This skill defines the mandatory protocol for AI agents to poll, track, reply to, and resolve code review comments during GitHub Pull Request execution loops.

## Protocol Workflow

### 1. Poll Review Threads via GraphQL

Do not rely solely on REST API comments. Use `gh api graphql` to inspect active review threads and verify `isResolved` state:

```bash
gh api graphql -f query='
query($owner: String!, $repo: String!, $prNumber: Int!) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $prNumber) {
      reviewThreads(first: 50) {
        nodes {
          id
          isResolved
          comments(first: 5) {
            nodes {
              id
              body
              path
              line
              author { login }
            }
          }
        }
      }
    }
  }
}' -F owner="borch-ai" -F repo="pithos" -F prNumber=<PR_NUMBER>
```

### 2. Apply Code Refactors & Push Fix Commit

For any thread with `isResolved: false`:
1. Modify local source/test files to address the reviewer's feedback.
2. Run validation locally (`gofmt -w . && make lint && make test`).
3. Commit and push the fixes to the feature branch.

### 3. Post Thread Reply & Resolve Thread

If a review thread remains unresolved after pushing the fix commit, post a reply comment to the comment thread and invoke the `resolveReviewThread` GraphQL mutation:

```bash
# Post reply comment referencing commit SHA
gh api /repos/borch-ai/pithos/pulls/<PR_NUMBER>/comments/<COMMENT_ID>/replies -f body="Addressed in commit <SHA>."

# Execute GraphQL resolution mutation
gh api graphql -f query='
mutation($threadId: ID!) {
  resolveReviewThread(input: {threadId: $threadId}) {
    thread {
      id
      isResolved
    }
  }
}' -F threadId="<THREAD_ID>"
```

### 4. Verification Check

Re-query the GraphQL endpoint to verify `unresolved_count == 0` for all review threads on the latest commit before completing the review loop.
