package ui

import (
	"testing"
)

func TestUIStyles(t *testing.T) {
	// Simple validation to ensure the styles render without crashing
	text := "Test"

	if HeaderStyle.Render(text) == "" {
		t.Error("HeaderStyle rendered empty string")
	}
	if BoxStyle.Render(text) == "" {
		t.Error("BoxStyle rendered empty string")
	}
	if KeyStyle.Render(text) == "" {
		t.Error("KeyStyle rendered empty string")
	}
	if ValStyle.Render(text) == "" {
		t.Error("ValStyle rendered empty string")
	}
	if HighlightStyle.Render(text) == "" {
		t.Error("HighlightStyle rendered empty string")
	}
	if SuccessStyle.Render(text) == "" {
		t.Error("SuccessStyle rendered empty string")
	}
	if FailStyle.Render(text) == "" {
		t.Error("FailStyle rendered empty string")
	}

	// Badges
	if BadgeOk.Render(text) == "" {
		t.Error("BadgeOk rendered empty string")
	}
	if BadgeFail.Render(text) == "" {
		t.Error("BadgeFail rendered empty string")
	}
	if BadgeWarn.Render(text) == "" {
		t.Error("BadgeWarn rendered empty string")
	}
	if BadgeSkip.Render(text) == "" {
		t.Error("BadgeSkip rendered empty string")
	}
}
