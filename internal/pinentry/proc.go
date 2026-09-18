package pinentry

import (
	"context"
	"fmt"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
)

// terminalPinentryNames are pinentry program basenames that render
// directly on the terminal and therefore need a terminal handover. GUI
// flavors (pinentry-mac, pinentry-qt, pinentry-gnome3, pinentry-gtk, ...)
// are deliberately excluded: they do not touch the terminal. The plain
// "pinentry" dispatcher is included because it is commonly a curses build
// (e.g. Homebrew) or a distribution alternative symlink.
var terminalPinentryNames = map[string]struct{}{
	"pinentry":        {},
	"pinentry-curses": {},
	"pinentry-tty":    {},
}

// gpgNames are the basenames of gpg binaries, watched after a passphrase
// dialog closes to detect a security key touch wait.
var gpgNames = map[string]struct{}{
	"gpg":  {},
	"gpg2": {},
}

// Proc is a process table snapshot entry.
type Proc struct {
	// PID is the process ID.
	PID int
	// Name is the basename of the process executable.
	Name string
	// Path is the full path of the process executable when the process
	// table provides one (e.g. ps comm on macOS), empty otherwise.
	Path string
}

// Lister returns a snapshot of the process table.
type Lister func(ctx context.Context) ([]Proc, error)

// psLister lists processes via ps(1). On Windows it returns nothing:
// pinentry flavors there are GUI programs that never need the terminal.
func psLister(ctx context.Context) ([]Proc, error) {
	if runtime.GOOS == "windows" {
		return nil, nil
	}
	out, err := exec.CommandContext(ctx, "ps", "-eo", "pid=,comm=").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list processes: %w", err)
	}
	return parsePS(out), nil
}

// parsePS parses the output of `ps -eo pid=,comm=`. The comm column is a
// full path on some platforms (macOS) and a bare name on others (Linux),
// and it may contain spaces, so everything after the pid field is treated
// as the command and reduced to its basename.
func parsePS(out []byte) []Proc {
	var procs []Proc
	for line := range strings.Lines(string(out)) {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		comm := strings.Join(fields[1:], " ")
		p := Proc{PID: pid, Name: path.Base(comm)}
		if strings.Contains(comm, "/") {
			p.Path = comm
		}
		procs = append(procs, p)
	}
	return procs
}

// classify inspects a process table snapshot for pinentry and gpg
// processes. handover reports that a terminal pinentry dialog is present
// and needs exclusive control of the terminal. ctrlL reports that the
// dialog is an ncurses flavor that redraws on Ctrl-L. gpg reports that a
// gpg process is running.
func classify(procs []Proc) (handover, ctrlL, gpg bool) {
	for i := range procs {
		p := procs[i]
		if _, ok := gpgNames[p.Name]; ok {
			gpg = true
		}
		if _, ok := terminalPinentryNames[p.Name]; !ok {
			continue
		}
		switch resolveFlavor(p) {
		case flavorCurses:
			handover, ctrlL = true, true
		case flavorTTY:
			handover = true
		case flavorGUI:
			// GUI pinentry flavors draw their own window and never
			// touch the terminal.
		}
	}
	return handover, ctrlL, gpg
}
