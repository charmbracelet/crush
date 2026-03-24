package httpext

import (
	"context"
	"io"
	"net/http"
)

type streamActivityKeyType struct{}

// WithStreamActivity returns a derived context carrying a stream-activity
// channel. Any HTTP response body wrapped by an activity-tracking transport
// will signal on this channel whenever bytes are read, allowing callers to
// detect network liveness even when higher-level parsers (e.g. SSE) silently
// consume keep-alive events without yielding application-level data.
func WithStreamActivity(ctx context.Context) (context.Context, <-chan struct{}) {
	ch := make(chan struct{}, 1)
	return context.WithValue(ctx, streamActivityKeyType{}, ch), ch
}

// streamActivityChan returns the activity channel stored in ctx, or nil.
func streamActivityChan(ctx context.Context) chan struct{} {
	ch, _ := ctx.Value(streamActivityKeyType{}).(chan struct{})
	return ch
}

// WrapActivityTrackingHTTPClient wraps an HTTP client so that SSE response
// body reads signal through a context-provided activity channel. If the
// request context does not carry an activity channel the response is returned
// unmodified.
func WrapActivityTrackingHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		return &http.Client{
			Transport: &activityTrackingTransport{base: http.DefaultTransport},
		}
	}
	clone := *client
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	clone.Transport = &activityTrackingTransport{base: base}
	return &clone
}

type activityTrackingTransport struct {
	base http.RoundTripper
}

func (t *activityTrackingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil {
		return resp, err
	}

	ch := streamActivityChan(req.Context())
	if ch == nil {
		return resp, nil
	}

	resp.Body = &activityTrackingReadCloser{
		ReadCloser: resp.Body,
		ch:         ch,
	}
	return resp, nil
}

type activityTrackingReadCloser struct {
	io.ReadCloser
	ch chan struct{}
}

func (r *activityTrackingReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		select {
		case r.ch <- struct{}{}:
		default:
		}
	}
	return n, err
}
