//go:build !windows

package pty

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

func newPlatformPTY(opts Options) (*PTY, error) {
	cmd := exec.Command(opts.Command, opts.Args...)
	cmd.Dir = opts.Dir
	cmd.Env = append(os.Environ(), opts.Env...)

	winSize := &pty.Winsize{
		Rows: uint16(opts.Rows),
		Cols: uint16(opts.Cols),
	}

	ptmx, err := pty.StartWithSize(cmd, winSize)
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}

	return &PTY{
		cmd:    cmd,
		stdin:  ptmx,
		stdout: ptmx,
		handle: ptmx,
		cols:   opts.Cols,
		rows:   opts.Rows,
	}, nil
}

func (p *PTY) waitPlatform() (int, error) {
	return -1, fmt.Errorf("waitPlatform not used on Unix — use cmd.Wait()")
}

func (p *PTY) resizePlatform(cols, rows int) error {
	f, ok := p.handle.(*os.File)
	if !ok {
		return fmt.Errorf("invalid PTY handle")
	}
	return pty.Setsize(f, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

func (p *PTY) closePlatform() error {
	if f, ok := p.handle.(*os.File); ok {
		return f.Close()
	}
	return nil
}
