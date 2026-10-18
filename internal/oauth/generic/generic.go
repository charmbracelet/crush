// Package generic drives OAuth 2.0 flows from a declarative
// [oauth.AuthSpec], so a provider can be added from config alone instead
// of shipping its own Go client. It supports the authorization code flow
// with PKCE and a loopback redirect, the RFC 8628 device flow, and refresh.
package generic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

// HTTPClient is used for token and discovery requests. Tests replace the
// endpoints instead, so the client only needs a sane timeout.
var HTTPClient = &http.Client{Timeout: 30 * time.Second}

// Endpoints are the OAuth endpoints a spec resolves to.
type Endpoints struct {
	AuthorizeURL  string
	TokenURL      string
	DeviceAuthURL string
}

// metadata is an RFC 8414 authorization server metadata document. Only the
// endpoints this package needs are decoded.
type metadata struct {
	Issuer                      string `json:"issuer"`
	AuthorizationEndpoint       string `json:"authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
}

// Resolve returns the endpoints for the spec. Fields set on the spec win;
// missing ones are discovered from the issuer's well-known documents.
// Resolve returns the endpoints for the spec. Fields set on the spec win;
// missing ones are discovered from the issuer's well-known documents. Only
// the token endpoint is required outright: a refresh needs nothing else,
// while the flows report a missing authorization or device endpoint when
// they start.
func Resolve(ctx context.Context, spec *oauth.AuthSpec) (Endpoints, error) {
	if spec == nil {
		return Endpoints{}, errNoSpec
	}
	endpoints := Endpoints{
		AuthorizeURL:  spec.AuthorizeURL,
		TokenURL:      spec.TokenURL,
		DeviceAuthURL: spec.DeviceAuthURL,
	}
	if endpoints.TokenURL != "" &&
		(endpoints.AuthorizeURL != "" || endpoints.DeviceAuthURL != "") {
		return endpoints, nil
	}

	// Without an issuer there is nothing to look up, so the written-down
	// endpoints are all there is.
	if spec.Issuer == "" {
		if endpoints.TokenURL == "" {
			return Endpoints{}, fmt.Errorf(
				"provider OAuth configuration needs an issuer or explicit token and authorization URLs",
			)
		}
		return endpoints, nil
	}

	meta, err := discover(ctx, spec.Issuer)
	if err != nil {
		return Endpoints{}, err
	}
	endpoints.AuthorizeURL = or(endpoints.AuthorizeURL, meta.AuthorizationEndpoint)
	endpoints.TokenURL = or(endpoints.TokenURL, meta.TokenEndpoint)
	endpoints.DeviceAuthURL = or(endpoints.DeviceAuthURL, meta.DeviceAuthorizationEndpoint)

	if endpoints.TokenURL == "" {
		return Endpoints{}, fmt.Errorf(
			"authorization server %s did not publish a token endpoint", spec.Issuer,
		)
	}
	return endpoints, nil
}

func or(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

// discoveryPaths are the well-known documents tried, in order, for an
// issuer without a path component.
var discoveryPaths = []string{
	"oauth-authorization-server",
	"openid-configuration",
}

// discover fetches the issuer's metadata document.
func discover(ctx context.Context, issuer string) (metadata, error) {
	for _, candidate := range metadataURLs(issuer) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
		if err != nil {
			return metadata{}, err
		}
		req.Header.Set("Accept", "application/json")

		resp, err := HTTPClient.Do(req)
		if err != nil {
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil || resp.StatusCode != http.StatusOK {
			continue
		}

		var meta metadata
		if err := json.Unmarshal(body, &meta); err != nil {
			continue
		}
		if meta.TokenEndpoint != "" {
			return meta, nil
		}
	}
	return metadata{}, fmt.Errorf(
		"unable to discover OAuth endpoints for issuer %s; set authorize_url and token_url explicitly",
		issuer,
	)
}

// metadataURLs builds the candidate metadata URLs for an issuer. An issuer
// with a path component gets .well-known inserted between the host and the
// path (RFC 8414 section 3.1), and the appended form is tried as well
// because some servers publish it that way instead.
func metadataURLs(issuer string) []string {
	trimmed := strings.TrimRight(issuer, "/")
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil
	}
	var urls []string
	for _, path := range discoveryPaths {
		host := *u
		host.Path = "/.well-known/" + path
		urls = append(urls, host.String())
		if u.Path != "" {
			appended := *u
			appended.Path = u.Path + "/.well-known/" + path
			urls = append(urls, appended.String())
		}
	}
	return urls
}

// deviceCodeFields are the extra form fields the device grant needs.
const deviceGrant = "urn:ietf:params:oauth:grant-type:device_code"

// RefreshToken exchanges a refresh token for a fresh access token,
// resolving endpoints from the spec when necessary. Servers that do not
// rotate refresh tokens keep the previous one.
func RefreshToken(ctx context.Context, spec *oauth.AuthSpec, refreshToken string) (*oauth.Token, error) {
	endpoints, err := Resolve(ctx, spec)
	if err != nil {
		return nil, err
	}
	vals := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	if spec.UsesJSONToken() && len(spec.Scopes) > 0 {
		vals.Set("scope", strings.Join(spec.Scopes, " "))
	}
	token, err := requestToken(ctx, spec, endpoints.TokenURL, vals)
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	return token, nil
}

// ExchangeCode trades an authorization code for tokens. The state the flow
// generated is included when the token endpoint takes JSON: the servers
// behind that shape, Anthropic among them, require it.
func ExchangeCode(
	ctx context.Context,
	spec *oauth.AuthSpec,
	tokenURL, code, redirectURI, verifier, state string,
) (*oauth.Token, error) {
	vals := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}
	if state != "" {
		vals.Set("state", state)
	}
	return requestToken(ctx, spec, tokenURL, vals)
}

// authorizeValues seeds the parameters shared by the browser and device
// flows.
func authorizeValues(spec *oauth.AuthSpec) url.Values {
	vals := url.Values{"client_id": {spec.ClientID}}
	if len(spec.Scopes) > 0 {
		vals.Set("scope", strings.Join(spec.Scopes, " "))
	}
	for k, v := range spec.ExtraParams {
		vals.Add(k, v)
	}
	return vals
}

// requestToken posts a grant to the token endpoint and decodes the response
// into an [oauth.Token]. Failures are reported as *oauth.TokenExchangeError
// so callers can tell a revoked refresh token from a transient error.
func requestToken(ctx context.Context, spec *oauth.AuthSpec, tokenURL string, vals url.Values) (*oauth.Token, error) {
	var tr struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := postForm(ctx, spec, tokenURL, vals, &tr); err != nil {
		return nil, err
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("token response contained no access token")
	}

	token := &oauth.Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		IDToken:      tr.IDToken,
		ExpiresIn:    tr.ExpiresIn,
		// AccountID labels which account signed in, which is what OIDC
		// providers carry in the ID token.
		AccountID: oauth.EmailFromIDToken(tr.IDToken),
	}
	token.SetExpiresAt()
	return token, nil
}

// postForm sends a form-encoded request to the endpoint, attaching client
// credentials the way the spec asks, and decodes the JSON response into
// target. A non-200 response becomes *oauth.TokenExchangeError, carrying
// the status and body so callers can classify the failure.
func postForm(ctx context.Context, spec *oauth.AuthSpec, endpoint string, vals url.Values, target any) error {
	if spec.ClientID != "" {
		vals.Set("client_id", spec.ClientID)
	}
	if spec.ClientSecret != "" && !spec.ClientSecretBasic {
		vals.Set("client_secret", spec.ClientSecret)
	}

	payload := vals.Encode()
	contentType := "application/x-www-form-urlencoded"
	if spec.UsesJSONToken() {
		// A JSON endpoint expects the request fields as an object, with the
		// grant's fields in their plain (unencoded) shape.
		object := make(map[string]any, len(vals))
		for key, values := range vals {
			if len(values) > 0 {
				object[key] = values[0]
			}
		}
		encoded, err := json.Marshal(object)
		if err != nil {
			return err
		}
		payload = string(encoded)
		contentType = "application/json"
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		strings.NewReader(payload),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	// Declared headers win: they are how a server that gates on client
	// identity is satisfied.
	for key, value := range spec.TokenHeaders {
		if value != "" {
			req.Header.Set(key, value)
		}
	}
	if spec.ClientSecret != "" && spec.ClientSecretBasic {
		req.SetBasicAuth(spec.ClientID, spec.ClientSecret)
	}

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		err := &oauth.TokenExchangeError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
		// A rejected client is a configuration problem, and the credential
		// can arrive from two places, so name them: without this the user
		// sees only the server's wording and has no idea where to look.
		if errorCode(string(body)) == "invalid_client" {
			return fmt.Errorf(
				"%w (check the provider's auth.client_id and auth.client_secret, including any environment variable your plugin expands into them)",
				err,
			)
		}
		return err
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// errorCode extracts the OAuth error code from a token endpoint error body.
func errorCode(body string) string {
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal([]byte(body), &e)
	return e.Error
}
