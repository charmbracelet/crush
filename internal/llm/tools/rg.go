package tools

import (
	"context"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/log"
)

var getRg = sync.OnceValue(func() string {
	path, err := exec.LookPath("rg")
	if err != nil {
		if log.Initialized() {
			slog.Warn("Ripgrep (rg) not found in $PATH. Some grep features might be limited or slower.")
		}
		return ""
	}
	return path
})

func getRgCmd(ctx context.Context, globPattern string) *exec.Cmd {
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
	return exec.CommandContext(ctx, name, args...)
}

type execCmd interface {
	AddArgs(arg ...string)
	Output() ([]byte, error)
}

type wrappedCmd struct {
	cmd *exec.Cmd
}

func (w wrappedCmd) AddArgs(arg ...string) {
	w.cmd.Args = append(w.cmd.Args, arg...)
}

func (w wrappedCmd) Output() ([]byte, error) {
	return w.cmd.Output()
}

type resolveRgSearchCmd func(ctx context.Context, pattern, path, include string) execCmd

func getRgSearchCmd(ctx context.Context, pattern, path, include string) execCmd {
	name := getRg()
	if name == "" {
		return nil
	}
	// Use -n to show line numbers, -0 for null separation to handle Windows paths
	args := []string{"-H", "-n", "-0", pattern}
	if include != "" {
		args = append(args, "--glob", include)
	}
	args = append(args, path)

	return wrappedCmd{
		cmd: exec.CommandContext(ctx, name, args...),
	}
}
