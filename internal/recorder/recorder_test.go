package recorder

import (
	"testing"
	"time"

	"github.com/chenzhenrui/webrtc2nas/internal/config"
	"github.com/chenzhenrui/webrtc2nas/internal/storage"
)

func TestBackoffDelay(t *testing.T) {
	cases := []struct {
		attempt int
		max     time.Duration
		want    time.Duration
	}{
		{0, 60 * time.Second, 1 * time.Second},
		{1, 60 * time.Second, 2 * time.Second},
		{2, 60 * time.Second, 4 * time.Second},
		{5, 60 * time.Second, 32 * time.Second},
	}
	for _, c := range cases {
		got := backoffDelay(c.attempt, c.max)
		if got != c.want {
			t.Errorf("backoffDelay(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}
}

func TestRecorderCreatesStorage(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		OutputDir: root,
		Streams: []config.StreamConfig{
			{Name: "cam1", URL: "rtsp://x", MaxFiles: 10, SegmentMinutes: 10},
		},
	}
	sm := storage.NewManager(root)
	rec := New(cfg, sm, nil)

	// Verify storage manager is set.
	if rec.storage == nil {
		t.Fatal("storage manager not set")
	}
}
