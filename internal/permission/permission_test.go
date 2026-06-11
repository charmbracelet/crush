package permission

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPermissionService_AllowedCommands(t *testing.T) {
	tests := []struct {
		name         string
		allowedTools []string
		toolName     string
		action       string
		expected     bool
	}{
		{
			name:         "tool in allowlist",
			allowedTools: []string{"bash", "view"},
			toolName:     "bash",
			action:       "execute",
			expected:     true,
		},
		{
			name:         "tool:action in allowlist",
			allowedTools: []string{"bash:execute", "edit:create"},
			toolName:     "bash",
			action:       "execute",
			expected:     true,
		},
		{
			name:         "tool not in allowlist",
			allowedTools: []string{"view", "ls"},
			toolName:     "bash",
			action:       "execute",
			expected:     false,
		},
		{
			name:         "tool:action not in allowlist",
			allowedTools: []string{"bash:read", "edit:create"},
			toolName:     "bash",
			action:       "execute",
			expected:     false,
		},
		{
			name:         "empty allowlist",
			allowedTools: []string{},
			toolName:     "bash",
			action:       "execute",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewPermissionService("/tmp", false, tt.allowedTools, nil)

			// Create a channel to capture the permission request
			// Since we're testing the allowlist logic, we need to simulate the request
			ps := service.(*permissionService)

			// Test the allowlist logic directly
			commandKey := tt.toolName + ":" + tt.action
			allowed := false
			for _, cmd := range ps.allowedTools {
				if cmd == commandKey || cmd == tt.toolName {
					allowed = true
					break
				}
			}

			if allowed != tt.expected {
				t.Errorf("expected %v, got %v for tool %s action %s with allowlist %v",
					tt.expected, allowed, tt.toolName, tt.action, tt.allowedTools)
			}
		})
	}
}

func TestSkipRace(t *testing.T) {
	svc := NewPermissionService("/tmp", false, nil, nil)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		svc.SetSkipRequests(true)
	}()
	go func() {
		defer wg.Done()
		svc.SkipRequests()
	}()
	wg.Wait()
}

func TestPermissionService_SkipMode(t *testing.T) {
	service := NewPermissionService("/tmp", true, []string{}, nil)

	result, err := service.Request(t.Context(), CreatePermissionRequest{
		SessionID:   "test-session",
		ToolName:    "bash",
		Action:      "execute",
		Description: "test command",
		Path:        "/tmp",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected permission to be granted in skip mode")
	}
}

func TestPermissionService_HookApproval(t *testing.T) {
	t.Parallel()

	t.Run("matching tool call ID short-circuits the prompt", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		ctx := WithHookApproval(t.Context(), "call-42")
		granted, err := service.Request(ctx, CreatePermissionRequest{
			SessionID:   "s1",
			ToolCallID:  "call-42",
			ToolName:    "bash",
			Action:      "execute",
			Description: "hook-approved command",
			Path:        "/tmp",
		})
		require.NoError(t, err)
		assert.True(t, granted, "hook-approved call should bypass the prompt")
	})

	t.Run("approval is scoped to the stamped tool call ID", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		// Stamp for call-42, ask for a different call ID — must not leak.
		ctx := WithHookApproval(t.Context(), "call-42")

		// Kick off a real request that will need a subscriber to resolve it.
		events := service.Subscribe(t.Context())
		var (
			wg      sync.WaitGroup
			granted bool
			err     error
		)
		wg.Go(func() {
			granted, err = service.Request(ctx, CreatePermissionRequest{
				SessionID:   "s1",
				ToolCallID:  "call-other",
				ToolName:    "bash",
				Action:      "execute",
				Description: "unrelated call",
				Path:        "/tmp",
			})
		})

		// Confirm the service published a real request (i.e. didn't bypass).
		event := <-events
		service.Deny(event.Payload)
		wg.Wait()
		require.NoError(t, err)
		assert.False(t, granted, "stamped approval must not apply to a different tool call")
	})

	t.Run("notifies subscribers that permission was granted", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		notifications := service.SubscribeNotifications(t.Context())

		ctx := WithHookApproval(t.Context(), "call-99")
		granted, err := service.Request(ctx, CreatePermissionRequest{
			SessionID:  "s1",
			ToolCallID: "call-99",
			ToolName:   "view",
			Action:     "read",
			Path:       "/tmp",
		})
		require.NoError(t, err)
		assert.True(t, granted)

		event := <-notifications
		assert.Equal(t, "call-99", event.Payload.ToolCallID)
		assert.True(t, event.Payload.Granted, "subscribers should see a granted notification")
	})
}

