package tui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/Juanstudy/music-downloader/internal/config"
	"github.com/Juanstudy/music-downloader/internal/core/domain"
	"github.com/Juanstudy/music-downloader/internal/core/ports"
	"github.com/Juanstudy/music-downloader/internal/core/service"
	"github.com/charmbracelet/bubbles/textinput"

	tea "github.com/charmbracelet/bubbletea"
)

var errTest = errors.New("test error")

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func modelWithTracks(t *testing.T) Model {
	t.Helper()
	m := Model{
		Screen: ScreenPlaylist,
		Ready:  true,
		Height: 30,
		tracks: sampleTracks(),
		cursor: 0,
		scroll: 0,
	}
	return m
}

func sampleTracks() []domain.Media {
	return []domain.Media{
		{URL: "https://youtube.com/watch?v=1", Title: "Track One", Artist: "Artist A", Source: "youtube"},
		{URL: "https://youtube.com/watch?v=2", Title: "Track Two", Artist: "Artist B", Source: "youtube"},
		{URL: "https://youtube.com/watch?v=3", Title: "Track Three", Artist: "Artist C", Source: "youtube"},
	}
}

// stubSearcher implements ports.Searcher for testing.
type stubSearcher struct {
	result ports.SearchResult
	err    error
}

func (s *stubSearcher) Search(ctx context.Context, url string) (ports.SearchResult, error) {
	return s.result, s.err
}

// ---------------------------------------------------------------------------
// 1. Initial state
// ---------------------------------------------------------------------------

func TestInitialScreenIsInput(t *testing.T) {
	m := Model{Ready: true}
	if m.Screen != ScreenInput {
		t.Errorf("expected ScreenInput (%d), got %d", ScreenInput, m.Screen)
	}
}

// ---------------------------------------------------------------------------
// 2. WindowSizeMsg sets ready
// ---------------------------------------------------------------------------

func TestWindowSizeMsgSetsReady(t *testing.T) {
	m := Model{}
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	updated := m2.(Model)

	if !updated.Ready {
		t.Error("expected Ready=true after WindowSizeMsg")
	}
	if updated.Width != 80 {
		t.Errorf("expected Width=80, got %d", updated.Width)
	}
	if updated.Height != 30 {
		t.Errorf("expected Height=30, got %d", updated.Height)
	}
}

// ---------------------------------------------------------------------------
// 3. Ctrl+C on input quits
// ---------------------------------------------------------------------------

func TestCtrlCOnInputQuits(t *testing.T) {
	m := Model{Screen: ScreenInput, Ready: true}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd == nil {
		t.Fatal("expected non-nil quit cmd")
	}
}

// ---------------------------------------------------------------------------
// 4. Enter with empty URL shows error
// ---------------------------------------------------------------------------

func TestEnterEmptyURLShowsError(t *testing.T) {
	m := Model{Screen: ScreenInput, Ready: true, Input: newInput()}
	m.Input.SetValue("")

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	if updated.Screen != ScreenInput {
		t.Errorf("expected stay on ScreenInput, got %d", updated.Screen)
	}
	if updated.inputErr == "" {
		t.Error("expected non-empty inputErr for empty URL")
	}
}

func newInput() textinput.Model {
	ti := textinput.New()
	ti.Focus()
	return ti
}

func newFilterInput() textinput.Model {
	fi := textinput.New()
	fi.Focus()
	return fi
}

// saveConfigSpy records invocations of the saveConfig seam for no-write assertions.
type saveConfigSpy struct {
	called int
	path   string
	cfg    config.Config
}

func (s *saveConfigSpy) save(path string, cfg config.Config) error {
	s.called++
	s.path = path
	s.cfg = cfg
	return nil
}

// recordingDownloader implements ports.Downloader and records the audio bitrate
// applied via SetAudioBitrate so tests can assert mid-session quality changes.
type recordingDownloader struct {
	bitrate string
}

func (d *recordingDownloader) Download(ctx context.Context, media domain.Media, outputDir string) (ports.DownloadResult, error) {
	return ports.DownloadResult{Media: media}, nil
}

func (d *recordingDownloader) SetAudioBitrate(q string) {
	d.bitrate = q
}

// ---------------------------------------------------------------------------
// 5. Enter with valid URL transitions to resolving
// ---------------------------------------------------------------------------

func TestEnterValidURLTransitionsToResolving(t *testing.T) {
	m := Model{Screen: ScreenInput, Ready: true}
	m.Input.SetValue("https://youtube.com/playlist?list=xyz")

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	if updated.Screen != ScreenResolving {
		t.Errorf("expected ScreenResolving (%d), got %d", ScreenResolving, updated.Screen)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for resolve")
	}
}

// ---------------------------------------------------------------------------
// 6. Resolve success with single track goes to downloading
// ---------------------------------------------------------------------------

func TestResolveSingleTrackGoesToDownloading(t *testing.T) {
	m := Model{Screen: ScreenResolving, Ready: true, tracks: nil}

	msg := resolveFinishedMsg{
		tracks: []domain.Media{
			{URL: "https://youtube.com/watch?v=1", Title: "Single Track", Artist: "Artist"},
		},
		err: nil,
	}

	m2, cmd := m.Update(msg)
	updated := m2.(Model)

	if updated.Screen != ScreenDownloading {
		t.Errorf("expected ScreenDownloading (%d), got %d", ScreenDownloading, updated.Screen)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to start download")
	}
	if len(updated.tracks) != 1 {
		t.Errorf("expected 1 track, got %d", len(updated.tracks))
	}
}

