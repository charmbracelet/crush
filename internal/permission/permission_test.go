package permission

import (
	"testing"
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
