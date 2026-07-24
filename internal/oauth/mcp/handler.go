package mcpoauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"github.com/pkg/browser"
	"golang.org/x/oauth2"
)

// ErrInteractiveAuthRequired is returned by Authorize when a server needs
// interactive (browser) authorization but the current context does not
// permit it. Background connections such as startup deliberately withhold
// permission so a failed or missing token surfaces as a needs-auth state
// instead of silently opening a browser and blocking initialization. The
// user then triggers the interactive flow explicitly.
var ErrInteractiveAuthRequired = errors.New("interactive OAuth authorization required")

// interactiveKey marks a context as permitting the interactive browser flow.
type interactiveKey struct{}

// WithInteractive returns a context that permits the interactive browser
// authorization flow. Only user-initiated authentication should use it.
func WithInteractive(ctx context.Context) context.Context {
	return context.WithValue(ctx, interactiveKey{}, true)
}

// IsInteractive reports whether ctx permits the interactive browser flow.
func IsInteractive(ctx context.Context) bool {
	v, _ := ctx.Value(interactiveKey{}).(bool)
	return v
}

// callbackPorts are the localhost ports tried, in order, for the OAuth
// redirect listener. The first available one is used. They match the
// redirect URIs declared in the Client ID Metadata Document.
var callbackPorts = []int{
	40704, 40705, 40706, 40707, 40708,
	40709, 40710, 40711, 40712, 40713,
}

// cimdURL is the Client ID Metadata Document URL offered to authorization
// servers that support CIMD. When used, this URL serves as the client ID.
//
// TODO: host this at a stable charm.land URL before relying on it.
const cimdURL = "https://raw.githubusercontent.com/charmbracelet/crush/refs/heads/mcp-oauth/oauth-client-metadata.json"

// Handler implements auth.OAuthHandler for MCP HTTP servers. It wraps
// the go-sdk AuthorizationCodeHandler and persists the token (plus the
// client registration and endpoints needed to refresh it) so that
// restarts and background refreshes never force the user back through
// the browser.
//
// Persistence is wired through the SDK's own hooks: NewTokenSource is
// invoked after the code exchange and for every refresh, and
// InitialTokenSource injects a restored token at startup. That removes
// the need to hand-roll authorization-server discovery.
type Handler struct {
	inner    auth.OAuthHandler
	receiver *callbackReceiver

	// openURL opens the authorization URL in the user's browser. It is
	// a field so tests can simulate a headless environment or drive the
	// callback directly.
	openURL func(string) error

	// interactive permits the browser authorization flow. It is false for
	// background connections (startup) so a missing or unrefreshable token
	// surfaces as a needs-auth state instead of opening a browser and
	// blocking initialization.
	interactive bool

	mu             sync.Mutex
	cachedToken    *oauth2.Token
	authURL        string
	serverURL      string
	onTokenRefresh func(*oauth.Token)
}

var _ auth.OAuthHandler = (*Handler)(nil)