// ---------------------------------------------------------------------------
// 7. Resolve success with multiple tracks goes to playlist
// ---------------------------------------------------------------------------

func TestResolveMultipleTracksGoesToPlaylist(t *testing.T) {
	m := Model{Screen: ScreenResolving, Ready: true, tracks: nil}

	msg := resolveFinishedMsg{
		tracks: sampleTracks(),
		err:    nil,
	}

	m2, cmd := m.Update(msg)
	updated := m2.(Model)

	if updated.Screen != ScreenPlaylist {
		t.Errorf("expected ScreenPlaylist (%d), got %d", ScreenPlaylist, updated.Screen)
	}
	if cmd != nil {
		t.Error("expected nil cmd for playlist")
	}
	if len(updated.tracks) != 3 {
		t.Errorf("expected 3 tracks, got %d", len(updated.tracks))
	}
}

// ---------------------------------------------------------------------------
// 8. Resolve error goes back to input
// ---------------------------------------------------------------------------

func TestResolveErrorGoesBackToInput(t *testing.T) {
	m := Model{Screen: ScreenResolving, Ready: true, tracks: nil, Input: newInput()}

	msg := resolveFinishedMsg{
		tracks: nil,
		err:    errTest,
	}

	m2, _ := m.Update(msg)
	updated := m2.(Model)

	if updated.Screen != ScreenInput {
		t.Errorf("expected ScreenInput (%d), got %d", ScreenInput, updated.Screen)
	}
	if updated.resolveErr == "" {
		t.Error("expected resolveErr to be set")
	}
}

// ---------------------------------------------------------------------------
// 9. Resolve with partial results
// ---------------------------------------------------------------------------

func TestResolvePartialResultsShowsTracksWithWarning(t *testing.T) {
	m := Model{Screen: ScreenResolving, Ready: true, tracks: nil, Input: newInput()}

	msg := resolveFinishedMsg{
		tracks: sampleTracks(),
		err:    errTest,
	}

	m2, _ := m.Update(msg)
	updated := m2.(Model)

	if updated.Screen != ScreenPlaylist {
		t.Errorf("expected ScreenPlaylist with partial results, got %d", updated.Screen)
	}
	if updated.resolveErr == "" {
		t.Error("expected resolveErr warning for partial results")
	}
	if len(updated.tracks) != 3 {
		t.Errorf("expected 3 tracks from partial results, got %d", len(updated.tracks))
	}
}

// ---------------------------------------------------------------------------
// 10. Enter selects track
// ---------------------------------------------------------------------------

func TestPlaylist_EnterSelectedStartsDownload(t *testing.T) {
	m := modelWithTracks(t)
	m.tracks[0].Status = domain.StatusDone

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	if updated.Screen != ScreenDownloading {
		t.Errorf("expected ScreenDownloading (%d), got %d", ScreenDownloading, updated.Screen)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to start download")
	}
}

// ---------------------------------------------------------------------------
// 11. Space toggles selection
// ---------------------------------------------------------------------------

func TestPlaylist_SpaceTogglesSelection(t *testing.T) {
	m := modelWithTracks(t)
	m.tracks[0].Status = domain.StatusPending

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})

	if m.tracks[0].Status != domain.StatusDone {
		t.Error("expected first track to be toggled to StatusDone")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})

	if m.tracks[0].Status != domain.StatusPending {
		t.Error("expected first track to be toggled back to StatusPending")
	}
}

// ---------------------------------------------------------------------------
// 12. Select all with 'a'
// ---------------------------------------------------------------------------

func TestPlaylist_SelectAll(t *testing.T) {
	m := modelWithTracks(t)
	for i := range m.tracks {
		m.tracks[i].Status = domain.StatusPending
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})

	for i, track := range m.tracks {
		if track.Status != domain.StatusDone {
			t.Errorf("track[%d] should be StatusDone after select all, got %v", i, track.Status)
		}
	}
}

// ---------------------------------------------------------------------------
// 13. Deselect all with 'n'
// ---------------------------------------------------------------------------

func TestPlaylist_DeselectAll(t *testing.T) {
	m := modelWithTracks(t)
	for i := range m.tracks {
		m.tracks[i].Status = domain.StatusDone
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})

	for i, track := range m.tracks {
		if track.Status != domain.StatusPending {
			t.Errorf("track[%d] should be StatusPending after deselect all, got %v", i, track.Status)
		}
	}
}

// ---------------------------------------------------------------------------
// 14. Cursor navigation
// ---------------------------------------------------------------------------

func TestPlaylist_CursorDown(t *testing.T) {
	m := modelWithTracks(t)
	m.cursor = 0

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	updated := m2.(Model)

	if updated.cursor != 1 {
		t.Errorf("expected cursor=1 after down, got %d", updated.cursor)
	}
}

func TestPlaylist_CursorUp(t *testing.T) {
	m := modelWithTracks(t)
	m.cursor = 1

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	updated := m2.(Model)

	if updated.cursor != 0 {
		t.Errorf("expected cursor=0 after up, got %d", updated.cursor)
	}
}

