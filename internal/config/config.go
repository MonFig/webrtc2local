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
