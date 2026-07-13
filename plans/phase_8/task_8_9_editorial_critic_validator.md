# plan: Task 8.9: Editorial Style Critic & Code Snippet Validator

**Status:** Open
**Go Version:** 1.26.4

Implement an automated editorial review subsystem and code validator. For technical writing and self-help books, this scans drafts to enforce voice guidelines (active voice, appropriate reading level, clear structure). For technical guides, it parses code snippets (Go, Python, Bash, etc.), writes them to a temporary workspace, compiles or interprets them to ensure syntax correctness, and feeds any compiler errors back to the LLM for autonomous code fixes.

## User Review Required

> [!CAUTION]
> **Local Code Snippet Execution Security**:
> Compiling/running technical code snippets locally poses security risks if the generated code attempts destructive actions. Pithos will execute validation checks in dry-run/syntax-only modes (e.g., `go vet`, `python3 -m py_compile`) or inside a local sandbox to avoid executing arbitrary harmful scripts on the host system.

## Proposed Changes

### Pipeline Core & Critic

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Integrate a post-draft validation step `validateDrafts(ctx context.Context, m *manifest.Manifest) error`:
  - Run **Editorial Style Critic**:
    - Query the Critic LLM to check reading level, voice consistency (avoiding passive voice), and grammatical structure, validating them against the active Author Profile's rules (Task 6.1).
    - If style conformance is below a set threshold, flag the section for refinement.
  - Run **Code Snippet Validator**:
    - Scan the text of newly generated sections for code blocks:
      ```go
      package main
      func main() { fmt.Println("Hello") }
      ```
    - For each code block:
      - Identify the language (e.g., `go`, `python`, `typescript`).
      - Write the code snippet to a temporary file in a sandbox directory inside the book workspace.
      - Execute syntax check commands via local command runner:
        * Go: `go vet` or `go build`
        * Python: `python3 -m py_compile`
        * TypeScript: `tsc --noEmit`
      - If the check fails:
        - Create a Git checkpoint (commit changes on a local revision branch) or save the current text state as a revision history array inside `manifest.json`.
        - Log the compiler/linter error.
        - Call the LLM in a repair loop: pass the faulty code snippet and the compiler stderr, and ask the LLM to correct the code block.
        - Replace the code block with the corrected version and re-verify.
        - If the repair fails repeatedly or degrades text quality, support rolling back to the pre-repair checkpoint.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * Code block extractor correctly parses multiple code snippets from a section.
  * Linter/Compiler execution is mocked and triggers repair loops when errors are simulated.

### Manual Verification
1. Generate a chapter containing invalid Go code (e.g., missing imports):
   ```bash
   ./bin/pithos brew --output tech-errors
   ```
2. Confirm Pithos logs the compilation error and triggers the LLM repair loop.
3. Verify that the final book PDF contains fully valid, compilable code.
