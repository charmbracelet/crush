package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/charmbracelet/crush/internal/config"
	crushoauth "github.com/charmbracelet/crush/internal/oauth"
	"github.com/pkg/browser"
	"golang.org/x/oauth2"
)

type AuthRequiredError struct {
	Name                string
	ResourceURL         string
	ResourceMetadataURL string
	Scopes              []string
	Cause               error
}

func (e *AuthRequiredError) Error() string {
	if e == nil {
		return "authentication required"
	}
	parts := []string{"authentication required"}
	if len(e.Scopes) > 0 {
		parts = append(parts, fmt.Sprintf("scopes: %s", strings.Join(e.Scopes, ", ")))
	}
	if e.Cause != nil {
		parts = append(parts, e.Cause.Error())
	}
	return strings.Join(parts, " (") + strings.Repeat(")", len(parts)-1)
}

func (e *AuthRequiredError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NeedsAuth(err error) (*AuthRequiredError, bool) {
	var authErr *AuthRequiredError
	if errors.As(err, &authErr) {
		return authErr, true
	}
	return nil, false
}

type oauthRoundTripper struct {
	base       http.RoundTripper
	headers    map[string]string
	authorizer *mcpOAuthAuthorizer
}

func (rt *oauthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned, err := cloneRequest(req)
	if err != nil {
		return nil, err
	}
	for k, v := range rt.headers {
		cloned.Header.Set(k, v)
	}
	if rt.authorizer != nil && rt.authorizer.store != nil {
		token, err := rt.authorizer.currentAccessToken(cloned.Context(), true)
		if err != nil {
			return nil, err
		}
		if token != "" {
			cloned.Header.Set("Authorization", "Bearer "+token)
		}
	}
	resp, err := rt.transport().RoundTrip(cloned)
	if err != nil || resp == nil || rt.authorizer == nil {
		return resp, err
	}
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
		return resp, nil
	}
	authErr := rt.authorizer.authRequiredForResponse(cloned, resp)
	if authErr == nil {
		return resp, nil
	}
	resp.Body.Close()
	return nil, authErr
}

func (rt *oauthRoundTripper) transport() http.RoundTripper {
	if rt.base != nil {
		return rt.base
	}
	return http.DefaultTransport
}

type mcpOAuthAuthorizer struct {
	name    string
	store   *config.ConfigStore
	headers map[string]string
	client  *http.Client
	mu      sync.Mutex
}

type protectedResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers,omitempty"`
	ScopesSupported      []string `json:"scopes_supported,omitempty"`
}

type authServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported,omitempty"`
}

type clientRegistrationMetadata struct {
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
}

type clientRegistrationResponse struct {
	ClientID                string `json:"client_id"`
	ClientSecret            string `json:"client_secret,omitempty"`
	TokenEndpointAuthMethod string `json:"token_endpoint_auth_method,omitempty"`
}

type wwwAuthChallenge struct {
	Scheme string
	Params map[string]string
}

type resolvedClientRegistration struct {
	clientID     string
	clientSecret string
	authStyle    oauth2.AuthStyle
	registration *config.MCPOAuthRegistration
}

type callbackResult struct {
	code  string
	state string
	err   error
}

type authCallbackServer struct {
	redirectURL string
	result      chan callbackResult
	server      *http.Server
	listener    net.Listener
}

func newMCPOAuthAuthorizer(name string, store *config.ConfigStore, headers map[string]string) *mcpOAuthAuthorizer {
	return &mcpOAuthAuthorizer{
		name:    name,
		store:   store,
		headers: cloneStringMap(headers),
		client:  &http.Client{Transport: http.DefaultTransport},
	}
}

func Authenticate(ctx context.Context, cfg *config.ConfigStore, name string) error {
	m, ok := cfg.Config().MCP[name]
	if !ok {
		return fmt.Errorf("mcp %s not found", name)
	}
	if m.Type != config.MCPHttp {
		return fmt.Errorf("mcp %s does not support interactive authentication", name)
	}
	authorizer := newMCPOAuthAuthorizer(name, cfg, m.ResolvedHeaders())
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := authorizer.authenticate(ctx, m.URL, nil); err != nil {
		prev, _ := states.Get(name)
		updateState(name, stateForError(err), err, nil, prev.Counts)
		return err
	}
	return nil
}

