package agent

import (
	"errors"
	"net/http"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/assert"
)

func TestCoordinator_isBedrockAuthError(t *testing.T) {
	c := &coordinator{}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"401 unauthorized", &fantasy.ProviderError{StatusCode: http.StatusUnauthorized}, true},
		{"403 forbidden (ExpiredToken)", &fantasy.ProviderError{StatusCode: http.StatusForbidden}, true},
		{"500 internal", &fantasy.ProviderError{StatusCode: http.StatusInternalServerError}, false},
		{"429 rate limit", &fantasy.ProviderError{StatusCode: http.StatusTooManyRequests}, false},
		{"nil error", nil, false},
		{"non-provider error", errors.New("boom"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, c.isBedrockAuthError(tc.err))
		})
	}
}
