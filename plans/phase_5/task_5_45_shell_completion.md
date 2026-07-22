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

### Pipeline Auto-Setup Integration

#### [MODIFY] [setup.go](file://../../internal/pipeline/setup.go) (Task 5.43)
- Extend `SetupMCPBinaries` or setup flow to:
  - Check if the current user's shell is Zsh (`$SHELL` contains `/zsh`).
  - Detect standard Oh-My-Zsh path: `~/.oh-my-zsh/`.
  - If Oh-My-Zsh is detected:
    - Create `~/.oh-my-zsh/completions/` if it does not exist.
    - Write Zsh completions to `~/.oh-my-zsh/completions/_pithos`.
  - If standard Zsh is detected without Oh-My-Zsh:
    - Attempt to locate a writable path in Zsh's `$fpath` (e.g., `/usr/local/share/zsh/site-functions` or `~/.zsh/completions/`).
    - Write completions to `_pithos` at that path.
    - If no writable path is found, print instructions for adding `fpath+=~/.zsh/completions` and `autoload -Uz compinit && compinit` to `.zshrc`.

---

## Verification Plan

### Automated Tests
- Test that `pithos completion` prints valid shell scripts for all supported shells (checking for standard keywords like `#compdef pithos` for zsh).

### Manual Verification
- Run `pithos setup` and verify it detects Zsh / Oh-My-Zsh.
- Check if `~/.oh-my-zsh/completions/_pithos` was created.
- Start a new shell session and type `pithos [TAB]` to verify subcommands (e.g. `initiate`, `brew`, `assemble`, `ls`, `setup`) autocomplete correctly.
