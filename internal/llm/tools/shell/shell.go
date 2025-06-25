// Package shell provides cross-platform shell execution capabilities.
// 
// WINDOWS COMPATIBILITY:
// This implementation provides both POSIX shell emulation (mvdan.cc/sh/v3) and 
// native Windows shell support (cmd.exe/PowerShell) for optimal compatibility:
// - On Windows: Uses native cmd.exe or PowerShell for Windows-specific commands
// - Cross-platform: Falls back to POSIX emulation for Unix-style commands
// - Automatic detection: Chooses the best shell based on command and platform
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

	"github.com/charmbracelet/crush/internal/logging"
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

type PersistentShell struct {
	env []string
	cwd string
	mu  sync.Mutex
}

var (
	once          sync.Once
	shellInstance *PersistentShell
)

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

func GetPersistentShell(cwd string) *PersistentShell {
	once.Do(func() {
		shellInstance = newPersistentShell(cwd)
	})
	return shellInstance
}

func newPersistentShell(cwd string) *PersistentShell {
	return &PersistentShell{
		cwd: cwd,
		env: os.Environ(),
	}
}

func (s *PersistentShell) Exec(ctx context.Context, command string) (string, string, error) {
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

// determineShellType decides which shell to use based on platform and command
func (s *PersistentShell) determineShellType(command string) ShellType {
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

// execWindows executes commands using native Windows shells (cmd.exe or PowerShell)
func (s *PersistentShell) execWindows(ctx context.Context, command string, shell string) (string, string, error) {
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
	
	logging.InfoPersist("Windows command finished", "shell", shell, "command", command, "err", err)
	return stdout.String(), stderr.String(), err
}

// handleWindowsCD handles directory changes for Windows shells
func (s *PersistentShell) handleWindowsCD(command string) (string, string, error) {
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
func (s *PersistentShell) execPOSIX(ctx context.Context, command string) (string, string, error) {
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
	logging.InfoPersist("POSIX command finished", "command", command, "err", err)
	return stdout.String(), stderr.String(), err
}

func IsInterrupt(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

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
