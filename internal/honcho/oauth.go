package honcho

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/oauth/callback"
)

// OAuth endpoints are discovered rather than hardcoded, so a
// self-hosted deployment on a different layout works without changes.
const discoveryPath = "/.well-known/oauth-authorization-server"

// oauthScopes is what Crush needs: read memory back and write turns.
const oauthScopes = "read write"

// deviceGrant is advertised in Honcho's discovery document but is
// rejected both at client registration ("unsupported grant_types") and
// at the device endpoint ("unauthorized_client"). Authorization code
// with PKCE over a loopback redirect is the flow that actually works
// for a native client, and it is what RFC 8252 recommends anyway.
const authCodeGrant = "authorization_code"

// authTimeout bounds how long Crush waits for a browser sign-in
// before giving up and releasing the loopback port.
const authTimeout = 5 * time.Minute

// callbackSubject names what is being authorized on the browser
// landing page, matching how the other flows label themselves.
const callbackSubject = "Honcho memory"

// AuthServer describes an authorization server, as published at the
// RFC 8414 discovery endpoint.
type AuthServer struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RegistrationEndpoint  string   `json:"registration_endpoint"`
	RevocationEndpoint    string   `json:"revocation_endpoint"`
	ScopesSupported       []string `json:"scopes_supported"`
	GrantTypes            []string `json:"grant_types_supported"`
	CodeChallengeMethods  []string `json:"code_challenge_methods_supported"`
}

// SupportsPKCE reports whether the server accepts S256 challenges. A
// public client with no secret has nothing else protecting the code
// exchange, so Crush refuses to proceed without it.
func (a *AuthServer) SupportsPKCE() bool {
	for _, m := range a.CodeChallengeMethods {
		if m == "S256" {
			return true
		}
	}
	return false
}

// OAuthToken is a credential obtained from the authorization server.
type OAuthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	Scope        string `json:"scope,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	// ExpiresAt is derived from ExpiresIn at receipt so a stored token
	// can be checked without knowing when it was issued.
	ExpiresAt int64 `json:"expires_at,omitempty"`
	// ClientID is the dynamically registered client this token belongs
	// to. It is needed to refresh, so it travels with the token.
	ClientID string `json:"client_id,omitempty"`
	// Issuer identifies the deployment, so a token minted against a
	// self-hosted server is never replayed against Honcho Cloud.
	Issuer string `json:"issuer,omitempty"`
}

// Expired reports whether the token needs refreshing. A minute of
// slack keeps a turn from starting with a credential that dies
// mid-request.
func (t *OAuthToken) Expired() bool {
	if t == nil || t.AccessToken == "" {
		return true
	}
	if t.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() >= t.ExpiresAt-60
}

// stamp derives ExpiresAt from ExpiresIn at the moment of receipt.
func (t *OAuthToken) stamp() {
	if t.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second).Unix()
	}
}

// ErrGrantRevoked means the authorization server rejected the refresh
// token itself, so no retry can help and the user must sign in again.
//
// It is deliberately distinct from a transport failure: being offline
// must never be mistaken for a revoked grant, or a laptop opened on a
// plane would sign itself out for good.
var ErrGrantRevoked = errors.New("honcho: sign-in is no longer valid")

// Discover fetches the authorization server metadata for a
// deployment.
func Discover(ctx context.Context, baseURL string) (*AuthServer, error) {
	base := strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = DefaultBaseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+discoveryPath, nil)
	if err != nil {
		return nil, fmt.Errorf("honcho: build discovery request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("honcho: discovery request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("honcho: this deployment does not advertise OAuth (status %d); use an API key instead", resp.StatusCode)
	}

	// Bound the body. A self-hosted deployment is not necessarily
	// well-behaved, and an unbounded decode is an easy way to run a
	// client out of memory.
	var meta AuthServer
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&meta); err != nil {
		return nil, fmt.Errorf("honcho: decode discovery document: %w", err)
	}
	if meta.AuthorizationEndpoint == "" || meta.TokenEndpoint == "" {
		return nil, errors.New("honcho: discovery document is missing an authorization or token endpoint")
	}
	return &meta, nil
}

// Register performs RFC 7591 dynamic client registration.
//
// Honcho issues a client ID with no secret, so Crush needs no
// pre-registered credentials baked into the binary and each install
// gets its own identity.
func Register(ctx context.Context, meta *AuthServer, redirectURI string) (string, error) {
	if meta.RegistrationEndpoint == "" {
		return "", errors.New("honcho: deployment does not support dynamic client registration")
	}

	body, err := json.Marshal(map[string]any{
		"client_name":                "Crush",
		"redirect_uris":              []string{redirectURI},
		"grant_types":                []string{authCodeGrant, "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
		"application_type":           "native",
		"scope":                      oauthScopes,
	})
	if err != nil {
		return "", fmt.Errorf("honcho: encode registration: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.RegistrationEndpoint, strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("honcho: build registration request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("honcho: registration request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("honcho: client registration failed (%s): %s", resp.Status, strings.TrimSpace(string(raw)))
	}

	var out struct {
		ClientID string `json:"client_id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("honcho: decode registration: %w", err)
	}
	if out.ClientID == "" {
		return "", errors.New("honcho: registration returned no client id")
	}
	return out.ClientID, nil
}

