package grok

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/callback"
)

const (
	// callbackPath must stay exactly "/callback": xAI registers the
	// shared client's loopback redirect URIs by path
	// (http://127.0.0.1:{any port}/callback), so any other path is
	// rejected with "redirect_uri does not match any registered URI".
	// The port is exempt per RFC 8252, which is why the listener may
	// bind an OS-assigned one.
	callbackPath = "/callback"
	startPath    = "/auth/start"

	// accountsOrigin is the origin of xAI's accounts app, which serves
	// the consent screen. Rather than a plain redirect, the consent
	// page may hand the authorization code to the loopback server with
	// a cross-origin fetch from this origin, so the callback responses
	// carry CORS headers naming it.
	accountsOrigin = "https://accounts.x.ai"
)

// BrowserFlow runs the interactive authorization: it serves the loopback
// redirect target while the user completes authorization in their
// browser, then exchanges the resulting code for tokens.
//
// Unlike OpenAI, xAI does not allow-list fixed callback ports, so the
// listener binds an OS-assigned port and the redirect URI carries it.
type BrowserFlow struct {
	authURL     string
	pkce        PKCE
	state       string
	nonce       string
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
	nonce, err := Nonce()
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
		nonce:       nonce,
		redirectURI: fmt.Sprintf("http://127.0.0.1:%d%s", port, callbackPath),
		listener:    listener,
		result:      make(chan *http.Request, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(startPath, flow.handleStart)
	mux.HandleFunc(callbackPath, flow.handleCallback)
	flow.server = &http.Server{Handler: mux}
	flow.authURL = AuthorizeURL(flow.redirectURI, pkce, state, nonce)
	flow.startURL = fmt.Sprintf("http://127.0.0.1:%d%s", port, startPath)

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

// CompleteWithCode finishes the flow with a code pasted back from the
// authorization page: either the full callback URL or the bare code.
// The state is validated when the paste carries one; a bare code has
// none to check, exactly like the Grok CLI's paste fallback. It lets a
// user who declined the browser permission, or whose browser cannot
// reach the loopback callback, finish signing in by hand.
func (f *BrowserFlow) CompleteWithCode(ctx context.Context, input string) (*oauth.Token, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("no code entered")
	}

	var code, state string
	if u, err := url.Parse(input); err == nil && u.Scheme != "" && u.Host != "" {
		query := u.Query()
		if errCode := query.Get("error"); errCode != "" {
			return nil, fmt.Errorf("authorization failed: %s: %s", errCode, query.Get("error_description"))
		}
		code = query.Get("code")
		if code == "" {
			return nil, fmt.Errorf("pasted URL contained no code")
		}
		state = query.Get("state")
	} else {
		code = input
	}

	if state != "" && state != f.state {
		return nil, fmt.Errorf("authorization state mismatch")
	}

	return ExchangeCode(ctx, code, f.redirectURI, f.pkce)
}

// handleStart serves the handoff page that opens the real authorization
// URL in a self-closable tab.
func (f *BrowserFlow) handleStart(w http.ResponseWriter, _ *http.Request) {
	_ = callback.Serve(w, callback.Result{
		Subject:     "Grok",
		ContinueURL: f.authURL,
	})
}

// handleCallback renders the landing page and hands the redirect request
// to the waiting flow.
func (f *BrowserFlow) handleCallback(w http.ResponseWriter, r *http.Request) {
	// The consent page at accounts.x.ai may deliver the code with a
	// cross-origin fetch instead of a redirect, so every callback
	// response names that origin. The preflight answer also grants
	// private-network access: the fetch targets a loopback address from
	// a public page, which browsers gate behind a preflight.
	w.Header().Set("Access-Control-Allow-Origin", accountsOrigin)
	w.Header().Set("Access-Control-Allow-Private-Network", "true")
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	query := r.URL.Query()
	result := callback.Result{
		Subject:          "Grok",
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

// listenCallback binds a loopback listener on an OS-assigned port.
func listenCallback() (net.Listener, int, error) {
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, fmt.Errorf("listen OAuth callback: %w", err)
	}
	return listener, listener.Addr().(*net.TCPAddr).Port, nil
}

// RedirectURI is the loopback URI this flow presents to the authorization
// server. Exposed for tests and diagnostics.
func (f *BrowserFlow) RedirectURI() string {
	return f.redirectURI
}
