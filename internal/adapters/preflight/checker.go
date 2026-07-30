package preflight

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/Juanstudy/music-downloader/internal/core/ports"
)

// Checker validates that required external binaries are available on $PATH.
type Checker struct {
	binaries []string
}

// NewChecker creates a Checker with the given binary names to verify.
func NewChecker(binaries ...string) *Checker {
	return &Checker{binaries: binaries}
}

// Check runs exec.LookPath for each configured binary and collects all
// missing ones. It is not fail-fast — all binaries are checked.
func (c *Checker) Check(ctx context.Context) []ports.PreflightError {
	var errs []ports.PreflightError
	for _, binary := range c.binaries {
		_, err := exec.LookPath(binary)
		if err != nil {
			errs = append(errs, ports.PreflightError{
				Binary: binary,
				Err:    fmt.Errorf("%s not found on $PATH: %w", binary, err),
			})
		}
	}
	return errs
}
