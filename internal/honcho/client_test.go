package honcho

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// Workspace, session, and peer IDs deliberately contain a space and a
// slash so every path assertion doubles as an escaping assertion.
const (
	testWorkspace    = "sp ace/ws"
	testWorkspaceEsc = "sp%20ace%2Fws"
	testSession      = "se ss/1"
	testSessionEsc   = "se%20ss%2F1"
	testPeer         = "pe er/1"
	testPeerEsc      = "pe%20er%2F1"
)

// capturedRequest is one request as the test server saw it.
type capturedRequest struct {
	Method string
	Path   string
	Query  string
	Body   string
	Header http.Header
}

// recorder is a test server that records every request and replies
// with a canned status and body.
type recorder struct {
	mu       sync.Mutex
	requests []capturedRequest

	// status and body are the canned reply. statuses, when set,
	// takes precedence and is consumed one entry per request, with
	// the final entry repeating.
	status   int
	body     string
	statuses []int
}

func (r *recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var buf []byte
	if req.Body != nil {
		buf, _ = io.ReadAll(req.Body)
	}

	r.mu.Lock()
	n := len(r.requests)
	r.requests = append(r.requests, capturedRequest{
		Method: req.Method,
		Path:   req.URL.EscapedPath(),
		Query:  req.URL.RawQuery,
		Body:   string(buf),
		Header: req.Header.Clone(),
	})
	status := r.status
	if len(r.statuses) > 0 {
		if n >= len(r.statuses) {
			n = len(r.statuses) - 1
		}
		status = r.statuses[n]
	}
	body := r.body
	r.mu.Unlock()

	if status == 0 {
		status = http.StatusOK
	}
	if body == "" {
		body = "{}"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.requests)
}

func (r *recorder) last() capturedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.requests) == 0 {
		return capturedRequest{}
	}
	return r.requests[len(r.requests)-1]
}

// newTestClient starts a recorder-backed server and returns a client
// pointed at it. Retries are disabled unless a case opts in.
func newTestClient(t *testing.T, rec *recorder, mutate func(*Options)) *Client {
	t.Helper()
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)

	opts := Options{
		BaseURL:    srv.URL,
		APIKey:     "secret-key",
		Workspace:  testWorkspace,
		MaxRetries: -1,
		Timeout:    5 * time.Second,
	}
	if mutate != nil {
		mutate(&opts)
	}
	c, err := New(opts)
	require.NoError(t, err)
	return c
}

func boolPtr(b bool) *bool { return &b }

