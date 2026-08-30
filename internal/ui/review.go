package ui

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ReviewMode represents the current interactive sub-state in the review TUI.
type ReviewMode int

const (
	ModeNormal ReviewMode = iota
	ModeEditStanza
	ModeEditPrompt
	ModeEditStyle
	ModeEditCharacter
)

// ReviewTUIConfig contains configuration parameters for launching the review TUI.
type ReviewTUIConfig struct {
	Manifest     *manifest.Manifest
	OutputDir    string
	SyncCallback func(m *manifest.Manifest) error
	InStream     io.Reader
	OutStream    io.Writer
}

// ReviewModel represents the Bubbletea UI state for manuscript review.
type ReviewModel struct {
	Manifest      *manifest.Manifest
	OutputDir     string
	SelectedIndex int
	ActiveMode    ReviewMode
	Approved      bool
	Quitted       bool
	StatusMessage string
	SyncCallback  func(m *manifest.Manifest) error

	Width  int
	Height int

	TextInput textinput.Model
	TextArea  textarea.Model
}

// NewReviewModel initializes a new ReviewModel.
func NewReviewModel(m *manifest.Manifest, outputDir string, syncCB func(m *manifest.Manifest) error) ReviewModel {
	ti := textinput.New()
	ti.Focus()

	ta := textarea.New()
	ta.Focus()
	ta.SetWidth(60)
	ta.SetHeight(5)

	return ReviewModel{
		Manifest:      m,
		OutputDir:     outputDir,
		SelectedIndex: 0,
		ActiveMode:    ModeNormal,
		StatusMessage: "Use j/k or arrows to navigate. Press e (stanza), p (prompt), s (style), c (character), r (redo), a (approve), q (quit).",
		SyncCallback:  syncCB,
		TextInput:     ti,
		TextArea:      ta,
		Width:         80,
		Height:        24,
	}
}

func (m ReviewModel) Init() tea.Cmd {
	return nil
}

func (m ReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.TextArea.SetWidth(max(40, msg.Width-25))
		return m, nil

	case tea.KeyMsg:
		switch m.ActiveMode {
		case ModeNormal:
			return m.updateNormal(msg)
		case ModeEditStanza:
			return m.updateEditStanza(msg)
		case ModeEditPrompt:
			return m.updateEditPrompt(msg)
		case ModeEditStyle:
			return m.updateEditStyle(msg)
		case ModeEditCharacter:
			return m.updateEditCharacter(msg)
		}
	}

	return m, cmd
}

func (m ReviewModel) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	pageCount := len(m.Manifest.Progress.Pages)

	switch msg.String() {
	case "q", "esc", "ctrl+c":
		m.Quitted = true
		return m, tea.Quit

	case "a", "enter":
		m.Approved = true
		m.StatusMessage = "Manuscript approved! Proceeding to illustration generation..."
		return m, tea.Quit

	case "j", "down":
		if pageCount > 0 && m.SelectedIndex < pageCount-1 {
			m.SelectedIndex++
		}

	case "k", "up":
		if m.SelectedIndex > 0 {
			m.SelectedIndex--
		}

	case "e":
		if pageCount > 0 && m.SelectedIndex < pageCount {
			m.ActiveMode = ModeEditStanza
			m.TextArea.SetValue(m.Manifest.Progress.Pages[m.SelectedIndex].Text)
			m.TextArea.Focus()
			m.StatusMessage = "Editing Stanza text. Press Ctrl+S to save, Esc to cancel."
			return m, textarea.Blink
		}

	case "p":
		if pageCount > 0 && m.SelectedIndex < pageCount {
			m.ActiveMode = ModeEditPrompt
			m.TextArea.SetValue(m.Manifest.Progress.Pages[m.SelectedIndex].IllustrationPrompt)
			m.TextArea.Focus()
			m.StatusMessage = "Editing Illustration Prompt. Press Ctrl+S to save, Esc to cancel."
			return m, textarea.Blink
		}

	case "s":
		m.ActiveMode = ModeEditStyle
		m.TextInput.SetValue(m.Manifest.BookProperties.Style)
		m.TextInput.Focus()
		m.StatusMessage = "Editing Global Visual Style. Press Enter to save, Esc to cancel."
		return m, textinput.Blink

	case "c":
		m.ActiveMode = ModeEditCharacter
		m.TextInput.SetValue(m.Manifest.BookProperties.CharacterProfile)
		m.TextInput.Focus()
		m.StatusMessage = "Editing Character Profile. Press Enter to save, Esc to cancel."
		return m, textinput.Blink

	case "r":
		if pageCount > 0 && m.SelectedIndex < pageCount {
			m.Manifest.Progress.Pages[m.SelectedIndex].Status = manifest.StatusPending
			m.Manifest.Progress.Pages[m.SelectedIndex].ImagePath = ""
			m.StatusMessage = fmt.Sprintf("Marked Page %d for redo.", m.Manifest.Progress.Pages[m.SelectedIndex].PageIndex)
			m.syncState()
		}
	}

	return m, nil
}