// NewHandler creates a new OAuth handler for an MCP server. savedToken,
// if present, restores a prior session: its access/refresh tokens and
// captured client registration are injected so the SDK can use and
// silently refresh them without a browser round-trip. preregistered, if
// set, supplies an explicit OAuth client for servers that do not support
// dynamic client registration. onTokenRefresh is called whenever a token
// is obtained or refreshed so the caller can persist it. interactive
// permits the browser flow; pass false for background connections
// (startup) so a bad token never opens a browser.
func NewHandler(
	serverName string,
	serverURL string,
	savedToken *oauth.Token,
	preregistered *oauth.OAuthClient,
	onTokenRefresh func(*oauth.Token),
	interactive bool,
	callbackPort int,
) (*Handler, error) {
	receiver := &callbackReceiver{
		authChan: make(chan *auth.AuthorizationResult, 1),
		errChan:  make(chan error, 1),
	}

	lc := &net.ListenConfig{}
	var listener net.Listener
	var port int
	if callbackPort > 0 {
		// A fixed port was requested (e.g. for providers that enforce
		// exact-match redirect URIs). Bind it directly; if it's busy the
		// user needs to free it or pick another.
		var err error
		listener, err = lc.Listen(context.Background(), "tcp", fmt.Sprintf("localhost:%d", callbackPort))
		if err != nil {
			receiver.close()
			return nil, fmt.Errorf("failed to bind OAuth callback port %d: %w", callbackPort, err)
		}
		port = callbackPort
	} else {
		for _, p := range callbackPorts {
			var err error
			listener, err = lc.Listen(context.Background(), "tcp", fmt.Sprintf("localhost:%d", p))
			if err == nil {
				port = p
				break
			}
		}
		if listener == nil {
			receiver.close()
			return nil, fmt.Errorf("failed to start OAuth callback listener: all candidate ports in use")
		}
	}

	redirectURL := fmt.Sprintf("http://localhost:%d/callback", port)

	go receiver.serve(listener)

	h := &Handler{
		receiver:       receiver,
		serverURL:      serverURL,
		openURL:        browser.OpenURL,
		interactive:    interactive,
		onTokenRefresh: onTokenRefresh,
	}
	receiver.handler = h

	// newTokenSource is the SDK hook invoked once after a successful code
	// exchange. The token it hands us is brand new, so persist it right
	// away, then wrap the source so later refreshes persist on change. The
	// resolved oauth2.Config carries the registered client ID and
	// discovered endpoints, which we persist alongside the token so a
	// later start can refresh without rediscovery.
	newTokenSource := func(ctx context.Context, cfg *oauth2.Config, tok *oauth2.Token) (oauth2.TokenSource, error) {
		h.persist(cfg, tok)
		base := cfg.TokenSource(ctx, tok)
		return NewSavingTokenSource(base, cfg, tok, func(c *oauth2.Config, t *oauth2.Token) {
			h.persist(c, t)
		}), nil
	}

	cfg := &auth.AuthorizationCodeHandlerConfig{
		RedirectURL:              redirectURL,
		AuthorizationCodeFetcher: receiver.fetchAuthorizationCode,
		RequestRefreshToken:      true,
		NewTokenSource:           newTokenSource,
		// Use a metadata-fixing HTTP client so trailing-slash issuers in
		// OAuth metadata responses don't trip the SDK's strict RFC 8414
		// validation. Based on Bruno Krugel's fix from PR #3396.
		Client: &http.Client{
			Transport: newMetadataFixupRoundTripper(http.DefaultTransport),
		},
		// The SDK tries CIMD, then a pre-registered client, then dynamic
		// registration. Offering CIMD first lets servers that support it
		// identify Crush by its published metadata; others fall back to DCR.
		ClientIDMetadataDocumentConfig: &auth.ClientIDMetadataDocumentConfig{
			URL: cimdURL,
		},
		DynamicClientRegistrationConfig: &auth.DynamicClientRegistrationConfig{
			Metadata: &oauthex.ClientRegistrationMetadata{
				ClientName:   "Crush",
				RedirectURIs: []string{redirectURL},
				GrantTypes:   []string{"authorization_code", "refresh_token"},
			},
		},
	}

	// Restore a saved client registration as a pre-registered client so
	// Use a pre-registered client so the SDK skips dynamic registration.
	// An explicitly configured client (for servers that don't support DCR,
	// like GitHub or Slack) takes precedence over one captured from a
	// previous registration.
	client := preregistered
	if client == nil || client.ClientID == "" {
		if savedToken != nil && savedToken.Client != nil {
			client = savedToken.Client
		}
	}
	if client != nil && client.ClientID != "" {
		cfg.PreregisteredClient = &oauthex.ClientCredentials{
			ClientID: client.ClientID,
		}
		if client.ClientSecret != "" {
			cfg.PreregisteredClient.ClientSecretAuth = &oauthex.ClientSecretAuth{
				ClientSecret: client.ClientSecret,
			}
		}
	}

	// Restore a saved token as the initial token source so the SDK uses
	// it directly (and refreshes it) instead of triggering the browser
	// flow. Seed the saver with the restored token so only a genuine
	// refresh writes to disk; a plain restart causes no token churn.
	if hasRefreshableToken(savedToken) {
		restored := &oauth2.Token{
			AccessToken:  savedToken.AccessToken,
			RefreshToken: savedToken.RefreshToken,
			Expiry:       time.Unix(savedToken.ExpiresAt, 0),
		}
		oc := &oauth2.Config{
			ClientID:     savedToken.Client.ClientID,
			ClientSecret: savedToken.Client.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:   savedToken.Client.AuthURL,
				TokenURL:  savedToken.Client.TokenURL,
				AuthStyle: oauth2.AuthStyle(savedToken.Client.AuthStyle),
			},
		}
		base := oc.TokenSource(context.Background(), restored)
		cfg.InitialTokenSource = NewSavingTokenSource(base, oc, restored, func(c *oauth2.Config, t *oauth2.Token) {
			h.persist(c, t)
		})
		h.cachedToken = restored
	}

	inner, err := auth.NewAuthorizationCodeHandler(cfg)
	if err != nil {
		receiver.close()
		return nil, fmt.Errorf("failed to create OAuth handler: %w", err)
	}
	h.inner = inner

	slog.Info("MCP OAuth handler created",
		"name", serverName,
		"redirect_url", redirectURL,
		"restored_token", h.cachedToken != nil,
	)

	return h, nil
}

