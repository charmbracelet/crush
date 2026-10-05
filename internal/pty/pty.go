// Package pty provides a cross-platform pseudo-terminal abstraction.
// On Windows it uses ConPTY via direct CreateProcessW syscall.
// On Unix it uses POSIX openpty via creack/pty (planned).
package pty

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

// PTY represents a pseudo-terminal connected to a child process.
type PTY struct {
	mu     sync.Mutex
	cmd    *exec.Cmd      // used on Unix
	stdin  io.WriteCloser
	stdout io.ReadCloser
	closed bool

	// Platform-specific handle (ConPTY HPCON on Windows, *os.File on Unix).
	handle interface{}

	// Windows: direct process handle from CreateProcessW (stored as uintptr for cross-platform compilation).
	procHandle uintptr

	// Cached exit code from Wait.
	exitCode int
	exited   bool

	// Size tracking.
	cols, rows int
}

// Options configures PTY creation.
type Options struct {
	// Command to run. Defaults to the user's shell.
	Command string
	// Args for the command.
	Args []string
	// Dir is the working directory.
	Dir string
	// Env is additional environment variables.
	Env []string
	// Initial terminal size.
	Cols int
	Rows int
}

// DefaultShell returns the default shell for the current platform.
func DefaultShell() string {
	if runtime.GOOS == "windows" {
		if ps, err := exec.LookPath("pwsh.exe"); err == nil {
			return ps
		}
		return "powershell.exe"
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

// New creates a new PTY with the given options. The child process is started
// immediately. Call Close() to terminate.
func New(opts Options) (*PTY, error) {
	if opts.Command == "" {
		opts.Command = DefaultShell()
	}
	if opts.Cols <= 0 {
		opts.Cols = 80
	}
	if opts.Rows <= 0 {
		opts.Rows = 24
	}

	return newPlatformPTY(opts)
}

// Read reads from the PTY output (child process stdout+stderr).
func (p *PTY) Read(buf []byte) (int, error) {
	return p.stdout.Read(buf)
}

// Write writes to the PTY input (child process stdin).
func (p *PTY) Write(data []byte) (int, error) {
	return p.stdin.Write(data)
}

// WriteString writes a string to the PTY input.
func (p *PTY) WriteString(s string) (int, error) {
	return p.Write([]byte(s))
}

// Resize changes the PTY dimensions.
func (p *PTY) Resize(cols, rows int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return fmt.Errorf("pty is closed")
	}
	p.cols = cols
	p.rows = rows
	return p.resizePlatform(cols, rows)
}

// Size returns the current PTY dimensions.
func (p *PTY) Size() (cols, rows int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cols, p.rows
}

// Close terminates the child process and releases PTY resources.
func (p *PTY) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true

	if p.stdin != nil {
		p.stdin.Close()
	}

	// Kill via exec.Cmd (Unix) or direct handle (Windows).
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		_ = p.cmd.Wait()
	}

	return p.closePlatform()
}

// Wait waits for the child process to exit and returns its exit code.
func (p *PTY) Wait() (int, error) {
	p.mu.Lock()
	if p.exited {
		code := p.exitCode
		p.mu.Unlock()
		return code, nil
	}
	p.mu.Unlock()

	if p.cmd != nil {
		// Unix path: use exec.Cmd.Wait.
		err := p.cmd.Wait()
		p.mu.Lock()
		p.exited = true
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				p.exitCode = exitErr.ExitCode()
			} else {
				p.exitCode = -1
			}
		}
		code := p.exitCode
		p.mu.Unlock()
		return code, err
	}

	// Windows path: wait on process handle.
	if p.procHandle != 0 {
		code, err := p.waitPlatform()
		if err != nil {
			return -1, err
		}
		p.mu.Lock()
		p.exited = true
		p.exitCode = code
		p.mu.Unlock()
		return code, nil
	}

	return -1, fmt.Errorf("no process to wait on")
}

// IsRunning returns true if the child process is still running.
func (p *PTY) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.exited && !p.closed
}
