package shell

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

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

func (s *PersistentShell) Exec(
	ctx context.Context,
	command string,
	timeout time.Duration,
) (string, string, int, bool, error) {
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

	doneCh := make(chan struct{}, 1)
	errCh := make(chan error, 1)
	runCtx, cancel := context.WithTimeout(ctx, cmp.Or(timeout, time.Hour*999))
	defer cancel()

	go func() {
		errCh <- runner.Run(runCtx, line)
	}()

	var interrupted bool
	var exitStatus int
	go func() {
	out:
		for {
			select {
			case err := <-errCh:
				status, ok := interp.IsExitStatus(err)
				if err != nil && !ok {
					status = 1
				}
				exitStatus = int(status)
				interrupted = errors.Is(err, context.Canceled) ||
					errors.Is(err, context.DeadlineExceeded)
				break out
			case <-runCtx.Done():
				interrupted = true
				exitStatus = 1
				break out
			case <-ctx.Done():
				interrupted = true
				exitStatus = 1
				cancel() // cancel the run context
				break out
			}
		}
		doneCh <- struct{}{}
	}()

	<-doneCh

	s.cwd = runner.Dir
	s.env = []string{}
	for name, vr := range runner.Vars {
		s.env = append(s.env, fmt.Sprintf("%s=%s", name, vr.Str))
	}

	logging.InfoPersist("Command finished", "command", command, "interrupted", interrupted, "exitStatus", exitStatus)
	return stdout.String(), stderr.String(), int(exitStatus), interrupted, err
}
