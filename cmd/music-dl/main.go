// music-dl is a TUI music downloader powered by yt-dlp.
//
// Usage: music-dl
//
// Opens an interactive terminal UI. Paste a YouTube or YouTube Music URL,
// select tracks, and download them as MP3 with embedded metadata.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Juanstudy/music-downloader/internal/adapters/downloader"
	"github.com/Juanstudy/music-downloader/internal/adapters/preflight"
	"github.com/Juanstudy/music-downloader/internal/adapters/querysearcher"
	"github.com/Juanstudy/music-downloader/internal/adapters/searcher"
	"github.com/Juanstudy/music-downloader/internal/adapters/spotify"
	"github.com/Juanstudy/music-downloader/internal/config"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
	"github.com/Juanstudy/music-downloader/internal/core/service"
	"github.com/Juanstudy/music-downloader/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	log.SetPrefix("music-dl: ")
	log.SetFlags(0)

	// Pre-flight check: verify dependencies before starting the TUI.
	checker := preflight.NewChecker("yt-dlp")
	if err := checker.Check(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  music-dl requires yt-dlp and ffmpeg to be installed:")
		fmt.Fprintln(os.Stderr, "    brew install yt-dlp ffmpeg          (macOS)")
		fmt.Fprintln(os.Stderr, "    sudo apt install yt-dlp ffmpeg      (Debian/Ubuntu)")
		fmt.Fprintln(os.Stderr, "    sudo pacman -S yt-dlp ffmpeg        (Arch Linux)")
		fmt.Fprintln(os.Stderr, "    pip install yt-dlp && install ffmpeg (others)")
		os.Exit(1)
	}

	// Determine output directory (default: ~/Music/music-dl/).
	outputDir := defaultOutputDir()

	// Load the audio quality setting (defaults to 320k). A missing config file
	// falls back silently; a malformed one logs a warning and keeps the default
	// — both are non-fatal (AQ-003, AQ-016).
	quality := config.DefaultQuality
	qualityCfg, qualityErr := config.LoadConfig(config.ConfigPath())
	if qualityErr != nil {
		log.Printf("warning: failed to load config: %v", qualityErr)
	} else {
		quality = qualityCfg.Quality.Value
	}

	// Wire hexagonal dependencies.
	searcherImpl := searcher.NewSearcher()
	querySearcherImpl := querysearcher.NewQuerySearcher()
	downloaderImpl := downloader.NewDownloader(downloader.WithAudioBitrate(quality))
	orch := service.NewOrchestrator(searcherImpl, downloaderImpl)

	// Optional: wire Spotify adapter if configured.
	var spotifySearcher ports.Searcher
	cfgPath := spotify.ConfigPath()
	cfg, err := spotify.LoadConfig(cfgPath)
	if err != nil {
		log.Printf("warning: failed to load Spotify config: %v", err)
	} else if cfg != nil && cfg.Spotify.ClientID != "" && cfg.Spotify.ClientSecret != "" {
		spotifySearcher, err = spotify.NewSpotifySearcher(cfg.Spotify.ClientID, cfg.Spotify.ClientSecret, searcherImpl)
		if err != nil {
			log.Printf("warning: failed to create Spotify searcher: %v", err)
		} else {
			log.Println("Spotify adapter configured")
		}
	}

	// Start the Bubble Tea TUI program.
	m := tui.NewModel(orch, searcherImpl, spotifySearcher, querySearcherImpl, outputDir, quality)
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
