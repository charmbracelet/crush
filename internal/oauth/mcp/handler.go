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
	"github.com/charmbracelet/crush/internal/oauth/callback"
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

// callbackPath is the path the authorization server redirects back to. It
// is part of the registered redirect URI, so it must not change without
// re-registering clients.
const callbackPath = "/callback"

// callbackPorts are the localhost ports tried, in order, for the OAuth
// redirect listener. The first available one is used.
var callbackPorts = []int{
	40704, 40705, 40706, 40707, 40708,
	40709, 40710, 40711, 40712, 40713,
}

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

	// suppressBrowser, when true, prevents openURL from being invoked
	// (the authorization URL is still recorded and logged). Used when
	// the flow is driven remotely, e.g. by a connected client that opens
	// the browser on its own machine.
	suppressBrowser bool
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
		serverName: serverName,
		fixedPort:  callbackPort,
	}

	// Resolve the redirect port without binding it. The listener is only
	// opened when an authorization actually runs (see fetchAuthorizationCode),
	// so a handler that restores a valid token never occupies the port and
	// several Crush processes can share it; only the one doing a live login
	// binds, and only for the duration of that login.
	//
	// A fixed port comes straight from config. Otherwise we probe for the
	// first free candidate, just long enough to learn which is open. Either
	// way the chosen port is baked into the redirect URI below and pinned on
	// the receiver, so bindLocked always rebinds the SAME port. The probe is
	// not a reservation — another process can take the port before the first
	// real login — but a busy port then fails loudly rather than silently
	// binding a port the redirect URI does not point at.
	port := callbackPort
	if port == 0 {
		lc := &net.ListenConfig{}
		for _, p := range callbackPorts {
			probe, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf("localhost:%d", p))
			if err == nil {
				_ = probe.Close()
				port = p
				break
			}
		}
		if port == 0 {
			return nil, errors.New("failed to start OAuth callback listener: all candidate ports in use")
		}
	}
	receiver.fixedPort = port

	redirectURL := fmt.Sprintf("http://localhost:%d%s", port, callbackPath)

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
		// validation. Also rewrite internal-cluster redirects back to the
		// external hostname so the flow works outside the cluster.
		// Based on Bruno Krugel's fix from PR #3396.
		Client: newOAuthMetadataClient(http.DefaultTransport, serverURL),
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

	slog.Info(
		"MCP OAuth handler created",
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

// SetBrowserSuppress controls whether the browser is opened automatically
// when the authorization URL is generated. Pass an unlock function that
// restores the previous behavior; the handler re-enables the browser when
// the returned function is called. This is used by the server-driven flow
// where a remote client opens the browser locally.
func (h *Handler) SetBrowserSuppress(suppress bool) func() {
	h.mu.Lock()
	prev := h.suppressBrowser
	h.suppressBrowser = suppress
	h.mu.Unlock()
	return func() {
		h.mu.Lock()
		h.suppressBrowser = prev
		h.mu.Unlock()
	}
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

// callbackReceiver owns the localhost listener that the authorization
// server redirects back to, and hands each authorization result to the
// flow waiting for it.
//
// The listener is bound lazily, on the first authorization attempt, and
// released as soon as that attempt settles. A handler that holds a valid
// token never binds at all, so any number of Crush processes may coexist;
// the callback port is occupied only for the few seconds an actual login
// is in flight.
type callbackReceiver struct {
	handler *Handler
	// serverName labels the callback page so the user can see which MCP
	// server they just authorized.
	serverName string

	// fixedPort, when > 0, is the OAuthCallbackPort config: the single
	// port the registered redirect URI points at. When 0, a port is
	// chosen from callbackPorts at bind time.
	fixedPort int

	mu sync.Mutex
	// flight is the authorization currently awaiting a redirect, if any.
	// Connecting to a server can issue several requests at once, so more
	// than one of them can meet a 401 and ask to authorize. They share a
	// single flight rather than each opening their own browser tab and
	// racing for one redirect.
	flight *authFlight
	// server and port describe the live listener. They are set only while
	// an authorization is in flight and cleared the moment it settles.
	server *http.Server
	port   int
	closed bool
}

// authFlight is one authorization attempt: a browser tab was opened and a
// redirect is expected. It settles exactly once, whichever arrives first —
// a result, an error, or the receiver shutting down.
type authFlight struct {
	done   chan struct{}
	once   sync.Once
	result *auth.AuthorizationResult
	err    error
}

// settle records the outcome of the flight and wakes everyone waiting on
// it. Only the first call has any effect, so a duplicate redirect (a
// reloaded tab, say) cannot overwrite a result already in use.
func (f *authFlight) settle(result *auth.AuthorizationResult, err error) {
	f.once.Do(func() {
		f.result, f.err = result, err
		close(f.done)
	})
}

// begin returns the flight to wait on and whether the caller created it.
// The creator is responsible for opening the browser and, once the attempt
// is over, for clearing it so a later authorization can start fresh. The
// first (creating) caller also binds the callback listener; joiners wait
// on the flight that is already serving.
func (r *callbackReceiver) begin() (*authFlight, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.flight != nil {
		return r.flight, false, nil
	}
	if err := r.bindLocked(); err != nil {
		return nil, false, err
	}
	r.flight = &authFlight{done: make(chan struct{})}
	return r.flight, true, nil
}

// end retires the flight if it is still the current one, so the next
// authorization opens a fresh browser tab instead of waiting on a redirect
// that will never come.
func (r *callbackReceiver) end(flight *authFlight) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.flight == flight {
		r.flight = nil
	}
}

// current returns the in-progress flight, or nil when no authorization is
// waiting on a redirect.
func (r *callbackReceiver) current() *authFlight {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.flight
}

// bind starts the listener if one is not already running. The port was
// resolved and pinned at construction, so this always targets the port the
// redirect URI points at; if it is busy the error surfaces loudly rather
// than silently binding a port nobody will redirect to.
func (r *callbackReceiver) bind() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.bindLocked()
}

