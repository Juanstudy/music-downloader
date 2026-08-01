package tui

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Config view tests (AQ-014, AQ-015)
// ---------------------------------------------------------------------------

// configModel returns a ready Model on the Config screen with the given state.
func configModel(audioQuality string, qualityCursor int) Model {
	return Model{
		Screen:        ScreenConfig,
		Ready:         true,
		Width:         80,
		Height:        24,
		audioQuality:  audioQuality,
		qualityCursor: qualityCursor,
	}
}

func TestConfigView_RendersOptionsAndCursor(t *testing.T) {
	m := configModel("320k", 2) // cursor on 320k

	out := m.View()

	for _, opt := range []string{"128k", "192k", "320k"} {
		if !strings.Contains(out, opt) {
			t.Errorf("expected output to contain option %q", opt)
		}
	}
	if !strings.Contains(out, "▸ ● 320k") {
		t.Errorf("expected the 320k line to be marked with cursor/selection, got:\n%s", out)
	}
}

func TestConfigView_RendersCurrentQualityAndFooter(t *testing.T) {
	m := configModel("192k", 0)

	out := m.View()

	if !strings.Contains(out, "Current:") {
		t.Errorf("expected output to indicate the current effective quality, got:\n%s", out)
	}
	if !strings.Contains(out, "192k") {
		t.Errorf("expected output to show current quality 192k, got:\n%s", out)
	}
	for _, hint := range []string{"j/k", "Enter", "Esc"} {
		if !strings.Contains(out, hint) {
			t.Errorf("expected footer to mention %q, got:\n%s", hint, out)
		}
	}
}

func TestHelpShowsCKey(t *testing.T) {
	out := helpView(80)

	if !strings.Contains(out, "c") {
		t.Errorf("expected help overlay to list the c key, got:\n%s", out)
	}
	if !strings.Contains(out, "Configure quality") {
		t.Errorf("expected help overlay to describe the c binding, got:\n%s", out)
	}
}
