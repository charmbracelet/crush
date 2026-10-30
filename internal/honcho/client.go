package honcho

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// DefaultBaseURL is Honcho Cloud. Self-hosted deployments override it.
const DefaultBaseURL = "https://api.honcho.dev"

// Transport defaults. Honcho's reasoning endpoints are slow by design,
// so the timeout is generous, but it is always bounded: a hung memory
// lookup must never hold up a turn.
const (
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 3
	maxRetryWait      = 8 * time.Second
)

// maxMessageChars clamps a single message body. Honcho rejects
// oversized messages, and a runaway tool output should be truncated
// rather than fail the whole batch.
const maxMessageChars = 25_000

// APIError is a non-2xx response from Honcho.
type APIError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("honcho: %s", e.Status)
	}
	return fmt.Sprintf("honcho: %s: %s", e.Status, e.Body)
}

// Retryable reports whether retrying the request could succeed.
// Rate limits and server faults are worth another attempt; a 4xx that
// is not 408 or 429 means the request itself is wrong.
func (e *APIError) Retryable() bool {
	switch {
	case e.StatusCode == http.StatusTooManyRequests:
		return true
	case e.StatusCode == http.StatusRequestTimeout:
		return true
	case e.StatusCode >= 500:
		return true
	default:
		return false
	}
}

// Client talks to a Honcho deployment. The zero value is not usable;
// construct one with New. A nil *Client is valid and turns every
// method into a no-op returning ErrDisabled, which is how the
// integration stays free when unconfigured.
type Client struct {
	baseURL    string
	apiKey     string
	workspace  string
	http       *http.Client
	userAgent  string
	pluginVer  string
	maxRetries int
}

// ErrDisabled is returned by every method on a nil Client.
var ErrDisabled = errors.New("honcho: not configured")

// Options configures a Client.
type Options struct {
	// BaseURL is the Honcho deployment. Empty means Honcho Cloud.
	BaseURL string
	// APIKey authenticates to the deployment. Local deployments
	// generally need none.
	APIKey string
	// Workspace is the workspace ID all operations are scoped to.
	Workspace string
	// HostVersion identifies Crush in telemetry headers, matching the
	// convention the other Honcho harness plugins follow.
	HostVersion string
	// HTTPClient overrides the transport, chiefly for tests.
	HTTPClient *http.Client
	// Timeout bounds a single request. Zero uses the default.
	Timeout time.Duration
	// MaxRetries bounds retry attempts on transient failures. Zero
	// uses the default; negative disables retries.
	MaxRetries int
}

// New builds a Client. It returns an error only when the options are
// internally inconsistent; it performs no network I/O, so a Client can
// be constructed before knowing whether the deployment is reachable.
func New(opts Options) (*Client, error) {
	base := strings.TrimSuffix(strings.TrimSpace(opts.BaseURL), "/")
	if base == "" {
		base = DefaultBaseURL
	}
	if _, err := url.Parse(base); err != nil {
		return nil, fmt.Errorf("honcho: invalid base URL %q: %w", base, err)
	}
	if strings.TrimSpace(opts.Workspace) == "" {
		return nil, errors.New("honcho: workspace is required")
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: timeout}
	}
	retries := opts.MaxRetries
	if retries == 0 {
		retries = defaultMaxRetries
	}
	if retries < 0 {
		retries = 0
	}

	version := opts.HostVersion
	if version == "" {
		version = "dev"
	}

	return &Client{
		baseURL:    base,
		apiKey:     strings.TrimSpace(opts.APIKey),
		workspace:  opts.Workspace,
		http:       hc,
		userAgent:  "crush/" + version,
		pluginVer:  "crush-honcho/" + version,
		maxRetries: retries,
	}, nil
}

// Workspace returns the workspace ID this client is scoped to.
func (c *Client) Workspace() string {
	if c == nil {
		return ""
	}
	return c.workspace
}

// authorization returns the bearer credential for a request.
//
// An explicitly configured API key wins, because a user who set one
// meant to use it. Otherwise Crush falls back to a stored OAuth
// sign-in, refreshing it transparently when it has expired. A local
// deployment that needs no auth yields the empty string.
func (c *Client) authorization(ctx context.Context) string {
	if c.apiKey != "" {
		return c.apiKey
	}
	token, err := AccessToken(ctx, c.baseURL)
	if err != nil {
		logFailure("access token", err)
		return ""
	}
	return token
}