// bindLocked is bind with r.mu already held. The listener starts accepting
// before this returns, so a browser opened immediately after cannot beat
// the server to the port.
func (r *callbackReceiver) bindLocked() error {
	if r.closed {
		return errors.New("OAuth callback listener closed")
	}
	if r.server != nil {
		return nil
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", r.handleCallback)
	server := &http.Server{Handler: mux}

	lc := &net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf("localhost:%d", r.fixedPort))
	if err != nil {
		return fmt.Errorf("failed to bind OAuth callback port %d: %w", r.fixedPort, err)
	}
	r.port = r.fixedPort
	go r.serve(server, listener)
	r.server = server
	return nil
}

// serve runs the HTTP server until the listener is closed. An unexpected
// Serve error settles the in-flight flight so a waiting flow fails
// promptly instead of hanging until its context expires.
func (r *callbackReceiver) serve(server *http.Server, listener net.Listener) {
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		if flight := r.current(); flight != nil {
			flight.settle(nil, err)
		}
	}
}

// release shuts the listener down and clears it, so the callback port is
// free for the next authorization (or another process). It is the mirror
// of bind and is called once an authorization settles and on close.
//
// The callback page is rendered before the flight settles, so the response
// is already on its way out by the time release runs. Shutdown (not Close)
// lets any in-flight write finish draining before the socket closes.
func (r *callbackReceiver) release() {
	r.mu.Lock()
	server := r.server
	r.server = nil
	r.mu.Unlock()
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}
}

// handleCallback receives the authorization redirect, hands the result to
// the waiting flow, and renders the page the user sees.
//
// Only the redirect path is treated as a callback. Browsers request extras
// such as /favicon.ico against the same origin, and letting one of those
// settle the flight would abort authorization with an empty code.
func (r *callbackReceiver) handleCallback(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != callbackPath {
		http.NotFound(w, req)
		return
	}

	query := req.URL.Query()
	result := callback.Result{
		Subject:          r.serverName,
		ErrorCode:        query.Get("error"),
		ErrorDescription: query.Get("error_description"),
	}

	// Render the page BEFORE settling the flight. settle unblocks await,
	// which releases the listener; settling first would close the connection
	// out from under this write and show the user a browser error instead
	// of the success page. Writing first keeps the response on the wire.
	if err := callback.Serve(w, result); err != nil {
		slog.Warn("Failed to render OAuth callback page", "error", err)
	}

	// A redirect with no flight waiting means the tab was reloaded or
	// revisited after the flow finished. The page above already described
	// the outcome accurately, so do not disturb any later authorization.
	if flight := r.current(); flight != nil {
		if result.Failed() {
			flight.settle(nil, fmt.Errorf("OAuth error: %s: %s", result.ErrorCode, result.ErrorDescription))
		} else {
			flight.settle(&auth.AuthorizationResult{
				Code:  query.Get("code"),
				State: query.Get("state"),
				// Required by servers that implement RFC 9207.
				Iss: query.Get("iss"),
			}, nil)
		}
	}
}