func (m ReviewModel) updateEditStanza(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		m.ActiveMode = ModeNormal
		m.StatusMessage = "Stanza edit cancelled."
		return m, nil
	case "ctrl+s":
		if m.SelectedIndex < len(m.Manifest.Progress.Pages) {
			m.Manifest.Progress.Pages[m.SelectedIndex].Text = m.TextArea.Value()
			m.ActiveMode = ModeNormal
			m.StatusMessage = fmt.Sprintf("Updated stanza text for Page %d.", m.Manifest.Progress.Pages[m.SelectedIndex].PageIndex)
			m.syncState()
		}
		return m, nil
	}

	m.TextArea, cmd = m.TextArea.Update(msg)
	return m, cmd
}

func (m ReviewModel) updateEditPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		m.ActiveMode = ModeNormal
		m.StatusMessage = "Prompt edit cancelled."
		return m, nil
	case "ctrl+s":
		if m.SelectedIndex < len(m.Manifest.Progress.Pages) {
			m.Manifest.Progress.Pages[m.SelectedIndex].IllustrationPrompt = m.TextArea.Value()
			m.ActiveMode = ModeNormal
			m.StatusMessage = fmt.Sprintf("Updated illustration prompt for Page %d.", m.Manifest.Progress.Pages[m.SelectedIndex].PageIndex)
			m.syncState()
		}
		return m, nil
	}

	m.TextArea, cmd = m.TextArea.Update(msg)
	return m, cmd
}

func (m ReviewModel) updateEditStyle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		m.ActiveMode = ModeNormal
		m.StatusMessage = "Style edit cancelled."
		return m, nil
	case "enter":
		m.Manifest.BookProperties.Style = m.TextInput.Value()
		m.ActiveMode = ModeNormal
		m.StatusMessage = "Updated global visual style guide."
		m.syncState()
		return m, nil
	}

	m.TextInput, cmd = m.TextInput.Update(msg)
	return m, cmd
}

func (m ReviewModel) updateEditCharacter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		m.ActiveMode = ModeNormal
		m.StatusMessage = "Character profile edit cancelled."
		return m, nil
	case "enter":
		m.Manifest.BookProperties.CharacterProfile = m.TextInput.Value()
		m.ActiveMode = ModeNormal
		m.StatusMessage = "Updated character profile seed."
		m.syncState()
		return m, nil
	}

	m.TextInput, cmd = m.TextInput.Update(msg)
	return m, cmd
}

func (m *ReviewModel) syncState() {
	if m.SyncCallback != nil {
		if err := m.SyncCallback(m.Manifest); err != nil {
			m.StatusMessage = fmt.Sprintf("Sync error: %v", err)
			return
		}
	} else if m.OutputDir != "" {
		manifestPath := filepath.Join(m.OutputDir, "manifest.json")
		if err := m.Manifest.SaveTo(manifestPath); err != nil {
			m.StatusMessage = fmt.Sprintf("Manifest save error: %v", err)
			return
		}
	}
}

func (m ReviewModel) View() string {
	var sb strings.Builder

	// Header Banner
	header := HeaderStyle.Render("🏺 PITHOS INTERACTIVE MANUSCRIPT REVIEW")
	sb.WriteString(header + "\n\n")

	// Global Book Info Card
	sb.WriteString(m.renderGlobalCard() + "\n\n")

	pages := m.Manifest.Progress.Pages
	if len(pages) == 0 {
		sb.WriteString(ValStyle.Render("No pages in manuscript manifest.") + "\n")
	} else {
		sb.WriteString(m.renderMainLayout(pages) + "\n\n")
	}

	// Status / Footer Bar
	status := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorMuted)).
		Italic(true).
		Render(m.StatusMessage)
	sb.WriteString(status + "\n")

	return sb.String()
}

