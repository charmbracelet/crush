package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

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
	shell, err := interp.New(
		interp.StdIO(nil, &stdout, &stderr),
		interp.Interactive(false),
		interp.Env(expand.ListEnviron(s.env...)),
		interp.Dir(s.cwd),
	)
	if err != nil {
		return "", "", 1, false, fmt.Errorf("could not run command: %w", err)
	}

	if err := shell.Run(ctx, line); err != nil {
		status, ok := interp.IsExitStatus(err)
		if !ok {
			status = 1
		}
		interrupted := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
		return stdout.String(), stderr.String(), int(status), interrupted, err
	}

	s.cwd = shell.Dir
	s.env = []string{}
	for name, vr := range shell.Vars {
		if vr.Exported {
			s.env = append(s.env, fmt.Sprintf("%s=%s", name, vr.Str))
		}
	}
	return stdout.String(), stderr.String(), 0, false, nil
}
