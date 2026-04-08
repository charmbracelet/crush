package permission

import (
	"testing"
)

func TestGetGrade(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		action   string
		expected PermissionLevel
	}{
		{"read tool", "view", "read", LevelRead},
		{"grep tool", "grep", "search", LevelRead},
		{"write tool", "write", "create", LevelWrite},
		{"edit tool", "edit", "modify", LevelWrite},
		{"delete action", "edit", "delete", LevelAdmin},
		{"bash tool", "bash", "execute", LevelAdmin},
		{"sudo tool", "sudo", "execute", LevelAdmin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grade := GetGrade(tt.tool, tt.action)
			if grade.Level != tt.expected {
				t.Errorf("GetGrade(%s, %s) = %v, want %v", tt.tool, tt.action, grade.Level, tt.expected)
			}
		})
	}
}

func TestPermissionGrade_RequiresApproval(t *testing.T) {
	tests := []struct {
		name     string
		level    PermissionLevel
		expected bool
	}{
		{"LevelRead", LevelRead, false},
		{"LevelWrite", LevelWrite, true},
		{"LevelAdmin", LevelAdmin, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grade := PermissionGrade{Level: tt.level}
			if grade.RequiresApproval() != tt.expected {
				t.Errorf("RequiresApproval() = %v, want %v", grade.RequiresApproval(), tt.expected)
			}
		})
	}
}

func TestPermissionGrade_IsHighRisk(t *testing.T) {
	tests := []struct {
		name     string
		level    PermissionLevel
		expected bool
	}{
		{"LevelRead", LevelRead, false},
		{"LevelWrite", LevelWrite, false},
		{"LevelAdmin", LevelAdmin, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grade := PermissionGrade{Level: tt.level}
			if grade.IsHighRisk() != tt.expected {
				t.Errorf("IsHighRisk() = %v, want %v", grade.IsHighRisk(), tt.expected)
			}
		})
	}
}
