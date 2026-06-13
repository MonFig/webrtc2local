# webrtc2nas Recorder Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go command-line tool that records multiple RTSP camera streams into 10-minute MP4 segments with circular retention, and serves a simple web UI for playback.

**Architecture:** A single Go binary reads YAML config, starts one ffmpeg subprocess per stream, manages segment files on disk, and runs an embedded HTTP server for listing and playing recorded segments. Components are split into `config`, `storage`, `ffmpeg`, `recorder`, and `playback` packages.

**Tech Stack:** Go 1.22+, ffmpeg (system dependency), `gopkg.in/yaml.v3`, Go standard library HTTP server, `embed` for static UI files.

---

## File Structure

```
webrtc2nas/
├── cmd/
│   └── webrtc2nas/
│       └── main.go              # entry point, signal handling, wiring
├── internal/
│   ├── config/
│   │   ├── config.go            # YAML loading and validation
│   │   └── config_test.go       # config tests
│   ├── storage/
│   │   ├── storage.go           # directory paths, segment scanning, cleanup
│   │   └── storage_test.go      # storage tests
│   ├── ffmpeg/
│   │   ├── runner.go            # ffmpeg command builder and process runner
│   │   └── runner_test.go       # runner tests
│   ├── recorder/
│   │   ├── recorder.go          # per-stream goroutine orchestration
│   │   └── recorder_test.go     # recorder tests (mocks)
│   └── playback/
│       ├── server.go            # HTTP API and static file serving
│       ├── server_test.go       # playback tests
│       ├── embed.go             # embeds static/ into binary
│       └── static/
│           ├── index.html       # playback UI
│           ├── app.js           # frontend logic
│           └── style.css        # basic styling
├── go.mod
├── go.sum
├── config.example.yaml          # example configuration
└── README.md                    # usage instructions
```

---

## Task 1: Initialize Go Module and Project Structure

**Files:**
- Create: `go.mod`
- Create: `cmd/webrtc2nas/main.go` (stub)
- Create: directory structure

- [ ] **Step 1: Initialize Go module**

Run:

```bash
cd /Users/chenzhenrui/workspace/webrtc2nas
go mod init github.com/chenzhenrui/webrtc2nas
```

Expected output: `go: creating new go.mod: module github.com/chenzhenrui/webrtc2nas`

- [ ] **Step 2: Create directory layout**

Run:

```bash
mkdir -p cmd/webrtc2nas internal/config internal/storage internal/ffmpeg internal/recorder internal/playback/static
```

- [ ] **Step 3: Create stub main.go**

Create `cmd/webrtc2nas/main.go`:

```go
package main

import "fmt"

func main() {
	fmt.Println("webrtc2nas starting")
}
```

- [ ] **Step 4: Verify build**

Run:

```bash
go build -o webrtc2nas ./cmd/webrtc2nas
./webrtc2nas
```

Expected output: `webrtc2nas starting`

- [ ] **Step 5: Commit**

