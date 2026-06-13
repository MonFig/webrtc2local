package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/chenzhenrui/webrtc2nas/internal/config"
	"github.com/chenzhenrui/webrtc2nas/internal/storage"
)

// Runner manages a single ffmpeg subprocess for one stream.
type Runner struct {
	cfg     config.StreamConfig
	storage *storage.Manager
}

// NewRunner creates a new ffmpeg runner.
func NewRunner(cfg config.StreamConfig, sm *storage.Manager) *Runner {
	return &Runner{cfg: cfg, storage: sm}
}

// LookPath returns the path to the ffmpeg binary or an error if not found.
func LookPath() (string, error) {
	return exec.LookPath("ffmpeg")
}

// Command builds the ffmpeg exec.Cmd for this stream.
func (r *Runner) Command(ctx context.Context) *exec.Cmd {
	bin, _ := LookPath()
	outputPattern := filepath.Join(
		r.storage.StreamDir(r.cfg.Name),
		"%Y-%m-%d",
		"video_%H-%M-%S.mp4",
	)

	args := []string{
		"-loglevel", "error",
		"-rtsp_transport", "tcp",
		"-i", r.cfg.URL,
		"-c:v", "copy",
		"-c:a", "aac",
		"-ar", "8000",
		"-f", "segment",
		"-segment_time", fmt.Sprintf("%d", r.cfg.SegmentMinutes*60),
		"-reset_timestamps", "1",
		"-strftime", "1",
		outputPattern,
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// Run starts ffmpeg and blocks until it exits.
func (r *Runner) Run(ctx context.Context) error {
	if err := r.storage.EnsureStreamDir(r.cfg.Name); err != nil {
		return fmt.Errorf("ensure stream dir: %w", err)
	}
	cmd := r.Command(ctx)
	return cmd.Run()
}
