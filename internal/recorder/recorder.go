package recorder

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/chenzhenrui/webrtc2nas/internal/config"
	"github.com/chenzhenrui/webrtc2nas/internal/ffmpeg"
	"github.com/chenzhenrui/webrtc2nas/internal/storage"
)

// Recorder manages recording goroutines for all configured streams.
type Recorder struct {
	cfg     *config.Config
	storage *storage.Manager
	logger  *slog.Logger
	wg      sync.WaitGroup
}

// New creates a new Recorder.
func New(cfg *config.Config, sm *storage.Manager, logger *slog.Logger) *Recorder {
	if logger == nil {
		logger = slog.Default()
	}
	return &Recorder{
		cfg:     cfg,
		storage: sm,
		logger:  logger,
	}
}

// Start launches recording goroutines for all enabled streams.
func (r *Recorder) Start(ctx context.Context) error {
	for _, s := range r.cfg.Streams {
		enabled := true
		if s.Enabled != nil {
			enabled = *s.Enabled
		}
		if !enabled {
			continue
		}
		r.wg.Add(1)
		go r.runStream(ctx, s)
	}
	return nil
}

// Wait blocks until all recording goroutines have stopped.
func (r *Recorder) Wait() {
	r.wg.Wait()
}

func (r *Recorder) runStream(ctx context.Context, s config.StreamConfig) {
	defer r.wg.Done()

	cleanupTicker := time.NewTicker(30 * time.Second)
	defer cleanupTicker.Stop()

	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		r.logger.Info("starting stream", "stream", s.Name, "url", s.URL)
		runner := ffmpeg.NewRunner(s, r.storage)

		streamCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() {
			done <- runner.Run(streamCtx)
		}()

		// Run cleanup loop concurrently with the stream.
		cleanupDone := make(chan struct{})
		go func() {
			defer close(cleanupDone)
			for {
				select {
				case <-cleanupTicker.C:
					if err := r.storage.Cleanup(s.Name, s.MaxFiles); err != nil {
						r.logger.Error("cleanup failed", "stream", s.Name, "error", err)
					}
				case <-streamCtx.Done():
					return
				}
			}
		}()

		err := <-done
		cancel()
		<-cleanupDone

		if ctx.Err() != nil {
			// Graceful shutdown due to context cancellation.
			r.logger.Info("stream stopped", "stream", s.Name)
			return
		}

		if err != nil {
			r.logger.Error("stream exited", "stream", s.Name, "error", err)
		} else {
			r.logger.Info("stream ended gracefully", "stream", s.Name)
			return
		}

		delay := backoffDelay(attempt, 60*time.Second)
		attempt++
		r.logger.Info("reconnecting stream", "stream", s.Name, "delay", delay)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

func backoffDelay(attempt int, max time.Duration) time.Duration {
	d := time.Duration(1 << attempt) * time.Second
	if d > max {
		d = max
	}
	if d < 1*time.Second {
		d = 1 * time.Second
	}
	return d
}