func (a *mcpOAuthAuthorizer) currentAccessToken(ctx context.Context, allowRefresh bool) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	m, err := a.currentConfig()
	if err != nil {
		return "", err
	}
	if m.OAuth == nil || m.OAuth.Token == nil {
		return "", nil
	}
	if allowRefresh && m.OAuth.Token.IsExpired() {
		refreshed, refreshErr := a.refreshTokenLocked(ctx, m)
		if refreshErr == nil && refreshed != nil {
			return refreshed.AccessToken, nil
		}
	}
	return m.OAuth.Token.AccessToken, nil
}

func (a *mcpOAuthAuthorizer) authRequiredForResponse(req *http.Request, resp *http.Response) error {
	if resp == nil {
		return nil
	}
	challenges, parseErr := parseWWWAuthenticate(resp.Header.Values("WWW-Authenticate"))
	if resp.StatusCode == http.StatusForbidden && bearerError(challenges) != "insufficient_scope" {
		return nil
	}
	return &AuthRequiredError{
		Name:                a.name,
		ResourceURL:         req.URL.String(),
		ResourceMetadataURL: resourceMetadataURL(challenges),
		Scopes:              scopesFromChallenges(challenges),
		Cause:               parseErr,
	}
}

func (a *mcpOAuthAuthorizer) authenticate(ctx context.Context, resourceURL string, existing *AuthRequiredError) (*crushoauth.Token, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	m, err := a.currentConfig()
	if err != nil {
		return nil, err
	}
	required := existing
	if required == nil {
		required, err = a.probeForAuthRequired(ctx, resourceURL)
		if err != nil {
			return nil, err
		}
	}
	prm, err := a.discoverProtectedResourceMetadata(ctx, required)
	if err != nil {
		return nil, err
	}
	asm, err := a.discoverAuthServerMetadata(ctx, prm)
	if err != nil {
		return nil, err
	}
	callback, err := startAuthCallbackServer(m.OAuth)
	if err != nil {
		return nil, err
	}
	defer callback.Close()
	resolved, err := a.resolveClientRegistration(ctx, m, asm, callback.redirectURL)
	if err != nil {
		return nil, err
	}
	scopes := required.Scopes
	if len(scopes) == 0 {
		scopes = slices.Clone(prm.ScopesSupported)
	}
	oauthCfg := oauth2.Config{
		ClientID:     resolved.clientID,
		ClientSecret: resolved.clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:   asm.AuthorizationEndpoint,
			TokenURL:  asm.TokenEndpoint,
			AuthStyle: resolved.authStyle,
		},
		RedirectURL: callback.redirectURL,
		Scopes:      scopes,
	}
	verifier := oauth2.GenerateVerifier()
	state := oauth2.GenerateVerifier()
	authURL := oauthCfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("resource", prm.Resource))
	if err := browser.OpenURL(authURL); err != nil {
		return nil, fmt.Errorf("failed to open browser: %w; open %s manually", err, authURL)
	}
	result, err := callback.Wait(ctx)
	if err != nil {
		return nil, err
	}
	if result.state != state {
		return nil, fmt.Errorf("oauth state mismatch")
	}
	token, err := oauthCfg.Exchange(context.WithValue(ctx, oauth2.HTTPClient, a.client), result.code, oauth2.VerifierOption(verifier), oauth2.SetAuthURLParam("resource", prm.Resource))
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	internalToken := fromOAuth2Token(token)
	updated := cloneMCPOAuthConfig(m.OAuth)
	if updated == nil {
		updated = &config.MCPOAuthConfig{}
	}
	updated.Enabled = true
	updated.Token = internalToken
	updated.Resource = prm.Resource
	updated.Scopes = slices.Clone(scopes)
	updated.AuthServer = &config.MCPOAuthAuthServer{
		Issuer:                asm.Issuer,
		AuthorizationEndpoint: asm.AuthorizationEndpoint,
		TokenEndpoint:         asm.TokenEndpoint,
		RegistrationEndpoint:  asm.RegistrationEndpoint,
	}
	if resolved.registration != nil {
		updated.Registration = resolved.registration
	}
	if err := a.store.SetMCPOAuthConfig(config.ScopeGlobal, a.name, updated); err != nil {
		return nil, err
	}
	return internalToken, nil
}

