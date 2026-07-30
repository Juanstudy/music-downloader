package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
	"github.com/Juanstudy/music-downloader/internal/core/service"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const downloadTimeout = 5 * time.Minute

// ---------------------------------------------------------------------------
// Tea Update entry point
// ---------------------------------------------------------------------------

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Global messages — handled regardless of screen
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Ready = true
		m.Input.Width = msg.Width - 10
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.Screen == ScreenInput {
				return m, tea.Quit
			}
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		}

	case spinner.TickMsg:
		if m.Screen == ScreenResolving {
			var cmd tea.Cmd
			m.Spinner, cmd = m.Spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case resolveFinishedMsg:
		return m.handleResolveDone(msg)

	case trackDownloadedMsg:
		return m.handleTrackDone(msg)
	}

	// Screen-specific routing
	switch m.Screen {
	case ScreenInput:
		return m.handleInputKeys(msg)
	case ScreenPlaylist:
		return m.handlePlaylistKeys(msg)
	case ScreenResolving:
		if km, ok := msg.(tea.KeyMsg); ok {
			return m.handleResolvingKeys(km)
		}
		return m, nil
	case ScreenDownloading:
		if km, ok := msg.(tea.KeyMsg); ok {
			return m.handleDownloadingKeys(km)
		}
		return m, nil
	case ScreenDone:
		if km, ok := msg.(tea.KeyMsg); ok {
			return m.handleDoneKeys(km)
		}
		return m, nil
	default:
		return m, nil
	}
}

// ---------------------------------------------------------------------------
// Input screen
// ---------------------------------------------------------------------------

func (m Model) handleInputKeys(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Search mode toggle — intercept before input widget
		if msg.String() == "s" {
			return m.toggleSearchMode()
		}
		switch msg.Type {
		case tea.KeyEnter:
			val := strings.TrimSpace(m.Input.Value())
			m.inputErr = ""
			if val == "" {
				m.inputErr = "Please enter a URL"
				return m, nil
			}
			if m.searchMode == SearchModeQuery {
				return m.startQuerySearch(val)
			}
			// URL mode: check if the input looks like a URL
			if !strings.Contains(val, "://") {
				m.inputErr = "That doesn't look like a URL. Press 's' to switch to Search mode."
				return m, nil
			}
			return m.startResolve(val)
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyTab:
			switch m.sourceMode {
			case SourceAuto:
				if m.spotifySearcher != nil {
					m.sourceMode = SourceSpotify
				} else {
					m.sourceMode = SourceYouTube
				}
			case SourceYouTube:
				if m.spotifySearcher != nil {
					m.sourceMode = SourceSpotify
				} else {
					m.sourceMode = SourceAuto
				}
			case SourceSpotify:
				m.sourceMode = SourceAuto
			}
			return m, nil
		}
	}

	// Default: let the input handle its own key events
	var cmd tea.Cmd
	m.Input, cmd = m.Input.Update(msg)
	return m, cmd
}

func (m Model) startResolve(url string) (tea.Model, tea.Cmd) {
	m.Screen = ScreenResolving
	m.PrevScreen = ScreenInput
	m.Input.Blur()
	// Bump ID so spinner resets on re-resolve
	m.InputID++
	return m, resolveCmd(m.selectedSearcher(), url)
}

// selectedSearcher returns the Searcher that should be used based on the current source mode.
func (m Model) selectedSearcher() ports.Searcher {
	switch m.sourceMode {
	case SourceSpotify:
		if m.spotifySearcher != nil {
			return m.spotifySearcher
		}
		return m.searcher
	case SourceYouTube:
		return m.searcher
	case SourceAuto:
		// Auto-detect by URL — default to Spotify when available
		if m.spotifySearcher != nil {
			return m.spotifySearcher
		}
		return m.searcher
	default:
		return m.searcher
	}
}

