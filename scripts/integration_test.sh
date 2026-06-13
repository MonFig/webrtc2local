#!/bin/bash
set -e

# Build the binary first.
go build -o webrtc2nas ./cmd/webrtc2nas

# Clean any previous test output.
rm -rf /tmp/webrtc2nas-test-recordings

# Pre-create fake recorded segments to test storage + playback API.
mkdir -p /tmp/webrtc2nas-test-recordings/test_cam/2026-06-14
ffmpeg -f lavfi -i testsrc=duration=5:size=640x480:rate=30 -c:v libx264 -pix_fmt yuv420p /tmp/webrtc2nas-test-recordings/test_cam/2026-06-14/video_09-00-00.mp4 -y 2>/dev/null
ffmpeg -f lavfi -i testsrc=duration=5:size=640x480:rate=30 -c:v libx264 -pix_fmt yuv420p /tmp/webrtc2nas-test-recordings/test_cam/2026-06-14/video_09-10-00.mp4 -y 2>/dev/null

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

# Start webrtc2nas in background. It will fail to connect to RTSP but playback API works.
./webrtc2nas -config /tmp/test-config.yaml &
APP_PID=$!

cleanup() {
  kill $APP_PID 2>/dev/null || true
}
trap cleanup EXIT

# Wait for HTTP server to start.
sleep 2

# Verify files exist.
COUNT=$(find /tmp/webrtc2nas-test-recordings/test_cam -name '*.mp4' | wc -l)
echo "Recorded $COUNT segment(s)"
if [ "$COUNT" -lt 1 ]; then
  echo "FAIL: no segments found"
  exit 1
fi

# Verify API works.
CAMERAS=$(curl -s http://127.0.0.1:18080/api/cameras)
echo "Cameras: $CAMERAS"

FILES=$(curl -s http://127.0.0.1:18080/api/timeline/test_cam)
echo "Timeline: $FILES"

# Verify the timeline contains our fake segments.
if ! echo "$FILES" | grep -q "video_09-00-00.mp4"; then
  echo "FAIL: expected segment not in timeline"
  exit 1
fi

echo "PASS"
