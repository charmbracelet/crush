package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"github.com/pkg/browser"
	"golang.org/x/oauth2"
)

// tokenFileName is the file where MCP OAuth tokens are persisted.
const tokenFileName = "mcp-oauth-tokens.json"

// oauthEndpoints holds the authorization-server endpoints persisted alongside
// a token so the token source can be rebuilt on restart.
type oauthEndpoints struct {
	AuthURL  string `json:"auth_url"`
	TokenURL string `json:"token_url"`
}

// mcptoken stores a serialised OAuth token plus the client registration
// info needed to refresh it.
type mcptoken struct {
	Token     *oauth2.Token  `json:"token"`
	ClientID  string         `json:"client_id,omitempty"`
	Endpoints oauthEndpoints `json:"endpoints"`
}

// tokenFileMu serializes token-file access across ALL tokenStore
// instances. Each HTTP MCP server's handler owns its own store; with only
// the per-store mutex, two handlers refreshing concurrently could both
// read the file before either wrote it, and the last writer erased the
// other's just-saved token — with OAuth 2.1 refresh-token rotation the
// surviving on-disk copy is the invalidated one, forcing a browser
// re-auth on the next restart.
var tokenFileMu sync.Mutex

// tokenStore persists MCP OAuth tokens keyed by server URL. It is safe
// for concurrent access.
type tokenStore struct {
	mu   sync.Mutex
	path string
	data map[string]mcptoken
}

func newTokenStore() *tokenStore {
	// globalConfigPath already returns the crush config directory, so the
	// token file sits alongside crush.json (e.g. ~/.config/crush/). Do not
	// take filepath.Dir again — that would drop the file a level too high
	// (e.g. ~/.config/).
	dir := globalConfigPath()
	return &tokenStore{
		path: filepath.Join(dir, tokenFileName),
		data: make(map[string]mcptoken),
	}
}

func (s *tokenStore) load() {
	tokenFileMu.Lock()
	defer tokenFileMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, &s.data)
}

func (s *tokenStore) get(serverURL string) (mcptoken, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.data[serverURL]
	return t, ok
}

func (s *tokenStore) save(serverURL string, t mcptoken) {
	tokenFileMu.Lock()
	defer tokenFileMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	// Merge with what's on disk before rewriting the file. Each HTTP MCP
	// server gets its own handler with its own store snapshot; without the
	// merge, server B's save rewrote the whole file from a snapshot that
	// never saw server A's token, erasing it. Disk wins for every server
	// except the one being saved now.
	if b, err := os.ReadFile(s.path); err == nil {
		var onDisk map[string]mcptoken
		if json.Unmarshal(b, &onDisk) == nil {
			for k, v := range onDisk {
				if k != serverURL {
					s.data[k] = v
				}
			}
		}
	}
	s.data[serverURL] = t
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return
	}
	// Persist atomically (temp file + rename) so a crash mid-write can't
	// truncate the token file for EVERY server at once — the corrupt-file
	// load path degrades to "no tokens", which would force a browser
	// re-auth for all of them. Reuses the shared writer in internal/config.
	// A persist failure is logged, not fatal: tokens are regenerable via
	// re-auth, but a silently unwritable config dir is worth surfacing.
	if err := config.AtomicWriteFile(s.path, b, 0o600); err != nil {
		slog.Warn("Failed to persist MCP OAuth tokens", "path", s.path, "err", err)
	}
}

// mcpOAuthHandler implements auth.OAuthHandler for MCP HTTP servers.
// It delegates the heavy lifting to the SDK's AuthorizationCodeHandler
// and persists tokens across restarts.
type mcpOAuthHandler struct {
	serverURL string
	store     *tokenStore
	mu        sync.Mutex
	tokenSrc  oauth2.TokenSource
	// clientID is the dynamically registered OAuth client ID, captured from
	// the authorization URL (the only place the SDK surfaces it). It is
	// persisted so token refresh keeps working across restarts — a public
	// client (TokenEndpointAuthMethod "none") must send client_id on every
	// refresh, so a restart without it re-prompts the browser flow as soon
	// as the access token expires.
	clientID string
	// endpoints caches the discovered authorization-server endpoints so
	// re-persisting a refreshed token doesn't re-run network discovery.
	endpoints oauthEndpoints
}

var _ auth.OAuthHandler = (*mcpOAuthHandler)(nil)