// resolveCmd creates a tea.Cmd that runs URL resolution in a goroutine.
func resolveCmd(s ports.Searcher, url string) tea.Cmd {
	return func() tea.Msg {
		result, err := s.Search(context.Background(), url)
		if err != nil {
			return resolveFinishedMsg{tracks: result.Tracks, err: err}
		}
		return resolveFinishedMsg{tracks: result.Tracks, err: nil}
	}
}

// ---------------------------------------------------------------------------
// Search mode (PR3)
// ---------------------------------------------------------------------------

// toggleSearchMode switches between URL input and free-text search.
// It cleans up search state and cancels any in-flight search.
func (m Model) toggleSearchMode() (tea.Model, tea.Cmd) {
	// Cancel any in-flight search before switching mode
	if m.searchCancel != nil {
		m.searchCancel()
		m.searchCancel = nil
	}

	if m.searchMode == SearchModeURL {
		m.searchMode = SearchModeQuery
		m.Input.Placeholder = "search query..."
	} else {
		m.searchMode = SearchModeURL
		m.Input.Placeholder = "https://music.youtube.com/..."
	}
	m.Input.SetValue("")
	m.inputErr = ""
	m.resolveErr = ""
	m.filter = ""

	if m.Screen != ScreenInput {
		m.tracks = nil
		m.cursor = 0
		m.scroll = 0
		m.filter = ""
		m.Screen = ScreenInput
		m.PrevScreen = ScreenInput
		m.Input.Focus()
	}
	return m, nil
}

// startQuerySearch initiates a YouTube Music search for the given query.
func (m Model) startQuerySearch(query string) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(query) == "" {
		m.inputErr = "Please enter a search query"
		return m, nil
	}
	if m.querySearcher == nil {
		m.inputErr = "Search is not available: no search adapter configured"
		return m, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.searchCancel = cancel
	m.Screen = ScreenResolving
	m.PrevScreen = ScreenInput
	m.Input.Blur()
	m.InputID++
	return m, searchResolveCmd(ctx, m.querySearcher, query, 10)
}

// searchResolveCmd creates a tea.Cmd that runs a YouTube Music search query
// with a 30-second timeout to prevent hanging. The ctx parameter allows the
// caller to cancel the in-flight search (e.g., when navigating away).
func searchResolveCmd(ctx context.Context, qs ports.QuerySearcher, query string, limit int) tea.Cmd {
	return func() tea.Msg {
		if qs == nil {
			return resolveFinishedMsg{err: errors.New("search adapter not configured")}
		}
		// Derive a timeout context from the parent (which may be cancelled by navigation)
		searchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		result, err := qs.SearchByQuery(searchCtx, query, limit)
		if err != nil {
			return resolveFinishedMsg{tracks: result.Tracks, err: err}
		}
		return resolveFinishedMsg{tracks: result.Tracks, err: nil}
	}
}

// ---------------------------------------------------------------------------
// Resolving screen
// ---------------------------------------------------------------------------

func (m Model) handleResolvingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "s":
		return m.toggleSearchMode()
	case "esc":
		if m.searchCancel != nil {
			m.searchCancel()
			m.searchCancel = nil
		}
		m.Screen = ScreenInput
		m.PrevScreen = ScreenResolving
		m.Input.Focus()
		return m, nil
	}
	return m, nil
}

