package spotify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_Valid(t *testing.T) {
	tomlContent := `[spotify]
client_id = "abc123"
client_secret = "secret456"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(tomlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Spotify.ClientID != "abc123" {
		t.Errorf("expected ClientID 'abc123', got %q", cfg.Spotify.ClientID)
	}
	if cfg.Spotify.ClientSecret != "secret456" {
		t.Errorf("expected ClientSecret 'secret456', got %q", cfg.Spotify.ClientSecret)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config for missing file, got %+v", cfg)
	}
}

func TestLoadConfig_Malformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(path, []byte("this is not toml {{{"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for malformed TOML")
	}
	if cfg != nil {
		t.Fatalf("expected nil config on error, got %+v", cfg)
	}
}

func TestConfigPath_Default(t *testing.T) {
	// Ensure XDG_CONFIG_HOME is unset for this test
	os.Unsetenv("XDG_CONFIG_HOME")

	path := ConfigPath()
	if !strings.Contains(path, ".config/music-dl/config.toml") {
		t.Errorf("expected path to contain '.config/music-dl/config.toml', got %q", path)
	}
	if !strings.HasPrefix(path, "/") {
		t.Errorf("expected absolute path, got %q", path)
	}
}

func TestConfigPath_XDG(t *testing.T) {
	xdgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgHome)

	path := ConfigPath()
	expected := filepath.Join(xdgHome, "music-dl", "config.toml")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}
