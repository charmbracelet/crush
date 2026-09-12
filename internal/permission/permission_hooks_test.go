package permission

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePermissionHooks records hook invocations for assertions.
type fakePermissionHooks struct {
	mu        sync.Mutex
	decision  PreHookResult
	preCalls  []PermissionRequest
	denyCalls []PermissionRequest
}

func (f *fakePermissionHooks) PrePermission(_ context.Context, req PermissionRequest) PreHookResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.preCalls = append(f.preCalls, req)
	return f.decision
}

func (f *fakePermissionHooks) PermissionDenied(_ context.Context, req PermissionRequest) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.denyCalls = append(f.denyCalls, req)
}

func (f *fakePermissionHooks) preCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.preCalls)
}

func (f *fakePermissionHooks) denyCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.denyCalls)
}

// awaitOutcome reads notifications until an event with a final decision
// (granted or denied) arrives, skipping the leading "requested" bookkeeping
// event the service publishes for every prompt-bound request.
func awaitOutcome(t *testing.T, ch <-chan pubsub.Event[PermissionNotification]) PermissionNotification {
	t.Helper()
	for {
		select {
		case ev, ok := <-ch:
			require.True(t, ok, "notification channel closed before an outcome arrived")
			if ev.Payload.Granted || ev.Payload.Denied {
				return ev.Payload
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for a permission outcome notification")
		}
	}
}

func TestPermissionService_PrePermissionAllow(t *testing.T) {
	t.Parallel()
	service := NewPermissionService("/tmp", false, nil)
	hooks := &fakePermissionHooks{decision: PreHookResult{Decision: HookDecisionAllow}}
	service.SetPermissionHooks(hooks)

	notifications := service.SubscribeNotifications(t.Context())
	granted, err := service.Request(t.Context(), CreatePermissionRequest{
		SessionID:  "s1",
		ToolCallID: "call-1",
		ToolName:   "bash",
		Action:     "execute",
		Path:       "/tmp",
	})
	require.NoError(t, err)
	assert.True(t, granted, "PrePermission allow should bypass the prompt")

	event := awaitOutcome(t, notifications)
	assert.Equal(t, "call-1", event.ToolCallID)
	assert.True(t, event.Granted)
	assert.Equal(t, 1, hooks.preCallCount(), "PrePermission should run exactly once")
}

func TestPermissionService_PrePermissionDeny(t *testing.T) {
	t.Parallel()
	service := NewPermissionService("/tmp", false, nil)
	hooks := &fakePermissionHooks{decision: PreHookResult{
		Decision: HookDecisionDeny,
		Reason:   "command matches a dangerous pattern",
	}}
	service.SetPermissionHooks(hooks)

	notifications := service.SubscribeNotifications(t.Context())
	granted, err := service.Request(t.Context(), CreatePermissionRequest{
		SessionID:  "s1",
		ToolCallID: "call-2",
		ToolName:   "bash",
		Action:     "execute",
		Path:       "/tmp",
	})
	require.NoError(t, err)
	assert.False(t, granted, "PrePermission deny should block the call without prompting")

	event := awaitOutcome(t, notifications)
	assert.Equal(t, "call-2", event.ToolCallID)
	assert.True(t, event.Denied)
	assert.Equal(t, "command matches a dangerous pattern", event.Reason)
}

func TestPermissionService_PrePermissionNoneFallsThroughToPrompt(t *testing.T) {
	t.Parallel()
	service := NewPermissionService("/tmp", false, nil)
	hooks := &fakePermissionHooks{decision: PreHookResult{Decision: HookDecisionNone}}
	service.SetPermissionHooks(hooks)

	events := service.Subscribe(t.Context())
	var (
		wg      sync.WaitGroup
		granted bool
		err     error
	)
	wg.Go(func() {
		granted, err = service.Request(t.Context(), CreatePermissionRequest{
			SessionID:  "s1",
			ToolCallID: "call-3",
			ToolName:   "bash",
			Action:     "execute",
			Path:       "/tmp",
		})
	})

	event := <-events
	service.Grant(event.Payload)
	wg.Wait()
	require.NoError(t, err)
	assert.True(t, granted, "silent hook should fall through to the normal prompt")
	assert.Equal(t, 1, hooks.preCallCount())
}

func TestPermissionService_PrePermissionSkippedByEarlierGrants(t *testing.T) {
	t.Run("allowlisted tool never consults hooks", func(t *testing.T) {
		service := NewPermissionService("/tmp", false, []string{"bash"})
		hooks := &fakePermissionHooks{decision: PreHookResult{Decision: HookDecisionDeny}}
		service.SetPermissionHooks(hooks)

		granted, err := service.Request(t.Context(), CreatePermissionRequest{
			SessionID:  "s1",
			ToolCallID: "call-4",
			ToolName:   "bash",
			Action:     "execute",
			Path:       "/tmp",
		})
		require.NoError(t, err)
		assert.True(t, granted, "allowlist should win before PrePermission runs")
		assert.Zero(t, hooks.preCallCount())
	})

	t.Run("hook approval never consults PrePermission hooks", func(t *testing.T) {
		service := NewPermissionService("/tmp", false, nil)
		hooks := &fakePermissionHooks{decision: PreHookResult{Decision: HookDecisionDeny}}
		service.SetPermissionHooks(hooks)

		ctx := WithHookApproval(t.Context(), "call-5")
		granted, err := service.Request(ctx, CreatePermissionRequest{
			SessionID:  "s1",
			ToolCallID: "call-5",
			ToolName:   "bash",
			Action:     "execute",
			Path:       "/tmp",
		})
		require.NoError(t, err)
		assert.True(t, granted)
		assert.Zero(t, hooks.preCallCount())
	})

	t.Run("session auto-approve never consults PrePermission hooks", func(t *testing.T) {
		service := NewPermissionService("/tmp", false, nil)
		hooks := &fakePermissionHooks{decision: PreHookResult{Decision: HookDecisionDeny}}
		service.SetPermissionHooks(hooks)
		service.AutoApproveSession("s1")

		granted, err := service.Request(t.Context(), CreatePermissionRequest{
			SessionID:  "s1",
			ToolCallID: "call-6",
			ToolName:   "bash",
			Action:     "execute",
			Path:       "/tmp",
		})
		require.NoError(t, err)
		assert.True(t, granted)
		assert.Zero(t, hooks.preCallCount())
	})
}

func TestPermissionService_PermissionDeniedHook(t *testing.T) {
	t.Parallel()
	service := NewPermissionService("/tmp", false, nil)
	hooks := &fakePermissionHooks{}
	service.SetPermissionHooks(hooks)

	events := service.Subscribe(t.Context())
	var (
		wg      sync.WaitGroup
		granted bool
		err     error
	)
	wg.Go(func() {
		granted, err = service.Request(t.Context(), CreatePermissionRequest{
			SessionID:  "s1",
			ToolCallID: "call-7",
			ToolName:   "bash",
			Action:     "execute",
			Path:       "/tmp",
		})
	})

	event := <-events
	service.Deny(event.Payload)
	wg.Wait()
	require.NoError(t, err)
	assert.False(t, granted)

	// The dispatch is asynchronous; wait for it to land.
	service.(*permissionService).waitForHookDispatches()
	assert.Equal(t, 1, hooks.denyCallCount(), "PermissionDenied hook should fire once on denial")
	assert.Equal(t, "call-7", hooks.denyCalls[0].ToolCallID)
}

func TestPermissionService_NoHooksUnchanged(t *testing.T) {
	t.Parallel()
	service := NewPermissionService("/tmp", false, nil)

	events := service.Subscribe(t.Context())
	var (
		wg      sync.WaitGroup
		granted bool
		err     error
	)
	wg.Go(func() {
		granted, err = service.Request(t.Context(), CreatePermissionRequest{
			SessionID:  "s1",
			ToolCallID: "call-8",
			ToolName:   "bash",
			Action:     "execute",
			Path:       "/tmp",
		})
	})

	event := <-events
	service.Grant(event.Payload)
	wg.Wait()
	require.NoError(t, err)
	assert.True(t, granted, "no hooks installed: normal flow must be unchanged")
}
func TestPermissionService_EscalationNote(t *testing.T) {
	t.Parallel()

	t.Run("granted escalation leaves a note for the tool", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil)

		events := service.Subscribe(t.Context())
		var (
			wg      sync.WaitGroup
			granted bool
			err     error
		)
		wg.Go(func() {
			granted, err = service.Request(t.Context(), CreatePermissionRequest{
				SessionID:  "s1",
				ToolCallID: "call-esc",
				ToolName:   "bash",
				Action:     "execute",
				Path:       "/tmp",
			})
		})

		event := <-events
		service.Grant(event.Payload)
		wg.Wait()
		require.NoError(t, err)
		require.True(t, granted)

		note := service.EscalationNote("call-esc")
		assert.Contains(t, note, "approved by the user", "escalated grant must leave a note")
		assert.Empty(t, service.EscalationNote("call-esc"), "note is consumed once")
	})

	t.Run("automatic grants leave no note", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, []string{"bash"})

		granted, err := service.Request(t.Context(), CreatePermissionRequest{
			SessionID:  "s1",
			ToolCallID: "call-auto",
			ToolName:   "bash",
			Action:     "execute",
			Path:       "/tmp",
		})
		require.NoError(t, err)
		require.True(t, granted)
		assert.Empty(t, service.EscalationNote("call-auto"))
	})

	t.Run("denied escalation leaves no note", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil)

		events := service.Subscribe(t.Context())
		var wg sync.WaitGroup
		wg.Go(func() {
			_, _ = service.Request(t.Context(), CreatePermissionRequest{
				SessionID:  "s1",
				ToolCallID: "call-den",
				ToolName:   "bash",
				Action:     "execute",
				Path:       "/tmp",
			})
		})

		event := <-events
		service.Deny(event.Payload)
		wg.Wait()
		assert.Empty(t, service.EscalationNote("call-den"))
	})
}