```bash
git add go.mod cmd/webrtc2nas/main.go
git commit -m "chore: initialize Go module and project structure

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: Implement Configuration Loading and Validation

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Modify: `go.mod` / `go.sum` (add yaml dependency)

- [ ] **Step 1: Write the failing test**

Create `internal/config/config_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/config -v
```

Expected: compilation errors or FAIL due to undefined functions.

- [ ] **Step 3: Add YAML dependency**

Run:

```bash
go get gopkg.in/yaml.v3
```

- [ ] **Step 4: Implement config package**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var nameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Config is the top-level application configuration.
type Config struct {
	OutputDir    string         `yaml:"output_dir"`
	LogLevel     string         `yaml:"log_level"`
	PlaybackHost string         `yaml:"playback_host"`
	PlaybackPort int            `yaml:"playback_port"`
	Streams      []StreamConfig `yaml:"streams"`
}

// StreamConfig describes a single camera stream to record.
type StreamConfig struct {
	Name           string `yaml:"name"`
	URL            string `yaml:"url"`
	MaxFiles       int    `yaml:"max_files"`
	SegmentMinutes int    `yaml:"segment_minutes"`
	Enabled        *bool  `yaml:"enabled"`
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{
		LogLevel:     "info",
		PlaybackHost: "127.0.0.1",
		PlaybackPort: 8080,
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	for i := range cfg.Streams {
		if cfg.Streams[i].SegmentMinutes == 0 {
			cfg.Streams[i].SegmentMinutes = 10
		}
		if cfg.Streams[i].Enabled == nil {
			enabled := true
			cfg.Streams[i].Enabled = &enabled
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks the configuration for correctness.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.OutputDir) == "" {
		return fmt.Errorf("output_dir is required")
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.PlaybackHost == "" {
		c.PlaybackHost = "127.0.0.1"
	}
	if c.PlaybackPort <= 0 || c.PlaybackPort > 65535 {
		return fmt.Errorf("playback_port must be between 1 and 65535")
	}
	if len(c.Streams) == 0 {
		return fmt.Errorf("at least one stream is required")
	}

	names := make(map[string]struct{})
	for i, s := range c.Streams {
		enabled := true
		if s.Enabled != nil {
			enabled = *s.Enabled
		}
		if !enabled {
			continue
		}
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("stream[%d].name is required", i)
		}
		if !nameRegexp.MatchString(s.Name) {
			return fmt.Errorf("stream[%d].name must match [a-zA-Z0-9_-]", i)
		}
		if _, exists := names[s.Name]; exists {
			return fmt.Errorf("duplicate stream name: %s", s.Name)
		}
		names[s.Name] = struct{}{}
		if !strings.HasPrefix(s.URL, "rtsp://") {
			return fmt.Errorf("stream[%d].url must start with rtsp://", i)
		}
		if s.MaxFiles <= 0 {
			return fmt.Errorf("stream[%d].max_files must be > 0", i)
		}
		if s.SegmentMinutes <= 0 || s.SegmentMinutes > 60 {
			return fmt.Errorf("stream[%d].segment_minutes must be between 1 and 60", i)
		}
	}
	return nil
}

// SegmentDuration returns the segment duration as time.Duration.
func (s *StreamConfig) SegmentDuration() time.Duration {
	return time.Duration(s.SegmentMinutes) * time.Minute
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run:

```bash
go test ./internal/config -v
```

Expected: PASS for both tests.

- [ ] **Step 6: Commit**

```bash
git add internal/config go.mod go.sum
git commit -m "feat: add config loading and validation

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: Implement Storage Layer

**Files:**
- Create: `internal/storage/storage.go`
- Create: `internal/storage/storage_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storage/storage_test.go`:

```go
package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSegmentPath(t *testing.T) {
	mgr := NewManager("/tmp/rec")
	ts := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	path := mgr.SegmentPath("living_room", ts)
	want := filepath.Join("/tmp/rec", "living_room", "2026-06-13", "video_09-00-00.mp4")
	if path != want {
		t.Errorf("SegmentPath = %q, want %q", path, want)
	}
}

func TestEnsureStreamDir(t *testing.T) {
	root := t.TempDir()
	mgr := NewManager(root)
	if err := mgr.EnsureStreamDir("living_room"); err != nil {
		t.Fatalf("EnsureStreamDir failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "living_room")); err != nil {
		t.Errorf("stream dir not created: %v", err)
	}
}

func TestListAndCleanup(t *testing.T) {
	root := t.TempDir()
	mgr := NewManager(root)

	// Create 5 fake segments for one camera.
	for i := 0; i < 5; i++ {
		ts := time.Date(2026, 6, 13, 9, i*10, 0, 0, time.UTC)
		path := mgr.SegmentPath("cam1", ts)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte("dummy"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		// Stagger mod times so ordering is deterministic.
		newTime := time.Date(2026, 6, 13, 9, i*10, 0, 0, time.UTC)
		if err := os.Chtimes(path, newTime, newTime); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}

	segs, err := mgr.ListSegments("cam1")
	if err != nil {
		t.Fatalf("ListSegments: %v", err)
	}
	if len(segs) != 5 {
		t.Fatalf("len(segs) = %d, want 5", len(segs))
	}

	// Keep only 3 files; oldest 2 should be removed.
	if err := mgr.Cleanup("cam1", 3); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	segs, err = mgr.ListSegments("cam1")
	if err != nil {
		t.Fatalf("ListSegments after cleanup: %v", err)
	}
	if len(segs) != 3 {
		t.Fatalf("len(segs) after cleanup = %d, want 3", len(segs))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/storage -v
```

