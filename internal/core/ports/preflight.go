package ports

import "context"

// PreflightError describes a missing external binary.
type PreflightError struct {
	Binary string
	Err    error
}

// PreflightChecker validates that required external binaries are available.
type PreflightChecker interface {
	Check(ctx context.Context) []PreflightError
}