func TestClientEndpoints(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		resp       string
		wantMethod string
		wantPath   string
		wantQuery  url.Values
		wantBody   string
		call       func(t *testing.T, c *Client)
	}{
		{
			name:       "Health",
			wantMethod: http.MethodGet,
			wantPath:   "/health",
			call: func(t *testing.T, c *Client) {
				require.NoError(t, c.Health(t.Context()))
			},
		},
		{
			name:       "EnsureWorkspace",
			resp:       `{"id":"sp ace/ws","metadata":{"k":"v"}}`,
			wantMethod: http.MethodPost,
			wantPath:   "/v3/workspaces",
			wantBody:   `{"id":"sp ace/ws"}`,
			call: func(t *testing.T, c *Client) {
				ws, err := c.EnsureWorkspace(t.Context())
				require.NoError(t, err)
				require.Equal(t, testWorkspace, ws.ID)
				require.Equal(t, "v", ws.Metadata["k"])
			},
		},
		{
			name:       "EnsurePeer nil config",
			resp:       `{"id":"pe er/1"}`,
			wantMethod: http.MethodPost,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/peers",
			wantBody:   `{"id":"pe er/1"}`,
			call: func(t *testing.T, c *Client) {
				p, err := c.EnsurePeer(t.Context(), testPeer, nil)
				require.NoError(t, err)
				require.Equal(t, testPeer, p.ID)
			},
		},
		{
			name:       "EnsurePeer with config",
			resp:       `{"id":"pe er/1"}`,
			wantMethod: http.MethodPost,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/peers",
			wantBody:   `{"id":"pe er/1","configuration":{"observe_me":true,"observe_others":false}}`,
			call: func(t *testing.T, c *Client) {
				_, err := c.EnsurePeer(t.Context(), testPeer, &PeerConfig{
					ObserveMe:     boolPtr(true),
					ObserveOthers: boolPtr(false),
				})
				require.NoError(t, err)
			},
		},
		{
			name:       "EnsureSession",
			resp:       `{"id":"se ss/1"}`,
			wantMethod: http.MethodPost,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/sessions",
			wantBody:   `{"id":"se ss/1","peers":{"u":{"observe_me":true}}}`,
			call: func(t *testing.T, c *Client) {
				s, err := c.EnsureSession(t.Context(), testSession, map[string]PeerConfig{
					"u": {ObserveMe: boolPtr(true)},
				})
				require.NoError(t, err)
				require.Equal(t, testSession, s.ID)
			},
		},
		{
			name:       "EnsureSession no peers",
			resp:       `{"id":"se ss/1"}`,
			wantMethod: http.MethodPost,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/sessions",
			wantBody:   `{"id":"se ss/1"}`,
			call: func(t *testing.T, c *Client) {
				_, err := c.EnsureSession(t.Context(), testSession, nil)
				require.NoError(t, err)
			},
		},
		{
			name:       "AddPeers",
			wantMethod: http.MethodPost,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/sessions/" + testSessionEsc + "/peers",
			wantBody:   `{"u":{"observe_me":true,"observe_others":false},"a":{"observe_others":true}}`,
			call: func(t *testing.T, c *Client) {
				err := c.AddPeers(t.Context(), testSession, map[string]PeerConfig{
					"u": {ObserveMe: boolPtr(true), ObserveOthers: boolPtr(false)},
					"a": {ObserveOthers: boolPtr(true)},
				})
				require.NoError(t, err)
			},
		},
		{
			name:       "AddMessages",
			wantMethod: http.MethodPost,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/sessions/" + testSessionEsc + "/messages",
			wantBody:   `{"messages":[{"peer_id":"u","content":"hello"},{"peer_id":"a","content":"hi","token_count":4}]}`,
			call: func(t *testing.T, c *Client) {
				err := c.AddMessages(t.Context(), testSession, []Message{
					{PeerID: "u", Content: "hello"},
					{PeerID: "a", Content: "hi", TokenCount: 4},
				})
				require.NoError(t, err)
			},
		},
		{
			name:       "SessionContext",
			resp:       `{"id":"se ss/1","messages":[{"peer_id":"u","content":"hi"}],"peer_representation":"rep","peer_card":["a","b"]}`,
			wantMethod: http.MethodGet,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/sessions/" + testSessionEsc + "/context",
			wantQuery: url.Values{
				"tokens":                []string{"1500"},
				"search_query":          []string{"memory"},
				"summary":               []string{"false"},
				"peer_perspective":      []string{"u"},
				"peer_target":           []string{"a"},
				"limit_to_session":      []string{"true"},
				"search_top_k":          []string{"4"},
				"search_max_distance":   []string{"0.35"},
				"include_most_frequent": []string{"true"},
				"max_conclusions":       []string{"6"},
			},
			call: func(t *testing.T, c *Client) {
				got, err := c.SessionContext(t.Context(), testSession, ContextOptions{
					Tokens:              1500,
					SearchQuery:         "memory",
					IncludeSummary:      boolPtr(false),
					PeerPerspective:     "u",
					PeerTarget:          "a",
					LimitToSession:      true,
					SearchTopK:          4,
					SearchMaxDistance:   0.35,
					IncludeMostFrequent: true,
					MaxConclusions:      6,
				})
				require.NoError(t, err)
				require.Equal(t, "rep", got.Representation)
				require.Len(t, got.Messages, 1)
				require.Equal(t, []string{"a", "b"}, got.PeerCard)
			},
		},
		{
			name:       "SessionContext zero options",
			resp:       `{"id":"se ss/1"}`,
			wantMethod: http.MethodGet,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/sessions/" + testSessionEsc + "/context",
			call: func(t *testing.T, c *Client) {
				_, err := c.SessionContext(t.Context(), testSession, ContextOptions{})
				require.NoError(t, err)
			},
		},
		{
			name:       "Summaries",
			resp:       `{"id":"se ss/1","short_summary":{"content":"short","token_count":3}}`,
			wantMethod: http.MethodGet,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/sessions/" + testSessionEsc + "/summaries",
			call: func(t *testing.T, c *Client) {
				got, err := c.Summaries(t.Context(), testSession)
				require.NoError(t, err)
				require.NotNil(t, got.Short)
				require.Equal(t, "short", got.Short.Content)
				require.Nil(t, got.Long)
			},
		},
		{
			name:       "PeerContext",
			resp:       `{"peer_id":"pe er/1","target_id":"u","representation":"rep"}`,
			wantMethod: http.MethodGet,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/peers/" + testPeerEsc + "/context",
			wantQuery: url.Values{
				"target":                []string{"u"},
				"search_query":          []string{"q"},
				"search_top_k":          []string{"2"},
				"search_max_distance":   []string{"0.5"},
				"include_most_frequent": []string{"true"},
				"max_conclusions":       []string{"3"},
			},
			call: func(t *testing.T, c *Client) {
				got, err := c.PeerContext(t.Context(), testPeer, PeerContextOptions{
					Target:              "u",
					SearchQuery:         "q",
					SearchTopK:          2,
					SearchMaxDistance:   0.5,
					IncludeMostFrequent: true,
					MaxConclusions:      3,
				})
				require.NoError(t, err)
				require.Equal(t, "rep", got.Representation)
			},
		},
		{
			name:       "Chat",
			resp:       `{"content":"answer"}`,
			wantMethod: http.MethodPost,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/peers/" + testPeerEsc + "/chat",
			wantBody:   `{"query":"what do you know","target":"u","session_id":"se ss/1","reasoning_level":"high","include_evidence":true}`,
			call: func(t *testing.T, c *Client) {
				got, err := c.Chat(t.Context(), testPeer, ChatQuery{
					Query:           "what do you know",
					Target:          "u",
					SessionID:       testSession,
					ReasoningLevel:  ReasoningHigh,
					IncludeEvidence: true,
					// Stream is forced off by the client.
					Stream: true,
				})
				require.NoError(t, err)
				require.Equal(t, "answer", got)
			},
		},
		{
			name:       "SearchSession",
			resp:       `{"items":[{"peer_id":"u","content":"m1"},{"peer_id":"a","content":"m2"}]}`,
			wantMethod: http.MethodPost,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/sessions/" + testSessionEsc + "/search",
			wantBody:   `{"query":"needle","limit":5}`,
			call: func(t *testing.T, c *Client) {
				got, err := c.SearchSession(t.Context(), testSession, SearchQuery{
					Query: "needle",
					Limit: 5,
				})
				require.NoError(t, err)
				require.Len(t, got, 2)
				require.Equal(t, "m1", got[0].Content)
			},
		},
		{
			name:       "CreateConclusions",
			wantMethod: http.MethodPost,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/conclusions",
			wantBody:   `{"conclusions":[{"content":"likes go","observer_id":"u","observed_id":"u"}]}`,
			call: func(t *testing.T, c *Client) {
				err := c.CreateConclusions(t.Context(), []Conclusion{
					{Content: "likes go", ObserverID: "u", ObservedID: "u"},
				})
				require.NoError(t, err)
			},
		},
		{
			name:       "QueryConclusions",
			resp:       `{"items":[{"content":"c1","observer_id":"u","observed_id":"u"}]}`,
			wantMethod: http.MethodPost,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/conclusions/query",
			wantBody:   `{"query":"prefs","top_k":7}`,
			call: func(t *testing.T, c *Client) {
				got, err := c.QueryConclusions(t.Context(), ConclusionQuery{
					Query: "prefs",
					TopK:  7,
				})
				require.NoError(t, err)
				require.Len(t, got, 1)
				require.Equal(t, "c1", got[0].Content)
			},
		},
		{
			name:       "QueueStatus scoped",
			resp:       `{"total_work_units":3,"pending_work_units":1}`,
			wantMethod: http.MethodGet,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/queue/status",
			wantQuery:  url.Values{"session_id": []string{testSession}},
			call: func(t *testing.T, c *Client) {
				got, err := c.QueueStatus(t.Context(), testSession)
				require.NoError(t, err)
				require.Equal(t, 3, got.Total)
				require.Equal(t, 1, got.Pending)
			},
		},
		{
			name:       "QueueStatus global",
			resp:       `{"empty":true}`,
			wantMethod: http.MethodGet,
			wantPath:   "/v3/workspaces/" + testWorkspaceEsc + "/queue/status",
			call: func(t *testing.T, c *Client) {
				got, err := c.QueueStatus(t.Context(), "")
				require.NoError(t, err)
				require.True(t, got.Empty)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := &recorder{body: tc.resp}
			c := newTestClient(t, rec, nil)
			tc.call(t, c)

			require.Equal(t, 1, rec.count())
			got := rec.last()
			require.Equal(t, tc.wantMethod, got.Method)
			require.Equal(t, tc.wantPath, got.Path)
			require.Equal(t, tc.wantQuery.Encode(), got.Query)

			if tc.wantBody == "" {
				require.Empty(t, got.Body)
				require.Empty(t, got.Header.Get("Content-Type"))
			} else {
				require.JSONEq(t, tc.wantBody, got.Body)
				require.Equal(t, "application/json", got.Header.Get("Content-Type"))
			}
			require.Equal(t, "application/json", got.Header.Get("Accept"))
		})
	}
}

