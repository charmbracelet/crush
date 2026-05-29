package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/taigrr/fantasy"
	"github.com/stretchr/testify/require"
)

func TestPickBestResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		results []context7SearchResult
		wantID  string
	}{
		{
			name:    "empty list returns nil",
			results: nil,
			wantID:  "",
		},
		{
			name: "finalized beats non-finalized regardless of trust",
			results: []context7SearchResult{
				{ID: "/a", State: "initial", TrustScore: 10},
				{ID: "/b", State: "finalized", TrustScore: 1},
			},
			wantID: "/b",
		},
		{
			name: "higher trust wins among finalized",
			results: []context7SearchResult{
				{ID: "/a", State: "finalized", TrustScore: 5, TotalTokens: 100},
				{ID: "/b", State: "finalized", TrustScore: 9, TotalTokens: 1},
			},
			wantID: "/b",
		},
		{
			name: "ties broken by total tokens",
			results: []context7SearchResult{
				{ID: "/a", State: "finalized", TrustScore: 5, TotalTokens: 1000},
				{ID: "/b", State: "finalized", TrustScore: 5, TotalTokens: 100},
			},
			wantID: "/a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := pickBestResult(tt.results)
			if tt.wantID == "" {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, tt.wantID, got.ID)
		})
	}
}

func TestContext7Get_StatusHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		body    string
		wantSub string
		wantOK  bool
	}{
		{name: "200 returns body", status: 200, body: "OK BODY", wantOK: true},
		{name: "202 indexing message", status: 202, wantSub: "still being indexed"},
		{name: "401 auth message", status: 401, wantSub: "rejected the API key"},
		{name: "429 rate limit message", status: 429, wantSub: "rate limit"},
		{name: "500 generic message", status: 500, body: "boom", wantSub: "HTTP 500"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(srv.Close)

			body, err := context7Get(t.Context(), srv.Client(), srv.URL+"/anything")
			if tt.wantOK {
				require.NoError(t, err)
				require.Equal(t, tt.body, string(body))
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantSub)
		})
	}
}

// stubServer wires a fake context7 endpoint so we can exercise the full
// resolve-then-query flow without hitting the network.
func stubServer(t *testing.T, search context7SearchResponse, docs string, status int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/libs/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(search)
	})
	mux.HandleFunc("/v2/context", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(docs))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// rewriteHost is an http.RoundTripper that redirects requests targeting
// context7.com to a test server, transparently — lets us drive the
// tool end-to-end without making the base URL configurable in prod.
type rewriteHost struct {
	to string
}

func (r rewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	if !strings.HasPrefix(req.URL.String(), context7BaseURL) {
		return http.DefaultTransport.RoundTrip(req)
	}
	target := r.to + strings.TrimPrefix(req.URL.RequestURI(), "/api")
	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, target, req.Body)
	if err != nil {
		return nil, err
	}
	newReq.Header = req.Header.Clone()
	return http.DefaultTransport.RoundTrip(newReq)
}

func TestContext7Tool_HappyPath(t *testing.T) {
	t.Parallel()

	srv := stubServer(t,
		context7SearchResponse{Results: []context7SearchResult{
			{ID: "/vercel/next.js", Title: "Next.js", State: "finalized", TrustScore: 9},
		}},
		"# Next.js docs\n\nfetched", http.StatusOK)

	hc := &http.Client{Transport: rewriteHost{to: srv.URL}}
	tool := NewContext7Tool(hc)
	resp, err := tool.Run(t.Context(), fantasy.ToolCall{
		Input: `{"library":"Next.js","query":"app router"}`,
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, "got: %s", resp.Content)
	require.Contains(t, resp.Content, "Resolved \"Next.js\"")
	require.Contains(t, resp.Content, "Next.js docs")
}

func TestContext7Tool_RequiresQuery(t *testing.T) {
	t.Parallel()
	tool := NewContext7Tool(nil)
	resp, err := tool.Run(t.Context(), fantasy.ToolCall{Input: `{"library":"react"}`})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "query is required")
}

func TestContext7Tool_RequiresLibrary(t *testing.T) {
	t.Parallel()
	tool := NewContext7Tool(nil)
	resp, err := tool.Run(t.Context(), fantasy.ToolCall{Input: `{"query":"how do hooks work"}`})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "library or library_id is required")
}

func TestContext7Tool_PinnedLibraryIDSkipsResolve(t *testing.T) {
	t.Parallel()

	resolved := false
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/libs/search", func(http.ResponseWriter, *http.Request) {
		resolved = true
	})
	mux.HandleFunc("/v2/context", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("pinned docs"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	hc := &http.Client{Transport: rewriteHost{to: srv.URL}}
	tool := NewContext7Tool(hc)
	resp, err := tool.Run(t.Context(), fantasy.ToolCall{
		Input: `{"library_id":"/vercel/next.js","query":"app router"}`,
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, "got: %s", resp.Content)
	require.False(t, resolved, "should not have called resolve when library_id was pinned")
	require.Contains(t, resp.Content, "pinned docs")
	require.False(t, strings.Contains(resp.Content, "Resolved"))
}
