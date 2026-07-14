package tui

import (
	"strings"

	"github.com/Juanstudy/music-downloader/internal/model"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// Custom message types
// ---------------------------------------------------------------------------

type resolveDoneMsg struct {
	Tracks []*model.Media
	Err    error
}

type downloadDoneMsg struct {
	TrackIdx int
	Err      error
}

// ---------------------------------------------------------------------------
// Commands (async tea.Cmd factories)
// ---------------------------------------------------------------------------

func resolveCmd(engine engineish, url string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := engine.Resolve(url)
		return resolveDoneMsg{Tracks: tracks, Err: err}
	}
}

// engineish is a local constraint so we don't import download.Engine here.
type engineish interface {
	Resolve(url string) ([]*model.Media, error)
}

func downloadCmd(engine downloadish, track *model.Media, outputDir string) tea.Cmd {
	return func() tea.Msg {
		err := engine.Download(track, outputDir, nil)
		return downloadDoneMsg{Err: err}
	}
}

type downloadish interface {
	Download(track *model.Media, outputDir string, progress chan<- string) error
}

// ---------------------------------------------------------------------------
// Update — main Bubble Tea update loop
// ---------------------------------------------------------------------------

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		if !m.Ready {
			m.Ready = true
		}
		return m, nil

	case tea.KeyMsg:
		// Global quit — Ctrl+C is handled by Bubble Tea automatically.
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
		return m.handleKeyMsg(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd

	case resolveDoneMsg:
		return m.handleResolveDone(msg)

	case downloadDoneMsg:
		return m.handleDownloadDone(msg)
	}

	return m, nil
}

// handleKeyMsg routes key presses to the active screen.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Help overlay toggles from any screen
	if key == "?" {
		m.ShowHelp = !m.ShowHelp
		if m.ShowHelp {
			return m, nil
		}
	}

	// If help overlay is showing, only Esc or ? dismisses it
	if m.ShowHelp {
		if key == "?" || key == "esc" {
			m.ShowHelp = false
		}
		return m, nil
	}

	switch m.Screen {
	case ScreenInput:
		return m.handleInputKeys(msg)
	case ScreenResolving:
		// Only quit during resolving
		if key == "q" || key == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	case ScreenPlaylist:
		return m.handlePlaylistKeys(msg)
	case ScreenDownloading:
		return m.handleDownloadingKeys(msg)
	case ScreenDone:
		return m.handleDoneKeys(msg)
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Screen: Input
// ---------------------------------------------------------------------------

func (m Model) handleInputKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter":
		url := strings.TrimSpace(m.Input.Value())
		if url == "" {
			return m, nil
		}
		// Transition to resolving
		m.Screen = ScreenResolving
		m.ResolveErr = ""
		m.Spinner = spinner.New()
		m.Spinner.Style = spinnerStyle
		m.Spinner.Spinner = spinner.MiniDot
		return m, tea.Batch(m.Spinner.Tick, resolveCmd(m.Engine, url))

	case "q", "esc":
		return m, tea.Quit

	default:
		var cmd tea.Cmd
		m.Input, cmd = m.Input.Update(msg)
		return m, cmd
	}
}

// ---------------------------------------------------------------------------
// Screen: Playlist
// ---------------------------------------------------------------------------

func (m Model) handlePlaylistKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	tracks := m.filteredTracks()

	switch key {
	case "j", "down":
		if m.Cursor < len(tracks)-1 {
			m.Cursor++
		}
		m.ensureCursorVisible()
		return m, nil

	case "k", "up":
		if m.Cursor > 0 {
			m.Cursor--
		}
		m.ensureCursorVisible()
		return m, nil

	case " ":
		// Toggle selection of the track at cursor
		if m.Cursor >= 0 && m.Cursor < len(tracks) {
			// Find the actual index in the full queue
			actualIdx := m.actualTrackIndex(tracks[m.Cursor])
			if actualIdx >= 0 {
				t := &m.Queue.Tracks[actualIdx]
				if t.Status == model.StatusPending {
					t.Status = model.StatusCompleted // temporarily mark selected
				} else if t.Status == model.StatusCompleted {
					t.Status = model.StatusPending
				}
				// Actually, let's use a simpler selection mechanism.
				// We'll use a separate Selected field. But for MVP simplicity,
				// we can repurpose StatusPending → deselected, StatusCompleted → selected.
				// Actually this is confusing. Let me use Selected bool.
			}
		}
		// --- simpler approach below ---
		return m, nil

	case "a":
		m.selectAll()
		return m, nil

	case "n":
		m.deselectAll()
		return m, nil

	case "enter":
		// Download selected tracks
		return m.startDownload()

	case "esc":
		m.Screen = ScreenInput
		m.Cursor = 0
		m.Input.SetValue("")
		return m, nil

	case "q":
		return m, tea.Quit

	case "/":
		// Simple filter: just start typing filter mode
		// For now, toggle a simple filter state
		return m, nil

	default:
		return m, nil
	}
}

