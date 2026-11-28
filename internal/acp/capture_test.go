package acp

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFrameLoggerCapturesBothDirections(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "capture.jsonl")
	fl, err := OpenFrameLogger(path)
	if err != nil {
		t.Fatal(err)
	}

	// Drive the incoming wrapper in 4-byte chunks so a frame spans reads.
	const incoming = `{"method":"session/new","id":"1"}` + "\n"
	r := fl.Reader(&chunkReader{data: []byte(incoming), size: 4})
	var got []byte
	for {
		buf := make([]byte, 4)
		n, err := r.Read(buf)
		got = append(got, buf[:n]...)
		if err != nil {
			break
		}
	}
	if string(got) != incoming {
		t.Fatalf("reader altered payload: %q", got)
	}

	// Two complete frames in one write; the wrapped writer must pass them
	// through untouched.
	const outgoing = `{"id":"1","result":{}}` + "\n" + `{"method":"session/update"}` + "\n"
	var sb strings.Builder
	if _, err := fl.Writer(&sb).Write([]byte(outgoing)); err != nil {
		t.Fatal(err)
	}
	if sb.String() != outgoing {
		t.Fatalf("writer altered payload: %q", sb.String())
	}

	fl.Close()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3:\n%s", len(lines), raw)
	}

	wantDirs := []string{"incoming", "outgoing", "outgoing"}
	for i, line := range lines {
		var frame map[string]any
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Fatalf("line %d not JSON: %v: %s", i, err, line)
		}
		if frame["_direction"] != wantDirs[i] {
			t.Errorf("line %d _direction = %v, want %s", i, frame["_direction"], wantDirs[i])
		}
		if frame["_ts"] == "" {
			t.Errorf("line %d missing _ts", i)
		}
	}

	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first["method"] != "session/new" || first["id"] != "1" {
		t.Errorf("frame fields not merged into line 0: %s", lines[0])
	}
}

// TestFrameLoggerKeepsUnparseableFramesRaw verifies non-JSON lines are
// captured under "raw" instead of being dropped.
func TestFrameLoggerKeepsUnparseableFramesRaw(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "capture.jsonl")
	fl, err := OpenFrameLogger(path)
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	// One valid frame, one garbage line.
	if _, err := fl.Writer(&sb).Write([]byte(`{"id":"2"}` + "\ngarbage\n")); err != nil {
		t.Fatal(err)
	}
	fl.Close()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), raw)
	}
	var second map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("line 1 not JSON: %v", err)
	}
	if second["raw"] != "garbage" {
		t.Errorf("raw = %v, want %q", second["raw"], "garbage")
	}
}

// chunkReader returns fixed-size chunks until EOF, forcing the wrapper to
// reassemble frames across reads.
type chunkReader struct {
	data  []byte
	off   int
	size  int
	ended bool
}

func (c *chunkReader) Read(p []byte) (int, error) {
	if c.ended {
		return 0, io.EOF
	}
	if c.off >= len(c.data) {
		c.ended = true
		return 0, nil
	}
	n := c.size
	if n > len(c.data)-c.off {
		n = len(c.data) - c.off
	}
	copy(p, c.data[c.off:c.off+n])
	c.off += n
	if c.off >= len(c.data) {
		c.ended = true
	}
	return n, nil
}
