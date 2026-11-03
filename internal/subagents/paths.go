package subagents

import (
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fsext"
)

// InGlobalDir reports whether path lies inside one of the global (user-scope)
// subagents directories. Anything else — project directories, monorepo roots,
// custom subagents_paths — can arrive with a cloned repository, so callers use
// this to gate trust-sensitive operations: deleting a definition file and
// honoring bypassPermissions without a prompt.
func InGlobalDir(path string) bool {
	if path == "" {
		return false
	}
	for _, dir := range config.GlobalSubagentsDirs() {
		if fsext.HasPrefix(path, dir) {
			return true
		}
	}
	return false
}
