// Package gateway runs a provider's traffic through the jq programs its
// configuration declares: requests and responses are rewritten in process,
// which lets a plugin adapt a provider whose wire format differs from what
// the SDK speaks without any provider-specific code in Crush.
package gateway

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/charmbracelet/crush/internal/jq"
	"github.com/charmbracelet/crush/internal/oauth"
)

// maxBody bounds a rewritten request or response body, matching what the
// SDKs themselves are willing to carry.
const maxBody = 64 << 20

// Adapter rewrites HTTP traffic through the two jq programs of a
// [oauth.GatewaySpec]. Create one per provider with [New]; the compiled
// programs are reused across requests.
type Adapter struct {
	Base     http.RoundTripper
	Token    string
	request  *jq.Program
	response *jq.Program
}

// New compiles the spec's programs and returns an adapter that forwards to
// base (http.DefaultTransport when nil), presenting token to both programs.
func New(spec *oauth.GatewaySpec, token string, base http.RoundTripper) (*Adapter, error) {
	if !spec.Usable() {
		return nil, fmt.Errorf("gateway adapter needs both a request and a response program")
	}
	reqCode, err := jq.Compile(spec.Request)
	if err != nil {
		return nil, fmt.Errorf("gateway request program: %w", err)
	}
	respCode, err := jq.Compile(spec.Response)
	if err != nil {
		return nil, fmt.Errorf("gateway response program: %w", err)
	}
	if base == nil {
		base = http.DefaultTransport
	}
	if spec.HTTP1 {
		base = forceHTTP1(base)
	}
	return &Adapter{Base: base, Token: token, request: reqCode, response: respCode}, nil
}

// forceHTTP1 returns a transport that will not negotiate HTTP/2. Go enables it
// by default against servers that offer it, and a client that impersonates a
// Node-based one is expected to speak HTTP/1.1. Only a concrete
// [*http.Transport] can be reconfigured; anything else passes through.
func forceHTTP1(base http.RoundTripper) http.RoundTripper {
	transport, ok := base.(*http.Transport)
	if !ok {
		return base
	}
	clone := transport.Clone()
	clone.ForceAttemptHTTP2 = false
	if clone.TLSClientConfig == nil {
		clone.TLSClientConfig = &tls.Config{}
	}
	clone.TLSClientConfig.NextProtos = []string{"http/1.1"}
	return clone
}

// RoundTrip implements [http.RoundTripper]: it rewrites the request through
// the request program, sends it, and rewrites the response through the
// response program — as one unit for plain bodies, and as events for
// server-sent streams.
func (a *Adapter) RoundTrip(req *http.Request) (*http.Response, error) {
	requestID, err := randomToken(12)
	if err != nil {
		return nil, err
	}

	rewritten, err := a.rewriteRequest(req, requestID)
	if err != nil {
		return nil, err
	}

	resp, err := a.Base.RoundTrip(rewritten)
	if err != nil {
		return nil, err
	}
	if err := a.rewriteResponse(resp, requestID); err != nil {
		resp.Body.Close() //nolint:errcheck // the request is failing anyway
		return nil, err
	}
	return resp, nil
}

