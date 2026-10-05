package grok

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// stubDeviceServer serves a device authorization endpoint that always
// returns the given device code and records the last request.
func stubDeviceServer(t *testing.T, resp any) (*httptest.Server, *url.Values) {
	t.Helper()
	var got url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		got = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(server.Close)

	orig := deviceCodeEndpoint
	deviceCodeEndpoint = server.URL
	t.Cleanup(func() { deviceCodeEndpoint = orig })
	return server, &got
}

func TestRequestDeviceCode(t *testing.T) {
	_, got := stubDeviceServer(t, DeviceCode{
		DeviceCode:      "dc-123",
		UserCode:        "94mJX-FLQP",
		VerificationURI: "https://accounts.x.ai/oauth2/device",
		ExpiresIn:       900,
		Interval:        5,
	})

	dc, err := RequestDeviceCode(context.Background())
	require.NoError(t, err)

	require.Equal(t, ClientID, got.Get("client_id"))
	require.Equal(t, Scope, got.Get("scope"))
	require.Equal(t, "crush", got.Get("referrer"))

	require.Equal(t, "dc-123", dc.DeviceCode)
	require.Equal(t, "94mJX-FLQP", dc.UserCode)
	require.Equal(t, "https://accounts.x.ai/oauth2/device", dc.VerificationURI)
}

func TestRequestDeviceCode_MissingCodes(t *testing.T) {
	_, _ = stubDeviceServer(t, map[string]any{"user_code": "only-user"})

	_, err := RequestDeviceCode(context.Background())
	require.ErrorContains(t, err, "no device or user code")
}

// stubTokenSequence serves a token endpoint that returns the given
// error responses in order, then a successful token response.
func stubTokenSequence(t *testing.T, errors ...string) {
	t.Helper()
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := int(calls.Add(1))
		if n <= len(errors) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"` + errors[n-1] + `"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenResponse{
			AccessToken:  "at-device",
			RefreshToken: "rt-device",
			ExpiresIn:    3600,
		})
	}))
	t.Cleanup(server.Close)

	orig := tokenEndpoint
	tokenEndpoint = server.URL
	t.Cleanup(func() { tokenEndpoint = orig })
}

func TestPollForToken_PendingThenApproved(t *testing.T) {
	// Not parallel: shortens the package poll interval.
	orig := pollInterval
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = orig })

	stubTokenSequence(t, "authorization_pending", "slow_down")

	token, err := PollForToken(context.Background(), "dc-123", 900)
	require.NoError(t, err)
	require.Equal(t, "at-device", token.AccessToken)
	require.Equal(t, "rt-device", token.RefreshToken)
}

func TestPollForToken_Denied(t *testing.T) {
	// Not parallel: shortens the package poll interval.
	orig := pollInterval
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = orig })

	stubTokenSequence(t, "authorization_pending", "access_denied")

	_, err := PollForToken(context.Background(), "dc-123", 900)
	require.ErrorContains(t, err, "denied")
}

func TestPollForToken_Expired(t *testing.T) {
	stubTokenSequence(t, "expired_token")

	_, err := PollForToken(context.Background(), "dc-123", 900)
	require.ErrorContains(t, err, "expired")
}

func TestPollForToken_Deadline(t *testing.T) {
	// Not parallel: shortens the package poll interval.
	orig := pollInterval
	pollInterval = 50 * time.Millisecond
	t.Cleanup(func() { pollInterval = orig })

	// Always pending: the one-second grant expiry must surface as the
	// poll's expired error.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
	}))
	t.Cleanup(server.Close)

	origEndpoint := tokenEndpoint
	tokenEndpoint = server.URL
	t.Cleanup(func() { tokenEndpoint = origEndpoint })

	_, err := PollForToken(context.Background(), "dc-123", 1)
	require.ErrorContains(t, err, "expired")
}

func TestPollForToken_Cancelled(t *testing.T) {
	stubTokenSequence(t, "authorization_pending")

	ctx, cancel := context.WithCancel(context.Background())
	// Poll starts with an immediate request, then sleeps; cancel during
	// the sleep so the poll aborts instead of spinning.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	_, err := PollForToken(ctx, "dc-123", 900)
	require.ErrorIs(t, err, context.Canceled)
}
