// Package config owns the music-dl config file path, the [quality] section,
// and a safe load-merge-save writer so the TUI can read and persist the audio
// quality without touching the Spotify adapter's config handling.
package config

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Quality levels. Exactly three; DefaultQuality is the fallback everywhere.
const (
	Quality128     = "128k"
	Quality192     = "192k"
	Quality320     = "320k"
	DefaultQuality = Quality320
)

// Quality holds the [quality] TOML section.
type Quality struct {
	Value string `toml:"value"`
}

// Config mirrors the config file: [quality] plus the existing [spotify] section
// (so a save never drops Spotify credentials).
type Config struct {
	Quality Quality `toml:"quality"`
	Spotify struct {
		ClientID     string `toml:"client_id"`
		ClientSecret string `toml:"client_secret"`
	} `toml:"spotify"`
}

// ConfigPath returns $XDG_CONFIG_HOME/music-dl/config.toml, falling back to
// ~/.config/music-dl/config.toml. Same file the Spotify adapter reads (canonical
// copy of the spotify package logic — AQ-019 keeps spotify/config.go untouched).
func ConfigPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "~"
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "music-dl", "config.toml")
}

// ValidQualities returns the three levels in display order: 128k, 192k, 320k.
func ValidQualities() []string {
	return []string{Quality128, Quality192, Quality320}
}

// IsValidQuality reports whether q is one of the three valid levels.
func IsValidQuality(q string) bool {
	return q == Quality128 || q == Quality192 || q == Quality320
}

// LoadConfig returns the effective configuration. Missing file, missing
// [quality] section, and invalid values are normalized to DefaultQuality with
// a warning (invalid only) and no error. Malformed TOML returns a non-nil error
// and a zero Config (caller falls back to DefaultQuality).
func LoadConfig(path string) (Config, error) {
	var cfg Config
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg.Quality.Value = DefaultQuality
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, err
	}

	switch {
	case cfg.Quality.Value == "":
		cfg.Quality.Value = DefaultQuality
	case !IsValidQuality(cfg.Quality.Value):
		log.Printf("warning: invalid audio quality %q, using %s", cfg.Quality.Value, DefaultQuality)
		cfg.Quality.Value = DefaultQuality
	}
	return cfg, nil
}

// SaveConfig persists cfg with load-merge-save: the existing file is decoded
// first so the [spotify] section survives, then the [quality] section is
// replaced by cfg.Quality. Creates the music-dl directory and file on first
// save. If the existing file is malformed TOML, SaveConfig returns an error
// (the broken file is never clobbered).
func SaveConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	var merged Config
	if _, err := os.Stat(path); err == nil {
		if _, err := toml.DecodeFile(path, &merged); err != nil {
			return fmt.Errorf("save config: existing config is malformed: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat config file: %w", err)
	}

	// Overwrite only the [quality] section; cfg.Spotify is ignored by design
	// (this slice only writes quality).
	merged.Quality = cfg.Quality

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(merged); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
