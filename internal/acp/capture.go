package acp

// FrameLogger captures JSON-RPC frames on the ACP stdio transport to a
// timestamped JSONL file, one line per frame in both directions. Enabled by
// the CRUSH_ACP_LOG env var in runACP.

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

type FrameLogger struct {
	f  *os.File
	mu sync.Mutex
}

// OpenFrameLogger creates the capture file at path.
func OpenFrameLogger(path string) (*FrameLogger, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &FrameLogger{f: f}, nil
}

// Close shuts the capture file.
func (l *FrameLogger) Close() { l.f.Close() }

// Reader wraps r, logging each complete frame it passes through.
func (l *FrameLogger) Reader(r io.Reader) io.Reader {
	return &readFrame{log: l, r: r}
}

type readFrame struct {
	log *FrameLogger
	r   io.Reader
	buf []byte // pending partial frames on the incoming side
}

func (rf *readFrame) Read(p []byte) (int, error) {
	n, err := rf.r.Read(p)
	rf.log.accumulate(&rf.buf, p[:n], "incoming")
	return n, err
}

// Writer wraps w, logging each complete frame written to it.
func (l *FrameLogger) Writer(w io.Writer) io.Writer {
	return &writeFrame{log: l, w: w}
}

type writeFrame struct {
	log *FrameLogger
	w   io.Writer
	buf []byte // pending partial frames on the outgoing side
}

func (wf *writeFrame) Write(p []byte) (int, error) {
	n, err := wf.w.Write(p)
	if err == nil {
		wf.log.accumulate(&wf.buf, p, "outgoing")
	}
	return n, err
}

// accumulate splits newline-delimited input into complete frames and
// records each with a timestamp. Frames are NDJSON: one JSON object per
// line. Each direction keeps its own pending buffer, guarded by mu.
func (l *FrameLogger) accumulate(buf *[]byte, data []byte, direction string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	*buf = append(*buf, data...)
	for {
		i := bytes.IndexByte(*buf, '\n')
		if i < 0 {
			break
		}
		line := (*buf)[:i]
		(*buf) = (*buf)[i+1:]
		l.record(line, direction)
	}
}

// record writes one captured frame as a JSONL line with _ts and _direction
// merged into the frame's fields.
// record writes one captured frame as a JSONL line with _ts and _direction
// merged into the frame's fields. Unparseable frames are kept verbatim
// under "raw".
func (l *FrameLogger) record(line []byte, direction string) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}
	obj := map[string]any{
		"_ts":        time.Now().UTC().Format(time.RFC3339Nano),
		"_direction": direction,
		"raw":        string(line),
	}
	if json.Unmarshal(line, &obj) == nil {
		delete(obj, "raw")
	}
	out, _ := json.Marshal(obj)
	l.f.Write(append(out, '\n'))
}
