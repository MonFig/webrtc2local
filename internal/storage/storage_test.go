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