func TestPermissionService_SequentialProperties(t *testing.T) {
	t.Run("Sequential permission requests with persistent grants", func(t *testing.T) {
		service := NewPermissionService("/tmp", false, []string{}, nil)

		req1 := CreatePermissionRequest{
			SessionID:   "session1",
			ToolName:    "file_tool",
			Description: "Read file",
			Action:      "read",
			Params:      map[string]string{"file": "test.txt"},
			Path:        "/tmp/test.txt",
		}

		var result1 bool
		var wg sync.WaitGroup
		wg.Add(1)

		events := service.Subscribe(t.Context())

		go func() {
			defer wg.Done()
			result1, _ = service.Request(t.Context(), req1)
		}()

		var permissionReq PermissionRequest
		event := <-events

		permissionReq = event.Payload
		service.GrantPersistent(permissionReq)

		wg.Wait()
		assert.True(t, result1, "First request should be granted")

		// Second identical request should be automatically approved due to persistent permission
		req2 := CreatePermissionRequest{
			SessionID:   "session1",
			ToolName:    "file_tool",
			Description: "Read file again",
			Action:      "read",
			Params:      map[string]string{"file": "test.txt"},
			Path:        "/tmp/test.txt",
		}
		result2, err := service.Request(t.Context(), req2)
		require.NoError(t, err)
		assert.True(t, result2, "Second request should be auto-approved")
	})
	t.Run("Sequential requests with temporary grants", func(t *testing.T) {
		service := NewPermissionService("/tmp", false, []string{}, nil)

		req := CreatePermissionRequest{
			SessionID:   "session2",
			ToolName:    "file_tool",
			Description: "Write file",
			Action:      "write",
			Params:      map[string]string{"file": "test.txt"},
			Path:        "/tmp/test.txt",
		}

		events := service.Subscribe(t.Context())
		var result1 bool
		var wg sync.WaitGroup

		wg.Go(func() {
			result1, _ = service.Request(t.Context(), req)
		})

		var permissionReq PermissionRequest
		event := <-events
		permissionReq = event.Payload

		service.Grant(permissionReq)
		wg.Wait()
		assert.True(t, result1, "First request should be granted")

		var result2 bool

		wg.Go(func() {
			result2, _ = service.Request(t.Context(), req)
		})

		event = <-events
		permissionReq = event.Payload
		service.Deny(permissionReq)
		wg.Wait()
		assert.False(t, result2, "Second request should be denied")
	})
	t.Run("Concurrent requests with different outcomes", func(t *testing.T) {
		service := NewPermissionService("/tmp", false, []string{}, nil)

		events := service.Subscribe(t.Context())

		var wg sync.WaitGroup
		results := make([]bool, 3)

		requests := []CreatePermissionRequest{
			{
				SessionID:   "concurrent1",
				ToolName:    "tool1",
				Action:      "action1",
				Path:        "/tmp/file1.txt",
				Description: "First concurrent request",
			},
			{
				SessionID:   "concurrent2",
				ToolName:    "tool2",
				Action:      "action2",
				Path:        "/tmp/file2.txt",
				Description: "Second concurrent request",
			},
			{
				SessionID:   "concurrent3",
				ToolName:    "tool3",
				Action:      "action3",
				Path:        "/tmp/file3.txt",
				Description: "Third concurrent request",
			},
		}

		for i, req := range requests {
			wg.Add(1)
			go func(index int, request CreatePermissionRequest) {
				defer wg.Done()
				result, _ := service.Request(t.Context(), request)
				results[index] = result
			}(i, req)
		}

		for range 3 {
			event := <-events
			switch event.Payload.ToolName {
			case "tool1":
				service.Grant(event.Payload)
			case "tool2":
				service.GrantPersistent(event.Payload)
			case "tool3":
				service.Deny(event.Payload)
			}
		}
		wg.Wait()
		grantedCount := 0
		for _, result := range results {
			if result {
				grantedCount++
			}
		}

		assert.Equal(t, 2, grantedCount, "Should have 2 granted and 1 denied")
		secondReq := requests[1]
		secondReq.Description = "Repeat of second request"
		result, err := service.Request(t.Context(), secondReq)
		require.NoError(t, err)
		assert.True(t, result, "Repeated request should be auto-approved due to persistent permission")
	})
}

