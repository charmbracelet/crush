package openai

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type failingBody struct {
	io.Reader
	closeErr error
}

func (b failingBody) Close() error { return b.closeErr }

func withHTTPClient(t *testing.T, client *http.Client) {
	t.Helper()
	old := HTTPClient
	HTTPClient = client
	t.Cleanup(func() { HTTPClient = old })
}

func TestParseJWTClaimsOnlyAcceptsDefensivePayloadShape(t *testing.T) {
	t.Parallel()
	payload := "eyJjaGF0Z3B0X2FjY291bnRfaWQiOiJhY2N0IiwiaHR0cHM6Ly9hcGkub3BlbmFpLmNvbS9hdXRoIjp7ImNoYXRncHRfY29tcHV0ZV9yZXNpZGVuY3kiOiJ1cyJ9fQ"
	token := "aGVhZGVy." + payload + ".c2lnbmF0dXJl"
	claims, err := ParseJWTClaims(token)
	require.NoError(t, err)
	require.Equal(t, "acct", AccountID(claims))
	require.Equal(t, "us", Residency(token))

	for _, token := range []string{"", "header.payload", "..", "header.%%%%.signature", "header.bm90LWpzb24.signature", "header.bnVsbA.signature"} {
		_, err := ParseJWTClaims(token)
		require.Error(t, err, token)
	}
}

func TestRequestDeviceCodeIncludesResponseContextWithoutSecrets(t *testing.T) {
	withHTTPClient(t, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Status:     "502 Bad Gateway",
			Body:       io.NopCloser(strings.NewReader(`{"error":"bad","access_token":"secret"}`)),
			Request:    r,
		}, nil
	})})
	_, err := RequestDeviceCode(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "502 Bad Gateway")
	require.Contains(t, err.Error(), "bad")
	require.NotContains(t, err.Error(), "secret")
}

func TestRequestDeviceCodeReturnsBodyErrors(t *testing.T) {
	closeErr := errors.New("close failed")
	withHTTPClient(t, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: failingBody{Reader: strings.NewReader(`{}`), closeErr: closeErr}, Request: r}, nil
	})})
	_, err := RequestDeviceCode(context.Background())
	require.ErrorIs(t, err, closeErr)
}

func TestRequestDeviceCodeDoesNotExposeOversizedResponseBody(t *testing.T) {
	withHTTPClient(t, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadGateway, Status: "502 Bad Gateway", Body: io.NopCloser(strings.NewReader(strings.Repeat("secret", 2000))), Request: r}, nil
	})})
	_, err := RequestDeviceCode(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "response body exceeds")
	require.NotContains(t, err.Error(), "secret")
}

func TestRequestDeviceCodeDoesNotExposeNoncanonicalResponseBody(t *testing.T) {
	withHTTPClient(t, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadGateway, Status: "502 Bad Gateway", Body: io.NopCloser(strings.NewReader(`{"unexpected":"secret"}`)), Request: r}, nil
	})})
	_, err := RequestDeviceCode(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected error response")
	require.NotContains(t, err.Error(), "secret")
}

func TestBrowserFlowInvalidStateDoesNotFinishFlow(t *testing.T) {
	withHTTPClient(t, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(`{"access_token":"token","refresh_token":"refresh","expires_in":3600}`)), Request: r}, nil
	})})
	flow, err := StartBrowserFlow(context.Background())
	if err != nil {
		t.Skipf("callback port is unavailable: %v", err)
	}
	defer flow.Close()

	authURL, err := url.Parse(flow.URL())
	require.NoError(t, err)
	state := authURL.Query().Get("state")
	_, err = http.Get(RedirectURL + "?state=invalid&code=ignored")
	require.NoError(t, err)

	resp, err := http.Get(RedirectURL + "?state=" + url.QueryEscape(state) + "&code=valid")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	token, err := flow.Wait(context.Background())
	require.NoError(t, err)
	require.Equal(t, "token", token.AccessToken)
}

func TestStartBrowserFlowReportsRequiredOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:1455")
	if err != nil {
		t.Skipf("callback port is unavailable for test setup: %v", err)
	}
	defer listener.Close()

	_, err = StartBrowserFlow(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "port 1455 is required")
	require.Contains(t, err.Error(), "release the port")
}
