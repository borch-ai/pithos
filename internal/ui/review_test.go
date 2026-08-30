package ui

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/borch-ai/pithos/internal/manifest"
	tea "github.com/charmbracelet/bubbletea"
)

func createTestManifest() *manifest.Manifest {
	return &manifest.Manifest{
		BookProperties: manifest.BookProperties{
			Theme:            "existential cat dread",
			Style:            "dark parody oil painting",
			CharacterProfile: "a fluffy tabby cat with cynicism",
		},
		Progress: manifest.Progress{
			Pages: []manifest.PageState{
				{
					PageIndex:          1,
					Text:               "The void stares back,\nthe food bowl is empty.",
					IllustrationPrompt: "A cat looking into an empty bowl",
					Status:             manifest.StatusCompleted,
					ImagePath:          "images/page_1.png",
				},
				{
					PageIndex:          2,
					Text:               "Rain falls outside,\nnaps bring no relief.",
					IllustrationPrompt: "A cat looking at rain through window",
					Status:             manifest.StatusCompleted,
					ImagePath:          "images/page_2.png",
				},
			},
		},
	}
}

func TestReviewModel_Navigation(t *testing.T) {
	m := createTestManifest()
	model := NewReviewModel(m, t.TempDir(), nil)

	if model.SelectedIndex != 0 {
		t.Fatalf("expected selected index 0, got %d", model.SelectedIndex)
	}

	// Move down
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(ReviewModel)
	if model.SelectedIndex != 1 {
		t.Fatalf("expected selected index 1, got %d", model.SelectedIndex)
	}

	// Move down again (should clamp at max page index 1)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(ReviewModel)
	if model.SelectedIndex != 1 {
		t.Fatalf("expected selected index 1, got %d", model.SelectedIndex)
	}

	// Move up
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(ReviewModel)
	if model.SelectedIndex != 0 {
		t.Fatalf("expected selected index 0, got %d", model.SelectedIndex)
	}
}

func TestReviewModel_EditStanza(t *testing.T) {
	m := createTestManifest()
	synced := false
	syncCB := func(man *manifest.Manifest) error {
		synced = true
		return nil
	}

	model := NewReviewModel(m, t.TempDir(), syncCB)

	// Enter edit mode
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	model = updated.(ReviewModel)
	if model.ActiveMode != ModeEditStanza {
		t.Fatalf("expected ModeEditStanza, got %v", model.ActiveMode)
	}

	// Change value
	model.TextArea.SetValue("New stanza line 1\nNew stanza line 2")

	// Save with Ctrl+S
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	model = updated.(ReviewModel)

	if model.ActiveMode != ModeNormal {
		t.Fatalf("expected return to ModeNormal, got %v", model.ActiveMode)
	}
	if !synced {
		t.Fatalf("expected sync callback to be called")
	}
	if m.Progress.Pages[0].Text != "New stanza line 1\nNew stanza line 2" {
		t.Fatalf("unexpected updated stanza text: %s", m.Progress.Pages[0].Text)
	}
}

func TestReviewModel_EditPrompt(t *testing.T) {
	m := createTestManifest()
	model := NewReviewModel(m, t.TempDir(), nil)

	// Enter prompt edit mode
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	model = updated.(ReviewModel)
	if model.ActiveMode != ModeEditPrompt {
		t.Fatalf("expected ModeEditPrompt, got %v", model.ActiveMode)
	}

	model.TextArea.SetValue("Updated painting prompt")

	// Cancel with Esc
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(ReviewModel)
	if model.ActiveMode != ModeNormal {
		t.Fatalf("expected ModeNormal after Esc, got %v", model.ActiveMode)
	}
	if m.Progress.Pages[0].IllustrationPrompt != "A cat looking into an empty bowl" {
		t.Fatalf("prompt should not have changed on Esc: %s", m.Progress.Pages[0].IllustrationPrompt)
	}

	// Enter prompt edit mode again and save
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	model = updated.(ReviewModel)
	model.TextArea.SetValue("Updated painting prompt")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	model = updated.(ReviewModel)

	if m.Progress.Pages[0].IllustrationPrompt != "Updated painting prompt" {
		t.Fatalf("expected prompt to be updated: %s", m.Progress.Pages[0].IllustrationPrompt)
	}
}

