package playback

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenzhenrui/webrtc2nas/internal/config"
	"github.com/chenzhenrui/webrtc2nas/internal/storage"
)

func TestListCameras(t *testing.T) {
	cfg := &config.Config{
		Streams: []config.StreamConfig{
			{Name: "cam1", URL: "rtsp://x", MaxFiles: 10},
			{Name: "cam2", URL: "rtsp://x", MaxFiles: 10},
		},
	}
	sm := storage.NewManager(t.TempDir())
	srv := New(cfg, sm)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp struct {
		Cameras []struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		} `json:"cameras"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Cameras) != 2 {
		t.Fatalf("cameras len = %d, want 2", len(resp.Cameras))
	}
}

func TestTimeline(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		Streams: []config.StreamConfig{
			{Name: "cam1", URL: "rtsp://x", MaxFiles: 10},
		},
	}
	sm := storage.NewManager(root)

	// Create a fake segment.
	ts := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	path := sm.SegmentPath("cam1", ts)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("dummy"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	srv := New(cfg, sm)
	req := httptest.NewRequest(http.MethodGet, "/api/timeline/cam1", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp struct {
		Files []struct {
			URL string `json:"url"`
		} `json:"files"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Files) != 1 {
		t.Fatalf("files len = %d, want 1", len(resp.Files))
	}
}
