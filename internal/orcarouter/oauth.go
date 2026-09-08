package orcarouter

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const authorizationLifetime = 10 * time.Minute

// Authorization is one loopback OAuth 2.0 + PKCE attempt.
type Authorization struct {
	URL string

	authBase string
	verifier string
	state    string
	server   *http.Server
	result   chan callbackResult
	cancel   context.CancelFunc
	once     sync.Once
}

type callbackResult struct {
	code string
	err  error
}

// StartAuthorization creates a fresh S256 verifier and starts a loopback
// callback listener before returning the browser authorization URL.
func StartAuthorization() (*Authorization, error) {
	authBase, err := AuthBaseURL()
	if err != nil {
		return nil, err
	}
	verifier, err := randomBase64URL(32)
	if err != nil {
		return nil, fmt.Errorf("generate PKCE verifier: %w", err)
	}
	state, err := randomBase64URL(24)
	if err != nil {
		return nil, fmt.Errorf("generate OAuth state: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), authorizationLifetime)
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("start OrcaRouter callback listener: %w", err)
	}
	a := &Authorization{
		authBase: authBase,
		verifier: verifier,
		state:    state,
		result:   make(chan callbackResult, 1),
		cancel:   cancel,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", a.handleCallback)
	a.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	callbackURL := "http://" + listener.Addr().String() + "/callback"
	a.URL, err = authorizationURL(authBase, callbackURL, verifier, state)
	if err != nil {
		cancel()
		_ = listener.Close()
		return nil, err
	}

	go func() {
		<-ctx.Done()
		a.finish(callbackResult{err: ctx.Err()})
		_ = a.server.Close()
	}()
	go func() {
		if serveErr := a.server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			a.finish(callbackResult{err: fmt.Errorf("serve OrcaRouter callback: %w", serveErr)})
		}
	}()

	return a, nil
}

func authorizationURL(authBase, callbackURL, verifier, state string) (string, error) {
	u, err := url.Parse(authBase)
	if err != nil {
		return "", fmt.Errorf("parse OrcaRouter auth URL: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/auth"
	q := u.Query()
	q.Set("callback_url", callbackURL)
	challenge := sha256.Sum256([]byte(verifier))
	q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(challenge[:]))
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	q.Set("app_name", "Crush")
	q.Set("scope", "api")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func randomBase64URL(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (a *Authorization) handleCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Query().Get("state") != a.state {
		writeCallbackPage(w, http.StatusBadRequest, "Authorization could not be verified. You can close this tab.")
		a.finish(callbackResult{err: errors.New("OrcaRouter authorization state mismatch")})
		return
	}
	if authError := r.URL.Query().Get("error"); authError != "" {
		writeCallbackPage(w, http.StatusBadRequest, "Authorization was declined. You can close this tab.")
		a.finish(callbackResult{err: fmt.Errorf("OrcaRouter authorization failed: %s", safeOAuthError(authError))})
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		writeCallbackPage(w, http.StatusBadRequest, "Authorization code was missing. You can close this tab.")
		a.finish(callbackResult{err: errors.New("OrcaRouter authorization code was missing")})
		return
	}

	writeCallbackPage(w, http.StatusOK, "Authorization received. Return to Crush to continue.")
	a.finish(callbackResult{code: code})
}

func writeCallbackPage(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; base-uri 'none'; form-action 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, callbackPage(message, status == http.StatusOK))
}

func callbackPage(message string, closeWindow bool) string {
	script := `history.replaceState(null, "", location.pathname);`
	if closeWindow {
		script += `window.setTimeout(function () { window.close(); }, 50);`
	}
	return "<!doctype html><html><head><meta charset=\"utf-8\"><title>Crush</title></head>" +
		"<body><main><h1>Crush</h1><p>" + html.EscapeString(message) + "</p></main>" +
		"<script>" + script + "</script></body></html>"
}

func safeOAuthError(value string) string {
	for _, r := range value {
		switch {
		case r == '_', r == '-', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
			return "authorization_error"
		}
	}
	if value == "" {
		return "authorization_error"
	}
	return value
}

func (a *Authorization) finish(result callbackResult) {
	a.once.Do(func() {
		a.result <- result
		a.cancel()
	})
}

// Wait waits for the callback and exchanges the one-time code for a durable
// OrcaRouter API key.
func (a *Authorization) Wait(ctx context.Context) (key, scope string, err error) {
	select {
	case result := <-a.result:
		if result.err != nil {
			return "", "", result.err
		}
		return a.exchange(ctx, result.code)
	case <-ctx.Done():
		return "", "", ctx.Err()
	}
}

func (a *Authorization) exchange(ctx context.Context, code string) (string, string, error) {
	payload, err := json.Marshal(map[string]string{
		"code":                  code,
		"code_verifier":         a.verifier,
		"code_challenge_method": "S256",
	})
	if err != nil {
		return "", "", fmt.Errorf("encode OrcaRouter code exchange: %w", err)
	}
	u, err := url.Parse(a.authBase)
	if err != nil {
		return "", "", fmt.Errorf("parse OrcaRouter token URL: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/v1/auth/keys"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(payload))
	if err != nil {
		return "", "", fmt.Errorf("create OrcaRouter code exchange: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("exchange OrcaRouter authorization code: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", errors.New("read OrcaRouter code exchange response")
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("exchange OrcaRouter authorization code: %s", resp.Status)
	}

	var result struct {
		Key   string `json:"key"`
		Scope string `json:"scope"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.Key == "" {
		return "", "", errors.New("decode OrcaRouter code exchange response")
	}
	if result.Scope != "api" {
		return "", "", errors.New("OrcaRouter authorization returned an unexpected scope")
	}
	return result.Key, result.Scope, nil
}

// Cancel releases the loopback listener and invalidates the attempt.
func (a *Authorization) Cancel() {
	a.finish(callbackResult{err: context.Canceled})
	_ = a.server.Close()
}
