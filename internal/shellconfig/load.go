package shellconfig

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/version"
)

// LoadShellConfig executes a crush.sh script and returns its config as a
// single JSON object. The script uses config builtins (provider, model, mcp,
// etc.) that mutate a ConfigBuilder in execution order; the builder is then
// marshaled to JSON, which the config loader merges with any other config
// files.
//
// The script runs with the same shell interpreter used by the bash tool and
// hooks, so source, $VAR, $(cmd), and other shell constructs all work.
func LoadShellConfig(path string, src []byte) ([]byte, error) {
	slog.Info("Loading shell config", "path", path)

	builder := newConfigBuilder()
	ctx := withConfigBuilder(context.Background(), builder)

	cwd := filepath.Dir(path)

	// Expose the running Crush version so scripts can feature-detect, e.g.
	// [[ "$CRUSH_VERSION" == "devel" ]] or branch on the release.
	env := append(os.Environ(), "CRUSH_VERSION="+version.Version)

	err := shell.Run(ctx, shell.RunOptions{
		Command: string(src),
		Cwd:     cwd,
		Env:     env,
	})
	if err != nil {
		slog.Error("Shell config execution failed", "path", path, "error", err)
		return nil, fmt.Errorf("executing shell config %s: %w", path, err)
	}

	if builder.empty() {
		slog.Warn("Shell config produced no config", "path", path)
		return nil, nil
	}

	data, err := builder.JSON()
	if err != nil {
		slog.Error("Failed to marshal shell config", "path", path, "error", err)
		return nil, fmt.Errorf("marshaling shell config %s: %w", path, err)
	}

	slog.Info("Shell config loaded successfully", "path", path, "bytes", len(data))
	return data, nil
}
