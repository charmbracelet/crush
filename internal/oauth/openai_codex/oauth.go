package openai_codex

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

const (
	ProviderID    = "openai-codex"
	ProviderName  = "ChatGPT Plus/Pro (Codex Subscription)"
	clientID      = "app_EMoamEEZ73f0CkXaXp7hrann"
	authorizeURL  = "https://auth.openai.com/oauth/authorize"
	tokenURL      = "https://auth.openai.com/oauth/token"
	redirectURI   = "http://localhost:1455/auth/callback"
	callbackHost  = "127.0.0.1:1455"
	callbackPath  = "/auth/callback"
	oauthScope    = "openid profile email offline_access"
	jwtClaimKey   = "https://api.openai.com/auth"
	defaultOrigin = "crush"
)

type AuthFlow struct {
	State    string
	Verifier string
	AuthURL  string
}

type CallbackResult struct {
	Code string
	Err  error
}

type CallbackServer struct {
	server *http.Server
	ln     net.Listener

	resultC chan CallbackResult
	once    sync.Once
}

func StartAuthFlow(originator string) (*AuthFlow, error) {
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, err
	}
	state, err := randomHex(16)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(authorizeURL)
	if err != nil {
		return nil, fmt.Errorf("parse authorize URL: %w", err)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", oauthScope)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	q.Set("id_token_add_organizations", "true")
	q.Set("codex_cli_simplified_flow", "true")
	q.Set("originator", cmpOr(originator, defaultOrigin))
	u.RawQuery = q.Encode()

	return &AuthFlow{
		State:    state,
		Verifier: verifier,
		AuthURL:  u.String(),
	}, nil
}

func StartLocalCallbackServer(expectedState string) (*CallbackServer, error) {
	resultC := make(chan CallbackResult, 1)
	cs := &CallbackServer{
		resultC: resultC,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != expectedState {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "OAuth failed: state mismatch.")
			cs.emit(CallbackResult{Err: fmt.Errorf("state mismatch")})
			return
		}
		code := strings.TrimSpace(q.Get("code"))
		if code == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "OAuth failed: missing authorization code.")
			cs.emit(CallbackResult{Err: fmt.Errorf("missing authorization code")})
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "OpenAI authentication completed. You can close this window.")
		cs.emit(CallbackResult{Code: code})
	})

	ln, err := net.Listen("tcp", callbackHost)
	if err != nil {
		return nil, err
	}

	cs.ln = ln
	cs.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		_ = cs.server.Serve(ln)
	}()

	return cs, nil
}

func (c *CallbackServer) WaitForCode(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-c.resultC:
		return res.Code, res.Err
	}
}

func (c *CallbackServer) Cancel() {
	c.emit(CallbackResult{Err: context.Canceled})
}

func (c *CallbackServer) Close(ctx context.Context) error {
	if c.server == nil {
		return nil
	}
	return c.server.Shutdown(ctx)
}

func (c *CallbackServer) emit(res CallbackResult) {
	c.once.Do(func() {
		c.resultC <- res
	})
}

func ParseAuthorizationInput(input string) (code, state string) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", ""
	}

	if u, err := url.Parse(value); err == nil && u.Scheme != "" {
		return strings.TrimSpace(u.Query().Get("code")), strings.TrimSpace(u.Query().Get("state"))
	}

	if strings.Contains(value, "#") {
		parts := strings.SplitN(value, "#", 2)
		code = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			state = strings.TrimSpace(parts[1])
		}
		return code, state
	}

	if strings.Contains(value, "code=") {
		q, err := url.ParseQuery(value)
		if err == nil {
			return strings.TrimSpace(q.Get("code")), strings.TrimSpace(q.Get("state"))
		}
	}

	return value, ""
}

func ExchangeAuthorizationCode(ctx context.Context, code, verifier string) (*oauth.Token, error) {
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("client_id", clientID)
	values.Set("code", strings.TrimSpace(code))
	values.Set("code_verifier", verifier)
	values.Set("redirect_uri", redirectURI)
	return exchangeToken(ctx, values)
}

func RefreshToken(ctx context.Context, refreshToken string) (*oauth.Token, error) {
	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("client_id", clientID)
	values.Set("refresh_token", strings.TrimSpace(refreshToken))
	return exchangeToken(ctx, values)
}

func ExtractAccountID(accessToken string) (string, error) {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid JWT format")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode JWT payload: %w", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return "", fmt.Errorf("parse JWT payload: %w", err)
	}

	authAny, ok := payload[jwtClaimKey]
	if !ok {
		return "", fmt.Errorf("missing %q JWT claim", jwtClaimKey)
	}
	authMap, ok := authAny.(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid %q claim shape", jwtClaimKey)
	}
	accountAny, ok := authMap["chatgpt_account_id"]
	if !ok {
		return "", fmt.Errorf("missing chatgpt_account_id claim")
	}
	accountID, ok := accountAny.(string)
	if !ok || strings.TrimSpace(accountID) == "" {
		return "", fmt.Errorf("invalid chatgpt_account_id claim")
	}

	return accountID, nil
}

func exchangeToken(ctx context.Context, values url.Values) (*oauth.Token, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		tokenURL,
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed: status %d body %q", resp.StatusCode, string(body))
	}

	var token oauth.Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	token.SetExpiresAt()
	return &token, nil
}

func generatePKCE() (verifier, challenge string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate PKCE verifier: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func randomHex(bytesN int) (string, error) {
	raw := make([]byte, bytesN)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

func cmpOr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