func TestClientHeaders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		apiKey      string
		hostVersion string
		wantAuth    string
		wantHost    string
		wantPlugin  string
	}{
		{
			name:        "key set",
			apiKey:      "abc123",
			hostVersion: "1.2.3",
			wantAuth:    "Bearer abc123",
			wantHost:    "crush/1.2.3",
			wantPlugin:  "crush-honcho/1.2.3",
		},
		{
			name:       "key empty",
			apiKey:     "",
			wantAuth:   "",
			wantHost:   "crush/dev",
			wantPlugin: "crush-honcho/dev",
		},
		{
			name:       "key whitespace only",
			apiKey:     "   ",
			wantAuth:   "",
			wantHost:   "crush/dev",
			wantPlugin: "crush-honcho/dev",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := &recorder{}
			c := newTestClient(t, rec, func(o *Options) {
				o.APIKey = tc.apiKey
				o.HostVersion = tc.hostVersion
			})
			require.NoError(t, c.Health(t.Context()))

			got := rec.last()
			require.Equal(t, tc.wantAuth, got.Header.Get("Authorization"))
			require.Equal(t, tc.wantHost, got.Header.Get("X-Honcho-Host"))
			require.Equal(t, tc.wantPlugin, got.Header.Get("X-Honcho-Plugin"))
		})
	}
}

