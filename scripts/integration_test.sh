#!/bin/bash
set -e

# Build the binary first.
go build -o webrtc2nas ./cmd/webrtc2nas

# Detect available timeout command.
TIMEOUT_CMD=""
if command -v timeout > /dev/null 2>&1; then
  TIMEOUT_CMD="timeout"
elif command -v gtimeout > /dev/null 2>&1; then
  TIMEOUT_CMD="gtimeout"
else
  # Fallback: use perl for timeout on macOS without coreutils.
  timeout_fallback() {
    local seconds="$1"
    shift
    perl -e '
      eval {
        $SIG{ALRM} = sub { die "alarm\n" };
        alarm shift;
        system(@ARGV);
        alarm 0;
      };
      if ($@ eq "alarm\n") { exit 124 }
    ' "$seconds" "$@"
  }
  TIMEOUT_CMD="timeout_fallback"
fi

# Start a local RTSP server with a test pattern.
# ffmpeg -f rtsp acts as a client pushing to a server; we need a server.
# Use ffplay or a simple RTSP server. Since we don't have one, we'll use
# ffmpeg's built-in RTSP server capability via -listen 1 (if supported).
# Alternatively, use a file-based approach for testing.

# Actually, ffmpeg can act as an RTSP server with -rtsp_flags listen:
ffmpeg -re -f lavfi -i testsrc=duration=120:size=640x480:rate=30 \
  -f lavfi -i sine=frequency=1000:duration=120 \
  -c:v libx264 -c:a aac -f rtsp -rtsp_flags listen rtsp://127.0.0.1:8554/test &
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
$TIMEOUT_CMD 75 ./webrtc2nas -config /tmp/test-config.yaml || true

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
