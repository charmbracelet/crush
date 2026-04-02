package config

import "fmt"

// Scope determines which config file is targeted for read/write operations.
type Scope int

const (
	// ScopeGlobal targets the global data config (~/.local/share/crush/crush.json).
	ScopeGlobal Scope = iota
	// ScopeWorkspace targets the workspace config (.crush/crush.json).
	ScopeWorkspace
)

// String returns a human-readable label for the scope.
func (s Scope) String() string {
	switch s {
	case ScopeGlobal:
		return "global"
	case ScopeWorkspace:
		return "workspace"
	default:
		return fmt.Sprintf("Scope(%d)", int(s))
	}
}

// ErrNoWorkspaceConfig is returned when a workspace-scoped write is
// attempted on a ConfigStore that has no workspace config path.
var ErrNoWorkspaceConfig = fmt.Errorf("no workspace config path configured")
