package tui

import (
	"fmt"
	"strings"

	"github.com/Juanstudy/music-downloader/internal/model"
	"github.com/charmbracelet/lipgloss"
)

// View renders the entire TUI based on the current screen.
func (m Model) View() string {
	if !m.Ready {
		return "Loading..."
	}

	// Fatal error overrides everything
	if m.Err != nil {
		return m.renderFatalError()
	}

	var content string

	switch m.Screen {
	case ScreenInput:
		content = m.renderInputView()
	case ScreenResolving:
		content = m.renderResolvingView()
	case ScreenPlaylist:
		content = m.renderPlaylistView()
	case ScreenDownloading:
		content = m.renderDownloadingView()
	case ScreenDone:
		content = m.renderDoneView()
	}

	// Wrap in app container
	body := appStyle.Render(content)

	// If help overlay is active, layer it on top
	if m.ShowHelp {
		help := m.renderHelpOverlay()
		body = lipgloss.JoinVertical(lipgloss.Top, body, "\n", help)
	}

	return body
}

// ---------------------------------------------------------------------------
// Fatal error screen
// ---------------------------------------------------------------------------

func (m Model) renderFatalError() string {
	return errorStyle.Render(fmt.Sprintf("Error: %v", m.Err))
}

// ---------------------------------------------------------------------------
// Screen: Input
// ---------------------------------------------------------------------------

func (m Model) renderInputView() string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("♪ music-dl"))
	b.WriteString("\n\n")

	// Error message from previous resolve attempt
	if m.ResolveErr != "" {
		b.WriteString(errorStyle.Render("✗ " + m.ResolveErr))
		b.WriteString("\n\n")
	}

	// Input field
	b.WriteString(textStyle.Render("Enter URL:"))
	b.WriteString("\n")
	b.WriteString(inputStyle.Render(m.Input.View()))
	b.WriteString("\n\n")

	// Footer hints
	b.WriteString(m.renderFooter("enter", "resolve", "q", "quit"))

	return b.String()
}

// ---------------------------------------------------------------------------
// Screen: Resolving
// ---------------------------------------------------------------------------

func (m Model) renderResolvingView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("♪ music-dl"))
	b.WriteString("\n\n")

	// Spinner + message centered-ish
	spin := infoStyle.Render(m.Spinner.View())
	b.WriteString(fmt.Sprintf("\n  %s %s\n\n", spin, textStyle.Render("Resolving URL...")))
	b.WriteString(mutedStyle.Render("  This should only take a moment"))
	b.WriteString("\n\n")

	// Footer
	b.WriteString(m.renderFooter("q", "cancel"))

	return b.String()
}

// ---------------------------------------------------------------------------
// Screen: Playlist
// ---------------------------------------------------------------------------

func (m Model) renderPlaylistView() string {
	var b strings.Builder

	// Header
	total := len(m.Queue.Tracks)
	title := "Playlist"
	if total > 0 && m.Queue.Tracks[0].Artist != "" {
		title = fmt.Sprintf("%s — %d tracks", m.Queue.Tracks[0].Artist, total)
	} else {
		title = fmt.Sprintf("%d tracks", total)
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("♪ %s", title)))
	b.WriteString("\n")

	// Divider
	b.WriteString(dividerStyle.Render(strings.Repeat("─", m.Width-6)))
	b.WriteString("\n")

	// Track list
	tracks := m.filteredTracks()
	visible := m.Height - 8
	if visible < 3 {
		visible = 3
	}

	start := m.PlaylistScroll
	end := start + visible
	if end > len(tracks) {
		end = len(tracks)
	}
	if start > len(tracks) {
		start = len(tracks)
	}

	for i, track := range tracks {
		if i < start || i >= end {
			continue
		}

		line := m.renderTrackLine(track, i == m.Cursor)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n")
	footerText := "[Space] toggle  [a]ll  [n]one  [Enter] download  [/] filter  [q] quit"
	b.WriteString(mutedStyle.Render(footerText))

	return b.String()
}

func (m Model) renderTrackLine(track model.Media, isCursor bool) string {
	// Selection indicator
	var sel string
	if track.Status == model.StatusCompleted {
		sel = successStyle.Render("✓")
	} else {
		sel = mutedStyle.Render("☐")
	}

	// Cursor indicator
	cursor := " "
	if isCursor {
		cursor = emphStyle.Render("▸")
	}

	// Title + Artist
	title := track.Title
	artist := track.Artist

	// Trim to fit
	maxWidth := m.Width - 16
	if maxWidth < 20 {
		maxWidth = 20
	}
	title = ellipsis(title, maxWidth/2)
	artist = ellipsis(artist, maxWidth/2)

	line := fmt.Sprintf("%s %s %-*s  %s",
		cursor, sel, maxWidth/2, title, mutedStyle.Render(artist))

	if isCursor {
		return selectedStyle.Render(line)
	}
	return textStyle.Render(line)
}

// ---------------------------------------------------------------------------
// Screen: Downloading
// ---------------------------------------------------------------------------

