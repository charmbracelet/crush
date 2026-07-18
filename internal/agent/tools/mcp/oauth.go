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

// oauthInteractiveTimeout bounds the interactive browser authorization flow.
// It is deliberately much longer than the MCP connection timeout because the
// user has to switch to a browser, sign in, and grant consent.
const oauthInteractiveTimeout = 3 * time.Minute

// oauthEndpoints holds the authorization and token endpoints needed to rebuild
// an oauth2 config (and therefore refresh a token) on a later startup.
type oauthEndpoints struct {
	AuthURL  string `json:"auth_url"`
	TokenURL string `json:"token_url"`
}

// mcptoken stores a serialised OAuth token plus the client registration
// info needed to refresh it without going through the browser again.
type mcptoken struct {
	Token        *oauth2.Token  `json:"token"`
	ClientID     string         `json:"client_id,omitempty"`
	ClientSecret string         `json:"client_secret,omitempty"`
	AuthStyle    int            `json:"auth_style,omitempty"`
	Endpoints    oauthEndpoints `json:"endpoints"`
}

// tokenStore persists MCP OAuth tokens keyed by server URL. It is safe
// for concurrent access.
type tokenStore struct {
	mu   sync.Mutex
	path string
	data map[string]mcptoken
}

// storeCache holds one tokenStore per on-disk path so that every OAuth handler
// in the process shares a single store (and mutex). Without this, each handler
// would load an independent snapshot of the token file and later rewrite the
// whole file, letting one server's save erase another server's entry.
var (
	storeCacheMu sync.Mutex
	storeCache   = map[string]*tokenStore{}
)

// sharedTokenStore returns the process-wide tokenStore for the current crush
// config directory, creating and loading it on first use.
func sharedTokenStore() *tokenStore {
	// globalConfigPath already returns the crush config directory, so the
	// token file sits alongside crush.json (e.g. ~/.config/crush/). Do not
	// take filepath.Dir again — that would drop the file a level too high
	// (e.g. ~/.config/).
	path := filepath.Join(globalConfigPath(), tokenFileName)

	storeCacheMu.Lock()
	defer storeCacheMu.Unlock()
	if s, ok := storeCache[path]; ok {
		return s
	}
	s := &tokenStore{
		path: path,
		data: make(map[string]mcptoken),
	}
	s.load()
	storeCache[path] = s
	return s
}

