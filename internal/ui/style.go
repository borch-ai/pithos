package ui

import "github.com/charmbracelet/lipgloss"

// Theme color constants
const (
	ColorPink    = "205"
	ColorPurple  = "99"
	ColorGreen   = "#A3E635" // Lime green
	ColorMuted   = "#9CA3AF" // Muted gray
	ColorWhite   = "#FFFFFF"
	ColorFailRed = "#EF4444"

	// Diagnostics colors
	ColorOkText   = "#10B981"
	ColorOkBg     = "#064E3B"
	ColorFailText = "#F87171"
	ColorFailBg   = "#7F1D1D"
	ColorWarnText = "#FBBF24"
	ColorWarnBg   = "#78350F"
	ColorSkipText = "#F3F4F6"
	ColorSkipBg   = "#6B7280"
)

// Shared styles
var (
	// HeaderStyle is the standard styled header/title (e.g. for ls, or titles inside cards)
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorPink))

	// BoxStyle is the standard rounded border container box used for cards
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorPurple)).
			Padding(1, 2).
			Margin(1, 0)

	// KeyStyle is for bold/white metadata keys
	KeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorWhite))

	// ValStyle is for muted/gray metadata values
	ValStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted))

	// HighlightStyle is for lime green highlighted sections/headers inside cards
	HighlightStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorGreen))

	// SuccessStyle is for overall success status notifications
	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorGreen)).
			MarginTop(1)

	// FailStyle is for overall failure notifications
	FailStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorFailRed)).
			MarginTop(1)
)

// Standard diagnostic badge styles
var (
	BadgeOk = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorOkText)).
		Background(lipgloss.Color(ColorOkBg)).
		Width(8).
		Align(lipgloss.Center)

	BadgeFail = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorFailText)).
			Background(lipgloss.Color(ColorFailBg)).
			Width(8).
			Align(lipgloss.Center)

	BadgeWarn = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorWarnText)).
			Background(lipgloss.Color(ColorWarnBg)).
			Width(8).
			Align(lipgloss.Center)

	BadgeSkip = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorSkipText)).
			Background(lipgloss.Color(ColorSkipBg)).
			Width(8).
			Align(lipgloss.Center)
)
