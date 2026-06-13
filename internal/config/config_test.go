package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	content := `
output_dir: ./recordings
log_level: info
playback_host: 127.0.0.1
playback_port: 8080
streams:
  - name: living_room
    url: rtsp://homeassistant.local:8554/living_room
    max_files: 144
    segment_minutes: 10
    enabled: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.OutputDir != "./recordings" {
		t.Errorf("output_dir = %q, want %q", cfg.OutputDir, "./recordings")
	}
	if cfg.PlaybackPort != 8080 {
		t.Errorf("playback_port = %d, want 8080", cfg.PlaybackPort)
	}
	if len(cfg.Streams) != 1 {
		t.Fatalf("streams len = %d, want 1", len(cfg.Streams))
	}
	if cfg.Streams[0].Name != "living_room" {
		t.Errorf("stream name = %q, want living_room", cfg.Streams[0].Name)
	}
}

func TestValidateMissingName(t *testing.T) {
	cfg := &Config{
		OutputDir: "./recordings",
		Streams: []StreamConfig{
			{Name: "", URL: "rtsp://x", MaxFiles: 10},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestValidateInvalidURL(t *testing.T) {
	cfg := &Config{
		OutputDir: "./recordings",
		Streams: []StreamConfig{
			{Name: "cam1", URL: "http://x", MaxFiles: 10},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for invalid url, got nil")
	}
}