// TestPermissionService_ResolveIdempotency covers the multi-subscriber
// resolve guarantees added for client/server mode: exactly one
// notification per resolution, racing callers see "already resolved",
// and stray Grant/Deny calls for unknown IDs are safe no-ops.
func TestPermissionService_ResolveIdempotency(t *testing.T) {
	t.Parallel()

	t.Run("concurrent grants resolve exactly once", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		req := CreatePermissionRequest{
			SessionID:  "race-session",
			ToolCallID: "race-call",
			ToolName:   "tool",
			Action:     "act",
			Path:       "/tmp/race",
		}

		var (
			wg         sync.WaitGroup
			granted    bool
			requestErr error
		)
		wg.Go(func() {
			granted, requestErr = service.Request(t.Context(), req)
		})

		// Wait for the request to be published so we have a real
		// PermissionRequest (with its server-side ID) to race on.
		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("permission request was never published")
		}

		// Drain the initial "request opened" notification (Granted ==
		// false && Denied == false) so the next read is the resolution
		// itself.
		select {
		case ev := <-notifications:
			require.False(t, ev.Payload.Granted, "initial notification must not be granted")
			require.False(t, ev.Payload.Denied, "initial notification must not be denied")
		case <-time.After(2 * time.Second):
			t.Fatal("initial notification was never published")
		}

		// Race two grants from two goroutines.
		var (
			resolvedCount atomic.Int32
			start         = make(chan struct{})
			racers        sync.WaitGroup
		)
		for range 2 {
			racers.Go(func() {
				<-start
				if service.Grant(pending) {
					resolvedCount.Add(1)
				}
			})
		}
		close(start)
		racers.Wait()

		// Original Request must return granted exactly once.
		wg.Wait()
		require.NoError(t, requestErr)
		assert.True(t, granted, "request should observe its grant")

		// Exactly one of the two grants resolved the request.
		assert.Equal(t, int32(1), resolvedCount.Load(),
			"exactly one Grant should report it resolved the request")

		// Exactly one resolution notification, and no further ones.
		select {
		case ev := <-notifications:
			assert.True(t, ev.Payload.Granted, "resolution notification should be granted")
			assert.Equal(t, "race-call", ev.Payload.ToolCallID)
		case <-time.After(2 * time.Second):
			t.Fatal("resolution notification was never published")
		}
		select {
		case ev := <-notifications:
			t.Fatalf("unexpected duplicate notification: %+v", ev.Payload)
		case <-time.After(50 * time.Millisecond):
			// good: no duplicate.
		}

		// pendingRequests must be empty: no goroutine is left blocked
		// on a send, and a future Grant for the same ID is a no-op.
		ps := service.(*permissionService)
		assert.Equal(t, 0, ps.pendingRequests.Len(),
			"pendingRequests must be empty after resolution")

		assert.False(t, service.Grant(pending),
			"a third Grant should report already-resolved")
	})

	t.Run("grant after deny is a no-op", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		req := CreatePermissionRequest{
			SessionID:  "deny-first",
			ToolCallID: "df-call",
			ToolName:   "tool",
			Action:     "act",
			Path:       "/tmp/df",
		}

		var (
			wg         sync.WaitGroup
			granted    bool
			requestErr error
		)
		wg.Go(func() {
			granted, requestErr = service.Request(t.Context(), req)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("permission request was never published")
		}

		// Drain the initial neither-granted-nor-denied notification.
		<-notifications

		assert.True(t, service.Deny(pending), "Deny should resolve the request")
		wg.Wait()
		require.NoError(t, requestErr)
		assert.False(t, granted, "request should observe denial")

		// A follow-up Grant must be a no-op and must not flip the
		// outcome or publish anything new.
		assert.False(t, service.Grant(pending),
			"Grant after Deny should report already-resolved")

		select {
		case ev := <-notifications:
			// The first resolution notification (denial) is expected;
			// anything after that is a bug.
			require.True(t, ev.Payload.Denied,
				"the only post-initial notification must be the denial")
		case <-time.After(2 * time.Second):
			t.Fatal("denial notification was never published")
		}
		select {
		case ev := <-notifications:
			t.Fatalf("Grant after Deny must not publish: %+v", ev.Payload)
		case <-time.After(50 * time.Millisecond):
			// good.
		}
	})

	t.Run("losing GrantPersistent does not record session permission", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		req := CreatePermissionRequest{
			SessionID:  "race-persist",
			ToolCallID: "rp-call",
			ToolName:   "tool",
			Action:     "act",
			Path:       "/tmp/rp",
		}

		var (
			wg         sync.WaitGroup
			granted    bool
			requestErr error
		)
		wg.Go(func() {
			granted, requestErr = service.Request(t.Context(), req)
		})

		// Wait for the request to be published so we have the real
		// pending PermissionRequest to race on.
		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("permission request was never published")
		}

		// Drain the initial neither-granted-nor-denied notification.
		<-notifications

		// Deny wins, then a competing GrantPersistent loses.
		assert.True(t, service.Deny(pending), "Deny should resolve the request")
		assert.False(t, service.GrantPersistent(pending),
			"GrantPersistent after Deny should report already-resolved")

		wg.Wait()
		require.NoError(t, requestErr)
		assert.False(t, granted, "request should observe denial")

		// The losing GrantPersistent must not have inserted an
		// auto-approve entry. Issue a matching follow-up request and
		// confirm the service still publishes a pending request (i.e.
		// not auto-approved). We then Deny it to drain the goroutine.
		var (
			wg2         sync.WaitGroup
			granted2    bool
			requestErr2 error
		)
		wg2.Go(func() {
			granted2, requestErr2 = service.Request(t.Context(), req)
		})

		select {
		case ev := <-events:
			assert.Equal(t, pending.SessionID, ev.Payload.SessionID)
			service.Deny(ev.Payload)
		case <-time.After(2 * time.Second):
			t.Fatal("follow-up request was auto-approved; persistent grant leaked")
		}

		wg2.Wait()
		require.NoError(t, requestErr2)
		assert.False(t, granted2, "follow-up request should be denied, not auto-approved")
	})

	t.Run("grant for unknown id is a safe no-op", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		notifications := service.SubscribeNotifications(t.Context())

		bogus := PermissionRequest{
			ID:         "does-not-exist",
			ToolCallID: "ghost",
			ToolName:   "tool",
			Action:     "act",
			Path:       "/tmp/ghost",
		}

		assert.NotPanics(t, func() {
			assert.False(t, service.Grant(bogus),
				"Grant for unknown ID should report already-resolved")
			assert.False(t, service.GrantPersistent(bogus),
				"GrantPersistent for unknown ID should report already-resolved")
			assert.False(t, service.Deny(bogus),
				"Deny for unknown ID should report already-resolved")
		})

		select {
		case ev := <-notifications:
			t.Fatalf("unknown-ID resolution must not publish: %+v", ev.Payload)
		case <-time.After(50 * time.Millisecond):
			// good: no notification.
		}
	})
}

// =============================================================================
// Phase 2 Tests: Generic Contextual Permissions
// =============================================================================

