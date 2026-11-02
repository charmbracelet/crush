package permission

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startPendingPrompt issues a request that no allowlist, grant, or hook
// short-circuits, so it blocks on the human prompt. It waits for the
// prompt to be published before returning, so callers can toggle the
// level against a request that is genuinely open.
func startPendingPrompt(t *testing.T, svc Service, toolCallID string) (<-chan bool, <-chan error) {
	t.Helper()
	events := svc.Subscribe(t.Context())
	grantedCh := make(chan bool, 1)
	errCh := make(chan error, 1)
	go func() {
		granted, err := svc.Request(t.Context(), CreatePermissionRequest{
			SessionID:  "s1",
			ToolCallID: toolCallID,
			ToolName:   "bash",
			Action:     "execute",
			Path:       t.TempDir(),
		})
		grantedCh <- granted
		errCh <- err
	}()
	select {
	case <-events:
	case <-time.After(2 * time.Second):
		t.Fatal("request never surfaced as a prompt")
	}
	return grantedCh, errCh
}

// awaitRequestOutcome waits for the request to finish, failing the test
// if it is still pending after the timeout.
func awaitRequestOutcome(t *testing.T, grantedCh <-chan bool, errCh <-chan error) (bool, error) {
	t.Helper()
	select {
	case granted := <-grantedCh:
		return granted, <-errCh
	case <-time.After(2 * time.Second):
		t.Fatal("a mid-run level change must resolve the open prompt")
		return false, nil
	}
}

// TestSetLevelBypassSettlesPendingPrompt pins the apply-immediately
// behavior: switching to bypass while a prompt is open grants the open
// request instead of leaving the turn waiting on a stale prompt.
func TestSetLevelBypassSettlesPendingPrompt(t *testing.T) {
	t.Parallel()
	svc := NewPermissionService(t.TempDir(), false, nil, nil)
	grantedCh, errCh := startPendingPrompt(t, svc, "call-bypass")

	svc.SetLevel(LevelBypass)

	granted, err := awaitRequestOutcome(t, grantedCh, errCh)
	require.NoError(t, err)
	assert.True(t, granted, "bypass grants the open prompt")
	assert.Empty(t, svc.EscalationNote("call-bypass"),
		"a grant nobody answered must not read as human approval")
	_, active := svc.ActiveRequest()
	assert.False(t, active, "no request may stay pending after bypass")
}

// TestSetLevelAutoReclassifiesPendingPrompt pins the auto path: an open
// prompt is re-decided by the freshly armed classifier, so a safe call
// proceeds and a risky one is denied without waiting for a human answer.
func TestSetLevelAutoReclassifiesPendingPrompt(t *testing.T) {
	t.Parallel()

	t.Run("allow grants", func(t *testing.T) {
		t.Parallel()
		svc := NewPermissionService(t.TempDir(), false, nil, nil)
		hooks := &fakePermissionHooks{decision: PreHookResult{Decision: HookDecisionNone}}
		svc.SetPermissionHooks(hooks)
		grantedCh, errCh := startPendingPrompt(t, svc, "call-auto-allow")

		hooks.mu.Lock()
		hooks.decision = PreHookResult{Decision: HookDecisionAllow}
		hooks.mu.Unlock()
		svc.SetLevel(LevelAuto)

		granted, err := awaitRequestOutcome(t, grantedCh, errCh)
		require.NoError(t, err)
		assert.True(t, granted, "a classifier allow settles the open prompt")
		assert.Equal(t, 2, hooks.preCallCount(),
			"the open request is classified once when it arrives and once on the level change")
		assert.Equal(t, 0, hooks.denyCallCount())
	})

	t.Run("deny denies with the reason", func(t *testing.T) {
		t.Parallel()
		svc := NewPermissionService(t.TempDir(), false, nil, nil)
		hooks := &fakePermissionHooks{decision: PreHookResult{Decision: HookDecisionNone}}
		svc.SetPermissionHooks(hooks)
		grantedCh, errCh := startPendingPrompt(t, svc, "call-auto-deny")

		hooks.mu.Lock()
		hooks.decision = PreHookResult{Decision: HookDecisionDeny, Reason: "too risky"}
		hooks.mu.Unlock()
		svc.SetLevel(LevelAuto)

		granted, err := awaitRequestOutcome(t, grantedCh, errCh)
		require.NoError(t, err)
		assert.False(t, granted)
		assert.Equal(t, "too risky", svc.DenialReason("call-auto-deny"))
		svc.(*permissionService).waitForHookDispatches()
		assert.Equal(t, 1, hooks.denyCallCount(),
			"PermissionDenied fires for a classifier denial, same as a fresh request")
	})

	t.Run("unsure leaves the prompt for the human", func(t *testing.T) {
		t.Parallel()
		svc := NewPermissionService(t.TempDir(), false, nil, nil)
		hooks := &fakePermissionHooks{decision: PreHookResult{Decision: HookDecisionNone}}
		svc.SetPermissionHooks(hooks)
		grantedCh, errCh := startPendingPrompt(t, svc, "call-auto-none")

		svc.SetLevel(LevelAuto)
		svc.(*permissionService).waitForReevaluation()

		select {
		case <-grantedCh:
			t.Fatal("a classifier with no verdict must not settle the prompt")
		case <-time.After(100 * time.Millisecond):
		}
		// The human can still answer the prompt it was asked.
		req, active := svc.ActiveRequest()
		require.True(t, active)
		require.True(t, svc.Grant(req))
		granted, err := awaitRequestOutcome(t, grantedCh, errCh)
		require.NoError(t, err)
		assert.True(t, granted)
	})
}