func TestPlaylist_CursorStaysInBounds(t *testing.T) {
	m := modelWithTracks(t)
	m.cursor = 2 // at the last track

	// Press j (down) — should stay at last element
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	updated := m2.(Model)

	if updated.cursor != 2 {
		t.Errorf("expected cursor=2 (clamped), got %d", updated.cursor)
	}

	// Press k (up) twice from updated — should eventually stop at 0
	m3, _ := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	updated2 := m3.(Model)
	if updated2.cursor != 1 {
		t.Errorf("expected cursor=1 after one up, got %d", updated2.cursor)
	}

	m4, _ := updated2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	updated3 := m4.(Model)
	if updated3.cursor != 0 {
		t.Errorf("expected cursor=0 after two ups, got %d", updated3.cursor)
	}

	// Press k again — should stay at 0
	m5, _ := updated3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	updated4 := m5.(Model)
	if updated4.cursor != 0 {
		t.Errorf("expected cursor=0 (clamped), got %d", updated4.cursor)
	}
}

// ---------------------------------------------------------------------------
// 15. Esc on playlist goes back to input
// ---------------------------------------------------------------------------

func TestPlaylist_EscGoesBackToInput(t *testing.T) {
	m := modelWithTracks(t)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := m2.(Model)

	if updated.Screen != ScreenInput {
		t.Errorf("expected ScreenInput (%d), got %d", ScreenInput, updated.Screen)
	}
}

// ---------------------------------------------------------------------------
// 16. q on playlist quits
// ---------------------------------------------------------------------------

func TestPlaylist_QQuits(t *testing.T) {
	m := modelWithTracks(t)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	if cmd == nil {
		t.Error("expected non-nil quit cmd")
	}
}

// ---------------------------------------------------------------------------
// 17. Help toggle
// ---------------------------------------------------------------------------

func TestHelpToggle(t *testing.T) {
	m := Model{Screen: ScreenInput, Ready: true}
	m.showHelp = false

	// Press ? to toggle on
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !m2.(Model).showHelp {
		t.Error("expected showHelp=true after ?")
	}

	// Press ? again to toggle off
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if m3.(Model).showHelp {
		t.Error("expected showHelp=false after second ?")
	}
}

// ---------------------------------------------------------------------------
// 18. Esc on resolving goes back
// ---------------------------------------------------------------------------

func TestResolvingEscGoesBack(t *testing.T) {
	m := Model{Screen: ScreenResolving, Ready: true, Input: newInput()}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := m2.(Model)

	if updated.Screen != ScreenInput {
		t.Errorf("expected ScreenInput (%d), got %d", ScreenInput, updated.Screen)
	}
}

// ---------------------------------------------------------------------------
// 19. Track download success
// ---------------------------------------------------------------------------

func TestTrackDownloadSuccess(t *testing.T) {
	m := Model{Screen: ScreenDownloading, Ready: true, tracks: sampleTracks()}
	m.tracks[0].Status = domain.StatusDownloading
	m.downloadIdx = 0
	m.succeeded = 0

	msg := trackDownloadedMsg{
		index: 0,
		media: domain.Media{URL: "https://youtube.com/watch?v=1", Title: "Track One", OutputPath: "/tmp/music/Track One.mp3"},
		err:   nil,
	}

	m2, _ := m.Update(msg)
	updated := m2.(Model)

	if updated.tracks[0].Status != domain.StatusDone {
		t.Errorf("expected StatusDone, got %v", updated.tracks[0].Status)
	}
	if updated.tracks[0].OutputPath != "/tmp/music/Track One.mp3" {
		t.Errorf("expected OutputPath to be set, got %q", updated.tracks[0].OutputPath)
	}
	if updated.succeeded != 1 {
		t.Errorf("expected succeeded=1, got %d", updated.succeeded)
	}
}

// ---------------------------------------------------------------------------
// 20. Track download failure
// ---------------------------------------------------------------------------

func TestTrackDownloadFailure(t *testing.T) {
	m := Model{Screen: ScreenDownloading, Ready: true, tracks: sampleTracks()}
	m.tracks[0].Status = domain.StatusDownloading
	m.downloadIdx = 0

	msg := trackDownloadedMsg{
		index: 0,
		media: domain.Media{},
		err:   errTest,
	}

	m2, _ := m.Update(msg)
	updated := m2.(Model)

	if updated.tracks[0].Status != domain.StatusFailed {
		t.Errorf("expected StatusFailed, got %v", updated.tracks[0].Status)
	}
	if updated.failed != 1 {
		t.Errorf("expected failed=1, got %d", updated.failed)
	}
	if len(updated.failedTracks) != 1 {
		t.Errorf("expected 1 failedTrack, got %d", len(updated.failedTracks))
	}
}

// ---------------------------------------------------------------------------
// 21. Sequential downloads chain correctly
// ---------------------------------------------------------------------------