func TestReviewModel_EditStyleAndCharacter(t *testing.T) {
	m := createTestManifest()
	model := NewReviewModel(m, t.TempDir(), nil)

	// Edit Style
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model = updated.(ReviewModel)
	if model.ActiveMode != ModeEditStyle {
		t.Fatalf("expected ModeEditStyle, got %v", model.ActiveMode)
	}
	model.TextInput.SetValue("gothic watercolor")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(ReviewModel)
	if m.BookProperties.Style != "gothic watercolor" {
		t.Fatalf("expected updated style, got %s", m.BookProperties.Style)
	}

	// Edit Character
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model = updated.(ReviewModel)
	if model.ActiveMode != ModeEditCharacter {
		t.Fatalf("expected ModeEditCharacter, got %v", model.ActiveMode)
	}
	model.TextInput.SetValue("a stoic ginger cat")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(ReviewModel)
	if m.BookProperties.CharacterProfile != "a stoic ginger cat" {
		t.Fatalf("expected updated character profile, got %s", m.BookProperties.CharacterProfile)
	}
}

func TestReviewModel_MarkRedoAndApprove(t *testing.T) {
	m := createTestManifest()
	tmpDir := t.TempDir()
	model := NewReviewModel(m, tmpDir, nil)

	// Mark page for redo
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	model = updated.(ReviewModel)

	if m.Progress.Pages[0].Status != manifest.StatusPending {
		t.Fatalf("expected page status 'pending', got %s", m.Progress.Pages[0].Status)
	}
	if m.Progress.Pages[0].ImagePath != "" {
		t.Fatalf("expected image path to be cleared")
	}

	// Approve
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model = updated.(ReviewModel)

	if !model.Approved {
		t.Fatalf("expected model.Approved to be true")
	}
	if cmd == nil {
		t.Fatalf("expected tea.Quit command")
	}
}

func TestReviewModel_ViewRendering(t *testing.T) {
	m := createTestManifest()
	model := NewReviewModel(m, t.TempDir(), nil)

	// Window resize
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model = updated.(ReviewModel)

	viewStr := model.View()
	if !bytes.Contains([]byte(viewStr), []byte("PITHOS INTERACTIVE MANUSCRIPT REVIEW")) {
		t.Fatalf("view string missing header title")
	}
	if !bytes.Contains([]byte(viewStr), []byte("existential cat dread")) {
		t.Fatalf("view string missing theme")
	}
}

func TestRunReviewTUI_MockQuit(t *testing.T) {
	m := createTestManifest()
	tmpDir := t.TempDir()
	m.SetFilePath(filepath.Join(tmpDir, "manifest.json"))
	if err := m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	inBuf := bytes.NewBufferString("q")
	outBuf := new(bytes.Buffer)

	approved, err := RunReviewTUI(ReviewTUIConfig{
		Manifest:  m,
		OutputDir: tmpDir,
		InStream:  inBuf,
		OutStream: outBuf,
	})

	if err != nil {
		t.Fatalf("unexpected error running review TUI: %v", err)
	}
	if approved {
		t.Fatalf("expected approved=false when user quits with 'q'")
	}
}