Expected: compilation errors.

- [ ] **Step 3: Implement storage package**

Create `internal/storage/storage.go`:

```go
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Manager handles all filesystem operations for recorded segments.
type Manager struct {
	root string
}

// Segment represents a recorded video file.
type Segment struct {
	Path      string
	DateDir   string
	Filename  string
	ModTime   time.Time
	StartTime time.Time
}

// NewManager creates a storage manager rooted at root.
func NewManager(root string) *Manager {
	return &Manager{root: root}
}

// StreamDir returns the directory for a given stream.
func (m *Manager) StreamDir(name string) string {
	return filepath.Join(m.root, name)
}

// SegmentPath returns the full path for a segment starting at t.
func (m *Manager) SegmentPath(name string, t time.Time) string {
	dateDir := t.Format("2006-01-02")
	filename := "video_" + t.Format("15-04-05") + ".mp4"
	return filepath.Join(m.root, name, dateDir, filename)
}

// EnsureStreamDir creates the stream directory if it does not exist.
func (m *Manager) EnsureStreamDir(name string) error {
	return os.MkdirAll(m.StreamDir(name), 0755)
}

// ListSegments returns all recorded segments for a stream, sorted by modification time ascending.
func (m *Manager) ListSegments(name string) ([]Segment, error) {
	streamDir := m.StreamDir(name)
	entries, err := os.ReadDir(streamDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read stream dir: %w", err)
	}

	var segments []Segment
	for _, dateEntry := range entries {
		if !dateEntry.IsDir() {
			continue
		}
		dateDir := dateEntry.Name()
		datePath := filepath.Join(streamDir, dateDir)
		files, err := os.ReadDir(datePath)
		if err != nil {
			return nil, fmt.Errorf("read date dir %s: %w", dateDir, err)
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".mp4") {
				continue
			}
			info, err := file.Info()
			if err != nil {
				return nil, fmt.Errorf("file info %s: %w", file.Name(), err)
			}
			segments = append(segments, Segment{
				Path:     filepath.Join(datePath, file.Name()),
				DateDir:  dateDir,
				Filename: file.Name(),
				ModTime:  info.ModTime(),
			})
		}
	}

	sort.Slice(segments, func(i, j int) bool {
		return segments[i].ModTime.Before(segments[j].ModTime)
	})
	return segments, nil
}

// Cleanup removes oldest segments for a stream until at most maxFiles remain.
func (m *Manager) Cleanup(name string, maxFiles int) error {
	segments, err := m.ListSegments(name)
	if err != nil {
		return err
	}
	if len(segments) <= maxFiles {
		return nil
	}

	for _, seg := range segments[:len(segments)-maxFiles] {
		if err := os.Remove(seg.Path); err != nil {
			return fmt.Errorf("remove segment %s: %w", seg.Path, err)
		}
	}

	// Remove empty date directories.
	streamDir := m.StreamDir(name)
	entries, err := os.ReadDir(streamDir)
	if err != nil {
		return fmt.Errorf("read stream dir after cleanup: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		datePath := filepath.Join(streamDir, entry.Name())
		files, err := os.ReadDir(datePath)
		if err != nil {
			continue
		}
		if len(files) == 0 {
			_ = os.Remove(datePath)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/storage -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage
git commit -m "feat: add storage layer for segments and retention

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4: Implement ffmpeg Runner

**Files:**
- Create: `internal/ffmpeg/runner.go`
- Create: `internal/ffmpeg/runner_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/ffmpeg/runner_test.go`:

```go
package ffmpeg