func TestSequentialDownloadChain(t *testing.T) {
	m := Model{Screen: ScreenDownloading, Ready: true, tracks: sampleTracks()}
	m.tracks[0].Status = domain.StatusDownloading
	m.tracks[1].Status = domain.StatusResolved
	m.tracks[2].Status = domain.StatusResolved
	m.downloadIdx = 0
	m.succeeded = 0

	// Complete track 0
	msg1 := trackDownloadedMsg{
		index: 0,
		media: domain.Media{URL: "https://youtube.com/watch?v=1", Title: "Track One", OutputPath: "/tmp/m1.mp3"},
		err:   nil,
	}

	m2, cmd := m.Update(msg1)
	updated := m2.(Model)

	if cmd == nil {
		t.Fatal("expected non-nil cmd for next download")
	}
	if updated.downloadIdx != 1 {
		t.Errorf("expected downloadIdx=1, got %d", updated.downloadIdx)
	}
	if updated.tracks[1].Status != domain.StatusDownloading {
		t.Errorf("expected track[1] to be StatusDownloading, got %v", updated.tracks[1].Status)
	}
	if updated.succeeded != 1 {
		t.Errorf("expected succeeded=1, got %d", updated.succeeded)
	}

	// Complete track 1
	msg2 := trackDownloadedMsg{
		index: 1,
		media: domain.Media{URL: "https://youtube.com/watch?v=2", Title: "Track Two", OutputPath: "/tmp/m2.mp3"},
		err:   nil,
	}

	m3, cmd2 := m2.Update(msg2)
	updated2 := m3.(Model)

	if cmd2 == nil {
		t.Fatal("expected non-nil cmd for next download")
	}
	if updated2.downloadIdx != 2 {
		t.Errorf("expected downloadIdx=2, got %d", updated2.downloadIdx)
	}

	// Complete track 2
	msg3 := trackDownloadedMsg{
		index: 2,
		media: domain.Media{URL: "https://youtube.com/watch?v=3", Title: "Track Three", OutputPath: "/tmp/m3.mp3"},
		err:   nil,
	}

	m4, cmd3 := m3.Update(msg3)
	updated3 := m4.(Model)

	if cmd3 != nil {
		t.Error("expected nil cmd — all tracks done")
	}
	if updated3.Screen != ScreenDone {
		t.Errorf("expected ScreenDone (%d), got %d", ScreenDone, updated3.Screen)
	}
	if updated3.succeeded != 3 {
		t.Errorf("expected succeeded=3, got %d", updated3.succeeded)
	}
}

// ---------------------------------------------------------------------------
// 22. Done screen r resets to input
// ---------------------------------------------------------------------------

func TestDoneReset(t *testing.T) {
	m := Model{
		Screen:    ScreenDone,
		Ready:     true,
		tracks:    sampleTracks(),
		succeeded: 3,
		Input:     newInput(),
	}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	updated := m2.(Model)

	if updated.Screen != ScreenInput {
		t.Errorf("expected ScreenInput (%d), got %d", ScreenInput, updated.Screen)
	}
	if updated.succeeded != 0 {
		t.Errorf("expected succeeded reset to 0, got %d", updated.succeeded)
	}
	if updated.tracks != nil {
		t.Error("expected tracks to be nil after reset")
	}
}

// ---------------------------------------------------------------------------
// Edge cases (4R review)
// ---------------------------------------------------------------------------

func TestResolveEmptyResult(t *testing.T) {
	m := Model{Screen: ScreenResolving, Ready: true, Input: newInput()}

	msg := resolveFinishedMsg{
		tracks: nil,
		err:    nil,
	}

	m2, _ := m.Update(msg)
	updated := m2.(Model)

	if updated.Screen != ScreenInput {
		t.Errorf("expected ScreenInput for empty result, got %d", updated.Screen)
	}
	if updated.resolveErr == "" {
		t.Error("expected resolveErr for empty result")
	}
}

func TestAllTracksFailDuringDownload(t *testing.T) {
	m := Model{Screen: ScreenDownloading, Ready: true, tracks: sampleTracks(), Input: newInput()}
	m.tracks[0].Status = domain.StatusDownloading
	m.tracks[1].Status = domain.StatusResolved
	m.tracks[2].Status = domain.StatusResolved
	m.downloadIdx = 0

	errMsg := "disk full"

	// Track 0 fails
	msg1 := trackDownloadedMsg{index: 0, media: domain.Media{}, err: errors.New(errMsg)}
	m2, cmd := m.Update(msg1)
	updated := m2.(Model)

	if updated.failed != 1 {
		t.Errorf("expected failed=1, got %d", updated.failed)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd to continue chain")
	}

	// Track 1 fails
	msg2 := trackDownloadedMsg{index: 1, media: domain.Media{}, err: errors.New(errMsg)}
	m3, cmd2 := updated.Update(msg2)
	updated2 := m3.(Model)

	if updated2.failed != 2 {
		t.Errorf("expected failed=2, got %d", updated2.failed)
	}
	if cmd2 == nil {
		t.Fatal("expected non-nil cmd to continue chain")
	}

	// Track 2 fails — end of chain
	msg3 := trackDownloadedMsg{index: 2, media: domain.Media{}, err: errors.New(errMsg)}
	m4, cmd3 := m3.Update(msg3)
	updated3 := m4.(Model)

	if cmd3 != nil {
		t.Error("expected nil cmd — all tracks processed")
	}
	if updated3.Screen != ScreenDone {
		t.Errorf("expected ScreenDone after all failed, got %d", updated3.Screen)
	}
	if updated3.failed != 3 {
		t.Errorf("expected failed=3, got %d", updated3.failed)
	}
}

func TestNoTracksSelectedForDownload(t *testing.T) {
	m := modelWithTracks(t)
	// All tracks stay as StatusPending (not selected)

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	if updated.Screen != ScreenPlaylist {
		t.Errorf("expected stay on ScreenPlaylist when nothing selected, got %d", updated.Screen)
	}
	if cmd != nil {
		t.Error("expected nil cmd when nothing selected")
	}
}

func TestInputWhitespaceURL(t *testing.T) {
	m := Model{Screen: ScreenInput, Ready: true, Input: newInput()}
	m.Input.SetValue("   ")

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	if updated.Screen != ScreenInput {
		t.Errorf("expected ScreenInput for whitespace URL, got %d", updated.Screen)
	}
	if updated.inputErr == "" {
		t.Error("expected inputErr for whitespace URL")
	}
}