// BaseURL returns the deployment this client talks to.
func (c *Client) BaseURL() string {
	if c == nil {
		return ""
	}
	return c.baseURL
}

// do performs a request with retries, decoding a JSON response into
// out when out is non-nil. body is marshalled as JSON when non-nil.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	if c == nil {
		return ErrDisabled
	}

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("honcho: encode request: %w", err)
		}
	}

	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepCtx(ctx, backoff(attempt, lastErr)); err != nil {
				return err
			}
		}

		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
		if err != nil {
			return fmt.Errorf("honcho: build request: %w", err)
		}
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")
		if auth := c.authorization(ctx); auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		// Telemetry headers matching the convention used by the
		// other Honcho harness integrations.
		req.Header.Set("X-Honcho-Host", c.userAgent)
		req.Header.Set("X-Honcho-Plugin", c.pluginVer)

		resp, err := c.http.Do(req)
		if err != nil {
			// A cancelled or expired context is terminal; retrying
			// only burns the caller's remaining budget.
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = err
			continue
		}

		err = decodeResponse(resp, out)
		if err == nil {
			return nil
		}
		lastErr = err

		var apiErr *APIError
		if errors.As(err, &apiErr) && !apiErr.Retryable() {
			return err
		}
	}

	return fmt.Errorf("honcho: %s %s: %w", method, path, lastErr)
}

// decodeResponse reads and closes resp, returning an *APIError for
// non-2xx status and decoding the body into out otherwise.
func decodeResponse(resp *http.Response, out any) error {
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Cap the error body: Honcho can return large validation
		// payloads and they end up in logs.
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return &APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       strings.TrimSpace(string(snippet)),
		}
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("honcho: decode response: %w", err)
	}
	return nil
}

