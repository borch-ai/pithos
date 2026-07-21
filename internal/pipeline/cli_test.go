//go:build integration

package pipeline

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/borch-ai/pithos/internal/manifest"
)

var buildOnce sync.Once

func getPithosBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		tmpDir, err := os.MkdirTemp("", "pithos-cli-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		TestPithosBinaryPath = filepath.Join(tmpDir, "pithos")
		
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", TestPithosBinaryPath, "../../cmd/pithos")
		if out, err := cmd.CombinedOutput(); err != nil {
			os.RemoveAll(tmpDir)
			t.Fatalf("failed to build pithos binary: %v\nOutput: %s", err, string(out))
		}
		TestPithosBinaryCleanup = func() {
			os.RemoveAll(tmpDir)
		}
	})
	return TestPithosBinaryPath
}

func newPithosCmd(ctx context.Context, homeDir string, args ...string) *exec.Cmd {
	if TestPithosBinaryPath == "" {
		panic("TestPithosBinaryPath is not initialized; call getPithosBinary(t) first")
	}
	cmd := exec.CommandContext(ctx, TestPithosBinaryPath, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"XDG_CONFIG_HOME="+filepath.Join(homeDir, ".config"),
		"XDG_DATA_HOME="+filepath.Join(homeDir, ".local", "share"),
	)
	return cmd
}

func TestCLI_Initiate_Basic(t *testing.T) {
	getPithosBinary(t)

	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	if err := os.MkdirAll(homeDir, 0755); err != nil {
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
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatalf("manifest.json was not created at %s", manifestPath)
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
	if err := os.MkdirAll(homeDir, 0755); err != nil {
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
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}

	bookDir := filepath.Join(tempDir, "overwritebook")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// First run to create the directory
	cmd1 := newPithosCmd(ctx, homeDir, "initiate", "--output", bookDir, "--theme", "test1", "--dry-run")
	if out, err := cmd1.CombinedOutput(); err != nil {
		t.Fatalf("first pithos initiate failed: %v\nOutput: %s", err, string(out))
	}

	// Second run, input 'n' to decline overwrite
	cmd2 := newPithosCmd(ctx, homeDir, "initiate", "--output", bookDir, "--theme", "test2", "--dry-run")
	stdin2, err := cmd2.StdinPipe()
	if err != nil {
		t.Fatalf("failed to get stdin pipe: %v", err)
	}

	go func() {
		defer stdin2.Close()
		io.WriteString(stdin2, "n\n")
	}()

	out2, err2 := cmd2.CombinedOutput()
	if err2 == nil {
		t.Fatalf("expected command to fail when overwrite is declined, but it succeeded\nOutput: %s", string(out2))
	}
	if !strings.Contains(string(out2), "initiation cancelled") {
		t.Errorf("expected 'initiation cancelled' in output, got: %s", string(out2))
	}

	// Third run, input 'y' to accept overwrite
	cmd3 := newPithosCmd(ctx, homeDir, "initiate", "--output", bookDir, "--theme", "test3", "--dry-run")
	stdin3, err := cmd3.StdinPipe()
	if err != nil {
		t.Fatalf("failed to get stdin pipe: %v", err)
	}

	go func() {
		defer stdin3.Close()
		io.WriteString(stdin3, "y\n")
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