func TestInputVeryLongURL(t *testing.T) {
	m := Model{Screen: ScreenInput, Ready: true, Input: newInput()}
	url := "https://youtube.com/" + strings.Repeat("a", 500)
	m.Input.SetValue(url)

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	// Should transition to resolving, not truncate or error
	if updated.Screen != ScreenResolving {
		t.Errorf("expected ScreenResolving for long URL, got %d", updated.Screen)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for resolve")
	}
}

// ---------------------------------------------------------------------------
// Search Mode Tests (PR3)
// ---------------------------------------------------------------------------

type stubQuerySearcher struct {
	result ports.SearchResult
	err    error
}

func (s *stubQuerySearcher) SearchByQuery(ctx context.Context, query string, limit int) (ports.SearchResult, error) {
	return s.result, s.err
}

func TestSearchMode_ToggleOnInput(t *testing.T) {
	m := Model{Screen: ScreenInput, Ready: true, searchMode: SearchModeURL, Input: newInput()}
	m.Input.SetValue("https://example.com")

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	updated := m2.(Model)

	if updated.searchMode != SearchModeQuery {
		t.Errorf("expected searchMode=SearchModeQuery (%d), got %d", SearchModeQuery, updated.searchMode)
	}
	if updated.Screen != ScreenInput {
		t.Errorf("expected ScreenInput (%d), got %d", ScreenInput, updated.Screen)
	}
	if updated.Input.Value() != "" {
		t.Errorf("expected input to be cleared, got %q", updated.Input.Value())
	}
}

func TestSearchMode_ToggleOnPlaylist(t *testing.T) {
	m := Model{Screen: ScreenPlaylist, Ready: true, tracks: sampleTracks(), searchMode: SearchModeURL, Input: newInput()}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	updated := m2.(Model)

	if updated.Screen != ScreenInput {
		t.Errorf("expected ScreenInput (%d), got %d", ScreenInput, updated.Screen)
	}
	if updated.tracks != nil {
		t.Error("expected tracks to be cleared")
	}
	if updated.searchMode != SearchModeQuery {
		t.Errorf("expected searchMode=SearchModeQuery (%d), got %d", SearchModeQuery, updated.searchMode)
	}
}

func TestSearchMode_ToggleOnResolving(t *testing.T) {
	m := Model{Screen: ScreenResolving, Ready: true, searchMode: SearchModeURL, Input: newInput()}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	updated := m2.(Model)

	if updated.Screen != ScreenInput {
		t.Errorf("expected ScreenInput (%d), got %d", ScreenInput, updated.Screen)
	}
	if updated.searchMode != SearchModeQuery {
		t.Errorf("expected searchMode=SearchModeQuery (%d), got %d", SearchModeQuery, updated.searchMode)
	}
}

func TestSearchMode_ToggleTwice(t *testing.T) {
	m := Model{Screen: ScreenInput, Ready: true, searchMode: SearchModeURL, Input: newInput()}

	// First toggle: URL → Query
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	afterFirst := m2.(Model)
	if afterFirst.searchMode != SearchModeQuery {
		t.Fatalf("after first toggle: expected SearchModeQuery (%d), got %d", SearchModeQuery, afterFirst.searchMode)
	}

	// Second toggle: Query → URL
	m3, _ := afterFirst.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	afterSecond := m3.(Model)
	if afterSecond.searchMode != SearchModeURL {
		t.Errorf("after second toggle: expected SearchModeURL (%d), got %d", SearchModeURL, afterSecond.searchMode)
	}
}

func TestSearchMode_EnterTriggersSearch(t *testing.T) {
	m := Model{
		Screen:        ScreenInput,
		Ready:         true,
		searchMode:    SearchModeQuery,
		Input:         newInput(),
		querySearcher: &stubQuerySearcher{},
	}
	m.Input.SetValue("rock baladas 90s")

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	if updated.Screen != ScreenResolving {
		t.Errorf("expected ScreenResolving (%d), got %d", ScreenResolving, updated.Screen)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for search resolve")
	}
}

func TestSearchMode_EmptyQuery(t *testing.T) {
	m := Model{Screen: ScreenInput, Ready: true, searchMode: SearchModeQuery, Input: newInput()}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	if updated.Screen != ScreenInput {
		t.Errorf("expected ScreenInput (%d), got %d", ScreenInput, updated.Screen)
	}
	if updated.inputErr == "" {
		t.Error("expected non-empty inputErr for empty query")
	}
}

func TestURLMode_NonURLSuggestion(t *testing.T) {
	m := Model{Screen: ScreenInput, Ready: true, searchMode: SearchModeURL, Input: newInput()}
	m.Input.SetValue("hello world")

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	if updated.Screen != ScreenInput {
		t.Errorf("expected ScreenInput (%d), got %d", ScreenInput, updated.Screen)
	}
	if updated.inputErr == "" {
		t.Error("expected non-empty inputErr for non-URL text")
	}
	if !strings.Contains(updated.inputErr, "URL") {
		t.Errorf("expected inputErr to mention URL, got %q", updated.inputErr)
	}
}

func TestURLMode_ValidURLStillResolves(t *testing.T) {
	m := Model{Screen: ScreenInput, Ready: true, searchMode: SearchModeURL, Input: newInput()}
	m.Input.SetValue("https://music.youtube.com/playlist?list=xyz")

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	if updated.Screen != ScreenResolving {
		t.Errorf("expected ScreenResolving (%d), got %d", ScreenResolving, updated.Screen)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for URL resolve")
	}
}

