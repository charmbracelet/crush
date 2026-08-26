package openai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

const (
	// ClientID is the public OAuth client ID registered for Crush.
	ClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	// Issuer is the OpenAI OAuth issuer.
	Issuer = "https://auth.openai.com"
	// CodexEndpoint is the ChatGPT backend endpoint used for Codex requests.
	CodexEndpoint = "https://chatgpt.com/backend-api/codex/responses"
	// CallbackPort is the fixed loopback port registered for the desktop OAuth client.
	CallbackPort = 1455
)

// PKCE contains the verifier and S256 challenge for an OAuth authorization.
type PKCE struct {
	Verifier, Challenge string
}

// GeneratePKCE creates cryptographically random PKCE values.
func GeneratePKCE() (PKCE, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return PKCE{}, err
	}
	v := base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(v))
	return PKCE{Verifier: v, Challenge: base64.RawURLEncoding.EncodeToString(h[:])}, nil
}

// AuthorizeURL builds the OpenAI authorization URL for a browser flow.
func AuthorizeURL(redirect string, pkce PKCE, state string) string {
	v := url.Values{"response_type": {"code"}, "client_id": {ClientID}, "redirect_uri": {redirect}, "scope": {"openid profile email offline_access"}, "code_challenge": {pkce.Challenge}, "code_challenge_method": {"S256"}, "id_token_add_organizations": {"true"}, "codex_cli_simplified_flow": {"true"}, "state": {state}, "originator": {"crush"}}
	v.Set("originator", "crush")
	return Issuer + "/oauth/authorize?" + v.Encode()
}

// TokenResponse is the token response returned by OpenAI's OAuth endpoint.
type TokenResponse struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// Claims contains non-authoritative account metadata extracted from a JWT
// payload. It must not be used for authorization decisions.
type Claims struct {
	ChatGPTAccountID        string `json:"chatgpt_account_id"`
	ChatGPTComputeResidency string `json:"chatgpt_compute_residency"`
	Organizations           []struct {
		ID string `json:"id"`
	} `json:"organizations"`
	Auth *struct {
		ChatGPTAccountID        string `json:"chatgpt_account_id"`
		ChatGPTComputeResidency string `json:"chatgpt_compute_residency"`
	} `json:"https://api.openai.com/auth"`
}

// ParseJWTClaims extracts account and residency metadata from an untrusted JWT
// payload. It does not authenticate the token and must not be used for
// authorization. The signature is intentionally not checked because this is
// metadata extraction only; no network key discovery is performed.
func ParseJWTClaims(token string) (*Claims, error) {
	p := strings.Split(token, ".")
	if len(p) != 3 || p[0] == "" || p[1] == "" || p[2] == "" {
		return nil, fmt.Errorf("invalid JWT shape")
	}
	for i, part := range p {
		if _, err := base64.RawURLEncoding.DecodeString(part); err != nil {
			return nil, fmt.Errorf("decode JWT segment %d: %w", i, err)
		}
	}
	b, err := base64.RawURLEncoding.DecodeString(p[1])
	if err != nil {
		return nil, fmt.Errorf("decode JWT payload: %w", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("decode JWT payload JSON: %w", err)
	}
	if raw == nil {
		return nil, fmt.Errorf("JWT payload must be a JSON object")
	}
	var c Claims
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("decode JWT claims: %w", err)
	}
	return &c, nil
}

// AccountID returns the account identifier from untrusted token metadata.
func AccountID(c *Claims) string {
	if c == nil {
		return ""
	}
	if c.ChatGPTAccountID != "" {
		return c.ChatGPTAccountID
	}
	if c.Auth != nil && c.Auth.ChatGPTAccountID != "" {
		return c.Auth.ChatGPTAccountID
	}
	if len(c.Organizations) > 0 {
		return c.Organizations[0].ID
	}
	return ""
}

// ValidateState compares the callback state without accepting an empty value.
func ValidateState(expected, actual string) bool {
	return expected != "" && actual != "" && expected == actual
}

// Residency returns the requested compute residency from untrusted token
// metadata, or an empty string when no constraint is present.
func Residency(token string) string {
	c, _ := ParseJWTClaims(token)
	if c == nil {
		return ""
	}
	r := c.ChatGPTComputeResidency
	if c.Auth != nil && c.Auth.ChatGPTComputeResidency != "" {
		r = c.Auth.ChatGPTComputeResidency
	}
	if r == "no_constraint" {
		return ""
	}
	return r
}