func (a *mcpOAuthAuthorizer) probeForAuthRequired(ctx context.Context, resourceURL string) (*AuthRequiredError, error) {
	methods := []string{http.MethodGet, http.MethodPost}
	for _, method := range methods {
		req, err := http.NewRequestWithContext(ctx, method, resourceURL, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range a.headers {
			req.Header.Set(k, v)
		}
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusMethodNotAllowed {
			resp.Body.Close()
			continue
		}
		authErr := a.authRequiredForResponse(req, resp)
		resp.Body.Close()
		if authErr == nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil, nil
			}
			return nil, fmt.Errorf("authentication not requested by server")
		}
		required, ok := NeedsAuth(authErr)
		if !ok {
			return nil, authErr
		}
		return required, nil
	}
	return nil, fmt.Errorf("authentication probe failed")
}

func (a *mcpOAuthAuthorizer) discoverProtectedResourceMetadata(ctx context.Context, required *AuthRequiredError) (*protectedResourceMetadata, error) {
	if required == nil {
		return nil, fmt.Errorf("authentication metadata unavailable")
	}
	for _, candidate := range protectedResourceMetadataURLs(required.ResourceMetadataURL, required.ResourceURL) {
		prm, status, err := fetchJSON[protectedResourceMetadata](ctx, a.client, http.MethodGet, candidate.metadataURL, nil, a.headers)
		if err != nil {
			continue
		}
		if status >= 400 && status < 500 {
			continue
		}
		if prm == nil {
			continue
		}
		if prm.Resource != candidate.resourceURL {
			continue
		}
		if err := validateAuthServerURLs(prm.AuthorizationServers); err != nil {
			continue
		}
		return prm, nil
	}
	return nil, fmt.Errorf("failed to discover oauth protected resource metadata")
}

func (a *mcpOAuthAuthorizer) discoverAuthServerMetadata(ctx context.Context, prm *protectedResourceMetadata) (*authServerMetadata, error) {
	authServerURL := prm.Resource
	if len(prm.AuthorizationServers) > 0 {
		authServerURL = prm.AuthorizationServers[0]
	} else {
		parsed, err := url.Parse(prm.Resource)
		if err != nil {
			return nil, err
		}
		parsed.Path = ""
		parsed.RawPath = ""
		parsed.RawQuery = ""
		parsed.Fragment = ""
		authServerURL = parsed.String()
	}
	for _, candidate := range authorizationServerMetadataURLs(authServerURL) {
		asm, status, err := fetchJSON[authServerMetadata](ctx, a.client, http.MethodGet, candidate, nil, a.headers)
		if err != nil {
			continue
		}
		if status >= 400 && status < 500 {
			continue
		}
		if asm == nil {
			continue
		}
		if asm.Issuer != "" && asm.Issuer != authServerURL {
			continue
		}
		if asm.AuthorizationEndpoint == "" || asm.TokenEndpoint == "" {
			continue
		}
		return asm, nil
	}
	return &authServerMetadata{
		Issuer:                        authServerURL,
		AuthorizationEndpoint:         strings.TrimRight(authServerURL, "/") + "/authorize",
		TokenEndpoint:                 strings.TrimRight(authServerURL, "/") + "/token",
		RegistrationEndpoint:          strings.TrimRight(authServerURL, "/") + "/register",
		CodeChallengeMethodsSupported: []string{"S256"},
	}, nil
}

