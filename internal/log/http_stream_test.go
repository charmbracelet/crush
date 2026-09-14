package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Draining the body to log it waits for the stream to end, which turns every
// stream into one blocking read.
func TestHTTPLoggerDoesNotBufferStreamingResponses(t *testing.T) {
	t.Parallel()

	slog.SetLogLoggerLevel(slog.LevelDebug)

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "event: ping\ndata: {}\n\n")
		w.(http.Flusher).Flush()
		// Hold the response open, the way a provider does mid-turn.
		<-release
	}))
	defer srv.Close()
	defer close(release)

	client := NewHTTPClient()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	first := make(chan string, 1)
	go func() {
		buf := make([]byte, 64)
		n, _ := resp.Body.Read(buf)
		first <- string(buf[:n])
	}()

	select {
	case got := <-first:
		require.Contains(t, got, "ping", "the first event should arrive before the response ends")
	case <-time.After(2 * time.Second):
		t.Fatal("the first streamed event never arrived: the logger buffered the whole body")
	}
}

// The tail is what matters when diagnosing a stall, so the head is dropped.
func TestRecordingBodyKeepsTail(t *testing.T) {
	t.Parallel()

	var body string
	var truncated bool
	r := &recordingBody{
		inner: io.NopCloser(io.LimitReader(repeatReader{'a'}, maxRecordedBodyBytes+1024)),
		log:   func(b string, tr bool) { body, truncated = b, tr },
	}
	_, err := io.Copy(io.Discard, r)
	require.NoError(t, err)
	require.NoError(t, r.Close())

	require.True(t, truncated, "an over-long body should report truncation")
	require.Len(t, body, maxRecordedBodyBytes)
}

// Closing early (a cancelled turn) must still produce exactly one log line.
func TestRecordingBodyLogsOnceOnEarlyClose(t *testing.T) {
	t.Parallel()

	var calls int
	r := &recordingBody{
		inner: io.NopCloser(io.LimitReader(repeatReader{'x'}, 1<<20)),
		log:   func(string, bool) { calls++ },
	}
	buf := make([]byte, 16)
	_, err := r.Read(buf)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	require.NoError(t, r.Close())
	require.Equal(t, 1, calls)
}

type repeatReader struct{ b byte }

func (r repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.b
	}
	return len(p), nil
}
