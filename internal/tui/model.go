// Package tui implements the Bubble Tea terminal UI for music-downloader.
package tui

import (
	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
	"github.com/Juanstudy/music-downloader/internal/core/service"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Screen identifies the active view in the TUI.
type Screen int

const (
	ScreenInput Screen = iota
	ScreenResolving
	ScreenPlaylist
	ScreenDownloading
	ScreenDone
)

// SourceMode controls which search backend is active.
type SourceMode int

const (
	SourceAuto SourceMode = iota
	SourceYouTube
	SourceSpotify
)

// Model holds all application state for the Bubble Tea program.
type Model struct {
	Screen     Screen
	PrevScreen Screen
	Width      int
	Height     int
	Ready      bool // true after first WindowSizeMsg

	// Dependencies (injected)
	orchestrator     *service.Orchestrator
	searcher         ports.Searcher
	spotifySearcher  ports.Searcher
	outputDir        string

	// Source selection
	sourceMode SourceMode

	// Input screen
	Input   textinput.Model
	InputID int // incrementing ID to reset spinner on retry

	// Resolving screen
	resolveErr string
	Spinner    spinner.Model

	// Track list (populated after resolve)
	tracks      []domain.Media
	cursor      int
	scroll      int
	showHelp    bool
	filter      string
	filterInput textinput.Model
	isFiltering bool

	// Download progress
	succeeded    int
	failed       int
	failedTracks []domain.Media
	downloadIdx  int // index of track currently being downloaded
	progressMsg  string

	// Output tracking
	inputErr string

	Err error // fatal error to display before quitting
}

// Init initializes the Bubble Tea program. Returns any initial commands.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// NewModel creates the initial application model with hexagonal wiring.
// youtubeSearcher is required; spotifySearcher is optional (pass nil when unavailable).
func NewModel(orch *service.Orchestrator, youtubeSearcher, spotifySearcher ports.Searcher, outputDir string) Model {
	ti := textinput.New()
	ti.Placeholder = "https://music.youtube.com/..."
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 60

	s := spinner.New()
	s.Style = spinnerStyle
	s.Spinner = spinner.MiniDot

	fi := textinput.New()
	fi.Placeholder = "Filter by title or artist..."
	fi.CharLimit = 60
	fi.Width = 40

	return Model{
		Screen:         ScreenInput,
		PrevScreen:     ScreenInput,
		Ready:          false,
		orchestrator:   orch,
		searcher:       youtubeSearcher,
		spotifySearcher: spotifySearcher,
		sourceMode:     SourceAuto,
		outputDir:      outputDir,
		Input:          ti,
		Spinner:        s,
		filterInput:    fi,
		cursor:         0,
		scroll:         0,
	}
}
