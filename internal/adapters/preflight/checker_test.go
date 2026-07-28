package preflight

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewChecker(t *testing.T) {
	c := NewChecker("yt-dlp", "ffmpeg")
	if c == nil {
		t.Fatal("NewChecker() returned nil")
	}
}

func TestCheck_AllBinariesPresent(t *testing.T) {
	// Create fake binaries in TempDir and set PATH
	dir := t.TempDir()
	for _, name := range []string{"yt-dlp", "ffmpeg"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	oldPATH := os.Getenv("PATH")
	t.Setenv("PATH", dir)
	defer os.Setenv("PATH", oldPATH)

	c := NewChecker("yt-dlp", "ffmpeg")
	errs := c.Check(context.Background())
	if len(errs) != 0 {
		t.Errorf("Check() returned %d errors, want 0: %v", len(errs), errs)
	}
}

func TestCheck_MissingBinary(t *testing.T) {
	// Set PATH to empty TempDir — no binaries present
	dir := t.TempDir()
	t.Setenv("PATH", dir)

	c := NewChecker("yt-dlp")
	errs := c.Check(context.Background())
	if len(errs) != 1 {
		t.Fatalf("Check() returned %d errors, want 1", len(errs))
	}
	if errs[0].Binary != "yt-dlp" {
		t.Errorf("Check()[0].Binary = %q, want %q", errs[0].Binary, "yt-dlp")
	}
	if errs[0].Err == nil {
		t.Error("Check()[0].Err is nil, want non-nil error")
	}
}

func TestCheck_CollectsAllMissing(t *testing.T) {
	// Set PATH to empty TempDir
	dir := t.TempDir()
	t.Setenv("PATH", dir)

	c := NewChecker("yt-dlp", "ffmpeg")
	errs := c.Check(context.Background())
	if len(errs) != 2 {
		t.Fatalf("Check() returned %d errors, want 2", len(errs))
	}

	// Collect binary names for order-independent check
	binaries := make(map[string]bool)
	for _, e := range errs {
		binaries[e.Binary] = true
	}
	if !binaries["yt-dlp"] {
		t.Error("Check() missing error for yt-dlp")
	}
	if !binaries["ffmpeg"] {
		t.Error("Check() missing error for ffmpeg")
	}
}

func TestCheck_NoBinariesConfigured(t *testing.T) {
	c := NewChecker()
	errs := c.Check(context.Background())
	if len(errs) != 0 {
		t.Errorf("Check() with empty binary list returned %d errors, want 0", len(errs))
	}
}
