// Package shell provides cross-platform shell execution capabilities.
//
// This package provides Shell instances for executing commands with their own
// working directory and environment. Each shell execution is independent.
//
// WINDOWS COMPATIBILITY:
// This implementation provides POSIX shell emulation (mvdan.cc/sh/v3) even on
// Windows. Commands should use forward slashes (/) as path separators to work
// correctly on all platforms.
package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/x/exp/slice"
	"mvdan.cc/sh/moreinterp/coreutils"
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
	InfoPersist(msg string, keysAndValues ...any)
}

// noopLogger is a logger that does nothing
type noopLogger struct{}

func (noopLogger) InfoPersist(msg string, keysAndValues ...any) {}

// RuntimeEnvInput contains dynamic environment hook inputs.
type RuntimeEnvInput struct {
	CWD string
}

// RuntimeEnvHook can inject environment variables at command execution time.
type RuntimeEnvHook func(ctx context.Context, input RuntimeEnvInput) map[string]string

var (
	runtimeEnvHook   RuntimeEnvHook
	runtimeEnvHookMu sync.RWMutex
)

// SetRuntimeEnvHook sets the global runtime environment hook.
func SetRuntimeEnvHook(hook RuntimeEnvHook) {
	runtimeEnvHookMu.Lock()
	defer runtimeEnvHookMu.Unlock()
	runtimeEnvHook = hook
}

func getRuntimeEnvHook() RuntimeEnvHook {
	runtimeEnvHookMu.RLock()
	defer runtimeEnvHookMu.RUnlock()
	return runtimeEnvHook
}

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

	env = append(
		env,
		"CRUSH=1",
		"AGENT=crush",
		"AI_AGENT=crush",
	)

	// Extend PATH with native shell tools (Git Bash, WSL) on Windows
	// This allows the interpreter to find Unix tools like grep, sed, awk, etc.
	if runtime.GOOS == "windows" {
		env = ExtendEnvWithNativeTools(env)
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

	return s.exec(ctx, command)
}

// ExecStream executes a command in the shell with streaming output to provided writers
func (s *Shell) ExecStream(ctx context.Context, command string, stdout, stderr io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.execStream(ctx, command, stdout, stderr)
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

// CommandsBlocker creates a BlockFunc that blocks exact command matches
func CommandsBlocker(cmds []string) BlockFunc {
	bannedSet := make(map[string]struct{})
	for _, cmd := range cmds {
		bannedSet[cmd] = struct{}{}
	}

	return func(args []string) bool {
		if len(args) == 0 {
			return false
		}
		_, ok := bannedSet[args[0]]
		return ok
	}
}

// ArgumentsBlocker creates a BlockFunc that blocks specific subcommand
func ArgumentsBlocker(cmd string, args []string, flags []string) BlockFunc {
	return func(parts []string) bool {
		if len(parts) == 0 || parts[0] != cmd {
			return false
		}

		argParts, flagParts := splitArgsFlags(parts[1:])
		if len(argParts) < len(args) || len(flagParts) < len(flags) {
			return false
		}

		argsMatch := slices.Equal(argParts[:len(args)], args)
		flagsMatch := slice.IsSubset(flags, flagParts)

		return argsMatch && flagsMatch
	}
}

func splitArgsFlags(parts []string) (args []string, flags []string) {
	args = make([]string, 0, len(parts))
	flags = make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.HasPrefix(part, "-") {
			// Extract flag name before '=' if present
			flag := part
			if before, _, ok := strings.Cut(part, "="); ok {
				flag = before
			}
			flags = append(flags, flag)
		} else {
			args = append(args, part)
		}
	}
	return args, flags
}

func (s *Shell) blockHandler() func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return next(ctx, args)
			}

			for _, blockFunc := range s.blockFuncs {
				if blockFunc(args) {
					return fmt.Errorf("command is not allowed for security reasons: %q", args[0])
				}
			}

			return next(ctx, args)
		}
	}
}