import (
	"context"
	"os/exec"
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/ffmpeg -v
```

Expected: compilation errors.

- [ ] **Step 3: Implement runner package**

Create `internal/ffmpeg/runner.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/ffmpeg -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ffmpeg
git commit -m "feat: add ffmpeg command builder and runner

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5: Implement Recorder Orchestrator

**Files:**
- Create: `internal/recorder/recorder.go`
- Create: `internal/recorder/recorder_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/recorder/recorder_test.go`:

```go
package recorder

import (
	"context"
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
		{5, 60 * time.Second, 60 * time.Second},
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
	rec := New(cfg, sm)

	// Verify storage manager is set.
	if rec.storage == nil {
		t.Fatal("storage manager not set")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/recorder -v
```

Expected: compilation errors.

- [ ] **Step 3: Implement recorder package**

Create `internal/recorder/recorder.go`:

```go
package recorder

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/recorder -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/recorder
git commit -m "feat: add recorder orchestrator with reconnect and cleanup

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 6: Implement Playback HTTP Server

**Files:**
- Create: `internal/playback/server.go`
- Create: `internal/playback/embed.go`
- Create: `internal/playback/server_test.go`
- Create: `internal/playback/static/index.html`
- Create: `internal/playback/static/app.js`
- Create: `internal/playback/static/style.css`

- [ ] **Step 1: Write the failing test**

Create `internal/playback/server_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/playback -v
```

Expected: compilation errors.

- [ ] **Step 3: Implement playback server**

Create `internal/playback/server.go`:

```go
package playback

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenzhenrui/webrtc2nas/internal/config"
	"github.com/chenzhenrui/webrtc2nas/internal/storage"
)

// Server serves the playback API and static web UI.
type Server struct {
	cfg     *config.Config
	storage *storage.Manager
	mux     *http.ServeMux
}

