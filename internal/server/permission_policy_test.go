package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

type policyAdmissionCoordinator struct {
	*runCoordinator
	accepted atomic.Int32
}

func (c *policyAdmissionCoordinator) BeginAccepted(string) *agent.AcceptedRun {
	c.accepted.Add(1)
	return nil
}

func TestPostAgentPermissionPolicyValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, field string
		valid       bool
		policy      permission.RequestPolicy
	}{
		{"omitted", "", true, permission.RequestPolicyPrompt},
		{"empty", `,"permission_policy":""`, true, permission.RequestPolicyPrompt},
		{"null", `,"permission_policy":null`, true, permission.RequestPolicyPrompt},
		{"auto", `,"permission_policy":"auto_approve"`, true, permission.RequestPolicyAutoApprove},
		{"unknown", `,"permission_policy":"prompt"`, false, permission.RequestPolicyPrompt},
		{"boolean", `,"permission_policy":true`, false, permission.RequestPolicyPrompt},
		{"number", `,"permission_policy":1`, false, permission.RequestPolicyPrompt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			observed := make(chan permission.RequestPolicy, 1)
			coord := &policyAdmissionCoordinator{runCoordinator: newRunCoordinator(func(ctx context.Context) error {
				observed <- permission.RequestPolicyFromContext(ctx)
				return nil
			})}
			close(coord.release)
			c, wsID := buildAgentWorkspace(t, coord)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(fmt.Sprintf(`{"session_id":"session","prompt":"hello"%s}`, tc.field)))
			req.SetPathValue("id", wsID)
			rec := httptest.NewRecorder()
			c.handlePostWorkspaceAgent(rec, req)
			if !tc.valid {
				require.Equal(t, http.StatusBadRequest, rec.Code)
				require.Zero(t, coord.accepted.Load())
				require.Zero(t, coord.ranCount.Load())
				return
			}
			require.Equal(t, http.StatusAccepted, rec.Code)
			require.EqualValues(t, 1, coord.accepted.Load())
			select {
			case policy := <-observed:
				require.Equal(t, tc.policy, policy)
			case <-time.After(5 * time.Second):
				t.Fatal("accepted turn did not run")
			}
		})
	}
}