func (r *callbackReceiver) fetchAuthorizationCode(ctx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
	flight, owned, err := r.begin()
	if err != nil {
		return nil, err
	}
	if !owned {
		// Another request already opened the browser for this server. Wait
		// for that redirect instead of opening a second tab.
		slog.Debug("Joining in-progress MCP OAuth authorization", "name", r.serverName)
		return r.await(ctx, flight, false)
	}
	defer r.end(flight)

	// Some authorization servers reject the "resource" query parameter in
	// the authorization URL (RFC 8707) but accept it during token exchange.
	// Strip it from the browser URL to avoid server_error responses.
	authURL := stripResourceParam(args.URL)
	slog.Info("Opening browser for MCP OAuth authorization")

	r.handler.mu.Lock()
	r.handler.authURL = authURL
	open := r.handler.openURL
	suppress := r.handler.suppressBrowser
	r.handler.mu.Unlock()

	if suppress {
		slog.Info("Browser suppressed; remote client must open the authorization URL", "url", authURL)
	} else if err := open(authURL); err != nil {
		// If the browser can't be opened (headless, remote SSH), keep
		// the callback listener running and tell the user to open the
		// URL manually.
		slog.Warn("Failed to open browser automatically", "error", err)
		slog.Info("Please open the following URL in your browser to authorize", "url", authURL)
	}

	return r.await(ctx, flight, true)
}

// await blocks until the flight settles or ctx is cancelled. The owner
// releases the callback listener once the flow is done, so the port is
// free again as soon as the authorization completes (or is abandoned).
func (r *callbackReceiver) await(ctx context.Context, flight *authFlight, owned bool) (*auth.AuthorizationResult, error) {
	select {
	case <-flight.done:
		if owned {
			r.release()
		}
		if flight.err != nil {
			slog.Error("MCP OAuth authorization failed", "error", flight.err)
			return nil, flight.err
		}
		slog.Info("MCP OAuth authorization completed")
		return flight.result, nil
	case <-ctx.Done():
		slog.Warn("MCP OAuth authorization cancelled")
		if owned {
			// Abandoning the tab we opened; make sure nobody keeps
			// waiting on a redirect that is no longer coming.
			flight.settle(nil, ctx.Err())
			r.release()
		}
		return nil, ctx.Err()
	}
}

// close shuts the receiver down permanently and fails any authorization
// still waiting on a redirect, so a pending flow ends promptly instead of
// hanging until its context expires. After close, bind refuses to start a
// new listener.
func (r *callbackReceiver) close() {
	r.mu.Lock()
	r.closed = true
	server, flight := r.server, r.flight
	r.server = nil
	r.flight = nil
	r.mu.Unlock()

	if server != nil {
		_ = server.Close()
	}
	if flight != nil {
		flight.settle(nil, errors.New("OAuth callback listener closed"))
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

// newOAuthMetadataClient creates an HTTP client for the OAuth flow that
// smooths over two nonstandard behaviors seen behind corporate proxies:
//
//  1. Trailing-slash issuers in metadata responses, normalized by
//     metadataFixupRoundTripper so they pass the SDK's strict RFC 8414
//     validation.
//  2. Metadata discovery requests that get 3xx-redirected to an
//     unreachable internal host (e.g. a cluster address behind a proxy).
//     Well-known discovery is never supposed to hop hosts via redirects
//     (the authorization server location comes from the metadata body,
//     not a Location header), so for metadata endpoints we rewrite the
//     redirect back to the original MCP host. Token, registration, and
//     authorize requests are left untouched, so a separately hosted
//     identity provider keeps working.
func newOAuthMetadataClient(base http.RoundTripper, serverURL string) *http.Client {
	var originalHost, originalScheme string
	if u, err := url.Parse(serverURL); err == nil {
		originalHost = u.Host
		originalScheme = u.Scheme
	}
	return &http.Client{
		Transport: newMetadataFixupRoundTripper(base),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Supplying CheckRedirect replaces net/http's default, so
			// re-enforce its 10-redirect cap here.
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			if originalHost != "" && isMetadataEndpoint(req.URL.Path) && req.URL.Host != originalHost {
				slog.Debug("Rewriting OAuth metadata redirect back to original host",
					"from", req.URL.Host, "to", originalHost)
				req.URL.Host = originalHost
				req.URL.Scheme = originalScheme
				req.Host = originalHost
			}
			return nil
		},
	}
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
