package grok

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

// deviceCodeEndpoint is a variable so tests can point it at a stub
// server.
var deviceCodeEndpoint = Issuer + "/oauth2/device/code"

// DeviceCode is the RFC 8628 device authorization grant issued by the
// authorization server.
type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// RequestDeviceCode starts the device authorization flow. It is the
// sign-in path for machines where the browser cannot reach the loopback
// callback — or when the user declines the browser permission and enters
// a code on accounts.x.ai instead.
func RequestDeviceCode(ctx context.Context) (*DeviceCode, error) {
	vals := url.Values{
		"client_id": {ClientID},
		"scope":     {Scope},
		"referrer":  {"crush"},
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		deviceCodeEndpoint,
		strings.NewReader(vals.Encode()),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read device code response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request device code: status %d body %q", resp.StatusCode, body)
	}

	var dc DeviceCode
	if err := json.Unmarshal(body, &dc); err != nil {
		return nil, fmt.Errorf("decode device code response: %w", err)
	}
	if dc.DeviceCode == "" || dc.UserCode == "" {
		return nil, fmt.Errorf("device code response contained no device or user code")
	}
	return &dc, nil
}

// pollInterval is the RFC 8628 polling interval used when the server
// does not advertise one. A variable so tests can shorten the sleeps.
var pollInterval = 5 * time.Second

// PollForToken polls the token endpoint until the user approves the
// device code on the verification page, it expires, or the authorization
// is denied. ctx cancellation aborts the poll.
func PollForToken(ctx context.Context, deviceCode string, expiresIn int) (*oauth.Token, error) {
	interval := pollInterval
	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
	if expiresIn <= 0 {
		deadline = time.Now().Add(15 * time.Minute)
	}

	for {
		token, err := requestToken(ctx, url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"device_code": {deviceCode},
			"client_id":   {ClientID},
		})
		if err == nil {
			return token, nil
		}

		var exchangeErr *oauth.TokenExchangeError
		if !errors.As(err, &exchangeErr) {
			return nil, err
		}
		switch oauthErrorCode(exchangeErr.Body) {
		case "authorization_pending":
			// Keep waiting for the user.
		case "slow_down":
			interval += pollInterval
		case "expired_token":
			return nil, fmt.Errorf("device code expired, please try again")
		case "access_denied":
			return nil, fmt.Errorf("authorization was denied")
		default:
			return nil, err
		}

		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("device code expired, please try again")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// oauthErrorCode extracts the OAuth error code from a token endpoint
// error body, which requestToken reports verbatim.
func oauthErrorCode(body string) string {
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal([]byte(body), &e)
	return e.Error
}