func TestPermissionService_ContextualAutoApprove(t *testing.T) {
	t.Parallel()

	t.Run("context-less request falls back to legacy key (no regression)", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		// First request: no contexts (legacy tool)
		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		req := CreatePermissionRequest{
			SessionID:   "session-legacy",
			ToolCallID:  "legacy-call",
			ToolName:    "edit",
			Action:      "create",
			Description: "Edit a file",
			Path:        "/tmp/test.txt",
		}

		var wg sync.WaitGroup
		var granted bool
		var err error
		wg.Go(func() {
			granted, err = service.Request(t.Context(), req)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		<-notifications
		assert.True(t, service.GrantPersistent(pending))

		wg.Wait()
		require.NoError(t, err)
		assert.True(t, granted, "first legacy request should be granted")

		// Second identical request should auto-approve via legacy key
		req2 := CreatePermissionRequest{
			SessionID:   "session-legacy",
			ToolCallID:  "legacy-call-2",
			ToolName:    "edit",
			Action:      "create",
			Description: "Edit same file again",
			Path:        "/tmp/test.txt",
		}

		result2, err2 := service.Request(t.Context(), req2)
		require.NoError(t, err2)
		assert.True(t, result2, "second legacy request with same key should auto-approve")
	})

	t.Run("all contexts granted → auto-approve", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		// Grant a request with contexts
		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		req := CreatePermissionRequest{
			SessionID:   "session-cmd",
			ToolCallID:  "cmd-call",
			ToolName:    "bash",
			Action:      "execute",
			Description: "Run command chain",
			Path:        "/tmp",
			Contexts:    []string{"command:cd", "command:ls", "command:pwd", "path:/tmp"},
		}

		var wg sync.WaitGroup
		var granted bool
		var reqErr error
		wg.Go(func() {
			granted, reqErr = service.Request(t.Context(), req)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		<-notifications
		assert.True(t, service.GrantPersistent(pending))

		wg.Wait()
		require.NoError(t, reqErr)
		assert.True(t, granted)

		// Verify each context was recorded as a separate key.
		// Contextual grants intentionally omit Path — location semantics
		// are captured by path: context tokens instead.
		for _, ctx := range req.Contexts {
			key := PermissionKey{
				SessionID: pending.SessionID,
				ToolName:  pending.ToolName,
				Action:    pending.Action,
				Context:   ctx,
			}
			allowed, ok := service.(*permissionService).sessionPermissions.Get(key)
			assert.True(t, ok, "context %q should be in sessionPermissions", ctx)
			assert.True(t, allowed, "context %q should be approved", ctx)
		}

		// Follow-up with same contexts should auto-approve
		req2 := CreatePermissionRequest{
			SessionID:   "session-cmd",
			ToolCallID:  "cmd-call-2",
			ToolName:    "bash",
			Action:      "execute",
			Description: "Same command chain",
			Path:        "/tmp",
			Contexts:    []string{"command:cd", "command:ls", "command:pwd", "path:/tmp"},
		}

		result2, err2 := service.Request(t.Context(), req2)
		require.NoError(t, err2)
		assert.True(t, result2, "follow-up with all contexts should auto-approve")
	})

	t.Run("any context missing → prompt", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		// Grant a request with 3 contexts
		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		req := CreatePermissionRequest{
			SessionID:   "session-partial",
			ToolCallID:  "partial-call",
			ToolName:    "bash",
			Action:      "execute",
			Description: "Partial command chain",
			Path:        "/tmp",
			Contexts:    []string{"command:cd", "command:ls", "path:/tmp"},
		}

		var wg sync.WaitGroup
		var granted bool
		var reqErr error
		wg.Go(func() {
			granted, reqErr = service.Request(t.Context(), req)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		<-notifications
		assert.True(t, service.GrantPersistent(pending))

		wg.Wait()
		require.NoError(t, reqErr)
		assert.True(t, granted)

		// Follow-up with missing "command:ls" → should NOT auto-approve
		req2 := CreatePermissionRequest{
			SessionID:   "session-partial",
			ToolCallID:  "partial-call-2",
			ToolName:    "bash",
			Action:      "execute",
			Description: "Missing one context",
			Path:        "/tmp",
			Contexts:    []string{"command:cd", "command:pwd", "path:/tmp"},
		}

		// Note: "command:pwd" was NOT granted — it was not in the original
		events2 := service.Subscribe(t.Context())
		notifications2 := service.SubscribeNotifications(t.Context())

		var wg2 sync.WaitGroup
		var granted2 bool
		var reqErr2 error
		wg2.Go(func() {
			granted2, reqErr2 = service.Request(t.Context(), req2)
		})

		// Should still publish a request (not auto-approve)
		select {
		case ev := <-events2:
			assert.Equal(t, pending.SessionID, ev.Payload.SessionID)
			// Drain the request
			<-notifications2
			service.Deny(ev.Payload)
		case <-time.After(2 * time.Second):
			t.Fatal("request with missing context should have been published")
		}

		wg2.Wait()
		require.NoError(t, reqErr2)
		assert.False(t, granted2, "request with missing context should NOT auto-approve")
	})
}

func TestPermissionService_ContextualRecordAndReuse(t *testing.T) {
	t.Parallel()

	t.Run("GrantPersistent records every context", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		contexts := []string{"command:go", "command:build", "path:/repo", "path:/repo/cmd"}

		req := CreatePermissionRequest{
			SessionID:   "session-record",
			ToolCallID:  "record-call",
			ToolName:    "bash",
			Action:      "execute",
			Description: "Go build",
			Path:        "/repo",
			Contexts:    contexts,
		}

		var wg sync.WaitGroup
		var granted bool
		var reqErr error
		wg.Go(func() {
			granted, reqErr = service.Request(t.Context(), req)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		<-notifications
		assert.True(t, service.GrantPersistent(pending))

		wg.Wait()
		require.NoError(t, reqErr)
		assert.True(t, granted)

		// Verify all contexts are individually recorded.
		// Contextual grants intentionally omit Path — location semantics
		// are captured by path: context tokens instead.
		for _, ctx := range contexts {
			key := PermissionKey{
				SessionID: pending.SessionID,
				ToolName:  pending.ToolName,
				Action:    pending.Action,
				Context:   ctx,
			}
			val, ok := service.(*permissionService).sessionPermissions.Get(key)
			assert.True(t, ok, "context %q should be recorded", ctx)
			assert.True(t, val, "context %q should be approved", ctx)
		}
	})

	t.Run("subsequent isolated requests auto-approve", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		// Grant a chain request
		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		chainReq := CreatePermissionRequest{
			SessionID:   "session-isolated",
			ToolCallID:  "chain-call",
			ToolName:    "bash",
			Action:      "execute",
			Description: "Chain",
			Path:        "/tmp",
			Contexts:    []string{"command:cd", "command:ls", "path:/tmp"},
		}

		var wg sync.WaitGroup
		var granted bool
		var reqErr error
		wg.Go(func() {
			granted, reqErr = service.Request(t.Context(), chainReq)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		<-notifications
		assert.True(t, service.GrantPersistent(pending))

		wg.Wait()
		require.NoError(t, reqErr)
		assert.True(t, granted)

		// Now each individual context should auto-approve
		testCases := []CreatePermissionRequest{
			{
				SessionID:   "session-isolated",
				ToolCallID:  "isolated-call-1",
				ToolName:    "bash",
				Action:      "execute",
				Description: "cd only",
				Path:        "/tmp",
				Contexts:    []string{"command:cd"},
			},
			{
				SessionID:   "session-isolated",
				ToolCallID:  "isolated-call-2",
				ToolName:    "bash",
				Action:      "execute",
				Description: "ls only",
				Path:        "/tmp",
				Contexts:    []string{"command:ls"},
			},
			{
				SessionID:   "session-isolated",
				ToolCallID:  "isolated-call-3",
				ToolName:    "bash",
				Action:      "execute",
				Description: "path only",
				Path:        "/tmp",
				Contexts:    []string{"path:/tmp"},
			},
		}

		for i, tc := range testCases {
			t.Run(tc.Description, func(t *testing.T) {
				result, err := service.Request(t.Context(), tc)
				require.NoError(t, err)
				assert.True(t, result, "isolated context %d should auto-approve", i+1)
			})
		}
	})

	t.Run("recombined requests auto-approve", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		// Grant a chain with 3 contexts
		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		chainReq := CreatePermissionRequest{
			SessionID:   "session-recombine",
			ToolCallID:  "chain-recombine",
			ToolName:    "bash",
			Action:      "execute",
			Description: "Chain",
			Path:        "/tmp",
			Contexts:    []string{"command:cd", "command:ls", "command:pwd"},
		}

		var wg sync.WaitGroup
		var granted bool
		var reqErr error
		wg.Go(func() {
			granted, reqErr = service.Request(t.Context(), chainReq)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		<-notifications
		assert.True(t, service.GrantPersistent(pending))

		wg.Wait()
		require.NoError(t, reqErr)
		assert.True(t, granted)

		// Now recombine a subset — should auto-approve
		recombinedReq := CreatePermissionRequest{
			SessionID:   "session-recombine",
			ToolCallID:  "recombined-call",
			ToolName:    "bash",
			Action:      "execute",
			Description: "Recombined",
			Path:        "/tmp",
			Contexts:    []string{"command:cd", "command:pwd"},
		}

		// Note: "command:ls" is not in this request, so it's fine
		result, err := service.Request(t.Context(), recombinedReq)
		require.NoError(t, err)
		assert.True(t, result, "recombined subset of contexts should auto-approve")
	})
}

func TestPermissionService_ContextualConservativeApproval(t *testing.T) {
	t.Parallel()

	t.Run("different command with same path does not auto-approve", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		req := CreatePermissionRequest{
			SessionID:   "session-conservative",
			ToolCallID:  "conservative-call",
			ToolName:    "bash",
			Action:      "execute",
			Description: "ls /tmp",
			Path:        "/tmp",
			Contexts:    []string{"command:ls", "path:/tmp"},
		}

		var wg sync.WaitGroup
		var granted bool
		var reqErr error
		wg.Go(func() {
			granted, reqErr = service.Request(t.Context(), req)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		<-notifications
		assert.True(t, service.GrantPersistent(pending))

		wg.Wait()
		require.NoError(t, reqErr)
		assert.True(t, granted)

		// Different command: "command:cd" was NOT granted
		req2 := CreatePermissionRequest{
			SessionID:   "session-conservative",
			ToolCallID:  "conservative-call-2",
			ToolName:    "bash",
			Action:      "execute",
			Description: "cd /tmp (should NOT auto-approve)",
			Path:        "/tmp",
			Contexts:    []string{"command:cd", "path:/tmp"},
		}

		events2 := service.Subscribe(t.Context())
		notifications2 := service.SubscribeNotifications(t.Context())

		var wg2 sync.WaitGroup
		var granted2 bool
		var reqErr2 error
		wg2.Go(func() {
			granted2, reqErr2 = service.Request(t.Context(), req2)
		})

		select {
		case ev := <-events2:
			// Should have published (not auto-approved)
			assert.Equal(t, pending.SessionID, ev.Payload.SessionID)
			<-notifications2
			service.Deny(ev.Payload)
		case <-time.After(2 * time.Second):
			t.Fatal("different command should NOT auto-approve")
		}

		wg2.Wait()
		require.NoError(t, reqErr2)
		assert.False(t, granted2, "different command with same path should NOT auto-approve")
	})

	t.Run("same command with different path does not auto-approve", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		req := CreatePermissionRequest{
			SessionID:   "session-path",
			ToolCallID:  "path-call",
			ToolName:    "bash",
			Action:      "execute",
			Description: "ls /tmp",
			Path:        "/tmp",
			Contexts:    []string{"command:ls", "path:/tmp"},
		}

		var wg sync.WaitGroup
		var granted bool
		var reqErr error
		wg.Go(func() {
			granted, reqErr = service.Request(t.Context(), req)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		<-notifications
		assert.True(t, service.GrantPersistent(pending))

		wg.Wait()
		require.NoError(t, reqErr)
		assert.True(t, granted)

		// Different path: "path:/other" was NOT granted
		req2 := CreatePermissionRequest{
			SessionID:   "session-path",
			ToolCallID:  "path-call-2",
			ToolName:    "bash",
			Action:      "execute",
			Description: "ls /other (should NOT auto-approve)",
			Path:        "/other",
			Contexts:    []string{"command:ls", "path:/other"},
		}

		events2 := service.Subscribe(t.Context())
		_ = service.SubscribeNotifications(t.Context())

		var wg2 sync.WaitGroup
		var granted2 bool
		var reqErr2 error
		wg2.Go(func() {
			granted2, reqErr2 = service.Request(t.Context(), req2)
		})

		select {
		case ev := <-events2:
			// Should have published (not auto-approved)
			service.Deny(ev.Payload)
		case <-time.After(2 * time.Second):
			t.Fatal("same command with different path should NOT auto-approve")
		}

		wg2.Wait()
		require.NoError(t, reqErr2)
		assert.False(t, granted2, "same command with different path should NOT auto-approve")
	})
}

func TestPermissionService_ContextualRace(t *testing.T) {
	t.Parallel()

	t.Run("concurrent context grants resolve exactly once", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		req := CreatePermissionRequest{
			SessionID:   "session-race-ctx",
			ToolCallID:  "race-ctx-call",
			ToolName:    "bash",
			Action:      "execute",
			Description: "Race test",
			Path:        "/tmp",
			Contexts:    []string{"command:cd", "command:ls"},
		}

		var wg sync.WaitGroup
		var granted bool
		var reqErr error
		wg.Go(func() {
			granted, reqErr = service.Request(t.Context(), req)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		// Drain initial neither-granted-nor-denied notification
		<-notifications

		// Concurrent GrantPersistent calls — only the first should resolve
		var resolveWg sync.WaitGroup
		resolveCount := atomic.Int32{}
		for i := 0; i < 3; i++ {
			resolveWg.Add(1)
			go func() {
				defer resolveWg.Done()
				if service.GrantPersistent(pending) {
					resolveCount.Add(1)
				}
			}()
		}
		resolveWg.Wait()

		assert.Equal(t, int32(1), resolveCount.Load(), "exactly one grant should resolve")

		wg.Wait()
		require.NoError(t, reqErr)
		assert.True(t, granted)

		// Only one notification (from first grant)
		select {
		case ev := <-notifications:
			assert.True(t, ev.Payload.Granted, "first notification should be granted")
		case <-time.After(2 * time.Second):
			t.Fatal("no notification received")
		}

		// Second notification should be timeout (no more)
		select {
		case ev := <-notifications:
			t.Fatalf("extra notification: %+v", ev.Payload)
		case <-time.After(50 * time.Millisecond):
			// good: no extra notification
		}
	})

	t.Run("losing GrantPersistent with contexts does not leak", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		req := CreatePermissionRequest{
			SessionID:   "session-lose-ctx",
			ToolCallID:  "lose-ctx-call",
			ToolName:    "bash",
			Action:      "execute",
			Description: "Lose test",
			Path:        "/tmp",
			Contexts:    []string{"command:cd", "command:ls"},
		}

		var wg sync.WaitGroup
		var granted bool
		var reqErr error
		wg.Go(func() {
			granted, reqErr = service.Request(t.Context(), req)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		// Drain initial neither-granted-nor-denied notification
		<-notifications

		// Deny wins, then a competing GrantPersistent loses
		assert.True(t, service.Deny(pending), "Deny should resolve")
		assert.False(t, service.GrantPersistent(pending),
			"GrantPersistent after Deny should report already-resolved")

		wg.Wait()
		require.NoError(t, reqErr)
		assert.False(t, granted)

		// Follow-up request should NOT auto-approve (losing grant leaked nothing)
		var wg2 sync.WaitGroup
		var granted2 bool
		var reqErr2 error
		wg2.Go(func() {
			granted2, reqErr2 = service.Request(t.Context(), req)
		})

		select {
		case ev := <-events:
			// Should publish (not auto-approved)
			assert.Equal(t, pending.SessionID, ev.Payload.SessionID)
			<-notifications
			service.Deny(ev.Payload)
		case <-time.After(2 * time.Second):
			t.Fatal("follow-up should be published; losing GrantPersistent leaked an approval")
		}

		wg2.Wait()
		require.NoError(t, reqErr2)
		assert.False(t, granted2)
	})
}

func TestPermissionService_ContextEmptyFallback(t *testing.T) {
	t.Parallel()

	t.Run("empty contexts uses legacy key", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		req := CreatePermissionRequest{
			SessionID:   "session-empty-ctx",
			ToolCallID:  "empty-ctx-call",
			ToolName:    "view",
			Action:      "read",
			Description: "View file",
			Path:        "/tmp/file.txt",
			Contexts:    nil, // explicitly empty
		}

		var wg sync.WaitGroup
		var granted bool
		var reqErr error
		wg.Go(func() {
			granted, reqErr = service.Request(t.Context(), req)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		<-notifications
		assert.True(t, service.GrantPersistent(pending))

		wg.Wait()
		require.NoError(t, reqErr)
		assert.True(t, granted)

		// Verify legacy key was recorded (Context field should be empty)
		legacyKey := PermissionKey{
			SessionID: pending.SessionID,
			ToolName:  pending.ToolName,
			Action:    pending.Action,
			Path:      pending.Path,
			// Context is empty — legacy style
		}
		val, ok := service.(*permissionService).sessionPermissions.Get(legacyKey)
		assert.True(t, ok, "legacy key should be recorded")
		assert.True(t, val, "legacy key should be approved")

		// Follow-up with same legacy key should auto-approve
		req2 := CreatePermissionRequest{
			SessionID:   "session-empty-ctx",
			ToolCallID:  "empty-ctx-call-2",
			ToolName:    "view",
			Action:      "read",
			Description: "View same file again",
			Path:        "/tmp/file.txt",
			Contexts:    nil,
		}

		result2, err2 := service.Request(t.Context(), req2)
		require.NoError(t, err2)
		assert.True(t, result2, "follow-up legacy request should auto-approve")
	})

	t.Run("nil contexts treated same as empty", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		events := service.Subscribe(t.Context())

		req := CreatePermissionRequest{
			SessionID:   "session-nil-ctx",
			ToolCallID:  "nil-ctx-call",
			ToolName:    "write",
			Action:      "create",
			Description: "Write file",
			Path:        "/tmp/write.txt",
			Contexts:    nil,
		}

		var wg sync.WaitGroup
		var granted bool
		var reqErr error
		wg.Go(func() {
			granted, reqErr = service.Request(t.Context(), req)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		assert.True(t, service.GrantPersistent(pending))

		wg.Wait()
		require.NoError(t, reqErr)
		assert.True(t, granted)

		// Follow-up should auto-approve via legacy key
		req2 := CreatePermissionRequest{
			SessionID:   "session-nil-ctx",
			ToolCallID:  "nil-ctx-call-2",
			ToolName:    "write",
			Action:      "create",
			Description: "Write same file",
			Path:        "/tmp/write.txt",
			Contexts:    nil,
		}

		result2, err2 := service.Request(t.Context(), req2)
		require.NoError(t, err2)
		assert.True(t, result2)
	})
}

func TestPermissionService_PathPrefixMatching(t *testing.T) {
	t.Parallel()

	t.Run("approved parent path auto-approves child file", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		// First request: user approves cat within /tmp.
		req := CreatePermissionRequest{
			SessionID:   "session-prefix",
			ToolCallID:  "prefix-call",
			ToolName:    "bash",
			Action:      "execute",
			Description: "cat /tmp/file.txt",
			Path:        "/tmp",
			Contexts:    []string{"command:cat", "path:/tmp"},
		}

		var wg sync.WaitGroup
		var granted bool
		var reqErr error
		wg.Go(func() {
			granted, reqErr = service.Request(t.Context(), req)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		<-notifications
		assert.True(t, service.GrantPersistent(pending))

		wg.Wait()
		require.NoError(t, reqErr)
		assert.True(t, granted)

		// Second request: same command to a child path under /tmp should auto-approve.
		req2 := CreatePermissionRequest{
			SessionID:   "session-prefix",
			ToolCallID:  "prefix-call-2",
			ToolName:    "bash",
			Action:      "execute",
			Description: "cat /tmp/subdir/file.txt",
			Path:        "/tmp",
			Contexts:    []string{"command:cat", "path:/tmp/subdir/file.txt"},
		}

		result2, err2 := service.Request(t.Context(), req2)
		require.NoError(t, err2)
		assert.True(t, result2, "child path of approved /tmp should auto-approve")
	})

	t.Run("approved parent path auto-approves nested child path", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		// First request: user approves mkdir within /tmp/subpath.
		req := CreatePermissionRequest{
			SessionID:   "session-nested",
			ToolCallID:  "nested-call",
			ToolName:    "bash",
			Action:      "execute",
			Description: "mkdir -p /tmp/subpath",
			Path:        "/tmp",
			Contexts:    []string{"command:mkdir", "path:/tmp/subpath"},
		}

		var wg sync.WaitGroup
		var granted bool
		var reqErr error
		wg.Go(func() {
			granted, reqErr = service.Request(t.Context(), req)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		<-notifications
		assert.True(t, service.GrantPersistent(pending))

		wg.Wait()
		require.NoError(t, reqErr)
		assert.True(t, granted)

		// Second request: same command to a deeply nested child path should auto-approve.
		req2 := CreatePermissionRequest{
			SessionID:   "session-nested",
			ToolCallID:  "nested-call-2",
			ToolName:    "bash",
			Action:      "execute",
			Description: "mkdir -p /tmp/subpath/deep/dir",
			Path:        "/tmp",
			Contexts:    []string{"command:mkdir", "path:/tmp/subpath/deep/dir"},
		}

		result2, err2 := service.Request(t.Context(), req2)
		require.NoError(t, err2)
		assert.True(t, result2, "deeply nested child path of approved /tmp/subpath should auto-approve")
	})

	t.Run("approved exact path does not auto-approve sibling path", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		// First request: user approves exact path /tmp/file.txt only.
		req := CreatePermissionRequest{
			SessionID:   "session-exact",
			ToolCallID:  "exact-call",
			ToolName:    "bash",
			Action:      "execute",
			Description: "cat /tmp/file.txt",
			Path:        "/tmp",
			Contexts:    []string{"command:cat", "path:/tmp/file.txt"},
		}

		var wg sync.WaitGroup
		var granted bool
		var reqErr error
		wg.Go(func() {
			granted, reqErr = service.Request(t.Context(), req)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		<-notifications
		assert.True(t, service.GrantPersistent(pending))

		wg.Wait()
		require.NoError(t, reqErr)
		assert.True(t, granted)

		// Second request: writing to a sibling path should NOT auto-approve
		req2 := CreatePermissionRequest{
			SessionID:   "session-exact",
			ToolCallID:  "exact-call-2",
			ToolName:    "bash",
			Action:      "execute",
			Description: "cat /tmp/other.txt",
			Path:        "/tmp",
			Contexts:    []string{"command:cat", "path:/tmp/other.txt"},
		}

		events2 := service.Subscribe(t.Context())
		notifications2 := service.SubscribeNotifications(t.Context())

		var wg2 sync.WaitGroup
		var granted2 bool
		var reqErr2 error
		wg2.Go(func() {
			granted2, reqErr2 = service.Request(t.Context(), req2)
		})

		select {
		case ev := <-events2:
			// Should have published (not auto-approved)
			assert.Equal(t, pending.SessionID, ev.Payload.SessionID)
			<-notifications2
			service.Deny(ev.Payload)
		case <-time.After(2 * time.Second):
			t.Fatal("sibling path should NOT auto-approve")
		}

		wg2.Wait()
		require.NoError(t, reqErr2)
		assert.False(t, granted2, "sibling path of /tmp/file.txt should NOT auto-approve")
	})

	t.Run("approved / prefix-approves all paths", func(t *testing.T) {
		t.Parallel()
		service := NewPermissionService("/tmp", false, nil, nil)

		events := service.Subscribe(t.Context())
		notifications := service.SubscribeNotifications(t.Context())

		// First request: user approves ls with path:/ (root prefix — covers all paths).
		req := CreatePermissionRequest{
			SessionID:   "session-root",
			ToolCallID:  "root-call",
			ToolName:    "bash",
			Action:      "execute",
			Description: "ls /",
			Path:        "/",
			Contexts:    []string{"command:ls", "path:/"},
		}

		var wg sync.WaitGroup
		var granted bool
		var reqErr error
		wg.Go(func() {
			granted, reqErr = service.Request(t.Context(), req)
		})

		var pending PermissionRequest
		select {
		case ev := <-events:
			pending = ev.Payload
		case <-time.After(2 * time.Second):
			t.Fatal("request was never published")
		}

		<-notifications
		assert.True(t, service.GrantPersistent(pending))

		wg.Wait()
		require.NoError(t, reqErr)
		assert.True(t, granted)

		// Second request: same command, any path under / should auto-approve.
		req2 := CreatePermissionRequest{
			SessionID:   "session-root",
			ToolCallID:  "root-call-2",
			ToolName:    "bash",
			Action:      "execute",
			Description: "ls /etc",
			Path:        "/etc",
			Contexts:    []string{"command:ls", "path:/etc/hosts"},
		}

		result2, err2 := service.Request(t.Context(), req2)
		require.NoError(t, err2)
		assert.True(t, result2, "any path under / should auto-approve after / is approved with same command")
	})
}

// TestPermissionService_AllowedContexts verifies that context tokens supplied
// at construction time (from config allowed_commands) auto-approve matching
// requests without a session grant.
func TestPermissionService_AllowedContexts(t *testing.T) {
	t.Parallel()

	t.Run("exact configured command auto-approves", func(t *testing.T) {
		t.Parallel()
		svc := NewPermissionService("/tmp", false, nil, []string{
			"command:go test",
			"command:git diff",
		})

		result, err := svc.Request(t.Context(), CreatePermissionRequest{
			SessionID:   "s1",
			ToolName:    "bash",
			Action:      "execute",
			Description: "run tests",
			Contexts:    []string{"command:go test"},
		})
		require.NoError(t, err)
		assert.True(t, result, "command:go test should be auto-approved by config")
	})

	t.Run("configured base command satisfies subcommand token", func(t *testing.T) {
		t.Parallel()
		svc := NewPermissionService("/tmp", false, nil, []string{
			"command:go",
		})

		result, err := svc.Request(t.Context(), CreatePermissionRequest{
			SessionID:   "s1",
			ToolName:    "bash",
			Action:      "execute",
			Description: "run tests",
			Contexts:    []string{"command:go test"},
		})
		require.NoError(t, err)
		assert.True(t, result, "configured command:go should satisfy command:go test")
	})

	t.Run("unconfigured command still prompts", func(t *testing.T) {
		t.Parallel()
		svc := NewPermissionService("/tmp", false, nil, []string{
			"command:go test",
		})

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		_, err := svc.Request(ctx, CreatePermissionRequest{
			SessionID:   "s1",
			ToolName:    "bash",
			Action:      "execute",
			Description: "run rm",
			Contexts:    []string{"command:rm"},
		})
		require.Error(t, err, "unconfigured command:rm should not auto-approve")
	})

	t.Run("all contexts must pass — partial config is insufficient", func(t *testing.T) {
		t.Parallel()
		// Only command:go is configured; path:/tmp is not.
		svc := NewPermissionService("/tmp", false, nil, []string{
			"command:go",
		})

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		_, err := svc.Request(ctx, CreatePermissionRequest{
			SessionID:   "s1",
			ToolName:    "bash",
			Action:      "execute",
			Description: "go test with path",
			Contexts:    []string{"command:go test", "path:/tmp"},
		})
		require.Error(t, err, "path:/tmp is not configured so request should not auto-approve")
	})

	t.Run("configured base does not match different command word", func(t *testing.T) {
		t.Parallel()
		svc := NewPermissionService("/tmp", false, nil, []string{
			"command:py",
		})

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		_, err := svc.Request(ctx, CreatePermissionRequest{
			SessionID:   "s1",
			ToolName:    "bash",
			Action:      "execute",
			Description: "python3 script",
			Contexts:    []string{"command:python3"},
		})
		require.Error(t, err, "command:py must not match command:python3")
	})
}