func (m ReviewModel) renderGlobalCard() string {
	themeStr := ValStyle.Render(m.Manifest.BookProperties.Theme)
	if themeStr == "" {
		themeStr = ValStyle.Render("(none)")
	}
	styleStr := ValStyle.Render(m.Manifest.BookProperties.Style)
	if styleStr == "" {
		styleStr = ValStyle.Render("(none)")
	}
	charStr := ValStyle.Render(m.Manifest.BookProperties.CharacterProfile)
	if charStr == "" {
		charStr = ValStyle.Render("(none)")
	}

	globalInfo := fmt.Sprintf("%s %s\n%s %s\n%s %s",
		KeyStyle.Render("Theme:"), themeStr,
		KeyStyle.Render("Style:"), styleStr,
		KeyStyle.Render("Character:"), charStr,
	)
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(ColorPurple)).Padding(0, 1).Render(globalInfo)
}

func (m ReviewModel) renderMainLayout(pages []manifest.PageState) string {
	sidebarView := m.renderSidebar(pages)
	activePage := pages[m.SelectedIndex]
	rightPanel := m.renderRightPanel(activePage)

	detailView := lipgloss.NewStyle().
		PaddingLeft(2).
		Render(rightPanel)

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, detailView)
}

func (m ReviewModel) renderSidebar(pages []manifest.PageState) string {
	var leftSidebar []string
	for i, page := range pages {
		prefix := "  "
		if i == m.SelectedIndex {
			prefix = "❯ "
		}

		badge := BadgeOk.Render("DONE")
		if page.Status == manifest.StatusPending || page.Status == manifest.StatusGeneratingImages {
			badge = BadgeWarn.Render("PEND")
		}

		line := fmt.Sprintf("%sPage %d %s", prefix, page.PageIndex, badge)
		if i == m.SelectedIndex {
			line = HighlightStyle.Render(line)
		}
		leftSidebar = append(leftSidebar, line)
	}

	return lipgloss.NewStyle().
		Width(22).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color(ColorMuted)).
		Render(strings.Join(leftSidebar, "\n"))
}

func (m ReviewModel) renderRightPanel(activePage manifest.PageState) string {
	switch m.ActiveMode {
	case ModeEditStanza, ModeEditPrompt:
		var editLabel string
		if m.ActiveMode == ModeEditStanza {
			editLabel = HighlightStyle.Render("Editing Stanza (Ctrl+S to save, Esc to cancel):")
		} else {
			editLabel = HighlightStyle.Render("Editing Prompt (Ctrl+S to save, Esc to cancel):")
		}
		return fmt.Sprintf("%s\n\n%s", editLabel, m.TextArea.View())

	case ModeEditStyle:
		return fmt.Sprintf("%s\n\n%s",
			HighlightStyle.Render("Edit Global Style Guide (Enter to save, Esc to cancel):"),
			m.TextInput.View(),
		)

	case ModeEditCharacter:
		return fmt.Sprintf("%s\n\n%s",
			HighlightStyle.Render("Edit Character Profile Seed (Enter to save, Esc to cancel):"),
			m.TextInput.View(),
		)

	default:
		stanzaView := activePage.Text
		if stanzaView == "" {
			stanzaView = ValStyle.Render("(empty stanza)")
		}

		promptView := activePage.IllustrationPrompt
		if promptView == "" {
			promptView = ValStyle.Render("(empty prompt)")
		}

		imgView := activePage.ImagePath
		if imgView == "" {
			imgView = ValStyle.Render("(none generated)")
		}

		return fmt.Sprintf("%s %d\n\n%s\n%s\n\n%s\n%s\n\n%s\n%s",
			KeyStyle.Render("--- PAGE DETAIL --- Page"), activePage.PageIndex,
			KeyStyle.Render("Stanza:"), stanzaView,
			KeyStyle.Render("Prompt:"), promptView,
			KeyStyle.Render("Image Asset:"), imgView,
		)
	}
}

// RunReviewTUI launches the Bubbletea interactive review program.
func RunReviewTUI(cfg ReviewTUIConfig) (bool, error) {
	model := NewReviewModel(cfg.Manifest, cfg.OutputDir, cfg.SyncCallback)

	var opts []tea.ProgramOption
	if cfg.InStream != nil {
		opts = append(opts, tea.WithInput(cfg.InStream))
	}
	if cfg.OutStream != nil {
		opts = append(opts, tea.WithOutput(cfg.OutStream))
	}

	p := tea.NewProgram(model, opts...)
	finalModel, err := p.Run()
	if err != nil {
		return false, fmt.Errorf("failed to run review TUI: %w", err)
	}

	m, ok := finalModel.(ReviewModel)
	if !ok {
		return false, fmt.Errorf("unexpected model type returned from TUI")
	}

	return m.Approved, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