func TestSearchMode_SearchResultsFlow(t *testing.T) {
	m := Model{Screen: ScreenResolving, Ready: true, tracks: nil}

	msg := resolveFinishedMsg{
		tracks: sampleTracks(),
		err:    nil,
	}

	m2, cmd := m.Update(msg)
	updated := m2.(Model)

	if updated.Screen != ScreenPlaylist {
		t.Errorf("expected ScreenPlaylist (%d), got %d", ScreenPlaylist, updated.Screen)
	}
	if cmd != nil {
		t.Error("expected nil cmd for playlist")
	}
	if len(updated.tracks) != 3 {
		t.Errorf("expected 3 tracks, got %d", len(updated.tracks))
	}
}

func TestSearchMode_SeamlessToggle(t *testing.T) {
	m := Model{Screen: ScreenInput, Ready: true, searchMode: SearchModeURL, Input: newInput()}

	// Press s → SearchModeQuery
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	state := m2.(Model)
	if state.searchMode != SearchModeQuery {
		t.Fatalf("step 1: expected SearchModeQuery, got %d", state.searchMode)
	}
	if state.Screen != ScreenInput {
		t.Fatalf("step 1: expected ScreenInput, got %d", state.Screen)
	}

	// Press s again → SearchModeURL
	m3, _ := state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	state = m3.(Model)
	if state.searchMode != SearchModeURL {
		t.Fatalf("step 2: expected SearchModeURL, got %d", state.searchMode)
	}

	// Press s again → SearchModeQuery
	m4, _ := state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	state = m4.(Model)
	if state.searchMode != SearchModeQuery {
		t.Fatalf("step 3: expected SearchModeQuery, got %d", state.searchMode)
	}

	// Input should still be empty after toggles
	if state.Input.Value() != "" {
		t.Errorf("expected input to remain empty, got %q", state.Input.Value())
	}
}

func TestSearchMode_NilQuerySearcherShowsError(t *testing.T) {
	m := Model{
		Screen:     ScreenInput,
		Ready:      true,
		searchMode: SearchModeQuery,
		Input:      newInput(),
		// querySearcher is nil — no adapter wired
	}
	m.Input.SetValue("rock baladas 90s")

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	if cmd != nil {
		t.Error("expected nil cmd when querySearcher is nil")
	}
	if updated.Screen != ScreenInput {
		t.Errorf("expected ScreenInput, got %d", updated.Screen)
	}
	if updated.inputErr == "" {
		t.Error("expected inputErr when querySearcher is nil")
	}
}

type timeoutCheckStub struct {
	t *testing.T
}

func (s *timeoutCheckStub) SearchByQuery(ctx context.Context, query string, limit int) (ports.SearchResult, error) {
	_, ok := ctx.Deadline()
	if !ok {
		s.t.Error("SearchByQuery context has no deadline — timeout not set")
	}
	return ports.SearchResult{}, nil
}

func TestSearchResolveCmd_HasTimeout(t *testing.T) {
	stub := &timeoutCheckStub{t: t}
	cmd := searchResolveCmd(context.Background(), stub, "test query", 5)
	msg := cmd()

	if _, ok := msg.(resolveFinishedMsg); !ok {
		t.Errorf("expected resolveFinishedMsg, got %T", msg)
	}
}

type contextCaptureStub struct {
	capturedCtx context.Context
}

func (s *contextCaptureStub) SearchByQuery(ctx context.Context, query string, limit int) (ports.SearchResult, error) {
	s.capturedCtx = ctx
	return ports.SearchResult{}, nil
}

func TestSearch_ContextCancelledOnNavigateAway(t *testing.T) {
	stub := &contextCaptureStub{}
	m := Model{
		Screen:        ScreenInput,
		Ready:         true,
		searchMode:    SearchModeQuery,
		Input:         newInput(),
		querySearcher: stub,
	}
	m.Input.SetValue("rock baladas 90s")

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	if updated.searchCancel == nil {
		t.Fatal("expected searchCancel to be set after starting search")
	}

	// Navigate away by toggling mode
	m3, _ := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	navigated := m3.(Model)

	if navigated.searchCancel != nil {
		t.Error("expected searchCancel to be nil after navigating away")
	}

	// The captured context should now be cancelled
	if stub.capturedCtx != nil && stub.capturedCtx.Err() == nil {
		t.Error("expected context to be cancelled after navigating away")
	}
}

func TestCtrlCOnPlaylistQuits(t *testing.T) {
	m := Model{Screen: ScreenPlaylist, Ready: true, tracks: sampleTracks()}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd == nil {
		t.Error("expected non-nil quit cmd from playlist via Ctrl+C")
	}
}

func TestCtrlCOnResolvingQuits(t *testing.T) {
	m := Model{Screen: ScreenResolving, Ready: true}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd == nil {
		t.Error("expected non-nil quit cmd from resolving via Ctrl+C")
	}
}

// ---------------------------------------------------------------------------
// 23. Config screen — open/navigation (AQ-008…AQ-011)
// ---------------------------------------------------------------------------

