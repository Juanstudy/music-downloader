package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// Output manages the download output directory.
type Output struct {
	basePath string // absolute, tilde-expanded
}

// NewOutput creates an Output with the given base path.
// Tilde (~) prefixes are expanded to the user's home directory.
// Relative paths are resolved to absolute paths.
func NewOutput(basePath string) (*Output, error) {
	if strings.HasPrefix(basePath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		basePath = filepath.Join(home, basePath[2:])
	}
	abs, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}
	return &Output{basePath: abs}, nil
}

// Ensure creates the output directory tree. It is idempotent.
func (o *Output) Ensure(ctx context.Context) error {
	return os.MkdirAll(o.basePath, 0755)
}

// FullPath returns the absolute path to the output directory.
func (o *Output) FullPath() string {
	return o.basePath
}