func (a *mcpOAuthAuthorizer) resolveClientRegistration(ctx context.Context, m config.MCPConfig, asm *authServerMetadata, redirectURL string) (*resolvedClientRegistration, error) {
	if m.OAuth != nil {
		if m.OAuth.ClientID != "" {
			return &resolvedClientRegistration{
				clientID:     m.OAuth.ClientID,
				clientSecret: m.OAuth.ClientSecret,
				authStyle:    selectAuthStyle(asm.TokenEndpointAuthMethodsSupported, m.OAuth.ClientSecret != ""),
			}, nil
		}
		if m.OAuth.Registration != nil && m.OAuth.Registration.ClientID != "" {
			return &resolvedClientRegistration{
				clientID:     m.OAuth.Registration.ClientID,
				clientSecret: m.OAuth.Registration.ClientSecret,
				authStyle:    selectAuthStyle(asm.TokenEndpointAuthMethodsSupported, m.OAuth.Registration.ClientSecret != ""),
				registration: cloneMCPOAuthRegistration(m.OAuth.Registration),
			}, nil
		}
	}
	if asm.RegistrationEndpoint == "" {
		return nil, fmt.Errorf("oauth client registration is not configured for %s", a.name)
	}
	meta := clientRegistrationMetadata{
		RedirectURIs:            []string{redirectURL},
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "none",
		ClientName:              cmpOr(clientName(m.OAuth), "Crush"),
	}
	registered, err := registerClient(ctx, a.client, asm.RegistrationEndpoint, meta, a.headers)
	if err != nil {
		return nil, err
	}
	return &resolvedClientRegistration{
		clientID:     registered.ClientID,
		clientSecret: registered.ClientSecret,
		authStyle:    authMethodToStyle(cmpOr(registered.TokenEndpointAuthMethod, meta.TokenEndpointAuthMethod), registered.ClientSecret != ""),
		registration: &config.MCPOAuthRegistration{ClientID: registered.ClientID, ClientSecret: registered.ClientSecret},
	}, nil
}

func (a *mcpOAuthAuthorizer) currentConfig() (config.MCPConfig, error) {
	m, ok := a.store.Config().MCP[a.name]
	if !ok {
		return config.MCPConfig{}, fmt.Errorf("mcp %s not found", a.name)
	}
	return m, nil
}

func (a *mcpOAuthAuthorizer) refreshTokenLocked(ctx context.Context, m config.MCPConfig) (*crushoauth.Token, error) {
	if m.OAuth == nil || m.OAuth.Token == nil || m.OAuth.AuthServer == nil {
		return nil, fmt.Errorf("oauth refresh is not configured")
	}
	clientID := ""
	clientSecret := ""
	if m.OAuth.ClientID != "" {
		clientID = m.OAuth.ClientID
		clientSecret = m.OAuth.ClientSecret
	} else if m.OAuth.Registration != nil {
		clientID = m.OAuth.Registration.ClientID
		clientSecret = m.OAuth.Registration.ClientSecret
	}
	if clientID == "" || m.OAuth.Token.RefreshToken == "" || m.OAuth.AuthServer.TokenEndpoint == "" {
		return nil, fmt.Errorf("oauth refresh is not configured")
	}
	cfg := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL:  m.OAuth.AuthServer.TokenEndpoint,
			AuthStyle: selectAuthStyle(nil, clientSecret != ""),
		},
	}
	tokenSource := cfg.TokenSource(context.WithValue(ctx, oauth2.HTTPClient, a.client), toOAuth2Token(m.OAuth.Token))
	token, err := tokenSource.Token()
	if err != nil {
		return nil, err
	}
	internalToken := fromOAuth2Token(token)
	updated := cloneMCPOAuthConfig(m.OAuth)
	updated.Token = internalToken
	if err := a.store.SetMCPOAuthConfig(config.ScopeGlobal, a.name, updated); err != nil {
		return nil, err
	}
	return internalToken, nil
}

