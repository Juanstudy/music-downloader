package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewOutput_TildeExpansion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	o, err := NewOutput("~/sub/dir")
	if err != nil {
		t.Fatalf("NewOutput() returned error: %v", err)
	}
	expected := filepath.Join(home, "sub", "dir")
	if o.FullPath() != expected {
		t.Errorf("FullPath() = %q, want %q", o.FullPath(), expected)
	}
}

func TestNewOutput_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	o, err := NewOutput(dir)
	if err != nil {
		t.Fatalf("NewOutput() returned error: %v", err)
	}
	if o.FullPath() != dir {
		t.Errorf("FullPath() = %q, want %q", o.FullPath(), dir)
	}
}

func TestNewOutput_RelativePath(t *testing.T) {
	// Change to TempDir so relative path resolves there
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	o, err := NewOutput("relative/path")
	if err != nil {
		t.Fatalf("NewOutput() returned error: %v", err)
	}
	expected := filepath.Join(dir, "relative", "path")
	if o.FullPath() != expected {
		t.Errorf("FullPath() = %q, want %q", o.FullPath(), expected)
	}
}

func TestEnsure_CreatesDirectory(t *testing.T) {
	base := filepath.Join(t.TempDir(), "sub", "dir")
	o, err := NewOutput(base)
	if err != nil {
		t.Fatalf("NewOutput() returned error: %v", err)
	}

	if err := o.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure() returned error: %v", err)
	}

	if _, err := os.Stat(base); os.IsNotExist(err) {
		t.Errorf("Ensure() did not create directory %q", base)
	}
}

func TestEnsure_Idempotent(t *testing.T) {
	base := filepath.Join(t.TempDir(), "exists")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}

	o, err := NewOutput(base)
	if err != nil {
		t.Fatalf("NewOutput() returned error: %v", err)
	}

	// Second call to Ensure on an existing directory should not error
	if err := o.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure() on existing dir returned error: %v", err)
	}
}

func TestFullPath_ReturnsAbsolute(t *testing.T) {
	o, err := NewOutput(t.TempDir())
	if err != nil {
		t.Fatalf("NewOutput() returned error: %v", err)
	}
	path := o.FullPath()
	if !strings.HasPrefix(path, "/") {
		t.Errorf("FullPath() = %q, want absolute path (starts with /)", path)
	}
}