// rewriteRequest applies the request program. The returned request shares
// nothing mutable with the original.
func (a *Adapter) rewriteRequest(req *http.Request, requestID string) (*http.Request, error) {
	raw, err := readBody(req.Body)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	// A body that is not JSON is still forwarded untouched; the program only
	// sees values it can reason about.
	body, _ := parseJSON(raw)

	input := map[string]any{
		"method":     req.Method,
		"url":        req.URL.String(),
		"headers":    firstHeaders(req.Header),
		"body":       body,
		"request_id": requestID,
		"token":      a.Token,
	}
	output, err := a.request.Run(input)
	if err != nil {
		return nil, fmt.Errorf("rewrite request: %w", err)
	}
	rules, ok := output.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("rewrite request: program output must be an object, got %T", output)
	}

	clone := req.Clone(req.Context())
	if url, ok := rules["url"].(string); ok && url != "" {
		parsed, err := parseAbsolute(url)
		if err != nil {
			return nil, fmt.Errorf("rewrite request url: %w", err)
		}
		// Clone keeps the original Host header, and some front ends route on
		// it: a rewritten URL must be addressed to the host it names.
		clone.Host = ""
		clone.URL = parsed
	}
	if headers, ok := rules["headers"].(map[string]any); ok {
		for key, value := range headers {
			if text, ok := value.(string); ok {
				clone.Header.Set(key, text)
			}
		}
	}
	if drops, ok := rules["drop_headers"].([]any); ok {
		for _, key := range drops {
			if text, ok := key.(string); ok {
				clone.Header.Del(text)
			}
		}
	}
	sent := raw
	if replacement, present := rules["body"]; present && replacement != nil {
		encoded, err := json.Marshal(replacement)
		if err != nil {
			return nil, fmt.Errorf("encode rewritten body: %w", err)
		}
		sent = encoded
		if clone.Header.Get("Content-Type") == "" {
			clone.Header.Set("Content-Type", "application/json")
		}
	}
	// Reading the body consumed it, so whatever is sent has to be put back,
	// rewritten or not.
	clone.Body = io.NopCloser(bytes.NewReader(sent))
	clone.ContentLength = int64(len(sent))
	if len(sent) == 0 {
		clone.Body = http.NoBody
	}
	return clone, nil
}

func parseAbsolute(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw) //nolint:varnamelen // matches the surrounding style
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%q is not an http URL", raw)
	}
	return parsed, nil
}

// rewriteResponse applies the response program to the reply: once for a plain
// body, per event for a stream. Errors leave the response unreadable, so
// callers fail the request rather than serve a half-rewritten reply.
func (a *Adapter) rewriteResponse(resp *http.Response, requestID string) error {
	base := map[string]any{
		"status":     resp.StatusCode,
		"headers":    firstHeaders(resp.Header),
		"request_id": requestID,
		"token":      a.Token,
	}

	if isEventStream(resp) {
		resp.Body = &eventConverter{
			base:     base,
			program:  a.response,
			upstream: resp.Body,
		}
		// The rewritten stream has its own framing; a stale length would
		// truncate it.
		resp.ContentLength = -1
		resp.Header.Del("Content-Length")
		return nil
	}

	raw, err := readBody(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	body, _ := parseJSON(raw)
	input := mapsWith(base, map[string]any{"body": body})
	output, err := a.response.Run(input)
	if err != nil {
		return fmt.Errorf("rewrite response: %w", err)
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("encode rewritten response: %w", err)
	}
	resp.Body = io.NopCloser(bytes.NewReader(encoded))
	resp.ContentLength = int64(len(encoded))
	resp.Header.Del("Content-Length")
	return nil
}

// eventConverter rewrites a server-sent event stream event by event. The
// last event is held back until the upstream ends, because the program cannot
// know an event is final until nothing follows it; a stream through a gateway
// therefore lags the upstream by one event.
type eventConverter struct {
	base     map[string]any
	program  *jq.Program
	upstream io.ReadCloser
	buffered *bufio.Reader
	pending  bytes.Buffer
	done     bool
}

// Read implements [io.Reader], producing rewritten events on demand.
func (c *eventConverter) Read(p []byte) (int, error) {
	for c.pending.Len() == 0 {
		if c.done {
			return 0, io.EOF
		}
		if err := c.nextEvent(); err != nil {
			return 0, err
		}
	}
	return c.pending.Read(p)
}

// Close implements [io.Closer].
func (c *eventConverter) Close() error {
	return c.upstream.Close() //nolint:errcheck // closing a finished stream has nothing to report
}

// nextEvent reads one upstream event, rewrites it, and queues the result.
func (c *eventConverter) nextEvent() error {
	name, data, err := readEvent(c.reader())
	switch {
	case err != nil && err != io.EOF:
		return err
	case data == "" && err == io.EOF:
		c.done = true
		return nil
	case data == "":
		// A blank-line run between events; nothing to rewrite.
		return nil
	}

	// An event is final only when nothing follows it, so one byte of
	// lookahead decides: the event is held back until the next byte arrives
	// (or the upstream ends), which is the single event of lag a rewritten
	// stream carries.
	atEOF := err == io.EOF
	if !atEOF {
		if _, peekErr := c.reader().Peek(1); peekErr == io.EOF {
			atEOF = true
		}
	}
	if atEOF {
		c.done = true
	}

	payload, ok := parseJSON([]byte(data))
	if !ok {
		// Comments, keepalives, and any non-JSON data pass through with their
		// framing intact; the program only sees events it can parse.
		writeEvent(&c.pending, name, data)
		return nil
	}

	input := map[string]any{
		"event":      true,
		"status":     c.base["status"],
		"headers":    c.base["headers"],
		"data":       payload,
		"final":      atEOF,
		"request_id": c.base["request_id"],
		"token":      c.base["token"],
	}
	output, err := c.program.Run(input)
	if err != nil {
		return fmt.Errorf("rewrite stream event: %w", err)
	}
	events, ok := output.([]any)
	if !ok {
		return fmt.Errorf("rewrite stream event: program output must be an array of events, got %T", output)
	}
	for _, event := range events {
		encoded, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("encode rewritten event: %w", err)
		}
		writeEvent(&c.pending, name, string(encoded))
	}
	return nil
}