// pkce holds a PKCE verifier and its S256 challenge.
type pkce struct {
	verifier  string
	challenge string
}

// newPKCE generates a fresh verifier and challenge.
func newPKCE() (*pkce, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("honcho: generate PKCE verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	return &pkce{
		verifier:  verifier,
		challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
	}, nil
}

// randomState generates an anti-CSRF state parameter.
func randomState() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("honcho: generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// AuthFlow is a browser sign-in in progress. Start returns one with
// the URL to open; Wait blocks until the browser comes back.
type AuthFlow struct {
	// URL is the page the user must visit to approve access.
	URL string

	meta     *AuthServer
	clientID string
	verifier string
	state    string
	redirect string

	listener  net.Listener
	server    *http.Server
	results   chan authResult
	closeOnce sync.Once
}

// authResult carries the outcome of the redirect back from the browser.
type authResult struct {
	code string
	err  error
}

// StartAuth begins a browser sign-in against a deployment.
//
// It discovers the authorization server, registers a client, opens a
// loopback listener for the redirect, and returns the URL to visit.
// The caller opens the URL and then calls Wait.
func StartAuth(ctx context.Context, baseURL string) (*AuthFlow, error) {
	meta, err := Discover(ctx, baseURL)
	if err != nil {
		return nil, err
	}
	if !meta.SupportsPKCE() {
		return nil, errors.New("honcho: deployment does not support PKCE, which a client with no secret requires")
	}

	// Bind before registering: the redirect URI must name the actual
	// port, and the server records it at registration time.
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("honcho: open loopback listener: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirect := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	clientID, err := Register(ctx, meta, redirect)
	if err != nil {
		listener.Close() //nolint:errcheck
		return nil, err
	}

	challenge, err := newPKCE()
	if err != nil {
		listener.Close() //nolint:errcheck
		return nil, err
	}
	state, err := randomState()
	if err != nil {
		listener.Close() //nolint:errcheck
		return nil, err
	}

	flow := &AuthFlow{
		meta:     meta,
		clientID: clientID,
		verifier: challenge.verifier,
		state:    state,
		redirect: redirect,
		listener: listener,
		results:  make(chan authResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", flow.handleCallback)
	flow.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go flow.server.Serve(listener) //nolint:errcheck

	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirect},
		"scope":                 {oauthScopes},
		"state":                 {state},
		"code_challenge":        {challenge.challenge},
		"code_challenge_method": {"S256"},
	}
	flow.URL = meta.AuthorizationEndpoint + "?" + params.Encode()
	return flow, nil
}

// handleCallback receives the redirect from the browser.
func (f *AuthFlow) handleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	send := func(res authResult) {
		select {
		case f.results <- res:
		default:
		}
	}

	// The browser is committed to whatever we send by this point, so a
	// render failure has nothing useful left to do. Every Crush
	// authorization flow lands on this same page.
	fail := func(code, desc string, err error) {
		_ = callback.Serve(w, callback.Result{
			Subject:          callbackSubject,
			ErrorCode:        code,
			ErrorDescription: desc,
		})
		send(authResult{err: err})
	}

	// Check state before anything else. An unsolicited request must
	// not be able to steer the flow, including by claiming an error,
	// so the CSRF guard runs ahead of every other branch.
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(f.state)) != 1 {
		fail("invalid_state",
			"This response did not come from the sign-in Crush started.",
			errors.New("honcho: authorization state mismatch"))
		return
	}

	if errCode := q.Get("error"); errCode != "" {
		desc := q.Get("error_description")
		fail(errCode, desc, fmt.Errorf("honcho: authorization denied: %s %s", errCode, desc))
		return
	}

	code := q.Get("code")
	if code == "" {
		fail("invalid_request",
			"The authorization server did not return a code.",
			errors.New("honcho: authorization returned no code"))
		return
	}

	_ = callback.Serve(w, callback.Result{Subject: callbackSubject})
	send(authResult{code: code})
}