// newInterp creates a new interpreter with the current shell state
func (s *Shell) newInterp(ctx context.Context, stdout, stderr io.Writer) (*interp.Runner, map[string]runtimeEnvOverride, error) {
	env := slices.Clone(s.env)
	overrides := make(map[string]runtimeEnvOverride)
	if hook := getRuntimeEnvHook(); hook != nil {
		for key, value := range hook(ctx, RuntimeEnvInput{CWD: s.cwd}) {
			previousValue, hadOriginal := envValue(env, key)
			overrides[key] = runtimeEnvOverride{
				InjectedValue: value,
				HadOriginal:   hadOriginal,
				OriginalValue: previousValue,
			}
			setEnvValue(&env, key, value)
		}
	}

	runner, err := interp.New(
		interp.StdIO(nil, stdout, stderr),
		interp.Interactive(false),
		interp.Env(expand.ListEnviron(env...)),
		interp.Dir(s.cwd),
		interp.ExecHandlers(s.execHandlers()...),
	)
	if err != nil {
		return nil, nil, err
	}
	return runner, overrides, nil
}

func envValue(env []string, key string) (string, bool) {
	keyPrefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(item, keyPrefix) {
			return strings.TrimPrefix(item, keyPrefix), true
		}
	}
	return "", false
}

func setEnvValue(env *[]string, key, value string) {
	keyPrefix := key + "="
	for i, item := range *env {
		if strings.HasPrefix(item, keyPrefix) {
			(*env)[i] = keyPrefix + value
			return
		}
	}
	*env = append(*env, keyPrefix+value)
}

func removeEnvValue(env *[]string, key string) {
	keyPrefix := key + "="
	for i, item := range *env {
		if strings.HasPrefix(item, keyPrefix) {
			*env = append((*env)[:i], (*env)[i+1:]...)
			return
		}
	}
}

type runtimeEnvOverride struct {
	InjectedValue string
	HadOriginal   bool
	OriginalValue string
}

// updateShellFromRunner updates the shell from the interpreter after execution.
func (s *Shell) updateShellFromRunner(runner *interp.Runner, overrides map[string]runtimeEnvOverride) {
	s.cwd = runner.Dir
	s.env = s.env[:0]
	for name, vr := range runner.Vars {
		if vr.Exported {
			s.env = append(s.env, name+"="+vr.Str)
		}
	}
	for key, override := range overrides {
		currentValue, exists := envValue(s.env, key)
		if !exists || currentValue != override.InjectedValue {
			continue
		}
		if override.HadOriginal {
			setEnvValue(&s.env, key, override.OriginalValue)
			continue
		}
		removeEnvValue(&s.env, key)
	}
}

// execCommon is the shared implementation for executing commands
func (s *Shell) execCommon(ctx context.Context, command string, stdout, stderr io.Writer) (err error) {
	var runner *interp.Runner
	overrides := make(map[string]runtimeEnvOverride)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("command execution panic: %v", r)
		}
		if runner != nil {
			s.updateShellFromRunner(runner, overrides)
		}
		s.logger.InfoPersist("command finished", "command", command, "err", err)
	}()

	line, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return fmt.Errorf("could not parse command: %w", err)
	}

	runner, overrides, err = s.newInterp(ctx, stdout, stderr)
	if err != nil {
		return fmt.Errorf("could not run command: %w", err)
	}

	err = runner.Run(ctx, line)
	return err
}

// exec executes commands using a cross-platform shell interpreter.
func (s *Shell) exec(ctx context.Context, command string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	err := s.execCommon(ctx, command, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

// execStream executes commands using POSIX shell emulation with streaming output
func (s *Shell) execStream(ctx context.Context, command string, stdout, stderr io.Writer) error {
	return s.execCommon(ctx, command, stdout, stderr)
}

func (s *Shell) execHandlers() []func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	handlers := []func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc{
		s.blockHandler(),
	}
	if useGoCoreUtils {
		handlers = append(handlers, coreutils.ExecHandler)
	}
	return handlers
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
	var exitErr interp.ExitStatus
	if errors.As(err, &exitErr) {
		return int(exitErr)
	}
	return 1
}
