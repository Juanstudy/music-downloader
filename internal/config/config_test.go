package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// ----- AQ-001: exactly three quality levels, 320k default -----

func TestValidQualities(t *testing.T) {
	got := ValidQualities()
	want := []string{"128k", "192k", "320k"}
	if len(got) != len(want) {
		t.Fatalf("ValidQualities() length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ValidQualities()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestIsValidQuality_RejectsNonLevels(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"128k is valid", "128k", true},
		{"192k is valid", "192k", true},
		{"320k is valid", "320k", true},
		{"64k rejected", "64k", false},
		{"best rejected", "best", false},
		{"lossless rejected", "lossless", false},
		{"empty rejected", "", false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidQuality(tt.in); got != tt.want {
				t.Errorf("IsValidQuality(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestDefaultQuality(t *testing.T) {
	if DefaultQuality != "320k" {
		t.Errorf("DefaultQuality = %q, want %q", DefaultQuality, "320k")
	}
	if DefaultQuality != Quality320 {
		t.Errorf("DefaultQuality = %q, want Quality320 (%q)", DefaultQuality, Quality320)
	}
}

// ----- AQ-002: XDG-aware config path -----

func TestConfigPath_XDGSet(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")

	got := ConfigPath()
	want := filepath.Join("/custom/config", "music-dl", "config.toml")
	if got != want {
		t.Errorf("ConfigPath() = %q, want %q", got, want)
	}
}

func TestConfigPath_XDGUnset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() error: %v", err)
	}

	got := ConfigPath()
	want := filepath.Join(home, ".config", "music-dl", "config.toml")
	if got != want {
		t.Errorf("ConfigPath() = %q, want %q", got, want)
	}
}

// ----- AQ-003: missing/invalid quality falls back to 320k -----

func TestLoadConfig_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig on missing file returned error: %v", err)
	}
	if cfg.Quality.Value != DefaultQuality {
		t.Errorf("Quality.Value = %q, want default %q", cfg.Quality.Value, DefaultQuality)
	}
}

func TestLoadConfig_MissingQualitySection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := "[spotify]\nclient_id = \"abc123\"\nclient_secret = \"secret456\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.Quality.Value != DefaultQuality {
		t.Errorf("Quality.Value = %q, want default %q", cfg.Quality.Value, DefaultQuality)
	}
	if cfg.Spotify.ClientID != "abc123" {
		t.Errorf("Spotify.ClientID = %q, want %q (section must still decode)", cfg.Spotify.ClientID, "abc123")
	}
}

func TestLoadConfig_InvalidValueFallsBack(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"999k", "999k"},
		{"flac", "flac"},
		{"best", "best"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			content := "[quality]\nvalue = " + tomlQuote(tt.value) + "\n"
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}

			cfg, err := LoadConfig(path)
			if err != nil {
				t.Fatalf("LoadConfig(%q) returned error, want nil (fallback): %v", tt.value, err)
			}
			if cfg.Quality.Value != DefaultQuality {
				t.Errorf("Quality.Value = %q, want default %q", cfg.Quality.Value, DefaultQuality)
			}
		})
	}
}

func TestLoadConfig_MalformedTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("this is not toml {{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig on malformed TOML returned nil error")
	}
	if cfg.Quality.Value != "" {
		t.Errorf("expected zero Config on error, got Quality.Value %q", cfg.Quality.Value)
	}
}

// ----- AQ-004: saving quality preserves the spotify section -----

// spotifySeed encodes a config with only the [spotify] section so the round-trip
// comparison is against canonically-encoded bytes (design Open Decisions #5).
func spotifySeed(t *testing.T, clientID, clientSecret string) []byte {
	t.Helper()
	var seed struct {
		Spotify struct {
			ClientID     string `toml:"client_id"`
			ClientSecret string `toml:"client_secret"`
		} `toml:"spotify"`
	}
	seed.Spotify.ClientID = clientID
	seed.Spotify.ClientSecret = clientSecret

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(seed); err != nil {
		t.Fatalf("seeding spotify config: %v", err)
	}
	return buf.Bytes()
}

// spotifyBlock returns the canonical [spotify] section of a file, from the
// section header to EOF, for byte-identical comparison.
func spotifyBlock(t *testing.T, data []byte) string {
	t.Helper()
	idx := strings.Index(string(data), "[spotify]")
	if idx < 0 {
		t.Fatalf("file does not contain a [spotify] section:\n%s", data)
	}
	return string(data[idx:])
}

func TestSaveConfig_RoundTripPreservesSpotify(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	seed := spotifySeed(t, "abc123", "secret456")
	if err := os.WriteFile(path, seed, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SaveConfig(path, Config{Quality: Quality{Value: "192k"}}); err != nil {
		t.Fatalf("SaveConfig returned error: %v", err)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}

	// [quality] section must carry the new value.
	var decoded Config
	if _, err := toml.Decode(string(saved), &decoded); err != nil {
		t.Fatalf("decoding saved file: %v", err)
	}
	if decoded.Quality.Value != "192k" {
		t.Errorf("saved Quality.Value = %q, want %q", decoded.Quality.Value, "192k")
	}

	// [spotify] section must be byte-identical to the seeded section.
	if got, want := spotifyBlock(t, saved), spotifyBlock(t, seed); got != want {
		t.Errorf("[spotify] section changed after save:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSaveConfig_FirstSaveCreatesDirAndFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does", "not", "exist")
	path := filepath.Join(dir, "config.toml")

	if err := SaveConfig(path, Config{Quality: Quality{Value: "128k"}}); err != nil {
		t.Fatalf("SaveConfig returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	var decoded Config
	if _, err := toml.Decode(string(data), &decoded); err != nil {
		t.Fatalf("decoding first-save file: %v", err)
	}
	if decoded.Quality.Value != "128k" {
		t.Errorf("first-save Quality.Value = %q, want %q", decoded.Quality.Value, "128k")
	}
}

func TestSaveConfig_MalformedExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	garbage := []byte("this is not toml {{{")
	if err := os.WriteFile(path, garbage, 0o644); err != nil {
		t.Fatal(err)
	}

	err := SaveConfig(path, Config{Quality: Quality{Value: "192k"}})
	if err == nil {
		t.Fatal("SaveConfig on malformed existing file returned nil error")
	}

	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("reading file after failed save: %v", readErr)
	}
	if !bytes.Equal(after, garbage) {
		t.Errorf("malformed file was clobbered by failed save:\n--- got ---\n%s\n--- want ---\n%s", after, garbage)
	}
}

// tomlQuote renders a Go string as a TOML basic string literal.
func tomlQuote(s string) string {
	return `"` + s + `"`
}