// writeEvent queues one server-sent event. The event name is framing, not data
// the program rewrites, so it is carried over verbatim: a provider whose
// events are named (Anthropic's message_start, message_delta, ...) dispatches
// on it, and dropping it would leave the stream unreadable.
func writeEvent(w *bytes.Buffer, name, data string) {
	if name != "" {
		w.WriteString("event: ")
		w.WriteString(name)
		w.WriteString("\n")
	}
	w.WriteString("data: ")
	w.WriteString(data)
	w.WriteString("\n\n")
}

func (c *eventConverter) reader() *bufio.Reader {
	if c.buffered == nil {
		c.buffered = bufio.NewReader(c.upstream)
	}
	return c.buffered
}

// readEvent reads one SSE event: its name, from an event: line, and its data,
// the accumulated data lines of a blank-line-terminated block. io.EOF is
// returned with the final block when the stream ends inside one.
func readEvent(r *bufio.Reader) (name, data string, err error) {
	var dataLines strings.Builder
	for {
		line, readErr := r.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		switch {
		case trimmed == "" && readErr == nil:
			if dataLines.Len() > 0 {
				return name, strings.TrimSuffix(dataLines.String(), "\n"), nil
			}
			// Blank line outside an event: keep reading.
		case strings.HasPrefix(trimmed, ":"):
			// Comment line; ignored.
		case strings.HasPrefix(trimmed, "event:"):
			name = strings.TrimPrefix(strings.TrimPrefix(trimmed, "event:"), " ")
		case strings.HasPrefix(trimmed, "data:"):
			// SSE allows one optional space after the prefix, and multiple
			// data lines join with newlines.
			dataLines.WriteString(strings.TrimPrefix(strings.TrimPrefix(trimmed, "data:"), " "))
			dataLines.WriteString("\n")
		}
		if readErr != nil {
			if readErr == io.EOF && dataLines.Len() > 0 {
				return name, strings.TrimSuffix(dataLines.String(), "\n"), io.EOF
			}
			return name, "", readErr
		}
	}
}

// firstHeaders flattens a header set to one value per key, the shape the
// programs work with. The values are boxed as any so a program can iterate
// the map: jq understands map[string]any, not the map[string]string Go
// headers natively are.
func firstHeaders(h http.Header) map[string]any {
	flat := make(map[string]any, len(h))
	for key, values := range h {
		if len(values) > 0 {
			flat[key] = values[0]
		}
	}
	return flat
}

// mapsWith merges two maps for a program input.
func mapsWith(base, extra map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(extra))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range extra {
		merged[k] = v
	}
	return merged
}

// readBody reads a body whole, bounded by maxBody.
func readBody(body io.Reader) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	return io.ReadAll(io.LimitReader(body, maxBody))
}

// parseJSON reports whether raw parses as JSON, returning the value.
func parseJSON(raw []byte) (any, bool) {
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

func isEventStream(resp *http.Response) bool {
	return strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream")
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
