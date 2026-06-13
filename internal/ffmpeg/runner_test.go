package ffmpeg

import (
	"context"
	"strings"
	"testing"

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

	cmd, err := runner.Command(context.Background())
	if err != nil {
		t.Fatalf("Command() error = %v", err)
	}
	if len(cmd.Args) == 0 {
		t.Fatal("no ffmpeg args built")
	}
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "ffmpeg") {
		t.Errorf("expected ffmpeg in args: %s", args)
	}
	if !strings.Contains(args, "-loglevel error") {
		t.Errorf("missing -loglevel error")
	}
	if !strings.Contains(args, "-i rtsp://homeassistant.local:8554/living_room") {
		t.Errorf("missing -i rtsp://...")
	}
	if !strings.Contains(args, "-ar 8000") {
		t.Errorf("missing -ar 8000")
	}
	if !strings.Contains(args, "-f segment") {
		t.Errorf("missing -f segment")
	}
	if !strings.Contains(args, "-reset_timestamps 1") {
		t.Errorf("missing -reset_timestamps 1")
	}
	if !strings.Contains(args, "-strftime 1") {
		t.Errorf("missing -strftime 1")
	}
	if !strings.Contains(args, "%Y-%m-%d/video_%H-%M-%S.mp4") {
		t.Errorf("missing output pattern with %%Y-%%m-%%d/video_%%H-%%M-%%S.mp4")
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
