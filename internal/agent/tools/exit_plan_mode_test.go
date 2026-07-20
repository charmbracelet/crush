package tools

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type exitPlanModePerms struct {
	*pubsub.Broker[permission.PermissionRequest]
	planMode bool
	allow    bool
	setCalls int
	lastPlan bool
	reqCount int
	lastReq  permission.CreatePermissionRequest
}

func (m *exitPlanModePerms) Request(ctx context.Context, req permission.CreatePermissionRequest) (bool, error) {
	m.reqCount++
	m.lastReq = req
	return m.allow, nil
}

func (m *exitPlanModePerms) Grant(req permission.PermissionRequest) bool { return true }
func (m *exitPlanModePerms) Deny(req permission.PermissionRequest) bool  { return true }
func (m *exitPlanModePerms) GrantPersistent(req permission.PermissionRequest) bool {
	return true
}
func (m *exitPlanModePerms) AutoApproveSession(sessionID string) {}
func (m *exitPlanModePerms) SetSkipRequests(skip bool)           {}
func (m *exitPlanModePerms) SkipRequests() bool                  { return false }
func (m *exitPlanModePerms) SetPlanMode(enabled bool) {
	m.setCalls++
	m.lastPlan = enabled
	m.planMode = enabled
}
func (m *exitPlanModePerms) PlanMode() bool { return m.planMode }
func (m *exitPlanModePerms) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

func runExitPlanMode(t *testing.T, tool fantasy.AgentTool, plan string) fantasy.ToolResponse {
	t.Helper()
	input, err := json.Marshal(ExitPlanModeParams{Plan: plan})
	require.NoError(t, err)
	resp, err := tool.Run(context.WithValue(context.Background(), SessionIDContextKey, "s1"), fantasy.ToolCall{
		ID:    "call-1",
		Name:  ExitPlanModeToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	return resp
}

func TestExitPlanModeTool(t *testing.T) {
	t.Parallel()

	t.Run("errors when not in plan mode", func(t *testing.T) {
		t.Parallel()
		perms := &exitPlanModePerms{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
		tool := NewExitPlanModeTool(perms)
		resp := runExitPlanMode(t, tool, "# plan")
		assert.True(t, resp.IsError)
		assert.Contains(t, resp.Content, "Not in plan mode")
		assert.Equal(t, 0, perms.reqCount)
	})

	t.Run("errors when plan is empty", func(t *testing.T) {
		t.Parallel()
		perms := &exitPlanModePerms{
			Broker:   pubsub.NewBroker[permission.PermissionRequest](),
			planMode: true,
		}
		tool := NewExitPlanModeTool(perms)
		resp := runExitPlanMode(t, tool, "")
		assert.True(t, resp.IsError)
		assert.Contains(t, resp.Content, "plan is required")
		assert.Equal(t, 0, perms.reqCount)
	})

	t.Run("denied keeps plan mode on", func(t *testing.T) {
		t.Parallel()
		perms := &exitPlanModePerms{
			Broker:   pubsub.NewBroker[permission.PermissionRequest](),
			planMode: true,
			allow:    false,
		}
		tool := NewExitPlanModeTool(perms)
		resp := runExitPlanMode(t, tool, "# plan")
		assert.True(t, resp.IsError)
		assert.True(t, perms.planMode)
		assert.Equal(t, 0, perms.setCalls)
		assert.Equal(t, "plan", perms.lastReq.Action)
		assert.Equal(t, ExitPlanModeToolName, perms.lastReq.ToolName)
	})

	t.Run("granted exits plan mode", func(t *testing.T) {
		t.Parallel()
		perms := &exitPlanModePerms{
			Broker:   pubsub.NewBroker[permission.PermissionRequest](),
			planMode: true,
			allow:    true,
		}
		tool := NewExitPlanModeTool(perms)
		resp := runExitPlanMode(t, tool, "# plan")
		assert.False(t, resp.IsError)
		assert.Contains(t, resp.Content, "Plan approved")
		assert.Equal(t, 1, perms.setCalls)
		assert.False(t, perms.lastPlan)
		assert.False(t, perms.planMode)
	})
}
