package shell

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

// JobShell provides asynchronous shell execution for long-running jobs
type JobShell struct {
	workingDir string
	stdout     io.Writer
	stderr     io.Writer
	cmd        *exec.Cmd
	done       chan error
	mu         sync.Mutex
}

// NewJobShell creates a new JobShell instance
func NewJobShell(workingDir string, stdout, stderr io.Writer) (*JobShell, error) {
	if workingDir == "" {
		var err error
		workingDir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	return &JobShell{
		workingDir: workingDir,
		stdout:     stdout,
		stderr:     stderr,
		done:       make(chan error, 1),
	}, nil
}

// ExecAsync starts a command asynchronously
func (js *JobShell) ExecAsync(ctx context.Context, command string) error {
	js.mu.Lock()
	defer js.mu.Unlock()

	if js.cmd != nil {
		return errors.New("job already running")
	}

	// Determine shell command based on platform
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	cmd.Dir = js.workingDir
	cmd.Stdout = js.stdout
	cmd.Stderr = js.stderr
	cmd.Env = os.Environ()

	js.cmd = cmd

	// Start the command
	if err := cmd.Start(); err != nil {
		js.cmd = nil
		return err
	}

	// Wait for completion in a goroutine
	go func() {
		err := cmd.Wait()
		js.done <- err
		close(js.done)
	}()

	return nil
}

// Done returns a channel that will receive the exit error when the job completes
func (js *JobShell) Done() <-chan error {
	return js.done
}

// Kill terminates the running job
func (js *JobShell) Kill() error {
	js.mu.Lock()
	defer js.mu.Unlock()

	if js.cmd == nil || js.cmd.Process == nil {
		return nil
	}

	return js.cmd.Process.Kill()
}