func (m Model) handleResolveDone(msg resolveFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		// Partial results: we have tracks AND an error (mid-playlist failure)
		if len(msg.tracks) > 0 {
			m.tracks = msg.tracks
			m.cursor = 0
			m.scroll = 0
			m.Screen = ScreenPlaylist
			m.PrevScreen = ScreenResolving
			m.resolveErr = fmt.Sprintf("warning: resolve completed with errors: %v", msg.err)
			return m, nil
		}
		m.Screen = ScreenInput
		m.PrevScreen = ScreenResolving
		m.Input.Focus()
		m.resolveErr = msg.err.Error()
		return m, nil
	}

	if len(msg.tracks) == 0 {
		m.Screen = ScreenInput
		m.PrevScreen = ScreenResolving
		m.Input.Focus()
		m.resolveErr = "no tracks found"
		return m, nil
	}

	m.tracks = msg.tracks
	m.cursor = 0
	m.scroll = 0
	m.filter = ""
	m.isFiltering = false
	m.filterInput.SetValue("")
	m.resolveErr = ""

	// Single track: auto-select and start download
	if len(m.tracks) == 1 {
		m.tracks[0].Status = domain.StatusDone
		m.Screen = ScreenDownloading
		m.PrevScreen = ScreenResolving
		return m.startDownload()
	}

	// Multiple tracks: show playlist for user selection
	m.Screen = ScreenPlaylist
	m.PrevScreen = ScreenResolving
	return m, nil
}

// ---------------------------------------------------------------------------
// Playlist screen
// ---------------------------------------------------------------------------

func (m Model) filteredTracks() []domain.Media {
	if m.filter == "" {
		return m.tracks
	}
	lower := strings.ToLower(m.filter)
	var filtered []domain.Media
	for _, t := range m.tracks {
		if strings.Contains(strings.ToLower(t.Title), lower) ||
			strings.Contains(strings.ToLower(t.Artist), lower) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func (m Model) findTrackIndex(track domain.Media) int {
	for i, t := range m.tracks {
		if t.URL == track.URL {
			return i
		}
	}
	return -1
}

func (m Model) handlePlaylistKeys(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Filter mode active: route keys to the filter input
	if m.isFiltering {
		return m.handlePlaylistFilterInput(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		tracks := m.filteredTracks()

		switch msg.String() {
		case "j", "down":
			if m.cursor < len(tracks)-1 {
				m.cursor++
			}
			m.ensureVisible()
			return m, nil

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
			m.ensureVisible()
			return m, nil

		case " ":
			if m.cursor >= 0 && m.cursor < len(tracks) {
				idx := m.findTrackIndex(tracks[m.cursor])
				if idx >= 0 {
					if m.tracks[idx].Status == domain.StatusPending {
						m.tracks[idx].Status = domain.StatusDone
					} else {
						m.tracks[idx].Status = domain.StatusPending
					}
				}
			}
			return m, nil

		case "a":
			for i := range m.tracks {
				if m.tracks[i].Status == domain.StatusPending {
					m.tracks[i].Status = domain.StatusDone
				}
			}
			return m, nil

		case "n":
			for i := range m.tracks {
				if m.tracks[i].Status == domain.StatusDone {
					m.tracks[i].Status = domain.StatusPending
				}
			}
			return m, nil

		case "/":
			// Enter filter mode
			m.isFiltering = true
			m.filterInput.Focus()
			m.filterInput.SetValue(m.filter)
			return m, textinput.Blink

		case "enter":
			return m.startDownload()

		case "esc":
			if m.searchCancel != nil {
				m.searchCancel()
				m.searchCancel = nil
			}
			m.Screen = ScreenInput
			m.tracks = nil
			m.cursor = 0
			m.scroll = 0
			m.filter = ""
			m.resolveErr = ""
			m.Input.SetValue("")
			return m, nil

		case "s":
			if m.searchCancel != nil {
				m.searchCancel()
				m.searchCancel = nil
			}
			return m.toggleSearchMode()

		case "q":
			return m, tea.Quit

		default:
			return m, nil
		}
	}
	return m, nil
}

// handlePlaylistFilterInput routes key events to the filter text input
// when the user is actively typing a filter.
func (m Model) handlePlaylistFilterInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Exit filter mode and clear filter
			m.isFiltering = false
			m.filter = ""
			m.filterInput.SetValue("")
			m.filterInput.Blur()
			return m, nil

		case "enter":
			// Apply filter and exit filter mode
			m.filter = m.filterInput.Value()
			m.isFiltering = false
			m.filterInput.Blur()
			// Clamp cursor to the new filtered list
			m.clampCursor(len(m.filteredTracks()))
			return m, nil
		}
	}

	// Route to the filter input widget
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	// Update filter text in real-time so filteredTracks() stays in sync
	m.filter = m.filterInput.Value()
	m.clampCursor(len(m.filteredTracks()))
	m.ensureVisible()
	return m, cmd
}

