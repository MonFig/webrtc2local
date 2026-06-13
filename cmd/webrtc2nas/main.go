package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chenzhenrui/webrtc2nas/internal/config"
	"github.com/chenzhenrui/webrtc2nas/internal/ffmpeg"
	"github.com/chenzhenrui/webrtc2nas/internal/playback"
	"github.com/chenzhenrui/webrtc2nas/internal/recorder"
	"github.com/chenzhenrui/webrtc2nas/internal/storage"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		logger.Error("failed to create output dir", "error", err)
		os.Exit(1)
	}

	// Verify output directory is writable.
	tmpFile, err := os.CreateTemp(cfg.OutputDir, ".write-test-*")
	if err != nil {
		logger.Error("output dir is not writable", "dir", cfg.OutputDir, "error", err)
		os.Exit(1)
	}
	_ = tmpFile.Close()
	_ = os.Remove(tmpFile.Name())

	// Check ffmpeg availability.
	if _, err := ffmpeg.LookPath(); err != nil {
		logger.Error("ffmpeg not found in PATH", "error", err)
		os.Exit(1)
	}

	storageManager := storage.NewManager(cfg.OutputDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := recorder.New(cfg, storageManager, logger)
	if err := rec.Start(ctx); err != nil {
		logger.Error("failed to start recorder", "error", err)
		os.Exit(1)
	}

	pb := playback.New(cfg, storageManager)
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.PlaybackHost, cfg.PlaybackPort),
		Handler: pb,
	}
	go func() {
		logger.Info("playback server listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("playback server error", "error", err)
		}
	}()

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)

	rec.Wait()
	logger.Info("goodbye")
}
