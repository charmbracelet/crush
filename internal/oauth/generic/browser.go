package generic

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/callback"
)

const (
	// defaultCallbackPath is the redirect path used when the spec does not
	// carry one. Loopback redirect URIs are matched on path by most
	// authorization servers, so a spec that registers a different path
	// must set redirect_uri.
	defaultCallbackPath = "/callback"
	startPath           = "/auth/start"
)

// BrowserFlow runs the authorization code flow with PKCE for a spec-driven
// provider: it serves the loopback redirect while the user authorizes in
// their browser, then exchanges the resulting code for tokens.
type BrowserFlow struct {
	spec        *oauth.AuthSpec
	subject     string
	endpoints   Endpoints
	verifier    string
	challenge   string
	state       string
	authURL     string
	startURL    string
	redirectURI string
	listener    net.Listener
	server      *http.Server
	result      chan *http.Request
}

// StartBrowser opens the loopback callback listener for the spec and
// returns a flow whose Start builds the authorization URL.
func StartBrowser(spec *oauth.AuthSpec, subject string) (*BrowserFlow, error) {
	if spec == nil {
		return nil, errors.New("provider has no OAuth configuration")
	}

	verifier, err := randomToken(64)
	if err != nil {
		return nil, fmt.Errorf("generate code verifier: %w", err)
	}
	state, err := randomToken(32)
	if err != nil {
		return nil, err
	}

	host, port, path, err := callbackTarget(spec)
	if err != nil {
		return nil, err
	}
	listener, err := (&net.ListenConfig{}).Listen(
		context.Background(),
		"tcp",
		net.JoinHostPort(host, fmt.Sprint(port)),
	)
	if err != nil {
		return nil, fmt.Errorf("listen OAuth callback: %w", err)
	}
	bound := listener.Addr().(*net.TCPAddr)

	flow := &BrowserFlow{
		spec:        spec,
		subject:     subject,
		verifier:    verifier,
		challenge:   challengeS256(verifier),
		state:       state,
		redirectURI: fmt.Sprintf("http://%s%s", net.JoinHostPort(host, fmt.Sprint(bound.Port)), path),
		listener:    listener,
		result:      make(chan *http.Request, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(startPath, flow.handleStart)
	mux.HandleFunc(path, flow.handleCallback)
	flow.server = &http.Server{Handler: mux}
	flow.startURL = fmt.Sprintf("http://%s%s", net.JoinHostPort(host, fmt.Sprint(bound.Port)), startPath)

	go func() {
		// Serve until Close. The error is ignored: a listener closed
		// mid-flow reports ErrServerClosed, and the browser already has
		// the landing page it needs.
		_ = flow.server.Serve(listener)
	}()

	return flow, nil
}

// callbackTarget resolves the host, port, and path the flow listens on. A
// redirect_uri in the spec wins, so providers that register a fixed
// loopback URI can be satisfied exactly.
func callbackTarget(spec *oauth.AuthSpec) (host string, port int, path string, err error) {
	host, path = "127.0.0.1", defaultCallbackPath
	port = spec.CallbackPort
	if spec.RedirectURI != "" {
		u, parseErr := url.Parse(spec.RedirectURI)
		if parseErr != nil || u.Scheme != "http" || u.Hostname() == "" {
			return "", 0, "", fmt.Errorf("invalid redirect_uri %q: must be an http loopback URL", spec.RedirectURI)
		}
		host = u.Hostname()
		if p := u.Port(); p != "" {
			port, err = strconv.Atoi(p)
			if err != nil {
				return "", 0, "", fmt.Errorf("invalid redirect_uri %q: %w", spec.RedirectURI, err)
			}
		}
		if u.Path != "" {
			path = u.Path
		}
	}
	return host, port, path, nil
}

// Start resolves the authorization endpoints and returns the browser URL
// the user opens. The device flow is the only one that shows a user code,
// so the second return value is always empty here.
func (f *BrowserFlow) Start(ctx context.Context) (string, string, error) {
	endpoints, err := Resolve(ctx, f.spec)
	if err != nil {
		return "", "", err
	}
	f.endpoints = endpoints
	if endpoints.AuthorizeURL == "" {
		return "", "", fmt.Errorf("authorization server %s does not offer a browser flow", f.spec.Issuer)
	}

	vals := authorizeValues(f.spec)
	vals.Set("response_type", "code")
	vals.Set("redirect_uri", f.redirectURI)
	vals.Set("code_challenge", f.challenge)
	vals.Set("code_challenge_method", "S256")
	vals.Set("state", f.state)
	f.authURL = endpoints.AuthorizeURL + "?" + vals.Encode()

	// The handoff page opens the consent screen in a tab that can close
	// itself once the callback lands, the same trick the built-in
	// providers use.
	return f.startURL, "", nil
}

// URL returns the raw authorization URL.
func (f *BrowserFlow) URL() string { return f.authURL }

// RedirectURI returns the loopback URI presented to the authorization
// server. Exposed for diagnostics and tests.
func (f *BrowserFlow) RedirectURI() string { return f.redirectURI }

// Wait blocks until the browser redirects back with the authorization
// result and returns the exchanged token.
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
	// A callback that carries state must match it. Some providers redirect
	// back without echoing it, and the exchange is still bound to this flow
	// by the single-use code and the PKCE verifier, so their absence is not a
	// reason to refuse the sign-in.
	if got := query.Get("state"); got != "" && got != f.state {
		return nil, errors.New("authorization state mismatch")
	}
	code := query.Get("code")
	if code == "" {
		return nil, errors.New("authorization response contained no code")
	}
	return f.exchange(ctx, code)
}

// CompleteWithCode finishes the flow with a code pasted back from the
// authorization page: either the full callback URL or the bare code. It
// covers the case where the browser cannot reach the loopback callback.
func (f *BrowserFlow) CompleteWithCode(ctx context.Context, input string) (*oauth.Token, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, errors.New("no code entered")
	}

	var code, state string
	if u, err := url.Parse(input); err == nil && u.Scheme != "" && u.Host != "" {
		query := u.Query()
		if errCode := query.Get("error"); errCode != "" {
			return nil, fmt.Errorf("authorization failed: %s: %s", errCode, query.Get("error_description"))
		}
		code = query.Get("code")
		if code == "" {
			return nil, errors.New("pasted URL contained no code")
		}
		state = query.Get("state")
	} else {
		code = input
	}
	if state != "" && state != f.state {
		return nil, errors.New("authorization state mismatch")
	}
	return f.exchange(ctx, code)
}

