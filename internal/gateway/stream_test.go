package gateway

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/iotest"
	"testing/synctest"

	"github.com/stretchr/testify/require"
)

func TestStreamFinalEvent(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, suffix string }{
		{name: "clean EOF"},
		{name: "blank line", suffix: "\n"},
		{name: "comment", suffix: ": trailing comment\r\n\r\n"},
		{name: "terminator", suffix: "data: [DONE]\n\n"},
		{name: "unterminated comment", suffix: ": trailing comment"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			upstream := "data: 1\n\ndata: 2\n\n" + tt.suffix
			resp := stubGateway(t, `{}`, `[{data, final}]`, "text/event-stream", io.NopCloser(strings.NewReader(upstream)))
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, "data: {\"data\":1,\"final\":false}\n\ndata: {\"data\":2,\"final\":true}\n\n"+tt.suffix, string(body))
		})
	}
}

func TestStreamMultilineFraming(t *testing.T) {
	t.Parallel()

	heartbeat := ": heartbeat\r\n\r\n"
	passthrough := "event: unknown\r\nid: 42\r\ndata: line 1\r\ndata: line 2\r\n\r\n"
	upstream := heartbeat + "event: update\r\n: metadata\r\nid: 43\r\ndata: {\r\ndata: \"ok\":true}\r\n\r\n" + passthrough
	resp := stubGateway(t, `{}`, `[.data]`, "text/event-stream", io.NopCloser(strings.NewReader(upstream)))
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, heartbeat+": metadata\r\nid: 43\r\nevent: update\ndata: {\"ok\":true}\n\n"+passthrough, string(body))
}

func TestStreamReadError(t *testing.T) {
	t.Parallel()

	readErr := errors.New("upstream failure")
	upstream := io.MultiReader(strings.NewReader("data: 1\n\n"), &oneShotReadError{Reader: iotest.ErrReader(readErr)})
	resp := stubGateway(t, `{}`, `[{data, final}]`, "text/event-stream", io.NopCloser(upstream))
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.ErrorIs(t, err, readErr)
	require.Equal(t, "data: {\"data\":1,\"final\":false}\n\n", string(body))
}

type oneShotReadError struct{ io.Reader }

func (r *oneShotReadError) Read(p []byte) (int, error) {
	if r.Reader == nil {
		return 0, io.EOF
	}
	n, err := r.Reader.Read(p)
	r.Reader = nil
	return n, err
}

func TestStreamDeliversProgressively(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		upstream, writer := io.Pipe()
		defer writer.Close()
		resp := stubGateway(t, `{}`, `[{data, final}]`, "text/event-stream", upstream) //nolint:bodyclose // Closed by the defer inside the synctest bubble.
		defer resp.Body.Close()

		const first = "data: {\"data\":1,\"final\":false}\n\n"
		result := make(chan string, 1)
		go func() {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, int64(len(first))))
			result <- string(body)
		}()
		_, err := io.WriteString(writer, "data: 1\n\n")
		require.NoError(t, err)
		_, err = io.WriteString(writer, ": heartbeat\n\n")
		require.NoError(t, err)
		synctest.Wait()
		require.Empty(t, result, "a keepalive does not resolve finality")

		_, err = io.WriteString(writer, "data: 2\n\n")
		require.NoError(t, err)
		synctest.Wait()
		require.Len(t, result, 1, "a parsed successor releases the previous event without requiring EOF")
		require.Equal(t, first, <-result)

		require.NoError(t, writer.Close())
		tail, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, ": heartbeat\n\ndata: {\"data\":2,\"final\":true}\n\n", string(tail))
	})
}

func stubGateway(t *testing.T, request, response, contentType string, body io.ReadCloser) *http.Response {
	t.Helper()
	adapter := newStubAdapter(t, request, response, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		require.NoError(t, req.Body.Close())
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {contentType}}, Body: body}, nil
	}))
	resp, err := postEmpty(t, adapter, "https://provider.example")
	require.NoError(t, err)
	return resp
}

// Passthrough frames held behind an unresolved event are bounded like a body.
func TestStreamCapsHeldPassthrough(t *testing.T) {
	t.Parallel()

	frame := ":" + strings.Repeat("x", 64<<10) + "\n\n"
	upstream := io.MultiReader(strings.NewReader("data: 1\n\n"), &repeatReader{frame: frame})
	resp := stubGateway(t, `{}`, `[.data]`, "text/event-stream", io.NopCloser(upstream))
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.ErrorContains(t, err, "exceeds")
	require.Empty(t, body, "the held event never resolved")
}

// repeatReader yields frame forever.
type repeatReader struct {
	frame string
	off   int
}

func (r *repeatReader) Read(p []byte) (int, error) {
	n := copy(p, r.frame[r.off:])
	r.off = (r.off + n) % len(r.frame)
	return n, nil
}