func startAuthCallbackServer(oauthCfg *config.MCPOAuthConfig) (*authCallbackServer, error) {
	redirectURL := ""
	if oauthCfg != nil {
		redirectURL = oauthCfg.RedirectURL
	}
	listenAddress := "127.0.0.1:0"
	callbackPath := "/callback"
	if redirectURL != "" {
		parsed, err := url.Parse(redirectURL)
		if err != nil {
			return nil, err
		}
		if parsed.Scheme != "http" || !isLoopbackHost(parsed.Hostname()) {
			return nil, fmt.Errorf("redirect_url must use a loopback http address")
		}
		listenAddress = parsed.Host
		if listenAddress == "" {
			return nil, fmt.Errorf("redirect_url host is required")
		}
		if parsed.Path != "" {
			callbackPath = parsed.Path
		}
	}
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return nil, err
	}
	result := make(chan callbackResult, 1)
	server := &http.Server{}
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		if authErr := r.URL.Query().Get("error"); authErr != "" {
			result <- callbackResult{err: errors.New(authErr)}
			_, _ = io.WriteString(w, "Authentication failed. You can close this window.")
			return
		}
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		if code == "" || state == "" {
			result <- callbackResult{err: fmt.Errorf("missing authorization code")}
			_, _ = io.WriteString(w, "Authentication failed. You can close this window.")
			return
		}
		result <- callbackResult{code: code, state: state}
		_, _ = io.WriteString(w, "Authentication complete. You can close this window.")
	})
	server.Handler = mux
	go func() {
		_ = server.Serve(listener)
	}()
	host := listener.Addr().String()
	return &authCallbackServer{
		redirectURL: "http://" + host + callbackPath,
		result:      result,
		server:      server,
		listener:    listener,
	}, nil
}

func (s *authCallbackServer) Wait(ctx context.Context) (callbackResult, error) {
	select {
	case result := <-s.result:
		if result.err != nil {
			return callbackResult{}, result.err
		}
		return result, nil
	case <-ctx.Done():
		return callbackResult{}, ctx.Err()
	}
}

func (s *authCallbackServer) Close() error {
	if s == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.server.Shutdown(ctx)
	return s.listener.Close()
}

func protectedResourceMetadataURLs(metadataURL, resourceURL string) []struct {
	metadataURL string
	resourceURL string
} {
	urls := []struct {
		metadataURL string
		resourceURL string
	}{}
	if metadataURL != "" {
		urls = append(urls, struct {
			metadataURL string
			resourceURL string
		}{metadataURL: metadataURL, resourceURL: resourceURL})
	}
	parsed, err := url.Parse(resourceURL)
	if err != nil {
		return urls
	}
	atPath := *parsed
	atPath.Path = "/.well-known/oauth-protected-resource/" + strings.TrimLeft(parsed.Path, "/")
	urls = append(urls, struct {
		metadataURL string
		resourceURL string
	}{metadataURL: atPath.String(), resourceURL: resourceURL})
	atRoot := *parsed
	atRoot.Path = "/.well-known/oauth-protected-resource"
	resourceRoot := *parsed
	resourceRoot.Path = ""
	resourceRoot.RawPath = ""
	resourceRoot.RawQuery = ""
	resourceRoot.Fragment = ""
	urls = append(urls, struct {
		metadataURL string
		resourceURL string
	}{metadataURL: atRoot.String(), resourceURL: resourceRoot.String()})
	return urls
}

func authorizationServerMetadataURLs(issuerURL string) []string {
	parsed, err := url.Parse(issuerURL)
	if err != nil {
		return nil
	}
	var urls []string
	if parsed.Path == "" {
		base := *parsed
		base.Path = "/.well-known/oauth-authorization-server"
		urls = append(urls, base.String())
		base.Path = "/.well-known/openid-configuration"
		urls = append(urls, base.String())
		return urls
	}
	originalPath := parsed.Path
	withInsertion := *parsed
	withInsertion.Path = "/.well-known/oauth-authorization-server/" + strings.TrimLeft(originalPath, "/")
	urls = append(urls, withInsertion.String())
	withInsertion.Path = "/.well-known/openid-configuration/" + strings.TrimLeft(originalPath, "/")
	urls = append(urls, withInsertion.String())
	withAppend := *parsed
	withAppend.Path = "/" + strings.Trim(originalPath, "/") + "/.well-known/openid-configuration"
	urls = append(urls, withAppend.String())
	return urls
}