func (f *BrowserFlow) exchange(ctx context.Context, code string) (*oauth.Token, error) {
	if f.endpoints.TokenURL == "" {
		endpoints, err := Resolve(ctx, f.spec)
		if err != nil {
			return nil, err
		}
		f.endpoints = endpoints
	}
	token, err := ExchangeCode(ctx, f.spec, f.endpoints.TokenURL, code, f.redirectURI, f.verifier, f.state)
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}
	return token, nil
}

// Close shuts down the callback listener. It is safe to call multiple
// times.
func (f *BrowserFlow) Close() {
	if f.server != nil {
		_ = f.server.Close()
	}
}

// handleStart serves the handoff page that opens the authorization URL in a
// self-closable tab.
func (f *BrowserFlow) handleStart(w http.ResponseWriter, _ *http.Request) {
	_ = callback.Serve(w, callback.Result{
		Subject:     f.subject,
		ContinueURL: f.authURL,
	})
}

// handleCallback renders the landing page and hands the redirect request to
// the waiting flow.
func (f *BrowserFlow) handleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if err := callback.Serve(w, callback.Result{
		Subject:          f.subject,
		ErrorCode:        query.Get("error"),
		ErrorDescription: query.Get("error_description"),
	}); err != nil {
		// The browser is committed to whatever we send at this point.
		_ = err
	}
	select {
	case f.result <- r:
	default:
	}
}

// challengeS256 derives the PKCE challenge from a verifier.
func challengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// randomToken returns a fresh n-byte random value, base64url encoded, for
// PKCE verifiers and state parameters.
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
