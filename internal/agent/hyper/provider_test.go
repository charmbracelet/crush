package hyper

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	t.Run("parses delta seconds", func(t *testing.T) {
		t.Parallel()

		resp := &http.Response{Header: http.Header{"Retry-After": {"30"}}}
		require.Equal(t, 30*time.Second, parseRetryAfter(resp))
	})

	t.Run("parses http date", func(t *testing.T) {
		t.Parallel()

		retryAt := time.Now().UTC().Add(2 * time.Hour)
		resp := &http.Response{Header: http.Header{"Retry-After": {retryAt.Format(http.TimeFormat)}}}
		delay := parseRetryAfter(resp)
		require.GreaterOrEqual(t, delay, 119*time.Minute)
		require.LessOrEqual(t, delay, 2*time.Hour)
	})

	t.Run("returns zero for past date", func(t *testing.T) {
		t.Parallel()

		retryAt := time.Now().UTC().Add(-5 * time.Minute)
		resp := &http.Response{Header: http.Header{"Retry-After": {retryAt.Format(http.TimeFormat)}}}
		require.Equal(t, time.Duration(0), parseRetryAfter(resp))
	})

	t.Run("returns zero for invalid value", func(t *testing.T) {
		t.Parallel()

		resp := &http.Response{Header: http.Header{"Retry-After": {"not-a-valid-value"}}}
		require.Equal(t, time.Duration(0), parseRetryAfter(resp))
	})

	t.Run("returns zero for nil response", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, time.Duration(0), parseRetryAfter(nil))
	})
}
