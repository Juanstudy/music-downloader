package spotify

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ConfigPath returns the default config file path.
// It respects $XDG_CONFIG_HOME if set, falling back to ~/.config.
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

// Config holds optional provider credentials loaded from a TOML file.
type Config struct {
	Spotify struct {
		ClientID     string `toml:"client_id"`
		ClientSecret string `toml:"client_secret"`
	} `toml:"spotify"`
}

// LoadConfig reads and parses a TOML config file.
// Returns (nil, nil) if the file does not exist (graceful degradation).
// Returns an error if the file exists but contains malformed TOML.
func LoadConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}


