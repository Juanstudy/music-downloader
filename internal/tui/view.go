package tui

import (
	"fmt"
	"strings"

	"github.com/Juanstudy/music-downloader/internal/config"
	"github.com/Juanstudy/music-downloader/internal/core/domain"

	"github.com/charmbracelet/lipgloss"
)

// View renders the entire TUI based on the current screen.
func (m Model) View() string {
	if !m.Ready {
		return "Loading..."
	}

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
	case ScreenConfig:
		content = m.renderConfigView()
	}

	body := appStyle.Render(content)

	if m.showHelp {
		help := helpView(m.Width)
		body = lipgloss.JoinVertical(lipgloss.Top, body, "\n", help)
	}

	return body
}

// ---------------------------------------------------------------------------
// Shared UI helpers
// ---------------------------------------------------------------------------

func (m Model) renderFatalError() string {
	return errorStyle.Render(fmt.Sprintf("Fatal error: %v", m.Err))
}

func (m Model) renderHeader(title string) string {
	return titleStyle.Render(title)
}

func (m Model) renderFooter() string {
	keys := []string{}
	if m.Screen == ScreenInput {
		keys = append(keys, keyStyle.Render("Enter")+" "+keyDescStyle.Render("resolve"))
		keys = append(keys, keyStyle.Render("Tab")+" "+keyDescStyle.Render("source"))
		keys = append(keys, keyStyle.Render("s")+" "+keyDescStyle.Render("search"))
	}
	if m.Screen == ScreenPlaylist || m.Screen == ScreenDownloading || m.Screen == ScreenDone {
		keys = append(keys, keyStyle.Render("q")+" "+keyDescStyle.Render("quit"))
	}
	if m.Screen == ScreenDownloading {
		keys = append(keys, keyStyle.Render("q")+" "+keyDescStyle.Render("quit (downloads continue)"))
	}
	if m.Screen == ScreenInput || m.Screen == ScreenPlaylist {
		keys = append(keys, keyStyle.Render("?")+" "+keyDescStyle.Render("help"))
	}
	if m.Screen == ScreenPlaylist {
		keys = append(keys,
			keyStyle.Render("Space")+" "+keyDescStyle.Render("toggle"),
			keyStyle.Render("a")+" "+keyDescStyle.Render("all"),
			keyStyle.Render("n")+" "+keyDescStyle.Render("none"),
			keyStyle.Render("/")+" "+keyDescStyle.Render("filter"),
			keyStyle.Render("Enter")+" "+keyDescStyle.Render("download"),
			keyStyle.Render("Esc")+" "+keyDescStyle.Render("back"),
		)
	}
	if m.Screen == ScreenDone {
		keys = append(keys,
			keyStyle.Render("r")+" "+keyDescStyle.Render("new URL"),
			keyStyle.Render("Esc")+" "+keyDescStyle.Render("quit"),
		)
	}
	if m.Screen == ScreenConfig {
		keys = append(keys,
			keyStyle.Render("j/k")+" "+keyDescStyle.Render("move"),
			keyStyle.Render("Enter")+" "+keyDescStyle.Render("confirm"),
			keyStyle.Render("Esc")+" "+keyDescStyle.Render("back"),
		)
	}
	return footerStyle.Render(strings.Join(keys, "  │  "))
}

// ---------------------------------------------------------------------------
// Input screen
// ---------------------------------------------------------------------------

func (m Model) renderInputView() string {
	var b strings.Builder

	b.WriteString(m.renderHeader("♪ music-dl"))
	b.WriteString("\n\n")
	if m.searchMode == SearchModeQuery {
		b.WriteString("Search YouTube Music:\n\n")
	} else {
		b.WriteString("Paste a YouTube or YouTube Music URL:\n\n")
	}
	b.WriteString(inputStyle.Render(m.Input.View()))
	b.WriteString("\n")

	// Source mode indicator
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Source: ") + m.renderSourceMode())

	// Search mode indicator
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Search: ") + m.renderSearchMode())

	if m.inputErr != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("✗ " + m.inputErr))
		b.WriteString("\n")
	}

	if m.resolveErr != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("✗ " + m.resolveErr))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderFooter())

	return b.String()
}

