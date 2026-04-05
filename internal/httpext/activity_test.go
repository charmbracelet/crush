package httpext

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWrapActivityTrackingHTTPClientNil(t *testing.T) {
	client := WrapActivityTrackingHTTPClient(nil)
	require.NotNil(t, client)
	require.NotNil(t, client.Transport)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWrapActivityTrackingHTTPClientPreservesTransport(t *testing.T) {
	custom := &http.Transport{DisableKeepAlives: true}
	original := &http.Client{Transport: custom}

	wrapped := WrapActivityTrackingHTTPClient(original)
	require.NotNil(t, wrapped)

	att, ok := wrapped.Transport.(*activityTrackingTransport)
	require.True(t, ok, "transport should be *activityTrackingTransport")
	require.Same(t, custom, att.base, "base transport should be the original custom transport")
}

func TestActivityTrackingSignalsOnRead(t *testing.T) {
	body := "hello world"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer ts.Close()

	client := WrapActivityTrackingHTTPClient(nil)

	ctx, actCh := WithStreamActivity(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf := make([]byte, len(body))
	n, err := resp.Body.Read(buf)
	require.Greater(t, n, 0)
	// err may be nil or io.EOF when entire body fits in one read.
	if err != nil {
		require.ErrorIs(t, err, io.EOF)
	}

	select {
	case <-actCh:
	default:
		t.Fatal("expected activity signal after Read")
	}
}

func TestActivityTrackingNoSignalWithoutContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
	}))
	defer ts.Close()

	client := WrapActivityTrackingHTTPClient(nil)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	_, ok := resp.Body.(*activityTrackingReadCloser)
	require.False(t, ok, "body should NOT be wrapped when context has no activity channel")

	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
}

func TestActivityTrackingNonBlockingSignal(t *testing.T) {
	body := "abcdef"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer ts.Close()

	client := WrapActivityTrackingHTTPClient(nil)

	ctx, actCh := WithStreamActivity(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf := make([]byte, 2)

	n, err := resp.Body.Read(buf)
	require.NoError(t, err)
	require.Greater(t, n, 0)

	n, err = resp.Body.Read(buf)
	require.NoError(t, err)
	require.Greater(t, n, 0)

	received := 0
	for {
		select {
		case <-actCh:
			received++
		default:
			goto done
		}
	}
done:
	require.Equal(t, 1, received, "channel is buffered(1); second Read must not block, only one signal should be pending")
}
