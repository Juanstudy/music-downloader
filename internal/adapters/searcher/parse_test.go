package searcher

import (
	"testing"
	"time"

	"github.com/Juanstudy/music-downloader/internal/core/domain"
)

func TestParseLine_ValidJSON(t *testing.T) {
	tests := []struct {
		name         string
		json         string
		wantURL      string
		wantTitle    string
		wantArtist   string
		wantDuration time.Duration
		wantSource   string
		wantStatus   domain.Status
	}{
		{
			name:         "complete JSON with all fields",
			json:         `{"webpage_url":"https://youtube.com/watch?v=dQw4w9WgXcQ","title":"Never Gonna Give You Up","channel":"Rick Astley","duration":212.0,"id":"dQw4w9WgXcQ"}`,
			wantURL:      "https://youtube.com/watch?v=dQw4w9WgXcQ",
			wantTitle:    "Never Gonna Give You Up",
			wantArtist:   "Rick Astley",
			wantDuration: 212 * time.Second,
			wantSource:   "youtube",
			wantStatus:   domain.StatusPending,
		},
		{
			name:       "channel maps to artist",
			json:       `{"webpage_url":"https://youtube.com/watch?v=abc","title":"Song","channel":"Artist Channel","uploader":"Uploader Name","creator":"Creator Name"}`,
			wantURL:    "https://youtube.com/watch?v=abc",
			wantTitle:  "Song",
			wantArtist: "Artist Channel",
		},
		{
			name:       "uploader fallback when channel empty",
			json:       `{"webpage_url":"https://youtube.com/watch?v=abc","title":"Song","channel":"","uploader":"Uploader Name","creator":"Creator Name"}`,
			wantURL:    "https://youtube.com/watch?v=abc",
			wantTitle:  "Song",
			wantArtist: "Uploader Name",
		},
		{
			name:       "creator fallback when channel and uploader empty",
			json:       `{"webpage_url":"https://youtube.com/watch?v=abc","title":"Song","channel":"","uploader":"","creator":"Creator Name"}`,
			wantURL:    "https://youtube.com/watch?v=abc",
			wantTitle:  "Song",
			wantArtist: "Creator Name",
		},
		{
			name:         "float duration maps to time.Duration",
			json:         `{"webpage_url":"https://youtube.com/watch?v=abc","title":"Song","duration":180.5}`,
			wantURL:      "https://youtube.com/watch?v=abc",
			wantTitle:    "Song",
			wantDuration: 180*time.Second + 500*time.Millisecond,
		},
		{
			name:         "zero duration",
			json:         `{"webpage_url":"https://youtube.com/watch?v=abc","title":"Song","duration":0}`,
			wantURL:      "https://youtube.com/watch?v=abc",
			wantTitle:    "Song",
			wantDuration: 0,
		},
		{
			name:       "no artist fields yields empty artist",
			json:       `{"webpage_url":"https://youtube.com/watch?v=abc","title":"Song"}`,
			wantURL:    "https://youtube.com/watch?v=abc",
			wantTitle:  "Song",
			wantArtist: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLine(tt.json)
			if err != nil {
				t.Fatalf("ParseLine() returned unexpected error: %v", err)
			}
			if got.URL != tt.wantURL {
				t.Errorf("ParseLine().URL = %q, want %q", got.URL, tt.wantURL)
			}
			if got.Title != tt.wantTitle {
				t.Errorf("ParseLine().Title = %q, want %q", got.Title, tt.wantTitle)
			}
			if tt.wantArtist != "" && got.Artist != tt.wantArtist {
				t.Errorf("ParseLine().Artist = %q, want %q", got.Artist, tt.wantArtist)
			}
			if tt.wantDuration != 0 && got.Duration != tt.wantDuration {
				t.Errorf("ParseLine().Duration = %v, want %v", got.Duration, tt.wantDuration)
			}
			if tt.wantSource != "" && got.Source != tt.wantSource {
				t.Errorf("ParseLine().Source = %q, want %q", got.Source, tt.wantSource)
			}
			if tt.wantStatus != 0 && got.Status != tt.wantStatus {
				t.Errorf("ParseLine().Status = %v, want %v", got.Status, tt.wantStatus)
			}
		})
	}
}

func TestParseLine_InvalidJSON(t *testing.T) {
	_, err := ParseLine("this is not json")
	if err == nil {
		t.Fatal("ParseLine() expected error for invalid JSON, got nil")
	}
}

func TestParseLine_MissingTitle(t *testing.T) {
	json := `{"webpage_url":"https://youtube.com/watch?v=abc","duration":100.0}`
	_, err := ParseLine(json)
	if err == nil {
		t.Fatal("ParseLine() expected error for missing title, got nil")
	}
}

func TestParseLine_EmptyString(t *testing.T) {
	_, err := ParseLine("")
	if err == nil {
		t.Fatal("ParseLine() expected error for empty string, got nil")
	}
}

func TestParseLine_AllEmptyFields(t *testing.T) {
	json := `{"webpage_url":"https://youtube.com/watch?v=abc","title":"","channel":""}`
	_, err := ParseLine(json)
	if err == nil {
		t.Fatal("ParseLine() expected error for empty title, got nil")
	}
}

func TestParseLine_VeryLongTitle(t *testing.T) {
	title := ""
	for i := 0; i < 1000; i++ {
		title += "x"
	}
	json := `{"webpage_url":"https://youtube.com/watch?v=abc","title":"` + title + `","channel":"Artist"}`
	got, err := ParseLine(json)
	if err != nil {
		t.Fatalf("ParseLine() returned unexpected error for long title: %v", err)
	}
	if got.Title != title {
		t.Errorf("ParseLine().Title length = %d, want %d", len(got.Title), len(title))
	}
}

func TestParseLine_NegativeDuration(t *testing.T) {
	json := `{"webpage_url":"https://youtube.com/watch?v=abc","title":"Song","duration":-1}`
	got, err := ParseLine(json)
	if err != nil {
		t.Fatalf("ParseLine() returned unexpected error for negative duration: %v", err)
	}
	if got.Duration != -1*time.Second {
		t.Errorf("ParseLine().Duration = %v, want -1s", got.Duration)
	}
}

func TestParseLine_VeryLargeDuration(t *testing.T) {
	json := `{"webpage_url":"https://youtube.com/watch?v=abc","title":"Song","duration":999999999}`
	got, err := ParseLine(json)
	if err != nil {
		t.Fatalf("ParseLine() returned unexpected error for large duration: %v", err)
	}
	if got.Duration <= 0 {
		t.Errorf("ParseLine().Duration = %v, want > 0", got.Duration)
	}
}

func TestParseLine_URLWithSpecialChars(t *testing.T) {
	json := `{"webpage_url":"https://youtube.com/watch?v=a b&q=foo#bar","title":"Song","channel":"Artist"}`
	got, err := ParseLine(json)
	if err != nil {
		t.Fatalf("ParseLine() returned unexpected error for URL with special chars: %v", err)
	}
	if got.URL != "https://youtube.com/watch?v=a b&q=foo#bar" {
		t.Errorf("ParseLine().URL = %q, want URL with special chars preserved", got.URL)
	}
}

func TestParseLine_DurationAsNull(t *testing.T) {
	json := `{"webpage_url":"https://youtube.com/watch?v=abc","title":"Song","duration":null}`
	got, err := ParseLine(json)
	if err != nil {
		t.Fatalf("ParseLine() returned unexpected error for null duration: %v", err)
	}
	if got.Duration != 0 {
		t.Errorf("ParseLine().Duration for null = %v, want 0", got.Duration)
	}
}