// renderSourceMode returns a styled string for the current source mode.
func (m Model) renderSourceMode() string {
	switch m.sourceMode {
	case SourceAuto:
		return emphStyle.Render("Auto") + mutedStyle.Render(" (Tab to switch)")
	case SourceYouTube:
		return emphStyle.Render("YouTube")
	case SourceSpotify:
		return emphStyle.Render("Spotify")
	default:
		return emphStyle.Render("Auto")
	}
}

// renderSearchMode returns a styled string for the current search mode.
func (m Model) renderSearchMode() string {
	if m.searchMode == SearchModeQuery {
		return emphStyle.Render("Search") + mutedStyle.Render(" (s to switch)")
	}
	return emphStyle.Render("URL")
}

// ---------------------------------------------------------------------------
// Resolving screen
// ---------------------------------------------------------------------------

func (m Model) renderResolvingView() string {
	var b strings.Builder

	b.WriteString(m.renderHeader("♪ music-dl"))
	b.WriteString("\n\n")
	b.WriteString(m.Spinner.View())
	if m.sourceMode == SourceSpotify {
		b.WriteString(" Resolving via Spotify...\n\n")
	} else if m.searchMode == SearchModeQuery {
		b.WriteString(" Searching YouTube Music...\n\n")
	} else {
		b.WriteString(" Resolving URL...\n\n")
	}
	b.WriteString(mutedStyle.Render(m.Input.Value()))
	b.WriteString("\n\n")
	b.WriteString(m.renderFooter())

	return b.String()
}

// ---------------------------------------------------------------------------
// Playlist screen
// ---------------------------------------------------------------------------

func (m Model) renderPlaylistView() string {
	var b strings.Builder

	b.WriteString(m.renderHeader("♪ music-dl"))
	b.WriteString("\n")

	if m.resolveErr != "" {
		b.WriteString("\n")
		b.WriteString(warningStyle.Render(m.resolveErr))
		b.WriteString("\n")
	}

	// Show filter bar when active
	if m.isFiltering {
		b.WriteString("\n")
		b.WriteString(emphStyle.Render("  Filter "))
		b.WriteString(inputStyle.Render(m.filterInput.View()))
		b.WriteString("\n")
	}

	// Show counter
	tracks := m.filteredTracks()
	filteredCount := ""
	if m.filter != "" {
		filteredCount = mutedStyle.Render(fmt.Sprintf("  (%d/%d)", len(tracks), len(m.tracks)))
	}
	b.WriteString(fmt.Sprintf("\n%s%s\n\n",
		mutedStyle.Render(fmt.Sprintf("%d tracks — select with Space, then Enter to download", len(m.tracks))),
		filteredCount))
	start := m.scroll
	end := start + (m.Height - 10)
	if end > len(tracks) {
		end = len(tracks)
	}
	if start > end {
		start = 0
	}
	visible := tracks[start:end]
	globalStart := start

	for i, track := range visible {
		idx := globalStart + i
		prefix := "  "
		cursor := " "
		globalIdx := m.findTrackIndex(track)

		if idx == m.cursor {
			cursor = "▸"
		}

		status := statusChar(track.Status)
		title := track.Title
		if idx == m.cursor && m.cursor < len(tracks) {
			title = emphStyle.Render(track.Title)
		}

		artist := ""
		if track.Artist != "" {
			artist = mutedStyle.Render(" — " + track.Artist)
		}

		if idx == m.cursor {
			b.WriteString(selectedStyle.Render(fmt.Sprintf("%s %s %s%s", cursor, status, title, artist)))
		} else {
			b.WriteString(fmt.Sprintf("%s %s %s%s", prefix, status, title, artist))
		}

		// Show queue info for this track
		if m.tracks[globalIdx].Status != domain.StatusPending {
			b.WriteString("  " + statusLabel(m.tracks[globalIdx].Status))
		}

		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderFooter())

	return b.String()
}

// ---------------------------------------------------------------------------
// Downloading screen
// ---------------------------------------------------------------------------

