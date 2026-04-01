package permission

import (
	"encoding/json"
	"sync"
	"testing"

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
			service := NewPermissionService("/tmp", false, tt.allowedTools)

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
	service := NewPermissionService("/tmp", true, []string{})

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

func TestPermissionService_SequentialProperties(t *testing.T) {
	t.Run("Persistent grants are stored and can be cleared", func(t *testing.T) {
		service := NewPermissionService("/tmp", false, []string{})

		granted := PermissionRequest{
			SessionID: "session1",
			ToolName:  "file_tool",
			Action:    "read",
			Path:      "/tmp/test.txt",
		}
		requested := PermissionRequest{
			SessionID: "session1",
			ToolName:  "file_tool",
			Action:    "read",
			Path:      "/tmp/test.txt",
		}

		service.GrantPersistent(granted)
		assert.True(t, service.HasPersistentPermission(requested))

		service.ClearPersistentPermissions("session1")
		assert.False(t, service.HasPersistentPermission(requested))
	})
	t.Run("Sequential requests with temporary grants", func(t *testing.T) {
		service := NewPermissionService("/tmp", false, []string{})

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
		service := NewPermissionService("/tmp", false, []string{})

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
		assert.True(t, service.HasPersistentPermission(PermissionRequest{
			SessionID: "concurrent2",
			ToolName:  "tool2",
			Action:    "action2",
			Path:      "/tmp/file2.txt",
		}))
	})
}

func TestCreatePermissionRequestJSONRoundTrip_AuthoritySessionID(t *testing.T) {
	t.Parallel()

	input := CreatePermissionRequest{
		SessionID:          "child-session",
		AuthoritySessionID: "parent-session",
		ToolCallID:         "tool-1",
		ToolName:           "write",
		Description:        "write file",
		Action:             "write",
		Params:             map[string]any{"file_path": "a.txt"},
		Path:               ".",
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)
	require.Contains(t, string(data), "authority_session_id")

	var output CreatePermissionRequest
	require.NoError(t, json.Unmarshal(data, &output))
	require.Equal(t, input.AuthoritySessionID, output.AuthoritySessionID)
}

func TestPermissionRequestJSONRoundTrip_AuthoritySessionID(t *testing.T) {
	t.Parallel()

	input := PermissionRequest{
		ID:                 "perm-1",
		SessionID:          "child-session",
		AuthoritySessionID: "parent-session",
		ToolCallID:         "tool-1",
		ToolName:           "write",
		Description:        "write file",
		Action:             "write",
		Params:             map[string]any{"file_path": "a.txt"},
		Path:               ".",
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)
	require.Contains(t, string(data), "authority_session_id")

	var output PermissionRequest
	require.NoError(t, json.Unmarshal(data, &output))
	require.Equal(t, input.AuthoritySessionID, output.AuthoritySessionID)
}

func TestPermissionService_BuildPermissionRequestDefaultsAuthoritySessionID(t *testing.T) {
	t.Parallel()

	svc := NewPermissionService(".", true, nil).(*permissionService)
	built, err := svc.buildPermissionRequest(CreatePermissionRequest{SessionID: "child-session", Path: "."})
	require.NoError(t, err)
	require.Equal(t, "child-session", built.AuthoritySessionID)
}
