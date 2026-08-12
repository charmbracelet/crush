package client

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCheckStatus_TypesActionableStatuses is the regression for the bug
// that made a lost workspace unrecoverable: every response used to
// collapse into an opaque "status code %d" string, so no caller could
// tell "the server no longer knows my workspace" from any other failure,
// and the client just retried the dead ID forever.
func TestCheckStatus_TypesActionableStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		body    string
		accept  []int
		wantErr error
		// wantUntyped asserts the error carries no lifecycle meaning, so
		// callers keep treating it as an ordinary failure.
		wantUntyped bool
		wantMsg     string
	}{
		{name: "ok by default", status: http.StatusOK},
		{
			name:   "accepted when listed",
			status: http.StatusAccepted,
			accept: []int{http.StatusOK, http.StatusAccepted},
		},
		{
			name:    "not found triggers workspace recovery",
			status:  http.StatusNotFound,
			wantErr: ErrNotFound,
		},
		{
			name:    "service unavailable means retry against a replacement",
			status:  http.StatusServiceUnavailable,
			wantErr: ErrServerShuttingDown,
		},
		{
			name:    "server message is surfaced",
			status:  http.StatusNotFound,
			body:    `{"message":"workspace not found"}`,
			wantErr: ErrNotFound,
			wantMsg: "workspace not found",
		},
		{
			name:        "conflict carries no lifecycle meaning",
			status:      http.StatusConflict,
			wantUntyped: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := tc.body
			if body == "" {
				body = "{}"
			}
			err := checkStatus(&http.Response{
				StatusCode: tc.status,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, tc.accept...)

			if tc.wantErr == nil && !tc.wantUntyped {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), "status code")
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			}
			if tc.wantUntyped {
				require.NotErrorIs(t, err, ErrNotFound)
				require.NotErrorIs(t, err, ErrServerShuttingDown)
			}
			if tc.wantMsg != "" {
				require.Contains(t, err.Error(), tc.wantMsg)
			}
		})
	}
}