func TestClientRetries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		status    int
		wantCalls int
		wantErr   bool
	}{
		{name: "429 retries", status: http.StatusTooManyRequests, wantCalls: 2, wantErr: true},
		{name: "503 retries", status: http.StatusServiceUnavailable, wantCalls: 2, wantErr: true},
		{name: "500 retries", status: http.StatusInternalServerError, wantCalls: 2, wantErr: true},
		{name: "408 retries", status: http.StatusRequestTimeout, wantCalls: 2, wantErr: true},
		{name: "400 is terminal", status: http.StatusBadRequest, wantCalls: 1, wantErr: true},
		{name: "404 is terminal", status: http.StatusNotFound, wantCalls: 1, wantErr: true},
		{name: "403 is terminal", status: http.StatusForbidden, wantCalls: 1, wantErr: true},
		{name: "200 succeeds once", status: http.StatusOK, wantCalls: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := &recorder{status: tc.status, body: `{"detail":"nope"}`}
			c := newTestClient(t, rec, func(o *Options) {
				o.MaxRetries = 1
				o.Timeout = 2 * time.Second
			})

			err := c.Health(t.Context())
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantCalls, rec.count())
		})
	}
}

func TestClientRetryThenSucceed(t *testing.T) {
	t.Parallel()

	rec := &recorder{
		statuses: []int{http.StatusServiceUnavailable, http.StatusOK},
		body:     `{}`,
	}
	c := newTestClient(t, rec, func(o *Options) { o.MaxRetries = 1 })

	require.NoError(t, c.Health(t.Context()))
	require.Equal(t, 2, rec.count())
}

