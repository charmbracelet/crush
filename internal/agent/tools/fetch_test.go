package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func runFetchTool(t *testing.T, client *http.Client, url string) (fantasy.ToolResponse, error) {
	t.Helper()

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	tool := NewFetchTool(&mockPermissionService{}, t.TempDir(), client)

	input, err := json.Marshal(FetchParams{URL: url, Format: "text"})
	require.NoError(t, err)

	return tool.Run(ctx, fantasy.ToolCall{
		ID:    "fetch-call",
		Name:  FetchToolName,
		Input: string(input),
	})
}

// A fetch that never completes must be reported to the model as a failed tool
// result. Returning a Go error instead ends the whole agent turn.
func TestFetchToolReportsNetworkFailuresAsToolErrors(t *testing.T) {
	t.Parallel()

	t.Run("connection refused", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("unused"))
		}))
		url := server.URL
		server.Close()

		resp, err := runFetchTool(t, server.Client(), url)
		require.NoError(t, err)
		require.True(t, resp.IsError)
		require.Contains(t, resp.Content, "Failed to fetch URL")
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.Write([]byte("too late"))
		}))
		defer server.Close()

		client := &http.Client{Timeout: 20 * time.Millisecond}
		resp, err := runFetchTool(t, client, server.URL)
		require.NoError(t, err)
		require.True(t, resp.IsError)
		require.Contains(t, resp.Content, "Failed to fetch URL")
		require.Contains(t, resp.Content, "context deadline exceeded")
	})
}

func TestFetchToolReportsStatusCodesAsToolErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	resp, err := runFetchTool(t, server.Client(), server.URL)
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "Request failed with status code: 404 Not Found")
}

func TestFetchToolReturnsContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("hello from fetch"))
	}))
	defer server.Close()

	resp, err := runFetchTool(t, server.Client(), server.URL)
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "hello from fetch")
}

func TestFetchToolUTF8Truncation(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		body      string
		format    string
		want      string
		truncated bool
		wantError bool
	}{
		{
			name:      "response limit inside a multibyte character",
			body:      strings.Repeat("a", MaxFetchSize-1) + "界rest",
			format:    "text",
			want:      strings.Repeat("a", MaxFetchSize-1),
			truncated: true,
		},
		{
			name:      "response limit inside a two-byte character",
			body:      strings.Repeat("a", MaxFetchSize-1) + "érest",
			format:    "text",
			want:      strings.Repeat("a", MaxFetchSize-1),
			truncated: true,
		},
		{
			name:      "response limit inside a four-byte character",
			body:      strings.Repeat("a", MaxFetchSize-3) + "🙂rest",
			format:    "text",
			want:      strings.Repeat("a", MaxFetchSize-3),
			truncated: true,
		},
		{
			name:      "formatted output limit inside a multibyte character",
			body:      strings.Repeat("a", MaxFetchSize-5) + "界",
			format:    "markdown",
			want:      "```\n" + strings.Repeat("a", MaxFetchSize-5),
			truncated: true,
		},
		{
			name:      "ASCII response over the limit",
			body:      strings.Repeat("a", MaxFetchSize+1),
			format:    "text",
			want:      strings.Repeat("a", MaxFetchSize),
			truncated: true,
		},
		{
			name:   "exact response limit",
			body:   strings.Repeat("a", MaxFetchSize),
			format: "text",
		},
		{
			name:   "small multibyte response",
			body:   "hello 世界",
			format: "text",
		},
		{
			name:      "invalid UTF-8 remains an error",
			body:      "hello\xff",
			format:    "text",
			wantError: true,
		},
		{
			name:      "invalid UTF-8 crossing response limit remains an error",
			body:      strings.Repeat("a", MaxFetchSize-1) + "\xf0\x9fXrest",
			format:    "text",
			wantError: true,
		},
		{
			name:      "invalid UTF-8 before response limit remains an error",
			body:      "\xff" + strings.Repeat("a", MaxFetchSize) + "界",
			format:    "text",
			wantError: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)

			tool := NewFetchTool(&mockPermissionService{}, t.TempDir(), server.Client())
			input, err := json.Marshal(FetchParams{URL: server.URL, Format: tt.format})
			require.NoError(t, err)
			ctx := context.WithValue(t.Context(), SessionIDContextKey, "test-session")
			response, err := tool.Run(ctx, fantasy.ToolCall{ID: "fetch-test", Name: FetchToolName, Input: string(input)})
			require.NoError(t, err)
			require.Equal(t, tt.wantError, response.IsError)
			if tt.wantError {
				require.Equal(t, "Response content is not valid UTF-8", response.Content)
				return
			}

			require.True(t, utf8.ValidString(response.Content), "fetched content must remain valid UTF-8")
			if tt.truncated {
				want := tt.want + "\n\n[Content truncated to 102400 bytes]"
				require.True(t, want == response.Content, "truncation should retain the complete UTF-8 prefix and its notice")
			} else {
				require.True(t, tt.body == response.Content, "a complete response should be returned unchanged")
			}
		})
	}
}

func TestFetchURLAndConvertUTF8Truncation(t *testing.T) {
	t.Parallel()

	const maxSize = 5 * 1024 * 1024
	for _, tt := range []struct {
		name      string
		body      string
		wantError bool
	}{
		{
			name: "response limit inside a multibyte character",
			body: strings.Repeat("a", maxSize-1) + "界rest",
		},
		{
			name:      "invalid UTF-8 before response limit remains an error",
			body:      "\xff" + strings.Repeat("a", maxSize) + "界",
			wantError: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)

			content, err := FetchURLAndConvert(t.Context(), server.Client(), server.URL)
			if tt.wantError {
				require.EqualError(t, err, "response content is not valid UTF-8")
				return
			}
			require.NoError(t, err)
			require.True(t, utf8.ValidString(content))
			require.LessOrEqual(t, len(content), maxSize)
			require.Equal(t, maxSize-1, len(content))
		})
	}
}
