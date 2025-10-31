package permission

import (
	"context"
	"sync"
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
			service := NewPermissionService(t.Context(), "/tmp", false, tt.allowedTools, nil)

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

func TestPermissionService_SkipMode(t *testing.T) {
	service := NewPermissionService(t.Context(), "/tmp", true, []string{}, nil)

	result := service.Request(CreatePermissionRequest{
		SessionID:   "test-session",
		ToolName:    "bash",
		Action:      "execute",
		Description: "test command",
		Path:        "/tmp",
	})

	if !result {
		t.Error("expected permission to be granted in skip mode")
	}
}

func TestPermissionService_SequentialProperties(t *testing.T) {
	t.Run("Sequential permission requests with persistent grants", func(t *testing.T) {
		service := NewPermissionService(t.Context(), "/tmp", false, []string{}, nil)

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
			result1 = service.Request(req1)
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
		result2 := service.Request(req2)
		assert.True(t, result2, "Second request should be auto-approved")
	})
	t.Run("Sequential requests with temporary grants", func(t *testing.T) {
		service := NewPermissionService(t.Context(), "/tmp", false, []string{}, nil)

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
			result1 = service.Request(req)
		})

		var permissionReq PermissionRequest
		event := <-events
		permissionReq = event.Payload

		service.Grant(permissionReq)
		wg.Wait()
		assert.True(t, result1, "First request should be granted")

		var result2 bool

		wg.Go(func() {
			result2 = service.Request(req)
		})

		event = <-events
		permissionReq = event.Payload
		service.Deny(permissionReq)
		wg.Wait()
		assert.False(t, result2, "Second request should be denied")
	})
	t.Run("Concurrent requests with different outcomes", func(t *testing.T) {
		service := NewPermissionService(t.Context(), "/tmp", false, []string{}, nil)

		events := service.Subscribe(t.Context())

		var wg sync.WaitGroup
		results := make([]bool, 0)

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
				results = append(results, service.Request(request))
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
		result := service.Request(secondReq)
		assert.True(t, result, "Repeated request should be auto-approved due to persistent permission")
	})
}

func TestPermissionService_NotifierSchedulesAndCancelsOnGrant(t *testing.T) {
	notifier := &testPermissionNotifier{}
	service := NewPermissionService(t.Context(), "/tmp", false, []string{}, notifier)

	events := service.Subscribe(t.Context())
	req := CreatePermissionRequest{
		SessionID:  "session-grant",
		ToolCallID: "tool-call-grant",
		ToolName:   "bash",
		Action:     "execute",
		Path:       "/tmp/script.sh",
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result := service.Request(req)
		require.True(t, result, "Request should be granted")
	}()

	event := <-events
	calls := notifier.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "💘 Crush is waiting", calls[0].title)
	assert.Equal(t, "Permission required to execute \"bash\"", calls[0].message)
	assert.Equal(t, permissionNotificationDelay, calls[0].delay)

	service.Grant(event.Payload)
	wg.Wait()

	calls = notifier.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, 1, calls[0].cancelCount)
}

func TestPermissionService_NotifyInteractionCancelsNotification(t *testing.T) {
	notifier := &testPermissionNotifier{}
	service := NewPermissionService(t.Context(), "/tmp", false, []string{}, notifier)

	events := service.Subscribe(t.Context())
	req := CreatePermissionRequest{
		SessionID:  "session-interact",
		ToolCallID: "tool-call-interact",
		ToolName:   "edit",
		Action:     "write",
		Path:       "/tmp/file.txt",
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result := service.Request(req)
		require.False(t, result, "Request should be denied")
	}()

	event := <-events
	calls := notifier.Calls()
	require.Len(t, calls, 1)

	service.NotifyInteraction(event.Payload.ToolCallID)
	calls = notifier.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, 1, calls[0].cancelCount)

	service.Deny(event.Payload)
	wg.Wait()
}

type notificationCall struct {
	title       string
	message     string
	delay       time.Duration
	cancelCount int
}

type testPermissionNotifier struct {
	mu    sync.Mutex
	calls []*notificationCall
}

func (n *testPermissionNotifier) NotifyPermissionRequest(ctx context.Context, title, message string, delay time.Duration) context.CancelFunc {
	call := &notificationCall{
		title:   title,
		message: message,
		delay:   delay,
	}
	n.mu.Lock()
	n.calls = append(n.calls, call)
	n.mu.Unlock()

	return func() {
		n.mu.Lock()
		call.cancelCount++
		n.mu.Unlock()
	}
}

func (n *testPermissionNotifier) Calls() []notificationCall {
	n.mu.Lock()
	defer n.mu.Unlock()

	result := make([]notificationCall, len(n.calls))
	for i, call := range n.calls {
		result[i] = *call
	}
	return result
}
