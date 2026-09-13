package mcp

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The standalone SSE stream is the hanging GET that a streamable-HTTP MCP
// server uses to push server-initiated notifications. For channel-enabled
// servers it is the stream that notifications/claude/channel doorbells ride.
//
// Interception note: channel notifications must be filtered below the go-sdk
// connection layer for streamable-HTTP transports. The SDK only starts the
// standalone SSE stream by type-asserting the connection returned from
// Transport.Connect to its internal connection type; wrapping the connection
// (as channelConn does for other transports) silently defeats that assert,
// so the stream is never opened and every doorbell is rejected server-side
// with "stream not connected or already closed".

// installChannelSSEFilter wires the SSE-filtering round-tripper into a
// streamable-HTTP transport.
func installChannelSSEFilter(t *mcp.StreamableClientTransport, name string, gate *channelGate) {
	inner := http.RoundTripper(http.DefaultTransport)
	if t.HTTPClient != nil && t.HTTPClient.Transport != nil {
		inner = t.HTTPClient.Transport
	}
	t.HTTPClient = &http.Client{Transport: &channelSSEFilter{
		inner: inner,
		name:  name,
		gate:  gate,
	}}
}

// channelSSEFilter is an http.RoundTripper that watches event-stream
// response bodies. It filters notifications/claude/channel events out of
// every stream (doorbells may ride any SSE body, in practice the standalone
// GET) and dispatches them through the channel gate.
type channelSSEFilter struct {
	inner http.RoundTripper
	name  string
	gate  *channelGate
}

func (f *channelSSEFilter) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := f.inner.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	if resp.StatusCode != http.StatusOK ||
		!isEventStreamContentType(resp.Header.Get("Content-Type")) {
		return resp, nil
	}
	resp.Body = &channelSSEBody{
		ctx:    req.Context(),
		body:   resp.Body,
		filter: f,
	}
	return resp, nil
}

func isEventStreamContentType(ct string) bool {
	const prefix = "text/event-stream"
	if len(ct) < len(prefix) {
		return false
	}
	ct = ct[:len(prefix)]
	for i := 0; i < len(ct); i++ {
		c := ct[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != prefix[i] {
			return false
		}
	}
	return true
}

// maxUnboundedBuffer caps how many bytes are held waiting for an event
// boundary. A doorbell is capped far below this (parseChannelParams enforces
// maxChannelContentBytes), so anything this large is not a channel event and
// is forwarded to the SDK untouched rather than buffered forever.
const maxUnboundedBuffer = 1 << 20

// channelSSEBody filters one event-stream body. Complete events are checked
// for the channel notification method; matching events are dispatched to the
// gate and stripped, everything else passes through unchanged and in order.
type channelSSEBody struct {
	ctx    context.Context
	body   io.ReadCloser
	filter *channelSSEFilter

	out     bytes.Buffer // filtered bytes not yet consumed by the reader
	buf     []byte       // raw bytes awaiting an event boundary
	eof     bool
	readErr error
}

// Read returns filtered stream data, blocking on the underlying body only
// when no filtered bytes are available.
func (b *channelSSEBody) Read(p []byte) (int, error) {
	for b.out.Len() == 0 {
		if b.eof {
			if b.readErr != nil {
				return 0, b.readErr
			}
			return 0, io.EOF
		}
		if err := b.pump(); err != nil {
			return 0, err
		}
	}
	return b.out.Read(p)
}

// pump reads more raw bytes from the stream, extracts any complete events,
// and appends the filtered result to the output buffer.
func (b *channelSSEBody) pump() error {
	chunk := make([]byte, 4096)
	n, err := b.body.Read(chunk)
	if n > 0 {
		b.buf = append(b.buf, chunk[:n]...)
		b.extractEvents()
	}
	if err == nil {
		return nil
	}
	// Stream ended: flush anything left (a trailing event without a final
	// blank line) before propagating EOF/error to the reader.
	b.out.Write(b.buf)
	b.buf = nil
	b.eof = true
	b.readErr = err
	return nil
}

// extractEvents drains complete events from buf. An SSE event ends at a
// blank line; both \n\n and \r\n\r\n are accepted as delimiters. If buf
// grows past maxUnboundedBuffer without a boundary the data is forwarded
// as-is: it cannot be a valid doorbell (those are size-capped) and holding
// it would stall the stream.
func (b *channelSSEBody) extractEvents() {
	for len(b.buf) > 0 {
		end := eventBoundary(b.buf)
		if end < 0 {
			if len(b.buf) > maxUnboundedBuffer {
				b.out.Write(b.buf)
				b.buf = nil
			}
			return
		}
		event := b.buf[:end]
		b.buf = b.buf[end:]
		b.handleEvent(event)
	}
}

// eventBoundary returns the length of the first event in data (including the
// terminating blank line), or -1 if no complete event is buffered.
func eventBoundary(data []byte) int {
	if i := bytes.Index(data, []byte("\r\n\r\n")); i >= 0 {
		return i + 4
	}
	if i := bytes.Index(data, []byte("\n\n")); i >= 0 {
		return i + 2
	}
	return -1
}

// handleEvent inspects one complete SSE event. Events whose data decodes to
// a notifications/claude/channel request are dispatched through the channel
// gate and dropped from the stream; all other events pass through to the
// SDK untouched.
func (b *channelSSEBody) handleEvent(event []byte) {
	raw := eventData(event)
	if raw == nil || !utf8.Valid(raw) || !bytes.Contains(raw, []byte(channelNotificationMethod)) {
		b.out.Write(event)
		return
	}
	msg, err := jsonrpc.DecodeMessage(raw)
	if err != nil {
		b.out.Write(event)
		return
	}
	req, ok := msg.(*jsonrpc.Request)
	if !ok || req.IsCall() || req.Method != channelNotificationMethod {
		b.out.Write(event)
		return
	}
	if parsed := b.filter.gate.accept(req.Params); parsed != nil {
		publishChannelMessage(b.ctx, b.filter.name, parsed)
	}
}

// eventData joins the data lines of an SSE event into the JSON-RPC payload.
// Returns nil for comments (keepalives) and events without data.
func eventData(event []byte) []byte {
	var lines [][]byte
	for _, line := range bytes.Split(event, []byte("\n")) {
		line = bytes.TrimSuffix(line, []byte("\r"))
		if len(line) == 0 || line[0] == ':' {
			continue
		}
		name, value, found := bytes.Cut(line, []byte(":"))
		if !found || string(name) != "data" {
			continue
		}
		value = bytes.TrimPrefix(value, []byte(" "))
		lines = append(lines, value)
	}
	if len(lines) == 0 {
		return nil
	}
	return bytes.Join(lines, []byte("\n"))
}

// Close implements io.Closer.
func (b *channelSSEBody) Close() error {
	return b.body.Close()
}