// clampCursor ensures cursor stays within valid bounds for the given track count.
func (m *Model) clampCursor(trackCount int) {
	if trackCount == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= trackCount {
		m.cursor = trackCount - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m Model) ensureVisible() {
	visible := m.Height - 8
	if visible < 3 {
		visible = 3
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+visible {
		m.scroll = m.cursor - visible + 1
	}
}

// ---------------------------------------------------------------------------
// Download flow
// ---------------------------------------------------------------------------

func (m Model) startDownload() (tea.Model, tea.Cmd) {
	// Count selected tracks
	var selected []int
	for i, t := range m.tracks {
		if t.Status == domain.StatusDone || t.Status == domain.StatusResolved {
			selected = append(selected, i)
		}
	}
	if len(selected) == 0 {
		// Nothing selected, nothing to download
		return m, nil
	}

	// Mark first selected track as downloading
	firstIdx := selected[0]
	if m.tracks[firstIdx].Status != domain.StatusResolved {
		m.tracks[firstIdx].Status = domain.StatusResolved
	}
	m.downloadIdx = firstIdx
	m.tracks[firstIdx].Status = domain.StatusDownloading
	m.succeeded = 0
	m.failed = 0
	m.failedTracks = nil
	m.Screen = ScreenDownloading
	m.PrevScreen = ScreenPlaylist

	return m, downloadTrackCmd(m.orchestrator, m.tracks[firstIdx], m.outputDir, firstIdx)
}

func downloadTrackCmd(o *service.Orchestrator, media domain.Media, outputDir string, idx int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
		defer cancel()
		updated, err := o.DownloadTrack(ctx, media, outputDir)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				err = fmt.Errorf("download timed out after 5m: check your connection")
			}
		}
		return trackDownloadedMsg{index: idx, media: updated, err: err}
	}
}

// ---------------------------------------------------------------------------
// Downloading screen
// ---------------------------------------------------------------------------

func (m Model) handleDownloadingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleTrackDone(msg trackDownloadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.tracks[msg.index].Status = domain.StatusFailed
		m.tracks[msg.index].Error = msg.err.Error()
		m.failed++
		m.failedTracks = append(m.failedTracks, m.tracks[msg.index])
	} else {
		m.tracks[msg.index].Status = domain.StatusDone
		m.tracks[msg.index].OutputPath = msg.media.OutputPath
		m.succeeded++
	}

	// Find next track in StatusResolved
	nextIdx := -1
	for i, t := range m.tracks {
		if t.Status == domain.StatusResolved {
			nextIdx = i
			break
		}
	}

	if nextIdx >= 0 {
		// Start next download
		m.downloadIdx = nextIdx
		m.tracks[nextIdx].Status = domain.StatusDownloading
		return m, downloadTrackCmd(m.orchestrator, m.tracks[nextIdx], m.outputDir, nextIdx)
	}

	// All selected tracks processed
	m.Screen = ScreenDone
	m.PrevScreen = ScreenDownloading
	return m, nil
}

// ---------------------------------------------------------------------------
// Done screen
// ---------------------------------------------------------------------------

func (m Model) handleDoneKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		return m, tea.Quit
	case "r":
		// Reset to input screen for another URL
		m.Screen = ScreenInput
		m.tracks = nil
		m.cursor = 0
		m.scroll = 0
		m.succeeded = 0
		m.failed = 0
		m.failedTracks = nil
		m.filter = ""
		m.isFiltering = false
		m.filterInput.SetValue("")
		m.resolveErr = ""
		m.Input.SetValue("")
		m.Input.Focus()
		return m, nil
	}
	return m, nil
}
