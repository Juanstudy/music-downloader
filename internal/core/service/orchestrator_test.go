package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
)

// ----- manual mocks -----

type mockSearcher struct {
	searchFunc func(ctx context.Context, url string) (ports.SearchResult, error)
}

func (m *mockSearcher) Search(ctx context.Context, url string) (ports.SearchResult, error) {
	// Check context cancellation before delegating.
	if ctx.Err() != nil {
		return ports.SearchResult{}, ctx.Err()
	}
	return m.searchFunc(ctx, url)
}

type mockDownloader struct {
	downloadFunc func(ctx context.Context, media domain.Media, outputDir string) (ports.DownloadResult, error)
}

func (m *mockDownloader) Download(ctx context.Context, media domain.Media, outputDir string) (ports.DownloadResult, error) {
	return m.downloadFunc(ctx, media, outputDir)
}

// ----- test defaults -----

var defaultTrack = domain.Media{
	URL:    "https://youtube.com/watch?v=abc123",
	Title:  "Test Song",
	Artist: "Test Artist",
	Status: domain.StatusPending,
}

func newMockSearcher(result ports.SearchResult, err error) *mockSearcher {
	return &mockSearcher{
		searchFunc: func(_ context.Context, _ string) (ports.SearchResult, error) {
			return result, err
		},
	}
}

func newMockDownloader(result ports.DownloadResult, err error) *mockDownloader {
	return &mockDownloader{
		downloadFunc: func(_ context.Context, _ domain.Media, _ string) (ports.DownloadResult, error) {
			return result, err
		},
	}
}

// ----- NewOrchestrator -----

func TestNewOrchestrator(t *testing.T) {
	s := &mockSearcher{}
	d := &mockDownloader{}

	o := NewOrchestrator(s, d)
	if o == nil {
		t.Fatal("NewOrchestrator returned nil")
	}
}

// ----- ResolveTrack -----

func TestResolveTrack_Success(t *testing.T) {
	tracks := []domain.Media{
		{URL: "https://youtube.com/watch?v=abc", Title: "Track 1"},
		{URL: "https://youtube.com/watch?v=def", Title: "Track 2"},
	}
	result := ports.SearchResult{Tracks: tracks, Source: "youtube"}
	searcher := newMockSearcher(result, nil)
	orch := NewOrchestrator(searcher, &mockDownloader{})

	got, err := orch.ResolveTrack(context.Background(), "https://youtube.com/playlist?list=abc")
	if err != nil {
		t.Fatalf("ResolveTrack returned unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d tracks, want 2", len(got))
	}
	for i, tr := range got {
		if tr.Status != domain.StatusResolved {
			t.Errorf("track[%d].Status = %v, want StatusResolved", i, tr.Status)
		}
		if tr.Title != tracks[i].Title {
			t.Errorf("track[%d].Title = %q, want %q", i, tr.Title, tracks[i].Title)
		}
	}
}

func TestResolveTrack_EmptyURL(t *testing.T) {
	searcher := newMockSearcher(ports.SearchResult{}, nil)
	orch := NewOrchestrator(searcher, &mockDownloader{})

	got, err := orch.ResolveTrack(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}

	var domErr domain.Error
	if !errors.As(err, &domErr) {
		t.Fatalf("error type = %T, want domain.Error", err)
	}
	if domErr.Code != domain.ErrorInvalidURL {
		t.Errorf("ErrorCode = %v, want ErrorInvalidURL", domErr.Code)
	}
	if got != nil {
		t.Fatalf("expected nil tracks, got %d", len(got))
	}
}

func TestResolveTrack_EmptyURLWhitespace(t *testing.T) {
	searcher := newMockSearcher(ports.SearchResult{}, nil)
	orch := NewOrchestrator(searcher, &mockDownloader{})

	got, err := orch.ResolveTrack(context.Background(), "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only URL, got nil")
	}

	var domErr domain.Error
	if !errors.As(err, &domErr) {
		t.Fatalf("error type = %T, want domain.Error", err)
	}
	if domErr.Code != domain.ErrorInvalidURL {
		t.Errorf("ErrorCode = %v, want ErrorInvalidURL", domErr.Code)
	}
	if got != nil {
		t.Fatalf("expected nil tracks, got %d", len(got))
	}
}

func TestResolveTrack_SearcherError(t *testing.T) {
	wantErr := errors.New("searcher timeout")
	searcher := newMockSearcher(ports.SearchResult{}, wantErr)
	orch := NewOrchestrator(searcher, &mockDownloader{})

	got, err := orch.ResolveTrack(context.Background(), "https://youtube.com/watch?v=abc")
	if err == nil {
		t.Fatal("expected error from searcher, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Fatalf("expected nil tracks on error, got %d", len(got))
	}
}

func TestResolveTrack_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// searcher that would succeed if called, but context is cancelled
	searcher := &mockSearcher{
		searchFunc: func(ctx context.Context, _ string) (ports.SearchResult, error) {
			// Context is checked by the mock wrapper, but guard here too
			if ctx.Err() != nil {
				return ports.SearchResult{}, ctx.Err()
			}
			return ports.SearchResult{Tracks: []domain.Media{defaultTrack}}, nil
		},
	}
	orch := NewOrchestrator(searcher, &mockDownloader{})

	_, err := orch.ResolveTrack(ctx, "https://youtube.com/watch?v=abc")
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// ----- DownloadTrack -----

func TestDownloadTrack_Success(t *testing.T) {
	media := defaultTrack
	result := ports.DownloadResult{
		Media:      media,
		OutputPath: "/tmp/music/Test Artist - Test Song.mp3",
	}
	downloader := newMockDownloader(result, nil)
	orch := NewOrchestrator(&mockSearcher{}, downloader)

	got, err := orch.DownloadTrack(context.Background(), media, "/tmp/music")
	if err != nil {
		t.Fatalf("DownloadTrack returned unexpected error: %v", err)
	}
	if got.Status != domain.StatusDone {
		t.Errorf("Status = %v, want StatusDone", got.Status)
	}
	if got.OutputPath != result.OutputPath {
		t.Errorf("OutputPath = %q, want %q", got.OutputPath, result.OutputPath)
	}
}

func TestDownloadTrack_OutputPathSet(t *testing.T) {
	media := defaultTrack
	expectedPath := "/tmp/music/Test Artist - Test Song.mp3"
	result := ports.DownloadResult{
		Media:      media,
		OutputPath: expectedPath,
	}
	downloader := newMockDownloader(result, nil)
	orch := NewOrchestrator(&mockSearcher{}, downloader)

	got, err := orch.DownloadTrack(context.Background(), media, "/tmp/music")
	if err != nil {
		t.Fatalf("DownloadTrack returned unexpected error: %v", err)
	}
	if got.OutputPath != expectedPath {
		t.Errorf("OutputPath = %q, want %q", got.OutputPath, expectedPath)
	}
}

func TestDownloadTrack_DownloaderError(t *testing.T) {
	media := defaultTrack
	wantErr := errors.New("disk full")
	downloader := newMockDownloader(ports.DownloadResult{}, wantErr)
	orch := NewOrchestrator(&mockSearcher{}, downloader)

	got, err := orch.DownloadTrack(context.Background(), media, "/tmp/music")
	if err == nil {
		t.Fatal("expected error from downloader, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
	if got.Status != domain.StatusFailed {
		t.Errorf("Status = %v, want StatusFailed", got.Status)
	}
	if got.Error == "" {
		t.Error("Media.Error is empty, expected error message")
	}
}
