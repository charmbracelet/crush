package tools

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/crush/internal/log"
)

// rgMaxConcurrency limits the number of concurrent ripgrep processes spawned
// by this package. A bounded number prevents a runaway agent from spawning
// an unbounded number of expensive searches.
const rgMaxConcurrency = 8

var rgSem = make(chan struct{}, rgMaxConcurrency)

var getRg = sync.OnceValue(func() string {
	if testing.Testing() {
		return ""
	}
	path, err := exec.LookPath("rg")
	if err != nil {
		if log.Initialized() {
			slog.Warn("Ripgrep (rg) not found in $PATH. Some grep features might be limited or slower.")
		}
		return ""
	}
	return path
})

func getRgCmd(ctx context.Context, globPattern string) *rgCmd {
	name := getRg()
	if name == "" {
		return nil
	}
	args := []string{"--files", "-L", "--null"}
	if globPattern != "" {
		if !filepath.IsAbs(globPattern) && !strings.HasPrefix(globPattern, "/") {
			globPattern = "/" + globPattern
		}
		args = append(args, "--glob", globPattern)
	}
	return newRgCmd(ctx, name, args...)
}

func getRgSearchCmd(ctx context.Context, pattern, path, include string) *rgCmd {
	name := getRg()
	if name == "" {
		return nil
	}
	// Use -n to show line numbers, -0 for null separation to handle Windows paths
	args := []string{"--json", "-H", "-n", "-0", pattern}
	if include != "" {
		args = append(args, "--glob", include)
	}
	args = append(args, path)

	return newRgCmd(ctx, name, args...)
}

// rgCmd wraps an exec.Cmd with a package-level semaphore that limits the
// number of concurrent ripgrep processes. Callers can use the standard
// *exec.Cmd methods; the wrapper blocks on Start until a slot is available
// and releases the slot after Wait returns.
type rgCmd struct {
	*exec.Cmd
	ctx context.Context
}

func newRgCmd(ctx context.Context, name string, args ...string) *rgCmd {
	return &rgCmd{Cmd: exec.CommandContext(ctx, name, args...), ctx: ctx}
}

func (c *rgCmd) acquire() error {
	select {
	case rgSem <- struct{}{}:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *rgCmd) release() {
	<-rgSem
}

func (c *rgCmd) Start() error {
	if err := c.acquire(); err != nil {
		return err
	}
	if err := c.Cmd.Start(); err != nil {
		c.release()
		return err
	}
	return nil
}

func (c *rgCmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

func (c *rgCmd) Wait() error {
	defer c.release()
	return c.Cmd.Wait()
}

func (c *rgCmd) Output() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	var b bytes.Buffer
	c.Stdout = &b
	err := c.Run()
	return b.Bytes(), err
}

func (c *rgCmd) CombinedOutput() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if c.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	err := c.Run()
	return b.Bytes(), err
}