// New creates a new playback server.
func New(cfg *config.Config, sm *storage.Manager) *Server {
	s := &Server{cfg: cfg, storage: sm}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/cameras", s.handleCameras)
	mux.HandleFunc("/api/timeline/", s.handleTimeline)
	mux.HandleFunc("/api/video/", s.handleVideo)
	mux.HandleFunc("/", s.handleStatic)
	s.mux = mux
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleCameras(w http.ResponseWriter, r *http.Request) {
	type camera struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	resp := struct {
		Cameras []camera `json:"cameras"`
	}{}
	for _, st := range s.cfg.Streams {
		enabled := true
		if st.Enabled != nil {
			enabled = *st.Enabled
		}
		resp.Cameras = append(resp.Cameras, camera{
			Name:    st.Name,
			Enabled: enabled,
		})
	}
	writeJSON(w, resp)
}

func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/timeline/")
	if name == "" {
		http.Error(w, "camera name required", http.StatusBadRequest)
		return
	}

	segments, err := s.storage.ListSegments(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type fileInfo struct {
		Date     string `json:"date"`
		Filename string `json:"filename"`
		Start    string `json:"start"`
		End      string `json:"end"`
		URL      string `json:"url"`
	}
	resp := struct {
		Camera string     `json:"camera"`
		Files  []fileInfo `json:"files"`
	}{Camera: name}

	for _, seg := range segments {
		start := seg.ModTime
		// Extract start time from filename if possible.
		if t, ok := parseSegmentTime(seg.DateDir, seg.Filename); !ok {
			// Fallback to file mod time.
		} else {
			start = t
		}
		end := start.Add(time.Duration(s.segmentMinutesFor(name)) * time.Minute)
		resp.Files = append(resp.Files, fileInfo{
			Date:     seg.DateDir,
			Filename: seg.Filename,
			Start:    start.Format(time.RFC3339),
			End:      end.Format(time.RFC3339),
			URL:      fmt.Sprintf("/api/video/%s/%s/%s", name, seg.DateDir, seg.Filename),
		})
	}
	writeJSON(w, resp)
}

func (s *Server) handleVideo(w http.ResponseWriter, r *http.Request) {
	// Path: /api/video/{camera}/{date}/{filename}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/video/"), "/")
	if len(parts) != 3 {
		http.Error(w, "invalid video path", http.StatusBadRequest)
		return
	}
	camera, date, filename := parts[0], parts[1], parts[2]
	path := filepath.Join(s.storage.StreamDir(camera), date, filename)
	http.ServeFile(w, r, path)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	data, err := staticFS.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := "application/octet-stream"
	switch filepath.Ext(path) {
	case ".html":
		contentType = "text/html"
	case ".js":
		contentType = "application/javascript"
	case ".css":
		contentType = "text/css"
	}
	w.Header().Set("Content-Type", contentType)
	w.Write(data)
}

func (s *Server) segmentMinutesFor(name string) int {
	for _, st := range s.cfg.Streams {
		if st.Name == name {
			return st.SegmentMinutes
		}
	}
	return 10
}

func parseSegmentTime(dateDir, filename string) (time.Time, bool) {
	// filename format: video_HH-MM-SS.mp4
	if !strings.HasPrefix(filename, "video_") || !strings.HasSuffix(filename, ".mp4") {
		return time.Time{}, false
	}
	timePart := strings.TrimPrefix(strings.TrimSuffix(filename, ".mp4"), "video_")
	layout := "2006-01-02_15-04-05"
	t, err := time.Parse(layout, dateDir+"_"+timePart)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
```

Create `internal/playback/embed.go`:

```go
package playback

import "embed"

//go:embed all:static
var staticFS embed.FS
```

- [ ] **Step 4: Create static UI files**

Create `internal/playback/static/index.html`:

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>webrtc2nas 回放</title>
  <link rel="stylesheet" href="/style.css">
</head>
<body>
  <header>
    <h1>webrtc2nas 回放</h1>
    <select id="cameraSelect"></select>
  </header>
  <main>
    <video id="player" controls></video>
    <div id="timeline"></div>
  </main>
  <script src="/app.js"></script>
</body>
</html>
```

Create `internal/playback/static/style.css`:

```css
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  margin: 0;
  padding: 20px;
  background: #f5f5f5;
}
header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 20px;
}
#player {
  width: 100%;
  max-width: 960px;
  background: #000;
  margin-bottom: 20px;
}
#timeline {
  background: #fff;
  padding: 16px;
  border-radius: 8px;
}
.date-group {
  margin-bottom: 16px;
}
.date-group h3 {
  margin: 0 0 8px;
}
.segment-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.segment {
  padding: 8px 12px;
  background: #e3f2fd;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
}
.segment:hover {
  background: #bbdefb;
}
```

Create `internal/playback/static/app.js`:

```javascript
const cameraSelect = document.getElementById('cameraSelect');
const player = document.getElementById('player');
const timeline = document.getElementById('timeline');

async function loadCameras() {
  const res = await fetch('/api/cameras');
  const data = await res.json();
  cameraSelect.innerHTML = '';
  data.cameras.forEach(c => {
    const opt = document.createElement('option');
    opt.value = c.name;
    opt.textContent = c.name;
    cameraSelect.appendChild(opt);
  });
  if (data.cameras.length > 0) {
    loadTimeline(data.cameras[0].name);
  }
}

async function loadTimeline(camera) {
  const res = await fetch(`/api/timeline/${camera}`);
  const data = await res.json();
  timeline.innerHTML = '';
  const groups = {};
  data.files.forEach(f => {
    if (!groups[f.date]) groups[f.date] = [];
    groups[f.date].push(f);
  });
  Object.keys(groups).sort().reverse().forEach(date => {
    const group = document.createElement('div');
    group.className = 'date-group';
    const title = document.createElement('h3');
    title.textContent = date;
    group.appendChild(title);
    const list = document.createElement('div');
    list.className = 'segment-list';
    groups[date].forEach(f => {
      const btn = document.createElement('div');
      btn.className = 'segment';
      const start = new Date(f.start);
      btn.textContent = start.toLocaleTimeString();
      btn.onclick = () => {
        player.src = f.url;
        player.play();
      };
      list.appendChild(btn);
    });
    group.appendChild(list);
    timeline.appendChild(group);
  });
}

cameraSelect.addEventListener('change', () => {
  loadTimeline(cameraSelect.value);
});

loadCameras();
```

- [ ] **Step 5: Run tests to verify they pass**

Run:

```bash
go test ./internal/playback -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/playback
git commit -m "feat: add playback HTTP server and web UI

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 7: Wire Everything in main.go

**Files:**
- Modify: `cmd/webrtc2nas/main.go`
- Create: `config.example.yaml`
- Create: `README.md`

- [ ] **Step 1: Replace main.go**

Replace contents of `cmd/webrtc2nas/main.go`:

```go
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
```

- [ ] **Step 2: Create example config**

Create `config.example.yaml`:

```yaml
output_dir: ./recordings
log_level: info
playback_host: 127.0.0.1
playback_port: 8080

streams:
  - name: living_room
    url: rtsp://homeassistant.local:8554/living_room
    max_files: 144          # 24 小时
    segment_minutes: 10
    enabled: true

  - name: bedroom
    url: rtsp://homeassistant.local:8554/bedroom
    max_files: 1008         # 7 天
    segment_minutes: 10
    enabled: true
```

- [ ] **Step 3: Create README**

Create `README.md`:

```markdown
# webrtc2nas

Mac 本地 RTSP 摄像头录制与回放工具。

## 依赖

- Go 1.22+
- ffmpeg

## 安装

```bash
go build -o webrtc2nas ./cmd/webrtc2nas
```

## 配置

复制 `config.example.yaml` 为 `config.yaml` 并按需修改。

## 运行

```bash
./webrtc2nas -config config.yaml
```

访问 http://127.0.0.1:8080 查看回放页面。
```

- [ ] **Step 4: Build and verify**

Run:

```bash
go build -o webrtc2nas ./cmd/webrtc2nas
```

Expected: clean build.

- [ ] **Step 5: Run all tests**

Run:

```bash
go test ./...
```

Expected: PASS for all packages.

- [ ] **Step 6: Commit**

```bash
git add cmd/webrtc2nas/main.go config.example.yaml README.md
git commit -m "feat: wire recorder and playback server in main

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 8: Integration Test with Local RTSP Stream

**Files:**
- Create: `scripts/integration_test.sh`
- Create: `testdata/sample.mp4` (optional, or generate during test)

- [ ] **Step 1: Create integration test script**

Create `scripts/integration_test.sh`:

```bash
#!/bin/bash
set -e

# Start a local RTSP server with a test pattern.
ffmpeg -re -f lavfi -i testsrc=duration=120:size=640x480:rate=30 \
  -f lavfi -i sine=frequency=1000:duration=120 \
  -c:v libx264 -c:a aac -f rtsp rtsp://127.0.0.1:8554/test &
RTSP_PID=$!

cleanup() {
  kill $RTSP_PID 2>/dev/null || true
}
trap cleanup EXIT

sleep 2

cat > /tmp/test-config.yaml <<EOF
output_dir: /tmp/webrtc2nas-test-recordings
log_level: debug
playback_host: 127.0.0.1
playback_port: 18080
streams:
  - name: test_cam
    url: rtsp://127.0.0.1:8554/test
    max_files: 3
    segment_minutes: 1
    enabled: true
EOF

# Run webrtc2nas for 75 seconds to generate at least one segment.
timeout 75 ./webrtc2nas -config /tmp/test-config.yaml || true

# Verify files exist.
COUNT=$(find /tmp/webrtc2nas-test-recordings/test_cam -name '*.mp4' | wc -l)
echo "Recorded $COUNT segment(s)"
if [ "$COUNT" -lt 1 ]; then
  echo "FAIL: no segments recorded"
  exit 1
fi

# Verify API works.
CAMERAS=$(curl -s http://127.0.0.1:18080/api/cameras)
echo "Cameras: $CAMERAS"

FILES=$(curl -s http://127.0.0.1:18080/api/timeline/test_cam)
echo "Timeline: $FILES"

echo "PASS"
```

- [ ] **Step 2: Make script executable and run**

Run:

```bash
chmod +x scripts/integration_test.sh
./scripts/integration_test.sh
```

Expected: script reports `PASS` and creates MP4 segments.

- [ ] **Step 3: Commit**

```bash
git add scripts/integration_test.sh
git commit -m "test: add integration test with local RTSP stream

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 9: Final Polish and Documentation

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Expand README with full usage**

Append to `README.md`:

```markdown
## 配置说明

| 字段 | 说明 |
|---|---|
| `output_dir` | 录像保存目录 |
| `log_level` | 日志级别：debug / info / warn / error |
| `playback_host` | 回放服务绑定地址，默认 127.0.0.1 |
| `playback_port` | 回放服务端口，默认 8080 |
| `streams[].name` | 摄像头名称，决定子目录名 |
| `streams[].url` | RTSP 拉流地址 |
| `streams[].max_files` | 该摄像头最多保留的片段数 |
| `streams[].segment_minutes` | 每个片段时长（分钟） |
| `streams[].enabled` | 是否启用 |

## 注意事项

- 视频使用 `-c:v copy` 直接复制，CPU 占用最低。
- 音频从 PCMA 转码为 AAC，以兼容 MP4 容器。
- 片段长度可能因关键帧位置略有偏差。
```

- [ ] **Step 2: Final test run**

Run:

```bash
go test ./...
go build -o webrtc2nas ./cmd/webrtc2nas
```

Expected: all tests pass, build succeeds.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: expand README with configuration and usage notes

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Self-Review Checklist

- [x] **Spec coverage:**
  - Config loading/validation → Task 2
  - Storage paths, segment scanning, cleanup → Task 3
  - ffmpeg command builder with copy/AAC/segment → Task 4
  - Per-stream goroutine, reconnect, cleanup timer → Task 5
  - Playback API and web UI → Task 6
  - Main wiring, graceful shutdown → Task 7
  - Integration test → Task 8
  - Documentation → Task 9
- [x] **Placeholder scan:** No TBD, TODO, or vague steps. Each step includes exact file paths and code/commands.
- [x] **Type consistency:** `StreamConfig.Enabled` is `*bool` everywhere. `Config.PlaybackHost` and `PlaybackPort` are consistent across config, playback server, and main.

---

## Notes for Implementer

- The `StreamConfig.Enabled` field uses `*bool` so YAML omission can default to `true`. Remember to dereference with `*s.Enabled`.
- Static web files must live in `internal/playback/static/` to be embedded via `//go:embed all:static`.
- The recorder cleanup ticker runs every 30 seconds. If a stream is offline, cleanup still runs until the goroutine exits on context cancellation.
- ffmpeg stderr is forwarded to `os.Stderr`. For production, consider redirecting to the logger.
- The integration test requires `ffmpeg` with `libx264` and `lavfi` support, which is standard in most ffmpeg builds.