// HTTPClient is the HTTP client used by OpenAI OAuth requests. Tests may
// replace it temporarily.
var HTTPClient = &http.Client{Timeout: 30 * time.Second}

func exchange(ctx context.Context, endpoint string, values url.Values) (*oauth.Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, &oauth.TokenExchangeError{StatusCode: resp.StatusCode, Body: string(body)}
	}
	var r TokenResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	t := &oauth.Token{AccessToken: r.AccessToken, RefreshToken: r.RefreshToken, ExpiresIn: r.ExpiresIn, IDToken: r.IDToken}
	t.SetExpiresAt()
	claims, _ := ParseJWTClaims(r.IDToken)
	if claims == nil {
		claims, _ = ParseJWTClaims(r.AccessToken)
	}
	t.AccountID = AccountID(claims)
	t.Residency = Residency(r.IDToken)
	if t.Residency == "" {
		t.Residency = Residency(r.AccessToken)
	}
	return t, nil
}

// ExchangeCode exchanges an authorization code for an OAuth token.
func ExchangeCode(ctx context.Context, code, redirect string, pkce PKCE) (*oauth.Token, error) {
	return exchange(ctx, Issuer+"/oauth/token", url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirect}, "client_id": {ClientID}, "code_verifier": {pkce.Verifier}})
}

// RefreshToken exchanges a refresh token for a new OAuth token.
func RefreshToken(ctx context.Context, refresh string) (*oauth.Token, error) {
	return exchange(ctx, Issuer+"/oauth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {ClientID}})
}

// RedirectURL is the fixed loopback redirect registered for the desktop app.
const RedirectURL = "http://localhost:1455/auth/callback"

// BrowserFlow owns the short-lived loopback server used by the interactive
// authorization-code flow.
type BrowserFlow struct {
	server *http.Server
	result chan browserResult
	url    string
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
}

type browserResult struct {
	token *oauth.Token
	err   error
}

// StartBrowserFlow starts listening before returning the authorization URL,
// avoiding the race where the browser redirects before the callback server is
// ready.
func StartBrowserFlow(ctx context.Context) (*BrowserFlow, error) {
	flowCtx, cancel := context.WithCancel(ctx)
	pkce, err := GeneratePKCE()
	if err != nil {
		cancel()
		return nil, err
	}
	stateBytes := make([]byte, 24)
	if _, err := rand.Read(stateBytes); err != nil {
		cancel()
		return nil, err
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", CallbackPort))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("start OAuth callback server on %s: %w; port %d is required by the registered OAuth redirect, so close the process using it or release the port before retrying", RedirectURL, err, CallbackPort)
	}

	flow := &BrowserFlow{
		result: make(chan browserResult, 1),
		url:    AuthorizeURL(RedirectURL, pkce, state),
		ctx:    flowCtx,
		cancel: cancel,
	}
	flow.server = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/callback" {
			http.NotFound(w, r)
			return
		}
		if !ValidateState(state, r.URL.Query().Get("state")) {
			http.Error(w, "Invalid OAuth state.", http.StatusBadRequest)
			return
		}
		if callbackErr := r.URL.Query().Get("error"); callbackErr != "" {
			flow.finish(nil, fmt.Errorf("OAuth authorization failed: %s", callbackErr))
			fmt.Fprintln(w, "Authorization cancelled. You can close this window.")
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			flow.finish(nil, fmt.Errorf("OAuth callback did not include an authorization code"))
			http.Error(w, "Missing authorization code.", http.StatusBadRequest)
			return
		}
		token, exchangeErr := ExchangeCode(flow.ctx, code, RedirectURL, pkce)
		flow.finish(token, exchangeErr)
		if exchangeErr != nil {
			http.Error(w, "Authorization failed. You can close this window.", http.StatusBadGateway)
			return
		}
		fmt.Fprintln(w, "Authorization successful. You can close this window.")
	})}
	go func() {
		if serveErr := flow.server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			flow.finish(nil, serveErr)
		}
	}()
	go func() {
		<-flow.ctx.Done()
		flow.Close()
	}()
	return flow, nil
}