// TestSetLevelPromptLeavesPendingPrompt pins that returning to the
// prompting level never auto-resolves a prompt: the human is being asked
// again, so the open request stays open.
func TestSetLevelPromptLeavesPendingPrompt(t *testing.T) {
	t.Parallel()
	svc := NewPermissionService(t.TempDir(), false, nil, nil)
	svc.SetLevel(LevelAuto)
	grantedCh, errCh := startPendingPrompt(t, svc, "call-prompt")

	svc.SetLevel(LevelPrompt)
	svc.(*permissionService).waitForReevaluation()

	select {
	case <-grantedCh:
		t.Fatal("switching to the prompting level must leave the prompt open")
	case <-time.After(100 * time.Millisecond):
	}
	req, active := svc.ActiveRequest()
	require.True(t, active)
	require.True(t, svc.Deny(req))
	granted, err := awaitRequestOutcome(t, grantedCh, errCh)
	require.NoError(t, err)
	assert.False(t, granted)
}

// TestSeedAutoModeRuntimeChoiceWins pins the re-initialization behavior:
// the config seed applies at first construction, but once the level was
// chosen at runtime a rebuilt coordinator re-arms its fresh hooks with
// the live state instead of reverting to the config value.
func TestSeedAutoModeRuntimeChoiceWins(t *testing.T) {
	t.Parallel()

	svc := NewPermissionService(t.TempDir(), false, nil, nil)
	hooks := &togglingHooks{}
	svc.SetPermissionHooks(hooks)

	// First construction: the config value seeds the state.
	svc.SeedAutoMode(true)
	assert.True(t, svc.AutoMode())
	assert.Equal(t, []bool{true}, hooks.enabled)

	// The user moves to the prompting level at runtime.
	svc.SetLevel(LevelPrompt)
	assert.False(t, svc.AutoMode())

	// Re-initialization with auto mode still enabled in config must keep
	// the runtime choice and re-forward it to the fresh hooks.
	svc.SeedAutoMode(true)
	assert.False(t, svc.AutoMode(), "a re-seed must not clobber the runtime level")
	assert.Equal(t, false, hooks.enabled[len(hooks.enabled)-1],
		"fresh hooks learn the live state, not the config value")

	// A service the user never touched still follows the config.
	fresh := NewPermissionService(t.TempDir(), false, nil, nil)
	fresh.SeedAutoMode(true)
	assert.True(t, fresh.AutoMode())
}
