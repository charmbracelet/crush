package shellconfig

import (
	"context"
	"encoding/json"
	"fmt"
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
		return nil, fmt.Errorf("executing shell config %s: %w", path, err)
	}

	if len(builder.Fragments) == 0 {
		return nil, nil
	}

	merged, err := jsons.Merge(builder.Fragments)
	if err != nil {
		return nil, fmt.Errorf("merging shell config fragments from %s: %w", path, err)
	}

	// Validate that the merged result is a valid JSON object.
	if !json.Valid(merged) {
		return nil, fmt.Errorf("shell config %s produced invalid JSON", path)
	}

	return merged, nil
}
