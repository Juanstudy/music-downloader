package tui

import (
	"github.com/Juanstudy/music-downloader/internal/model"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Semantic color slots (from tui-design skill §4)
//
// Strategy: start with ANSI 16 as foundation (works on any terminal).
// Next layer: adaptive colors that respond to light/dark themes.
// Future layer: true color hex values.
//
// Using lipgloss.AdaptiveColor so light and dark terminal themes both work.
// ---------------------------------------------------------------------------

var (
	// Colors — semantic slots
	colorDefault   = lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#c0caf5"}
	colorMuted     = lipgloss.AdaptiveColor{Light: "#888888", Dark: "#565f89"}
	colorEmphasis  = lipgloss.AdaptiveColor{Light: "#000000", Dark: "#e0e0e0"}
	colorBase      = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#1a1b26"}
	colorSurface   = lipgloss.AdaptiveColor{Light: "#f0f0f0", Dark: "#24283b"}
	colorSelection = lipgloss.AdaptiveColor{Light: "#d4e0ff", Dark: "#364a82"}
	colorAccent    = lipgloss.AdaptiveColor{Light: "#2255cc", Dark: "#7aa2f7"}
	colorSuccess   = lipgloss.AdaptiveColor{Light: "#1a8a1a", Dark: "#9ece6a"}
	colorError     = lipgloss.AdaptiveColor{Light: "#cc2222", Dark: "#f7768e"}
	colorWarning   = lipgloss.AdaptiveColor{Light: "#cc8800", Dark: "#e0af68"}
	colorInfo      = lipgloss.AdaptiveColor{Light: "#2266cc", Dark: "#7dcfff"}
)

// ---------------------------------------------------------------------------
// Component styles
// ---------------------------------------------------------------------------

var (
	// App container — full viewport
	appStyle = lipgloss.NewStyle().
			Padding(1, 2)

	// Title bar
	titleStyle = lipgloss.NewStyle().
			Foreground(colorEmphasis).
			Bold(true).
			MarginBottom(1)

	// Footer bar
	footerStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(1).
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorSurface)

	// Key hint in footer — e.g. "[q]uit"
	keyStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	// Label for key descriptions in footer
	keyDescStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Active input field
	inputStyle = lipgloss.NewStyle().
			BorderForeground(colorAccent).
			BorderStyle(lipgloss.RoundedBorder()).
			Padding(0, 1)

	// Normal text
	textStyle = lipgloss.NewStyle().
			Foreground(colorDefault)

	// Emphasised text (headers, active items)
	emphStyle = lipgloss.NewStyle().
			Foreground(colorEmphasis).
			Bold(true)

	// Muted text (metadata, timestamps, counters)
	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Selected row highlight
	selectedStyle = lipgloss.NewStyle().
			Foreground(colorEmphasis).
			Background(colorSelection).
			Padding(0, 1)

	// Status indicators
	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)
	errorStyle = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)
	warningStyle = lipgloss.NewStyle().
			Foreground(colorWarning)
	infoStyle = lipgloss.NewStyle().
			Foreground(colorInfo)

	// Divider line
	dividerStyle = lipgloss.NewStyle().
			Foreground(colorSurface).
			Padding(0, 1)

	// Help overlay box
	helpStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2).
			Background(colorSurface)

	// Dialog/overlay background dim
	dialogBgStyle = lipgloss.NewStyle().
			Background(colorBase)

	// Spinner style (color applied to the spinner text)
	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorInfo).
			Bold(true)
)

// ---------------------------------------------------------------------------
// Helper: render a status indicator character
// ---------------------------------------------------------------------------

func statusChar(status model.Status) string {
	switch status {
	case model.StatusCompleted:
		return successStyle.Render("✓")
	case model.StatusFailed:
		return errorStyle.Render("✗")
	case model.StatusDownloading:
		return emphStyle.Render("█")
	case model.StatusPending:
		return mutedStyle.Render(" ")
	default:
		return " "
	}
}

func statusLabel(status model.Status) string {
	switch status {
	case model.StatusCompleted:
		return successStyle.Render("✓ downloaded")
	case model.StatusFailed:
		return errorStyle.Render("✗ failed")
	case model.StatusDownloading:
		return emphStyle.Render("▸ downloading")
	case model.StatusPending:
		return mutedStyle.Render("  pending")
	default:
		return ""
	}
}
