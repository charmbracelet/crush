package term

import (
	"os"
	"strings"
)

// SupportsProgressBar tries to determine whether the current terminal supports
// progress bars by looking into environment variables.
func SupportsProgressBar() bool {
	termProg := os.Getenv("TERM_PROGRAM")
	_, isWindowsTerminal := os.LookupEnv("WT_SESSION")

	return isWindowsTerminal || strings.Contains(strings.ToLower(termProg), "ghostty")
}
