package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// buildOAuthHandler creates an auth.OAuthHandler from the given MCP
// OAuth configuration. The handler implements the MCP authorization-code
// flow: metadata discovery, PKCE, optional client registration, and
// token exchange. Tokens are persisted to disk so that subsequent
// sessions can reuse them without re-prompting the user.
func buildOAuthHandler(serverName string, serverURL string, cfg *config.MCPOAuthConfig) (auth.OAuthHandler, error) {
	fetcher, redirectURL, err := newCallbackServer(cfg.CallbackPort)
	if err != nil {
		return nil, fmt.Errorf("oauth callback server: %w", err)
	}

	handlerCfg := &auth.AuthorizationCodeHandlerConfig{
		RedirectURL:              redirectURL,
		AuthorizationCodeFetcher: fetcher,
		Client: &http.Client{
			Transport: newMetadataFixupRoundTripper(http.DefaultTransport),
		},
	}

	registrationCfg := buildRegistrationConfig(cfg, redirectURL)
	handlerCfg.ClientIDMetadataDocumentConfig = registrationCfg.ClientIDMetadataDocumentConfig
	handlerCfg.PreregisteredClient = registrationCfg.PreregisteredClient
	handlerCfg.DynamicClientRegistrationConfig = registrationCfg.DynamicClientRegistrationConfig

	inner, err := auth.NewAuthorizationCodeHandler(handlerCfg)
	if err != nil {
		return nil, fmt.Errorf("oauth handler: %w", err)
	}

	store := newTokenStore(serverName)

	return &persistentOAuthHandler{
		inner:  inner,
		store:  store,
		server: serverURL,
	}, nil
}

// buildRegistrationConfig populates the client registration fields of
// the handler config based on which method the user configured.
func buildRegistrationConfig(cfg *config.MCPOAuthConfig, redirectURL string) auth.AuthorizationCodeHandlerConfig {
	switch {
	case cfg.ClientIDMetadataURL != "":
		return auth.AuthorizationCodeHandlerConfig{
			ClientIDMetadataDocumentConfig: &auth.ClientIDMetadataDocumentConfig{
				URL: cfg.ClientIDMetadataURL,
			},
		}
	case cfg.ClientID != "":
		creds := &oauthex.ClientCredentials{
			ClientID: cfg.ClientID,
		}
		if cfg.ClientSecret != "" {
			creds.ClientSecretAuth = &oauthex.ClientSecretAuth{
				ClientSecret: cfg.ClientSecret,
			}
		}
		return auth.AuthorizationCodeHandlerConfig{
			PreregisteredClient: creds,
		}
	default:
		return auth.AuthorizationCodeHandlerConfig{
			DynamicClientRegistrationConfig: &auth.DynamicClientRegistrationConfig{
				Metadata: &oauthex.ClientRegistrationMetadata{
					ClientName:              "Crush MCP Client",
					RedirectURIs:            []string{redirectURL},
					TokenEndpointAuthMethod: "none",
					GrantTypes:              []string{"authorization_code", "refresh_token"},
					ResponseTypes:           []string{"code"},
				},
			},
		}
	}
}

// persistentOAuthHandler wraps the SDK's AuthorizationCodeHandler to
// add token persistence. On startup, if a valid token is on disk, it
// is used directly (avoiding the browser flow). After a successful
// Authorize call, the new token is saved.
type persistentOAuthHandler struct {
	inner  *auth.AuthorizationCodeHandler
	store  *tokenStore
	server string

	mu     sync.Mutex
	source oauth2.TokenSource
}

func (h *persistentOAuthHandler) TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.source != nil {
		slog.Debug("MCP OAuth TokenSource: returning active source", "server", h.server)
		return h.source, nil
	}

	if token, err := h.store.Load(); err == nil && token != nil && token.Valid() {
		slog.Info("Using persisted MCP OAuth token", "server", h.server)
		h.source = oauth2.StaticTokenSource(token)
		return h.source, nil
	}

	slog.Debug("MCP OAuth TokenSource: returning nil (no token yet)", "server", h.server)
	return nil, nil
}

