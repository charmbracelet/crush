package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/slicesext"
	"mvdan.cc/sh/moreinterp/coreutils"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// BlockFunc is a function that determines if a command should be blocked
type BlockFunc func(args []string) bool

// Shell provides cross-platform shell execution with optional state persistence
type Shell struct {
	env        []string
	cwd        string
	blockFuncs []BlockFunc
}

// Options for creating a new shell
type Options struct {
	WorkingDir string
	Env        []string
	BlockFuncs []BlockFunc
}

// NewShell creates a new shell instance with the given options
func NewShell(opts Options) *Shell {
	sh := &Shell{
		cwd:        opts.WorkingDir,
		env:        opts.Env,
		blockFuncs: opts.BlockFuncs,
	}
	if sh.cwd == "" {
		sh.cwd, _ = os.Getwd()
	}
	if sh.env == nil {
		sh.env = os.Environ()
	}
	return sh
}

// Exec executes a command in the shell
func (s *Shell) Exec(ctx context.Context, command string) (string, string, error) {
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
		interp.ExecHandlers(s.blockHandler(), coreutils.ExecHandler),
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
	slog.Info("POSIX command finished", "command", command, "err", err)
	return stdout.String(), stderr.String(), err
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
		flagsMatch := slicesext.IsSubset(flags, flagParts)

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
			if idx := strings.IndexByte(part, '='); idx != -1 {
				flag = part[:idx]
			}
			flags = append(flags, flag)
		} else {
			args = append(args, part)
		}
	}
	return
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