func fetchJSON[T any](ctx context.Context, client *http.Client, method, rawURL string, body []byte, headers map[string]string) (*T, int, error) {
	if err := checkHTTPSOrLoopback(rawURL); err != nil {
		return nil, 0, err
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nil, resp.StatusCode, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, resp.StatusCode, fmt.Errorf("request to %s failed: %s", rawURL, strings.TrimSpace(string(bodyBytes)))
	}
	var out T
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, resp.StatusCode, err
	}
	return &out, resp.StatusCode, nil
}

func registerClient(ctx context.Context, client *http.Client, endpoint string, metadata clientRegistrationMetadata, headers map[string]string) (*clientRegistrationResponse, error) {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	registered, _, err := fetchJSON[clientRegistrationResponse](ctx, client, http.MethodPost, endpoint, payload, headers)
	if err != nil {
		return nil, err
	}
	if registered == nil || registered.ClientID == "" {
		return nil, fmt.Errorf("dynamic client registration failed")
	}
	return registered, nil
}

func cloneRequest(req *http.Request) (*http.Request, error) {
	cloned := req.Clone(req.Context())
	if req.Body == nil {
		return cloned, nil
	}
	if req.GetBody == nil {
		return nil, fmt.Errorf("request body cannot be retried")
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	cloned.Body = body
	return cloned, nil
}

func parseWWWAuthenticate(headers []string) ([]wwwAuthChallenge, error) {
	var challenges []wwwAuthChallenge
	for _, header := range headers {
		parts, err := splitChallenges(header)
		if err != nil {
			return nil, err
		}
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			challenge, err := parseSingleChallenge(part)
			if err != nil {
				return nil, err
			}
			challenges = append(challenges, challenge)
		}
	}
	return challenges, nil
}

func splitChallenges(header string) ([]string, error) {
	var challenges []string
	inQuotes := false
	start := 0
	for i, r := range header {
		switch {
		case r == '"':
			if i > 0 && header[i-1] != '\\' {
				inQuotes = !inQuotes
			} else if i == 0 {
				return nil, errors.New("invalid challenge")
			}
		case r == ',' && !inQuotes:
			lookahead := strings.TrimSpace(header[i+1:])
			eqPos := strings.Index(lookahead, "=")
			isParam := false
			if eqPos > 0 {
				token := lookahead[:eqPos]
				if strings.IndexFunc(token, unicode.IsSpace) == -1 {
					isParam = true
				}
			}
			if !isParam {
				challenges = append(challenges, header[start:i])
				start = i + 1
			}
		}
	}
	challenges = append(challenges, header[start:])
	return challenges, nil
}

func parseSingleChallenge(s string) (wwwAuthChallenge, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return wwwAuthChallenge{}, errors.New("empty challenge")
	}
	scheme, paramsStr, found := strings.Cut(s, " ")
	challenge := wwwAuthChallenge{Scheme: strings.ToLower(scheme), Params: map[string]string{}}
	if !found {
		return challenge, nil
	}
	for paramsStr != "" {
		keyEnd := strings.Index(paramsStr, "=")
		if keyEnd <= 0 {
			return wwwAuthChallenge{}, fmt.Errorf("malformed challenge")
		}
		key := strings.ToLower(strings.TrimSpace(paramsStr[:keyEnd]))
		paramsStr = strings.TrimSpace(paramsStr[keyEnd+1:])
		var value string
		if strings.HasPrefix(paramsStr, "\"") {
			paramsStr = paramsStr[1:]
			var builder strings.Builder
			i := 0
			for ; i < len(paramsStr); i++ {
				if paramsStr[i] == '\\' && i+1 < len(paramsStr) {
					builder.WriteByte(paramsStr[i+1])
					i++
					continue
				}
				if paramsStr[i] == '"' {
					break
				}
				builder.WriteByte(paramsStr[i])
			}
			if i >= len(paramsStr) {
				return wwwAuthChallenge{}, fmt.Errorf("unterminated challenge value")
			}
			value = builder.String()
			paramsStr = strings.TrimSpace(paramsStr[i+1:])
		} else {
			comma := strings.Index(paramsStr, ",")
			if comma == -1 {
				value = strings.TrimSpace(paramsStr)
				paramsStr = ""
			} else {
				value = strings.TrimSpace(paramsStr[:comma])
				paramsStr = strings.TrimSpace(paramsStr[comma+1:])
			}
		}
		challenge.Params[key] = value
		if strings.HasPrefix(paramsStr, ",") {
			paramsStr = strings.TrimSpace(paramsStr[1:])
		}
	}
	return challenge, nil
}

