package shell

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/logging"
	"github.com/shirou/gopsutil/v4/process"
)

type PersistentShell struct {
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	isAlive      bool
	cwd          string
	mu           sync.Mutex
	commandQueue chan *commandExecution
}

type commandExecution struct {
	command    string
	timeout    time.Duration
	resultChan chan commandResult
	ctx        context.Context
}

type commandResult struct {
	stdout      string
	stderr      string
	exitCode    int
	interrupted bool
	err         error
}

var shellInstance *PersistentShell

func GetPersistentShell(workingDir string) *PersistentShell {
	if shellInstance == nil {
		shellInstance = newPersistentShell(workingDir)
	}
	if !shellInstance.isAlive {
		shellInstance = newPersistentShell(shellInstance.cwd)
	}
	return shellInstance
}

func newPersistentShell(cwd string) *PersistentShell {
	// Get shell configuration from config
	cfg := config.Get()

	// Default to environment variable if config is not set or nil
	var shellPath string
	var shellArgs []string

	if cfg != nil {
		shellPath = cfg.Shell.Path
		shellArgs = cfg.Shell.Args
	}

	shellPath = cmp.Or(shellPath, os.Getenv("SHELL"), "/bin/bash")
	if !strings.HasSuffix(shellPath, "bash") && !strings.HasSuffix(shellPath, "zsh") {
		logging.Warn("only bash and zsh are supported at this time", "shell", shellPath)
		shellPath = "/bin/bash"
	}

	// Default shell args
	if len(shellArgs) == 0 {
		shellArgs = []string{"--login"}
	}

	cmd := exec.Command(shellPath, shellArgs...)
	cmd.Dir = cwd

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil
	}

	cmd.Env = append(os.Environ(), "GIT_EDITOR=true")

	err = cmd.Start()
	if err != nil {
		return nil
	}

	shell := &PersistentShell{
		cmd:          cmd,
		stdin:        stdinPipe,
		isAlive:      true,
		cwd:          cwd,
		commandQueue: make(chan *commandExecution, 10),
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "Panic in shell command processor: %v\n", r)
				shell.isAlive = false
				close(shell.commandQueue)
			}
		}()
		shell.processCommands()
	}()

	go func() {
		err := cmd.Wait()
		if err != nil {
			// Log the error if needed
		}
		shell.isAlive = false
		close(shell.commandQueue)
	}()

	return shell
}

func (s *PersistentShell) processCommands() {
	for cmd := range s.commandQueue {
		cmd.resultChan <- s.execCommand(cmd.ctx, cmd.command, cmd.timeout)
	}
}

const runBashCommandFormat = `%s </dev/null >%q 2>%q
echo $? >%q
pwd >%q`

func (s *PersistentShell) execCommand(ctx context.Context, command string, timeout time.Duration) commandResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isAlive {
		return commandResult{
			stderr:   "Shell is not alive",
			exitCode: 1,
			err:      errors.New("shell is not alive"),
		}
	}

	tmp := os.TempDir()
	now := time.Now().UnixNano()
	stdoutFile := filepath.Join(tmp, fmt.Sprintf("crush-stdout-%d", now))
	stderrFile := filepath.Join(tmp, fmt.Sprintf("crush-stderr-%d", now))
	statusFile := filepath.Join(tmp, fmt.Sprintf("crush-status-%d", now))
	cwdFile := filepath.Join(tmp, fmt.Sprintf("crush-cwd-%d", now))

	defer func() {
		_ = os.Remove(stdoutFile)
		_ = os.Remove(stderrFile)
		_ = os.Remove(statusFile)
		_ = os.Remove(cwdFile)
	}()

	script := fmt.Sprintf(runBashCommandFormat, command, stdoutFile, stderrFile, statusFile, cwdFile)
	if _, err := s.stdin.Write([]byte(script + "\n")); err != nil {
		return commandResult{
			stderr:   fmt.Sprintf("Failed to write command to shell: %v", err),
			exitCode: 1,
			err:      err,
		}
	}

	interrupted := false
	done := make(chan bool)
	go func() {
		// Use exponential backoff polling
		pollInterval := 10 * time.Millisecond
		maxPollInterval := time.Second

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		timeoutTicker := time.NewTicker(cmp.Or(timeout, time.Hour*99999))
		defer timeoutTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.killChildren()
				interrupted = true
				done <- true
				return

			case <-timeoutTicker.C:
				s.killChildren()
				interrupted = true
				done <- true
				return

			case <-ticker.C:
				if fileSize(statusFile) > 0 {
					done <- true
					return
				}

				// Exponential backoff to reduce CPU usage for longer-running commands
				if pollInterval < maxPollInterval {
					pollInterval = min(time.Duration(float64(pollInterval)*1.5), maxPollInterval)
					ticker.Reset(pollInterval)
				}
			}
		}
	}()

	<-done

	stdout := readFileOrEmpty(stdoutFile)
	stderr := readFileOrEmpty(stderrFile)
	exitCodeStr := readFileOrEmpty(statusFile)
	newCwd := readFileOrEmpty(cwdFile)

	exitCode := 0
	if exitCodeStr != "" {
		fmt.Sscanf(exitCodeStr, "%d", &exitCode)
	} else if interrupted {
		exitCode = 143
		stderr += "\nCommand execution timed out or was interrupted"
	}

	if newCwd != "" {
		s.cwd = strings.TrimSpace(newCwd)
	}

	return commandResult{
		stdout:      stdout,
		stderr:      stderr,
		exitCode:    exitCode,
		interrupted: interrupted,
	}
}

func (s *PersistentShell) killChildren() {
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}
	p, err := process.NewProcess(int32(s.cmd.Process.Pid))
	if err != nil {
		logging.WarnPersist("could not kill persistent shell child processes", "err", err)
		return
	}

	children, err := p.Children()
	if err != nil {
		logging.WarnPersist("could not kill persistent shell child processes", "err", err)
		return
	}

	for _, child := range children {
		if err := child.SendSignal(syscall.SIGTERM); err != nil {
			logging.WarnPersist("could not kill persistent shell child processes", "err", err, "pid", child.Pid)
		}
	}
}

func (s *PersistentShell) Exec(ctx context.Context, command string, timeoutMs int) (string, string, int, bool, error) {
	if !s.isAlive {
		return "", "Shell is not alive", 1, false, errors.New("shell is not alive")
	}

	resultChan := make(chan commandResult)
	s.commandQueue <- &commandExecution{
		command:    command,
		timeout:    time.Duration(timeoutMs) * time.Millisecond,
		resultChan: resultChan,
		ctx:        ctx,
	}

	result := <-resultChan
	return result.stdout, result.stderr, result.exitCode, result.interrupted, result.err
}

func (s *PersistentShell) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isAlive {
		return
	}

	s.stdin.Write([]byte("exit\n"))

	if err := s.cmd.Process.Kill(); err != nil {
		logging.WarnPersist("could not kill persistent shell", "err", err)
	}
	s.isAlive = false
}

func readFileOrEmpty(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