// backoff returns how long to wait before the given attempt. It
// honours a Retry-After hint when the previous failure carried one.
func backoff(attempt int, prev error) time.Duration {
	var apiErr *APIError
	if errors.As(prev, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests {
		if d := retryAfter(apiErr); d > 0 {
			return min(d, maxRetryWait)
		}
	}
	// Exponential with jitter, so concurrent writers do not resynchronise.
	wait := time.Duration(1<<uint(attempt-1)) * 250 * time.Millisecond
	wait += time.Duration(rand.N(250)) * time.Millisecond
	return min(wait, maxRetryWait)
}

// retryAfter extracts a Retry-After delay from an error body. Honcho
// returns the header, but the body is what survives into APIError, so
// this stays best-effort and returns zero when absent.
func retryAfter(e *APIError) time.Duration {
	const key = "retry-after"
	lower := strings.ToLower(e.Body)
	i := strings.Index(lower, key)
	if i < 0 {
		return 0
	}
	rest := e.Body[i+len(key):]
	rest = strings.TrimLeft(rest, `":' `)
	end := strings.IndexFunc(rest, func(r rune) bool { return r < '0' || r > '9' })
	if end == 0 {
		return 0
	}
	if end < 0 {
		end = len(rest)
	}
	secs, err := strconv.Atoi(rest[:end])
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

// sleepCtx waits for d or until ctx is done.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// esc escapes a path segment so peer and session IDs containing
// separators cannot alter the route.
func esc(s string) string { return url.PathEscape(s) }

// wsPath builds a workspace-scoped path.
func (c *Client) wsPath(format string, args ...any) string {
	escaped := make([]any, len(args))
	for i, a := range args {
		escaped[i] = esc(fmt.Sprint(a))
	}
	return "/v3/workspaces/" + esc(c.workspace) + fmt.Sprintf(format, escaped...)
}

// Health reports whether the deployment is reachable. It is the
// cheapest way to validate a base URL during setup.
func (c *Client) Health(ctx context.Context) error {
	if c == nil {
		return ErrDisabled
	}
	return c.do(ctx, http.MethodGet, "/health", nil, nil, nil)
}

// EnsureWorkspace creates the configured workspace if it does not
// already exist and returns it.
func (c *Client) EnsureWorkspace(ctx context.Context) (*Workspace, error) {
	if c == nil {
		return nil, ErrDisabled
	}
	var out Workspace
	body := map[string]any{"id": c.workspace}
	if err := c.do(ctx, http.MethodPost, "/v3/workspaces", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EnsurePeer creates a peer if it does not exist. cfg may be nil to
// accept the deployment's defaults.
func (c *Client) EnsurePeer(ctx context.Context, peerID string, cfg *PeerConfig) (*Peer, error) {
	if c == nil {
		return nil, ErrDisabled
	}
	body := map[string]any{"id": peerID}
	if cfg != nil {
		body["configuration"] = cfg
	}
	var out Peer
	if err := c.do(ctx, http.MethodPost, c.wsPath("/peers"), nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EnsureSession creates a session if it does not exist, attaching the
// given peers with their per-session observation config.
func (c *Client) EnsureSession(ctx context.Context, sessionID string, peers map[string]PeerConfig) (*Session, error) {
	if c == nil {
		return nil, ErrDisabled
	}
	body := map[string]any{"id": sessionID}
	if len(peers) > 0 {
		body["peers"] = peers
	}
	var out Session
	if err := c.do(ctx, http.MethodPost, c.wsPath("/sessions"), nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AddPeers attaches peers to an existing session, creating any that do
// not yet exist.
func (c *Client) AddPeers(ctx context.Context, sessionID string, peers map[string]PeerConfig) error {
	if c == nil {
		return ErrDisabled
	}
	if len(peers) == 0 {
		return nil
	}
	return c.do(ctx, http.MethodPost, c.wsPath("/sessions/%s/peers", sessionID), nil, peers, nil)
}

// AddMessages writes messages to a session. This is the call that
// triggers Honcho's background reasoning.
//
// Message content is clamped to a size the API accepts; an
// over-long tool output is truncated rather than rejected.
func (c *Client) AddMessages(ctx context.Context, sessionID string, msgs []Message) error {
	if c == nil {
		return ErrDisabled
	}
	if len(msgs) == 0 {
		return nil
	}
	clamped := make([]Message, len(msgs))
	for i, m := range msgs {
		m.Content = clamp(m.Content, maxMessageChars)
		clamped[i] = m
	}
	body := map[string]any{"messages": clamped}
	return c.do(ctx, http.MethodPost, c.wsPath("/sessions/%s/messages", sessionID), nil, body, nil)
}

// SessionContext assembles context for a session: recent messages, an
// optional summary, and a peer representation.
func (c *Client) SessionContext(ctx context.Context, sessionID string, opts ContextOptions) (*SessionContext, error) {
	if c == nil {
		return nil, ErrDisabled
	}
	q := url.Values{}
	if opts.Tokens > 0 {
		q.Set("tokens", strconv.Itoa(opts.Tokens))
	}
	if opts.SearchQuery != "" {
		q.Set("search_query", opts.SearchQuery)
	}
	if opts.IncludeSummary != nil {
		q.Set("summary", strconv.FormatBool(*opts.IncludeSummary))
	}
	if opts.PeerPerspective != "" {
		q.Set("peer_perspective", opts.PeerPerspective)
	}
	if opts.PeerTarget != "" {
		q.Set("peer_target", opts.PeerTarget)
	}
	if opts.LimitToSession {
		q.Set("limit_to_session", "true")
	}
	if opts.SearchTopK > 0 {
		q.Set("search_top_k", strconv.Itoa(opts.SearchTopK))
	}
	if opts.SearchMaxDistance > 0 {
		q.Set("search_max_distance", strconv.FormatFloat(opts.SearchMaxDistance, 'f', -1, 64))
	}
	if opts.IncludeMostFrequent {
		q.Set("include_most_frequent", "true")
	}
	if opts.MaxConclusions > 0 {
		q.Set("max_conclusions", strconv.Itoa(opts.MaxConclusions))
	}

	var out SessionContext
	if err := c.do(ctx, http.MethodGet, c.wsPath("/sessions/%s/context", sessionID), q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Summaries returns the short and long summaries Honcho maintains for
// a session.
func (c *Client) Summaries(ctx context.Context, sessionID string) (*Summaries, error) {
	if c == nil {
		return nil, ErrDisabled
	}
	var out Summaries
	if err := c.do(ctx, http.MethodGet, c.wsPath("/sessions/%s/summaries", sessionID), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PeerContext returns a peer's standing representation and peer card.
func (c *Client) PeerContext(ctx context.Context, peerID string, opts PeerContextOptions) (*PeerContext, error) {
	if c == nil {
		return nil, ErrDisabled
	}
	q := url.Values{}
	if opts.Target != "" {
		q.Set("target", opts.Target)
	}
	if opts.SearchQuery != "" {
		q.Set("search_query", opts.SearchQuery)
	}
	if opts.SearchTopK > 0 {
		q.Set("search_top_k", strconv.Itoa(opts.SearchTopK))
	}
	if opts.SearchMaxDistance > 0 {
		q.Set("search_max_distance", strconv.FormatFloat(opts.SearchMaxDistance, 'f', -1, 64))
	}
	if opts.IncludeMostFrequent {
		q.Set("include_most_frequent", "true")
	}
	if opts.MaxConclusions > 0 {
		q.Set("max_conclusions", strconv.Itoa(opts.MaxConclusions))
	}

	var out PeerContext
	if err := c.do(ctx, http.MethodGet, c.wsPath("/peers/%s/context", peerID), q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Chat queries a peer's representation in natural language. This is
// the dialectic endpoint: it reasons rather than retrieves, so it is
// the slowest call in this package.
func (c *Client) Chat(ctx context.Context, peerID string, q ChatQuery) (string, error) {
	if c == nil {
		return "", ErrDisabled
	}
	// Streaming would need a different response path; callers that
	// want it should use a dedicated method rather than have this one
	// silently return a partial body.
	q.Stream = false
	var out ChatResponse
	if err := c.do(ctx, http.MethodPost, c.wsPath("/peers/%s/chat", peerID), nil, q, &out); err != nil {
		return "", err
	}
	return out.Content, nil
}

// SearchSession performs a search over the messages in one session.
func (c *Client) SearchSession(ctx context.Context, sessionID string, q SearchQuery) ([]Message, error) {
	if c == nil {
		return nil, ErrDisabled
	}
	var out page[Message]
	if err := c.do(ctx, http.MethodPost, c.wsPath("/sessions/%s/search", sessionID), nil, q, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// CreateConclusions writes durable conclusions directly, bypassing
// background derivation.
func (c *Client) CreateConclusions(ctx context.Context, concl []Conclusion) error {
	if c == nil {
		return ErrDisabled
	}
	if len(concl) == 0 {
		return nil
	}
	body := map[string]any{"conclusions": concl}
	return c.do(ctx, http.MethodPost, c.wsPath("/conclusions"), nil, body, nil)
}

// QueryConclusions searches stored conclusions semantically.
func (c *Client) QueryConclusions(ctx context.Context, q ConclusionQuery) ([]Conclusion, error) {
	if c == nil {
		return nil, ErrDisabled
	}
	var out page[Conclusion]
	if err := c.do(ctx, http.MethodPost, c.wsPath("/conclusions/query"), nil, q, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// QueueStatus reports the background reasoning backlog, optionally
// scoped to a session.
func (c *Client) QueueStatus(ctx context.Context, sessionID string) (*QueueStatus, error) {
	if c == nil {
		return nil, ErrDisabled
	}
	q := url.Values{}
	if sessionID != "" {
		q.Set("session_id", sessionID)
	}
	var out QueueStatus
	if err := c.do(ctx, http.MethodGet, c.wsPath("/queue/status"), q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// page is Honcho's paginated envelope. Only the first page is read:
// every list call this package makes is explicitly limited, so
// following cursors would just add latency to a bounded result.
//
// Not every collection endpoint is paginated — search returns a bare
// array — so the envelope accepts either shape rather than making the
// caller know which is which.
type page[T any] struct {
	Items []T `json:"items"`
}

func (p *page[T]) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return json.Unmarshal(data, &p.Items)
	}
	// Alias to avoid recursing back into this method.
	type envelope struct {
		Items []T `json:"items"`
	}
	var e envelope
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	p.Items = e.Items
	return nil
}

// clamp truncates s to at most n bytes, cutting on a rune boundary so
// the result stays valid UTF-8. Honcho's limit is in characters, so
// bounding bytes is conservative and never over-truncates a message
// the API would have accepted.
func clamp(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	cut := s[:n]
	// Drop a trailing partial rune. A real U+FFFD decodes with size
	// 3, so only size <= 1 indicates a truncated sequence.
	for len(cut) > 0 {
		if r, size := utf8.DecodeLastRuneInString(cut); r != utf8.RuneError || size > 1 {
			break
		}
		cut = cut[:len(cut)-1]
	}
	return cut
}

// logFailure records a Honcho error without surfacing it. Every read
// path in this integration is best-effort: memory being unavailable
// degrades to no memory, never to a broken turn.
func logFailure(op string, err error) {
	if err == nil || errors.Is(err, ErrDisabled) || errors.Is(err, context.Canceled) {
		return
	}
	slog.Debug("Honcho request failed", "op", op, "error", err)
}