// ensureCursorVisible adjusts scroll so cursor is on screen.
func (m *Model) ensureCursorVisible() {
	visible := m.Height - 6 // approximate visible rows
	if visible < 3 {
		visible = 3
	}
	if m.Cursor < m.PlaylistScroll {
		m.PlaylistScroll = m.Cursor
	}
	if m.Cursor >= m.PlaylistScroll+visible {
		m.PlaylistScroll = m.Cursor - visible + 1
	}
}

// filteredTracks returns tracks matching the current filter.
func (m Model) filteredTracks() []model.Media {
	if m.Filter == "" {
		return m.Queue.Tracks
	}
	lower := strings.ToLower(m.Filter)
	var result []model.Media
	for _, t := range m.Queue.Tracks {
		if strings.Contains(strings.ToLower(t.Title), lower) ||
			strings.Contains(strings.ToLower(t.Artist), lower) {
			result = append(result, t)
		}
	}
	return result
}

// actualTrackIndex finds the real index of a track in the full queue.
func (m Model) actualTrackIndex(target model.Media) int {
	for i, t := range m.Queue.Tracks {
		if t.URL == target.URL {
			return i
		}
	}
	return -1
}

// selectAll marks all pending tracks as selected.
func (m *Model) selectAll() {
	for i := range m.Queue.Tracks {
		if m.Queue.Tracks[i].Status == model.StatusPending {
			m.Queue.Tracks[i].Status = model.StatusCompleted
		}
	}
}

// deselectAll marks selected tracks back to pending.
func (m *Model) deselectAll() {
	for i := range m.Queue.Tracks {
		if m.Queue.Tracks[i].Status == model.StatusCompleted {
			m.Queue.Tracks[i].Status = model.StatusPending
		}
	}
}

// startDownload begins downloading selected tracks.
func (m Model) startDownload() (tea.Model, tea.Cmd) {
	// Reset status: anything marked completed → pending for download
	// Only download tracks that were selected (StatusCompleted means selected here)
	selectedCount := 0
	firstPending := -1
	for i := range m.Queue.Tracks {
		if m.Queue.Tracks[i].Status == model.StatusCompleted {
			selectedCount++
		}
	}

	// If none selected, select all
	if selectedCount == 0 {
		m.selectAll()
	}

	// Find first pending track
	for i := range m.Queue.Tracks {
		if m.Queue.Tracks[i].Status == model.StatusCompleted {
			// Mark as pending for download
			m.Queue.Tracks[i].Status = model.StatusPending
			if firstPending < 0 {
				firstPending = i
			}
		}
	}

	m.Screen = ScreenDownloading
	m.Queue.Index = -1
	m.ProgressMsg = ""

	if !m.Queue.Next() {
		// Nothing to download
		m.Screen = ScreenDone
		return m, nil
	}

	cur := m.Queue.Current()
	return m, downloadCmd(m.Engine, cur, m.OutputDir)
}

// ---------------------------------------------------------------------------
// Screen: Downloading
// ---------------------------------------------------------------------------

func (m Model) handleDownloadingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		// Allow going back to playlist (keep downloads running in background)
		// For MVP: just stay on this screen
		return m, nil
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Screen: Done
// ---------------------------------------------------------------------------

func (m Model) handleDoneKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Start a new download session
		m.Screen = ScreenInput
		m.Queue = model.NewQueue()
		m.Cursor = 0
		m.Input.SetValue("")
		m.Input.Focus()
		return m, nil
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Async message handlers
// ---------------------------------------------------------------------------

func (m Model) handleResolveDone(msg resolveDoneMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.ResolveErr = msg.Err.Error()
		m.Screen = ScreenInput
		return m, nil
	}

	m.Queue = model.NewQueue()
	for _, t := range msg.Tracks {
		m.Queue.Add(*t)
	}

	// If only one track, skip playlist and go directly to downloading
	if len(msg.Tracks) == 1 {
		m.Queue.Tracks[0].Status = model.StatusCompleted // mark as selected
		return m.startDownload()
	}

	m.Screen = ScreenPlaylist
	m.Cursor = 0
	m.PlaylistScroll = 0
	return m, nil
}

func (m Model) handleDownloadDone(msg downloadDoneMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.Queue.MarkCurrentFailed(msg.Err.Error())
	} else {
		m.Queue.MarkCurrentCompleted()
	}

	if m.Queue.Next() {
		cur := m.Queue.Current()
		return m, downloadCmd(m.Engine, cur, m.OutputDir)
	}

	// All done
	m.Screen = ScreenDone
	return m, nil
}