func (s *tokenStore) load() {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	// Merge any entries written to disk since we last read it — another MCP
	// server authorizing concurrently, or a second crush process — so we do
	// not clobber them by writing back a stale snapshot.
	if b, err := os.ReadFile(s.path); err == nil {
		var onDisk map[string]mcptoken
		if json.Unmarshal(b, &onDisk) == nil {
			for k, v := range onDisk {
				if _, ok := s.data[k]; !ok {
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
	_ = os.WriteFile(s.path, b, 0o600)
}

// mcpOAuthHandler implements auth.OAuthHandler for MCP HTTP servers.
// It delegates the heavy lifting to the SDK's AuthorizationCodeHandler
// and persists tokens across restarts.
type mcpOAuthHandler struct {
	serverURL string
	store     *tokenStore
	// stopConnTimeout stops the MCP connection timeout timer, if any. The
	// interactive browser flow can take minutes, far longer than the
	// connection timeout, so we suspend that timeout once authorization
	// actually starts. It may be nil (e.g. in tests).
	stopConnTimeout func()

	mu       sync.Mutex
	tokenSrc oauth2.TokenSource
}

var _ auth.OAuthHandler = (*mcpOAuthHandler)(nil)

func newMCPOAuthHandler(serverURL string, stopConnTimeout func()) *mcpOAuthHandler {
	store := sharedTokenStore()
	h := &mcpOAuthHandler{
		serverURL:       serverURL,
		store:           store,
		stopConnTimeout: stopConnTimeout,
	}
	// Restore any saved token so we can skip the browser flow on
	// subsequent startups. The client ID, secret and endpoints are
	// persisted alongside the token so the oauth2 library can refresh it
	// when it expires.
	if saved, ok := store.get(serverURL); ok && saved.Token != nil {
		cfg := &oauth2.Config{
			ClientID:     saved.ClientID,
			ClientSecret: saved.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:   saved.Endpoints.AuthURL,
				TokenURL:  saved.Endpoints.TokenURL,
				AuthStyle: oauth2.AuthStyle(saved.AuthStyle),
			},
		}
		h.tokenSrc = cfg.TokenSource(context.Background(), saved.Token)
	} else {
		slog.Debug("No saved MCP OAuth token found", "server", serverURL)
	}
	return h
}

// TokenSource implements auth.OAuthHandler.
func (h *mcpOAuthHandler) TokenSource(_ context.Context) (oauth2.TokenSource, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.tokenSrc, nil
}

// clientRegistration is the result of discovering the authorization server and
// dynamically registering a client with it. Capturing the registered client ID
// ourselves (rather than letting the SDK register internally) is what lets us
// persist enough information to refresh the token later.
type clientRegistration struct {
	clientID     string
	clientSecret string
	authURL      string
	tokenURL     string
	authStyle    oauth2.AuthStyle
}

// Authorize implements auth.OAuthHandler. It performs the full OAuth
// authorization-code flow (discovery, registration, browser, PKCE,
// token exchange) then persists the resulting token.
func (h *mcpOAuthHandler) Authorize(ctx context.Context, req *http.Request, resp *http.Response) error {
	// Mirror the SDK: a 403 that is not an insufficient_scope challenge is a
	// genuine permission error, not an authorization prompt. Return nil so the
	// request is retried and the real error surfaces — without registering a
	// throwaway client or discarding a previously restored token.
	if resp.StatusCode == http.StatusForbidden && !isInsufficientScope(resp) {
		resp.Body.Close()
		return nil
	}

	// Interactive OAuth (browser login + consent) routinely takes longer than
	// the MCP connection timeout that bounds ctx. Suspend that timeout so the
	// flow is not cancelled mid-login, then run under our own, more generous
	// deadline.
	if h.stopConnTimeout != nil {
		h.stopConnTimeout()
	}
	authCtx, cancel := context.WithTimeout(ctx, oauthInteractiveTimeout)
	defer cancel()

	port, err := allocateCallbackPort(authCtx)
	if err != nil {
		resp.Body.Close()
		return err
	}
	redirectURL := fmt.Sprintf("http://localhost:%d/callback", port)

	// Register a client ourselves so we know its ID/secret and can persist
	// them for refresh. The SDK's internal dynamic registration never exposes
	// the resulting client ID.
	reg, err := h.discoverAndRegister(authCtx, redirectURL)
	if err != nil {
		resp.Body.Close()
		return err
	}

	inner, err := h.buildInner(reg, port, redirectURL)
	if err != nil {
		resp.Body.Close()
		return err
	}
	// inner.Authorize takes ownership of resp and closes its body.
	if err := inner.Authorize(authCtx, req, resp); err != nil {
		return err
	}

	ts, _ := inner.TokenSource(authCtx)
	if ts == nil {
		// The SDK short-circuits non-OAuth 403s (e.g. a genuine permission
		// error, not an auth challenge) by returning nil without establishing
		// a token source. Don't discard any previously restored token in that
		// case.
		return nil
	}

	// After a successful Authorize the inner handler holds a fresh token
	// source. Cache it and persist to disk.
	h.mu.Lock()
	h.tokenSrc = ts
	h.mu.Unlock()
	if tok, err := ts.Token(); err == nil {
		h.store.save(h.serverURL, mcptoken{
			Token:        tok,
			ClientID:     reg.clientID,
			ClientSecret: reg.clientSecret,
			AuthStyle:    int(reg.authStyle),
			Endpoints: oauthEndpoints{
				AuthURL:  reg.authURL,
				TokenURL: reg.tokenURL,
			},
		})
	}
	return nil
}

// buildInner creates the SDK AuthorizationCodeHandler using the client we
// already registered, so that the token is issued under a client ID we can
// persist and later refresh with.
func (h *mcpOAuthHandler) buildInner(reg *clientRegistration, port int, redirectURL string) (*auth.AuthorizationCodeHandler, error) {
	creds := &oauthex.ClientCredentials{ClientID: reg.clientID}
	if reg.clientSecret != "" {
		creds.ClientSecretAuth = &oauthex.ClientSecretAuth{ClientSecret: reg.clientSecret}
	}
	cfg := &auth.AuthorizationCodeHandlerConfig{
		PreregisteredClient: creds,
		RedirectURL:         redirectURL,
		AuthorizationCodeFetcher: func(fetchCtx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
			return openBrowserAndCapture(fetchCtx, args.URL, port, h.serverURL)
		},
	}
	return auth.NewAuthorizationCodeHandler(cfg)
}

// discoverAndRegister discovers the authorization server for the MCP server and
// dynamically registers a client with it, returning the client credentials and
// endpoints needed to complete and later refresh the flow.
func (h *mcpOAuthHandler) discoverAndRegister(ctx context.Context, redirectURL string) (*clientRegistration, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	asm, err := h.discoverAuthServer(ctx, client)
	if err != nil {
		return nil, err
	}
	if asm.RegistrationEndpoint == "" {
		return nil, fmt.Errorf("authorization server %q does not support dynamic client registration", asm.Issuer)
	}
	meta := &oauthex.ClientRegistrationMetadata{
		ClientName:              "Crush",
		RedirectURIs:            []string{redirectURL},
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "none",
		Scope:                   "mcp",
	}
	reg, err := oauthex.RegisterClient(ctx, asm.RegistrationEndpoint, meta, client)
	if err != nil {
		return nil, fmt.Errorf("dynamic client registration failed: %w", err)
	}
	if reg.ClientID == "" {
		return nil, fmt.Errorf("dynamic client registration returned an empty client_id")
	}
	return &clientRegistration{
		clientID:     reg.ClientID,
		clientSecret: reg.ClientSecret,
		authURL:      asm.AuthorizationEndpoint,
		tokenURL:     asm.TokenEndpoint,
		authStyle:    authStyleForRegistration(reg, asm),
	}, nil
}

// discoverAuthServer resolves the OAuth authorization server metadata for the
// MCP server URL, following RFC 9728 protected-resource-metadata discovery and
// falling back to treating the server origin as the authorization server.
func (h *mcpOAuthHandler) discoverAuthServer(ctx context.Context, client *http.Client) (*oauthex.AuthServerMeta, error) {
	u, err := url.Parse(h.serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid MCP server URL %q: %w", h.serverURL, err)
	}
	origin := fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	// Try RFC 9728 protected resource metadata.
	prmURLs := []string{
		fmt.Sprintf("%s/.well-known/oauth-protected-resource/%s", origin, strings.TrimLeft(u.Path, "/")),
		fmt.Sprintf("%s/.well-known/oauth-protected-resource", origin),
	}
	for _, pURL := range prmURLs {
		prm, err := oauthex.GetProtectedResourceMetadata(ctx, pURL, h.serverURL, client)
		if err != nil || prm == nil || len(prm.AuthorizationServers) == 0 {
			continue
		}
		asm, err := auth.GetAuthServerMetadata(ctx, prm.AuthorizationServers[0], client)
		if err != nil || asm == nil {
			continue
		}
		return asm, nil
	}

	// Fallback: server root as authorization server.
	asm, err := auth.GetAuthServerMetadata(ctx, origin, client)
	if err != nil {
		return nil, fmt.Errorf("could not discover OAuth authorization server for %s: %w", h.serverURL, err)
	}
	if asm == nil {
		return nil, fmt.Errorf("no OAuth authorization server metadata found for %s", h.serverURL)
	}
	return asm, nil
}

// isInsufficientScope reports whether the response's WWW-Authenticate header
// carries a Bearer "insufficient_scope" error, i.e. a step-up authorization
// prompt rather than a hard permission denial.
func isInsufficientScope(resp *http.Response) bool {
	challenges, err := oauthex.ParseWWWAuthenticate(resp.Header[http.CanonicalHeaderKey("WWW-Authenticate")])
	if err != nil {
		return false
	}
	for _, c := range challenges {
		if c.Scheme == "bearer" && c.Params["error"] == "insufficient_scope" {
			return true
		}
	}
	return false
}

// authStyleForRegistration mirrors the SDK's choice of how the client
// authenticates at the token endpoint, so the persisted config refreshes the
// same way the initial exchange happened.
func authStyleForRegistration(reg *oauthex.ClientRegistrationResponse, asm *oauthex.AuthServerMeta) oauth2.AuthStyle {
	if reg.ClientSecret == "" {
		// Public client (PKCE): the client_id is sent as a request parameter.
		return oauth2.AuthStyleInParams
	}
	for _, m := range asm.TokenEndpointAuthMethodsSupported {
		switch m {
		case "client_secret_post":
			return oauth2.AuthStyleInParams
		case "client_secret_basic":
			return oauth2.AuthStyleInHeader
		}
	}
	return oauth2.AuthStyleAutoDetect
}

// allocateCallbackPort reserves a free localhost port for the OAuth callback
// server. The listener is closed immediately; the callback server re-listens on
// the same port.
func allocateCallbackPort(ctx context.Context) (int, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to allocate callback port: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port, nil
}

// openBrowserAndCapture opens the authorization URL in the user's
// browser and listens on the given port for the OAuth callback redirect.
func openBrowserAndCapture(ctx context.Context, authURL string, port int, serverName string) (*auth.AuthorizationResult, error) {
	resultCh := make(chan auth.AuthorizationResult, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if codeErr := q.Get("error"); codeErr != "" {
			desc := q.Get("error_description")
			if desc == "" {
				desc = codeErr
			}
			fmt.Fprintf(w, "Authorization failed: %s", desc)
			errCh <- fmt.Errorf("authorization error: %s", desc)
			return
		}
		code := q.Get("code")
		state := q.Get("state")
		if code == "" {
			http.Error(w, "missing code parameter", http.StatusBadRequest)
			errCh <- fmt.Errorf("callback missing code parameter")
			return
		}
		fmt.Fprint(w, "Authorization successful. You can close this tab and return to Crush.")
		resultCh <- auth.AuthorizationResult{Code: code, State: state}
	})

	srv := &http.Server{Handler: mux}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on callback port: %w", err)
	}
	go srv.Serve(ln)
	defer srv.Shutdown(context.Background())

	slog.Info("Opening browser for MCP server authorization", "url", authURL)
	if err := browser.OpenURL(authURL); err != nil {
		// If the browser can't be opened (headless, remote SSH, etc.), keep the
		// callback listener running and tell the user to open the URL manually.
		// Returning here would tear down the listener and make manual
		// completion impossible.
		slog.Warn("Could not open browser for MCP OAuth; open this URL manually to authorize",
			"server", serverName,
			"url", authURL,
			"error", err,
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
