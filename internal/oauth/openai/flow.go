package openai

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/callback"
)

// The redirect URI must be a loopback address on a port OpenAI's
// authorization server allow-lists for this client. Codex registers
// 1455 with 1457 as the fallback, so both are tried in order.
var callbackPorts = []int{1455, 1457}

const (
	callbackPath = "/auth/callback"
	startPath    = "/auth/start"
)

// BrowserFlow runs the interactive authorization: it serves the loopback
// redirect target while the user completes authorization in their
// browser, then exchanges the resulting code for tokens.
type BrowserFlow struct {
	authURL     string
	pkce        PKCE
	state       string
	redirectURI string
	startURL    string
	listener    net.Listener
	server      *http.Server
	result      chan *http.Request
}

// StartBrowserFlow opens the loopback callback listener and returns the
// flow holding the authorization URL to open in a browser.
func StartBrowserFlow() (*BrowserFlow, error) {
	pkce, err := NewPKCE()
	if err != nil {
		return nil, err
	}
	state, err := State()
	if err != nil {
		return nil, err
	}

	listener, port, err := listenCallback()
	if err != nil {
		return nil, err
	}

	flow := &BrowserFlow{
		pkce:        pkce,
		state:       state,
		redirectURI: fmt.Sprintf("http://localhost:%d%s", port, callbackPath),
		listener:    listener,
		result:      make(chan *http.Request, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(startPath, flow.handleStart)
	mux.HandleFunc(callbackPath, flow.handleCallback)
	flow.server = &http.Server{Handler: mux}
	flow.authURL = AuthorizeURL(flow.redirectURI, pkce, state)
	flow.startURL = fmt.Sprintf("http://localhost:%d%s", port, startPath)

	go func() {
		// Serve until Close. The error is ignored: a listener closed
		// mid-flow reports ErrServerClosed, and the browser has the
		// landing page it needs either way.
		_ = flow.server.Serve(listener)
	}()

	return flow, nil
}

// URL returns the authorization URL to open in a browser.
func (f *BrowserFlow) URL() string {
	return f.authURL
}

// StartURL returns the local handoff page to open instead. One click
// there opens the authorization URL in a tab that keeps the handoff page
// as its opener — the one arrangement browsers allow to close itself
// after a consent flow, so the tab tidies up when authorization finishes.
func (f *BrowserFlow) StartURL() string {
	return f.startURL
}

// Wait blocks until the browser redirects back with the authorization
// result and returns the exchanged token. It fails if the callback
// reports an error, the state does not match, or ctx is cancelled.
func (f *BrowserFlow) Wait(ctx context.Context) (*oauth.Token, error) {
	var req *http.Request
	select {
	case req = <-f.result:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	query := req.URL.Query()
	if code := query.Get("error"); code != "" {
		return nil, fmt.Errorf("authorization failed: %s: %s", code, query.Get("error_description"))
	}
	if got := query.Get("state"); got != f.state {
		return nil, fmt.Errorf("authorization state mismatch")
	}
	code := query.Get("code")
	if code == "" {
		return nil, fmt.Errorf("authorization response contained no code")
	}

	return ExchangeCode(ctx, code, f.redirectURI, f.pkce)
}

// Close shuts down the callback listener. It is safe to call multiple
// times and on a flow that never started.
func (f *BrowserFlow) Close() {
	if f.server != nil {
		_ = f.server.Close()
	}
}

// handleStart serves the handoff page that opens the real authorization
// URL in a self-closable tab.
func (f *BrowserFlow) handleStart(w http.ResponseWriter, _ *http.Request) {
	_ = callback.Serve(w, callback.Result{
		Subject:     "OpenAI",
		ContinueURL: f.authURL,
	})
}

// handleCallback renders the landing page and hands the redirect request
// to the waiting flow.
func (f *BrowserFlow) handleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	result := callback.Result{
		Subject:          "OpenAI",
		ErrorCode:        query.Get("error"),
		ErrorDescription: query.Get("error_description"),
	}
	if err := callback.Serve(w, result); err != nil {
		// The browser is committed to whatever we send at this point;
		// there is nothing useful left to do with the error.
		_ = err
	}

	select {
	case f.result <- r:
	default:
	}
}

// listenCallback binds a listener on the first available callback port.
func listenCallback() (net.Listener, int, error) {
	var lastErr error
	for _, port := range callbackPorts {
		listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return listener, port, nil
		}
		lastErr = err
	}
	return nil, 0, fmt.Errorf("open OAuth callback port: %w", lastErr)
}

// RedirectURI is the loopback URI this flow registers with the
// authorization server. Exposed for tests and diagnostics.
func (f *BrowserFlow) RedirectURI() string {
	return f.redirectURI
}