func (m Model) renderDownloadingView() string {
	var b strings.Builder

	b.WriteString(m.renderHeader("♪ music-dl — Downloading"))
	b.WriteString("\n\n")

	total := m.succeeded + m.failed + 1 // +1 for current
	for i, track := range m.tracks {
		if track.Status == domain.StatusDone || track.Status == domain.StatusFailed || track.Status == domain.StatusDownloading {
			prefix := "  "
			if track.Status == domain.StatusDownloading {
				prefix = m.Spinner.View() + " "
			}

			idx := fmt.Sprintf("%d/%d ", i+1, total)
			title := track.Title
			artist := ""
			if track.Artist != "" {
				artist = mutedStyle.Render(" — " + track.Artist)
			}

			status := ""
			switch track.Status {
			case domain.StatusDownloading:
				status = emphStyle.Render(" DOWNLOADING")
			case domain.StatusDone:
				status = successStyle.Render(" ✓ done")
			case domain.StatusFailed:
				status = errorStyle.Render(" ✗ " + track.Error)
			}

			b.WriteString(fmt.Sprintf("%s%s%s%s%s\n", prefix, mutedStyle.Render(idx), title, artist, status))
		}
	}

	b.WriteString("\n")
	b.WriteString(m.renderFooter())

	return b.String()
}

// ---------------------------------------------------------------------------
// Done screen
// ---------------------------------------------------------------------------

func (m Model) renderDoneView() string {
	var b strings.Builder

	b.WriteString(m.renderHeader("♪ music-dl — Complete"))
	b.WriteString(fmt.Sprintf("\n\n  %s  %s\n\n",
		successStyle.Render(fmt.Sprintf("%d downloaded", m.succeeded)),
		errorStyle.Render(fmt.Sprintf("%d failed", m.failed)),
	))

	if m.succeeded > 0 {
		b.WriteString(emphStyle.Render("  Downloaded files:"))
		b.WriteString("\n")
		for _, track := range m.tracks {
			if track.Status == domain.StatusDone && track.OutputPath != "" {
				title := track.Title
				if track.Artist != "" {
					title = track.Artist + " - " + title
				}
				b.WriteString(fmt.Sprintf("    %s  %s\n", successStyle.Render("✓"), mutedStyle.Render(title)))
				b.WriteString(fmt.Sprintf("       %s\n", mutedStyle.Render(track.OutputPath)))
			}
		}
		b.WriteString("\n")
	}

	if m.failed > 0 {
		b.WriteString(errorStyle.Render("  Failed tracks:"))
		b.WriteString("\n")
		for _, track := range m.failedTracks {
			b.WriteString(fmt.Sprintf("    %s  %s — %s\n",
				errorStyle.Render("✗"),
				track.Title,
				mutedStyle.Render(track.Error),
			))
		}
		b.WriteString("\n")
	}

	b.WriteString(mutedStyle.Render("  Press r to start a new URL, Esc/q to quit."))
	b.WriteString("\n\n")
	b.WriteString(m.renderFooter())

	return b.String()
}

// ---------------------------------------------------------------------------
// Config screen
// ---------------------------------------------------------------------------

// renderConfigView lists the three quality options with a cursor/selection
// indicator, the current effective quality, a save-failure warning when
// present, and the footer hint.
func (m Model) renderConfigView() string {
	var b strings.Builder

	b.WriteString(m.renderHeader("♪ music-dl — Configure Quality"))
	b.WriteString("\n\n")

	if m.configWarn != "" {
		b.WriteString(warningStyle.Render("⚠ " + m.configWarn))
		b.WriteString("\n\n")
	}

	b.WriteString(mutedStyle.Render("Audio quality (yt-dlp never up-samples: a lower source stays lower)"))
	b.WriteString("\n\n")

	for i, q := range config.ValidQualities() {
		marker := " "
		cursor := "  "
		if i == m.qualityCursor {
			cursor = "▸"
			marker = "●"
		}
		line := fmt.Sprintf("%s %s %s", cursor, marker, q)
		if i == m.qualityCursor {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Current: ") + emphStyle.Render(m.audioQuality))
	b.WriteString("\n\n")
	b.WriteString(m.renderFooter())

	return b.String()
}