// hasRefreshableToken reports whether a saved token carries enough state
// to be used and refreshed without re-authorizing: an access token plus
// the token endpoint captured previously.
func hasRefreshableToken(t *oauth.Token) bool {
	return t != nil && t.AccessToken != "" && t.Client != nil && t.Client.TokenURL != ""
}

// AuthURL returns the last authorization URL opened in the browser.
func (h *Handler) AuthURL() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.authURL
}

// Token returns the current OAuth token, or nil if not yet authorized.
func (h *Handler) Token() *oauth2.Token {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cachedToken
}

// TokenSource implements auth.OAuthHandler. It delegates to the inner
// handler, whose token source is already wrapped for persistence via
// NewTokenSource and seeded (when restoring) via InitialTokenSource.
func (h *Handler) TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	return h.inner.TokenSource(ctx)
}

// Authorize implements auth.OAuthHandler. It runs the SDK authorization
// flow; the resulting token is captured and persisted through the
// NewTokenSource saver.
func (h *Handler) Authorize(ctx context.Context, req *http.Request, resp *http.Response) error {
	// Never open a browser for a background connection (e.g. startup). The
	// caller surfaces a needs-auth state and the user triggers the
	// interactive flow via a handler created with interactive=true.
	if !h.interactive {
		return ErrInteractiveAuthRequired
	}
	if err := h.inner.Authorize(ctx, req, resp); err != nil {
		// The SDK reports this when the server supports none of the
		// registration methods offered and no client was pre-registered.
		// Point the user at the config field that fixes it.
		if strings.Contains(err.Error(), "no configured client registration methods") {
			return fmt.Errorf("%q does not support automatic OAuth client registration; register an OAuth app with the provider and set oauth_client_id (and oauth_client_secret if required) for this MCP server: %w", h.serverURL, err)
		}
		return err
	}
	ts, err := h.inner.TokenSource(ctx)
	if err != nil {
		return err
	}
	if ts == nil {
		// The SDK short-circuits non-authorization responses (e.g. a
		// genuine 403) without establishing a token source. Leave any
		// restored token in place.
		return nil
	}
	// Reading the token drives the saver, which persists it.
	if _, err := ts.Token(); err != nil {
		return err
	}
	slog.Info("MCP OAuth token captured")
	return nil
}

