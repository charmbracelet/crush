package shell

import (
	"charm.land/turtle/shcoreutils"
)

// CoreutilsMode selects how core utilities run in Crush shells.
type CoreutilsMode string

const (
	// CoreutilsTurtle always uses the Turtle implementations.
	CoreutilsTurtle CoreutilsMode = "turtle"
	// CoreutilsSystem prefers the system utilities, falling back to Turtle
	// when a command is not found in PATH.
	CoreutilsSystem CoreutilsMode = "system"
	// CoreutilsAuto uses the system utilities where they exist and Turtle
	// on Windows.
	CoreutilsAuto CoreutilsMode = "auto"
	// CoreutilsOff skips the coreutils middleware entirely.
	CoreutilsOff CoreutilsMode = "off"
)

// coreutilsHandler builds the Turtle coreutils middleware for mode, or nil
// when mode is off. An empty or unknown mode means no workspace config is
// available (crushrc execution, standalone helpers) and runs Turtle.
func coreutilsHandler(mode CoreutilsMode) execMiddleware {
	switch mode {
	case CoreutilsOff:
		return nil
	case CoreutilsSystem:
		return shcoreutils.New(shcoreutils.WithPriority(shcoreutils.PrioritySystem))
	case CoreutilsAuto:
		return shcoreutils.New(shcoreutils.WithPriority(shcoreutils.PriorityAuto))
	default:
		return shcoreutils.New(shcoreutils.WithPriority(shcoreutils.PriorityTurtle))
	}
}
