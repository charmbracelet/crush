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

// mcptoken stores a serialised OAuth token plus the client registration
// info needed to refresh it.
type mcptoken struct {
	Token     *oauth2.Token `json:"token"`
	ClientID  string        `json:"client_id,omitempty"`
	Endpoints struct {
		AuthURL  string `json:"auth_url"`
		TokenURL string `json:"token_url"`
	} `json:"endpoints"`
}

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
	inner     *auth.AuthorizationCodeHandler
	mu        sync.Mutex
	tokenSrc  oauth2.TokenSource
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
		cfg := &oauth2.Config{
			ClientID: saved.ClientID,
			Endpoint: oauth2.Endpoint{
				AuthURL:  saved.Endpoints.AuthURL,
				TokenURL: saved.Endpoints.TokenURL,
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

// Authorize implements auth.OAuthHandler. It performs the full OAuth
// authorization-code flow (discovery, registration, browser, PKCE,
// token exchange) then persists the resulting token.
func (h *mcpOAuthHandler) Authorize(ctx context.Context, req *http.Request, resp *http.Response) error {
	inner, err := h.buildInner()
	if err != nil {
		resp.Body.Close()
		return err
	}
	if err := inner.Authorize(ctx, req, resp); err != nil {
		return err
	}
	// After a successful Authorize the inner handler holds a fresh
	// token source. Cache it and persist to disk.
	ts, _ := inner.TokenSource(ctx)
	h.mu.Lock()
	h.tokenSrc = ts
	h.inner = inner
	h.mu.Unlock()
	if ts != nil {
		if tok, err := ts.Token(); err == nil {
			h.persistToken(tok)
		}
	}
	return nil
}

// buildInner lazily creates the SDK AuthorizationCodeHandler. The
// redirect URL is allocated on first use because we need a free port.
func (h *mcpOAuthHandler) buildInner() (*auth.AuthorizationCodeHandler, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to allocate callback port: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close() // the handler's callback server will re-listen
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
			return openBrowserAndCapture(fetchCtx, args.URL, port, h.serverURL)
		},
	}
	return auth.NewAuthorizationCodeHandler(cfg)
}

// persistToken saves the current token to the store. It is called
// after Authorize succeeds and lazily captures the endpoint/client info
// from the inner handler.
func (h *mcpOAuthHandler) persistToken(tok *oauth2.Token) {
	entry := mcptoken{Token: tok}
	h.mu.Lock()
	inner := h.inner
	h.mu.Unlock()
	if inner != nil {
		// Best-effort: the inner handler doesn't expose its config, so
		// we fetch the metadata endpoints now to persist them for use
		// on the next startup.
		endpoints, clientID := h.discoverEndpoints(context.Background(), h.serverURL)
		entry.Endpoints.AuthURL = endpoints.AuthURL
		entry.Endpoints.TokenURL = endpoints.TokenURL
		entry.ClientID = clientID
	}
	h.store.save(h.serverURL, entry)
}

// discoverEndpoints fetches the OAuth metadata for the server URL to
// extract the authorization and token endpoints. This is used to
// persist enough info to restore the token source on restart.
func (h *mcpOAuthHandler) discoverEndpoints(ctx context.Context, serverURL string) (struct{ AuthURL, TokenURL string }, string) {
	result := struct{ AuthURL, TokenURL string }{}
	u, err := url.Parse(serverURL)
	if err != nil {
		return result, ""
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
		return result, ""
	}
	// Fallback: server root as authorization server.
	authServer := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	asm, err := auth.GetAuthServerMetadata(ctx, authServer, client)
	if err == nil && asm != nil {
		result.AuthURL = asm.AuthorizationEndpoint
		result.TokenURL = asm.TokenEndpoint
	}
	return result, ""
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
