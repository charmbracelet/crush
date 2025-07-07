//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || aix || zos
// +build darwin dragonfly freebsd linux netbsd openbsd solaris aix zos

package uv

import (
	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/unix"
)

func (t *Terminal) makeRaw() error {
	var err error

	if t.inTty == nil && t.outTty == nil {
		return ErrNotTerminal
	}

	// Check if we have a terminal.
	for _, f := range []term.File{t.inTty, t.outTty} {
		if f == nil {
			continue
		}
		t.inTtyState, err = term.MakeRaw(f.Fd())
		if err == nil {
			break
		}
	}

	if err != nil {
		return err
	}

	return nil
}

func (t *Terminal) getSize() (w, h int, err error) {
	// Try both inTty and outTty to get the size.
	err = ErrNotTerminal
	for _, f := range []term.File{t.inTty, t.outTty} {
		if f == nil {
			continue
		}
		w, h, err = term.GetSize(f.Fd())
		if err == nil {
			return
		}
	}
	return
}

func (t *Terminal) optimizeMovements() {
	// Try both inTty and outTty to get the size.
	var state *term.State
	var err error
	for _, f := range []term.File{t.inTty, t.outTty} {
		if f == nil {
			continue
		}
		state, err = term.GetState(f.Fd())
		if err == nil {
			break
		}
	}
	if state == nil {
		return
	}
	t.useTabs = state.Oflag&unix.TABDLY == unix.TAB0
	t.useBspace = state.Lflag&unix.BSDLY == unix.BS0
}

func (*Terminal) enableWindowsMouse() error  { return nil }
func (*Terminal) disableWindowsMouse() error { return nil }
