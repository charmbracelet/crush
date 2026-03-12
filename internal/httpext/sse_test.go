package httpext

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/openai/openai-go/v2/packages/ssestream"
	"github.com/openai/openai-go/v2/responses"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestWrapSSESanitizingHTTPClientSkipsLeadingBlankEvent(t *testing.T) {
	t.Parallel()

	respBody := "\n\nevent: response.created\ndata: {\"type\":\"response.created\"}\n\n"
	resp := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(respBody)),
	}

	client := WrapSSESanitizingHTTPClient(&http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return resp, nil
		}),
	})

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/stream", nil)
	require.NoError(t, err)

	gotResp, err := client.Do(req)
	require.NoError(t, err)
	defer gotResp.Body.Close()

	stream := ssestream.NewStream[responses.ResponseStreamEventUnion](ssestream.NewDecoder(gotResp), nil)
	require.True(t, stream.Next())
	require.Equal(t, "response.created", stream.Current().Type)
	require.NoError(t, stream.Err())
}

func TestWrapSSESanitizingHTTPClientLeavesNonSSEUntouched(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(strings.NewReader("{\"ok\":true}")),
	}

	client := WrapSSESanitizingHTTPClient(&http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return resp, nil
		}),
	})

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/data", nil)
	require.NoError(t, err)

	gotResp, err := client.Do(req)
	require.NoError(t, err)
	defer gotResp.Body.Close()

	body, err := io.ReadAll(gotResp.Body)
	require.NoError(t, err)
	require.Equal(t, "{\"ok\":true}", string(body))
}