func newMCPOAuthHandler(serverURL string) *mcpOAuthHandler {
	store := newTokenStore()
	store.load()
	h := &mcpOAuthHandler{
		serverURL: serverURL,
		store:     store,
	}
	// Restore any saved token so we can skip the browser flow on
	// subsequent startups.
	if saved, ok := store.get(serverURL); ok && saved.Token != nil {
		h.clientID = saved.ClientID
		h.endpoints = saved.Endpoints
		cfg := &oauth2.Config{
			ClientID: saved.ClientID,
			Endpoint: oauth2.Endpoint{
				AuthURL:  saved.Endpoints.AuthURL,
				TokenURL: saved.Endpoints.TokenURL,
			},
		}
		// Wrap the source so refreshes are re-persisted: OAuth 2.1 public
		// clients rotate the refresh token on use, so a refresh that only
		// lives in memory invalidates the one on disk and the NEXT restart
		// falls back to the browser flow.
		h.tokenSrc = &persistingTokenSource{
			h:    h,
			src:  cfg.TokenSource(context.Background(), saved.Token),
			last: saved.Token,
		}
	} else {
		slog.Debug("No saved MCP OAuth token found", "server", serverURL)
	}
	return h
}

// persistingTokenSource wraps a TokenSource and re-persists the token
// whenever it changes (refresh, rotation), keeping the on-disk copy usable
// across restarts.
type persistingTokenSource struct {
	h    *mcpOAuthHandler
	src  oauth2.TokenSource
	mu   sync.Mutex
	last *oauth2.Token
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.src.Token()
	if err != nil {
		return nil, err
	}
	// Persist while holding p.mu: two Token() calls straddling an expiry
	// could otherwise interleave so the earlier (rotated-out) token is
	// persisted after the newer one. Lock order p.mu -> h.mu -> store.mu
	// has no reverse path.
	p.mu.Lock()
	defer p.mu.Unlock()
	changed := p.last == nil ||
		p.last.AccessToken != tok.AccessToken ||
		p.last.RefreshToken != tok.RefreshToken
	p.last = tok
	if changed {
		p.h.persistToken(tok)
	}
	return tok, nil
}

// TokenSource implements auth.OAuthHandler.
func (h *mcpOAuthHandler) TokenSource(_ context.Context) (oauth2.TokenSource, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.tokenSrc, nil
}

// Authorize implements auth.OAuthHandler. It performs the full OAuth
// authorization-code flow (discovery, registration, browser, PKCE,
// token exchange) then persists the resulting token.
func (h *mcpOAuthHandler) Authorize(ctx context.Context, req *http.Request, resp *http.Response) error {
	// A full re-authorization means the cached endpoints may be stale
	// (the server may have moved them — often why we are re-authorizing
	// at all). Clear the token endpoint so the post-auth persistToken
	// re-runs discovery; captureFromAuthURL refreshes the auth endpoint.
	h.mu.Lock()
	h.endpoints.TokenURL = ""
	h.mu.Unlock()

	inner, ln, err := h.buildInner(ctx)
	if err != nil {
		resp.Body.Close()
		return err
	}
	// The callback server closes ln when it shuts down; this extra Close is
	// a no-op then, and the cleanup path when Authorize fails before the
	// fetcher ever runs.
	defer func() { _ = ln.Close() }()
	if err := inner.Authorize(ctx, req, resp); err != nil {
		return err
	}
	// After a successful Authorize the inner handler holds a fresh
	// token source. Cache it (wrapped so refreshes re-persist) and
	// persist the initial token to disk.
	ts, _ := inner.TokenSource(ctx)
	var wrapped oauth2.TokenSource
	if ts != nil {
		var last *oauth2.Token
		if tok, err := ts.Token(); err == nil {
			h.persistToken(tok)
			last = tok
		}
		wrapped = &persistingTokenSource{h: h, src: ts, last: last}
	}
	h.mu.Lock()
	h.tokenSrc = wrapped
	h.mu.Unlock()
	return nil
}

