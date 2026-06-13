package ffmpeg

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chenzhenrui/webrtc2nas/internal/config"
	"github.com/chenzhenrui/webrtc2nas/internal/storage"
)

func TestBuildCommand(t *testing.T) {
	cfg := config.StreamConfig{
		Name:           "living_room",
		URL:            "rtsp://homeassistant.local:8554/living_room",
		MaxFiles:       144,
		SegmentMinutes: 10,
	}
	sm := storage.NewManager("/tmp/rec")
	runner := NewRunner(cfg, sm)

	cmd := runner.Command(context.Background())
	if len(cmd.Args) == 0 {
		t.Fatal("no ffmpeg args built")
	}
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "ffmpeg") {
		t.Errorf("expected ffmpeg in args: %s", args)
	}
	if !strings.Contains(args, "-rtsp_transport tcp") {
		t.Errorf("missing -rtsp_transport tcp")
	}
	if !strings.Contains(args, "-c:v copy") {
		t.Errorf("missing -c:v copy")
	}
	if !strings.Contains(args, "-c:a aac") {
		t.Errorf("missing -c:a aac")
	}
	if !strings.Contains(args, "-segment_time 600") {
		t.Errorf("missing -segment_time 600")
	}
}

func TestSegmentDuration(t *testing.T) {
	cfg := config.StreamConfig{SegmentMinutes: 10}
	if cfg.SegmentDuration() != 10*time.Minute {
		t.Errorf("segment duration = %v", cfg.SegmentDuration())
	}
}