func (m Model) renderDownloadingView() string {
	var b strings.Builder

	completed := m.Queue.Completed()
	failed := m.Queue.Failed()
	total := m.Queue.Len()

	// Header
	b.WriteString(titleStyle.Render(fmt.Sprintf("♪ Downloading (%d/%d)", completed+failed, total)))
	b.WriteString("\n")

	// Divider
	b.WriteString(dividerStyle.Render(strings.Repeat("─", m.Width-6)))
	b.WriteString("\n")

	// Queue list
	for i, track := range m.Queue.Tracks {
		isCurrent := i == m.Queue.Index
		line := m.renderQueueLine(track, isCurrent)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Summary line
	b.WriteString("\n")

	var parts []string
	if m.Queue.Failed() > 0 {
		parts = append(parts, errorStyle.Render(fmt.Sprintf("%d failed", m.Queue.Failed())))
	}
	if m.Queue.Completed() > 0 {
		parts = append(parts, successStyle.Render(fmt.Sprintf("%d downloaded", m.Queue.Completed())))
	}
	if m.Queue.Pending() > 0 {
		parts = append(parts, mutedStyle.Render(fmt.Sprintf("%d remaining", m.Queue.Pending())))
	}
	if len(parts) > 0 {
		b.WriteString(mutedStyle.Render(strings.Join(parts, " · ")))
		b.WriteString("\n")
	}

	// Output dir
	b.WriteString(mutedStyle.Render(fmt.Sprintf("→ %s", m.OutputDir)))
	b.WriteString("\n")

	// Footer
	b.WriteString("\n")
	b.WriteString(m.renderFooter("q", "cancel & quit"))

	return b.String()
}

func (m Model) renderQueueLine(track model.Media, isCurrent bool) string {
	status := statusLabel(track.Status)
	title := track.Title
	if track.Artist != "" {
		title = fmt.Sprintf("%s - %s", track.Artist, track.Title)
	}

	maxWidth := m.Width - 20
	if maxWidth < 20 {
		maxWidth = 20
	}
	title = ellipsis(title, maxWidth)

	line := fmt.Sprintf("  %s %s", status, textStyle.Render(title))

	if isCurrent {
		return emphStyle.Render(fmt.Sprintf("▸ %s", title))
	}
	return line
}

// ---------------------------------------------------------------------------
// Screen: Done
// ---------------------------------------------------------------------------

func (m Model) renderDoneView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("♪ Downloads Complete"))
	b.WriteString("\n\n")

	completed := m.Queue.Completed()
	failed := m.Queue.Failed()

	b.WriteString(fmt.Sprintf("  %s %d downloaded\n", successStyle.Render("✓"), completed))

	if failed > 0 {
		b.WriteString(fmt.Sprintf("  %s %d failed\n", errorStyle.Render("✗"), failed))
		// Show which tracks failed
		for _, t := range m.Queue.Tracks {
			if t.Status == model.StatusFailed {
				b.WriteString(fmt.Sprintf("     %s: %s\n",
					mutedStyle.Render(t.Title),
					errorStyle.Render(t.ErrorMsg)))
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(fmt.Sprintf("  → %s", m.OutputDir)))
	b.WriteString("\n\n")

	b.WriteString(m.renderFooter("enter", "new download", "q", "quit"))

	return b.String()
}

// ---------------------------------------------------------------------------
// Footer helper
// ---------------------------------------------------------------------------

// renderFooter builds a footer like: "[key] action  [key] action"
func (m Model) renderFooter(keysAndActions ...string) string {
	if len(keysAndActions)%2 != 0 {
		return ""
	}

	var parts []string
	for i := 0; i < len(keysAndActions); i += 2 {
		key := keysAndActions[i]
		action := keysAndActions[i+1]
		part := fmt.Sprintf("%s %s",
			keyStyle.Render("["+key+"]"),
			keyDescStyle.Render(action))
		parts = append(parts, part)
	}

	return footerStyle.Render(strings.Join(parts, "  "))
}

// ---------------------------------------------------------------------------
// Help overlay
// ---------------------------------------------------------------------------

func (m Model) renderHelpOverlay() string {
	content := strings.TrimPrefix(`
  ┌─ Keyboard Shortcuts ──────────────────────────────────┐
  │                                                        │
  │  Universal                                             │
  │    ↑/k        Move up                                  │
  │    ↓/j        Move down                                │
  │    Enter      Confirm / select                          │
  │    Esc        Go back / cancel                          │
  │    q          Quit                                      │
  │    ?          Toggle this help                          │
  │                                                        │
  │  Playlist Screen                                        │
  │    Space      Toggle track selection                   │
  │    a          Select all                               │
  │    n          Deselect all                             │
  │    /          Filter tracks                            │
  │                                                        │
  │  Download Screen                                       │
  │    q          Cancel downloads and quit                │
  │                                                        │
  │  Done Screen                                           │
  │    Enter      Start a new download                     │
  │    q          Quit                                     │
  │                                                        │
  └────────────────────────────────────────────────────────┘
`, "\n")

	return helpStyle.Render(content)
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

// ellipsis truncates s to maxLen-1 and appends "…" if necessary.
func ellipsis(s string, maxLen int) string {
	if len(s) <= maxLen || maxLen < 1 {
		return s
	}
	return s[:maxLen-1] + "…"
}
