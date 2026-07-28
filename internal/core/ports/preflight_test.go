package ports

import (
	"errors"
	"testing"
)

func TestPreflightError_ZeroValue(t *testing.T) {
	var pe PreflightError

	if pe.Binary != "" {
		t.Errorf("PreflightError.Binary zero value = %q, want empty", pe.Binary)
	}
	if pe.Err != nil {
		t.Errorf("PreflightError.Err zero value = %v, want nil", pe.Err)
	}
}

func TestPreflightError_StructLiteral(t *testing.T) {
	pe := PreflightError{
		Binary: "yt-dlp",
		Err:    errors.New("not found in PATH"),
	}

	if pe.Binary != "yt-dlp" {
		t.Errorf("PreflightError.Binary = %q, want %q", pe.Binary, "yt-dlp")
	}
	if pe.Err == nil || pe.Err.Error() != "not found in PATH" {
		t.Errorf("PreflightError.Err = %v, want error with message 'not found in PATH'", pe.Err)
	}
}
