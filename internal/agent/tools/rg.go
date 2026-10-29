package tools

import (
	"context"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/crush/internal/log"
)

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

func getRgCmd(ctx context.Context, globPattern string) *exec.Cmd {
	name := getRg()
	if name == "" {
		return nil
	}
	// Note: we intentionally do not pass -L (follow symlinks). Following
	// symlinks lets rg escape the search root (into module caches, the nix
	// store, $HOME, etc.) and chase cycles, which pins all cores and can
	// hang. This keeps glob scoped to the tree it was pointed at, matching
	// the grep search command.
	args := []string{"--files", "--null"}
	if globPattern != "" {
		if !filepath.IsAbs(globPattern) && !strings.HasPrefix(globPattern, "/") {
			globPattern = "/" + globPattern
		}
		args = append(args, "--glob", globPattern)
	}
	return exec.CommandContext(ctx, name, args...)
}

func rgSearchArgs(pattern, path, include string) []string {
	// -e keeps a pattern that starts with "-" from being parsed as a flag.
	// A bare "--" does not work here: searchWithRipgrep appends --ignore-file
	// after the path, and ripgrep would treat that option as another path.
	args := []string{"--json", "-H", "-n", "-0", "-e", pattern}
	if include != "" {
		args = append(args, "--glob", include)
	}
	args = append(args, path)
	return args
}

func getRgSearchCmd(ctx context.Context, pattern, path, include string) *exec.Cmd {
	name := getRg()
	if name == "" {
		return nil
	}
	return exec.CommandContext(ctx, name, rgSearchArgs(pattern, path, include)...)
}