// Wait blocks until the browser completes the sign-in, then exchanges
// the code for a token. It always releases the loopback listener.
func (f *AuthFlow) Wait(ctx context.Context) (*OAuthToken, error) {
	defer f.Close()

	ctx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("honcho: timed out waiting for browser sign-in: %w", ctx.Err())
	case res := <-f.results:
		if res.err != nil {
			return nil, res.err
		}
		return f.exchange(ctx, res.code)
	}
}

// Close shuts down the loopback server and releases its port.
//
// Wait calls it, but a caller that abandons a flow before waiting
// (say, because opening the browser failed) must call it too, or the
// listener and its goroutine leak for the life of the process. It is
// safe to call more than once.
func (f *AuthFlow) Close() {
	f.closeOnce.Do(func() {
		if f.server != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			f.server.Shutdown(shutdownCtx) //nolint:errcheck
		}
		if f.listener != nil {
			f.listener.Close() //nolint:errcheck
		}
	})
}

// exchange trades an authorization code for a token.
func (f *AuthFlow) exchange(ctx context.Context, code string) (*OAuthToken, error) {
	form := url.Values{
		"grant_type":    {authCodeGrant},
		"code":          {code},
		"redirect_uri":  {f.redirect},
		"client_id":     {f.clientID},
		"code_verifier": {f.verifier},
	}
	tok, err := postToken(ctx, f.meta.TokenEndpoint, form)
	if err != nil {
		return nil, err
	}
	tok.ClientID = f.clientID
	tok.Issuer = f.meta.Issuer
	return tok, nil
}

// RefreshToken exchanges a refresh token for a fresh access token.
func RefreshToken(ctx context.Context, baseURL string, tok *OAuthToken) (*OAuthToken, error) {
	if tok == nil || tok.RefreshToken == "" {
		return nil, errors.New("honcho: no refresh token available; sign in again")
	}
	meta, err := Discover(ctx, baseURL)
	if err != nil {
		return nil, err
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tok.RefreshToken},
		"client_id":     {tok.ClientID},
	}
	fresh, err := postToken(ctx, meta.TokenEndpoint, form)
	if err != nil {
		return nil, err
	}
	fresh.ClientID = tok.ClientID
	fresh.Issuer = meta.Issuer
	// Servers that do not rotate refresh tokens omit the field, so the
	// existing one must carry forward or the next refresh has nothing
	// to present.
	if fresh.RefreshToken == "" {
		fresh.RefreshToken = tok.RefreshToken
	}
	return fresh, nil
}

// postToken performs a token endpoint request.
func postToken(ctx context.Context, endpoint string, form url.Values) (*OAuthToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("honcho: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("honcho: token request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var oauthErr struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		if json.Unmarshal(raw, &oauthErr) == nil && oauthErr.Error != "" {
			// Only these codes mean the grant itself is dead. Any
			// other failure is worth retrying, so the caller must be
			// able to tell them apart before discarding a token.
			switch oauthErr.Error {
			case "invalid_grant", "invalid_client", "unauthorized_client":
				return nil, fmt.Errorf("%w: %s %s", ErrGrantRevoked, oauthErr.Error, oauthErr.Description)
			}
			return nil, fmt.Errorf("honcho: token exchange failed: %s %s", oauthErr.Error, oauthErr.Description)
		}
		return nil, fmt.Errorf("honcho: token exchange failed (%s): %s", resp.Status, strings.TrimSpace(string(raw)))
	}

	var tok OAuthToken
	if err := json.Unmarshal(raw, &tok); err != nil {
		return nil, fmt.Errorf("honcho: decode token: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, errors.New("honcho: token response contained no access token")
	}
	tok.stamp()
	return &tok, nil
}
