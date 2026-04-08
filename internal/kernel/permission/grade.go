package permission

// PermissionLevel represents the 3-tier permission grading system
type PermissionLevel int

const (
	// LevelRead: L1 - Read-only operations (view, search, inspect)
	LevelRead PermissionLevel = iota + 1

	// LevelWrite: L2 - Write operations that don't modify system state
	LevelWrite

	// LevelAdmin: L3 - High-risk operations (delete, system config, network access)
	LevelAdmin
)

// PermissionGrade represents a permission grade with metadata
type PermissionGrade struct {
	Level       PermissionLevel
	Name        string
	Description string
	RiskLevel   string
}

// GetGrade returns the permission grade for a given tool and action
func GetGrade(toolName, action string) PermissionGrade {
	grade := classifyPermission(toolName, action)
	return grade
}

// classifyPermission classifies a tool/action combination into a permission grade
func classifyPermission(toolName, action string) PermissionGrade {
	toolLower := toLowerCase(toolName)
	actionLower := toLowerCase(action)

	// L1 Read - safe operations that only read data
	if isReadTool(toolLower) {
		return PermissionGrade{
			Level:       LevelRead,
			Name:        "Read",
			Description: "Read-only access to view files, search content, inspect system state",
			RiskLevel:   "low",
		}
	}

	// L3 Admin - high-risk destructive operations
	if isDestructiveAction(actionLower) || isSystemLevelTool(toolLower) {
		return PermissionGrade{
			Level:       LevelAdmin,
			Name:        "Admin",
			Description: "High-risk operations including deletions, system config, network access",
			RiskLevel:   "high",
		}
	}

	// L2 Write - operations that modify files but are generally safe
	if isWriteTool(toolLower) {
		return PermissionGrade{
			Level:       LevelWrite,
			Name:        "Write",
			Description: "File modification operations (create, edit, write)",
			RiskLevel:   "medium",
		}
	}

	// Default to L2 Write for unknown tools
	return PermissionGrade{
		Level:       LevelWrite,
		Name:        "Write",
		Description: "Standard file operations (assumed safe)",
		RiskLevel:   "medium",
	}
}

// isReadTool returns true if the tool is a read-only tool
func isReadTool(tool string) bool {
	readTools := map[string]bool{
		"view":        true,
		"read":        true,
		"cat":         true,
		"grep":        true,
		"find":        true,
		"search":      true,
		"ls":          true,
		"dir":         true,
		"glob":        true,
		"stat":        true,
		"file":        true,
		"head":        true,
		"tail":        true,
		"wc":          true,
		"diff":        true,
		"inspect":     true,
		"fetch":       true,
		"url":         true,
		"webfetch":    true,
		"http":        true,
	}
	return readTools[tool]
}

// isWriteTool returns true if the tool modifies files
func isWriteTool(tool string) bool {
	writeTools := map[string]bool{
		"write":    true,
		"edit":     true,
		"create":   true,
		"save":     true,
		"mkdir":    true,
		"touch":    true,
		"append":   true,
		"patch":    true,
		"replace":  true,
	}
	return writeTools[tool]
}

// isDestructiveAction returns true if the action is destructive
func isDestructiveAction(action string) bool {
	destructiveActions := map[string]bool{
		"delete":    true,
		"remove":    true,
		"rm":        true,
		"destroy":   true,
		"drop":      true,
		"truncate":  true,
		"format":    true,
		"erase":     true,
		"kill":      true,
		"terminate": true,
		"stop":      true,
		"shutdown":  true,
		"reboot":    true,
		"halt":      true,
	}
	return destructiveActions[action]
}

// isSystemLevelTool returns true if the tool accesses system-level functions
func isSystemLevelTool(tool string) bool {
	systemTools := map[string]bool{
		"bash":     true,
		"shell":    true,
		"exec":     true,
		"run":      true,
		"sudo":     true,
		"chmod":    true,
		"chown":    true,
		"system":   true,
		"config":   true,
		"network":  true,
		"ssh":      true,
		"scp":      true,
		"curl":     true,
		"wget":     true,
		"request":  true,
		"http":     true,
		"postgres": true,
		"mysql":    true,
		"redis":    true,
		"mongo":    true,
		"docker":   true,
		"kubectl":  true,
		"git":      true,
	}
	return systemTools[tool]
}

// toLowerCase converts a string to lowercase
func toLowerCase(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// RequiresApproval returns true if the permission grade requires user approval
func (pg PermissionGrade) RequiresApproval() bool {
	return pg.Level >= LevelWrite
}

// IsHighRisk returns true if the permission grade is high risk
func (pg PermissionGrade) IsHighRisk() bool {
	return pg.Level == LevelAdmin
}