func TestConfig_CFromScreens(t *testing.T) {
	tests := []struct {
		name       string
		screen     Screen
		expected   Screen
		prevScreen Screen
	}{
		{"from resolving", ScreenResolving, ScreenConfig, ScreenResolving},
		{"from playlist", ScreenPlaylist, ScreenConfig, ScreenPlaylist},
		{"from downloading", ScreenDownloading, ScreenConfig, ScreenDownloading},
		{"from done", ScreenDone, ScreenConfig, ScreenDone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{Screen: tt.screen, Ready: true}

			m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
			updated := m2.(Model)

			if updated.Screen != ScreenConfig {
				t.Errorf("expected ScreenConfig (%d), got %d", ScreenConfig, updated.Screen)
			}
			if updated.PrevScreen != tt.prevScreen {
				t.Errorf("expected PrevScreen=%d, got %d", tt.prevScreen, updated.PrevScreen)
			}
		})
	}
}

func TestOpenConfig_CursorAtCurrentQuality(t *testing.T) {
	m := Model{Screen: ScreenPlaylist, Ready: true, audioQuality: "192k"}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	updated := m2.(Model)

	if updated.Screen != ScreenConfig {
		t.Errorf("expected ScreenConfig (%d), got %d", ScreenConfig, updated.Screen)
	}
	if updated.qualityCursor != 1 {
		t.Errorf("expected qualityCursor=1 (192k), got %d", updated.qualityCursor)
	}
}

func TestConfig_COnInputTypes(t *testing.T) {
	m := Model{Screen: ScreenInput, Ready: true, Input: newInput()}
	m.Input.SetValue("music.youtube.com/watch")

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	updated := m2.(Model)

	if updated.Screen != ScreenInput {
		t.Errorf("expected stay on ScreenInput, got %d", updated.Screen)
	}
	if !strings.HasSuffix(updated.Input.Value(), "c") {
		t.Errorf("expected 'c' to be appended to input, got %q", updated.Input.Value())
	}
}

func TestConfig_CInFilterTypes(t *testing.T) {
	m := Model{Screen: ScreenPlaylist, Ready: true, isFiltering: true, filterInput: newFilterInput()}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	updated := m2.(Model)

	if !updated.isFiltering {
		t.Error("expected filter to stay open after typing 'c'")
	}
	if updated.filter != "c" {
		t.Errorf("expected filter text to gain 'c', got %q", updated.filter)
	}
}

func TestConfig_JKMovesBounded(t *testing.T) {
	m := Model{Screen: ScreenConfig, Ready: true, qualityCursor: 0}

	// k above the first option stays at 0
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if got := m2.(Model).qualityCursor; got != 0 {
		t.Errorf("expected cursor=0 after k at first option, got %d", got)
	}

	// j moves down to 1
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if got := m3.(Model).qualityCursor; got != 1 {
		t.Errorf("expected cursor=1 after j, got %d", got)
	}

	// j again to 2, then j below the last option stays at 2
	m4, _ := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if got := m4.(Model).qualityCursor; got != 2 {
		t.Errorf("expected cursor=2 after second j, got %d", got)
	}
	m5, _ := m4.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if got := m5.(Model).qualityCursor; got != 2 {
		t.Errorf("expected cursor clamped at 2, got %d", got)
	}
}

func TestConfig_EscCancelsWithoutWrite(t *testing.T) {
	spy := &saveConfigSpy{}
	m := Model{
		Screen:        ScreenConfig,
		PrevScreen:    ScreenPlaylist,
		Ready:         true,
		audioQuality:  "128k",
		qualityCursor: 1,
		configPath:    "/tmp/config.toml",
		saveConfig:    spy.save,
	}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := m2.(Model)

	if updated.Screen != ScreenPlaylist {
		t.Errorf("expected ScreenPlaylist (%d) after Esc, got %d", ScreenPlaylist, updated.Screen)
	}
	if updated.audioQuality != "128k" {
		t.Errorf("expected audioQuality unchanged (128k), got %q", updated.audioQuality)
	}
	if spy.called != 0 {
		t.Errorf("expected saveConfig not called on Esc, called %d times", spy.called)
	}
}

func TestConfig_QStillQuits(t *testing.T) {
	m := Model{Screen: ScreenConfig, Ready: true}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	if cmd == nil {
		t.Error("expected non-nil quit cmd on Config via q")
	}
}

func TestConfig_HelpStillToggles(t *testing.T) {
	m := Model{Screen: ScreenConfig, Ready: true, showHelp: false}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !m2.(Model).showHelp {
		t.Error("expected showHelp=true after ? on Config")
	}

	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if m3.(Model).showHelp {
		t.Error("expected showHelp=false after second ? on Config")
	}
}

// ---------------------------------------------------------------------------
// 24. Config screen — confirm/persist/failure (AQ-012, AQ-013)
// ---------------------------------------------------------------------------

func TestConfig_EnterConfirmsAndPersists(t *testing.T) {
	rec := &recordingDownloader{}
	orch := service.NewOrchestrator(&stubSearcher{}, rec)
	configPath := filepath.Join(t.TempDir(), "config.toml")

	m := Model{
		Screen:        ScreenConfig,
		PrevScreen:    ScreenPlaylist,
		Ready:         true,
		audioQuality:  "320k",
		qualityCursor: 1, // 192k
		orchestrator:  orch,
		configPath:    configPath,
		saveConfig:    config.SaveConfig,
	}

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	if cmd != nil {
		t.Errorf("expected nil cmd on confirm, got %v", cmd)
	}
	if updated.audioQuality != "192k" {
		t.Errorf("expected audioQuality=192k, got %q", updated.audioQuality)
	}
	if rec.bitrate != "192k" {
		t.Errorf("expected downloader bitrate 192k, got %q", rec.bitrate)
	}
	if updated.Screen != ScreenPlaylist {
		t.Errorf("expected return to ScreenPlaylist (%d), got %d", ScreenPlaylist, updated.Screen)
	}

	var persisted config.Config
	if _, err := toml.DecodeFile(configPath, &persisted); err != nil {
		t.Fatalf("decode persisted config: %v", err)
	}
	if persisted.Quality.Value != "192k" {
		t.Errorf("expected persisted [quality] value=192k, got %q", persisted.Quality.Value)
	}
}