func (h *persistentOAuthHandler) Authorize(ctx context.Context, req *http.Request, resp *http.Response) error {
	slog.Info("MCP OAuth authorization triggered", "server", h.server, "status", resp.StatusCode)

	if err := h.inner.Authorize(ctx, req, resp); err != nil {
		slog.Warn("MCP OAuth authorization failed", "server", h.server, "error", err)
		return err
	}

	slog.Info("MCP OAuth authorization succeeded", "server", h.server)

	ts, err := h.inner.TokenSource(ctx)
	if err != nil {
		slog.Warn("Failed to get token source after OAuth authorization", "error", err)
		return nil
	}
	if ts == nil {
		slog.Warn("Token source is nil after OAuth authorization", "server", h.server)
		return nil
	}

	token, err := ts.Token()
	if err != nil {
		slog.Warn("Failed to get token after OAuth authorization", "error", err)
		return nil
	}
	if token == nil {
		slog.Warn("Token is nil after OAuth authorization", "server", h.server)
		return nil
	}

	slog.Info("MCP OAuth token obtained",
		"server", h.server,
		"token_type", token.TokenType,
		"has_refresh", token.RefreshToken != "",
		"expiry", token.Expiry,
	)

	h.mu.Lock()
	h.source = &persistingTokenSource{
		base: ts,
		save: h.store.Save,
	}
	h.mu.Unlock()

	if err := h.store.Save(token); err != nil {
		slog.Warn("Failed to persist MCP OAuth token", "error", err)
	}

	return nil
}

// persistingTokenSource wraps an oauth2.TokenSource and saves the
// token whenever a new one is obtained (i.e. after a refresh).
type persistingTokenSource struct {
	base      oauth2.TokenSource
	save      func(*oauth2.Token) error
	lastToken *oauth2.Token
	saveMu    sync.Mutex
}

func (s *persistingTokenSource) Token() (*oauth2.Token, error) {
	token, err := s.base.Token()
	if err != nil {
		return nil, err
	}

	s.saveMu.Lock()
	if s.lastToken == nil || token.AccessToken != s.lastToken.AccessToken {
		s.lastToken = token
		if saveErr := s.save(token); saveErr != nil {
			slog.Warn("Failed to save refreshed MCP OAuth token", "error", saveErr)
		}
	}
	s.saveMu.Unlock()

	return token, nil
}

// tokenStore handles reading and writing OAuth tokens to disk.
// Tokens are stored as JSON in the Crush data directory under
// mcp-tokens/<server-name>.json.
type tokenStore struct {
	path string
}

func newTokenStore(serverName string) *tokenStore {
	dir := filepath.Join(filepath.Dir(config.GlobalConfigData()), "mcp-tokens")
	return &tokenStore{
		path: filepath.Join(dir, fmt.Sprintf("%s.json", serverName)),
	}
}

func (s *tokenStore) Load() (*oauth2.Token, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}
	return &token, nil
}

func (s *tokenStore) Save(token *oauth2.Token) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create token dir: %w", err)
	}
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}
	return os.WriteFile(s.path, data, 0o600)
}

// callbackResult is sent from the HTTP handler to the waiting fetcher.
type callbackResult struct {
	code  string
	state string
	err   error
}

// callbackServer runs a local HTTP server that receives the OAuth
// redirect after the user authorizes in their browser.
type callbackServer struct {
	resultCh chan callbackResult
	srv      *http.Server
}

// newCallbackServer starts a local HTTP server and returns an
// auth.AuthorizationCodeFetcher function that opens the browser and
// waits for the callback, along with the redirect URL to register.
func newCallbackServer(port int) (auth.AuthorizationCodeFetcher, string, error) {
	cs := &callbackServer{
		resultCh: make(chan callbackResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", cs.handleCallback)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, "", fmt.Errorf("listen on port %d: %w", port, err)
	}

	listenPort := ln.Addr().(*net.TCPAddr).Port
	cs.srv = &http.Server{Handler: mux}

	go func() {
		if err := cs.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Warn("MCP OAuth callback server error", "error", err)
		}
	}()

	redirectURL := fmt.Sprintf("http://localhost:%d/callback", listenPort)

	fetcher := auth.AuthorizationCodeFetcher(func(ctx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
		authURL := stripResourceParam(args.URL)
		slog.Info("Opening browser for MCP OAuth authorization", "url", authURL)
		if err := browser.OpenURL(authURL); err != nil {
			slog.Warn("Failed to open browser for OAuth, please open the URL manually", "url", authURL, "error", err)
		}

		select {
		case result := <-cs.resultCh:
			cs.shutdown()
			if result.err != nil {
				return nil, result.err
			}
			return &auth.AuthorizationResult{
				Code:  result.code,
				State: result.state,
			}, nil
		case <-ctx.Done():
			cs.shutdown()
			return nil, ctx.Err()
		}
	})

	return fetcher, redirectURL, nil
}