func TestReviewModel_EditCancelsAndErrors(t *testing.T) {
	m := createTestManifest()

	// 1. Edit Stanza Cancel
	model := NewReviewModel(m, t.TempDir(), nil)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	model = updated.(ReviewModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(ReviewModel)
	if model.ActiveMode != ModeNormal {
		t.Fatalf("expected ModeNormal after Esc")
	}

	// 2. Edit Style Cancel
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model = updated.(ReviewModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(ReviewModel)
	if model.ActiveMode != ModeNormal {
		t.Fatalf("expected ModeNormal after Esc")
	}

	// 3. Edit Character Cancel
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model = updated.(ReviewModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(ReviewModel)
	if model.ActiveMode != ModeNormal {
		t.Fatalf("expected ModeNormal after Esc")
	}

	// 4. SyncCallback Error
	syncErr := errors.New("sync failed")
	modelErr := NewReviewModel(m, t.TempDir(), func(man *manifest.Manifest) error {
		return syncErr
	})
	updated, _ = modelErr.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	modelErr = updated.(ReviewModel)
	updated, _ = modelErr.Update(tea.KeyMsg{Type: tea.KeyEnter})
	modelErr = updated.(ReviewModel)
	if !bytes.Contains([]byte(modelErr.StatusMessage), []byte("Sync error")) {
		t.Fatalf("expected sync error message, got %s", modelErr.StatusMessage)
	}

	// 5. Manifest save error when OutputDir invalid
	modelSaveErr := NewReviewModel(m, "/invalid/path/that/cannot/exist", nil)
	updated, _ = modelSaveErr.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	modelSaveErr = updated.(ReviewModel)
	if !bytes.Contains([]byte(modelSaveErr.StatusMessage), []byte("Manifest save error")) {
		t.Fatalf("expected manifest save error message, got %s", modelSaveErr.StatusMessage)
	}
}

func TestReviewModel_ViewModesAndMax(t *testing.T) {
	m := createTestManifest()

	// 1. Edit Stanza View
	model := NewReviewModel(m, t.TempDir(), nil)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	model = updated.(ReviewModel)
	viewStanza := model.View()
	if !bytes.Contains([]byte(viewStanza), []byte("Editing Stanza")) {
		t.Fatalf("view missing Editing Stanza header")
	}

	// 2. Edit Prompt View
	model = NewReviewModel(m, t.TempDir(), nil)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	model = updated.(ReviewModel)
	viewPrompt := model.View()
	if !bytes.Contains([]byte(viewPrompt), []byte("Editing Prompt")) {
		t.Fatalf("view missing Editing Prompt header")
	}

	// 3. Edit Style View
	model = NewReviewModel(m, t.TempDir(), nil)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model = updated.(ReviewModel)
	viewStyle := model.View()
	if !bytes.Contains([]byte(viewStyle), []byte("Edit Global Style Guide")) {
		t.Fatalf("view missing Edit Global Style Guide header")
	}

	// 4. Edit Character View
	model = NewReviewModel(m, t.TempDir(), nil)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model = updated.(ReviewModel)
	viewChar := model.View()
	if !bytes.Contains([]byte(viewChar), []byte("Edit Character Profile Seed")) {
		t.Fatalf("view missing Edit Character Profile Seed header")
	}

	// 5. Empty pages View
	mEmpty := &manifest.Manifest{}
	modelEmpty := NewReviewModel(mEmpty, t.TempDir(), nil)
	viewEmpty := modelEmpty.View()
	if !bytes.Contains([]byte(viewEmpty), []byte("No pages in manuscript manifest")) {
		t.Fatalf("view missing No pages message")
	}

	// 6. Max helper test
	if max(10, 5) != 10 {
		t.Fatalf("max(10, 5) failed")
	}
	if max(3, 8) != 8 {
		t.Fatalf("max(3, 8) failed")
	}
}

func TestReviewModel_TypingAndBranchCoverage(t *testing.T) {
	m := &manifest.Manifest{
		BookProperties: manifest.BookProperties{
			Theme:            "Test Theme",
			Style:            "Test Style",
			CharacterProfile: "Test Character",
		},
		Progress: manifest.Progress{
			Pages: []manifest.PageState{
				{
					PageIndex:          1,
					Text:               "",
					IllustrationPrompt: "",
					Status:             manifest.StatusGeneratingImages,
					ImagePath:          "",
				},
			},
		},
	}

	model := NewReviewModel(m, t.TempDir(), nil)

	// Test view rendering with empty page fields & statusGeneratingImages
	viewStr := model.View()
	if !bytes.Contains([]byte(viewStr), []byte("PEND")) {
		t.Fatalf("expected PEND badge for StatusGeneratingImages")
	}

	// Test typing in EditStanza mode
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	model = updated.(ReviewModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model = updated.(ReviewModel)

	// Test typing in EditPrompt mode
	model.ActiveMode = ModeNormal
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	model = updated.(ReviewModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	model = updated.(ReviewModel)

	// Test typing in EditStyle mode
	model.ActiveMode = ModeNormal
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model = updated.(ReviewModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model = updated.(ReviewModel)

	// Test typing in EditCharacter mode
	model.ActiveMode = ModeNormal
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model = updated.(ReviewModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	model = updated.(ReviewModel)

	// Test unknown keypress in ModeNormal
	model.ActiveMode = ModeNormal
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	model = updated.(ReviewModel)
}
