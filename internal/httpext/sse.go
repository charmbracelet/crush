package httpext

import (
	"bufio"
	"io"
	"net/http"
	"strings"
)

// WrapSSESanitizingHTTPClient wraps an HTTP client so SSE responses ignore any
// leading blank frame bytes before the first real event.
func WrapSSESanitizingHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		return &http.Client{
			Transport: sseSanitizingTransport{base: http.DefaultTransport},
		}
	}

	clone := *client
	clone.Transport = sseSanitizingTransport{base: client.Transport}
	return &clone
}

// sseSanitizingTransport works around providers that prepend empty SSE frames
// before the first real event, which openai-go does not ignore.
type sseSanitizingTransport struct {
	base http.RoundTripper
}

func (t sseSanitizingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.Body == nil {
		return resp, nil
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.HasPrefix(contentType, "text/event-stream") {
		return resp, nil
	}

	resp.Body = &sseSanitizingReadCloser{
		ReadCloser: resp.Body,
		reader:     bufio.NewReader(resp.Body),
	}
	return resp, nil
}

type sseSanitizingReadCloser struct {
	io.ReadCloser
	reader  *bufio.Reader
	started bool
}

func (r *sseSanitizingReadCloser) Read(p []byte) (int, error) {
	if !r.started {
		r.started = true
		for {
			b, err := r.reader.Peek(1)
			if err != nil {
				return 0, err
			}
			if b[0] != '\n' && b[0] != '\r' {
				break
			}
			if _, err := r.reader.ReadByte(); err != nil {
				return 0, err
			}
		}
	}

	return r.reader.Read(p)
}
