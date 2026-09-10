package pinentry

import (
	"fmt"
	"path"
	"path/filepath"
	"runtime"
)

// flavor describes a pinentry program's terminal behavior.
type flavor uint8

const (
	// flavorGUI is a pinentry that does not use the terminal (GUI flavors
	// like pinentry-mac or pinentry-qt): no terminal handover needed.
	flavorGUI flavor = iota
	// flavorCurses is an ncurses-based pinentry: it takes over the
	// terminal and redraws its dialog when it receives Ctrl-L.
	flavorCurses
	// flavorTTY is a line-based pinentry: it takes over the terminal, but
	// Ctrl-L must not be injected because it reads input as plain bytes
	// and would treat the byte as passphrase content.
	flavorTTY
)

// guiPinentryNames are resolved basenames of pinentry flavors that render
// their own window and never touch the terminal.
var guiPinentryNames = map[string]struct{}{
	"pinentry-mac":    {},
	"pinentry-qt":     {},
	"pinentry-qt5":    {},
	"pinentry-qt6":    {},
	"pinentry-gnome3": {},
	"pinentry-gtk":    {},
	"pinentry-gtk2":   {},
	"pinentry-fltk":   {},
	"pinentry-efl":    {},
	"pinentry-w32":    {},
}

// resolveFlavor determines the terminal behavior of a detected pinentry
// process. The plain "pinentry" dispatcher is usually a symlink (Linux
// alternatives) or a curses binary (Homebrew), so its symlink chain is
// resolved to find the real implementation.
func resolveFlavor(p Proc) flavor {
	switch p.Name {
	case "pinentry-curses":
		return flavorCurses
	case "pinentry-tty":
		return flavorTTY
	}
	if _, ok := guiPinentryNames[p.Name]; ok {
		return flavorGUI
	}

	if target := resolveExe(p); target != "" {
		switch path.Base(target) {
		case "pinentry-curses":
			return flavorCurses
		case "pinentry-tty":
			return flavorTTY
		}
		if _, ok := guiPinentryNames[path.Base(target)]; ok {
			return flavorGUI
		}
	}

	// A plain "pinentry" that does not resolve to a known flavor is
	// commonly the curses build itself (e.g. Homebrew).
	return flavorCurses
}

// resolveExe resolves the executable path of a process, following
// symlinks. It uses the full command path when the process table provides
// one (e.g. ps on macOS) and falls back to /proc on Linux.
func resolveExe(p Proc) string {
	target := p.Path
	if target == "" && runtime.GOOS == "linux" {
		target = fmt.Sprintf("/proc/%d/exe", p.PID)
	}
	if target == "" {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return ""
	}
	return resolved
}
