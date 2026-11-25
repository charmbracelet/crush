package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// seenRequest is what a stub gateway received.
type seenRequest struct {
	req  *http.Request
	body []byte
}

// echoServer answers with body and reports the request over a channel, so the
// test observes it without racing the server goroutine.
func echoServer(t *testing.T, body string, contentType string) (*httptest.Server, <-chan seenRequest) {
	t.Helper()
	requests := make(chan seenRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		select {
		case requests <- seenRequest{req: r.Clone(r.Context()), body: raw}:
		default:
		}
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		io.WriteString(w, body) //nolint:errcheck // test stub
	}))
	t.Cleanup(server.Close)
	return server, requests
}

func TestRequestRewrite(t *testing.T) {
	t.Parallel()

	server, requests := echoServer(t, `{"ok":true}`, "application/json")

	adapter, err := New(&oauth.GatewaySpec{
		Request: `
			{
				url: "` + server.URL + `/gateway:generate?alt=sse",
				headers: {Authorization: ("Bearer " + .token), "X-Added": "yes"},
				drop_headers: ["x-goog-api-key"],
				body: {model: .body.model, request: .body, id: .request_id}
			}`,
		Response: `.body`,
	}, "at-123", nil)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://sdk.example.com/v1beta/models/m:generateContent",
		strings.NewReader(`{"model":"m","contents":[]}`))
	require.NoError(t, err)
	req.Header.Set("x-goog-api-key", "at-123")
	req.Header.Set("X-Original", "kept")

	client := &http.Client{Transport: adapter}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	seen := <-requests
	raw := seen.body
	require.Equal(t, "/gateway:generate", seen.req.URL.Path)
	require.Equal(t, "alt=sse", seen.req.URL.RawQuery)
	require.Equal(t, "Bearer at-123", seen.req.Header.Get("Authorization"))
	require.Equal(t, "yes", seen.req.Header.Get("X-Added"))
	require.Empty(t, seen.req.Header.Get("x-goog-api-key"), "a dropped header must not reach the gateway")
	require.Equal(t, "kept", seen.req.Header.Get("X-Original"))
	// The rewritten URL is addressed to its own host, not the SDK's.
	require.Equal(t, seen.req.Host, strings.TrimPrefix(server.URL, "http://"))

	var sent map[string]any
	require.NoError(t, json.Unmarshal(raw, &sent))
	require.Equal(t, "m", sent["model"])
	require.NotEmpty(t, sent["id"], "the program can label a request with the id it was given")
	require.NotNil(t, sent["request"])
}

