package openai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrAuthorizationTimedOut = fmt.Errorf("authorization timed out")
)

const (
	requiredPort     = 1455
	defaultIssuer    = "https://auth.openai.com"
	tokenEndpoint    = "/oauth/token"
	authEndpoint     = "/oauth/authorize"
	redirectURI      = "http://localhost:1455/auth/callback"
	defaultClientID  = "app_EMoamEEZ73f0CkXaXp7hrann"
	loginSuccessHTML = `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Login successful</title>
    <style>
      :root {
        --bg: #171717;
        --fg: #eeeeee;
        --pink: #ff5f87;
        --gray: #626262;
      }
      body {
        background-color: var(--bg);
        color: var(--fg);
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100vh;
        margin: 0;
        -webkit-font-smoothing: antialiased;
      }
      .container {
        max-width: 480px;
        padding: 2rem;
        border-left: 4px solid var(--pink);
      }
      h1 {
        font-size: 1.5rem;
        font-weight: bold;
        margin: 0 0 1rem 0;
        letter-spacing: -0.02em;
      }
      p {
        line-height: 1.6;
        color: var(--fg);
        margin: 0;
      }
      .muted {
        color: var(--gray);
        margin-top: 1.5rem;
        font-size: 0.875rem;
      }
      code {
        color: var(--pink);
      }
    </style>
  </head>
  <body>
    <div class="container">
      <h1>Login successful</h1>
      <p>Your identity has been verified. You can now safely close this tab and return to your terminal.</p>
      <div class="muted">
        Press <code>Ctrl+C</code> in the terminal if the process doesn't resume automatically.
      </div>
    </div>
  </body>
</html>`
)