// buildInner lazily creates the SDK AuthorizationCodeHandler along with the
// live callback listener. The listener is allocated once and handed to the
// callback server directly — allocating a port, closing it, and re-listening
// later would let another process steal the port in between.
func (h *mcpOAuthHandler) buildInner(ctx context.Context) (*auth.AuthorizationCodeHandler, net.Listener, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to allocate callback port: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://localhost:%d/callback", port)

	cfg := &auth.AuthorizationCodeHandlerConfig{
		DynamicClientRegistrationConfig: &auth.DynamicClientRegistrationConfig{
			Metadata: &oauthex.ClientRegistrationMetadata{
				ClientName:              "Crush",
				RedirectURIs:            []string{redirectURL},
				GrantTypes:              []string{"authorization_code", "refresh_token"},
				ResponseTypes:           []string{"code"},
				TokenEndpointAuthMethod: "none",
				Scope:                   "mcp",
			},
		},
		RedirectURL: redirectURL,
		AuthorizationCodeFetcher: func(fetchCtx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
			// The authorization URL is the only place the SDK surfaces the
			// dynamically registered client_id; capture it (and the auth
			// endpoint) for persistence before opening the browser.
			h.captureFromAuthURL(args.URL)
			return openBrowserAndCapture(fetchCtx, args.URL, ln, h.serverURL)
		},
	}
	handler, err := auth.NewAuthorizationCodeHandler(cfg)
	if err != nil {
		_ = ln.Close()
		return nil, nil, err
	}
	return handler, ln, nil
}

// captureFromAuthURL records the registered client_id and the authorization
// endpoint from the authorization URL the SDK built. persistToken saves both
// so the token source — and, critically, token refresh for this public
// client — can be reconstructed after a restart.
func (h *mcpOAuthHandler) captureFromAuthURL(authURL string) {
	u, err := url.Parse(authURL)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if cid := u.Query().Get("client_id"); cid != "" {
		h.clientID = cid
	}
	// Overwrite unconditionally: this runs during a live authorization,
	// which is the freshest possible source. Keeping a cached value here
	// would pin a stale endpoint forever after the AS moves.
	base := *u
	base.RawQuery = ""
	base.Fragment = ""
	h.endpoints.AuthURL = base.String()
}

// persistToken saves the current token to the store, together with the
// registered client ID (captured from the authorization URL) and the
// authorization-server endpoints — everything a restart needs to rebuild a
// refreshable token source. Endpoints are cached on the handler so token
// refreshes don't re-run network discovery.
func (h *mcpOAuthHandler) persistToken(tok *oauth2.Token) {
	entry := mcptoken{Token: tok}
	h.mu.Lock()
	entry.ClientID = h.clientID
	entry.Endpoints = h.endpoints
	h.mu.Unlock()
	if entry.Endpoints.TokenURL == "" {
		// Best-effort: the inner handler doesn't expose its config, so fetch
		// the metadata endpoints once and cache them.
		endpoints := h.discoverEndpoints(context.Background(), h.serverURL)
		h.mu.Lock()
		if endpoints.AuthURL != "" {
			h.endpoints.AuthURL = endpoints.AuthURL
		}
		if endpoints.TokenURL != "" {
			h.endpoints.TokenURL = endpoints.TokenURL
		}
		entry.Endpoints = h.endpoints
		h.mu.Unlock()
	}
	h.store.save(h.serverURL, entry)
}

// discoverEndpoints fetches the OAuth metadata for the server URL to
// extract the authorization and token endpoints. This is used to
// persist enough info to restore the token source on restart.
func (h *mcpOAuthHandler) discoverEndpoints(ctx context.Context, serverURL string) oauthEndpoints {
	result := oauthEndpoints{}
	u, err := url.Parse(serverURL)
	if err != nil {
		return result
	}
	// Try RFC 9728 protected resource metadata.
	prmURLs := []string{
		fmt.Sprintf("%s/.well-known/oauth-protected-resource/%s", fmt.Sprintf("%s://%s", u.Scheme, u.Host), strings.TrimLeft(u.Path, "/")),
		fmt.Sprintf("%s://%s/.well-known/oauth-protected-resource", u.Scheme, u.Host),
	}
	client := &http.Client{Timeout: 10 * time.Second}
	for _, pURL := range prmURLs {
		prm, err := oauthex.GetProtectedResourceMetadata(ctx, pURL, serverURL, client)
		if err != nil || prm == nil || len(prm.AuthorizationServers) == 0 {
			continue
		}
		asm, err := auth.GetAuthServerMetadata(ctx, prm.AuthorizationServers[0], client)
		if err != nil || asm == nil {
			continue
		}
		result.AuthURL = asm.AuthorizationEndpoint
		result.TokenURL = asm.TokenEndpoint
		return result
	}
	// Fallback: server root as authorization server.
	authServer := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	asm, err := auth.GetAuthServerMetadata(ctx, authServer, client)
	if err == nil && asm != nil {
		result.AuthURL = asm.AuthorizationEndpoint
		result.TokenURL = asm.TokenEndpoint
	}
	return result
}