func TestConfig_EnterSaveFailureNonFatal(t *testing.T) {
	rec := &recordingDownloader{}
	orch := service.NewOrchestrator(&stubSearcher{}, rec)

	m := Model{
		Screen:        ScreenConfig,
		PrevScreen:    ScreenPlaylist,
		Ready:         true,
		audioQuality:  "320k",
		qualityCursor: 0, // 128k
		orchestrator:  orch,
		configPath:    "/tmp/config.toml",
		saveConfig: func(string, config.Config) error {
			return errors.New("disk full")
		},
	}

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	if cmd != nil {
		t.Errorf("expected no quit cmd on save failure, got %v", cmd)
	}
	if updated.audioQuality != "128k" {
		t.Errorf("expected audioQuality=128k (still applied), got %q", updated.audioQuality)
	}
	if rec.bitrate != "128k" {
		t.Errorf("expected downloader bitrate 128k, got %q", rec.bitrate)
	}
	if updated.configWarn == "" {
		t.Error("expected configWarn to be set on save failure")
	}
	if updated.Screen != ScreenConfig {
		t.Errorf("expected stay on ScreenConfig (%d), got %d", ScreenConfig, updated.Screen)
	}
}

// ---------------------------------------------------------------------------
// 25. URL-aware searcher routing (ARF-001) + spotify: gate (ARF-002, ARF-008)
// ---------------------------------------------------------------------------

func TestSelectedSearcherRouting(t *testing.T) {
	yt := &stubSearcher{}
	sp := &stubSearcher{}
	tests := []struct {
		name       string
		mode       SourceMode
		url        string
		configured bool
		want       ports.Searcher
	}{
		// case 1 (issue #19): Auto + YouTube Music URL + configured → yt
		{name: "auto youtube url configured", mode: SourceAuto, url: "https://music.youtube.com/watch?v=...", configured: true, want: yt},
		// case 2: Auto + open.spotify.com/track + configured → sp
		{name: "auto spotify track configured", mode: SourceAuto, url: "https://open.spotify.com/track/{id}", configured: true, want: sp},
		// case 3: Auto + spotify: URI + configured → sp
		{name: "auto spotify uri configured", mode: SourceAuto, url: "spotify:track:{id}", configured: true, want: sp},
		// case 4: Auto + Spotify URL, no credentials → yt
		{name: "auto spotify url no creds", mode: SourceAuto, url: "https://open.spotify.com/track/{id}", configured: false, want: yt},
		// case 5: Auto + non-Spotify URL, no credentials → yt
		{name: "auto youtube url no creds", mode: SourceAuto, url: "https://music.youtube.com/watch?v=...", configured: false, want: yt},
		// case 6: SourceYouTube ignores the URL → yt (both Spotify URL and URI forms)
		{name: "youtube mode spotify url", mode: SourceYouTube, url: "https://open.spotify.com/track/{id}", configured: true, want: yt},
		{name: "youtube mode spotify uri", mode: SourceYouTube, url: "spotify:track:{id}", configured: true, want: yt},
		// case 7a: SourceSpotify + configured → sp
		{name: "spotify mode configured", mode: SourceSpotify, url: "https://music.youtube.com/watch?v=...", configured: true, want: sp},
		// case 7b: SourceSpotify + not configured → yt
		{name: "spotify mode no creds", mode: SourceSpotify, url: "https://open.spotify.com/track/{id}", configured: false, want: yt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{sourceMode: tt.mode, searcher: yt}
			if tt.configured {
				m.spotifySearcher = sp
			}
			if got := m.selectedSearcher(tt.url); got != tt.want {
				t.Errorf("selectedSearcher(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

// spotify: URI accepted — no "That doesn't look like a URL" (ARF-002)
func TestURLMode_SpotifyURIAccepted(t *testing.T) {
	m := Model{Screen: ScreenInput, Ready: true, searchMode: SearchModeURL, Input: newInput()}
	m.Input.SetValue("spotify:track:4iV5W9uYEdYUVa79Axb7Rh")

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	if updated.Screen != ScreenResolving {
		t.Errorf("expected ScreenResolving (%d), got %d", ScreenResolving, updated.Screen)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for spotify: URI resolve")
	}
	if strings.Contains(updated.inputErr, "That doesn't look like a URL") {
		t.Errorf("spotify: URI must not be rejected, got inputErr %q", updated.inputErr)
	}
}

// any other scheme stays blocked (ARF-008)
func TestURLMode_OtherSchemeStillBlocked(t *testing.T) {
	m := Model{Screen: ScreenInput, Ready: true, searchMode: SearchModeURL, Input: newInput()}
	m.Input.SetValue("itunes:track:123")

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := m2.(Model)

	if updated.Screen != ScreenInput {
		t.Errorf("expected stay on ScreenInput (%d), got %d", ScreenInput, updated.Screen)
	}
	if !strings.Contains(updated.inputErr, "That doesn't look like a URL") {
		t.Errorf("expected inputErr to contain \"That doesn't look like a URL\", got %q", updated.inputErr)
	}
}