func TestClientTerminalErrorIsAPIError(t *testing.T) {
	t.Parallel()

	rec := &recorder{status: http.StatusBadRequest, body: `{"detail":"bad workspace"}`}
	c := newTestClient(t, rec, func(o *Options) { o.MaxRetries = 1 })

	_, err := c.EnsureWorkspace(t.Context())
	require.Error(t, err)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.Contains(t, apiErr.Body, "bad workspace")
	require.Contains(t, apiErr.Error(), "bad workspace")
}

func TestAPIErrorRetryable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status int
		want   bool
	}{
		{status: http.StatusBadRequest, want: false},
		{status: http.StatusUnauthorized, want: false},
		{status: http.StatusForbidden, want: false},
		{status: http.StatusNotFound, want: false},
		{status: http.StatusRequestTimeout, want: true},
		{status: http.StatusConflict, want: false},
		{status: http.StatusUnprocessableEntity, want: false},
		{status: http.StatusTooManyRequests, want: true},
		{status: http.StatusInternalServerError, want: true},
		{status: http.StatusBadGateway, want: true},
		{status: http.StatusServiceUnavailable, want: true},
		{status: http.StatusGatewayTimeout, want: true},
		{status: 599, want: true},
		{status: http.StatusOK, want: false},
		{status: http.StatusMovedPermanently, want: false},
	}

	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			t.Parallel()
			e := &APIError{StatusCode: tc.status, Status: http.StatusText(tc.status)}
			require.Equal(t, tc.want, e.Retryable())
		})
	}
}

func TestAPIErrorMessage(t *testing.T) {
	t.Parallel()

	require.Equal(t, "honcho: 404 Not Found",
		(&APIError{StatusCode: 404, Status: "404 Not Found"}).Error())
	require.Equal(t, "honcho: 404 Not Found: missing",
		(&APIError{StatusCode: 404, Status: "404 Not Found", Body: "missing"}).Error())
}

