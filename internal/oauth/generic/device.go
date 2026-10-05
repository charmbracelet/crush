package generic

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

// pollInterval is the RFC 8628 interval used when the server does not
// advertise one. A variable so tests can shorten the sleeps.
var pollInterval = 5 * time.Second

// DeviceCode is the RFC 8628 device authorization granted by the server.
type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	// VerificationURIComplete carries the pre-filled code when the server
	// offers it, which saves the user typing it.
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// DeviceFlow runs the RFC 8628 device code flow for a spec-driven
// provider: the user opens a verification page and enters a code, while
// Crush polls the token endpoint until the authorization lands.
type DeviceFlow struct {
	spec      *oauth.AuthSpec
	endpoints Endpoints
	code      DeviceCode
}

// StartDevice returns a flow that requests a device code on Start.
func StartDevice(spec *oauth.AuthSpec) *DeviceFlow {
	return &DeviceFlow{spec: spec}
}

// Start requests the device authorization and returns the verification URL
// with the user code to show the user.
func (f *DeviceFlow) Start(ctx context.Context) (string, string, error) {
	endpoints, err := Resolve(ctx, f.spec)
	if err != nil {
		return "", "", err
	}
	if endpoints.DeviceAuthURL == "" {
		return "", "", fmt.Errorf("authorization server %s does not offer a device flow", f.spec.Issuer)
	}
	f.endpoints = endpoints

	code, err := f.requestDeviceCode(ctx)
	if err != nil {
		return "", "", err
	}
	f.code = code

	// The complete URI, when offered, already carries the user code, so
	// the user does not have to type it.
	verifyURL := code.VerificationURI
	if code.VerificationURIComplete != "" {
		verifyURL = code.VerificationURIComplete
	}
	return verifyURL, code.UserCode, nil
}

func (f *DeviceFlow) requestDeviceCode(ctx context.Context) (DeviceCode, error) {
	var code DeviceCode
	if err := postForm(ctx, f.spec, f.endpoints.DeviceAuthURL, authorizeValues(f.spec), &code); err != nil {
		return DeviceCode{}, fmt.Errorf("request device code: %w", err)
	}
	if code.DeviceCode == "" || code.UserCode == "" {
		return DeviceCode{}, errors.New("device authorization response contained no device or user code")
	}
	return code, nil
}

// Wait polls the token endpoint until the user approves the device code,
// it expires, or the authorization is denied.
func (f *DeviceFlow) Wait(ctx context.Context) (*oauth.Token, error) {
	interval := time.Duration(f.code.Interval) * time.Second
	if interval <= 0 {
		interval = pollInterval
	}
	deadline := time.Now().Add(15 * time.Minute)
	if f.code.ExpiresIn > 0 {
		deadline = time.Now().Add(time.Duration(f.code.ExpiresIn) * time.Second)
	}

	for {
		token, err := requestToken(ctx, f.spec, f.endpoints.TokenURL, url.Values{
			"grant_type":  {deviceGrant},
			"device_code": {f.code.DeviceCode},
		})
		if err == nil {
			return token, nil
		}

		var exchangeErr *oauth.TokenExchangeError
		if !errors.As(err, &exchangeErr) {
			return nil, err
		}
		switch errorCode(exchangeErr.Body) {
		case "authorization_pending":
			// Keep waiting for the user.
		case "slow_down":
			interval += pollInterval
		case "expired_token":
			return nil, errors.New("device code expired, please try again")
		case "access_denied":
			return nil, errors.New("authorization was denied")
		default:
			return nil, err
		}

		if !time.Now().Before(deadline) {
			return nil, errors.New("device code expired, please try again")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// Authorization returns the device authorization granted by Start, which
// carries the code the caller must show and the polling hints the server
// advertised.
func (f *DeviceFlow) Authorization() DeviceCode { return f.code }

// Close is a no-op: the device flow holds no local resources.
func (f *DeviceFlow) Close() {}