// persist records the latest token in memory and hands a serialisable
// copy (including the client registration and endpoints from cfg) to the
// caller-supplied saver.
func (h *Handler) persist(cfg *oauth2.Config, tok *oauth2.Token) {
	h.mu.Lock()
	h.cachedToken = tok
	h.mu.Unlock()

	if h.onTokenRefresh == nil {
		return
	}

	out := &oauth.Token{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
	}
	if !tok.Expiry.IsZero() {
		out.ExpiresIn = int(time.Until(tok.Expiry).Seconds())
	}
	out.SetExpiresAt()
	if cfg != nil {
		out.Client = &oauth.OAuthClient{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			AuthURL:      cfg.Endpoint.AuthURL,
			TokenURL:     cfg.Endpoint.TokenURL,
			AuthStyle:    int(cfg.Endpoint.AuthStyle),
		}
	}
	h.onTokenRefresh(out)
}

// Close shuts down the callback server.
func (h *Handler) Close() {
	h.receiver.close()
}

type callbackReceiver struct {
	handler  *Handler
	authChan chan *auth.AuthorizationResult
	errChan  chan error
	server   *http.Server
	mu       sync.Mutex
	once     sync.Once
}

func (r *callbackReceiver) serve(listener net.Listener) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		code := req.URL.Query().Get("code")
		state := req.URL.Query().Get("state")

		if errParam := req.URL.Query().Get("error"); errParam != "" {
			desc := req.URL.Query().Get("error_description")
			fmt.Fprintf(w, "Authentication failed: %s — %s\nYou can close this window.", errParam, desc)
			r.once.Do(func() {
				r.errChan <- fmt.Errorf("OAuth error: %s: %s", errParam, desc)
			})
			return
		}

		r.once.Do(func() {
			r.authChan <- &auth.AuthorizationResult{
				Code:  code,
				State: state,
			}
		})

		fmt.Fprint(w, "Authentication successful! You can close this window.")
	})

	r.mu.Lock()
	r.server = &http.Server{Handler: mux}
	r.mu.Unlock()

	if err := r.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		r.errChan <- err
	}
}

func (r *callbackReceiver) fetchAuthorizationCode(ctx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
	// Some authorization servers reject the "resource" query parameter in
	// the authorization URL (RFC 8707) but accept it during token exchange.
	// Strip it from the browser URL to avoid server_error responses.
	authURL := stripResourceParam(args.URL)
	slog.Info("Opening browser for MCP OAuth authorization")

	r.handler.mu.Lock()
	r.handler.authURL = authURL
	open := r.handler.openURL
	r.handler.mu.Unlock()

	if err := open(authURL); err != nil {
		// If the browser can't be opened (headless, remote SSH), keep
		// the callback listener running and tell the user to open the
		// URL manually.
		slog.Warn("Failed to open browser automatically", "error", err)
		slog.Info("Please open the following URL in your browser to authorize", "url", authURL)
	}

	select {
	case result := <-r.authChan:
		slog.Info("MCP OAuth authorization completed")
		return result, nil
	case err := <-r.errChan:
		slog.Error("MCP OAuth authorization failed", "error", err)
		return nil, err
	case <-ctx.Done():
		slog.Warn("MCP OAuth authorization cancelled")
		return nil, ctx.Err()
	}
}

func (r *callbackReceiver) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.server != nil {
		_ = r.server.Close()
	}
}

// metadataFixupRoundTripper normalizes trailing-slash issuers in OAuth
// metadata responses. Some servers return an issuer with a trailing slash
// that doesn't match the URL the metadata was fetched from, causing the
// SDK's strict RFC 8414 validation to reject it. Based on Bruno Krugel's
// fix from PR #3396.
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
	if !isMetadataEndpoint(req.URL.Path) || resp.StatusCode != http.StatusOK || resp.Body == nil {
		return resp, nil
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read metadata response: %w", err)
	}

	var raw map[string]any
	if json.Unmarshal(body, &raw) != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	issuer, ok := raw["issuer"].(string)
	if !ok || !strings.HasSuffix(issuer, "/") {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	raw["issuer"] = strings.TrimSuffix(issuer, "/")
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
// authorization URL. Some authorization servers reject it in the
// authorize request but accept it during token exchange. Based on Bruno
// Krugel's fix from PR #3396.
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
