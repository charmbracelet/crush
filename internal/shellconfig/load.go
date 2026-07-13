package shellconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/qjebbs/go-jsons"
)

// LoadShellConfig executes a crush.sh script and returns the merged JSON
// config bytes. The script uses config builtins (provider, model, mcp, etc.)
// that accumulate JSON fragments on a ConfigBuilder. After execution, the
// fragments are deep-merged into a single JSON object.
//
// The script runs with the same shell interpreter used by the bash tool and
// hooks, so source, $VAR, $(cmd), and other shell constructs all work.
func LoadShellConfig(path string, src []byte) ([]byte, error) {
	slog.Info("Loading shell config", "path", path)

	builder := &shell.ConfigBuilder{}
	ctx := shell.WithConfigBuilder(context.Background(), builder)

	cwd := filepath.Dir(path)
	env := os.Environ()

	err := shell.Run(ctx, shell.RunOptions{
		Command: string(src),
		Cwd:     cwd,
		Env:     env,
	})
	if err != nil {
		slog.Error("Shell config execution failed", "path", path, "error", err)
		return nil, fmt.Errorf("executing shell config %s: %w", path, err)
	}

	slog.Info("Shell config executed successfully", "path", path, "fragments", len(builder.Fragments))

	if len(builder.Fragments) == 0 {
		slog.Warn("Shell config produced no config fragments", "path", path)
		return nil, nil
	}

	merged, err := jsons.Merge(builder.Fragments)
	if err != nil {
		slog.Error("Failed to merge shell config fragments", "path", path, "error", err)
		return nil, fmt.Errorf("merging shell config fragments from %s: %w", path, err)
	}

	// Validate that the merged result is a valid JSON object.
	if !json.Valid(merged) {
		slog.Error("Shell config produced invalid JSON", "path", path)
		return nil, fmt.Errorf("shell config %s produced invalid JSON", path)
	}

	slog.Info("Shell config loaded successfully", "path", path, "bytes", len(merged))
	return merged, nil
}
