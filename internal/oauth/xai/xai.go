// Package xai implements the browser-based OAuth2 flow (authorization code
// with PKCE) for "Sign in with Grok" against xAI's auth server. It mirrors
// the flow used by goose and OpenClaw.
package xai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

const (
	// defaultClientID is xAI's shared public OAuth client for CLI "Sign in
	// with Grok" tooling — the same client embedded by goose and OpenClaw.
	// A client ID is a public, non-secret identifier; xAI labels the
	// consent screen "Grok Build".
	//
	// TODO(PoC): before shipping beyond a proof of concept, Crush should
	// register its own xAI OAuth client rather than reusing the shared one.
	// See https://github.com/aaif-goose/goose/pull/9339 (comment 4508201758).
	defaultClientID = "b1a00492-073a-47ea-816f-4c329264a828"

	// clientIDEnv overrides defaultClientID for anyone who registers their
	// own xAI OAuth client.
	clientIDEnv = "XAI_OAUTH_CLIENT_ID"

	oauthScope  = "openid profile email offline_access grok-cli:access api:access"
	oauthIssuer = "https://auth.x.ai"

	// callbackHost and callbackPort form the loopback redirect URI. The
	// port is fixed because it is part of the shared client's registered
	// redirect URI; it must not be swapped for an ephemeral port.
	callbackHost = "127.0.0.1"
	callbackPort = "56121"
	callbackPath = "/callback"

	// referrer is a free-text attribution field accepted by xAI's authorize
	// endpoint; it does not affect the consent screen.
	referrer = "crush"

	// oauthTimeout bounds how long we wait for the user to finish the
	// browser flow.
	oauthTimeout = 5 * time.Minute

	// defaultTokenLifetimeSecs is used when the token response carries no
	// expiry information.
	defaultTokenLifetimeSecs = 3600

	userAgent = "crush"
)

// discoveryURL is xAI's OIDC discovery endpoint. It is a package variable so
// tests can point it at a local server.
var discoveryURL = oauthIssuer + "/.well-known/openid-configuration"

// clientID returns the OAuth client ID, honouring the XAI_OAUTH_CLIENT_ID
// override.
func clientID() string {
	if v := strings.TrimSpace(os.Getenv(clientIDEnv)); v != "" {
		return v
	}
	return defaultClientID
}

// redirectURI is the loopback URL xAI redirects back to after consent.
func redirectURI() string {
	return "http://" + net.JoinHostPort(callbackHost, callbackPort) + callbackPath
}

// discoveryDoc holds the subset of xAI's OIDC discovery document we use.
type discoveryDoc struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

// fetchDiscovery loads and validates xAI's OIDC discovery document.
func fetchDiscovery(ctx context.Context) (*discoveryDoc, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create discovery request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xAI OAuth discovery request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read discovery response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("xAI OAuth discovery failed with status %d", resp.StatusCode)
	}

	var doc discoveryDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("decode discovery document: %w", err)
	}

	authEndpoint, err := trustedEndpoint(doc.AuthorizationEndpoint, "authorization endpoint")
	if err != nil {
		return nil, err
	}
	tokenEndpoint, err := trustedEndpoint(doc.TokenEndpoint, "token endpoint")
	if err != nil {
		return nil, err
	}
	return &discoveryDoc{
		AuthorizationEndpoint: authEndpoint,
		TokenEndpoint:         tokenEndpoint,
	}, nil
}

// trustedEndpoint validates that an OAuth endpoint returned by discovery is an
// HTTPS URL on an x.ai host, guarding against a hijacked discovery document.
func trustedEndpoint(endpoint, label string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("xAI OAuth %s is not a valid URL: %w", label, err)
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("xAI OAuth %s is not served over HTTPS", label)
	}
	host := u.Hostname()
	if host != "x.ai" && !strings.HasSuffix(host, ".x.ai") {
		return "", fmt.Errorf("xAI OAuth %s has untrusted host %q", label, host)
	}
	return endpoint, nil
}

// pkceChallenge is a PKCE verifier/challenge pair (RFC 7636).
type pkceChallenge struct {
	verifier  string
	challenge string
}

// generatePKCE produces a PKCE verifier and its S256 challenge.
func generatePKCE() (pkceChallenge, error) {
	verifier, err := randomHex(32)
	if err != nil {
		return pkceChallenge{}, err
	}
	sum := sha256.Sum256([]byte(verifier))
	return pkceChallenge{
		verifier:  verifier,
		challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
	}, nil
}

