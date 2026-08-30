package pipeline

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/borch-ai/pithos/internal/manifest"
)

var (
	buildOnce sync.Once
	buildErr  error
	buildOut  string
)

func getPithosBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		tmpDir, err := os.MkdirTemp("", "pithos-cli-test-*")
		if err != nil {
			buildErr = err
			return
		}

		binName := "pithos"
		if runtime.GOOS == "windows" {
			binName = "pithos.exe"
		}
		TestPithosBinaryPath = filepath.Join(tmpDir, binName)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		absBinaryPath, err := filepath.Abs(TestPithosBinaryPath)
		if err != nil {
			buildErr = err
			return
		}

		absSourceDir, err := filepath.Abs("../../cmd/pithos")
		if err != nil {
			buildErr = err
			return
		}

		//nolint:gosec // absolute paths and cmd.Dir set explicitly
		cmd := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", absBinaryPath, ".")
		cmd.Dir = absSourceDir
		if out, err := cmd.CombinedOutput(); err != nil {
			_ = os.RemoveAll(tmpDir)
			buildErr = err
			buildOut = string(out)
			return
		}
		TestPithosBinaryCleanup = func() {
			_ = os.RemoveAll(tmpDir)
		}
	})

	if buildErr != nil {
		if buildOut != "" {
			t.Fatalf("failed to build pithos binary: %v\nOutput: %s", buildErr, buildOut)
		}
		t.Fatalf("failed to setup pithos binary test env: %v", buildErr)
	}
	return TestPithosBinaryPath
}

func newPithosCmd(ctx context.Context, homeDir string, args ...string) *exec.Cmd {
	if TestPithosBinaryPath == "" {
		panic("TestPithosBinaryPath is not initialized; call getPithosBinary(t) first")
	}
	//nolint:gosec // TestPithosBinaryPath is verified to be built from current workspace source
	cmd := exec.CommandContext(ctx, TestPithosBinaryPath, args...)

	var cleanEnv []string
	for _, envVar := range os.Environ() {
		upper := strings.ToUpper(envVar)
		if strings.HasPrefix(upper, "PITHOS_") ||
			strings.HasPrefix(upper, "HOME=") ||
			strings.HasPrefix(upper, "XDG_CONFIG_HOME=") ||
			strings.HasPrefix(upper, "XDG_DATA_HOME=") ||
			strings.HasPrefix(upper, "USERPROFILE=") ||
			strings.HasPrefix(upper, "HOMEDRIVE=") ||
			strings.HasPrefix(upper, "HOMEPATH=") ||
			strings.HasPrefix(upper, "APPDATA=") ||
			strings.HasPrefix(upper, "LOCALAPPDATA=") {
			continue
		}
		cleanEnv = append(cleanEnv, envVar)
	}

	cleanEnv = append(cleanEnv,
		"HOME="+homeDir,
		"XDG_CONFIG_HOME="+filepath.Join(homeDir, ".config"),
		"XDG_DATA_HOME="+filepath.Join(homeDir, ".local", "share"),
		"USERPROFILE="+homeDir,
		"HOMEDRIVE="+filepath.VolumeName(homeDir),
		"HOMEPATH="+homeDir[len(filepath.VolumeName(homeDir)):],
		"APPDATA="+filepath.Join(homeDir, "AppData", "Roaming"),
		"LOCALAPPDATA="+filepath.Join(homeDir, "AppData", "Local"),
	)
	cmd.Env = cleanEnv
	return cmd
}

func TestCLI_Initiate_Basic(t *testing.T) {
	getPithosBinary(t)

	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	if err := os.MkdirAll(homeDir, 0700); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}

	bookDir := filepath.Join(tempDir, "mybook")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := newPithosCmd(ctx, homeDir, "initiate", "--output", bookDir, "--theme", "existential dread of a house cat", "--pages", "10", "--dry-run")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pithos initiate failed: %v\nOutput: %s", err, string(out))
	}

	manifestPath := filepath.Join(bookDir, "manifest.json")
	if _, statErr := os.Stat(manifestPath); statErr != nil {
		if os.IsNotExist(statErr) {
			t.Fatalf("manifest.json was not created at %s", manifestPath)
		}
		t.Fatalf("failed to stat manifest.json: %v", statErr)
	}

	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	if m.BookProperties.Theme != "existential dread of a house cat" {
		t.Errorf("expected theme %q, got %q", "existential dread of a house cat", m.BookProperties.Theme)
	}
	if m.BookProperties.TargetPageCount != 10 {
		t.Errorf("expected 10 pages, got %d", m.BookProperties.TargetPageCount)
	}
}