func (f *BrowserFlow) finish(token *oauth.Token, err error) {
	select {
	case f.result <- browserResult{token: token, err: err}:
	default:
	}
}

// URL returns the URL that must be opened in a browser.
func (f *BrowserFlow) URL() string { return f.url }

// Wait waits for and returns the callback token.
func (f *BrowserFlow) Wait(ctx context.Context) (*oauth.Token, error) {
	select {
	case result := <-f.result:
		f.Close()
		return result.token, result.err
	case <-f.ctx.Done():
		f.Close()
		return nil, f.ctx.Err()
	case <-ctx.Done():
		f.Close()
		return nil, ctx.Err()
	}
}

// Close stops the callback server. It is safe to call more than once.
func (f *BrowserFlow) Close() {
	f.once.Do(func() {
		f.cancel()
		f.finish(nil, context.Canceled)
		_ = f.server.Shutdown(context.Background())
	})
}

// DeviceCode contains the values needed to complete the OpenAI device flow.
type DeviceCode struct {
	DeviceAuthID string `json:"device_auth_id"`
	UserCode     string `json:"user_code"`
	Interval     string `json:"interval"`
}

// RequestDeviceCode starts the OpenAI device authorization flow.
func RequestDeviceCode(ctx context.Context) (d *DeviceCode, err error) {
	body, err := json.Marshal(map[string]string{"client_id": ClientID})
	if err != nil {
		return nil, fmt.Errorf("encode device authorization request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, Issuer+"/api/accounts/deviceauth/usercode", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create device authorization request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, readErr := readResponseBody(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read device authorization response (status %s): %w", resp.Status, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close device authorization response (status %s): %w", resp.Status, closeErr)
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("device authorization failed: %s: %s", resp.Status, safeResponseBody(body))
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, fmt.Errorf("decode device authorization response: %w", err)
	}
	if d == nil {
		return nil, errors.New("decode device authorization response: empty device code")
	}
	return d, nil
}

// PollDevice waits for the user to complete device authorization and exchanges
// the resulting authorization code for an OAuth token.
func PollDevice(ctx context.Context, d *DeviceCode) (*oauth.Token, error) {
	if d == nil {
		return nil, errors.New("device code is nil")
	}
	n, _ := strconv.Atoi(d.Interval)
	if n < 1 {
		n = 5
	}
	ticker := time.NewTicker(time.Duration(n) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
		b, err := json.Marshal(map[string]string{"device_auth_id": d.DeviceAuthID, "user_code": d.UserCode})
		if err != nil {
			return nil, fmt.Errorf("encode device token request: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, Issuer+"/api/accounts/deviceauth/token", strings.NewReader(string(b)))
		if err != nil {
			return nil, fmt.Errorf("create device token request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := readResponseBody(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read device token response (status %s): %w", resp.Status, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close device token response (status %s): %w", resp.Status, closeErr)
		}
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
			continue
		}
		if resp.StatusCode/100 != 2 {
			return nil, fmt.Errorf("device authorization failed: %s: %s", resp.Status, safeResponseBody(body))
		}
		var v struct {
			AuthorizationCode string `json:"authorization_code"`
			CodeVerifier      string `json:"code_verifier"`
		}
		if err := json.Unmarshal(body, &v); err != nil {
			return nil, fmt.Errorf("decode device token response: %w", err)
		}
		return exchange(ctx, Issuer+"/oauth/token", url.Values{"grant_type": {"authorization_code"}, "code": {v.AuthorizationCode}, "redirect_uri": {Issuer + "/deviceauth/callback"}, "client_id": {ClientID}, "code_verifier": {v.CodeVerifier}})
	}
}

func safeResponseBody(body []byte) string {
	var response struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		Message          string `json:"message"`
	}
	if json.Unmarshal(body, &response) != nil {
		return "unexpected error response"
	}
	for _, detail := range []string{response.ErrorDescription, response.Error, response.Message} {
		if detail != "" && len(detail) <= maxResponseBodyDetail {
			return detail
		}
	}
	return "unexpected error response"
}

const (
	maxDeviceResponseBody = 4096
	maxResponseBodyDetail = 256
)

func readResponseBody(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxDeviceResponseBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxDeviceResponseBody {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxDeviceResponseBody)
	}
	return body, nil
}
