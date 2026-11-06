package hyper

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchCredits(t *testing.T) {
	var (
		mu       sync.Mutex
		status   = http.StatusOK
		body     = `{"balance":1234}`
		lastPath string
		lastAuth string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		lastPath = r.URL.Path
		lastAuth = r.Header.Get("Authorization")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("HYPER_URL", srv.URL)

	respond := func(code int, payload string) {
		mu.Lock()
		defer mu.Unlock()
		status, body = code, payload
	}

	t.Run("stores and returns the balance", func(t *testing.T) {
		respond(http.StatusOK, `{"balance":1234}`)

		balance, err := FetchCredits(t.Context(), "test-key")
		require.NoError(t, err)
		require.NotNil(t, balance)
		require.Equal(t, 1234, *balance)
		require.Equal(t, 1234, *Balance())

		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, "/v1/credits", lastPath)
		require.Equal(t, "Bearer test-key", lastAuth)
	})

	t.Run("clears the balance when credits are reported in dollars", func(t *testing.T) {
		respond(http.StatusOK, `{"balance_usd":1.5}`)

		balance, err := FetchCredits(t.Context(), "test-key")
		require.NoError(t, err)
		require.Nil(t, balance)
		require.Nil(t, Balance(), "a team with hypercredit display off must not keep a stale figure")
	})

	t.Run("reports a non-200 response", func(t *testing.T) {
		respond(http.StatusUnauthorized, `{"error":"unauthorized"}`)

		balance, err := FetchCredits(t.Context(), "test-key")
		require.Error(t, err)
		require.Nil(t, balance)
	})

	t.Run("reports a malformed response", func(t *testing.T) {
		respond(http.StatusOK, `not json`)

		balance, err := FetchCredits(t.Context(), "test-key")
		require.Error(t, err)
		require.Nil(t, balance)
	})
}
