package tui

import (
	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Semantic color slots
// ---------------------------------------------------------------------------

var (
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
	appStyle = lipgloss.NewStyle().
			Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(colorEmphasis).
			Bold(true).
			MarginBottom(1)

	footerStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(1).
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorSurface)

	keyStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	keyDescStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	inputStyle = lipgloss.NewStyle().
			BorderForeground(colorAccent).
			BorderStyle(lipgloss.RoundedBorder()).
			Padding(0, 1)

	textStyle = lipgloss.NewStyle().
			Foreground(colorDefault)

	emphStyle = lipgloss.NewStyle().
			Foreground(colorEmphasis).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	selectedStyle = lipgloss.NewStyle().
			Foreground(colorEmphasis).
			Background(colorSelection).
			Padding(0, 1)

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

	dividerStyle = lipgloss.NewStyle().
			Foreground(colorSurface).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2).
			Background(colorSurface)

	dialogBgStyle = lipgloss.NewStyle().
			Background(colorBase)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorInfo).
			Bold(true)
)

// ---------------------------------------------------------------------------
// Helper: render a status indicator character
// ---------------------------------------------------------------------------

func statusChar(status domain.Status) string {
	switch status {
	case domain.StatusDone:
		return successStyle.Render("✓")
	case domain.StatusFailed:
		return errorStyle.Render("✗")
	case domain.StatusDownloading:
		return emphStyle.Render("█")
	case domain.StatusResolved:
		return infoStyle.Render("▸")
	case domain.StatusPending:
		return mutedStyle.Render(" ")
	default:
		return " "
	}
}

func statusLabel(status domain.Status) string {
	switch status {
	case domain.StatusDone:
		return successStyle.Render("✓ downloaded")
	case domain.StatusFailed:
		return errorStyle.Render("✗ failed")
	case domain.StatusDownloading:
		return emphStyle.Render("▸ downloading")
	case domain.StatusResolved:
		return infoStyle.Render("▸ queued")
	case domain.StatusPending:
		return mutedStyle.Render("  pending")
	default:
		return ""
	}
}