func resourceMetadataURL(challenges []wwwAuthChallenge) string {
	for _, challenge := range challenges {
		if value := challenge.Params["resource_metadata"]; value != "" {
			return value
		}
	}
	return ""
}

func scopesFromChallenges(challenges []wwwAuthChallenge) []string {
	for _, challenge := range challenges {
		if challenge.Scheme == "bearer" && challenge.Params["scope"] != "" {
			return strings.Fields(challenge.Params["scope"])
		}
	}
	return nil
}

func bearerError(challenges []wwwAuthChallenge) string {
	for _, challenge := range challenges {
		if challenge.Scheme == "bearer" && challenge.Params["error"] != "" {
			return challenge.Params["error"]
		}
	}
	return ""
}

func validateAuthServerURLs(urls []string) error {
	for _, rawURL := range urls {
		if err := checkHTTPSOrLoopback(rawURL); err != nil {
			return err
		}
	}
	return nil
}

func checkHTTPSOrLoopback(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname()) {
		return nil
	}
	return fmt.Errorf("url must use https or a loopback http address")
}

func isLoopbackHost(host string) bool {
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func selectAuthStyle(supported []string, hasSecret bool) oauth2.AuthStyle {
	for _, method := range supported {
		style := authMethodToStyle(method, hasSecret)
		if style != oauth2.AuthStyleAutoDetect {
			return style
		}
	}
	if hasSecret {
		return oauth2.AuthStyleInHeader
	}
	return oauth2.AuthStyleInParams
}

func authMethodToStyle(method string, hasSecret bool) oauth2.AuthStyle {
	switch method {
	case "client_secret_post":
		if hasSecret {
			return oauth2.AuthStyleInParams
		}
	case "client_secret_basic":
		if hasSecret {
			return oauth2.AuthStyleInHeader
		}
	case "none", "":
		return oauth2.AuthStyleInParams
	}
	return oauth2.AuthStyleAutoDetect
}

func toOAuth2Token(token *crushoauth.Token) *oauth2.Token {
	if token == nil {
		return nil
	}
	return &oauth2.Token{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expiry:       time.Unix(token.ExpiresAt, 0),
	}
}

func fromOAuth2Token(token *oauth2.Token) *crushoauth.Token {
	if token == nil {
		return nil
	}
	internal := &crushoauth.Token{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
	}
	if !token.Expiry.IsZero() {
		internal.ExpiresAt = token.Expiry.Unix()
		internal.SetExpiresIn()
	}
	return internal
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneMCPOAuthRegistration(in *config.MCPOAuthRegistration) *config.MCPOAuthRegistration {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneMCPOAuthConfig(in *config.MCPOAuthConfig) *config.MCPOAuthConfig {
	if in == nil {
		return nil
	}
	out := *in
	if in.Token != nil {
		token := *in.Token
		out.Token = &token
	}
	if in.Registration != nil {
		registration := *in.Registration
		out.Registration = &registration
	}
	if in.AuthServer != nil {
		authServer := *in.AuthServer
		out.AuthServer = &authServer
	}
	if in.Scopes != nil {
		out.Scopes = slices.Clone(in.Scopes)
	}
	return &out
}

func clientName(oauthCfg *config.MCPOAuthConfig) string {
	if oauthCfg == nil {
		return ""
	}
	return oauthCfg.ClientName
}

func cmpOr(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
