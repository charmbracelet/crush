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
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/x/exp/slice"
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

// CrushEnvMarkers returns a fresh slice of the environment variables that
// Crush unconditionally sets on every shell it spawns — both the interactive
// bash tool's [Shell] and the hook runner's [Run] calls. Tools that want to
// detect "am I being invoked by an AI agent?" can check any of these.
// Keeping them in one place guarantees the two shell surfaces cannot drift.
// A fresh slice is returned on every call so callers may append freely.
func CrushEnvMarkers() []string {
	return []string{
		"CRUSH=1",
		"AGENT=crush",
		"AI_AGENT=crush",
	}
}

// Logger interface for optional logging
type Logger interface {
	InfoPersist(msg string, keysAndValues ...any)
}

// noopLogger is a logger that does nothing
type noopLogger struct{}

func (noopLogger) InfoPersist(msg string, keysAndValues ...any) {}

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

	// Strip herdr pane-ownership vars so subprocesses (including test
	// binaries and nested crush instances) can't attach to or release
	// the parent pane's agent authority.
	env = withoutHerdrEnv(env)

	// Allow tools to detect execution by Crush.
	env = append(env, CrushEnvMarkers()...)

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

// CommandsBlocker creates a BlockFunc that blocks a command by name,
// regardless of any leading path (so "/bin/curl" is blocked the same as
// "curl").
func CommandsBlocker(cmds []string) BlockFunc {
	bannedSet := make(map[string]struct{}, len(cmds))
	for _, cmd := range cmds {
		bannedSet[cmd] = struct{}{}
	}

	return func(args []string) bool {
		if len(args) == 0 {
			return false
		}
		_, ok := bannedSet[normalizeCommand(args[0])]
		return ok
	}
}

// Rule blocks a single command invocation. It matches when the command name
// (with any leading path stripped) equals Command, the leading positional
// arguments match Args in order, and every flag in Flags is present.
type Rule struct {
	// Command is the command name to match, without a path, e.g. "npm".
	Command string
	// Args are the required leading positional arguments, e.g. ["install"].
	Args []string
	// Flags are flags that must all be present for the rule to match, e.g.
	// ["--global"]. Clustered short flags are matched too, so a rule naming
	// "-S" matches "pacman -Syu".
	Flags []string
}

// Match reports whether the given expanded argument list is blocked by the
// rule.
func (r Rule) Match(args []string) bool {
	if len(args) == 0 || normalizeCommand(args[0]) != r.Command {
		return false
	}

	pos, flags := splitArgsFlags(args[1:])
	if len(pos) < len(r.Args) || !slices.Equal(pos[:len(r.Args)], r.Args) {
		return false
	}
	return slice.IsSubset(r.Flags, flags)
}

// ArgumentsBlocker creates a BlockFunc that blocks a specific subcommand
// invocation. It is a thin adapter over [Rule].
func ArgumentsBlocker(cmd string, args []string, flags []string) BlockFunc {
	return Rule{Command: cmd, Args: args, Flags: flags}.Match
}

// normalizeCommand reduces a command word to a bare command name for matching:
// it strips any directory prefix and a trailing ".exe" so "/usr/bin/rm" and
// "rm.exe" both normalize to "rm".
func normalizeCommand(cmd string) string {
	cmd = filepath.Base(filepath.FromSlash(cmd))
	return strings.TrimSuffix(cmd, ".exe")
}

// splitArgsFlags separates positional arguments from flags. It understands the
// "--" end-of-options marker, "--flag=value" (matched as "--flag"), and
// clustered short flags ("-Syu" also yields "-S", "-y", "-u") so that rules
// naming a single short flag still match it inside a cluster.
func splitArgsFlags(parts []string) (args []string, flags []string) {
	args = make([]string, 0, len(parts))
	flags = make([]string, 0, len(parts))
	endOfFlags := false
	for _, part := range parts {
		if endOfFlags || part == "-" || !strings.HasPrefix(part, "-") {
			args = append(args, part)
			continue
		}
		if part == "--" {
			endOfFlags = true
			continue
		}
		name := part
		if before, _, ok := strings.Cut(part, "="); ok {
			name = before
		}
		flags = append(flags, name)
		// Expand clustered short flags (e.g. "-Syu"). Long ("--") flags and
		// single-character shorts need no expansion.
		if !strings.HasPrefix(name, "--") && len(name) > 2 {
			for _, c := range name[1:] {
				flags = append(flags, "-"+string(c))
			}
		}
	}
	return args, flags
}

// IsCommandBlocked reports whether a command string would likely be blocked
// by the given block functions.
//
// It is a static check used to warn about dangerous commands before they run
// and to gate auto-approval. Each command in the script is expanded to fields
// the way the shell would (quotes are removed, word parts joined, globbing
// disabled), but nothing is executed: a command substitution or any other
// expansion that would require running a command is treated as dangerous
// rather than resolved. Unparseable input is likewise treated as dangerous.
// This fails safe, but it cannot see the results of runtime expansion, so it
// is a conservative approximation of the authoritative blockHandler check.
func IsCommandBlocked(command string, blockFuncs []BlockFunc) bool {
	file, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		// If we can't parse it, consider it potentially dangerous.
		return true
	}

	// Empty environment and nil CmdSubst: variables resolve to empty and
	// command substitutions error out instead of executing.
	cfg := &expand.Config{Env: expand.FuncEnviron(func(string) string { return "" })}

	blocked := false
	syntax.Walk(file, func(node syntax.Node) bool {
		callExpr, ok := node.(*syntax.CallExpr)
		if !ok || len(callExpr.Args) == 0 {
			return true
		}
		args, err := expand.Fields(cfg, callExpr.Args...)
		if err != nil {
			// A substitution or expansion we can't resolve without running
			// something. Be conservative and treat it as dangerous.
			blocked = true
			return false
		}
		for _, blockFunc := range blockFuncs {
			if blockFunc(args) {
				blocked = true
				return false
			}
		}
		return true
	})

	return blocked
}

// newInterp creates a new interpreter with the current shell state. A nil
// stdin is equivalent to an empty input stream.
func (s *Shell) newInterp(stdin io.Reader, stdout, stderr io.Writer) (*interp.Runner, error) {
	return newRunner(s.cwd, s.env, stdin, stdout, stderr, s.blockFuncs)
}

// updateShellFromRunner updates the shell from the interpreter after execution.
func (s *Shell) updateShellFromRunner(runner *interp.Runner) {
	s.cwd = runner.Dir
	s.env = s.env[:0]
	for name, vr := range runner.Vars {
		if vr.Exported {
			s.env = append(s.env, name+"="+vr.Str)
		}
	}
}

// execCommon is the shared implementation for executing commands
func (s *Shell) execCommon(ctx context.Context, command string, stdout, stderr io.Writer) (err error) {
	var runner *interp.Runner
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("command execution panic: %v", r)
		}
		if runner != nil {
			s.updateShellFromRunner(runner)
		}
		s.logger.InfoPersist("command finished", "command", command, "err", err)
	}()

	line, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return fmt.Errorf("could not parse command: %w", err)
	}

	runner, err = s.newInterp(nil, stdout, stderr)
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
	if exitErr, ok := errors.AsType[interp.ExitStatus](err); ok {
		return int(exitErr)
	}
	return 1
}
