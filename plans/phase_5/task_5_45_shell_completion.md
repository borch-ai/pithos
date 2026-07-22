# plan: Task 5.45: Shell Autocompletion Support (Zsh/Oh-My-Zsh)

**Status:** Completed
**Date Completed:** 2026-07-22
**Unit Test Coverage:** 91.0%
**Go Version:** 1.26.5

This task implements autocomplete generation for the `pithos` CLI using Cobra's built-in autocompletion capabilities, and provides automatic installation scripts targeting Zsh and Oh-My-Zsh environments.

## User Review Required

> [!NOTE]
> Oh-My-Zsh autocompletions are stored under `~/.oh-my-zsh/completions/` or directories in the `$fpath` array. If these directories do not exist or are write-restricted, Pithos will fallback to printing the script and instructing the user on how to add it manually to their `.zshrc`.

## Proposed Changes

### Command Layer

#### [NEW] [completion.go](file://../../cmd/pithos/completion.go)
- Create a new Cobra command `completionCmd` for `pithos completion <shell>`.
- Support options: `zsh`, `bash`, `fish`, `powershell`.
- Use Cobra's built-in functions (e.g. `rootCmd.GenZshCompletion`) to output completion scripts to standard output.

### Automatic Installation Support

#### [NEW] [completion.go](file://../../cmd/pithos/completion.go)
- Implement an `--install` / `-i` flag on the `completion` command:
  - Check user's home directory.
  - Detect standard Oh-My-Zsh path: `~/.oh-my-zsh/`.
  - If Oh-My-Zsh is detected:
    - Create `~/.oh-my-zsh/completions/` if it does not exist.
    - Write Zsh completions to `~/.oh-my-zsh/completions/_pithos`.
  - If standard Zsh is detected without Oh-My-Zsh:
    - Create `~/.zsh/completions/` directory.
    - Write completions to `_pithos` at that path.
    - Output Zsh configuration instructions for updating `.zshrc`.

---

## Verification Plan

### Automated Tests
- Test standard script output for zsh, bash, fish, and powershell.
- Test `--install` behavior with mock home environments (verifying Oh-My-Zsh path creation and standard Zsh fallback setup).
- Add integration tests in `cli_test.go` running the compiled `pithos` binary subprocess.

### Manual Verification
- Run `pithos completion zsh --install` and verify it detects and creates the target files.
- Verify shell completions work correctly in the terminal.
