package ports

import (
	"testing"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
)

func TestDownloadResult_ZeroValue(t *testing.T) {
	var dr DownloadResult

	if dr.Media != (domain.Media{}) {
		t.Errorf("DownloadResult.Media zero value = %v, want zero Media", dr.Media)
	}
	if dr.OutputPath != "" {
		t.Errorf("DownloadResult.OutputPath zero value = %q, want empty", dr.OutputPath)
	}
}

func TestDownloadResult_StructLiteral(t *testing.T) {
	dr := DownloadResult{
		Media: domain.Media{
			URL:    "https://youtube.com/watch?v=test",
			Title:  "Test Track",
			Artist: "Test Artist",
			Status: domain.StatusDone,
		},
		OutputPath: "/tmp/test.mp3",
	}

	if dr.Media.Title != "Test Track" {
		t.Errorf("DownloadResult.Media.Title = %q, want %q", dr.Media.Title, "Test Track")
	}
	if dr.OutputPath != "/tmp/test.mp3" {
		t.Errorf("DownloadResult.OutputPath = %q, want %q", dr.OutputPath, "/tmp/test.mp3")
	}
}
