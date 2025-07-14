// Package shell provides cross-platform shell execution capabilities.
//
// This package offers two main types:
// - Shell: A general-purpose shell executor for one-off or managed commands
// - PersistentShell: A singleton shell that maintains state across the application
//
// WINDOWS COMPATIBILITY:
// This implementation provides both POSIX shell emulation (mvdan.cc/sh/v3) and
// native Windows shell support (cmd.exe/PowerShell) for optimal compatibility.
package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// ShellType represents the type of shell to use
type ShellType int

const (
	ShellTypePOSIX ShellType = iota
	ShellTypeCmd
	ShellTypePowerShell
)

// Logger interface for optional logging
type Logger interface {
	InfoPersist(msg string, keysAndValues ...interface{})
}

// noopLogger is a logger that does nothing
type noopLogger struct{}

func (noopLogger) InfoPersist(msg string, keysAndValues ...interface{}) {}

// BlockFunc is a function that determines if a command should be blocked
type BlockFunc func(args []string) bool

// Shell provides cross-platform shell execution with optional state persistence
type Shell struct {
	env        []string
	cwd        string
	mu         sync.Mutex
	logger     Logger
	blockFuncs []BlockFunc
}

// Options for creating a new shell
type Options struct {
	WorkingDir string
	Env        []string
	Logger     Logger
	BlockFuncs []BlockFunc
}

// NewShell creates a new shell instance with the given options
func NewShell(opts *Options) *Shell {
	if opts == nil {
		opts = &Options{}
	}

	cwd := opts.WorkingDir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	env := opts.Env
	if env == nil {
		env = os.Environ()
	}

	logger := opts.Logger
	if logger == nil {
		logger = noopLogger{}
	}

	return &Shell{
		cwd:        cwd,
		env:        env,
		logger:     logger,
		blockFuncs: opts.BlockFuncs,
	}
}

// Exec executes a command in the shell
func (s *Shell) Exec(ctx context.Context, command string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Determine which shell to use based on platform and command
	shellType := s.determineShellType(command)

	switch shellType {
	case ShellTypeCmd:
		return s.execWindows(ctx, command, "cmd")
	case ShellTypePowerShell:
		return s.execWindows(ctx, command, "powershell")
	default:
		return s.execPOSIX(ctx, command)
	}
}

// GetWorkingDir returns the current working directory
func (s *Shell) GetWorkingDir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cwd
}

// SetWorkingDir sets the working directory
func (s *Shell) SetWorkingDir(dir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify the directory exists
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("directory does not exist: %w", err)
	}

	s.cwd = dir
	return nil
}

// GetEnv returns a copy of the environment variables
func (s *Shell) GetEnv() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	env := make([]string, len(s.env))
	copy(env, s.env)
	return env
}

// SetEnv sets an environment variable
func (s *Shell) SetEnv(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update or add the environment variable
	keyPrefix := key + "="
	for i, env := range s.env {
		if strings.HasPrefix(env, keyPrefix) {
			s.env[i] = keyPrefix + value
			return
		}
	}
	s.env = append(s.env, keyPrefix+value)
}

// SetBlockFuncs sets the command block functions for the shell
func (s *Shell) SetBlockFuncs(blockFuncs []BlockFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blockFuncs = blockFuncs
}

// Windows-specific commands that should use native shell
var windowsNativeCommands = map[string]bool{
	"dir":      true,
	"type":     true,
	"copy":     true,
	"move":     true,
	"del":      true,
	"md":       true,
	"mkdir":    true,
	"rd":       true,
	"rmdir":    true,
	"cls":      true,
	"where":    true,
	"tasklist": true,
	"taskkill": true,
	"net":      true,
	"sc":       true,
	"reg":      true,
	"wmic":     true,
}

// determineShellType decides which shell to use based on platform and command
func (s *Shell) determineShellType(command string) ShellType {
	if runtime.GOOS != "windows" {
		return ShellTypePOSIX
	}

	// Extract the first command from the command line
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ShellTypePOSIX
	}

	firstCmd := strings.ToLower(parts[0])

	// Check if it's a Windows-specific command
	if windowsNativeCommands[firstCmd] {
		return ShellTypeCmd
	}

	// Check for PowerShell-specific syntax
	if strings.Contains(command, "Get-") || strings.Contains(command, "Set-") ||
		strings.Contains(command, "New-") || strings.Contains(command, "$_") ||
		strings.Contains(command, "| Where-Object") || strings.Contains(command, "| ForEach-Object") {
		return ShellTypePowerShell
	}

	// Default to POSIX emulation for cross-platform compatibility
	return ShellTypePOSIX
}

