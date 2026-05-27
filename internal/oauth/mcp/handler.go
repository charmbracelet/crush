package mcpoauth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"github.com/pkg/browser"
	"golang.org/x/oauth2"
)

// Handler implements auth.OAuthHandler for MCP servers with token
// persistence. On successful auth, the token is available via Token().
type Handler struct {
	inner    auth.OAuthHandler
	receiver *callbackReceiver

	mu          sync.Mutex
	cachedToken *oauth2.Token
	authURL     string
}

// NewHandler creates a new OAuth handler with an optional cached token.
func NewHandler(serverName string, cachedToken *oauth.Token) (*Handler, error) {
	receiver := &callbackReceiver{
		authChan: make(chan *auth.AuthorizationResult, 1),
		errChan:  make(chan error, 1),
	}

	lc := &net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start OAuth callback listener: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://localhost:%d", port)

	go receiver.serve(listener)

	inner, err := auth.NewAuthorizationCodeHandler(&auth.AuthorizationCodeHandlerConfig{
		RedirectURL:              redirectURL,
		AuthorizationCodeFetcher: receiver.fetchAuthorizationCode,
		DynamicClientRegistrationConfig: &auth.DynamicClientRegistrationConfig{
			Metadata: &oauthex.ClientRegistrationMetadata{
				ClientName:   "Crush",
				RedirectURIs: []string{redirectURL},
			},
		},
	})
	if err != nil {
		receiver.close()
		return nil, fmt.Errorf("failed to create OAuth handler: %w", err)
	}

	h := &Handler{
		inner:    inner,
		receiver: receiver,
	}

	receiver.handler = h

	if cachedToken != nil && !cachedToken.IsExpired() {
		h.cachedToken = &oauth2.Token{
			AccessToken:  cachedToken.AccessToken,
			RefreshToken: cachedToken.RefreshToken,
			Expiry:       time.Unix(cachedToken.ExpiresAt, 0),
		}
	}

	slog.Info("MCP OAuth handler created",
		"name", serverName,
		"redirect_url", redirectURL,
		"has_cached_token", h.cachedToken != nil,
	)

	return h, nil
}

// AuthURL returns the last authorization URL opened in the browser.
func (h *Handler) AuthURL() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.authURL
}

// Token returns the OAuth token after successful authorization.
func (h *Handler) Token() *oauth2.Token {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cachedToken
}

// TokenSource returns a token source for outgoing requests.
func (h *Handler) TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	h.mu.Lock()
	cached := h.cachedToken
	h.mu.Unlock()

	if cached != nil {
		return oauth2.StaticTokenSource(cached), nil
	}
	return h.inner.TokenSource(ctx)
}

// Authorize performs the OAuth authorization flow. After success the
// token is available via Token().
func (h *Handler) Authorize(ctx context.Context, req *http.Request, resp *http.Response) error {
	if err := h.inner.Authorize(ctx, req, resp); err != nil {
		return err
	}

	// Capture the token after successful authorization.
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

	// Store the auth URL so the user can re-open it.
	r.handler.mu.Lock()
	r.handler.authURL = args.URL
	r.handler.mu.Unlock()

	if err := browser.OpenURL(args.URL); err != nil {
		slog.Warn("Failed to open browser automatically", "error", err)
		slog.Info("Please open the following URL in your browser", "url", args.URL)
	}

	select {
	case result := <-r.authChan:
		return result, nil
	case err := <-r.errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *callbackReceiver) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.server != nil {
		r.server.Close()
	}
}