type TokenData struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	AccountID    string `json:"account_id"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
}

type AuthBundle struct {
	APIKey      string    `json:"api_key"`
	TokenData   TokenData `json:"tokens"`
	LastRefresh string    `json:"last_refresh"`
}

type pkce struct {
	Verifier  string
	Challenge string
}

type OAuthServer struct {
	server   *http.Server
	clientID string
	issuer   string
	pkce     *pkce
	state    string
	err      error
	bundle   *AuthBundle
	done     chan struct{}
	stopOnce sync.Once
}

func (s *OAuthServer) AuthBundle() *AuthBundle {
	return s.bundle
}

func NewOAuthServer() (*OAuthServer, error) {
	p, err := generatePKCE()
	if err != nil {
		return nil, err
	}

	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, err
	}

	return &OAuthServer{
		clientID: defaultClientID,
		issuer:   defaultIssuer,
		pkce:     p,
		state:    fmt.Sprintf("%x", stateBytes),
		done:     make(chan struct{}),
		bundle:   nil,
	}, nil
}

func (s *OAuthServer) AuthURL() string {
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", s.clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", "openid profile email offline_access")
	params.Set("code_challenge", s.pkce.Challenge)
	params.Set("code_challenge_method", "S256")
	params.Set("id_token_add_organizations", "true")
	params.Set("codex_cli_simplified_flow", "true")
	params.Set("state", s.state)

	return fmt.Sprintf("%s%s?%s", s.issuer, authEndpoint, params.Encode())
}

func (s *OAuthServer) Start() error {
	select {
	case <-s.done:
		return s.err
	default:
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/success", s.handleSuccess)
	mux.HandleFunc("/auth/callback", s.handleCallback)

	s.server = &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", requiredPort),
		Handler: mux,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.err = err
			s.shutdown()
		}
	}()

	<-s.done
	return s.err
}

func (s *OAuthServer) handleSuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(loginSuccessHTML))

	go func() {
		s.shutdown()
	}()
}

func (s *OAuthServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing auth code", http.StatusBadRequest)
		go s.shutdown()
		return
	}

	bundle, err := s.exchangeCode(code)
	if err != nil {
		s.err = err
		http.Error(w, err.Error(), http.StatusInternalServerError)
		go s.shutdown()
		return
	}

	s.bundle = bundle

	s.handleSuccess(w, r)
}

func (s *OAuthServer) shutdown() {
	s.stopOnce.Do(func() {
		if s.server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.server.Shutdown(ctx)
		}
		close(s.done)
	})
}

func (s *OAuthServer) Stop() error {
	s.shutdown()
	if s.err != nil {
		return s.err
	}
	return nil
}

func (s *OAuthServer) exchangeCode(code string) (*AuthBundle, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", s.clientID)
	data.Set("code_verifier", s.pkce.Verifier)

	resp, err := http.PostForm(s.issuer+tokenEndpoint, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token exchange failed with status: %d", resp.StatusCode)
	}

	var payload struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	idClaims, _ := parseJWTClaims(payload.IDToken)
	accessClaims, _ := parseJWTClaims(payload.AccessToken)

	var accountID string
	if auth, ok := idClaims["https://api.openai.com/auth"].(map[string]interface{}); ok {
		if val, ok := auth["chatgpt_account_id"].(string); ok {
			accountID = val
		}
	}

	expiresAt := extractUnixClaim(accessClaims, "exp")
	expiresIn := payload.ExpiresIn
	if expiresIn <= 0 && expiresAt > 0 {
		secondsLeft := int(time.Until(time.Unix(expiresAt, 0)).Seconds())
		if secondsLeft > 0 {
			expiresIn = secondsLeft
		}
	}

	tokenData := TokenData{
		IDToken:      payload.IDToken,
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		AccountID:    accountID,
		ExpiresIn:    expiresIn,
		ExpiresAt:    expiresAt,
	}

	apiKey, err := s.maybeObtainAPIKey(idClaims, tokenData)
	if err != nil {
		return nil, err
	}

	finalKey := ""
	if apiKey != nil {
		finalKey = *apiKey
	}

	bundle := &AuthBundle{
		APIKey:      finalKey,
		TokenData:   tokenData,
		LastRefresh: time.Now().UTC().Format(time.RFC3339),
	}

	return bundle, nil
}

func (s *OAuthServer) maybeObtainAPIKey(idClaims map[string]interface{}, tokenData TokenData) (*string, error) {
	orgID, _ := idClaims["organization_id"].(string)
	projectID, _ := idClaims["project_id"].(string)

	if orgID == "" || projectID == "" {
		vals := url.Values{}
		vals.Set("needs_setup", "false")
		return nil, nil
	}

	today := time.Now().UTC().Format("2006-01-02")
	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	data.Set("client_id", s.clientID)
	data.Set("requested_token", "openai-api-key")
	data.Set("subject_token", tokenData.IDToken)
	data.Set("subject_token_type", "urn:ietf:params:oauth:token-type:id_token")
	data.Set("name", fmt.Sprintf("ChatGPT Local [auto-generated] (%s)", today))

	req, err := http.NewRequest("POST", s.issuer+tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil
	}

	vals := url.Values{}
	vals.Set("needs_setup", "false")
	vals.Set("access_token", tokenData.AccessToken)
	vals.Set("refresh_token", tokenData.RefreshToken)
	vals.Set("exchanged_access_token", result.AccessToken)

	return &result.AccessToken, nil
}

func generatePKCE() (*pkce, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(b)
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return &pkce{Verifier: verifier, Challenge: challenge}, nil
}

func parseJWTClaims(token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, errors.New("invalid token format")
	}
	segment := parts[1]
	if l := len(segment) % 4; l > 0 {
		segment += strings.Repeat("=", 4-l)
	}
	decoded, err := base64.URLEncoding.DecodeString(segment)
	if err != nil {
		return nil, err
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func extractUnixClaim(claims map[string]interface{}, key string) int64 {
	if claims == nil {
		return 0
	}

	raw, ok := claims[key]
	if !ok || raw == nil {
		return 0
	}

	switch value := raw.(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case json.Number:
		n, err := value.Int64()
		if err == nil {
			return n
		}
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err == nil {
			return n
		}
	}

	return 0
}

func (s *OAuthServer) WaitForAuthorization(ctx context.Context) error {
	timer := time.NewTimer(5 * time.Minute)
	defer timer.Stop()

	select {
	case <-s.done:
		if s.err != nil {
			return s.err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ErrAuthorizationTimedOut
	}
}