func (cs *callbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	code := query.Get("code")
	state := query.Get("state")

	if errParam := query.Get("error"); errParam != "" {
		errorDesc := query.Get("error_description")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "<h1>Authorization failed</h1><p>%s: %s</p>", errParam, errorDesc)
		cs.resultCh <- callbackResult{err: fmt.Errorf("%s: %s", errParam, errorDesc)}
		return
	}

	if code == "" {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "<h1>Missing authorization code</h1>")
		cs.resultCh <- callbackResult{err: fmt.Errorf("missing authorization code")}
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "<h1>Authorization successful</h1><p>You can close this tab and return to Crush.</p>")

	cs.resultCh <- callbackResult{code: code, state: state}
}

func (cs *callbackServer) shutdown() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = cs.srv.Shutdown(shutdownCtx)
}

// oauthRoundTripper injects a bearer token from an OAuth token source
// into outgoing requests. Used for SSE transports that don't support
// the OAuthHandler interface natively.
type oauthRoundTripper struct {
	base    http.RoundTripper
	handler auth.OAuthHandler
}

func newOAuthRoundTripperWithBase(handler auth.OAuthHandler, base http.RoundTripper) *oauthRoundTripper {
	return &oauthRoundTripper{base: base, handler: handler}
}

func (rt *oauthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	resp, err := rt.doRequestWithToken(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		if authErr := rt.handler.Authorize(ctx, req, resp); authErr != nil {
			return resp, nil
		}
		resp.Body.Close()

		return rt.doRequestWithToken(ctx, req.Clone(ctx))
	}

	return resp, nil
}

func (rt *oauthRoundTripper) doRequestWithToken(ctx context.Context, req *http.Request) (*http.Response, error) {
	ts, err := rt.handler.TokenSource(ctx)
	if err != nil {
		return nil, fmt.Errorf("oauth token source: %w", err)
	}

	if ts != nil {
		token, err := ts.Token()
		if err == nil && token != nil {
			req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		}
	}

	return rt.base.RoundTrip(req)
}

// metadataFixupRoundTripper intercepts responses from OAuth metadata
// endpoints (/.well-known/oauth-authorization-server and
// /.well-known/oauth-protected-resource) and normalizes the "issuer"
// field to strip trailing slashes. Some OAuth servers return an issuer
// with a trailing slash that doesn't match the URL the metadata was
// fetched from, causing the SDK's strict RFC 8414 validation to fail.
type metadataFixupRoundTripper struct {
	base http.RoundTripper
}

func newMetadataFixupRoundTripper(base http.RoundTripper) *metadataFixupRoundTripper {
	return &metadataFixupRoundTripper{base: base}
}

func (rt *metadataFixupRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := rt.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if !isMetadataEndpoint(req.URL.Path) {
		return resp, nil
	}

	if resp.StatusCode != http.StatusOK || resp.Body == nil {
		return resp, nil
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read metadata response: %w", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	changed := false
	if issuer, ok := raw["issuer"].(string); ok && strings.HasSuffix(issuer, "/") {
		raw["issuer"] = strings.TrimSuffix(issuer, "/")
		changed = true
	}

	if !changed {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	fixed, err := json.Marshal(raw)
	if err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	slog.Debug("Normalized OAuth metadata issuer trailing slash", "url", req.URL.String())
	resp.Body = io.NopCloser(bytes.NewReader(fixed))
	resp.ContentLength = int64(len(fixed))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(fixed)))
	return resp, nil
}

func isMetadataEndpoint(path string) bool {
	return strings.Contains(path, "/.well-known/oauth-authorization-server") ||
		strings.Contains(path, "/.well-known/oauth-protected-resource")
}

// stripResourceParam removes the "resource" query parameter from an
// authorization URL. Some authorization servers reject the "resource"
// parameter (RFC 8707) in the authorization request with server_error.
// However, they DO support it in the token exchange, which is needed
// to scope the access token to the correct resource. So we strip it
// from the authorization URL only.
func stripResourceParam(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	if q.Has("resource") {
		q.Del("resource")
		u.RawQuery = q.Encode()
		slog.Debug("Stripped resource parameter from authorization URL")
	}
	return u.String()
}