// randomHex returns n cryptographically random bytes, hex-encoded.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// buildAuthorizeURL builds the xAI authorization URL for the browser flow.
func buildAuthorizeURL(authorizationEndpoint string, p pkceChallenge, state, nonce string) (string, error) {
	u, err := url.Parse(authorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("parse authorization endpoint: %w", err)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID())
	q.Set("redirect_uri", redirectURI())
	q.Set("scope", oauthScope)
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", p.challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("plan", "generic")
	q.Set("referrer", referrer)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// callbackResult is the outcome of the loopback OAuth callback.
type callbackResult struct {
	code string
	err  error
}

// callbackServer is the loopback HTTP server that receives the OAuth redirect.
type callbackServer struct {
	server        *http.Server
	expectedState string
	resultCh      chan callbackResult
	once          sync.Once
}

// startCallbackServer binds the fixed loopback redirect URI and serves the
// OAuth callback. It fails if the port is already in use.
func startCallbackServer(expectedState string) (*callbackServer, error) {
	addr := net.JoinHostPort(callbackHost, callbackPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("bind xAI OAuth callback listener on %s: %w", addr, err)
	}

	cs := &callbackServer{
		expectedState: expectedState,
		resultCh:      make(chan callbackResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, cs.handle)
	cs.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() { _ = cs.server.Serve(listener) }()
	return cs, nil
}

// handle processes the OAuth redirect, validating state and extracting the
// authorization code.
func (cs *callbackServer) handle(w http.ResponseWriter, r *http.Request) {
	applyCallbackCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	q := r.URL.Query()
	switch {
	case q.Get("error") != "":
		desc := q.Get("error_description")
		if desc == "" {
			desc = q.Get("error")
		}
		cs.finish(callbackResult{err: fmt.Errorf("xAI OAuth was denied: %s", desc)})
		writeCallbackPage(w, false)
	case q.Get("state") != cs.expectedState:
		cs.finish(callbackResult{err: errors.New("xAI OAuth callback state mismatch")})
		writeCallbackPage(w, false)
	case q.Get("code") == "":
		cs.finish(callbackResult{err: errors.New("xAI OAuth callback is missing the authorization code")})
		writeCallbackPage(w, false)
	default:
		cs.finish(callbackResult{code: q.Get("code")})
		writeCallbackPage(w, true)
	}
}

// finish delivers the callback result exactly once.
func (cs *callbackServer) finish(res callbackResult) {
	cs.once.Do(func() { cs.resultCh <- res })
}

// close shuts down the callback server. It is safe to call multiple times.
func (cs *callbackServer) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = cs.server.Shutdown(ctx)
}

// callbackCORSAllowlist is the set of xAI hosts whose CORS preflight against
// the loopback redirect URI is echoed; mirrors OpenClaw/goose behaviour.
var callbackCORSAllowlist = map[string]bool{
	"auth.x.ai":     true,
	"accounts.x.ai": true,
}

// applyCallbackCORS echoes CORS headers for trusted xAI origins only.
func applyCallbackCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	u, err := url.Parse(origin)
	if err != nil || !callbackCORSAllowlist[u.Hostname()] {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
}

// writeCallbackPage renders the page shown in the user's browser tab.
func writeCallbackPage(w http.ResponseWriter, success bool) {
	heading := "xAI sign-in complete"
	body := "Authentication succeeded. You can close this tab and return to Crush."
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if !success {
		heading = "xAI sign-in failed"
		body = "Authentication failed. Return to Crush for details."
		w.WriteHeader(http.StatusBadRequest)
	}
	_, _ = fmt.Fprintf(w, callbackPageTemplate, heading, body)
}

const callbackPageTemplate = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>%[1]s</title></head>
<body style="font-family:system-ui,sans-serif;text-align:center;padding-top:4rem">
<h2>%[1]s</h2>
<p>%[2]s</p>
</body>
</html>
`

// Authorizer holds the state of an in-progress browser OAuth flow.
type Authorizer struct {
	// AuthURL is the xAI authorization URL the user must open in a browser.
	AuthURL string

	discovery *discoveryDoc
	pkce      pkceChallenge
	server    *callbackServer
}

// NewAuthorizer begins a browser OAuth flow: it fetches xAI's OIDC discovery
// document, generates PKCE parameters, starts the loopback callback server,
// and builds the authorization URL. The caller must open AuthURL in a
// browser, call Wait to obtain the token, and call Close when done.
func NewAuthorizer(ctx context.Context) (*Authorizer, error) {
	discovery, err := fetchDiscovery(ctx)
	if err != nil {
		return nil, err
	}

	pkce, err := generatePKCE()
	if err != nil {
		return nil, err
	}

	state, err := randomHex(32)
	if err != nil {
		return nil, err
	}

	nonce, err := randomHex(16)
	if err != nil {
		return nil, err
	}

	authURL, err := buildAuthorizeURL(discovery.AuthorizationEndpoint, pkce, state, nonce)
	if err != nil {
		return nil, err
	}

	server, err := startCallbackServer(state)
	if err != nil {
		return nil, err
	}

	return &Authorizer{
		AuthURL:   authURL,
		discovery: discovery,
		pkce:      pkce,
		server:    server,
	}, nil
}

// Wait blocks until the user completes the browser flow, then exchanges the
// authorization code for an OAuth token. It returns an error if the context
// is cancelled or the flow times out. The callback server is shut down before
// Wait returns, so the loopback port is not held past the flow.
func (a *Authorizer) Wait(ctx context.Context) (*oauth.Token, error) {
	defer a.server.close()

	ctx, cancel := context.WithTimeout(ctx, oauthTimeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("xAI OAuth did not complete: %w", ctx.Err())
	case res := <-a.server.resultCh:
		if res.err != nil {
			return nil, res.err
		}
		return exchangeCode(ctx, a.discovery.TokenEndpoint, res.code, a.pkce)
	}
}

// Close shuts down the loopback callback server. It is safe to call multiple
// times.
func (a *Authorizer) Close() {
	a.server.close()
}

// tokenResponse is the JSON body of an xAI OAuth token endpoint response.
type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// exchangeCode swaps an authorization code for an OAuth token.
func exchangeCode(ctx context.Context, tokenEndpoint, code string, p pkceChallenge) (*oauth.Token, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI())
	form.Set("client_id", clientID())
	form.Set("code_verifier", p.verifier)
	// xAI re-validates the PKCE challenge fields at token exchange for the
	// shared client, so they are resent here alongside the verifier.
	form.Set("code_challenge", p.challenge)
	form.Set("code_challenge_method", "S256")

	token, err := postToken(ctx, tokenEndpoint, form, "")
	if err != nil {
		return nil, err
	}
	if token.RefreshToken == "" {
		return nil, errors.New("xAI OAuth token response is missing a refresh token")
	}
	return token, nil
}

// RefreshToken exchanges a refresh token for a fresh xAI OAuth token. It is
// called by the config store to renew expired credentials.
func RefreshToken(ctx context.Context, refreshToken string) (*oauth.Token, error) {
	if refreshToken == "" {
		return nil, errors.New("xAI OAuth credential is missing a refresh token")
	}

	discovery, err := fetchDiscovery(ctx)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", clientID())
	form.Set("refresh_token", refreshToken)

	return postToken(ctx, discovery.TokenEndpoint, form, refreshToken)
}

// postToken posts a token request and parses the response into an oauth.Token.
// priorRefresh is retained when the response omits a new refresh token.
func postToken(ctx context.Context, tokenEndpoint string, form url.Values, priorRefresh string) (*oauth.Token, error) {
	endpoint, err := trustedEndpoint(tokenEndpoint, "token endpoint")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xAI OAuth token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	return parseTokenResponse(body, resp.StatusCode, priorRefresh)
}

// parseTokenResponse decodes and validates an xAI token endpoint response.
func parseTokenResponse(body []byte, statusCode int, priorRefresh string) (*oauth.Token, error) {
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("decode token response (status %d): %w", statusCode, err)
	}
	if tr.Error != "" {
		desc := tr.ErrorDescription
		if desc == "" {
			desc = tr.Error
		}
		return nil, fmt.Errorf("xAI OAuth token request failed: %s", desc)
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("xAI OAuth token request failed with status %d", statusCode)
	}
	if tr.AccessToken == "" {
		return nil, errors.New("xAI OAuth token response is missing an access token")
	}
	return newToken(tr, priorRefresh), nil
}

// newToken converts a token response into an oauth.Token, computing the
// expiry from expires_in or, failing that, the access token's JWT exp claim.
func newToken(tr tokenResponse, priorRefresh string) *oauth.Token {
	token := &oauth.Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
	}
	if token.RefreshToken == "" {
		token.RefreshToken = priorRefresh
	}

	switch {
	case tr.ExpiresIn > 0:
		token.ExpiresIn = tr.ExpiresIn
		token.SetExpiresAt()
	default:
		if exp := jwtExpiry(tr.AccessToken); exp > 0 {
			token.ExpiresAt = exp
			token.SetExpiresIn()
		} else {
			token.ExpiresIn = defaultTokenLifetimeSecs
			token.SetExpiresAt()
		}
	}
	return token
}

// jwtExpiry returns the exp claim (unix seconds) of a JWT access token, or 0
// if it cannot be determined.
func jwtExpiry(token string) int64 {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return 0
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0
	}
	return claims.Exp
}
