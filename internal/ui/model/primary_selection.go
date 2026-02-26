package model

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// readPrimarySelection reads text from the X11 PRIMARY selection (used for
// middle-click paste on Linux). Returns empty string if not available.
func readPrimarySelection() string {
	if runtime.GOOS != "linux" {
		return ""
	}

	// Try wl-paste for Wayland first.
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if path, err := exec.LookPath("wl-paste"); err == nil {
			cmd := exec.Command(path, "--no-newline", "--primary")
			out, err := cmd.Output()
			if err == nil {
				return string(out)
			}
		}
	}

	// Try xclip for X11.
	if path, err := exec.LookPath("xclip"); err == nil {
		cmd := exec.Command(path, "-selection", "primary", "-o")
		out, err := cmd.Output()
		if err == nil {
			return strings.TrimSuffix(string(out), "\n")
		}
	}

	// Fall back to xsel.
	if path, err := exec.LookPath("xsel"); err == nil {
		cmd := exec.Command(path, "--primary", "--output")
		out, err := cmd.Output()
		if err == nil {
			return strings.TrimSuffix(string(out), "\n")
		}
	}

	return ""
}