func TestCLI_Initiate_Brainstorm_OptOut(t *testing.T) {
	getPithosBinary(t)

	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	if err := os.MkdirAll(homeDir, 0700); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}

	bookDir := filepath.Join(tempDir, "turtlebook")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := newPithosCmd(ctx, homeDir, "initiate", "--output", bookDir, "--theme", "turtle", "--no-brainstorm", "--dry-run")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pithos initiate failed: %v\nOutput: %s", err, string(out))
	}

	manifestPath := filepath.Join(bookDir, "manifest.json")
	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	if m.BookProperties.Style != "" {
		t.Errorf("expected empty Style, got %q", m.BookProperties.Style)
	}
	if m.BookProperties.CharacterProfile != "" {
		t.Errorf("expected empty CharacterProfile, got %q", m.BookProperties.CharacterProfile)
	}
}

func TestCLI_Initiate_Overwrite(t *testing.T) {
	getPithosBinary(t)

	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	if err := os.MkdirAll(homeDir, 0700); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}

	bookDir := filepath.Join(tempDir, "overwritebook")

	// First run to create the directory
	ctx1, cancel1 := context.WithTimeout(context.Background(), 10*time.Second)
	cmd1 := newPithosCmd(ctx1, homeDir, "initiate", "--output", bookDir, "--theme", "test1", "--dry-run")
	out1, err1 := cmd1.CombinedOutput()
	cancel1()
	if err1 != nil {
		t.Fatalf("first pithos initiate failed: %v\nOutput: %s", err1, string(out1))
	}

	// Second run, input 'n' to decline overwrite
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	cmd2 := newPithosCmd(ctx2, homeDir, "initiate", "--output", bookDir, "--theme", "test2", "--dry-run")
	stdin2, err := cmd2.StdinPipe()
	if err != nil {
		t.Fatalf("failed to get stdin pipe: %v", err)
	}

	go func() {
		defer func() { _ = stdin2.Close() }()
		_, _ = io.WriteString(stdin2, "n\n")
	}()

	out2, err2 := cmd2.CombinedOutput()
	if err2 == nil {
		t.Fatalf("expected command to fail when overwrite is declined, but it succeeded\nOutput: %s", string(out2))
	}
	var exitErr *exec.ExitError
	if errors.As(err2, &exitErr) {
		if exitErr.ExitCode() != 1 {
			t.Errorf("expected exit code 1 when overwrite is declined, got %d. Output: %s", exitErr.ExitCode(), string(out2))
		}
	} else {
		t.Fatalf("expected exec.ExitError when overwrite is declined, got %T: %v", err2, err2)
	}
	if !strings.Contains(string(out2), "initiation cancelled") {
		t.Errorf("expected 'initiation cancelled' in output, got: %s", string(out2))
	}

	// Third run, input 'y' to accept overwrite
	ctx3, cancel3 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel3()
	cmd3 := newPithosCmd(ctx3, homeDir, "initiate", "--output", bookDir, "--theme", "test3", "--dry-run")
	stdin3, err := cmd3.StdinPipe()
	if err != nil {
		t.Fatalf("failed to get stdin pipe: %v", err)
	}

	go func() {
		defer func() { _ = stdin3.Close() }()
		_, _ = io.WriteString(stdin3, "y\n")
	}()

	out3, err3 := cmd3.CombinedOutput()
	if err3 != nil {
		t.Fatalf("expected command to succeed when overwrite is accepted, but failed: %v\nOutput: %s", err3, string(out3))
	}

	// Verify the theme was overwritten
	manifestPath := filepath.Join(bookDir, "manifest.json")
	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	if m.BookProperties.Theme != "test3" {
		t.Errorf("expected theme 'test3' after overwrite, got %q", m.BookProperties.Theme)
	}
}

func TestCLI_Completion(t *testing.T) {
	getPithosBinary(t)

	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	if err := os.MkdirAll(homeDir, 0700); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}

	// Create mock .oh-my-zsh directory under home
	omzDir := filepath.Join(homeDir, ".oh-my-zsh")
	if err := os.MkdirAll(omzDir, 0700); err != nil {
		t.Fatalf("failed to create mock OMZ directory: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Test stdout printing
	cmd := newPithosCmd(ctx, homeDir, "completion", "zsh")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pithos completion zsh failed: %v\nOutput: %s", err, string(out))
	}

	if !strings.Contains(string(out), "#compdef pithos") {
		t.Errorf("expected zsh completion script to contain '#compdef pithos', got: %s", string(out))
	}

	// 2. Test auto-install flag
	cmdInstall := newPithosCmd(ctx, homeDir, "completion", "zsh", "--install")
	outInstall, err := cmdInstall.CombinedOutput()
	if err != nil {
		t.Fatalf("pithos completion zsh --install failed: %v\nOutput: %s", err, string(outInstall))
	}

	expectedPath := filepath.Join(omzDir, "completions", "_pithos")
	if _, statErr := os.Stat(expectedPath); statErr != nil {
		t.Errorf("expected completion file to be installed at %q, but got error: %v", expectedPath, statErr)
	}

	// 3. Test invalid shell returns error
	cmdInvalid := newPithosCmd(ctx, homeDir, "completion", "invalid-shell-name")
	outInvalid, err := cmdInvalid.CombinedOutput()
	if err == nil {
		t.Fatalf("expected failure for invalid shell name, but command succeeded\nOutput: %s", string(outInvalid))
	}
}
