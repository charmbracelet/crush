// Package interactive routes bash-tool commands that need the user's
// terminal to the TUI process. Commands that prompt for input (SSH key
// passphrases, GPG signing via pinentry) or draw their own UI
// (pinentry-curses, editors) cannot run through the captured,
// terminal-less execution path. When a command is run interactively the
// TUI pauses, the command takes over the terminal, and the TUI resumes
// once the command exits.
//
// The TUI registers a [Handler] at startup via [SetHandler]. Tool
// goroutines call [Run], which blocks until the command finishes. When
// no TUI is attached (`crush run`, client/server mode, sub-agents on a
// headless server) Run fails with [ErrNoHandler].
package interactive

import (
	"context"
	"errors"
	"sync"
)

// Request describes a command that must run attached to the user's
// terminal.
type Request struct {
	// Command is the shell source to execute.
	Command string `json:"command"`
	// WorkingDir is the directory to execute the command in.
	WorkingDir string `json:"working_dir"`
	// Description is a short summary of what the command does.
	Description string `json:"description"`
}

// Result is the outcome of an interactive command run.
type Result struct {
	// Output is a cleaned, truncated copy of what the command printed
	// to the terminal. Full-screen TUI output may be noisy or empty;
	// the user saw the real output live.
	Output string `json:"output"`
	// ExitCode is the command's exit status.
	ExitCode int `json:"exit_code"`
}

// Handler executes a request with the user's terminal handed over. It
// must block until the command finishes and then return its outcome.
// The context is the caller's (tool) context; handlers should return
// promptly when it is cancelled.
type Handler func(ctx context.Context, req Request) (Result, error)

// ErrNoHandler is returned by [Run] when no TUI is attached to the
// process, so interactive commands cannot reach the user's terminal.
var ErrNoHandler = errors.New("interactive commands are unavailable: no terminal UI is attached to run them")

var (
	handlerMu sync.RWMutex
	handler   Handler

	// runMu serializes interactive runs: the terminal can only be
	// handed to one command at a time.
	runMu sync.Mutex
)

// SetHandler registers the handler used by [Run]. The TUI calls this at
// startup and clears it with a nil handler on exit. Passing nil
// unregisters the current handler.
func SetHandler(h Handler) {
	handlerMu.Lock()
	defer handlerMu.Unlock()
	handler = h
}

// CurrentHandler returns the registered handler, or nil when none is.
func CurrentHandler() Handler {
	handlerMu.RLock()
	defer handlerMu.RUnlock()
	return handler
}

// Run executes req with the user's terminal handed over. It blocks
// until the command completes (the user interacts with it directly in
// the meantime) and returns the command's outcome. Interactive runs are
// serialized; a second Run waits for the first to finish.
func Run(ctx context.Context, req Request) (Result, error) {
	h := CurrentHandler()
	if h == nil {
		return Result{}, ErrNoHandler
	}

	// Wait for our turn while staying responsive to cancellation.
	if err := acquireLock(ctx, &runMu); err != nil {
		return Result{}, err
	}
	defer runMu.Unlock()

	return h(ctx, req)
}

// acquireLock locks mu, aborting early if ctx is cancelled first.
func acquireLock(ctx context.Context, mu *sync.Mutex) error {
	// Fast path: uncontended.
	if mu.TryLock() {
		return nil
	}

	locked := make(chan struct{})
	go func() {
		mu.Lock()
		close(locked)
	}()
	select {
	case <-locked:
		return nil
	case <-ctx.Done():
		// The goroutine still holds a pending lock; hand ownership back
		// by unlocking in its place once it completes.
		go func() {
			<-locked
			mu.Unlock()
		}()
		return ctx.Err()
	}
}
