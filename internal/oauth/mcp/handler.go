package mcpoauth

import (
	"context"
	"errors"
	"fmt"
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

// cimdURL is the Client ID Metadata Document URL used for OAuth
// registration. When the authorization server supports CIMD, this URL
// serves as the client ID.
const cimdURL = "https://raw.githubusercontent.com/charmbracelet/crush/refs/heads/mcp-oauth/oauth-client-metadata.json"

// oauthCallbackPorts are the ports listed in our Client ID Metadata
// Document. We try each in order and use the first available.
var oauthCallbackPorts = []int{
	40704, 40705, 40706, 40707, 40708,
	40709, 40710, 40711, 40712, 40713,
}

// Handler implements auth.OAuthHandler for MCP servers with token and
// client credential persistence. On every token refresh, OnTokenRefresh
// is called so tokens survive restarts.
type Handler struct {
	inner    auth.OAuthHandler
	receiver *callbackReceiver

	mu                sync.Mutex
	cachedToken       *oauth2.Token
	authURL           string
	serverURL         string
	savedClientID     string
	savedClientSecret string
	onTokenRefresh    func(*oauth2.Token)
}

// NewHandler creates a new OAuth handler. savedToken, if non-nil and
// not expired, is used as the initial cached token. onTokenRefresh is
// called whenever a token is obtained or refreshed, enabling callers
// to persist tokens automatically.
func NewHandler(
	serverName string,
	serverURL string,
	savedToken *oauth.Token,
	onTokenRefresh func(*oauth2.Token),
) (*Handler, error) {
	receiver := &callbackReceiver{
		authChan: make(chan *auth.AuthorizationResult, 1),
		errChan:  make(chan error, 1),
	}

	lc := &net.ListenConfig{}
	var listener net.Listener
	var port int
	for _, p := range oauthCallbackPorts {
		var err error
		listener, err = lc.Listen(context.Background(), "tcp", fmt.Sprintf("localhost:%d", p))
		if err == nil {
			port = p
			break
		}
	}
	if listener == nil {
		receiver.close()
		return nil, fmt.Errorf("failed to start OAuth callback listener: all ports in use")
	}

	redirectURL := fmt.Sprintf("http://localhost:%d/callback", port)

	go receiver.serve(listener)

	// Load saved client credentials.
	var savedClientID, savedClientSecret string
	if savedToken != nil && savedToken.Client != nil {
		savedClientID = savedToken.Client.ClientID
		savedClientSecret = savedToken.Client.ClientSecret
	}

	h := &Handler{
		receiver:          receiver,
		serverURL:         serverURL,
		savedClientID:     savedClientID,
		savedClientSecret: savedClientSecret,
		onTokenRefresh:    onTokenRefresh,
	}
	receiver.handler = h

	cfg := &auth.AuthorizationCodeHandlerConfig{
		RedirectURL:              redirectURL,
		AuthorizationCodeFetcher: receiver.fetchAuthorizationCode,
		ClientIDMetadataDocumentConfig: &auth.ClientIDMetadataDocumentConfig{
			URL: cimdURL,
		},
		DynamicClientRegistrationConfig: &auth.DynamicClientRegistrationConfig{
			Metadata: &oauthex.ClientRegistrationMetadata{
				ClientName:   "Crush",
				RedirectURIs: []string{redirectURL},
			},
		},
	}

	// If we have saved client credentials, pass them as pre-registered
	// so the SDK skips DCR/CIMD and uses the stored client ID.
	if savedToken != nil && savedToken.Client != nil && savedToken.Client.ClientID != "" {
		cfg.PreregisteredClient = &oauthex.ClientCredentials{
			ClientID: savedToken.Client.ClientID,
		}
		if savedToken.Client.ClientSecret != "" {
			cfg.PreregisteredClient.ClientSecretAuth = &oauthex.ClientSecretAuth{
				ClientSecret: savedToken.Client.ClientSecret,
			}
		}
	}

	inner, err := auth.NewAuthorizationCodeHandler(cfg)
	if err != nil {
		receiver.close()
		return nil, fmt.Errorf("failed to create OAuth handler: %w", err)
	}
	h.inner = inner

	// Load saved token if it has credentials. Even expired tokens are
	// loaded so that oauth2.TokenSource can refresh them automatically
	// using the refresh token rather than forcing re-authorization.
	if savedToken != nil && savedToken.AccessToken != "" {
		h.cachedToken = &oauth2.Token{
			AccessToken:  savedToken.AccessToken,
			RefreshToken: savedToken.RefreshToken,
			Expiry:       time.Unix(savedToken.ExpiresAt, 0),
		}
	}

	// TODO: Once the go-sdk exposes registered client credentials after
	// DCR (see https://github.com/modelcontextprotocol/go-sdk/issues/901),
	// capture them here and persist via onTokenRefresh so subsequent
	// sessions skip re-registration automatically. For now, pre-registered
	// clients work if oauth_token.client is set manually in config.

	slog.Info("MCP OAuth handler created",
		"name", serverName,
		"redirect_url", redirectURL,
		"has_saved_token", h.cachedToken != nil,
		"has_client_creds", savedToken != nil && savedToken.Client != nil,
	)

	return h, nil
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

// TokenSource returns a token source for outgoing requests. If a
// cached token exists and the SDK hasn't authorized yet, it builds a
// refreshing token source from the cached credentials so that token
// refreshes are persisted without requiring re-authorization.
func (h *Handler) TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	h.mu.Lock()
	cached := h.cachedToken
	h.mu.Unlock()

	innerTS, err := h.inner.TokenSource(ctx)
	if err != nil || innerTS == nil {
		// SDK hasn't run Authorize() yet. If we have a cached token,
		// build a token source from it directly so requests can
		// proceed and refreshes get persisted.
		if cached != nil && h.onTokenRefresh != nil {
			cfg := &oauth2.Config{}
			// Discover token endpoint from MCP server metadata so
			// that expired tokens can be refreshed without a full
			// re-authorization flow.
			if disc, ok := h.discoverOAuthConfig(ctx); ok {
				cfg.Endpoint = disc.Endpoint
				cfg.ClientID = disc.ClientID
				cfg.ClientSecret = disc.ClientSecret
			}
			ts := cfg.TokenSource(ctx, cached)
			return NewSavingTokenSource(ts, cfg, cached, func(_ *oauth2.Config, tok *oauth2.Token) {
				h.mu.Lock()
				h.cachedToken = tok
				h.mu.Unlock()
				h.onTokenRefresh(tok)
			}), nil
		}
		if cached != nil {
			return oauth2.StaticTokenSource(cached), nil
		}
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("no token available: authorize first")
	}

	if h.onTokenRefresh == nil {
		return innerTS, nil
	}

	return NewSavingTokenSource(innerTS, nil, cached, func(_ *oauth2.Config, tok *oauth2.Token) {
		h.mu.Lock()
		h.cachedToken = tok
		h.mu.Unlock()
		h.onTokenRefresh(tok)
	}), nil
}

// discoveredEndpoints holds OAuth endpoints and client credentials
// discovered from MCP server metadata.
type discoveredEndpoints struct {
	oauth2.Endpoint
	ClientID     string
	ClientSecret string
}

// discoverTokenEndpoint attempts to find the OAuth token endpoint and
// client credentials for the MCP server by trying well-known metadata
// URLs. It mirrors the discovery logic in the MCP SDK's
// AuthorizationCodeHandler but works without a prior WWW-Authenticate
// challenge. Returns the result and true on success, or zero-value and
// false if discovery fails.
func (h *Handler) discoverOAuthConfig(ctx context.Context) (discoveredEndpoints, bool) {
	if h.serverURL == "" {
		return discoveredEndpoints{}, false
	}

	parsed, err := url.Parse(h.serverURL)
	if err != nil {
		slog.Warn("Failed to parse MCP server URL for endpoint discovery", "url", h.serverURL, "error", err)
		return discoveredEndpoints{}, false
	}

	// Try well-known protected resource metadata URLs per MCP spec.
	metadataURLs := []string{
		parsed.Scheme + "://" + parsed.Host + "/.well-known/oauth-protected-resource/" + strings.TrimLeft(parsed.Path, "/"),
		parsed.Scheme + "://" + parsed.Host + "/.well-known/oauth-protected-resource",
	}

	var asm *oauthex.AuthServerMeta
	for _, metaURL := range metadataURLs {
		prm, err := oauthex.GetProtectedResourceMetadata(ctx, metaURL, h.serverURL, nil)
		if err != nil || prm == nil || len(prm.AuthorizationServers) == 0 {
			continue
		}
		asm, err = auth.GetAuthServerMetadata(ctx, prm.AuthorizationServers[0], nil)
		if err == nil && asm != nil && asm.TokenEndpoint != "" {
			break
		}
		// Fallback: use conventional paths on the auth server.
		asm = &oauthex.AuthServerMeta{
			AuthorizationEndpoint: prm.AuthorizationServers[0] + "/authorize",
			TokenEndpoint:         prm.AuthorizationServers[0] + "/token",
		}
		break
	}

	if asm == nil || asm.TokenEndpoint == "" {
		// Final fallback: assume the MCP server root is the auth server.
		root := parsed.Scheme + "://" + parsed.Host
		asm = &oauthex.AuthServerMeta{
			AuthorizationEndpoint: root + "/authorize",
			TokenEndpoint:         root + "/token",
		}
	}

	result := discoveredEndpoints{
		Endpoint: oauth2.Endpoint{
			AuthURL:  asm.AuthorizationEndpoint,
			TokenURL: asm.TokenEndpoint,
		},
	}

	// Resolve client credentials using the same priority as the SDK:
	// 1. Saved client creds from a previous session.
	// 2. CIMD URL (our default registration method).
	// 3. Dynamic client registration (not attempted here since it
	//    requires a fresh registration and we only need refresh).
	if h.savedClientID != "" {
		result.ClientID = h.savedClientID
		result.ClientSecret = h.savedClientSecret
	} else {
		// Use our CIMD URL as the client ID, matching what the SDK
		// does in handleRegistration when CIMD is supported.
		result.ClientID = cimdURL
	}

	return result, true
}

// Authorize performs the OAuth authorization flow. After success the
// token is available via Token().
func (h *Handler) Authorize(ctx context.Context, req *http.Request, resp *http.Response) error {
	if err := h.inner.Authorize(ctx, req, resp); err != nil {
		return err
	}

	ts, err := h.inner.TokenSource(ctx)
	if err != nil {
		return err
	}
	tok, err := ts.Token()
	if err != nil {
		return err
	}

	h.mu.Lock()
	h.cachedToken = tok
	h.mu.Unlock()

	if h.onTokenRefresh != nil {
		h.onTokenRefresh(tok)
	}
	slog.Info("MCP OAuth token captured")
	return nil
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
	slog.Info("Opening browser for MCP OAuth authorization")

	r.handler.mu.Lock()
	r.handler.authURL = args.URL
	r.handler.mu.Unlock()

	if err := browser.OpenURL(args.URL); err != nil {
		slog.Warn("Failed to open browser automatically", "error", err)
		slog.Info("Please open the following URL in your browser", "url", args.URL)
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