// callbackHandler builds the /callback handler used during the OAuth flow.
// Sends are non-blocking one-shots: the channels are buffered (capacity 1)
// and anything beyond the first delivery is dropped. A duplicate callback
// hit — browser refresh, link prefetcher, AS retry — used to block its
// handler goroutine forever on the full channel, which then wedged
// srv.Shutdown (it waits for active handlers) and leaked the whole
// authorization flow.
//
// expectedState is the state value the SDK baked into the authorization URL.
// When non-empty, a callback whose state parameter does not match is
// rejected with 400 and dropped — it consumes neither the result slot nor
// errCh — so a stray prefetch or browser refresh with the wrong state
// cannot crowd out or abort the legitimate callback, which can still win
// the resultCh race. When empty, state validation is skipped (defensive
// backward-compat for a malformed authorization URL).
func callbackHandler(expectedState string, resultCh chan auth.AuthorizationResult, errCh chan error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if codeErr := q.Get("error"); codeErr != "" {
			desc := q.Get("error_description")
			if desc == "" {
				desc = codeErr
			}
			fmt.Fprintf(w, "Authorization failed: %s", desc)
			select {
			case errCh <- fmt.Errorf("authorization error: %s", desc):
			default:
			}
			return
		}
		code := q.Get("code")
		state := q.Get("state")
		if code == "" {
			http.Error(w, "missing code parameter", http.StatusBadRequest)
			select {
			case errCh <- fmt.Errorf("callback missing code parameter"):
			default:
			}
			return
		}
		if expectedState != "" && state != expectedState {
			// Reject the mismatched callback but do NOT report it on
			// errCh. errCh is a terminal signal for openBrowserAndCapture's
			// select (a delivery there returns an error and tears the flow
			// down), so sending here would let a single stray wrong-state
			// GET abort an in-flight legitimate authorization. Dropping it
			// (400 + return) leaves resultCh untouched so the real callback
			// can still win the race — which is the whole point of
			// validating state. A run where no valid callback ever arrives
			// is still bounded by the caller's context.
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, "Authorization successful. You can close this tab and return to Crush.")
		select {
		case resultCh <- auth.AuthorizationResult{Code: code, State: state}:
		default:
		}
	}
}

// stateFromAuthURL extracts the OAuth state parameter from the authorization
// URL the SDK builds. The SDK generates a random state and bakes it into
// the URL via cfg.AuthCodeURL(state, ...); the authorization server echoes
// it back on the callback redirect. We validate the echoed state against
// this value so a stray request with the wrong state cannot crowd out the
// legitimate callback. Returns "" if the URL is malformed or has no state.
func stateFromAuthURL(authURL string) string {
	u, err := url.Parse(authURL)
	if err != nil {
		return ""
	}
	return u.Query().Get("state")
}

// openBrowserAndCapture opens the authorization URL in the user's browser
// and serves the OAuth callback redirect on the provided listener (the one
// whose port is baked into the redirect URL — reusing it avoids the
// close-and-relisten window in which another process could steal the port).
func openBrowserAndCapture(ctx context.Context, authURL string, ln net.Listener, serverName string) (*auth.AuthorizationResult, error) {
	resultCh := make(chan auth.AuthorizationResult, 1)
	errCh := make(chan error, 1)

	// Echo the SDK-generated state through to the callback handler so a
	// stray /callback hit (browser refresh, prefetcher) with the wrong
	// state cannot crowd out the legitimate callback. PKCE already
	// prevents code injection at the token exchange; this is cheap
	// defense in depth and makes the duplicate-hit path deterministic.
	expectedState := stateFromAuthURL(authURL)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", callbackHandler(expectedState, resultCh, errCh))

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Shutdown(context.Background())

	slog.Info("Opening browser for MCP server authorization", "url", authURL)
	if err := browser.OpenURL(authURL); err != nil {
		// If the browser can't be opened (headless, remote SSH, etc.), return
		// a user-facing error containing the exact URL so the TUI can show it.
		return nil, fmt.Errorf(
			"could not open browser for MCP OAuth (%s): %w\nopen this URL manually to continue:\n%s",
			serverName,
			err,
			authURL,
		)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case result := <-resultCh:
		return &result, nil
	}
}

// globalConfigPath returns the directory containing the global crush
// config file. The token store lives alongside it.
func globalConfigPath() string {
	return filepath.Dir(config.GlobalConfig())
}
