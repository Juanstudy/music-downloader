// music-dl is a TUI music downloader powered by yt-dlp.
//
// Usage: music-dl
//
// Opens an interactive terminal UI. Paste a YouTube or YouTube Music URL,
// select tracks, and download them as MP3 with embedded metadata.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Juanstudy/music-downloader/internal/download"
	"github.com/Juanstudy/music-downloader/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	log.SetPrefix("music-dl: ")
	log.SetFlags(0)

	// Pre-flight check: verify dependencies before starting the TUI.
	// Per ADR-004: check pre-TUI, fail fast with a clear message.
	engine := &download.YtDlpEngine{}
	if err := engine.CheckInstalled(); err != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  music-dl requires yt-dlp and ffmpeg to be installed:")
		fmt.Fprintln(os.Stderr, "    brew install yt-dlp ffmpeg          (macOS)")
		fmt.Fprintln(os.Stderr, "    sudo apt install yt-dlp ffmpeg      (Debian/Ubuntu)")
		fmt.Fprintln(os.Stderr, "    sudo pacman -S yt-dlp ffmpeg        (Arch Linux)")
		fmt.Fprintln(os.Stderr, "    pip install yt-dlp && install ffmpeg (others)")
		os.Exit(1)
	}

	// Determine output directory (ADR-005: ~/Music/music-dl/ by default).
	outputDir := defaultOutputDir()

	// Start the Bubble Tea TUI program.
	m := tui.NewModel(engine, outputDir)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		log.Fatalf("error running TUI: %v", err)
	}
}

// defaultOutputDir returns the default download directory.
// Follows the XDG/Music convention: $HOME/Music/music-dl/.
func defaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./downloads"
	}
	return filepath.Join(home, "Music", "music-dl")
}
