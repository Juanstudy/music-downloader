// Package tui implements the Bubble Tea terminal UI for music-downloader.
package tui

import (
	"github.com/Juanstudy/music-downloader/internal/download"
	"github.com/Juanstudy/music-downloader/internal/model"
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

// Model holds all application state for the Bubble Tea program.
type Model struct {
	Screen     Screen
	PrevScreen Screen
	Width      int
	Height     int
	Ready      bool // true after first WindowSizeMsg

	Queue     *model.Queue
	Engine    download.Engine
	OutputDir string

	// Input screen
	Input textinput.Model

	// Resolving screen
	ResolveErr string
	Spinner    spinner.Model

	// Playlist screen
	Cursor         int
	PlaylistScroll int
	Filter         string
	ShowHelp       bool

	// Downloading screen
	ProgressMsg string

	// Done screen state is read from Queue

	Err error // fatal error to display before quitting
}

// Init initializes the Bubble Tea program. Returns any initial commands.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// NewModel creates the initial application model.
func NewModel(engine download.Engine, outputDir string) Model {
	ti := textinput.New()
	ti.Placeholder = "https://music.youtube.com/..."
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 60

	s := spinner.New()
	s.Style = spinnerStyle
	s.Spinner = spinner.MiniDot

	return Model{
		Screen:     ScreenInput,
		PrevScreen: ScreenInput,
		Ready:      false,
		Queue:      model.NewQueue(),
		Engine:     engine,
		OutputDir:  outputDir,
		Input:      ti,
		Spinner:    s,
		Cursor:     0,
	}
}