func TestNilClientReturnsErrDisabled(t *testing.T) {
	t.Parallel()

	var c *Client
	ctx := t.Context()

	require.ErrorIs(t, c.Health(ctx), ErrDisabled)

	_, err := c.EnsureWorkspace(ctx)
	require.ErrorIs(t, err, ErrDisabled)

	_, err = c.EnsurePeer(ctx, "p", nil)
	require.ErrorIs(t, err, ErrDisabled)

	_, err = c.EnsureSession(ctx, "s", nil)
	require.ErrorIs(t, err, ErrDisabled)

	require.ErrorIs(t, c.AddPeers(ctx, "s", nil), ErrDisabled)
	require.ErrorIs(t, c.AddPeers(ctx, "s", map[string]PeerConfig{"u": {}}), ErrDisabled)
	require.ErrorIs(t, c.AddMessages(ctx, "s", nil), ErrDisabled)
	require.ErrorIs(t, c.AddMessages(ctx, "s", []Message{{PeerID: "u"}}), ErrDisabled)

	_, err = c.SessionContext(ctx, "s", ContextOptions{})
	require.ErrorIs(t, err, ErrDisabled)

	_, err = c.Summaries(ctx, "s")
	require.ErrorIs(t, err, ErrDisabled)

	_, err = c.PeerContext(ctx, "p", PeerContextOptions{})
	require.ErrorIs(t, err, ErrDisabled)

	answer, err := c.Chat(ctx, "p", ChatQuery{})
	require.ErrorIs(t, err, ErrDisabled)
	require.Empty(t, answer)

	_, err = c.SearchSession(ctx, "s", SearchQuery{})
	require.ErrorIs(t, err, ErrDisabled)

	require.ErrorIs(t, c.CreateConclusions(ctx, nil), ErrDisabled)
	require.ErrorIs(t, c.CreateConclusions(ctx, []Conclusion{{Content: "x"}}), ErrDisabled)

	_, err = c.QueryConclusions(ctx, ConclusionQuery{})
	require.ErrorIs(t, err, ErrDisabled)

	_, err = c.QueueStatus(ctx, "s")
	require.ErrorIs(t, err, ErrDisabled)

	require.Empty(t, c.Workspace())
	require.Empty(t, c.BaseURL())
}

func TestNew(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		opts        Options
		wantErr     bool
		wantBaseURL string
		wantRetries int
	}{
		{
			name:    "empty workspace is rejected",
			opts:    Options{Workspace: ""},
			wantErr: true,
		},
		{
			name:    "whitespace workspace is rejected",
			opts:    Options{Workspace: "   "},
			wantErr: true,
		},
		{
			name:        "empty base URL defaults to cloud",
			opts:        Options{Workspace: "ws"},
			wantBaseURL: DefaultBaseURL,
			wantRetries: defaultMaxRetries,
		},
		{
			name:        "trailing slash is trimmed",
			opts:        Options{Workspace: "ws", BaseURL: "http://localhost:8000/"},
			wantBaseURL: "http://localhost:8000",
			wantRetries: defaultMaxRetries,
		},
		{
			name:        "surrounding whitespace is trimmed",
			opts:        Options{Workspace: "ws", BaseURL: "  http://localhost:8000/  "},
			wantBaseURL: "http://localhost:8000",
			wantRetries: defaultMaxRetries,
		},
		{
			name:        "negative retries clamp to zero",
			opts:        Options{Workspace: "ws", MaxRetries: -5},
			wantBaseURL: DefaultBaseURL,
			wantRetries: 0,
		},
		{
			name:        "explicit retries are kept",
			opts:        Options{Workspace: "ws", MaxRetries: 9},
			wantBaseURL: DefaultBaseURL,
			wantRetries: 9,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c, err := New(tc.opts)
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, c)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantBaseURL, c.BaseURL())
			require.Equal(t, tc.wantRetries, c.maxRetries)
			require.Equal(t, strings.TrimSpace(tc.opts.Workspace), c.Workspace())
		})
	}
}

func TestNewDefaultTimeout(t *testing.T) {
	t.Parallel()

	c, err := New(Options{Workspace: "ws"})
	require.NoError(t, err)
	require.Equal(t, defaultTimeout, c.http.Timeout)

	custom := &http.Client{Timeout: time.Second}
	c, err = New(Options{Workspace: "ws", HTTPClient: custom, Timeout: time.Hour})
	require.NoError(t, err)
	require.Same(t, custom, c.http)
}

func TestClientContextCancellation(t *testing.T) {
	t.Parallel()

	rec := &recorder{status: http.StatusServiceUnavailable}
	c := newTestClient(t, rec, func(o *Options) { o.MaxRetries = 5 })

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	start := time.Now()
	err := c.Health(ctx)
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, elapsed, time.Second, "cancelled context should not exhaust retries")
	require.Zero(t, rec.count())
}

