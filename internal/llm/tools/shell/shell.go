package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/logging"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
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

func (s *PersistentShell) Exec(ctx context.Context, command string) (string, string, int, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	line, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return "", "", 1, false, fmt.Errorf("could not parse command: %w", err)
	}

	var stdout, stderr bytes.Buffer
	runner, err := interp.New(
		interp.StdIO(nil, &stdout, &stderr),
		interp.Interactive(false),
		interp.Env(expand.ListEnviron(s.env...)),
		interp.Dir(s.cwd),
	)
	if err != nil {
		return "", "", 1, false, fmt.Errorf("could not run command: %w", err)
	}

	var interrupted bool
	var exitStatus int
	err = runner.Run(ctx, line)
	if err != nil {
		exitStatus = errorExit(err)
		interrupted = errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded)
	}
	if interrupted {
		// the caller expects a nil err if interrupted
		err = nil
	}

	s.cwd = runner.Dir
	s.env = []string{}
	for name, vr := range runner.Vars {
		s.env = append(s.env, fmt.Sprintf("%s=%s", name, vr.Str))
	}
	logging.InfoPersist("Command finished", "command", command, "interrupted", interrupted, "exitStatus", exitStatus, "err", err)
	return stdout.String(), stderr.String(), exitStatus, interrupted, err
}

func errorExit(err error) int {
	status, ok := interp.IsExitStatus(err)
	if ok {
		return int(status)
	}
	return 1
}
