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
		if t, ok := parseSegmentTime(seg.DateDir, seg.Filename); ok {
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
	for _, part := range []string{camera, date, filename} {
		if part == "" || part == "." || part == ".." || strings.Contains(part, "/") || strings.Contains(part, "\\") {
			http.Error(w, "invalid video path", http.StatusBadRequest)
			return
		}
	}
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