func TestResponseRewriteUnary(t *testing.T) {
	t.Parallel()

	server, _ := echoServer(t, `{"response":{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}}`, "application/json")

	adapter, err := New(&oauth.GatewaySpec{
		Request:  `{}`,
		Response: `(.body.response // .body) | .usageMetadata //= {}`,
	}, "", nil)
	require.NoError(t, err)

	resp, err := postEmpty(t, adapter, server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// The envelope is gone and the field a mapper would dereference exists.
	require.JSONEq(t, `{"candidates":[{"content":{"parts":[{"text":"hi"}]}}],"usageMetadata":{}}`, string(body))
}

func TestStreamRewrite(t *testing.T) {
	t.Parallel()

	upstream := `data: {"response":{"candidates":[{"content":{"parts":[{"text":"a"}]}}],"usageMetadata":{"totalTokenCount":5}}}

data: {"response":{"candidates":[{"content":{"parts":[{"text":"b"}]},"finishReason":"STOP"}],"usageMetadata":{"totalTokenCount":9}}}

`
	server, _ := echoServer(t, upstream, "text/event-stream")

	// The program unwraps, keeps usage only where the turn ends, and appends
	// a closing event when the backend omitted one.
	adapter, err := New(&oauth.GatewaySpec{
		Request: `{}`,
		Response: `
			if .event then
				(.data.response // .data) as $r
				| (if ($r.candidates[0].finishReason != null) or .final
					then $r else ($r | del(.usageMetadata)) end) as $r2
				| if .final and ($r2.candidates[0].finishReason == null)
					then [$r2, {candidates:[{finishReason:"STOP"}]}]
					else [$r2] end
			else .body end`,
	}, "", nil)
	require.NoError(t, err)

	resp, err := postEmpty(t, adapter, server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	events := parseEvents(t, string(body))
	require.Len(t, events, 2, "each upstream event yields one rewritten event")

	// The first chunk is not the end of the turn, so its cumulative usage is
	// withheld: crush sums token counts across chunks.
	require.Nil(t, events[0]["usageMetadata"])
	require.Equal(t, "a", firstText(t, events[0]))

	// The final chunk keeps its usage and the turn's finish.
	require.NotNil(t, events[1]["usageMetadata"])
	require.Equal(t, "STOP", events[1]["candidates"].([]any)[0].(map[string]any)["finishReason"])
}

func TestStreamSynthesizesMissingFinish(t *testing.T) {
	t.Parallel()

	server, _ := echoServer(t, `data: {"response":{"candidates":[{"content":{"parts":[{"text":"hi"}]}}],"usageMetadata":{"totalTokenCount":7}}}`+"\n\n", "text/event-stream")

	adapter, err := New(&oauth.GatewaySpec{
		Request: `{}`,
		Response: `
			if .event then
				(.data.response // .data) as $r
				| (if ($r.candidates[0].finishReason != null) or .final
					then $r else ($r | del(.usageMetadata)) end) as $r2
				| if .final and ($r2.candidates[0].finishReason == null)
					then [$r2, {candidates:[{finishReason:"STOP"}]}]
					else [$r2] end
			else .body end`,
	}, "", nil)
	require.NoError(t, err)

	resp, err := postEmpty(t, adapter, server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	events := parseEvents(t, string(body))
	require.Len(t, events, 2, "a stream that ends without a finish reason gets one appended")
	require.NotNil(t, events[0]["usageMetadata"], "the last real chunk keeps the usage")
	require.Equal(t, "STOP", events[1]["candidates"].([]any)[0].(map[string]any)["finishReason"])
}

func TestStreamPassesNonJSONThrough(t *testing.T) {
	t.Parallel()

	server, _ := echoServer(t, ": keepalive\n\ndata: not json\n\ndata: {\"ok\":true}\n\n", "text/event-stream")

	adapter, err := New(&oauth.GatewaySpec{
		Request:  `{}`,
		Response: `if .event then [.data] else .body end`,
	}, "", nil)
	require.NoError(t, err)

	resp, err := postEmpty(t, adapter, server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// The unparsable event survives verbatim rather than being dropped or
	// failing the stream.
	require.Contains(t, string(body), "data: not json")
	require.Contains(t, string(body), `{"ok":true}`)
}

func TestAdapterRejectsBadPrograms(t *testing.T) {
	t.Parallel()

	_, err := New(&oauth.GatewaySpec{Request: `{`, Response: `.`}, "", nil)
	require.ErrorContains(t, err, "gateway request program")

	_, err = New(&oauth.GatewaySpec{Request: `.`, Response: `if`}, "", nil)
	require.ErrorContains(t, err, "gateway response program")

	_, err = New(&oauth.GatewaySpec{Request: `.`}, "", nil)
	require.ErrorContains(t, err, "needs both")
}

// A provider whose events are named (Anthropic's message_start, message_delta,
// ...) has its reader dispatch on the event: line, so a rewritten stream must
// carry it through. The name is framing, not data the program rewrites.
func TestStreamPreservesEventNames(t *testing.T) {
	t.Parallel()

	upstream := "event: message_start\n" + `data: {"type":"message_start"}` + "\n\n" +
		"event: content_block_delta\n" + `data: {"type":"content_block_delta","delta":{"text":"hi"}}` + "\n\n" +
		"event: message_stop\n" + `data: {"type":"message_stop"}` + "\n\n"
	server, _ := echoServer(t, upstream, "text/event-stream")

	adapter, err := New(&oauth.GatewaySpec{
		Request:  `{}`,
		Response: `if .event then [.data] else .body end`,
	}, "", nil)
	require.NoError(t, err)

	resp, err := postEmpty(t, adapter, server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// The data is re-encoded (and its keys reordered), but every event keeps
	// the name the reader dispatches on.
	for _, name := range []string{"message_start", "content_block_delta", "message_stop"} {
		require.Contains(t, string(body), "event: "+name+"\n", "the event name survives the rewrite")
	}
	require.Contains(t, string(body), `"delta":{"text":"hi"}`)
}

// sha256hex and uuid are the functions the adapter adds to jq, and the headers
// a program reads must be iterable: Go's map[string]string would panic in jq.
func TestProgramFunctions(t *testing.T) {
	t.Parallel()

	server, requests := echoServer(t, `{}`, "application/json")
	adapter, err := New(&oauth.GatewaySpec{
		Request: `{headers: {
			"X-Sig": sha256hex(.body.text),
			"X-Id": uuid,
			"X-Seen": (.headers | to_entries | map(.key) | join(","))
		}}`,
		Response: `.body`,
	}, "", nil)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, strings.NewReader(`{"text":"hello"}`))
	require.NoError(t, err)
	req.Header.Set("X-Existing", "v")
	resp, err := (&http.Client{Transport: adapter}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	seen := <-requests
	require.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", seen.req.Header.Get("X-Sig"))
	require.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, seen.req.Header.Get("X-Id"))
	require.Contains(t, seen.req.Header.Get("X-Seen"), "X-Existing")
}

func TestNewReusesTransportAndPrograms(t *testing.T) {
	t.Parallel()

	spec := &oauth.GatewaySpec{Request: `{}`, Response: `.body`, HTTP1: true}
	first, err := New(spec, "token 1", nil)
	require.NoError(t, err)
	second, err := New(spec, "token 2", http.DefaultTransport)
	require.NoError(t, err)
	require.Same(t, http.DefaultTransport, first.Base, "HTTP1 is the caller's base to choose")
	require.Same(t, first.Base, second.Base)
	require.Same(t, first.request, second.request)
	require.Same(t, first.response, second.response)

	swapped, err := New(&oauth.GatewaySpec{Request: spec.Response, Response: spec.Request}, "", nil)
	require.NoError(t, err)
	require.Same(t, first.request, swapped.response)
	require.Same(t, first.response, swapped.request)
}

func TestRewrittenBodyOwnershipAndReplay(t *testing.T) {
	t.Parallel()

	original := &trackedBody{Reader: strings.NewReader(`{"original":true}`)}
	upstream := &trackedBody{Reader: strings.NewReader(`{"response":true}`)}
	adapter := newStubAdapter(t, `{body: {rewritten: true}}`, `.body`, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, 1, original.closes)
		defer req.Body.Close()
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"rewritten":true}`, string(body))
		require.EqualValues(t, len(body), req.ContentLength)

		replay, err := req.GetBody()
		require.NoError(t, err)
		defer replay.Close()
		replayed, err := io.ReadAll(replay)
		require.NoError(t, err)
		require.Equal(t, body, replayed)
		return &http.Response{StatusCode: http.StatusOK, Body: upstream}, nil
	}))
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://provider.example", strings.NewReader(`{"original":true}`))
	require.NoError(t, err)
	req.Body = original
	resp, err := adapter.RoundTrip(req)
	require.NoError(t, err)
	require.Equal(t, 1, upstream.closes)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, 1, upstream.closes)
}

func TestBodyClosedOnFailure(t *testing.T) {
	t.Parallel()

	readErr := errors.New("body failure")
	tests := []struct {
		name    string
		request bool
		body    io.Reader
		want    string
	}{
		{name: "request read failure", request: true, body: iotest.ErrReader(readErr), want: "read request body: body failure"},
		{name: "response read failure", body: iotest.ErrReader(readErr), want: "read response body: body failure"},
		{name: "oversized request", request: true, body: &io.LimitedReader{R: zeroReader{}, N: maxBody + 1}, want: "read request body: body exceeds"},
		{name: "oversized response", body: &io.LimitedReader{R: zeroReader{}, N: maxBody + 1}, want: "read response body: body exceeds"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := &trackedBody{Reader: tt.body}
			adapter := newStubAdapter(t, `{}`, `.body`, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				defer req.Body.Close()
				require.False(t, tt.request, "invalid request bodies must not be sent")
				return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
			}))
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://provider.example", strings.NewReader(`{}`))
			require.NoError(t, err)
			if tt.request {
				req.Body = body
			}
			resp, err := adapter.RoundTrip(req)
			if resp != nil {
				defer resp.Body.Close()
			}
			require.ErrorContains(t, err, tt.want)
			require.Equal(t, 1, body.closes)
		})
	}
}

func TestStreamTransfersBodyOwnership(t *testing.T) {
	t.Parallel()

	upstream := &trackedBody{Reader: strings.NewReader("data: 1\n\n")}
	resp := stubGateway(t, `{}`, `[.data]`, "text/event-stream", upstream)
	require.Zero(t, upstream.closes)
	_, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Zero(t, upstream.closes)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, 1, upstream.closes)
}

func TestGatewayJQCancellation(t *testing.T) {
	t.Parallel()

	for _, stage := range []string{"request", "response", "stream event"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			request, response := `{}`, `.body`
			if stage == "request" {
				request = `{headers: {"X-Rewritten": "yes"}}`
				cancel()
			}
			adapter := newStubAdapter(t, request, response, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				defer req.Body.Close()
				switch stage {
				case "request":
					t.Fatal("a canceled request program must not forward a request")
				case "response":
					cancel()
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
				case "stream event":
					return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: 1\n\n"))}, nil
				}
				panic("unknown stage")
			}))
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://provider.example", nil)
			require.NoError(t, err)
			resp, err := adapter.RoundTrip(req)
			if stage == "stream event" {
				require.NoError(t, err)
				defer resp.Body.Close()
				cancel()
				_, err = io.ReadAll(resp.Body)
			}
			require.ErrorIs(t, err, context.Canceled)
			require.ErrorContains(t, err, "rewrite "+stage)
		})
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

type trackedBody struct {
	io.Reader
	closes int
}

func (b *trackedBody) Close() error {
	b.closes++
	return nil
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func newStubAdapter(t *testing.T, request, response string, base http.RoundTripper) *Adapter {
	t.Helper()
	adapter, err := New(&oauth.GatewaySpec{Request: request, Response: response}, "", base)
	require.NoError(t, err)
	return adapter
}

func parseEvents(t *testing.T, stream string) []map[string]any {
	t.Helper()
	var events []map[string]any
	for _, block := range strings.Split(stream, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		require.True(t, strings.HasPrefix(block, "data:"), "every event is a data line: %q", block)
		var event map[string]any
		require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(block, "data:"))), &event))
		events = append(events, event)
	}
	return events
}

func firstText(t *testing.T, event map[string]any) string {
	t.Helper()
	candidates := event["candidates"].([]any)
	content := candidates[0].(map[string]any)["content"].(map[string]any)
	parts := content["parts"].([]any)
	return parts[0].(map[string]any)["text"].(string)
}

// postEmpty POSTs an empty JSON object to url through rt.
func postEmpty(t *testing.T, rt http.RoundTripper, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, strings.NewReader(`{}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	return (&http.Client{Transport: rt}).Do(req)
}