// CommandsBlocker creates a BlockFunc that blocks exact command matches
func CommandsBlocker(bannedCommands []string) BlockFunc {
	bannedSet := make(map[string]bool)
	for _, cmd := range bannedCommands {
		bannedSet[cmd] = true
	}

	return func(args []string) bool {
		if len(args) == 0 {
			return false
		}
		return bannedSet[args[0]]
	}
}

// ArgumentsBlocker creates a BlockFunc that blocks specific subcommands
func ArgumentsBlocker(blockedSubCommands [][]string) BlockFunc {
	return func(args []string) bool {
		for _, blocked := range blockedSubCommands {
			if len(args) >= len(blocked) {
				match := true
				for i, part := range blocked {
					if args[i] != part {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
		}
		return false
	}
}

func (s *Shell) blockHandler() func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return next(ctx, args)
			}

			for _, blockFunc := range s.blockFuncs {
				if blockFunc(args) {
					return fmt.Errorf("command is not allowed for security reasons: %s", strings.Join(args, " "))
				}
			}

			return next(ctx, args)
		}
	}
}

// execWindows executes commands using native Windows shells (cmd.exe or PowerShell)
func (s *Shell) execWindows(ctx context.Context, command string, shell string) (string, string, error) {
	var cmd *exec.Cmd

	// Handle directory changes specially to maintain persistent shell behavior
	if strings.HasPrefix(strings.TrimSpace(command), "cd ") {
		return s.handleWindowsCD(command)
	}

	switch shell {
	case "cmd":
		// Use cmd.exe for Windows commands
		// Add current directory context to maintain state
		fullCommand := fmt.Sprintf("cd /d \"%s\" && %s", s.cwd, command)
		cmd = exec.CommandContext(ctx, "cmd", "/C", fullCommand)
	case "powershell":
		// Use PowerShell for PowerShell commands
		// Add current directory context to maintain state
		fullCommand := fmt.Sprintf("Set-Location '%s'; %s", s.cwd, command)
		cmd = exec.CommandContext(ctx, "powershell", "-Command", fullCommand)
	default:
		return "", "", fmt.Errorf("unsupported Windows shell: %s", shell)
	}

	// Set environment variables
	cmd.Env = s.env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	s.logger.InfoPersist("Windows command finished", "shell", shell, "command", command, "err", err)
	return stdout.String(), stderr.String(), err
}

// handleWindowsCD handles directory changes for Windows shells
func (s *Shell) handleWindowsCD(command string) (string, string, error) {
	// Extract the target directory from the cd command
	parts := strings.Fields(command)
	if len(parts) < 2 {
		return "", "cd: missing directory argument", fmt.Errorf("missing directory argument")
	}

	targetDir := parts[1]

	// Handle relative paths
	if !strings.Contains(targetDir, ":") && !strings.HasPrefix(targetDir, "\\") {
		// Relative path - resolve against current directory
		if targetDir == ".." {
			// Go up one directory
			if len(s.cwd) > 3 { // Don't go above drive root (C:\)
				lastSlash := strings.LastIndex(s.cwd, "\\")
				if lastSlash > 2 { // Keep drive letter
					s.cwd = s.cwd[:lastSlash]
				}
			}
		} else if targetDir != "." {
			// Go to subdirectory
			s.cwd = s.cwd + "\\" + targetDir
		}
	} else {
		// Absolute path
		s.cwd = targetDir
	}

	// Verify the directory exists
	if _, err := os.Stat(s.cwd); err != nil {
		return "", fmt.Sprintf("cd: %s: No such file or directory", targetDir), err
	}

	return "", "", nil
}

// execPOSIX executes commands using POSIX shell emulation (cross-platform)
func (s *Shell) execPOSIX(ctx context.Context, command string) (string, string, error) {
	line, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return "", "", fmt.Errorf("could not parse command: %w", err)
	}

	var stdout, stderr bytes.Buffer
	runner, err := interp.New(
		interp.StdIO(nil, &stdout, &stderr),
		interp.Interactive(false),
		interp.Env(expand.ListEnviron(s.env...)),
		interp.Dir(s.cwd),
		interp.ExecHandlers(s.blockHandler()),
	)
	if err != nil {
		return "", "", fmt.Errorf("could not run command: %w", err)
	}

	err = runner.Run(ctx, line)
	s.cwd = runner.Dir
	s.env = []string{}
	for name, vr := range runner.Vars {
		s.env = append(s.env, fmt.Sprintf("%s=%s", name, vr.Str))
	}
	s.logger.InfoPersist("POSIX command finished", "command", command, "err", err)
	return stdout.String(), stderr.String(), err
}

// IsInterrupt checks if an error is due to interruption
func IsInterrupt(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

// ExitCode extracts the exit code from an error
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	status, ok := interp.IsExitStatus(err)
	if ok {
		return int(status)
	}
	return 1
}