func TestClientContextCancelledMidRetry(t *testing.T) {
	t.Parallel()

	rec := &recorder{status: http.StatusServiceUnavailable}
	c := newTestClient(t, rec, func(o *Options) { o.MaxRetries = 20 })

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := c.Health(ctx)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.Less(t, elapsed, 5*time.Second, "deadline should cut retries short")
	require.Less(t, rec.count(), 20)
}

func TestAddMessagesEmptyMakesNoCall(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	c := newTestClient(t, rec, nil)

	require.NoError(t, c.AddMessages(t.Context(), testSession, nil))
	require.NoError(t, c.AddMessages(t.Context(), testSession, []Message{}))
	require.Zero(t, rec.count())
}

func TestAddPeersEmptyMakesNoCall(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	c := newTestClient(t, rec, nil)

	require.NoError(t, c.AddPeers(t.Context(), testSession, nil))
	require.NoError(t, c.AddPeers(t.Context(), testSession, map[string]PeerConfig{}))
	require.Zero(t, rec.count())
}

func TestCreateConclusionsEmptyMakesNoCall(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	c := newTestClient(t, rec, nil)

	require.NoError(t, c.CreateConclusions(t.Context(), nil))
	require.NoError(t, c.CreateConclusions(t.Context(), []Conclusion{}))
	require.Zero(t, rec.count())
}

func TestAddMessagesTruncatesOnRuneBoundary(t *testing.T) {
	t.Parallel()

	// Three-byte runes so the byte limit never lands on a boundary.
	const rune3 = "世"
	require.Len(t, rune3, 3)
	long := strings.Repeat(rune3, 9000)
	require.Greater(t, len(long), maxMessageChars)

	rec := &recorder{}
	c := newTestClient(t, rec, nil)

	require.NoError(t, c.AddMessages(t.Context(), testSession, []Message{
		{PeerID: "u", Content: long},
		{PeerID: "a", Content: "short"},
	}))

	var sent struct {
		Messages []Message `json:"messages"`
	}
	require.NoError(t, json.Unmarshal([]byte(rec.last().Body), &sent))
	require.Len(t, sent.Messages, 2)

	got := sent.Messages[0].Content
	require.True(t, utf8.ValidString(got), "truncation split a multibyte rune")
	require.LessOrEqual(t, len(got), maxMessageChars)
	// 25000 is not a multiple of three, so the last partial rune is
	// dropped entirely.
	require.Len(t, got, 24999)
	require.Equal(t, 8333, utf8.RuneCountInString(got))
	require.True(t, strings.HasPrefix(long, got))

	// Short messages pass through untouched.
	require.Equal(t, "short", sent.Messages[1].Content)
}

func TestClampTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{name: "under limit", in: "hello", n: 10, want: "hello"},
		{name: "exact limit", in: "hello", n: 5, want: "hello"},
		{name: "ascii truncated", in: "hello", n: 3, want: "hel"},
		{name: "zero limit", in: "hello", n: 0, want: "hello"},
		{name: "negative limit", in: "hello", n: -1, want: "hello"},
		{name: "empty input", in: "", n: 4, want: ""},
		{name: "cuts one byte into rune", in: "a世", n: 2, want: "a"},
		{name: "cuts two bytes into rune", in: "a世", n: 3, want: "a"},
		{name: "keeps whole rune", in: "a世", n: 4, want: "a世"},
		{name: "replacement char survives", in: "a\uFFFDb", n: 4, want: "a\uFFFD"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := clamp(tc.in, tc.n)
			require.Equal(t, tc.want, got)
			require.True(t, utf8.ValidString(got))
		})
	}
}

func TestClientAccessors(t *testing.T) {
	t.Parallel()

	c, err := New(Options{Workspace: "ws", BaseURL: "http://example.test/"})
	require.NoError(t, err)
	require.Equal(t, "ws", c.Workspace())
	require.Equal(t, "http://example.test", c.BaseURL())
}
