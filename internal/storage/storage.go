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
